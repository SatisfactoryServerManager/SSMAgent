package steamcmd

import (
	"archive/zip"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

	utils.InfoLogger.Println("Depot Downloaded is Valid")
}

func DownloadSteamCMD() error {

	steamExe := filepath.Join(SteamDir, vars.DepotDownloaderExeName)

	_, err := os.Stat(steamExe)
	if !os.IsNotExist(err) {
		return nil
	}

	file, err := os.CreateTemp(os.TempDir(), "ssm_temp_*."+vars.Extension)
	if err != nil {
		return err
	}

	utils.InfoLogger.Printf("Downloading Depot Downloader to: %s\r\n", file.Name())

	resp, err := http.Get(vars.DepotDownloaderDownloadURL)
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
	steamExe := filepath.Join(SteamDir, vars.DepotDownloaderExeName)
	_, err := os.Stat(steamExe)
	return !os.IsNotExist(err)
}

func BuildScriptFile() (string, error) {

	steamExe := filepath.Join(SteamDir, vars.DepotDownloaderExeName)
	exeArgs := fmt.Sprintf(`-app "%d" -depot "%d" -beta "%s" -dir "%s"`, vars.SteamAppId, vars.DepotId, config.GetConfig().SF.SFBranch, config.GetConfig().SFDir)

	tempfile, err := os.CreateTemp(os.TempDir(), "ssm_temp_*.ps1")
	if err != nil {
		return "", err
	}

	file, err := os.OpenFile(tempfile.Name(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		return "", err
	}

	datawriter := bufio.NewWriter(file)

	datawriter.WriteString("& " + steamExe + " " + exeArgs)

	datawriter.Flush()
	file.Close()
	tempfile.Close()

	return tempfile.Name(), nil
}

func InstallSFServer() (string, error) {

	reader, writer := io.Pipe()

	cmdCtx, cmdDone := context.WithCancel(context.Background())

	output := ""

	scannerStopped := make(chan struct{})
	go func() {
		defer close(scannerStopped)

		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			m := scanner.Text()
			output += m + "\n"
			utils.SteamLogger.Println(m)
		}
	}()

	filename, _ := BuildScriptFile()
	fmt.Printf("%v\n", filename)

	cmd := exec.Command("pwsh", filename)
	cmd.Dir = SteamDir

	fmt.Printf("%v\n", cmd.String())

	cmd.Stdout = writer
	cmd.Stderr = writer

	_ = cmd.Start()

	go func() {
		_ = cmd.Wait()
		cmdDone()
		writer.Close()
	}()
	<-cmdCtx.Done()

	<-scannerStopped

	return output, nil
}

type DepotData struct {
	Data struct {
		AppId struct {
			Depots struct {
				Branches struct {
					Public struct {
						Buildid string `json:"buildid"`
					} `json:"public"`
					Experimental struct {
						Buildid string `json:"buildid"`
					} `json:"experimental"`
				} `json:"branches"`
			} `json:"depots"`
		} `json:"1690800"`
	} `json:"data"`
}

func GetLatestVersion() (int64, error) {
	client := http.DefaultClient
	requestUrl := "https://api.steamcmd.net/v1/info/" + strconv.Itoa(vars.SteamAppId)

	req, err := http.NewRequest(http.MethodGet, requestUrl, nil)
	if err != nil {
		return 0, err
	}

	res, err := client.Do(req)
	if err != nil {
		return 0, err
	}

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		utils.SteamLogger.Printf("error couldn't get latest sf version: %s", err.Error())
		os.Exit(1)
	}

	var data = DepotData{}

	err = json.Unmarshal([]byte(resBody), &data)
	if err != nil {
		return 0, err
	}

	if config.GetConfig().SF.SFBranch == "public" {
		version, err := strconv.ParseInt(data.Data.AppId.Depots.Branches.Public.Buildid, 10, 64)
		return version, err
	} else {
		version, err := strconv.ParseInt(data.Data.AppId.Depots.Branches.Experimental.Buildid, 10, 64)
		return version, err
	}
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

func ExtractArchive(file *os.File) error {
	utils.InfoLogger.Println("Extracting Depot Downloader...")
	defer os.Remove(file.Name())

	archive, err := zip.OpenReader(file.Name())
	if err != nil {
		return err
	}
	defer archive.Close()

	for _, f := range archive.File {
		filePath := filepath.Join(SteamDir, f.Name)
		utils.DebugLogger.Println("unzipping file ", filePath)

		if !strings.HasPrefix(filePath, filepath.Clean(SteamDir)+string(os.PathSeparator)) {
			utils.DebugLogger.Println("invalid file path")
			return nil
		}
		if f.FileInfo().IsDir() {
			utils.DebugLogger.Println("creating directory...")
			os.MkdirAll(filePath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			return err
		}

		dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
		if err != nil {
			return err
		}

		fileInArchive, err := f.Open()
		if err != nil {
			return err
		}

		if _, err := io.Copy(dstFile, fileInArchive); err != nil {
			return err
		}

		dstFile.Close()
		fileInArchive.Close()
	}

	err = file.Close()
	if err != nil {
		return err
	}

	err = archive.Close()
	if err != nil {
		return err
	}

	err = os.Remove(file.Name())
	if err != nil {
		return err
	}

	utils.InfoLogger.Println("Extracted Steam CMD")

	return nil
}
