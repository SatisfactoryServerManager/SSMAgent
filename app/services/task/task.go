package task

import (
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated"

	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
)

// TemporarySink logs assignments and does nothing else. Replaced by the executor
// in Task 12. It exists so this commit compiles without silently dropping work.
type TemporarySink struct{}

func (TemporarySink) Submit(a *pb.TaskAssignment) {
	utils.WarnLogger.Printf("task %s (%s) received but executor is not implemented yet", a.TaskId, a.Action)
}

func (TemporarySink) RunningTask() (string, string) { return "", "" }
