module github.com/CoreumFoundation/crust/build

// 1.18 is used here because still not all distros deliver 1.19.
// Build tool installs newer go, but the tool itself must be built using a preexisting version.
go 1.18

require (
	github.com/CoreumFoundation/coreum-tools v0.3.0
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.8.0
	github.com/samber/lo v1.37.0
	go.uber.org/zap v1.21.0
	golang.org/x/mod v0.6.0-dev.0.20211013180041-c96bc1413d57
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	golang.org/x/exp v0.0.0-20220303212507-bbda1eaf7a17 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
