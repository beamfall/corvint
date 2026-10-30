package corpusbridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

const evalDiagnosticsCases = `{"schema":"corvint-corpus-eval-cases/1","origin":"labelled","cases":[
{"id":"beyond-k","operation":"search","arguments":{"query":"Checkout step","limit":1},"expect":{"subjects":["native:file:docs/p2.md"]}},
{"id":"never-served","operation":"search","arguments":{"query":"Checkout step","limit":1},"expect":{"paths":["docs/absent.md"]}},
{"id":"passing","operation":"search","arguments":{"query":"Checkout step","limit":1},"expect":{"subjects":["native:file:docs/p0.md"]}}
]}`

// DCX-V1-473 (GitHub #1666): a failing case lists its served top-k rows with
// rank, id, kind, name and path, and a missed answer reports the first rank
// it is served at beyond k; a passing case carries neither, and the score is
// unchanged.
func TestDCXV1473FailingCasesListReturnedRows(t *testing.T) {
	root, _ := lazyFixture(t)
	report := evalRun(t, root, evalDiagnosticsCases)
	beyond := evalCase(t, report, "beyond-k")
	if beyond.Status != "fail" || beyond.Hit || beyond.Returned != 1 || len(beyond.ReturnedRows) != 1 {
		t.Fatalf("beyond-k = %+v", beyond)
	}
	row := beyond.ReturnedRows[0]
	if row.Rank != 1 || row.ID != "native:file:docs/p0.md" || row.Kind == "" || row.Name == "" || row.Path != "docs/p0.md" {
		t.Fatalf("returned row = %+v", row)
	}
	if beyond.ExpectedRank == nil || *beyond.ExpectedRank < 2 {
		t.Fatalf("expected rank beyond k missing: %+v", beyond)
	}
	never := evalCase(t, report, "never-served")
	if never.Status != "fail" || never.ExpectedRank != nil || len(never.ReturnedRows) != 1 {
		t.Fatalf("never-served = %+v", never)
	}
	passing := evalCase(t, report, "passing")
	if passing.Status != "pass" || passing.ReturnedRows != nil || passing.ExpectedRank != nil {
		t.Fatalf("passing case carries diagnostics: %+v", passing)
	}
	if got := boundedEvalText(strings.Repeat("é", 100), 9); got != "éééé..." {
		t.Fatalf("bounded text = %q", got)
	}
}

// DCX-V1-474 (GitHub #1666): a suite opens the artifact and observes the
// checkout once, answers every case from that observation, and checks once
// after the last case; a checkout that changed while the cases ran refuses
// the evaluation instead of mixing answers from two states.
func TestDCXV1474EvalSuiteObservesOnce(t *testing.T) {
	root, _ := lazyFixture(t)
	observed := 0
	saved := observeCorpus
	t.Cleanup(func() { observeCorpus = saved })
	observeCorpus = func(ctx context.Context, root string, a *doccorpus.Artifact) (string, error) {
		observed++
		return saved(ctx, root, a)
	}
	entries := make([]string, 12)
	for i := range entries {
		entries[i] = fmt.Sprintf(`{"id":"c%02d","operation":"search","arguments":{"query":"Checkout step 1"},"expect":{"subjects":["native:file:docs/p1.md"]}}`, i)
	}
	body := `{"schema":"corvint-corpus-eval-cases/1","origin":"labelled","cases":[` + strings.Join(entries, ",") + `]}`
	report := evalRun(t, root, body)
	if report.Summary.Overall.Passed != 12 {
		t.Fatalf("cases failed: %+v", report.Summary.Overall)
	}
	if observed > 2 {
		t.Fatalf("a 12-case suite observed the checkout %d times, want at most 2 (open and close)", observed)
	}

	// The checkout changes after the suite's observation: the close check
	// sees a changed stamp, reobserves, and refuses.
	calls := 0
	observeCorpus = func(ctx context.Context, root string, a *doccorpus.Artifact) (string, error) {
		calls++
		key, err := saved(ctx, root, a)
		if calls == 1 {
			if werr := os.WriteFile(filepath.Join(root, "docs", "p1.md"), []byte("# Page 1\n\nChanged.\n"), 0o600); werr != nil {
				t.Fatal(werr)
			}
			return key, err
		}
		return key + "-changed", err
	}
	parsed, err := ParseEvalCases([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	registry, rerr := New(root, "corpus.json")
	if rerr != nil {
		t.Fatal(rerr)
	}
	if _, err := registry.Evaluate(context.Background(), parsed, "cases.json", []byte(body)); err == nil || !strings.Contains(err.Error(), "checkout changed while the cases ran") {
		t.Fatalf("a checkout changed mid-suite should refuse the evaluation: %v", err)
	}
}
