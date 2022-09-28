module github.com/CoreumFoundation/crust/build

// 1.18 is used here because still not all distros deliver 1.19.
// Build tool installs newer go, but the tool itself must be built using a preexisting version.
go 1.18

require (
	github.com/CoreumFoundation/coreum-tools v0.3.0
	github.com/pkg/errors v0.9.1
	go.uber.org/zap v1.21.0
	golang.org/x/mod v0.5.1
)

require (
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/stretchr/testify v1.8.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
