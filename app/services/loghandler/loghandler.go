package loghandler

import (
	"path/filepath"

	"github.com/SatisfactoryServerManager/SSMAgent/app/api"
	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	"github.com/hpcloud/tail"
)

var (
	_quit = make(chan int)
	tails []*tail.Tail
)

func watchFile(filePath string) (*tail.Tail, error) {
	t, err := tail.TailFile(filePath, tail.Config{
		Follow:    true,
		ReOpen:    true,
		MustExist: false,
		Poll:      true,
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
			// Send the log update to the API
			if err := api.SendFile("/api/v1/agent/upload/log", filePath); err != nil {
				utils.ErrorLogger.Printf("Error sending log file %s: %v\n", filePath, err)
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
		t.Stop()
		t.Cleanup()
	}
	tails = nil

	_quit <- 0

	utils.InfoLogger.Println("Shutdown Log Handler")
	return nil
}
