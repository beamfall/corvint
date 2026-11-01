package affected

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// runtimeEdgeSite is the s plugin's end-to-end runtime frontier entry, whose
// site is the spec the g server's change can break at run time.
const runtimeEdgeSite = "s:e2e-runtime:spec.s:1 `runner`"

// runtimeEdgePlan builds a g plugin (a server package and the command that
// builds it) beside an s plugin whose reader names server.g and whose spec
// drives the built server at run time, an edge neither plugin models. The s
// plugin declares its e2e-runtime reason cross-language when cross is set,
// the root holds edges as .corvint/runtime-edges.json when it is not empty,
// and dirty is planned.
func runtimeEdgePlan(t *testing.T, cross bool, edges string, dirty ...string) (Plan, error) {
	t.Helper()
	g := fake{name: "g", units: []Unit{
		{ID: "g:server", Sources: []string{"server.g"}, Tests: []string{"server_test.g"}},
		{ID: "g:cmd", Sources: []string{"cmd.g"}, Imports: []string{"g:server"}},
	}}
	s := fake{name: "s", frontier: []string{runtimeEdgeSite}, units: []Unit{
		{ID: "s:reader", Sources: []string{"reader.s"}, Tests: []string{"reader_test.s"}, PathTokens: []string{"server.g"}},
		{ID: "s:spec", Tests: []string{"spec.s"}},
	}}
	if cross {
		s.crossLanguage = []string{"s:e2e-runtime"}
	}
	root := t.TempDir()
	if edges != "" {
		if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(RuntimeEdgesPath)), []byte(edges), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	graph, err := Build(context.Background(), root, g, s)
	if err != nil {
		return Plan{}, err
	}
	return Select(graph, dirty), nil
}

const runtimeEdgeDeclaration = `{"profile":"corvint-runtime-edges/0","edges":[{"tests":"spec.s","runs":"g:cmd","reason":"the spec drives the server cmd builds"}]}`

// AFP-V0-818 (V1-2906): a change that reaches the s plugin only through a
// terminal reader no longer leaves out s's cross-language runtime entry, so
// the plan stays UNKNOWN naming it instead of excluding it; a reason the
// plugin does not declare cross-language is still excluded (AFP-V0-560).
func TestCrossLanguageRuntimeFrontierIsNeverExcluded_AFP_V0_818(t *testing.T) {
	plan, err := runtimeEdgePlan(t, true, "", "server.g")
	if err != nil {
		t.Fatal(err)
	}
	if got := frontierOf(plan); got != "UNKNOWN ["+runtimeEdgeSite+"]" {
		t.Fatalf("plan %s, want the cross-language entry named", got)
	}
	if plan.FrontierExcluded != nil {
		t.Fatalf("frontierExcluded=%+v, want none", plan.FrontierExcluded)
	}
	if selected := witnesses(plan); len(selected) != 2 || selected["s:reader"].Kind != WitnessPathLiteralReader {
		t.Fatalf("selected=%+v", plan.Selected)
	}
	plan, err = runtimeEdgePlan(t, false, "", "server.g")
	if err != nil {
		t.Fatal(err)
	}
	want := []FrontierExclusion{{Language: "s", Reason: FrontierExclusionReason, Frontier: []string{runtimeEdgeSite}, Units: []string{"s:reader"}}}
	if got := frontierOf(plan); got != "BOUNDED []" || !reflect.DeepEqual(plan.FrontierExcluded, want) {
		t.Fatalf("an undeclared reason: plan %s frontierExcluded=%+v", got, plan.FrontierExcluded)
	}
}

// AFP-V0-819 (V1-2906): a declared runtime edge selects the spec with a
// BINARY_EXEC witness through the command it runs whenever the server's
// change reaches that command, and models the entry's site, so the plan is
// BOUNDED; a change that reaches no command it runs leaves the spec out.
func TestDeclaredRuntimeEdgeSelectsTheSpec_AFP_V0_819(t *testing.T) {
	plan, err := runtimeEdgePlan(t, true, runtimeEdgeDeclaration, "server.g")
	if err != nil {
		t.Fatal(err)
	}
	selected := witnesses(plan)
	if spec, ok := selected["s:spec"]; !ok || spec.Kind != WitnessBinaryExec || spec.DirtyPath != "server.g" {
		t.Fatalf("the declared edge did not select the spec: %+v", plan.Selected)
	}
	if got := frontierOf(plan); got != "BOUNDED []" {
		t.Fatalf("plan %s unknown=%+v, want the modelled site left out", got, plan.Unknown)
	}
	plan, err = runtimeEdgePlan(t, true, runtimeEdgeDeclaration, "reader.s")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := witnesses(plan)["s:spec"]; ok {
		t.Fatalf("a change that builds no server selected the spec: %+v", plan.Selected)
	}
}

// AFP-V0-819: an invalid or stale declaration fails the build, naming the
// declaration, so it never silently stops selecting.
func TestRuntimeEdgesDeclarationRefusals_AFP_V0_819(t *testing.T) {
	for name, c := range map[string]struct{ edges, want string }{
		"unknown unit":     {`{"profile":"corvint-runtime-edges/0","edges":[{"tests":"spec.s","runs":"g:gone","reason":"r"}]}`, `runs "g:gone" names no unit`},
		"no test held":     {`{"profile":"corvint-runtime-edges/0","edges":[{"tests":"e2e/","runs":"g:cmd","reason":"r"}]}`, `tests "e2e/" holds no test file`},
		"wrong profile":    {`{"profile":"corvint-runtime-edges/1","edges":[]}`, "profile must be"},
		"unknown member":   {`{"profile":"corvint-runtime-edges/0","edges":[],"x":1}`, RuntimeEdgesPath},
		"blank reason":     {`{"profile":"corvint-runtime-edges/0","edges":[{"tests":"spec.s","runs":"g:cmd","reason":""}]}`, "reason of"},
		"unqualified runs": {`{"profile":"corvint-runtime-edges/0","edges":[{"tests":"spec.s","runs":"cmd","reason":"r"}]}`, `runs "cmd"`},
		"escaping tests":   {`{"profile":"corvint-runtime-edges/0","edges":[{"tests":"../spec.s","runs":"g:cmd","reason":"r"}]}`, `tests "../spec.s"`},
		"descending":       {`{"profile":"corvint-runtime-edges/0","edges":[{"tests":"z.s","runs":"g:cmd","reason":"r"},{"tests":"spec.s","runs":"g:cmd","reason":"r"}]}`, "must ascend"},
	} {
		_, err := runtimeEdgePlan(t, true, c.edges, "server.g")
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err=%v, want %q", name, err, c.want)
		}
	}
}
