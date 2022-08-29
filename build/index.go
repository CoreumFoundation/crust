package build

import (
	"github.com/CoreumFoundation/crust/build/golang"
	"github.com/CoreumFoundation/crust/build/tools"
)

// Commands is a definition of commands available in build system
var Commands = map[string]interface{}{
	"build":                   buildAll,
	"build/crust":             buildCrust,
	"build/cored":             buildCored,
	"build/faucet":            buildFaucet,
	"build/znet":              buildZNet,
	"build/integration-tests": buildAllIntegrationTests,
	"lint":                    golang.Lint,
	"setup":                   tools.InstallAll,
	"test":                    golang.Test,
	"tidy":                    golang.Tidy,
}
