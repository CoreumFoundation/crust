package cored

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/crust/exec"
	"github.com/CoreumFoundation/crust/infra/targets"
)

const CovdataDirName = "covdatafiles"

func CoverageConvert(ctx context.Context, coredHomeDir string, dstFilePath string) error {
	srcCovdataDir := filepath.Join(coredHomeDir, CovdataDirName)

	cmd := exec.Go("tool", "covdata", "textfmt", fmt.Sprintf("-i=%s", srcCovdataDir), fmt.Sprintf("-o=%s", dstFilePath))

	if err := libexec.Exec(ctx, cmd); err != nil {
		return errors.WithStack(err)
	}

	logger.Get(ctx).Info(
		"Successfully converted and stored coverage data in text format",
		zap.String("source covdata dir", srcCovdataDir),
		zap.String("destination text file", dstFilePath),
	)
	return nil
}

func (c Cored) GoCoverDir() string {
	return filepath.Join(targets.AppHomeDir, string(c.config.NetworkConfig.ChainID()), CovdataDirName)
}
