package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/changee2e"
	"github.com/Beamfall/corvint/internal/execfile"
)

func gitTest(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
	c.Dir = root
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func fixture(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	for _, src := range []string{"../../internal/appmap/testdata/fixture", "../../internal/changee2e/testdata/overlay"} {
		err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(src, p)
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(root, rel), data, 0o644)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	commit := func(msg string) string {
		gitTest(t, root, "add", "-A")
		gitTest(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", msg)
		return gitTest(t, root, "rev-parse", "HEAD")
	}
	gitTest(t, root, "init", "-q")
	base := commit("base")
	head, err := os.ReadFile(filepath.Join(root, "app/components/teesheet.head.html"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app/components/teesheet.html"), head, 0o644); err != nil {
		t.Fatal(err)
	}
	return root, base, commit("book")
}

// CCE-V0-015: each subcommand is reachable as `corvint-test e2e <sub>`; anything else is a usage
// error, and --out inside the repository refuses.
func TestCCEV0015SubcommandsReachable(t *testing.T) {
	root, base, head := fixture(t)
	ctx := context.Background()
	for _, args := range [][]string{
		{"e2e", "catalog", "--root", root, "--rev", head},
		{"e2e", "harvest", "--root", root, "--rev", head},
		{"e2e", "obligations", "--root", root, "--base", base, "--head", head},
		{"e2e", "compile", "--root", root, "--base", base, "--head", head, "--out", t.TempDir()},
	} {
		var out bytes.Buffer
		if err := run(ctx, args, &out); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		var doc struct {
			Schema string `json:"schema"`
		}
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil || !strings.HasPrefix(doc.Schema, "e2e-") {
			t.Fatalf("%v: %v %s", args, err, out.Bytes())
		}
	}
	out := t.TempDir()
	if err := run(ctx, []string{"e2e", "compile", "--root", root, "--base", base, "--head", head, "--out", out}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "e2e/corvint-compiled/book-tee-time-booked-e77a467370e1.spec.ts")); err != nil {
		t.Fatal(err)
	}
	window := filepath.Join(t.TempDir(), "window.json")
	if err := os.WriteFile(window, []byte(`{"schema":"e2e-shadow-window/0","registration":{"operators":["text-literal-swap"],"seed":"1",`+
		`"catalog_digest":"sha256:00","compiler_version":"corvint-test-e2e-compile/0"},"gate":{},"hosted_tests":[],"entries":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var shadow bytes.Buffer
	if err := run(ctx, []string{"e2e", "shadow", "--window", window}, &shadow); err != nil || !strings.Contains(shadow.String(), `"schema": "e2e-shadow/0"`) {
		t.Fatalf("shadow: %v %s", err, shadow.Bytes())
	}
	// prove is registered: a bad flag fails at parse, not as an unknown subcommand.
	for _, args := range [][]string{{}, {"e2e"}, {"e2e", "unknown"}, {"test"}, {"e2e", "prove", "--bogus"}, {"e2e", "shadow", "--bogus"}} {
		if err := run(ctx, args, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), usage) {
			t.Fatalf("%v: %v", args, err)
		}
	}
	var r *changee2e.Refusal
	err := run(ctx, []string{"e2e", "catalog", "--root", root, "--rev", head, "--out", filepath.Join(root, ".corvint", "c.json")}, &bytes.Buffer{})
	if !errors.As(err, &r) || r.Code != "e2e-out-not-allowed" {
		t.Fatalf("%v", err)
	}
	// CCE-V0-041: the proposals document is written outside the repository only.
	err = run(ctx, []string{"e2e", "harvest", "--root", root, "--rev", head, "--out", filepath.Join(root, "h.json")}, &bytes.Buffer{})
	if !errors.As(err, &r) || r.Code != "e2e-out-not-allowed" {
		t.Fatalf("harvest: %v", err)
	}
}

// CCE-V0-034: prove --hosted-test selects an existing hosted test and reaches Prove without
// compiling; it refuses a title without a file and a file that is not a Playwright test.
func TestCCEV0034HostedTestFlag(t *testing.T) {
	root, base, head := fixture(t)
	ctx := context.Background()
	t.Setenv("CORVINT_E2E_NODE_BIN", "")
	for _, tc := range []struct {
		args []string
		code string
	}{
		{[]string{"--hosted-test", "e2e/specs/teesheet-click.spec.ts"}, "e2e-toolchain-unavailable"},
		{[]string{"--hosted-test", "e2e/specs/teesheet-click.spec.ts", "--hosted-title", "books a slot reached by clicking"}, "e2e-toolchain-unavailable"},
		{[]string{"--hosted-title", "books a slot reached by clicking"}, "e2e-hosted-test-invalid"},
		{[]string{"--hosted-test", "app/components/teesheet.html"}, "e2e-hosted-test-invalid"},
		{[]string{"--hosted-test", "e2e/specs/teesheet-click.spec.ts", "--hosted-title", "no such test"}, "e2e-hosted-test-invalid"},
		{[]string{"--hosted-test", "e2e/specs/teesheet-click.spec.ts", "--coverage", "coverage.json"}, "e2e-hosted-test-invalid"},
		{[]string{"--hosted-test", "e2e/specs/teesheet-click.spec.ts", "--ledger", "ledger.json"}, "e2e-hosted-test-invalid"},
	} {
		out := t.TempDir()
		args := append([]string{"e2e", "prove", "--root", root, "--base", base, "--head", head, "--out", out}, tc.args...)
		var r *changee2e.Refusal
		if err := run(ctx, args, &bytes.Buffer{}); !errors.As(err, &r) || r.Code != tc.code {
			t.Fatalf("%v: %v, want %s", tc.args, err, tc.code)
		}
		if entries, _ := os.ReadDir(out); len(entries) != 0 {
			t.Fatalf("%v: hosted prove compiled into the out directory: %v", tc.args, entries)
		}
	}
}

// CCE-V0-045: --coverage reaches obligations and compile, which refuse a document not bound to head.
func TestCCEV0045CoverageFlag(t *testing.T) {
	root, base, head := fixture(t)
	doc := filepath.Join(t.TempDir(), "coverage.json")
	if err := os.WriteFile(doc, []byte(`{"schema":"e2e-runtime-coverage/0","revision":"`+base+`","screens":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"obligations", "compile"} {
		args := []string{"e2e", sub, "--root", root, "--base", base, "--head", head, "--coverage", doc}
		if sub == "compile" {
			args = append(args, "--out", t.TempDir())
		}
		var r *changee2e.Refusal
		if err := run(context.Background(), args, &bytes.Buffer{}); !errors.As(err, &r) || r.Code != "e2e-coverage-invalid" {
			t.Fatalf("%s: %v", sub, err)
		}
	}
}

// CCE-V0-103: --ledger reaches obligations and compile, which refuse a ledger not bound to the head map.
func TestCCEV0103LedgerFlag(t *testing.T) {
	root, base, head := fixture(t)
	doc := filepath.Join(t.TempDir(), "ledger.json")
	if err := os.WriteFile(doc, []byte(`{"schema":"application-edge-ledger/0","app":"other","revision":"`+head+`","map_digest":"sha256:x","edges":[],"omitted":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"obligations", "compile"} {
		args := []string{"e2e", sub, "--root", root, "--base", base, "--head", head, "--ledger", doc}
		if sub == "compile" {
			args = append(args, "--out", t.TempDir())
		}
		var r *changee2e.Refusal
		if err := run(context.Background(), args, &bytes.Buffer{}); !errors.As(err, &r) || r.Code != "e2e-ledger-invalid" {
			t.Fatalf("%s: %v", sub, err)
		}
	}
}

// CCE-V0-015: core `corvint` never imports the companion or its package; the companion never
// names `corvint` as an executable, and its diagnostics stay disjoint from core `corvint test`'s
// `corvint-test:` prefix.
func TestCCEV0015CoreBoundary(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "../corvint").Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			t.Fatalf("%v: %s", err, ee.Stderr)
		}
		t.Fatal(err)
	}
	for _, dep := range strings.Fields(string(out)) {
		if strings.HasSuffix(dep, "/internal/changee2e") || strings.HasSuffix(dep, "/cmd/corvint-test") {
			t.Fatalf("core corvint imports %s", dep)
		}
	}
	srcs, _ := filepath.Glob("../../internal/changee2e/*.go")
	srcs = append(srcs, "main.go")
	corvintArgv := regexp.MustCompile(`"corvint"|"corvint\s|/corvint"`)
	bareDiag := regexp.MustCompile(`"corvint-test:`)
	for _, p := range srcs {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if corvintArgv.Match(data) || bareDiag.Match(data) {
			t.Fatalf("%s names the core executable or its diagnostic prefix", p)
		}
	}
}

// CCE-V0-015 subprocess check: run the built companion with a `corvint` that records any call
// first on PATH; no subcommand may reach it.
func TestCCEV0015NeverExecutesCorvint(t *testing.T) {
	root, base, head := fixture(t)
	bin := filepath.Join(t.TempDir(), "corvint-test")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	fake := t.TempDir()
	marker := filepath.Join(t.TempDir(), "called")
	if err := execfile.WriteFile(filepath.Join(fake, "corvint"), []byte("#!/bin/sh\necho \"$@\" >> '"+marker+"'\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fake+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, args := range [][]string{
		{"e2e", "catalog", "--root", root, "--rev", head},
		{"e2e", "obligations", "--root", root, "--base", base, "--head", head},
		{"e2e", "compile", "--root", root, "--base", base, "--head", head, "--out", t.TempDir()},
	} {
		if out, err := exec.Command(bin, args...).CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	if _, err := os.Stat(marker); !errors.Is(err, fs.ErrNotExist) {
		data, _ := os.ReadFile(marker)
		t.Fatalf("companion executed corvint: %s", data)
	}
}

// CCE-V0-001/007: a symlink below --out (here <out>/e2e pointing at another directory) refuses
// compile before anything is written through it.
func TestCCEV0001OutNestedSymlinkRefused(t *testing.T) {
	root, base, head := fixture(t)
	out, target := t.TempDir(), t.TempDir()
	if err := os.Symlink(target, filepath.Join(out, "e2e")); err != nil {
		t.Fatal(err)
	}
	var r *changee2e.Refusal
	err := run(context.Background(), []string{"e2e", "compile", "--root", root, "--base", base, "--head", head, "--out", out}, &bytes.Buffer{})
	if !errors.As(err, &r) || r.Code != "e2e-out-unsafe" {
		t.Fatalf("%v", err)
	}
	if entries, err := os.ReadDir(target); err != nil || len(entries) != 0 {
		t.Fatalf("link target written: %v %v", entries, err)
	}
	// Duplicate placements refuse before any write.
	sc := changee2e.Scenario{Placement: "e2e/corvint-compiled/a.spec.ts", Text: []byte("x")}
	dup := t.TempDir()
	if err := writeScenarios(dup, []changee2e.Scenario{sc, sc}); !errors.As(err, &r) || r.Code != "e2e-out-unsafe" {
		t.Fatalf("%v", err)
	}
	if entries, _ := os.ReadDir(dup); len(entries) != 0 {
		t.Fatalf("duplicate placement wrote %v", entries)
	}
}

// CCE-V0-001: a symlinked final file under --out (for example e2e-prove.json) refuses and leaves
// its target untouched; a reserved name is validated before scenario writes.
func TestCCEV0001OutFileSymlinkRefused(t *testing.T) {
	out := t.TempDir()
	victim := filepath.Join(t.TempDir(), "victim")
	if err := os.WriteFile(victim, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, filepath.Join(out, proveReport)); err != nil {
		t.Fatal(err)
	}
	var r *changee2e.Refusal
	if err := writeUnder(out, proveReport, []byte("x"), nil); !errors.As(err, &r) || r.Code != "e2e-out-unsafe" {
		t.Fatalf("%v", err)
	}
	if err := writeFile(filepath.Join(out, proveReport), []byte("x"), nil); !errors.As(err, &r) || r.Code != "e2e-out-unsafe" {
		t.Fatalf("%v", err)
	}
	sc := changee2e.Scenario{Placement: "e2e/corvint-compiled/a.spec.ts", Text: []byte("x")}
	if err := writeScenarios(out, []changee2e.Scenario{sc}, proveReport); !errors.As(err, &r) || r.Code != "e2e-out-unsafe" {
		t.Fatalf("%v", err)
	}
	if _, err := os.Lstat(filepath.Join(out, "e2e")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("scenario written before validation: %v", err)
	}
	if data, _ := os.ReadFile(victim); string(data) != "keep" {
		t.Fatalf("victim overwritten: %q", data)
	}
	for _, rel := range []string{"../x", "/abs", ""} {
		if _, err := checkUnder(out, rel); !errors.As(err, &r) || r.Code != "e2e-out-unsafe" {
			t.Fatalf("%q: %v", rel, err)
		}
	}
}

// repoSnapshot is every worktree file's mode and bytes plus HEAD, the refs and the git config.
func repoSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	snap := map[string]string{"HEAD": gitTest(t, root, "rev-parse", "HEAD"), "refs": gitTest(t, root, "for-each-ref")}
	if cfg, err := os.ReadFile(filepath.Join(root, ".git", "config")); err == nil {
		snap["config"] = string(cfg)
	}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		data := ""
		if d.Type().IsRegular() {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			data = string(b)
		}
		snap["file:"+rel] = info.Mode().String() + "\x00" + data
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

// CCE-V0-010, CCE-V0-013 (V1-1666): no subcommand deletes, disables, skips, moves or rewrites a
// hosted test or any other repository file, ref or configuration, including a hosted prove that
// names the test it runs.
func TestCCEV0013NoRepositoryWrites(t *testing.T) {
	root, base, head := fixture(t)
	ctx := context.Background()
	t.Setenv("CORVINT_E2E_NODE_BIN", "")
	before := repoSnapshot(t, root)
	window := filepath.Join(t.TempDir(), "window.json")
	if err := os.WriteFile(window, []byte(`{"schema":"e2e-shadow-window/0","registration":{"operators":["text-literal-swap"],"seed":"1",`+
		`"catalog_digest":"sha256:00","compiler_version":"corvint-test-e2e-compile/0"},"gate":{},"hosted_tests":[],"entries":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"e2e", "catalog", "--root", root, "--rev", head, "--out", filepath.Join(t.TempDir(), "c.json")},
		{"e2e", "harvest", "--root", root, "--rev", head, "--out", filepath.Join(t.TempDir(), "h.json")},
		{"e2e", "obligations", "--root", root, "--base", base, "--head", head},
		{"e2e", "compile", "--root", root, "--base", base, "--head", head, "--out", t.TempDir()},
		{"e2e", "prove", "--root", root, "--base", base, "--head", head, "--out", t.TempDir()},
		{"e2e", "prove", "--root", root, "--base", base, "--head", head, "--out", t.TempDir(), "--hosted-test", "e2e/specs/teesheet-click.spec.ts"},
		{"e2e", "shadow", "--window", window},
	} {
		_ = run(ctx, args, &bytes.Buffer{}) // prove refuses without a toolchain after resolving its inputs
		if after := repoSnapshot(t, root); len(after) != len(before) {
			t.Fatalf("%v changed the repository file set: %d -> %d entries", args[1], len(before), len(after))
		} else {
			for k, v := range before {
				if after[k] != v {
					t.Fatalf("%v changed %s", args[1], k)
				}
			}
		}
	}
}

// CCE-V0-051: --oracle-acceptance reaches Prove on both the compiled and the hosted path, which
// refuse a document recorded inside the repository before any run.
func TestCCEV0051OracleAcceptanceFlag(t *testing.T) {
	root, base, head := fixture(t)
	ctx := context.Background()
	inside := filepath.Join(root, "acceptance.json")
	if err := os.WriteFile(inside, []byte(`{"schema":"e2e-oracle-acceptance/0","base":"`+base+`","head":"`+head+`","acceptances":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, extra := range [][]string{nil, {"--hosted-test", "e2e/specs/teesheet-click.spec.ts"}} {
		args := append([]string{"e2e", "prove", "--root", root, "--base", base, "--head", head, "--out", t.TempDir(), "--oracle-acceptance", inside}, extra...)
		var r *changee2e.Refusal
		if err := run(ctx, args, &bytes.Buffer{}); !errors.As(err, &r) || r.Code != "e2e-oracle-acceptance-invalid" {
			t.Fatalf("%v: %v, want e2e-oracle-acceptance-invalid", extra, err)
		}
	}
}

// CCE-V0-053: --record-coverage is a prove flag, and writeProof writes the coverage document
// derived from the proof beside it; a document that is not a proof writes neither.
func TestCCEV0053RecordCoverageFlag(t *testing.T) {
	root, base, head := fixture(t)
	t.Setenv("CORVINT_E2E_NODE_BIN", "")
	var r *changee2e.Refusal
	err := run(context.Background(), []string{"e2e", "prove", "--root", root, "--base", base, "--head", head, "--out", t.TempDir(), "--record-coverage"}, &bytes.Buffer{})
	if !errors.As(err, &r) || r.Code != "e2e-toolchain-unavailable" {
		t.Fatalf("prove --record-coverage: %v", err)
	}
	dir := t.TempDir()
	proof := []byte(`{"schema":"` + changee2e.ProveSchema + `","base":"` + base + `","head":"` + head + `","digest":"sha256:p","scenarios":[]}`)
	if err := writeProof(dir, proof, true, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, proveCoverage))
	if err != nil || !strings.Contains(string(data), `"schema": "e2e-runtime-coverage/0"`) || !strings.Contains(string(data), `"revision": "`+head+`"`) {
		t.Fatalf("coverage %v: %s", err, data)
	}
	if _, err := os.Stat(filepath.Join(dir, proveReport)); err != nil {
		t.Fatal(err)
	}
	empty := t.TempDir()
	if err := writeProof(empty, []byte(`{}`), true, &bytes.Buffer{}); !errors.As(err, &r) || r.Code != "e2e-coverage-invalid" {
		t.Fatalf("non-proof: %v", err)
	}
	if entries, _ := os.ReadDir(empty); len(entries) != 0 {
		t.Fatalf("non-proof wrote %v", entries)
	}
}

// CCE-V0-054: --module-graph names the TJAA module graph as consulted in the obligations report;
// without it the relation is listed as not consulted.
func TestCCEV0054ModuleGraphFlag(t *testing.T) {
	root, base, head := fixture(t)
	for _, on := range []bool{false, true} {
		args := []string{"e2e", "obligations", "--root", root, "--base", base, "--head", head}
		if on {
			args = append(args, "--module-graph")
		}
		var out bytes.Buffer
		if err := run(context.Background(), args, &out); err != nil {
			t.Fatal(err)
		}
		var doc struct {
			Relations struct {
				Consulted    []string `json:"consulted"`
				NotConsulted []string `json:"not_consulted"`
			} `json:"relations"`
			ModuleGraph *struct {
				Digest string `json:"digest"`
			} `json:"module_graph"`
		}
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
			t.Fatal(err)
		}
		consulted := strings.Contains(strings.Join(doc.Relations.Consulted, ","), changee2e.ModuleGraphBasis)
		listedNot := strings.Contains(strings.Join(doc.Relations.NotConsulted, ","), changee2e.ModuleGraphBasis)
		if consulted != on || listedNot == on || (doc.ModuleGraph != nil) != on || on && doc.ModuleGraph.Digest == "" {
			t.Fatalf("--module-graph=%v: %s", on, out.Bytes())
		}
	}
}

// CCE-V0-088: `e2e harvest --manifest` binds proposals through the configured map path; without
// it the default appmap.json is absent and every proposal reads app-map-unavailable.
func TestCCEV0088HarvestManifestFlag(t *testing.T) {
	root, _, _ := fixture(t)
	const manifest = "config/club.appmap.json"
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "appmap.json"), filepath.Join(root, manifest)); err != nil {
		t.Fatal(err)
	}
	gitTest(t, root, "add", "-A")
	gitTest(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "move map")
	rev := gitTest(t, root, "rev-parse", "HEAD")
	harvest := func(extra ...string) (bound, unavailable int) {
		t.Helper()
		var out bytes.Buffer
		if err := run(context.Background(), append([]string{"e2e", "harvest", "--root", root, "--rev", rev}, extra...), &out); err != nil {
			t.Fatal(err)
		}
		var doc changee2e.Proposals
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil || len(doc.Proposals) == 0 {
			t.Fatalf("%v %s", err, out.Bytes())
		}
		for _, p := range doc.Proposals {
			switch {
			case p.Binding.Basis != "":
				bound++
			case p.Binding.Unbound == "app-map-unavailable":
				unavailable++
			}
		}
		return bound, unavailable
	}
	if bound, _ := harvest("--manifest", manifest); bound == 0 {
		t.Fatal("no proposal bound through --manifest")
	}
	if bound, unavailable := harvest(); bound != 0 || unavailable == 0 {
		t.Fatalf("default manifest: bound=%d unavailable=%d", bound, unavailable)
	}
}

// CCE-V0-310: `e2e harvest --source DIR` reaches the site-knowledge IR, and a source the IR cannot
// read refuses with e2e-site-knowledge-unavailable.
func TestCCEV0310HarvestSourceFlag(t *testing.T) {
	root, _, _ := fixture(t)
	rev := gitTest(t, root, "rev-parse", "HEAD")
	var out bytes.Buffer
	if err := run(context.Background(), []string{"e2e", "harvest", "--root", root, "--rev", rev, "--source", "e2e"}, &out); err != nil {
		t.Fatal(err)
	}
	var doc changee2e.Proposals
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil || len(doc.Proposals) == 0 {
		t.Fatalf("%v %s", err, out.Bytes())
	}
	var r *changee2e.Refusal
	err := run(context.Background(), []string{"e2e", "harvest", "--root", root, "--rev", rev, "--source", "no-such-dir"}, &bytes.Buffer{})
	if !errors.As(err, &r) || r.Code != "e2e-site-knowledge-unavailable" {
		t.Fatalf("unreadable source: %v", err)
	}
}

// CCE-V0-134: every map-reading verb binds --repo, --manifest-repo and --repo-revision. In a
// two-repository layout catalog, harvest and obligations record the test root's pin (stated
// when --repo-revision names it); compile and prove run across both trees; a revision for an
// undeclared alias is a usage error.
func TestCCEV0134TwoRepoFlags(t *testing.T) {
	root, _, head := fixture(t)
	ctx := context.Background()
	raw, err := os.ReadFile(filepath.Join(root, "appmap.json"))
	if err != nil {
		t.Fatal(err)
	}
	e2e := t.TempDir()
	if err := os.CopyFS(filepath.Join(e2e, "e2e"), os.DirFS(filepath.Join(root, "e2e"))); err != nil {
		t.Fatal(err)
	}
	gitTest(t, e2e, "init", "-q")
	gitTest(t, e2e, "add", "-A")
	gitTest(t, e2e, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "suite")
	e2eRev := gitTest(t, e2e, "rev-parse", "HEAD")
	gitTest(t, root, "rm", "-rq", "e2e")
	if err := os.WriteFile(filepath.Join(root, "appmap.json"), []byte(strings.Replace(string(raw), `"tests": {`, `"tests": {"repo": "e2e",`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	gitTest(t, root, "add", "-A")
	gitTest(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "split")
	split := gitTest(t, root, "rev-parse", "HEAD")
	repo := []string{"--repo", "e2e=" + e2e}
	for _, args := range [][]string{
		append([]string{"e2e", "catalog", "--root", root, "--rev", split}, repo...),
		append([]string{"e2e", "harvest", "--root", root, "--rev", split}, repo...),
		append([]string{"e2e", "obligations", "--root", root, "--base", head, "--head", split}, repo...),
		append([]string{"e2e", "catalog", "--root", root, "--rev", split, "--repo-revision", "e2e=" + e2eRev}, repo...),
	} {
		var out bytes.Buffer
		if err := run(ctx, args, &out); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		var doc struct {
			Roots []struct {
				Repo     string `json:"repo"`
				Revision string `json:"revision"`
				Stated   bool   `json:"stated"`
			} `json:"roots"`
		}
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil || len(doc.Roots) != 1 || doc.Roots[0].Repo != "e2e" || doc.Roots[0].Revision != e2eRev ||
			doc.Roots[0].Stated != slices.Contains(args, "--repo-revision") {
			t.Fatalf("%v: %v %s", args, err, out.Bytes())
		}
	}
	var r *changee2e.Refusal
	if err := run(ctx, []string{"e2e", "catalog", "--root", root, "--rev", split}, &bytes.Buffer{}); !errors.As(err, &r) || r.Code != "e2e-map-unavailable" {
		t.Fatalf("undeclared test root: %v", err)
	}
	// CCE-V0-135/136: compile and prove run across the two trees; the diff is --root's, the
	// compile report records the test root's pin, and output stays outside both trees.
	var compiled bytes.Buffer
	if err := run(ctx, append([]string{"e2e", "compile", "--root", root, "--base", head, "--head", split, "--out", t.TempDir()}, repo...), &compiled); err != nil {
		t.Fatalf("two-tree compile: %v", err)
	}
	var cr struct {
		Roots []struct {
			Repo     string `json:"repo"`
			Revision string `json:"revision"`
		} `json:"roots"`
	}
	if err := json.Unmarshal(compiled.Bytes(), &cr); err != nil || len(cr.Roots) != 1 || cr.Roots[0].Repo != "e2e" || cr.Roots[0].Revision != e2eRev {
		t.Fatalf("two-tree compile pins: %v %s", err, compiled.Bytes())
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"e2e", "prove", "--root", root, "--base", head, "--head", split, "--out", t.TempDir()}, "e2e-scenarios-required"},
		{[]string{"e2e", "prove", "--root", root, "--base", head, "--head", split, "--out", t.TempDir(), "--hosted-test", "e2e/a.spec.ts"}, "e2e-roots-unsupported"},
		{[]string{"e2e", "compile", "--root", root, "--base", head, "--head", split, "--out", filepath.Join(e2e, "out")}, "e2e-out-not-allowed"},
	} {
		if err := run(ctx, append(tc.args, repo...), &bytes.Buffer{}); !errors.As(err, &r) || r.Code != tc.want {
			t.Fatalf("%v: %v, want %s", tc.args, err, tc.want)
		}
	}
	if err := run(ctx, []string{"e2e", "catalog", "--root", root, "--rev", split, "--repo-revision", "e2e=" + e2eRev}, &bytes.Buffer{}); err == nil || errors.As(err, &r) {
		t.Fatalf("--repo-revision without --repo: %v", err)
	}
}
