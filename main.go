package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/SatisfactoryServerManager/SSMAgent/app/api"
	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/backup"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/loghandler"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/mod"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/savemanager"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/sf"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/state"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/task"
	"github.com/SatisfactoryServerManager/SSMAgent/app/steamcmd"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
)

var _quit = make(chan int)

var connectionTestRetryCount = 0

func isFlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func main() {
	flag.String("name", "", "The name of the ssm agent")
	flag.String("url", "https://api-ssmcloud.hostxtra.co.uk", "The url for SSM Cloud")
	flag.String("grpcaddr", "grpc://api-ssmcloud.hostxtra.co.uk:50051", "The grpc address for SSM Cloud")
	flag.String("apikey", "", "The agents api key used to connect to SSM Cloud")
	flag.String("datadir", "/SSM/data", "The directory where SF and Steam will be stored")
	flag.Int("p", 0, "The port offset from 7777 defaults to 0")

	flag.Parse()

	if !isFlagPassed("name") {
		log.Fatal("Agent name flag was not passed!")
	}

	if !isFlagPassed("apikey") {
		log.Fatal("Agent apikey flag was not passed!")
	}

	wait := gracefulShutdown(context.Background(), 30*time.Second, map[string]operation{

		"sf": func(ctx context.Context) error {
			return sf.ShutdownSFHandler()
		},
		"task": func(ctx context.Context) error {
			return task.ShutdownMessageQueue()
		},
		"loghandler": func(ctx context.Context) error {
			return loghandler.ShutdownLogHandler()
		},
		"savemanager": func(ctx context.Context) error {
			return savemanager.ShutdownSaveManager()
		},
		"backupmanager": func(ctx context.Context) error {
			return backup.ShutdownBackupManager()
		},
		"modmanager": func(ctx context.Context) error {
			return mod.ShutdownModManager()
		},
		"state": func(ctx context.Context) error {
			MarkAgentOffline()
			return state.ShutdownStateStream()
		},
		"grpc": func(ctx context.Context) error {
			return api.ShutdownGRPCClient()
		},
		"main": func(ctx context.Context) error {
			_quit <- 0
			return nil
		},
	})

	config.GetConfig()

	err := api.InitGRPCClient()
	if err != nil {
		panic(err)
	}

	utils.CheckError(TestSSMCloudAPI())
	utils.CheckError(state.InitStateStream())

	MarkAgentOnline()

	SendConfig()
	GetConfigFromAPI()

	ticker := time.NewTicker(20 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				GetConfigFromAPI()
				SendConfig()
			case <-_quit:
				ticker.Stop()
				return
			}
		}
	}()

	steamcmd.InitSteamCMD()
	sf.InitSFHandler()

	go task.InitMessageQueue()
	go loghandler.InitLogHandler()
	go savemanager.InitSaveManager()
	go backup.InitBackupManager()
	go mod.InitModManager()

	<-wait

}

func TestSSMCloudAPI() error {

	utils.InfoLogger.Printf("Testing connection to: %s\r\n", config.GetConfig().URL)
	var test interface{}
	err := api.SendGetRequest("/api/v1/ping", &test)

	if err != nil {
		utils.ErrorLogger.Printf("Retrying connection test due to failed to send agent status with error: %s\n", err.Error())
		time.Sleep(time.Second)
		connectionTestRetryCount++

		if connectionTestRetryCount >= 300 {
			return fmt.Errorf("ssm agent connection test timed out")
		}

		TestSSMCloudAPI()
	}

	utils.InfoLogger.Println("Connection test succeeded!")

	return nil
}

func MarkAgentOnline() {
	state.Online = true
}

func MarkAgentOffline() {
	state.Online = false
}

func SendConfig() {

	state.InstalledSFVersion = config.GetConfig().SF.InstalledVer
	state.LatestSFVersion = config.GetConfig().SF.AvilableVer

	if err := api.GetAgentServiceClient().UpdateConfigVersionIp(); err != nil {
		utils.ErrorLogger.Printf("Error sending config data to API with error: %s\r\n", err.Error())
	}
}

func GetConfigFromAPI() error {

	resConfig, err := api.GetAgentServiceClient().GetConfig()
	if err != nil {
		return err
	}

	oldBranch := config.GetConfig().SF.SFBranch

	config.GetConfig().Backup.Interval = int(resConfig.Config.BackupInterval)
	config.GetConfig().Backup.Keep = int(resConfig.Config.BackupKeepAmount)

	config.GetConfig().SF.MaxPlayers = int(resConfig.ServerConfig.MaxPlayers)
	config.GetConfig().SF.WorkerThreads = int(resConfig.ServerConfig.WorkerThreads)
	config.GetConfig().SF.SFBranch = resConfig.ServerConfig.Branch

	config.GetConfig().SF.UpdateSFOnStart = resConfig.ServerConfig.UpdateSFOnStart
	config.GetConfig().SF.AutoRestart = resConfig.ServerConfig.AutoRestart
	config.GetConfig().SF.AutoPause = resConfig.ServerConfig.AutoPause
	config.GetConfig().SF.AutoSaveOnDisconnect = resConfig.ServerConfig.AutoSaveOnDisconnect
	config.GetConfig().SF.AutoSaveInterval = float32(resConfig.ServerConfig.AutoSaveInterval)
	config.GetConfig().SF.DisableSeasonalEvents = resConfig.ServerConfig.DisableSeasonalEvents

	config.SaveConfig()

	if oldBranch != config.GetConfig().SF.SFBranch {
		sf.GetLatestedVersion()
		SendConfig()
	}

	if !sf.IsInstalled() {
		return nil
	}

	if sf.IsRunning() {
		return nil
	}

	if err := config.UpdateIniFiles(); err != nil {
		utils.ErrorLogger.Printf("error updating game ini files with error: %s\n", err.Error())
	}

	return nil
}
