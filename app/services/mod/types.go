package mod

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SatisfactoryServerManager/SSMAgent/app/api"
	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	"github.com/SatisfactoryServerManager/SSMAgent/app/vars"
	"golang.org/x/mod/semver"
)

type ModState struct {
	ID                  string        `json:"_id"`
	InstalledSMLVersion string        `json:"installedSMLVersion"`
	SMLInstalled        bool          `json:"smlInstalled"`
	SelectedMods        []SelectedMod `json:"selectedMods"`
}

type SelectedMod struct {
	ID               string `json:"_id"`
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

type SMLConfig struct {
	Installed        bool
	InstalledVersion string
	DesiredVersion   string
	ModPath          string
}

func (obj *SelectedMod) Init() error {

	if err := obj.CheckInstalledOnDisk(); err != nil {
		return err
	}

	if err := obj.CheckMeetsDesiredVersion(); err != nil {
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

	url := fmt.Sprintf("https://api.ficsit.dev%s", versionTarget.Link)

	err := api.DownloadNonSSMFile(url, DownloadedModFilePath)

	return err
}

func (obj *SMLConfig) Init() error {

	if obj.DesiredVersion == "" {
		obj.DesiredVersion = "v0.0.0"
	}

	return nil
}

func (obj *SMLConfig) Update(selectedMods []SelectedMod) error {

	obj.FindDesiredVersion(selectedMods)

	if err := obj.CheckInstalledOnDisk(); err != nil {
		return err
	}

	if err := obj.CheckMeetsDesiredVersion(); err != nil {
		return err
	}

	return nil
}

func (obj *SMLConfig) FindDesiredVersion(selectedMods []SelectedMod) {
	obj.DesiredVersion = "v0.0.0"

	for idx := range selectedMods {
		selectedMod := &selectedMods[idx]

		if !selectedMod.Installed {
			continue
		}

		desiredVer := "v" + selectedMod.InstalledVersion
		for _, mv := range selectedMod.Mod.Versions {

			mvVersion := "v" + mv.Version

			verdiff := semver.Compare(mvVersion, desiredVer)

			if verdiff == 0 {
				smlver := "v" + strings.Replace(mv.SMLVersion, "^", "", -1)

				if semver.Compare(smlver, obj.DesiredVersion) > 0 {
					obj.DesiredVersion = smlver
				}
			}
		}
	}

	_SMLConfig.DesiredVersion = strings.Replace(_SMLConfig.DesiredVersion, "v", "", -1)
}

func (obj *SMLConfig) CheckInstalledOnDisk() error {
	utils.CreateFolder(config.GetConfig().ModsDir)

	obj.ModPath = filepath.Join(config.GetConfig().ModsDir, "SML")

	if !utils.CheckFileExists(obj.ModPath) {
		obj.Installed = false
		obj.InstalledVersion = "0.0.0"
		return nil
	}

	UPluginPath := filepath.Join(obj.ModPath, "SML.uplugin")

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

func (obj *SMLConfig) CheckMeetsDesiredVersion() error {

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

func (obj *SMLConfig) Uninstall() error {

	if !utils.CheckFileExists(obj.ModPath) {
		return nil
	}

	utils.InfoLogger.Println("Uninstalling Mod (SML) ...\r\n")

	err := os.RemoveAll(obj.ModPath)

	if err != nil {
		return err
	}

	utils.InfoLogger.Println("Uninstalled mod (SML)\r\n")

	return nil
}

func (obj *SMLConfig) Install() error {

	ModFileName := "SML." + obj.DesiredVersion + ".zip"
	DownloadedModFilePath := filepath.Join(ModCachePatch, ModFileName)

	if utils.CheckFileExists(DownloadedModFilePath) {
		return nil
	}

	url := "https://github.com/satisfactorymodding/SatisfactoryModLoader/releases/download/v" + obj.DesiredVersion + "/" + vars.SMLFileName

	if err := api.DownloadNonSSMFile(url, DownloadedModFilePath); err != nil {
		return err
	}

	if err := ExtractArchive(DownloadedModFilePath, obj.ModPath); err != nil {
		return err
	}

	if err := obj.CheckInstalledOnDisk(); err != nil {
		return err
	}

	return nil
}
