package dynamic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// qualification0592 parses a raw native report retained from an actual V1-0592 run through
// cmd/corvint-test-runner, after checking its bytes against provenance.json.
func qualification0592(t *testing.T, runner, name string) tr.Observation {
	t.Helper()
	o, err := qualification0592Parse(t, runner, name)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return o
}

func qualification0592Parse(t *testing.T, runner, name string) (tr.Observation, error) {
	t.Helper()
	dir := "testdata/qualification-0592"
	raw, err := os.ReadFile(filepath.Join(dir, "provenance.json"))
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		Fixtures map[string]struct {
			ExitCode int    `json:"exitCode"`
			SHA256   string `json:"sha256"`
		} `json:"fixtures"`
	}
	if err = json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	f, ok := p.Fixtures[name]
	if !ok {
		t.Fatalf("%s missing from provenance", name)
	}
	report, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	if tr.Digest(report) != f.SHA256 {
		t.Fatalf("%s bytes differ from provenance", name)
	}
	in := tr.Input{Runner: runner, ExitCode: f.ExitCode, SuccessExitCodes: []int{0}, FailureExitCodes: []int{1}}
	if runner == "ava" {
		in.Stdout = report
	} else {
		in.Reports = map[string][]byte{name: report}
	}
	return Parse(in)
}

func states0592(o tr.Observation) map[string]int {
	m := map[string]int{}
	for _, x := range o.Tests {
		m[x.State]++
	}
	return m
}

// TRE-V0-060: Node's synthetic file-level event is a no-tests or a collection result, never a test.
func TestNodeFileLevelEventsAreNotTests(t *testing.T) {
	// The zero and collection runs report Node's file-level event; the collision runs report a
	// real top-level test at 1:1 named after its file (path or basename), which Node 22.23.3
	// reports with the same fields. Neither may become a test or a no-tests/collection verdict.
	for _, file := range []string{"node-zero.jsonl", "node-collection.jsonl", "node-filename-collision.jsonl", "node-filename-collision-fail.jsonl", "node-basename-collision.jsonl"} {
		o := qualification0592(t, "node-test", file)
		codes := problemCodes(o)
		if o.Complete || len(o.Tests) != 0 || !codes["ambiguous-test-identity"] {
			t.Fatalf("%s: complete=%v tests=%+v problems=%+v", file, o.Complete, o.Tests, o.Problems)
		}
		if codes["collection-or-hook-error"] || codes["count-mismatch"] || codes["success-conflict"] || codes["node-run-unsuccessful"] {
			t.Fatalf("%s: unproven or misleading codes %+v", file, o.Problems)
		}
	}
	o := qualification0592(t, "node-test", "node-mixed.jsonl")
	if s := states0592(o); !o.Complete || s[tr.Passed] != 3 || s[tr.Failed] != 1 || s[tr.Skipped] != 2 {
		t.Fatalf("mixed regressed: complete=%v states=%v problems=%+v", o.Complete, s, o.Problems)
	}
}

func TestAVARemainingTestsAfterHookFailure(t *testing.T) {
	o := qualification0592(t, "ava", "ava-hook.tap")
	if o.Complete || !problemCodes(o)["collection-or-hook-error"] {
		t.Fatalf("hook failure accepted: complete=%v problems=%+v", o.Complete, o.Problems)
	}
	o = qualification0592(t, "ava", "ava-mixed.tap")
	if s := states0592(o); !o.Complete || s[tr.Passed] != 3 || s[tr.Failed] != 1 || s[tr.Skipped] != 2 {
		t.Fatalf("mixed regressed: complete=%v states=%v problems=%+v", o.Complete, s, o.Problems)
	}
}

// TRE-V0-062: Bun describe suites carry a line attribute; an unnamed hook case is a setup error;
// a retried test repeats its identity and stays ambiguous.
func TestBunNativeJUnitShapes(t *testing.T) {
	o := qualification0592(t, "bun-test", "bun-mixed.xml")
	if s := states0592(o); !o.Complete || s[tr.Passed] != 3 || s[tr.Failed] != 1 || s[tr.Skipped] != 2 {
		t.Fatalf("mixed: complete=%v states=%v problems=%+v", o.Complete, s, o.Problems)
	}
	o = qualification0592(t, "bun-test", "bun-hook.xml")
	codes := problemCodes(o)
	if o.Complete || !codes["setup-or-runtime-error"] || codes["count-mismatch"] || states0592(o)[tr.Failed] != 0 {
		t.Fatalf("hook: complete=%v tests=%+v problems=%+v", o.Complete, o.Tests, o.Problems)
	}
	o, err := qualification0592Parse(t, "bun-test", "bun-retry.xml")
	if err == nil || o.Complete {
		t.Fatalf("retry: err=%v complete=%v problems=%+v", err, o.Complete, o.Problems)
	}
	if closedXML([]byte(`<testsuites><testsuite name="s" tests="0" line="x" line="y"/></testsuites>`)) == nil {
		t.Fatal("duplicate suite line attribute accepted")
	}
}

// TRE-V0-063: the legacy Storybook test-runner's skipped "no-op" placeholder for a CSF file without
// stories is not a story outcome, so a run of only such files reports no tests.
func TestStorybookRunnerEmptyCSFPlaceholderIsNotATest(t *testing.T) {
	o := qualification0592(t, "storybook-test-runner", "storybook-runner-zero.json")
	codes := problemCodes(o)
	if o.Complete || len(o.Tests) != 0 || !codes["no-tests"] || codes["count-mismatch"] || o.RetryInformation == tr.Retained {
		t.Fatalf("zero: complete=%v tests=%+v problems=%+v", o.Complete, o.Tests, o.Problems)
	}
	o = qualification0592(t, "storybook-test-runner", "storybook-runner-mixed.json")
	if s := states0592(o); !o.Complete || s[tr.Passed] != 3 || s[tr.Failed] != 1 || s[tr.Skipped] != 1 {
		t.Fatalf("mixed regressed: complete=%v states=%v problems=%+v", o.Complete, s, o.Problems)
	}
	// The same row is an ordinary skipped Jest test outside the Storybook profile.
	o = qualification0592(t, "jest", "storybook-runner-zero.json")
	if len(o.Tests) != 1 || o.Tests[0].State != tr.Skipped {
		t.Fatalf("jest placeholder handling leaked: tests=%+v", o.Tests)
	}
}
