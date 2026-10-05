package supervisor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// Supervised host names (CAL-V0-074, CAL-V0-076). A capsule, config or
// policy without a host is Codex, so Codex bytes stay unchanged.
const (
	HostCodex      = "codex"
	HostClaudeCode = "claude-code"
	HostOpenCode   = "opencode"
)

// Vocabulary reads one supervised host's retained standard output: the
// handoff result of a zero exit, the host session, and token usage.
type Vocabulary struct {
	Decode  func([]byte) (string, HostResult, error)
	Session func([]byte) string
	Usage   func([]byte) (uint64, uint64, bool)
}

// HostVocabulary returns the vocabulary of host; "" is Codex. An unknown host
// has none.
func HostVocabulary(host string) (Vocabulary, bool) {
	switch host {
	case "", HostCodex:
		return Vocabulary{Decode: DecodeEvents, Session: ObservedSession, Usage: ObservedUsage}, true
	case HostClaudeCode:
		return Vocabulary{Decode: DecodeClaudeResult, Session: ObservedClaudeSession, Usage: ObservedClaudeUsage}, true
	case HostOpenCode:
		return Vocabulary{Decode: DecodeOpenCodeEvents, Session: ObservedOpenCodeSession, Usage: ObservedOpenCodeUsage}, true
	}
	return Vocabulary{}, false
}

// claudeResult is the single `--output-format json` object of Claude Code.
// Fields this profile does not read are ignored.
type claudeResult struct {
	Type      string                     `json:"type"`
	Subtype   string                     `json:"subtype"`
	IsError   *bool                      `json:"is_error"`
	Result    *string                    `json:"result"`
	SessionID string                     `json:"session_id"`
	Usage     map[string]json.RawMessage `json:"usage"`
}

// readClaudeResult admits exactly one JSON object, optionally followed by
// whitespace.
func readClaudeResult(raw []byte) (claudeResult, error) {
	var r claudeResult
	d := json.NewDecoder(bytes.NewReader(raw))
	if e := d.Decode(&r); e != nil {
		return r, e
	}
	if _, e := d.Token(); e != io.EOF {
		return r, fmt.Errorf("trailing host output")
	}
	if r.Type != "result" {
		return r, fmt.Errorf("unqualified result type %q", r.Type)
	}
	return r, nil
}

// DecodeClaudeResult decodes the Claude Code result object (CAL-V0-075): a
// successful, non-error result carrying a bounded session and a result string
// that strictly decodes to the minimum handoff.
func DecodeClaudeResult(raw []byte) (string, HostResult, error) {
	result := HostResult{}
	r, e := readClaudeResult(raw)
	if e != nil {
		return "", result, e
	}
	if r.SessionID == "" || len(r.SessionID) > 128 {
		return "", result, fmt.Errorf("missing session identity")
	}
	if r.Subtype != "success" || r.IsError == nil || *r.IsError {
		return r.SessionID, result, fmt.Errorf("host reported failure")
	}
	if r.Result == nil || *r.Result == "" {
		return r.SessionID, result, fmt.Errorf("incomplete host turn")
	}
	if e := decode([]byte(*r.Result), &result); e != nil {
		return r.SessionID, result, e
	}
	if !validHandoff(result) {
		return r.SessionID, result, fmt.Errorf("invalid minimum handoff result")
	}
	return r.SessionID, result, nil
}

// ObservedClaudeSession is the bounded session of a Claude Code result
// object, or "" when the output is not one.
func ObservedClaudeSession(raw []byte) string {
	r, e := readClaudeResult(raw)
	if e != nil || len(r.SessionID) > 128 {
		return ""
	}
	return r.SessionID
}

// ObservedClaudeUsage admits integer input and output counters from the
// result object's usage. Input adds the cache creation and cache read
// counters when present, so it counts all input as Codex's counter does. A
// missing or non-integer counter, or an overflow, leaves usage unobserved.
func ObservedClaudeUsage(raw []byte) (uint64, uint64, bool) {
	r, e := readClaudeResult(raw)
	if e != nil {
		return 0, 0, false
	}
	input, ok1 := claudeCounter(r.Usage["input_tokens"])
	output, ok2 := claudeCounter(r.Usage["output_tokens"])
	if !ok1 || !ok2 {
		return 0, 0, false
	}
	for _, k := range []string{"cache_creation_input_tokens", "cache_read_input_tokens"} {
		v, present := r.Usage[k]
		if !present {
			continue
		}
		n, ok := claudeCounter(v)
		if !ok || input+n < input {
			return 0, 0, false
		}
		input += n
	}
	return input, output, true
}

// claudeCounter admits only a nonnegative JSON integer; null, absent,
// fractional or string counters are not observations.
func claudeCounter(v json.RawMessage) (uint64, bool) {
	var n uint64
	if len(v) == 0 || bytes.Equal(bytes.TrimSpace(v), []byte("null")) || json.Unmarshal(v, &n) != nil {
		return 0, false
	}
	return n, true
}

func validHandoff(r HostResult) bool {
	return (r.Kind == "HANDOFF" || r.Kind == "BUILT" || r.Kind == "REVIEW" || r.Kind == "WAIT") && r.Summary != "" && r.NextAction != ""
}
