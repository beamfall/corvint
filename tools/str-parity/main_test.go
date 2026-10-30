package main

import (
	"os"
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

	if got, agrees := invokedPackages([]string{"test", "-json", "m/a"}, []string{"m/a", "m/b"}, universe); agrees || !reflect.DeepEqual(sortedKeys(got), []string{"m/a"}) {
		t.Fatalf("units naming a package argv omits: %v agrees=%v", sortedKeys(got), agrees)
	}
	if got, agrees := invokedPackages([]string{"test", "./..."}, universe, universe); !agrees || len(got) != 2 {
		t.Fatalf("./... with every unit: %v agrees=%v", sortedKeys(got), agrees)
	}

	ciEnv := append([]string{"CORVINT_CONFINE_ROOT=/w"}, ciGoEnv...)
	closed := append(append([]string{}, ciDriverFixed...), ciGoEnv...)
	for _, key := range ciDriverPaths {
		closed = append(closed, key+"=/r/"+key)
	}
	driverEnv := func(drop string, add ...string) []string {
		out := []string{}
		for _, entry := range closed {
			if entry != drop {
				out = append(out, entry)
			}
		}
		return append(out, add...)
	}
	local := Plan{Argv: []string{"test", "-json", "-p", "1"}, Count: 1, Env: []string{"A=1"}, Packages: []string{"m/a"}, Scope: "NARROWED"}
	driver := Plan{Argv: []string{"test", "-p", "1"}, Count: 1, Env: []string{"A=1"}, Packages: []string{"m/a"}, Scope: "NARROWED"}
	widened := Plan{Argv: local.Argv, Count: 2, Env: local.Env, Packages: universe, Reason: "plan.unknown entry outside the AFP-V0-028 allowlist: UNINDEXED_SOURCE_PATH x.go", Scope: "FULL"}
	ci := Plan{Argv: []string{"test", "-json"}, Count: 2, Env: closed, Packages: universe, Reason: "qualification unavailable: no trusted qualification pin", Scope: "FULL"}
	corvintCI := Plan{Argv: []string{"test", "-exec", confinePath, "-json"}, Count: 1, Env: ciEnv, Packages: []string{"m/a"}, Scope: "NARROWED"}
	incomplete := func(plan *Plan, reasons ...string) {
		plan.Incomplete, plan.Error = reasons, "INCOMPLETE: "+strings.Join(reasons, "; ")
	}
	const typescript = "typescript: no qualified adapter and .github/cishards/test-commands.json declares no typescript full command"
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
		"admission below universe": {func(p *Pair) { p.CI.Packages = []string{"m/a"}; p.CI.Count = 1 }, "ci admission UNEXPLAINED"},
		"executed omits a package": {func(p *Pair) { p.CI.Packages = []string{"m/b"} }, "ci executed UNEXPLAINED"},
		"undeclared language":      {func(p *Pair) { incomplete(&p.CorvintLoc, typescript) }, "local error str-v0-004-undeclared-language"},
		"another INCOMPLETE":       {func(p *Pair) { incomplete(&p.CorvintLoc, "the manifest exceeds 1048576 bytes") }, "local error UNEXPLAINED"},
		"look-alike reason": {func(p *Pair) {
			incomplete(&p.CorvintLoc, "typescript: no qualified adapter and .github/cishards/test-commands.json declares no ruby full command")
		}, "local error UNEXPLAINED"},
		"error beyond reasons":      {func(p *Pair) { incomplete(&p.CorvintLoc, typescript); p.CorvintLoc.Error += " exit 1" }, "local error UNEXPLAINED"},
		"units mismatch":            {func(p *Pair) { p.CorvintLoc.Mismatch = "arguments name [m/a], units name [m/a m/b]" }, "local units UNEXPLAINED"},
		"ci variable added":         {func(p *Pair) { p.CorvintCI.Env = append(append([]string{}, ciEnv...), "GOFLAGS=-mod=mod") }, "ci env UNEXPLAINED"},
		"ci variable dropped":       {func(p *Pair) { p.CorvintCI.Env = []string{"CORVINT_CONFINE_ROOT=/w"} }, "ci env UNEXPLAINED"},
		"confine root elsewhere":    {func(p *Pair) { p.Root = "/other" }, "ci env UNEXPLAINED"},
		"driver drops GOENV":        {func(p *Pair) { p.CI.Env = driverEnv("GOENV=off") }, "ci env UNEXPLAINED"},
		"driver changes GOMAXPROCS": {func(p *Pair) { p.CI.Env = driverEnv("GOMAXPROCS=2", "GOMAXPROCS=8") }, "ci env UNEXPLAINED"},
		"driver adds GODEBUG":       {func(p *Pair) { p.CI.Env = driverEnv("", "GODEBUG=x=1") }, "ci env UNEXPLAINED"},
		"driver drops a path":       {func(p *Pair) { p.CI.Env = driverEnv("HOME=/r/HOME") }, "ci env UNEXPLAINED"},
		"driver path elsewhere":     {func(p *Pair) { p.CI.Env = driverEnv("HOME=/r/HOME", "HOME=/elsewhere") }, "ci env ci-closed-environment"},
		"another exec wrapper":      {func(p *Pair) { p.CorvintCI.Argv = []string{"test", "-exec", "/bin/true", "-json"} }, "ci argv UNEXPLAINED"},
		"local environment differs": {func(p *Pair) { p.Local.Env = []string{"A=2"} }, "local env UNEXPLAINED"},
		"incomplete hides a dropped ci package": {func(p *Pair) {
			p.CorvintCI = Plan{Argv: corvintCI.Argv, Count: 2, Env: ciEnv, Packages: universe, Scope: "FULL"}
			incomplete(&p.CorvintCI, typescript)
			p.CIPlanned, p.CIPlannedPackages, p.CI.Packages = "FULL", universe, []string{"m/a"}
		}, "ci executed UNEXPLAINED"},
		"declared command": {func(p *Pair) {
			declaredCommands = []string{"make host-adapter-test"}
			p.CorvintCI.Declared = []string{"make host-adapter-test"}
		}, "ci declared str-v0-004-declared-language"},
		"declared and undeclared commands": {func(p *Pair) {
			declaredCommands = []string{"make host-adapter-test"}
			p.CorvintLoc.Declared = []string{"make host-adapter-test", "make typescript-test"}
		}, "local declared UNEXPLAINED"},
		"undeclared command": {func(p *Pair) { declaredCommands = nil; p.CorvintCI.Declared = []string{"make typescript-test"} }, "ci declared UNEXPLAINED"},
	} {
		pair := fresh()
		check.edit(&pair)
		if got := categories(pair); !strings.Contains(got, check.want) {
			t.Errorf("%s: want %q in\n%s", name, check.want, got)
		}
	}
	// An undeclared-language INCOMPLETE plan whose go invocation omits a unit
	// keeps the mismatch as its own UNEXPLAINED difference.
	both := fresh()
	incomplete(&both.CorvintLoc, typescript)
	both.CorvintLoc.Mismatch = "arguments name [m/a], units name [m/a m/b]"
	joined := categories(both)
	for _, want := range []string{"local units UNEXPLAINED", "local error str-v0-004-undeclared-language"} {
		if !strings.Contains(joined, want) {
			t.Errorf("undeclared language with a units mismatch: want %q in\n%s", want, joined)
		}
	}
}

// STR-V0-013: the retained corpus is read as newest-first OID triples and
// anything else is refused, so parity never runs over a guessed pair list.
func TestSTRV0013CorpusReading(t *testing.T) {
	a, b, c := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)
	got, err := readCorpus([]byte("# comment\n\n" + a + "  " + b + " " + c + "\n" + b + " " + c + " " + a + "\n"))
	if err != nil || !reflect.DeepEqual(got, []string{a + " " + b + " " + c, b + " " + c + " " + a}) {
		t.Fatalf("corpus %v err %v", got, err)
	}
	for name, raw := range map[string]string{
		"empty":        "# only a comment\n",
		"short oid":    a[:39] + " " + b + " " + c + "\n",
		"two fields":   a + " " + b + "\n",
		"four fields":  a + " " + b + " " + c + " " + a + "\n",
		"repeated":     a + " " + b + " " + c + "\n" + a + " " + c + " " + b + "\n",
		"uppercase":    strings.ToUpper(a) + " " + b + " " + c + "\n",
		"trailing tag": a + " " + b + " " + c + " # merge\n",
	} {
		if _, err := readCorpus([]byte(raw)); err == nil {
			t.Errorf("%s: accepted %q", name, raw)
		}
	}
	committed, err := os.ReadFile("testdata/pairs-645304a2.txt")
	if err != nil {
		t.Fatal(err)
	}
	pairs, err := readCorpus(committed)
	if err != nil || len(pairs) != 50 || !strings.HasPrefix(pairs[0], "645304a2fb71065f9061d8d8d866600d703e84c6 ") {
		t.Fatalf("retained corpus: %d pairs, err %v", len(pairs), err)
	}
}
