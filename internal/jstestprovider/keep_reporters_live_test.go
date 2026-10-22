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
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// V1-1002 (PWP-V0-010..014): live keep-reporters witness on a real installed
// Playwright. The project keeps list, html, json and junit reporters plus a
// shape reporter that records the FullConfig.reporter Playwright resolved.
// html writes to its default folder and json/junit to environment-named
// files; all three must land under the original config directory, which
// proves the injected configDir is honoured rather than Playwright's own
// (the provider's scratch controlled-config directory).
const keepLiveConfig = `module.exports = {
  testDir: '.',
  testMatch: /keep\.spec\.cjs$/,
  workers: 1,
  outputDir: 'results',
  projects: [{name: 'chromium', use: {browserName: 'chromium'}}],
  reporter: [['list'], ['html', {open: 'never'}], ['json'], ['junit'], ['./shape-reporter.cjs']],
};
`

const keepLiveShapeReporter = `const fs = require('node:fs');
const path = require('node:path');
class ShapeReporter {
  constructor(options) { this.options = options; }
  onBegin(config) {
    fs.writeFileSync(path.join(__dirname, 'shape.json'), JSON.stringify({reporter: config.reporter, options: this.options === undefined ? null : this.options}));
  }
}
module.exports = ShapeReporter;
`

const keepLiveSpec = `const { test, expect } = require('@playwright/test');
test('keep passes', () => { expect(1).toBe(1); });
test('keep fails', () => { expect(1).toBe(2); });
`

func TestPWPV0010KeepReportersLive(t *testing.T) {
	modules := os.Getenv("CORVINT_PLAYWRIGHT_MODULES")
	if modules == "" {
		t.Skip("NOT_RUN: set CORVINT_PLAYWRIGHT_MODULES to installed node_modules for the live keep-reporters witness")
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
	scopeLiveNode(t, "CORVINT_PLAYWRIGHT_NODE")
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"keep.config.cjs": keepLiveConfig, "shape-reporter.cjs": keepLiveShapeReporter, "keep.spec.cjs": keepLiveSpec}
	for name, body := range files {
		if err = os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err = os.Symlink(modules, filepath.Join(root, "node_modules")); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("keep fixture"))
	}))
	defer server.Close()
	t.Setenv("PLAYWRIGHT_JSON_OUTPUT_NAME", "reports/run.json")
	t.Setenv("PLAYWRIGHT_JUNIT_OUTPUT_NAME", "reports/junit.xml")
	cfg := jstestprovider.E2EConfig{
		Config: jstestprovider.Config{
			Dir: root, ConfigFile: filepath.Join(root, "keep.config.cjs"), TestFiles: []string{filepath.Join(root, "keep.spec.cjs")},
			RunnerName: "playwright", RunnerVersion: pkg.Version,
			DeclaredEnvKeys: []string{"PLAYWRIGHT_BROWSERS_PATH", "PLAYWRIGHT_JSON_OUTPUT_NAME", "PLAYWRIGHT_JUNIT_OUTPUT_NAME"}, Timeout: 45 * time.Second,
		},
		ExternalServer: true, AppIdentity: "keep-reporters-v1", ServerReadyURL: server.URL,
		TestArgv: []string{"keep.spec.cjs"}, KeepReporters: true,
	}
	// The replace-only control run comes first, as qualify-keep-reporters
	// runs it (PWP-V0-015).
	controlCfg := cfg
	controlCfg.KeepReporters = false
	control, controlErr := jstestprovider.RunE2E(context.Background(), controlCfg)
	receipt, err := jstestprovider.RunE2E(context.Background(), cfg)
	encoded, encodeErr := json.MarshalIndent(receipt, "", " ")
	if encodeErr != nil {
		t.Fatal(encodeErr)
	}
	if dir := os.Getenv("CORVINT_KEEP_REPORTERS_EVIDENCE"); dir != "" {
		retainKeepEvidence(t, dir, root, encoded)
	}
	if err != nil || receipt.Infrastructure != nil {
		t.Fatalf("keep-reporters run failed: %v %+v\n%s", err, receipt.Infrastructure, encoded)
	}
	t.Run("PWP-V0-011 kept entries bound in provider-first order", func(t *testing.T) {
		pr := receipt.External.ProjectReporters
		if pr == nil || pr.Effects != "unknown" || len(pr.Entries) != 5 {
			t.Fatalf("project reporters binding: %+v", pr)
		}
		want := []string{"list", "html", "json", "junit", filepath.Join(root, "shape-reporter.cjs")}
		for i, entry := range pr.Entries {
			if entry.Name != want[i] {
				t.Fatalf("entry %d name %q, want %q", i, entry.Name, want[i])
			}
		}
		if pr.Entries[4].Module != "bound" || pr.Entries[0].Module != "builtin" {
			t.Fatalf("module binding: %+v", pr.Entries)
		}
	})
	t.Run("PWP-V0-055 provider scratch files are not config inputs", func(t *testing.T) {
		for _, r := range []jstestprovider.Receipt{control, receipt} {
			for path := range r.Identity.ConfigInputDigests {
				if strings.Contains(path, "/corvint-playwright-") {
					t.Fatalf("provider scratch file bound as a config input: %s", path)
				}
			}
		}
	})
	t.Run("PWP-V0-010 configDir override honoured", func(t *testing.T) {
		for _, rel := range []string{"playwright-report/index.html", "reports/run.json", "reports/junit.xml"} {
			if _, statErr := os.Stat(filepath.Join(root, rel)); statErr != nil {
				t.Fatalf("kept reporter output %s missing under the original config directory: %v", rel, statErr)
			}
		}
		data, readErr := os.ReadFile(filepath.Join(root, "shape.json"))
		if readErr != nil {
			t.Fatal(readErr)
		}
		t.Logf("observed FullConfig.reporter shape: %s", data)
		// Playwright passes every reporter its own configDir (the provider's
		// controlled-config directory) and spreads the entry's options over
		// it, so the injected configDir must win for html, json and junit.
		var shape struct {
			Reporter []json.RawMessage `json:"reporter"`
			Options  struct {
				ConfigDir string `json:"configDir"`
			} `json:"options"`
		}
		if err = json.Unmarshal(data, &shape); err != nil {
			t.Fatal(err)
		}
		// The provider removes its controlled-config directory after the run,
		// so an output written there would be lost; the existence checks
		// above under the original directory are the positive witness.
		controlled := shape.Options.ConfigDir
		if !strings.HasPrefix(filepath.Base(controlled), "corvint-playwright-") || len(shape.Reporter) != 6 {
			t.Fatalf("Playwright's own reporter configDir %q is not the controlled-config directory, or %d reporter entries", controlled, len(shape.Reporter))
		}
		var first []json.RawMessage
		var firstName string
		if json.Unmarshal(shape.Reporter[0], &first) != nil || len(first) == 0 || json.Unmarshal(first[0], &firstName) != nil || filepath.Base(firstName) != "reporter.cjs" {
			t.Fatalf("provider reporter is not first (PWP-V0-014): %s", shape.Reporter[0])
		}
		for i, name := range []string{"html", "json", "junit"} {
			var entry []json.RawMessage
			if err = json.Unmarshal(shape.Reporter[2+i], &entry); err != nil || len(entry) != 2 {
				t.Fatalf("reporter %s entry shape %s: %v", name, shape.Reporter[2+i], err)
			}
			var gotName string
			var options struct {
				ConfigDir string `json:"configDir"`
			}
			if json.Unmarshal(entry[0], &gotName) != nil || json.Unmarshal(entry[1], &options) != nil || gotName != name || options.ConfigDir != root {
				t.Fatalf("reporter %s did not receive configDir %q: %s", name, root, shape.Reporter[2+i])
			}
		}
	})
	t.Run("PWP-V0-012 keep receipt abstains without qualification", func(t *testing.T) {
		if len(receipt.Tests) != 2 {
			t.Fatalf("want 2 outcomes, got %+v", receipt.Tests)
		}
		failed := 0
		for _, outcome := range receipt.Tests {
			if outcome.State != jstestprovider.StatePassed {
				failed++
			}
			if projection := jstestprovider.ReceiptTestProjection(receipt, outcome); projection.Execution.State == testvalidity.ExecutionPassed {
				t.Fatalf("unqualified keep receipt projected passing: %+v", projection)
			}
		}
		if failed != 1 {
			t.Fatalf("want exactly one failing outcome, got %d: %+v", failed, receipt.Tests)
		}
	})
	t.Run("PWP-V0-015 PWP-V0-016 live qualification projects the passing outcome", func(t *testing.T) {
		q := jstestprovider.QualifyKeepReporters(control, controlErr, receipt, err)
		record, _ := json.Marshal(q)
		t.Logf("keep-reporters qualification: %s", record)
		if dir := os.Getenv("CORVINT_KEEP_REPORTERS_EVIDENCE"); dir != "" {
			if writeErr := os.WriteFile(filepath.Join(dir, "qualification.json"), record, 0600); writeErr != nil {
				t.Fatal(writeErr)
			}
		}
		if q.Verdict != jstestprovider.KeepReportersQualified {
			t.Fatalf("live keep-reporters run did not qualify: %v", q.Reasons)
		}
		qualifiedCfg := cfg
		qualifiedCfg.KeepReportersQualification = &q
		qualified, runErr := jstestprovider.RunE2E(context.Background(), qualifiedCfg)
		if runErr != nil || qualified.Infrastructure != nil || len(qualified.Tests) != 2 {
			t.Fatalf("qualified keep run failed: %v %+v %+v", runErr, qualified.Infrastructure, qualified.Tests)
		}
		for _, outcome := range qualified.Tests {
			projection := jstestprovider.ReceiptTestProjection(qualified, outcome)
			if (outcome.State == jstestprovider.StatePassed) != (projection.Execution.State == testvalidity.ExecutionPassed) {
				t.Fatalf("qualified keep outcome %q (%s) projected %+v", outcome.Name, outcome.State, projection.Execution)
			}
		}
	})
}

func retainKeepEvidence(t *testing.T, dir, root string, receipt []byte) {
	t.Helper()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "receipt.json"), receipt, 0600); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"shape.json", "reports/run.json", "reports/junit.xml"} {
		if data, err := os.ReadFile(filepath.Join(root, rel)); err == nil {
			if err = os.WriteFile(filepath.Join(dir, filepath.Base(rel)), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
}
