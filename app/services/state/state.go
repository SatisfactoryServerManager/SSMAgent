package state

var (
	Online             bool
	Installed          bool
	Running            bool
	CPU                float32
	MEM                float32
	InstalledSFVersion int64
	LatestSFVersion    int64
)

func MarkAgentOnline() {
	Online = true
}

func MarkAgentOffline() {
	Online = false
}
