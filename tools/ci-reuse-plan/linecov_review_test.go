// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/linecov"
)

// GLS-V0-046: execute the actual rerun function, including package-level
// failures without Test, while rejecting setup/build and unrelated failures.
func TestLinecovPackageMissConfirmation(t *testing.T) {
	workflow := readWorkflow(t, "linecov-miss.yml")
	body := runScript(t, stepBlock(t, workflow, "Confirm each suspected miss"))
	start := strings.Index(body, "rerun() {")
	end := strings.Index(body[start:], "\njq -c ")
	if start < 0 || end < 0 {
		t.Fatal("rerun function missing")
	}
	fn := body[start : start+end]
	for _, tc := range []struct {
		name, test, phase, events string
		status                    int
		want                      bool
	}{
		{"package panic", "(outside every test)", "head1", `{"Action":"start","Package":"m/p"}
{"Action":"output","Package":"m/p","Output":"panic: initialization"}
{"Action":"fail","Package":"m/p"}`, 1, true},
		{"testmain exit", "(outside every test)", "head2", `{"Action":"start","Package":"m/p"}
{"Action":"fail","Package":"m/p"}`, 1, true},
		{"package pass", "(outside every test)", "base", `{"Action":"start","Package":"m/p"}
{"Action":"pass","Package":"m/p"}`, 0, true},
		{"build fail", "(outside every test)", "head1", `{"Action":"start","Package":"m/p"}
{"Action":"output","Package":"m/p","Output":"FAIL m/p [build failed]"}
{"Action":"fail","Package":"m/p"}`, 1, false},
		{"setup fail", "(outside every test)", "head1", `{"Action":"output","Package":"m/p","Output":"FAIL m/p [setup failed]"}
{"Action":"fail","Package":"m/p"}`, 1, false},
		{"other test", "TestTarget", "head1", `{"Action":"fail","Package":"m/p","Test":"TestOther"}
{"Action":"fail","Package":"m/p"}`, 1, false},
		{"other package", "(outside every test)", "head1", `{"Action":"start","Package":"m/other"}
{"Action":"fail","Package":"m/other"}`, 1, false},
		{"malformed", "(outside every test)", "head1", "invalid json", 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "events"), []byte(tc.events), 0600); err != nil {
				t.Fatal(err)
			}
			script := "set -eu\nd=" + shellQuoteLinecov(dir) + "\nmodule=m\nRUNNER_TEMP=" + shellQuoteLinecov(dir) + "\ndeadline=50m0s\ngo() { cat \"$d/events\"; return " + string(rune('0'+tc.status)) + "; }\n" + fn + "\nrerun \"$d\" p " + shellQuoteLinecov(tc.test) + " " + tc.phase
			cmd := exec.Command("bash", "-c", script)
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.want {
				t.Fatalf("accepted=%v want %v: %v %s", err == nil, tc.want, err, out)
			}
		})
	}
}
func shellQuoteLinecov(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }

// GLS-V0-044: only shards that actually applied narrowing depend on the
// completed artifact; the pre-test proof cannot substitute for it.
func TestLinecovCompletedEvidenceRequired(t *testing.T) {
	ci := readWorkflow(t, "ci.yml")
	upload := stepBlock(t, ci, "Retain the line-level selection (GLS-V0-044)")
	for _, line := range []string{
		"        id: linecov-evidence", "        if: always() && steps.linecov.outcome != 'skipped'",
		"        continue-on-error: true", "          name: linecov-narrow-${{ strategy.job-index }}",
		"          path: ${{ runner.temp }}/linecov-narrow/", "          if-no-files-found: error",
	} {
		wantLine(t, "completed evidence", upload, line)
	}
	gate := stepBlock(t, ci, "Require completed evidence for a narrowed shard")
	if strings.Contains(gate, "continue-on-error:") || strings.Index(ci, gate) < strings.Index(ci, upload) {
		t.Fatal("evidence gate must fail the shard after the completed upload")
	}
	m := regexp.MustCompile(`(?m)^        if: (.*)$`).FindStringSubmatch(gate)
	if len(m) != 2 {
		t.Fatal("missing evidence gate condition")
	}
	for _, mode := range []string{"NARROWED", "SHADOW", "FALLBACK", ""} {
		for _, outcome := range []string{"success", "failure", "cancelled", "skipped", ""} {
			ctx := map[string]string{"always()": "true", "steps.tests.outputs.linecov": mode, "steps.linecov-evidence.outcome": outcome}
			if got, want := evalActionsIf(t, m[1], ctx), mode == "NARROWED" && outcome != "success"; got != want {
				t.Errorf("mode=%q upload=%q: gate=%v, want %v", mode, outcome, got, want)
			}
		}
	}
	dir := t.TempDir()
	summary := filepath.Join(dir, "summary")
	code, out := runBash(t, dir, runScript(t, gate), "GITHUB_STEP_SUMMARY="+summary)
	if code == 0 || !strings.Contains(out, "::error::") || !strings.Contains(readText(t, summary), "not retained") {
		t.Fatalf("lost evidence did not fail visibly: exit=%d %s", code, out)
	}
}

// GLS-V0-040/044: execute the workflow's receipt admission and rewrite
// blocks. A SHADOW receipt must produce a comparison, but never applied
// evidence or changed execution lists, even with narrowing switched on.
func TestLinecovShadowWorkflow(t *testing.T) {
	ci := readWorkflow(t, "ci.yml")
	selection := runScript(t, stepBlock(t, ci, "Line-level selection (GLS-V0-040"))
	start := strings.Index(selection, "state=$(jq -r '.state'")
	if start < 0 {
		t.Fatal("selection state admission missing")
	}
	admit := selection[start:]
	tests := runScript(t, stepBlock(t, ci, "Go tests (full until matching shadow qualification)"))
	start, end := strings.Index(tests, `line_narrowed=""`), strings.Index(tests, `[ "$placement" = BALANCED ]`)
	if start < 0 || end <= start {
		t.Fatal("line selection rewrite missing")
	}
	rewrite := tests[start:end]
	for _, tc := range []struct {
		name, state, mode, plan, wantMode string
		applied                           bool
	}{
		{"qualified narrowing", "NARROWED", "NARROWED", "NARROWED", "NARROWED", true},
		{"unqualified with switch on", "SHADOW", "NARROWED", "SHADOW", "SHADOW", false},
		{"unqualified in shadow", "SHADOW", "SHADOW", "SHADOW", "SHADOW", false},
		{"qualified with switch off", "NARROWED", "SHADOW", "NARROWED", "SHADOW", false},
		{"shadow plan cannot authorize narrowing", "NARROWED", "NARROWED", "SHADOW", "NARROWED", false},
		{"fallback", "FALLBACK", "NARROWED", "FALLBACK", "FALLBACK", false},
		{"unknown state", "UNKNOWN", "NARROWED", "UNKNOWN", "FALLBACK", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			ln := filepath.Join(dir, "linecov-narrow")
			if err := os.MkdirAll(ln, 0700); err != nil {
				t.Fatal(err)
			}
			writeJSON(t, filepath.Join(ln, "selection.json"), map[string]string{"state": tc.state})
			output := filepath.Join(dir, "output")
			env := []string{"RUNNER_TEMP=" + dir, "GITHUB_OUTPUT=" + output, "SHARD=0", "PLAN_STATE=" + tc.plan}
			code, out := runBash(t, dir, "d="+shellQuoteLinecov(ln)+"\nmode="+tc.mode+"\n"+`stop() { echo mode=FALLBACK >> "$GITHUB_OUTPUT"; exit 0; }
`+admit, env...)
			if code != 0 || readText(t, output) != "mode="+tc.wantMode+"\n" {
				t.Fatalf("receipt admission: exit=%d %s", code, out)
			}
			// The real CLI's SHADOW plan is pinned in qualified_workflow_test.go.
			// This fixture also injects a SHADOW plan into NARROWED mode.
			writeExecutable(t, filepath.Join(dir, "linecov"), `set -eu
[ "$1" = ci ] && [ "$2" = -selection ] && [ "$4" = -packages ] && [ "$6" = -slices ] && [ "$8" = -out ]
test -s "$3" && test -s "$5" && test -s "$7"
mkdir -p "$9"
printf 'm/keep\n' > "$9/packages"
printf '%s\n' '-run ^(TestA)$ m/tests' '-run ^TestSplit$ m/split' > "$9/slices"
jq -n --arg state "$PLAN_STATE" '{state:$state, excluded:["m/drop"], sliced:["m/tests"]}' > "$9/plan.json"
`)
			script := `packages=$(printf 'm/drop\nm/keep\nm/tests')
slices='-run ^TestSplit$ m/split'
` + rewrite + `
printf '%s\n' "$packages" > "$RUNNER_TEMP/executed-packages"
printf '%s\n' "$slices" > "$RUNNER_TEMP/executed-slices"
printf '%s' "$line_narrowed" > "$RUNNER_TEMP/narrowed"
`
			code, out = runBash(t, dir, script, append(env, "LINECOV_MODE="+tc.wantMode)...)
			if code != 0 {
				t.Fatalf("rewrite: exit=%d %s", code, out)
			}
			wantPackages, wantSlices := "m/drop\nm/keep\nm/tests\n", "-run ^TestSplit$ m/split\n"
			if tc.applied {
				wantPackages, wantSlices = "m/keep\n", "-run ^(TestA)$ m/tests\n-run ^TestSplit$ m/split\n"
			}
			if readText(t, filepath.Join(dir, "executed-packages")) != wantPackages || readText(t, filepath.Join(dir, "executed-slices")) != wantSlices {
				t.Fatalf("execution lists disagree with narrowing authority: %s", out)
			}
			_, err := os.Stat(filepath.Join(ln, "applied"))
			if (err == nil) != tc.applied || strings.Contains(readText(t, output), "linecov=NARROWED") != tc.applied || (readText(t, filepath.Join(dir, "narrowed")) == "1") != tc.applied {
				t.Fatal("applied marker, step output or coverage marker disagrees with actual narrowing")
			}
			if tc.wantMode == "SHADOW" {
				if readText(t, filepath.Join(ln, "before-packages")) != "m/drop\nm/keep\nm/tests\n" || readText(t, filepath.Join(ln, "after", "packages")) != "m/keep\n" || !strings.Contains(out, "(SHADOW)") {
					t.Fatalf("shadow comparison was not retained and reported: %s", out)
				}
			}
		})
	}
}

// GLS-V0-046: a direct PR's tested synthetic commit need not be reachable
// from main. Exercise the confirmation loop against a local Git remote,
// proving that it fetches the exact recorded SHA before comparing trees.
func TestLinecovMissFetchesSyntheticTestedCommit(t *testing.T) {
	body := runScript(t, stepBlock(t, readWorkflow(t, "linecov-miss.yml"), "Confirm each suspected miss"))
	start := strings.Index(body, `jq -c ".[:$MAX_SUSPECTS][]"`)
	if start < 0 {
		t.Fatal("confirmation loop missing")
	}
	loop := body[start:]
	writer := stepBlock(t, readWorkflow(t, "linecov-miss.yml"), "Persist coverage exclusion kill switch")
	wantLine(t, "kill persistence after a partial confirmation failure", writer, "        if: always() && steps.confirm.outputs.confirmed == '1'")
	remote := t.TempDir()
	git := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git(remote, "init", "--initial-branch=main")
	git(remote, "config", "user.name", "Fixture")
	git(remote, "config", "user.email", "fixture@example.invalid")
	git(remote, "config", "commit.gpgsign", "false")
	writeFile := func(name, data string) {
		t.Helper()
		if err := os.WriteFile(name, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(filepath.Join(remote, "p"), "base\n")
	git(remote, "add", "p")
	git(remote, "commit", "-m", "base")
	base := git(remote, "rev-parse", "HEAD")
	writeFile(filepath.Join(remote, "p"), "changed\n")
	git(remote, "commit", "-am", "synthetic tested commit")
	head := git(remote, "rev-parse", "HEAD")
	git(remote, "update-ref", "refs/pull/1/merge", head)
	git(remote, "reset", "--hard", base)
	writeFile(filepath.Join(remote, "p"), "changed\n")
	git(remote, "commit", "-am", "landed commit")
	merged := git(remote, "rev-parse", "HEAD")
	if head == merged || git(remote, "rev-parse", head+"^{tree}") != git(remote, "rev-parse", merged+"^{tree}") {
		t.Fatal("fixture needs distinct commits with identical tested and landed trees")
	}
	for _, tc := range []struct {
		name, fetch string
		confirmed   bool
		failed      bool
	}{
		{"fetch succeeds", `command git "$@"`, true, false},
		{"fetch fails", "return 23", false, true},
		{"failure after confirmation", `[ "$(wc -l < "$d/fetches")" -eq 1 ] || return 23; command git "$@"`, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "source")
			git(dir, "clone", "--single-branch", "--branch", "main", "file://"+remote, source)
			cmd := exec.Command("git", "cat-file", "-e", head+"^{commit}")
			cmd.Dir = source
			if err := cmd.Run(); err == nil {
				t.Fatal("fixture already contains the synthetic tested commit")
			}
			suspects := []map[string]string{{"package": "m/p", "test": "TestMiss", "base": base, "head": head, "merged": merged, "runner": "fixture"}}
			if tc.confirmed && tc.failed {
				suspects = append(suspects, map[string]string{"package": "m/p", "test": "TestOther", "base": base, "head": head, "merged": merged, "runner": "fixture"})
			}
			writeJSON(t, filepath.Join(dir, "suspects.json"), suspects)
			writeFile(filepath.Join(dir, "confirmed.jsonl"), "")
			writeFile(filepath.Join(dir, "fetch-errors"), "")
			script := `set -euo pipefail
d=$RUNNER_TEMP
module=m
MAX_SUSPECTS=5
BUDGET_SECONDS=17400
reproducible() { deadline=1s; seconds=1; }
rerun() { echo "$4" >> "$d/reruns"; }
git() {
 if [ "$1" = fetch ]; then
  printf '%s\n' "$*" >> "$d/fetches"
  ` + tc.fetch + `
 else
  command git "$@"
 fi
}
` + loop
			code, out := runBash(t, source, script, "RUNNER_TEMP="+dir, "GITHUB_STEP_SUMMARY="+filepath.Join(dir, "summary"), "GITHUB_OUTPUT="+filepath.Join(dir, "output"))
			if got := readText(t, filepath.Join(dir, "fetches")); got != strings.Repeat("fetch --no-tags origin "+head+"\n", len(suspects)) {
				t.Fatalf("did not fetch the recorded commit: %q", got)
			}
			summary := readText(t, filepath.Join(dir, "summary"))
			if (code != 0) != tc.failed {
				t.Fatalf("exit=%d, want failure=%v: %s", code, tc.failed, out)
			}
			if tc.failed && (!strings.Contains(out, "::error::") || !strings.Contains(summary, "failed to fetch recorded tested commit "+head)) {
				t.Fatalf("fetch failure was not reported: %s %s", out, summary)
			}
			if tc.confirmed {
				if readText(t, filepath.Join(dir, "reruns")) != "base\nhead1\nhead2\n" || !strings.Contains(summary, ": confirmed") || readText(t, filepath.Join(dir, "output")) != "confirmed=1\n" || !strings.Contains(readText(t, filepath.Join(dir, "kill", "kill.json")), "TestMiss") {
					t.Fatalf("fetched tested tree did not reach confirmation: exit=%d %s %s", code, out, summary)
				}
			} else {
				if readText(t, filepath.Join(dir, "confirmed.jsonl")) != "" {
					t.Fatal("fetch failure was confirmed")
				}
				if _, err := os.Stat(filepath.Join(dir, "reruns")); !os.IsNotExist(err) {
					t.Fatal("fetch failure reached reruns")
				}
			}
		})
	}
}

// GLS-V0-046, GLS-V0-009: a confirmation rerun reproduces the runner the
// narrowed selection's index names, which the shard ran under: the -race
// build, the test-confine wrapper (built as ci.yml builds it) confined to the
// rerun's worktree, and the recorded test deadline. Execute the workflow's own
// reproducible and rerun functions on identities linecov computes, with a go
// stub whose test needs 30 minutes, so it times out under a shorter deadline,
// and that refuses an unwrapped or unconfined run. A runner that differs in
// anything but the deadline, or names none, is not reproduced at all.
func TestLinecovMissConfirmationReproducesRecordedRunner(t *testing.T) {
	for _, tool := range []string{"bash", "jq", "sha256sum", "go"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is required: %v", tool, err)
		}
	}
	workflow, ci := readWorkflow(t, "linecov-miss.yml"), readWorkflow(t, "ci.yml")
	commands := func(script string) string {
		var out []string
		for _, l := range strings.Split(script, "\n") {
			if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
				out = append(out, l)
			}
		}
		return strings.Join(out, "\n")
	}
	build := commands(runScript(t, stepBlock(t, workflow, "Build the protected test read-scope wrapper as the CI shards do")))
	if want := commands(runScript(t, stepBlock(t, ci, "Build protected test read-scope wrapper independently of PR module directives"))); build != want {
		t.Fatalf("the miss workflow builds test-confine unlike the shards:\n%s\nwant\n%s", build, want)
	}
	const invocation = `CORVINT_CONFINE_ROOT="$PWD" GOFLAGS=-race go test -exec "$RUNNER_TEMP/test-confine" `
	shards := 0
	for _, l := range strings.Split(ci, "\n") {
		if strings.Contains(l, `CORVINT_CONFINE_ROOT="$PWD" go test -exec "$RUNNER_TEMP/test-confine" `) && strings.Contains(l, " -race -timeout 50m") {
			shards++
		}
	}
	body := runScript(t, stepBlock(t, workflow, "Confirm each suspected miss by reruns (GLS-V0-046)"))
	start := strings.Index(body, "wrapper_id=")
	end := strings.Index(body[max(start, 0):], "\njq -c ")
	if shards == 0 || start < 0 || end < 0 || !strings.Contains(body[start:start+end], invocation) {
		t.Fatalf("the rerun does not invoke the shard's runner (%d shard lines):\n%s", shards, body)
	}
	fn := body[start : start+end]
	if want := "arch_vars=(" + strings.Join(linecov.ArchFeatureVars, " ") + ")"; !strings.Contains(fn, want+" ") {
		t.Fatalf("the rerun reads other arch features than the runner identity, want %s", want)
	}

	dir := t.TempDir()
	wrapper := filepath.Join(dir, "test-confine")
	other := filepath.Join(t.TempDir(), "test-confine")
	for file, text := range map[string]string{wrapper: "#!/bin/sh\nexec \"$@\"\n", other: "#!/bin/sh\nexec env \"$@\"\n"} {
		if err := execfile.WriteFile(file, []byte(text), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module m\n\ngo 1.27\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A hosted shard's environment: no runtime or arch-feature settings.
	for _, name := range append(append([]string{"GOEXPERIMENT", "GOWORK"}, linecov.RuntimeEnvVars...), linecov.ArchFeatureVars...) {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	t.Setenv("GOENV", "off")
	t.Setenv("CGO_ENABLED", "1")
	id := func(goflags string, timeout time.Duration, wrapper string) string {
		t.Setenv("GOFLAGS", goflags)
		runner, _, err := linecov.RunnerID(context.Background(), root, nil, timeout, wrapper)
		if err != nil {
			t.Fatal(err)
		}
		return runner
	}
	cases := []struct {
		name, runner string
		want         int // 0 confirmed, 1 not confirmed, 3 not reproduced
	}{
		{"recorded 50m under the wrapper", id("-race", 50*time.Minute, wrapper), 0},
		{"recorded 20m deadline", id("-race", 20*time.Minute, wrapper), 1},
		{"no wrapper", id("-race", 50*time.Minute, ""), 3},
		{"another wrapper", id("-race", 50*time.Minute, other), 3},
		{"no deadline", id("-race", 0, wrapper), 3},
		{"no race build", id("", 50*time.Minute, wrapper), 3},
		{"unrecorded", "", 3},
	}
	version, err := exec.Command("go", "env", "GOVERSION").Output()
	if err != nil {
		t.Fatal(err)
	}
	stub := `go() {
 case "$1" in
 env) command go "$@"; return ;;
 test) shift ;;
 *) return 9 ;;
 esac
 local wrap="" limit="" re='^(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?$' secs
 while [ $# -gt 0 ]; do
  case "$1" in -exec) wrap=$2; shift ;; -timeout) limit=$2; shift ;; esac
  shift
 done
 if [ "$wrap" != "$RUNNER_TEMP/test-confine" ] || [ "${CORVINT_CONFINE_ROOT:-}" != "$PWD" ] || [ "$GOFLAGS" != -race ]; then
  echo "FAIL m/p [setup failed]: not the shard's runner"; return 2
 fi
 [[ "$limit" =~ $re ]] || return 2
 secs=$((10#${BASH_REMATCH[2]:-0} * 3600 + 10#${BASH_REMATCH[4]:-0} * 60 + 10#${BASH_REMATCH[6]:-0}))
 if [ "$secs" -lt 1800 ]; then
  printf '%s\n' '{"Action":"start","Package":"m/p"}' '{"Action":"output","Package":"m/p","Test":"TestSlow","Output":"panic: test timed out"}' '{"Action":"fail","Package":"m/p","Test":"TestSlow"}' '{"Action":"fail","Package":"m/p"}'
  return 1
 fi
 case "$PWD" in
 */at-base) printf '%s\n' '{"Action":"start","Package":"m/p"}' '{"Action":"pass","Package":"m/p","Test":"TestSlow"}' '{"Action":"pass","Package":"m/p"}'; return 0 ;;
 *) printf '%s\n' '{"Action":"start","Package":"m/p"}' '{"Action":"fail","Package":"m/p","Test":"TestSlow"}' '{"Action":"fail","Package":"m/p"}'; return 1 ;;
 esac
}
`
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			temp := t.TempDir()
			for _, p := range []string{"at-base/p", "at-head/p"} {
				if err := os.MkdirAll(filepath.Join(temp, p), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			data, err := os.ReadFile(wrapper)
			if err != nil {
				t.Fatal(err)
			}
			if err := execfile.WriteFile(filepath.Join(temp, "test-confine"), data, 0o755); err != nil {
				t.Fatal(err)
			}
			script := "set -eu\nd=" + shellQuoteLinecov(temp) + "\nmodule=m\nRUNNER_TEMP=" + shellQuoteLinecov(temp) + "\ngo_version=" + strings.TrimSpace(string(version)) + "\n" + stub + fn +
				"\nreproducible " + shellQuoteLinecov(tc.runner) + " || exit 3\n" +
				`rerun "$RUNNER_TEMP/at-base" p TestSlow base && rerun "$RUNNER_TEMP/at-head" p TestSlow head1 && rerun "$RUNNER_TEMP/at-head" p TestSlow head2`
			out, err := exec.Command("bash", "-c", script).CombinedOutput()
			got := 0
			if exit, ok := err.(*exec.ExitError); ok {
				got = exit.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}
			if got != tc.want && !(tc.want == 1 && got != 0 && got != 3) {
				t.Fatalf("exit %d, want %d (runner %q): %s", got, tc.want, tc.runner, out)
			}
		})
	}
}

// GLS-V0-047: retention expiry cannot re-enable a family. Execute CI's
// switch check with an expired artifact and an independent durable marker.
func TestLinecovKillSurvivesArtifactExpiry(t *testing.T) {
	body := runScript(t, stepBlock(t, readWorkflow(t, "ci.yml"), "Line-level selection (GLS-V0-040"))
	start := strings.Index(body, "# GLS-V0-047:")
	end := strings.Index(body[start:], "\ngit worktree add")
	if start < 0 || end < 0 {
		t.Fatal("switch check missing")
	}
	check := body[start : start+end]
	for _, tc := range []struct {
		name, marker string
		want         bool
	}{{"expired but disabled", "present", false}, {"operator deleted", "absent", true}, {"unreadable", "error", false}} {
		t.Run(tc.name, func(t *testing.T) {
			// Stub every gh call: artifact listing has no unexpired kills. A durable
			// ref is present, explicitly absent (404), or unreadable (503).
			script := `set -eu
REPO=m/r
stop() { exit 42; }
gh() {
 case "$*" in
 *git/ref/tags/linecov-kill-go*)
  case ` + tc.marker + ` in
   present) printf '{"ref":"refs/tags/linecov-kill-go"}\n' ;;
   absent) printf '{"message":"Not Found","status":"404"}\n'; return 1 ;;
   error) printf '{"message":"Unavailable","status":"503"}\n'; return 1 ;;
  esac ;;
 *) echo 0 ;;
 esac
}
` + check
			out, err := exec.Command("bash", "-c", script).CombinedOutput()
			if (err == nil) != tc.want {
				t.Fatalf("enabled=%v want %v: %v %s", err == nil, tc.want, err, out)
			}
		})
	}
	// The writer must persist the same marker independently of artifact upload.
	miss := readWorkflow(t, "linecov-miss.yml")
	if !strings.Contains(miss, "refs/tags/linecov-kill-go") || !strings.Contains(miss, "contents: write") {
		t.Error("no persistent switch writer")
	}
}

// GLS-V0-047: the writer creates durable state before the expiring detail
// artifact, preserves concurrent markers and surfaces failure to persist it.
func TestLinecovKillMarkerWriter(t *testing.T) {
	body := runScript(t, stepBlock(t, readWorkflow(t, "linecov-miss.yml"), "Persist coverage exclusion kill switch"))
	for _, tc := range []struct {
		name, existing string
		denied         bool
		want           bool
	}{
		{"new marker", "", false, true}, {"concurrent marker", "earlier", false, true}, {"write denied", "", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			marker := filepath.Join(dir, "marker")
			if tc.existing != "" {
				if err := os.WriteFile(marker, []byte(tc.existing), 0600); err != nil {
					t.Fatal(err)
				}
			}
			script := `set -euo pipefail
REPO=m/r
HEAD_SHA=confirmed
marker=` + shellQuoteLinecov(marker) + `
gh() {
 case "$*" in
 *"--method POST"*)
  [ ! -e "$marker" ] || return 1
  [ ` + shellQuoteLinecov(tc.name) + ` != 'write denied' ] || return 1
  printf '%s' "$HEAD_SHA" > "$marker" ;;
 *git/ref/tags/linecov-kill-go*)
  [ -s "$marker" ] || return 1
  echo refs/tags/linecov-kill-go ;;
 *) return 1 ;;
 esac
}
` + body
			out, err := exec.Command("bash", "-c", script).CombinedOutput()
			if (err == nil) != tc.want {
				t.Fatalf("persisted=%v want %v: %v %s", err == nil, tc.want, err, out)
			}
			if tc.want {
				data, err := os.ReadFile(marker)
				want := "confirmed"
				if tc.existing != "" {
					want = tc.existing
				}
				if err != nil || string(data) != want {
					t.Fatalf("marker=%q want %s: %v", data, want, err)
				}
			}
		})
	}
}
