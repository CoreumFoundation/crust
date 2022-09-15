package build

import (
	"github.com/CoreumFoundation/crust/build/coreum"
	"github.com/CoreumFoundation/crust/build/faucet"
	"github.com/CoreumFoundation/crust/build/golang"
	"github.com/CoreumFoundation/crust/build/tools"
)

// Commands is a definition of commands available in build system
var Commands = map[string]interface{}{
	"build":                   buildAll,
	"build/crust":             buildCrust,
	"build/cored":             coreum.BuildCored,
	"build/faucet":            faucet.Build,
	"build/znet":              buildZNet,
	"build/integration-tests": buildAllIntegrationTests,
	"images":                  buildAllDockerImages,
	"images/cored":            coreum.BuildCoredDockerImage,
	"images/faucet":           faucet.BuildDockerImage,
	"lint":                    golang.Lint,
	"setup":                   tools.InstallAll,
	"test":                    golang.Test,
	"tidy":                    golang.Tidy,
}
