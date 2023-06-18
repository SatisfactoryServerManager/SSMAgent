package mod

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SatisfactoryServerManager/SSMAgent/app/api"
	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/sf"
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
	ModReference   string
	ModPath        string
	ModDisplayName string `json:"FriendlyName"`
	ModUPluginPath string
	ModVersion     string `json:"SemVersion"`
}

var (
	_ModState      ModState
	_InstalledMods []InstalledMod
	_ModCachePath  string
	_quit          = make(chan int)
)

func InitModManager() {

	log.Println("Initialising Mod Manager...")

	_ModCachePath = filepath.Join(config.GetConfig().DataDir, "modcache")
	utils.CreateFolder(_ModCachePath)

	GetModState()
	log.Println("Initialised Mod Manager")

	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				GetModState()
			case <-_quit:
				ticker.Stop()
				return
			}
		}
	}()
}

func ShutdownModManager() error {
	log.Println("Shutting Down Mod Manager...")
	_quit <- 0
	log.Println("Shutdown Mod Manager")

	return nil
}

func FindInstalledMods() {
	fmt.Printf("Finding Mods in: %s\r\n", config.GetConfig().ModsDir)

	files, err := os.ReadDir(config.GetConfig().ModsDir)
	if err != nil {
		log.Printf("Error cand open mods directory %s\r\n", err.Error())
		return
	}

	_InstalledMods = make([]InstalledMod, 0)

	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		modName := file.Name()
		modPath := filepath.Join(config.GetConfig().ModsDir, modName)
		UPluginPath := filepath.Join(modPath, modName+".uplugin")

		if !utils.CheckFileExists(UPluginPath) {
			continue
		}

		fmt.Printf("Found Mod (%s) at %s\r\n", modName, modPath)

		var newInstalledMod = InstalledMod{
			ModReference:   modName,
			ModPath:        modPath,
			ModUPluginPath: UPluginPath,
		}

		file, _ := os.ReadFile(UPluginPath)
		_ = json.Unmarshal([]byte(file), &newInstalledMod)

		_InstalledMods = append(_InstalledMods, newInstalledMod)
	}

}

func GetInstalledMod(modReference string) *InstalledMod {

	for idx := range _InstalledMods {
		mod := &_InstalledMods[idx]

		if mod.ModReference == modReference {
			return mod
		}
	}

	return nil
}

func IsModInstalled(modReference string) bool {
	return GetInstalledMod(modReference) != nil
}

func DoesInstalledModMeetVersion(modReference string, version string) bool {

	mod := GetInstalledMod(modReference)

	if mod == nil {
		return false
	}

	installedVersion := "v" + mod.ModVersion
	desiredVersion := "v" + version

	versionDiff := semver.Compare(installedVersion, desiredVersion)

	// fmt.Printf("mod Ref: %s, version dif %d\r\n", modReference, versionDiff)
	// fmt.Printf("installed ver: %s, version %s\r\n", installedVersion, desiredVersion)
	return versionDiff == 0
}

func GetModState() {

	FindInstalledMods()

	err := api.SendGetRequest("/api/agent/modstate", &_ModState)
	if err != nil {
		log.Printf("Failed to get Mod State with error: %s\r\n", err.Error())
		return
	}

	ProcessModState()
	SendModState()
}

func ProcessModState() {

	for idx := range _ModState.SelectedMods {
		selectedMod := &_ModState.SelectedMods[idx]

		if !IsModInstalled(selectedMod.Mod.ModReference) {
			selectedMod.Installed = false
			continue
		}

		if !DoesInstalledModMeetVersion(selectedMod.Mod.ModReference, selectedMod.DesiredVersion) {
			selectedMod.Installed = false
			continue
		}

		mod := GetInstalledMod(selectedMod.Mod.ModReference)

		selectedMod.Installed = true
		selectedMod.InstalledVersion = mod.ModVersion

	}

	SMLMod := GetInstalledMod("SML")

	if SMLMod != nil {
		_ModState.SMLInstalled = true
		_ModState.InstalledSMLVersion = SMLMod.ModVersion
	} else {
		_ModState.SMLInstalled = false
		_ModState.InstalledSMLVersion = "0.0.0"
	}

	InstallAllMods()
	InstallSML()
}

func InstallAllMods() {

	if sf.IsRunning() {
		return
	}

	for idx := range _ModState.SelectedMods {
		selectedMod := &_ModState.SelectedMods[idx]

		if selectedMod.Installed {
			continue
		}

		var modVersion ModVersion

		for _, mv := range selectedMod.Mod.Versions {
			versiondiff := semver.Compare(selectedMod.DesiredVersion, mv.Version)

			if versiondiff == 0 {
				modVersion = mv
			}
		}

		if len(modVersion.Targets) == 0 {
			continue
		}

		err := DownloadMod(selectedMod.Mod.ModReference, modVersion)

		if err != nil {
			log.Printf("Failed to download mod (%s)\r\n", selectedMod.Mod.ModReference)
			continue
		}

		log.Printf("Downloaded mod (%s)\r\n", selectedMod.Mod.ModReference)

		ModFileName := selectedMod.Mod.ModReference + "." + modVersion.Version + ".zip"
		DownloadedModFilePath := filepath.Join(_ModCachePath, ModFileName)

		modFile, err := os.Open(DownloadedModFilePath)

		if err != nil {
			continue
		}
		modPath := filepath.Join(config.GetConfig().ModsDir, selectedMod.Mod.ModReference)

		ExtractArchive(modFile, modPath)
	}

	FindInstalledMods()
}

func DownloadMod(modReference string, modVersion ModVersion) error {

	ModFileName := modReference + "." + modVersion.Version + ".zip"
	DownloadedModFilePath := filepath.Join(_ModCachePath, ModFileName)

	if utils.CheckFileExists(DownloadedModFilePath) {
		return nil
	}

	var versionTarget ModVersionTarget

	for _, vt := range modVersion.Targets {
		if vt.TargetName == vars.ModPlatform {
			versionTarget = vt
		}
	}

	if versionTarget.Link == "" {
		return errors.New("mod version has no link")
	}

	url := "https://ficsit-api.mircearoata.duckdns.org" + versionTarget.Link

	err := api.DownloadNonSSMFile(url, DownloadedModFilePath)

	return err
}

func ExtractArchive(file *os.File, modDirectory string) error {
	log.Printf("Extracting Mod (%s) ...\r\n", file.Name())

	archive, err := zip.OpenReader(file.Name())
	if err != nil {
		return err
	}
	defer archive.Close()

	if utils.CheckFileExists(modDirectory) {
		os.RemoveAll(modDirectory)
	}

	for _, f := range archive.File {
		filePath := filepath.Join(modDirectory, f.Name)
		fmt.Println("unzipping file ", filePath)

		if !strings.HasPrefix(filePath, filepath.Clean(modDirectory)+string(os.PathSeparator)) {
			return nil
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(filePath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			return err
		}

		dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
		if err != nil {
			return err
		}

		fileInArchive, err := f.Open()
		if err != nil {
			return err
		}

		if _, err := io.Copy(dstFile, fileInArchive); err != nil {
			return err
		}

		dstFile.Close()
		fileInArchive.Close()
	}

	err = file.Close()
	if err != nil {
		return err
	}

	err = archive.Close()
	if err != nil {
		return err
	}

	log.Printf("Extracted Mod (%s)\r\n", file.Name())

	return nil
}

func InstallSML() {

	var MaxSMLVersion = "v0.0.0"

	for idx := range _ModState.SelectedMods {
		selectedMod := &_ModState.SelectedMods[idx]

		desiredVer := "v" + selectedMod.DesiredVersion
		for _, mv := range selectedMod.Mod.Versions {

			mvVersion := "v" + mv.Version

			verdiff := semver.Compare(mvVersion, desiredVer)

			if verdiff == 0 {
				smlver := "v" + strings.Replace(mv.SMLVersion, "^", "", -1)

				if semver.Compare(smlver, MaxSMLVersion) > 0 {
					MaxSMLVersion = smlver
				}
			}

		}
	}

	fmt.Printf("Found Max SML Version: %s\r\n", MaxSMLVersion)

	installedSMLVersion := "v" + _ModState.InstalledSMLVersion

	verDiff := semver.Compare(installedSMLVersion, MaxSMLVersion)

	fmt.Printf("SML Version %s,%s\r\n", installedSMLVersion, MaxSMLVersion)

	if verDiff == 0 {
		return
	}

	MaxSMLVersion = strings.Replace(MaxSMLVersion, "v", "", -1)

	err := DownloadSML(MaxSMLVersion)

	if err != nil {
		log.Printf("[Error] - Couldn't Download SML error: %s\r\n", err.Error())
		_ModState.InstalledSMLVersion = "0.0.0"
		return
	}

	ModFileName := "SML." + MaxSMLVersion + ".zip"
	DownloadedModFilePath := filepath.Join(_ModCachePath, ModFileName)

	modFile, err := os.Open(DownloadedModFilePath)

	if err != nil {
		return
	}
	modPath := filepath.Join(config.GetConfig().ModsDir, "SML")

	err = ExtractArchive(modFile, modPath)
	if err != nil {
		log.Printf("[Error] - Couldn't Extract SML error: %s\r\n", err.Error())
		_ModState.InstalledSMLVersion = "0.0.0"
		return
	}

	_ModState.InstalledSMLVersion = MaxSMLVersion
}

func DownloadSML(smlVersion string) error {

	ModFileName := "SML." + smlVersion + ".zip"
	DownloadedModFilePath := filepath.Join(_ModCachePath, ModFileName)

	if utils.CheckFileExists(DownloadedModFilePath) {
		return nil
	}

	url := "https://github.com/satisfactorymodding/SatisfactoryModLoader/releases/download/v" + smlVersion + "/SML.zip"

	err := api.DownloadNonSSMFile(url, DownloadedModFilePath)

	return err
}

func SendModState() error {

	var resData interface{}

	err := api.SendPostRequest("/api/agent/modstate", _ModState, resData)
	return err

}
