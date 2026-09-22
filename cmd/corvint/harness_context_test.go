package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// ledgerRepository is a repository the beamfall profile recognises: it carries
// both record ledgers, so a query over it must rank feature records.
func ledgerRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		".gitignore": ".context-corvint/\n",
		"go.mod":     "module example.test/harness\n\ngo 1.27.0\n",
		"testing/features.yaml": "features:\n" +
			"  - id: session-revocation\n    area: auth\n    summary: Revoke an expired device session and enforce future denial.\n    adr: []\n    applies: [server]\n    status: shipped\n",
		"testing/scenarios.yaml": "scenarios:\n" +
			"  - id: revoked-device\n    area: auth\n    summary: Expired device session is revoked.\n    features: [session-revocation]\n    applies: [server]\n    status: shipped\n",
		"internal/auth/session.go": "package auth\n\n// feature:session-revocation\nfunc EnforceSessionRevocation() bool { return true }\n",
	}
	for path, content := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"},
		{"config", "user.name", "Corvint Test"}, {"add", "."}, {"commit", "-qm", "initial"},
	} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	return root
}

// TestHarnessUserPromptRanksLedgerRecords pins the index the user-prompt event
// compiles. The narrow query build loads only authority blobs and leaves the
// feature, scenario, marker and symbol tables empty, so an EvalQuery over it
// generates no candidates at all on a repository whose evidence lives in those
// ledgers -- the whole context packet silently degrades to OUT_OF_SCOPE.
func TestHarnessUserPromptRanksLedgerRecords(t *testing.T) {
	t.Parallel()
	root := ledgerRepository(t)
	var stdout, stderr bytes.Buffer
	input := strings.NewReader(`{"task":"session expiry device revocation enforcement"}`)
	if code := run(cliArguments(root, "user-prompt"), input, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, &stderr)
	}
	var response struct {
		Context struct {
			State    string `json:"state"`
			Coverage struct {
				RequestedResults int `json:"requested_results"`
			} `json:"coverage"`
			Results []struct {
				Kind string `json:"kind"`
				ID   string `json:"id"`
			} `json:"results"`
		} `json:"context"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Context.Coverage.RequestedResults == 0 {
		t.Fatalf("no candidates generated: %s", stdout.Bytes())
	}
	if response.Context.State == "OUT_OF_SCOPE" || len(response.Context.Results) == 0 {
		t.Fatalf("state = %q, results = %d", response.Context.State, len(response.Context.Results))
	}
	for _, result := range response.Context.Results {
		if result.Kind == "feature" && result.ID == "session-revocation" {
			return
		}
	}
	t.Fatalf("feature record missing from results: %s", stdout.Bytes())
}

func TestHarnessUserPromptRetainsRetrievalStateInMixedWorktree(t *testing.T) {
	t.Parallel()
	root := ledgerRepository(t)
	path := filepath.Join(root, "internal", "auth", "session.go")
	if err := os.WriteFile(path, []byte("package auth\n\nfunc DirtyOnly() bool { return false }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	input := strings.NewReader(`{"task":"session expiry device revocation enforcement"}`)
	if code := run(cliArguments(root, "user-prompt"), input, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, &stderr)
	}
	var response struct {
		Context struct {
			State     string `json:"state"`
			Freshness struct {
				State          string `json:"state"`
				MixedPathCount int    `json:"mixed_path_count"`
			} `json:"freshness"`
		} `json:"context"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Context.State != "READY" || response.Context.Freshness.State != "mixed-worktree" || response.Context.Freshness.MixedPathCount != 1 {
		t.Fatalf("context state=%q freshness=%+v", response.Context.State, response.Context.Freshness)
	}
}

// harnessQueryTaskTexts are contrasting prompts: an exact record hit, a symbol
// hit, a documentation hit, an agent-tooling intent, a nonsense prompt that must
// abstain, and a prompt whose terms straddle records and code. A table narrowing
// that is wrong for any one of them shows up here as a receipt byte difference.
var harnessQueryTaskTexts = []string{
	"session expiry device revocation enforcement",
	"session-revocation",
	"EnforceSessionRevocation",
	"where is the session heartbeat written",
	"corvint context packet retrieval budget for the agent tooling",
	"how does the plugin catalog validate a manifest signature",
	"zzqqxx nothing in this repository matches this prompt",
	"auth",
}

// harnessEvalReceipt runs the exact user-prompt pipeline over a supplied index,
// so the only variable between two calls is which build produced that index.
// EvalQuery's history learning re-reads the worktree and refuses when it moved,
// which a repository under concurrent edit does legitimately; that is reported
// as an error so the caller can skip the root rather than record a false failure.
func harnessEvalReceipt(index *contextindex.Index, task string) ([]byte, error) {
	budget := 8_000 - gokernel.OutputOverheadBytes
	result, err := contextindex.EvalQuery(context.Background(), index, task, harnessContextLimit, &budget)
	if err != nil {
		return nil, err
	}
	sanitized, err := sanitizeHarnessQueryContext(result, task)
	if err != nil {
		return nil, err
	}
	return gokernel.CanonicalJSON(sanitized)
}

// TestBuildEvalReceiptsAreByteIdenticalToBuild is the standing proof that the
// widened query build is a pure latency change. BuildEval drops exactly one
// table's surplus -- the whole-repository import extraction -- and this asserts
// the receipt EvalQuery emits over it is byte-for-byte the receipt it emits over
// the full Build index. A future narrowing that silently empties a table
// EvalQuery ranks from cannot pass this, which is the regression 08dfd77 shipped
// and 7a0678e reverted at 5x the cost.
//
// CORVINT_EVAL_PARITY_REPOS=/path/a:/path/b widens it over real repositories.
func TestBuildEvalReceiptsAreByteIdenticalToBuild(t *testing.T) {
	t.Parallel()
	roots := []string{ledgerRepository(t)}
	if extra := os.Getenv("CORVINT_EVAL_PARITY_REPOS"); extra != "" {
		roots = append(roots, strings.Split(extra, ":")...)
	}
	for _, root := range roots {
		full, err := contextindex.Build(context.Background(), root)
		if err != nil {
			t.Fatalf("Build(%s): %v", root, err)
		}
		eval, err := contextindex.BuildEval(context.Background(), root)
		if err != nil {
			t.Fatalf("BuildEval(%s): %v", root, err)
		}
		compared := 0
		for _, task := range harnessQueryTaskTexts {
			expected, expectedErr := harnessEvalReceipt(full, task)
			actual, actualErr := harnessEvalReceipt(eval, task)
			if expectedErr != nil && actualErr != nil {
				t.Logf("skipping %q in %s: %v / %v", task, root, expectedErr, actualErr)
				continue
			}
			// A race against a live worktree can make both calls refuse together;
			// only one side refusing is not that race; it is a real divergence
			// (an eval-only or full-only defect) and must not be dropped silently.
			if (expectedErr == nil) != (actualErr == nil) {
				t.Fatalf("one-sided error in %s for %q: full=%v eval=%v", root, task, expectedErr, actualErr)
			}
			compared++
			if !bytes.Equal(expected, actual) {
				t.Fatalf("receipt diverged in %s for %q:\n full = %s\n eval = %s", root, task, expected, actual)
			}
		}
		if compared == 0 {
			t.Errorf("no receipt could be compared in %s", root)
		}
		t.Logf("%s: %d/%d receipts byte-identical", root, compared, len(harnessQueryTaskTexts))
	}
}

// TestBuildEvalCarriesEveryTableEvalQueryRanksFrom fails on the specific defect
// rather than on its downstream symptom: EvalQuery ranks out of Features,
// Scenarios, Documents, Markers and Symbols, so the query build must carry each
// of them exactly as the full build compiled it. Asserting equality with Build
// rather than mere non-emptiness makes the assertion independent of which of
// them a given fixture happens to populate.
func TestBuildEvalCarriesEveryTableEvalQueryRanksFrom(t *testing.T) {
	t.Parallel()
	root := ledgerRepository(t)
	full, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	eval, err := contextindex.BuildEval(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(full.Features) == 0 || len(full.Scenarios) == 0 || len(full.Markers) == 0 || len(full.Symbols) == 0 {
		t.Fatal("fixture no longer populates the ranked tables")
	}
	for name, tables := range map[string][2]any{
		"Features":  {full.Features, eval.Features},
		"Scenarios": {full.Scenarios, eval.Scenarios},
		"Documents": {full.Documents, eval.Documents},
		"Markers":   {full.Markers, eval.Markers},
		"Symbols":   {full.Symbols, eval.Symbols},
		"Module":    {full.Module, eval.Module},
		"ProfileID": {full.ProfileID, eval.ProfileID},
	} {
		if !reflect.DeepEqual(tables[0], tables[1]) {
			t.Errorf("BuildEval %s differs from Build: %v vs %v", name, tables[0], tables[1])
		}
	}
}
