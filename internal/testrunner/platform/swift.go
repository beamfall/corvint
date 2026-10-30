package platform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// checkJSON rejects duplicate keys before ordinary decoding can discard them.
func checkJSON(b []byte) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if depth > 32 {
			return fmt.Errorf("JSON nesting bound")
		}
		t, err := d.Token()
		if err != nil {
			return err
		}
		switch t {
		case json.Delim('{'):
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return err
				}
				s, ok := key.(string)
				if !ok || seen[s] {
					return fmt.Errorf("duplicate or invalid JSON key")
				}
				seen[s] = true
				if err := value(depth + 1); err != nil {
					return err
				}
			}
			end, err := d.Token()
			if err != nil || end != json.Delim('}') {
				return fmt.Errorf("unterminated JSON object")
			}
		case json.Delim('['):
			for d.More() {
				if err := value(depth + 1); err != nil {
					return err
				}
			}
			end, err := d.Token()
			if err != nil || end != json.Delim(']') {
				return fmt.Errorf("unterminated JSON array")
			}
		}
		return nil
	}
	if err := value(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("trailing JSON")
	}
	return nil
}

type swiftDefinition struct {
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	Parameterized bool   `json:"isParameterized"`
	Source        struct {
		FileID string `json:"fileID"`
	} `json:"sourceLocation"`
}
type swiftEvent struct {
	Kind      string `json:"kind"`
	TestID    string `json:"testID"`
	Iteration *int   `json:"iteration"`
	Issue     *struct {
		Known      *bool           `json:"isKnown"`
		Expression json.RawMessage `json:"_expression"`
	} `json:"issue"`
	Messages []struct {
		Symbol string `json:"symbol"`
		Text   string `json:"text"`
	} `json:"messages"`
}
type swiftState struct {
	definition swiftDefinition
	started    bool
	ended      bool
	skipped    bool
	issues     []tr.Attempt
}

func parseSwift(in tr.Input, o *tr.Observation) error {
	b, ok := in.Reports["swift-events.jsonl"]
	if !ok || len(in.Reports) != 1 || len(b) == 0 || b[len(b)-1] != '\n' || !utf8.Valid(b) {
		return fmt.Errorf("missing or truncated Swift event stream")
	}
	states := map[string]*swiftState{}
	started, ended := false, false
	failedRun := false
	for _, line := range bytes.Split(bytes.TrimSuffix(b, []byte("\n")), []byte("\n")) {
		if err := checkJSON(line); err != nil {
			return err
		}
		var record struct {
			Version *int            `json:"version"`
			Kind    string          `json:"kind"`
			Payload json.RawMessage `json:"payload"`
		}
		d := json.NewDecoder(bytes.NewReader(line))
		d.DisallowUnknownFields()
		if err := d.Decode(&record); err != nil || record.Version == nil || *record.Version != 0 || ended {
			return fmt.Errorf("unsupported Swift record version, shape or ordering")
		}
		if record.Kind == "test" {
			var def swiftDefinition
			if err := json.Unmarshal(record.Payload, &def); err != nil || def.ID == "" || def.Name == "" || states[def.ID] != nil || started {
				return fmt.Errorf("invalid Swift test definition")
			}
			if def.Kind != "function" && def.Kind != "suite" {
				return fmt.Errorf("unknown Swift test definition kind")
			}
			if def.Parameterized {
				return fmt.Errorf("Swift parameterized case identities are not qualified")
			}
			states[def.ID] = &swiftState{definition: def}
			if len(states) > tr.MaxTests {
				return fmt.Errorf("Swift test bound")
			}
			continue
		}
		if record.Kind != "event" {
			return fmt.Errorf("unknown Swift record kind")
		}
		var e swiftEvent
		if err := json.Unmarshal(record.Payload, &e); err != nil {
			return err
		}
		if e.Iteration != nil && *e.Iteration != 0 {
			return fmt.Errorf("Swift repeated iteration needs qualified attempt identity")
		}
		s := states[e.TestID]
		switch e.Kind {
		case "runStarted":
			if started || e.TestID != "" {
				return fmt.Errorf("duplicate Swift run start")
			}
			started = true
		case "runEnded":
			if !started || e.TestID != "" {
				return fmt.Errorf("invalid Swift run end")
			}
			ended = true
			for _, m := range e.Messages {
				if m.Symbol == "fail" {
					failedRun = true
				}
			}
		case "testStarted":
			if !started || s == nil || s.started || s.ended {
				return fmt.Errorf("unknown or repeated Swift test start")
			}
			s.started = true
		case "testEnded":
			if !started || s == nil || !s.started || s.ended {
				return fmt.Errorf("missing or duplicate Swift test start/end")
			}
			s.ended = true
		case "testSkipped":
			if !started || s == nil || s.started || s.ended {
				return fmt.Errorf("invalid Swift skip")
			}
			s.skipped, s.ended = true, true
		case "issueRecorded":
			if !started || s == nil || !s.started || s.ended || e.Issue == nil || e.Issue.Known == nil {
				return fmt.Errorf("unbound Swift issue")
			}
			if *e.Issue.Known {
				problem(o, "KNOWN_ISSUE", "Swift known issue does not establish a passing criterion")
			}
			kind := tr.Unknown
			if len(e.Issue.Expression) != 0 && string(e.Issue.Expression) != "null" {
				kind = tr.Assertion
			}
			messages := []string{}
			for _, m := range e.Messages {
				messages = append(messages, m.Text)
			}
			s.issues = append(s.issues, tr.Attempt{State: tr.Failed, FailureKind: kind, Message: strings.Join(messages, "\n")})
		default:
			return fmt.Errorf("Swift event %q needs qualified case/attachment handling", e.Kind)
		}
	}
	if !started || !ended {
		return fmt.Errorf("incomplete Swift run lifecycle")
	}
	failed := false
	for id, s := range states {
		if s.definition.Kind == "suite" {
			if len(s.issues) != 0 || s.started != s.ended {
				problem(o, "SUITE_FAILURE", "Swift suite lifecycle or issue prevents complete observation")
			}
			continue
		}
		if !s.ended {
			return fmt.Errorf("Swift selected test has no terminal event")
		}
		state, kind, message := tr.Passed, "", ""
		if s.skipped {
			state = tr.Skipped
		} else if len(s.issues) != 0 {
			failed = true
			state, kind = tr.Failed, s.issues[0].FailureKind
			parts := []string{}
			for _, issue := range s.issues {
				parts = append(parts, issue.Message)
				if issue.FailureKind != kind {
					kind = tr.Unknown
				}
			}
			message = strings.Join(parts, "\n")
		}
		o.Tests = append(o.Tests, test(id, s.definition.Name, s.definition.Source.FileID, "", state, kind, message))
	}
	if failedRun != failed || (failed && in.ExitCode != 1) || (!failed && in.ExitCode != 0) {
		problem(o, "EXIT_CONTRADICTION", "Swift issues, final report and process exit disagree")
	}
	o.Complete = len(o.Problems) == 0
	return nil
}
