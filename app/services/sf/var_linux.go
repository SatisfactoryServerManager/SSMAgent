//go:build linux
// +build linux

package sf

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"syscall"

	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
)

var (
	ExeName    = "FactoryServer.sh"
	SubExeName = "UE4Server-Linux-Shipping"
)

func StartSFServer() error {

	SF_PID = GetSFPID()

	if IsRunning() {
		log.Println("Server is already running")
		return nil
	}

	log.Println("Starting SF Server..")

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
			nil,
			nil,
		},
		Sys: sysproc,
	}

	exeArgs := make([]string, 0)
	exeArgs = append(exeArgs, GetStartArgs()...)

	sfExe := filepath.Join(config.GetConfig().SFDir, ExeName)

	//fmt.Println(exeArgs)
	process, err := os.StartProcess(sfExe, exeArgs, &attr)
	if err != nil {
		return err
	}

	log.Println("Started SF Server")

	fmt.Printf("Started process with pid: %d\r\n", process.Pid)
	// It is not clear from docs, but Realease actually detaches the process
	err = process.Release()
	if err != nil {
		return err
	}

	SF_PID = process.Pid

	return nil
}
