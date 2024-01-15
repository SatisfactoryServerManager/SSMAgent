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
	PortOffset           int    `json:"portOffset"`
	UpdateSFOnStart      bool   `json:"updateSFOnStart"`
	AutoRestart          bool   `json:"autoRestart"`
	AutoPause            bool   `json:"autoPause"`
	AutoSaveOnDisconnect bool   `json:"autoSaveOnDisconnect"`
	SFBranch             string `json:"sfbranch"`
	InstalledVer         int    `json:"installedVer"`
	AvilableVer          int    `json:"avaliableVer"`
	WorkerThreads        int    `json:"workerThreads"`
	MaxPlayers           int    `json:"maxPlayers"`
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

	_config.Version = "v1.0.49"

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

func UpdateIniFiles() error {

	EngineFilePath := filepath.Join(GetConfig().SFConfigDir, "Engine.ini")
	GameFilePath := filepath.Join(GetConfig().SFConfigDir, "Game.ini")
	ServerSettingsFilePath := filepath.Join(GetConfig().SFConfigDir, "ServerSettings.ini")

	if err := utils.CreateFolder(GetConfig().SFConfigDir); err != nil {
		return err
	}

	if !utils.CheckFileExists(EngineFilePath) {
		file, err := os.Create(EngineFilePath)
		if err != nil {
			return err
		}
		file.Close()
	}

	if !utils.CheckFileExists(GameFilePath) {
		file, err := os.Create(GameFilePath)
		if err != nil {
			return err
		}
		file.Close()
	}

	if !utils.CheckFileExists(ServerSettingsFilePath) {
		file, err := os.Create(ServerSettingsFilePath)
		if err != nil {
			return err
		}
		file.Close()
	}

	cfg, err := ini.Load(EngineFilePath)
	if err != nil {
		return err
	}

	cfg.Section("/Script/Engine.Player").Key("ConfiguredInternetSpeed").SetValue("104857600")
	cfg.Section("/Script/Engine.Player").Key("ConfiguredLanSpeed").SetValue("104857600")

	cfg.Section("/Script/Engine.Engine").Key("NetClientTicksPerSecond=").SetValue("60")

	cfg.Section("/Script/OnlineSubsystemUtils.IpNetDriver").Key("NetServerMaxTickRate").SetValue("120")
	cfg.Section("/Script/OnlineSubsystemUtils.IpNetDriver").Key("MaxNetTickRate").SetValue("400")
	cfg.Section("/Script/OnlineSubsystemUtils.IpNetDriver").Key("MaxInternetClientRate").SetValue("104857600")
	cfg.Section("/Script/OnlineSubsystemUtils.IpNetDriver").Key("MaxClientRate").SetValue("104857600")
	cfg.Section("/Script/OnlineSubsystemUtils.IpNetDriver").Key("LanServerMaxTickRate").SetValue("400")
	cfg.Section("/Script/OnlineSubsystemUtils.IpNetDriver").Key("InitialConnectTimeout").SetValue("300")
	cfg.Section("/Script/OnlineSubsystemUtils.IpNetDriver").Key("ConnectionTimeout").SetValue("300")
	cfg.Section("/Script/OnlineSubsystemUtils.IpNetDriver").Key("MaxClientRate").SetValue("104857600")
	cfg.Section("/Script/OnlineSubsystemUtils.IpNetDriver").Key("MaxInternetClientRate").SetValue("104857600")

	cfg.Section("/Script/SocketSubsystemEpic.EpicNetDriver").Key("NetServerMaxTickRate").SetValue("120")
	cfg.Section("/Script/SocketSubsystemEpic.EpicNetDriver").Key("LanServerMaxTickRate").SetValue("120")

	if err := cfg.SaveTo(GameFilePath); err != nil {
		return err
	}

	cfg, err = ini.Load(GameFilePath)
	if err != nil {
		return err
	}

	cfg.Section("/Script/Engine.GameNetworkManager").Key("TotalNetBandwidth").SetValue("104857600")
	cfg.Section("/Script/Engine.GameNetworkManager").Key("MaxDynamicBandwidth").SetValue("104857600")
	cfg.Section("/Script/Engine.GameNetworkManager").Key("MinDynamicBandwidth").SetValue("104857600")
	cfg.Section("/Script/Engine.GameSession").Key("MaxPlayers").SetValue(strconv.Itoa(GetConfig().SF.MaxPlayers))

	if err := cfg.SaveTo(GameFilePath); err != nil {
		return err
	}

	cfg, err = ini.Load(ServerSettingsFilePath)
	if err != nil {
		return err
	}

	if GetConfig().SF.AutoPause {
		cfg.Section("/Script/FactoryGame.FGServerSubsystem").Key("mAutoPause").SetValue("True")
	} else {
		cfg.Section("/Script/FactoryGame.FGServerSubsystem").Key("mAutoPause").SetValue("False")
	}

	if GetConfig().SF.AutoSaveOnDisconnect {
		cfg.Section("/Script/FactoryGame.FGServerSubsystem").Key("mAutoSaveOnDisconnect").SetValue("True")
	} else {
		cfg.Section("/Script/FactoryGame.FGServerSubsystem").Key("mAutoSaveOnDisconnect").SetValue("False")
	}

	if err := cfg.SaveTo(ServerSettingsFilePath); err != nil {
		return err
	}

	return nil
}
