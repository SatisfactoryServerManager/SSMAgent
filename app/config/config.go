package config

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	"github.com/SatisfactoryServerManager/SSMAgent/app/vars"
	"gopkg.in/ini.v1"
)

var (
	_config        *Config
	ConfigFileName = "SSM.json"
	ConfigFile     = ""
	SSMHomeDir     = ""
)

type Backup struct {
	Keep       int       `json:"keep"`
	Interval   int       `json:"interval"`
	NextBackup time.Time `json:"nextBackup"`
}

type SFConfig struct {
	PortOffset      int    `json:"portOffset"`
	UpdateSFOnStart bool   `json:"updateSFOnStart"`
	AutoRestart     bool   `json:"autoRestart"`
	SFBranch        string `json:"sfbranch"`
	InstalledVer    int    `json:"installedVer"`
	AvilableVer     int    `json:"avaliableVer"`
	WorkerThreads   int    `json:"workerThreads"`
	MaxPlayers      int    `json:"maxPlayers"`
}

type Config struct {
	HomeDir       string   `json:"homedir"`
	DataDir       string   `json:"datadir"`
	SFDir         string   `json:"sfdir"`
	LogDir        string   `json:"logdir"`
	BackupDir     string   `json:"backupdir"`
	SFConfigDir   string   `json:"sfconfigdir"`
	ModsDir       string   `json:"sfmodsdir"`
	ModConfigsDir string   `json:"sfmodconfigsdir"`
	APIKey        string   `json:"apikey"`
	URL           string   `json:"ssmurl"`
	SF            SFConfig `json:"sf"`
	Version       string   `json:"version"`
	Backup        Backup   `json:"backup"`
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

	_config.Version = "v1.0.46"

	_config.HomeDir = SSMHomeDir
	_config.LogDir, _ = filepath.Abs(path.Join(SSMHomeDir, "logs"))

	if _config.URL == "" {
		_config.SF.UpdateSFOnStart = true
	}

	_config.URL = flag.Lookup("url").Value.(flag.Getter).Get().(string)

	_config.APIKey = flag.Lookup("apikey").Value.(flag.Getter).Get().(string)

	_config.DataDir = flag.Lookup("datadir").Value.(flag.Getter).Get().(string)
	_config.DataDir, _ = filepath.Abs(_config.DataDir)
	_config.SFDir = filepath.Join(_config.DataDir, "sfserver")

	_config.BackupDir = filepath.Join(_config.DataDir, "backups")

	_config.ModsDir = filepath.Join(_config.SFDir, "FactoryGame", "Mods")
	_config.ModConfigsDir = filepath.Join(_config.SFDir, "FactoryGame", "Configs")

	utils.CreateFolder(_config.BackupDir)
	utils.CreateFolder(_config.ModsDir)

	_config.SFConfigDir = filepath.Join(
		_config.SFDir,
		"FactoryGame",
		"Saved",
		"Config",
		vars.PlatformFolder)

	utils.CreateFolder(_config.SFConfigDir)

	_config.SF.PortOffset = flag.Lookup("p").Value.(flag.Getter).Get().(int)

	if _config.SF.SFBranch == "" {
		_config.SF.SFBranch = "public"
	}

	if _config.SF.WorkerThreads < 20 {
		_config.SF.WorkerThreads = 20
	}

	if _config.Backup.Keep == 0 {
		_config.Backup.Keep = 24
		_config.Backup.Interval = 1
	}

	utils.CreateFolder(_config.DataDir)
	utils.CreateFolder(_config.SFDir)
	utils.CreateFolder(_config.LogDir)

	utils.SetupLoggers(_config.LogDir)

	utils.DebugLogger.Printf("Config File Location: %s", ConfigFile)
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

func UpdateIniFiles() {

	EngineFilePath := filepath.Join(GetConfig().SFConfigDir, "Engine.ini")
	GameFilePath := filepath.Join(GetConfig().SFConfigDir, "Game.ini")

	if !utils.CheckFileExists(EngineFilePath) {
		file, err := os.Create(EngineFilePath)
		if err != nil {
			log.Fatal(err)
		}
		file.Close()
	}

	if !utils.CheckFileExists(GameFilePath) {
		file, err := os.Create(GameFilePath)
		if err != nil {
			log.Fatal(err)
		}
		file.Close()
	}

	cfg, err := ini.Load(EngineFilePath)
	if err != nil {
		utils.WarnLogger.Printf("Fail to read file: %v\r\n", err)
		return
	}

	cfg.Section("/Script/Engine.Player").Key("ConfiguredInternetSpeed").SetValue("104857600")
	cfg.Section("/Script/Engine.Player").Key("ConfiguredLanSpeed").SetValue("104857600")

	cfg.Section("/Script/OnlineSubsystemUtils.IpNetDriver").Key("NetServerMaxTickRate").SetValue("120")
	cfg.Section("/Script/OnlineSubsystemUtils.IpNetDriver").Key("MaxNetTickRate").SetValue("400")
	cfg.Section("/Script/OnlineSubsystemUtils.IpNetDriver").Key("MaxInternetClientRate").SetValue("104857600")
	cfg.Section("/Script/OnlineSubsystemUtils.IpNetDriver").Key("MaxClientRate").SetValue("104857600")
	cfg.Section("/Script/OnlineSubsystemUtils.IpNetDriver").Key("LanServerMaxTickRate").SetValue("400")
	cfg.Section("/Script/OnlineSubsystemUtils.IpNetDriver").Key("InitialConnectTimeout").SetValue("300")
	cfg.Section("/Script/OnlineSubsystemUtils.IpNetDriver").Key("ConnectionTimeout").SetValue("300")
	cfg.Section("/Script/OnlineSubsystemUtils.IpNetDriver").Key("MaxClientRate").SetValue("104857600")
	cfg.Section("/Script/OnlineSubsystemUtils.IpNetDriver").Key("MaxInternetClientRate").SetValue("104857600")

	cfg.SaveTo(EngineFilePath)

	cfg, err = ini.Load(GameFilePath)
	if err != nil {
		utils.WarnLogger.Printf("Fail to read file: %v\r\n", err)
		return
	}

	cfg.Section("/Script/Engine.GameNetworkManager").Key("TotalNetBandwidth").SetValue("104857600")
	cfg.Section("/Script/Engine.GameNetworkManager").Key("MaxDynamicBandwidth").SetValue("104857600")
	cfg.Section("/Script/Engine.GameNetworkManager").Key("MinDynamicBandwidth").SetValue("104857600")
	cfg.Section("/Script/Engine.GameSession").Key("MaxPlayers").SetValue(strconv.Itoa(GetConfig().SF.MaxPlayers))

	cfg.SaveTo(GameFilePath)
}
