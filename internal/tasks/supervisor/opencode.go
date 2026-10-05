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
}

// readOpenCodeEvents admits only JSON-object lines of the qualified event
// types, all naming one bounded session. A host `error` line may carry an
// empty session (a run error reported before the session exists); it marks
// the stream failed instead of naming a session.
func readOpenCodeEvents(raw []byte) (openCodeStream, error) {
	s := openCodeStream{lastFinish: -1, lastText: -1}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for i := 0; scanner.Scan(); i++ {
		var ev openCodeEvent
		if e := json.Unmarshal(scanner.Bytes(), &ev); e != nil {
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
		case "step_start", "reasoning", "tool_use", "error":
		case "text":
			var part struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			if e := json.Unmarshal(ev.Part, &part); e != nil || part.Type != "text" {
				return s, fmt.Errorf("unqualified text part")
			}
			s.final, s.lastText = part.Text, i
		case "step_finish":
			s.finishes = append(s.finishes, ev.Part)
			s.lastFinish = i
		default:
			return s, fmt.Errorf("unqualified event %q", ev.Type)
		}
	}
	return s, scanner.Err()
}

// DecodeOpenCodeEvents decodes an OpenCode run (CAL-V0-077): one constant
// bounded session, no host error, a final step that finished with reason
// "stop", and a last text part, inside a finished step, that strictly
// decodes to the minimum handoff.
func DecodeOpenCodeEvents(raw []byte) (string, HostResult, error) {
	result := HostResult{}
	s, e := readOpenCodeEvents(raw)
	if e != nil {
		return s.session, result, e
	}
	if s.failed {
		return s.session, result, fmt.Errorf("host reported failure")
	}
	if s.session == "" || len(s.finishes) == 0 || s.final == "" || s.lastText > s.lastFinish {
		return s.session, result, fmt.Errorf("incomplete host turn")
	}
	var finish struct {
		Type   string `json:"type"`
		Reason string `json:"reason"`
	}
	if e := json.Unmarshal(s.finishes[len(s.finishes)-1], &finish); e != nil || finish.Type != "step-finish" || finish.Reason != "stop" {
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
// and output is output plus reasoning. A stream with a host error (whose
// failed step reports no tokens), no finished step, a missing or non-integer
// counter, or an overflow leaves usage unobserved.
func ObservedOpenCodeUsage(raw []byte) (uint64, uint64, bool) {
	s, e := readOpenCodeEvents(raw)
	if e != nil || s.failed || len(s.finishes) == 0 {
		return 0, 0, false
	}
	var input, output uint64
	for _, raw := range s.finishes {
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
