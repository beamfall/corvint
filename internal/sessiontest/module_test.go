package sessiontest

import (
	"reflect"
	"strings"
	"testing"
)

// STR-V0-064: an AFP-V0-650 unobserved-module frontier is admitted on the
// local profile only by a nested-module unit that runs the module's own
// go test ./... from its directory; the root selection is unchanged, the ci
// profile and every malformed or non-runnable detail stay FULL, and a FULL
// run still carries the module unit.
func TestSTRV0064NestedModuleUnit(t *testing.T) {
	module := func(detail string) Unknown { return Unknown{Reason: "LANGUAGE_FRONTIER", Detail: detail} }
	full := []string{"0 go:example.test/m/a whole", "0 go:example.test/m/b whole", "0 go:example.test/m/c whole"}
	cases := []struct {
		name   string
		edit   func(*Input)
		scope  string
		reason string
		units  []string
	}{
		{name: "one module narrows", edit: func(in *Input) {
			in.Unknown = []Unknown{module("go:unobserved-module:sub/mod:sub/mod/x.go")}
		}, scope: ScopeNarrowed, units: []string{"0 go:example.test/m/a whole", "0 go-module:sub/mod whole"}},
		{name: "one unit per module, sorted", edit: func(in *Input) {
			in.Unknown = []Unknown{
				module("go:unobserved-module:z/mod:z/mod/a.go"),
				module("go:unobserved-module:sub/mod:sub/mod/x.go"),
				module("go:unobserved-module:sub/mod:sub/mod/deep/y_test.go"),
			}
		}, scope: ScopeNarrowed, units: []string{"0 go:example.test/m/a whole", "0 go-module:sub/mod whole", "0 go-module:z/mod whole"}},
		{name: "a module-only change is not NOTHING", edit: func(in *Input) {
			in.GoSelection = "NOTHING no Go package attributed"
			in.Unknown = []Unknown{module("go:unobserved-module:sub/mod:sub/mod/x.go")}
		}, scope: ScopeNarrowed, units: []string{"0 go-module:sub/mod whole"}},
		{name: "a directory with a colon", edit: func(in *Input) {
			in.Unknown = []Unknown{module("go:unobserved-module:a:b:a:b/x.go")}
		}, scope: ScopeNarrowed, units: []string{"0 go:example.test/m/a whole", "0 go-module:a:b whole"}},
		{name: "FULL still runs the module", edit: func(in *Input) {
			in.Unknown = []Unknown{module("go:unobserved-module:sub/mod:sub/mod/x.go"), {Reason: "UNINDEXED_SOURCE_PATH", Detail: "x/y.go"}}
		}, scope: ScopeFull, reason: "plan.unknown entry outside the AFP-V0-028 allowlist: UNINDEXED_SOURCE_PATH x/y.go", units: append(append([]string{}, full...), "0 go-module:sub/mod whole")},
		{name: "ci profile keeps FULL", edit: func(in *Input) {
			in.RunnerProfile = ProfileCI
			in.Unknown = []Unknown{module("go:unobserved-module:sub/mod:sub/mod/x.go")}
		}, scope: ScopeFull, reason: "plan.unknown entry outside the AFP-V0-028 allowlist: LANGUAGE_FRONTIER go:unobserved-module:sub/mod:sub/mod/x.go", units: full},
		{name: "a non-runnable module keeps FULL", edit: func(in *Input) {
			in.Unknown = []Unknown{module("go:unobserved-directory:sub/mod:sub/mod/x.go")}
		}, scope: ScopeFull, reason: "plan.unknown entry outside the AFP-V0-028 allowlist: LANGUAGE_FRONTIER go:unobserved-directory:sub/mod:sub/mod/x.go", units: full},
	}
	for _, detail := range []string{
		"go:unobserved-module:../x:../x/y.go",
		"go:unobserved-module:.:./x.go",
		"go:unobserved-module:/abs:/abs/x.go",
		"go:unobserved-module:a/./b:a/./b/x.go",
		`go:unobserved-module:a\b:a\b/x.go`,
		"go:unobserved-module:a:b/x.go",
		"go:unobserved-module:a",
		"go:unobserved-module::x.go",
	} {
		cases = append(cases, struct {
			name   string
			edit   func(*Input)
			scope  string
			reason string
			units  []string
		}{name: "malformed " + detail, edit: func(in *Input) { in.Unknown = []Unknown{module(detail)} },
			scope: ScopeFull, reason: "plan.unknown entry outside the AFP-V0-028 allowlist: LANGUAGE_FRONTIER " + detail, units: full})
	}
	cases = append(cases, struct {
		name   string
		edit   func(*Input)
		scope  string
		reason string
		units  []string
	}{name: "wrong reason", edit: func(in *Input) {
		in.Unknown = []Unknown{{Reason: "UNINDEXED_SOURCE_PATH", Detail: "go:unobserved-module:a:a/x.go"}}
	}, scope: ScopeFull, reason: "plan.unknown entry outside the AFP-V0-028 allowlist: UNINDEXED_SOURCE_PATH go:unobserved-module:a:a/x.go", units: full})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := fixtureInput()
			tc.edit(&input)
			m, err := Build(input)
			if err != nil {
				t.Fatal(err)
			}
			if m.Scope != tc.scope || m.Reason != tc.reason || !reflect.DeepEqual(unitIDs(m), tc.units) {
				t.Fatalf("got scope=%s reason=%q units=%v", m.Scope, m.Reason, unitIDs(m))
			}
			for _, unit := range m.Units {
				if dir, isModule := strings.CutPrefix(unit.Unit, ModuleUnitPrefix); isModule && (unit.Dir != dir || unit.Reason != moduleReason || unit.Adapter != AdapterGo) {
					t.Fatalf("module unit %+v", unit)
				}
			}
		})
	}
}

// STR-V0-064: a nested-module unit is its own invocation, never part of the
// root batch: the declared Go flags over ./..., the declared environment plus
// GOWORK=off, and the module directory.
func TestSTRV0064ModuleInvocation(t *testing.T) {
	moduleUnit := Unit{Adapter: AdapterGo, Dir: "sub/mod", Reason: moduleReason, Shard: 0, Slice: SliceWhole, Unit: ModuleUnitPrefix + "sub/mod"}
	m := Manifest{RunnerProfile: ProfileLocal, Units: []Unit{
		{Adapter: AdapterGo, Shard: 0, Slice: SliceWhole, Unit: "go:example.test/m/a"},
		moduleUnit,
	}}
	got := Invocations(m, parsedDeclared(), 0, "/repo", "")
	want := []Planned{
		{Adapter: AdapterGo, Argv: []string{"go", "test", "-json", "-p", "1", "-timeout", "30m", "-count=1", "example.test/m/a"}, Env: []string{"GOTOOLCHAIN=local"}, Units: m.Units[:1]},
		{Adapter: AdapterGo, Argv: []string{"go", "test", "-json", "-p", "1", "-timeout", "30m", "-count=1", "./..."}, Dir: "sub/mod", Env: []string{"GOTOOLCHAIN=local", "GOWORK=off"}, Units: m.Units[1:]},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %#v\nwant %#v", got, want)
	}
	if runnerKey(got[0]) == runnerKey(got[1]) {
		t.Fatal("the module invocation shares the root runner discovery")
	}
}

// STR-V0-064: a module unit's outcome folds its packages: the first failing
// outcome, else FAIL on a nonzero exit, else PASS or NO_TESTS.
func TestSTRV0064ModuleOutcome(t *testing.T) {
	for _, tc := range []struct {
		packages map[string]string
		exit     int
		want     string
	}{
		{map[string]string{"m/a": OutcomePass, "m/b": OutcomeNoTests}, 0, OutcomePass},
		{map[string]string{"m/a": OutcomePass, "m/b": OutcomeFail}, 1, OutcomeFail},
		{map[string]string{"m/a": OutcomeFail, "m/b": OutcomeBuildFailed}, 1, OutcomeBuildFailed},
		{map[string]string{"m/a": OutcomePass, "m/b": OutcomeTimedOut}, 1, OutcomeTimedOut},
		{map[string]string{"m/a": OutcomePass}, 2, OutcomeFail},
		{map[string]string{}, 1, OutcomeFail},
		{map[string]string{}, 0, OutcomeNoTests},
	} {
		if got := moduleOutcome(tc.packages, tc.exit); got != tc.want {
			t.Errorf("moduleOutcome(%v, %d) = %s, want %s", tc.packages, tc.exit, got, tc.want)
		}
	}
}

// STR-V0-064: only a non-empty dirty set wholly named by well-formed
// unobserved-module frontiers is covered.
func TestSTRV0064ModuleCovered(t *testing.T) {
	module := func(detail string) Unknown { return Unknown{Reason: "LANGUAGE_FRONTIER", Detail: detail} }
	unknown := []Unknown{module("go:unobserved-module:sub/mod:sub/mod/x.go"), module("go:unobserved-module:../x:../x/y.go"), module("go:unobserved-directory:d:d/z.go")}
	for _, tc := range []struct {
		dirty []string
		want  bool
	}{
		{[]string{"sub/mod/x.go"}, true},
		{[]string{"sub/mod/x.go", "a/a.go"}, false},
		{[]string{"../x/y.go"}, false},
		{[]string{"d/z.go"}, false},
		{[]string{}, false},
	} {
		if got := ModuleCovered(tc.dirty, unknown); got != tc.want {
			t.Errorf("ModuleCovered(%v) = %v, want %v", tc.dirty, got, tc.want)
		}
	}
}
