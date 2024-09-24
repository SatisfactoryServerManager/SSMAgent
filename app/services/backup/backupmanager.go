package backup

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"archive/tar"
	"compress/gzip"

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

type zipFile struct {
	FilePath string
	DestPath string
}

func CreateBackupFile() error {
	t := time.Now()

	formatted := fmt.Sprintf("%d-%02d-%02d_%02d%02d",
		t.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute())

	zipFileName := "Backup_" + formatted + ".tar.gz"
	zipFilePath := filepath.Join(config.GetConfig().BackupDir, zipFileName)

	utils.InfoLogger.Printf("Creating backup %s\r\n", zipFileName)

	out, err := os.Create(zipFilePath)
	if err != nil {
		log.Fatalln("Error writing archive:", err)
	}
	defer out.Close()

	filesToZip := make([]zipFile, 0)

	// Old System

	logFile := filepath.Join(config.GetConfig().LogDir, "SSMAgent-combined.log")
	filesToZip = append(filesToZip, zipFile{
		FilePath: logFile,
		DestPath: "Logs/SSMAgent-combined.log",
	})

	filesToZip = append(filesToZip, zipFile{
		FilePath: config.ConfigFile,
		DestPath: "Configs/SSM.json",
	})

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

	filesToZip = append(filesToZip, zipFile{
		FilePath: gamelogfile,
		DestPath: "Logs/FactoryGame.log",
	})

	savemanager.GetSaveFiles()

	for _, saveFile := range savemanager.GetCachedSaveFiles() {

		filesToZip = append(filesToZip, zipFile{
			FilePath: saveFile.FilePath,
			DestPath: "Saves/" + saveFile.FileName,
		})
	}

	err = createArchive(filesToZip, out)
	if err != nil {
		utils.ErrorLogger.Println("Error creating archive:", err)
	}

	utils.InfoLogger.Println("Backup created successfully")

	err = api.SendFile("/api/v1/agent/upload/backup", zipFilePath)
	if err != nil {
		return err
	}
	return nil
}

func createArchive(files []zipFile, buf io.Writer) error {
	// Create new Writers for gzip and tar
	// These writers are chained. Writing to the tar writer will
	// write to the gzip writer which in turn will write to
	// the "buf" writer
	gw := gzip.NewWriter(buf)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Iterate over files and add them to the tar archive
	for _, file := range files {
		err := addToArchive(tw, file)
		if err != nil {
			return err
		}
	}

	return nil
}

func addToArchive(tw *tar.Writer, file zipFile) error {
	// Open the file which will be written into the archive
	f, err := os.Open(file.FilePath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Get FileInfo about our file providing file size, mode, etc.
	info, err := f.Stat()
	if err != nil {
		return err
	}

	// Create a tar Header from the FileInfo data
	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return err
	}

	// Use full path as name (FileInfoHeader only takes the basename)
	// If we don't do this the directory strucuture would
	// not be preserved
	// https://golang.org/src/archive/tar/common.go?#L626
	header.Name = file.DestPath

	// Write file header to the tar archive
	err = tw.WriteHeader(header)
	if err != nil {
		return err
	}

	// Copy file content to tar archive
	_, err = io.Copy(tw, f)
	if err != nil {
		return err
	}

	return nil
}
