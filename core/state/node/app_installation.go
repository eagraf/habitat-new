package node

// TODO some fields should be ignored by the REST api
type AppInstallation struct {
	ID      string `json:"id" yaml:"id"`
	UserID  string `json:"user_id" yaml:"user_id"`
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version" yaml:"version"`
	Package `yaml:",inline"`
}
