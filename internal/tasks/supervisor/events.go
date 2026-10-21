package supervisor

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
)

type HostResult struct {
	Accepted   bool     `json:"accepted,omitempty"`
	Claims     []string `json:"claims,omitempty"`
	Question   string   `json:"question,omitempty"`
	Kind       string   `json:"kind"`
	Summary    string   `json:"summary"`
	NextAction string   `json:"nextAction"`
}

// codexEventMembers names every member the Codex readers take from one event
// line. Each line must repeat no member at any depth and must not alias a
// read name by case: encoding/json keeps the last duplicate and matches names
// case-insensitively, so a hostile turn could otherwise hide an error event,
// swap the session identity or the final handoff, or merge partial usage
// counters (CAL-V0-075; V1-0799, V1-0950).
var codexEventMembers = memberTree{names: []string{"type", "thread_id", "item", "usage"}, nested: map[string]memberTree{
	"item":  {names: []string{"type", "text"}},
	"usage": {names: []string{"input_tokens", "output_tokens"}},
}}

func checkCodexLine(line []byte) error {
	if e := uniqueMembers(line); e != nil {
		return e
	}
	return codexEventMembers.check(line)
}

// codexMembers is the one member verdict the Codex result, session and usage
// readers share: every line passes checkCodexLine, and a final agent_message
// text that is JSON repeats no member and aliases no handoff member. Any
// refusal leaves the session and usage unobserved as well as refusing the
// result; a prose final text is not a handoff and keeps them observable
// (CAL-V0-075; V1-0799, V1-0950).
func codexMembers(raw []byte) error {
	final := ""
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		if e := checkCodexLine(scanner.Bytes()); e != nil {
			return e
		}
		var event struct {
			Type string `json:"type"`
			Item struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"item"`
		}
		if json.Unmarshal(scanner.Bytes(), &event) == nil && event.Type == "item.completed" && event.Item.Type == "agent_message" {
			final = event.Item.Text
		}
	}
	if e := scanner.Err(); e != nil {
		return e
	}
	if json.Valid([]byte(final)) {
		return exactMembers([]byte(final), handoffMembers)
	}
	return nil
}

func DecodeEvents(raw []byte) (string, HostResult, error) {
	session := ""
	result := HostResult{}
	if e := codexMembers(raw); e != nil {
		return "", result, e
	}
	completed := false
	final := ""
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		var event map[string]json.RawMessage
		if e := json.Unmarshal(scanner.Bytes(), &event); e != nil {
			return "", result, e
		}
		var kind string
		json.Unmarshal(event["type"], &kind)
		switch kind {
		case "thread.started":
			var id string
			if e := json.Unmarshal(event["thread_id"], &id); e != nil || id == "" || len(id) > 128 {
				return session, result, fmt.Errorf("missing session identity")
			}
			if session != "" && session != id {
				return session, result, fmt.Errorf("session identity changed")
			}
			session = id
		case "turn.completed":
			completed = true
		case "turn.started", "item.started", "item.updated":
		case "item.completed":
			var item struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			if e := json.Unmarshal(event["item"], &item); e != nil {
				return session, result, e
			}
			if item.Type == "agent_message" {
				final = item.Text
			}
		case "turn.failed", "error":
			return session, result, fmt.Errorf("host reported failure")
		default:
			return session, result, fmt.Errorf("unqualified event %q", kind)
		}
	}
	if e := scanner.Err(); e != nil {
		return session, result, e
	}
	if session == "" || !completed || final == "" {
		return session, result, fmt.Errorf("incomplete host turn")
	}
	if e := decode([]byte(final), &result); e != nil {
		return session, result, e
	}
	if !validHandoff(result) {
		return session, result, fmt.Errorf("invalid minimum handoff result")
	}
	return session, result, nil
}

// ObservedUsage admits only complete integer usage counters from the qualified
// turn-completed event. Missing or null dimensions, and any line that repeats
// or case-aliases a member, remain unobserved (V1-0950).
func ObservedUsage(raw []byte) (uint64, uint64, bool) {
	if codexMembers(raw) != nil {
		return 0, 0, false
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	var input, output uint64
	found := false
	for scanner.Scan() {
		var event struct {
			Type  string                     `json:"type"`
			Usage map[string]json.RawMessage `json:"usage"`
		}
		if json.Unmarshal(scanner.Bytes(), &event) != nil {
			return 0, 0, false
		}
		if event.Type != "turn.completed" {
			continue
		}
		var i, o uint64
		if nullCounter(event.Usage["input_tokens"]) || nullCounter(event.Usage["output_tokens"]) {
			return 0, 0, false
		}
		if json.Unmarshal(event.Usage["input_tokens"], &i) != nil || json.Unmarshal(event.Usage["output_tokens"], &o) != nil {
			return 0, 0, false
		}
		if input+i < input || output+o < output {
			return 0, 0, false
		}
		input += i
		output += o
		found = true
	}
	return input, output, found && scanner.Err() == nil
}

func ObservedSession(raw []byte) string {
	if codexMembers(raw) != nil {
		return ""
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	session := ""
	for scanner.Scan() {
		var e struct {
			Type string `json:"type"`
			ID   string `json:"thread_id"`
		}
		if json.Unmarshal(scanner.Bytes(), &e) != nil {
			return ""
		}
		if e.Type == "thread.started" {
			if e.ID == "" || len(e.ID) > 128 || (session != "" && session != e.ID) {
				return ""
			}
			session = e.ID
		}
	}
	return session
}

// nullCounter reports a JSON null counter, which encoding/json would decode
// as zero instead of refusing.
func nullCounter(raw json.RawMessage) bool {
	return string(bytes.TrimSpace(raw)) == "null"
}
