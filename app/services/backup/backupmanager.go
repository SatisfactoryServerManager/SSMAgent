package backup

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/SatisfactoryServerManager/SSMAgent/app/api"
	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/savemanager"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
)

var _quit = make(chan int)

func InitBackupManager() {

	utils.InfoLogger.Println("Initialising Backup Manager...")
	CreateBackupFile()

	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for {
			select {
			case <-ticker.C:
				AutoBackup()
			case <-_quit:
				ticker.Stop()
				return
			}
		}
	}()

	utils.InfoLogger.Println("Initialised Backup Manager")
}

func ShutdownBackupManager() error {
	_quit <- 0
	return nil
}

func AutoBackup() {
	t := time.Now()

	if t.After(config.GetConfig().Backup.NextBackup) {
		CreateBackupFile()

		nextAdd := time.Duration(config.GetConfig().Backup.Interval) * time.Hour

		config.GetConfig().Backup.NextBackup = t.Add(nextAdd)
		config.SaveConfig()
	}
}

func CreateBackupFile() error {
	t := time.Now()

	formatted := fmt.Sprintf("%d-%02d-%02d_%02d%02d",
		t.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute())

	zipFileName := "Backup_" + formatted + ".zip"
	zipFilePath := filepath.Join(config.GetConfig().BackupDir, zipFileName)

	utils.InfoLogger.Printf("Creating backup %s\r\n", zipFileName)

	archive, err := os.Create(zipFilePath)
	if err != nil {
		return err
	}
	defer archive.Close()
	zipWriter := zip.NewWriter(archive)

	logFile := filepath.Join(config.GetConfig().LogDir, "SSMAgent-combined.log")

	err = AddFileToZipZile(zipWriter, logFile, "Logs/SSMAgent-combined.log")
	if err != nil {
		return err
	}
	err = AddFileToZipZile(zipWriter, config.ConfigFile, "Configs/SSM.json")
	if err != nil {
		return err
	}

	gamelogdir := filepath.Join(
		config.GetConfig().SFDir,
		"FactoryGame",
		"Saved",
		"Logs",
	)

	utils.CreateFolder(gamelogdir)

	gamelogfile := filepath.Join(
		gamelogdir,
		"FactoryGame.log",
	)

	err = AddFileToZipZile(zipWriter, gamelogfile, "Logs/FactoryGame.log")
	if err != nil {
		return err
	}

	savemanager.GetSaveFiles()

	for _, saveSession := range savemanager.GetSaveSessions() {
		for _, saveFile := range saveSession.SaveFiles {
			err = AddFileToZipZile(zipWriter, saveFile.FilePath, "Saves/"+saveSession.SessionName+"/"+saveFile.FileName)
			if err != nil {
				return err
			}
		}
	}

	zipWriter.Close()

	utils.InfoLogger.Println("Backup created successfully")

	err = api.SendFile("/api/agent/uploadbackup", zipFilePath)
	if err != nil {
		return err
	}
	return nil
}

func AddFileToZipZile(zipWriter *zip.Writer, filePath string, destPath string) error {

	if !utils.CheckFileExists(filePath) {
		return nil
	}

	utils.DebugLogger.Printf("Adding File: %s\r\n", filePath)
	f1, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f1.Close()

	w1, err := zipWriter.Create(destPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w1, f1); err != nil {
		return err
	}

	return nil
}
