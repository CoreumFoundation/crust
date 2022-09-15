package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
)

// HeadHash returns hash of the latest commit in the repository
func HeadHash(ctx context.Context, repoPath string) (string, error) {
	buf := &bytes.Buffer{}
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	cmd.Stdout = buf
	if err := libexec.Exec(ctx, cmd); err != nil {
		return "", errors.Wrap(err, "git command failed")
	}
	return buf.String(), nil
}

// StatusClean checks that there are no uncommitted files in the repo
func StatusClean(ctx context.Context, repoPath string) (bool, error) {
	buf := &bytes.Buffer{}
	cmd := exec.Command("git", "status", "-s")
	cmd.Dir = repoPath
	cmd.Stdout = buf
	if err := libexec.Exec(ctx, cmd); err != nil {
		return false, errors.Wrap(err, "git command failed")
	}
	if buf.Len() > 0 {
		fmt.Println("git status:")
		fmt.Println(buf)
		return false, nil
	}
	return true, nil
}

// EnsureRepo ensures that repository is cloned
func EnsureRepo(ctx context.Context, repoURL string) error {
	repoName := strings.TrimSuffix(filepath.Base(repoURL), ".git")
	info, err := os.Stat("../" + repoName)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Get(ctx).Info("Cloning repository", zap.String("name", repoName), zap.String("url", repoURL))
			cmd := exec.Command("git", "clone", repoURL)
			cmd.Dir = "../"
			if err := libexec.Exec(ctx, cmd); err != nil {
				return errors.Wrapf(err, "cloning repository `%s` failed", repoURL)
			}
			return nil
		}
		return errors.WithStack(err)
	}
	if !info.IsDir() {
		return errors.Errorf("path '%s' is not a directory, while repository is expected", repoURL)
	}
	return nil
}
