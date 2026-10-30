package changee2e

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// CCE-V0-117: a qualified runner whose headless shell is missing from the browsers path is
// refused as e2e-toolchain-unavailable, naming the path, before any side runs; it no longer
// surfaces per repeat as report-identity-unknown:project-location-unknown.
func TestCCEV0117MissingHeadlessShellRefused(t *testing.T) {
	modules := filepath.Join(t.TempDir(), "node_modules")
	writePlaywrightPackage(t, modules, "1.64.0")
	browsers := t.TempDir()
	// A stale revision is present; the qualified 1248 one is not.
	stale := filepath.Join(browsers, "chromium_headless_shell-1234", "chrome-headless-shell-mac-arm64", "chrome-headless-shell")
	writeExecutable(t, stale)
	t.Setenv(envBrowsers, browsers)
	want := filepath.Join(browsers, "chromium_headless_shell-1248", "chrome-headless-shell-mac-arm64", "chrome-headless-shell")
	if got := bundledBrowserRefusal("darwin", "arm64", modules, "1.64.0"); !strings.Contains(got, want) {
		t.Fatalf("refusal = %q, want it to name %q", got, want)
	}
	for _, c := range []struct{ goos, goarch, version string }{{"linux", "amd64", "1.64.0"}, {"darwin", "amd64", "1.64.0"}, {"darwin", "arm64", "1.60.0"}} {
		if got := bundledBrowserRefusal(c.goos, c.goarch, modules, c.version); got != "" {
			t.Fatalf("%v: no bundled tuple applies, got %q", c, got)
		}
	}
	writeExecutable(t, want)
	if got := bundledBrowserRefusal("darwin", "arm64", modules, "1.64.0"); got != "" {
		t.Fatalf("installed headless shell refused: %q", got)
	}
	// A relative browsers path would resolve against each side's working directory.
	t.Setenv(envBrowsers, "relative-browsers")
	if got := bundledBrowserRefusal("darwin", "arm64", modules, "1.64.0"); !strings.Contains(got, "is relative") {
		t.Fatalf("relative browsers path: refusal = %q", got)
	}
	t.Setenv(envBrowsers, "0")
	local := filepath.Join(modules, "playwright-core", ".local-browsers", "chromium_headless_shell-1248", "chrome-headless-shell-mac-arm64", "chrome-headless-shell")
	if got := bundledBrowserRefusal("darwin", "arm64", modules, "1.64.0"); !strings.Contains(got, local) {
		t.Fatalf("PLAYWRIGHT_BROWSERS_PATH=0: refusal = %q, want it to name %q", got, local)
	}

	// Through proveToolchain on a host with the bundled tuple, the refusal names the path.
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		return
	}
	t.Setenv(envBrowsers, browsers)
	if err := os.Remove(want); err != nil {
		t.Fatal(err)
	}
	nodeDir := t.TempDir()
	writeExecutable(t, filepath.Join(nodeDir, "node"))
	writeExecutable(t, filepath.Join(nodeDir, "npx"))
	hook, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	_, err = proveToolchain(t.TempDir(), ProveOptions{NodeModules: modules, NodeBin: filepath.Join(nodeDir, "node"), HookExecutable: hook})
	var r *Refusal
	if !errors.As(err, &r) || r.Code != "e2e-toolchain-unavailable" || !strings.Contains(err.Error(), want) {
		t.Fatalf("proveToolchain = %v, want e2e-toolchain-unavailable naming %s", err, want)
	}
}

func writePlaywrightPackage(t *testing.T, modules, version string) {
	t.Helper()
	dir := filepath.Join(modules, "@playwright", "test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"version":"`+version+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeExecutable(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}
