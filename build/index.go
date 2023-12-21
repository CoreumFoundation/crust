package build

import (
	"context"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/coreum"
	"github.com/CoreumFoundation/crust/build/crust"
	"github.com/CoreumFoundation/crust/build/faucet"
	"github.com/CoreumFoundation/crust/build/gaia"
	"github.com/CoreumFoundation/crust/build/hermes"
	"github.com/CoreumFoundation/crust/build/osmosis"
	"github.com/CoreumFoundation/crust/build/tools"
)

// Commands is a definition of commands available in build system.
var Commands = map[string]build.CommandFunc{
	"build":                                  buildBinaries,
	"build/crust":                            crust.BuildBuilder,
	"build/cored":                            coreum.BuildCored,
	"build/faucet":                           faucet.Build,
	"build/znet":                             crust.BuildZNet,
	"build/integration-tests":                buildIntegrationTests,
	"build/integration-tests/coreum":         coreum.BuildAllIntegrationTests,
	"build/integration-tests/coreum/ibc":     coreum.BuildIntegrationTests(coreum.TestIBC),
	"build/integration-tests/coreum/modules": coreum.BuildIntegrationTests(coreum.TestModules),
	"build/integration-tests/coreum/upgrade": coreum.BuildIntegrationTests(coreum.TestUpgrade),
	"build/integration-tests/faucet":         faucet.BuildIntegrationTests,
	"generate":                               generate,
	"generate/coreum":                        coreum.Generate,
	"images":                                 buildDockerImages,
	"images/cored":                           coreum.BuildCoredDockerImage,
	"images/faucet":                          faucet.BuildDockerImage,
	"images/gaiad":                           gaia.BuildDockerImage,
	"images/hermes":                          hermes.BuildDockerImage,
	"lint":                                   lint,
	"lint/current-dir":                       crust.LintCurrentDir,
	"lint/coreum":                            coreum.Lint,
	"lint/crust":                             crust.Lint,
	"lint/faucet":                            faucet.Lint,
	"release":                                release,
	"release/cored":                          coreum.ReleaseCored,
	"release/faucet":                         faucet.Release,
	"release/images":                         releaseImages,
	"release/images/cored":                   coreum.ReleaseCoredImage,
	"release/images/faucet":                  faucet.ReleaseImage,
	"setup":                                  tools.InstallAll,
	"test":                                   test,
	"test/coreum":                            coreum.Test,
	"test/crust":                             crust.Test,
	"test/faucet":                            faucet.Test,
	"tidy":                                   tidy,
	"tidy/coreum":                            coreum.Tidy,
	"tidy/crust":                             crust.Tidy,
	"tidy/faucet":                            faucet.Tidy,
	"wasm":                                   coreum.CompileAllSmartContracts,
	"remove":                                 crust.Remove,
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
	deps(coreum.BuildAllIntegrationTests, faucet.BuildIntegrationTests)
	return nil
}

func buildDockerImages(ctx context.Context, deps build.DepsFunc) error {
	deps(
		coreum.BuildCoredDockerImage,
		faucet.BuildDockerImage,
		gaia.BuildDockerImage,
		osmosis.BuildDockerImage,
		hermes.BuildDockerImage,
	)

	return nil
}

func release(ctx context.Context, deps build.DepsFunc) error {
	deps(coreum.ReleaseCored, faucet.Release)
	return nil
}

func releaseImages(ctx context.Context, deps build.DepsFunc) error {
	deps(coreum.ReleaseCoredImage, faucet.ReleaseImage)
	return nil
}

func generate(ctx context.Context, deps build.DepsFunc) error {
	deps(coreum.Generate)
	return nil
}
