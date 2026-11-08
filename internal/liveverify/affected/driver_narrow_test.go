package affected

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"
)

// driverStepScan scans script as a workflow step whose env blocks declare
// env (nil: unknown, as for make recipes and composite actions) and returns
// the sorted computed reasons, empty when the step is not computed.
func driverStepScan(t *testing.T, script string, env map[string]bool) ([]string, *driverScan) {
	t.Helper()
	scan := newDriverScan()
	scan.envKnown, scan.envNames = env != nil, env
	scan.scanShell(script, ".", "example.test/m")
	if scan.computed != (len(scan.reasons) != 0) {
		t.Fatalf("%q: computed=%v with reasons %v", script, scan.computed, scan.reasons)
	}
	reasons := make([]string, 0, len(scan.reasons))
	for reason := range scan.reasons {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	return reasons, scan
}

type driverNarrowCase struct {
	script string
	env    map[string]bool
	reason string // "" when the step must not be computed
}

func runDriverNarrowCases(t *testing.T, cases []driverNarrowCase) {
	t.Helper()
	for _, tc := range cases {
		reasons, _ := driverStepScan(t, tc.script, tc.env)
		if tc.reason == "" && len(reasons) != 0 {
			t.Errorf("%q (env %v): computed %v, want narrowed", tc.script, tc.env, reasons)
		}
		if tc.reason != "" && !slices.Contains(reasons, tc.reason) {
			t.Errorf("%q (env %v): reasons %v, want %s", tc.script, tc.env, reasons, tc.reason)
		}
	}
}

var driverNoEnv = map[string]bool{}

// AFP-V0-9621: only allowlisted `set` options are narrowed; every
// neighbouring form stays computed.
func TestDriverSetNarrowing_AFP_V0_9621(t *testing.T) {
	runDriverNarrowCases(t, []driverNarrowCase{
		{"set -euo pipefail\ngo test ./...", driverNoEnv, ""},
		{"set -eu", driverNoEnv, ""},
		{"set +e\nfalse\nset -e", driverNoEnv, ""},
		{"set -o pipefail -o errexit +o nounset", driverNoEnv, ""},
		{"set -Eeuxo pipefail", driverNoEnv, ""},
		{"set -v", driverNoEnv, ""},
		{"set -e", nil, ""}, // No environment is read: narrowed for make too.
		// Fault injection: unsafe neighbours stay computed.
		{"set -a", driverNoEnv, "shell:builtin-set"},
		{"set -o allexport", driverNoEnv, "shell:builtin-set"},
		{"set -ea", driverNoEnv, "shell:builtin-set"},
		{"set -- $pair", driverNoEnv, "shell:builtin-set"},
		{"set -f", driverNoEnv, "shell:builtin-set"},
		{"set -o noglob", driverNoEnv, "shell:builtin-set"},
		{"set -o", driverNoEnv, "shell:builtin-set"},
		{"set -oe pipefail", driverNoEnv, "shell:builtin-set"},
		{"set x y", driverNoEnv, "shell:builtin-set"},
		{"set", driverNoEnv, "shell:builtin-set"},
		{"set -x", map[string]bool{"ps4": true}, "shell:builtin-set"},
		{"set -o xtrace", nil, "shell:builtin-set"},
	})
}

// AFP-V0-9622: a standalone assignment to a safe lower-case name is a plain
// shell variable; every unsafe neighbour stays computed.
func TestDriverLocalAssignment_AFP_V0_9622(t *testing.T) {
	runDriverNarrowCases(t, []driverNarrowCase{
		{"out=dist\necho \"$out\"", driverNoEnv, ""},
		{"x=1 y=2", driverNoEnv, ""},
		{"base=$(git rev-parse HEAD)\ngo test ./...", driverNoEnv, ""},
		{"count+=1", driverNoEnv, ""},
		{"if ok=1; then :; fi", driverNoEnv, ""},
		{"for f in a b; do echo \"$f\"; done", driverNoEnv, ""},
		// Fault injection: names a shell, zsh or a tool gives a meaning.
		{"PATH=/x", driverNoEnv, "shell:assignment"},
		{"path=/x", driverNoEnv, "shell:assignment"},
		{"Path=x", driverNoEnv, "shell:assignment"},
		{"ifs=x", driverNoEnv, "shell:assignment"},
		{"http_proxy=x", driverNoEnv, "shell:assignment"},
		{"no_proxy=x", driverNoEnv, "shell:assignment"},
		{"cdpath=x", driverNoEnv, "shell:assignment"},
		{"bash_env=x", driverNoEnv, "shell:assignment"},
		{"lc_all=C", driverNoEnv, "shell:assignment"},
		{"_=x", driverNoEnv, "shell:assignment"},
		// Declared by an env block, in any case, or an unknown environment.
		{"out=1", map[string]bool{"out": true}, "shell:assignment"},
		{"out=1", nil, "shell:assignment"},
		// Not standalone: the command's own environment.
		{"out=1 go test ./...", driverNoEnv, "shell:assignment"},
		{"env out=1 go test ./...", driverNoEnv, "shell:assignment"},
		{"nohup out=1 go test ./...", driverNoEnv, "shell:assignment"},
		{"out=1 exec go test ./...", driverNoEnv, "shell:assignment"},
		// Exported later, or assigned in another shell.
		{"out=1\nexport out", driverNoEnv, "shell:builtin-export"},
		{"out=1\ndeclare -x out", driverNoEnv, "shell:builtin-declare"},
		{"out=1\ntypeset -x out", driverNoEnv, "shell:builtin-typeset"},
		{"set -a\nout=1", driverNoEnv, "shell:builtin-set"},
		{"zsh -c 'out=1'", driverNoEnv, "shell:assignment"},
		// A value prefix naming the working directory or escaping it.
		{"dir=.", driverNoEnv, "shell:assignment"},
		{"dir=../x", driverNoEnv, "shell:assignment"},
		// Later used to name or prefix a command, anywhere in the script.
		{"d=tools\n$d/run.sh", driverNoEnv, "shell:local-command-word"},
		{"d=tools\n\"${d}/run.sh\"", driverNoEnv, "shell:local-command-word"},
		{"d=tools\nbash \"$d/run.sh\"", driverNoEnv, "shell:local-command-word"},
		{"d=tools\nsudo $d/run.sh", driverNoEnv, "shell:local-command-word"},
		{"d=tools\n. \"$d/env.sh\"", driverNoEnv, "shell:local-command-word"},
		{"t=x\ntrap \"$t/cleanup\" EXIT", driverNoEnv, "shell:local-command-word"},
		{"t=x\nfind . -exec $t/run {} \\;", driverNoEnv, "shell:local-command-word"},
		{"t=x\necho a | xargs $t/run", driverNoEnv, "shell:local-command-word"},
		{"t=x\ncat <<EOF\n$($t/run)\nEOF", driverNoEnv, "shell:local-command-word"},
		{"$d/run.sh\nd=tools", driverNoEnv, "shell:local-command-word"},
		{"for f in a; do $f/run.sh; done", driverNoEnv, "shell:local-command-word"},
		{"for PATH in /x; do :; done", driverNoEnv, "shell:loop-variable"},
		{"for out in a; do :; done", map[string]bool{"out": true}, "shell:loop-variable"},
		// Fault injection: an unmodelled $X/ command is still root-like (AZU parity).
		{"$X/run.sh", driverNoEnv, ""},
	})
}

// AFP-V0-9622: a literal value prefix is retained as a read path and its
// directory token, so later `$d/...` operands stay covered.
func TestDriverLocalAssignmentPaths_AFP_V0_9622(t *testing.T) {
	_, scan := driverStepScan(t, "d=tools/gen\ncat \"$d/x/input.json\"", driverNoEnv)
	if scan.computed || !scan.paths["tools/gen"] || !scan.tokens["/tools/gen/"] {
		t.Fatalf("computed=%v paths=%v tokens=%v", scan.computed, scan.paths, scan.tokens)
	}
	scan = newDriverScan()
	scan.envKnown, scan.envNames = true, driverNoEnv
	scan.scanShell("cd \"$X\"\nd=tools", "", "example.test/m")
	if !scan.computed {
		t.Fatal("a relative value in an unknown directory must stay computed")
	}
}

// AFP-V0-9624: `read -r` into safe names is narrowed; other reads are not.
func TestDriverReadNarrowing_AFP_V0_9624(t *testing.T) {
	runDriverNarrowCases(t, []driverNarrowCase{
		{"while IFS= read -r line; do echo \"$line\"; done < go.mod", driverNoEnv, ""},
		{"read -r flag pattern pkg < x", driverNoEnv, ""},
		{"read -r -a argv < x", driverNoEnv, "shell:builtin-read"},
		{"read line < x", driverNoEnv, "shell:builtin-read"},
		{"read -r PATH < x", driverNoEnv, "shell:builtin-read"},
		{"read -r reply < x", driverNoEnv, "shell:builtin-read"},
		{"read -r line < x", map[string]bool{"line": true}, "shell:builtin-read"},
		{"read -r line < x", nil, "shell:builtin-read"},
		{"IFS=, read -r a b < x", driverNoEnv, "shell:assignment"},
		{"IFS= read -r -a a < x", driverNoEnv, "shell:assignment"},
		{"read -r line < x\n$line/run", driverNoEnv, "shell:local-command-word"},
	})
}

// AFP-V0-9625 to AFP-V0-9627: assigning expansions, the job's environment
// files and builtins that set variables or options stay computed.
func TestDriverComputedNeighbours_AFP_V0_9625(t *testing.T) {
	runDriverNarrowCases(t, []driverNarrowCase{
		{": \"${out:=dist}\"", driverNoEnv, "shell:assigning-expansion"},
		{"echo \"${GOFLAGS=-x}\"", driverNoEnv, "shell:assigning-expansion"},
		{"echo \"${out:-dist}\"", driverNoEnv, ""},
		{"echo $(( PATH = 1 ))", driverNoEnv, "shell:arithmetic-assignment"},
		{"echo $(( n++ ))", driverNoEnv, "shell:arithmetic-assignment"},
		{"echo $(( n == 1 ))", driverNoEnv, ""},
		{"((PATH=1))", driverNoEnv, "shell:assignment"},
		{"echo x=1 >> \"$GITHUB_ENV\"", driverNoEnv, "shell:github-env-file"},
		{"echo /x >> \"$GITHUB_PATH\"", driverNoEnv, "shell:github-env-file"},
		{"echo x=1 >> \"$GITHUB_OUTPUT\"", driverNoEnv, ""},
		{"echo ok >> \"$GITHUB_STEP_SUMMARY\"", driverNoEnv, ""},
		{"shopt -s nullglob", driverNoEnv, ""},
		{"shopt -s expand_aliases", driverNoEnv, "shell:builtin-shopt"},
		{"shopt nullglob", driverNoEnv, "shell:builtin-shopt"},
		{"printf -v out %s 1", driverNoEnv, "shell:builtin-printf-v"},
		{"mapfile -t lines < x", driverNoEnv, "shell:builtin-mapfile"},
		{"let x=1", driverNoEnv, "shell:builtin-let"},
		{"hash -p /x go", driverNoEnv, "shell:builtin-hash"},
	})
}

// AFP-V0-9623 and AFP-V0-9626: the workflow's env blocks and environment
// files reach the step scan.
func TestDriverWorkflowEnv_AFP_V0_9623(t *testing.T) {
	job := func(name, env, stepEnv, script string) string {
		return "  " + name + ":\n    runs-on: x\n" + env + "    steps:\n      - run: |\n          " +
			strings.ReplaceAll(script, "\n", "\n          ") + "\n" + stepEnv
	}
	workflow := "env:\n  TOP: 1\njobs:\n" +
		job("plain", "", "", "out=dist\ngo test ./...") +
		job("top", "", "", "top=1") +
		job("jobenv", "    env:\n      OUT: x\n", "", "out=1") +
		job("container", "    container:\n      image: x\n      env:\n        out: x\n", "", "out=1") +
		job("stepenv", "", "        env:\n          out: x\n", "out=1") +
		job("computedenv", "", "        env: ${{ fromJSON(inputs.env) }}\n", "out=1") +
		job("expression", "", "", "echo x >> ${{ github.env }}") +
		job("output", "", "", "echo x=1 >> \"$GITHUB_OUTPUT\"")
	reasons := driverComputedReasons(t, memorySource(t, map[string]string{
		"go.mod":                   "module example.test/m\n",
		".github/workflows/ci.yml": workflow,
	}))
	id := func(job string) string { return "driver:ci:.github/workflows/ci.yml#" + job }
	want := map[string][]string{
		id("top"):         {"shell:assignment"},
		id("jobenv"):      {"shell:assignment"},
		id("container"):   {"shell:assignment"},
		id("stepenv"):     {"shell:assignment"},
		id("computedenv"): {"shell:assignment"},
		id("expression"):  {"workflow:github-env-file"},
	}
	if fmt.Sprint(reasons) != fmt.Sprint(want) {
		t.Fatalf("reasons = %v\nwant      %v", reasons, want)
	}
	// Composite actions do not see the caller's step env (AFP-V0-9623).
	reasons = driverComputedReasons(t, memorySource(t, map[string]string{
		"go.mod":                       "module example.test/m\n",
		".github/actions/a/action.yml": "runs:\n  using: composite\n  steps:\n    - run: out=1\n      shell: bash\n",
	}))
	if len(reasons) != 1 {
		t.Fatalf("action reasons = %v, want the assignment computed", reasons)
	}
}
