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
		{"no-link-check", strings.ReplaceAll(source, `if (stat $file)[3] != 1`, "if 0"), false},
		// Trap strings are not parsed as the file's commands: a DEBUG trap runs
		// before the source line, and a signal handler can rewrite command.
		{"inert-traps", "f() { exit 1; }\ntrap '/bin/rm -rf \"$w\"' EXIT\ntrap 'exit 130' INT TERM\ntrap 'f 143' HUP\ntrap - EXIT INT\n" + source, true},
		{"heavy-serve-traps", `heavy_cleanup_run_tmp() {
  [[ -n ${run_tmp-} ]] && { chmod -R u+rwx "$run_tmp"; /bin/rm -rf "$run_tmp"; }
}
trap 'git -C "${clone:-/nonexistent}" update-ref -d "refs/heavy/$id" > /dev/null 2>&1
  heavy_cleanup_run_tmp > /dev/null 2>&1
  /bin/rm -rf "$w"' EXIT
trap 'exit 130' INT TERM HUP
` + source, true},
		{"heavy-offload-traps", `heavy_abort() { exit "$1"; }
trap '/bin/rm -rf "${heavy_tmp-}"' EXIT
trap 'heavy_abort 130' INT
trap 'heavy_abort 143' TERM
trap 'heavy_abort 129' HUP
` + source, true},
		{"cleanup-commands", `trap 'rm -f x; /bin/rm -f y; rmdir x; /bin/rmdir y' EXIT
` + source, true},
		{"trap-bash", `trap 'bash -c "$SETUP"' EXIT
` + source, false},
		{"trap-env", "trap 'env X=1 sh' EXIT\n" + source, false},
		{"trap-env-wrapper", "trap 'env rm -f x' EXIT\n" + source, false},
		{"trap-git-config-option", "trap 'git -c core.hooksPath=/x update-ref -d r' EXIT\n" + source, false},
		{"trap-git-config-env", "trap 'git --config-env=x=y update-ref -d r' EXIT\n" + source, false},
		{"trap-git-exec-path", "trap 'git --exec-path=/x update-ref -d r' EXIT\n" + source, false},
		{"trap-git-config-command", "trap 'git -C /x config x y' EXIT\n" + source, false},
		{"trap-git-expanded-dir", "trap 'git -C $dir update-ref -d r' EXIT\n" + source, false},
		{"trap-git-expanded-array", `trap 'git -C "$@" update-ref -d r' EXIT
` + source, false},
		{"trap-git-glob-dir", "trap 'git -C /x/* update-ref -d r' EXIT\n" + source, false},
		{"trap-git-missing-dir", "trap 'git -C' EXIT\n" + source, false},
		{"trap-git-repeated-dir", "trap 'git -C /x -C y update-ref -d r' EXIT\n" + source, true},
		{"debug-trap", "trap 'command() { printf /repo/file; }' DEBUG\n" + source, false},
		{"return-trap", "trap 'true' RETURN\n" + source, false},
		{"signal-trap-rewrite", "trap 'command() { printf /repo/file; }' INT\n" + source, false},
		{"exit-trap-source", "trap '. ./setup.sh' EXIT\n" + source, false},
		{"trap-eval", "trap 'eval \"$SETUP\"' TERM\n" + source, false},
		{"trap-dynamic-command", "trap '$handler' INT\n" + source, false},
		{"trap-assignment", "trap 'HOME=/repo' INT\n" + source, false},
		// perl -MCwd and -MFile::Basename load modules through @INC and PERL5OPT.
		{"perl5lib-export", "export PERL5LIB=/repo/lib\n" + source, false},
		{"perl5lib-split-quote", "export P''ERL5LIB=/x\n" + source, false},
		{"perl5opt-escape", "export P\\ERL5OPT=-Mx\n" + source, false},
		{"perl5lib-quoted", "export \"PERL5LIB\"=/x\n" + source, false},
		{"perl5lib-continuation", "PE\\\nRL5LIB=/x\n" + source, false},
		{"not-found-split-quote", "command_not''_found_handle() { :; }\n" + source, false},
		{"perl5opt-assign", "f() {\nPERL5OPT=-Mbad\n" + source + "}\n", false},
		{"perllib-declare", "declare -x PERLLIB=/repo/lib\n" + source, false},
		{"perl5-other", "export PERL5DB='BEGIN { require \"/repo/x\" }'\n" + source, false},
		{"not-found-handler", "command_not_found_handle() { printf /repo/file; }\n" + source, false},
		{"not-found-keyword", "function command_not_found_handle { printf /repo/file; }\n" + source, false},
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
