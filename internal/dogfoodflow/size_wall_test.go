package dogfoodflow

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tracerecordrepo"
)

// overLimitFixture commits 201 source files over an empty base and returns the
// root, BASE and the complete aggregate receipt for BASE..HEAD.
func overLimitFixture(t *testing.T) (root, base string, expected tracerecordrepo.AggregateExpected, receipt []byte) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "init", "-q")
	testGit(t, root, "commit", "--allow-empty", "-qm", "base")
	base = testGit(t, root, "rev-parse", "HEAD")
	for i := 0; i < 201; i++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("f%03d.go", i)), []byte(fmt.Sprintf("package fixture\nconst V%d = %d\n", i, i)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "source")
	expected, err = tracerecordrepo.AggregateExpectedFor(context.Background(), root, base, "HEAD", "large change", []string{"go test ./..."}, "passed")
	if err != nil {
		t.Fatal(err)
	}
	if receipt, err = tracerecordrepo.ProduceAggregateOutcome(context.Background(), root, expected); err != nil {
		t.Fatal(err)
	}
	return root, base, expected, receipt
}

// DCW-V0-374: corvint dogfood change opts the recorder into the nonlearning
// aggregate over 200 paths and admits only a canonical aggregate receipt; a
// run without the opt-in (local completion) never accepts one.
func TestDCWV0374LocalOutcomeAdmitsOptedInAggregate(t *testing.T) {
	root, base, expected, receipt := overLimitFixture(t)
	evidence := filepath.Join(root, ".git", "corvint")
	if err := os.MkdirAll(evidence, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		optIn  bool
		output []byte
		want   step
	}{
		{"aggregate", true, receipt, step{"local-outcome", "PRODUCED", "none"}},
		{"forged", true, bytes.Replace(receipt, []byte(`"f000.go"`), []byte(`"f999.go"`), 1), step{"local-outcome", "NOT_PRODUCED", "invalid-record-admission-output"}},
		{"not-opted-in", false, receipt, step{"local-outcome", "NOT_PRODUCED", "invalid-record-admission-output"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var seen []string
			c := &change{flow: flow{ctx: context.Background(), root: root, base: base, target: expected.Target}, evidence: evidence,
				options: ChangeOptions{Verify: "go test ./...", Outcome: "passed", AggregateOverLimit: tc.optIn, Steps: Runner{Run: func(_ context.Context, _ string, args []string, stdout, _ io.Writer) int {
					seen = args
					_, _ = stdout.Write(tc.output)
					return 0
				}}},
			}
			if row := c.localOutcome("large change"); row != tc.want {
				t.Fatalf("row=%+v", row)
			}
			if slices.Contains(seen, "--aggregate-over-limit") != tc.optIn {
				t.Fatalf("opt-in flag mismatch: %q", seen)
			}
		})
	}
}

// DCW-V0-375: dogfood check reacquires an aggregate local outcome from BASE and
// HEAD; a receipt for another range, or one the current tree no longer
// reproduces, fails closed.
func TestDCWV0375CheckReverifiesAggregateOutcome(t *testing.T) {
	root, base, expected, receipt := overLimitFixture(t)
	evidence := filepath.Join(root, ".git", "corvint", "local-outcome.json")
	if err := os.MkdirAll(filepath.Dir(evidence), 0o700); err != nil {
		t.Fatal(err)
	}
	run := func(raw []byte, checkBase string) (code int, stderr string) {
		if err := os.WriteFile(evidence, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		out := new(bytes.Buffer)
		c := &check{flow: flow{ctx: context.Background(), root: root, base: checkBase, target: expected.Target, stderr: out}}
		defer func() {
			if r := recover(); r != nil {
				code, stderr = r.(stop).code, out.String()
			}
		}()
		c.verifyAggregateOutcome(evidence)
		return 0, out.String()
	}
	if code, stderr := run(receipt, base); code != 0 {
		t.Fatalf("valid aggregate refused: %d %s", code, stderr)
	}
	if code, stderr := run(receipt, expected.Target); code != 1 || !strings.Contains(stderr, "FAIL local-outcome-evidence-drift") {
		t.Fatalf("foreign range accepted: %d %s", code, stderr)
	}
	if code, _ := run([]byte(`{"state":"recorded"}`+"\n"), base); code != 0 {
		t.Fatalf("a legacy receipt keeps the digest binding alone: %d", code)
	}
	if err := os.WriteFile(filepath.Join(root, "f000.go"), []byte("package fixture\nconst Changed = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, stderr := run(receipt, base); code != 1 || !strings.Contains(stderr, "does not reproduce") {
		t.Fatalf("unreproducible aggregate accepted: %d %s", code, stderr)
	}
}

// DCW-V0-374: the aggregate receipt an unenrolled change records is that run's
// evidence while its plain change report binds the receipt's digest, so the
// change can rerun; an unbound receipt or an aggregate report stays a marker.
func TestDCWV0374UnenrolledAggregateReceiptIsNotAMarker(t *testing.T) {
	root, _, _, receipt := overLimitFixture(t)
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(filepath.Join(gitDir, "corvint"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o700); err != nil {
		t.Fatal(err)
	}
	write := func(name string, raw []byte) {
		if err := os.WriteFile(name, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	report := filepath.Join(root, ".corvint", "dogfood-report.json")
	write(filepath.Join(gitDir, "corvint", "local-outcome.json"), receipt)
	changeReport := func(digest string) []byte {
		return []byte("{\n  \"profile\": \"corvint-dogfood-change/0\"\n  ,\"localOutcomeEvidenceSha256\": " + digest + "\n}\n")
	}
	for _, tc := range []struct {
		name   string
		report []byte
		marker bool
	}{
		{"bound", changeReport(`"sha256:` + sha256Hex(receipt) + `"`), false},
		{"other-digest", changeReport(`"sha256:` + strings.Repeat("0", 64) + `"`), true},
		{"null-digest", changeReport("null"), true},
		{"no-report", nil, true},
		{"aggregate-report", []byte(`{"profile":"` + AggregateReportProfile + `"}`), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_ = os.Remove(report)
			if tc.report != nil {
				write(report, tc.report)
			}
			marker, err := AggregateMarkerPresent(root, gitDir)
			if err != nil || marker != tc.marker {
				t.Fatalf("marker=%v err=%v, want %v", marker, err, tc.marker)
			}
		})
	}
}

// DCW-V0-376: a manifest of more than 16 well-formed scopes reports the scope
// bound it exceeds, not a missing scope, and its fix names the routes.
func TestDCWV0376IntentCountOverBoundIsNamed(t *testing.T) {
	manifest := filepath.Join(t.TempDir(), "intents")
	lines := new(strings.Builder)
	for i := 0; i <= maxAggregateScopes; i++ {
		fmt.Fprintf(lines, "docs/specs/s%02d.md\n", i)
	}
	if err := os.WriteFile(manifest, []byte(lines.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	c := new(change)
	c.options.IntentsFile = manifest
	if got := c.validateIntentManifest(); got != "intent-scope-limit" {
		t.Fatalf("17 scopes: %q", got)
	}
	if hint := c.fixHint(step{name: "ocm-aggregate", reason: "intent-scope-limit"}); !strings.Contains(hint, "deferred intent (DCW-V0-371)") {
		t.Fatalf("hint %q", hint)
	}
}
