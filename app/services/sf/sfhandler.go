package sf

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/SatisfactoryServerManager/SSMAgent/app/api"
	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/steamcmd"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	"github.com/SatisfactoryServerManager/SSMAgent/app/vars"
	"github.com/buger/jsonparser"
	"github.com/shirou/gopsutil/process"
)

var (
	SF_PID     int32 = -1
	SF_SUB_PID int32 = -1
	_quit            = make(chan int)
)

func InitSFHandler() {

	log.Println("Initalising SF Handler...")

	GetLatestedVersion()

	if config.GetConfig().SF.UpdateSFOnStart {
		err := UpdateSFServer()
		utils.CheckError(err)
	}

	SF_PID = GetSFPID()
	SendStates()

	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				SF_PID = GetSFPID()
				SendStates()
			case <-_quit:
				ticker.Stop()
				return
			}
		}
	}()

	log.Println("Initalised SF Handler")
}

func ShutdownSFHandler() error {
	log.Println("Shutting down SF Handler")

	_quit <- 0
	err := ShutdownSFServer()
	if err != nil {
		return err
	}

	SF_PID = -1
	SendStates()

	log.Println("Shut down SF Handler")
	return nil
}

func RemoveSFServer() error {

	log.Println("Removing existing SF Installation..")
	err := os.RemoveAll(config.GetConfig().SFDir)

	if err != nil {
		return err
	}
	log.Println("Removed SF Installation")
	return nil
}

func InstallSFServer(force bool) error {

	if IsInstalled() && !force {
		return nil
	} else if IsInstalled() && force {
		err := RemoveSFServer()
		utils.CheckError(err)
	}

	log.Println("Installing SF Server..")
	commands := make([]string, 0)
	commands = append(commands, "force_install_dir "+config.GetConfig().SFDir)
	commands = append(commands, "app_update 1690800 -beta public")

	_, err := steamcmd.Run(commands)
	utils.CheckError(err)

	log.Println("Installed SF Server!")

	config.GetConfig().SF.InstalledVer = config.GetConfig().SF.AvilableVer
	config.SaveConfig()

	SendStates()

	return nil
}

func UpdateSFServer() error {
	installedVer := config.GetConfig().SF.InstalledVer
	avaliableVer := config.GetConfig().SF.AvilableVer
	if installedVer < avaliableVer {
		return InstallSFServer(true)
	}

	return nil
}

func ShutdownSFServer() error {

	if !IsRunning() {
		log.Println("Shutdown skipped - Server not running")
		return nil
	}

	log.Println("Shutting down SF Server...")

	newProcess, err := process.NewProcess(SF_PID)
	if err != nil {
		return err
	}

	err = newProcess.Terminate()
	SF_PID = GetSFPID()
	log.Println("SF Server is now shutdown")
	return err
}

func KillSFServer() error {

	if !IsRunning() {
		log.Println("Kill skipped - Server not running")
		return nil
	}

	log.Println("Killing SF Server...")

	newProcess, err := process.NewProcess(SF_PID)
	if err != nil {
		return err
	}

	err = newProcess.Kill()
	SF_PID = GetSFPID()
	log.Println("SF Server is now killed")
	return err
}

func GetLatestedVersion() {

	out, err := steamcmd.AppInfo()
	utils.CheckError(err)
	//fmt.Println(out)

	in := []byte(out)

	branch := config.GetConfig().SF.SFBranch
	val, err := jsonparser.GetString(in, "depots", "branches", branch, "buildid")

	utils.CheckError(err)

	config.GetConfig().SF.AvilableVer, _ = strconv.Atoi(val)
	config.SaveConfig()
}

func GetStartArgs() []string {

	port := 7777 + config.GetConfig().SF.PortOffset
	sqport := 15777 + config.GetConfig().SF.PortOffset
	bport := 15000 + config.GetConfig().SF.PortOffset

	agentName := flag.Lookup("name").Value.(flag.Getter).Get().(string)

	workerthreads := config.GetConfig().SF.WorkerThreads

	exeArgs := make([]string, 0)
	exeArgs = append(exeArgs, "?listen")
	exeArgs = append(exeArgs, "-Port="+strconv.Itoa(port))
	exeArgs = append(exeArgs, "-ServerQuertPort="+strconv.Itoa(sqport))
	exeArgs = append(exeArgs, "-BeaconPort="+strconv.Itoa(bport))
	exeArgs = append(exeArgs, "-unattended")
	exeArgs = append(exeArgs, "-MaxWorkerThreads="+strconv.Itoa(workerthreads))
	exeArgs = append(exeArgs, "-ssmagentname="+agentName)

	return exeArgs
}

func GetSFPID() int32 {

	fmt.Println("Getting process id for SF Server")
	processes, err := process.Processes()
	utils.CheckError(err)

	agentName := flag.Lookup("name").Value.(flag.Getter).Get().(string)

	for _, process := range processes {
		pid := process.Pid
		name, _ := process.Name()
		cmd, _ := process.CmdlineSlice()

		if !strings.Contains(strings.ToLower(name), "ue4server-") {
			continue
		}

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
			fmt.Printf("Successfully found SF Server PID: %s\r\n", strconv.Itoa(int(pid)))
			return pid
		}
	}

	fmt.Println("Couldn't find process id, Server not running?")

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
	bodyData := api.HttpRequestBody_SFState{}
	bodyData.Installed = IsInstalled()
	bodyData.Running = IsRunning()

	var resData interface{}

	err := api.SendPostRequest("/api/agent/state", bodyData, &resData)
	if err != nil {
		log.Println(err.Error())
	}
}