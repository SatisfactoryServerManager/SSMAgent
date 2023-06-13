package mod

import (
	"fmt"
	"log"

	"github.com/SatisfactoryServerManager/SSMAgent/app/api"
)

type ModState struct {
	ID                  string        `json:"_id"`
	InstalledSMLVersion string        `json:"installedSMLVersion"`
	SelectedMods        []SelectedMod `json:"selectedMods"`
}

type SelectedMod struct {
	Mod              Mod    `json:"mod"`
	DesiredVersion   string `json:"desiredVersion"`
	InstalledVersion string `json:"installedVersion"`
	Installed        bool   `json:"installed"`
	NeedsUpdate      bool   `json:"needsUpdate"`
}

type Mod struct {
	ID           string       `json:"_id"`
	ModID        string       `json:"modId"`
	ModName      string       `json:"modName"`
	ModReference string       `json:"modReference"`
	Hidden       bool         `json:"hidden"`
	Versions     []ModVersion `json:"versions"`
}

type ModVersion struct {
	Version    string             `json:"version"`
	Link       string             `json:"link"`
	SMLVersion string             `json:"sml_version"`
	Targets    []ModVersionTarget `json:"targets"`
}

type ModVersionTarget struct {
	TargetName string `json:"targetName"`
	Link       string `json:"link"`
}

var _ModState ModState

func InitModManager() {

	log.Println("Initialising Mod Manager...")
	FindInstalledMods()
	GetModState()
	log.Println("Initialised Mod Manager")
}

func FindInstalledMods() {

}

func GetModState() {

	err := api.SendGetRequest("/api/agent/modstate", &_ModState)
	if err != nil {
		log.Printf("Failed to get Mod State with error: %s\r\n", err.Error())
		return
	}

	fmt.Println(_ModState)

	ProcessModState()
}

func ProcessModState() {

}
