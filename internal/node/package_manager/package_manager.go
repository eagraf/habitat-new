package package_manager

type PackageSpec struct {
	DriverType         string
	RegistryURLBase    string
	RegistryPackageID  string
	RegistryPackageTag string
}

type PackageManager interface {
	IsInstalled(packageSpec *PackageSpec, version string) (bool, error)
	InstallPackage(packageSpec *PackageSpec, version string) error
	UninstallPackage(packageSpec *PackageSpec, version string) error
}
