package tools

import (
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
)

// Platform defines platform to install tool on.
type Platform struct {
	OS   string
	Arch string
}

func (p Platform) String() string {
	return p.OS + "." + p.Arch
}

// Platform definitions.
var (
	PlatformLocal = Platform{OS: runtime.GOOS, Arch: runtime.GOARCH}
)

// CacheDir returns path to cache directory.
func CacheDir() string {
	return must.String(os.UserCacheDir()) + "/crust"
}

// BinariesRootPath returns the root path of cached binaries.
func BinariesRootPath(platform Platform) string {
	return filepath.Join(CacheDir(), "bin", platform.String())
}

// CopyFile copies file from src to dst.
func CopyFile(src, dst string, perm os.FileMode) error {
	fSrc, err := os.Open(src)
	if err != nil {
		return errors.WithStack(err)
	}
	defer fSrc.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return errors.WithStack(err)
	}

	fDst, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return errors.WithStack(err)
	}
	defer fDst.Close()

	if _, err = io.Copy(fDst, fSrc); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// CopyDirFiles copies all root level files from src director to dst directory and ignores all directories.
func CopyDirFiles(src, dst string, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return errors.WithStack(err)
	}

	err := filepath.WalkDir(src, func(path string, info os.DirEntry, err error) error {
		if info.IsDir() {
			return nil
		}

		return CopyFile(path, filepath.Join(dst, info.Name()), perm)
	})

	return errors.WithStack(err)
}
