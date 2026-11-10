package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

var workBoundBuild struct {
	once sync.Once
	path string
	err  error
}

// workBoundCorvint builds this command once per test process and returns the
// executable. It reuses the caller's compiler cache, because a heavy.sh run
// has its own temporary root (V1-4381).
func workBoundCorvint(t *testing.T) string {
	t.Helper()
	workBoundBuild.once.Do(func() {
		root, err := os.MkdirTemp("", "corvint-work-bound-")
		if err == nil {
			root, err = filepath.EvalSymlinks(root)
		}
		if err != nil {
			workBoundBuild.err = err
			return
		}
		cache := os.Getenv("CORVINT_GOCACHE")
		if cache == "" {
			cache = os.Getenv("GOCACHE")
		}
		if cache == "" {
			cache = filepath.Join(os.TempDir(), "corvint-go-build-cache")
		}
		workBoundBuild.path = filepath.Join(root, "corvint")
		command := exec.Command("go", "build", "-trimpath", "-ldflags", "-X main.build=9001", "-o", workBoundBuild.path, ".")
		command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOCACHE="+cache)
		if out, err := command.CombinedOutput(); err != nil {
			workBoundBuild.err = fmt.Errorf("build corvint: %w: %s", err, out)
		}
	})
	if workBoundBuild.err != nil {
		t.Fatal(workBoundBuild.err)
	}
	return workBoundBuild.path
}
