package task

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type Handler struct {
	conn   *grpc.ClientConn
	client pb.AgentTaskServiceClient

	connectionID string

	mu        sync.Mutex
	accepting bool

	masterDone chan struct{}
	stopOnce   sync.Once
}

// Sink receives assignments from the stream. The executor implements it (Task 12).
type Sink interface {
	Submit(a *pb.TaskAssignment)
	RunningTask() (taskID string, leaseToken string)
}

var sink Sink

func SetSink(s Sink) { sink = s }

func contextWithAPIKey(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "x-api-key", config.GetConfig().APIKey)
}

func NewHandler(conn *grpc.ClientConn) *Handler {
	return &Handler{
		conn:         conn,
		client:       pb.NewAgentTaskServiceClient(conn),
		connectionID: uuid.NewString(),
		accepting:    true,
		masterDone:   make(chan struct{}),
	}
}

func (h *Handler) Client() pb.AgentTaskServiceClient { return h.client }

func (h *Handler) Context() context.Context { return contextWithAPIKey(context.Background()) }

func (h *Handler) Run() {
	go h.subscribeLoop()
}

// StopAccepting closes the subscription so no further assignments arrive. The
// executor drains separately, so an in-flight task keeps running.
func (h *Handler) StopAccepting(ctx context.Context) error {
	h.mu.Lock()
	h.accepting = false
	h.mu.Unlock()

	utils.InfoLogger.Println("Task client stopped accepting new tasks")
	return nil
}

func (h *Handler) isAccepting() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.accepting
}

func (h *Handler) subscribeLoop() {
	backoff := time.Second

	for {
		select {
		case <-h.masterDone:
			return
		default:
		}

		if !h.isAccepting() {
			return
		}

		if err := h.subscribe(); err != nil && err != io.EOF {
			utils.ErrorLogger.Printf("task stream error: %s", err.Error())
		}

		select {
		case <-h.masterDone:
			return
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > 15*time.Second {
			backoff = 15 * time.Second
		}
	}
}

func (h *Handler) subscribe() error {
	ctx, cancel := context.WithCancel(contextWithAPIKey(context.Background()))
	defer cancel()

	go func() {
		<-h.masterDone
		cancel()
	}()

	req := &pb.SubscribeTasksRequest{
		AgentVersion: config.GetConfig().Version,
		ConnectionId: h.connectionID,
	}

	if sink != nil {
		req.RunningTaskId, req.RunningLeaseToken = sink.RunningTask()
	}

	stream, err := h.client.SubscribeTasks(ctx, req)
	if err != nil {
		return err
	}

	utils.InfoLogger.Println("Subscribed to agent task stream")

	for {
		assignment, err := stream.Recv()
		if err != nil {
			return err
		}

		if !h.isAccepting() {
			return nil
		}

		utils.InfoLogger.Printf("Received task %s (%s)", assignment.TaskId, assignment.Action)

		if sink == nil {
			utils.ErrorLogger.Println("No task sink registered, dropping assignment")
			continue
		}
		sink.Submit(assignment)
	}
}

func (h *Handler) Stop() {
	h.stopOnce.Do(func() {
		utils.DebugLogger.Println("Stopping Task Handler")
		close(h.masterDone)
		utils.DebugLogger.Println("Stopped Task Handler")
	})
}
