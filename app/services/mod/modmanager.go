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

	"github.com/SatisfactoryServerManager/SSMAgent/app/api"
	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/sf"
	"github.com/SatisfactoryServerManager/SSMAgent/app/types"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	"github.com/SatisfactoryServerManager/SSMAgent/app/vars"
	"github.com/SatisfactoryServerManager/ssmcloud-resources/models"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"golang.org/x/mod/semver"
)

// GameFeaturesDirName is the subdirectory of Mods that the game loads game
// feature plugins from. A game feature mod placed alongside the ordinary mods is
// simply never loaded.
const GameFeaturesDirName = "GameFeatures"

func gameFeaturesDir() string {
	return filepath.Join(config.GetConfig().ModsDir, GameFeaturesDirName)
}

// modInstallDir is where a mod belongs, given its GameFeature flag.
func modInstallDir(modReference string, gameFeature bool) string {
	if gameFeature {
		return filepath.Join(gameFeaturesDir(), modReference)
	}
	return filepath.Join(config.GetConfig().ModsDir, modReference)
}

// findModDir returns the directory a mod is actually installed in, checking both
// layouts. A mod that changes its GameFeature flag between versions moves, so its
// location on disk cannot be inferred from the manifest we are about to install.
func findModDir(modReference string) (string, bool) {
	for _, gameFeature := range []bool{false, true} {
		dir := modInstallDir(modReference, gameFeature)

		if utils.CheckFileExists(filepath.Join(dir, modReference+".uplugin")) {
			return dir, true
		}
	}
	return "", false
}

func FindModsOnDisk() []types.InstalledMod {

	installedMods := make([]types.InstalledMod, 0)

	installedMods = append(installedMods, findModsInDir(config.GetConfig().ModsDir)...)
	installedMods = append(installedMods, findModsInDir(gameFeaturesDir())...)

	return installedMods
}

func findModsInDir(dir string) []types.InstalledMod {

	installedMods := make([]types.InstalledMod, 0)

	files, err := os.ReadDir(dir)
	if err != nil {
		// GameFeatures only exists once a game feature mod has been installed.
		if !os.IsNotExist(err) {
			utils.ErrorLogger.Printf("error cant open mods directory %s\r\n", err.Error())
		}
		return installedMods
	}

	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		modName := file.Name()

		// GameFeatures is a container, not a mod. It is scanned separately.
		if modName == GameFeaturesDirName {
			continue
		}

		modPath := filepath.Join(dir, modName)
		UPluginPath := filepath.Join(modPath, modName+".uplugin")

		if !utils.CheckFileExists(UPluginPath) {
			continue
		}

		var newInstalledMod = types.InstalledMod{
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

func ProcessModConfig(modConfig *v2.AgentModConfig) error {
    
	utils.CreateFolder(config.GetConfig().ModsDir)

	for idx := range modConfig.SelectedMods {
		selectedMod := &modConfig.SelectedMods[idx]

		if err := CheckSelectedModInstalledOnDisk(selectedMod); err != nil {
			utils.ErrorLogger.Printf("error checking selected mod installed with error %s\n", err.Error())
			continue
		}

		if err := CheckSelectedModVersion(selectedMod); err != nil {
			utils.ErrorLogger.Printf("error checking selected mod versions with error %s\n", err.Error())
			continue
		}

		if err := GetSelectedModConfig(selectedMod); err != nil {
			utils.ErrorLogger.Printf("error getting selected mod config with error %s\n", err.Error())
			continue
		}
	}

	installedMods := FindModsOnDisk()

	for idx := range installedMods {
		installedMod := &installedMods[idx]

		foundSelectedMod := false

		for _, sm := range modConfig.SelectedMods {
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

	if err := InstallAllMods(modConfig); err != nil {
		return fmt.Errorf("error failed to install mods with error: %s", err.Error())
	}

	return nil
}

func InstallAllMods(modConfig *v2.AgentModConfig) error {

	if sf.IsRunning() {
		return nil
	}

	ModCachePatch := filepath.Join(config.GetConfig().DataDir, "modcache")
	utils.CreateFolder(ModCachePatch)

	for idx := range modConfig.SelectedMods {
		selectedMod := &modConfig.SelectedMods[idx]

		if selectedMod.Installed {
			continue
		}

		var modVersion models.ModVersion

		for _, mv := range selectedMod.Mod.Versions {
			versiondiff := semver.Compare("v"+selectedMod.DesiredVersion, "v"+mv.Version)

			if versiondiff == 0 {
				modVersion = mv
				break
			}
		}

		utils.DebugLogger.Printf("Installing Mod: %s - %s", selectedMod.Mod.ModReference, modVersion.Version)

		if len(modVersion.Targets) == 0 {
			utils.DebugLogger.Printf("Skipping mod install %s with reason: no mod version targets\n", selectedMod.Mod.ModReference)
			continue
		}

		if err := DownloadSelectedModVersion(selectedMod, modVersion); err != nil {
			utils.WarnLogger.Printf("Failed to download mod (%s) with error: %s\n", selectedMod.Mod.ModReference, err.Error())
			continue
		}

		utils.InfoLogger.Printf("Downloaded mod (%s)\r\n", selectedMod.Mod.ModReference)

		ModFileName := selectedMod.Mod.ModReference + "." + modVersion.Version + ".zip"
		DownloadedModFilePath := filepath.Join(ModCachePatch, ModFileName)

		if err := InstallModArchive(DownloadedModFilePath, selectedMod.Mod.ModReference); err != nil {
			return fmt.Errorf("error installing mod zip file with error: %s", err.Error())
		}

		if err := CheckSelectedModInstalledOnDisk(selectedMod); err != nil {
			return err
		}
	}

	return nil
}

// InstallModArchive unpacks a mod and puts it where the game will load it from.
//
// The GameFeature flag lives in the .uplugin inside the archive, so the target
// directory is not known until the archive has been unpacked. Extract to a
// staging directory first, read the manifest, then move it into place.
func InstallModArchive(modFilePath string, modReference string) error {

	staging := filepath.Join(config.GetConfig().DataDir, "modcache", ".staging", modReference)
	defer os.RemoveAll(staging)

	if err := ExtractArchive(modFilePath, staging); err != nil {
		return err
	}

	gameFeature, err := IsGameFeatureMod(staging, modReference)
	if err != nil {
		return err
	}

	// The flag can change between versions, so clear both layouts. Leaving the old
	// copy behind would load the mod twice, or load the stale one.
	for _, gf := range []bool{false, true} {
		if err := os.RemoveAll(modInstallDir(modReference, gf)); err != nil {
			return err
		}
	}

	modPath := modInstallDir(modReference, gameFeature)

	if err := os.MkdirAll(filepath.Dir(modPath), os.ModePerm); err != nil {
		return err
	}

	if err := os.Rename(staging, modPath); err != nil {
		return err
	}

	if gameFeature {
		utils.InfoLogger.Printf("Installed game feature mod (%s) to %s\r\n", modReference, modPath)
	} else {
		utils.InfoLogger.Printf("Installed mod (%s) to %s\r\n", modReference, modPath)
	}

	return nil
}

// IsGameFeatureMod reads the GameFeature flag out of an unpacked mod's manifest.
func IsGameFeatureMod(modDirectory string, modReference string) (bool, error) {

	UPluginPath := filepath.Join(modDirectory, modReference+".uplugin")

	if !utils.CheckFileExists(UPluginPath) {
		return false, fmt.Errorf("mod (%s) has no %s.uplugin", modReference, modReference)
	}

	b, err := os.ReadFile(UPluginPath)
	if err != nil {
		return false, err
	}

	var UPluginData = types.UPluginFile{}
	if err := json.Unmarshal(b, &UPluginData); err != nil {
		return false, err
	}

	return UPluginData.GameFeature, nil
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

		// Returning nil here would abandon the extraction half-done and report
		// success, leaving a partial mod on disk.
		if !strings.HasPrefix(filePath, filepath.Clean(modDirectory)+string(os.PathSeparator)) {
			return fmt.Errorf("mod archive contains an illegal path: %s", f.Name)
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

func UninstallMod(modReference string) error {

	allInstalledMods := FindModsOnDisk()

	var installedMod *types.InstalledMod

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

func CheckSelectedModInstalledOnDisk(selectedMod *v2.AgentModConfigSelectedModSchema) error {
	utils.CreateFolder(config.GetConfig().ModsDir)

	// A game feature mod lives under Mods/GameFeatures, so looking only in Mods
	// would report it missing and reinstall it on every pass.
	modPath, found := findModDir(selectedMod.Mod.ModReference)

	if !found {
		selectedMod.Installed = false
		selectedMod.InstalledVersion = "0.0.0"
		return nil
	}

	UPluginPath := filepath.Join(modPath, selectedMod.Mod.ModReference+".uplugin")

	var UPluginData = types.UPluginFile{}

	b, _ := os.ReadFile(UPluginPath)
	if err := json.Unmarshal([]byte(b), &UPluginData); err != nil {
		selectedMod.Installed = false
		selectedMod.InstalledVersion = "0.0.0"
		return err
	}

	selectedMod.Installed = true
	selectedMod.InstalledVersion = UPluginData.SemVersion

	return nil
}

func CheckSelectedModVersion(selectedMod *v2.AgentModConfigSelectedModSchema) error {

	if !selectedMod.Installed {
		return nil
	}

	installedVersion := "v" + selectedMod.InstalledVersion
	desiredVersion := "v" + selectedMod.DesiredVersion

	versionDiff := semver.Compare(installedVersion, desiredVersion)

	if versionDiff != 0 {
		selectedMod.Installed = false
	}

	return nil
}

func GetSelectedModConfig(selectedMod *v2.AgentModConfigSelectedModSchema) error {
	if !selectedMod.Installed {
		return nil
	}

	utils.CreateFolder(config.GetConfig().ModConfigsDir)

	configfile := filepath.Join(config.GetConfig().ModConfigsDir, selectedMod.Mod.ModReference+".cfg")

	if utils.CheckFileExists(configfile) {

		data, err := os.ReadFile(configfile)
		if err != nil {
			return err
		}

		selectedMod.Config = string(data)
	} else {

		if selectedMod.Mod.ModReference == "SatisfactoryServerManager" {
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

func DownloadSelectedModVersion(selectedMod *v2.AgentModConfigSelectedModSchema, version models.ModVersion) error {
	ModCachePatch := filepath.Join(config.GetConfig().DataDir, "modcache")
	utils.CreateFolder(ModCachePatch)

	ModFileName := selectedMod.Mod.ModReference + "." + version.Version + ".zip"
	DownloadedModFilePath := filepath.Join(ModCachePatch, ModFileName)

	if utils.CheckFileExists(DownloadedModFilePath) {
		return nil
	}

	var versionTarget models.ModVersionTarget

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
