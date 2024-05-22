package cored

import (
	"context"
	"fmt"
	"path/filepath"

	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/crust/exec"
	"github.com/CoreumFoundation/crust/infra/targets"
)

const covdataDirName = "covdatafiles"

// CoverageConvert converts and stores cored coverage data in text format.
func CoverageConvert(ctx context.Context, coredHomeDir, dstFilePath string) error {
	srcCovdataDir := filepath.Join(coredHomeDir, covdataDirName)

	cmd := exec.Go("tool", "covdata", "textfmt", fmt.Sprintf("-i=%s", srcCovdataDir), fmt.Sprintf("-o=%s", dstFilePath))

	if err := libexec.Exec(ctx, cmd); err != nil {
		return err
	}

	logger.Get(ctx).Info(
		"Successfully converted and stored coverage data in text format",
		zap.String("source covdata dir", srcCovdataDir),
		zap.String("destination text file", dstFilePath),
	)
	return nil
}

// GoCoverDir returns go coverage data directory inside container.
func (c Cored) GoCoverDir() string {
	return filepath.Join(targets.AppHomeDir, string(c.config.GenesisInitConfig.ChainID), covdataDirName)
}
