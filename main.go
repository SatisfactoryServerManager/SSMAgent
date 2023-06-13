package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/SatisfactoryServerManager/SSMAgent/app/api"
	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/backup"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/loghandler"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/messagequeue"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/mod"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/savemanager"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/sf"
	"github.com/SatisfactoryServerManager/SSMAgent/app/steamcmd"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
)

var _quit = make(chan int)

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
	flag.String("url", "https://ssmcloud.hostxtra.co.uk", "The url for SSM Cloud defaults to https://ssmcloud.hostxtra.co.uk")
	flag.String("apikey", "", "The agents api key used to connect to SSM Cloud")
	flag.String("datadir", "/SSM/data", "The directory where SF and Steam will be stored")
	flag.Int("p", 0, "The port offset from 15777 defaults to 0")

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
		"mq": func(ctx context.Context) error {
			return messagequeue.ShutdownMessageQueue()
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
		"main": func(ctx context.Context) error {
			_quit <- 0
			MarkAgentOffline()
			return nil
		},
	})

	config.GetConfig()

	err := TestSSMCloudAPI()
	utils.CheckError(err)

	MarkAgentOnline()

	SendConfig()
	GetConfigFromAPI()

	ticker := time.NewTicker(20 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				GetConfigFromAPI()
			case <-_quit:
				ticker.Stop()
				return
			}
		}
	}()

	steamcmd.InitSteamCMD()
	sf.InitSFHandler()

	go messagequeue.InitMessageQueue()
	go loghandler.InitLogHandler()
	go savemanager.InitSaveManager()
	go backup.InitBackupManager()
	go mod.InitModManager()

	<-wait

}

func TestSSMCloudAPI() error {

	log.Printf("Testing connection to: %s\r\n", config.GetConfig().URL)
	var test interface{}
	err := api.SendGetRequest("/api/ping", &test)

	if err != nil {
		return err
	}

	log.Println("Connection test succeeded!")

	return nil

}

func MarkAgentOnline() {
	bodyData := api.HttpRequestBody_ActiveState{}
	bodyData.Active = true

	var resData interface{}

	err := api.SendPostRequest("/api/agent/activestate", &bodyData, &resData)
	utils.CheckError(err)
}

func MarkAgentOffline() {
	var body api.HttpRequestBody_ActiveState
	body.Active = false

	var resData interface{}

	err := api.SendPostRequest("/api/agent/activestate", body, &resData)
	utils.CheckError(err)
}

func SendConfig() {

	ip := api.GetPublicIP()

	var req = api.HTTPRequestBody_Config{
		Version:     config.GetConfig().Version,
		SFInstalled: config.GetConfig().SF.InstalledVer,
		SFAvailable: config.GetConfig().SF.AvilableVer,
		IP:          ip,
	}

	var resData interface{}
	err := api.SendPostRequest("/api/agent/config", req, &resData)

	utils.CheckError(err)
}

func GetConfigFromAPI() {
	var resData = api.HttpResponseBody_Config{}
	err := api.SendGetRequest("/api/agent/config", &resData)

	if err != nil {
		return
	}

	config.GetConfig().SF.MaxPlayers = resData.MaxPlayers
	config.GetConfig().SF.WorkerThreads = resData.WorkerThreads
	config.GetConfig().SF.SFBranch = resData.SFBranch
	config.GetConfig().Backup.Interval = resData.Backup.Interval
	config.GetConfig().Backup.Keep = resData.Backup.Keep
	config.GetConfig().SF.UpdateSFOnStart = resData.UpdateOnStart

	config.SaveConfig()

	config.UpdateIniFiles()

}
