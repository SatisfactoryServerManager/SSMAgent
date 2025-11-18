package mod

import (
	"context"
	"fmt"
	"time"

	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/mod"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type Handler struct {
	conn       *grpc.ClientConn
	client     pb.AgentModConfigServiceClient
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
		client:     pb.NewAgentModConfigServiceClient(conn),
		context:    ctx,
		masterDone: make(chan struct{}),
	}
}

func (h *Handler) Run() {
	go h.PollModConfig()
}

// Poll once per second just like PollTasks
func (h *Handler) PollModConfig() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cfg, err := h.GetModConfig()
			if err != nil {
				utils.ErrorLogger.Println(err.Error())
				continue
			}

			if cfg == nil {
				continue
			}

			// Process mod config
			h.ProcessModConfig(cfg)

			// Send it back
			if err := h.UpdateModConfig(cfg); err != nil {
				utils.ErrorLogger.Println("Error updating mod config:", err.Error())
			}

		case <-h.masterDone:
			ticker.Stop()
			return
		}
	}
}

func (h *Handler) GetModConfig() (*pb.ModConfig, error) {
	resp, err := h.client.GetModConfig(h.context, &pb.Empty{})
	if err != nil {
		return nil, fmt.Errorf("error getting mod config: %s", err.Error())
	}

	return resp.Config, nil
}

func (h *Handler) UpdateModConfig(cfg *pb.ModConfig) error {
	_, err := h.client.UpdateModConfig(
		h.context,
		&pb.AgentModConfigRequest{Config: cfg},
	)
	return err
}

// Logic for processing / updating the mods on the client
func (h *Handler) ProcessModConfig(cfg *pb.ModConfig) error {
	modConfig := &v2.AgentModConfig{}
	utils.CopyStruct(cfg, modConfig)

	if err := mod.ProcessModConfig(modConfig); err != nil {
		return err
	}

	utils.CopyStruct(modConfig, cfg)

	return nil
}

func (h *Handler) Stop() {
	close(h.masterDone)
}
