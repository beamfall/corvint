// SPDX-License-Identifier: Apache-2.0
package forwardrow

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testplatform"
)

// shardTestFunctions returns run_slice and run_tests from the go-product-shard Go tests step,
// dedented to plain bash.
func shardTestFunctions(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	ci := string(b)
	start := strings.Index(ci, "\n          run_slice() {\n")
	if start < 0 {
		t.Fatal("ci.yml has no run_slice function")
	}
	tests := strings.Index(ci[start:], "\n          run_tests() {\n")
	if tests < 0 {
		t.Fatal("ci.yml has no run_tests function after run_slice")
	}
	end := strings.Index(ci[start+tests:], "\n          }\n")
	if end < 0 {
		t.Fatal("ci.yml run_tests has no closing brace")
	}
	var out []string
	for _, line := range strings.Split(ci[start+1:start+tests+end+len("\n          }")], "\n") {
		out = append(out, strings.TrimPrefix(line, "          "))
	}
	return strings.Join(out, "\n") + "\n"
}

// fakeGo stands in for go test: it logs its argv, marks a slice's start, makes the whole-package
// invocation wait (when FAKE_WAIT is set) until both slices started, and prints one start and
// one terminal event per package; FAKE_FAIL names a package that fails.
const fakeGo = `#!/bin/sh
[ "$1" = test ] || exit 2
shift
echo "argv $*" >> "$FAKE_LOG"
while [ $# -gt 0 ]; do
  case "$1" in
    -exec | -p | -timeout) shift 2 ;;
    -json | -count=1 | -race) shift ;;
    *) break ;;
  esac
done
status=0
case "$1" in
  -run | -skip)
    : > "$FAKE_MARKS/$(basename "$3")"
    echo "slice stderr $3" >&2
    [ "$3" != "$FAKE_SLOW" ] || sleep 0.3
    shift 2
    ;;
  *)
    i=0
    while [ -n "$FAKE_WAIT" ] && [ "$(ls "$FAKE_MARKS" | wc -l)" -lt 2 ] && [ "$i" -lt 100 ]; do
      sleep 0.1
      i=$((i + 1))
    done
    echo "whole saw $(ls "$FAKE_MARKS" | wc -l | tr -d ' ') slice start(s)" >> "$FAKE_LOG"
    ;;
esac
for pkg in "$@"; do
  action=pass
  if [ "$pkg" = "$FAKE_FAIL" ]; then action=fail; status=1; fi
  printf '{"Action":"start","Package":"%s"}\n{"Action":"%s","Package":"%s"}\n' "$pkg" "$action" "$pkg"
done
exit "$status"
`

type shardRun struct {
	rc            string
	packages      []string
	failedSlices  string
	log, stderr   string
	wholeFailedOK bool
}

// runShardTests runs the workflow's run_tests with the fake go on PATH and returns the
// capture's package order, the failed-slices file, the fake's log and the function's status.
func runShardTests(t *testing.T, parallel, wait, fail string) shardRun {
	t.Helper()
	testplatform.SkipPOSIXShell(t, "the Go tests step's functions run under bash")
	if _, err := exec.LookPath("bash"); err != nil {
		t.Fatalf("the Go tests step needs bash on PATH: %v", err)
	}
	dir := t.TempDir()
	bin, marks, outcomes := filepath.Join(dir, "bin"), filepath.Join(dir, "marks"), filepath.Join(dir, "outcomes")
	for _, d := range []string{bin, marks, outcomes} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(bin, "go"), []byte(fakeGo))
	if err := os.Chmod(filepath.Join(bin, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(outcomes, "failed-slices"), nil)
	body := shardTestFunctions(t) + `packages="ex/a ex/b"
slices=$(printf '%s\n' '-run ^(TestOne)$ ex/s1' '-skip ^(TestOne)$ ex/s2')
outcomes_dir="$OUT"
rc=0
run_tests > "$CAPTURE" || rc=$?
echo "$rc"
`
	cmd := exec.Command("bash", "-c", body)
	cmd.Env = []string{
		"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH"), "HOME=" + dir, "RUNNER_TEMP=" + dir,
		"SHARD_TEST_P=" + parallel, "FAKE_LOG=" + filepath.Join(dir, "log"), "FAKE_MARKS=" + marks,
		"FAKE_WAIT=" + wait, "FAKE_FAIL=" + fail, "FAKE_SLOW=ex/s1", "OUT=" + outcomes, "CAPTURE=" + filepath.Join(dir, "capture"),
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("run_tests: %v\n%s", err, stderr.String())
	}
	capture, err := os.ReadFile(filepath.Join(dir, "capture"))
	if err != nil {
		t.Fatal(err)
	}
	var run shardRun
	for _, line := range strings.Split(strings.TrimSpace(string(capture)), "\n") {
		var ev struct{ Action, Package string }
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("capture line %q: %v", line, err)
		}
		run.packages = append(run.packages, ev.Action+" "+ev.Package)
	}
	failed, _ := os.ReadFile(filepath.Join(outcomes, "failed-slices"))
	log, _ := os.ReadFile(filepath.Join(dir, "log"))
	_, statErr := os.Stat(filepath.Join(outcomes, "whole-failed"))
	run.rc, run.failedSlices, run.log, run.stderr, run.wholeFailedOK = strings.TrimSpace(string(out)), string(failed), string(log), stderr.String(), os.IsNotExist(statErr)
	return run
}

// TestAFPV0541ParallelSlicesKeepTheCaptureLayout: with SHARD_TEST_P above 1 a shard's slices run
// beside its whole packages, yet the capture still holds the whole packages first and then each
// slice in slice order with one terminal event per package, a failed slice is named once in
// failed-slices, its stderr is replayed, and every invocation carries the same -p. With -p 1 the
// slices start only after the whole packages, as before.
func TestAFPV0541ParallelSlicesKeepTheCaptureLayout(t *testing.T) {
	want := []string{"start ex/a", "pass ex/a", "start ex/b", "pass ex/b", "start ex/s1", "fail ex/s1", "start ex/s2", "pass ex/s2"}
	for _, tc := range []struct {
		parallel, wait, saw string
	}{
		{"4", "1", "whole saw 2 slice start(s)"},
		{"1", "", "whole saw 0 slice start(s)"},
	} {
		run := runShardTests(t, tc.parallel, tc.wait, "ex/s1")
		if run.rc != "1" {
			t.Errorf("-p %s: run_tests exited %q, want 1 for the failed slice", tc.parallel, run.rc)
		}
		if strings.Join(run.packages, "\n") != strings.Join(want, "\n") {
			t.Errorf("-p %s: capture events\n%s\nwant\n%s", tc.parallel, strings.Join(run.packages, "\n"), strings.Join(want, "\n"))
		}
		if run.failedSlices != "-run ^(TestOne)$ ex/s1\n" || !run.wholeFailedOK {
			t.Errorf("-p %s: failed-slices %q (whole-failed absent %v), want only the failed slice", tc.parallel, run.failedSlices, run.wholeFailedOK)
		}
		if !strings.Contains(run.log, tc.saw) {
			t.Errorf("-p %s: fake go log %q, want %q", tc.parallel, run.log, tc.saw)
		}
		if strings.Count(run.log, "argv ") != 3 || strings.Count(run.log, " -json -p "+tc.parallel+" -count=1 -race -timeout 50m ") != 3 {
			t.Errorf("-p %s: invocations %q, want three with -json -p %s -count=1 -race -timeout 50m", tc.parallel, run.log, tc.parallel)
		}
		if !strings.Contains(run.stderr, "slice stderr ex/s1") || !strings.Contains(run.stderr, "slice stderr ex/s2") {
			t.Errorf("-p %s: stderr %q, want each slice's stderr", tc.parallel, run.stderr)
		}
	}
}

// TestAFPV0540PullRequestParallelismBinding: only a self-hosted pull-request shard runs above
// -p 1; the whole, slice and rerun invocations all take SHARD_TEST_P, and run_tests runs slices
// in the background only when it is above 1 and their packages are distinct.
func TestAFPV0540PullRequestParallelismBinding(t *testing.T) {
	b, err := os.ReadFile("../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	ci := string(b)
	const env = "          SHARD_TEST_P: ${{ github.event_name == 'pull_request' && runner.environment == 'self-hosted' && '4' || '1' }}\n"
	if strings.Count(ci, env) != 1 || strings.Count(ci, "SHARD_TEST_P:") != 1 {
		t.Fatalf("ci.yml must set SHARD_TEST_P once, as %q", env)
	}
	if n := len(regexp.MustCompile(`go test -exec "\$RUNNER_TEMP/test-confine" -json -p "\$SHARD_TEST_P" -count=1 -race -timeout 50m `).FindAllString(ci, -1)); n != 3 {
		t.Fatalf("ci.yml has %d go test invocations with -p \"$SHARD_TEST_P\", want the whole, slice and rerun ones", n)
	}
	if !strings.Contains(ci, `if [ -n "$slices" ] && [ "$SHARD_TEST_P" != 1 ] && [ -z "$(awk '{ print $3 }' <<< "$slices" | sort | uniq -d)" ]; then`) {
		t.Fatal("run_tests must run slices in the background only above -p 1 and for distinct packages")
	}
}
