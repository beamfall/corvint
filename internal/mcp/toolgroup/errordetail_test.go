package toolgroup

import (
	"context"
	"strings"
	"testing"
)

func errorObject(t *testing.T, result map[string]any) map[string]any {
	t.Helper()
	if result["isError"] != true {
		t.Fatalf("not a tool error: %#v", result)
	}
	value, _ := result["structuredContent"].(map[string]any)
	return value
}

// DMF-V1-010: a cancelled call is the cancelled tool error with a message, retryable true and a
// present next, never the -32603 internal error; a failed child keeps its last stderr line in
// message and its next step; every tool error keeps the old error member and adds the new ones.
func TestDMFV1010CancelledAndChildFailure(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	result, failure := Answer(cancelled, DocsErrorProfile, "corvint.docs_draft", nil, "", &Coded{Code: CancelledCode, Message: CancelledCode}, nil)
	if failure != nil {
		t.Fatalf("cancelled returned a JSON-RPC error: %#v", failure)
	}
	value := errorObject(t, result)
	if value["code"] != CancelledCode || value["message"] == CancelledCode || value["message"] == "" || value["retryable"] != true || value["next"] != "" {
		t.Fatalf("cancelled %#v", value)
	}
	line := "no corpus artifact at .corvint/corpus"
	child := &Coded{Code: ChildFailedCode, Message: "corvint-docs docs-tool failed for corvint.docs_draft: exit status 2: " + line, Next: "build the corpus artifact: corvint-docs docs corpus build --manifest FILE"}
	result, failure = Answer(context.Background(), DocsErrorProfile, "corvint.docs_draft", nil, "", child, nil)
	value = errorObject(t, result)
	if failure != nil || value["code"] != ChildFailedCode || !strings.Contains(value["message"].(string), line) || value["retryable"] != true ||
		!strings.Contains(value["next"].(string), "corvint-docs docs corpus build") || value["error"] != child.Message {
		t.Fatalf("child failure %#v %#v", value, failure)
	}
	result, _ = Failure(DocsErrorProfile, "corvint.docs_draft", "repository-unavailable", "repository-unavailable")
	value = errorObject(t, result)
	if value["message"] == "repository-unavailable" || value["retryable"] != false || value["next"] == "" || value["error"] != "repository-unavailable" {
		t.Fatalf("a message equal to its code %#v", value)
	}
	if text, retryable, next := Detail("never-seen", ""); text == "" || text == "never-seen" || retryable || next != "" {
		t.Fatalf("unknown code %q %v %q", text, retryable, next)
	}
}
