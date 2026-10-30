package affected

import (
	"context"
	json "encoding/json/v2"
	"slices"
	"strings"
	"testing"
)

// taskStatePlan builds chain() after edit, with a frontier on the x plugin,
// and plans dirty (AFP-V0-088).
func taskStatePlan(t *testing.T, edit func(*fake), dirty ...string) Plan {
	t.Helper()
	language := chain()
	language.frontier = []string{"x:dynamic-dispatch"}
	if edit != nil {
		edit(&language)
	}
	graph, err := Build(context.Background(), t.TempDir(), language)
	if err != nil {
		t.Fatal(err)
	}
	return Select(graph, dirty)
}

func nonSourcePaths(plan Plan) []string {
	paths := []string{}
	for _, group := range plan.NonSource {
		if group.Class != ClassTaskStoreProjection || group.Reason != taskStoreProjectionReason || len(plan.NonSource) != 1 {
			panic("unexpected non-source group " + group.Class)
		}
		paths = append(paths, group.Paths...)
	}
	return paths
}

func unknownDetails(plan Plan) []string {
	details := []string{}
	for _, entry := range plan.Unknown {
		details = append(details, entry.Reason+" "+entry.Detail)
	}
	return details
}

// AFP-V0-088 (1)-(3): a dirty Tasks store projection that no unit names is
// task state with a stated reason, raises no unknown entry, and involves no
// plugin's frontier, so it leaves the plan BOUNDED. A path no plugin claims
// outside the projection keeps the AFP-V0-004 widening.
func TestTaskStoreProjectionIsTaskStateNotUnknown_AFPV0088(t *testing.T) {
	projection := []string{".taskman/policy.json", ".taskman/queue.json", ".taskman/releases/v1-0.json", ".taskman/tickets/V1-0001.json"}
	plan := taskStatePlan(t, nil, projection...)
	if plan.Scope != ScopeBounded || len(plan.Unknown) != 0 || len(plan.Selected) != 0 {
		t.Fatalf("scope=%s unknown=%v selected=%+v", plan.Scope, unknownDetails(plan), plan.Selected)
	}
	if got := nonSourcePaths(plan); !slices.Equal(got, projection) {
		t.Fatalf("nonSource=%v want %v", got, projection)
	}
	if plan := taskStatePlan(t, nil, "notes.md"); plan.Scope != ScopeUnknown || !slices.Equal(unknownDetails(plan), []string{"LANGUAGE_FRONTIER x:dynamic-dispatch", "UNOWNED_DIRTY_PATH notes.md"}) || len(plan.NonSource) != 0 {
		t.Fatalf("an unowned non-projection path: scope=%s unknown=%v nonSource=%v", plan.Scope, unknownDetails(plan), plan.NonSource)
	}
}

// AFP-V0-088 (4): beside a source change the projection adds nothing: the
// selection and the unknown entries are those of the source change alone.
func TestTaskStoreProjectionBesideASourceChangeAddsNothing_AFPV0088(t *testing.T) {
	alone := taskStatePlan(t, nil, "core.x")
	mixed := taskStatePlan(t, nil, ".taskman/queue.json", "core.x")
	if !slices.Equal(unknownDetails(alone), unknownDetails(mixed)) || len(alone.Selected) != len(mixed.Selected) {
		t.Fatalf("alone unknown=%v selected=%d; mixed unknown=%v selected=%d", unknownDetails(alone), len(alone.Selected), unknownDetails(mixed), len(mixed.Selected))
	}
	for index := range alone.Selected {
		if alone.Selected[index].UnitID != mixed.Selected[index].UnitID || alone.Selected[index].Witness.Kind != mixed.Selected[index].Witness.Kind {
			t.Fatalf("alone=%+v mixed=%+v", alone.Selected, mixed.Selected)
		}
	}
	if !slices.Equal(nonSourcePaths(mixed), []string{".taskman/queue.json"}) {
		t.Fatalf("nonSource=%v", mixed.NonSource)
	}
}

// AFP-V0-088 (2): a projection path a unit names by path literal, or holds
// in its declared read scope, is read by a test: that unit is selected and
// the path keeps its UNOWNED_DIRTY_PATH entry, while an unnamed sibling is
// still task state. An unbounded reader is selected on the projection but
// does not count as naming it; only its own plugin's frontier, as a reached
// unit's, remains, because the unbounded set is a traversal closure and no
// terminal selection (AFP-V0-560).
func TestTaskStoreProjectionReaderIsSelectedAndKeepsItUnknown_AFPV0088(t *testing.T) {
	cases := []struct {
		name    string
		edit    func(*fake)
		witness string
	}{
		{"file-name literal", func(language *fake) { language.units[3].PathTokens = []string{"policy.json"} }, WitnessPathLiteralReader},
		{"anchored literal", func(language *fake) { language.units[3].PathTokens = []string{".taskman/policy.json"} }, WitnessPathLiteralReader},
		{"declared read scope", func(language *fake) {
			language.units[3].Sources = []string{"solo/solo.x"}
			language.units[3].Tests = []string{"solo/solo_test.x"}
			language.units[3].ReadScoped = true
			language.units[3].ReadScope = []string{".taskman/policy.json"}
		}, WitnessDeclaredReadScope},
	}
	for _, tc := range cases {
		plan := taskStatePlan(t, tc.edit, ".taskman/policy.json", ".taskman/tickets/V1-0001.json")
		if got := witnesses(plan)["x:solo"]; got.Kind != tc.witness || got.DirtyPath != ".taskman/policy.json" {
			t.Fatalf("%s: selected=%+v", tc.name, plan.Selected)
		}
		if plan.Scope != ScopeUnknown || !slices.Contains(unknownDetails(plan), "UNOWNED_DIRTY_PATH .taskman/policy.json") {
			t.Fatalf("%s: scope=%s unknown=%v", tc.name, plan.Scope, unknownDetails(plan))
		}
		if !slices.Equal(nonSourcePaths(plan), []string{".taskman/tickets/V1-0001.json"}) {
			t.Fatalf("%s: nonSource=%v", tc.name, plan.NonSource)
		}
	}
	plan := taskStatePlan(t, func(language *fake) { language.units[3].UnboundedReads = "root-locator" }, ".taskman/queue.json")
	if got := witnesses(plan)["x:solo"]; got.Kind != WitnessUnboundedReader || !slices.Equal(unknownDetails(plan), []string{"LANGUAGE_FRONTIER x:dynamic-dispatch"}) || plan.FrontierExcluded != nil || !slices.Equal(nonSourcePaths(plan), []string{".taskman/queue.json"}) {
		t.Fatalf("unbounded reader: scope=%s selected=%+v nonSource=%v unknown=%v excluded=%+v", plan.Scope, plan.Selected, plan.NonSource, unknownDetails(plan), plan.FrontierExcluded)
	}
}

// excludesSolo reports whether the plan records x's frontier as left out for
// its terminal reader x:solo alone (AFP-V0-560).
func excludesSolo(plan Plan) bool {
	return len(plan.FrontierExcluded) == 1 && plan.FrontierExcluded[0].Language == "x" &&
		slices.Equal(plan.FrontierExcluded[0].Frontier, []string{"x:dynamic-dispatch"}) &&
		slices.Equal(plan.FrontierExcluded[0].Units, []string{"x:solo"})
}

// AFP-V0-088 (2), V1-2069: a literal names a projection file only through its
// whole file name. A directory fragment (`/tickets/`, `.taskman/tickets/`,
// `/.taskman`) or a file-name fragment (the `.taskman/tickets/V1-` of
// `".taskman/tickets/V1-%04d.json"`) still selects its unit as a path-literal
// reader, but names a family of records, not this one, so the path is task
// state and the reader's frontier is left out (AFP-V0-560); the whole name,
// alone or with its directories, keeps it unknown.
func TestTaskStoreProjectionFragmentLiteralDoesNotNameARecord_AFPV0088(t *testing.T) {
	const ticket = ".taskman/tickets/V1-0001.json"
	for _, tc := range []struct {
		token, dirty string
		named        bool
	}{
		{"/tickets/", ticket, false},
		{"/tickets", ticket, false},
		{".taskman/tickets/", ticket, false},
		{"/.taskman", ticket, false},
		{"/.taskman/", ".taskman/queue.json", false},
		{".taskman/tickets/V1-", ticket, false},
		{".taskman/releases/", ".taskman/releases/v1-0.json", false},
		{".taskman/tickets/V1-0001.json", ticket, true},
		{"tickets/V1-0001.json", ticket, true},
		{"V1-0001.json", ticket, true},
		{"/.taskman/queue.json", ".taskman/queue.json", true},
	} {
		plan := taskStatePlan(t, func(language *fake) { language.units[3].PathTokens = []string{tc.token} }, tc.dirty)
		if got := witnesses(plan)["x:solo"]; got.Kind != WitnessPathLiteralReader || got.DirtyPath != tc.dirty {
			t.Fatalf("%q: the literal's unit must stay selected: %+v", tc.token, plan.Selected)
		}
		unowned := slices.Contains(unknownDetails(plan), "UNOWNED_DIRTY_PATH "+tc.dirty)
		if tc.named {
			if !unowned || len(plan.NonSource) != 0 {
				t.Fatalf("%q names %s: unknown=%v nonSource=%v", tc.token, tc.dirty, unknownDetails(plan), plan.NonSource)
			}
			continue
		}
		if unowned || !slices.Equal(nonSourcePaths(plan), []string{tc.dirty}) || len(plan.Unknown) != 0 || !excludesSolo(plan) {
			t.Fatalf("%q must not name %s: unknown=%v nonSource=%v excluded=%+v", tc.token, tc.dirty, unknownDetails(plan), plan.NonSource, plan.FrontierExcluded)
		}
	}
}

// AFP-V0-088 (1): only the recognised projection layout is task state.
func TestTaskStoreProjectionLayoutIsClosed_AFPV0088(t *testing.T) {
	for path, want := range map[string]bool{
		".taskman/policy.json":           true,
		".taskman/queue.json":            true,
		".taskman/tickets/V1-2049.json":  true,
		".taskman/releases/v1-0.json":    true,
		".taskman/notes.md":              false,
		".taskman/queue.json.bak":        false,
		".taskman/tickets/.json":         false,
		".taskman/tickets/nested/a.json": false,
		".taskman/tickets/V1-2049.md":    false,
		".taskman/claims/lane.json":      false,
		".taskman":                       false,
		"sub/.taskman/queue.json":        false,
		"taskman/queue.json":             false,
		".taskmanx/queue.json":           false,
	} {
		if got := IsTaskStoreProjectionPath(path); got != want {
			t.Errorf("%s: %v, want %v", path, got, want)
		}
	}
	plan := taskStatePlan(t, nil, ".taskman/notes.md")
	if plan.Scope != ScopeUnknown || len(plan.NonSource) != 0 || !slices.Contains(unknownDetails(plan), "UNOWNED_DIRTY_PATH .taskman/notes.md") {
		t.Fatalf("scope=%s unknown=%v nonSource=%v", plan.Scope, unknownDetails(plan), plan.NonSource)
	}
}

// AFP-V0-088 (3): the plan wire names the class and reason, and is unchanged
// when nothing is task state.
func TestTaskStoreProjectionWire_AFPV0088(t *testing.T) {
	without, err := json.Marshal(taskStatePlan(t, nil, "core.x"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(without), "nonSource") {
		t.Fatalf("a plan without task state names nonSource: %s", without)
	}
	with, err := json.Marshal(taskStatePlan(t, nil, ".taskman/queue.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := `"nonSource":[{"class":"TASK_STORE_PROJECTION","reason":"` + taskStoreProjectionReason + `","paths":[".taskman/queue.json"]}]`
	if !strings.Contains(string(with), want) {
		t.Fatalf("wire=%s want %s", with, want)
	}
}
