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

	local := Plan{Argv: []string{"test", "-json", "-p", "1"}, Count: 1, Env: []string{"A=1"}, Packages: []string{"m/a"}, Scope: "NARROWED"}
	driver := Plan{Argv: []string{"test", "-p", "1"}, Count: 1, Env: []string{"A=1"}, Packages: []string{"m/a"}, Scope: "NARROWED"}
	widened := Plan{Argv: local.Argv, Count: 2, Env: local.Env, Packages: universe, Reason: "plan.unknown entry outside the AFP-V0-028 allowlist: UNINDEXED_SOURCE_PATH x.go", Scope: "FULL"}
	ci := Plan{Argv: []string{"test", "-json"}, Count: 1, Env: []string{"B=2", "C=3", "HOME=/r"}, Packages: []string{"m/a"}, Reason: "qualification unavailable: no trusted qualification pin", Scope: "FULL"}
	corvintCI := Plan{Argv: []string{"test", "-exec", "/c", "-json"}, Count: 1, Env: []string{"C=3", "CORVINT_CONFINE_ROOT=/w"}, Packages: []string{"m/a"}, Scope: "NARROWED"}
	categories := func(pair Pair) []string {
		out := []string{}
		for _, difference := range compare(pair) {
			out = append(out, difference.Profile+" "+difference.Field+" "+difference.Category)
		}
		return out
	}
	pair := Pair{CorvintLoc: local, Local: driver, CorvintCI: corvintCI, CI: ci, CIPlanned: "NARROWED"}
	want := []string{
		"local argv local-json-flag",
		"ci admission ci-admission-no-qualification",
		"ci argv ci-exec-confine",
		"ci env ci-closed-environment",
	}
	if got := categories(pair); !reflect.DeepEqual(got, want) {
		t.Fatalf("categories\ngot  %v\nwant %v", got, want)
	}
	pair.CorvintCI.Env = append(pair.CorvintCI.Env, "GOFLAGS=-mod=mod")
	if got := strings.Join(categories(pair), "\n"); !strings.Contains(got, "ci env UNEXPLAINED") {
		t.Fatalf("a variable outside the closed environment was categorised: %s", got)
	}
	pair.CorvintCI = corvintCI
	pair.CorvintLoc = widened
	if got := strings.Join(categories(pair), "\n"); !strings.Contains(got, "local verdict str-v0-003-unknown-widening") || !strings.Contains(got, "local packages str-v0-003-unknown-widening") {
		t.Fatalf("widening %s", got)
	}
	pair.CorvintLoc = local
	pair.Local.Packages = []string{"m/b"}
	if got := strings.Join(categories(pair), "\n"); !strings.Contains(got, "local packages UNEXPLAINED") {
		t.Fatalf("an unexplained package difference was categorised: %s", got)
	}
	pair.Local = driver
	pair.CorvintLoc.Error = "INCOMPLETE: typescript: no qualified adapter and no declared typescript-test full command"
	if got := strings.Join(categories(pair), "\n"); !strings.Contains(got, "local error str-v0-004-undeclared-language") {
		t.Fatalf("undeclared language %s", got)
	}
	pair.CorvintLoc.Error = "INCOMPLETE: the manifest exceeds 1048576 bytes"
	if got := strings.Join(categories(pair), "\n"); !strings.Contains(got, "local error UNEXPLAINED") {
		t.Fatalf("another INCOMPLETE was categorised: %s", got)
	}
}
