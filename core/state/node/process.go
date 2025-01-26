package node

type ProcessID string

type Driver string

const (
	DriverUnknown Driver = "unknown"
	DriverNoop    Driver = "noop"
	DriverDocker  Driver = "docker"
	DriverWeb     Driver = "web"
)

// Types related to running processes, mostly used by internal/process
type Process struct {
	ID      ProcessID `json:"id"`
	AppID   string    `json:"app_id"`
	UserID  string    `json:"user_id"`
	Created string    `json:"created"`
	Driver  Driver    `json:"driver"`
}
