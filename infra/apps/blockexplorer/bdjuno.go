package blockexplorer

import (
	_ "embed"
)

// BDJunoConfigTemplate contains bdjuno configuration template
//nolint:gofmt,goimports // Looks like gofmt linter has a bug and it produces error because of go:embed
//go:embed bdjuno/config/config.tmpl.yaml
var BDJunoConfigTemplate string
