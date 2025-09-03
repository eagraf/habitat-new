package app

type DriverType string

const (
	DriverTypeUnknown DriverType = "unknown"
	DriverTypeNoop    DriverType = "noop"
	DriverTypeDocker  DriverType = "docker"
	DriverTypeWeb     DriverType = "web"
)

func (d DriverType) String() string {
	return string(d)
}

func DriverTypeFromString(s string) DriverType {
	switch s {
	case "docker":
		return DriverTypeDocker
	case "web":
		return DriverTypeWeb
	case "noop":
		return DriverTypeNoop
	}
	return DriverTypeUnknown
}
