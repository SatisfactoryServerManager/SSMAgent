package api

import (
	"context"
	"time"

	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
)

type GRPCClient struct {
	conn   *grpc.ClientConn
	client pb.AgentServiceClient
	addr   string
}

var (
	AgentGRPCClient *GRPCClient
)

func NewGRPCClient(addr string) (*grpc.ClientConn, error) {

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

func ContextWithAPIKey(ctx context.Context) context.Context {
	apiKey := config.GetConfig().APIKey
	return metadata.AppendToOutgoingContext(ctx, "x-api-key", apiKey)
}

func NewAgentServiceClient(addr string) (*GRPCClient, error) {
	conn, err := NewGRPCClient(addr)
	if err != nil {
		return nil, err
	}

	return &GRPCClient{
		conn:   conn,
		client: pb.NewAgentServiceClient(conn),
		addr:   addr,
	}, nil
}

func InitGRPCClient() error {
	grpcAddr := config.GetConfig().GRPCAddress
	var err error
	AgentGRPCClient, err = NewAgentServiceClient(grpcAddr)
	return err
}

func ShutdownGRPCClient() error {
	if AgentGRPCClient == nil || AgentGRPCClient.conn == nil {
		return nil
	}
	return AgentGRPCClient.conn.Close()
}

func GetAgentServiceClient() *GRPCClient {
	return AgentGRPCClient
}

func (c *GRPCClient) GetClient() pb.AgentServiceClient {
	EnsureConnected(c.conn)
	return c.client
}

func (c *GRPCClient) GetConfig() (*pb.AgentConfigResponse, error) {

	EnsureConnected(c.conn)
	ctx := ContextWithAPIKey(context.Background())
	resp, err := c.client.GetAgentConfig(
		ctx,
		&pb.Empty{},
	)
	return resp, err
}

func (c *GRPCClient) UpdateConfigVersionIp() error {

	EnsureConnected(c.conn)
	ctx := ContextWithAPIKey(context.Background())

	ip, err := GetPublicIP()
	if err != nil {
		ip = ""
	}

	_, err = c.client.UpdateAgentConfigVersionIp(
		ctx,
		&pb.AgentConfigRequest{
			Version: config.GetConfig().Version,
			Ip:      ip,
		},
	)
	return err
}

func (c *GRPCClient) GetAgentTasks() (*pb.AgentTaskList, error) {
	EnsureConnected(c.conn)
	ctx := ContextWithAPIKey(context.Background())
	return c.client.GetAgentTasks(
		ctx,
		&pb.Empty{},
	)
}

func (c *GRPCClient) MarkAgentTaskCompleted(req *pb.AgentTaskCompletedRequest) error {
	EnsureConnected(c.conn)
	ctx := ContextWithAPIKey(context.Background())
	_, err := c.client.MarkAgentTaskCompleted(
		ctx,
		req,
	)
	return err
}

func (c *GRPCClient) MarkAgentTaskFailed(req *pb.AgentTaskFailedRequest) error {
	EnsureConnected(c.conn)
	ctx := ContextWithAPIKey(context.Background())
	_, err := c.client.MarkAgentTaskFailed(
		ctx,
		req,
	)
	return err
}

func (c *GRPCClient) UpdateAgentState(req *pb.AgentStateRequest) error {

	EnsureConnected(c.conn)
	ctx := ContextWithAPIKey(context.Background())

	_, err := c.client.UpdateAgentState(
		ctx,
		req,
	)
	return err
}
