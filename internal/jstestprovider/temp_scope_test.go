package jstestprovider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// NEA-V0-010: observe actual native config/reporter allocation before cancelling
// readiness. This exercises the native producer without claiming browser parity.
func TestNEAV0010NativeTempScope(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	scope := filepath.Join(root, "scope")
	if err := os.Mkdir(scope, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", scope)
	repo := filepath.Join(root, "repo")
	if err := os.Mkdir(repo, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"test.js", "config.js", "package.json", "lock.json"} {
		if err := os.WriteFile(filepath.Join(repo, name), []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	observed := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entries, err := os.ReadDir(scope)
		if err != nil || len(entries) != 1 {
			http.Error(w, "not allocated", 500)
			return
		}
		scratch := filepath.Join(scope, entries[0].Name())
		for _, file := range []string{"playwright.config.cjs", "reporter.cjs"} {
			if _, err := os.Stat(filepath.Join(scratch, file)); err != nil {
				http.Error(w, "not prepared", 500)
				return
			}
		}
		config, err := os.ReadFile(filepath.Join(scratch, "playwright.config.cjs"))
		if err != nil || !strings.Contains(string(config), "module.exports.outputDir = ") || !strings.Contains(string(config), filepath.Join(scratch, "output")) || !strings.Contains(string(config), "project.outputDir = ") {
			http.Error(w, "output not scoped", 500)
			return
		}
		select {
		case observed <- scratch:
		default:
		}
		cancel()
		w.Write([]byte("ready"))
	}))
	defer server.Close()
	cfg := E2EConfig{Config: Config{Dir: repo, TestFiles: []string{filepath.Join(repo, "test.js")}, ConfigFile: filepath.Join(repo, "config.js"), PackageJSON: filepath.Join(repo, "package.json"), Lockfile: filepath.Join(repo, "lock.json"), RunnerName: "playwright", RunnerVersion: "1.60.0", Timeout: time.Second}, ExternalServer: true, AppIdentity: "fixture", ScopedTempOutput: true, ServerReadyURL: server.URL, ServerReadyLimit: time.Second}
	_, err = RunE2E(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case scratch := <-observed:
		if filepath.Dir(scratch) != scope || !strings.HasPrefix(filepath.Base(scratch), "corvint-playwright-") {
			t.Fatal(scratch)
		}
	case <-time.After(time.Second):
		t.Fatal("native allocation was not reached")
	}
	entries, err := os.ReadDir(scope)
	if err != nil || len(entries) != 0 {
		t.Fatalf("native residue: %v %v", entries, err)
	}
}

// NEA-V0-010: native output selection is provider-owned, leaving caller option
// admission and the default generated configuration unchanged.
func TestNEAV0010NativeOutputSelection(t *testing.T) {
	for _, scoped := range []bool{false, true} {
		scratch := t.TempDir()
		_, argv, _, err := externalCommand(E2EConfig{Config: Config{ConfigFile: "/repo/config.js"}, ScopedTempOutput: scoped}, scratch)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(scratch, "playwright.config.cjs"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "module.exports.outputDir = ") != scoped {
			t.Fatalf("scoped=%v: %s", scoped, data)
		}
		for _, arg := range argv {
			if strings.HasPrefix(arg, "--output") {
				t.Fatal("output escaped closed option boundary")
			}
		}
	}
	if allowedExternalOption("--output=/outside") {
		t.Fatal("caller option admission weakened")
	}
}
