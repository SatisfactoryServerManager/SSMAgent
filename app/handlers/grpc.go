package handlers

import (
	"time"

	mainConfig "github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/handlers/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/handlers/log"
	"github.com/SatisfactoryServerManager/SSMAgent/app/handlers/state"
	"github.com/SatisfactoryServerManager/SSMAgent/app/handlers/task"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

var (
	stateHandler  *state.Handler
	taskHandler   *task.Handler
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

	return grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(credentials.NewTLS(nil)),
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
	taskHandler.Run()

	logHandler = log.NewHandler(grpcConn)
	logHandler.Run()

	configHandler = config.NewHandler(grpcConn)

	return err
}

func ShutdownGRPCClient() error {
	stateHandler.Stop()
	taskHandler.Stop()
	logHandler.Stop()
	return nil
}

// func GetAgentServiceClient() *GRPCClient {
// 	return AgentGRPCClient
// }

// func (c *GRPCClient) GetClient() pb.AgentServiceClient {
// 	EnsureConnected(c.conn)
// 	return c.client
// }

// func (c *GRPCClient) GetConfig() (*pb.AgentConfigResponse, error) {

// 	EnsureConnected(c.conn)
// 	ctx := ContextWithAPIKey(context.Background())
// 	resp, err := c.client.GetAgentConfig(
// 		ctx,
// 		&pb.Empty{},
// 	)
// 	return resp, err
// }

// func (c *GRPCClient) UpdateConfigVersionIp() error {

// 	EnsureConnected(c.conn)
// 	ctx := ContextWithAPIKey(context.Background())

// 	ip, err := GetPublicIP()
// 	if err != nil {
// 		ip = ""
// 	}

// 	_, err = c.client.UpdateAgentConfigVersionIp(
// 		ctx,
// 		&pb.AgentConfigRequest{
// 			Version: config.GetConfig().Version,
// 			Ip:      ip,
// 		},
// 	)
// 	return err
// }

// func (c *GRPCClient) UpdateAgentState(req *pb.AgentStateRequest) error {

// 	EnsureConnected(c.conn)
// 	ctx := ContextWithAPIKey(context.Background())

// 	_, err := c.client.UpdateAgentState(
// 		ctx,
// 		req,
// 	)
// 	return err
// }
