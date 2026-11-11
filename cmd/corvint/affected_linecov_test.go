package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/linecov"
)

// lineFixture is a module whose user package imports calc and references
// Double: a change to Double's body keeps TestDouble and leaves user out,
// since no test of user runs Double.
var lineFixture = map[string]string{
	"go.mod": "module example.com/lc\n\ngo 1.22\n",
	"calc/calc.go": `package calc

func Classify(n int) string {
	if n < 0 {
		return "neg"
	}
	return "other"
}

func Double(n int) int {
	return n * 2
}
`,
	"calc/calc_test.go": `package calc

import "testing"

func TestNeg(t *testing.T) {
	if Classify(-1) != "neg" {
		t.Fatal("neg")
	}
}

func TestDouble(t *testing.T) {
	if Double(2) != 4 {
		t.Fatal("double")
	}
}
`,
	"user/user.go": `package user

import "example.com/lc/calc"

func Label(n int) string { return calc.Classify(n) }

// Twice references Double, so the static plan selects user for a Double
// change, but no test of user runs it.
func Twice(n int) int { return calc.Double(n) }
`,
	"user/user_test.go": `package user

import "testing"

func TestLabel(t *testing.T) {
	if Label(1) != "other" {
		t.Fatal("label")
	}
}
`,
	".github/cishards/test-commands.json": `{"profile": "corvint-test-commands/0", "go": {"env": {"GOTOOLCHAIN": "local"}, "flags": ["-count=1"]}, "languages": {}}
`,
}

// lineFixtureRepository writes and commits lineFixture, collects its
// index under this process's runner identity and returns the root, the
// base commit and the index file.
func lineFixtureRepository(t *testing.T, overrides ...map[string]string) (string, string, string) {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil || runtime.GOOS == "windows" {
		t.Skip("needs a POSIX go toolchain")
	}
	root := t.TempDir()
	files := map[string]string{}
	for name, body := range lineFixture {
		files[name] = body
	}
	for _, extra := range overrides {
		for name, body := range extra {
			files[name] = body
		}
	}
	for name, body := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"},
		{"config", "maintenance.auto", "false"}, {"config", "gc.auto", "0"}, {"add", "."}, {"commit", "-qm", "fixture"},
	} {
		affectedGit(t, root, arguments...)
	}
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	ctx := context.Background()
	runner, goflags, err := linecov.RunnerID(ctx, root, nil, linecov.DefaultTestTimeout, "")
	if err != nil {
		t.Fatal(err)
	}
	outcomes, err := linecov.Collect(ctx, root, linecov.CollectOptions{Module: "example.com/lc", Packages: []string{"calc", "user"}, Work: t.TempDir(), Parallel: 4, Timeout: 5 * time.Minute, SuiteTimeout: 5 * time.Minute, TestTimeout: linecov.DefaultTestTimeout, GOFLAGS: goflags})
	if err != nil {
		t.Fatal(err)
	}
	var records []linecov.Record
	for _, o := range outcomes {
		if o.Record == nil {
			t.Fatalf("%s.%s diverged: %s", o.Package, o.Name, o.Divergent)
		}
		records = append(records, *o.Record)
	}
	oids, err := linecov.OIDs(ctx, root, base)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := linecov.Build("example.com/lc", base, runner, oids, records)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "linecov.idx")
	var b bytes.Buffer
	if err := ix.Encode(&b); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, base, file
}

// lineCommit replaces old with new in name and commits it.
func lineCommit(t *testing.T, root, name, old, new string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(name))
	src, err := os.ReadFile(full)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, bytes.Replace(src, []byte(old), []byte(new), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "commit", "-qam", "change")
}

// lineUnits renders a lineSelection's units as unit=mode[:run].
func lineUnits(t *testing.T, receipt map[string]any) (string, []string) {
	t.Helper()
	sel, ok := receipt["lineSelection"].(map[string]any)
	if !ok {
		t.Fatalf("no lineSelection: %v", receipt["lineSelection"])
	}
	var out []string
	units, _ := sel["units"].([]any)
	for _, u := range units {
		m := u.(map[string]any)
		entry := strings.TrimPrefix(m["unit"].(string), "go:example.com/lc/") + "=" + m["mode"].(string)
		if run, _ := m["run"].(string); run != "" {
			entry += ":" + run
		}
		out = append(out, entry)
	}
	sort.Strings(out)
	state, _ := sel["state"].(string)
	if reason, _ := sel["reason"].(string); reason != "" {
		state += " " + reason
	}
	return state, out
}

// GLS-V0-010: --line-index adds lineSelection, narrows by line, leaves plan
// unchanged, and refuses every other form.
func TestAffectedLineIndexNarrowsTheBasePlan(t *testing.T) {
	root, base, file := lineFixtureRepository(t)
	lineCommit(t, root, "calc/calc.go", "return n * 2", "return n * 3")
	plain, _, stderr, code := runAffectedCLI(t, root, "--base", base)
	if code != 0 {
		t.Fatalf("plain exit %d: %s", code, stderr)
	}
	receipt, _, stderr, code := runAffectedCLI(t, root, "--base", base, "--line-index", file)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if !reflect.DeepEqual(plain["plan"], receipt["plan"]) || !reflect.DeepEqual(plain["advice"], receipt["advice"]) || plain["lineSelection"] != nil {
		t.Fatal("--line-index changed the plan or advice")
	}
	state, units := lineUnits(t, receipt)
	if want := []string{"calc=whole", "user=exclude"}; state != linecov.StateShadow+" "+linecov.ReasonUnqualified || !reflect.DeepEqual(units, want) {
		t.Fatalf("%s %v, want shadow %v", state, units, want)
	}
	sel := receipt["lineSelection"].(map[string]any)
	// SHADOW retains the proposed exclusion as evidence (GLS-V0-011): user,
	// which reads no changed file and whose test executed no changed line.
	if sel["exclusion"] == nil {
		t.Fatal("the shadow selection retains no exclusion entry")
	}

	for _, key := range []string{"loadMillis", "selectMillis", "staticMillis", "gitMillis"} {
		if _, ok := sel[key].(float64); !ok {
			t.Errorf("no %s", key)
		}
	}
	// A dirty worktree is the worktree-dirty FALLBACK, deciding nothing.
	if err := os.WriteFile(filepath.Join(root, "calc", "calc.go"), []byte("package calc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	receipt, _, stderr, code = runAffectedCLI(t, root, "--base", base, "--line-index", file)
	if code != 0 {
		t.Fatalf("dirty exit %d: %s", code, stderr)
	}
	if state, units := lineUnits(t, receipt); state != "FALLBACK "+linecov.ReasonWorktreeDirty || len(units) != 0 {
		t.Fatalf("dirty: %s %v", state, units)
	}
	affectedGit(t, root, "checkout", "--", ".")
	// A missing index is the index-unreadable FALLBACK, not a refusal.
	receipt, _, _, code = runAffectedCLI(t, root, "--base", base, "--line-index", filepath.Join(t.TempDir(), "absent.idx"))
	if state, _ := lineUnits(t, receipt); code != 0 || state != "FALLBACK "+linecov.ReasonIndexUnreadable {
		t.Fatalf("absent index: exit %d %s", code, state)
	}
}

// GLS-V0-009, GLS-V0-011: an index collected under GOAMD64=v1 is the
// runner-mismatch FALLBACK for a GOAMD64=v2 runner, so the static plan stands.
func TestAffectedLineArchFeatureMismatchKeepsStaticPlan(t *testing.T) {
	t.Setenv("GOAMD64", "v1")
	root, base, file := lineFixtureRepository(t)
	lineCommit(t, root, "calc/calc.go", "return n * 2", "return n * 3")
	t.Setenv("GOAMD64", "v2")
	plain, _, stderr, code := runAffectedCLI(t, root, "--base", base)
	if code != 0 {
		t.Fatalf("plain exit %d: %s", code, stderr)
	}
	receipt, _, stderr, code := runAffectedCLI(t, root, "--base", base, "--line-index", file)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if !reflect.DeepEqual(plain["plan"], receipt["plan"]) {
		t.Fatal("--line-index changed the plan")
	}
	detail, _ := receipt["lineSelection"].(map[string]any)["detail"].(string)
	if state, units := lineUnits(t, receipt); state != "FALLBACK "+linecov.ReasonRunnerMismatch || len(units) != 0 || !strings.Contains(detail, "GOAMD64=v2") {
		t.Fatalf("GOAMD64 v1 index, v2 runner: %s %v %q", state, units, detail)
	}
}

// GLS-V0-020: the index runner names the test runtime it was collected
// under, so a selection for tests run under another go test -timeout is the
// runner-mismatch FALLBACK, and a GOFLAGS test flag is runner-unavailable;
// the matching default timeout decides.
func TestAffectedLineTestRuntimeMismatchKeepsStaticPlan(t *testing.T) {
	root, base, file := lineFixtureRepository(t)
	lineCommit(t, root, "calc/calc.go", "return n * 2", "return n * 3")
	plain, _, stderr, code := runAffectedCLI(t, root, "--base", base)
	if code != 0 {
		t.Fatalf("plain exit %d: %s", code, stderr)
	}
	for _, c := range []struct {
		args          []string
		goflags, want string
		detail        string
	}{
		{args: []string{"--line-test-timeout", "50m"}, want: "FALLBACK " + linecov.ReasonRunnerMismatch, detail: "-test.timeout=50m0s"},
		{args: []string{"--line-test-timeout", "0"}, want: "FALLBACK " + linecov.ReasonRunnerMismatch, detail: `test="-test.paniconexit0 -test.v=test2json"`},
		{goflags: "-short", want: "FALLBACK " + linecov.ReasonRunnerUnavailable, detail: "-short"},
		{want: linecov.StateShadow + " " + linecov.ReasonUnqualified},
		{args: []string{"--line-test-timeout", "10m"}, want: linecov.StateShadow + " " + linecov.ReasonUnqualified},
	} {
		t.Setenv("GOFLAGS", c.goflags)
		receipt, _, stderr, code := runAffectedCLI(t, root, append([]string{"--base", base, "--line-index", file}, c.args...)...)
		if code != 0 {
			t.Fatalf("%v: exit %d: %s", c.args, code, stderr)
		}
		if !reflect.DeepEqual(plain["plan"], receipt["plan"]) {
			t.Fatalf("%v: --line-index changed the plan", c.args)
		}
		detail, _ := receipt["lineSelection"].(map[string]any)["detail"].(string)
		if state, _ := lineUnits(t, receipt); state != c.want || !strings.Contains(detail, c.detail) {
			t.Errorf("%v GOFLAGS=%q: %s %q, want %s naming %q", c.args, c.goflags, state, detail, c.want, c.detail)
		}
	}
}

func TestAffectedLineIndexProfileRefusals(t *testing.T) {
	for _, extra := range [][]string{
		{"--full"}, {"--summary"}, {"--format", "text"}, {"--path", "a.go"}, {"--snapshot", "s.json"},
		{"--provider", "p.json"}, {"--playwright-config", "playwright.config.ts"}, {"--appmap-impact", "r.json"},
	} {
		args := append([]string{"--line-index", "x.idx"}, extra...)
		if extra[0] != "--path" && extra[0] != "--snapshot" {
			args = append(args, "--base", "HEAD")
		}
		_, err := parseAffectedOptions(args)
		if err == nil || !strings.Contains(err.Error(), "--line-index") {
			t.Errorf("%v: %v", extra, err)
		}
	}
	if _, err := parseAffectedOptions([]string{"--line-index", "x.idx"}); err == nil || !strings.Contains(err.Error(), affectedLineIndexProfileCode) && !strings.Contains(err.Error(), "--base") {
		t.Errorf("without --base: %v", err)
	}
	if invocation, err := parseAffectedOptions([]string{"--base", "HEAD", "--line-index", "x.idx"}); err != nil || invocation.LineIndex != "x.idx" || invocation.lineTimeout != linecov.DefaultTestTimeout {
		t.Fatalf("supported form: %v %+v", err, invocation)
	}
	if invocation, err := parseAffectedOptions([]string{"--base", "HEAD", "--line-index", "x.idx", "--line-test-timeout", "50m"}); err != nil || invocation.lineTimeout != 50*time.Minute {
		t.Fatalf("--line-test-timeout 50m: %v %+v", err, invocation)
	}
	// GLS-V0-009, GLS-V0-010: --line-exec names the go test -exec wrapper the
	// identity binds; without --line-index it is refused.
	if invocation, err := parseAffectedOptions([]string{"--base", "HEAD", "--line-index", "x.idx", "--line-exec", "/w/test-confine"}); err != nil || invocation.LineExec != "/w/test-confine" {
		t.Fatalf("--line-exec: %v %+v", err, invocation)
	}
	if _, err := parseAffectedOptions([]string{"--base", "HEAD", "--line-exec", "/w/test-confine"}); err == nil || !strings.Contains(err.Error(), "--line-exec") {
		t.Errorf("--line-exec without --line-index: %v, want a refusal", err)
	}
	for _, args := range [][]string{{"--base", "HEAD", "--line-test-timeout", "50m"}, {"--base", "HEAD", "--line-index", "x.idx", "--line-test-timeout", "soon"}, {"--base", "HEAD", "--line-index", "x.idx", "--line-test-timeout", "-1m"}} {
		if _, err := parseAffectedOptions(args); err == nil || !strings.Contains(err.Error(), "--line-test-timeout") {
			t.Errorf("%v: %v, want a --line-test-timeout refusal", args, err)
		}
	}
}

// GLS-V0-020: corvint test --line-index runs the kept tests of a narrowed
// package, drops an excluded one into excluded, and records lineIndex.
func TestCorvintTestLineIndexNarrowsTheRun(t *testing.T) {
	root, base, file := lineFixtureRepository(t)
	lineCommit(t, root, "calc/calc.go", "return n * 2", "return n * 3")
	manifest := filepath.Join(t.TempDir(), "manifest.json")
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"--root", root, "test", "--base", base, "--line-index", file, "--dry-run", "--manifest", manifest}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	var m struct {
		Scope     string   `json:"scope"`
		Excluded  []string `json:"excluded"`
		LineIndex string   `json:"lineIndex"`
		Units     []struct {
			Unit, Flag, Pattern, Slice string
		} `json:"units"`
	}
	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if m.Scope != "NARROWED" || !strings.HasPrefix(m.LineIndex, "SHADOW sha256:") {
		t.Fatalf("scope %s lineIndex %q", m.Scope, m.LineIndex)
	}
	if len(m.Units) != 2 || len(m.Excluded) != 0 {
		t.Fatalf("unqualified manifest units %+v excluded %v", m.Units, m.Excluded)
	}

	// The same dry run reproduces the manifest byte for byte.
	stdout.Reset()
	stderr.Reset()
	if code := runContext(context.Background(), []string{"--root", root, "test", "--base", base, "--line-index", file, "--dry-run", "--manifest", manifest + ".2"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("second exit %d: %s", code, stderr.String())
	}
	again, err := os.ReadFile(manifest + ".2")
	if err != nil || !bytes.Equal(again, data) {
		t.Fatalf("the manifest differs between two dry runs: %v", err)
	}
}

// GLS-V0-011, GLS-V0-020: a selection whose qualification is admitted is
// NARROWED, and corvint test applies it: the user package, which no changed
// line reaches, is excluded from the run. The selection and the run share
// this process's environment, so every binding holds (GLS-V0-018).
func TestLineNarrowingAppliesWhenQualified(t *testing.T) {
	qualify := affectedLineQualify
	affectedLineQualify = func(context.Context, string, string, string, *debug.BuildInfo) error { return nil }
	t.Cleanup(func() { affectedLineQualify = qualify })
	root, base, file := lineFixtureRepository(t)
	lineCommit(t, root, "calc/calc.go", "return n * 2", "return n * 3")
	receipt, _, stderr, code := runAffectedCLI(t, root, "--base", base, "--line-index", file)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	state, units := lineUnits(t, receipt)
	if want := []string{"calc=whole", "user=exclude"}; state != linecov.StateNarrowed || !reflect.DeepEqual(units, want) {
		t.Fatalf("%s %v, want NARROWED %v\nselection %v", state, units, want, receipt["lineSelection"])
	}
	manifest := filepath.Join(t.TempDir(), "manifest.json")
	var stdout, errOut bytes.Buffer
	if code := runContext(context.Background(), []string{"--root", root, "test", "--base", base, "--line-index", file, "--dry-run", "--manifest", manifest}, strings.NewReader(""), &stdout, &errOut); code != 0 {
		t.Fatalf("test exit %d: %s", code, errOut.String())
	}
	var m struct {
		Excluded  []string `json:"excluded"`
		LineIndex string   `json:"lineIndex"`
		Units     []struct{ Unit, Slice string }
	}
	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(m.LineIndex, linecov.StateNarrowed+" sha256:") || len(m.Units) != 1 || len(m.Excluded) != 1 || !strings.Contains(m.Excluded[0], "user") {
		t.Fatalf("lineIndex %q units %+v excluded %+v", m.LineIndex, m.Units, m.Excluded)
	}
}

// GLS-V0-051, GLS-V0-018 (V1-4514): an index collected without FC decides
// nothing for a selection with FC set, which `go env -changed` does not
// report: the runner-unavailable FALLBACK keeps the static plan. A test that
// read CI when the index was collected keeps its package whole when CI
// differs at selection, and the same CI narrows as collected.
func TestAffectedLineFCOrCIChangeKeepsStaticPlan(t *testing.T) {
	qualify := affectedLineQualify
	affectedLineQualify = func(context.Context, string, string, string, *debug.BuildInfo) error { return nil }
	t.Cleanup(func() { affectedLineQualify = qualify })
	t.Setenv("FC", "")
	t.Setenv("CI", "collect")
	root, base, file := lineFixtureRepository(t, map[string]string{"user/user_test.go": `package user

import (
	"os"
	"testing"
)

func TestLabel(t *testing.T) {
	if os.Getenv("CI") == "skip" {
		return
	}
	if Label(1) != "other" {
		t.Fatal("label")
	}
}
`})
	lineCommit(t, root, "calc/calc.go", "return n * 2", "return n * 3")
	plain, _, stderr, code := runAffectedCLI(t, root, "--base", base)
	if code != 0 {
		t.Fatalf("plain exit %d: %s", code, stderr)
	}
	for _, c := range []struct {
		name, fc, ci, want string
		units              []string
	}{
		{name: "as collected", ci: "collect", want: linecov.StateNarrowed, units: []string{"calc=whole", "user=exclude"}},
		{name: "FC set", fc: "gfortran-linecov", ci: "collect", want: "FALLBACK " + linecov.ReasonRunnerUnavailable},
		{name: "CI changed", ci: "skip", want: linecov.StateNarrowed, units: []string{"calc=whole", "user=whole"}},
		{name: "CI unset", want: linecov.StateNarrowed, units: []string{"calc=whole", "user=whole"}},
	} {
		t.Setenv("FC", c.fc)
		t.Setenv("CI", c.ci)
		if c.ci == "" {
			os.Unsetenv("CI")
		}
		receipt, _, stderr, code := runAffectedCLI(t, root, "--base", base, "--line-index", file)
		if code != 0 {
			t.Fatalf("%s: exit %d: %s", c.name, code, stderr)
		}
		if !reflect.DeepEqual(plain["plan"], receipt["plan"]) {
			t.Fatalf("%s: --line-index changed the plan", c.name)
		}
		state, units := lineUnits(t, receipt)
		if state != c.want || len(units) != len(c.units) || len(c.units) > 0 && !reflect.DeepEqual(units, c.units) {
			t.Errorf("%s: %s %v, want %s %v\nselection %v", c.name, state, units, c.want, c.units, receipt["lineSelection"])
		}
		if c.fc != "" {
			if detail, _ := receipt["lineSelection"].(map[string]any)["detail"].(string); !strings.Contains(detail, "FC") {
				t.Errorf("%s: detail %q does not name FC", c.name, detail)
			}
		}
		if c.ci != "collect" {
			for _, u := range receipt["lineSelection"].(map[string]any)["units"].([]any) {
				if m := u.(map[string]any); strings.HasSuffix(m["unit"].(string), "/user") && m["category"] != linecov.CategoryEnvironment {
					t.Errorf("%s: user kept as %v, want %s", c.name, m["category"], linecov.CategoryEnvironment)
				}
			}
		}
	}
}
