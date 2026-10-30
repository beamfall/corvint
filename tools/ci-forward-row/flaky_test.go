// SPDX-License-Identifier: Apache-2.0
package forwardrow

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testplatform"
)

// runFlaky runs flaky.sh on DIR and STREAM with only PATH, HOME, the outcomes environment and
// GITHUB_STEP_SUMMARY, and returns its stdout and exit status.
func runFlaky(t *testing.T, dir, stream, summary string) (string, int) {
	t.Helper()
	testplatform.SkipPOSIXShell(t, "flaky.sh is a POSIX shell script run by sh")
	cmd := exec.Command("sh", script(t, "flaky.sh"), dir, stream)
	cmd.Env = append([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir(), "GITHUB_STEP_SUMMARY=" + summary},
		outcomesEnv("pull_request", "38476173623", "406dce90998aa2f4fb52613ef099267dfd5c2a60", "", "", "7")...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return string(out), 0
	case errors.As(err, &exit):
		return string(out) + stderr.String(), exit.ExitCode()
	}
	t.Fatalf("flaky.sh: %v", err)
	return "", 0
}

// TestAFPV0441FlakyShard: a failed go-product-shard passes, labelled FLAKY, only when every
// failed invocation passed its one bounded rerun; a second failure still fails it.
func TestAFPV0441FlakyShard(t *testing.T) {
	fixture := func(t *testing.T, name string) string {
		p, err := filepath.Abs(filepath.Join("testdata", "streams", name))
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	event := func(pkg, action string) string { return fmt.Sprintf(`{"Action":%q,"Package":%q}`, action, pkg) }
	failedTest := `{"Action":"fail","Package":"github.com/Beamfall/corvint/internal/contextindex","Test":"TestFlake"}`
	// shard runs outcomes.sh first on STREAM and, when it names a rerun, outcomes.sh rerun on
	// RERUN with rerun-status RERUNSTATUS, as the go-product-shard tests step does.
	type shard struct {
		status, rerunStatus string
		expected, whole     []string
		slices              []string
		stream, rerun       string
		wholeFailed         bool
	}
	prepare := func(t *testing.T, s shard) string {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "expected"), lines(s.expected...))
		writeFile(t, filepath.Join(dir, "whole"), lines(s.whole...))
		writeFile(t, filepath.Join(dir, "failed-slices"), nil)
		if len(s.slices) > 0 {
			writeFile(t, filepath.Join(dir, "failed-slices"), lines(s.slices...))
		}
		if s.wholeFailed {
			writeFile(t, filepath.Join(dir, "whole-failed"), nil)
		}
		env := outcomesEnv("pull_request", "38476173623", "406dce90998aa2f4fb52613ef099267dfd5c2a60", "", "", "7")
		run(t, env, "sh", script(t, "outcomes.sh"), "first", dir, s.status, s.stream)
		if b, _ := os.ReadFile(filepath.Join(dir, "rerun-lines")); len(b) > 0 && s.rerun != "" {
			run(t, env, "sh", script(t, "outcomes.sh"), "rerun", dir, s.rerun)
			writeFile(t, filepath.Join(dir, "rerun-status"), []byte(s.rerunStatus+"\n"))
		}
		return dir
	}
	write := func(t *testing.T, events ...string) string {
		p := filepath.Join(t.TempDir(), "stream.json")
		writeFile(t, p, lines(events...))
		return p
	}

	t.Run("the real flaky shard passes its rerun and is labelled FLAKY", func(t *testing.T) {
		first := fixture(t, "pr38476173623-shard7-attempt1.json")
		starts := run(t, nil, "jq", "-R", "-r", `fromjson? | select(.Action == "start" and .Test == null) | .Package`, first)
		pkgs := strings.Fields(starts)
		dir := prepare(t, shard{status: "1", rerunStatus: "0", expected: pkgs, whole: pkgs, wholeFailed: true,
			stream: first, rerun: fixture(t, "pr38476173623-shard7-attempt2-contextindex.json")})
		summary := filepath.Join(t.TempDir(), "summary.md")
		out, code := runFlaky(t, dir, first, summary)
		if code != 0 {
			t.Fatalf("flaky.sh exited %d, want 0 (FLAKY):\n%s", code, out)
		}
		const test = "TestTaskContextCancellationNeverReturnsAPacket"
		if !regexp.MustCompile(`(?m)^::warning title=FLAKY go-product-shard 7::shard 7 failed its first attempt and passed its one rerun; first-attempt failures: .*` + flakyPkg + `\.` + test).MatchString(out) {
			t.Fatalf("no FLAKY annotation naming %s.%s:\n%s", flakyPkg, test, out)
		}
		b, err := os.ReadFile(summary)
		if err != nil || !strings.Contains(string(b), "### FLAKY: go-product-shard 7 of 8 passed only on its rerun") ||
			!strings.Contains(string(b), "- `"+flakyPkg+"."+test+"`") {
			t.Fatalf("job summary %q (%v) does not label the shard FLAKY with its failed test", b, err)
		}
		d := readDoc(t, filepath.Join(dir, "flaky.json"))
		if d["profile"] != "corvint-ci-flaky/0" || d["verdict"] != "FLAKY" || num(d["shard"]) != 7 || num(d["firstStatus"]) != 1 ||
			num(d["rerunInvocations"]) != 1 || num(d["runId"]) != 38476173623 || strings.Join(strs(d["failedPackages"]), ",") != flakyPkg {
			t.Fatalf("flaky.json = %v", d)
		}
		var named bool
		for _, x := range d["failedTests"].([]any) {
			if e := x.(doc); e["package"] == flakyPkg && e["test"] == test {
				named = true
			}
		}
		if !named {
			t.Fatalf("flaky.json failedTests %v do not name %s", d["failedTests"], test)
		}
	})

	notFlaky := []struct {
		name, reason string
		shard        shard
	}{
		{"a second failure still fails the shard", "the rerun exited 1", shard{status: "1", rerunStatus: "1",
			expected: []string{"a"}, whole: []string{"a"}, wholeFailed: true,
			stream: write(t, failedTest, event("a", "fail")), rerun: write(t, event("a", "fail"))}},
		{"a failed rerun outcome fails the shard even when the rerun exited 0", "a rerun invocation did not pass", shard{status: "1", rerunStatus: "0",
			expected: []string{"a"}, whole: []string{"a"}, wholeFailed: true,
			stream: write(t, event("a", "fail")), rerun: write(t, event("a", "fail"))}},
		{"a rerun its own timeout stopped fails the shard", "the rerun exited 124", shard{status: "1", rerunStatus: "124",
			expected: []string{"a"}, whole: []string{"a"}, wholeFailed: true,
			stream: write(t, event("a", "fail")), rerun: write(t, event("a", "pass"))}},
		{"a rerun missing its package fails the shard", "a rerun invocation did not pass", shard{status: "1", rerunStatus: "0",
			expected: []string{"a"}, whole: []string{"a"}, wholeFailed: true,
			stream: write(t, event("a", "fail")), rerun: write(t, event("b", "pass"))}},
		{"an interrupted rerun fails the shard", "the rerun state is INTERRUPTED, not RAN", shard{status: "1",
			expected: []string{"a"}, whole: []string{"a"}, wholeFailed: true, stream: write(t, event("a", "fail"))}},
		{"more than eight failed invocations fail the shard", "no failed invocation was named for the rerun", shard{status: "1",
			expected: []string{"p0", "p1", "p2", "p3", "p4", "p5", "p6", "p7", "p8"}, whole: []string{"p0", "p1", "p2", "p3", "p4", "p5", "p6", "p7", "p8"},
			stream: write(t, event("p0", "fail"), event("p1", "fail"), event("p2", "fail"), event("p3", "fail"), event("p4", "fail"),
				event("p5", "fail"), event("p6", "fail"), event("p7", "fail"), event("p8", "fail"))}},
		{"a failed whole invocation with no whole package named fails the shard",
			"the whole-package invocation failed with no whole package named for the rerun", shard{status: "1", rerunStatus: "0",
				expected: []string{"a", "s"}, whole: []string{"a"}, slices: []string{"-run ^TestTwo$ s"}, wholeFailed: true,
				stream: write(t, event("a", "pass"), event("s", "fail")), rerun: write(t, event("s", "pass"))}},
		{"a failed package left out of the rerun fails the shard", "failed package(s) not rerun: s", shard{status: "1", rerunStatus: "0",
			expected: []string{"a", "s"}, whole: []string{"a"}, wholeFailed: true,
			stream: write(t, event("a", "fail")), rerun: write(t, event("a", "pass"))}},
		{"a failed slice whose record was lost fails the shard", "failed invocation(s) not all rerun: s", shard{status: "1", rerunStatus: "0",
			expected: []string{"s", "s"}, slices: []string{"-run ^TestOne$ s"},
			stream: write(t, event("s", "fail"), event("s", "fail")), rerun: write(t, event("s", "pass"))}},
		{"a passing first attempt is never FLAKY", "the first attempt did not fail", shard{status: "0", rerunStatus: "0",
			expected: []string{"a"}, whole: []string{"a"}, stream: write(t, event("a", "pass"))}},
	}
	for _, tc := range notFlaky {
		t.Run(tc.name, func(t *testing.T) {
			dir := prepare(t, tc.shard)
			summary := filepath.Join(t.TempDir(), "summary.md")
			out, code := runFlaky(t, dir, tc.shard.stream, summary)
			if code == 0 {
				t.Fatalf("flaky.sh exited 0 (FLAKY), want a failure:\n%s", out)
			}
			if !strings.Contains(out, "AFP-V0-441: not FLAKY: "+tc.reason+"; the shard fails") || strings.Contains(out, "::warning") {
				t.Fatalf("output does not refuse with %q:\n%s", tc.reason, out)
			}
			if _, err := os.Stat(filepath.Join(dir, "flaky.json")); err == nil {
				t.Fatal("a refused shard wrote flaky.json")
			}
			if _, err := os.Stat(summary); err == nil {
				t.Fatal("a refused shard wrote a FLAKY job summary")
			}
		})
	}
}

// TestAFPV0441WorkflowBindings pins the go-product-shard wiring: the rerun has its own timeout and
// records its status, flaky.sh alone can clear a failed status before the exit check, the FLAKY
// record is retained, and no other job reruns or calls flaky.sh.
func TestAFPV0441WorkflowBindings(t *testing.T) {
	b, err := os.ReadFile("../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	ci := string(b)
	m := regexp.MustCompile(`(?ms)^  go-product-shard:\n(.*?)\n  [a-z][a-z0-9-]*:\n`).FindStringSubmatch(ci)
	if m == nil {
		t.Fatal("ci.yml has no go-product-shard job")
	}
	job := m[1]
	if strings.Count(ci, "flaky.sh") != strings.Count(job, "flaky.sh") || strings.Count(ci, "RERUN_MINUTES") != strings.Count(job, "RERUN_MINUTES") {
		t.Fatal("flaky.sh or RERUN_MINUTES appears outside the go-product-shard job")
	}
	order := []string{
		`RERUN_MINUTES: "15"`,
		`[ -z "$outcomes_dir" ] || : > "$outcomes_dir/whole-failed" || true`,
		`rm -f -- "$outcomes_dir/rerun-status"`,
		`timeout -k 1m "${RERUN_MINUTES}m" bash -c "$(declare -f rerun_invocations)"'; rerun_invocations "$1"' rerun "$outcomes_dir/rerun-lines"`,
		`echo "$rerun_status" > "$outcomes_dir/rerun-status"`,
		`sh .github/forward-rows/outcomes.sh rerun "$outcomes_dir" "$outcomes_dir/rerun.stream"`,
		`forward_outcomes "$status" || { outcomes_ok=""; `,
		`if [ "$status" != 0 ] && [ -n "$outcomes_ok" ] && sh .github/forward-rows/flaky.sh "$outcomes_dir" "$capture" &&`,
		`echo "flaky=1" >> "$GITHUB_OUTPUT"; then`,
		`[ "$status" = 0 ] || exit "$status"`,
		`[ -n "$flaky" ] || [ -n "$narrowed" ] || echo "costs=1" >> "$GITHUB_OUTPUT" || true`,
		`if: always() && steps.tests.outputs.flaky == '1'`,
		`name: ci-flaky-${{ github.event_name }}-${{ strategy.job-index }}`,
		`path: ${{ runner.temp }}/forward-outcomes/flaky.json`,
		`if-no-files-found: error`,
	}
	at := 0
	for _, want := range order {
		i := strings.Index(job[at:], want)
		if i < 0 {
			t.Fatalf("go-product-shard lacks %q after offset %d", want, at)
		}
		at += i + len(want)
	}
	// The FLAKY pass depends on its record: the upload step may not continue past a failure.
	upload := regexp.MustCompile(`(?ms)^      - name: Retain the FLAKY record \(AFP-V0-441\)\n(.*?)\n      - name: `).FindStringSubmatch(job)
	if upload == nil || strings.Contains(upload[1], "continue-on-error") || strings.Contains(upload[1], "if-no-files-found: ignore") {
		t.Fatal("the FLAKY record upload is missing or may fail without failing the shard")
	}
	step := regexp.MustCompile(`(?ms)^      - name: Go tests \(full until matching shadow qualification\)\n(.*?)\n      - name: `).FindStringSubmatch(job)
	if step == nil {
		t.Fatal("go-product-shard has no Go tests step")
	}
	if n := len(regexp.MustCompile(`(?m)^\s+status=0$`).FindAllString(step[1], -1)); n != 1 {
		t.Fatalf("the Go tests step clears its status %d times, want once, through flaky.sh", n)
	}
}
