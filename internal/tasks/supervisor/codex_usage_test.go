package supervisor

import "testing"

// TestV10950_CodexUsageNullRepeatedOrFoldedIsUnobserved: a null, repeated
// or case-folded usage counter, or a repeated or case-folded usage or type
// member, leaves Codex usage unobserved instead of reading 0 or the last
// duplicate (V1-0950).
func TestV10950_CodexUsageNullRepeatedOrFoldedIsUnobserved(t *testing.T) {
	if i, o, ok := ObservedUsage(codexStream(codexDone)); !ok || i != 4 || o != 1 {
		t.Fatalf("control: %d %d %v", i, o, ok)
	}
	for name, line := range map[string]string{
		"null input":          `{"type":"turn.completed","usage":{"input_tokens":null,"output_tokens":1}}`,
		"null output":         `{"type":"turn.completed","usage":{"input_tokens":4,"output_tokens":null}}`,
		"repeated counter":    `{"type":"turn.completed","usage":{"input_tokens":4,"output_tokens":9,"output_tokens":1}}`,
		"folded counter":      `{"type":"turn.completed","usage":{"input_tokens":4,"output_tokens":1,"Output_Tokens":0}}`,
		"repeated usage":      `{"type":"turn.completed","usage":{"input_tokens":4},"usage":{"output_tokens":1}}`,
		"folded usage":        `{"type":"turn.completed","Usage":{"input_tokens":4,"output_tokens":1}}`,
		"repeated type":       `{"type":"turn.completed","type":"turn.completed","usage":{"input_tokens":4,"output_tokens":1}}`,
		"folded type":         `{"type":"turn.completed","TYPE":"x","usage":{"input_tokens":4,"output_tokens":1}}`,
		"other line repeated": `{"type":"turn.started","type":"turn.started"}`,
	} {
		raw := codexStream(codexDone, line)
		if name == "other line repeated" {
			raw = codexStream(line, codexDone)
		}
		if _, _, ok := ObservedUsage(raw); ok {
			t.Errorf("%s: usage observed", name)
		}
	}
}
