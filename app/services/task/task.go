package task

import (
	"encoding/json"
	"errors"

	"github.com/SatisfactoryServerManager/SSMAgent/app/services/mod"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/sf"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
)

var (
	_completedTaskIds = make([]string, 0)
)

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
