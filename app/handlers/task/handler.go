package task

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/task"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type Handler struct {
	conn       *grpc.ClientConn
	client     pb.AgentTaskServiceClient
	context    context.Context
	masterDone chan struct{}
}

func contextWithAPIKey(ctx context.Context) context.Context {
	apiKey := config.GetConfig().APIKey
	return metadata.AppendToOutgoingContext(ctx, "x-api-key", apiKey)
}

func NewHandler(conn *grpc.ClientConn) *Handler {
	ctx := contextWithAPIKey(context.Background())
	return &Handler{
		conn:       conn,
		client:     pb.NewAgentTaskServiceClient(conn),
		context:    ctx,
		masterDone: make(chan struct{}),
	}
}

func (h *Handler) Run() {
	go h.PollTasks()
}

func (h *Handler) PollTasks() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tasks, err := h.GetTasks()
			if err != nil {
				utils.ErrorLogger.Println(err.Error())
			}

			for idx := range tasks {
				theTask := &tasks[idx]
				if err := task.ProcessMessageQueueItem(theTask); err != nil {
					utils.ErrorLogger.Printf("Error processing task item %s (%s) with error: %s\r\n", theTask.Action, theTask.ID.Hex(), err.Error())
					h.MarkAgentTaskFailed(&pb.AgentTaskFailedRequest{Id: theTask.ID.Hex()})
					continue
				}
				h.MarkAgentTaskCompleted(&pb.AgentTaskCompletedRequest{Id: theTask.ID.Hex()})
			}
		case <-h.masterDone:
			ticker.Stop()
			return
		}
	}
}

func (h *Handler) GetTasks() ([]v2.AgentTask, error) {
	resp, err := h.client.GetAgentTasks(h.context, &pb.Empty{})
	if err != nil {
		return nil, fmt.Errorf("error getting agent tasks with error: %s", err.Error())
	}

	tasks := make([]v2.AgentTask, 0)
	for _, t := range resp.Tasks {
		// parse ObjectID
		objID, err := primitive.ObjectIDFromHex(t.Id)
		if err != nil {
			fmt.Println("Invalid task ID:", t.Id)
			continue
		}

		// parse Data JSON into a map or concrete struct
		var data interface{}
		if t.Data != "" {
			err := json.Unmarshal([]byte(t.Data), &data)
			if err != nil {
				fmt.Println("Failed to unmarshal data:", err)
			}
		}

		task := v2.AgentTask{
			ID:        objID,
			Action:    t.Action,
			Data:      data,
			Completed: t.Completed,
			Retries:   int(t.Retries),
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

func (h *Handler) MarkAgentTaskCompleted(req *pb.AgentTaskCompletedRequest) error {
	_, err := h.client.MarkAgentTaskCompleted(
		h.context,
		req,
	)
	return err
}

func (h *Handler) MarkAgentTaskFailed(req *pb.AgentTaskFailedRequest) error {
	_, err := h.client.MarkAgentTaskFailed(
		h.context,
		req,
	)
	return err
}

func (h *Handler) Stop() {
	close(h.masterDone)
}
