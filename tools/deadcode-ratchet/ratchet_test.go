// SPDX-License-Identifier: Apache-2.0

package main

import (
	"embed"
	"slices"
	"strings"
	"testing"
)

// The tests read only these embedded fixtures: they never run the analyzer,
// load the module or use the network (DCR-V0-006).
//
//go:embed testdata
var fixtures embed.FS

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := fixtures.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func parsed(t *testing.T, name string) run {
	t.Helper()
	r, err := parseRun(fixture(t, name))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return r
}

// sixRuns gives linux and darwin the unix fixture and windows the windows
// fixture, which adds a function only windows reports.
func sixRuns(t *testing.T) map[config]run {
	t.Helper()
	unix, windows := parsed(t, "run-unix.json"), parsed(t, "run-windows.json")
	runs := map[config]run{}
	for _, c := range configs {
		if c.GOOS == "windows" {
			runs[c] = windows
		} else {
			runs[c] = unix
		}
	}
	return runs
}

func report(t *testing.T, runs map[config]run) run {
	t.Helper()
	r, err := intersect(runs)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func ids(r run) []string {
	var out []string
	for _, id := range sortedIDs(r) {
		out = append(out, id.String())
	}
	return out
}

func baselineFixture(t *testing.T, name string) []baselineEntry {
	t.Helper()
	b, err := decodeBaseline(fixture(t, name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// DCR-V0-003, V1-4801 criterion 2: a function only the windows runs report
// is absent from the report; generated and marker functions are never in it.
func TestReportExcludesFunctionDeadOnOneOS(t *testing.T) {
	runs := sixRuns(t)
	if _, ok := runs[config{"windows", "amd64"}][identity{"example.com/m/a", "posixOnly"}]; !ok {
		t.Fatal("fixture: the windows run must report posixOnly")
	}
	got := ids(report(t, runs))
	want := []string{"example.com/m/a T.Method", "example.com/m/a Unused", "example.com/m/b helper"}
	if !slices.Equal(got, want) {
		t.Fatalf("report = %q, want %q", got, want)
	}
	for _, name := range []string{"Gen", "T.isMarker"} {
		if _, ok := runs[config{"windows", "arm64"}][identity{"example.com/m/a", name}]; ok {
			t.Errorf("parseRun kept %s, which is generated or a marker", name)
		}
	}
}

// DCR-V0-005: the report equals the baseline, so the verdict is empty.
func TestVerdictAcceptsMatchingBaseline(t *testing.T) {
	if lines := verdict(report(t, sixRuns(t)), baselineFixture(t, "baseline-ok.json"), "b.json"); len(lines) != 0 {
		t.Fatalf("verdict = %q, want none", lines)
	}
}

// DCR-V0-005, V1-4801 criterion 1: a function in every run that the baseline
// lacks fails with its package, name and position.
func TestVerdictNamesNewFunction(t *testing.T) {
	runs := sixRuns(t)
	added := identity{"example.com/m/b", "NewlyDead"}
	for _, c := range configs {
		r := run{}
		for id, pos := range runs[c] {
			r[id] = pos
		}
		r[added] = "b/new.go:3:6"
		runs[c] = r
	}
	lines := verdict(report(t, runs), baselineFixture(t, "baseline-ok.json"), "b.json")
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "NEW_DEAD_FUNCTION example.com/m/b.NewlyDead at b/new.go:3:6:") {
		t.Fatalf("verdict = %q, want one NEW_DEAD_FUNCTION naming example.com/m/b.NewlyDead at b/new.go:3:6", lines)
	}
}

// DCR-V0-005: a baseline entry the report lacks fails and says to delete it,
// so the baseline can only shrink.
func TestVerdictNamesStaleEntry(t *testing.T) {
	runs := sixRuns(t)
	gone := identity{"example.com/m/b", "helper"}
	for _, c := range configs {
		r := run{}
		for id, pos := range runs[c] {
			if id != gone {
				r[id] = pos
			}
		}
		runs[c] = r
	}
	lines := verdict(report(t, runs), baselineFixture(t, "baseline-ok.json"), "b.json")
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "STALE_BASELINE_ENTRY example.com/m/b.helper: no longer reported; delete its entry from b.json") {
		t.Fatalf("verdict = %q, want one STALE_BASELINE_ENTRY naming example.com/m/b.helper", lines)
	}
}

// DCR-V0-003: moving a function inside its package changes its position,
// not its identity, so the verdict does not change.
func TestMovedFunctionKeepsIdentity(t *testing.T) {
	runs := sixRuns(t)
	moved := identity{"example.com/m/a", "Unused"}
	for _, c := range configs {
		r := run{}
		for id, pos := range runs[c] {
			r[id] = pos
		}
		r[moved] = "a/other.go:99:6"
		runs[c] = r
	}
	if lines := verdict(report(t, runs), baselineFixture(t, "baseline-ok.json"), "b.json"); len(lines) != 0 {
		t.Fatalf("verdict after a move = %q, want none", lines)
	}
}

// DCR-V0-002: a report is never computed from fewer than six runs, and the
// refusal names the missing configuration. An empty run is still a run.
func TestIntersectRefusesMissingRun(t *testing.T) {
	runs := sixRuns(t)
	delete(runs, config{"windows", "arm64"})
	_, err := intersect(runs)
	if err == nil || !strings.Contains(err.Error(), "windows/arm64") {
		t.Fatalf("intersect without windows/arm64: err = %v, want one naming windows/arm64", err)
	}
	runs[config{"windows", "arm64"}] = run{}
	if r, err := intersect(runs); err != nil || len(r) != 0 {
		t.Fatalf("intersect with an empty windows/arm64 run = %v, %v; want an empty report", r, err)
	}
	runs[config{"plan9", "amd64"}] = run{}
	if _, err := intersect(runs); err == nil || !strings.Contains(err.Error(), "plan9/amd64") {
		t.Fatalf("intersect with an extra run: err = %v, want one naming plan9/amd64", err)
	}
}

// DCR-V0-004: each kind of malformed baseline is refused with a message that
// names what is wrong.
func TestDecodeBaselineRefusals(t *testing.T) {
	for _, tc := range []struct{ file, want string }{
		{"baseline-unsorted.json", "entry 2 is out of order: example.com/m/a T.Method"},
		{"baseline-duplicate.json", "duplicate entry 2: example.com/m/a T.Method"},
		{"baseline-unknown-field.json", `unknown field "line"`},
		{"baseline-wrong-profile.json", `profile "corvint-deadcode-baseline/1"`},
	} {
		_, err := decodeBaseline(fixture(t, tc.file))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want one containing %q", tc.file, err, tc.want)
		}
	}
	for name, data := range map[string]string{
		"missing list":   `{"profile":"corvint-deadcode-baseline/0"}`,
		"empty func":     `{"profile":"corvint-deadcode-baseline/0","functions":[{"package":"p","func":"","reason":""}]}`,
		"trailing value": `{"profile":"corvint-deadcode-baseline/0","functions":[]} {}`,
	} {
		if _, err := decodeBaseline([]byte(data)); err == nil {
			t.Errorf("%s: decodeBaseline accepted %s", name, data)
		}
	}
}

// encodeBaseline writes a sorted baseline the strict decoder accepts, keeps
// the reasons of entries still reported and drops the others.
func TestEncodeBaselineRoundTrip(t *testing.T) {
	old := append(baselineFixture(t, "baseline-ok.json"), baselineEntry{"example.com/m/z", "Gone", "kept for a reason"})
	data := encodeBaseline(report(t, sixRuns(t)), old)
	got, err := decodeBaseline(data)
	if err != nil {
		t.Fatalf("decode of encoded baseline: %v\n%s", err, data)
	}
	if want := baselineFixture(t, "baseline-ok.json"); !slices.Equal(got, want) {
		t.Fatalf("round trip = %v, want %v", got, want)
	}
	if empty, err := decodeBaseline(encodeBaseline(run{}, nil)); err != nil || len(empty) != 0 {
		t.Fatalf("empty baseline round trip = %v, %v", empty, err)
	}
}

func TestParseRunRefusesMalformedOutput(t *testing.T) {
	for _, data := range []string{``, `{}`, `[{"Path":"","Funcs":[]}]`, `[{"Path":"p","Funcs":[{"Name":""}]}]`, `[] []`} {
		if _, err := parseRun([]byte(data)); err == nil {
			t.Errorf("parseRun accepted %q", data)
		}
	}
	if r, err := parseRun([]byte("null\n")); err != nil || len(r) != 0 {
		t.Errorf("parseRun(null) = %v, %v; want an empty run", r, err)
	}
}

// DCR-V0-001: only an exact version is accepted.
func TestValidVersion(t *testing.T) {
	for _, v := range []string{"v0.52.0", "v1.2.3", "v0.0.0-20260707181000-a299dadba899", "v0.47.1-0.20260707181000-a299dadba899"} {
		if !validVersion(v) {
			t.Errorf("validVersion(%q) = false", v)
		}
	}
	for _, v := range []string{"", "latest", "master", "v0.52", "0.52.0", "v0.52.x", "upgrade", "v1.2.3@latest", ">=v1.0.0"} {
		if validVersion(v) {
			t.Errorf("validVersion(%q) = true", v)
		}
	}
}
