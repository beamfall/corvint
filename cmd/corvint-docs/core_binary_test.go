// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Beamfall/corvint/internal/docsenvelope"
)

var coreBuild struct {
	once sync.Once
	path string
	err  error
}

// useCoreBinary builds the corvint command once per test process and points the corpus join
// at it for the duration of the test, so a `--corpus=FILE` read runs the real core (CMS-V0-003)
// without needing one on PATH. It reuses the caller's compiler cache (V1-4381).
func useCoreBinary(t *testing.T) {
	t.Helper()
	coreBuild.once.Do(func() {
		root, err := os.MkdirTemp("", "corvint-docs-core-")
		if err == nil {
			root, err = filepath.EvalSymlinks(root)
		}
		if err != nil {
			coreBuild.err = err
			return
		}
		cache := os.Getenv("CORVINT_GOCACHE")
		if cache == "" {
			cache = os.Getenv("GOCACHE")
		}
		if cache == "" {
			cache = filepath.Join(os.TempDir(), "corvint-go-build-cache")
		}
		coreBuild.path = filepath.Join(root, docsenvelope.CoreBinary)
		command := exec.Command("go", "build", "-trimpath", "-o", coreBuild.path, "../corvint")
		command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOCACHE="+cache)
		if out, err := command.CombinedOutput(); err != nil {
			coreBuild.err = fmt.Errorf("build corvint: %w: %s", err, out)
		}
	})
	if coreBuild.err != nil {
		t.Fatal(coreBuild.err)
	}
	previous := locateCore
	locateCore = func() (string, error) { return coreBuild.path, nil }
	t.Cleanup(func() { locateCore = previous })
}

// coreCLI runs the built corvint command on root, as the corpus join tests' native baseline.
func coreCLI(t *testing.T, root string, args ...string) (int, string, string) {
	t.Helper()
	useCoreBinary(t)
	command := exec.Command(coreBuild.path, append([]string{"--root", root}, args...)...)
	command.Stdin = strings.NewReader("")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0, stdout.String(), stderr.String()
	case errors.As(err, &exit):
		return exit.ExitCode(), stdout.String(), stderr.String()
	}
	t.Fatal(err)
	return 0, "", ""
}
