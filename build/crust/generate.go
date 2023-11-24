package crust

import (
	"context"
	_ "embed"
	"path/filepath"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/github"
	"github.com/CoreumFoundation/crust/build/tools"
)

//go:embed workflows/ci.tmpl.yml
var ciTmpl string

// Generate regenerates everything in crust.
func Generate(ctx context.Context, deps build.DepsFunc) error {
	return github.GenerateWorkflow(filepath.Join(repoPath, ".github", "workflows", "ci.yml"), ciTmpl, struct {
		GoVersion       string
		GolangCIVersion string
	}{
		GoVersion:       tools.GoVersion,
		GolangCIVersion: tools.GolangCIVersion,
	})
}
