package supervisor

import (
	"encoding/json"
	"strings"
	"testing"
)

// claudeObject renders one Claude Code `--output-format json` result object
// whose fields can be overridden or removed (nil) per case.
func claudeObject(t *testing.T, overrides map[string]any) string {
	t.Helper()
	o := map[string]any{"type": "result", "subtype": "success", "is_error": false, "session_id": "sess-1", "result": `{"kind":"BUILT","summary":"done","nextAction":"review"}`, "usage": map[string]any{"input_tokens": 10, "output_tokens": 3, "cache_creation_input_tokens": 5, "cache_read_input_tokens": 100}, "num_turns": 2, "permission_denials": []any{}}
	for k, v := range overrides {
		if v == nil {
			delete(o, k)
		} else {
			o[k] = v
		}
	}
	raw, e := json.Marshal(o)
	if e != nil {
		t.Fatal(e)
	}
	return string(raw)
}

func TestCALV0075_ClaudeResultVocabulary(t *testing.T) {
	host, ok := HostVocabulary(HostClaudeCode)
	if !ok {
		t.Fatal("claude-code vocabulary absent")
	}
	session, result, e := host.Decode([]byte(claudeObject(t, nil) + "\n"))
	if e != nil || session != "sess-1" || result.Kind != "BUILT" || result.Summary != "done" || result.NextAction != "review" {
		t.Fatalf("valid result: %q %+v %v", session, result, e)
	}
	if got := host.Session([]byte(claudeObject(t, nil))); got != "sess-1" {
		t.Fatalf("session %q", got)
	}
	wait := claudeObject(t, map[string]any{"result": `{"kind":"WAIT","question":"which?","summary":"blocked","nextAction":"answer"}`})
	if _, result, e = host.Decode([]byte(wait)); e != nil || result.Kind != "WAIT" || result.Question != "which?" {
		t.Fatalf("wait result: %+v %v", result, e)
	}
	invalid := map[string]string{
		"prose result":       claudeObject(t, map[string]any{"result": "Here is the JSON: {\"kind\":\"BUILT\",\"summary\":\"done\",\"nextAction\":\"review\"}"}),
		"fenced result":      claudeObject(t, map[string]any{"result": "```json\n{\"kind\":\"BUILT\",\"summary\":\"done\",\"nextAction\":\"review\"}\n```"}),
		"unknown handoff":    claudeObject(t, map[string]any{"result": `{"kind":"BUILT","summary":"done","nextAction":"review","extra":1}`}),
		"bad kind":           claudeObject(t, map[string]any{"result": `{"kind":"DONE","summary":"done","nextAction":"review"}`}),
		"empty summary":      claudeObject(t, map[string]any{"result": `{"kind":"BUILT","summary":"","nextAction":"review"}`}),
		"empty result":       claudeObject(t, map[string]any{"result": ""}),
		"absent result":      claudeObject(t, map[string]any{"result": nil}),
		"is_error":           claudeObject(t, map[string]any{"is_error": true}),
		"absent is_error":    claudeObject(t, map[string]any{"is_error": nil}),
		"turn limit":         claudeObject(t, map[string]any{"subtype": "error_max_turns"}),
		"wrong type":         claudeObject(t, map[string]any{"type": "assistant"}),
		"absent session":     claudeObject(t, map[string]any{"session_id": nil}),
		"oversized session":  claudeObject(t, map[string]any{"session_id": strings.Repeat("s", 129)}),
		"two objects":        claudeObject(t, nil) + "\n" + claudeObject(t, nil),
		"trailing text":      claudeObject(t, nil) + " done",
		"codex events":       "{\"type\":\"thread.started\",\"thread_id\":\"t\"}\n{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}\n",
		"empty output":       "",
		"stream-json prefix": "{\"type\":\"system\",\"subtype\":\"init\"}\n" + claudeObject(t, nil),
	}
	for name, raw := range invalid {
		if _, _, e := host.Decode([]byte(raw)); e == nil {
			t.Errorf("%s: decoded as a valid handoff", name)
		}
	}
	if got := host.Session([]byte(claudeObject(t, map[string]any{"session_id": strings.Repeat("s", 129)}))); got != "" {
		t.Fatalf("oversized session observed: %q", got)
	}
	codex, ok := HostVocabulary("")
	if !ok {
		t.Fatal("codex vocabulary absent")
	}
	if _, _, e := codex.Decode([]byte(claudeObject(t, nil))); e == nil {
		t.Fatal("codex vocabulary decoded a Claude Code result")
	}
	for _, unknown := range []string{"gemini-cli", "Claude-Code", " claude-code"} {
		if _, ok := HostVocabulary(unknown); ok {
			t.Fatalf("unknown host %q has a vocabulary", unknown)
		}
	}
}

func TestCALV0075_ClaudeUsageObservedOrUnknown(t *testing.T) {
	host, _ := HostVocabulary(HostClaudeCode)
	if i, o, known := host.Usage([]byte(claudeObject(t, nil))); !known || i != 115 || o != 3 {
		t.Fatalf("usage with cache counters: %d %d %v", i, o, known)
	}
	plain := claudeObject(t, map[string]any{"usage": map[string]any{"input_tokens": 42, "output_tokens": 7}})
	if i, o, known := host.Usage([]byte(plain)); !known || i != 42 || o != 7 {
		t.Fatalf("plain usage: %d %d %v", i, o, known)
	}
	// Usage is observed even on a failed turn, so a refused result still
	// counts the tokens it consumed.
	failed := claudeObject(t, map[string]any{"is_error": true, "subtype": "error_during_execution"})
	if _, _, known := host.Usage([]byte(failed)); !known {
		t.Fatal("failed turn usage unobserved")
	}
	unknown := map[string]string{
		"absent usage":    claudeObject(t, map[string]any{"usage": nil}),
		"absent output":   claudeObject(t, map[string]any{"usage": map[string]any{"input_tokens": 1}}),
		"null input":      claudeObject(t, map[string]any{"usage": map[string]any{"input_tokens": nil, "output_tokens": 1}}),
		"negative":        claudeObject(t, map[string]any{"usage": map[string]any{"input_tokens": -1, "output_tokens": 1}}),
		"fractional":      claudeObject(t, map[string]any{"usage": map[string]any{"input_tokens": 1.5, "output_tokens": 1}}),
		"string counter":  claudeObject(t, map[string]any{"usage": map[string]any{"input_tokens": "1", "output_tokens": 1}}),
		"bad cache":       claudeObject(t, map[string]any{"usage": map[string]any{"input_tokens": 1, "output_tokens": 1, "cache_read_input_tokens": "9"}}),
		"cache overflow":  `{"type":"result","subtype":"success","is_error":false,"session_id":"s","result":"x","usage":{"input_tokens":18446744073709551615,"output_tokens":1,"cache_read_input_tokens":1}}`,
		"not one object":  claudeObject(t, nil) + claudeObject(t, nil),
		"codex usage":     "{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":42,\"output_tokens\":7}}\n",
		"empty output":    "",
		"wrong type":      claudeObject(t, map[string]any{"type": "assistant"}),
		"null cache read": claudeObject(t, map[string]any{"usage": map[string]any{"input_tokens": 1, "output_tokens": 1, "cache_read_input_tokens": nil}}),
	}
	for name, raw := range unknown {
		if i, o, known := host.Usage([]byte(raw)); known || i != 0 || o != 0 {
			t.Errorf("%s: invented usage %d %d %v", name, i, o, known)
		}
	}
}

// TestCALV0075_ClaudeDuplicateMembers proves a repeated member anywhere in the
// result object, its usage or the handoff string, or a member that aliases a
// read name by case folding (encoding/json matches struct fields
// case-insensitively), is refused rather than resolved last-wins, and that
// the result, session and usage readers all leave it unobserved.
func TestCALV0075_ClaudeDuplicateMembers(t *testing.T) {
	host, _ := HostVocabulary(HostClaudeCode)
	handoff := `"{\"kind\":\"BUILT\",\"summary\":\"done\",\"nextAction\":\"review\"}"`
	usage := `{"input_tokens":4,"output_tokens":2}`
	head := `{"type":"result","subtype":"success","is_error":false,"session_id":"s",`
	cases := map[string]string{
		"is_error true then false":  `{"type":"result","subtype":"success","is_error":true,"is_error":false,"session_id":"s","result":` + handoff + `,"usage":` + usage + `}`,
		"escaped repeated is_error": `{"type":"result","subtype":"success","is_error":true,"\u0069s_error":false,"session_id":"s","result":` + handoff + `,"usage":` + usage + `}`,
		"repeated usage objects":    head + `"result":` + handoff + `,"usage":{"input_tokens":4},"usage":{"output_tokens":2}}`,
		"repeated usage counter":    head + `"result":` + handoff + `,"usage":{"input_tokens":4,"output_tokens":2,"input_tokens":9}}`,
		"repeated session":          `{"type":"result","subtype":"success","is_error":false,"session_id":"s","session_id":"t","result":` + handoff + `,"usage":` + usage + `}`,
		"repeated handoff member":   head + `"result":"{\"kind\":\"WAIT\",\"kind\":\"BUILT\",\"summary\":\"done\",\"nextAction\":\"review\"}","usage":` + usage + `}`,
		"nested repeated member":    head + `"result":` + handoff + `,"usage":` + usage + `,"extra":[{"a":1,"a":2}]}`,
		"is_error case alias":       `{"type":"result","subtype":"success","is_error":true,"IS_ERROR":false,"session_id":"s","result":` + handoff + `,"usage":` + usage + `}`,
		"escaped is_error alias":    `{"type":"result","subtype":"success","is_error":true,"\u0049S_error":false,"session_id":"s","result":` + handoff + `,"usage":` + usage + `}`,
		"usage case alias":          head + `"result":` + handoff + `,"usage":{"input_tokens":4},"Usage":{"output_tokens":2}}`,
		"session case alias":        head + `"Session_ID":"t","result":` + handoff + `,"usage":` + usage + `}`,
		"handoff kind alias":        head + `"result":"{\"kind\":\"WAIT\",\"Kind\":\"BUILT\",\"summary\":\"done\",\"nextAction\":\"review\"}","usage":` + usage + `}`,
		"escaped handoff alias":     head + `"result":"{\"kind\":\"WAIT\",\"\\u004bind\":\"BUILT\",\"summary\":\"done\",\"nextAction\":\"review\"}","usage":` + usage + `}`,
		"handoff kelvin-sign alias": head + `"result":"{\"kind\":\"WAIT\",\"\\u212aind\":\"BUILT\",\"summary\":\"done\",\"nextAction\":\"review\"}","usage":` + usage + `}`,
		"escaped repeated handoff":  head + `"result":"{\"kind\":\"WAIT\",\"\\u006bind\":\"BUILT\",\"summary\":\"done\",\"nextAction\":\"review\"}","usage":` + usage + `}`,
	}
	for name, raw := range cases {
		if !json.Valid([]byte(raw)) {
			t.Fatalf("%s: fixture is not JSON", name)
		}
		if session, _, e := host.Decode([]byte(raw)); e == nil || session != "" {
			t.Fatalf("%s: decoded (session %q, %v)", name, session, e)
		}
		if _, _, ok := host.Usage([]byte(raw)); ok {
			t.Fatalf("%s: usage observed", name)
		}
		if got := host.Session([]byte(raw)); got != "" {
			t.Fatalf("%s: session observed as %q", name, got)
		}
	}
	// A prose result (an error report, not a handoff) keeps its session and
	// usage observable.
	prose := `{"type":"result","subtype":"error_max_turns","is_error":true,"session_id":"s","result":"ran out of turns","usage":` + usage + `}`
	if _, _, ok := host.Usage([]byte(prose)); !ok || host.Session([]byte(prose)) != "s" {
		t.Fatal("prose error result left unobserved")
	}
	distinct := `{"type":"result","subtype":"success","is_error":false,"session_id":"s","result":` + handoff + `,"usage":` + usage + `,"extra":[{"a":1},{"a":2}],"nested":{"a":{"a":1}}}`
	if _, _, e := host.Decode([]byte(distinct)); e != nil {
		t.Fatalf("same name in distinct objects refused: %v", e)
	}
}
