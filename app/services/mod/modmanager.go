package mod

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SatisfactoryServerManager/SSMAgent/app/api"
	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/sf"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	"golang.org/x/mod/semver"
)

var (
	_ModState     ModState
	ModCachePatch string
	_quit         = make(chan int)
)

func InitModManager() {

	utils.InfoLogger.Println("Initialising Mod Manager...")

	ModCachePatch = filepath.Join(config.GetConfig().DataDir, "modcache")
	utils.CreateFolder(ModCachePatch)

	GetModState()
	utils.InfoLogger.Println("Initialised Mod Manager")

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
	utils.InfoLogger.Println("Shutting Down Mod Manager...")
	_quit <- 0
	utils.InfoLogger.Println("Shutdown Mod Manager")

	return nil
}

func GetModState() {

	FindModsOnDisk()

	err := api.SendGetRequest("/api/v1/agent/modconfig", &_ModState)
	if err != nil {
		utils.ErrorLogger.Printf("Failed to get Mod State with error: %s\r\n", err.Error())
		return
	}

	ProcessModState()
	SendModState()
}

func FindModsOnDisk() []InstalledMod {

	installedMods := make([]InstalledMod, 0)

	utils.DebugLogger.Printf("Finding Mods in: %s\r\n", config.GetConfig().ModsDir)

	files, err := os.ReadDir(config.GetConfig().ModsDir)
	if err != nil {
		utils.ErrorLogger.Printf("Error cand open mods directory %s\r\n", err.Error())
		return installedMods
	}

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

		utils.DebugLogger.Printf("Found Mod (%s) at %s\r\n", modName, modPath)

		var newInstalledMod = InstalledMod{
			ModReference:   modName,
			ModPath:        modPath,
			ModUPluginPath: UPluginPath,
		}

		file, _ := os.ReadFile(UPluginPath)
		_ = json.Unmarshal([]byte(file), &newInstalledMod)

		installedMods = append(installedMods, newInstalledMod)
	}

	return installedMods
}

func ProcessModState() {

	utils.CreateFolder(config.GetConfig().ModsDir)

	for idx := range _ModState.SelectedMods {
		selectedMod := &_ModState.SelectedMods[idx]

		if err := selectedMod.Init(); err != nil {
			utils.ErrorLogger.Printf("error initialising selected mod with error %s\n", err.Error())
			continue
		}
	}

	installedMods := FindModsOnDisk()

	for idx := range installedMods {
		installedMod := &installedMods[idx]

		foundSelectedMod := false

		for _, sm := range _ModState.SelectedMods {
			if sm.Mod.ModReference == installedMod.ModReference {
				foundSelectedMod = true
			}
		}

		if !foundSelectedMod {
			err := UninstallMod(installedMod.ModReference)
			if err != nil {
				utils.ErrorLogger.Printf("Error failed to uninstall mod (%s) with error: %s\r\n", installedMod.ModReference, err.Error())
				continue
			}
		}
	}

	if err := InstallAllMods(); err != nil {
		utils.ErrorLogger.Printf("error failed to install mods with error: %s\n", err.Error())
	}
}

func InstallAllMods() error {

	if sf.IsRunning() {
		return nil
	}

	for idx := range _ModState.SelectedMods {
		selectedMod := &_ModState.SelectedMods[idx]

		if selectedMod.Installed {
			continue
		}

		utils.DebugLogger.Printf("Installing Mod: %s", selectedMod.Mod.ModReference)

		var modVersion ModVersion

		for _, mv := range selectedMod.Mod.Versions {
			versiondiff := semver.Compare(selectedMod.DesiredVersion, mv.Version)

			if versiondiff == 0 {
				modVersion = mv
				break
			}
		}

		if len(modVersion.Targets) == 0 {
			utils.DebugLogger.Printf("Skipping mod install %s with reason: no mod version targets\n", selectedMod.Mod.ModReference)
			continue
		}

		if err := selectedMod.DownloadVersion(modVersion); err != nil {
			utils.WarnLogger.Printf("Failed to download mod (%s)\r\n", selectedMod.Mod.ModReference)
			continue
		}

		utils.InfoLogger.Printf("Downloaded mod (%s)\r\n", selectedMod.Mod.ModReference)

		ModFileName := selectedMod.Mod.ModReference + "." + modVersion.Version + ".zip"
		DownloadedModFilePath := filepath.Join(ModCachePatch, ModFileName)

		modPath := filepath.Join(config.GetConfig().ModsDir, selectedMod.Mod.ModReference)

		if err := ExtractArchive(DownloadedModFilePath, modPath); err != nil {
			return fmt.Errorf("error extracting mod zip file with error: %s", err.Error())
		}

		if err := selectedMod.CheckInstalledOnDisk(); err != nil {
			return err
		}
	}

	return nil
}

func ExtractArchive(modFilePath string, modDirectory string) error {

	file, err := os.Open(modFilePath)

	if err != nil {
		return err
	}

	utils.InfoLogger.Printf("Extracting Mod (%s) ...\r\n", file.Name())

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
		utils.DebugLogger.Println("unzipping file ", filePath)

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

	utils.InfoLogger.Printf("Extracted Mod (%s)\r\n", file.Name())

	return nil
}

func UpdateModConfigFile(modReference string, modConfig string) error {

	if modReference == "" {
		return errors.New("mod reference is null")
	}

	if sf.IsRunning() {
		return errors.New("sf server is running")
	}

	utils.CreateFolder(config.GetConfig().ModConfigsDir)

	modReference = strings.Replace(modReference, "\"", "", -1)

	configfile := filepath.Join(config.GetConfig().ModConfigsDir, modReference+".cfg")

	if err := os.WriteFile(configfile, []byte(modConfig), 0777); err != nil {
		return err
	}

	return nil
}

func SendModState() error {

	newModState := BasicModState{
		SelectedMods: make([]BasicSelectedMod, 0),
	}

	for _, sm := range _ModState.SelectedMods {
		newMod := BasicMod{
			ModReference: sm.Mod.ModReference,
		}
		newSelectedMod := BasicSelectedMod{
			Mod:              newMod,
			Installed:        sm.Installed,
			InstalledVersion: sm.InstalledVersion,
			Config:           sm.Config,
		}
		newModState.SelectedMods = append(newModState.SelectedMods, newSelectedMod)
	}

	var resData interface{}

	err := api.SendPutRequest("/api/v1/agent/modconfig", newModState, resData)
	return err
}

func UninstallMod(modReference string) error {

	allInstalledMods := FindModsOnDisk()

	var installedMod *InstalledMod

	for idx := range allInstalledMods {
		i := &allInstalledMods[idx]

		if i.ModReference == modReference {
			installedMod = i
		}
	}

	if installedMod == nil {
		return nil
	}

	if !utils.CheckFileExists(installedMod.ModPath) {
		return nil
	}

	utils.InfoLogger.Printf("Uninstalling Mod (%s) ...\r\n", modReference)

	err := os.RemoveAll(installedMod.ModPath)

	if err != nil {
		return err
	}

	utils.InfoLogger.Printf("Uninstalled mod (%s)\r\n", modReference)
	return nil
}
