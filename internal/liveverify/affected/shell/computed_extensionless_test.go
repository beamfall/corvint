// SPDX-License-Identifier: Apache-2.0

package shell

import (
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AFP-V0-3144 (V1-4283, review fix 6): a computed run target whose file name
// is literal and carries no shell suffix is still expanded when an owned
// extensionless script (AFP-V0-2205) has that file name, so the value that
// names it stays an import edge and a change to the script selects the
// callers. The shortcut that skips the expansion holds only where no owned
// file carries the name: no value can then link an edge, so a value bound
// that cuts the list drops nothing.
func TestComputedTargetKeepsOwnedExtensionlessScripts_AFPV03144(t *testing.T) {
	const check = "#!/bin/sh\necho check\n"
	loops := func(target string) string {
		letters := "a b c d e f g h i"
		return "for x in " + letters + "\ndo\n  for y in " + letters + "\n  do\n    \"$x/$y/" + target + "\"\n  done\ndone\n"
	}
	t.Run("loop over an extensionless root links the script", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "script/check", check)
		write(t, root, "use/use.sh", "for dir in script; do \"$dir/check\"; done\n")
		use := unitOf(t, units(t, root), "use/use.sh")
		if !slices.Equal(use.Imports, []string{"shell:script/check"}) || use.UnboundedReads != "" || len(use.Frontier) != 0 {
			t.Errorf("use imports %v unbounded %q frontier %v, want the script/check edge alone", use.Imports, use.UnboundedReads, use.Frontier)
		}
	})
	t.Run("a value relative to the file's directory links the script", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "script/check", check)
		write(t, root, "script/run.sh", "for d in .; do \"$d/check\"; done\n")
		if run := unitOf(t, units(t, root), "script/run.sh"); !slices.Equal(run.Imports, []string{"shell:script/check"}) {
			t.Errorf("run imports %v, want the script/check edge", run.Imports)
		}
	})
	t.Run("an edit of the script selects the test that runs it", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "script/check", check)
		write(t, root, "script/other", check)
		write(t, root, "use/wrap_test.sh", "for dir in script; do \"$dir/check\"; done\n")
		graph := build(t, root)
		selected := func(dirty string) bool {
			plan := affected.Select(graph, []string{dirty})
			return slices.ContainsFunc(plan.Selected, func(s affected.Selection) bool { return s.UnitID == "shell:use/wrap_test.sh" })
		}
		if !selected("script/check") {
			t.Error("an edit of script/check did not select use/wrap_test.sh, which runs it through \"$dir/check\"")
		}
		if selected("script/other") {
			t.Error("an edit of script/other selected use/wrap_test.sh, which never runs it")
		}
	})
	t.Run("a cut list whose file name an owned script has is unbounded", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "script/check", check)
		write(t, root, "use/use_test.sh", loops("check"))
		if use := unitOf(t, units(t, root), "use/use_test.sh"); use.UnboundedReads != UnboundedDynamicSource {
			t.Errorf("unbounded reads %q, want %q: a dropped value may be script/check", use.UnboundedReads, UnboundedDynamicSource)
		}
	})
	for _, tc := range []struct{ name, target string }{
		{name: "a binary no owned file names stays bounded", target: "go"},
		{name: "a dotted name no owned file carries stays bounded", target: "check.py"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "script/check", check)
			// A non-test caller, like the repository's toolchain probe: a test
			// that runs go is a toolchain reader whatever its targets name.
			write(t, root, "use/probe.sh", loops(tc.target))
			use := unitOf(t, units(t, root), "use/probe.sh")
			if use.UnboundedReads != "" || use.LocatesRoot || len(use.Imports) != 0 {
				t.Errorf("unbounded reads %q imports %v, want neither: no owned file is named %s", use.UnboundedReads, use.Imports, tc.target)
			}
		})
	}
}
