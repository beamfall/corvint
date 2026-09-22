package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/provider"
)

// The harness compiler may differ from the frozen toolchain the real parent
// must observe. Select the latter explicitly; never rewrite its version output.
func providerFixtureGoRoot() (string, error) {
	root := os.Getenv("CORVINT_GO_LIVE_TEST_GOROOT")
	if root == "" {
		root = runtime.GOROOT()
	}
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("CORVINT_GO_LIVE_TEST_GOROOT must be absolute")
	}
	command := exec.Command(filepath.Join(root, "bin", executableName(runtime.GOOS)), "version")
	command.Env = []string{"GOROOT=" + root, "GOTOOLCHAIN=local", "GOENV=off"}
	output, err := command.CombinedOutput()
	want := "go version " + provider.GoVersion + " " + runtime.GOOS + "/" + runtime.GOARCH
	if err != nil || strings.TrimSpace(string(output)) != want {
		return "", fmt.Errorf("provider fixture requires %s; %s/bin/go version = %q (%v); set CORVINT_GO_LIVE_TEST_GOROOT to the installed pinned toolchain", provider.GoVersion, root, strings.TrimSpace(string(output)), err)
	}
	return root, nil
}

func TestProviderFixtureRejectsUnpinnedToolchain(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a POSIX shell executable")
	}
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(root, "bin", executableName(runtime.GOOS))
	body := fmt.Sprintf("#!/bin/sh\nprintf 'go version go0.0.0 %s/%s\\n'\n", runtime.GOOS, runtime.GOARCH)
	if err := os.WriteFile(executable, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CORVINT_GO_LIVE_TEST_GOROOT", root)
	if _, err := providerFixtureGoRoot(); err == nil || !strings.Contains(err.Error(), "requires "+provider.GoVersion) {
		t.Fatalf("unpinned fixture toolchain: %v", err)
	}
}
