//go:build linux

package sf

import (
	"os"
	"path/filepath"
	"syscall"

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

	// The Credential fields are used to set UID, GID and attitional GIDS of the process
	// You need to run the program as  root to do this
	var Uid = uint32(9999)
	var Gid = uint32(9999)

	var cred = &syscall.Credential{Uid, Gid, []uint32{}, true}
	// the Noctty flag is used to detach the process from parent tty
	var sysproc = &syscall.SysProcAttr{Credential: cred, Noctty: true, Setpgid: true}
	var attr = os.ProcAttr{
		Dir: ".",
		Env: os.Environ(),
		Files: []*os.File{
			os.Stdin,
			os.Stdout,
			os.Stderr,
		},
		Sys: sysproc,
	}

	exeArgs := make([]string, 0)
	exeArgs = append(exeArgs, GetStartArgs()...)

	sfExe := filepath.Join(config.GetConfig().SFDir, vars.ExeName)

	process, err := os.StartProcess(sfExe, exeArgs, &attr)
	if err != nil {
		return err
	}

	utils.InfoLogger.Println("Started SF Server")

	utils.DebugLogger.Printf("Started process with pid: %d\r\n", process.Pid)
	// It is not clear from docs, but Realease actually detaches the process
	err = process.Release()
	if err != nil {
		return err
	}

	SF_PID = int32(process.Pid)

	return nil
}
