package node

// Types related to running processes, mostly used by internal/process

const ProcessStateStarting = "starting"
const ProcessStateRunning = "running"

type Process struct {
	ID      string `json:"id"`
	AppID   string `json:"app_id"`
	UserID  string `json:"user_id"`
	Created string `json:"created"`
	Driver  string `json:"driver"`
}

type ProcessState struct {
	*Process    `tstype:",extends,required"`
	State       string `json:"state"`
	ExtDriverID string `json:"ext_driver_id"`
}
