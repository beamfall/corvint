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
		// Fault injection: an unknown environment, or one naming a file or
		// option a shell reads at startup, may shadow the builtin.
		{"set -e", nil, "shell:builtin-set"},
		{"set -e", map[string]bool{"bash_env": true}, "shell:builtin-set"},
		{"set -e", map[string]bool{"env": true}, "shell:builtin-set"},
		{"set -e", map[string]bool{"shellopts": true}, "shell:builtin-set"},
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
		{"x=1 y=2", driverNoEnv, ""},
		{"base=$(git rev-parse HEAD)\ngo test ./...", driverNoEnv, ""},
		{"count+=1", driverNoEnv, ""},
		// The redesign admits only fully understood simple-command scripts;
		// conditional assignments and every loop deliberately stay computed.
		{"if ok=1; then :; fi", driverNoEnv, "shell:assignment"},
		{"for f in a b; do :; done", driverNoEnv, "shell:loop-variable"},
		{"dir=.", driverNoEnv, ""},
		{"dir=../x", driverNoEnv, ""},
		// Fault injection: names a shell, zsh or a tool gives a meaning.
		{"PATH=/x", driverNoEnv, "shell:assignment"},
		{"path=/x", driverNoEnv, "shell:assignment"},
		{"Path=x", driverNoEnv, "shell:assignment"},
		{"ifs=x", driverNoEnv, "shell:assignment"},
		{"http_proxy=x", driverNoEnv, "shell:assignment"},
		{"no_proxy=x", driverNoEnv, "shell:assignment"},
		{"npm_config_userconfig=x", driverNoEnv, "shell:assignment"},
		{"yarn_cache_folder=x", driverNoEnv, "shell:assignment"},
		{"pnpm_home=x", driverNoEnv, "shell:assignment"},
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
		{"bash -c 'out=1'", driverNoEnv, ""},
		{"bash -e -c 'out=1'", driverNoEnv, "shell:interpreter-inline"},
		{"sh -xc 'out=1'", driverNoEnv, "shell:assignment"},
		// Named anywhere else in the script: its value is unbounded.
		{"out=dist\necho \"$out\"", driverNoEnv, "shell:local-referenced"},
		{"for f in a b; do echo \"$f\"; done", driverNoEnv, "shell:local-referenced"},
		{"d=tools\n$d/run.sh", driverNoEnv, "shell:local-referenced"},
		{"d=tools\n\"${d}/run.sh\"", driverNoEnv, "shell:local-referenced"},
		{"d=tools\nbash \"$d/run.sh\"", driverNoEnv, "shell:local-referenced"},
		{"d=tools\nsudo $d/run.sh", driverNoEnv, "shell:local-referenced"},
		{"d=tools\n. \"$d/env.sh\"", driverNoEnv, "shell:sourced"},
		{"t=x\ntrap \"$t/cleanup\" EXIT", driverNoEnv, "shell:local-referenced"},
		{"t=x\nfind . -exec $t/run {} \\;", driverNoEnv, "shell:local-referenced"},
		{"t=x\necho a | xargs $t/run", driverNoEnv, "shell:local-referenced"},
		{"t=x\ncat <<EOF\n$($t/run)\nEOF", driverNoEnv, "shell:local-referenced"},
		{"$d/run.sh\nd=tools", driverNoEnv, "shell:local-referenced"},
		{"for f in a; do $f/run.sh; done", driverNoEnv, "shell:local-referenced"},
		{"for PATH in /x; do :; done", driverNoEnv, "shell:loop-variable"},
		{"for out in a; do :; done", map[string]bool{"out": true}, "shell:loop-variable"},
		// Fault injection: an unmodelled $X/ command is still root-like (AZU parity).
		{"$X/run.sh", driverNoEnv, ""},
	})
}

// AFP-V0-9628: unsupported syntax anywhere revokes provisional narrowing,
// including before a candidate. Parsed references survive escaping and scope.
func TestDriverNarrowingRequiresCompleteParse_AFP_V0_9628(t *testing.T) {
	for _, script := range []string{
		"read -r 'dir' < choice; set -e",
		`read -r d\ir < choice; "$dir/run"`,
		"rea\\\nd -r dir < choice; \"$dir/run\"",
		"funct\\\nion read { \"$2/run\"; }; read -r tools",
		"read\\\n() { \"$2/run\"; }; read -r tools",
		`function 'read' { "$2/run"; }; read -r tools`,
		`out=1; cat <<EOF` + "\n${other:=value}\nEOF",
		`out=1; cat <<EOF` + "\n`source ci/setup.sh`\nEOF",
		"out=1; cat <<EOF\n$($tool/run)\nEOF",
		"out=1; cat <<EOF\n$out\nEOF",
		// Ordinary << does not allow tab-indented terminators; a # in the
		// body cannot hide executable substitutions as a shell comment.
		"unused=1\ncat <<EOF\n\tEOF\n# $(. ./ci/setup.sh; set -x; :)\nEOF",
		`sourc{e..e} ./ci/setup.sh; set -x; :`,
		`sourc[e] ./ci/setup.sh; set -x; :`,
		`*; set -e`,
		`~; set -e`,
		`unused=1; bash -ca '. ./ci/setup.sh; set -x; :'`,
		`unused=1; bash --norc -ca '. ./ci/setup.sh; set -x; :'`,
		`unused=1; bash -o allexport script.sh`,
		`out=1; echo "${name[out]}"`,
		`out=1; echo $[out]`,
		"read -r n < choice; cat <<EOF\n$[n]\nEOF",
		`out=1; echo $'\x24out'`,
		`out=1; echo "unterminated`,
		"set -e; cat <<EOF\nno terminator\nout=1",
		"out=1; cat <<EOF",
		`out=1; (echo ok)`,
		`out=1; cat <(echo ok)`,
		`out=1; env -S 'source ci/setup.sh'`,
		`out=1; bash -ac ':'`,
		`out=1; source ci/setup.sh`,
		`out=1; eval ':'`,
		`set -a; out=1`,
		`out=1; set -o allexport`,
	} {
		t.Run(script, func(t *testing.T) {
			_, scan := driverStepScan(t, script, driverNoEnv)
			if !scan.computed {
				t.Fatal("incomplete or unsupported script was narrowed")
			}
		})
	}
	// <<- strips tabs and the body parses, but AFP-V0-9400 still keeps any
	// here-doc computed: the scanner skips bodies a wrapper may execute.
	if reasons, _ := driverStepScan(t, "unused=1\ncat <<-EOF\n\tliteral\n\tEOF", driverNoEnv); !slices.Contains(reasons, "shell:heredoc-body") {
		t.Errorf("tab-stripped here-doc lost its AFP-V0-9400 reason: %v", reasons)
	}
	for _, script := range []string{`out=1; echo "$out"`, `read -r out; "$out/run"`, `for out in tools; do "$out/run"; done`} {
		t.Run("opaque/"+script, func(t *testing.T) {
			_, scan := driverStepScan(t, "bash -ac '"+script+"'", driverNoEnv)
			if !scan.locals["out"] || !scan.references["out"] {
				t.Fatal("opaque scope lost parsed declarations or references")
			}
			if strings.Contains(script, "/run") && !scan.commandRefs["out"] {
				t.Fatal("opaque scope lost parsed command references")
			}
		})
	}
}

// AFP-V0-9622: an assigned value is never recorded as a path; the step
// stays computed wherever the name is used again.
func TestDriverLocalAssignmentNoPaths_AFP_V0_9622(t *testing.T) {
	reasons, scan := driverStepScan(t, "d=tools/gen\ncat \"$d/x/input.json\"", driverNoEnv)
	if !slices.Contains(reasons, "shell:local-referenced") || scan.paths["tools/gen"] {
		t.Fatalf("reasons=%v paths=%v", reasons, scan.paths)
	}
}

// AFP-V0-9624: `read -r` into safe names is narrowed; other reads are not.
func TestDriverReadNarrowing_AFP_V0_9624(t *testing.T) {
	runDriverNarrowCases(t, []driverNarrowCase{
		{"while IFS= read -r line; do echo \"$line\"; done < go.mod", driverNoEnv, "shell:local-referenced"},
		{"read -r flag pattern pkg < x", driverNoEnv, ""},
		{"read -r -a argv < x", driverNoEnv, "shell:builtin-read"},
		{"read line < x", driverNoEnv, "shell:builtin-read"},
		{"read -r PATH < x", driverNoEnv, "shell:builtin-read"},
		{"read -r reply < x", driverNoEnv, "shell:builtin-read"},
		{"read -r http_proxy < x", driverNoEnv, "shell:builtin-read"},
		{"read -r line < x", map[string]bool{"line": true}, "shell:builtin-read"},
		{"read -r line < x", nil, "shell:builtin-read"},
		{"IFS=, read -r a b < x", driverNoEnv, "shell:assignment"},
		{"IFS= read -r -a a < x", driverNoEnv, "shell:assignment"},
		{"read -r line < x\n$line/run", driverNoEnv, "shell:local-referenced"},
	})
}

// AFP-V0-9625 to AFP-V0-9627: assigning expansions, the job's environment
// files and builtins that set variables or options stay computed.
func TestDriverComputedNeighbours_AFP_V0_9625(t *testing.T) {
	runDriverNarrowCases(t, []driverNarrowCase{
		{": \"${out:=dist}\"", driverNoEnv, "shell:assigning-expansion"},
		{"echo \"${GOFLAGS=-x}\"", driverNoEnv, "shell:assigning-expansion"},
		{"echo \"${out:-dist}\"", driverNoEnv, ""},
		{"echo \"${out:-$(date)}\"", driverNoEnv, "shell:brace-substitution"},
		{"echo \"${out:-`date`}\"", driverNoEnv, "shell:brace-substitution"},
		{"echo $(( $(cat n) + 1 ))", driverNoEnv, "shell:arithmetic-substitution"},
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

// AFP-V0-9628: each counterexample of the V1-5000 review, verbatim, keeps
// the step computed.
func TestDriverReviewCounterexamples_AFP_V0_9628(t *testing.T) {
	runDriverNarrowCases(t, []driverNarrowCase{
		// 1: a read value reaching a file operand.
		{"read -r dir < chosen-dir; cat \"$dir/payload\"", driverNoEnv, "shell:local-referenced"},
		// 2: an appended fragment is not a path.
		{"d=foo; d+=bar; cat \"$d/payload\"", driverNoEnv, "shell:local-referenced"},
		// 3: a command substitution inside ${...}.
		{"read -r d < chosen-dir; : \"${unset_name:-$(\"$d/run\")}\"", driverNoEnv, "shell:brace-substitution"},
		// 4: a function shadowing a narrowed builtin.
		{"function read { \"$2/run\"; }; read -r tools", driverNoEnv, "shell:function-definition"},
		{"read() { \"$2/run\"; }; read -r tools", driverNoEnv, "shell:function-definition"},
		{"set() { :; }\nset -e", driverNoEnv, "shell:function-definition"},
		{"alias read=x\nread -r tools", driverNoEnv, "shell:builtin-alias"},
		// 5: a sourced file may set PS4 or define functions.
		{". ./ci/setup.sh; set -x; :", driverNoEnv, "shell:sourced"},
		{"source ./ci/setup.sh\nshopt -s nullglob", driverNoEnv, "shell:builtin-shopt"},
		{"eval \"$x\"\nset -e", driverNoEnv, "shell:eval"},
		// Bash keeps the backslash of a double-quoted \t: the here-doc ends at
		// \true, so the source command runs (V1-5000 redesign review).
		{"cat <<\"\\true\"\n\\true\n. ./ci/setup.sh\ntrue\nread -r tools", driverNoEnv, "shell:builtin-read"},
		// Either decoding of an escaped delimiter revokes narrowing.
		{"cat <<\"\\true\"\ntrue\n. ./ci/setup.sh\n\\true\nread -r tools", driverNoEnv, "shell:builtin-read"},
		// 6: an inline shell with options, or set -a anywhere.
		{"bash -ac 'read -r npm_config_userconfig < chosen-config; npm ci'", driverNoEnv, "shell:builtin-read"},
		{"bash -o allexport -c 'out=1'", driverNoEnv, "shell:assignment"},
		{"set -a\nread -r tools < x", driverNoEnv, "shell:builtin-set"},
		// 7: a value written to GITHUB_OUTPUT reaches later steps.
		{"read -r dir < chosen-dir; echo \"dir=$dir\" >> \"$GITHUB_OUTPUT\"", driverNoEnv, "shell:github-output"},
		{"read -r dir < chosen-dir; cat x >> \"$GITHUB_OUTPUT\"", driverNoEnv, "shell:github-output"},
		// Indirect and arithmetic expansion name a local without its name.
		{"out=1; name=x; echo \"${!name}\"", driverNoEnv, "shell:indirect-expansion"},
		{"read -r n < x; [ \"$1\" -eq 1 ]", driverNoEnv, "shell:arithmetic-context"},
		// 8: a tool reads its configuration from these names.
		{"read -r npm_config_userconfig < chosen-config", driverNoEnv, "shell:builtin-read"},
	})
}

// AFP-V0-9628: a read value written to GITHUB_OUTPUT and run by a later
// step, and a step shell exporting every assignment, keep the job computed.
func TestDriverWorkflowCounterexamples_AFP_V0_9628(t *testing.T) {
	workflow := "jobs:\n" +
		"  choose:\n    runs-on: x\n    steps:\n" +
		"      - id: choose\n        run: |\n          read -r dir < chosen-dir\n          echo \"dir=$dir\" >> \"$GITHUB_OUTPUT\"\n" +
		"      - run: ${{ steps.choose.outputs.dir }}/run\n" +
		"  allexport:\n    runs-on: x\n    steps:\n" +
		"      - shell: bash -a {0}\n        run: read -r tools < x\n" +
		"  template:\n    runs-on: x\n    steps:\n" +
		"      - shell: bash --noprofile --norc -eo pipefail {0}\n        run: read -r tools < x\n"
	reasons := driverComputedReasons(t, memorySource(t, map[string]string{
		"go.mod":                   "module example.test/m\n",
		".github/workflows/ci.yml": workflow,
	}))
	id := func(job string) string { return "driver:ci:.github/workflows/ci.yml#" + job }
	if !slices.Contains(reasons[id("choose")], "shell:github-output") {
		t.Errorf("choose reasons %v, want shell:github-output", reasons[id("choose")])
	}
	if !slices.Contains(reasons[id("allexport")], "shell:builtin-read") {
		t.Errorf("allexport reasons %v, want shell:builtin-read", reasons[id("allexport")])
	}
	if got := reasons[id("template")]; len(got) != 0 {
		t.Errorf("template reasons %v, want narrowed", got)
	}
}
