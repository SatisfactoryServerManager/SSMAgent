package task

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/SatisfactoryServerManager/SSMAgent/app/services/lock"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto/generated"
)

type running struct {
	taskID     string
	leaseToken string
	cancel     context.CancelFunc
	done       chan struct{}
}

// Executor drains assignments one at a time. Serialization is the point: the
// backend's unique running-index guarantees it never sends a second task, and
// this goroutine guarantees we never start one.
type Executor struct {
	client pb.AgentTaskServiceClient
	ctx    context.Context

	queue chan *pb.TaskAssignment

	mu       sync.Mutex
	current  *running
	draining bool

	quit     chan struct{}
	quitOnce sync.Once
}

func NewExecutor(client pb.AgentTaskServiceClient, ctx context.Context) *Executor {
	return &Executor{
		client: client,
		ctx:    ctx,
		queue:  make(chan *pb.TaskAssignment, 1),
		quit:   make(chan struct{}),
	}
}

func (e *Executor) isDraining() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.draining
}

func (e *Executor) Submit(a *pb.TaskAssignment) {
	select {
	case e.queue <- a:
	default:
		utils.WarnLogger.Printf("executor busy, dropping assignment %s; backend will requeue on lease expiry", a.TaskId)
	}
}

// RunningTask lets the stream client re-announce in-flight work on reconnect.
func (e *Executor) RunningTask() (string, string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.current == nil {
		return "", ""
	}
	return e.current.taskID, e.current.leaseToken
}

func (e *Executor) Start() {
	go func() {
		for {
			select {
			case a := <-e.queue:
				e.run(a)
			case <-e.quit:
				return
			}
		}
	}()
}

func (e *Executor) run(a *pb.TaskAssignment) {
	// If a drain began after this assignment was queued but before we picked it
	// up, hand it straight back instead of starting a task the process is about
	// to abandon. Only one of run() or DrainAndRelease's drain reads a given
	// assignment off the channel, so this cannot double-report.
	if e.isDraining() {
		utils.InfoLogger.Printf("draining, releasing queued task %s", a.TaskId)
		e.releaseAssignment(a)
		return
	}

	handler, err := lookup(a.Action)
	if err != nil {
		e.report(a, pb.TaskStatus_FAILED, err.Error(), 0, "")
		return
	}

	leaseSec := a.LeaseSeconds
	if leaseSec <= 0 {
		leaseSec = 60
	}

	taskCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	e.mu.Lock()
	e.current = &running{taskID: a.TaskId, leaseToken: a.LeaseToken, cancel: cancel, done: done}
	e.mu.Unlock()

	defer func() {
		cancel()
		close(done)

		e.mu.Lock()
		e.current = nil
		e.mu.Unlock()
	}()

	// The lease renewer starts before runHandler blocks on lock.Server, so a task
	// stuck waiting for a long backup or auto-restart keeps its lease alive rather
	// than silently expiring it.
	go e.renewLease(taskCtx, cancel, a, time.Duration(leaseSec)*time.Second/3)

	e.report(a, pb.TaskStatus_RUNNING, "", 0, "started")

	progress := func(pct int32, msg string) {
		e.report(a, pb.TaskStatus_RUNNING, "", pct, msg)
	}

	err = e.runHandler(handler, taskCtx, a, progress)

	// A cancelled context during shutdown means DrainAndRelease already reported
	// RELEASED. Do not also report FAILED.
	if taskCtx.Err() != nil && e.isDraining() {
		return
	}

	if err != nil {
		utils.ErrorLogger.Printf("task %s (%s) failed: %s", a.TaskId, a.Action, err.Error())
		e.report(a, pb.TaskStatus_FAILED, err.Error(), 0, "")
		return
	}

	utils.InfoLogger.Printf("task %s (%s) completed", a.TaskId, a.Action)
	e.report(a, pb.TaskStatus_COMPLETED, "", 100, "")
}

// runHandler holds the server lock for the whole handler. The unlock is deferred
// so a panicking handler cannot strand the lock and wedge every later task and
// every background ticker.
func (e *Executor) runHandler(h Handler, ctx context.Context, a *pb.TaskAssignment, progress func(int32, string)) error {
	lock.Server.Lock()
	defer lock.Server.Unlock()

	return h(ctx, json.RawMessage(a.Data), progress)
}

// renewLease keeps the lease alive and carries cancellation back. It is a unary
// call, so a dropped task stream does not kill a running install.
func (e *Executor) renewLease(ctx context.Context, cancel context.CancelFunc, a *pb.TaskAssignment, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			resp, err := e.client.RenewTaskLease(e.ctx, &pb.TaskLeaseRequest{
				TaskId:     a.TaskId,
				LeaseToken: a.LeaseToken,
			})
			if err != nil {
				utils.ErrorLogger.Printf("lease renewal failed for task %s: %s", a.TaskId, err.Error())
				continue // transient; the lease has slack for a few misses
			}

			if !resp.Ok {
				utils.WarnLogger.Printf("lost lease on task %s, abandoning", a.TaskId)
				cancel()
				return
			}

			if resp.CancelRequested {
				utils.InfoLogger.Printf("cancellation requested for task %s", a.TaskId)
				cancel()
				return
			}
		}
	}
}

func (e *Executor) report(a *pb.TaskAssignment, status pb.TaskStatus, errMsg string, pct int32, msg string) {
	_, err := e.client.ReportTaskStatus(e.ctx, &pb.TaskStatusReport{
		TaskId:          a.TaskId,
		LeaseToken:      a.LeaseToken,
		Status:          status,
		Error:           errMsg,
		ProgressPercent: pct,
		Message:         msg,
	})
	if err != nil {
		utils.ErrorLogger.Printf("error reporting task %s status: %s", a.TaskId, err.Error())
	}
}

func (e *Executor) releaseAssignment(a *pb.TaskAssignment) {
	_, err := e.client.ReportTaskStatus(e.ctx, &pb.TaskStatusReport{
		TaskId:     a.TaskId,
		LeaseToken: a.LeaseToken,
		Status:     pb.TaskStatus_RELEASED,
	})
	if err != nil {
		utils.ErrorLogger.Printf("error releasing task %s: %s", a.TaskId, err.Error())
	}
}

// DrainAndRelease cancels the running task and hands it back to the queue without
// spending an attempt. A rolling restart therefore neither burns retry budget nor
// waits out the lease: the install resumes seconds later in the new process.
func (e *Executor) DrainAndRelease(ctx context.Context) error {
	e.mu.Lock()
	e.draining = true
	cur := e.current
	e.mu.Unlock()

	var relErr error
	if cur != nil {
		utils.InfoLogger.Printf("releasing in-flight task %s", cur.taskID)

		cur.cancel()

		select {
		case <-cur.done:
		case <-ctx.Done():
			utils.WarnLogger.Printf("task %s did not unwind before the deadline", cur.taskID)
		}

		_, relErr = e.client.ReportTaskStatus(e.ctx, &pb.TaskStatusReport{
			TaskId:     cur.taskID,
			LeaseToken: cur.leaseToken,
			Status:     pb.TaskStatus_RELEASED,
		})
	}

	e.stop()

	// Release any assignment that was accepted but never started. Reads compete
	// with run() for the same channel, so at most one side sees each assignment.
	e.drainQueue()

	return relErr
}

func (e *Executor) drainQueue() {
	for {
		select {
		case a := <-e.queue:
			utils.InfoLogger.Printf("releasing queued task %s", a.TaskId)
			e.releaseAssignment(a)
		default:
			return
		}
	}
}

func (e *Executor) stop() {
	e.quitOnce.Do(func() { close(e.quit) })
}
