package state

import "github.com/SatisfactoryServerManager/SSMAgent/app/api"

var (
	Online             bool
	Installed          bool
	Running            bool
	CPU                float64
	MEM                float32
	InstalledSFVersion int64
	LatestSFVersion    int64
)

func SendAgentState() error {

	body := api.HttpRequestBody_Status{
		Online:             Online,
		Installed:          Installed,
		Running:            Running,
		CPU:                CPU,
		MEM:                MEM,
		InstalledSFVersion: InstalledSFVersion,
		LatestSFVersion:    LatestSFVersion,
	}

	var resData interface{}
	if err := api.SendPutRequest("/api/v1/agent/status", body, &resData); err != nil {
		return err
	}

	return nil
}
