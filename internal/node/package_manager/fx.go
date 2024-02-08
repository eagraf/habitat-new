package package_manager

import "github.com/eagraf/habitat-new/internal/node/controller"

func NewAppLifecycleSubscriber(packageManager PackageManager, nodeController controller.NodeController) *AppLifecycleSubscriber {
	// TODO this should have a fx cleanup hook to cleanly handle interrupted installs
	// when the node shuts down.
	return &AppLifecycleSubscriber{
		packageManager: packageManager,
		nodeController: nodeController,
	}
}
