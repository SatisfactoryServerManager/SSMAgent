package mod

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/SatisfactoryServerManager/SSMAgent/app/api"
	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	"github.com/SatisfactoryServerManager/SSMAgent/app/vars"
	"golang.org/x/mod/semver"
)

type ModState struct {
	SelectedMods []SelectedMod `json:"selectedMods"`
}

type SelectedMod struct {
	Mod              Mod    `json:"mod"`
	DesiredVersion   string `json:"desiredVersion"`
	InstalledVersion string `json:"installedVersion"`
	Installed        bool   `json:"installed"`
	NeedsUpdate      bool   `json:"needsUpdate"`
	Config           string `json:"config"`
}

type Mod struct {
	ID           string       `json:"_id"`
	ModID        string       `json:"id"`
	ModName      string       `json:"name"`
	ModReference string       `json:"mod_reference"`
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

type InstalledMod struct {
	ModReference    string
	ModPath         string
	ModDisplayName  string `json:"FriendlyName"`
	ModUPluginPath  string
	ModVersion      string `json:"SemVersion"`
	ShouldUninstall bool
}

type UPluginFile struct {
	SemVersion string `json:"SemVersion"`
}

type BasicMod struct {
	ModReference string `json:"mod_reference"`
}
type BasicSelectedMod struct {
	Mod              BasicMod `json:"mod"`
	InstalledVersion string   `json:"installedVersion"`
	Installed        bool     `json:"installed"`
	Config           string   `json:"config"`
}
type BasicModState struct {
	SelectedMods []BasicSelectedMod `json:"selectedMods"`
}

func (obj *SelectedMod) Init() error {

	if err := obj.CheckInstalledOnDisk(); err != nil {
		return err
	}

	if err := obj.CheckMeetsDesiredVersion(); err != nil {
		return err
	}

	if err := obj.GetModConfig(); err != nil {
		return err
	}

	return nil
}

func (obj *SelectedMod) CheckInstalledOnDisk() error {
	utils.CreateFolder(config.GetConfig().ModsDir)

	modPath := filepath.Join(config.GetConfig().ModsDir, obj.Mod.ModReference)

	if !utils.CheckFileExists(modPath) {
		obj.Installed = false
		obj.InstalledVersion = "0.0.0"
		return nil
	}

	UPluginPath := filepath.Join(modPath, obj.Mod.ModReference+".uplugin")

	if !utils.CheckFileExists(UPluginPath) {
		obj.Installed = false
		obj.InstalledVersion = "0.0.0"
		return nil
	}

	var UPluginData = UPluginFile{}

	b, _ := os.ReadFile(UPluginPath)
	if err := json.Unmarshal([]byte(b), &UPluginData); err != nil {
		obj.Installed = false
		obj.InstalledVersion = "0.0.0"
		return err
	}

	obj.Installed = true
	obj.InstalledVersion = UPluginData.SemVersion

	return nil
}

func (obj *SelectedMod) CheckMeetsDesiredVersion() error {

	if !obj.Installed {
		return nil
	}

	installedVersion := "v" + obj.InstalledVersion
	desiredVersion := "v" + obj.DesiredVersion

	versionDiff := semver.Compare(installedVersion, desiredVersion)

	if versionDiff != 0 {
		obj.Installed = false
	}

	return nil
}

func (obj *SelectedMod) GetModConfig() error {
	if !obj.Installed {
		return nil
	}

	utils.CreateFolder(config.GetConfig().ModConfigsDir)

	configfile := filepath.Join(config.GetConfig().ModConfigsDir, obj.Mod.ModReference+".cfg")

	if utils.CheckFileExists(configfile) {

		data, err := os.ReadFile(configfile)
		if err != nil {
			return err
		}

		obj.Config = string(data)
	} else {

		if obj.Mod.ModReference == "SatisfactoryServerManager" {
			d1 := []byte("{\"apiKey\":\"" + config.GetConfig().APIKey + "\", \"url\":\"" + config.GetConfig().URL + "\"}")
			if err := os.WriteFile(configfile, d1, 0777); err != nil {
				return err
			}
			return nil
		}

		d1 := []byte("{}")
		if err := os.WriteFile(configfile, d1, 0777); err != nil {
			return err
		}
	}

	return nil
}

func (obj *SelectedMod) DownloadVersion(version ModVersion) error {
	ModFileName := obj.Mod.ModReference + "." + version.Version + ".zip"
	DownloadedModFilePath := filepath.Join(ModCachePatch, ModFileName)

	if utils.CheckFileExists(DownloadedModFilePath) {
		return nil
	}

	var versionTarget ModVersionTarget

	for _, vt := range version.Targets {
		if vt.TargetName == vars.ModPlatform {
			versionTarget = vt
		}
	}

	if versionTarget.Link == "" {
		return fmt.Errorf("mod version has no link")
	}

	url := fmt.Sprintf("https://api.ficsit.app%s", versionTarget.Link)

	err := api.DownloadNonSSMFile(url, DownloadedModFilePath)

	return err
}
