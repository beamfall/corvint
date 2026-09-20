//go:build darwin || linux

package jstestprovider_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/mcp/testvaliditybridge"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testvalidity"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

// PWP-V0-001..007: explicit qualification requires real installed Playwright
// and browser binaries. Ordinary Go gates do not silently download dependencies.
func TestQualifiedPlaywrightLive(t *testing.T) {
	modules := os.Getenv("CORVINT_PLAYWRIGHT_MODULES")
	if modules == "" {
		t.Skip("NOT_RUN: set CORVINT_PLAYWRIGHT_MODULES to installed node_modules for live qualification")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"playwright.config.cjs", "external.spec.cjs", "override.spec.cjs", "dynamic.spec.cjs", "setup.cjs", "teardown.cjs"} {
		data, err := os.ReadFile(filepath.Join("testdata", "external", name))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(root, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err = os.Symlink(modules, filepath.Join(root, "node_modules")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(modules, "@playwright/test/package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err = json.Unmarshal(data, &pkg); err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{})
	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cancel-ready" {
			once.Do(func() { close(ready) })
		}
		_, _ = w.Write([]byte("external fixture"))
	}))
	defer server.Close()
	t.Setenv("CORVINT_FIXTURE_URL", server.URL)
	t.Setenv("CORVINT_FIXTURE_MARKER", filepath.Join(root, "lifecycle"))
	cfg := jstestprovider.E2EConfig{Config: jstestprovider.Config{Dir: root, ConfigFile: filepath.Join(root, "playwright.config.cjs"), TestFiles: []string{filepath.Join(root, "external.spec.cjs"), filepath.Join(root, "override.spec.cjs")}, RunnerName: "playwright", RunnerVersion: pkg.Version, DeclaredEnvKeys: []string{"CORVINT_FIXTURE_URL", "CORVINT_FIXTURE_MARKER"}, Timeout: 45 * time.Second}, ExternalServer: true, AppIdentity: "fixture-v1", ServerReadyURL: server.URL, TestArgv: []string{"external.spec.cjs", "--project=chromium", "--project=react", "--grep-invert=cancellation"}}
	r, err := jstestprovider.RunE2E(context.Background(), cfg)
	if err != nil || r.Infrastructure != nil {
		t.Fatalf("run error %v; infrastructure %+v", err, r.Infrastructure)
	}
	if len(r.Tests) != 6 {
		t.Fatalf("want six real outcomes, got %+v", r.Tests)
	}
	states := map[jstestprovider.ExecutionState]int{}
	ids := map[string]bool{}
	t.Run("PWP-V0-003 distinct-project-identities", func(t *testing.T) {
		for _, test := range r.Tests {
			states[test.State]++
			if test.ID == "" || ids[test.ID] || test.Project == nil || test.Project.ConfigDigest == "" {
				t.Fatalf("unattributable test %+v", test)
			}
			ids[test.ID] = true
		}
	})
	t.Run("PWP-V0-004 preserve-real-attempt-states", func(t *testing.T) {
		for _, state := range []jstestprovider.ExecutionState{jstestprovider.StatePassed, jstestprovider.StateFailed, jstestprovider.StateTimedOut} {
			if states[state] != 2 {
				t.Fatalf("states %+v, want 2 %s", states, state)
			}
		}
	})
	t.Run("PWP-V0-001 external-server-survives", func(t *testing.T) { assertExternalSurvived(t, r, server.URL) })
	t.Run("PWP-V0-002 original-config-inputs-and-hooks", func(t *testing.T) {
		for _, suffix := range []string{".setup", ".teardown"} {
			if _, err := os.Stat(filepath.Join(root, "lifecycle") + suffix); err != nil {
				t.Fatal(err)
			}
		}
		if r.Identity.ConfigInputDigests[cfg.ConfigFile] != r.Identity.ConfigDigest || r.External.ConfigOverride == "" {
			t.Fatal("configuration inputs missing")
		}
	})
	t.Run("PWP-V0-005 retained-MCP-discovery", func(t *testing.T) {
		encoded, err := jstestprovider.EncodeQualified(r)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = testevidence.Retain(root, "corvint-js-test-provider", encoded); err != nil {
			t.Fatal(err)
		}
		doc, err := testvaliditydoc.Discover(root)
		if err != nil {
			t.Fatal(err)
		}
		if doc.Playwright == nil || doc.Playwright.External.ServerDescendants != "unknown" || doc.Playwright.ServerDescendantsGone != nil || doc.Run.Freshness.State == testvalidity.FreshnessCurrent {
			t.Fatalf("MCP discovery lost unknowns %+v", doc)
		}
		if len(doc.Tests) != 6 || doc.Tests[0].ID == "" || len(doc.Tests[0].Attempts) == 0 {
			t.Fatalf("discovery lost identity/attempts %+v", doc.Tests)
		}
		if err = os.Mkdir(filepath.Join(root, ".git"), 0700); err != nil {
			t.Fatal(err)
		}
		registry, registryErr := testvaliditybridge.New(root)
		if registryErr != nil {
			t.Fatal(registryErr)
		}
		object, _, toolErr, transportErr := registry.Call(context.Background(), testvaliditybridge.ToolTestValidity, []byte(`{"discover":true}`))
		if toolErr != nil || transportErr != nil {
			t.Fatalf("MCP discovery %v %v", toolErr, transportErr)
		}
		projected, _ := json.Marshal(object["document"])
		var mcpDoc testvaliditydoc.Document
		if err = json.Unmarshal(projected, &mcpDoc); err != nil {
			t.Fatal(err)
		}
		if mcpDoc.Playwright == nil || mcpDoc.Playwright.External.ServerDescendants != "unknown" || mcpDoc.Run.Freshness.State != testvalidity.FreshnessUnknown || len(mcpDoc.Tests) != 6 {
			t.Fatalf("MCP lost qualified facts %+v", mcpDoc)
		}
	})
	cfg.TestArgv = []string{"external.spec.cjs", "--project=broken-browser", "--grep=passing page"}
	infra, err := jstestprovider.RunE2E(context.Background(), cfg)
	if err != nil || len(infra.Tests) != 1 || infra.Tests[0].State != jstestprovider.StateInfrastructure {
		t.Fatalf("browser infrastructure: %v %+v", err, infra)
	}
	assertExternalSurvived(t, infra, server.URL)
	t.Run("PWP-V0-007 live-browser-matrix", func(t *testing.T) {
		if len(r.Tests) != 6 || len(infra.Tests) != 1 || infra.Tests[0].State != jstestprovider.StateInfrastructure {
			t.Fatal("live matrix incomplete")
		}
	})
	cfg.TestArgv = []string{"override.spec.cjs", "--project=chromium"}
	override, err := jstestprovider.RunE2E(context.Background(), cfg)
	if err != nil || override.Infrastructure != nil || len(override.Tests) != 1 {
		t.Fatalf("override run: %v %+v", err, override.Infrastructure)
	}
	var effective struct {
		Viewport struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"viewport"`
	}
	if err = json.Unmarshal(override.Tests[0].Project.Use, &effective); err != nil {
		t.Fatal(err)
	}
	if override.Tests[0].Project.Browser != "firefox" || effective.Viewport.Width != 321 || effective.Viewport.Height != 456 || jstestprovider.ReceiptTestProjection(override, override.Tests[0]).Execution.State != testvalidity.ExecutionPassed {
		t.Fatalf("wrong effective per-test use %+v", override.Tests[0])
	}
	cfg.TestFiles = append(cfg.TestFiles, filepath.Join(root, "dynamic.spec.cjs"))
	cfg.TestArgv = []string{"dynamic.spec.cjs", "--project=chromium"}
	dynamic, err := jstestprovider.RunE2E(context.Background(), cfg)
	if err != nil || dynamic.Infrastructure == nil || len(dynamic.Tests) != 1 || jstestprovider.ReceiptTestProjection(dynamic, dynamic.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
		t.Fatalf("executable override became green: %v %+v", err, dynamic.Infrastructure)
	}
	t.Run("PWP-V0-006 cancellation-preserves-external-server", func(t *testing.T) {
		cfg.TestArgv = []string{"external.spec.cjs", "--project=chromium", "--grep=cancellation"}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan jstestprovider.Receipt, 1)
		joined := false
		t.Cleanup(func() {
			cancel()
			if !joined {
				select {
				case <-done:
				case <-time.After(10 * time.Second):
					t.Error("cancelled runner did not join during cleanup")
				}
			}
		})
		go func() { result, _ := jstestprovider.RunE2E(ctx, cfg); done <- result }()
		select {
		case <-ready:
			cancel()
		case <-time.After(25 * time.Second):
			cancel()
			t.Fatal("browser did not reach cancellation fixture")
		}
		select {
		case cancelled := <-done:
			joined = true
			if !cancelled.Cancelled {
				t.Fatalf("not cancelled %+v", cancelled)
			}
			assertExternalSurvived(t, cancelled, server.URL)
		case <-time.After(10 * time.Second):
			t.Fatal("owned runner did not join after cancellation")
		}
	})
	t.Logf("qualified real Playwright %s: pass/assertion/timeout/browser infra, two projects, retained discovery, cancellation/server survival", pkg.Version)
}

func assertExternalSurvived(t *testing.T, r jstestprovider.Receipt, address string) {
	t.Helper()
	if r.External == nil || !r.External.ReadyAtStart || !r.External.ReadyAtPublish || !r.External.RunnerDescendantsGone || r.ServerDescendantsGone != nil {
		t.Fatalf("lifecycle %+v, owned-server=%v", r.External, r.ServerDescendantsGone)
	}
	resp, err := http.Get(address)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("external server status %d", resp.StatusCode)
	}
}
