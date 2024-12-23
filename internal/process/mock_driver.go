package process

import (
	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/google/uuid"
)

type entry struct {
	isStart bool
	id      string
}

type mockDriver struct {
	nextProcessID string
	log           []entry
}

var _ Driver = &mockDriver{}

func newMockDriver() *mockDriver {
	return &mockDriver{}
}

func (d *mockDriver) Type() string {
	return "test"
}

func (d *mockDriver) StartProcess(process *node.Process, app *node.AppInstallation) (string, error) {
	id := uuid.New().String()
	d.log = append(d.log, entry{isStart: true, id: id})
	return id, nil
}

func (d *mockDriver) StopProcess(extProcessID string) error {
	d.log = append(d.log, entry{isStart: false, id: extProcessID})
	return nil
}
