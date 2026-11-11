// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"slices"
	"testing"
)

// narrowableScan scans script as a bash CI step whose environment exports
// nothing, so a plain local assignment is narrowed unless something revokes it.
func narrowableScan(script string) *driverScan {
	scan := newDriverScan()
	scan.envKnown, scan.envNames = true, map[string]bool{}
	scan.scanShell(script, ".", "example.test/m")
	return scan
}

// AFP-V0-9130: a backslash-newline the shell removes is joined exactly, so a
// multi-line `$(...)` or command is one line and no longer revokes narrowing;
// one the shell keeps (single quotes, comments, an escaped backslash) is not
// joined; quoting the joiner does not follow falls back to the revoke.
func TestDriverLineContinuations_AFP_V0_9130(t *testing.T) {
	for _, tc := range []struct {
		name, script string
		computed     bool
	}{
		{"substitution", "set -euo pipefail\nfiles=$(git diff --name-only \\\n  base head)\ngo test ./core/...\n", false},
		{"substitution-quoted-parens", "x=1\nv=$(printf '%s' \"a(\" \\\n  \"b)\")\ngo test ./core/...\n", false},
		{"command", "x=1\ngo test \\\n  ./core/...\n", false},
		{"double-quoted", "x=1\nmsg=\"a \\\n b\"\ngo test ./core/...\n", false},
		{"comment", "x=1\n# note \\\ngo test ./core/...\n", false},
		{"escaped-backslash", "x=1\necho a\\\\\ngo test ./core/...\n", false},
		{"single-quoted", "x=1\necho 'a\\\nb'\ngo test ./core/...\n", true},
		{"backtick-fallback", "x=1\ny=`echo a \\\nb`\ngo test ./core/...\n", true},
		{"ansi-quote-fallback", "x=1\ny=$'a\\\nb'\ngo test ./core/...\n", true},
		{"unterminated-fallback", "x=1\necho \"a \\\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scan := narrowableScan(tc.script)
			if scan.computed != tc.computed {
				t.Fatalf("computed = %v, reasons %v; want %v", scan.computed, scan.reasons, tc.computed)
			}
			if tc.name != "unterminated-fallback" && !scan.paths["core/..."] {
				t.Fatalf("go test package not recorded: paths %v", scan.paths)
			}
		})
	}
}

// AFP-V0-9130: driverJoinContinuations removes exactly the backslash-newlines
// bash removes, and declines text whose quoting it does not follow.
func TestDriverJoinContinuations_AFP_V0_9130(t *testing.T) {
	for _, tc := range []struct {
		in, want string
		ok       bool
	}{
		{"a \\\nb", "a b", true},
		{"a \"x \\\ny\" b", "a \"x y\" b", true},
		{"a 'x \\\ny' b", "a 'x \\\ny' b", true},
		{"a\\\\\nb", "a\\\\\nb", true},
		{"x=$(foo \\\n  bar) \\\n z", "x=$(foo   bar)  z", true},
		{"x=\"$(foo 'a' \\\n \"b)\")\"", "x=\"$(foo 'a'  \"b)\")\"", true},
		{"a # c \\\nb", "a # c \\\nb", true},
		{"a#c \\\nb", "a#c b", true},
		{"${x:-\"a \\\nb\"}", "${x:-\"a b\"}", true},
		{"a `b \\\nc`", "a `b \\\nc`", false},
		{"a '`$\"<<' \\\nb", "a '`$\"<<' b", true},
		{"a \"x<<y \\\nb\"", "a \"x<<y b\"", true},
		{"a $'b \\\nc'", "a $'b \\\nc'", false},
		{"cat <<EOF \\\nx\nEOF\n", "cat <<EOF \\\nx\nEOF\n", false},
		{"a \"b \\\n", "a \"b \\\n", false},
		{"a $(b \\\n", "a $(b \\\n", false},
	} {
		got, ok := driverJoinContinuations(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("join(%q) = %q, %v; want %q, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

// AFP-V0-9130: a function or alias named as a command the scanner models
// keeps the site computed; a function of an unknown name changes nothing the
// scanner trusted.
func TestDriverFunctionShadowing_AFP_V0_9130(t *testing.T) {
	for _, tc := range []struct {
		name, script, reason string
	}{
		{"posix-function", "cat() { ./tools/run \"$@\"; }\ncat README.md\n", "shell:function-shadow"},
		{"keyword-function", "function go { ./tools/run \"$@\"; }\ngo test ./core/...\n", "shell:function-shadow"},
		{"alias", "alias cat=./tools/run\ncat README.md\n", "shell:builtin-alias"},
		{"unknown-name", "helper() { go test ./core/...; }\nhelper\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scan := newDriverScan()
			scan.scanShell(tc.script, ".", "example.test/m")
			if tc.reason == "" {
				if scan.computed {
					t.Fatalf("unknown function name computed the site: %v", scan.reasons)
				}
				if !scan.paths["core/..."] {
					t.Fatalf("function body not scanned: %v", scan.paths)
				}
				return
			}
			if !scan.computed || !scan.reasons[tc.reason] {
				t.Fatalf("computed = %v, reasons %v; want %s", scan.computed, scan.reasons, tc.reason)
			}
		})
	}
}

// AFP-V0-9132: a command word built from a step output is computed unless
// every write of the producing step is a literal.
func TestDriverStepOutputCommandWord_AFP_V0_9132(t *testing.T) {
	workflow := "jobs:\n" +
		"  computed:\n    runs-on: x\n    steps:\n" +
		"      - id: x\n        run: echo \"y=$(cmd)\" >> \"$GITHUB_OUTPUT\"\n" +
		"      - run: ${{ steps.x.outputs.y }}/run\n" +
		"  literal:\n    runs-on: x\n    steps:\n" +
		"      - id: x\n        run: |\n          # the tool directory\n          echo \"y=tools/ci\" >> \"$GITHUB_OUTPUT\"\n          echo z=1 >> $GITHUB_OUTPUT\n" +
		"      - run: ${{ steps.x.outputs.y }}/run\n" +
		"  action:\n    runs-on: x\n    steps:\n" +
		"      - id: x\n        uses: example/choose@v1\n" +
		"      - run: ${{ steps.x.outputs.y }}/run\n" +
		"  mixed:\n    runs-on: x\n    steps:\n" +
		"      - id: x\n        run: |\n          echo \"y=tools/ci\" >> \"$GITHUB_OUTPUT\"\n          ./tools/emit.sh\n" +
		"      - run: ${{ steps.x.outputs.y }}/run\n" +
		"  either:\n    runs-on: x\n    steps:\n" +
		"      - id: x\n        run: echo \"y=tools/ci\" >> \"$GITHUB_OUTPUT\"\n" +
		"      - id: z\n        run: echo \"y=$(cmd)\" >> \"$GITHUB_OUTPUT\"\n" +
		"      - run: ${{ steps.x.outputs.y || steps.z.outputs.y }}/run\n" +
		"  expression:\n    runs-on: x\n    steps:\n" +
		"      - id: x\n        run: echo \"y=${{ inputs.dir }}\" >> \"$GITHUB_OUTPUT\"\n" +
		"      - run: ${{ steps.x.outputs.y }}/run\n" +
		"  other:\n    runs-on: x\n    steps:\n" +
		"      - id: x\n        run: echo \"y=$(cmd)\" >> \"$GITHUB_OUTPUT\"\n" +
		"      - run: ${{ github.workspace }}/tools/run\n"
	reasons := driverComputedReasons(t, memorySource(t, map[string]string{
		"go.mod":                   "module example.test/m\n",
		".github/workflows/ci.yml": workflow,
	}))
	id := func(job string) string { return "driver:ci:.github/workflows/ci.yml#" + job }
	for _, job := range []string{"computed", "action", "mixed", "either", "expression"} {
		if !slices.Contains(reasons[id(job)], "shell:computed-command-word") {
			t.Errorf("%s reasons %v, want shell:computed-command-word", job, reasons[id(job)])
		}
	}
	for _, job := range []string{"literal", "other"} {
		if slices.Contains(reasons[id(job)], "shell:computed-command-word") {
			t.Errorf("%s reasons %v, want no computed command word", job, reasons[id(job)])
		}
	}
}
