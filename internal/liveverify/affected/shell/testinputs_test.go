// SPDX-License-Identifier: Apache-2.0
package shell

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AFP-V0-2400 (V1-3489): a declared shell test input joins the test unit's
// path globs, so a dirty path it holds selects the test by PATH_GLOB_READER
// naming the declared glob, while a path outside every input does not (the
// negative control). The declaration only adds: the test keeps the literal it
// observed itself. An invalid declaration, or an entry for no observed shell
// test, adds nothing and names FrontierShellTestInputsInvalid.
func TestShellTestInputsDeclaration_AFPV02400(t *testing.T) {
	base := map[string]string{
		"t/scan_test.sh":       "printf '%s\\n' Makefile | xargs grep -n python3\nfind script -maxdepth 1 -name '*.sh'\ncat conf/settings.txt\n",
		"Makefile":             "all:\n",
		"script/a.sh":          "echo a\n",
		"script/deep/b.sh":     "echo b\n",
		"integrations/x/y.mjs": "export {}\n",
		"conf/settings.txt":    "a=1\n",
		"docs/notes.txt":       "notes\n",
	}
	declare := func(test, inputs, reason string) string {
		return `{"profile":"corvint-shell-test-inputs/0","tests":{"` + test + `":{"inputs":` + inputs + `,"reason":"` + reason + `"}}}`
	}
	good := declare("t/scan_test.sh", `["Makefile","integrations/","script/*.sh"]`, "the scan prints Makefile and lists script/*.sh")
	for _, tc := range []struct {
		name, declaration string
		declared, invalid bool
	}{
		{name: "undeclared"},
		{name: "declared", declaration: good, declared: true},
		{name: "absent test", declaration: declare("t/gone_test.sh", `["Makefile"]`, "r"), invalid: true},
		{name: "source script", declaration: declare("script/a.sh", `["Makefile"]`, "r"), invalid: true},
		{name: "unsorted", declaration: declare("t/scan_test.sh", `["script/*.sh","Makefile"]`, "r"), invalid: true},
		{name: "empty inputs", declaration: declare("t/scan_test.sh", `[]`, "r"), invalid: true},
		{name: "wildcard directory", declaration: declare("t/scan_test.sh", `["*/a.sh"]`, "r"), invalid: true},
		{name: "bad pattern", declaration: declare("t/scan_test.sh", `["script/[.sh"]`, "r"), invalid: true},
		{name: "absolute", declaration: declare("t/scan_test.sh", `["/Makefile"]`, "r"), invalid: true},
		{name: "git", declaration: declare("t/scan_test.sh", `[".git/"]`, "r"), invalid: true},
		{name: "empty reason", declaration: declare("t/scan_test.sh", `["Makefile"]`, ""), invalid: true},
		{name: "unknown member", declaration: `{"profile":"corvint-shell-test-inputs/0","tests":{"t/scan_test.sh":{"inputs":["Makefile"],"reason":"r","reads":[]}}}`, invalid: true},
		{name: "bad profile", declaration: `{"profile":"corvint-shell-test-inputs/1","tests":{"t/scan_test.sh":{"inputs":["Makefile"],"reason":"r"}}}`, invalid: true},
		{name: "unparsed", declaration: `{`, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := maps.Clone(base)
			if tc.declaration != "" {
				files[ShellTestInputsPath] = tc.declaration
			}
			root := t.TempDir()
			for relative, content := range files {
				write(t, root, relative, content)
			}
			result := units(t, root)
			if got := slices.ContainsFunc(result.Frontier, func(reason string) bool { return strings.HasPrefix(reason, FrontierShellTestInputsInvalid) }); got != tc.invalid {
				t.Errorf("invalid frontier %v, want %v (frontier %v)", got, tc.invalid, result.Frontier)
			}
			graph := build(t, root)
			selects := func(dirty string) (affected.Selection, bool) {
				for _, selection := range affected.Select(graph, []string{dirty}).Selected {
					if selection.UnitID == "shell:t/scan_test.sh" {
						return selection, true
					}
				}
				return affected.Selection{}, false
			}
			for dirty, glob := range map[string]string{"Makefile": "/Makefile", "script/a.sh": "/script/*.sh", "integrations/x/y.mjs": "/integrations"} {
				selection, ok := selects(dirty)
				if ok != tc.declared || (ok && (selection.Witness.Kind != affected.WitnessPathGlobReader || selection.Witness.Glob != glob)) {
					t.Errorf("%s: selected %v by %+v, want %v by glob %s", dirty, ok, selection.Witness, tc.declared, glob)
				}
			}
			for _, dirty := range []string{"docs/notes.txt", "script/deep/b.sh"} {
				if _, ok := selects(dirty); ok {
					t.Errorf("%s is outside every declared input yet selected the test", dirty)
				}
			}
			if selection, ok := selects("conf/settings.txt"); !ok || selection.Witness.Kind != affected.WitnessPathLiteralReader {
				t.Errorf("the observed literal was lost: %v %+v", ok, selection.Witness)
			}
		})
	}
}

// AFP-V0-2400 (review round d P1): the declaration is read when the graph is
// built, so a plan over an edited declaration judges its inputs by the edit. A
// change that drops `script/*.sh` (or deletes the declaration) together with a
// fault in script/a.sh, which the scan test reads only through that input,
// would select nothing for it. Any dirty declaration, deletion included, makes
// the plan UNKNOWN with the shell:test-inputs-changed frontier, even where
// the runtime-readers declaration names the file's readers; the declaration
// alone left clean (control) adds no such unknown.
func TestShellTestInputsDeclarationChanged_AFPV02400(t *testing.T) {
	const frontier = "shell:test-inputs-changed"
	for _, tc := range []struct {
		name, declaration string
		dirty             []string
		unknown           bool
	}{
		{name: "removed input", declaration: `["Makefile","integrations/"]`, dirty: []string{ShellTestInputsPath, "script/a.sh"}, unknown: true},
		{name: "deleted declaration", dirty: []string{ShellTestInputsPath, "script/a.sh"}, unknown: true},
		{name: "clean declaration", declaration: `["Makefile","integrations/","script/*.sh"]`, dirty: []string{"script/a.sh"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "t/scan_test.sh", "printf '%s\\n' Makefile | xargs grep -n python3\nfind script -maxdepth 1 -name '*.sh'\n")
			write(t, root, "Makefile", "all:\n")
			write(t, root, "script/a.sh", "python3 -c 'print(1)'\n")
			write(t, root, "integrations/x/y.mjs", "export {}\n")
			write(t, root, ".corvint/runtime-readers.json", `{"profile":"corvint-runtime-readers/0","entries":[{"path":".corvint/shell-test-inputs.json","reason":"r"}]}`) //corvint:fixture-path
			if tc.declaration != "" {
				write(t, root, ShellTestInputsPath, `{"profile":"corvint-shell-test-inputs/0","tests":{"t/scan_test.sh":{"inputs":`+tc.declaration+`,"reason":"r"}}}`)
			}
			plan := affected.Select(build(t, root), tc.dirty)
			if got := hasUnknown(plan, affected.UnknownLanguageFrontier, frontier); got != tc.unknown || (tc.unknown && plan.Scope != affected.ScopeUnknown) {
				t.Errorf("frontier %v scope %s, want frontier %v (unknown %+v)", got, plan.Scope, tc.unknown, plan.Unknown)
			}
			if !tc.unknown && !slices.ContainsFunc(plan.Selected, func(selection affected.Selection) bool { return selection.UnitID == "shell:t/scan_test.sh" }) {
				t.Errorf("the declared input no longer selects the scan test: %+v", plan.Selected)
			}
		})
	}
}
