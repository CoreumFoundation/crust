package bridgexrpl

// Ports defines ports used by bridgexrpl application.
type Ports struct {
	Metrics int `json:"metrics"`
}

// DefaultPorts are the default ports bridgexrpl listens on.
var DefaultPorts = Ports{
	Metrics: 10090,
}
