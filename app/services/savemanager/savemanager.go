package savemanager

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/SatisfactoryServerManager/SSMAgent/app/api"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/savemanager/savedecoder"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
)

type SaveFileInfo struct {
	SessionName string    `json:"sessionName"`
	Level       string    `json:"level"`
	FileName    string    `json:"fileName"`
	FilePath    string    `json:"filePath"`
	ModTime     time.Time `json:"modTime"`
	Size        int64     `json:"size"`
}

type SaveFile struct {
	Level        string    `json:"level"`
	FilePath     string    `json:"filePath"`
	FileName     string    `json:"fileName"`
	ModTime      time.Time `json:"modTime"`
	UploadedTime time.Time `json:"-"`
	Size         int64     `json:"size"`
}

type SaveSession struct {
	SessionName string     `json:"sessionName"`
	SaveFiles   []SaveFile `json:"saveFiles"`
}

type HttpRequestBody_SaveInfo struct {
	SaveDatas []SaveSession `json:"saveDatas"`
}

var (
	_SaveSessions []SaveSession
	_quit         = make(chan int)
)

func InitSaveManager() {
	utils.InfoLogger.Println("Initialising Save Manager...")

	GetSaveFiles()
	UploadSaveFiles()
	UploadSaveInfo()

	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for {
			select {
			case <-ticker.C:
				GetSaveFiles()
				UploadSaveFiles()
				UploadSaveInfo()
			case <-_quit:
				ticker.Stop()
				return
			}
		}
	}()

	utils.InfoLogger.Println("Initialised Save Manager")
}

func GetSaveSessions() []SaveSession {
	return _SaveSessions
}

func ShutdownSaveManager() error {
	utils.InfoLogger.Println("Shutting down Save Manager")

	_quit <- 0

	utils.InfoLogger.Println("Shutdown Save Manager")
	return nil
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

	var saveFileInfos = make([]SaveFileInfo, 0)

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filepath := path.Join(saveDir, file.Name())
		fileInfo := GetSaveInfo(filepath)

		if fileInfo.Level == "" {
			continue
		}

		saveFileInfos = append(saveFileInfos, fileInfo)
	}

	for _, saveFileInfo := range saveFileInfos {

		var existingSaveSessionIndex = -1
		for idx := range _SaveSessions {
			saveSession := &_SaveSessions[idx]

			if saveSession.SessionName == saveFileInfo.SessionName {
				existingSaveSessionIndex = idx
			}
		}

		if existingSaveSessionIndex == -1 {
			var newSaveSession = SaveSession{SessionName: saveFileInfo.SessionName}

			var newSaveFile = SaveFile{
				Level:    saveFileInfo.Level,
				FileName: saveFileInfo.FileName,
				FilePath: saveFileInfo.FilePath,
				ModTime:  saveFileInfo.ModTime,
				Size:     saveFileInfo.Size,
			}

			newSaveSession.SaveFiles = append(newSaveSession.SaveFiles, newSaveFile)

			_SaveSessions = append(_SaveSessions, newSaveSession)
		} else {

			saveSession := &_SaveSessions[existingSaveSessionIndex]
			var existingSaveFileIndex = -1

			for sidx := range saveSession.SaveFiles {
				saveFile := &saveSession.SaveFiles[sidx]
				if saveFile.FilePath == saveFileInfo.FilePath {
					existingSaveFileIndex = sidx
					break
				}
			}

			if existingSaveFileIndex == -1 {
				var newSaveFile = SaveFile{
					Level:    saveFileInfo.Level,
					FileName: saveFileInfo.FileName,
					FilePath: saveFileInfo.FilePath,
					ModTime:  saveFileInfo.ModTime,
					Size:     saveFileInfo.Size,
				}

				saveSession.SaveFiles = append(saveSession.SaveFiles, newSaveFile)
			} else {
				saveFile := &saveSession.SaveFiles[existingSaveFileIndex]
				saveFile.ModTime = saveFileInfo.ModTime
				saveFile.Size = saveFileInfo.Size
			}

		}
	}
}

func GetSaveInfo(filePath string) SaveFileInfo {

	var res = SaveFileInfo{}
	res.FilePath = filePath

	fileInfo, _ := os.Stat(filePath)

	res.ModTime = fileInfo.ModTime()
	res.Size = fileInfo.Size()

	utils.DebugLogger.Printf("Reading File: %s\r\n", filePath)
	file, err := os.Open(filePath)
	if err != nil {
		utils.ErrorLogger.Printf("Failed to open save file %s with error: %s\r\n", filePath, err.Error())
		return SaveFileInfo{}
	}

	res.FileName = filepath.Base(filePath)

	decoder := savedecoder.NewSaveDecoder(file)

	decoder.Seek(12)

	level, _ := decoder.ReadString()
	res.Level = level

	sessionString, _ := decoder.ReadString()
	sessionSettings := strings.Split(sessionString, "=")

	if len(sessionSettings) > 1 {

		sessionNameData := strings.Split(sessionSettings[1], "?")
		res.SessionName = sessionNameData[0]

		if len(res.SessionName) > 50 {
			res.SessionName = "Unknown"
		}
	}

	decoder.Close()
	return res
}

func UploadSaveFiles() {
	for idx := range _SaveSessions {
		saveSession := &_SaveSessions[idx]

		for sidx := range saveSession.SaveFiles {
			saveFile := &saveSession.SaveFiles[sidx]

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
}

func UploadSaveFile(filePath string) error {
	err := api.SendFile("/api/agent/uploadsave", filePath)
	return err
}

func UploadSaveInfo() {
	var data = HttpRequestBody_SaveInfo{}
	data.SaveDatas = _SaveSessions

	var res interface{}
	err := api.SendPostRequest("/api/agent/saves/newinfo", data, &res)

	if err != nil {
		utils.ErrorLogger.Println(err.Error())
	}
}

func DownloadSaveFile(fileName string) error {

	fileName = strings.Replace(fileName, "\"", "", -1)
	utils.DebugLogger.Printf("Downloading Save File: %s\r\n", fileName)

	saveDir, err := GetSaveDir()
	if err != nil {
		return err
	}

	newFilePath := filepath.Join(saveDir, filepath.Clean(fileName))

	err = api.DownloadFile("/api/agent/saves/download/"+fileName, newFilePath)
	if err != nil {
		return err
	}

	utils.DebugLogger.Printf("Downloaded Save File to: %s\r\n", newFilePath)

	return nil
}
