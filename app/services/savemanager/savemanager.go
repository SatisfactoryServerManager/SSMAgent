package savemanager

import (
	"fmt"
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

var (
	_SaveFiles []SaveFile
	_quit      = make(chan int)
)

func InitSaveManager() {
	utils.InfoLogger.Println("Initialising Save Manager...")

	GetSaveFiles()
	if err := SyncSaveFiles(); err != nil {
		utils.ErrorLogger.Printf("error syncing saves with error: %s\n", err.Error())
	}

	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for {
			select {
			case <-ticker.C:
				GetSaveFiles()
				if err := SyncSaveFiles(); err != nil {
					utils.ErrorLogger.Printf("error syncing saves with error: %s\n", err.Error())
				}
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
			ModTime:  fileInfo.ModTime().UTC(),
			Size:     fileInfo.Size(),
			FileName: filepath.Base(filePath),
		}

		saveFiles = append(saveFiles, saveFile)
	}

	_SaveFiles = saveFiles
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

func SyncSaveFiles() error {

	resBody := api.HttpResponseBody_SaveSync{}
	if err := api.SendGetRequest("/api/v1/agent/save/sync", &resBody); err != nil {
		return err
	}

	// Check if the server needs to upload any outdated saves
	for apiidx := range resBody.Saves {
		apiSave := &resBody.Saves[apiidx]

		for localidx := range _SaveFiles {
			localSave := &_SaveFiles[localidx]

			if localSave.FileName == apiSave.FileName {
				if apiSave.ModTime.Unix() < localSave.ModTime.Unix() {
					fmt.Printf("localsave modTime: %d apiSave modTime: %d\n", localSave.ModTime.Unix(), apiSave.ModTime.Unix())
					apiSave.ModTime = localSave.ModTime
					apiSave.FilePath = localSave.FilePath
					apiSave.MarkForUpload = true
				}
			}
		}
	}

	// Check if server needs to upload new saves
	for localidx := range _SaveFiles {
		localSave := &_SaveFiles[localidx]

		foundSave := false
		for apiidx := range resBody.Saves {
			apiSave := &resBody.Saves[apiidx]
			if localSave.FileName == apiSave.FileName {
				foundSave = true
				break
			}
		}

		if !foundSave {
			fmt.Printf("save not found in api: %s\n", localSave.FileName)
			resBody.Saves = append(resBody.Saves, api.HttpResponseBody_SaveSync_Save{
				FileName:      localSave.FileName,
				FilePath:      localSave.FilePath,
				ModTime:       localSave.ModTime,
				Size:          localSave.Size,
				MarkForUpload: true,
			})
		}
	}

	// Check if server needs to download save files
	for apiidx := range resBody.Saves {
		apiSave := &resBody.Saves[apiidx]

		foundSave := false
		for localidx := range _SaveFiles {
			localSave := &_SaveFiles[localidx]

			if localSave.FileName == apiSave.FileName {
				foundSave = true
				break
			}
		}

		if !foundSave {
			apiSave.MarkForDownload = true
		}
	}

	// Check if the server needs to download any outdated saves
	for apiidx := range resBody.Saves {
		apiSave := &resBody.Saves[apiidx]

		for localidx := range _SaveFiles {
			localSave := &_SaveFiles[localidx]

			if localSave.FileName == apiSave.FileName {
				if apiSave.ModTime.Unix() > localSave.ModTime.Unix() {
					apiSave.ModTime = localSave.ModTime
					apiSave.MarkForDownload = true
				}
			}
		}
	}

	shouldSendPostSync := false
	for apiidx := range resBody.Saves {
		apiSave := &resBody.Saves[apiidx]

		if apiSave.MarkForUpload {
			if err := UploadSaveFile(apiSave.FilePath); err != nil {
				return err
			}
			shouldSendPostSync = true
		} else if apiSave.MarkForDownload {
			if err := DownloadSaveFile(apiSave.FileName); err != nil {
				return err
			}
			shouldSendPostSync = true
		}

	}

	if shouldSendPostSync {
		type emptyReturnData struct{}
		emptyRes := emptyReturnData{}

		if err := api.SendPostRequest("/api/v1/agent/save/sync", resBody, &emptyRes); err != nil {
			return err
		}
	}

	return nil
}
