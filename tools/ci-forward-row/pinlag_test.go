// SPDX-License-Identifier: Apache-2.0
package forwardrow

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testplatform"
)

// TestCINV0010PinLagRefusesNarrowing pins CIN-V0-010 (V1-3748): pin-lag.sh passes only while
// each pin is in main's history with main's selection paths, fails on any selection change after
// a pin or a pin outside main's history, ignores tests, fixtures, shard costs and other paths, and
// exits 2 on what it cannot read; the narrowing gate runs it from the compared main commit before
// narrow.sh and mq-narrow.sh, so a lagging pin leaves every shard FULL.
func TestCINV0010PinLagRefusesNarrowing(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "pin-lag.sh is a POSIX shell script")
	repo := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.org", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.org")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(path, body string) { writeFile(t, filepath.Join(repo, path), []byte(body)) }
	git("init", "-q", "-b", "main")
	for _, p := range []string{"internal/gateselect/select.go", "internal/gateselect/select_test.go", "internal/gateselect/testdata/plan.json",
		"internal/liveverify/affected/golang/graph.go", "internal/sessiontest/plan.go", "tools/corvint-pr-tests/main.go",
		"tools/gate-affected-select/main.go", ".github/cishards/order.go", ".github/cishards/package-costs.json",
		"cmd/corvint/affected.go", "cmd/corvint/test.go", "cmd/corvint/main.go", ".github/forward-rows/narrow.sh",
		".github/forward-rows/narrow.jq", ".github/forward-rows/mq-narrow.sh", "README.md"} {
		write(p, "v0\n")
	}
	git("add", "-A")
	git("commit", "-q", "-m", "pin")
	pin := git("rev-parse", "HEAD")
	// change commits one edit on top of the pin and returns it.
	change := func(path string) string {
		t.Helper()
		git("checkout", "-q", "--detach", pin)
		write(path, "v1\n")
		git("add", "-A")
		git("commit", "-q", "-m", "edit "+path)
		return git("rev-parse", "HEAD")
	}
	run := func(args ...string) (int, string) {
		t.Helper()
		cmd := exec.Command("sh", append([]string{script(t, "pin-lag.sh")}, args...)...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err == nil {
			return 0, string(out)
		}
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatal(err)
		}
		return ee.ExitCode(), string(out)
	}
	if code, out := run(pin, pin, pin); code != 0 || out != "" {
		t.Fatalf("a pin at main: exit %d\n%s", code, out)
	}
	for _, p := range []string{"internal/gateselect/select_test.go", "internal/gateselect/testdata/plan.json",
		".github/cishards/package-costs.json", "cmd/corvint/main.go", ".github/forward-rows/mq-narrow.sh", "README.md", "docs/new.md"} {
		if code, out := run(change(p), pin, pin); code != 0 || out != "" {
			t.Fatalf("a change to %s counted as lag: exit %d\n%s", p, code, out)
		}
	}
	for _, p := range []string{"internal/gateselect/select.go", "internal/liveverify/affected/golang/graph.go", "internal/sessiontest/plan.go",
		"tools/corvint-pr-tests/main.go", "tools/gate-affected-select/main.go", ".github/cishards/order.go", "cmd/corvint/affected.go",
		"cmd/corvint/test_slices.go", ".github/forward-rows/narrow.sh", ".github/forward-rows/narrow.jq"} {
		main := change(p)
		code, out := run(main, pin, pin)
		short := git("rev-parse", "--short", main)
		for _, name := range []string{"CORVINT_PR_TOOL_SOURCE", "CORVINT_TEST_SHADOW_SOURCE"} {
			want := "::error title=CI pin lags selection code (CIN-V0-010)::" + name + " " + pin + " builds older selection code than main " +
				main + " (1 commit(s): " + short + "); repin it to a reviewed main commit; this shard runs FULL\n"
			if code != 1 || !strings.Contains(out, want) {
				t.Fatalf("a change to %s after the pin: exit %d\n%s\nwant line:\n%s", p, code, out, want)
			}
		}
		// One lagging pin names only itself; the pin at main passes.
		if code, out := run(main, main, pin); code != 1 || strings.Contains(out, "CORVINT_PR_TOOL_SOURCE") || !strings.Contains(out, "CORVINT_TEST_SHADOW_SOURCE "+pin) {
			t.Fatalf("only the shadow pin lags %s: exit %d\n%s", p, code, out)
		}
	}
	// A pin outside main's history fails even with identical selection paths.
	side := change("README.md")
	if code, out := run(pin, side, pin); code != 1 || !strings.Contains(out, "CORVINT_PR_TOOL_SOURCE "+side+" is not in the history of main "+pin) {
		t.Fatalf("a pin outside main's history: exit %d\n%s", code, out)
	}
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"short pin", []string{pin, pin[:12], pin}, "CORVINT_PR_TOOL_SOURCE '" + pin[:12] + "' is not a full commit id"},
		{"upper-case pin", []string{pin, pin, strings.ToUpper(pin)}, "is not a full commit id"},
		{"empty pin", []string{pin, "", pin}, "CORVINT_PR_TOOL_SOURCE '' is not a full commit id"},
		{"unknown pin", []string{pin, pin, strings.Repeat("e", 40)}, "CORVINT_TEST_SHADOW_SOURCE " + strings.Repeat("e", 40) + " is not a commit of this checkout"},
		{"unknown main", []string{strings.Repeat("e", 40), pin, pin}, "is not a commit"},
		{"arguments", []string{pin, pin}, "usage: pin-lag.sh MAIN TOOL_PIN SHADOW_PIN"},
	} {
		if code, out := run(tc.args...); code != 2 || !strings.Contains(out, tc.want) {
			t.Fatalf("%s: exit %d, want 2 naming %q\n%s", tc.name, code, tc.want, out)
		}
	}

	// The committed pins are equal full ids, and the gate checks them against the compared main
	// commit, from that commit's own pin-lag.sh, before either narrowing script runs.
	b, err := os.ReadFile("../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	ci := string(b)
	pins := map[string]string{}
	for _, name := range []string{"CORVINT_PR_TOOL_SOURCE", "CORVINT_TEST_SHADOW_SOURCE"} {
		m := regexp.MustCompile(`(?m)^      `+name+`: "([0-9a-f]{40})"$`).FindAllStringSubmatch(ci, -1)
		if len(m) != 1 {
			t.Fatalf("%d pins of %s, want 1", len(m), name)
		}
		pins[name] = m[0][1]
	}
	if pins["CORVINT_PR_TOOL_SOURCE"] != pins["CORVINT_TEST_SHADOW_SOURCE"] {
		t.Fatalf("the pins differ: %v", pins)
	}
	m := regexp.MustCompile(`(?ms)^      - name: Narrowing gate .*?\n(.*?)\n      - name: Retain the narrowing record`).FindStringSubmatch(ci)
	if m == nil {
		t.Fatal("ci.yml lacks the narrowing gate")
	}
	gate := m[1]
	args := ` "$CORVINT_PR_TOOL_SOURCE" "$CORVINT_TEST_SHADOW_SOURCE"` + "\n"
	order := []string{
		`git archive "$scripts" .github/forward-rows | tar -x -C "$trusted"`,
		`sh "$trusted/.github/forward-rows/pin-lag.sh" "$scripts"` + args,
		`sh "$trusted/.github/forward-rows/mq-narrow.sh"`,
		`main=$(git rev-parse --verify "$TESTED_MERGE^1")`,
		`git archive "$main" .github/forward-rows/pin-lag.sh | tar -x -C "$RUNNER_TEMP/pin-lag"`,
		`sh "$RUNNER_TEMP/pin-lag/.github/forward-rows/pin-lag.sh" "$main"` + args,
		`sh "$GITHUB_WORKSPACE/trusted-tools/.github/forward-rows/narrow.sh"`,
	}
	at := 0
	for _, want := range order {
		i := strings.Index(gate[at:], want)
		if i < 0 || strings.Count(gate, want) != 1 {
			t.Fatalf("the narrowing gate lacks %q once, after the steps before it", want)
		}
		at += i + len(want)
	}
	// The gate fails on the check rather than ignoring it: no `|| true` or `|| exit 0` follows it.
	if strings.Contains(gate, "pin-lag.sh\" \"$scripts\""+args[:len(args)-1]+" ||") || strings.Contains(gate, "pin-lag.sh\" \"$main\""+args[:len(args)-1]+" ||") {
		t.Fatal("the narrowing gate ignores a failing pin-lag.sh")
	}
	if !regexp.MustCompile(`(?m)^        id: narrow\n`).MatchString(ci) || !strings.Contains(gate, "continue-on-error: true") {
		t.Fatal("the narrowing gate's failure no longer leaves the shard FULL")
	}
}

// TestCINV0011ToolsBuildFromComparedMain pins CIN-V0-011 (V1-4354): the pin-main step resolves
// the commit of main the CIN-V0-010 gate compares against, the tested merge's first parent or the
// queue base (its newest main ancestor when it is an unmerged entry), and the trusted tools build
// from it whenever it descends from the pin; otherwise the pin stays the floor. Every step that
// fetches, builds, records or checks the tools reads the same resolved commit.
func TestCINV0011ToolsBuildFromComparedMain(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the pin-main step is a shell script")
	b, err := os.ReadFile("../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	ci := string(b)
	m := regexp.MustCompile(`(?ms)^        id: pin-main\n.*?^        run: \|\n(.*?)^      - name: Capture trusted tool source outside tested checkout\n`).FindStringSubmatch(ci)
	if m == nil {
		t.Fatal("ci.yml lacks the pin-main step before the tool capture")
	}
	body := regexp.MustCompile(`(?m)^          `).ReplaceAllString(m[1], "")
	tool, shadow := "${{ steps.pin-main.outputs.tool || env.CORVINT_PR_TOOL_SOURCE }}", "${{ steps.pin-main.outputs.shadow || env.CORVINT_TEST_SHADOW_SOURCE }}"
	for _, want := range []struct {
		text string
		n    int
	}{{"          ref: " + tool + "\n", 1}, {"          ref: " + shadow + "\n", 1}, {"          TOOL_SOURCE: " + tool + "\n", 1},
		{"          CORVINT_PR_TOOL_SOURCE: " + tool + "\n", 3}, {"          CORVINT_TEST_SHADOW_SOURCE: " + shadow + "\n", 4},
		{"env.CORVINT_PR_TOOL_SOURCE }}\n", 5}, {"env.CORVINT_TEST_SHADOW_SOURCE }}\n", 5}} {
		if got := strings.Count(ci, want.text); got != want.n {
			t.Fatalf("%d bindings of %q, want %d", got, want.text, want.n)
		}
	}

	repo := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.org", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.org")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	commit := func(msg string, parents ...string) string {
		args := []string{"commit-tree", "-m", msg}
		for _, p := range parents {
			args = append(args, "-p", p)
		}
		return git(append(args, git("write-tree"))...)
	}
	old := commit("before the pin")
	pin := commit("pin", old)
	tip := commit("main moved", pin)
	git("update-ref", "refs/remotes/origin/main", tip)
	pr := commit("pr", pin)
	side := commit("off main", pin)
	entry := commit("queued entry", tip, pr)
	run := func(event, merge, base, floor string) string {
		t.Helper()
		out := filepath.Join(t.TempDir(), "out")
		cmd := exec.Command("bash", "-e", "-c", body)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GITHUB_OUTPUT="+out, "GITHUB_EVENT_NAME="+event, "TESTED_MERGE="+merge, "QUEUE_BASE="+base,
			"CORVINT_PR_TOOL_SOURCE="+floor, "CORVINT_TEST_SHADOW_SOURCE="+floor)
		if log, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("pin-main: %v\n%s", err, log)
		}
		got, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		return string(got)
	}
	want := func(c string) string { return "tool=" + c + "\nshadow=" + c + "\n" }
	for _, tc := range []struct{ name, event, merge, base, floor, want string }{
		{"pull request on moved main", "pull_request", commit("merge", tip, pr), "", pin, want(tip)},
		{"pull request whose first parent is off main", "pull_request", commit("merge", side, pr), "", pin, want(pin)},
		{"pull request on main older than the pin", "pull_request", commit("merge", old, pr), "", pin, want(pin)},
		{"merge-queue entry on main", "merge_group", "", tip, pin, want(tip)},
		{"merge-queue entry behind an unmerged entry", "merge_group", "", entry, pin, want(tip)},
		{"merge-queue entry with no main ancestor", "merge_group", "", commit("orphan"), pin, want(pin)},
		{"empty floor", "pull_request", commit("merge", tip, pr), "", "", want("")},
	} {
		if got := run(tc.event, tc.merge, tc.base, tc.floor); got != tc.want {
			t.Fatalf("%s: outputs %q, want %q", tc.name, got, tc.want)
		}
	}
}
