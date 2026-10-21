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
// Without CORVINT_GO_LIVE_TEST_GOROOT, a harness built by another admitted
// patch revision (decision 0547) uses the official pinned toolchain from the
// module cache when it is installed there, and otherwise its own GOROOT.
func providerFixtureGoRoot() (string, error) {
	root := os.Getenv("CORVINT_GO_LIVE_TEST_GOROOT")
	if root == "" {
		root = runtime.GOROOT()
		if runtime.Version() != provider.GoVersion {
			if cached := moduleCacheToolchain(); cached != "" {
				root = cached
			}
		}
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

// moduleCacheToolchain returns the GOROOT of the official pinned toolchain
// module (golang.org/toolchain@v0.0.1-<version>.<goos>-<goarch>) when the
// harness compiler's module cache holds it, or "". Its version output is still
// checked exactly by providerFixtureGoRoot.
func moduleCacheToolchain() string {
	command := exec.Command(filepath.Join(runtime.GOROOT(), "bin", executableName(runtime.GOOS)), "env", "GOMODCACHE")
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	output, err := command.Output()
	cache := strings.TrimSpace(string(output))
	if err != nil || cache == "" {
		return ""
	}
	root := filepath.Join(cache, "golang.org", "toolchain@v0.0.1-"+provider.GoVersion+"."+runtime.GOOS+"-"+runtime.GOARCH)
	if _, err := os.Stat(filepath.Join(root, "bin", executableName(runtime.GOOS))); err != nil {
		return ""
	}
	return root
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
