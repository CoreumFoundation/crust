package lint

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/crust/build/git"
	"github.com/CoreumFoundation/crust/build/golang"
	"github.com/CoreumFoundation/crust/build/types"
)

const (
	repoPath = "."
)

// Lint runs linters and check that git status is clean.
func Lint(ctx context.Context, deps types.DepsFunc) error {
	if err := typosLint(ctx, deps); err != nil {
		return err
	}

	if err := golang.Lint(ctx, deps); err != nil {
		return err
	}

	isClean, dirtyContent, err := git.StatusClean(ctx)
	if err != nil {
		return err
	}
	if !isClean {
		// fmt.Println is used intentionally here, because logger escapes special characters producing unreadable output
		fmt.Println("git status:")
		fmt.Println(dirtyContent)
		return errors.New("git status is not empty")
	}
	return nil
}
