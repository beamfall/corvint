// SPDX-License-Identifier: Apache-2.0

package testsupport

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// BuildOnPATH builds one command of this module into a fresh temporary directory and prepends
// that directory to PATH for the rest of the process, so a test package whose code reaches an
// app executable through its envelope (CMS-V0-003) exercises the current source instead of an
// installed copy. Call it from TestMain before m.Run and remove the directory afterwards with
// the returned cleanup. It reuses the caller's compiler cache, as the compiled-binary tests do
// (V1-4381).
func BuildOnPATH(importPath, name string) (cleanup func(), err error) {
	dir, err := os.MkdirTemp("", "corvint-built-"+name+"-")
	if err != nil {
		return nil, err
	}
	cleanup = func() { _ = os.RemoveAll(dir) }
	cache := os.Getenv("CORVINT_GOCACHE")
	if cache == "" {
		cache = os.Getenv("GOCACHE")
	}
	if cache == "" {
		cache = filepath.Join(os.TempDir(), "corvint-go-build-cache")
	}
	target := filepath.Join(dir, name)
	if runtime.GOOS == "windows" {
		target += ".exe"
	}
	command := exec.Command("go", "build", "-trimpath", "-o", target, importPath)
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOCACHE="+cache)
	if out, err := command.CombinedOutput(); err != nil {
		cleanup()
		return nil, fmt.Errorf("build %s: %w: %s", importPath, err, out)
	}
	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH")); err != nil {
		cleanup()
		return nil, err
	}
	return cleanup, nil
}
