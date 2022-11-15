package build

import (
	"context"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/coreum"
	"github.com/CoreumFoundation/crust/build/crust"
	"github.com/CoreumFoundation/crust/build/faucet"
	"github.com/CoreumFoundation/crust/build/gaia"
	"github.com/CoreumFoundation/crust/build/relayer"
	"github.com/CoreumFoundation/crust/build/tools"
)

// Commands is a definition of commands available in build system
var Commands = map[string]build.CommandFunc{
	"build":                   buildBinaries,
	"build/crust":             crust.BuildCrust,
	"build/cored":             coreum.BuildCored,
	"build/faucet":            faucet.Build,
	"build/znet":              crust.BuildZNet,
	"build/integration-tests": buildIntegrationTests,
	"images":                  buildDockerImages,
	"images/cored":            coreum.BuildCoredDockerImage,
	"images/faucet":           faucet.BuildDockerImage,
	"images/gaiad":            gaia.BuildDockerImage,
	"images/relayer":          relayer.BuildDockerImage,
	"lint":                    lint,
	"lint/coreum":             coreum.Lint,
	"lint/crust":              crust.Lint,
	"lint/faucet":             faucet.Lint,
	"setup":                   tools.InstallAll,
	"test":                    test,
	"test/coreum":             coreum.Test,
	"test/crust":              crust.Test,
	"test/faucet":             faucet.Test,
	"tidy":                    tidy,
	"tidy/coreum":             coreum.Tidy,
	"tidy/crust":              crust.Tidy,
	"tidy/faucet":             faucet.Tidy,
}

func tidy(ctx context.Context, deps build.DepsFunc) error {
	deps(crust.Tidy, coreum.Tidy, faucet.Tidy)
	return nil
}

func lint(ctx context.Context, deps build.DepsFunc) error {
	deps(crust.Lint, coreum.Lint, faucet.Lint)
	return nil
}

func test(ctx context.Context, deps build.DepsFunc) error {
	deps(crust.Test, coreum.Test, faucet.Test)
	return nil
}

func buildBinaries(ctx context.Context, deps build.DepsFunc) error {
	deps(coreum.BuildCored, faucet.Build, crust.BuildZNet, buildIntegrationTests)
	return nil
}

func buildIntegrationTests(ctx context.Context, deps build.DepsFunc) error {
	deps(coreum.BuildIntegrationTests, faucet.BuildIntegrationTests)
	return nil
}

func buildDockerImages(ctx context.Context, deps build.DepsFunc) error {
	deps(coreum.BuildCoredDockerImage, faucet.BuildDockerImage, gaia.BuildDockerImage, relayer.BuildDockerImage)
	return nil
}
