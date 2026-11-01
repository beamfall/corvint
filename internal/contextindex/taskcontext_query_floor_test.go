package contextindex

import (
	"context"
	"strings"
	"testing"
)

// TCP-V0-960 (V1-3709): for a fixed set of tasks, the context packet and
// `corvint query` agree on the abstention. An out-of-domain task that query
// withdraws below its relevance floor carries query's abstention block and an
// uncertainty line in the context packet; an in-domain task carries neither.
func TestTaskContextAgreesWithQueryAbstention(t *testing.T) {
	root := evalQueryRepository(t)
	// A lexical neighbour: a feature naming one word of each out-of-domain
	// task, so query ranks a candidate and its floor, not an empty universe,
	// decides.
	writeTestFile(t, root, "testing/features.yaml", "features:\n"+
		"  - id: session-revocation\n    area: auth\n    summary: Revoke an expired device session and enforce future denial.\n    adr: []\n    applies: [server]\n    status: shipped\n"+
		"  - id: billing-export\n    area: billing\n    summary: Export Stripe receipts each quarterly close.\n    adr: []\n    applies: [server]\n    status: shipped\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "lexical neighbours")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range []struct {
		text    string
		abstain bool
	}{
		{"kubernetes helm stripe webhook", true},
		{"quarterly marketing budget spreadsheet", true},
		{"session expiry device revocation enforcement", false},
	} {
		query, err := EvalQuery(context.Background(), index, task.text, 10, nil)
		if err != nil {
			t.Fatal(err)
		}
		queryAbstention, _ := query["abstention"].(map[string]any)
		if queryAbstained := query["state"] == "OUT_OF_SCOPE"; queryAbstained != task.abstain {
			t.Fatalf("%q: query state %v abstention %v, fixture expects abstain=%v", task.text, query["state"], queryAbstention, task.abstain)
		}
		packet, err := TaskContext(context.Background(), index, task.text, "", 10)
		if err != nil {
			t.Fatal(err)
		}
		contextAbstention, present := packet["abstention"].(map[string]any)
		if !task.abstain {
			if present {
				t.Fatalf("%q: query answers but context abstains: %v", task.text, contextAbstention)
			}
			continue
		}
		if !present || contextAbstention["active"] != true || contextAbstention["reason"] != queryAbstention["reason"] {
			t.Fatalf("%q: context abstention %v, query abstention %v", task.text, contextAbstention, queryAbstention)
		}
		disclosed := false
		for _, line := range anySlice(contextCoverage(t, packet)["uncertainty"]) {
			if text, _ := line.(string); strings.Contains(text, "corvint query abstains on this task") {
				disclosed = true
			}
		}
		if !disclosed {
			t.Fatalf("%q: no uncertainty line names the abstention: %v", task.text, contextCoverage(t, packet)["uncertainty"])
		}
	}
}

// TCP-V0-960: a query that ranks nothing at all (`no-relevant-candidates`)
// passes no judgement on the files the packet matched lexically, so the
// packet carries no abstention for it.
func TestTaskContextIgnoresQueryEmptyUniverse(t *testing.T) {
	index, err := Build(context.Background(), evalQueryRepository(t))
	if err != nil {
		t.Fatal(err)
	}
	reason, err := QueryRelevanceAbstention(index, "corvint semantic embeddings federation", 10)
	if err != nil || reason != "no-relevant-candidates" {
		t.Fatalf("QueryRelevanceAbstention = %q, %v; want no-relevant-candidates", reason, err)
	}
	packet, err := TaskContext(context.Background(), index, "corvint semantic embeddings federation", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if abstention, present := packet["abstention"]; present {
		t.Fatalf("an empty query universe set a context abstention: %v", abstention)
	}
}
