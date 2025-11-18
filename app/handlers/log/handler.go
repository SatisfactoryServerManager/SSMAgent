package log

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	pb "github.com/SatisfactoryServerManager/ssmcloud-resources/proto"
	"github.com/hpcloud/tail"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type logLine struct {
	source string
	line   string
	inital bool
}

type Handler struct {
	conn   *grpc.ClientConn
	client pb.AgentLogServiceClient
	stream pb.AgentLogService_StreamLogClient

	ctx    context.Context
	cancel context.CancelFunc

	logChan chan logLine
	tails   []*tail.Tail
	done    chan struct{}
}

func NewHandler(conn *grpc.ClientConn) *Handler {
	return &Handler{
		conn:    conn,
		client:  pb.NewAgentLogServiceClient(conn),
		logChan: make(chan logLine, 2000),
		done:    make(chan struct{}),
	}
}

func (h *Handler) connectStream() error {
	ctx, cancel := context.WithCancel(context.Background())
	h.ctx = metadata.AppendToOutgoingContext(ctx, "x-api-key", config.GetConfig().APIKey)
	h.cancel = cancel

	stream, err := h.client.StreamLog(h.ctx)
	if err != nil {
		return err
	}

	h.stream = stream
	return nil
}

func (h *Handler) senderLoop() {
	for {
	RECONNECT:
		utils.DebugLogger.Println("Connecting log stream...")

		connectOK := false
		for !connectOK {
			select {
			case <-h.done:
				return
			default:
			}

			if err := h.connectStream(); err != nil {
				utils.ErrorLogger.Println("Log stream failed:", err)
				time.Sleep(time.Second)
				continue
			}
			connectOK = true
		}

		utils.DebugLogger.Println("Log stream connected")

		for {
			select {
			case <-h.done:
				return

			case entry, ok := <-h.logChan:
				if !ok {
					return // channel closed
				}

				req := &pb.AgentLogLineRequest{
					Line:   entry.line,
					Type:   entry.source,
					Inital: entry.inital,
				}

				if err := h.stream.Send(req); err != nil {
					utils.ErrorLogger.Println("Stream send failed, reconnecting:", err)
					time.Sleep(time.Second)
					goto RECONNECT
				}
			}
		}
	}
}

func getSourceFromPath(filePath string) string {
	if strings.Contains(filePath, "SSMAgent") {
		return "Agent"
	}
	return "FactoryGame"
}

func (h *Handler) sendInitialContent(filePath, source string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	idx := 0
	for _, line := range lines {
		isInital := false
		if idx == 0 {
			isInital = true
		}
		if strings.TrimSpace(line) != "" {
			h.logChan <- logLine{source: source, line: line, inital: isInital}
			idx++
		}
	}
	return nil
}

func (h *Handler) watchFile(filePath string) (*tail.Tail, error) {
	source := getSourceFromPath(filePath)

	h.sendInitialContent(filePath, source)

	t, err := tail.TailFile(filePath, tail.Config{
		Follow:    true,
		ReOpen:    true,
		MustExist: false,
		Poll:      true,
		Location:  &tail.SeekInfo{Offset: 0, Whence: 2},
	})
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			select {
			case <-h.done:
				return
			case line, ok := <-t.Lines:
				if !ok {
					return
				}
				if line.Err == nil {
					h.logChan <- logLine{source: source, line: line.Text}
				}
			}
		}
	}()

	return t, nil
}

func (h *Handler) Run() {
	utils.InfoLogger.Println("Initialising Log Handler...")

	go h.senderLoop()

	agentLog := filepath.Join(config.GetConfig().LogDir, "SSMAgent-combined.log")
	if utils.CheckFileExists(agentLog) {
		if t, err := h.watchFile(agentLog); err == nil {
			h.tails = append(h.tails, t)
		}
	}

	gameLogDir := filepath.Join(config.GetConfig().SFDir, "FactoryGame", "Saved", "Logs")
	utils.CreateFolder(gameLogDir)
	gameLog := filepath.Join(gameLogDir, "FactoryGame.log")

	if t, err := h.watchFile(gameLog); err == nil {
		h.tails = append(h.tails, t)
	}

	utils.InfoLogger.Println("Initialised Log Handler")
}

func (h *Handler) Stop() error {
	close(h.done)    // stops senderLoop and tails
	close(h.logChan) // stops senderLoop read
	if h.cancel != nil {
		h.cancel()
	}

	for _, t := range h.tails {
		t.Kill(nil)
		t.Cleanup()
	}

	return nil
}
