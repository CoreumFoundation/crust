module github.com/CoreumFoundation/crust/build

// 1.21 is used here because still not all distros deliver 1.22.
// Build tool installs newer go, but the tool itself must be built using a preexisting version.
go 1.21

// Pin the x/exp dependency version because consmos-sdk breaking change is not compatible
// with cosmos-sdk v0.47.
// Details: https://github.com/cosmos/cosmos-sdk/issues/18415
replace golang.org/x/exp => golang.org/x/exp v0.0.0-20230711153332-06a737ee72cb

require (
	github.com/BurntSushi/toml v1.3.2
	github.com/CoreumFoundation/coreum-tools v0.4.1-0.20240321120602-0a9c50facc68
	github.com/pkg/errors v0.9.1
	github.com/samber/lo v1.38.1
	github.com/stretchr/testify v1.8.4
	go.uber.org/zap v1.24.0
	golang.org/x/mod v0.12.0
)

require (
	github.com/benbjohnson/clock v1.3.5 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.11.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/goleak v1.1.12 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/exp v0.0.0-20230713183714-613f0c0eb8a1 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
