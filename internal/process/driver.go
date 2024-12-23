package process

import "github.com/eagraf/habitat-new/core/state/node"

type Driver interface {
	Type() string
	StartProcess(*node.Process, *node.AppInstallation) (string, error)
	StopProcess(extProcessID string) error
}
