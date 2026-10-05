package supervisor

import (
	"encoding/json"
	"strings"
	"testing"
)

// ocLine is one OpenCode `run --format json` event line for session s.
func ocLine(t *testing.T, typ, s string, fields map[string]any) string {
	t.Helper()
	ev := map[string]any{"type": typ, "timestamp": 1, "sessionID": s}
	for k, v := range fields {
		ev[k] = v
	}
	raw, e := json.Marshal(ev)
	if e != nil {
		t.Fatal(e)
	}
	return string(raw) + "\n"
}

func ocText(t *testing.T, s, text string) string {
	return ocLine(t, "text", s, map[string]any{"part": map[string]any{"type": "text", "text": text}})
}

func ocFinish(t *testing.T, s, reason string, tokens any) string {
	part := map[string]any{"type": "step-finish", "reason": reason, "cost": 0}
	if tokens != nil {
		part["tokens"] = tokens
	}
	return ocLine(t, "step_finish", s, map[string]any{"part": part})
}

func ocTokens(in, out, reasoning, read, write any) map[string]any {
	return map[string]any{"input": in, "output": out, "reasoning": reasoning, "cache": map[string]any{"read": read, "write": write}}
}

const ocHandoff = `{"kind":"BUILT","summary":"done","nextAction":"review"}`

func TestCALV0077_OpenCodeResultVocabulary(t *testing.T) {
	s := "ses_1"
	tok := ocTokens(1, 1, 0, 0, 0)
	ok := ocLine(t, "step_start", s, map[string]any{"part": map[string]any{"type": "step-start"}}) +
		ocLine(t, "tool_use", s, map[string]any{"part": map[string]any{"type": "tool", "tool": "read"}}) +
		ocFinish(t, s, "tool-calls", tok) +
		ocLine(t, "reasoning", s, map[string]any{"part": map[string]any{"type": "reasoning", "text": "thinking"}}) +
		ocText(t, s, "interim prose") +
		ocText(t, s, ocHandoff) +
		ocFinish(t, s, "stop", tok)
	session, result, e := DecodeOpenCodeEvents([]byte(ok))
	if e != nil || session != s || result.Kind != "BUILT" || result.Summary != "done" || result.NextAction != "review" {
		t.Fatalf("qualified stream refused: %q %+v %v", session, result, e)
	}
	if got := ObservedOpenCodeSession([]byte(ok)); got != s {
		t.Fatalf("session %q", got)
	}
	refused := map[string]string{
		"empty":            "",
		"not json":         "plain text\n",
		"unknown event":    ocLine(t, "patch", s, nil) + ocText(t, s, ocHandoff) + ocFinish(t, s, "stop", tok),
		"host error":       ocText(t, s, ocHandoff) + ocFinish(t, s, "stop", tok) + ocLine(t, "error", s, map[string]any{"error": map[string]any{"type": "unknown", "message": "x"}}),
		"sessionless err":  ocText(t, s, ocHandoff) + ocFinish(t, s, "stop", tok) + ocLine(t, "error", "", map[string]any{"error": map[string]any{"type": "unknown", "message": "x"}}),
		"session changed":  ocText(t, s, ocHandoff) + ocFinish(t, "ses_2", "stop", tok),
		"missing session":  ocText(t, "", ocHandoff) + ocFinish(t, "", "stop", tok),
		"long session":     ocText(t, strings.Repeat("s", 129), ocHandoff) + ocFinish(t, strings.Repeat("s", 129), "stop", tok),
		"no finish":        ocText(t, s, ocHandoff),
		"text after step":  ocFinish(t, s, "stop", tok) + ocText(t, s, ocHandoff),
		"length finish":    ocText(t, s, ocHandoff) + ocFinish(t, s, "length", tok),
		"tool-call finish": ocText(t, s, ocHandoff) + ocFinish(t, s, "tool-calls", tok),
		"no text":          ocFinish(t, s, "stop", tok),
		"fenced handoff":   ocText(t, s, "```json\n"+ocHandoff+"\n```") + ocFinish(t, s, "stop", tok),
		"unknown field":    ocText(t, s, `{"kind":"BUILT","summary":"done","nextAction":"review","extra":1}`) + ocFinish(t, s, "stop", tok),
		"invalid kind":     ocText(t, s, `{"kind":"DONE","summary":"done","nextAction":"review"}`) + ocFinish(t, s, "stop", tok),
		"empty summary":    ocText(t, s, `{"kind":"BUILT","summary":"","nextAction":"review"}`) + ocFinish(t, s, "stop", tok),
		"bad text part":    ocLine(t, "text", s, map[string]any{"part": map[string]any{"type": "reasoning", "text": ocHandoff}}) + ocFinish(t, s, "stop", tok),
	}
	for name, raw := range refused {
		if _, _, e := DecodeOpenCodeEvents([]byte(raw)); e == nil {
			t.Errorf("%s: admitted", name)
		}
	}
	if got := ObservedOpenCodeSession([]byte(refused["session changed"])); got != "" {
		t.Fatalf("ambiguous session observed %q", got)
	}
	if got := ObservedOpenCodeSession([]byte(refused["host error"])); got != s {
		t.Fatalf("failed run session %q", got)
	}
}

func TestCALV0077_OpenCodeUsageObservedOrUnknown(t *testing.T) {
	s := "ses_1"
	raw := ocFinish(t, s, "tool-calls", ocTokens(10, 2, 3, 4, 5)) + ocText(t, s, ocHandoff) + ocFinish(t, s, "stop", ocTokens(20, 1, 0, 6, 0))
	i, o, known := ObservedOpenCodeUsage([]byte(raw))
	if !known || i != 10+4+5+20+6 || o != 2+3+1 {
		t.Fatal(i, o, known)
	}
	unknown := map[string]string{
		"empty":         "",
		"no finish":     ocText(t, s, ocHandoff),
		"no tokens":     ocFinish(t, s, "stop", nil),
		"missing cache": ocFinish(t, s, "stop", map[string]any{"input": 1, "output": 1, "reasoning": 0}),
		"null counter":  ocFinish(t, s, "stop", ocTokens(1, nil, 0, 0, 0)),
		"negative":      ocFinish(t, s, "stop", ocTokens(-1, 1, 0, 0, 0)),
		"fractional":    ocFinish(t, s, "stop", ocTokens(1.5, 1, 0, 0, 0)),
		"string":        ocFinish(t, s, "stop", ocTokens("1", 1, 0, 0, 0)),
		"partial step":  ocFinish(t, s, "tool-calls", ocTokens(1, 1, 0, 0, 0)) + ocFinish(t, s, "stop", nil),
		"overflow":      ocFinish(t, s, "stop", ocTokens(uint64(1<<63), 1, 0, uint64(1<<63), 0)),
		"host error":    ocFinish(t, s, "stop", ocTokens(1, 1, 0, 0, 0)) + ocLine(t, "error", s, map[string]any{"error": map[string]any{"type": "unknown", "message": "x"}}),
		"unqualified":   ocFinish(t, s, "stop", ocTokens(1, 1, 0, 0, 0)) + ocLine(t, "patch", s, nil),
	}
	for name, raw := range unknown {
		if _, _, known := ObservedOpenCodeUsage([]byte(raw)); known {
			t.Errorf("%s: invented observed usage", name)
		}
	}
}

// TestCALV0077_OpenCodeUsageIncompleteAccounting: usage is known only when
// every started step finished and the turn ended with "stop". An interrupted
// step, or a stream cut at the output limit mid-line or at a line boundary,
// is incomplete accounting and leaves usage unobserved and the turn refused.
func TestCALV0077_OpenCodeUsageIncompleteAccounting(t *testing.T) {
	s := "ses_1"
	start := ocLine(t, "step_start", s, map[string]any{"part": map[string]any{"type": "step-start"}})
	first := start + ocFinish(t, s, "tool-calls", ocTokens(10, 2, 0, 0, 0))
	whole := first + start + ocText(t, s, ocHandoff) + ocFinish(t, s, "stop", ocTokens(5, 1, 0, 0, 0))
	if i, o, known := ObservedOpenCodeUsage([]byte(whole)); !known || i != 15 || o != 3 {
		t.Fatalf("complete accounting %d %d %v", i, o, known)
	}
	incomplete := map[string]string{
		"interrupted step":       first + start + ocLine(t, "tool_use", s, map[string]any{"part": map[string]any{"type": "tool", "tool": "bash"}}),
		"interrupted after stop": start + ocText(t, s, ocHandoff) + ocFinish(t, s, "stop", ocTokens(1, 1, 0, 0, 0)) + start,
		"cut at line boundary":   first,
		"cut mid-line":           whole[:len(whole)-7],
		"cut inside next step":   first + start + ocText(t, s, ocHandoff),
	}
	for name, raw := range incomplete {
		if _, _, known := ObservedOpenCodeUsage([]byte(raw)); known {
			t.Errorf("%s: partial usage reported as known", name)
		}
		if _, _, e := DecodeOpenCodeEvents([]byte(raw)); e == nil {
			t.Errorf("%s: incomplete turn admitted", name)
		}
	}
	// A retained stream cut exactly at the output limit, on a line boundary
	// after a stop step, is still possibly partial accounting.
	tail := start + ocText(t, s, ocHandoff) + ocFinish(t, s, "stop", ocTokens(5, 1, 0, 0, 0))
	pad := MaxHostOutput - len(tail) - len(ocLine(t, "reasoning", s, map[string]any{"part": map[string]any{"type": "reasoning", "text": ""}}))
	full := ocLine(t, "reasoning", s, map[string]any{"part": map[string]any{"type": "reasoning", "text": strings.Repeat("r", pad)}}) + tail
	if len(full) != MaxHostOutput {
		t.Fatalf("boundary fixture is %d bytes", len(full))
	}
	if _, _, known := ObservedOpenCodeUsage([]byte(full)); known {
		t.Error("stream filling the output limit reported as known usage")
	}
	if _, _, known := ObservedOpenCodeUsage([]byte(full[len(full)-len(tail):])); !known {
		t.Error("the same complete stream under the limit left usage unknown")
	}
}

func TestCALV0076_OpenCodeVocabularySelected(t *testing.T) {
	s := "ses_1"
	raw := []byte(ocText(t, s, ocHandoff) + ocFinish(t, s, "stop", ocTokens(1, 2, 0, 0, 0)))
	host, ok := HostVocabulary(HostOpenCode)
	if !ok {
		t.Fatal("opencode vocabulary absent")
	}
	if session, result, e := host.Decode(raw); e != nil || session != s || result.Kind != "BUILT" || host.Session(raw) != s {
		t.Fatalf("opencode vocabulary %q %+v %v", session, result, e)
	}
	if i, o, known := host.Usage(raw); !known || i != 1 || o != 2 {
		t.Fatalf("opencode usage %d %d %v", i, o, known)
	}
	for _, other := range []string{HostCodex, HostClaudeCode} {
		v, _ := HostVocabulary(other)
		if _, _, e := v.Decode(raw); e == nil {
			t.Fatalf("%s vocabulary decoded an OpenCode stream", other)
		}
	}
}

// TestCALV0077_OpenCodeDuplicateMembers proves a repeated member anywhere in
// an event line, or a member aliasing a read name by case folding in the
// event, its text or step-finish part, the token counters or the JSON
// handoff, is refused rather than resolved last-wins, and that the result,
// session and usage readers all leave it unobserved.
func TestCALV0077_OpenCodeDuplicateMembers(t *testing.T) {
	s := "ses_1"
	text := func(part string) string {
		return `{"type":"text","timestamp":1,"sessionID":"ses_1","part":` + part + "}\n"
	}
	finish := func(part string) string {
		return `{"type":"step_finish","timestamp":2,"sessionID":"ses_1","part":` + part + "}\n"
	}
	handoff := text(`{"type":"text","text":"{\"kind\":\"BUILT\",\"summary\":\"done\",\"nextAction\":\"review\"}"}`)
	tokens := `{"input":1,"output":1,"reasoning":0,"cache":{"read":0,"write":0}}`
	stop := finish(`{"type":"step-finish","reason":"stop","tokens":` + tokens + `}`)
	review := func(h string) string { return text(`{"type":"text","text":"`+h+`"}`) + stop }
	if _, _, e := DecodeOpenCodeEvents([]byte(handoff + stop)); e != nil {
		t.Fatalf("baseline refused: %v", e)
	}
	if _, _, known := ObservedOpenCodeUsage([]byte(handoff + stop)); !known {
		t.Fatal("baseline usage unobserved")
	}
	cases := map[string]string{
		"error masked by type":      `{"type":"error","type":"step_start","sessionID":"ses_1","part":{"type":"step-start"}}` + "\n" + handoff + stop,
		"escaped repeated type":     `{"type":"error","\u0074ype":"step_start","sessionID":"ses_1","part":{"type":"step-start"}}` + "\n" + handoff + stop,
		"repeated session":          `{"type":"text","sessionID":"ses_2","sessionID":"ses_1","part":{"type":"text","text":"x"}}` + "\n" + handoff + stop,
		"type case alias":           `{"type":"error","Type":"step_start","sessionID":"ses_1","part":{"type":"step-start"}}` + "\n" + handoff + stop,
		"session case alias":        `{"type":"text","sessionID":"ses_1","SessionID":"ses_2","part":{"type":"text","text":"x"}}` + "\n" + handoff + stop,
		"repeated part":             text(`{"type":"text","text":"x"},"part":{"type":"text","text":"y"}`) + handoff + stop,
		"repeated text member":      text(`{"type":"text","text":"{\"kind\":\"BUILT\",\"summary\":\"done\",\"nextAction\":\"review\"}","text":"prose"}`) + stop,
		"text case alias":           text(`{"type":"text","text":"prose","Text":"{\"kind\":\"BUILT\",\"summary\":\"done\",\"nextAction\":\"review\"}"}`) + stop,
		"repeated reason":           handoff + finish(`{"type":"step-finish","reason":"length","reason":"stop","tokens":`+tokens+`}`),
		"reason case alias":         handoff + finish(`{"type":"step-finish","reason":"length","Reason":"stop","tokens":`+tokens+`}`),
		"repeated tokens objects":   handoff + finish(`{"type":"step-finish","reason":"stop","tokens":{"input":1,"output":1},"tokens":{"reasoning":0,"cache":{"read":0,"write":0}}}`),
		"repeated counter":          handoff + finish(`{"type":"step-finish","reason":"stop","tokens":{"input":1,"output":1,"reasoning":0,"cache":{"read":0,"write":0},"input":9}}`),
		"repeated cache objects":    handoff + finish(`{"type":"step-finish","reason":"stop","tokens":{"input":1,"output":1,"reasoning":0,"cache":{"read":0},"cache":{"write":0}}}`),
		"tokens case alias":         handoff + finish(`{"type":"step-finish","reason":"stop","Tokens":`+tokens+`}`),
		"counter case alias":        handoff + finish(`{"type":"step-finish","reason":"stop","tokens":{"input":1,"Output":1,"output":1,"reasoning":0,"cache":{"read":0,"write":0}}}`),
		"cache counter alias":       handoff + finish(`{"type":"step-finish","reason":"stop","tokens":{"input":1,"output":1,"reasoning":0,"cache":{"read":0,"Write":0,"write":0}}}`),
		"nested repeated member":    `{"type":"tool_use","sessionID":"ses_1","part":{"type":"tool","state":{"a":1,"a":2}}}` + "\n" + handoff + stop,
		"contradictory acceptance":  review(`{\"kind\":\"REVIEW\",\"accepted\":false,\"accepted\":true,\"claims\":[],\"summary\":\"verified\",\"nextAction\":\"integrate\"}`),
		"escaped repeated accepted": review(`{\"kind\":\"REVIEW\",\"accepted\":false,\"\\u0061ccepted\":true,\"claims\":[],\"summary\":\"verified\",\"nextAction\":\"integrate\"}`),
		"accepted case alias":       review(`{\"kind\":\"REVIEW\",\"accepted\":false,\"Accepted\":true,\"claims\":[],\"summary\":\"verified\",\"nextAction\":\"integrate\"}`),
		"handoff kelvin-sign alias": review(`{\"kind\":\"WAIT\",\"\\u212aind\":\"BUILT\",\"summary\":\"done\",\"nextAction\":\"review\"}`),
		"repeated handoff kind":     review(`{\"kind\":\"WAIT\",\"kind\":\"BUILT\",\"summary\":\"done\",\"nextAction\":\"review\"}`),
	}
	for name, raw := range cases {
		for i, line := range strings.Split(strings.TrimSuffix(raw, "\n"), "\n") {
			if !json.Valid([]byte(line)) {
				t.Fatalf("%s: fixture line %d is not JSON", name, i)
			}
		}
		if session, _, e := DecodeOpenCodeEvents([]byte(raw)); e == nil || session != "" {
			t.Fatalf("%s: decoded (session %q, %v)", name, session, e)
		}
		if _, _, known := ObservedOpenCodeUsage([]byte(raw)); known {
			t.Fatalf("%s: usage observed", name)
		}
		if got := ObservedOpenCodeSession([]byte(raw)); got != "" {
			t.Fatalf("%s: session observed as %q", name, got)
		}
	}
	// The same name in distinct objects, and a prose last text, stay admitted.
	distinct := `{"type":"tool_use","sessionID":"ses_1","part":{"type":"tool","state":{"a":{"a":1}},"list":[{"a":1},{"a":2}]}}` + "\n" + handoff + stop
	if _, _, e := DecodeOpenCodeEvents([]byte(distinct)); e != nil {
		t.Fatalf("same name in distinct objects refused: %v", e)
	}
	prose := text(`{"type":"text","text":"ran out of ideas"}`) + stop
	if got := ObservedOpenCodeSession([]byte(prose)); got != s {
		t.Fatalf("prose text left the session unobserved: %q", got)
	}
}
