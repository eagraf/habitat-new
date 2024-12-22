package web

import "github.com/eagraf/habitat-new/internal/package_manager"

type Driver struct {
	PackageManager package_manager.PackageManager
	ProcessDriver  *ProcessDriver
}

func NewDriver(webBundlePath string) (*Driver, error) {
	return &Driver{
		PackageManager: &AppDriver{
			webBundlePath: webBundlePath,
		},
	}, nil
}
