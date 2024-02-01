package package_manager

func NewAppLifecycleSubscriber(packageManager PackageManager) *AppLifecycleSubscriber {
	// TODO this should have a fx cleanup hook to cleanly handle interrupted installs
	// when the node shuts down.
	return &AppLifecycleSubscriber{
		packageManager: packageManager,
	}
}
