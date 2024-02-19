package bridgexrpl

// Ports defines ports used by cored application.
type Ports struct {
	Prometheus int `json:"prometheus"`
}

// DefaultPorts are the default ports cored listens on.
var DefaultPorts = Ports{
	Prometheus: 10090,
}
