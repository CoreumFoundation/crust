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
var Commands = map[string]build.Command{
	"build":    {Fn: crust.BuildZNet, Description: "Builds znet binary"},
	"build/me": {Fn: crust.BuildBuilder, Description: "Builds the builder"},
	"download": {Fn: crust.DownloadDependencies, Description: "Downloads go dependencies"},
	"images": {Fn: func(ctx context.Context, deps build.DepsFunc) error {
		deps(
			gaia.BuildDockerImage,
			hermes.BuildDockerImage,
			osmosis.BuildDockerImage,
		)
		return nil
	}, Description: "Builds docker images required by znet"},
	"images/gaiad":   {Fn: gaia.BuildDockerImage, Description: "Builds gaia docker image"},
	"images/hermes":  {Fn: hermes.BuildDockerImage, Description: "Builds hermes docker image"},
	"images/osmosis": {Fn: osmosis.BuildDockerImage, Description: "Builds osmosis docker image"},
	"lint":           {Fn: crust.Lint, Description: "Lints code"},
	"setup":          {Fn: tools.InstallAll, Description: "Installs all the required tools"},
	"test":           {Fn: crust.Test, Description: "Runs unit tests"},
	"tidy":           {Fn: crust.Tidy, Description: "Runs go mod tidy"},
	"remove":         {Fn: crust.Remove, Description: "Removes all artifacts created by crust"},
}
