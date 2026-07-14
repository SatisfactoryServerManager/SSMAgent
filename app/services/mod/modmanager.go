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

	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/types"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
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

// InstallModArchive unpacks a mod and puts it where the game will load it from.
//
// stagingDir is where an archive is unpacked before it is moved into place. It is
// invisible to findModsInDir, which requires a <dirname>/<dirname>.uplugin that
// ".staging" can never have - so a half-extracted mod left here by a killed agent
// is never reported as installed, and never lands in a sync plan's Remove list.
func stagingDir() string {
	return filepath.Join(config.GetConfig().ModsDir, ".staging")
}

// The GameFeature flag lives in the .uplugin inside the archive, so the target
// directory is not known until the archive has been unpacked. Extract to a
// staging directory first, read the manifest, then move it into place.
func InstallModArchive(modFilePath string, modReference string) error {

	// Staging lives inside ModsDir, NOT under DataDir: the final step is an
	// os.Rename of the staging directory into ModsDir, and that rename happens
	// AFTER the old version has been deleted. DataDir and SFDir (which ModsDir
	// lives under) are independent flags and are on different filesystems in a
	// normal Docker layout, where a cross-device rename fails with EXDEV — losing
	// the mod entirely, identically on every retry. Staging under ModsDir keeps the
	// rename intra-filesystem.
	staging := filepath.Join(stagingDir(), modReference)
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

func WriteModConfigFile(modReference string, modConfig string) error {

	if modReference == "" {
		return errors.New("mod reference is null")
	}

	// No sf.IsRunning() guard here: Sync asserts the server is stopped up front,
	// and a second check firing mid-sync would fail the task after half the mods
	// had already been written.
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
