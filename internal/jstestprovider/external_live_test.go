//go:build darwin || linux

package jstestprovider_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	for _, name := range []string{"playwright.config.cjs", "external.spec.cjs", "override.spec.cjs", "headed.spec.cjs", "connect.spec.cjs", "identity.spec.cjs", "dynamic.spec.cjs", "custom.spec.cjs", "retry.spec.cjs", "interruption.spec.cjs", "setup.cjs", "setup-dependency.cjs", "teardown.cjs"} {
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
	packageJSON, err := filepath.Abs(filepath.Join("..", "..", "conformance", "interactive-alpha", "fixture", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	lockfile, err := filepath.Abs(filepath.Join("..", "..", "conformance", "interactive-alpha", "fixture", "package-lock.json"))
	if err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{})
	var once sync.Once
	interruptReady := make(chan struct{})
	var interruptOnce sync.Once
	interruptWaiting := make(chan struct{})
	var interruptWaitingOnce sync.Once
	interruptHandlerDone := make(chan struct{})
	var interruptHandlerDoneOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cancel-ready" {
			once.Do(func() { close(ready) })
		}
		if r.URL.Path == "/interrupt-ready" {
			interruptOnce.Do(func() { close(interruptReady) })
		}
		if r.URL.Path == "/interrupt-wait" {
			defer interruptHandlerDoneOnce.Do(func() { close(interruptHandlerDone) })
			interruptWaitingOnce.Do(func() { close(interruptWaiting) })
			select {
			case <-interruptReady:
			case <-r.Context().Done():
				return
			}
		}
		_, _ = w.Write([]byte("external fixture"))
	}))
	defer server.Close()
	t.Run("interruption-wait-releases-on-request-cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/interrupt-wait", nil)
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan struct{})
		go func() {
			resp, _ := http.DefaultClient.Do(req)
			if resp != nil {
				resp.Body.Close()
			}
			close(done)
		}()
		select {
		case <-interruptWaiting:
		case <-time.After(time.Second):
			t.Fatal("interruption wait handler did not start")
		}
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("cancelled interruption wait did not release")
		}
		select {
		case <-interruptHandlerDone:
		case <-time.After(time.Second):
			t.Fatal("cancelled interruption handler did not complete")
		}
	})
	t.Setenv("CORVINT_FIXTURE_URL", server.URL)
	t.Setenv("CORVINT_FIXTURE_MARKER", filepath.Join(root, "lifecycle"))
	t.Setenv("CORVINT_FIXTURE_BROWSER_PATH", "")
	cfg := jstestprovider.E2EConfig{Config: jstestprovider.Config{Dir: root, ConfigFile: filepath.Join(root, "playwright.config.cjs"), TestFiles: []string{filepath.Join(root, "external.spec.cjs"), filepath.Join(root, "override.spec.cjs")}, PackageJSON: packageJSON, Lockfile: lockfile, RunnerName: "playwright", RunnerVersion: pkg.Version, DeclaredEnvKeys: []string{"CORVINT_FIXTURE_URL", "CORVINT_FIXTURE_MARKER", "CORVINT_FIXTURE_BROWSER_PATH"}, Timeout: 45 * time.Second}, ExternalServer: true, AppIdentity: "fixture-v1", ServerReadyURL: server.URL, TestArgv: []string{"external.spec.cjs", "--project=chromium", "--project=react", "--grep-invert=cancellation"}}
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
			var use struct {
				Locale        string `json:"locale"`
				Headless      bool   `json:"headless"`
				LaunchOptions struct {
					ExecutablePath string `json:"executablePath"`
				} `json:"launchOptions"`
				CorvintBrowser struct {
					ExecutableSource       string `json:"executableSource"`
					ExecutableName         string `json:"executableName"`
					ExecutablePath         string `json:"executablePath"`
					ExecutableSHA256       string `json:"executableSha256"`
					BrowserRevision        string `json:"browserRevision"`
					ManifestBrowserVersion string `json:"manifestBrowserVersion"`
					BrowserVersion         string `json:"browserVersion"`
				} `json:"corvintBrowser"`
			}
			if json.Unmarshal(test.Project.Use, &use) != nil || use.Locale != "en-CA" || !use.Headless || use.LaunchOptions.ExecutablePath != "" || use.CorvintBrowser.ExecutableSource != "playwright-bundled" || use.CorvintBrowser.ExecutableName != "chromium-headless-shell" || use.CorvintBrowser.ExecutableSHA256 != "a0bfe7b4da4787b66058477d696cd1d09065d25f06a548947722b9af77ee8282" || use.CorvintBrowser.BrowserRevision != "1243" || use.CorvintBrowser.ManifestBrowserVersion != "153.0.8010.12" || use.CorvintBrowser.BrowserVersion != "Google Chrome for Testing 153.0.8010.12" || !strings.HasSuffix(use.CorvintBrowser.ExecutablePath, "/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell") {
				t.Fatalf("global use/project inheritance lost %+v", test.Project)
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
		for _, suffix := range []string{".setup", ".setup-dependency", ".teardown"} {
			if _, err := os.Stat(filepath.Join(root, "lifecycle") + suffix); err != nil {
				t.Fatal(err)
			}
		}
		if r.Identity.ConfigInputDigests[cfg.ConfigFile] != r.Identity.ConfigDigest || r.Identity.ConfigInputDigests[filepath.Join(root, "setup-dependency.cjs")] == "" || r.External.ConfigOverride == "" {
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
	if err != nil || infra.Infrastructure == nil {
		t.Fatalf("browser infrastructure: %v %+v", err, infra)
	}
	assertExternalSurvived(t, infra, server.URL)
	t.Run("PWP-V0-007 live-browser-matrix", func(t *testing.T) {
		if len(r.Tests) != 6 || infra.Infrastructure == nil {
			t.Fatal("live matrix incomplete")
		}
	})
	cfg.TestArgv = []string{"override.spec.cjs", "--project=chromium"}
	override, err := jstestprovider.RunE2E(context.Background(), cfg)
	if err != nil || len(override.Tests) != 1 {
		t.Fatalf("override run: %v %+v", err, override.Infrastructure)
	}
	if override.Infrastructure == nil || len(override.Tests) != 1 || jstestprovider.ReceiptTestProjection(override, override.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
		t.Fatalf("unqualified Firefox override projected green: %v infrastructure=%+v project=%+v use=%s", err, override.Infrastructure, override.Tests[0].Project, override.Tests[0].Project.Use)
	}
	cfg.TestFiles = append(cfg.TestFiles, filepath.Join(root, "headed.spec.cjs"))
	cfg.TestArgv = []string{"headed.spec.cjs", "--project=chromium"}
	headed, err := jstestprovider.RunE2E(context.Background(), cfg)
	if err != nil || headed.Infrastructure == nil || len(headed.Tests) != 1 || jstestprovider.ReceiptTestProjection(headed, headed.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
		t.Fatalf("headed bundled-browser override projected green: %v %+v", err, headed)
	}
	cfg.TestFiles = append(cfg.TestFiles, filepath.Join(root, "connect.spec.cjs"))
	cfg.TestArgv = []string{"connect.spec.cjs", "--project=chromium"}
	connected, err := jstestprovider.RunE2E(context.Background(), cfg)
	if err != nil || connected.Infrastructure == nil || len(connected.Tests) != 1 || jstestprovider.ReceiptTestProjection(connected, connected.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
		t.Fatalf("remote-browser connection projected green: %v %+v", err, connected)
	}
	cfg.TestFiles = append(cfg.TestFiles, filepath.Join(root, "identity.spec.cjs"))
	cfg.TestArgv = []string{"identity.spec.cjs", "--project=chromium"}
	t.Setenv("PW_TEST_CONNECT_WS_ENDPOINT", "ws://127.0.0.1:1")
	environmentConnected, err := jstestprovider.RunE2E(context.Background(), cfg)
	t.Setenv("PW_TEST_CONNECT_WS_ENDPOINT", "")
	if err != nil || environmentConnected.Infrastructure == nil || len(environmentConnected.Tests) != 1 || jstestprovider.ReceiptTestProjection(environmentConnected, environmentConnected.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
		t.Fatalf("environment remote-browser connection projected green: %v %+v", err, environmentConnected)
	}
	cfg.TestFiles = append(cfg.TestFiles, filepath.Join(root, "dynamic.spec.cjs"))
	cfg.TestArgv = []string{"dynamic.spec.cjs", "--project=chromium"}
	dynamic, err := jstestprovider.RunE2E(context.Background(), cfg)
	if err != nil || dynamic.Infrastructure == nil || len(dynamic.Tests) != 1 || jstestprovider.ReceiptTestProjection(dynamic, dynamic.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
		t.Fatalf("executable override became green: %v %+v", err, dynamic.Infrastructure)
	}
	cfg.TestFiles = append(cfg.TestFiles, filepath.Join(root, "custom.spec.cjs"))
	cfg.TestArgv = []string{"custom.spec.cjs", "--project=chromium"}
	custom, err := jstestprovider.RunE2E(context.Background(), cfg)
	if err != nil || custom.Infrastructure == nil || len(custom.Tests) != 1 || jstestprovider.ReceiptTestProjection(custom, custom.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
		t.Fatalf("custom fixture metadata became green: %v %+v", err, custom.Infrastructure)
	}
	cfg.TestFiles = append(cfg.TestFiles, filepath.Join(root, "retry.spec.cjs"))
	cfg.TestArgv = []string{"retry.spec.cjs", "--project=chromium", "--retries=1", "--repeat-each=2", "--workers=2"}
	retried, err := jstestprovider.RunE2E(context.Background(), cfg)
	if err != nil || retried.Infrastructure != nil || len(retried.Tests) != 2 {
		t.Fatalf("retry state lost: %v %+v", err, retried)
	}
	if retried.Tests[0].ID == retried.Tests[1].ID {
		t.Fatal("repeat-each identities collided")
	}
	for _, outcome := range retried.Tests {
		if outcome.State != jstestprovider.StateFlaky || outcome.Retries != 1 || len(outcome.Attempts) != 2 || outcome.Attempts[0].State != jstestprovider.StateFailed || outcome.Attempts[1].State != jstestprovider.StatePassed {
			t.Fatalf("repeat/retry state lost: %+v", outcome)
		}
	}
	cfg.TestFiles = append(cfg.TestFiles, filepath.Join(root, "interruption.spec.cjs"))
	cfg.TestArgv = []string{"interruption.spec.cjs", "--project=chromium", "--workers=2", "--max-failures=1"}
	interrupted, err := jstestprovider.RunE2E(context.Background(), cfg)
	if err != nil || interrupted.Infrastructure == nil || interrupted.Infrastructure.Reason != "reporter-global-error" || len(interrupted.Tests) != 2 {
		t.Fatalf("interruption run: %v %+v", err, interrupted)
	}
	interruptionStates := map[jstestprovider.ExecutionState]int{}
	for _, outcome := range interrupted.Tests {
		interruptionStates[outcome.State]++
		if len(outcome.Attempts) != 1 || outcome.Attempts[0].State != outcome.State || jstestprovider.ReceiptTestProjection(interrupted, outcome).Execution.State == testvalidity.ExecutionPassed {
			t.Fatalf("interruption attempt projected green %+v", outcome)
		}
	}
	if interruptionStates[jstestprovider.StateFailed] != 1 || interruptionStates[jstestprovider.StateInterrupted] != 1 {
		t.Fatalf("interruption states lost %+v", interruptionStates)
	}
	assertExternalSurvived(t, interrupted, server.URL)
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
	t.Setenv("CORVINT_FIXTURE_BROWSER_PATH", "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome")
	cfg.TestFiles = []string{filepath.Join(root, "external.spec.cjs")}
	cfg.TestArgv = []string{"external.spec.cjs", "--project=chromium", "--grep=passing page"}
	system, err := jstestprovider.RunE2E(context.Background(), cfg)
	if err != nil || system.Infrastructure != nil || len(system.Tests) != 1 || jstestprovider.ReceiptTestProjection(system, system.Tests[0]).Execution.State != testvalidity.ExecutionPassed {
		t.Fatalf("previously qualified system-browser tuple regressed: %v %+v", err, system)
	}
	t.Logf("qualified real Playwright %s bundled headless-shell: pass/assertion/timeout/retry/browser infra, two projects, retained discovery, cancellation/server survival; system-browser smoke preserved", pkg.Version)
}

func TestQualifiedPlaywrightLiveDevicesSpread(t *testing.T) {
	modules := os.Getenv("CORVINT_PLAYWRIGHT_MODULES")
	if modules == "" {
		t.Skip("NOT_RUN: set CORVINT_PLAYWRIGHT_MODULES to installed node_modules for live qualification")
	}
	pkgData, err := os.ReadFile(filepath.Join(modules, "@playwright/test/package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err = json.Unmarshal(pkgData, &pkg); err != nil {
		t.Fatal(err)
	}
	if pkg.Version != "1.63.0" {
		t.Skipf("NOT_RUN: devices spread qualification requires Playwright 1.63.0, got %s", pkg.Version)
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"devices.config.cjs", "devices.spec.cjs"} {
		data, readErr := os.ReadFile(filepath.Join("testdata", "external", name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if writeErr := os.WriteFile(filepath.Join(root, name), data, 0600); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	if err = os.Symlink(modules, filepath.Join(root, "node_modules")); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("devices fixture"))
	}))
	defer server.Close()
	t.Setenv("CORVINT_FIXTURE_URL", server.URL)
	configFile := filepath.Join(root, "devices.config.cjs")
	testFile := filepath.Join(root, "devices.spec.cjs")
	cfg := jstestprovider.E2EConfig{
		Config: jstestprovider.Config{
			Dir: root, ConfigFile: configFile, TestFiles: []string{testFile},
			RunnerName: "playwright", RunnerVersion: pkg.Version,
			DeclaredEnvKeys: []string{"CORVINT_FIXTURE_URL"}, Timeout: 45 * time.Second,
		},
		ExternalServer: true, AppIdentity: "devices-spread-v1", ServerReadyURL: server.URL,
		TestArgv: []string{"devices.spec.cjs", "--project=chromium", "--no-deps"},
	}
	receipt, err := jstestprovider.RunE2E(context.Background(), cfg)
	if err != nil || receipt.Infrastructure != nil {
		t.Fatalf("standard devices spread did not qualify: %v %+v", err, receipt.Infrastructure)
	}
	if len(receipt.Tests) != 1 || receipt.Tests[0].Project == nil {
		t.Fatalf("devices spread identity missing: %+v", receipt.Tests)
	}
	outcome := receipt.Tests[0]
	t.Run("PWP-V0-003 standard-devices-spread-identity", func(t *testing.T) {
		if outcome.Project.Name != "chromium" || outcome.Project.Browser != "chromium" || outcome.Project.Device != "unknown" || outcome.Project.ConfigDigest != receipt.Identity.ConfigDigest || outcome.ID == "" {
			t.Fatalf("devices spread identity incomplete: %+v", outcome.Project)
		}
		var use struct {
			DefaultBrowserType string `json:"defaultBrowserType"`
			Headless           bool   `json:"headless"`
			UserAgent          string `json:"userAgent"`
			Viewport           struct {
				Width  int `json:"width"`
				Height int `json:"height"`
			} `json:"viewport"`
			CorvintBrowser struct {
				BrowserVersion string `json:"browserVersion"`
				ExecutablePath string `json:"executablePath"`
			} `json:"corvintBrowser"`
		}
		if err = json.Unmarshal(outcome.Project.Use, &use); err != nil {
			t.Fatal(err)
		}
		if use.DefaultBrowserType != "chromium" || !use.Headless || use.UserAgent == "" || use.Viewport.Width != 1280 || use.Viewport.Height != 720 || use.CorvintBrowser.BrowserVersion != "Google Chrome for Testing 153.0.8010.12" || !strings.HasSuffix(use.CorvintBrowser.ExecutablePath, "/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell") {
			t.Fatalf("devices spread effective use incomplete: %s", outcome.Project.Use)
		}
	})
	t.Run("PWP-V0-008 bundled-headless-qualified-tuple", func(t *testing.T) {
		if projection := jstestprovider.ReceiptTestProjection(receipt, outcome); projection.Execution.State != testvalidity.ExecutionPassed {
			t.Fatalf("devices spread projection did not pass: %+v", projection)
		}
	})
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
