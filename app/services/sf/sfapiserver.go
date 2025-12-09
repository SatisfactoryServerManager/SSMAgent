package sf

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"strings"

	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/types"
)

var (
	_client *http.Client
	url     = "https://127.0.0.1:7777/api/v1"
)

func API_SendRequest(dataString string, token string, returnModel interface{}) error {

	if _client == nil {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		_client = &http.Client{Transport: tr}
	}

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer([]byte(dataString)))

	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	r, err := _client.Do(req)

	if err != nil {
		return err
	}

	if r.StatusCode != http.StatusOK && r.StatusCode != http.StatusNoContent {
		return &types.APIError{ResponseCode: r.StatusCode}
	}

	if r.StatusCode == http.StatusNoContent {
		return nil
	}

	defer r.Body.Close()

	responseObject := types.HttpResponseBody{}

	if err := json.NewDecoder(r.Body).Decode(&responseObject); err != nil {
		return err
	}

	b, _ := json.Marshal(responseObject.Data)
	json.Unmarshal(b, returnModel)

	return nil
}

func API_CreatePasswordlessAuth() (string, error) {

	dataStr := `{
  "function": "PasswordlessLogin",
  "data":{
    "minimumPrivilegeLevel": "InitialAdmin"
  }
}`

	type data struct {
		AuthenticationToken string `json:"authenticationToken"`
	}

	var APIData data

	if err := API_SendRequest(dataStr, "", &APIData); err != nil {
		return "", err
	}

	return APIData.AuthenticationToken, nil
}

func API_ClaimServer(AdminPassword string, authToken string) (string, error) {
	agentName := flag.Lookup("name").Value.(flag.Getter).Get().(string)

	dataStr := fmt.Sprintf(`{
  "function": "ClaimServer",
  "data":{
    "AdminPassword": "%s",
	"ServerName": "%s"
  }
}`, AdminPassword, agentName)

	type data struct {
		AuthenticationToken string `json:"authenticationToken"`
	}

	var APIData data

	if err := API_SendRequest(dataStr, authToken, &APIData); err != nil {
		return "", err
	}

	return APIData.AuthenticationToken, nil
}

func API_RunCommand(Command string, authToken string) (string, error) {

	dataStr := fmt.Sprintf(`{
  "function": "RunCommand",
  "data":{
    "Command": "%s"
  }
}`, Command)

	type data struct {
		CommandResult string `json:"commandResult"`
	}

	var APIData data

	if err := API_SendRequest(dataStr, authToken, &APIData); err != nil {
		return "", err
	}

	return APIData.CommandResult, nil
}

func API_SetClientPassword(ClientPassword string, authToken string) error {

	dataStr := fmt.Sprintf(`{
		"function": "SetClientPassword",
		"data":{
		  "Password": "%s"
		}
	  }`, ClientPassword)

	type data struct{}

	var APIData data

	if err := API_SendRequest(dataStr, authToken, &APIData); err != nil {
		return err
	}

	return nil
}

func API_ValidateToken(authToken string) error {

	dataStr := `{
  "function": "VerifyAuthenticationToken"
}`

	type data struct{}

	var APIData data

	if err := API_SendRequest(dataStr, authToken, &APIData); err != nil {
		return err
	}

	return nil
}

func ClaimServer(AdminPassword string, ClientPassword string) error {

	if !IsRunning() {
		return errors.New("error server is not running")
	}

	configApiToken := config.GetConfig().SF.APIToken

	if configApiToken != "" {
		err := API_ValidateToken(configApiToken)

		if err == nil {
			return nil
		} else {
			fmt.Println(err)
		}
	}

	token, err := API_CreatePasswordlessAuth()
	if err != nil {
		return fmt.Errorf("error creating passwordless auth with error: %s", err.Error())
	}

	newToken, err := API_ClaimServer(AdminPassword, token)
	if err != nil {
		return fmt.Errorf("error claiming server with error:%s", err.Error())
	}

	res, err := API_RunCommand("server.GenerateAPIToken", newToken)
	if err != nil {
		return fmt.Errorf("error generating api token with error: %s", err.Error())
	}

	APIToken := strings.TrimSpace(strings.Split(res, ": ")[1])

	config.GetConfig().SF.APIToken = APIToken
	config.SaveConfig()

	if ClientPassword != "" {
		if err := API_SetClientPassword(ClientPassword, APIToken); err != nil {
			return fmt.Errorf("error failed to set client password with error: %s", err.Error())
		}
	}

	return nil
}
