package main

import (
	"reflect"
	"strings"
	"testing"
)

// STR-V0-013: argv flags, package sets and environment overlays are read the
// same way for every driver, and each known difference gets its category
// while anything else stays UNEXPLAINED.
func TestSTRV0013ParityReading(t *testing.T) {
	universe := []string{"m/a", "m/b"}
	argv := []string{"test", "-p", "1", "-timeout", "30m", "-count=1", "./..."}
	set := packageSet(argv, universe)
	if !reflect.DeepEqual(sortedKeys(set), universe) {
		t.Fatalf("./... expanded to %v", sortedKeys(set))
	}
	if got := flagsOnly(argv, set); !reflect.DeepEqual(got, []string{"test", "-p", "1", "-timeout", "30m", "-count=1"}) {
		t.Fatalf("flags %v", got)
	}
	narrowed := []string{"test", "-exec", "/c", "-json", "-p", "1", "m/a"}
	if got := sortedKeys(packageSet(narrowed, nil)); !reflect.DeepEqual(got, []string{"m/a"}) {
		t.Fatalf("packages %v", got)
	}
	if got := withoutExec(flagsOnly(narrowed, packageSet(narrowed, nil))); !reflect.DeepEqual(got, []string{"test", "-json", "-p", "1"}) {
		t.Fatalf("without exec %v", got)
	}
	base := []string{"PATH=/bin", "HOME=/h"}
	got := overlay([]string{"PATH=/bin", "HOME=/h", "PWD=/x", "SHLVL=2", "GOCACHE=/c", "GOTOOLCHAIN=local", "GO_TEST_COMMAND=x"}, base)
	if !reflect.DeepEqual(got, []string{"GOCACHE=/c", "GOTOOLCHAIN=local"}) {
		t.Fatalf("overlay %v", got)
	}

	ciEnv := append([]string{"CORVINT_CONFINE_ROOT=/w"}, ciGoEnv...)
	closed := append([]string{"HOME=/r", "PATH=/bin"}, ciGoEnv...)
	local := Plan{Argv: []string{"test", "-json", "-p", "1"}, Count: 1, Env: []string{"A=1"}, Packages: []string{"m/a"}, Scope: "NARROWED"}
	driver := Plan{Argv: []string{"test", "-p", "1"}, Count: 1, Env: []string{"A=1"}, Packages: []string{"m/a"}, Scope: "NARROWED"}
	widened := Plan{Argv: local.Argv, Count: 2, Env: local.Env, Packages: universe, Reason: "plan.unknown entry outside the AFP-V0-028 allowlist: UNINDEXED_SOURCE_PATH x.go", Scope: "FULL"}
	ci := Plan{Argv: []string{"test", "-json"}, Count: 2, Env: closed, Packages: universe, Reason: "qualification unavailable: no trusted qualification pin", Scope: "FULL"}
	corvintCI := Plan{Argv: []string{"test", "-exec", confinePath, "-json"}, Count: 1, Env: ciEnv, Packages: []string{"m/a"}, Scope: "NARROWED"}
	categories := func(pair Pair) string {
		out := []string{}
		for _, difference := range compare(pair) {
			out = append(out, difference.Profile+" "+difference.Field+" "+difference.Category)
		}
		return strings.Join(out, "\n")
	}
	fresh := func() Pair {
		return Pair{CorvintLoc: local, Local: driver, CorvintCI: corvintCI, CI: ci, CIPlanned: "NARROWED", CIPlannedPackages: []string{"m/a"}, Root: "/w", Universe: universe}
	}
	want := strings.Join([]string{
		"local argv local-json-flag",
		"ci admission ci-admission-no-qualification",
		"ci executed ci-admission-no-qualification",
		"ci argv ci-exec-confine",
		"ci env ci-closed-environment",
	}, "\n")
	if got := categories(fresh()); got != want {
		t.Fatalf("categories\ngot  %s\nwant %s", got, want)
	}
	for name, check := range map[string]struct {
		edit func(*Pair)
		want string
	}{
		"unknown widening":         {func(p *Pair) { p.CorvintLoc = widened }, "local packages str-v0-003-unknown-widening"},
		"widening that drops one":  {func(p *Pair) { p.CorvintLoc = widened; p.CorvintLoc.Packages = []string{"m/b"} }, "local packages UNEXPLAINED"},
		"unexplained package":      {func(p *Pair) { p.Local.Packages = []string{"m/b"} }, "local packages UNEXPLAINED"},
		"admission below universe": {func(p *Pair) { p.CI.Packages = []string{"m/a"}; p.CI.Count = 1 }, "ci admission ci-admission-no-qualification"},
		"executed omits a package": {func(p *Pair) { p.CI.Packages = []string{"m/b"} }, "ci executed UNEXPLAINED"},
		"undeclared language": {func(p *Pair) {
			p.CorvintLoc.Error = "INCOMPLETE: typescript: no qualified adapter and no declared typescript-test full command"
		}, "local error str-v0-004-undeclared-language"},
		"another INCOMPLETE":        {func(p *Pair) { p.CorvintLoc.Error = "INCOMPLETE: the manifest exceeds 1048576 bytes" }, "local error UNEXPLAINED"},
		"ci variable added":         {func(p *Pair) { p.CorvintCI.Env = append(append([]string{}, ciEnv...), "GOFLAGS=-mod=mod") }, "ci env UNEXPLAINED"},
		"ci variable dropped":       {func(p *Pair) { p.CorvintCI.Env = []string{"CORVINT_CONFINE_ROOT=/w"} }, "ci env UNEXPLAINED"},
		"confine root elsewhere":    {func(p *Pair) { p.Root = "/other" }, "ci env UNEXPLAINED"},
		"another exec wrapper":      {func(p *Pair) { p.CorvintCI.Argv = []string{"test", "-exec", "/bin/true", "-json"} }, "ci argv UNEXPLAINED"},
		"local environment differs": {func(p *Pair) { p.Local.Env = []string{"A=2"} }, "local env UNEXPLAINED"},
	} {
		pair := fresh()
		check.edit(&pair)
		if got := categories(pair); !strings.Contains(got, check.want) {
			t.Errorf("%s: want %q in\n%s", name, check.want, got)
		}
	}
}
