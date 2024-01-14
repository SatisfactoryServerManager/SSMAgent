package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"

	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
)

var (
	_client *http.Client
)

type HttpResponseBody struct {
	Success bool        `json:"success"`
	Error   string      `json:"error"`
	Data    interface{} `json:"data"`
}

type HttpRequestBody_ActiveState struct {
	Active bool `json:"active"`
}

type HttpRequestBody_SFState struct {
	Installed bool    `json:"installed"`
	Running   bool    `json:"running"`
	CPU       float64 `json:"cpu"`
	MEM       float32 `json:"mem"`
}

type HTTPRequestBody_Config struct {
	Version     string `json:"version"`
	SFInstalled int    `json:"sfinstalledver"`
	SFAvailable int    `json:"sfavailablever"`
	IP          string `json:"ipaddress"`
}

type HttpResponseBody_Backup struct {
	Interval   int `json:"interval"`
	Keep       int `json:"keep"`
	NextBackup int `json:"nextbackup"`
}

type HttpResponseBody_Config struct {
	SFBranch             string                  `json:"sfBranch"`
	WorkerThreads        int                     `json:"workerThreads"`
	MaxPlayers           int                     `json:"maxPlayers"`
	UpdateOnStart        bool                    `json:"checkForUpdatesOnStart"`
	AutoRestart          bool                    `json:"autoRestartServer"`
	AutoPause            bool                    `json:"autoPause"`
	AutoSaveOnDisconnect bool                    `json:"autoSaveOnDisconnect"`
	Backup               HttpResponseBody_Backup `json:"backup"`
}

type APIError struct {
	ResponseCode int
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API returned code: %d", e.ResponseCode)
}

func SendGetRequest(endpoint string, returnModel interface{}) error {

	if _client == nil {
		_client = http.DefaultClient
	}

	url := config.GetConfig().URL + endpoint

	utils.DebugLogger.Printf("#### GET #### url: %s\r\n", url)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("x-ssm-key", config.GetConfig().APIKey)

	r, err := _client.Do(req)

	if err != nil {
		return err
	}

	if r.StatusCode != http.StatusOK {
		return &APIError{ResponseCode: r.StatusCode}
	}
	defer r.Body.Close()

	responseObject := HttpResponseBody{}

	json.NewDecoder(r.Body).Decode(&responseObject)

	if !responseObject.Success {
		return errors.New("api returned an error: " + responseObject.Error)
	}

	b, _ := json.Marshal(responseObject.Data)
	json.Unmarshal(b, returnModel)

	return nil
}

func SendPostRequest(endpoint string, bodyModel interface{}, returnModel interface{}) error {

	if _client == nil {
		_client = http.DefaultClient
	}

	bodyJSON, err := json.Marshal(bodyModel)

	if err != nil {
		return err
	}

	url := config.GetConfig().URL + endpoint

	utils.DebugLogger.Printf("#### POST #### url: %s, data: %s\r\n", url, bytes.NewBuffer(bodyJSON))

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(bodyJSON))
	req.Header.Set("x-ssm-key", config.GetConfig().APIKey)
	req.Header.Set("Content-Type", "application/json")

	r, err := _client.Do(req)

	if err != nil {
		return err
	}

	if r.StatusCode != http.StatusOK {
		return &APIError{ResponseCode: r.StatusCode}
	}

	defer r.Body.Close()

	responseObject := HttpResponseBody{}

	json.NewDecoder(r.Body).Decode(&responseObject)

	if !responseObject.Success {
		utils.DebugLogger.Println(r.Body)
		return errors.New("api returned an error: " + responseObject.Error)
	}

	b, _ := json.Marshal(responseObject.Data)
	err = json.Unmarshal(b, returnModel)

	if err != nil {
		return err
	}

	return nil
}

func SendFile(endpoint string, filepath string) error {
	if _client == nil {
		_client = http.DefaultClient
	}

	if !utils.CheckFileExists(filepath) {
		return errors.New("file doesn't exist")
	}

	url := config.GetConfig().URL + endpoint

	utils.DebugLogger.Printf("#### FILE #### url: %s, file: %s\r\n", url, filepath)

	// New multipart writer.
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fw, err := writer.CreateFormFile("file", filepath)
	if err != nil {
		return err
	}
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	_, err = io.Copy(fw, file)
	if err != nil {
		return err
	}
	writer.Close()
	req, err := http.NewRequest("POST", url, bytes.NewReader(body.Bytes()))

	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("x-ssm-key", config.GetConfig().APIKey)

	rsp, err := _client.Do(req)

	if err != nil {
		return err
	}

	if rsp.StatusCode != http.StatusOK {
		return errors.New("request failed with response code: " + strconv.Itoa(rsp.StatusCode))
	}

	responseObject := HttpResponseBody{}

	json.NewDecoder(rsp.Body).Decode(&responseObject)

	if !responseObject.Success {
		return errors.New("api returned an error: " + responseObject.Error)
	}

	return nil
}

func DownloadFile(endpoint string, filePath string) error {
	if _client == nil {
		_client = http.DefaultClient
	}

	url := config.GetConfig().URL + endpoint

	utils.DebugLogger.Printf("#### DOWNLOAD #### url: %s\r\n", url)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("x-ssm-key", config.GetConfig().APIKey)

	r, err := _client.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	// Create the file
	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, r.Body)
	return err
}

func DownloadNonSSMFile(url string, filePath string) error {
	if _client == nil {
		_client = http.DefaultClient
	}

	utils.DebugLogger.Printf("#### DOWNLOAD #### url: %s\r\n", url)

	req, _ := http.NewRequest("GET", url, nil)

	r, err := _client.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	// Create the file
	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, r.Body)
	return err
}

type IP struct {
	Query string
}

func GetPublicIP() (string, error) {
	req, err := http.Get("http://ip-api.com/json/")
	if err != nil {
		return "", err
	}
	defer req.Body.Close()

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return "", err
	}

	var ip IP
	err = json.Unmarshal(body, &ip)
	if err != nil {
		return "", err
	}

	return ip.Query, nil
}
