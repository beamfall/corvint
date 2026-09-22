package gitstatus

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// appleGitShim is the xcrun shim macOS installs as `git`. It resolves the active
// developer directory and re-executes that directory's Git on every call, which
// costs several milliseconds per spawn on top of Git itself.
const appleGitShim = "/usr/bin/git"

var executableCache struct {
	sync.Mutex
	key   string
	value string
}

// Executable returns the path to spawn for Git. It is the executable `git`
// names on PATH, except that Apple's xcrun shim is resolved to the Git it would
// execute, so each spawn runs the same binary without paying the shim again.
// The result is memoised per PATH and DEVELOPER_DIR value; when `git` cannot be
// resolved the literal name is returned so the spawn fails as it did before.
func Executable() string {
	key := os.Getenv("PATH") + "\x00" + os.Getenv("DEVELOPER_DIR")
	executableCache.Lock()
	defer executableCache.Unlock()
	if executableCache.key == key && executableCache.value != "" {
		return executableCache.value
	}
	value := resolveExecutable()
	executableCache.key, executableCache.value = key, value
	return value
}

func resolveExecutable() string {
	resolved, err := exec.LookPath("git")
	if err != nil {
		return "git"
	}
	if runtime.GOOS != "darwin" || resolved != appleGitShim {
		return resolved
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "/usr/bin/xcrun", "--find", "git").Output()
	if err != nil {
		return resolved
	}
	target := strings.TrimSpace(string(output))
	info, err := os.Stat(target)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return resolved
	}
	return target
}
