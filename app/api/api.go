package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/types"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
)

var (
	_client *http.Client
)

// SendGetRequest is retained for the REST connectivity check against the
// backend /api/v1/ping endpoint (kept on REST for Uptime Kuma). All other
// backend calls now go over gRPC.
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
		return &types.APIError{ResponseCode: r.StatusCode}
	}
	defer r.Body.Close()

	responseObject := types.HttpResponseBody{}

	json.NewDecoder(r.Body).Decode(&responseObject)

	if !responseObject.Success {
		return errors.New("api returned an error: " + responseObject.Error)
	}

	b, _ := json.Marshal(responseObject.Data)
	json.Unmarshal(b, returnModel)

	return nil
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

	// Without this, a 404/500 error page is happily written to disk and reported as
	// a successful download. For a mod with no catalogue hash there is then nothing
	// left to catch it, and the garbage is unzipped (or cached) forever.
	if r.StatusCode < 200 || r.StatusCode > 299 {
		return fmt.Errorf("download of %s failed with status %d", url, r.StatusCode)
	}

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

// publicIPEndpoints are plain-text services that return just the caller's
// public IP. Multiple providers are tried in order for resilience.
var publicIPEndpoints = []string{
	"https://api.ipify.org",
	"https://ifconfig.me/ip",
	"https://icanhazip.com",
	"https://ipinfo.io/ip",
}

func GetPublicIP() (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	var lastErr error
	for _, url := range publicIPEndpoints {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			lastErr = err
			continue
		}
		// Some providers reject requests without a User-Agent.
		req.Header.Set("User-Agent", "SSMAgent")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("%s: %w", url, err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("%s: %w", url, err)
			continue
		}

		ip := strings.TrimSpace(string(body))
		if resp.StatusCode == http.StatusOK && net.ParseIP(ip) != nil {
			return ip, nil
		}
		lastErr = fmt.Errorf("%s returned status %d body %q", url, resp.StatusCode, ip)
	}

	return "", fmt.Errorf("all public ip lookups failed: %w", lastErr)
}
