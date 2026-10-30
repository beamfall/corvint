package jstestprovider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// keptFixture is the passing external fixture produced in keep-reporters mode.
func keptFixture(t *testing.T) Receipt {
	t.Helper()
	r := qualifiedFixture(t)
	r.External.ConfigOverride = "controlled-fixture-config" + keptConfigSuffix
	r.Identity.ConfigInputDigests["/repo/project-reporter.cjs"] = strings.Repeat("b", 64)
	r.Tests[0].ID = qualifiedTestID(r.Identity, r.Tests[0])
	if err := bindProjectReporters(&r, []reportedProjectReporter{
		{Name: "list", Options: "absent"},
		{Name: "/repo/project-reporter.cjs", Options: "bound", OptionsDigest: strings.Repeat("c", 64)},
		{Name: "/outside/reporter.mjs", Options: "unknown"},
	}); err != nil {
		t.Fatal(err)
	}
	return r
}

// PWP-V0-010: the default controlled config is byte-identical to the
// replace-only template; keep mode is opt-in.
func TestExternalCommandDefaultConfigUnchanged(t *testing.T) {
	scratch := t.TempDir()
	cfg := E2EConfig{Config: Config{ConfigFile: "/repo/playwright.config.cjs"}}
	config, _, reportPath, err := externalCommand(cfg, scratch)
	if err != nil {
		t.Fatal(err)
	}
	q := func(s string) string { b, _ := json.Marshal(s); return string(b) }
	want := "const imported = require(" + q("/repo/playwright.config.cjs") + ");\nconst original = imported.default || imported;\nconst base = " + q("/repo") + ";\nconst resolve = value => require('node:path').resolve(base, value);\nconst modulePath = value => Array.isArray(value) ? value.map(modulePath) : typeof value === 'string' ? require.resolve(value, {paths:[base]}) : value;\nconst paths = object => { const result = {...object}; for (const key of ['testDir', 'outputDir', 'snapshotDir', 'tsconfig']) if (typeof result[key] === 'string') result[key] = resolve(result[key]); return result; };\nmodule.exports = {...paths(original), testDir: original.testDir ? resolve(original.testDir) : base, globalSetup: modulePath(original.globalSetup), globalTeardown: modulePath(original.globalTeardown), projects: original.projects?.map(paths), webServer: undefined, reporter: [[" + q(filepath.Join(scratch, "reporter.cjs")) + ", {output:" + q(reportPath) + ", sensitiveInputPolicy:null}]]};\n"
	if config != want {
		t.Fatalf("default controlled config drifted:\n%s", config)
	}
	if keptConfig(config) {
		t.Fatal("default config classified as keep-reporters")
	}
	cfg.KeepReporters = true
	kept, _, _, err := externalCommand(cfg, scratch)
	if err != nil || !keptConfig(kept) {
		t.Fatalf("keep config not recognised: %v\n%s", err, kept)
	}
}

func evaluateReporters(t *testing.T, config string) (string, error) {
	t.Helper()
	out, err := exec.Command("node", "-e", "process.stdout.write(JSON.stringify(require(process.argv[1]).reporter))", config).CombinedOutput()
	return string(out), err
}

// PWP-V0-010, PWP-V0-014: keep mode places the provider reporter before every
// project reporter, resolving module paths and built-in output paths from the
// original config directory.
func TestKeptReporterListResolution(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(root, "project")
	if err := os.MkdirAll(filepath.Join(project, "node_modules", "pkg-reporter"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(project, "node_modules", "pkg-reporter", "index.js"), "module.exports = class {};\n")
	for name, test := range map[string]struct {
		reporter string
		want     string
		invalid  bool
	}{
		"array": {reporter: `[['list'], ['./project-reporter.cjs', {target: 'evidence.json'}], ['json', {outputFile: 'out/report.json'}], 'dot', ['html'], ['junit', {configDir: '/elsewhere'}], ['pkg-reporter'], ['unresolvable-reporter']]`,
			want: `[["list"],["` + filepath.Join(project, "project-reporter.cjs") + `",{"target":"evidence.json"}],["json",{"configDir":"` + project + `","outputFile":"` + filepath.Join(project, "out", "report.json") + `"}],["dot"],["html",{"configDir":"` + project + `"}],["junit",{"configDir":"/elsewhere"}],["` + filepath.Join(project, "node_modules", "pkg-reporter", "index.js") + `"],["unresolvable-reporter"],`},
		"string":    {reporter: `'line'`, want: `[["line"],`},
		"undefined": {reporter: `undefined`, want: `[`},
		"number":    {reporter: `42`, invalid: true},
		"object":    {reporter: `[{name: 'list'}]`, invalid: true},
		"too-long":  {reporter: `[['list', {}, 'extra']]`, invalid: true},
	} {
		t.Run(name, func(t *testing.T) {
			configFile := filepath.Join(project, name+".config.cjs")
			write(configFile, "module.exports = {reporter: "+test.reporter+"};\n")
			scratch := t.TempDir()
			config, _, reportPath, err := externalCommand(E2EConfig{Config: Config{ConfigFile: configFile}, KeepReporters: true}, scratch)
			if err != nil {
				t.Fatal(err)
			}
			controlled := filepath.Join(scratch, "playwright.config.cjs")
			if got, _ := os.ReadFile(controlled); string(got) != config {
				t.Fatal("retained override differs from the written config")
			}
			out, err := evaluateReporters(t, controlled)
			if test.invalid {
				if err == nil || !strings.Contains(out, projectReportersInvalid) {
					t.Fatalf("invalid reporter value accepted: %v %s", err, out)
				}
				return
			}
			if err != nil {
				t.Fatalf("%v\n%s", err, out)
			}
			want := `[["` + filepath.Join(scratch, "reporter.cjs") + `",{"output":"` + reportPath + `","sensitiveInputPolicy":null,"keepReporters":true}]`
			if entries := strings.TrimSuffix(strings.TrimPrefix(test.want, "["), ","); entries != "" {
				want += "," + entries
			}
			want += "]"
			if out != want {
				t.Fatalf("kept list:\n got %s\nwant %s", out, want)
			}
		})
	}
}

// PWP-V0-010..012: a generated config loaded by a stand-in runner runs the
// project reporter beside the provider reporter only in keep mode, and the
// provider reports the kept entries the receipt binds.
func TestKeptProjectReporterRunsBesideProvider(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "node_modules", "playwright"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"node_modules/playwright/index.js": "module.exports = {chromium: {executablePath: () => ''}};\n",
		"playwright.config.cjs":            "module.exports = {reporter: [['list'], ['./project-reporter.cjs', {target: 'evidence.json'}]]};\n",
		"project-reporter.cjs":             "const fs = require('node:fs'); const path = require('node:path');\nmodule.exports = class { constructor(options) { this.target = path.join(__dirname, options.target); } onBegin() {} onEnd(result) { fs.writeFileSync(this.target, JSON.stringify({status: result.status})); } };\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	runner := filepath.Join(t.TempDir(), "runner.cjs")
	builtins, _ := json.Marshal(builtinPlaywrightReporters)
	if err := os.WriteFile(runner, []byte("const config = require(process.argv[2]); const builtin = "+string(builtins)+";\nconst reporters = config.reporter.filter(([name]) => !builtin.includes(name)).map(([name, options]) => new (require(name))(options || {}));\nfor (const r of reporters) r.onBegin({workers: 1, version: '1.60.0', reporter: config.reporter}, {allTests: () => []});\nfor (const r of reporters) r.onEnd({status: 'passed'});\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, keep := range []bool{false, true} {
		scratch := t.TempDir()
		evidence := filepath.Join(root, "evidence.json")
		_ = os.Remove(evidence)
		config, _, reportPath, err := externalCommand(E2EConfig{Config: Config{ConfigFile: filepath.Join(root, "playwright.config.cjs")}, KeepReporters: keep}, scratch)
		if err != nil {
			t.Fatal(err)
		}
		command := exec.Command("node", runner, filepath.Join(scratch, "playwright.config.cjs"))
		command.Dir = root
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("keep=%v: %v\n%s", keep, err, out)
		}
		if _, err := os.Stat(evidence); (err == nil) != keep {
			t.Fatalf("keep=%v: project reporter evidence present=%v", keep, err == nil)
		}
		raw, err := os.ReadFile(reportPath)
		if err != nil {
			t.Fatal(err)
		}
		var report qualifiedReport
		if err := json.Unmarshal(raw, &report); err != nil {
			t.Fatal(err)
		}
		if !keep {
			if strings.Contains(string(raw), "projectReporters") || report.ProjectReporters != nil {
				t.Fatal("default report carried project reporter entries")
			}
			continue
		}
		custom := filepath.Join(root, "project-reporter.cjs")
		options := sha256.Sum256([]byte(`{"target":"evidence.json"}`))
		want := []reportedProjectReporter{{Name: "list", Options: "absent"}, {Name: custom, Options: "bound", OptionsDigest: hex.EncodeToString(options[:])}}
		if got, _ := json.Marshal(report.ProjectReporters); string(got) != string(mustJSON(t, want)) {
			t.Fatalf("reported entries %s", got)
		}
		r := qualifiedFixture(t)
		r.External.ConfigOverride = config
		r.Identity.ConfigInputDigests = report.ConfigFiles
		r.Identity.ConfigInputDigests[r.Identity.ConfigFile] = r.Identity.ConfigDigest
		r.Tests[0].ID = qualifiedTestID(r.Identity, r.Tests[0])
		if err := bindProjectReporters(&r, report.ProjectReporters); err != nil {
			t.Fatal(err)
		}
		if entry := r.External.ProjectReporters.Entries[1]; entry.Module != "bound" || entry.ModuleDigest != report.ConfigFiles[custom] {
			t.Fatalf("loaded project reporter module not bound: %+v", entry)
		}
		if _, err := EncodeQualified(r); err != nil {
			t.Fatalf("observed keep-reporters receipt refused: %v", err)
		}
		if ReceiptTestProjection(r, r.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
			t.Fatal("unqualified keep-reporters receipt projected passing")
		}
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// PWP-V0-011/012: the binding is closed, agrees with the retained config in
// both directions and never weakens the qualified projection.
func TestProjectReportersBindingShape(t *testing.T) {
	r := keptFixture(t)
	// PWP-V0-012: keep-reporters mode abstains until its own live
	// qualification; the same receipt without the binding passes.
	if ReceiptTestProjection(r, r.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
		t.Fatal("unqualified keep-reporters receipt projected passing")
	}
	control := r
	controlExternal := *r.External
	controlExternal.ConfigOverride, controlExternal.ProjectReporters = "controlled-fixture-config", nil
	control.External = &controlExternal
	if ReceiptTestProjection(control, control.Tests[0]).Execution.State != testvalidity.ExecutionPassed {
		t.Fatal("control receipt without keep-reporters did not pass")
	}
	encoded, err := EncodeQualified(r)
	if err != nil || !strings.Contains(string(encoded), `"projectReporters":{"entries":[{"name":"list","module":"builtin","options":"absent"},{"name":"/repo/project-reporter.cjs","module":"bound","moduleDigest":"`+strings.Repeat("b", 64)+`","options":"bound","optionsDigest":"`+strings.Repeat("c", 64)+`"},{"name":"/outside/reporter.mjs","module":"unknown","options":"unknown"}],"effects":"unknown"}`) {
		t.Fatalf("binding not retained: %v\n%s", err, encoded)
	}
	plain, err := EncodeQualified(qualifiedFixture(t))
	if err != nil || strings.Contains(string(plain), "projectReporters") {
		t.Fatalf("default receipt gained a binding: %v", err)
	}
	for name, mutate := range map[string]func(*Receipt){
		"binding-without-keep-config": func(r *Receipt) { r.External.ConfigOverride = "controlled-fixture-config" },
		"keep-config-without-binding": func(r *Receipt) { r.External.ProjectReporters = nil },
		"effects-trusted":             func(r *Receipt) { r.External.ProjectReporters.Effects = "none" },
		"unobserved-on-pass":          func(r *Receipt) { r.External.ProjectReporters.Entries = nil },
		"empty-name":                  func(r *Receipt) { r.External.ProjectReporters.Entries[2].Name = "" },
		"builtin-with-digest":         func(r *Receipt) { r.External.ProjectReporters.Entries[0].ModuleDigest = strings.Repeat("a", 64) },
		"builtin-as-unknown":          func(r *Receipt) { r.External.ProjectReporters.Entries[0].Module = "unknown" },
		"bound-module-drift":          func(r *Receipt) { r.External.ProjectReporters.Entries[1].ModuleDigest = strings.Repeat("d", 64) },
		"bound-module-not-input": func(r *Receipt) {
			r.External.ProjectReporters.Entries[2].Module = "bound"
			r.External.ProjectReporters.Entries[2].ModuleDigest = strings.Repeat("b", 64)
		},
		"unknown-with-digest":    func(r *Receipt) { r.External.ProjectReporters.Entries[2].ModuleDigest = strings.Repeat("b", 64) },
		"invented-module-state":  func(r *Receipt) { r.External.ProjectReporters.Entries[2].Module = "trusted" },
		"options-digest-shape":   func(r *Receipt) { r.External.ProjectReporters.Entries[1].OptionsDigest = "short" },
		"absent-with-digest":     func(r *Receipt) { r.External.ProjectReporters.Entries[0].OptionsDigest = strings.Repeat("c", 64) },
		"invented-options-state": func(r *Receipt) { r.External.ProjectReporters.Entries[0].Options = "none" },
		"too-many-entries": func(r *Receipt) {
			for len(r.External.ProjectReporters.Entries) <= maxProjectReporters {
				r.External.ProjectReporters.Entries = append(r.External.ProjectReporters.Entries, ProjectReporter{Name: "dot", Module: "builtin", Options: "absent"})
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := keptFixture(t)
			mutate(&r)
			if ReceiptTestProjection(r, r.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
				t.Fatal("malformed binding projected passing")
			}
			if _, err := EncodeQualified(r); err == nil || !strings.Contains(err.Error(), projectReportersInvalid) {
				t.Fatalf("malformed binding encoded: %v", err)
			}
		})
	}
	// An infrastructure receipt may retain unobserved entries and still encode.
	r = keptFixture(t)
	r.External.ProjectReporters.Entries = nil
	r.Infrastructure = &InfrastructureFailure{Reason: "report-unparseable"}
	if _, err := EncodeQualified(r); err != nil {
		t.Fatalf("unobserved infrastructure binding refused: %v", err)
	}
	r = keptFixture(t)
	if err := bindProjectReporters(&r, nil); err == nil || err.Error() != "project-reporters-unobserved" {
		t.Fatalf("missing entries bound: %v", err)
	}
	if err := bindProjectReporters(&r, []reportedProjectReporter{{Name: "list", Options: "maybe"}}); err == nil || r.External.ProjectReporters.Entries[0].Options != "absent" {
		t.Fatal("invalid entries replaced the existing binding")
	}
}

// PWP-V0-013: keep mode is refused outside the external profiles, including
// the direct freshness entry point, before any process starts.
func TestKeepReportersUnsupportedMode(t *testing.T) {
	if _, err := RunFreshE2E(context.Background(), E2EConfig{KeepReporters: true, Freshness: &FreshnessConfig{}}); err == nil || err.Error() != "keep-reporters-unsupported-mode" {
		t.Fatalf("direct freshness run: %v", err)
	}
	for name, cfg := range map[string]E2EConfig{
		"owned-server": {KeepReporters: true},
		"freshness":    {KeepReporters: true, ExternalServer: true, Freshness: &FreshnessConfig{}},
	} {
		if _, err := RunE2E(context.Background(), cfg); err == nil || err.Error() != "keep-reporters-unsupported-mode" {
			t.Fatalf("%s: %v", name, err)
		}
	}
}
