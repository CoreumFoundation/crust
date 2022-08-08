package build

import (
	"github.com/CoreumFoundation/crust/build/golang"
	"github.com/CoreumFoundation/crust/build/tools"
)

// Commands is a definition of commands available in build system
var Commands = map[string]interface{}{
	"build":           buildAll,
	"build/crust":     buildCrust,
	"build/cored":     buildCored,
	"build/znet":      buildZNet,
	"build/zstress":   buildZStress,
	"build/contracts": buildContracts,
	"lint":            golang.Lint,
	"setup":           tools.InstallAll,
	"test":            golang.Test,
	"tidy":            golang.Tidy,
}
