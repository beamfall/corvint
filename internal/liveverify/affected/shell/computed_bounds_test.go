// SPDX-License-Identifier: Apache-2.0

package shell

import (
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AFP-V0-3146 (V1-4283, review fix 7): a value bound of the computed-target
// expansion that replaces a variable's values with the wildcard is a cut, as
// the Cartesian bound already was (AFP-V0-3144). A loop over more than
// maxNameValues words drops every one of them, the one naming an owned
// extensionless script included, so the caller is an unbounded reader rather
// than a file that runs nothing.
func TestPerVariableBoundKeepsTheCallerSelected_AFPV03146(t *testing.T) {
	// script/check sources an environment-named file, so it is itself a
	// dynamic source: any edit, a configuration file's included, may change
	// what it runs.
	const check = "#!/bin/sh\n. \"$ENVFILE\"\necho check\n"
	const wrapper = "for dir in a b c d e f g h i j k l m n o p script; do \"$dir/check\"; done\n"
	root := t.TempDir()
	write(t, root, "script/check", check)
	write(t, root, "use/wrap_test.sh", wrapper)
	write(t, root, "config/settings.env", "MODE=fast\n")
	if use := unitOf(t, units(t, root), "use/wrap_test.sh"); use.UnboundedReads != UnboundedDynamicSource {
		t.Errorf("wrap_test.sh unbounded reads %q imports %v, want %q: the 17-word loop's dropped value may be script/check",
			use.UnboundedReads, use.Imports, UnboundedDynamicSource)
	}
	graph := build(t, root)
	for _, dirty := range []string{"config/settings.env", "script/check"} {
		plan := affected.Select(graph, []string{dirty})
		if !slices.ContainsFunc(plan.Selected, func(s affected.Selection) bool { return s.UnitID == "shell:use/wrap_test.sh" }) {
			t.Errorf("an edit of %s did not select use/wrap_test.sh, which may run script/check through \"$dir/check\"", dirty)
		}
	}
}

// AFP-V0-3147: every value bound of the expansion that stands the wildcard
// in for values the file does give is a cut: the per-variable count, the
// depth of a variable chain, of a nested default and of a `:+` alternative,
// and the expansion of a command string held in a variable. A value the file
// never gives (an environment input) is no cut: nothing was dropped.
func TestEveryExpansionBoundIsACut_AFPV03147(t *testing.T) {
	const check = "#!/bin/sh\necho check\n"
	seventeen := func(name, last string) string {
		var lines []string
		for _, letter := range strings.Fields("a b c d e f g h i j k l m n o p") {
			lines = append(lines, name+"="+letter)
		}
		return strings.Join(append(lines, name+"="+last), "\n") + "\n"
	}
	for _, tc := range []struct{ name, body string }{
		{name: "seventeen assignments", body: seventeen("dir", "script") + "\"$dir/check\"\n"},
		{name: "seventeen assignments braced", body: seventeen("dir", "script") + "\"${dir}/check\"\n"},
		{name: "a variable chain past the depth", body: "e=script\nd=$e\nc=$d\nb=$c\na=$b\n\"$a/check\"\n"},
		{name: "a nested default past the depth", body: "\"${a:-${b:-${c:-${d:-${e:-script}}}}}/check\"\n"},
		{name: "an alternative past the depth", body: "\"${a:+${b:+${c:+${d:+${e:+script/check}}}}}\"\n"},
		{name: "a command string past the per-variable bound", body: seventeen("cmd", "script/check") + "sh -c \"$cmd\"\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "script/check", check)
			write(t, root, "use/use_test.sh", tc.body)
			if use := unitOf(t, units(t, root), "use/use_test.sh"); use.UnboundedReads != UnboundedDynamicSource && !slices.Contains(use.Imports, "shell:script/check") {
				t.Errorf("unbounded reads %q imports %v, want %q or the script/check edge: a bound dropped a value that may be script/check",
					use.UnboundedReads, use.Imports, UnboundedDynamicSource)
			}
		})
	}
	t.Run("an environment input is no cut", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "script/check", check)
		write(t, root, "use/probe.sh", "\"$SDK/go\" version\n\"$TOOLS/check\"\n")
		if use := unitOf(t, units(t, root), "use/probe.sh"); use.UnboundedReads != "" {
			t.Errorf("unbounded reads %q, want none: no bound dropped a value the file gives", use.UnboundedReads)
		}
	})
	t.Run("sixteen values stay bounded and link", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "script/check", check)
		write(t, root, "use/use_test.sh", "for dir in a b c d e f g h i j k l m n o script; do \"$dir/check\"; done\n")
		use := unitOf(t, units(t, root), "use/use_test.sh")
		if use.UnboundedReads != "" || !slices.Equal(use.Imports, []string{"shell:script/check"}) {
			t.Errorf("unbounded reads %q imports %v, want the script/check edge alone", use.UnboundedReads, use.Imports)
		}
	})
}

// AFP-V0-3148: a value the file repeats verbatim counts once toward the
// per-name bound, so the same short word list in many loops stays expanded
// and bounded; seventeen distinct values are still a cut (AFP-V0-3146).
func TestRepeatedValuesCountOnce_AFPV03148(t *testing.T) {
	const check = "#!/bin/sh\necho check\n"
	var loops strings.Builder
	for range 6 {
		loops.WriteString("for dir in a b script; do \"$dir/check\"; done\n")
	}
	root := t.TempDir()
	write(t, root, "script/check", check)
	write(t, root, "use/use_test.sh", loops.String())
	use := unitOf(t, units(t, root), "use/use_test.sh")
	if use.UnboundedReads != "" || !slices.Equal(use.Imports, []string{"shell:script/check"}) {
		t.Errorf("unbounded reads %q imports %v, want the script/check edge alone: 18 words with 3 distinct values are within the bound",
			use.UnboundedReads, use.Imports)
	}
}
