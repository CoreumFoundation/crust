package golang

import (
	"context"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/crust/build/git"
	"github.com/CoreumFoundation/crust/build/tools"
)

// Lint runs linters and check that git status is clean
func Lint(deps build.DepsFunc) error {
	deps(git.EnsureAllRepos, lint, lintNewLines, Tidy, git.StatusClean)
	return nil
}

func lint(ctx context.Context, deps build.DepsFunc) error {
	deps(EnsureGo, EnsureGolangCI, git.EnsureAllRepos)
	log := logger.Get(ctx)
	config := must.String(filepath.Abs("build/.golangci.yaml"))
	err := onModule(func(path string) error {
		log.Info("Running linter", zap.String("path", path))
		cmd := exec.Command(tools.Path("golangci-lint"), "run", "--config", config)
		cmd.Dir = path
		if err := libexec.Exec(ctx, cmd); err != nil {
			return errors.Wrapf(err, "linter errors found in module '%s'", path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func lintNewLines() error {
	for _, repoPath := range git.Repositories {
		err := filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return errors.WithStack(err)
			}
			if info.Mode()&0o111 != 0 {
				// skip executable files
				return nil
			}
			f, err := os.Open(path)
			if err != nil {
				return errors.WithStack(err)
			}
			defer f.Close()

			if _, err := f.Seek(-2, io.SeekEnd); err != nil {
				return errors.WithStack(err)
			}

			buf := make([]byte, 2)
			if _, err := f.Read(buf); err != nil {
				return errors.WithStack(err)
			}
			if buf[1] != '\n' {
				return errors.Errorf("no empty line at the end of file '%s'", path)
			}
			if buf[0] == '\n' {
				return errors.Errorf("many empty lines at the end of file '%s'", path)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}
