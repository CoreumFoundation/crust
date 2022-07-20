package contracts

import (
	"context"
	"os"

	"os/exec"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
)

// InitConfig provides params for the init stage.
type InitConfig struct {
	// TargetDir is OS path to the target dir where contract source will be staged. It will be created if not existing.
	TargetDir string
	// TemplateRepoURL is public Git repo URL to clone smart-contract template from
	TemplateRepoURL string
	// TemplateVersion specifies the version of the template, e.g. 1.0, 1.0-minimal, 0.16
	TemplateVersion string
	// TemplateSubdir specifies a subfolder within the template repository to be used as the actual template
	TemplateSubdir string
	// ProjectName is the smart-contract name used in template scaffolding
	ProjectName string
}

func (c InitConfig) Sanitize() InitConfig {
	if len(c.ProjectName) == 0 {
		c.ProjectName = "new-project"
	}

	if len(c.TemplateVersion) == 0 {
		c.TemplateVersion = "1.0"
	}

	return c
}

// Init implements logic for "contracts init" CLI command
func Init(ctx context.Context, config InitConfig) error {
	log := logger.Get(ctx)
	config = config.Sanitize()

	if err := ensureInitTools(ctx); err != nil {
		err = errors.Wrap(err, "not all init dependencies are installed")
		return err
	}

	if err := os.MkdirAll(config.TargetDir, 0o700); err != nil {
		err = errors.Wrap(err, "failed to create contract source dir")
		return err
	}

	log.Info("Generating a new project",
		zap.String("name", config.ProjectName),
		zap.String("repoURL", config.TemplateRepoURL),
		zap.String("version", config.TemplateVersion),
		zap.String("targetDir", config.TargetDir),
	)

	cmdArgs := []string{
		"generate",
		"--init",
		"--git", config.TemplateRepoURL,
		"--branch", config.TemplateVersion,
		"--name", config.ProjectName,
	}
	if len(config.TemplateSubdir) > 0 {
		cmdArgs = append(cmdArgs, config.TemplateSubdir)
	}

	cmd := exec.Command("cargo", cmdArgs...)
	cmd.Dir = config.TargetDir

	if err := libexec.Exec(ctx, cmd); err != nil {
		err = errors.Wrap(err, "generation of the new project has failed")
		return err
	}

	return nil
}
