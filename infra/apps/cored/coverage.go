package cored

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/crust/exec"
)

const CovdataDirName = "covdatafiles"

func CoverageDump(ctx context.Context, coredHomeDir string, dstFilePath string) error {
	srcCovdata := filepath.Join(coredHomeDir, CovdataDirName)

	fmt.Printf("src-covdata: %v, dst-covdata: %v\n", srcCovdata, dstFilePath)
	cmd := exec.Go("tool", "covdata", "textfmt", fmt.Sprintf("-i=%s", srcCovdata), fmt.Sprintf("-o=%s", dstFilePath))
	return errors.WithStack(libexec.Exec(ctx, cmd))
}
