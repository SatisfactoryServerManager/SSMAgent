package file

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated"
	pbModels "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated/models"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	chunkSize   = 64 * 1024
	maxAttempts = 5
	baseBackoff = time.Second
	maxBackoff  = 15 * time.Second
)

type Handler struct {
	conn   *grpc.ClientConn
	client pb.AgentFileServiceClient
}

func NewHandler(conn *grpc.ClientConn) *Handler {
	return &Handler{
		conn:   conn,
		client: pb.NewAgentFileServiceClient(conn),
	}
}

// defaultHandler is the process-wide file transfer client, initialised by Init
// from the gRPC bootstrap. Package-level helpers delegate to it so consumers
// (savemanager, backupmanager) don't need to import the handlers bootstrap
// package, avoiding an import cycle.
var defaultHandler *Handler

func Init(conn *grpc.ClientConn) {
	defaultHandler = NewHandler(conn)
}

func Upload(ctx context.Context, kind pb.FileKind, localPath string) error {
	return defaultHandler.UploadFile(ctx, kind, localPath)
}

func Download(ctx context.Context, remoteFilename, localPath string) error {
	return defaultHandler.DownloadFile(ctx, remoteFilename, localPath)
}

func GetSaveSyncItems(ctx context.Context) ([]*pb.SaveSyncItem, error) {
	return defaultHandler.GetSaveSync(ctx)
}

func PostSaveSyncItems(ctx context.Context, items []*pb.SaveSyncItem) error {
	return defaultHandler.PostSaveSync(ctx, items)
}

func contextWithAPIKey(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "x-api-key", config.GetConfig().APIKey)
}

// transferID is a deterministic id for a (apiKey, kind, filename) tuple so that
// a retried/resumed transfer maps to the same server-side staging file.
func transferID(apiKey string, kind pb.FileKind, filename string) string {
	sum := sha256.Sum256([]byte(apiKey))
	return fmt.Sprintf("%s_%d_%s", hex.EncodeToString(sum[:])[:8], int32(kind), filepath.Base(filename))
}

func backoffFor(attempt int) time.Duration {
	d := baseBackoff * time.Duration(1<<attempt)
	if d > maxBackoff {
		d = maxBackoff
	}
	return d
}

// UploadFile streams localPath to the backend, resuming from the server-staged
// offset on reconnect. Retries with backoff on transient errors.
func (h *Handler) UploadFile(ctx context.Context, kind pb.FileKind, localPath string) error {
	id := transferID(config.GetConfig().APIKey, kind, localPath)
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(backoffFor(attempt))
		}
		if err := h.uploadOnce(ctx, id, kind, localPath); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return fmt.Errorf("upload failed after %d attempts: %w", maxAttempts, lastErr)
}

func (h *Handler) uploadOnce(ctx context.Context, id string, kind pb.FileKind, localPath string) error {
	mctx := contextWithAPIKey(ctx)

	off, err := h.client.GetUploadOffset(mctx, &pb.UploadOffsetRequest{TransferId: id})
	if err != nil {
		return err
	}

	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Seek(off.Offset, io.SeekStart); err != nil {
		return err
	}

	stream, err := h.client.UploadFile(mctx)
	if err != nil {
		return err
	}

	if err := stream.Send(&pb.UploadFileRequest{
		Data: &pb.UploadFileRequest_Init{Init: &pb.UploadInit{
			TransferId: id,
			Kind:       kind,
			Filename:   filepath.Base(localPath),
		}},
	}); err != nil {
		return err
	}

	buf := make([]byte, chunkSize)
	for {
		n, rerr := f.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			if serr := stream.Send(&pb.UploadFileRequest{
				Data: &pb.UploadFileRequest_Chunk{Chunk: chunk},
			}); serr != nil {
				return serr
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		return err
	}
	if !resp.Success {
		return errors.New(resp.Message)
	}
	return nil
}

// DownloadFile writes the remote file to localPath, resuming from the current
// local file length on reconnect. Retries with backoff on transient errors.
func (h *Handler) DownloadFile(ctx context.Context, remoteFilename, localPath string) error {
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(backoffFor(attempt))
		}
		if err := h.downloadOnce(ctx, remoteFilename, localPath); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return fmt.Errorf("download failed after %d attempts: %w", maxAttempts, lastErr)
}

func (h *Handler) downloadOnce(ctx context.Context, remoteFilename, localPath string) error {
	mctx := contextWithAPIKey(ctx)

	var start int64
	if fi, err := os.Stat(localPath); err == nil {
		start = fi.Size()
	}

	f, err := os.OpenFile(localPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	stream, err := h.client.DownloadFile(mctx, &pb.DownloadFileRequest{
		Filename:    remoteFilename,
		StartOffset: start,
	})
	if err != nil {
		return err
	}

	for {
		chunk, rerr := stream.Recv()
		if rerr == io.EOF {
			return nil
		}
		if rerr != nil {
			return rerr
		}
		if _, werr := f.Write(chunk.Chunk); werr != nil {
			return werr
		}
	}
}

func (h *Handler) GetSaveSync(ctx context.Context) ([]*pb.SaveSyncItem, error) {
	resp, err := h.client.GetSaveSync(contextWithAPIKey(ctx), &pbModels.SSMEmpty{})
	if err != nil {
		return nil, err
	}
	return resp.Saves, nil
}

func (h *Handler) PostSaveSync(ctx context.Context, items []*pb.SaveSyncItem) error {
	_, err := h.client.PostSaveSync(contextWithAPIKey(ctx), &pb.PostSaveSyncRequest{Saves: items})
	return err
}
