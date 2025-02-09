module github.com/CoreumFoundation/crust

go 1.23.3

replace github.com/CoreumFoundation/crust/znet => ./znet

require (
	github.com/BurntSushi/toml v1.4.0
	github.com/CoreumFoundation/coreum-tools v0.4.1-0.20241202115740-dbc6962a4d0a
	github.com/pkg/errors v0.9.1
	github.com/samber/lo v1.49.1
	github.com/stretchr/testify v1.10.0
	go.uber.org/zap v1.27.0
	golang.org/x/mod v0.23.0
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rogpeppe/go-internal v1.12.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
