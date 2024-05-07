package task

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/SatisfactoryServerManager/SSMAgent/app/api"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/mod"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/savemanager"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/sf"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
)

var (
	_quit       = make(chan int)
	_agentTasks []TaskItem
)

type TaskItem struct {
	ID        string      `json:"_id"`
	Action    string      `json:"action"`
	Data      interface{} `json:"data"`
	Completed bool        `json:"completed"`
	Retries   int         `json:"retries"`
	Created   time.Time   `json:"created"`
}

type HttpRequestBody_MessageItem struct {
	Item TaskItem `json:"item"`
}

func InitMessageQueue() {
	utils.InfoLogger.Println("Initialising Message Queue...")

	GetAllTasks()

	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				GetAllTasks()
				ProcessAllMessageQueueItems()
			case <-_quit:
				ticker.Stop()
				return
			}
		}
	}()

	utils.InfoLogger.Println("Initialised Message Queue")
}

func ShutdownMessageQueue() error {
	utils.InfoLogger.Println("Shutting down Message Queue")

	_quit <- 0

	utils.InfoLogger.Println("Shutdown Message Queue")
	return nil
}

func GetAllTasks() {

	err := api.SendGetRequest("/api/v1/agent/tasks", &_agentTasks)
	if err != nil {
		utils.ErrorLogger.Println(err.Error())
	}
}

func ProcessAllMessageQueueItems() {

	if len(_agentTasks) == 0 {
		return
	}

	for idx := range _agentTasks {
		taskItem := &_agentTasks[idx]

		if taskItem.Completed {
			continue
		}

		err := ProcessMessageQueueItem(taskItem)

		if err != nil {
			taskItem.Retries += 1
			utils.ErrorLogger.Printf("Error processing task item %s with error: %s\r\n", taskItem.ID, err.Error())
			continue
		}

		taskItem.Completed = true
	}

	for idx := range _agentTasks {
		taskItem := &_agentTasks[idx]

		itemBody := HttpRequestBody_MessageItem{Item: *taskItem}

		var resData interface{}
		err := api.SendPutRequest("/api/v1/agent/tasks/"+taskItem.ID, itemBody, &resData)

		if err != nil {
			utils.ErrorLogger.Printf("Error sending task item update %s with error: %s\r\n", taskItem.ID, err.Error())
		}

	}
}

type UpdateModConfigData struct {
	ModReference string `json:"modReference"`
	ModConfig    string `json:"modConfig"`
}

func ProcessMessageQueueItem(taskItem *TaskItem) error {

	utils.DebugLogger.Printf("Processing Message action %s\r\n", taskItem.Action)

	switch taskItem.Action {
	case "startsfserver":
		return sf.StartSFServer()
	case "stopsfserver":
		return sf.ShutdownSFServer()
	case "killsfserver":
		return sf.KillSFServer()
	case "installsfserver":
		return sf.InstallSFServer(true)
	case "updatesfserver":
		return sf.UpdateSFServer()
	case "downloadSave":
		var objmap map[string]json.RawMessage
		b, _ := json.Marshal(taskItem.Data)
		json.Unmarshal(b, &objmap)
		return savemanager.DownloadSaveFile(string(objmap["saveFile"]))
	case "updateconfig":
		return nil
	case "updateModConfig":
		var objmap []map[string]string
		b, _ := json.Marshal(taskItem.Data)
		json.Unmarshal(b, &objmap)

		var configData UpdateModConfigData

		for _, d := range objmap {
			if string(d["Key"]) == "modReference" {
				configData.ModReference = string(d["Value"])
			}
			if string(d["Key"]) == "modConfig" {
				configData.ModConfig = string(d["Value"])
			}
		}

		return mod.UpdateModConfigFile(configData.ModReference, configData.ModConfig)
	default:
		return errors.New("unknown task action")
	}
}
