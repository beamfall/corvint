// SPDX-License-Identifier: Apache-2.0
package shell

import (
	"strings"
	"testing"
)

// AFP-V0-3341: only the complete, quoted validator is a bounded host source.
// The same structure works under an unrelated script name; a changed guard,
// command rewrite or additional source must retain the conservative floor.
func TestHostEnvironmentSource_AFPV03341(t *testing.T) {
	source := ". " + hostEnvironmentOperand + " || exit 97\n"
	cases := []struct {
		name, text string
		bounded    bool
	}{
		{"guarded", source, true},
		{"source-builtin", "source " + hostEnvironmentOperand + "\n", true},
		{"in-function", "f() {\n" + source + "}\n", true},
		{"unguarded", ". \"$ENVFILE\"\n", false},
		{"other-source", source + ". \"$OTHER\"\n", false},
		{"no-realpath", strings.ReplaceAll(source, "realpath(shift @ARGV)", "shift @ARGV"), false},
		{"no-checkout-check", strings.ReplaceAll(source, `-e "$dir/.git"`, "0"), false},
		{"no-root-check", strings.ReplaceAll(source, `index($file, "$root/") == 0`, "0"), false},
		{"unquoted", ". " + strings.Trim(hostEnvironmentOperand, `"`) + "\n", false},
		{"suffix", ". " + hostEnvironmentOperand + "/other\n", false},
		{"prior-source", ". ./setup.sh\n" + source, false},
		// A function body runs in call order: a source later in the text can
		// still run first and replace command.
		{"later-source", "f() {\n" + source + "}\n. ./setup.sh\nf\n", false},
		{"command-shadow", "command() { printf /repo/file; }\n" + source, false},
		{"alias", "alias command=echo\n" + source, false},
		{"eval", "eval \"$SETUP\"\n" + source, false},
		{"prefix", "PERL5OPT=-Mbad " + source, false},
		{"redirect", strings.TrimSuffix(source, " || exit 97\n") + " > out\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "script/another-name.sh", tc.text)
			write(t, root, "setup.sh", "command() { printf /repo/file; }\n")
			got := unitOf(t, units(t, root), "script/another-name.sh")
			if tc.bounded {
				if got.UnboundedReads != "" || got.LocatesRoot {
					t.Fatalf("bounded form: %+v", got)
				}
			} else if got.UnboundedReads != UnboundedDynamicSource || !got.LocatesRoot {
				t.Fatalf("unproved source lost dynamic-source: %+v", got)
			}
		})
	}
}
