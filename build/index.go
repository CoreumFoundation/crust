package build

import (
	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/coreum"
	"github.com/CoreumFoundation/crust/build/crust"
	"github.com/CoreumFoundation/crust/build/faucet"
	"github.com/CoreumFoundation/crust/build/tools"
)

// Commands is a definition of commands available in build system
var Commands = map[string]interface{}{
	"build":                   buildBinaries,
	"build/crust":             crust.BuildCrust,
	"build/cored":             coreum.BuildCored,
	"build/faucet":            faucet.Build,
	"build/znet":              crust.BuildZNet,
	"build/integration-tests": buildIntegrationTests,
	"images":                  buildDockerImages,
	"images/cored":            coreum.BuildCoredDockerImage,
	"images/faucet":           faucet.BuildDockerImage,
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

func tidy(deps build.DepsFunc) {
	deps(crust.Tidy, coreum.Tidy, faucet.Tidy)
}

func lint(deps build.DepsFunc) {
	deps(crust.Lint, coreum.Lint, faucet.Lint)
}

func test(deps build.DepsFunc) {
	deps(crust.Test, coreum.Test, faucet.Test)
}

func buildBinaries(deps build.DepsFunc) {
	deps(coreum.BuildCored, faucet.Build, crust.BuildZNet, buildIntegrationTests)
}

func buildIntegrationTests(deps build.DepsFunc) {
	deps(coreum.BuildIntegrationTests, faucet.BuildIntegrationTests)
}

func buildDockerImages(deps build.DepsFunc) {
	deps(coreum.BuildCoredDockerImage, faucet.BuildDockerImage)
}
