package affected

import (
	"context"
	json "encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// frontierScopeLoader is the s plugin's dynamic-import frontier reason, the
// only one the fixtures raise plugin-wide.
const frontierScopeLoader = "s:dynamic-import:loader.s:3 `import`"

// frontierScopePlan builds a g plugin (a Go-like core and its importer) beside
// an s plugin with a dynamic-import frontier, whose reader names core.g and
// whose loader is the site of the dynamic import, after edit, over a root
// holding declaration as .corvint/documentation-paths.json (none when empty),
// and plans dirty.
func frontierScopePlan(t *testing.T, declaration string, edit func(g, s *fake), dirty ...string) Plan {
	t.Helper()
	g := fake{name: "g", units: []Unit{
		{ID: "g:core", Sources: []string{"core.g"}, Tests: []string{"core_test.g"}},
		{ID: "g:app", Sources: []string{"app.g"}, Tests: []string{"app_test.g"}, Imports: []string{"g:core"}},
	}}
	s := fake{name: "s", frontier: []string{frontierScopeLoader}, units: []Unit{
		{ID: "s:reader", Sources: []string{"reader.s"}, Tests: []string{"reader_test.s"}, PathTokens: []string{"core.g"}},
		{ID: "s:loader", Sources: []string{"loader.s"}, Tests: []string{"loader_test.s"}},
	}}
	if edit != nil {
		edit(&g, &s)
	}
	root := t.TempDir()
	if declaration != "" {
		if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(DocumentationPathsPath)), []byte(declaration), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	graph, err := Build(context.Background(), root, g, s)
	if err != nil {
		t.Fatal(err)
	}
	return Select(graph, dirty)
}

// frontierOf is the plan's scope and LANGUAGE_FRONTIER details.
func frontierOf(plan Plan) string {
	frontier := []string{}
	for _, unknown := range plan.Unknown {
		if unknown.Reason == UnknownLanguageFrontier {
			frontier = append(frontier, unknown.Detail)
		}
	}
	return plan.Scope + " " + fmt.Sprint(frontier)
}

// AFP-V0-560 (a): an unrelated dynamic import of another plugin does not make
// a change of g alone UNKNOWN, also when the change reaches a reader of that
// plugin, which is a terminal selection; the left-out frontier is recorded with
// its reason and the reader, and the selection is unchanged.
func TestUnrelatedDynamicImportDoesNotWidenAnotherLanguagesChange_AFP_V0_560(t *testing.T) {
	plan := frontierScopePlan(t, "", nil, "core.g")
	if got := frontierOf(plan); got != "BOUNDED []" {
		t.Fatalf("plan %s unknown=%+v", got, plan.Unknown)
	}
	selected := witnesses(plan)
	if len(selected) != 3 || selected["g:core"].Kind != WitnessDirectSource || selected["g:app"].Kind != WitnessDependency || selected["s:reader"].Kind != WitnessPathLiteralReader {
		t.Fatalf("selected=%+v", plan.Selected)
	}
	want := []FrontierExclusion{{Language: "s", Reason: FrontierExclusionReason, Frontier: []string{frontierScopeLoader}, Units: []string{"s:reader"}}}
	if !reflect.DeepEqual(plan.FrontierExcluded, want) {
		t.Fatalf("frontierExcluded=%+v want %+v", plan.FrontierExcluded, want)
	}
	// A change that reaches no unit of s involves s as before and records nothing.
	plan = frontierScopePlan(t, "", nil, "app.g")
	if got := frontierOf(plan); got != "BOUNDED []" || plan.FrontierExcluded != nil {
		t.Fatalf("plan %s frontierExcluded=%+v", got, plan.FrontierExcluded)
	}
	encoded, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "frontierExcluded") {
		t.Fatalf("an empty exclusion record must be omitted: %s", encoded)
	}
}

// AFP-V0-560 (b): a change to a file the dynamic import could load, in its
// unit or in a reader of that plugin, or to a path no plugin owns, keeps the
// frontier and records no exclusion.
func TestChangeTheDynamicImportCanReachStaysUnknown_AFP_V0_560(t *testing.T) {
	for _, dirty := range [][]string{
		{"loader.s"},
		{"core.g", "reader.s"},
		{"core.g", "payload.json"},
		{"widget.s"},
	} {
		plan := frontierScopePlan(t, "", nil, dirty...)
		if got := frontierOf(plan); !strings.HasPrefix(got, "UNKNOWN ") || !strings.Contains(got, frontierScopeLoader) {
			t.Errorf("dirty %v: plan %s, want the s frontier", dirty, got)
		}
		if plan.FrontierExcluded != nil {
			t.Errorf("dirty %v: frontierExcluded=%+v", dirty, plan.FrontierExcluded)
		}
	}
}

// AFP-V0-560 (c): a change that reaches a unit of the frontier's plugin through
// an edge the traversal continues on, that the plugin reads any path for, or
// that reaches a terminal unit with its own unit frontier, keeps that frontier.
func TestChangeInTheFrontierPluginsParticipantSetStaysUnknown_AFP_V0_560(t *testing.T) {
	cases := map[string]struct {
		edit func(g, s *fake)
		want string
	}{
		"cross-language import": {func(_, s *fake) {
			s.units = append(s.units, Unit{ID: "s:bridge", Sources: []string{"bridge.s"}, Tests: []string{"bridge_test.s"}, Imports: []string{"g:core"}})
		}, "UNKNOWN [" + frontierScopeLoader + "]"},
		"reader's unit frontier": {func(_, s *fake) {
			s.units[0].Frontier = []string{"s:variant"}
		}, "UNKNOWN [s:variant]"},
	}
	for name, tc := range cases {
		if got := frontierOf(frontierScopePlan(t, "", tc.edit, "core.g")); got != tc.want {
			t.Errorf("%s: plan %s, want %s", name, got, tc.want)
		}
	}
	g := fake{name: "g", units: []Unit{{ID: "g:core", Sources: []string{"core.g"}, Tests: []string{"core_test.g"}}}}
	s := pathReading{fake{name: "s", frontier: []string{frontierScopeLoader}}}
	graph, err := Build(context.Background(), t.TempDir(), g, s)
	if err != nil {
		t.Fatal(err)
	}
	if got := frontierOf(Select(graph, []string{"core.g"})); got != "UNKNOWN ["+frontierScopeLoader+"]" {
		t.Fatalf("a plugin that reads any path: plan %s", got)
	}
}

// AFP-V0-560: a test-less reader that testlessReaderUsers traverses from a
// declared documentation path is no terminal selection, so its plugin's
// frontier stays: the dynamic import may be the edge to the reader's users.
func TestTraversedTestlessReaderKeepsItsPluginsFrontier_AFP_V0_560(t *testing.T) {
	declaration := `{"profile":"corvint-documentation-paths/0","paths":["docs/"]}`
	edit := func(_, s *fake) {
		s.units = append(s.units, Unit{ID: "s:config", Sources: []string{"config.s"}, PathTokens: []string{"docs/guide.md"}})
	}
	plan := frontierScopePlan(t, declaration, edit, "docs/guide.md")
	if got := frontierOf(plan); got != "UNKNOWN ["+frontierScopeLoader+"]" || plan.FrontierExcluded != nil {
		t.Fatalf("plan %s frontierExcluded=%+v", got, plan.FrontierExcluded)
	}
	// The same reader with tests is terminal and recorded instead.
	plan = frontierScopePlan(t, declaration, func(g, s *fake) {
		edit(g, s)
		s.units[2].Tests = []string{"config_test.s"}
	}, "docs/guide.md")
	if got := frontierOf(plan); got != "BOUNDED []" || len(plan.FrontierExcluded) != 1 || !reflect.DeepEqual(plan.FrontierExcluded[0].Units, []string{"s:config"}) {
		t.Fatalf("plan %s frontierExcluded=%+v", got, plan.FrontierExcluded)
	}
}

// AFP-V0-560: an unbounded reader is no terminal selection. The unbounded set
// is the traversal closure of the root locators, so a dynamic import into a
// locator could add an unbounded reader the plan misses: the plugin's frontier
// stays for a root locator and for an importer of one alike.
func TestUnboundedReaderClosureKeepsItsPluginsFrontier_AFP_V0_560(t *testing.T) {
	for name, edit := range map[string]func(g, s *fake){
		"root locator": func(_, s *fake) {
			s.units = append(s.units, Unit{ID: "s:locator", Sources: []string{"locator.s"}, Tests: []string{"locator_test.s"}, LocatesRoot: true, UnboundedReads: "root-locator"})
		},
		// A reader unbounded by its own reads, not by locating the root, reached
		// first as a path-literal reader of the dirty path (risk found merging
		// AFP-V0-560 with AFP-V0-581).
		"own unbounded reader reached by literal": func(_, s *fake) {
			s.units[0].PathTokens = []string{"app.g"}
			s.units[0].UnboundedReads = "escaping-literal"
		},
		"importer of a root locator": func(_, s *fake) {
			s.units = append(s.units,
				Unit{ID: "s:locator", Sources: []string{"locator.s"}, LocatesRoot: true, UnboundedReads: "root-locator"},
				Unit{ID: "s:user", Sources: []string{"user.s"}, Tests: []string{"user_test.s"}, Imports: []string{"s:locator"}})
		},
	} {
		plan := frontierScopePlan(t, "", edit, "app.g")
		if got := frontierOf(plan); got != "UNKNOWN ["+frontierScopeLoader+"]" || plan.FrontierExcluded != nil {
			t.Errorf("%s: plan %s selected=%+v frontierExcluded=%+v", name, got, plan.Selected, plan.FrontierExcluded)
		}
	}
}

// AFP-V0-560 with AFP-V0-511: a dirty path no plugin owns turns the terminal
// rule off, so a plugin reached only through a terminal reader takes part with
// every frontier entry, even one it declares source-bound, and nothing is
// recorded as excluded. Without the unowned path the terminal rule leaves it
// out and records it; with only the unowned path it raises its path-open
// entries alone (AFP-V0-511 (3)).
func TestUnownedPathKeepsTerminalReadersPluginWholeFrontier_AFP_V0_560_AFP_V0_511(t *testing.T) {
	plan := func(dirty ...string) Plan {
		t.Helper()
		g := fake{name: "g", units: []Unit{
			{ID: "g:core", Sources: []string{"core.g"}, Tests: []string{"core_test.g"}},
		}}
		s := reaching{
			fake: fake{name: "s", frontier: []string{frontierScopeLoader}, units: []Unit{
				{ID: "s:reader", Sources: []string{"reader.s"}, Tests: []string{"reader_test.s"}, PathTokens: []string{"core.g"}},
				{ID: "s:loader", Sources: []string{"loader.s"}, Tests: []string{"loader_test.s"}},
			}},
			sourceBound: []string{"s:dynamic-import"},
		}
		graph, err := Build(context.Background(), t.TempDir(), g, s)
		if err != nil {
			t.Fatal(err)
		}
		return Select(graph, dirty)
	}
	both := plan("core.g", "payload.json")
	if got := frontierOf(both); !strings.HasPrefix(got, "UNKNOWN ") || !strings.Contains(got, frontierScopeLoader) || both.FrontierExcluded != nil {
		t.Fatalf("unowned and terminal: plan %s frontierExcluded=%+v", got, both.FrontierExcluded)
	}
	terminal := plan("core.g")
	if got := frontierOf(terminal); got != "BOUNDED []" || len(terminal.FrontierExcluded) != 1 {
		t.Fatalf("terminal only: plan %s frontierExcluded=%+v", got, terminal.FrontierExcluded)
	}
	unowned := plan("payload.json")
	if got := frontierOf(unowned); strings.Contains(got, frontierScopeLoader) || unowned.FrontierExcluded != nil {
		t.Fatalf("unowned only: plan %s frontierExcluded=%+v", got, unowned.FrontierExcluded)
	}
}
