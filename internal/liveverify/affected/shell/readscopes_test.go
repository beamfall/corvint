package shell

import (
	"maps"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TSP-V0-950: a declared toolchain-running shell test is read-scoped to its
// declared reads instead of being an unbounded reader, so an unrelated change
// no longer selects it while a dirty declared read or a file in its own
// directory does. A missing declaration changes nothing; a declaration that is
// invalid or names anything other than a toolchain-only unbounded test
// declares nothing and names FrontierShellReadScopesInvalid; a dirty
// declaration leaves the plan UNKNOWN.
func TestShellReadScopeDeclaration_TSP_V0_950(t *testing.T) {
	base := map[string]string{
		"t/build_test.sh":     "go build ./tool\n",
		"t/dynamic_test.sh":   "go build ./tool\n. \"$ENVFILE\"\n",
		"t/plain_test.sh":     "echo plain\n",
		"t/helped_test.sh":    "go build ./tool\n. lib/dynamic.sh\n",
		"t/twohop_test.sh":    "go build ./tool\n. lib/outer.sh\n",
		"lib/outer.sh":        ". lib/dynamic.sh\n",
		"lib/dynamic.sh":      ". \"$DYNAMIC\"\n",
		"t/notes.txt":         "beside the test\n",
		"script/helper.sh":    "echo helper\n",
		"tool/main.txt":       "what the toolchain reads\n",
		"other/other_test.sh": "echo unrelated\n",
	}
	declare := func(test, reads, reason string) string {
		return `{"profile":"corvint-shell-read-scopes/0","tests":{"` + test + `":{"reads":` + reads + `,"reason":"` + reason + `"}}}`
	}
	for _, tc := range []struct {
		name        string
		declaration string
		declared    bool
		invalid     bool
	}{
		{name: "undeclared"},
		{name: "declared", declaration: declare("t/build_test.sh", `["go.mod","tool/"]`, "go build reads the tool package"), declared: true},
		{name: "dynamic source too", declaration: declare("t/dynamic_test.sh", `["tool/"]`, "r"), invalid: true},
		{name: "sourced helper sources a dynamic path", declaration: declare("t/helped_test.sh", `["tool/"]`, "r"), invalid: true},
		{name: "dynamic path two sources away", declaration: declare("t/twohop_test.sh", `["tool/"]`, "r"), invalid: true},
		{name: "no toolchain", declaration: declare("t/plain_test.sh", `["tool/"]`, "r"), invalid: true},
		{name: "source script", declaration: declare("script/helper.sh", `["tool/"]`, "r"), invalid: true},
		{name: "absent test", declaration: declare("t/gone_test.sh", `["tool/"]`, "r"), invalid: true},
		{name: "one of two absent", declaration: `{"profile":"corvint-shell-read-scopes/0","tests":{"t/build_test.sh":{"reads":["tool/"],"reason":"r"},"t/gone_test.sh":{"reads":["tool/"],"reason":"r"}}}`, invalid: true},
		{name: "unsorted reads", declaration: declare("t/build_test.sh", `["tool/","go.mod"]`, "r"), invalid: true},
		{name: "empty reads", declaration: declare("t/build_test.sh", `[]`, "r"), invalid: true},
		{name: "git entry", declaration: declare("t/build_test.sh", `[".git/"]`, "r"), invalid: true},
		{name: "absolute entry", declaration: declare("t/build_test.sh", `["/tool"]`, "r"), invalid: true},
		{name: "empty reason", declaration: declare("t/build_test.sh", `["tool/"]`, ""), invalid: true},
		{name: "multi-line reason", declaration: declare("t/build_test.sh", `["tool/"]`, `a\nb`), invalid: true},
		{name: "bad profile", declaration: `{"profile":"corvint-shell-read-scopes/9","tests":{"t/build_test.sh":{"reads":["tool/"],"reason":"r"}}}`, invalid: true},
		{name: "no tests", declaration: `{"profile":"corvint-shell-read-scopes/0","tests":{}}`, invalid: true},
		{name: "unknown member", declaration: `{"profile":"corvint-shell-read-scopes/0","tests":{"t/build_test.sh":{"reads":["tool/"],"reason":"r","callers":[]}}}`, invalid: true},
		{name: "unparsed", declaration: `{`, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := maps.Clone(base)
			if tc.declaration != "" {
				files[affected.ShellReadScopesPath] = tc.declaration
			}
			root := t.TempDir()
			for relative, content := range files {
				write(t, root, relative, content)
			}
			result := units(t, root)
			test := unitOf(t, result, "t/build_test.sh")
			if got := test.UnboundedReads == ""; got != tc.declared {
				t.Errorf("bounded %v, want %v (%+v)", got, tc.declared, test)
			}
			if test.ReadScoped != tc.declared {
				t.Errorf("read-scoped %v, want %v (%+v)", test.ReadScoped, tc.declared, test)
			}
			for _, file := range []string{"t/helped_test.sh", "t/twohop_test.sh"} {
				if helped := unitOf(t, result, file); helped.ReadScoped || len(helped.Imports) == 0 {
					t.Errorf("%s sources a helper that reads a dynamic path, yet is read-scoped or has no edge: %+v", file, helped)
				}
			}
			if dynamic := unitOf(t, result, "t/dynamic_test.sh"); dynamic.UnboundedReads != UnboundedRunsToolchain || dynamic.ReadScoped {
				t.Errorf("a toolchain test that sources a dynamic path changed: %+v", dynamic)
			}
			if got := slices.Contains(result.Frontier, FrontierShellReadScopesInvalid); got != tc.invalid {
				t.Errorf("invalid frontier %v, want %v (frontier %v)", got, tc.invalid, result.Frontier)
			}

			graph := build(t, root)
			selects := func(dirty string) (affected.Selection, bool) {
				for _, selection := range affected.Select(graph, []string{dirty}).Selected {
					if selection.UnitID == "shell:t/build_test.sh" {
						return selection, true
					}
				}
				return affected.Selection{}, false
			}
			unrelated, selected := selects("other/other_test.sh")
			if selected == tc.declared {
				t.Errorf("unrelated change selected the test %v, want %v (%+v)", selected, !tc.declared, unrelated)
			}
			if !tc.declared {
				if unrelated.Witness.Kind != affected.WitnessUnboundedReader {
					t.Errorf("undeclared witness %+v, want an unbounded reader", unrelated.Witness)
				}
			} else {
				if plan := affected.Select(graph, []string{"other/other_test.sh"}); plan.Scope != affected.ScopeBounded {
					t.Errorf("unrelated change: scope %s (unknown %v), want BOUNDED", plan.Scope, plan.Unknown)
				}
				for _, dirty := range []string{"tool/main.txt", "go.mod", "t/notes.txt"} {
					if selection, ok := selects(dirty); !ok || selection.Witness.Kind != affected.WitnessDeclaredReadScope {
						t.Errorf("%s: selected %v by %+v, want a declared read scope", dirty, ok, selection.Witness)
					}
				}
			}
			if tc.declaration == "" {
				return
			}
			changed := affected.Select(graph, []string{affected.ShellReadScopesPath})
			if changed.Scope != affected.ScopeUnknown || !hasUnknown(changed, affected.UnknownLanguageFrontier, affected.FrontierShellReadScopesChanged) {
				t.Errorf("dirty declaration: scope %s, unknown %v; want the changed frontier", changed.Scope, changed.Unknown)
			}
		})
	}
}
