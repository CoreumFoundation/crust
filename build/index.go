package build

import (
	"github.com/CoreumFoundation/crust/build/crust"
	"github.com/CoreumFoundation/crust/build/golang"
	"github.com/CoreumFoundation/crust/build/lint"
	"github.com/CoreumFoundation/crust/build/types"
)

// Commands is a definition of commands available in build system.
var Commands = map[string]types.Command{
	"build/me":   {Fn: crust.BuildBuilder, Description: "Builds the builder"},
	"build/znet": {Fn: crust.BuildCrustZNet, Description: "Builds znet binary"},
	"lint":       {Fn: lint.Lint, Description: "Lints code and docs"},
	"test":       {Fn: golang.Test, Description: "Runs unit tests"},
	"tidy":       {Fn: golang.Tidy, Description: "Runs go mod tidy"},
	"remove":     {Fn: crust.Remove, Description: "Removes all artifacts created by crust"},
}
