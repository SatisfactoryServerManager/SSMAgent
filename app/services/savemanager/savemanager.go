package savemanager

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/SatisfactoryServerManager/SSMAgent/app/api"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
)

type SaveFile struct {
	FilePath     string    `json:"filePath"`
	FileName     string    `json:"fileName"`
	ModTime      time.Time `json:"modTime"`
	UploadedTime time.Time `json:"-"`
	Size         int64     `json:"size"`
}

type HttpRequestBody_SaveInfo struct {
	SaveFiles []SaveFile `json:"saveFiles"`
}

var (
	_SaveFiles []SaveFile
	_quit      = make(chan int)
)

func InitSaveManager() {
	utils.InfoLogger.Println("Initialising Save Manager...")

	GetSaveFiles()
	UploadSaveFiles()

	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for {
			select {
			case <-ticker.C:
				GetSaveFiles()
				UploadSaveFiles()
			case <-_quit:
				ticker.Stop()
				return
			}
		}
	}()

	utils.InfoLogger.Println("Initialised Save Manager")
}

func ShutdownSaveManager() error {
	utils.InfoLogger.Println("Shutting down Save Manager")

	_quit <- 0

	utils.InfoLogger.Println("Shutdown Save Manager")
	return nil
}

func GetCachedSaveFiles() []SaveFile {
	return _SaveFiles
}

func GetSaveFiles() {
	saveDir, err := GetSaveDir()
	if err != nil {
		utils.ErrorLogger.Printf("Error getting Save Directory path %s\r\n", err.Error())
		return
	}

	err = utils.CreateFolder(saveDir)

	if err != nil {
		utils.ErrorLogger.Printf("Error creating Save Directory %s\r\n", err.Error())
		return
	}

	utils.DebugLogger.Printf("Finding Save Files in: %s\r\n", saveDir)

	files, err := os.ReadDir(saveDir)
	if err != nil {
		utils.ErrorLogger.Printf("Error cant open save directory %s\r\n", err.Error())
		return
	}

	var saveFiles = make([]SaveFile, 0)

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filePath := path.Join(saveDir, file.Name())
		fileInfo, _ := os.Stat(filePath)

		saveFile := SaveFile{
			FilePath: filePath,
			ModTime:  fileInfo.ModTime(),
			Size:     fileInfo.Size(),
			FileName: filepath.Base(filePath),
		}

		saveFiles = append(saveFiles, saveFile)
	}

	_SaveFiles = saveFiles
}

func UploadSaveFiles() {

	for idx := range _SaveFiles {
		saveFile := &_SaveFiles[idx]

		if saveFile.ModTime.After(saveFile.UploadedTime) {
			err := UploadSaveFile(saveFile.FilePath)

			if err != nil {
				utils.ErrorLogger.Printf("Error uploading save file: %s with error: %s\r\n", saveFile.FileName, err.Error())
				continue
			}

			saveFile.UploadedTime = time.Now()
		}

	}
}

func UploadSaveFile(filePath string) error {
	err := api.SendFile("/api/v1/agent/upload/save", filePath)
	return err
}

func DownloadSaveFile(fileName string) error {

	fileName = strings.Replace(fileName, "\"", "", -1)
	utils.DebugLogger.Printf("Downloading Save File: %s\r\n", fileName)

	saveDir, err := GetSaveDir()
	if err != nil {
		return err
	}

	newFilePath := filepath.Join(saveDir, filepath.Clean(fileName))

	err = api.DownloadFile("/api/v1/agent/saves/download/"+fileName, newFilePath)
	if err != nil {
		return err
	}

	utils.DebugLogger.Printf("Downloaded Save File to: %s\r\n", newFilePath)

	return nil
}
