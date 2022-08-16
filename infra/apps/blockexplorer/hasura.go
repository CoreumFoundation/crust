package blockexplorer

import (
	_ "embed"
)

// HasuraMetadataTemplate contains hasura metadata template
//nolint:gofmt,goimports // Looks like gofmt linter has a bug and it produces error because of go:embed
//go:embed hasura/metadata/metadata.tmpl.json
var HasuraMetadataTemplate string
