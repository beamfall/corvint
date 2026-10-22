package corpusbridge

import (
	"context"
	"testing"
)

// TestDCPV1328And331SearchArguments: docs_search admits include:["claims"] and a
// subject kind filter; include stays closed to ["claims"] on docs_search and kind
// stays refused outside docs_gaps and docs_search.
func TestDCPV1328And331SearchArguments(t *testing.T) {
	root, _ := multiRepositoryFixture(t, [3]int{1, 1, 1})
	r, e := New(root, "corpus.json")
	if e != nil {
		t.Fatal(e)
	}
	for _, args := range []string{`{"query":"lib","include":["claims"]}`, `{"query":"lib","kind":"module"}`, `{"query":"lib","kind":["module","symbol"],"include":["claims"]}`} {
		envelopeCall(t, r, "corvint.docs_search", args)
	}
	for _, c := range []struct{ tool, args string }{
		{"corvint.docs_search", `{"query":"lib","include":["sections"]}`},
		{"corvint.docs_search", `{"query":"lib","include":[]}`},
		{"corvint.docs_search", `{"query":"lib","kind":[]}`},
		{"corvint.docs_get", `{"id":"adapter:lib1-0","include":["claims"]}`},
		{"corvint.docs_flow", `{"id":"adapter:lib1-0","kind":"x"}`},
	} {
		if _, _, _, protocol, _ := r.CallTimed(context.Background(), c.tool, []byte(c.args)); protocol == nil || protocol.Code != "invalid-arguments" {
			t.Fatalf("%s %s accepted: %v", c.tool, c.args, protocol)
		}
	}
}
