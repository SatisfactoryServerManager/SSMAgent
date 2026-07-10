package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
)

// operation is a clean up function on shutting down
type operation func(ctx context.Context) error

// NamedOperation is one shutdown step. Order matters: the task executor must
// drain before the SF server stops, and the gRPC stream must close last. A map
// could not express that.
type NamedOperation struct {
	Name string
	Op   operation
}

// gracefulShutdown waits for a termination syscall, then runs ops in the order given.
func gracefulShutdown(ctx context.Context, timeout time.Duration, ops []NamedOperation) <-chan struct{} {
	wait := make(chan struct{})
	go func() {
		s := make(chan os.Signal, 1)

		// add any other syscalls that you want to be notified with
		signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
		<-s

		utils.InfoLogger.Println("shutting down")

		// set timeout for the ops to be done to prevent system hang
		timeoutFunc := time.AfterFunc(timeout, func() {
			utils.WarnLogger.Printf("timeout %d ms has been elapsed, force exit", timeout.Milliseconds())
			os.Exit(0)
		})

		defer timeoutFunc.Stop()

		for _, entry := range ops {
			utils.InfoLogger.Printf("cleaning up: %s", entry.Name)

			// A failed step must not abort the rest: the later steps are the ones
			// that release the in-flight task and mark the agent offline.
			if err := entry.Op(ctx); err != nil {
				utils.InfoLogger.Printf("%s: clean up failed: %s", entry.Name, err.Error())
				continue
			}

			utils.InfoLogger.Printf("%s was shutdown gracefully", entry.Name)
		}

		close(wait)
	}()

	return wait
}
