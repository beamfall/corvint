package affected

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fake is a synthetic language used to exercise graph and selection invariants
// without depending on any real plugin.
type fake struct {
	name     string
	units    []Unit
	frontier []string
	owns     func(string) bool
	err      error
}

func (language fake) Name() string { return language.name }

func (language fake) Owns(path string) bool {
	if language.owns != nil {
		return language.owns(path)
	}
	return strings.HasSuffix(path, "."+language.name)
}

func (language fake) Units(string) (Result, error) {
	return Result{Units: language.units, Frontier: language.frontier}, language.err
}

func chain() fake {
	return fake{name: "x", units: []Unit{
		{ID: "x:core", Sources: []string{"core.x"}, Tests: []string{"core_test.x"}},
		{ID: "x:mid", Sources: []string{"mid.x"}, Tests: []string{"mid_test.x"}, Imports: []string{"x:core"}},
		{ID: "x:leaf", Sources: []string{"leaf.x"}, Tests: []string{"leaf_test.x"}, Imports: []string{"x:mid"}},
		{ID: "x:solo", Sources: []string{"solo.x"}, Tests: []string{"solo_test.x"}},
		{ID: "x:untested", Sources: []string{"untested.x"}, Imports: []string{"x:core"}},
	}}
}

func TestBuildRejectsUnitsThatBreakTheSeamContract(t *testing.T) {
	cases := map[string]struct {
		language fake
		want     error
	}{
		"unnamespaced identity": {
			language: fake{name: "x", units: []Unit{{ID: "core", Sources: []string{"core.x"}}}},
			want:     ErrInvalidUnit,
		},
		"foreign namespace": {
			language: fake{name: "x", units: []Unit{{ID: "y:core", Sources: []string{"core.x"}}}},
			want:     ErrInvalidUnit,
		},
		"unsorted sources": {
			language: fake{name: "x", units: []Unit{{ID: "x:core", Sources: []string{"b.x", "a.x"}}}},
			want:     ErrInvalidUnit,
		},
		"escaping path": {
			language: fake{name: "x", units: []Unit{{ID: "x:core", Sources: []string{"../outside.x"}}}},
			want:     ErrInvalidUnit,
		},
		"duplicate identity": {
			language: fake{name: "x", units: []Unit{
				{ID: "x:core", Sources: []string{"a.x"}},
				{ID: "x:core", Sources: []string{"b.x"}},
			}},
			want: ErrDuplicateUnit,
		},
		"two units claim one path": {
			language: fake{name: "x", units: []Unit{
				{ID: "x:core", Sources: []string{"shared.x"}},
				{ID: "x:other", Sources: []string{"shared.x"}},
			}},
			want: ErrDuplicateOwner,
		},
		"empty frontier reason": {
			language: fake{name: "x", frontier: []string{""}},
			want:     ErrInvalidLanguage,
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Build(t.TempDir(), testCase.language); !errors.Is(err, testCase.want) {
				t.Fatalf("err=%v want=%v", err, testCase.want)
			}
		})
	}
}

func TestBuildRejectsDuplicateLanguageNames(t *testing.T) {
	if _, err := Build(t.TempDir(), fake{name: "x"}, fake{name: "x"}); !errors.Is(err, ErrInvalidLanguage) {
		t.Fatalf("err=%v", err)
	}
	if _, err := Build(t.TempDir()); !errors.Is(err, ErrInvalidLanguage) {
		t.Fatalf("err=%v", err)
	}
}

// walking lists root through SourceFiles as a real plugin does, then runs after.
type walking struct {
	name   string
	accept func(string) bool
	listed *[]string
	after  func()
}

func (language walking) Name() string { return language.name }

func (language walking) Owns(string) bool { return false }

func (language walking) Units(root string) (Result, error) {
	files, err := SourceFiles(root, language.accept)
	*language.listed = files
	if language.after != nil {
		language.after()
	}
	return Result{}, err
}

func TestBuildPluginsShareOneWalkOfTheTree(t *testing.T) {
	root := t.TempDir()
	write := func(name string) {
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("a.txt")
	write("a.md")
	text := func(name string) bool { return strings.HasSuffix(name, ".txt") }
	markdown := func(name string) bool { return strings.HasSuffix(name, ".md") }
	var first, second, third []string
	if _, err := Build(root,
		walking{name: "p", accept: text, listed: &first, after: func() { write("b.txt") }},
		walking{name: "q", accept: text, listed: &second},
		walking{name: "r", accept: markdown, listed: &third},
	); err != nil {
		t.Fatal(err)
	}
	if strings.Join(first, ",") != "a.txt" || strings.Join(second, ",") != "a.txt" || strings.Join(third, ",") != "a.md" {
		t.Fatalf("plugins of one Build must filter one walk: first=%v second=%v third=%v", first, second, third)
	}
	outside, err := SourceFiles(root, text)
	if err != nil || strings.Join(outside, ",") != "a.txt,b.txt" {
		t.Fatalf("a walk after Build must see the tree as it is: %v %v", outside, err)
	}
	if _, err := Build(root, walking{name: "p", accept: text, listed: &first}); err != nil || strings.Join(first, ",") != "a.txt,b.txt" {
		t.Fatalf("a later Build must walk again: %v %v", first, err)
	}
}

func TestSelectTraversesUntestedUnitsWithoutSelectingThem(t *testing.T) {
	graph, err := Build(t.TempDir(), chain())
	if err != nil {
		t.Fatal(err)
	}
	plan := Select(graph, []string{"core.x"})
	for _, selection := range plan.Selected {
		if selection.UnitID == "x:untested" {
			t.Fatal("a unit with no tests was selected")
		}
	}
	for _, exclusion := range plan.Excluded {
		if exclusion.UnitID == "x:untested" {
			t.Fatal("a unit with no tests received an exclusion certificate")
		}
	}
}

func TestSelectGivesTheShortestWitnessChain(t *testing.T) {
	graph, err := Build(t.TempDir(), chain())
	if err != nil {
		t.Fatal(err)
	}
	plan := Select(graph, []string{"core.x"})
	want := map[string]int{"x:core": 1, "x:mid": 2, "x:leaf": 3}
	for _, selection := range plan.Selected {
		expected, tracked := want[selection.UnitID]
		if !tracked {
			t.Fatalf("unexpected selection %s", selection.UnitID)
		}
		if len(selection.Witness.Via) != expected {
			t.Fatalf("%s chain=%v want length %d", selection.UnitID, selection.Witness.Via, expected)
		}
	}
	if len(plan.Selected) != len(want) {
		t.Fatalf("selected=%d want=%d", len(plan.Selected), len(want))
	}
}

// AFP-V0-007: distance first, then the longer shared directory prefix with the
// dirty path, then unit id. x:a-docs sorts first by id but sits in examples/,
// so the package's own dependent x:pkg-user must precede it.
func TestSelectOrdersByDistanceThenSharedDirectoryThenUnitID(t *testing.T) {
	language := fake{name: "x", units: []Unit{
		{ID: "x:a-docs", Sources: []string{"examples/a.x"}, Tests: []string{"examples/a_test.x"}, Imports: []string{"x:core"}},
		{ID: "x:b-docs", Sources: []string{"examples/b.x"}, Tests: []string{"examples/b_test.x"}, Imports: []string{"x:core"}},
		{ID: "x:core", Sources: []string{"pkg/core/core.x"}, Tests: []string{"pkg/core/core_test.x"}},
		{ID: "x:far", Sources: []string{"pkg/core/far.x"}, Tests: []string{"pkg/core/far_test.x"}, Imports: []string{"x:a-docs"}},
		{ID: "x:pkg-user", Sources: []string{"pkg/core/sub/user.x"}, Tests: []string{"pkg/core/sub/user_test.x"}, Imports: []string{"x:core"}},
	}}
	graph, err := Build(t.TempDir(), language)
	if err != nil {
		t.Fatal(err)
	}
	first := Select(graph, []string{"pkg/core/core.x"})
	want := []string{"x:core", "x:pkg-user", "x:a-docs", "x:b-docs", "x:far"}
	got := make([]string, 0, len(first.Selected))
	for _, selection := range first.Selected {
		got = append(got, selection.UnitID)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("order=%v want=%v", got, want)
	}
	second := Select(graph, []string{"pkg/core/core.x"})
	firstBody, err := first.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	secondBody, err := second.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if string(firstBody) != string(secondBody) {
		t.Fatalf("plan order is not deterministic:\n%s\n%s", firstBody, secondBody)
	}
}

func TestSelectOnATestFileNamesADirectTestWitness(t *testing.T) {
	graph, err := Build(t.TempDir(), chain())
	if err != nil {
		t.Fatal(err)
	}
	plan := Select(graph, []string{"solo_test.x"})
	if len(plan.Selected) != 1 || plan.Selected[0].UnitID != "x:solo" {
		t.Fatalf("selected=%+v", plan.Selected)
	}
	if plan.Selected[0].Witness.Kind != WitnessDirectTest {
		t.Fatalf("kind=%s", plan.Selected[0].Witness.Kind)
	}
}

func TestSelectWithNoDirtyPathsSelectsNothingAndExcludesEverything(t *testing.T) {
	graph, err := Build(t.TempDir(), chain())
	if err != nil {
		t.Fatal(err)
	}
	plan := Select(graph, nil)
	if len(plan.Selected) != 0 {
		t.Fatalf("selected=%+v", plan.Selected)
	}
	if len(plan.Excluded) != 4 {
		t.Fatalf("excluded=%d want 4", len(plan.Excluded))
	}
	if plan.Scope != ScopeBounded {
		t.Fatalf("scope=%s", plan.Scope)
	}
}

func TestLanguageFrontierWidensEveryPlanFromThatGraph(t *testing.T) {
	language := chain()
	language.frontier = []string{"x:dynamic-dispatch"}
	graph, err := Build(t.TempDir(), language)
	if err != nil {
		t.Fatal(err)
	}
	plan := Select(graph, []string{"core.x"})
	if plan.Scope != ScopeUnknown {
		t.Fatalf("scope=%s", plan.Scope)
	}
	found := false
	for _, unknown := range plan.Unknown {
		if unknown.Reason == UnknownLanguageFrontier && unknown.Detail == "x:dynamic-dispatch" {
			found = true
		}
	}
	if !found {
		t.Fatalf("frontier not carried into the plan: %+v", plan.Unknown)
	}
}

func TestGraphDigestChangesWithEveryObservedInput(t *testing.T) {
	base, err := Build(t.TempDir(), chain())
	if err != nil {
		t.Fatal(err)
	}
	withFrontier := chain()
	withFrontier.frontier = []string{"x:dynamic-dispatch"}
	widened, err := Build(t.TempDir(), withFrontier)
	if err != nil {
		t.Fatal(err)
	}
	if base.Digest() == widened.Digest() {
		t.Fatal("frontier is outside the graph digest")
	}
	withEdge := chain()
	withEdge.units[3].Imports = []string{"x:core"}
	edged, err := Build(t.TempDir(), withEdge)
	if err != nil {
		t.Fatal(err)
	}
	if base.Digest() == edged.Digest() {
		t.Fatal("edges are outside the graph digest")
	}
}

func TestDecodeStatusReadsEveryPorcelainRecordShape(t *testing.T) {
	raw := []byte("?? untracked.x\x00 M modified.x\x00R  renamed_new.x\x00renamed_old.x\x00MM staged_and_dirty.x\x00?? vendor/nested/\x00")
	paths, err := DecodeStatus(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"modified.x", "renamed_new.x", "renamed_old.x", "staged_and_dirty.x", "untracked.x", "vendor/nested"}
	if len(paths) != len(want) {
		t.Fatalf("paths=%v want=%v", paths, want)
	}
	for index := range want {
		if paths[index] != want[index] {
			t.Fatalf("paths=%v want=%v", paths, want)
		}
	}
}

func TestDecodeStatusFailsClosedOnMalformedInput(t *testing.T) {
	cases := map[string][]byte{
		"short record":           []byte("?\x00"),
		"missing separator":      []byte("??untracked.x\x00"),
		"rename with nothing":    []byte("R  renamed_new.x\x00"),
		"escaping path":          []byte("?? ../outside.x\x00"),
		"absolute path":          []byte("?? /etc/passwd\x00"),
		"rename origin escape":   []byte("R  new.x\x00../old.x\x00"),
		"tracked trailing slash": []byte(" M dir/\x00"),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeStatus(raw); !errors.Is(err, ErrStatusMalformed) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestUnionMergesAnUnsavedBufferIntoTheDirtySet(t *testing.T) {
	merged := Union([]string{"b.x"}, Overlay{Paths: []string{"a.x", "b.x", "", "../escape.x"}})
	if len(merged) != 2 || merged[0] != "a.x" || merged[1] != "b.x" {
		t.Fatalf("merged=%v", merged)
	}
}

// An unsaved buffer must select exactly what saving the file would select.
// That equivalence is the whole point of the overlay: the editor need not
// write to disk for the plan to be correct.
func TestOverlayOnlyPathSelectsTheSameUnitsAsASavedEdit(t *testing.T) {
	graph, err := Build(t.TempDir(), chain())
	if err != nil {
		t.Fatal(err)
	}
	saved := Select(graph, []string{"core.x"})
	unsaved := Select(graph, Union(nil, Overlay{Paths: []string{"core.x"}}))
	savedBody, err := saved.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	unsavedBody, err := unsaved.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if string(savedBody) != string(unsavedBody) {
		t.Fatalf("overlay plan differs from saved plan:\n%s\n%s", savedBody, unsavedBody)
	}
}

func TestDirectoriesOfGroupsPathsByParent(t *testing.T) {
	got := DirectoriesOf([]string{"a/b/c.x", "a/b/d.x", "a/e.x", "top.x"})
	want := []string{"a", "a/b", "."}
	if len(got) != len(want) {
		t.Fatalf("got=%v", got)
	}
	for _, value := range want {
		found := false
		for _, candidate := range got {
			if candidate == value {
				found = true
			}
		}
		if !found {
			t.Fatalf("got=%v missing %s", got, value)
		}
	}
}

// A committed range is decoded with the same path rule as the worktree
// status: every name is a valid relative path or the whole capture fails.
func TestDecodeNameListNormalizesAndFailsClosed(t *testing.T) {
	paths, err := DecodeNameList([]byte("b/moved.x\x00a/removed.x\x00b/moved.x\x00"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || paths[0] != "a/removed.x" || paths[1] != "b/moved.x" {
		t.Fatalf("paths=%v", paths)
	}
	for name, raw := range map[string][]byte{"escaping": []byte("../outside.x\x00"), "absolute": []byte("/etc/passwd\x00"), "empty name": []byte("\x00a.x\x00")} {
		if _, err := DecodeNameList(raw); !errors.Is(err, ErrStatusMalformed) {
			t.Fatalf("%s: err=%v", name, err)
		}
	}
}

// TestSourceFilesRefusesAnUnrepresentableAcceptedName pins that an accepted
// file whose repository-relative path is no canonical unit path (go test runs
// a test file named with a backslash) refuses the walk instead of silently
// narrowing the unit set.
func TestSourceFilesRefusesAnUnrepresentableAcceptedName(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "p"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "p", `b\c_test.go`), []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	accept := func(name string) bool { return strings.HasSuffix(name, ".go") }
	if files, err := SourceFiles(root, accept); !errors.Is(err, ErrWalkUnrepresentable) {
		t.Fatalf("SourceFiles = %v, %v; want ErrWalkUnrepresentable", files, err)
	}
}
