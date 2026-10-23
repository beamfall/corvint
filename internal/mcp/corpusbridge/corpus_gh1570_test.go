package corpusbridge

import (
	"context"
	json "encoding/json/v2"
	"os/exec"
	"strings"
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
		// Codex P3: a valid flow record kind other than claims is invalid-arguments, not corpus-refused.
		{"corvint.docs_search", `{"query":"lib","include":["relations"]}`},
		{"corvint.docs_search", `{"query":"lib","include":["claims","paragraphs"]}`},
		{"corvint.docs_search", `{"query":"lib","kind":[]}`},
		{"corvint.docs_get", `{"id":"adapter:lib1-0","include":["claims"]}`},
		{"corvint.docs_flow", `{"id":"adapter:lib1-0","kind":"x"}`},
	} {
		if _, _, _, protocol, _ := r.CallTimed(context.Background(), c.tool, []byte(c.args)); protocol == nil || protocol.Code != "invalid-arguments" {
			t.Fatalf("%s %s accepted: %v", c.tool, c.args, protocol)
		}
	}
}

// TestDCPV1332CallerRevisionArgument: every corpus tool admits caller_revision;
// the live bridge relates it to the source revision in the trust envelope, an
// absent argument emits nothing, and a malformed one is invalid-arguments.
func TestDCPV1332CallerRevisionArgument(t *testing.T) {
	root, _ := multiRepositoryFixture(t, [3]int{1, 1, 1})
	r, e := New(root, "corpus.json")
	if e != nil {
		t.Fatal(e)
	}
	call := func(args string) (string, map[string]string) {
		t.Helper()
		_, text, failure, protocol, _ := r.CallTimed(context.Background(), "corvint.docs_search", []byte(args))
		var receipt struct {
			Trust struct {
				Source string            `json:"source_revision"`
				Caller map[string]string `json:"caller_revision"`
			} `json:"trust_envelope"`
		}
		if failure != nil || protocol != nil || json.Unmarshal([]byte(text), &receipt) != nil {
			t.Fatalf("%s: %v %v %s", args, failure, protocol, text)
		}
		return receipt.Trust.Source, receipt.Trust.Caller
	}
	source, absent := call(`{"query":"lib"}`)
	if absent != nil {
		t.Fatalf("caller_revision emitted without the argument: %v", absent)
	}
	gitOut := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", root}, args...)...).Output()
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(out))
	}
	head := gitOut("rev-parse", "HEAD")
	for caller, want := range map[string]string{source: "equal", head: "descendant", strings.Repeat("f", 40): "unknown"} {
		if caller == source && want != "equal" {
			continue
		}
		_, got := call(`{"query":"lib","caller_revision":"` + caller + `"}`)
		if got["relation"] != want || got["revision"] != caller || got["source_revision"] != source || got["reason"] == "" {
			t.Fatalf("caller %s: %v, want %s", caller, got, want)
		}
	}
	for _, bad := range []string{`""`, `"main"`, `"` + strings.ToUpper(head) + `"`, `"` + head[:39] + `"`, `1`} {
		args := `{"query":"lib","caller_revision":` + bad + `}`
		if _, _, _, protocol, _ := r.CallTimed(context.Background(), "corvint.docs_search", []byte(args)); protocol == nil || protocol.Code != "invalid-arguments" {
			t.Fatalf("%s accepted: %v", args, protocol)
		}
	}
	for _, tool := range r.Tools() {
		if _, ok := tool.InputSchema["properties"].(map[string]any)["caller_revision"]; !ok {
			t.Fatalf("%s schema lacks caller_revision", tool.Name)
		}
	}
}
