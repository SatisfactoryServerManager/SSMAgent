package loghandler

import (
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/SatisfactoryServerManager/SSMAgent/app/api"
	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
)

var (
	_quit = make(chan int)
)

func InitLogHandler() {
	log.Println("Initialising Log Handler...")

	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				SendLogFiles()
			case <-_quit:
				ticker.Stop()
				return
			}
		}
	}()

	log.Println("Initialised Log Handler")
}

func ShutdownLogHandler() error {
	log.Println("Shutting down Log Handler")

	_quit <- 0

	log.Println("Shutdown Log Handler")
	return nil
}

func SendLogFiles() {
	fmt.Println("Sending Log Files")

	ssmlogfile := filepath.Join(config.GetConfig().LogDir, "SSMAgent-combined.log")

	api.SendFile("/api/agent/uploadlog", ssmlogfile)

	gamelogdir := filepath.Join(
		config.GetConfig().SFDir,
		"FactoryGame",
		"Saved",
		"Logs",
	)

	utils.CreateFolder(gamelogdir)

	gamelogfile := filepath.Join(
		gamelogdir,
		"FactoryGame.log",
	)

	api.SendFile("/api/agent/uploadlog", gamelogfile)
}
