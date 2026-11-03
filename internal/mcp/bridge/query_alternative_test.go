package bridge

import (
	"bytes"
	"context"
	"testing"
)

// TestUnsupportedQueryIntentRefusalNamesTheAlternative pins MCPV0-630 (V1-3705): the text block
// of an UNSUPPORTED_QUERY_INTENT corvint.query abstention names corvint.context, its profile and
// the corvint context command, while the canonical object (structuredContent) stays unchanged.
func TestUnsupportedQueryIntentRefusalNamesTheAlternative(t *testing.T) {
	if runtimeUnsupported() {
		t.Skip("native query is qualified only on Darwin and Linux")
	}
	registry, err := New(makeRepository(t))
	if err != nil {
		t.Fatal(err)
	}
	result, callErr := registry.Call(context.Background(), ToolQuery, []byte(`{"task":"fix MCP server root binding"}`))
	if callErr != nil {
		t.Fatal(callErr)
	}
	text, textErr := result.TextJSON()
	canonical, canonicalErr := result.CanonicalJSON()
	if textErr != nil || canonicalErr != nil {
		t.Fatal(textErr, canonicalErr)
	}
	const alternative = `"alternative":{"command":"corvint context","profile":"task-review","tool":"corvint.context"}`
	if !bytes.Contains(text, []byte(alternative)) {
		t.Fatalf("refusal text %s lacks %s", text, alternative)
	}
	if bytes.Contains(canonical, []byte(`"alternative"`)) {
		t.Fatalf("structuredContent gained the text-only member: %s", canonical)
	}
	other, callErr := registry.Call(context.Background(), ToolQuery, []byte(`{"task":"   "}`))
	if callErr == nil {
		if text, _ := other.TextJSON(); bytes.Contains(text, []byte(`"alternative"`)) {
			t.Fatalf("a non-intent result named the alternative: %s", text)
		}
	}
}
