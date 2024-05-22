package golang

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/crust/build/git"
	"github.com/CoreumFoundation/crust/build/tools"
)

var (
	//go:embed "golangci.yaml"
	lintConfig                  []byte
	lintNewLinesSkipDirsRegexps = []string{
		`^\.`, `^vendor$`, `^target$`, `^tmp$`,
		`^.+\.db$`, // directories containing goleveldb
	}
	lintNewLinesSkipFilesRegexps = []string{`\.iml$`, `\.wasm$`, `\.png$`}
)

// Lint runs linters and check that git status is clean.
func Lint(ctx context.Context, repoPath string, deps build.DepsFunc) error {
	if err := lint(ctx, repoPath, deps); err != nil {
		return err
	}
	if err := lintNewLines(repoPath); err != nil {
		return err
	}
	if err := Tidy(ctx, repoPath, deps); err != nil {
		return err
	}

	isClean, dirtyContent, err := git.StatusClean(ctx, repoPath)
	if err != nil {
		return err
	}
	if !isClean {
		// fmt.Println is used intentionally here, because logger escapes special characters producing unreadable output
		fmt.Println("git status:")
		fmt.Println(dirtyContent)
		return errors.Errorf("git status of repository '%s' is not empty", filepath.Base(repoPath))
	}
	return nil
}

func lint(ctx context.Context, repoPath string, deps build.DepsFunc) error {
	deps(EnsureGo, EnsureGolangCI)
	log := logger.Get(ctx)
	config := lintConfigPath()

	return onModule(repoPath, func(path string) error {
		goCodePresent, err := containsGoCode(path)
		if err != nil {
			return err
		}
		if !goCodePresent {
			log.Info("No code to lint", zap.String("path", path))
			return nil
		}

		log.Info("Running linter", zap.String("path", path))
		cmd := exec.Command(tools.Path("bin/golangci-lint", tools.TargetPlatformLocal), "run", "--config", config)
		cmd.Dir = path
		if err := libexec.Exec(ctx, cmd); err != nil {
			return errors.Wrapf(err, "linter errors found in module '%s'", path)
		}
		return nil
	})
}

func lintNewLines(repoPath string) error {
	skipDirsRegexps, err := parseRegexps(lintNewLinesSkipDirsRegexps)
	if err != nil {
		return err
	}

	skipFilesRegexps, err := parseRegexps(lintNewLinesSkipFilesRegexps)
	if err != nil {
		return err
	}

	return filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			for _, reg := range skipDirsRegexps {
				if reg.MatchString(d.Name()) {
					return filepath.SkipDir
				}
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

		for _, reg := range skipFilesRegexps {
			if reg.MatchString(info.Name()) {
				return nil
			}
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
}

func parseRegexps(strRegexps []string) ([]*regexp.Regexp, error) {
	compiledRegexps := make([]*regexp.Regexp, 0, len(strRegexps))
	for _, strReg := range strRegexps {
		r, err := regexp.Compile(strReg)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid regexp '%s'", strReg)
		}
		compiledRegexps = append(compiledRegexps, r)
	}

	return compiledRegexps, nil
}

func lintConfigPath() string {
	return filepath.Join(tools.VersionedRootPath(tools.TargetPlatformLocal), "golangci.yaml")
}

func storeLintConfig(_ context.Context, _ build.DepsFunc) error {
	return errors.WithStack(os.WriteFile(lintConfigPath(), lintConfig, 0o600))
}
