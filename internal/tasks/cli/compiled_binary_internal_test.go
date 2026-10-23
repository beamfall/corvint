package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// compiledTasks is the corvint-tasks executable the tests that drive the
// compiled CLI share (V1-2366): built once per test binary from the working
// tree, with no flags, instead of once per test.
var compiledTasks struct {
	once sync.Once
	dir  string
	path string
	err  error
}

// CompiledTasksBinary returns the shared corvint-tasks executable, building
// it on first use. TestMain removes it after the run. A test that needs
// other build flags builds its own.
func CompiledTasksBinary(t *testing.T) string {
	t.Helper()
	compiledTasks.once.Do(func() {
		compiledTasks.dir, compiledTasks.err = os.MkdirTemp("", "corvint-tasks-bin-")
		if compiledTasks.err != nil {
			return
		}
		compiledTasks.path = filepath.Join(compiledTasks.dir, "corvint-tasks")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		build := exec.CommandContext(ctx, "go", "build", "-o", compiledTasks.path, "../../../cmd/corvint-tasks")
		build.WaitDelay = time.Second
		if out, err := build.CombinedOutput(); err != nil {
			compiledTasks.err = &buildError{err, out}
		}
	})
	if compiledTasks.err != nil {
		t.Fatalf("build corvint-tasks: %v", compiledTasks.err)
	}
	return compiledTasks.path
}

type buildError struct {
	err error
	out []byte
}

func (e *buildError) Error() string { return e.err.Error() + " " + string(e.out) }

// removeCompiledTasks deletes the shared executable's directory.
func removeCompiledTasks() {
	if compiledTasks.dir != "" {
		_ = os.RemoveAll(compiledTasks.dir)
	}
}
