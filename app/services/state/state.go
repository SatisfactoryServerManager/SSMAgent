package state

import (
	"context"
	"fmt"
	"time"

	"github.com/SatisfactoryServerManager/SSMAgent/app/api"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto"
)

var (
	Online             bool
	Installed          bool
	Running            bool
	CPU                float32
	MEM                float32
	InstalledSFVersion int64
	LatestSFVersion    int64
)

type StateUpdater struct {
	done       chan struct{}
	lastStream pb.AgentService_UpdateAgentStateStreamClient
	lastCancel context.CancelFunc
}

var updater *StateUpdater

func InitStateStream() error {
	updater = &StateUpdater{
		done: make(chan struct{}),
	}

	go updater.reconnectLoop()
	return nil
}

func ShutdownStateStream() error {
	utils.DebugLogger.Println("Shutting down state update stream")
	if err := updater.Stop(); err != nil {
		return err
	}
	payload := &pb.AgentStateRequest{
		Online:             Online,
		Installed:          Installed,
		Running:            Running,
		Cpu:                CPU,
		Ram:                MEM,
		InstalledSFVersion: InstalledSFVersion,
		LatestSFVersion:    LatestSFVersion,
	}

	if err := api.AgentGRPCClient.UpdateAgentState(payload); err != nil {
		return fmt.Errorf("error sending final state: %s", err.Error())
	}

	return nil
}

func (u *StateUpdater) reconnectLoop() {
	backoff := time.Second

	for {
		select {
		case <-u.done:
			return
		default:
		}

		utils.DebugLogger.Println("Opening state stream...")

		// Make cancellable context for the stream
		ctx, cancel := context.WithCancel(context.Background())
		u.lastCancel = cancel

		// attach API key metadata
		ctx = api.ContextWithAPIKey(ctx)

		// attempt stream creation
		var err error
		u.lastStream, err = api.GetAgentServiceClient().GetClient().UpdateAgentStateStream(ctx)
		if err != nil {
			utils.DebugLogger.Println("Failed to open stream:", err)
			cancel()
			time.Sleep(backoff)
			continue
		}

		// send initial state
		if err := u.sendState(); err != nil {
			utils.DebugLogger.Println("Failed initial send:", err)
			cancel()
			time.Sleep(backoff)
			continue
		}

		// send loop
		sendErr := u.sendLoop()

		utils.DebugLogger.Println("Stream closed:", sendErr)

		// cleanup
		cancel()
		time.Sleep(backoff)
	}
}

func (u *StateUpdater) sendLoop() error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-u.done:
			// tell server we're closing the stream
			_, _ = u.lastStream.CloseAndRecv()
			return nil

		case <-ticker.C:
			if err := u.sendState(); err != nil {
				return err
			}
		}
	}
}

func (u *StateUpdater) sendState() error {
	payload := &pb.AgentStateRequest{
		Online:             Online,
		Installed:          Installed,
		Running:            Running,
		Cpu:                CPU,
		Ram:                MEM,
		InstalledSFVersion: InstalledSFVersion,
		LatestSFVersion:    LatestSFVersion,
	}
	fmt.Printf("sending state: %v\n", payload)
	return u.lastStream.Send(payload)
}

func (u *StateUpdater) Stop() error {

	// stop the loop
	close(u.done)

	// cancel the active context to break Recv/Send
	if u.lastCancel != nil {
		u.lastCancel()
	}

	return nil
}
