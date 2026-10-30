package supervisor

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
)

// openCodeEvent is one `opencode run --format json` standard-output line
// (CAL-V0-077). Fields this profile does not read are ignored.
type openCodeEvent struct {
	Type      string          `json:"type"`
	SessionID string          `json:"sessionID"`
	Part      json.RawMessage `json:"part"`
}

// openCodeStream is the qualified reading of one OpenCode event stream.
type openCodeStream struct {
	session    string
	finishes   []json.RawMessage
	lastFinish int
	final      string
	lastText   int
	failed     bool
	// open is true while a step_start has no step_finish yet: the host was
	// interrupted mid-step, so its accounting is incomplete.
	open bool
}

// memberTree names the members the profile reads from one JSON object and
// the nested objects it reads below them.
type memberTree struct {
	names  []string
	nested map[string]memberTree
}

// The OpenCode objects this profile reads: the event line, a text part, and a
// step-finish part with its token counters.
var (
	openCodeEventMembers  = memberTree{names: []string{"type", "sessionID", "part"}}
	openCodeTextMembers   = memberTree{names: []string{"type", "text"}}
	openCodeFinishMembers = memberTree{names: []string{"type", "reason", "tokens"}, nested: map[string]memberTree{
		"tokens": {names: []string{"input", "output", "reasoning", "cache"}, nested: map[string]memberTree{
			"cache": {names: []string{"read", "write"}},
		}},
	}}
)

// check refuses a member of raw, or of a nested object the tree reads, that
// aliases a read name by case folding. Repeated members are refused for the
// whole line by uniqueMembers before any object is checked.
func (m memberTree) check(raw []byte) error {
	if e := aliasFree(raw, m.names); e != nil {
		return e
	}
	var top map[string]json.RawMessage
	if len(m.nested) == 0 || json.Unmarshal(raw, &top) != nil {
		return nil
	}
	for name, sub := range m.nested {
		if v, ok := top[name]; ok {
			if e := sub.check(v); e != nil {
				return e
			}
		}
	}
	return nil
}

// readOpenCodeEvents admits only JSON-object lines of the qualified event
// types, all naming one bounded session. A host `error` line may carry an
// empty session (a run error reported before the session exists); it marks
// the stream failed instead of naming a session. Every reader of the
// vocabulary (result, session and usage) shares it, so a repeated member in
// any line, a member aliasing a read name by case, or either in a JSON last
// text leaves all three unobserved (CAL-V0-077): encoding/json keeps the last
// duplicate and matches names case-insensitively, so an error could
// otherwise be masked, a refusal accepted or partial counters merged.
func readOpenCodeEvents(raw []byte) (openCodeStream, error) {
	s := openCodeStream{lastFinish: -1, lastText: -1}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for i := 0; scanner.Scan(); i++ {
		line := scanner.Bytes()
		if e := uniqueMembers(line); e != nil {
			return s, e
		}
		if e := openCodeEventMembers.check(line); e != nil {
			return s, e
		}
		var ev openCodeEvent
		if e := json.Unmarshal(line, &ev); e != nil {
			return s, e
		}
		if ev.Type == "error" {
			s.failed = true
			if ev.SessionID == "" {
				continue
			}
		}
		if ev.SessionID == "" || len(ev.SessionID) > 128 {
			return s, fmt.Errorf("missing session identity")
		}
		if s.session != "" && s.session != ev.SessionID {
			return s, fmt.Errorf("session identity changed")
		}
		s.session = ev.SessionID
		switch ev.Type {
		case "step_start":
			s.open = true
		case "reasoning", "tool_use", "error":
		case "text":
			var part struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			if e := openCodeTextMembers.check(ev.Part); e != nil {
				return s, e
			}
			if e := json.Unmarshal(ev.Part, &part); e != nil || part.Type != "text" {
				return s, fmt.Errorf("unqualified text part")
			}
			s.final, s.lastText = part.Text, i
		case "step_finish":
			if e := openCodeFinishMembers.check(ev.Part); e != nil {
				return s, e
			}
			s.finishes = append(s.finishes, ev.Part)
			s.lastFinish = i
			s.open = false
		default:
			return s, fmt.Errorf("unqualified event %q", ev.Type)
		}
	}
	if e := scanner.Err(); e != nil {
		return s, e
	}
	// A last text that is JSON is the handoff and obeys the same member rule;
	// a prose text is not one.
	if json.Valid([]byte(s.final)) {
		if e := exactMembers([]byte(s.final), handoffMembers); e != nil {
			return s, e
		}
	}
	return s, nil
}

// DecodeOpenCodeEvents decodes an OpenCode run (CAL-V0-077): one constant
// bounded session, no host error, a final step that finished with reason
// "stop", and a last text part, inside a finished step, that strictly
// decodes to the minimum handoff.
func DecodeOpenCodeEvents(raw []byte) (string, HostResult, error) {
	result := HostResult{}
	s, e := readOpenCodeEvents(raw)
	if e != nil {
		return "", result, e
	}
	if s.failed {
		return s.session, result, fmt.Errorf("host reported failure")
	}
	if s.session == "" || s.final == "" || s.lastText > s.lastFinish || !s.complete() {
		return s.session, result, fmt.Errorf("incomplete host turn")
	}
	if e := decode([]byte(s.final), &result); e != nil {
		return s.session, result, e
	}
	if !validHandoff(result) {
		return s.session, result, fmt.Errorf("invalid minimum handoff result")
	}
	return s.session, result, nil
}

// complete reports a finished turn: at least one step, no step left open,
// and a last step that finished with reason "stop". A step that finished with
// "tool-calls" is followed by another step, so a stream ending there was cut.
func (s openCodeStream) complete() bool {
	if s.open || len(s.finishes) == 0 {
		return false
	}
	var finish struct {
		Type   string `json:"type"`
		Reason string `json:"reason"`
	}
	return json.Unmarshal(s.finishes[len(s.finishes)-1], &finish) == nil && finish.Type == "step-finish" && finish.Reason == "stop"
}

// ObservedOpenCodeSession is the one bounded session an OpenCode stream
// names, or "" when the output is not a qualified stream.
func ObservedOpenCodeSession(raw []byte) string {
	s, e := readOpenCodeEvents(raw)
	if e != nil {
		return ""
	}
	return s.session
}

// ObservedOpenCodeUsage sums the token counters of every finished step. The
// counters are disjoint, so input is input plus cache read and cache write,
// and output is output plus reasoning. Usage is known only when accounting is
// complete: a stream with a host error (whose failed step reports no tokens),
// a step started but never finished (an interrupted or truncated run), a last
// step that did not finish with "stop", a missing or non-integer counter, an
// overflow, or output that fills the retained bound (the host may have
// written more than was kept, even if the kept prefix ends on a stop step)
// leaves usage unobserved. Only the retained bytes decide, so the store and
// the native transition derive the same usage.
func ObservedOpenCodeUsage(raw []byte) (uint64, uint64, bool) {
	if len(raw) >= MaxHostOutput {
		return 0, 0, false
	}
	s, e := readOpenCodeEvents(raw)
	if e != nil || s.failed || !s.complete() {
		return 0, 0, false
	}
	var input, output uint64
	for _, raw := range s.finishes {
		i, o, ok := openCodeStepCounters(raw)
		if !ok || input+i < input || output+o < output {
			return 0, 0, false
		}
		input, output = input+i, output+o
	}
	return input, output, true
}

// openCodeStepCounters sums one step-finish part's disjoint counters: input
// plus cache read and cache write, and output plus reasoning.
func openCodeStepCounters(raw json.RawMessage) (input, output uint64, ok bool) {
	var part struct {
		Tokens struct {
			Input     json.RawMessage `json:"input"`
			Output    json.RawMessage `json:"output"`
			Reasoning json.RawMessage `json:"reasoning"`
			Cache     struct {
				Read  json.RawMessage `json:"read"`
				Write json.RawMessage `json:"write"`
			} `json:"cache"`
		} `json:"tokens"`
	}
	if json.Unmarshal(raw, &part) != nil {
		return 0, 0, false
	}
	t := part.Tokens
	for _, add := range []struct {
		sum *uint64
		v   json.RawMessage
	}{{&input, t.Input}, {&input, t.Cache.Read}, {&input, t.Cache.Write}, {&output, t.Output}, {&output, t.Reasoning}} {
		n, ok := openCodeCounter(add.v)
		if !ok || *add.sum+n < *add.sum {
			return 0, 0, false
		}
		*add.sum += n
	}
	return input, output, true
}

// openCodeCounter admits only a nonnegative JSON integer; null, absent,
// fractional or string counters are not observations.
func openCodeCounter(v json.RawMessage) (uint64, bool) {
	var n uint64
	if len(v) == 0 || bytes.Equal(bytes.TrimSpace(v), []byte("null")) || json.Unmarshal(v, &n) != nil {
		return 0, false
	}
	return n, true
}

// OpenCodeLineUsage reads one OpenCode event line for the dispatcher's
// incremental token accounting (CAL-V0-157) under the same member rules as
// readOpenCodeEvents. It returns the event type and, for a step_finish, the
// part's type, finish reason and counters (openCodeStepCounters). ok is
// false for a line that is not a qualified event object, or a step_finish
// whose part or counters are not observations.
func OpenCodeLineUsage(line []byte) (kind, partType, reason string, input, output uint64, ok bool) {
	if uniqueMembers(line) != nil || openCodeEventMembers.check(line) != nil {
		return "", "", "", 0, 0, false
	}
	var ev openCodeEvent
	if json.Unmarshal(line, &ev) != nil {
		return "", "", "", 0, 0, false
	}
	if ev.Type != "step_finish" {
		return ev.Type, "", "", 0, 0, true
	}
	var part struct {
		Type   string `json:"type"`
		Reason string `json:"reason"`
	}
	if openCodeFinishMembers.check(ev.Part) != nil || json.Unmarshal(ev.Part, &part) != nil {
		return ev.Type, "", "", 0, 0, false
	}
	if input, output, ok = openCodeStepCounters(ev.Part); !ok {
		return ev.Type, "", "", 0, 0, false
	}
	return ev.Type, part.Type, part.Reason, input, output, true
}
