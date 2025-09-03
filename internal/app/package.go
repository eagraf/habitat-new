package app

type Package struct {
	Driver             DriverType             `json:"driver" yaml:"driver"`
	DriverConfig       map[string]interface{} `json:"driver_config" yaml:"driver_config"`
	RegistryURLBase    string                 `json:"registry_url_base" yaml:"registry_url_base"`
	RegistryPackageID  string                 `json:"registry_app_id" yaml:"registry_app_id"`
	RegistryPackageTag string                 `json:"registry_tag" yaml:"registry_tag"`
}
