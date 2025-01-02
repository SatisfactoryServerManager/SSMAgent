package sf

import (
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/state"
	"github.com/SatisfactoryServerManager/SSMAgent/app/steamcmd"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	"github.com/SatisfactoryServerManager/SSMAgent/app/vars"
	"github.com/shirou/gopsutil/process"
)

var (
	SF_PID                  int32   = -1
	SF_SUB_PID              int32   = -1
	_quit                           = make(chan int)
	cpu                     float64 = 0.0
	mem                     float32 = 0.0
	shouldBeRunning                 = false
	attemptingToAutoRestart         = false
)

func InitSFHandler() {

	utils.InfoLogger.Println("Initalising SF Handler...")

	GetLatestedVersion()

	if config.GetConfig().SF.UpdateSFOnStart {
		err := UpdateSFServer()
		if err != nil {
			utils.ErrorLogger.Printf("Error updating SF server: %s\r\n", err.Error())
		}
	}

	SF_PID = GetSFPID()
	SendStates()

	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				SF_PID = GetSFPID()
				AutoRestart()
				SendStates()
			case <-_quit:
				ticker.Stop()
				return
			}
		}
	}()

	utils.InfoLogger.Println("Initalised SF Handler")
}

func ShutdownSFHandler() error {
	utils.InfoLogger.Println("Shutting down SF Handler")

	_quit <- 0
	err := ShutdownSFServer()
	if err != nil {
		return err
	}

	SF_PID = -1
	SendStates()

	utils.InfoLogger.Println("Shut down SF Handler")
	return nil
}

func RemoveSFServer() error {

	utils.InfoLogger.Println("Removing existing SF Installation..")
	err := os.RemoveAll(config.GetConfig().SFDir)

	if err != nil {
		return err
	}
	utils.InfoLogger.Println("Removed SF Installation")
	return nil
}

func InstallSFServer(force bool) error {

	if IsRunning() {
		return nil
	}

	if IsInstalled() && !force {
		return nil
	} else if IsInstalled() && force {
		err := RemoveSFServer()
		if err != nil {
			utils.InfoLogger.Printf("Error removing existing SF Server install %s\r\n", err.Error())
			return err
		}

		state.Installed = false
		if err := state.SendAgentState(); err != nil {
			return err
		}
	}

	utils.InfoLogger.Println("Installing SF Server..")

	_, err := steamcmd.InstallSFServer()
	if err != nil {
		utils.ErrorLogger.Printf("Error installing SF Server %s\r\n", err.Error())
		return err
	}

	utils.InfoLogger.Println("Installed SF Server!")

	config.GetConfig().SF.InstalledVer = config.GetConfig().SF.AvilableVer
	config.SaveConfig()

	SendStates()

	return nil
}

func UpdateSFServer() error {
	installedVer := config.GetConfig().SF.InstalledVer
	avaliableVer := config.GetConfig().SF.AvilableVer
	if installedVer < avaliableVer {
		utils.InfoLogger.Printf("Found newer version of SF Server - Installed %d, Available: %d", installedVer, avaliableVer)
		return InstallSFServer(true)
	}

	return nil
}

func AutoRestart() error {

	if IsRunning() {
		if attemptingToAutoRestart {
			attemptingToAutoRestart = false
		}
		return nil
	}

	if !shouldBeRunning || attemptingToAutoRestart {
		return nil
	}

	if !config.GetConfig().SF.AutoRestart {
		return nil
	}

	utils.InfoLogger.Println("Server may have crashed.. Auto restarting..")
	attemptingToAutoRestart = true

	if err := KillSFServer(); err != nil {
		attemptingToAutoRestart = false
		return err
	}

	if err := StartSFServer(); err != nil {
		attemptingToAutoRestart = false
		return err
	}

	attemptingToAutoRestart = false

	return nil
}

func StartSFServer() error {

	SF_PID = GetSFPID()

	if IsRunning() {
		utils.InfoLogger.Println("Server is already running")
		return nil
	}

	if !IsInstalled() {
		return errors.New("sf server is not installed")
	}

	utils.InfoLogger.Println("Starting SF Server..")
	sfExe := filepath.Join(config.GetConfig().SFDir, vars.ExeName)

	cmd := exec.Command(sfExe, GetStartArgs()...)

	if err := cmd.Start(); err != nil {
		return err
	}

	cmd.Process.Release()

	time.Sleep(5 * time.Second)
	SF_PID = GetSFPID()

	utils.InfoLogger.Println("Started SF Server")
	utils.InfoLogger.Printf("Started process with pid: %d\r\n", SF_PID)

	shouldBeRunning = true

	SendStates()

	return nil
}

func ShutdownSFServer() error {

	if !IsRunning() {
		utils.InfoLogger.Println("Shutdown skipped - Server not running")
		return nil
	}

	if !IsInstalled() {
		return errors.New("sf server is not installed")
	}

	utils.InfoLogger.Println("Shutting down SF Server...")

	newProcess, err := process.NewProcess(SF_PID)
	if err != nil {
		return err
	}

	err = newProcess.Terminate()
	SF_PID = GetSFPID()
	utils.InfoLogger.Println("SF Server is now shutdown")
	shouldBeRunning = false
	return err
}

func KillSFServer() error {

	if !IsRunning() {
		utils.InfoLogger.Println("Kill skipped - Server not running")
		return nil
	}

	if !IsInstalled() {
		return errors.New("sf server is not installed")
	}

	utils.InfoLogger.Println("Killing SF Server...")

	newProcess, err := process.NewProcess(SF_PID)
	if err != nil {
		return err
	}

	err = newProcess.Kill()
	SF_PID = GetSFPID()
	utils.InfoLogger.Println("SF Server is now killed")

	shouldBeRunning = false

	return err
}

func GetLatestedVersion() {

	version, err := steamcmd.GetLatestVersion()

	if err != nil {
		utils.ErrorLogger.Printf("Couldn't get latest version from steam app info with error: %s", err.Error())
		return
	}

	utils.InfoLogger.Printf("Found Latest SF Version: %d", version)

	config.GetConfig().SF.AvilableVer = version
	config.SaveConfig()
}

func GetStartArgs() []string {

	port := 7777 + config.GetConfig().SF.PortOffset

	agentName := flag.Lookup("name").Value.(flag.Getter).Get().(string)

	workerthreads := config.GetConfig().SF.WorkerThreads

	exeArgs := make([]string, 0)
	exeArgs = append(exeArgs, "?listen")
	exeArgs = append(exeArgs, "-Port="+strconv.Itoa(port))
	exeArgs = append(exeArgs, "-unattended")
	exeArgs = append(exeArgs, "-MaxWorkerThreads="+strconv.Itoa(workerthreads))
	exeArgs = append(exeArgs, "-ssmagentname="+agentName)
	exeArgs = append(exeArgs, "-multihome=0.0.0.0")

	return exeArgs
}

func GetSFPID() int32 {

	utils.DebugLogger.Println("Getting process id for SF Server")
	processes, err := process.Processes()
	if err != nil {
		utils.ErrorLogger.Printf("Error getting SF Process %s\r\n", err.Error())
		return -1
	}

	agentName := flag.Lookup("name").Value.(flag.Getter).Get().(string)

	for _, process := range processes {
		pid := process.Pid
		name, _ := process.Name()
		cmd, _ := process.CmdlineSlice()

		if !strings.Contains(strings.ToLower(name), "factoryserver-") {
			continue
		}

		cpu, _ = process.CPUPercent()
		mem, _ = process.MemoryPercent()

		processAgentName := ""
		for _, c := range cmd {

			if !strings.HasPrefix(c, "-ssmagentname") {
				continue
			}
			stringSplit := strings.Split(c, "=")
			if len(stringSplit) < 2 {
				continue
			}
			processAgentName = stringSplit[1]
		}

		if processAgentName == "" {
			continue
		}

		if processAgentName == agentName {
			utils.DebugLogger.Printf("Successfully found SF Server PID: %s\r\n", strconv.Itoa(int(pid)))
			return pid
		}
	}

	utils.DebugLogger.Println("Couldn't find process id, Server not running?")

	cpu = 0.0
	mem = 0.0

	return -1
}

func IsRunning() bool {
	return SF_PID != -1
}

func IsInstalled() bool {
	sfExe := filepath.Join(config.GetConfig().SFDir, vars.ExeName)
	_, err := os.Stat(sfExe)
	return !os.IsNotExist(err)
}

func SendStates() {

	state.Installed = IsInstalled()
	state.Running = IsRunning()
	state.CPU = cpu
	state.MEM = mem

	if err := state.SendAgentState(); err != nil {
		utils.ErrorLogger.Println(err.Error())
	}
}
