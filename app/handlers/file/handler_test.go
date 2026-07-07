package file

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated"
	pbModels "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated/models"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestTransferIDStable(t *testing.T) {
	a := transferID("KEY", pb.FileKind_FILE_KIND_SAVE, "world.sav")
	b := transferID("KEY", pb.FileKind_FILE_KIND_SAVE, "world.sav")
	if a != b {
		t.Fatalf("transferID not deterministic: %q vs %q", a, b)
	}
	if a == transferID("KEY", pb.FileKind_FILE_KIND_BACKUP, "world.sav") {
		t.Fatalf("different kinds must produce different ids")
	}
}

// stubServer implements AgentFileServiceServer with in-memory staging and an
// injectable failure on the first UploadFile attempt to exercise resume.
type stubServer struct {
	pb.UnimplementedAgentFileServiceServer
	staged     map[string][]byte
	failFirst  bool
	uploadDone bool
	download   []byte
}

func (s *stubServer) GetUploadOffset(_ context.Context, in *pb.UploadOffsetRequest) (*pb.UploadOffsetResponse, error) {
	return &pb.UploadOffsetResponse{Offset: int64(len(s.staged[in.TransferId]))}, nil
}

func (s *stubServer) UploadFile(stream pb.AgentFileService_UploadFileServer) error {
	var id string
	received := 0
	for {
		req, err := stream.Recv()
		if err != nil {
			break
		}
		switch d := req.Data.(type) {
		case *pb.UploadFileRequest_Init:
			id = d.Init.TransferId
		case *pb.UploadFileRequest_Chunk:
			s.staged[id] = append(s.staged[id], d.Chunk...)
			received++
			if s.failFirst && received == 1 {
				s.failFirst = false
				return errors.New("simulated drop")
			}
		}
	}
	s.uploadDone = true
	return stream.SendAndClose(&pb.UploadFileResponse{Success: true})
}

func (s *stubServer) DownloadFile(in *pb.DownloadFileRequest, stream pb.AgentFileService_DownloadFileServer) error {
	data := s.download[in.StartOffset:]
	return stream.Send(&pb.DownloadChunk{Chunk: data})
}

func (s *stubServer) GetSaveSync(context.Context, *pbModels.SSMEmpty) (*pb.GetSaveSyncResponse, error) {
	return &pb.GetSaveSyncResponse{Saves: []*pb.SaveSyncItem{{FileName: "a.sav"}}}, nil
}

func dialStub(t *testing.T, srv *stubServer) *grpc.ClientConn {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	gs := grpc.NewServer()
	pb.RegisterAgentFileServiceServer(gs, srv)
	go gs.Serve(lis)
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return conn
}

func TestUploadResumesAfterDrop(t *testing.T) {
	config.SetConfig(&config.Config{APIKey: "testkey"})
	srv := &stubServer{staged: map[string][]byte{}, failFirst: true}
	h := NewHandler(dialStub(t, srv))

	// Source larger than one chunk so the first (failed) attempt is partial.
	src := bytes.Repeat([]byte("A"), chunkSize*2+123)
	dir := t.TempDir()
	path := filepath.Join(dir, "world.sav")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := h.UploadFile(context.Background(), pb.FileKind_FILE_KIND_SAVE, path); err != nil {
		t.Fatalf("UploadFile: %v", err)
	}

	id := transferID(config.GetConfig().APIKey, pb.FileKind_FILE_KIND_SAVE, path)
	if got := srv.staged[id]; !bytes.Equal(got, src) {
		t.Fatalf("resumed upload mismatch: got %d bytes want %d", len(got), len(src))
	}
}

func TestDownloadResumesFromOffset(t *testing.T) {
	config.SetConfig(&config.Config{APIKey: "testkey"})
	full := bytes.Repeat([]byte("B"), 500)
	srv := &stubServer{staged: map[string][]byte{}, download: full}
	h := NewHandler(dialStub(t, srv))

	dir := t.TempDir()
	path := filepath.Join(dir, "out.sav")
	// Pre-seed a partial local file; download should append the remainder.
	if err := os.WriteFile(path, full[:200], 0o644); err != nil {
		t.Fatal(err)
	}

	if err := h.DownloadFile(context.Background(), "out.sav", path); err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}

	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, full) {
		t.Fatalf("resumed download mismatch: got %d bytes want %d", len(got), len(full))
	}
}
