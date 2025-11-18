package utils

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
)

var (
	DebugLogger *log.Logger
	InfoLogger  *log.Logger
	WarnLogger  *log.Logger
	ErrorLogger *log.Logger
	SteamLogger *log.Logger
)

func CheckError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func CreateFolder(folderPath string) error {
	if _, err := os.Stat(folderPath); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(folderPath, os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}

func CheckFileExists(filepath string) bool {
	_, err := os.Stat(filepath)
	return !os.IsNotExist(err)
}

func SetupLoggers(logDir string) {
	logFile := filepath.Join(logDir, "SSMAgent-combined.log")
	errorlogFile := filepath.Join(logDir, "SSMAgent-error.log")
	steamlogFile := filepath.Join(logDir, "SSMAgent-steam.log")

	if CheckFileExists(logFile) {
		os.Remove(logFile)
	}
	if CheckFileExists(errorlogFile) {
		os.Remove(errorlogFile)
	}
	if CheckFileExists(steamlogFile) {
		os.Remove(steamlogFile)
	}

	f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0777)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	errorf, err := os.OpenFile(errorlogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0777)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}

	steamf, err := os.OpenFile(steamlogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0777)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}

	wrt := io.MultiWriter(os.Stdout, f)
	errorwrt := io.MultiWriter(wrt, errorf)
	steamwrt := io.MultiWriter(wrt, steamf)

	log.SetOutput(wrt)

	DebugLogger = log.New(os.Stdout, "[ DEBUG ] ", log.Ldate|log.Ltime)
	InfoLogger = log.New(wrt, "[ INFO ] ", log.Ldate|log.Ltime)
	WarnLogger = log.New(wrt, "[ WARN ] ", log.Ldate|log.Ltime)
	ErrorLogger = log.New(errorwrt, "[ ERROR ] ", log.Ldate|log.Ltime)
	SteamLogger = log.New(steamwrt, "[ STEAM ] ", log.Ldate|log.Ltime)

	InfoLogger.Printf("Log File Location: %s", logFile)
}

func CopyStruct(a interface{}, b interface{}) {
	bytes, _ := json.Marshal(a)
	json.Unmarshal(bytes, b)
}
