module github.com/CoreumFoundation/crust/build

// 1.19 is used here because still not all distros deliver 1.20.
// Build tool installs newer go, but the tool itself must be built using a preexisting version.
go 1.19

require (
	github.com/CoreumFoundation/coreum-tools v0.4.1-0.20230627094203-821c6a4eebab
	github.com/CoreumFoundation/coreum/v2 v2.0.3-0.20230810074019-b41dba895971
	github.com/pkg/errors v0.9.1
	github.com/samber/lo v1.38.1
	github.com/stretchr/testify v1.8.4
	go.uber.org/zap v1.23.0
	golang.org/x/mod v0.6.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/goleak v1.1.12 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	golang.org/x/exp v0.0.0-20230515195305-f3d0a9c9a5cc // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
