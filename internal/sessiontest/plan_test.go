package sessiontest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/.github/cishards"
)

func parsedDeclared() Declared {
	return Declared{
		GoEnv: []string{"GOTOOLCHAIN=local"}, GoFlags: []string{"-p", "1", "-timeout", "30m", "-count=1"},
		GoTest: GoTestParsed, Languages: []string{},
	}
}

func fixtureInput() Input {
	return Input{
		Base: strings.Repeat("a", 40), Declared: parsedDeclared(), GoSelection: "run example.test/m/a",
		GraphDigest: "sha256:graph", PlanDigest: "sha256:plan", RunnerProfile: ProfileLocal, ShardCount: 1,
		Source:   SourceIdentity{Head: strings.Repeat("b", 40), Paths: []PathDigest{}, Tree: strings.Repeat("c", 40)},
		Universe: []string{"example.test/m/a", "example.test/m/b", "example.test/m/c"},
	}
}

func unitIDs(m Manifest) []string {
	ids := []string{}
	for _, unit := range m.Units {
		ids = append(ids, fmt.Sprintf("%d %s %s", unit.Shard, unit.Unit, unit.Slice))
	}
	return ids
}

// STR-V0-002, STR-V0-003, STR-V0-004: one fixture per verdict, each asserting
// the exact scope, reason, units and incompleteness.
func TestSTRV0003BuildVerdicts(t *testing.T) {
	whole := func(pkgs ...string) []string {
		ids := []string{}
		for _, pkg := range pkgs {
			ids = append(ids, "0 go:example.test/m/"+pkg+" whole")
		}
		return ids
	}
	cases := []struct {
		name       string
		edit       func(*Input)
		scope      string
		reason     string
		units      []string
		incomplete []string
	}{
		{name: "narrowed", edit: func(*Input) {}, scope: ScopeNarrowed, units: whole("a")},
		{name: "allowlisted unknowns stay narrowed", edit: func(in *Input) {
			in.Unknown = []Unknown{{Reason: "UNOWNED_DIRTY_PATH", Detail: "notes.txt"}, {Reason: "LANGUAGE_FRONTIER", Detail: "go:build-constraint-variants"}, {Reason: "NO_SELECTABLE_TEST", Detail: "go:example.test/m/b"}}
		}, scope: ScopeNarrowed, units: whole("a")},
		{name: "attribution fallback", edit: func(in *Input) { in.GoSelection = "FALLBACK go.mod changed" },
			scope: ScopeFull, reason: "go.mod changed", units: whole("a", "b", "c")},
		{name: "unknown outside the allowlist", edit: func(in *Input) {
			in.Unknown = []Unknown{{Reason: "UNINDEXED_SOURCE_PATH", Detail: "x/y.go"}}
		}, scope: ScopeFull, reason: "plan.unknown entry outside the AFP-V0-028 allowlist: UNINDEXED_SOURCE_PATH x/y.go", units: whole("a", "b", "c")},
		{name: "module-level go frontier", edit: func(in *Input) {
			in.Unknown = []Unknown{{Reason: "LANGUAGE_FRONTIER", Detail: "go:module-replace"}}
		}, scope: ScopeFull, reason: "plan.unknown entry outside the AFP-V0-028 allowlist: LANGUAGE_FRONTIER go:module-replace", units: whole("a", "b", "c")},
		{name: "selected package outside the universe", edit: func(in *Input) { in.GoSelection = "run example.test/m/z" },
			scope: ScopeFull, reason: "selected package outside the Go universe: example.test/m/z", units: whole("a", "b", "c")},
		{name: "no attribution verdict", edit: func(in *Input) { in.GoSelection = "" },
			scope: ScopeFull, reason: "no AFP-V0-012 attribution verdict", units: whole("a", "b", "c")},
		{name: "nothing", edit: func(in *Input) { in.GoSelection = "NOTHING no Go package attributed" },
			scope: ScopeNothing, reason: "no Go package attributed", units: []string{}},
		{name: "no plan runs the declared scope", edit: func(in *Input) {
			in.PlanError = "unsupported-affected-graph: boom"
			in.Declared.Languages = []string{"rust"}
		}, scope: ScopeFull, reason: "no plan: unsupported-affected-graph: boom", units: []string{"0 declared:go-test whole", "0 declared:rust-test whole"}},
		{name: "no plan and nothing declared", edit: func(in *Input) {
			in.PlanError = "base unresolved"
			in.Declared = Declared{GoEnv: []string{}, GoFlags: []string{}, GoTest: GoTestAbsent, Languages: []string{}}
		}, scope: ScopeFull, reason: "no plan: base unresolved", units: []string{}, incomplete: []string{"the repository declares no full-suite command"}},
		{name: "no plan and an unreadable declaration", edit: func(in *Input) {
			in.PlanError = "base unresolved"
			in.Declared.Unreadable = "Makefile is unreadable"
		}, scope: ScopeFull, reason: "no plan: base unresolved", units: []string{}, incomplete: []string{"the declared full scope is unreadable: Makefile is unreadable"}},
		{name: "language without an adapter runs its declared command", edit: func(in *Input) {
			in.Selected = []string{"rust:crate"}
			in.Declared.Languages = []string{"rust"}
		}, scope: ScopeNarrowed, units: append(whole("a"), "0 declared:rust-test whole")},
		{name: "allowlisted non-Go frontier names no unit", edit: func(in *Input) {
			in.GoSelection = "NOTHING none"
			in.Unknown = []Unknown{{Reason: "LANGUAGE_FRONTIER", Detail: "swift:conditional-compilation"}}
		}, scope: ScopeNothing, reason: "none", units: []string{}},
		{name: "selected unit with no adapter and no declaration", edit: func(in *Input) {
			in.GoSelection = "NOTHING none"
			in.Selected = []string{"swift:Sources/App"}
		}, scope: ScopeNarrowed, units: []string{}, incomplete: []string{"swift: no qualified adapter and no declared swift-test full command"}},
		{name: "unknown-only non-Go unit runs its declared command", edit: func(in *Input) {
			in.GoSelection = "NOTHING none"
			in.Unknown = []Unknown{{Reason: "NO_SELECTABLE_TEST", Detail: "python:pkg/mod.py"}}
			in.Declared.Languages = []string{"python"}
		}, scope: ScopeNarrowed, units: []string{"0 declared:python-test whole"}},
		{name: "unknown-only non-Go unit with no declaration", edit: func(in *Input) {
			in.GoSelection = "NOTHING none"
			in.Unknown = []Unknown{{Reason: "NO_SELECTABLE_TEST", Detail: "python:pkg/mod.py"}}
		}, scope: ScopeNarrowed, units: []string{}, incomplete: []string{"python: no qualified adapter and no declared python-test full command"}},
		{name: "FULL still covers a selected non-Go language", edit: func(in *Input) {
			in.GoSelection = "FALLBACK go.mod changed"
			in.Selected = []string{"python:pkg/mod.py"}
			in.Declared.Languages = []string{"python"}
		}, scope: ScopeFull, reason: "go.mod changed", units: []string{"0 go:example.test/m/a whole", "0 go:example.test/m/b whole", "0 go:example.test/m/c whole", "0 declared:python-test whole"}},
		{name: "FULL from a path-only fallback runs every declared language", edit: func(in *Input) {
			in.GoSelection = "FALLBACK UNINDEXED_SOURCE_PATH pkg/gone.py"
			in.Declared.Languages = []string{"python"}
		}, scope: ScopeFull, reason: "UNINDEXED_SOURCE_PATH pkg/gone.py", units: []string{"0 go:example.test/m/a whole", "0 go:example.test/m/b whole", "0 go:example.test/m/c whole", "0 declared:python-test whole"}},
		{name: "FULL covers an undeclared language owning a dirty path", edit: func(in *Input) {
			in.GoSelection = "FALLBACK UNINDEXED_SOURCE_PATH pkg/gone.py"
			in.DirtyLanguages = []string{"python"}
		}, scope: ScopeFull, reason: "UNINDEXED_SOURCE_PATH pkg/gone.py", units: whole("a", "b", "c"), incomplete: []string{"python: no qualified adapter and no declared python-test full command"}},
		{name: "ci profile refuses make go-test with no plan", edit: func(in *Input) {
			in.PlanError = "base unresolved"
			in.RunnerProfile = ProfileCI
			in.Declared.Languages = []string{"rust"}
		}, scope: ScopeFull, reason: "no plan: base unresolved", units: []string{"0 declared:rust-test whole"}, incomplete: []string{"go: the ci profile cannot run make go-test, which drops the CI confinement, -race and environment"}},
		{name: "FULL with an unreadable declaration", edit: func(in *Input) {
			in.GoSelection = "FALLBACK go.mod changed"
			in.Declared.Unreadable = "Makefile is unreadable"
		}, scope: ScopeFull, reason: "go.mod changed", units: whole("a", "b", "c"), incomplete: []string{"the declared full scope is unreadable: Makefile is unreadable"}},
		{name: "untyped recipe cannot narrow", edit: func(in *Input) {
			in.Declared = Declared{GoEnv: []string{}, GoFlags: []string{}, GoTest: GoTestUnparsed, GoTestNote: "go-test recipe needs a shell", Languages: []string{}}
		}, scope: ScopeNarrowed, units: []string{}, incomplete: []string{"go: the go-test recipe cannot be typed (go-test recipe needs a shell); run make go-test"}},
		{name: "untyped recipe runs FULL through make", edit: func(in *Input) {
			in.GoSelection = "FALLBACK go.sum changed"
			in.Declared = Declared{GoEnv: []string{}, GoFlags: []string{}, GoTest: GoTestUnparsed, GoTestNote: "x", Languages: []string{}}
		}, scope: ScopeFull, reason: "go.sum changed", units: []string{"0 declared:go-test whole"}},
		{name: "no go-test declaration", edit: func(in *Input) {
			in.Declared = Declared{GoEnv: []string{}, GoFlags: []string{}, GoTest: GoTestAbsent, Languages: []string{}}
		}, scope: ScopeNarrowed, units: []string{}, incomplete: []string{"go: the repository declares no go-test full-suite command"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := fixtureInput()
			tc.edit(&input)
			m, err := Build(input)
			if err != nil {
				t.Fatal(err)
			}
			if tc.incomplete == nil {
				tc.incomplete = []string{}
			}
			if m.Scope != tc.scope || m.Reason != tc.reason || !reflect.DeepEqual(unitIDs(m), tc.units) || !reflect.DeepEqual(m.Incomplete, tc.incomplete) {
				t.Fatalf("got scope=%s reason=%q units=%v incomplete=%v", m.Scope, m.Reason, unitIDs(m), m.Incomplete)
			}
			again, _ := Build(input)
			first, _ := Digest(m)
			second, _ := Digest(again)
			if first != second {
				t.Fatal("the manifest is not a pure function of its input")
			}
		})
	}
	for _, count := range []int{0, cishards.MaxShards + 1} {
		input := fixtureInput()
		input.ShardCount = count
		if _, err := Build(input); err == nil {
			t.Fatalf("shard count %d accepted", count)
		}
	}
}

// STR-V0-015: an exceeded unit bound keeps no units and is INCOMPLETE.
func TestSTRV0015UnitBound(t *testing.T) {
	m := Manifest{Incomplete: []string{}, Units: []Unit{}}
	for index := 0; index <= MaxUnits; index++ {
		m.Units = append(m.Units, Unit{Adapter: AdapterGo, Slice: SliceWhole, Unit: fmt.Sprintf("go:example.test/m/p%05d", index)})
	}
	m = m.bounded()
	if len(m.Units) != 0 || len(m.Incomplete) != 1 || !strings.Contains(m.Incomplete[0], "4096") {
		t.Fatalf("units=%d incomplete=%v", len(m.Units), m.Incomplete)
	}
}

// STR-V0-007: universe placement over the repository's own placement files
// equals cishards.Plan (the go-product-shard membership) intersected with
// the run set, split packages and their catch-all slices included.
func TestSTRV0007UniversePlacementEqualsCIPlan(t *testing.T) {
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join("..", "..", ".github", "cishards", name))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	files := PartitionFiles{Allow: read("test-split-allow.json"), Costs: read("package-costs.json"), Slices: read("test-slices.json")}
	var costs struct {
		Milliseconds map[string]json.RawMessage `json:"milliseconds"`
	}
	if err := json.Unmarshal(files.Costs, &costs); err != nil {
		t.Fatal(err)
	}
	universe := []string{}
	for pkg := range costs.Milliseconds {
		if strings.HasPrefix(pkg, "github.com/") {
			universe = append(universe, pkg)
		}
	}
	sort.Strings(universe)
	if len(universe) < 10 {
		t.Fatalf("cost table names %d packages", len(universe))
	}
	run := map[string]bool{}
	for index, pkg := range universe {
		if index%3 == 0 || strings.HasSuffix(pkg, "/internal/tasks/store") {
			run[pkg] = true
		}
	}
	for _, total := range []int{1, 2, 3, 4, 8} {
		input := fixtureInput()
		input.Universe, input.Partition, input.ShardCount = universe, files, total
		input.GoSelection = "run " + strings.Join(sortedKeys(run), " ")
		m, err := Build(input)
		if err != nil || len(m.Incomplete) != 0 {
			t.Fatalf("total %d: %v %v", total, err, m.Incomplete)
		}
		shards, err := cishards.Plan(universe, total)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{}
		for index, shard := range shards {
			for _, pkg := range shard.Packages {
				if run[pkg] {
					want = append(want, fmt.Sprintf("%d go:%s whole", index, pkg))
				}
			}
			for _, slice := range shard.Slices {
				if run[slice.Package] {
					want = append(want, fmt.Sprintf("%d go:%s %d/%d", index, slice.Package, slice.Index, slice.Count))
				}
			}
		}
		got := unitIDs(m)
		sort.Strings(got)
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("total %d: placement differs\ngot  %v\nwant %v", total, got, want)
		}
		if total >= 2 && !strings.Contains(strings.Join(got, "\n"), "/internal/tasks/store 1/") {
			t.Fatalf("total %d: the split package has no slices: %v", total, got)
		}
		if m.PartitionDigest != PartitionDigest(files) {
			t.Fatal("partition digest not recorded")
		}
	}
}

// STR-V0-005, STR-V0-006: typed argv; only the target set differs between
// narrowed and FULL runs; CI adds the confine wrapper and the CI flags.
func TestSTRV0006Invocations(t *testing.T) {
	m := Manifest{RunnerProfile: ProfileLocal, Units: []Unit{
		{Adapter: AdapterGo, Shard: 0, Slice: SliceWhole, Unit: "go:example.test/m/a"},
		{Adapter: AdapterGo, Shard: 0, Slice: SliceWhole, Unit: "go:example.test/m/b"},
		{Adapter: AdapterGo, Flag: "-run", Pattern: "^(TestA)$", Shard: 0, Slice: "0/2", Unit: "go:example.test/m/s"},
		{Adapter: AdapterGo, Shard: 1, Slice: SliceWhole, Unit: "go:example.test/m/c"},
		{Adapter: AdapterDeclared, Shard: 0, Slice: SliceWhole, Unit: "declared:rust-test"},
	}}
	got := Invocations(m, parsedDeclared(), 0, "/repo", "")
	want := []Planned{
		{Adapter: AdapterGo, Argv: []string{"go", "test", "-json", "-p", "1", "-timeout", "30m", "-count=1", "example.test/m/a", "example.test/m/b"}, Env: []string{"GOTOOLCHAIN=local"}, Units: m.Units[:2]},
		{Adapter: AdapterGo, Argv: []string{"go", "test", "-json", "-p", "1", "-timeout", "30m", "-count=1", "-run", "^(TestA)$", "example.test/m/s"}, Env: []string{"GOTOOLCHAIN=local"}, Units: m.Units[2:3]},
		{Adapter: AdapterDeclared, Argv: []string{"make", "rust-test"}, Env: []string{}, Units: m.Units[4:5]},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("local:\ngot  %#v\nwant %#v", got, want)
	}
	m.RunnerProfile = ProfileCI
	ci := Invocations(m, parsedDeclared(), 1, "/repo", "/runner/test-confine")
	wantCI := Planned{
		Adapter: AdapterGo,
		Argv:    []string{"go", "test", "-exec", "/runner/test-confine", "-json", "-p", "1", "-count=1", "-race", "-timeout", "50m", "example.test/m/c"},
		Env:     []string{"CORVINT_CONFINE_ROOT=/repo", "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOWORK=off", "GOFLAGS=", "CGO_ENABLED=1"},
		Units:   m.Units[3:4],
	}
	if len(ci) != 1 || !reflect.DeepEqual(ci[0], wantCI) {
		t.Fatalf("ci: %#v", ci)
	}
	if empty := Invocations(m, parsedDeclared(), 5, "/repo", ""); len(empty) != 0 {
		t.Fatalf("an empty shard planned %v", empty)
	}
}

func TestSTRV0003ReadDeclared(t *testing.T) {
	write := func(t *testing.T, body string) string {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return root
	}
	none := func(string) (string, bool) { return "", false }
	makefile := "CACHE ?= /tmp/cache\nTIMEOUT ?= 30m\nFLAGS = -p 1 \\\n  -timeout $(TIMEOUT)\n" +
		"go-test:\n\tGOCACHE=$(CACHE) GOTOOLCHAIN=local go test $(FLAGS) -count=1 ./...\n" +
		"rust-test:\n\tcargo test\nswift-test:\n\tswift test\n"
	got := ReadDeclared(write(t, makefile+"ledger/%:\n\t@true\n.DEFAULT_GOAL := gate\n"), []string{"rust"}, none)
	want := Declared{GoEnv: []string{"GOCACHE=/tmp/cache", "GOTOOLCHAIN=local"}, GoFlags: []string{"-p", "1", "-timeout", "30m", "-count=1"}, GoTest: GoTestParsed, Languages: []string{"rust"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
	override := ReadDeclared(write(t, makefile), nil, func(name string) (string, bool) { return "45m", name == "TIMEOUT" })
	if !reflect.DeepEqual(override.GoFlags, []string{"-p", "1", "-timeout", "45m", "-count=1"}) {
		t.Fatalf("?= did not defer to the environment: %v", override.GoFlags)
	}
	for body, note := range map[string]string{
		"go-test:\n\tgo test -json ./...\n":                            "sets -json",
		"go-test:\n\tgo test -run=X ./...\n":                           "sets -run",
		"go-test:\n\tgo test ./... | tee log\n":                        "needs a shell",
		"go-test:\n\tgo vet ./...\n\tgo test ./...\n":                  "2 recipe lines",
		"go-test:\n\tgo test $(UNDEFINED) ./...\n":                     "UNDEFINED",
		"export GOFLAGS=-mod=mod\ngo-test:\n\tgo test ./...\n":         "uses export",
		"ifdef CI\nF = -race\nendif\ngo-test:\n\tgo test $(F) ./...\n": "F",
		"go-test:\n\tgo test pkg ./...\n":                              "before its flags",
		"F = -count=1\nF += -race\ngo-test:\n\tgo test $(F) ./...\n":   "+= or !=",
		"F != echo -race\ngo-test:\n\tgo test $(F) ./...\n":            "+= or !=",
		"go-test: build\n\tgo test ./...\n":                            "prerequisites",
		"go-test: F = -race\ngo-test:\n\tgo test ./...\n":              "target-specific",
		"go-test::\n\tgo test ./...\n":                                 "prerequisites",
		"%-test:\n\techo\ngo-test:\n\tgo test ./...\n":                 "pattern rule %-test",
		"go-test:\n\tgo test ./...\n\n\tgo test -race ./...\n":         "2 recipe lines",
		"go-test:\n\tgo test ./...\n# note\n\tgo test -race ./...\n":   "2 recipe lines",
		"F := $(UNDEFINED) -race\ngo-test:\n\tgo test $(F) ./...\n":    "F",
		"F := $$HOME\ngo-test:\n\tgo test $(F) ./...\n":                "F",
		"go-test:\n\tPATH=/opt/go/bin:/usr/bin go test ./...\n":        "sets PATH",
		"export\tGOFLAGS=-race\ngo-test:\n\tgo test ./...\n":           "uses export",
		"  override F = -race\ngo-test:\n\tgo test ./...\n":            "uses override",
		// Make keeps reading the recipe inside a conditional, so the
		// conditional -race line runs too.
		"go-test:\n\tgo test ./...\nifeq (1,1)\n\tgo test -race ./...\nendif\n":    "recipe contains a conditional",
		"ifeq (1,1)\ngo-test:\n\tgo test ./...\nendif\n":                           "under a conditional",
		"ifeq ($(UNSET),x)\nelse\ngo-test:\n\tgo test ./...\nendif\n":              "under a conditional",
		"PATH = /opt/go/bin:/usr/bin\ngo-test:\n\tgo test ./...\n":                 "assigns PATH",
		"GOFLAGS := -race\ngo-test:\n\tgo test ./...\n":                            "assigns GOFLAGS",
		"CGO_ENABLED = 0\ngo-test:\n\tgo test ./...\n":                             "assigns CGO_ENABLED",
		"SHELL = /bin/bash\ngo-test:\n\tgo test ./...\n":                           "assigns SHELL",
		"lint: export F = 1\ngo-test:\n\tgo test ./...\n":                          "uses export",
		".PHONY .EXPORT_ALL_VARIABLES:\ngo-test:\n\tgo test ./...\n":               "uses .EXPORT_ALL_VARIABLES",
		"define T\nrust-test:\nendef\ngo-test:\n\tgo test ./...\n":                 "uses define",
		"$(STEPS): | clear\ngo-test:\n\tgo test ./...\n":                           "uses $",
		"ifeq ($(UNSET),x)\n$(STEPS): | clear\nendif\ngo-test:\n\tgo test ./...\n": "uses $",
		// A conditional that is true for `make go-test` is read.
		"ifeq ($(filter go-test,$(or $(MAKECMDGOALS),gate)),go-test)\n$(STEPS): | clear\nendif\ngo-test:\n\tgo test ./...\n": "uses $",
		"vpath %.go src\ngo-test:\n\tgo test ./...\n":    "line 1 is not a construct",
		"$(info hi)\ngo-test:\n\tgo test ./...\n":        "line 1 is not a construct",
		".IGNORE: go-test\ngo-test:\n\tgo test ./...\n":  "special target .IGNORE",
		"\tF = -race\ngo-test:\n\tgo test ./...\n":       "tab line outside a rule",
		"go-test:\n\tgo test ./...\nF = 1\nifeq (1,1)\n": "without endif",
	} {
		got := ReadDeclared(write(t, body), nil, none)
		if got.GoTest != GoTestUnparsed || !strings.Contains(got.GoTestNote, note) {
			t.Errorf("%q: %s %q, want unparsed naming %q", body, got.GoTest, got.GoTestNote, note)
		}
	}
	// An included file or eval can declare any full-suite target, so the
	// declared scope is not complete.
	for _, body := range []string{"include rust.mk\ngo-test:\n\tgo test ./...\n", "-include x.mk\n", "$(eval $(call T,rust))\n"} {
		if got := ReadDeclared(write(t, body), []string{"rust"}, none); !strings.Contains(got.Unreadable, "cannot be read completely") {
			t.Errorf("%q: declared scope read as complete: %#v", body, got)
		}
	}
	// A rule naming several targets declares each of them.
	multi := ReadDeclared(write(t, "rust-test cargo-test:\n\tcargo test\ngo-test lint:\n\tgo test ./...\n"), []string{"rust"}, none)
	if !reflect.DeepEqual(multi.Languages, []string{"rust"}) || multi.GoTest != GoTestParsed {
		t.Fatalf("multi-target rules: %#v", multi)
	}
	// ?= leaves an earlier Makefile value alone, whatever the environment
	// says; a leading blank does not hide an assignment.
	precedence := ReadDeclared(write(t, "F = -race\nF ?= -count=1\n  G = -v\ngo-test:\n\tgo test $(F) $(G) ./...\n"), nil, none)
	if !reflect.DeepEqual(precedence.GoFlags, []string{"-race", "-v"}) {
		t.Fatalf("?= precedence: %#v", precedence)
	}
	// ?= on an inherited variable keeps the environment value; any other
	// assignment to one reaches the recipe through the environment.
	env := func(name string) (string, bool) { return "-short", name == "F" || name == "GOFLAGS" }
	if kept := ReadDeclared(write(t, "F ?= -count=1\ngo-test:\n\tgo test $(F) ./...\n"), nil, env); kept.GoTest != GoTestParsed || !reflect.DeepEqual(kept.GoFlags, []string{"-short"}) {
		t.Fatalf("?= over the environment: %#v", kept)
	}
	for body, note := range map[string]string{
		"F = -race\ngo-test:\n\tgo test $(F) ./...\n":    "assigns F",
		"GOFLAGS = -race\ngo-test:\n\tgo test ./...\n":   "assigns GOFLAGS",
		"GOFLAGS ::= -race\ngo-test:\n\tgo test ./...\n": "assigns GOFLAGS",
	} {
		if got := ReadDeclared(write(t, body), nil, env); got.GoTest != GoTestUnparsed || !strings.Contains(got.GoTestNote, note) {
			t.Errorf("%q with inherited F and GOFLAGS: %s %q, want unparsed naming %q", body, got.GoTest, got.GoTestNote, note)
		}
	}
	for flags, typed := range map[string]bool{"-k -- F=-race": false, "ek": false, "k -e": false, " --environment-overrides": false, "k -j4": true, " --no-print-directory": true} {
		makeflags := func(name string) (string, bool) { return flags, name == "MAKEFLAGS" }
		got := ReadDeclared(write(t, "go-test:\n\tgo test ./...\n"), nil, makeflags)
		if (got.GoTest == GoTestParsed) != typed || (!typed && !strings.Contains(got.GoTestNote, "MAKEFLAGS")) {
			t.Errorf("MAKEFLAGS=%q: %#v, want typed %v", flags, got, typed)
		}
	}
	// The conditional evaluator reads no Makefile or environment variable,
	// so each Codex round-6 scenario reads every branch and the GOFLAGS
	// assignment in it leaves go-test untyped. The last ones are outside the
	// exact subset: doubled blank, %, a tab and a trailing blank.
	// Every scenario inherits GOFLAGS and CHOICE; scenario 2 also inherits
	// MODE, which scenario 1 assigns and so must not inherit.
	inherited := map[string]string{"GOFLAGS": "", "CHOICE": "race"}
	lookupInherited := func(name string) (string, bool) { value, ok := inherited[name]; return value, ok }
	recipe := "go-test:\n\tgo test ./...\n"
	for _, body := range []string{
		"MODE = normal # space before comment is significant\nifneq ($(MODE),normal)\nGOFLAGS = -race\nendif\n",
		"ifeq ($(MODE),race)\nGOFLAGS = -race\nendif\n", // MODE='$(CHOICE)' is inherited below
		"CC ?= clang\nifeq ($(CC),cc)\nGOFLAGS = -race\nendif\n",
		"ifeq ($(filter x,a\u00a0x),)\nGOFLAGS = -race\nendif\n",
		"ifeq ($(filter  go-test,$(MAKECMDGOALS)),)\nGOFLAGS = -race\nendif\n",
		"ifeq ($(filter go%,$(MAKECMDGOALS)),)\nGOFLAGS = -race\nendif\n",
		"ifeq\t($(MAKECMDGOALS),gate)\nGOFLAGS = -race\nendif\n",
		"ifeq ($(MAKECMDGOALS),gate) \nGOFLAGS = -race\nendif\n",
		// Codex round 7: make takes the else; ${BAD) sits in an or argument
		// a short-circuit would never check.
		"ifeq ($(or x,${BAD)),x)\nelse\nGOFLAGS = -race\nendif\n",
		"ifeq ($(or x,$(BAD)),x)\nelse\nGOFLAGS = -race\nendif\n",
		"ifeq ($(or x,$$y),x)\nelse\nGOFLAGS = -race\nendif\n",
		"ifeq ($(or x,y)),x)\nelse\nGOFLAGS = -race\nendif\n",
	} {
		lookup := lookupInherited
		if strings.HasPrefix(body, "ifeq ($(MODE)") {
			lookup = func(name string) (string, bool) {
				if name == "MODE" {
					return "$(CHOICE)", true
				}
				return lookupInherited(name)
			}
		}
		want := "assigns GOFLAGS"
		if strings.Contains(body, "${BAD)") {
			// make refuses the unterminated reference as it reads the
			// line, so the scope is incomplete before any branch is read.
			want = "unterminated reference"
		}
		if got := ReadDeclared(write(t, body+recipe), nil, lookup); got.GoTest != GoTestUnparsed || !strings.Contains(got.GoTestNote, want) {
			t.Errorf("%q: %s %q, want every branch read and GOFLAGS untyped", body, got.GoTest, got.GoTestNote)
		}
	}
	// Codex round 7: an inherited MAKECMDGOALS, even empty, replaces the
	// goals make sees, so a conditional on it is not evaluated for go-test.
	goals := func(name string) (string, bool) { return "", name == "MAKECMDGOALS" }
	if got := ReadDeclared(write(t, "ifeq ($(MAKECMDGOALS),go-test)\nelse\nF = 1\nendif\n"+recipe), nil, goals); got.GoTest != GoTestUnparsed || !strings.Contains(got.GoTestNote, "MAKECMDGOALS") {
		t.Errorf("inherited empty MAKECMDGOALS: %#v", got)
	}
	// Inside the subset a false branch is skipped as make skips it.
	for _, body := range []string{
		"ifeq ($(filter gate,$(MAKECMDGOALS)),gate)\nGOFLAGS = -race\nendif\n",
		"ifneq ($(or $(filter x y,a b),$(MAKECMDGOALS)),go-test)\nGOFLAGS = -race\nendif\n",
		"ifeq ($(MAKECMDGOALS),go-test)\nelse\nGOFLAGS = -race\nendif\n",
		"ifeq ($(or x,$(MAKECMDGOALS)),x)\nelse\nGOFLAGS = -race\nendif\n",
	} {
		if got := ReadDeclared(write(t, body+recipe), nil, lookupInherited); got.GoTest != GoTestParsed {
			t.Errorf("%q: %s %q, want the false branch skipped", body, got.GoTest, got.GoTestNote)
		}
	}
	// A conditional outside the go-test span, false for `make go-test`,
	// is skipped as make skips it; an unevaluated one outside the span is
	// read and changes nothing here.
	conditionals := "ifneq ($(filter gate,$(or $(MAKECMDGOALS),gate)),)\n$(STEPS): | clear\nendif\n" +
		"ifeq ($(LEDGER),off)\nledger/%:\n\t@true\nelse\nledger/go-test:\n\t@true\nendif\ngo-test:\n\tgo test ./...\n"
	if got := ReadDeclared(write(t, conditionals), nil, none); got.GoTest != GoTestParsed {
		t.Fatalf("conditionals outside the go-test span: %#v", got)
	}
	// An immediate assignment keeps the value it had when assigned.
	immediate := ReadDeclared(write(t, "F = -race\nFLAGS := $(F) -count=1\nF = -short\ngo-test:\n\tgo test $(FLAGS) ./...\n"), nil, none)
	if immediate.GoTest != GoTestParsed || !reflect.DeepEqual(immediate.GoFlags, []string{"-race", "-count=1"}) {
		t.Fatalf(":= was expanded late: %#v", immediate)
	}
	if absent := ReadDeclared(t.TempDir(), nil, none); absent.GoTest != GoTestAbsent || absent.Names() || absent.Unreadable != "" {
		t.Fatalf("missing Makefile: %#v", absent)
	}
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "Makefile"), 0o755); err != nil {
		t.Fatal(err)
	}
	if unreadable := ReadDeclared(directory, nil, none); unreadable.Unreadable == "" {
		t.Fatal("a Makefile directory was read")
	}
}

// STR-V0-006: typing is exact by construction. Each case below is a
// construct whose make or shell meaning differs from a naive reading
// (review round 8 and the probes behind it); every one is untyped, or read
// conservatively, never guessed.
func TestSTRV0006ReaderGrammar(t *testing.T) {
	write := func(t *testing.T, files map[string]string) string {
		root := t.TempDir()
		for name, body := range files {
			if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return root
	}
	none := func(string) (string, bool) { return "", false }
	recipe := "go-test:\n\tgo test $(F) ./...\n"
	// 1. An inherited MAKECMDGOALS overrides the goals, so a conditional on
	// it is not evaluated: both branches are read and the go-test and
	// rust-test inside them are declared, never absent.
	goals := func(name string) (string, bool) {
		if name == "MAKECMDGOALS" {
			return "other", true
		}
		return "", false
	}
	hidden := "ifeq ($(MAKECMDGOALS),other)\nlint:\n\ttrue\nelse\ngo-test:\n\tgo test ./...\nrust-test:\n\tcargo test\nendif\n"
	if got := ReadDeclared(write(t, map[string]string{"Makefile": hidden}), []string{"rust"}, goals); got.GoTest != GoTestUnparsed ||
		!strings.Contains(got.GoTestNote, "MAKECMDGOALS") || !reflect.DeepEqual(got.Languages, []string{"rust"}) {
		t.Errorf("inherited MAKECMDGOALS: %#v, want unparsed naming MAKECMDGOALS and languages [rust]", got)
	}
	// Without it the evaluator takes the else branch for `make go-test`.
	if got := ReadDeclared(write(t, map[string]string{"Makefile": hidden}), []string{"rust"}, none); got.GoTest != GoTestUnparsed ||
		!strings.Contains(got.GoTestNote, "under a conditional") || !reflect.DeepEqual(got.Languages, []string{"rust"}) {
		t.Errorf("MAKECMDGOALS=go-test takes the else branch: %#v", got)
	}
	// 2. Trailing blanks in a value are significant: `$(F)-race` is two
	// words, so the typed flags keep them apart.
	trailing := ReadDeclared(write(t, map[string]string{"Makefile": "F = -tags=integration \ngo-test:\n\tgo test $(F)-race ./...\n"}), nil, none)
	if trailing.GoTest != GoTestParsed || !reflect.DeepEqual(trailing.GoFlags, []string{"-tags=integration", "-race"}) {
		t.Errorf("trailing blank in a value: %#v, want flags [-tags=integration -race]", trailing)
	}
	for body, note := range map[string]string{
		// 2. make cuts a comment from a makefile line (typed below), while
		// a `#` in a recipe line reaches the shell.
		"F = -race\ngo-test:\n\tgo test $(F) ./... # c\n": "'#'",
		// 3. `?=` on a make built-in keeps make's default (CC=cc), which
		// this reader does not know.
		"CC ?= clang\nF = -ldflags=-X,main.cc=$(CC)\n" + recipe: "undefined variable CC",
		"LINK.o ?= x\nF = $(LINK.o)\n" + recipe:                 "not $(NAME) or ${NAME}",
		"MAKE_X ?= -race\nF = $(MAKE_X)\n" + recipe:             "undefined variable MAKE_X",
		// 4. A backslash-newline in a recipe reaches the shell, which
		// joins the pieces without a blank.
		"go-test:\n\tgo test -count=1\\\n\t  ./...\n": "backslash-newline",
		// 4. An even number of backslashes is not a continuation.
		"F = -race\\\\\n" + recipe: `'\\'`,
		// 5. A non-ASCII blank is a byte of the word to the shell.
		"F = -race -count=1\n" + recipe: "outside the typed alphabet",
		"F = -race\r\n" + recipe:        `'\r'`,
		// 6. Mismatched reference delimiters are an error to make.
		"F = -race\ngo-test:\n\tgo test ${F) ./...\n": "not $(NAME) or ${NAME}",
		"F = -race\ngo-test:\n\tgo test $(F} ./...\n": "not $(NAME) or ${NAME}",
		"F = -race\ngo-test:\n\tgo test $F ./...\n":   "not a $(NAME) or ${NAME} reference",
		// A directive is a whole word; make reads `ifeq(` as a rule-less
		// line, and refuses text after else and a stray endef.
		"ifeq(1,1)\nendif\n" + recipe:                          "line 1 is not a construct",
		"ifeq (1,2)\nelse foo\nendif\n" + recipe:               "text after else",
		"endef\n" + recipe:                                     "endef without define",
		"F = -race\n.DEFAULT:\n\ttrue\n" + recipe:              ".DEFAULT rule",
		"F = -race\n%-test:\n\tgo test -race ./...\n" + recipe: "pattern rule %-test",
		// A `-`, `+` or doubled `@` prefix changes how make runs the line.
		"go-test:\n\t-go test ./...\n":  "not `[NAME=VALUE...] go test FLAGS ./...`",
		"go-test:\n\t@@go test ./...\n": "'@'",
		"go-test:\n\t+go test ./...\n":  "'+'",
	} {
		got := ReadDeclared(write(t, map[string]string{"Makefile": body}), nil, none)
		if got.GoTest != GoTestUnparsed || !strings.Contains(got.GoTestNote, note) {
			t.Errorf("%q: %s %q, want unparsed naming %q", body, got.GoTest, got.GoTestNote, note)
		}
	}
	// Absence is claimed only when certain: a tab line outside a rule is
	// read as makefile text (an include among them), a rule whose target
	// names are outside the alphabet, and a tab-prefixed conditional in a
	// skipped branch all leave the scope incomplete.
	for _, body := range []string{
		"F = 1\n\tinclude inc.mk\n",
		"F = 1\n\t-include inc.mk\n",
		"go-test @x:\n\tgo test ./...\n",
		"ifeq (1,2)\n\tifeq (a,b)\nendif\nendif\n",
		"MAKEFLAGS = -R\n",
		"MAKEFILES = x.mk\n",
		".RECIPEPREFIX = >\n",
	} {
		if got := ReadDeclared(write(t, map[string]string{"Makefile": body}), nil, none); got.Unreadable == "" {
			t.Errorf("%q: declared scope read as complete: %#v", body, got)
		}
	}
	// Codex round 9: a second go-test rule replaces the first recipe; an
	// empty or comment tab line is a recipe line; a makefile line that
	// touches MAKECMDGOALS makes a later conditional unknown, so the rule
	// under it is read; `.PHONY: go-test` alone runs nothing, which is
	// absent (Go INCOMPLETE), never `make go-test`.
	for body, note := range map[string]string{
		"go-test:\n\tgo test -race ./...\ngo-test:\n\t# no tests\n":                                         "more than one rule",
		"go-test:\n\tgo test ./...\n\t# c\n":                                                                "2 recipe lines",
		"go-test:\n\tgo test ./...\n\t\n":                                                                   "2 recipe lines",
		"MAKECMDGOALS = gate\nifeq ($(MAKECMDGOALS),gate)\ngo-test:\n\tgo test -race ./...\nendif\n":        "assigns MAKECMDGOALS",
		"export MAKECMDGOALS = gate\nifeq ($(MAKECMDGOALS),gate)\ngo-test:\n\tgo test ./...\nendif\n":       "uses export",
		"define MAKECMDGOALS\ngate\nendef\nifeq ($(MAKECMDGOALS),gate)\ngo-test:\n\tgo test ./...\nendif\n": "uses define",
	} {
		got := ReadDeclared(write(t, map[string]string{"Makefile": body}), nil, none)
		if got.GoTest != GoTestUnparsed || !strings.Contains(got.GoTestNote, note) {
			t.Errorf("%q: %s %q, want unparsed naming %q", body, got.GoTest, got.GoTestNote, note)
		}
	}
	// An assignment after the conditional does not change what make
	// already evaluated, so the false branch stays skipped.
	if got := ReadDeclared(write(t, map[string]string{"Makefile": "ifeq ($(MAKECMDGOALS),gate)\ngo-test:\n\tgo test ./...\nendif\nMAKECMDGOALS = gate\n"}), nil, none); got.GoTest != GoTestAbsent {
		t.Errorf("late MAKECMDGOALS assignment: %#v, want absent", got)
	}
	if got := ReadDeclared(write(t, map[string]string{"Makefile": ".PHONY: go-test\n"}), nil, none); got.GoTest != GoTestAbsent || !strings.Contains(got.GoTestNote, ".PHONY") || got.Unreadable != "" {
		t.Errorf(".PHONY: go-test alone: %#v, want absent naming .PHONY", got)
	}
	// Codex round 10. make cuts a comment anywhere in a makefile line and
	// keeps the blank before it in a value, so a commented value is typed,
	// and `else# c` is an else, so the go-test rule after it is read (and
	// untyped under the conditional), never absent; `\#` is outside the
	// grammar.
	commented := ReadDeclared(write(t, map[string]string{"Makefile": "F = -tags=integration # c\ngo-test:\n\tgo test $(F)-race ./...\n"}), nil, none)
	if commented.GoTest != GoTestParsed || !reflect.DeepEqual(commented.GoFlags, []string{"-tags=integration", "-race"}) {
		t.Errorf("comment after a value: %#v, want flags [-tags=integration -race]", commented)
	}
	for _, body := range []string{
		"ifeq (1,2)\nelse# comment\ngo-test:\n\tgo test -race ./...\nendif\n",
		"ifeq (1,1)# c\ngo-test: # c\n\tgo test ./...\nendif\n",
	} {
		if got := ReadDeclared(write(t, map[string]string{"Makefile": body}), nil, none); got.GoTest != GoTestUnparsed || !strings.Contains(got.GoTestNote, "under a conditional") || got.Unreadable != "" {
			t.Errorf("%q: %#v, want the go-test rule read under the conditional", body, got)
		}
	}
	// A line make expands as it reads it may hold only $(NAME) references
	// and pure functions. eval reached through call in an immediate
	// assignment, a conditional, an else-if, a prerequisite list, a
	// referenced variable, a define body or an inherited value, shell, a
	// bare $F and `\#` all leave the scope incomplete and go-test untyped.
	inheritedEval := func(name string) (string, bool) {
		if name == "E" {
			return "$(call eval,F = -short)", true
		}
		return "", false
	}
	for body, note := range map[string]string{
		"F = -race\nIGNORED := $(call eval,F = -short)\n" + recipe:                        "make function call",
		"F = -race\nIGNORED := $(or $(call eval,F = -short))\n" + recipe:                  "make function call",
		"F = -race\nifeq ($(call eval,F = -short),)\nendif\n" + recipe:                    "make function call",
		"F = -race\nifeq (1,2)\nelse ifeq ($(call eval,F = -short),)\nendif\n" + recipe:   "make function call",
		"F = -race\nall: $(call eval,F = -short)\n" + recipe:                              "make function call",
		"F = -race\nX = $(call eval,F = -short)\nIGNORED := $(X)\n" + recipe:              "make function call",
		"F = -race\ndefine X\n$(call eval,F = -short)\nendef\nIGNORED := $(X)\n" + recipe: "make function call",
		"F = -race\nIGNORED := $(E)\n" + recipe:                                           "make function call",
		"V := $(shell echo -short)\n" + recipe:                                            "make function shell",
		"IGNORED := $F\n" + recipe:                                                        "not a $(...) or ${...} reference",
		"F = a\\#b\n" + recipe:                                                            `\#`,
	} {
		got := ReadDeclared(write(t, map[string]string{"Makefile": body}), nil, inheritedEval)
		if got.GoTest != GoTestUnparsed || !strings.Contains(got.Unreadable, note) {
			t.Errorf("%q: %s %q %q, want unparsed and incomplete naming %q", body, got.GoTest, got.GoTestNote, got.Unreadable, note)
		}
	}
	// Without an explicit go-test rule make searches its built-in implicit
	// rules, so a go-test.<suffix> rule or root entry, a pattern or suffix
	// rule that can serve one, vpath, VPATH and an RCS directory leave
	// absence uncertain, with or without a makefile; a .PHONY go-test is
	// never built by one. A root file named go-test makes a go-test rule
	// with no prerequisites up to date unless it is .PHONY.
	for _, files := range []map[string]string{
		{"Makefile": "CC = /bin/echo\ngo-test.c:\n\t@echo built-in-prerequisite\n"},
		{"Makefile": "%.c:\n\t@echo $@\n"},
		{"Makefile": ".c.o:\n\t@echo x\n"},
		{"Makefile": "all:\n\t@echo all\n", "go-test.sh": "true\n"},
		{"Makefile": "vpath %.c src\n"},
		{"Makefile": "VPATH = src\n"},
		{"Makefile": "all:\n\t@echo all\n", "RCS": ""},
		{"go-test.sh": "true\n"},
	} {
		if got := ReadDeclared(write(t, files), nil, none); got.GoTest != GoTestAbsent || got.Unreadable == "" {
			t.Errorf("%v: %#v, want absent and incomplete", files, got)
		}
	}
	if got := ReadDeclared(write(t, map[string]string{"Makefile": ".PHONY: go-test\n", "go-test.c": ""}), nil, none); got.GoTest != GoTestAbsent || got.Unreadable != "" {
		t.Errorf(".PHONY go-test beside go-test.c: %#v, want absent and complete", got)
	}
	if got := ReadDeclared(write(t, map[string]string{"Makefile": "go-test:\n\tgo test ./...\n", "go-test": ""}), nil, none); got.GoTest != GoTestUnparsed || !strings.Contains(got.GoTestNote, "up to date") {
		t.Errorf("stale go-test file: %#v, want unparsed naming up to date", got)
	}
	if got := ReadDeclared(write(t, map[string]string{"Makefile": ".PHONY: go-test\ngo-test:\n\tgo test ./...\n", "go-test": ""}), nil, none); got.GoTest != GoTestParsed {
		t.Errorf("phony go-test beside a go-test file: %#v, want parsed", got)
	}
	// Round 11. make remakes the makefile it read when a rule or a built-in
	// implicit rule can, then reads it again, so a rule for it, a pattern
	// or suffix rule that can serve it, or a <makefile>.<suffix> source
	// beside it leaves the scope incomplete even when go-test is .PHONY;
	// with no makefile every default name may be remade. A source of a
	// default name make did not read is not one.
	for _, files := range []map[string]string{
		{"Makefile": "Makefile: replacement.mk\n\tcp replacement.mk Makefile\n.PHONY: go-test\ngo-test:\n\tgo test -race ./...\n", "replacement.mk": "go-test:\n\tgo test -short ./...\n"},
		{"Makefile": ".PHONY: go-test\ngo-test:\n\tgo test -race ./...\n", "Makefile.sh": "go-test:\n\tgo test -short ./...\n"},
		{"Makefile": ".PHONY: go-test\ngo-test:\n\tgo test -race ./...\n%: %.in\n\tcp $< $@\n"},
		{"Makefile": ".PHONY: go-test\ngo-test:\n\tgo test -race ./...\n.in:\n\tcp $< $@\n"},
		{"Makefile.sh": "go-test:\n\tgo test -short ./...\n"},
	} {
		if got := ReadDeclared(write(t, files), nil, none); got.GoTest == GoTestParsed || got.Unreadable == "" {
			t.Errorf("%v: %#v, want untyped and incomplete", files, got)
		}
	}
	if got := ReadDeclared(write(t, map[string]string{"GNUmakefile": "go-test:\n\tgo test -race ./...\n", "Makefile.sh": "go-test:\n\tgo test -short ./...\n"}), nil, none); got.GoTest != GoTestParsed || got.Unreadable != "" {
		t.Errorf("Makefile.sh beside a GNUmakefile: %#v, want parsed and complete", got)
	}
	// make expands a define line's variable name as it reads it.
	if got := ReadDeclared(write(t, map[string]string{"Makefile": "define $(eval go-test: ; @echo witness)X\nunused\nendef\n"}), nil, none); got.GoTest == GoTestParsed || !strings.Contains(got.Unreadable, "make function eval") {
		t.Errorf("eval in a define name: %#v, want incomplete naming eval", got)
	}
	// A .PHONY line under a conditional this reader did not evaluate may be
	// skipped by make, so it does not rule out an implicit rule.
	if got := ReadDeclared(write(t, map[string]string{"Makefile": "ifdef NEVER_SET\n.PHONY: go-test\nendif\ngo-test.c:\n\tprintf x > go-test.c\n"}), nil, none); got.GoTest != GoTestAbsent || got.Unreadable == "" {
		t.Errorf(".PHONY under ifdef beside a go-test.c rule: %#v, want absent and incomplete", got)
	}
	// VPATH from the environment finds implicit-rule sources too.
	vpath := func(name string) (string, bool) { return "src", name == "VPATH" }
	if got := ReadDeclared(write(t, map[string]string{"Makefile": "all:\n\t@true\n"}), nil, vpath); got.GoTest != GoTestAbsent || got.Unreadable == "" {
		t.Errorf("VPATH in the environment: %#v, want absent and incomplete", got)
	}
	// GNU make 3.81 reads ::= as a rule line and GNU make 4 as an
	// assignment; != runs a shell as make reads the line.
	for body, note := range map[string]string{
		"F ::= -race\n" + recipe:     "::=",
		"F != echo -race\n" + recipe: "!=",
	} {
		if got := ReadDeclared(write(t, map[string]string{"Makefile": body}), nil, none); got.GoTest != GoTestUnparsed || !strings.Contains(got.Unreadable, note) {
			t.Errorf("%q: %#v, want unparsed and incomplete naming %q", body, got, note)
		}
	}
	// A pattern rule alone answers `make go-test`, so go-test is declared
	// (untyped) and a matching <lang>-test is declared.
	pattern := ReadDeclared(write(t, map[string]string{"Makefile": "%-test:\n\tcargo test\n"}), []string{"rust"}, none)
	if pattern.GoTest != GoTestUnparsed || !reflect.DeepEqual(pattern.Languages, []string{"rust"}) {
		t.Errorf("pattern rule alone: %#v, want unparsed go-test and languages [rust]", pattern)
	}
	// make reads ./go-test as go-test.
	if got := ReadDeclared(write(t, map[string]string{"Makefile": "./go-test:\n\tgo test ./...\n"}), nil, none); got.GoTest != GoTestParsed {
		t.Errorf("./go-test: %#v, want parsed", got)
	}
	// 7. make reads GNUmakefile before makefile before Makefile.
	both := map[string]string{"GNUmakefile": "go-test:\n\tgo test -race ./...\n", "Makefile": "go-test:\n\tgo test ./...\n"}
	if got := ReadDeclared(write(t, both), nil, none); got.GoTest != GoTestParsed || !reflect.DeepEqual(got.GoFlags, []string{"-race"}) {
		t.Errorf("GNUmakefile beside Makefile: %#v, want the GNUmakefile's -race", got)
	}
	lower := write(t, map[string]string{"makefile": "go-test:\n\tgo test -short ./...\n"})
	if err := os.WriteFile(filepath.Join(lower, "Makefile"), []byte("go-test:\n\tgo test ./...\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(filepath.Join(lower, "makefile")); err == nil && strings.Contains(string(body), "-short") {
		// A case-sensitive filesystem keeps both; make reads makefile first.
		if got := ReadDeclared(lower, nil, none); got.GoTest != GoTestParsed || !reflect.DeepEqual(got.GoFlags, []string{"-short"}) {
			t.Errorf("makefile beside Makefile: %#v, want the makefile's -short", got)
		}
	}
	// MAKEFLAGS and GNUMAKEFLAGS: only harmless words are typed; an option
	// that changes what make reads or runs leaves go-test untyped and the
	// scope incomplete.
	for _, flags := range []string{"-R", "-f other.mk", "--eval=rust-test:", "n", "k --warn-undefined-variables", "-- F=-race"} {
		for _, name := range []string{"MAKEFLAGS", "GNUMAKEFLAGS"} {
			lookup := func(key string) (string, bool) { return flags, key == name }
			got := ReadDeclared(write(t, map[string]string{"Makefile": "F = -race\n" + recipe}), nil, lookup)
			if got.GoTest != GoTestUnparsed || !strings.Contains(got.GoTestNote, name) || got.Unreadable == "" {
				t.Errorf("%s=%q: %#v, want unparsed, incomplete, naming %s", name, flags, got, name)
			}
		}
	}
	for _, flags := range []string{"", "s", "-j8", "k --jobserver-auth=3,4", "--output-sync=line", "w"} {
		lookup := func(key string) (string, bool) { return flags, key == "GNUMAKEFLAGS" }
		if got := ReadDeclared(write(t, map[string]string{"Makefile": "F = -race\n" + recipe}), nil, lookup); got.GoTest != GoTestParsed {
			t.Errorf("GNUMAKEFLAGS=%q: %#v, want parsed", flags, got)
		}
	}
}

// STR-V0-006: the repository Makefile types go-test only because its
// computed-target line (49-50) sits under a conditional the evaluator
// proves false for `make go-test`; with that condition unknown the
// reader reads line 50 and refuses to type.
func TestSTRV0006RepositoryMakefileDependsOnConditional(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	const condition = "ifneq ($(filter gate,$(or $(MAKECMDGOALS),gate)),)\n$(GATE_STEPS) $(addprefix ledger/,$(GATE_STEPS)): | gate-receipt-clear\n"
	if strings.Count(string(body), condition) != 1 {
		t.Fatalf("the Makefile no longer has the gate conditional once:\n%s", condition)
	}
	mutated := strings.Replace(string(body), condition, "ifneq ($(UNKNOWN),)\n$(GATE_STEPS) $(addprefix ledger/,$(GATE_STEPS)): | gate-receipt-clear\n", 1)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte(mutated), 0o644); err != nil {
		t.Fatal(err)
	}
	none := func(string) (string, bool) { return "", false }
	if got := ReadDeclared(root, nil, none); got.GoTest != GoTestUnparsed || !strings.Contains(got.GoTestNote, "uses $") || got.Unreadable == "" {
		t.Fatalf("mutated Makefile: %#v, want unparsed naming the computed target and an incomplete scope", got)
	}
	if got := ReadDeclared(filepath.Join("..", ".."), nil, none); got.GoTest != GoTestParsed {
		t.Fatalf("unmutated Makefile: %#v, want parsed", got)
	}
}

// STR-V0-006: the repository's own Makefile still types go-test; its
// conditionals sit outside the go-test span and assign no Go variable.
func TestSTRV0006RepositoryMakefileTypesGoTest(t *testing.T) {
	got := ReadDeclared(filepath.Join("..", ".."), nil, func(string) (string, bool) { return "", false })
	want := Declared{GoEnv: []string{"GOCACHE=/tmp/corvint-go-build-cache", "GOTOOLCHAIN=local"}, GoFlags: []string{"-p", "1", "-timeout", "30m", "-count=1"}, GoTest: GoTestParsed, Languages: []string{}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("repository Makefile: %#v", got)
	}
}

// STR-V0-017: every dirty path is bound by content digest or a marker.
func TestSTRV0017Identify(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a.go", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	identity, err := Identify(root, "h", "t", []string{"link", "gone.go", "a.go"})
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]string{}
	for _, entry := range identity.Paths {
		paths[entry.Path] = entry.Digest
	}
	if identity.Paths[0].Path != "a.go" || paths["gone.go"] != "deleted" || !strings.HasPrefix(paths["link"], "symlink:sha256:") ||
		!strings.HasPrefix(paths["a.go"], "sha256:") {
		t.Fatalf("identity %#v", identity)
	}
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a // edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	edited, err := Identify(root, "h", "t", []string{"a.go", "gone.go", "link"})
	if err != nil || reflect.DeepEqual(edited, identity) {
		t.Fatalf("an edit to a dirty file is not drift: %v", err)
	}
	// A dirty submodule is reported as its directory; its contents cannot
	// be bound, so the identity is refused rather than bound to a constant.
	if err := os.Mkdir(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Identify(root, "h", "t", []string{"a.go", "sub"}); err == nil || !strings.Contains(err.Error(), "dirty path sub: it is a directory") {
		t.Fatalf("a dirty directory was bound: %v", err)
	}
}
