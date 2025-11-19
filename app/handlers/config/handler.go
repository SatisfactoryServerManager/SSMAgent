package config

import (
	"context"
	"fmt"
	"time"

	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/sf"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type Handler struct {
	conn       *grpc.ClientConn
	client     pb.AgentConfigServiceClient
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
		client:     pb.NewAgentConfigServiceClient(conn),
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
			if err := h.GetConfig(); err != nil {
				utils.ErrorLogger.Println(err.Error())
			}

		case <-h.masterDone:
			ticker.Stop()
			return
		}
	}
}

func (h *Handler) GetConfig() error {
	resConfig, err := h.client.GetAgentConfig(h.context, &pb.Empty{})
	if err != nil {
		return fmt.Errorf("error getting agent config with error: %s", err.Error())
	}

	oldBranch := config.GetConfig().SF.SFBranch

	config.GetConfig().Backup.Interval = int(resConfig.Config.BackupInterval)
	config.GetConfig().Backup.Keep = int(resConfig.Config.BackupKeepAmount)

	config.GetConfig().SF.MaxPlayers = int(resConfig.ServerConfig.MaxPlayers)
	config.GetConfig().SF.WorkerThreads = int(resConfig.ServerConfig.WorkerThreads)
	config.GetConfig().SF.SFBranch = resConfig.ServerConfig.Branch

	config.GetConfig().SF.UpdateSFOnStart = resConfig.ServerConfig.UpdateSFOnStart
	config.GetConfig().SF.AutoRestart = resConfig.ServerConfig.AutoRestart
	config.GetConfig().SF.AutoPause = resConfig.ServerConfig.AutoPause
	config.GetConfig().SF.AutoSaveOnDisconnect = resConfig.ServerConfig.AutoSaveOnDisconnect
	config.GetConfig().SF.AutoSaveInterval = float32(resConfig.ServerConfig.AutoSaveInterval)
	config.GetConfig().SF.DisableSeasonalEvents = resConfig.ServerConfig.DisableSeasonalEvents

	config.SaveConfig()

	if oldBranch != config.GetConfig().SF.SFBranch {
		sf.GetLatestedVersion()
	}

	if !sf.IsInstalled() {
		return nil
	}

	if sf.IsRunning() {
		return nil
	}

	if err := config.UpdateIniFiles(); err != nil {
		utils.ErrorLogger.Printf("error updating game ini files with error: %s\n", err.Error())
	}

	return nil
}

func (h *Handler) Stop() {
	utils.DebugLogger.Println("Stopping Config Handler")
	close(h.masterDone)
	utils.DebugLogger.Println("Stopped Config Handler")
}
