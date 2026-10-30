package dogfoodflow

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

// DCW-V0-346 (V1-2809): an OCM aggregate bound refusal prints its per-scope
// detail inline, and its fix line names the intent subset that fits the bound
// with the largest total and the scopes to bind separately; the refusal itself
// stands, and shares that do not cover the manifest fall back to the generic
// route instead of a guessed subset.
func TestDCWV0346AggregateBoundNamesAFittingSubset(t *testing.T) {
	evidence, root := t.TempDir(), t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "commit", "-q", "--allow-empty", "-m", "base")
	var stderr bytes.Buffer
	c := &change{flow: flow{ctx: context.Background(), prefix: "dogfood-change", root: root, stderr: &stderr}}
	c.evidence = evidence
	c.base = testGit(t, root, "rev-parse", "HEAD")
	c.target = c.base
	c.intents = []string{"docs/specs/a.md", "docs/specs/b.md", "docs/specs/c.md"}
	write := func(message string) {
		t.Helper()
		if err := os.WriteFile(evidence+"/ocm-aggregate.stderr", []byte(`{"code": "too-many-obligations", "error": "`+message+`", "ok": false}`+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	message := "aggregate OCM obligations exceed the limit: observed 400, bound 256; per scope 1=200, 2=120, 3=80; narrow: list fewer intent scopes in the manifest and bind the rest as a separate staged change"
	write(message)
	bound := step{"ocm-aggregate", "NOT_PRODUCED", "too-many-obligations"}
	c.rows = []step{bound}
	if code := c.reportFailures(); code != 1 {
		t.Fatalf("exit %d", code)
	}
	out := stderr.String()
	for _, want := range []string{
		"  ocm-aggregate: too-many-obligations\n    detail: " + message + "\n",
		"    fix: keep in DOGFOOD_INTENTS_FILE: docs/specs/a.md=200 (total 200 of bound 256); bind separately as a staged change: docs/specs/b.md=120 docs/specs/c.md=80 (OCM-V0-024, DCW-V0-346)\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("stderr lacks %q:\n%s", want, out)
		}
	}
	// The best fit is the largest total, not the first scope.
	write("aggregate OCM claims exceed the limit: observed 300, bound 256; per scope 1=200, 2=60, 3=40; narrow: x")
	if got := c.fixHint(step{"ocm-aggregate", "NOT_PRODUCED", "too-many-claims"}); !strings.HasPrefix(got, "keep in DOGFOOD_INTENTS_FILE: docs/specs/a.md=200 docs/specs/c.md=40 (total 240 of bound 256); bind separately as a staged change: docs/specs/b.md=60 ") {
		t.Fatalf("fit = %q", got)
	}
	// One scope over the bound alone: no subset fits; split the spec.
	write("aggregate OCM obligations exceed the limit: observed 900, bound 256; per scope 1=300, 2=300, 3=300; narrow: x")
	if got := c.fixHint(bound); !strings.HasPrefix(got, "no intent scope fits the bound 256 alone (docs/specs/a.md=300 docs/specs/b.md=300 docs/specs/c.md=300); split the governing spec") {
		t.Fatalf("no-fit = %q", got)
	}
	// Shares that do not cover every declared intent, or no readable refusal:
	// the generic route, never a guessed subset.
	write("aggregate OCM obligations exceed the limit: observed 400, bound 256; per scope 1=200, 2=200; narrow: x")
	if got := c.fixHint(bound); !strings.HasPrefix(got, "the per-scope shares are NOT_OBSERVED") {
		t.Fatalf("mismatched shares = %q", got)
	}
	if err := os.Remove(evidence + "/ocm-aggregate.stderr"); err != nil {
		t.Fatal(err)
	}
	if detail, _, shares := c.aggregateBoundShares(); detail != "" || shares != nil {
		t.Fatalf("absent refusal: %q %v", detail, shares)
	}
	if got := c.fixHint(bound); !strings.HasPrefix(got, "the per-scope shares are NOT_OBSERVED") {
		t.Fatalf("absent refusal fix = %q", got)
	}
}
