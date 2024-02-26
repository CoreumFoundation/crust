package build

import (
	"context"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/crust"
	"github.com/CoreumFoundation/crust/build/gaia"
	"github.com/CoreumFoundation/crust/build/hermes"
	"github.com/CoreumFoundation/crust/build/osmosis"
	"github.com/CoreumFoundation/crust/build/tools"
)

// Commands is a definition of commands available in build system.
var Commands = map[string]build.CommandFunc{
	"build":    crust.BuildZNet,
	"build/me": crust.BuildBuilder,
	"images": func(ctx context.Context, deps build.DepsFunc) error {
		deps(
			gaia.BuildDockerImage,
			osmosis.BuildDockerImage,
			hermes.BuildDockerImage,
		)
		return nil
	},
	"images/gaiad":  gaia.BuildDockerImage,
	"images/hermes": hermes.BuildDockerImage,
	"lint":          crust.Lint,
	// FIXME (wojciech): Remove `crust.LintCurrentDir` once coreumbridge-xrpl is migrated
	"lint/current-dir": crust.LintCurrentDir,
	"setup":            tools.InstallAll,
	"test":             crust.Test,
	"tidy":             crust.Tidy,
	"remove":           crust.Remove,
}
