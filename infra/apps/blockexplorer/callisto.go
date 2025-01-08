package blockexplorer

import (
	_ "embed"
)

// CallistoConfigTemplate contains callisto configuration template.
//
//go:embed callisto/config/config.tmpl.yaml
var CallistoConfigTemplate string
