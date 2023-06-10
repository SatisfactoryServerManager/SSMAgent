//go:build windows
// +build windows

package sf

import (
	"log"
	"os/exec"
	"path/filepath"

	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
)

var (
	ExeName    = "FactoryServer.exe"
	SubExeName = "UE4Server-Win64-Shipping.exe"
)

func StartSFServer() error {

	SF_PID = GetSFPID()

	if IsRunning() {
		log.Println("Server is already running")
		return nil
	}

	log.Println("Starting SF Server..")
	sfExe := filepath.Join(config.GetConfig().SFDir, ExeName)

	cmd := exec.Command(sfExe, GetStartArgs()...)

	if err := cmd.Start(); err != nil {
		return err
	}

	cmd.Process.Release()

	log.Println("Started SF Server")

	log.Printf("Started process with pid: %d\r\n", cmd.Process.Pid)
	SF_PID = int32(cmd.Process.Pid)

	return nil
}
