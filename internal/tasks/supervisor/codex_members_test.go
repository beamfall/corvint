package supervisor

import (
	"strings"
	"testing"
)

const (
	codexStarted = `{"type":"thread.started","thread_id":"s1"}`
	codexMessage = `{"type":"item.completed","item":{"type":"agent_message","text":"{\"kind\":\"BUILT\",\"summary\":\"s\",\"nextAction\":\"n\"}"}}`
	codexDone    = `{"type":"turn.completed","usage":{"input_tokens":4,"output_tokens":1}}`
)

func codexStream(lines ...string) []byte { return []byte(strings.Join(lines, "\n") + "\n") }

// TestV10799_CodexStreamRefusesRepeatedAndFoldedMembers: the Codex result
// and session readers refuse a line that repeats a member or aliases a read
// member by case, as the Claude Code and OpenCode readers do (V1-0799).
func TestV10799_CodexStreamRefusesRepeatedAndFoldedMembers(t *testing.T) {
	if s, r, e := DecodeEvents(codexStream(codexStarted, codexMessage, codexDone)); e != nil || s != "s1" || r.Kind != "BUILT" {
		t.Fatalf("control: %q %+v %v", s, r, e)
	}
	if s := ObservedSession(codexStream(codexStarted, codexDone)); s != "s1" {
		t.Fatalf("control session %q", s)
	}
	for name, stream := range map[string][]byte{
		"repeated type hides error": codexStream(codexStarted, `{"type":"error","type":"turn.started"}`, codexMessage, codexDone),
		"folded type":               codexStream(codexStarted, `{"type":"turn.started","Type":"error"}`, codexMessage, codexDone),
		"repeated thread id":        codexStream(`{"type":"thread.started","thread_id":"s2","thread_id":"s1"}`, codexMessage, codexDone),
		"folded thread id":          codexStream(`{"type":"thread.started","thread_id":"s1","Thread_Id":"s2"}`, codexMessage, codexDone),
		"repeated item":             codexStream(codexStarted, `{"type":"item.completed","item":{"type":"reasoning","text":"x"},"item":{"type":"agent_message","text":"{\"kind\":\"BUILT\",\"summary\":\"s\",\"nextAction\":\"n\"}"}}`, codexDone),
		"folded item text":          codexStream(codexStarted, `{"type":"item.completed","item":{"type":"agent_message","text":"{\"kind\":\"BUILT\",\"summary\":\"s\",\"nextAction\":\"n\"}","TEXT":"x"}}`, codexDone),
		"repeated handoff member":   codexStream(codexStarted, `{"type":"item.completed","item":{"type":"agent_message","text":"{\"kind\":\"WAIT\",\"kind\":\"BUILT\",\"summary\":\"s\",\"nextAction\":\"n\"}"}}`, codexDone),
		"folded handoff member":     codexStream(codexStarted, `{"type":"item.completed","item":{"type":"agent_message","text":"{\"kind\":\"BUILT\",\"Accepted\":true,\"summary\":\"s\",\"nextAction\":\"n\"}"}}`, codexDone),
	} {
		if _, _, e := DecodeEvents(stream); e == nil {
			t.Errorf("%s: DecodeEvents admitted", name)
		}
	}
	for name, stream := range map[string][]byte{
		"repeated thread id": codexStream(`{"type":"thread.started","thread_id":"s2","thread_id":"s1"}`, codexDone),
		"folded type":        codexStream(codexStarted, `{"type":"turn.completed","TYPE":"x"}`),
	} {
		if s := ObservedSession(stream); s != "" {
			t.Errorf("%s: session %q observed", name, s)
		}
	}
}

// TestV10799_HandoffRefusalLeavesSessionAndUsageUnobserved: the three Codex
// readers share one member verdict, so a JSON handoff that repeats or
// case-aliases a member leaves session and usage unobserved as well as
// refusing the result (V1-0799, V1-0950).
func TestV10799_HandoffRefusalLeavesSessionAndUsageUnobserved(t *testing.T) {
	for name, msg := range map[string]string{
		"repeated kind": `{"type":"item.completed","item":{"type":"agent_message","text":"{\"kind\":\"WAIT\",\"kind\":\"BUILT\",\"summary\":\"s\",\"nextAction\":\"n\"}"}}`,
		"folded kind":   `{"type":"item.completed","item":{"type":"agent_message","text":"{\"kind\":\"BUILT\",\"KIND\":\"WAIT\",\"summary\":\"s\",\"nextAction\":\"n\"}"}}`,
	} {
		stream := codexStream(codexStarted, msg, codexDone)
		if s, _, e := DecodeEvents(stream); e == nil || s != "" {
			t.Errorf("%s: DecodeEvents session %q err %v", name, s, e)
		}
		if s := ObservedSession(stream); s != "" {
			t.Errorf("%s: session %q observed", name, s)
		}
		if _, _, ok := ObservedUsage(stream); ok {
			t.Errorf("%s: usage observed", name)
		}
	}
	// A prose final text is not a handoff: the result is refused but the
	// session and usage stay observable.
	prose := codexStream(codexStarted, `{"type":"item.completed","item":{"type":"agent_message","text":"I could not finish"}}`, codexDone)
	if s := ObservedSession(prose); s != "s1" {
		t.Errorf("prose: session %q", s)
	}
	if _, _, ok := ObservedUsage(prose); !ok {
		t.Error("prose: usage unobserved")
	}
}
