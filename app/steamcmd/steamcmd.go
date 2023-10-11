package steamcmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	"github.com/SatisfactoryServerManager/SSMAgent/app/vars"
)

var (
	SteamDir = ""
)

func InitSteamCMD() {

	SteamDir = filepath.Join(config.GetConfig().DataDir, "steamcmd")
	err := utils.CreateFolder(SteamDir)
	if err != nil {
		utils.ErrorLogger.Printf("Error creating steam cmd dir %s\r\n", err.Error())
		return
	}

	err = DownloadSteamCMD()
	if err != nil {
		utils.ErrorLogger.Printf("Error downloading steam cmd %s\r\n", err.Error())
		return
	}

	utils.InfoLogger.Println("Running Steam CMD Validation..")
	commands := make([]string, 0)
	_, err = Run(commands)
	if err != nil {
		utils.ErrorLogger.Printf("Error running steam validation %s\r\n", err.Error())
		return
	}

	utils.InfoLogger.Println("Steam CMD is Valid")
}

func DownloadSteamCMD() error {

	steamExe := filepath.Join(SteamDir, vars.SteamExeName)

	_, err := os.Stat(steamExe)
	if !os.IsNotExist(err) {
		return nil
	}

	file, err := os.CreateTemp(os.TempDir(), "ssm_temp_*."+vars.Extension)
	if err != nil {
		return err
	}

	utils.InfoLogger.Printf("Downloading Steam CMD to: %s\r\n", file.Name())

	resp, err := http.Get(vars.DownloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(file.Name())
	if err != nil {
		return err
	}

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	err = out.Close()
	if err != nil {
		return err
	}

	return ExtractArchive(file)
}

func IsInstalled() bool {
	steamExe := filepath.Join(SteamDir, vars.SteamExeName)
	_, err := os.Stat(steamExe)
	return !os.IsNotExist(err)
}

func BuildScriptFile(commands []string) (string, error) {

	allCommands := make([]string, 0)

	allCommands = append(allCommands, "@ShutdownOnFailedCommand 1")
	allCommands = append(allCommands, "@NoPromptForPassword 1")
	allCommands = append(allCommands, "login anonymous")
	allCommands = append(allCommands, commands...)
	allCommands = append(allCommands, "quit")

	tempfile, err := os.CreateTemp(os.TempDir(), "ssm_temp_*.txt")
	if err != nil {
		return "", err
	}

	file, err := os.OpenFile(tempfile.Name(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		return "", err
	}

	datawriter := bufio.NewWriter(file)

	for _, data := range allCommands {
		_, _ = datawriter.WriteString(data + "\n")
	}

	datawriter.Flush()
	file.Close()

	return tempfile.Name(), nil
}

func Run(commands []string) (string, error) {
	steamExe := filepath.Join(SteamDir, vars.SteamExeName)

	tempFilePath, err := BuildScriptFile(commands)

	if err != nil {
		return "", err
	}

	exeArgs := make([]string, 0)
	exeArgs = append(exeArgs, steamExe)
	exeArgs = append(exeArgs, "+runscript "+tempFilePath)

	cmd := exec.Command(steamExe, exeArgs...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		utils.ErrorLogger.Printf("error steamcmd stdpipe with error: %s ", err.Error())
		return "", err
	}

	err = cmd.Start()
	utils.DebugLogger.Println("The steamcmd command is running")

	if err != nil {
		if err.Error() != "exit status 7" {
			utils.ErrorLogger.Printf("error steamcmd start with error: %s", err.Error())
			return "", err
		}

	}

	// print the output of the subprocess

	var output string

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		m := scanner.Text()
		output += m + "\n"
		utils.SteamLogger.Println(m)
	}
	cmd.Wait()

	return output, nil
}

func AppInfo() (string, error) {
	out, err := Run([]string{"app_info_update 1", "app_info_print 1690800"})
	if err != nil {
		return "", err
	}
	return appInfoFormat(out)
}

func appInfoFormat(appInfo string) (string, error) {
	// Remove everything before the first opening curly
	firstIndex := strings.LastIndex(reverse(appInfo), "{")
	if firstIndex > 0 {
		appInfo = reverse(trimLength(reverse(appInfo), firstIndex+1))
	}

	// Remove everything after the last closing curly brace
	lastIndex := strings.LastIndex(appInfo, "}")
	if lastIndex > 0 {
		appInfo = trimLength(appInfo, lastIndex+1)
	}

	// Get the app info part of the incoming data
	result := appInfo

	// Remove tabs
	result = strings.Replace(result, "\t", "", -1)
	result = strings.Replace(result, "\r", "", -1)

	// // Add missing semicolons
	result = strings.Replace(result, "\"\n{", "\":\n{", -1)
	result = strings.Replace(result, "\"\"", "\":\"", -1)

	// // Add missing commas
	result = strings.Replace(result, "}\n\"", "},\n\"", -1)
	result = strings.Replace(result, "\"\n\"", "\",\n\"", -1)

	// // Remove newlines last
	result = strings.Replace(result, "\n", "", -1)
	result = strings.Replace(result, "\r", "", -1)

	// Validate that we have a proper JSON string
	if !isJSON(result) {
		return "", errors.New("not valid json")
	}

	// Convert to pretty printed JSON
	in := []byte(result)
	var raw map[string]interface{}
	json.Unmarshal(in, &raw)

	// Return the parsed result
	return string(result), nil
}

func isJSON(s string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}

func trimLength(s string, i int) string {
	runes := []rune(s)
	if len(runes) > i {
		return string(runes[:i])
	}
	return s
}

func reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < len(r)/2; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}
