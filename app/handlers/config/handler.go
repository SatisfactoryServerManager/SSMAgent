package config

import (
	"context"

	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
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
