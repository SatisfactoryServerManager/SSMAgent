package state

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/state"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto"
)

type Handler struct {
	conn   *grpc.ClientConn
	client pb.AgentStateServiceClient
	stream pb.AgentStateService_UpdateAgentStateStreamClient

	loopCtx    context.Context
	loopCancel context.CancelFunc

	streamCtx    context.Context
	streamCancel context.CancelFunc

	masterDone chan struct{}
}

func NewHandler(conn *grpc.ClientConn) *Handler {
	ctx, cancel := context.WithCancel(context.Background())

	return &Handler{
		conn:       conn,
		client:     pb.NewAgentStateServiceClient(conn),
		loopCtx:    ctx,
		loopCancel: cancel,
		masterDone: make(chan struct{}),
	}
}

func (h *Handler) Run() {
	state.MarkAgentOnline()
	go h.reconnectLoop()
}

func (h *Handler) reconnectLoop() {
	backoff := time.Second

	for {
		select {
		case <-h.masterDone:
			utils.DebugLogger.Println("Stopping reconnection loop")
			return
		default:
		}

		if err := h.openStream(); err != nil {
			utils.DebugLogger.Println("Failed to open stream:", err)
			time.Sleep(backoff)
			continue
		}

		utils.DebugLogger.Println("State stream opened")

		if err := h.sendStateLoop(); err != nil {
			utils.DebugLogger.Println("State stream ended:", err)
		}

		h.closeStream()

		time.Sleep(backoff)
	}
}

func (h *Handler) openStream() error {
	// Create a NEW context for the stream each reconnect
	ctx, cancel := context.WithCancel(h.loopCtx)
	h.streamCtx = contextWithAPIKey(ctx)
	h.streamCancel = cancel

	stream, err := h.client.UpdateAgentStateStream(h.streamCtx)
	if err != nil {
		return err
	}

	h.stream = stream
	return nil
}

func contextWithAPIKey(ctx context.Context) context.Context {
	apiKey := config.GetConfig().APIKey
	return metadata.AppendToOutgoingContext(ctx, "x-api-key", apiKey)
}

func (h *Handler) sendStateLoop() error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-h.streamCtx.Done():
			return fmt.Errorf("stream context closed")

		case <-ticker.C:
			if err := h.sendState(); err != nil {
				return err
			}
		}
	}
}

func (h *Handler) sendState() error {
	state.InstalledSFVersion = config.GetConfig().SF.InstalledVer
	state.LatestSFVersion = config.GetConfig().SF.AvilableVer

	payload := &pb.AgentStateRequest{
		Online:             state.Online,
		Installed:          state.Installed,
		Running:            state.Running,
		Cpu:                state.CPU,
		Ram:                state.MEM,
		InstalledSFVersion: state.InstalledSFVersion,
		LatestSFVersion:    state.LatestSFVersion,
	}

	if h.stream == nil {
		_, err := h.client.UpdateAgentState(contextWithAPIKey(context.Background()), payload)
		return err
	}

	//utils.DebugLogger.Printf("sending state: %v\n", payload)
	return h.stream.Send(payload)
}

func (h *Handler) closeStream() {
	if h.streamCancel != nil {
		h.streamCancel()
	}

	if h.stream != nil {
		_ = h.stream.CloseSend()
	}
}

func (h *Handler) Stop() {

	utils.DebugLogger.Println("Stopping State Handler")
	h.sendState()

	// Stop the reconnection loop
	close(h.masterDone)

	// Cancel the outer loop context
	if h.loopCancel != nil {
		h.loopCancel()
	}

	// Cancel the current stream
	h.closeStream()
	utils.DebugLogger.Println("Stopped State Handler")
}
