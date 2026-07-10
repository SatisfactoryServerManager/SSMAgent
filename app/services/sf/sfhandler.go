package sf

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/lock"
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

	if err := utils.ChmodRecursive(config.GetConfig().SFDir, 0777); err != nil {
		utils.ErrorLogger.Println(err.Error())
	}

	GetLatestedVersion()

	// The backend owns install and update. The agent reports versions as state and
	// waits to be told. This deletion is the fix for the double-install: previously
	// this ran concurrently with the workflow's installsfserver task.

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

// EnsureInstalled is idempotent: it returns success if the server is already
// installed at the available version, and never removes an existing install.
// The task queue delivers at-least-once, so this must be safe to run twice.
func EnsureInstalled() error {
	if IsRunning() {
		return errors.New("cannot install while the server is running")
	}

	if IsInstalled() && config.GetConfig().SF.InstalledVer >= config.GetConfig().SF.AvilableVer {
		utils.InfoLogger.Println("SF Server already installed at the available version, nothing to do")
		return nil
	}

	return install()
}

// Reinstall destroys the existing installation first. Only ever user-triggered.
func Reinstall() error {
	if IsRunning() {
		return errors.New("cannot reinstall while the server is running")
	}

	if IsInstalled() {
		if err := RemoveSFServer(); err != nil {
			return err
		}
		state.Installed = false
	}

	return install()
}

func install() error {
	utils.InfoLogger.Println("Installing SF Server..")

	if _, err := steamcmd.InstallSFServer(); err != nil {
		utils.ErrorLogger.Printf("Error installing SF Server %s\r\n", err.Error())
		return err
	}

	utils.InfoLogger.Println("Installed SF Server!")

	config.GetConfig().SF.InstalledVer = config.GetConfig().SF.AvilableVer
	config.SaveConfig()

	SendStates()

	return utils.ChmodRecursive(config.GetConfig().SFDir, 0777)
}

// UpdateSFServer updates an existing installation. It is an error to call it on
// a machine with no install: retrying that cannot help, so the task dies rather
// than looping.
func UpdateSFServer() error {
	if !IsInstalled() {
		return errors.New("cannot update: SF server is not installed")
	}

	// Updating over a live install rewrites files the running server holds open.
	// Skipping rather than failing is deliberate: the boot update fires on every
	// subscribe, and failing here would burn the task's whole attempt budget.
	if IsRunning() {
		utils.InfoLogger.Println("SF Server is running, skipping update")
		return nil
	}

	installedVer := config.GetConfig().SF.InstalledVer
	avaliableVer := config.GetConfig().SF.AvilableVer

	if installedVer >= avaliableVer {
		utils.InfoLogger.Println("SF Server is up to date")
		return nil
	}

	utils.InfoLogger.Printf("Found newer version of SF Server - Installed %d, Available: %d", installedVer, avaliableVer)
	return install()
}

func AutoRestart() error {

	if !lock.TryServer() {
		utils.DebugLogger.Println("AutoRestart skipped: server is locked by another operation")
		return nil
	}
	defer lock.Server.Unlock()

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

	if err := utils.ChmodRecursive(config.GetConfig().SFDir, 0777); err != nil {
		return err
	}

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

	fmt.Println(sfExe, GetStartArgs())

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
	state.CPU = float32(cpu)
	state.MEM = mem
}
