package contextindex

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// evalContextParityTasks are deliberately contrasting: identifiers that only
// camelSplit reaches, hyphenated and snake_case ids, unicode -- including the two
// runes ToLower maps into ASCII -- a prompt no repository answers
// (OUT_OF_SCOPE), and queries wide enough to be truncated at a small budget
// (BUDGETED). A keep-set changes which terms a candidate yields, so a query
// class that ranks differently is the class that would expose a divergence.
var evalContextParityTasks = []string{
	"session expiry device revocation enforcement",
	"session-revocation",
	"EnforceSessionRevocation",
	"enforce_session_revocation",
	"where is the session heartbeat written",
	"corvint context packet retrieval budget for the agent tooling",
	"how does the plugin catalog validate a manifest signature",
	"queries parses aliases parameters maximum generator",
	"r\u00e9vocation na\u00efve caf\u00e9 session \u03a9",
	"\u212Aelvin session\u212Aey \u0130dentifier heartbeat",
	"zzqqxx nothing in this repository matches this prompt",
	"auth",
	"v2 API2Client HTTPServerHandler parseHTTPRequest",
}

// evalContextParityBudgets exercise the unbudgeted receipt and a budget tight
// enough to drive compileReceipt into BUDGETED.
var evalContextParityBudgets = []int{0, 1_500, 2_500, 3_500, 5_000, 8_000}

// evalContextReceipt runs the user-prompt pipeline over one index and returns
// the receipt as canonical JSON plus its state, so two constructions of
// candidate.context can be compared byte for byte.
func evalContextReceipt(index *Index, task string, budgetBytes int) ([]byte, string, error) {
	budget := &budgetBytes
	if budgetBytes == 0 {
		budget = nil
	}
	result, err := EvalQuery(context.Background(), index, task, 10, budget)
	if err != nil {
		return nil, "", err
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, "", err
	}
	return encoded, stringValue(result["state"]), nil
}

// TestKeepSetTermsAreByteIdenticalToUnfilteredTerms is the standing proof that
// every keepSetTerms call site is a pure allocation change. Each of them feeds a
// consumer that reads the term set only through an intersection with the
// keep-set -- evalRankSymbols against queryTerms, featureRelevantOverlap against
// featureTerms -- so a term outside that set can never reach a receipt. This
// asserts the claim where it counts, by emitting each receipt both ways in one
// binary and comparing the bytes rather than trusting the argument.
//
// CORVINT_EVAL_PARITY_REPOS=/path/a:/path/b widens it over real repositories.
func TestKeepSetTermsAreByteIdenticalToUnfilteredTerms(t *testing.T) {
	roots := []string{evalQueryRepository(t)}
	if extra := os.Getenv("CORVINT_EVAL_PARITY_REPOS"); extra != "" {
		roots = append(roots, strings.Split(extra, ":")...)
	}
	for _, root := range roots {
		index, err := BuildEval(context.Background(), root)
		if err != nil {
			t.Fatalf("BuildEval(%s): %v", root, err)
		}
		expected := withUnfilteredKeepSetTerms(t, func() []parityReceipt {
			return emitParityReceipts(index)
		})
		actual := emitParityReceipts(index)
		compared, states := compareParityReceipts(t, root, "unfiltered", "  filtered", expected, actual)
		if compared == 0 {
			t.Errorf("no receipt could be compared in %s", root)
		}
		for _, required := range []string{"OUT_OF_SCOPE", "BUDGETED"} {
			if states[required] == 0 {
				t.Errorf("%s: no %s receipt in the compared set: %v", root, required, states)
			}
		}
		t.Logf("%s: %d receipts byte-identical, states %v", root, compared, states)
	}
}

// withUnfilteredKeepSetTerms runs one matrix over the pre-filter construction.
// It is not parallel-safe by construction, which is why the parity test is the
// only caller and why it emits a whole matrix inside one bracket.
func withUnfilteredKeepSetTerms[Result any](t *testing.T, emit func() Result) Result {
	t.Helper()
	evalUnfilteredKeepSetTerms = true
	defer func() { evalUnfilteredKeepSetTerms = false }()
	return emit()
}
