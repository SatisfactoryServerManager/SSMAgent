package config

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"

	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
)

var (
	_config        *Config
	ConfigFileName = "SSM.json"
	ConfigFile     = ""
	SSMHomeDir     = ""
)

type SFConfig struct {
	PortOffset      int    `json:"portOffset"`
	UpdateSFOnStart bool   `json:"updateSFOnStart"`
	SFBranch        string `json:"sfbranch"`
	InstalledVer    int    `json:"installedVer"`
	AvilableVer     int    `json:"avaliableVer"`
	WorkerThreads   int    `json:"workerThreads"`
}

type Config struct {
	HomeDir string   `json:"homedir"`
	DataDir string   `json:"datadir"`
	SFDir   string   `json:"sfdir"`
	LogDir  string   `json:"logdir"`
	APIKey  string   `json:"apikey"`
	URL     string   `json:"ssmurl"`
	SF      SFConfig `json:"sf"`
}

func LoadConfigFile() {

	homedir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	SSMBaseDir, _ := filepath.Abs(path.Join(homedir, "SSM", "Agents"))

	agentName := flag.Lookup("name").Value.(flag.Getter).Get().(string)
	SSMHomeDir, _ = filepath.Abs(path.Join(SSMBaseDir, agentName))
	ConfigsDir, _ := filepath.Abs(path.Join(SSMHomeDir, "configs"))
	ConfigFile, _ = filepath.Abs(path.Join(ConfigsDir, ConfigFileName))

	utils.CreateFolder(ConfigsDir)

	newConfig := Config{}

	if !utils.CheckFileExists(ConfigFile) {
		file, err := os.Create(ConfigFile)
		if err != nil {
			log.Fatal(err)
		}
		file.Close()
	}

	f, err := os.Open(ConfigFile)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	byteValue, _ := io.ReadAll(f)

	json.Unmarshal(byteValue, &newConfig)
	_config = &newConfig

	SetDefaultValues()

	SaveConfig()
}

func SetDefaultValues() {
	if _config.HomeDir == "" {
		_config.HomeDir = SSMHomeDir
	}

	if _config.LogDir == "" {
		_config.LogDir, _ = filepath.Abs(path.Join(SSMHomeDir, "logs"))
	}

	if _config.URL == "" {
		_config.URL = flag.Lookup("url").Value.(flag.Getter).Get().(string)
		_config.SF.UpdateSFOnStart = true
	}

	_config.APIKey = flag.Lookup("apikey").Value.(flag.Getter).Get().(string)

	_config.DataDir = flag.Lookup("datadir").Value.(flag.Getter).Get().(string)
	_config.DataDir, _ = filepath.Abs(_config.DataDir)
	_config.SFDir = filepath.Join(_config.DataDir, "sfserver")

	_config.SF.PortOffset = flag.Lookup("p").Value.(flag.Getter).Get().(int)

	if _config.SF.SFBranch == "" {
		_config.SF.SFBranch = "public"
	}

	if _config.SF.WorkerThreads < 20 {
		_config.SF.WorkerThreads = 20
	}

	utils.CreateFolder(_config.DataDir)
	utils.CreateFolder(_config.SFDir)
	utils.CreateFolder(_config.LogDir)

	//fmt.Println(_config)
	setupLogger()

	log.Printf("Config File Location: %s", ConfigFile)
}

func setupLogger() {
	logFile := path.Join(_config.LogDir, "SSMAgent-combined.log")

	if utils.CheckFileExists(logFile) {
		os.Remove(logFile)
	}

	f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0777)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}

	wrt := io.MultiWriter(os.Stdout, f)

	log.SetOutput(wrt)

	log.Printf("Log File Location: %s", logFile)
}

func GetConfig() *Config {
	if _config == nil {
		LoadConfigFile()
	}

	return _config
}

func SaveConfig() {
	file, _ := json.MarshalIndent(GetConfig(), "", "    ")

	err := os.WriteFile(ConfigFile, file, 0755)

	if err != nil {
		panic(err)
	}
}
