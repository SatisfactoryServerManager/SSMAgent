package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	"gopkg.in/ini.v1"
)

type GameConfigFile interface {
	SetDefaults()
}

type Engine struct {
	ConfiguredInternetSpeed int64 `inisection:"/Script/Engine.Player" inikey:"ConfiguredInternetSpeed"`
	ConfiguredLanSpeed      int64 `inisection:"/Script/Engine.Player" inikey:"ConfiguredLanSpeed"`

	NetClientTicksPerSecond int64 `inisection:"/Script/Engine.Engine" inikey:"NetClientTicksPerSecond"`

	IpNetDriver_NetServerMaxTickRate  int64 `inisection:"/Script/OnlineSubsystemUtils.IpNetDriver" inikey:"NetServerMaxTickRate"`
	IpNetDriver_MaxNetTickRate        int64 `inisection:"/Script/OnlineSubsystemUtils.IpNetDriver" inikey:"MaxNetTickRate"`
	IpNetDriver_MaxInternetClientRate int64 `inisection:"/Script/OnlineSubsystemUtils.IpNetDriver" inikey:"MaxInternetClientRate"`
	IpNetDriver_MaxClientRate         int64 `inisection:"/Script/OnlineSubsystemUtils.IpNetDriver" inikey:"MaxClientRate"`
	IpNetDriver_LanServerMaxTickRate  int64 `inisection:"/Script/OnlineSubsystemUtils.IpNetDriver" inikey:"LanServerMaxTickRate"`
	IpNetDriver_InitialConnectTimeout int64 `inisection:"/Script/OnlineSubsystemUtils.IpNetDriver" inikey:"InitialConnectTimeout"`
	IpNetDriver_ConnectionTimeout     int64 `inisection:"/Script/OnlineSubsystemUtils.IpNetDriver" inikey:"ConnectionTimeout"`

	EpicNetDriver_NetServerMaxTickRate int64 `inisection:"/Script/SocketSubsystemEpic.EpicNetDriver" inikey:"NetServerMaxTickRate"`
	EpicNetDriver_LanServerMaxTickRate int64 `inisection:"/Script/SocketSubsystemEpic.EpicNetDriver" inikey:"LanServerMaxTickRate"`
}

type Game struct {
	TotalNetBandwidth   int64 `inisection:"/Script/Engine.GameNetworkManager" inikey:"TotalNetBandwidth"`
	MaxDynamicBandwidth int64 `inisection:"/Script/Engine.GameNetworkManager" inikey:"MaxDynamicBandwidth"`
	MinDynamicBandwidth int64 `inisection:"/Script/Engine.GameNetworkManager" inikey:"MinDynamicBandwidth"`

	MaxPlayers int64 `inisection:"/Script/Engine.GameSession" inikey:"MaxPlayers"`
}

type ServerSettings struct {
	AutoPause            string `inisection:"/Script/FactoryGame.FGServerSubsystem" inikey:"mAutoPause"`
	AutoSaveOnDisconnect string `inisection:"/Script/FactoryGame.FGServerSubsystem" inikey:"mAutoSaveOnDisconnect"`
}

type Scalability struct {
	ConfiguredInternetSpeed int64 `inisection:"NetworkQuality@3" inikey:"ConfiguredInternetSpeed"`
	ConfiguredLanSpeed      int64 `inisection:"NetworkQuality@3" inikey:"ConfiguredLanSpeed"`
	TotalNetBandwidth       int64 `inisection:"NetworkQuality@3" inikey:"TotalNetBandwidth"`
	MaxDynamicBandwidth     int64 `inisection:"NetworkQuality@3" inikey:"MaxDynamicBandwidth"`
	MinDynamicBandwidth     int64 `inisection:"NetworkQuality@3" inikey:"MinDynamicBandwidth"`
	MaxInternetClientRate   int64 `inisection:"NetworkQuality@3" inikey:"MaxInternetClientRate"`
	MaxClientRate           int64 `inisection:"NetworkQuality@3" inikey:"MaxClientRate"`
}

func (obj *Engine) SetDefaults() {
	setDefaultValue(&obj.ConfiguredInternetSpeed, 104857600)
	setDefaultValue(&obj.ConfiguredLanSpeed, 104857600)
	setDefaultValue(&obj.NetClientTicksPerSecond, 60)

	setDefaultValue(&obj.IpNetDriver_NetServerMaxTickRate, 120)
	setDefaultValue(&obj.IpNetDriver_MaxNetTickRate, 400)
	setDefaultValue(&obj.IpNetDriver_MaxInternetClientRate, 104857600)
	setDefaultValue(&obj.IpNetDriver_MaxClientRate, 104857600)
	setDefaultValue(&obj.IpNetDriver_LanServerMaxTickRate, 400)
	setDefaultValue(&obj.IpNetDriver_InitialConnectTimeout, 400)
	setDefaultValue(&obj.IpNetDriver_ConnectionTimeout, 400)

	setDefaultValue(&obj.EpicNetDriver_NetServerMaxTickRate, 120)
	setDefaultValue(&obj.EpicNetDriver_LanServerMaxTickRate, 120)
}

func (obj *Game) SetDefaults() {
	setDefaultValue(&obj.TotalNetBandwidth, 104857600)
	setDefaultValue(&obj.MaxDynamicBandwidth, 104857600)
	setDefaultValue(&obj.MinDynamicBandwidth, 104857600)
	setDefaultValue(&obj.MaxPlayers, 4)

	obj.MaxPlayers = int64(GetConfig().SF.MaxPlayers)
}

func (obj *ServerSettings) SetDefaults() {
	if GetConfig().SF.AutoPause {
		obj.AutoPause = "True"
	} else {
		obj.AutoPause = "False"
	}

	if GetConfig().SF.AutoSaveOnDisconnect {
		obj.AutoSaveOnDisconnect = "True"
	} else {
		obj.AutoSaveOnDisconnect = "False"
	}
}

func (obj *Scalability) SetDefaults() {
	obj.ConfiguredInternetSpeed = 104857600
	obj.ConfiguredLanSpeed = 104857600
	obj.TotalNetBandwidth = 104857600
	obj.MaxDynamicBandwidth = 104857600
	obj.MinDynamicBandwidth = 104857600
	obj.MaxInternetClientRate = 104857600
	obj.MaxClientRate = 104857600
}

func setDefaultValue(item *int64, defaultVal int64) {
	if *item == int64(0) {
		*item = defaultVal
	}
}

func CreateGameConfigFile(obj GameConfigFile) error {
	fileName := GetName(obj) + ".ini"
	filePath := filepath.Join(GetConfig().SFConfigDir, fileName)

	if err := utils.CreateFolder(GetConfig().SFConfigDir); err != nil {
		return err
	}

	if !utils.CheckFileExists(filePath) {
		file, err := os.Create(filePath)
		if err != nil {
			return err
		}
		file.Close()
	}
	return nil
}

func LoadGameConfigFiles(files ...GameConfigFile) error {
	for _, file := range files {
		if err := LoadGameConfigFile(file); err != nil {
			return err
		}
	}
	return nil
}

func LoadGameConfigFile(obj GameConfigFile) error {
	if err := CreateGameConfigFile(obj); err != nil {
		return err
	}

	fileName := GetName(obj) + ".ini"
	filePath := filepath.Join(GetConfig().SFConfigDir, fileName)
	cfg, err := ini.Load(filePath)
	if err != nil {
		return err
	}

	t := reflect.TypeOf(obj)
	tv := reflect.ValueOf(obj)

	if t.Kind() == reflect.Ptr {
		tv = reflect.Indirect(tv)
		t = t.Elem()
	}
	// Iterate over all available fields and read the tag value
	for i := 0; i < t.NumField(); i++ {
		// Get the field, returns https://golang.org/pkg/reflect/#StructField
		field := t.Field(i)

		// Get the field tag value
		section := field.Tag.Get("inisection")
		key := field.Tag.Get("inikey")

		mVal := tv.FieldByName(field.Name)

		if field.Type.Kind() == reflect.Float32 {
			val, _ := cfg.Section(section).Key(key).Float64()
			mVal.SetFloat(val)
		} else if field.Type.Kind() == reflect.Int64 {
			val, _ := cfg.Section(section).Key(key).Int64()
			mVal.SetInt(val)
		} else if field.Type.Kind() == reflect.String {
			val := cfg.Section(section).Key(key).String()
			mVal.SetString(val)
		}

	}

	return nil
}

func SaveGameConfigFiles(files ...GameConfigFile) error {
	for _, file := range files {
		if err := SaveGameConfigFile(file); err != nil {
			return err
		}
	}
	return nil
}

func SaveGameConfigFile(obj GameConfigFile) error {
	fileName := GetName(obj) + ".ini"
	filePath := filepath.Join(GetConfig().SFConfigDir, fileName)
	cfg, err := ini.Load(filePath)
	if err != nil {
		return err
	}

	t := reflect.TypeOf(obj)
	tv := reflect.ValueOf(obj)

	if t.Kind() == reflect.Ptr {
		tv = reflect.Indirect(tv)
		t = t.Elem()
	}
	// Iterate over all available fields and read the tag value
	for i := 0; i < t.NumField(); i++ {
		// Get the field, returns https://golang.org/pkg/reflect/#StructField
		field := t.Field(i)

		// Get the field tag value
		section := field.Tag.Get("inisection")
		key := field.Tag.Get("inikey")

		mVal := tv.FieldByName(field.Name)

		if field.Type.Kind() == reflect.Float32 {
			cfg.Section(section).Key(key).SetValue(fmt.Sprintf("%f", mVal.Float()))
		} else if field.Type.Kind() == reflect.Int32 || field.Type.Kind() == reflect.Int64 {
			cfg.Section(section).Key(key).SetValue(fmt.Sprintf("%d", mVal.Int()))
		} else if field.Type.Kind() == reflect.String {
			cfg.Section(section).Key(key).SetValue(mVal.String())
		}

	}

	if err := cfg.SaveTo(filePath); err != nil {
		return err
	}

	return nil
}

type GameUserSettings struct {
	AutosaveInterval      float32 `ini:"FG.AutosaveInterval"`
	NetworkQuality        int64   `ini:"FG.NetworkQuality"`
	DisableSeasonalEvents int64   `ini:"FG.DisableSeasonalEvents"`
}

func (obj *GameUserSettings) SetDefaults() {
	obj.AutosaveInterval = GetConfig().SF.AutoSaveInterval

	if obj.AutosaveInterval == 0 {
		obj.AutosaveInterval = 300
	}

	setDefaultValue(&obj.NetworkQuality, 3)

	if GetConfig().SF.DisableSeasonalEvents {
		obj.DisableSeasonalEvents = 1
	} else {
		obj.DisableSeasonalEvents = 0
	}
}

func (obj GameUserSettings) Save() error {

	if err := CreateGameConfigFile(&obj); err != nil {
		return err
	}

	if err := obj.UpdateInts(); err != nil {
		return err
	}
	if err := obj.UpdateFloats(); err != nil {
		return err
	}

	return nil
}

func (obj GameUserSettings) UpdateInts() error {
	fileName := GetName(obj) + ".ini"
	filePath := filepath.Join(GetConfig().SFConfigDir, fileName)
	cfg, err := ini.Load(filePath)
	if err != nil {
		return err
	}

	section := cfg.Section("/Script/FactoryGame.FGGameUserSettings")

	t := reflect.TypeOf(obj)
	tv := reflect.ValueOf(obj)

	if t.Kind() == reflect.Ptr {
		tv = reflect.Indirect(tv)
		t = t.Elem()
	}

	res := "("
	addedCount := 0
	// Iterate over all available fields and read the tag value
	for i := 0; i < t.NumField(); i++ {
		// Get the field, returns https://golang.org/pkg/reflect/#StructField
		field := t.Field(i)

		// Get the field tag value
		tag := field.Tag.Get("ini")

		if field.Type.Kind() != reflect.Int64 {
			continue
		}

		mVal := tv.FieldByName(field.Name)
		if addedCount > 0 {
			res += ", "
		}
		res += fmt.Sprintf("(\"%s\", %d)", tag, mVal.Int())
		addedCount++
	}
	res += ")"

	section.Key("mIntValues").SetValue(res)

	if err := cfg.SaveTo(filePath); err != nil {
		return err
	}

	return nil
}

func (obj GameUserSettings) UpdateFloats() error {
	fileName := GetName(obj) + ".ini"
	filePath := filepath.Join(GetConfig().SFConfigDir, fileName)
	cfg, err := ini.Load(filePath)
	if err != nil {
		return err
	}

	section := cfg.Section("/Script/FactoryGame.FGGameUserSettings")

	t := reflect.TypeOf(obj)
	tv := reflect.ValueOf(obj)

	if t.Kind() == reflect.Ptr {
		tv = reflect.Indirect(tv)
		t = t.Elem()
	}

	res := "("
	addedCount := 0
	// Iterate over all available fields and read the tag value
	for i := 0; i < t.NumField(); i++ {
		// Get the field, returns https://golang.org/pkg/reflect/#StructField
		field := t.Field(i)

		// Get the field tag value
		tag := field.Tag.Get("ini")

		if field.Type.Kind() != reflect.Float32 {
			continue
		}

		mVal := tv.FieldByName(field.Name)
		if addedCount > 0 {
			res += ", "
		}
		res += fmt.Sprintf("(\"%s\", %f)", tag, mVal.Float())
		addedCount++
	}
	res += ")"

	section.Key("mFloatValues").SetValue(res)

	if err := cfg.SaveTo(filePath); err != nil {
		return err
	}

	return nil
}

// GetName Returns the collection Name
func GetName(a interface{}) string {
	t := reflect.TypeOf(a)

	if t.Kind() == reflect.String {
		return fmt.Sprintf("%v", a)
	}

	return getName(t)
}
func getName(t reflect.Type) string {
	if t.Kind() == reflect.Slice || t.Kind() == reflect.Ptr || t.Kind() == reflect.Array || t.Kind() == reflect.Map || t.Kind() == reflect.Chan {
		return getName(t.Elem())
	}

	return t.Name()
}
