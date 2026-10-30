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
		Languages: []LanguageCommand{},
	}
}

func declare(in *Input, entries ...LanguageCommand) {
	in.Declared.Languages = append(in.Declared.Languages, entries...)
}

var (
	rustArgv   = LanguageCommand{Language: "rust", Argv: []string{"cargo", "test"}}
	pythonArgv = LanguageCommand{Language: "python", Argv: []string{"pytest", "-q"}}
	noPython   = LanguageCommand{Language: "python", None: "fixtures only"}
)

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
		checks     []string
		noSuite    []string
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
			declare(in, rustArgv, noPython)
		}, scope: ScopeFull, reason: "no plan: unsupported-affected-graph: boom", units: []string{"0 declared:go whole", "0 declared:rust whole"}, noSuite: []string{"python: fixtures only"}},
		{name: "no plan with only Go declared", edit: func(in *Input) {
			in.PlanError = "base unresolved"
		}, scope: ScopeFull, reason: "no plan: base unresolved", units: []string{"0 declared:go whole"}},
		{name: "no plan and an unreadable declaration", edit: func(in *Input) {
			in.PlanError = "base unresolved"
			in.Declared = Declared{Unreadable: "test-commands.json: open: no such file"}
		}, scope: ScopeFull, reason: "no plan: base unresolved", units: []string{}, incomplete: []string{"the declared full scope is unreadable: test-commands.json: open: no such file"}},
		{name: "language without an adapter runs its declared command", edit: func(in *Input) {
			in.Selected = []string{"rust:crate"}
			declare(in, rustArgv)
		}, scope: ScopeNarrowed, units: append(whole("a"), "0 declared:rust whole")},
		{name: "language without an adapter and a none declaration runs nothing", edit: func(in *Input) {
			in.Selected = []string{"python:pkg/mod.py"}
			declare(in, noPython)
		}, scope: ScopeNarrowed, units: whole("a"), noSuite: []string{"python: fixtures only"}},
		{name: "allowlisted non-Go frontier names no unit", edit: func(in *Input) {
			in.GoSelection = "NOTHING none"
			in.Unknown = []Unknown{{Reason: "LANGUAGE_FRONTIER", Detail: "swift:conditional-compilation"}}
		}, scope: ScopeNothing, reason: "none", units: []string{}},
		{name: "selected unit with no adapter and no declaration", edit: func(in *Input) {
			in.GoSelection = "NOTHING none"
			in.Selected = []string{"swift:Sources/App"}
		}, scope: ScopeNarrowed, units: []string{}, incomplete: []string{"swift: no qualified adapter and .github/cishards/test-commands.json declares no swift full command"}},
		{name: "unknown-only non-Go unit runs its declared command", edit: func(in *Input) {
			in.GoSelection = "NOTHING none"
			in.Unknown = []Unknown{{Reason: "NO_SELECTABLE_TEST", Detail: "python:pkg/mod.py"}}
			declare(in, pythonArgv)
		}, scope: ScopeNarrowed, units: []string{"0 declared:python whole"}},
		{name: "unknown-only non-Go unit with no declaration", edit: func(in *Input) {
			in.GoSelection = "NOTHING none"
			in.Unknown = []Unknown{{Reason: "NO_SELECTABLE_TEST", Detail: "python:pkg/mod.py"}}
		}, scope: ScopeNarrowed, units: []string{}, incomplete: []string{"python: no qualified adapter and .github/cishards/test-commands.json declares no python full command"}},
		{name: "FULL still covers a selected non-Go language", edit: func(in *Input) {
			in.GoSelection = "FALLBACK go.mod changed"
			in.Selected = []string{"python:pkg/mod.py"}
			declare(in, pythonArgv)
		}, scope: ScopeFull, reason: "go.mod changed", units: []string{"0 go:example.test/m/a whole", "0 go:example.test/m/b whole", "0 go:example.test/m/c whole", "0 declared:python whole"}},
		{name: "FULL from a path-only fallback runs every declared language", edit: func(in *Input) {
			in.GoSelection = "FALLBACK UNINDEXED_SOURCE_PATH pkg/gone.py"
			declare(in, pythonArgv, LanguageCommand{Language: "ruby", None: "no Ruby sources"})
		}, scope: ScopeFull, reason: "UNINDEXED_SOURCE_PATH pkg/gone.py", units: []string{"0 go:example.test/m/a whole", "0 go:example.test/m/b whole", "0 go:example.test/m/c whole", "0 declared:python whole"}, noSuite: []string{"ruby: no Ruby sources"}},
		{name: "FULL covers an undeclared language owning a dirty path", edit: func(in *Input) {
			in.GoSelection = "FALLBACK UNINDEXED_SOURCE_PATH pkg/gone.py"
			in.DirtyLanguages = []string{"python"}
		}, scope: ScopeFull, reason: "UNINDEXED_SOURCE_PATH pkg/gone.py", units: whole("a", "b", "c"), incomplete: []string{"python: no qualified adapter and .github/cishards/test-commands.json declares no python full command"}},
		{name: "ci profile with no plan runs the declared Go suite and reports declared commands as checks", edit: func(in *Input) {
			in.PlanError = "base unresolved"
			in.RunnerProfile = ProfileCI
			declare(in, rustArgv)
		}, scope: ScopeFull, reason: "no plan: base unresolved", units: []string{"0 declared:go whole"}, checks: []string{"cargo test"}},
		{name: "ci profile FULL never launches a declared command or make target", edit: func(in *Input) {
			in.GoSelection = "FALLBACK go.mod changed"
			in.RunnerProfile = ProfileCI
			in.Checks = []string{"make go-vet"}
			declare(in, LanguageCommand{Language: "typescript", Argv: []string{"make", "host-adapter-test"}}, noPython)
		}, scope: ScopeFull, reason: "go.mod changed", units: whole("a", "b", "c"), checks: []string{"make go-vet", "make host-adapter-test"}, noSuite: []string{"python: fixtures only"}},
		{name: "FULL with an unreadable declaration", edit: func(in *Input) {
			in.GoSelection = "FALLBACK go.mod changed"
			in.Declared = Declared{Unreadable: "test-commands.json: profile is \"x\""}
		}, scope: ScopeFull, reason: "the declared full scope is unreadable", units: []string{}, incomplete: []string{"the declared full scope is unreadable: test-commands.json: profile is \"x\""}},
		{name: "narrowed with an unreadable declaration", edit: func(in *Input) {
			in.Declared = Declared{Unreadable: "test-commands.json: open: no such file"}
		}, scope: ScopeFull, reason: "the declared full scope is unreadable", units: []string{}, incomplete: []string{"the declared full scope is unreadable: test-commands.json: open: no such file"}},
		{name: "FULL with no Go universe runs the declared Go suite", edit: func(in *Input) {
			in.GoSelection = "FALLBACK go.mod changed"
			in.Universe = nil
		}, scope: ScopeFull, reason: "go.mod changed", units: []string{"0 declared:go whole"}},
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
			if m.Scope != tc.scope || m.Reason != tc.reason || !reflect.DeepEqual(unitIDs(m), tc.units) || !reflect.DeepEqual(m.Incomplete, tc.incomplete) || !reflect.DeepEqual(m.NoSuite, nonNil(tc.noSuite)) {
				t.Fatalf("got scope=%s reason=%q units=%v incomplete=%v noSuite=%v", m.Scope, m.Reason, unitIDs(m), m.Incomplete, m.NoSuite)
			}
			if tc.checks != nil && !reflect.DeepEqual(m.Checks, tc.checks) {
				t.Fatalf("got checks=%v want %v", m.Checks, tc.checks)
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
		// The committed store split has eight slices (AFP-V0-060); a split
		// package with more slices than shards runs whole (AFP-V0-041).
		if total >= 8 && !strings.Contains(strings.Join(got, "\n"), "/internal/tasks/store 1/") {
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
		{Adapter: AdapterDeclared, Shard: 0, Slice: SliceWhole, Unit: "declared:rust"},
		{Adapter: AdapterDeclared, Shard: 0, Slice: SliceWhole, Unit: "declared:go"},
		{Adapter: AdapterDeclared, Shard: 0, Slice: SliceWhole, Unit: "declared:swift"},
	}}
	declared := parsedDeclared()
	declared.Languages = []LanguageCommand{rustArgv}
	got := Invocations(m, declared, 0, "/repo", "")
	want := []Planned{
		{Adapter: AdapterGo, Argv: []string{"go", "test", "-json", "-p", "1", "-timeout", "30m", "-count=1", "example.test/m/a", "example.test/m/b"}, Env: []string{"GOTOOLCHAIN=local"}, Units: m.Units[:2]},
		{Adapter: AdapterGo, Argv: []string{"go", "test", "-json", "-p", "1", "-timeout", "30m", "-count=1", "-run", "^(TestA)$", "example.test/m/s"}, Env: []string{"GOTOOLCHAIN=local"}, Units: m.Units[2:3]},
		{Adapter: AdapterDeclared, Argv: []string{"cargo", "test"}, Env: []string{}, Units: m.Units[4:5]},
		{Adapter: AdapterDeclared, Argv: []string{"go", "test", "-p", "1", "-timeout", "30m", "-count=1", "./..."}, Env: []string{"GOTOOLCHAIN=local"}, Units: m.Units[5:6]},
		{Adapter: AdapterDeclared, Argv: []string{}, Env: []string{}, Units: m.Units[6:7]},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("local:\ngot  %#v\nwant %#v", got, want)
	}
	m.RunnerProfile = ProfileCI
	ci := Invocations(m, declared, 1, "/repo", "/runner/test-confine")
	wantCI := Planned{
		Adapter: AdapterGo,
		Argv:    []string{"go", "test", "-exec", "/runner/test-confine", "-json", "-p", "1", "-count=1", "-race", "-timeout", "50m", "example.test/m/c"},
		Env:     []string{"CORVINT_CONFINE_ROOT=/repo", "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOWORK=off", "GOFLAGS=", "CGO_ENABLED=1"},
		Units:   m.Units[3:4],
	}
	if len(ci) != 1 || !reflect.DeepEqual(ci[0], wantCI) {
		t.Fatalf("ci: %#v", ci)
	}
	ciGo := Invocations(m, declared, 0, "/repo", "/runner/test-confine")[3]
	if !reflect.DeepEqual(ciGo.Argv, []string{"go", "test", "-exec", "/runner/test-confine", "-p", "1", "-count=1", "-race", "-timeout", "50m", "./..."}) || !reflect.DeepEqual(ciGo.Env, wantCI.Env) {
		t.Fatalf("ci declared go: %#v", ciGo)
	}
	if empty := Invocations(m, declared, 5, "/repo", ""); len(empty) != 0 {
		t.Fatalf("an empty shard planned %v", empty)
	}
}

// STR-V0-003, STR-V0-006: the declaration is one strict, bounded JSON file;
// every defect makes it unreadable rather than a partial declaration.
func TestSTRV0003ReadDeclared(t *testing.T) {
	write := func(t *testing.T, body string) string {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".github", "cishards"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(DeclarationPath)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return root
	}
	full := `{
  "profile": "corvint-test-commands/0",
  "go": {"env": {"GOTOOLCHAIN": "local", "GOCACHE": "/tmp/c"}, "flags": ["-p", "1", "-count=1"]},
  "languages": {"rust": ["cargo", "test"], "python": {"none": "fixtures only"}}
}
`
	got := ReadDeclared(write(t, full))
	want := Declared{
		GoEnv: []string{"GOCACHE=/tmp/c", "GOTOOLCHAIN=local"}, GoFlags: []string{"-p", "1", "-count=1"},
		Languages: []LanguageCommand{{Language: "python", None: "fixtures only"}, {Language: "rust", Argv: []string{"cargo", "test"}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("declaration %#v", got)
	}
	if got := ReadDeclared(t.TempDir()); !strings.HasPrefix(got.Unreadable, DeclarationPath+": ") || len(got.GoEnv)+len(got.GoFlags)+len(got.Languages) != 0 {
		t.Fatalf("absent declaration %#v", got)
	}
	minimal := `{"profile": "corvint-test-commands/0", "go": {"env": {}, "flags": []}}`
	if got := ReadDeclared(write(t, minimal)); got.Unreadable != "" || len(got.Languages) != 0 || got.GoEnv == nil || got.GoFlags == nil {
		t.Fatalf("minimal declaration %#v", got)
	}
	unreadable := map[string]string{
		"profile":          `{"profile": "corvint-test-commands/1", "go": {"env": {}, "flags": []}}`,
		"no go":            `{"profile": "corvint-test-commands/0", "languages": {}}`,
		"unknown member":   `{"profile": "corvint-test-commands/0", "go": {"env": {}, "flags": []}, "make": true}`,
		"unknown go field": `{"profile": "corvint-test-commands/0", "go": {"env": {}, "flags": [], "packages": ["./..."]}}`,
		"duplicate member": `{"profile": "corvint-test-commands/0", "go": {"env": {}, "flags": ["-p"]}, "go": {"env": {}, "flags": []}}`,
		"env name":         `{"profile": "corvint-test-commands/0", "go": {"env": {"A=B": "c"}, "flags": []}}`,
		"flag with space":  `{"profile": "corvint-test-commands/0", "go": {"env": {}, "flags": ["-timeout 30m"]}}`,
		"flag with cr":     `{"profile": "corvint-test-commands/0", "go": {"env": {}, "flags": ["-run=TestA\r"]}}`,
		"flag nbsp":        `{"profile": "corvint-test-commands/0", "go": {"env": {}, "flags": ["-p\u00a01"]}}`,
		"env nul name":     `{"profile": "corvint-test-commands/0", "go": {"env": {"GO\u0000CACHE": "/tmp/c"}, "flags": []}}`,
		"env dash name":    `{"profile": "corvint-test-commands/0", "go": {"env": {"GO-CACHE": "/tmp/c"}, "flags": []}}`,
		"env digit first":  `{"profile": "corvint-test-commands/0", "go": {"env": {"1GOCACHE": "/tmp/c"}, "flags": []}}`,
		"env PATH":         `{"profile": "corvint-test-commands/0", "go": {"env": {"PATH": "/opt/go/bin"}, "flags": []}}`,
		"env value tab":    `{"profile": "corvint-test-commands/0", "go": {"env": {"GOCACHE": "/tmp/c\t"}, "flags": []}}`,
		"language cr":      `{"profile": "corvint-test-commands/0", "go": {"env": {}, "flags": []}, "languages": {"rust\r": ["cargo", "test"]}}`,
		"argv escape":      `{"profile": "corvint-test-commands/0", "go": {"env": {}, "flags": []}, "languages": {"rust": ["cargo", "test\u001b"]}}`,
		"empty argv":       `{"profile": "corvint-test-commands/0", "go": {"env": {}, "flags": []}, "languages": {"rust": []}}`,
		"empty none":       `{"profile": "corvint-test-commands/0", "go": {"env": {}, "flags": []}, "languages": {"rust": {"none": " "}}}`,
		"none plus argv":   `{"profile": "corvint-test-commands/0", "go": {"env": {}, "flags": []}, "languages": {"rust": {"none": "x", "argv": ["cargo"]}}}`,
		"string value":     `{"profile": "corvint-test-commands/0", "go": {"env": {}, "flags": []}, "languages": {"rust": "cargo test"}}`,
		"go language":      `{"profile": "corvint-test-commands/0", "go": {"env": {}, "flags": []}, "languages": {"go": ["go", "test"]}}`,
		"trailing data":    `{"profile": "corvint-test-commands/0", "go": {"env": {}, "flags": []}} {}`,
		"oversize":         `{"profile": "corvint-test-commands/0", "go": {"env": {}, "flags": []}, "languages": {"rust": {"none": "` + strings.Repeat("x", MaxDocumentBytes) + `"}}}`,
	}
	for name, body := range unreadable {
		if got := ReadDeclared(write(t, body)); got.Unreadable == "" || len(got.Languages) != 0 || len(got.GoFlags) != 0 {
			t.Errorf("%s: read as %#v", name, got)
		}
	}
	// The manifest digest is the parsed declaration's canonical form, so
	// member order and whitespace do not change it.
	reordered := `{"languages": {"python": {"none": "fixtures only"}, "rust": ["cargo", "test"]}, "go": {"flags": ["-p", "1", "-count=1"], "env": {"GOCACHE": "/tmp/c", "GOTOOLCHAIN": "local"}}, "profile": "corvint-test-commands/0"}`
	a, _ := Digest(ReadDeclared(write(t, full)))
	b, _ := Digest(ReadDeclared(write(t, reordered)))
	if a != b {
		t.Fatalf("digests differ: %s %s", a, b)
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
