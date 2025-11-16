package task

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/SatisfactoryServerManager/SSMAgent/app/api"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/mod"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/sf"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"github.com/SatisfactoryServerManager/ssmcloud-resources/proto"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	_quit             = make(chan int)
	_agentTasks       []v2.AgentTask
	_completedTaskIds = make([]string, 0)
)

type TaskItem struct {
	ID        string      `json:"_id"`
	Action    string      `json:"action"`
	Data      interface{} `json:"data"`
	Completed bool        `json:"completed"`
	Retries   int         `json:"retries"`
	Created   time.Time   `json:"created"`
}

func InitMessageQueue() {
	utils.InfoLogger.Println("Initialising Message Queue...")
	go PollTasks()

	utils.InfoLogger.Println("Initialised Message Queue")
}

func PollTasks() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			resp, err := api.GetAgentServiceClient().GetAgentTasks()
			if err != nil {
				fmt.Println("Failed to fetch tasks:", err)
				continue
			}

			_agentTasks = make([]v2.AgentTask, 0)
			for _, t := range resp.Tasks {
				// parse ObjectID
				objID, err := primitive.ObjectIDFromHex(t.Id)
				if err != nil {
					fmt.Println("Invalid task ID:", t.Id)
					continue
				}

				// parse Data JSON into a map or concrete struct
				var data interface{}
				if t.Data != "" {
					err := json.Unmarshal([]byte(t.Data), &data)
					if err != nil {
						fmt.Println("Failed to unmarshal data:", err)
					}
				}

				task := v2.AgentTask{
					ID:        objID,
					Action:    t.Action,
					Data:      data,
					Completed: t.Completed,
					Retries:   int(t.Retries),
				}
				_agentTasks = append(_agentTasks, task)

				fmt.Printf("Task received: %+v\n", task)
			}

			ProcessAllMessageQueueItems()
		case <-_quit:
			ticker.Stop()
			return
		}
	}
}

func ShutdownMessageQueue() error {
	utils.InfoLogger.Println("Shutting down Message Queue")

	_quit <- 0

	utils.InfoLogger.Println("Shutdown Message Queue")
	return nil
}

func ProcessAllMessageQueueItems() {

	if len(_agentTasks) == 0 {
		return
	}

	for idx := range _agentTasks {
		taskItem := &_agentTasks[idx]

		err := ProcessMessageQueueItem(taskItem)

		if err != nil {
			api.GetAgentServiceClient().MarkAgentTaskFailed(&proto.AgentTaskFailedRequest{Id: taskItem.ID.Hex()})

			utils.ErrorLogger.Printf("Error processing task item %s (%s) with error: %s\r\n", taskItem.Action, taskItem.ID, err.Error())
			continue
		}
		api.GetAgentServiceClient().MarkAgentTaskCompleted(&proto.AgentTaskCompletedRequest{Id: taskItem.ID.Hex()})

		_completedTaskIds = append(_completedTaskIds, taskItem.ID.Hex())
	}

	//TODO: Send updates back to API

	// for idx := range _agentTasks {
	// 	taskItem := &_agentTasks[idx]

	// 	itemBody := HttpRequestBody_MessageItem{Item: *taskItem}

	// 	var resData interface{}
	// 	err := api.SendPutRequest("/api/v1/agent/tasks/"+taskItem.ID, itemBody, &resData)

	// 	if err != nil {
	// 		utils.ErrorLogger.Printf("Error sending task item update %s with error: %s\r\n", taskItem.ID, err.Error())
	// 	}

	// }
}

type UpdateModConfigData struct {
	ModReference string `json:"modReference"`
	ModConfig    string `json:"modConfig"`
}

func ProcessMessageQueueItem(taskItem *v2.AgentTask) error {

	AlreadyCompleted := false
	for _, completedTaskId := range _completedTaskIds {
		if taskItem.ID.Hex() == completedTaskId {
			AlreadyCompleted = true
			break
		}
	}

	if AlreadyCompleted {
		return nil
	}

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
	case "claimserver":

		type ClaimData struct {
			AdminPassword  string `json:"adminpass"`
			ClientPassword string `json:"clientpass"`
		}

		var objData ClaimData

		b, _ := json.Marshal(taskItem.Data)
		json.Unmarshal(b, &objData)

		return sf.ClaimServer(objData.AdminPassword, objData.ClientPassword)
	default:
		return errors.New("unknown task action")
	}
}
