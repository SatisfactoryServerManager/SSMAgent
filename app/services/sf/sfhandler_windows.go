package sf

import (
	"os/exec"
	"path/filepath"

	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	"github.com/SatisfactoryServerManager/SSMAgent/app/vars"
)

func StartSFServer() error {

	SF_PID = GetSFPID()

	if IsRunning() {
		utils.InfoLogger.Println("Server is already running")
		return nil
	}

	utils.InfoLogger.Println("Starting SF Server..")
	sfExe := filepath.Join(config.GetConfig().SFDir, vars.ExeName)

	cmd := exec.Command(sfExe, GetStartArgs()...)

	if err := cmd.Start(); err != nil {
		return err
	}

	cmd.Process.Release()

	utils.InfoLogger.Println("Started SF Server")

	utils.InfoLogger.Printf("Started process with pid: %d\r\n", cmd.Process.Pid)
	SF_PID = int32(cmd.Process.Pid)

	return nil
}
