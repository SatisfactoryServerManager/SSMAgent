package loghandler

import (
	"path/filepath"
	"strings"

	"github.com/SatisfactoryServerManager/SSMAgent/app/api"
	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	"github.com/hpcloud/tail"
)

var (
	tails []*tail.Tail
)

func getSourceFromPath(filePath string) string {
	if strings.Contains(filePath, "SSMAgent") {
		return "Agent"
	}
	return "FactoryGame"
}

// sendInitialContent sends the entire file content on startup
func sendInitialContent(filePath string, source string) error {
	// First send the full file content
	if err := api.SendFile("/api/v1/agent/upload/log", filePath); err != nil {
		return err
	}
	return nil
}

func watchFile(filePath string) (*tail.Tail, error) {
	source := getSourceFromPath(filePath)

	// Send initial content
	if err := sendInitialContent(filePath, source); err != nil {
		utils.ErrorLogger.Printf("Error sending initial content for %s: %v\n", filePath, err)
	}

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
		for line := range t.Lines {
			if line.Err != nil {
				utils.ErrorLogger.Printf("Error reading line from %s: %v\n", filePath, line.Err)
				continue
			}
			// Send just the new line to the API
			if err := api.SendLogLine(source, line.Text); err != nil {
				utils.ErrorLogger.Printf("Error sending log line from %s: %v\n", filePath, err)
			}
		}
	}()

	return t, nil
}

func InitLogHandler() {
	utils.InfoLogger.Println("Initialising Log Handler...")

	// Watch SSM Agent log
	ssmlogfile := filepath.Join(config.GetConfig().LogDir, "SSMAgent-combined.log")
	if utils.CheckFileExists(ssmlogfile) {
		t, err := watchFile(ssmlogfile)
		if err != nil {
			utils.ErrorLogger.Printf("Error setting up tail for SSM log: %v\n", err)
		} else {
			tails = append(tails, t)
		}
	}

	// Watch FactoryGame log
	gamelogdir := filepath.Join(config.GetConfig().SFDir, "FactoryGame", "Saved", "Logs")
	utils.CreateFolder(gamelogdir)
	gamelogfile := filepath.Join(gamelogdir, "FactoryGame.log")

	t, err := watchFile(gamelogfile)
	if err != nil {
		utils.ErrorLogger.Printf("Error setting up tail for game log: %v\n", err)
	} else {
		tails = append(tails, t)
	}

	utils.InfoLogger.Println("Initialised Log Handler")
}

func ShutdownLogHandler() error {
	utils.InfoLogger.Println("Shutting down Log Handler")

	// Stop all tails
	for _, t := range tails {
		utils.DebugLogger.Printf("Stopping tail for %s\n", t.Filename)
		t.Kill(nil)
		t.Cleanup()

		utils.DebugLogger.Printf("Stopped tail for %s\n", t.Filename)

	}
	tails = nil

	utils.InfoLogger.Println("Shutdown Log Handler")
	return nil
}
