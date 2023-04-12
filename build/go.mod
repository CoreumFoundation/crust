module github.com/CoreumFoundation/crust/build

// 1.19 is used here because still not all distros deliver 1.20.
// Build tool installs newer go, but the tool itself must be built using a preexisting version.
go 1.19

replace (
	github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1
	github.com/tendermint/tendermint => github.com/informalsystems/tendermint v0.34.26
	google.golang.org/grpc => google.golang.org/grpc v1.33.2
)

require (
	github.com/CoreumFoundation/coreum v0.1.2-0.20230301133054-73acab73fba1
	github.com/CoreumFoundation/coreum-tools v0.4.0
	github.com/pkg/errors v0.9.1
	github.com/samber/lo v1.37.0
	github.com/stretchr/testify v1.8.1
	go.uber.org/zap v1.23.0
	golang.org/x/mod v0.6.0-dev.0.20220419223038-86c51ed26bb4
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	golang.org/x/exp v0.0.0-20221019170559-20944726eadf // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
