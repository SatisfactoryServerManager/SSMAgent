package types

import (
	"fmt"
	"time"
)

type HttpResponseBody struct {
	Success bool        `json:"success"`
	Error   string      `json:"error"`
	Data    interface{} `json:"data"`
}

type HttpRequestBody_Status struct {
	Online             bool    `json:"online"`
	Installed          bool    `json:"installed"`
	Running            bool    `json:"running"`
	CPU                float64 `json:"cpu"`
	MEM                float32 `json:"mem"`
	InstalledSFVersion int64   `json:"installedSFVersion"`
	LatestSFVersion    int64   `json:"latestSFVersion"`
}

type HttpResponseBody_SaveSync struct {
	Saves []HttpResponseBody_SaveSync_Save `json:"saves"`
}

type HttpResponseBody_SaveSync_Save struct {
	UUID            string    `json:"uuid"`
	FileName        string    `json:"fileName"`
	FilePath        string    `json:"-"`
	Size            int64     `json:"size"`
	ModTime         time.Time `json:"modTime"`
	MarkForUpload   bool      `json:"-"`
	MarkForDownload bool      `json:"-"`
}

type APIError struct {
	ResponseCode int
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API returned code: %d", e.ResponseCode)
}


type InstalledMod struct {
	ModReference    string
	ModPath         string
	ModDisplayName  string `json:"FriendlyName"`
	ModUPluginPath  string
	ModVersion      string `json:"SemVersion"`
	ShouldUninstall bool
}

type UPluginFile struct {
	SemVersion string `json:"SemVersion"`
}