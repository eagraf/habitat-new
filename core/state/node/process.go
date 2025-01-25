package node

type ProcessID string

type Driver int

const (
	DriverNoop = iota
	DriverDocker
	DriverWeb
)

func (d Driver) String() string {
	switch d {
	case DriverNoop:
		return "noop"
	case DriverDocker:
		return "docker"
	case DriverWeb:
		return "web"
	}
	return "unknown"
}

// Types related to running processes, mostly used by internal/process
type Process struct {
	ID      ProcessID `json:"id"`
	AppID   string    `json:"app_id"`
	UserID  string    `json:"user_id"`
	Created string    `json:"created"`
	Driver  Driver    `json:"driver"`
}
