package supervisor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
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
//
// A Detached host starts helper processes in process groups of their own,
// outside the supervisor-owned group (CAL-V0-077). Its capsule must carry
// every DetachedEnv entry, and Run discovers, drains and proves gone those
// groups before it reports a clean stop.
//
// An InterruptedSession host reports its session as its run starts, so a
// stage stopped at its wall still names the session a checkpointed
// continuation resumes (CAL-V0-089). A host that reports its session only in
// its final result cannot continue an interrupted stage.
type Vocabulary struct {
	Decode             func([]byte) (string, HostResult, error)
	Session            func([]byte) string
	Usage              func([]byte) (uint64, uint64, bool)
	Detached           bool
	DetachedEnv        []string
	InterruptedSession bool
}

// HostVocabulary returns the vocabulary of host; "" is Codex. An unknown host
// has none.
func HostVocabulary(host string) (Vocabulary, bool) {
	switch host {
	case "", HostCodex:
		return Vocabulary{Decode: DecodeEvents, Session: ObservedSession, Usage: ObservedUsage, InterruptedSession: true}, true
	case HostClaudeCode:
		return Vocabulary{Decode: DecodeClaudeResult, Session: ObservedClaudeSession, Usage: ObservedClaudeUsage}, true
	case HostOpenCode:
		// OPENCODE_PRINT_LOGS=1 makes the detached standalone server inherit
		// the host standard error, so end of file on that pipe proves no
		// server outlived the run.
		return Vocabulary{Decode: DecodeOpenCodeEvents, Session: ObservedOpenCodeSession, Usage: ObservedOpenCodeUsage, Detached: true, DetachedEnv: []string{"OPENCODE_PRINT_LOGS=1"}, InterruptedSession: true}, true
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

// claudeMembers and handoffMembers are the member names the profile reads
// from the result object and from the handoff it carries.
var (
	claudeMembers  = []string{"type", "subtype", "is_error", "result", "session_id", "usage"}
	handoffMembers = []string{"accepted", "claims", "question", "kind", "summary", "nextAction"}
)

// readClaudeResult admits exactly one JSON object, optionally followed by
// whitespace. Every reader of the vocabulary (result, session and usage)
// shares it, so a repeated or aliased member in the object or in a JSON
// handoff leaves all three unobserved (CAL-V0-075).
func readClaudeResult(raw []byte) (claudeResult, error) {
	var r claudeResult
	if e := exactMembers(raw, claudeMembers); e != nil {
		return r, e
	}
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
	// A result text that is JSON is a handoff and obeys the same member
	// rule; a prose result (for example an error report) is not one.
	if r.Result != nil && json.Valid([]byte(*r.Result)) {
		if e := exactMembers([]byte(*r.Result), handoffMembers); e != nil {
			return r, e
		}
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

// exactMembers refuses a JSON text that repeats a member name in any object,
// or whose top-level object carries a member that differs from one of names
// only by case folding. encoding/json matches struct fields
// case-insensitively, so "IS_ERROR" or "Usage" would otherwise reach the same
// field as the exact name; with aliases refused the struct decode is exact.
func exactMembers(raw []byte, names []string) error {
	if e := uniqueMembers(raw); e != nil {
		return e
	}
	return aliasFree(raw, names)
}

// aliasFree refuses a JSON object carrying a member that differs from one of
// names only by case folding. A text that is not an object is left to the
// strict decode.
func aliasFree(raw []byte, names []string) error {
	var top map[string]json.RawMessage
	if json.Unmarshal(raw, &top) != nil {
		return nil // not an object: the strict decode refuses it
	}
	for k := range top {
		for _, name := range names {
			if k != name && strings.EqualFold(k, name) {
				return fmt.Errorf("JSON member %q aliases %q", k, name)
			}
		}
	}
	return nil
}

// uniqueMembers refuses a JSON text in which any object repeats a member
// name. encoding/json keeps the last duplicate and merges a repeated object,
// so without this an is_error true could be overridden, or two partial usage
// objects could combine into an apparently complete observation (CAL-V0-075).
func uniqueMembers(raw []byte) error {
	type frame struct {
		keys      map[string]bool
		object    bool
		expectKey bool
	}
	var stack []*frame
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	for {
		t, e := d.Token()
		if e == io.EOF {
			return nil
		}
		if e != nil {
			return e
		}
		var top *frame
		if len(stack) > 0 {
			top = stack[len(stack)-1]
		}
		switch v := t.(type) {
		case json.Delim:
			switch v {
			case '{', '[':
				if top != nil && top.object {
					top.expectKey = true
				}
				stack = append(stack, &frame{keys: map[string]bool{}, object: v == '{', expectKey: v == '{'})
			default:
				stack = stack[:len(stack)-1]
			}
		case string:
			if top != nil && top.object && top.expectKey {
				if top.keys[v] {
					return fmt.Errorf("duplicate JSON member %q", v)
				}
				top.keys[v] = true
				top.expectKey = false
				continue
			}
			if top != nil && top.object {
				top.expectKey = true
			}
		default:
			if top != nil && top.object {
				top.expectKey = true
			}
		}
	}
}

func validHandoff(r HostResult) bool {
	return (r.Kind == "HANDOFF" || r.Kind == "BUILT" || r.Kind == "REVIEW" || r.Kind == "WAIT") && r.Summary != "" && r.NextAction != ""
}
