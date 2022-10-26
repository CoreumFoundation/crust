package cored

// Ports defines ports used by cored application
type Ports struct {
	RPC        int `json:"rpc"`
	P2P        int `json:"p2p"`
	GRPC       int `json:"grpc"`
	GRPCWeb    int `json:"grpcWeb"`
	API        int `json:"api"`
	PProf      int `json:"pprof"`
	Prometheus int `json:"prometheus"`
}

// DefaultPorts are the default ports cored listens on
var DefaultPorts = Ports{
	RPC:        26657,
	P2P:        26656,
	GRPC:       9090,
	GRPCWeb:    9091,
	API:        1317,
	PProf:      6060,
	Prometheus: 26660,
}
