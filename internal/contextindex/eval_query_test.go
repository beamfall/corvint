package contextindex

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func evalQueryRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	files := map[string]string{
		".gitignore": ".context-corvint/\n",
		"go.mod":     "module example.test/eval\n\ngo 1.27.0\n",
		"testing/features.yaml": "features:\n" +
			"  - id: session-revocation\n    area: auth\n    summary: Revoke an expired device session and enforce future denial.\n    adr: []\n    applies: [server]\n    status: shipped\n",
		"testing/scenarios.yaml": "scenarios:\n" +
			"  - id: revoked-device\n    area: auth\n    summary: Expired device session is revoked.\n    features: [session-revocation]\n    applies: [server]\n    status: shipped\n",
		"internal/auth/session.go":      "package auth\n\nfunc EnforceSessionRevocation() bool { return true }\n",
		"internal/auth/session_test.go": "package auth\n\n// feature:session-revocation\nfunc TestEnforceSessionRevocation() {}\n",
		"web/session.ts":                "export function ignoredSessionRevocation() { return true }\n",
	}
	for path, content := range files {
		writeTestFile(t, root, path, content)
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "enforce expired device session revocation")
	return root
}

// GOC-V0-003: authored routing expectations remain independent of the candidate.
func TestEvalQueryGoRepositoryRouting(t *testing.T) {
	root := evalQueryRepository(t)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []struct{ text, state, required string }{
		{"session expiry device revocation enforcement", "READY", "internal/auth/session.go:EnforceSessionRevocation"},
		{"corvint semantic embeddings federation", "OUT_OF_SCOPE", ""},
	} {
		candidate, err := EvalQuery(context.Background(), index, query.text, 10, nil)
		if err != nil {
			t.Fatal(err)
		}
		if candidate["mode"] != "query" || candidate["state"] != query.state || candidate["revision"] != index.Revision {
			t.Fatalf("query %q envelope = %#v", query.text, candidate)
		}
		results := mapsFromAny(candidate["results"])
		if query.required == "" {
			if len(results) != 0 {
				t.Fatalf("unrelated query admitted results: %#v", results)
			}
			continue
		}
		found := false
		for _, result := range results {
			if result["id"] == query.required {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing required implementation: %#v", results)
		}
	}
}

// GPK-V0-044: a matched local trace's evidence blob always pins to the
// index's current content for that path, which may postdate what the trace
// actually observed. The reason MUST disclose the trace's own recorded
// revision so a reader can tell the two apart, rather than let the current
// blob stand in for the trace's evidence uncontested.
func TestEvalQueryLearnedPathDisclosesTraceRevisionOnDrift(t *testing.T) {
	root := evalQueryRepository(t)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	queryText := "session expiry device revocation enforcement"
	traceID := strings.Repeat("b", 64)
	staleRevision := strings.Repeat("a", 40)
	if staleRevision == index.Revision {
		t.Fatal("fixture's stale revision accidentally matches the current index revision")
	}
	snapshot, err := NewQueryTraceSnapshot("ready", []QueryTrace{{
		TraceID: traceID, Task: queryText, Outcome: "passed", Revision: staleRevision,
		ChangedPaths: []string{"internal/auth/session.go"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	packet, err := EvalQuery(context.Background(), index, queryText, 10, nil, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if packet["state"] != "READY" {
		t.Fatalf("packet state=%v, want READY: %#v", packet["state"], packet)
	}
	overlap := intersection(terms(queryText), terms(queryText))
	wantReason := fmt.Sprintf("successful local trace %s changed this path; matched %d task terms; trace recorded at commit %s",
		truncateRunes(traceID, 12), len(overlap), truncateRunes(staleRevision, 12))
	found := false
	for _, result := range mapsFromAny(packet["results"]) {
		if result["kind"] != "learned-path" || result["id"] != "internal/auth/session.go" {
			continue
		}
		for _, rawEvidence := range result["evidence"].([]any) {
			evidence := rawEvidence.(map[string]any)
			if evidence["authority"] == "local-task-trace" && evidence["reason"] == wantReason {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("missing revision-disclosing local-task-trace evidence, want reason %q in %#v", wantReason, packet["results"])
	}
}

func TestEvalQueryAcceptsStatusCleanIdentCheckout(t *testing.T) {
	t.Run("IDX-SNAP-V0-008", func(t *testing.T) {
		root := identAttributeRepository(t)
		index, err := BuildEval(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		// Preserve the legacy union shape independently of the B1 repair: the
		// history bracket must compare its status digest, not this derived set.
		index.DirtyPaths = append(index.DirtyPaths, "pkg/demux.go")
		if _, err := EvalQuery(context.Background(), index, "Demux", 10, nil); err != nil {
			t.Fatalf("status-clean query: %v", err)
		}
	})
}

func TestEvalFeatureAllowsGoRankingInMixedLanguageRepository(t *testing.T) {
	root := evalQueryRepository(t)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := FeatureBudget(index, "session-revocation", 10, nil); err == nil || !strings.Contains(err.Error(), "supports only Go") {
		t.Fatalf("public feature error=%v", err)
	}
	receipt, err := EvalFeature(index, "session-revocation", 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	results := mapsFromAny(receipt["results"])
	if len(results) < 2 || results[0]["id"] != "session-revocation" || results[1]["id"] != "internal/auth/session.go:EnforceSessionRevocation" {
		t.Fatalf("results=%v", results)
	}
}

func TestEvalImpactAppliesBudgetWithoutChangingImpact(t *testing.T) {
	root := evalQueryRepository(t)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	budget := 1800
	receipt, err := EvalImpact(index, []string{"internal/auth/session.go"}, 10, &budget)
	if err != nil {
		t.Fatal(err)
	}
	request := receipt["request"].(map[string]any)
	coverage := receipt["coverage"].(map[string]any)
	if request["budget_bytes"] != budget || coverage["budget_bytes"] != budget || coverage["within_budget"] != true {
		t.Fatalf("request=%v coverage=%v", request, coverage)
	}
}
