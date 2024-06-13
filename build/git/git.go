package git

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/mod/semver"

	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
)

const repoPath = "."

// HeadHash returns hash of the latest commit in the repository.
func HeadHash(ctx context.Context) (string, error) {
	buf := &bytes.Buffer{}
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	cmd.Stdout = buf
	if err := libexec.Exec(ctx, cmd); err != nil {
		return "", errors.Wrap(err, "git command failed")
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}

// DirtyHeadHash returns hash of the latest commit in the repository, adding "-dirty" suffix
// if there are uncommitted changes.
func DirtyHeadHash(ctx context.Context) (string, error) {
	hash, err := HeadHash(ctx)
	if err != nil {
		return "", err
	}

	clean, _, err := StatusClean(ctx)
	if err != nil {
		return "", err
	}
	if !clean {
		hash += "-dirty"
	}

	return hash, nil
}

// HeadTags returns the list of tags applied to the latest commit.
func HeadTags(ctx context.Context) ([]string, error) {
	buf := &bytes.Buffer{}
	cmd := exec.Command("git", "tag", "--points-at", "HEAD")
	cmd.Dir = repoPath
	cmd.Stdout = buf
	if err := libexec.Exec(ctx, cmd); err != nil {
		return nil, errors.Wrap(err, "git command failed")
	}
	return strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n"), nil
}

// StatusClean checks that there are no uncommitted files in the repo.
func StatusClean(ctx context.Context) (bool, string, error) {
	buf := &bytes.Buffer{}
	cmd := exec.Command("git", "status", "-s")
	cmd.Dir = repoPath
	cmd.Stdout = buf
	if err := libexec.Exec(ctx, cmd); err != nil {
		return false, "", errors.Wrap(err, "git command failed")
	}
	if buf.Len() > 0 {
		return false, buf.String(), nil
	}
	return true, "", nil
}

// EnsureRepo ensures that repository is cloned.
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

// VersionFromTag returns version taken from tag present in the commit.
func VersionFromTag(ctx context.Context) (string, error) {
	tags, err := HeadTags(ctx)
	if err != nil {
		return "", err
	}

	for _, tag := range tags {
		if semver.IsValid(tag) {
			return tag, nil
		}
	}
	return "", nil
}

// CloneLocalBranch clones specific branch from local repo to another directory.
func CloneLocalBranch(ctx context.Context, dstDir, srcDir, localBranch, remoteBranch string) error {
	srcAbs, err := filepath.Abs(srcDir)
	if err != nil {
		return errors.WithStack(err)
	}

	if err := os.MkdirAll(dstDir, 0o700); err != nil {
		return err
	}

	dstAbs, err := filepath.Abs(dstDir)
	if err != nil {
		return errors.WithStack(err)
	}

	cmd1 := exec.Command("git", "fetch", "origin", remoteBranch+":"+localBranch)
	cmd1.Dir = srcDir

	cmd2 := exec.Command("git", "clone", "--single-branch", "--no-tags", "-b", localBranch, srcAbs, ".")
	cmd2.Dir = dstAbs

	return libexec.Exec(ctx, cmd1, cmd2)
}

// CloneRemoteCommit clones remote commit.
func CloneRemoteCommit(ctx context.Context, repoURL, commitID, dstDir string) error {
	if err := os.MkdirAll(dstDir, 0o700); err != nil {
		return err
	}

	branch := "crust-build/" + commitID

	// Prepare common commands
	cmdBranchFromCommitID := exec.Command("git", "branch", branch, commitID)
	cmdBranchFromCommitID.Dir = dstDir

	cmdSwitchBranch := exec.Command("git", "switch", branch)
	cmdSwitchBranch.Dir = dstDir

	// Find the name of current branch.
	buf := &bytes.Buffer{}
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = dstDir
	cmd.Stdout = buf

	err := libexec.Exec(ctx, cmd)
	switch {
	// If command returns error it means there is no git repo.
	// Clone repo and create branch from commit ID.
	case err != nil:
		cmdClone := exec.Command("git", "clone", repoURL, ".")
		cmdClone.Dir = dstDir

		return libexec.Exec(ctx, cmdClone, cmdBranchFromCommitID, cmdSwitchBranch)
	// Current branch is equal to the expected one. Do nothing.
	case strings.TrimSuffix(buf.String(), "\n") == branch:
		return nil
	}

	// Find out if expected branch is already present in the local branches.
	buf = &bytes.Buffer{}
	cmd = exec.Command("git", "branch", "--format", "%(refname:short)")
	cmd.Dir = dstDir
	cmd.Stdout = buf

	if err := libexec.Exec(ctx, cmd); err != nil {
		return err
	}

	for _, b := range strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n") {
		// Branch is already there.
		// Switch and return.
		if b == branch {
			return libexec.Exec(ctx, cmdSwitchBranch)
		}
	}

	// Branch is not available.
	// Fetch fresh content from repo and create branch from commit ID.

	cmdFetch := exec.Command("git", "fetch", "-p")
	cmdFetch.Dir = dstDir

	return libexec.Exec(ctx, cmdFetch, cmdBranchFromCommitID, cmdSwitchBranch)
}

// RollbackChanges rolls back uncommitted changes made to the specified files.
func RollbackChanges(ctx context.Context, files ...string) error {
	cmd := exec.Command("git", append([]string{"checkout", "--"}, files...)...)
	cmd.Dir = repoPath

	return libexec.Exec(ctx, cmd)
}
