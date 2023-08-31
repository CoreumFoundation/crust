package tools

import (
	"os"
	"path/filepath"
	"runtime"

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

// DockerOS represents docker environment.
const DockerOS = "docker"

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
