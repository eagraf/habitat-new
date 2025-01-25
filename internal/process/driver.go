package process

import (
	"context"

	"github.com/eagraf/habitat-new/core/state/node"
)

type Driver interface {
	Type() node.Driver
	StartProcess(context.Context, *node.Process, *node.AppInstallation) error
	StopProcess(context.Context, node.ProcessID) error
}

type noopDriver struct {
	driverType node.Driver
}

func NewNoopDriver(driverType node.Driver) Driver {
	return &noopDriver{driverType: driverType}
}

func (d *noopDriver) Type() node.Driver {
	return node.DriverNoop
}

func (d *noopDriver) StartProcess(context.Context, *node.Process, *node.AppInstallation) error {
	return nil
}

func (d *noopDriver) StopProcess(context.Context, node.ProcessID) error {
	return nil
}
