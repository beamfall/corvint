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

func DecodeEvents(raw []byte) (string, HostResult, error) {
	session := ""
	result := HostResult{}
	completed := false
	final := ""
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		var event map[string]json.RawMessage
		if e := json.Unmarshal(scanner.Bytes(), &event); e != nil {
			return session, result, e
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
// turn-completed event. Missing dimensions remain unobserved.
func ObservedUsage(raw []byte) (uint64, uint64, bool) {
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
