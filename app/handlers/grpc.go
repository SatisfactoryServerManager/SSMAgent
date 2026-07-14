package handlers

import (
	"context"
	"os"
	"time"

	mainConfig "github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/handlers/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/handlers/file"
	"github.com/SatisfactoryServerManager/SSMAgent/app/handlers/log"
	"github.com/SatisfactoryServerManager/SSMAgent/app/handlers/state"
	"github.com/SatisfactoryServerManager/SSMAgent/app/handlers/task"
	taskservice "github.com/SatisfactoryServerManager/SSMAgent/app/services/task"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

var (
	stateHandler  *state.Handler
	taskHandler   *task.Handler
	taskExecutor  *taskservice.Executor
	logHandler    *log.Handler
	configHandler *config.Handler
)

func NewGRPCConnection(addr string) (*grpc.ClientConn, error) {

	cfg := grpc.ConnectParams{
		MinConnectTimeout: 5 * time.Second,
		Backoff: backoff.Config{
			BaseDelay:  1 * time.Second,
			Multiplier: 1.6,
			MaxDelay:   15 * time.Second,
		},
	}

	ka := keepalive.ClientParameters{
		Time:                10 * time.Second,
		Timeout:             3 * time.Second,
		PermitWithoutStream: true,
	}

	// When the backend serves plaintext gRPC (local/dev), use insecure
	// credentials. Controlled by the --grpcinsecure flag (SSM_INSECURE in the
	// container) or APP_MODE=development for `go run` workflows.
	creds := credentials.NewTLS(nil)
	if mainConfig.GetConfig().GRPCInsecure || os.Getenv("APP_MODE") == "development" {
		creds = insecure.NewCredentials()
		utils.InfoLogger.Println("Using insecure gRPC credentials")
	}

	return grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(creds),
		grpc.WithConnectParams(cfg),
		grpc.WithKeepaliveParams(ka),
	)
}

func EnsureConnected(conn *grpc.ClientConn) {
	if conn.GetState() == connectivity.TransientFailure || conn.GetState() == connectivity.Shutdown {
		utils.DebugLogger.Println("gRPC connection is in state", conn.GetState(), "reconnecting...")
		conn.Connect()
	}
}

func InitgRPC() error {
	grpcAddr := mainConfig.GetConfig().GRPCAddress
	grpcConn, err := NewGRPCConnection(grpcAddr)
	if err != nil {
		return err
	}

	stateHandler = state.NewHandler(grpcConn)
	stateHandler.Run()

	taskHandler = task.NewHandler(grpcConn)

	// Wire the client before any handler can run, so syncmods always has a way to
	// report back what it installed.
	taskservice.SetClient(taskHandler.Client())
	taskservice.RegisterDefaults()
	taskExecutor = taskservice.NewExecutor(taskHandler.Client(), taskHandler.Context())
	taskExecutor.Start()

	task.SetSink(taskExecutor)
	taskHandler.Run()

	logHandler = log.NewHandler(grpcConn)
	logHandler.Run()

	configHandler = config.NewHandler(grpcConn)
	configHandler.Run()

	file.Init(grpcConn)

	return nil
}

// StopAcceptingTasks closes the task subscription so no new work arrives. The
// in-flight task keeps running; DrainTasks deals with it.
func StopAcceptingTasks(ctx context.Context) error {
	if taskHandler == nil {
		return nil
	}
	return taskHandler.StopAccepting(ctx)
}

// DrainTasks releases the running task back to the queue so another agent run
// picks it up rather than waiting out its lease.
func DrainTasks(ctx context.Context) error {
	if taskExecutor == nil {
		return nil
	}
	return taskExecutor.DrainAndRelease(ctx)
}

func ShutdownGRPCClient() error {
	stateHandler.Stop()
	taskHandler.Stop()
	configHandler.Stop()
	logHandler.Stop()
	return nil
}
