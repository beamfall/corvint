// SPDX-License-Identifier: Apache-2.0

package shell

import (
	"slices"
	"strings"
	"testing"
)

// AFP-V0-3145 (V1-4283, review fix 6; amends NSA-V0-008): `[[` opens a
// conditional expression only in command position, at the start of a command
// or after the reserved words if, while, until, !, time, then, do, else and
// elif. A `[[` that is an argument (`echo [[ marker`) leaves `&&`, `||`, `|`
// and `;` as command separators, so the command after it is still scanned:
// an environment-named source there keeps shell:dynamic-source and a literal
// source keeps its edge.
func TestConditionalOpensOnlyInCommandPosition_AFPV03145(t *testing.T) {
	t.Run("an argument [[ does not swallow the next command", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "lib.sh", "true\n")
		forms := map[string]string{
			"and.sh":    "echo [[ marker && . \"$ENVFILE\"\n",
			"or.sh":     "printf '%s\\n' x [[ || source \"$ENVFILE\"\n",
			"pipe.sh":   "echo [[ | . \"$ENVFILE\"\n",
			"if-arg.sh": "if echo [[ x && . \"$ENVFILE\"; then :; fi\n",
		}
		for name, text := range forms {
			write(t, root, name, text)
		}
		write(t, root, "literal.sh", "echo [[ marker && . ./lib.sh\n")
		result := units(t, root)
		for name := range forms {
			if unit := unitOf(t, result, name); unit.UnboundedReads != UnboundedDynamicSource {
				t.Errorf("%s: the source after an argument [[ was lost: unbounded reads %q, want %q", name, unit.UnboundedReads, UnboundedDynamicSource)
			}
		}
		if unit := unitOf(t, result, "literal.sh"); !slices.Equal(unit.Imports, []string{"shell:lib.sh"}) {
			t.Errorf("literal.sh imports %v, want the lib.sh edge after an argument [[", unit.Imports)
		}
	})
	t.Run("a test in command position keeps its operators", func(t *testing.T) {
		// Split at `||`, the operand "$dir/x.sh" would be a computed run
		// target building a script name, which puts the foreign frontier on
		// the unit.
		test := `[[ -n ${a-} || "$dir/x.sh" == y ]]`
		for _, text := range []string{
			test + "\n",
			test + " && true\n",
			"true && " + test + "\n",
			"true || ! " + test + "\n",
			"true | " + test + "\n",
			"true; " + test + "\n",
			"if " + test + "; then :; fi\n",
			"if ! " + test + "; then :; fi\n",
			"while " + test + "; do :; done\n",
			"until " + test + "; do :; done\n",
			"if true; then " + test + "; fi\n",
			"if true; then :; elif " + test + "; then :; fi\n",
			"if true; then :; else " + test + "; fi\n",
			"for v in 1; do " + test + "; done\n",
			"{ " + test + "; }\n",
			"time " + test + "\n",
			"f() { " + test + "; }\n",
			"( " + test + " )\n",
			"case $x in a) " + test + " ;; esac\n",
		} {
			root := t.TempDir()
			write(t, root, "use.sh", text)
			unit := unitOf(t, units(t, root), "use.sh")
			if len(unit.Frontier) != 0 || unit.UnboundedReads != "" || len(unit.Imports) != 0 {
				t.Errorf("%q: a test operand was scanned as a command: %+v", strings.TrimSpace(text), unit)
			}
		}
	})
	t.Run("parsed commands", func(t *testing.T) {
		for _, tc := range []struct {
			text string
			want [][]string
		}{
			{"echo [[ marker && . \"$ENVFILE\"\n", [][]string{{"echo", "[[", "marker"}, {".", `"$ENVFILE"`}}},
			{"if ! [[ a || b ]]; then :; fi\n", [][]string{{"if", "!", "[[", "a", "||", "b", "]]"}, {"then", ":"}, {"fi"}}},
			{"[[ a ]] || [[ b && c ]]\n", [][]string{{"[[", "a", "]]"}, {"[[", "b", "&&", "c", "]]"}}},
		} {
			var got [][]string
			for _, cmd := range parse(tc.text).commands {
				got = append(got, plainWords(cmd.words))
			}
			if !slices.EqualFunc(got, tc.want, slices.Equal[[]string]) {
				t.Errorf("%q parsed as %q, want %q", strings.TrimSpace(tc.text), got, tc.want)
			}
		}
	})
}
