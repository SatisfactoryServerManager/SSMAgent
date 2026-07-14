package task

import (
	"context"

	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated"
	"google.golang.org/grpc/metadata"
)

// reporter is the agent's task client, used for the unary calls a handler makes
// on its own behalf (ReportInstalledMods) rather than the ones the executor makes
// about the task (ReportTaskStatus, RenewTaskLease).
var reporter pb.AgentTaskServiceClient

// SetClient wires the authenticated task client. It must be called before the
// executor starts, so a handler can never find a nil client.
func SetClient(c pb.AgentTaskServiceClient) { reporter = c }

// ReportInstalledMods tells the backend what is actually on the agent's disk.
func ReportInstalledMods(ctx context.Context, mods []v2.InstalledMod) error {
	if reporter == nil {
		return nil
	}

	out := make([]*pb.InstalledMod, 0, len(mods))
	for _, m := range mods {
		out = append(out, &pb.InstalledMod{
			ModReference:     m.ModReference,
			InstalledVersion: m.InstalledVersion,
			Installed:        m.Installed,
		})
	}

	// The task context carries cancellation, not credentials: the executor derives
	// it from context.Background(). Attach the API key here or the call is refused.
	ctx = metadata.AppendToOutgoingContext(ctx, "x-api-key", config.GetConfig().APIKey)

	_, err := reporter.ReportInstalledMods(ctx, &pb.InstalledModsReport{Mods: out})
	return err
}
