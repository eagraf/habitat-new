package process

import (
	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/constants"
)

// Currently the implementation is just no-ops because all we need is for the state machine
// to mark the process as started or stopped, in order for files from the web bundle to be
// served.
type webDriver struct{}

var _ Driver = &webDriver{}

func NewWebDriver() *webDriver {
	return &webDriver{}
}

func (d *webDriver) Type() string {
	return constants.AppDriverWeb
}

func (d *webDriver) StartProcess(process *node.Process, app *node.AppInstallation) (string, error) {
	// noop
	return "", nil
}

func (d *webDriver) StopProcess(extProcessID string) error {
	// noop
	return nil
}
