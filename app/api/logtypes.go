package api

import (
	"time"
)

type LogUpdate struct {
	Source    string `json:"source"`
	Line      string `json:"line"`
	Timestamp int64  `json:"timestamp"`
}

// SendLogLine sends a single log line update to the API
func SendLogLine(source string, line string) error {
	logUpdate := LogUpdate{
		Source:    source,
		Line:      line,
		Timestamp: time.Now().Unix(),
	}

	var response HttpResponseBody
	return SendPostRequest("/api/v1/agent/log/line", logUpdate, &response)
}
