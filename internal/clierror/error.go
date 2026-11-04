// SPDX-License-Identifier: Apache-2.0
// Package clierror defines the shared process-level CLI failure contract.
// Native protocol documents remain nested evidence; successes retain their own schemas.
package clierror

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
)

// Envelope is one CLI refusal on stderr. Exit 2 means usage/invalid input;
// exit 1 means an operational refusal; exit 0 means success. Command-specific
// advisory states (for example an open frontier) keep their documented status.
type Envelope struct {
	Binary  string `json:"binary"`
	Code    string `json:"code"`
	Error   string `json:"error"`
	Help    string `json:"help"`
	OK      bool   `json:"ok"`
	Details any    `json:"details,omitempty"`
}

// Validate is also the message/code lint used by the conformance suite.
func Validate(e Envelope) error {
	if e.Code == "" || strings.TrimSpace(e.Error) == "" {
		return fmt.Errorf("a refusal needs a code and a human-readable message")
	}
	if e.Error == e.Code {
		return fmt.Errorf("message equals code %q", e.Code)
	}
	if e.OK {
		return fmt.Errorf("an error cannot be ok")
	}
	return nil
}

func Write(w io.Writer, binary, code, message string, details any) {
	if code == "" {
		code = "command-failed"
	}
	if strings.TrimSpace(message) == "" || message == code {
		message = "Command refused: " + strings.ReplaceAll(strings.ReplaceAll(code, "-", " "), "_", " ") + "; inspect the retained details and command help."
	}
	envelope := map[string]any{}
	if native, ok := details.(map[string]any); ok {
		for key, value := range native {
			envelope[key] = value
		}
	}
	envelope["binary"], envelope["code"], envelope["error"], envelope["help"], envelope["ok"] = binary, code, message, binary+" --help", false
	if details != nil {
		envelope["details"] = details
	}
	_ = json.NewEncoder(w).Encode(envelope)
}

// Streams adapts native command writers at the executable boundary. Error
// documents go to stderr in the shared shape, with their original evidence in
// details. Success/protocol bytes and ordinary progress diagnostics pass through.
type Streams struct {
	Binary   string
	Out, Err io.Writer
	Exit     int
	Protocol bool
	mu       sync.Mutex
	pending  bytes.Buffer
	refusal  map[string]any
	finished bool
}
type streamWriter struct {
	s      *Streams
	stderr bool
}

func (s *Streams) Stdout() io.Writer {
	if s.Protocol {
		return s.Out
	}
	return streamWriter{s, false}
}
func (s *Streams) Stderr() io.Writer { return streamWriter{s, true} }

// Diagnostic preserves live progress output for long-running local servers.
func (s *Streams) Diagnostic() io.Writer { return s.Err }

// Finish flushes diagnostics on a naturally returning successful main.
func (s *Streams) Finish() { s.Status(0) }
func (s *Streams) Status(native int) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := native
	if s.Exit != 0 {
		status = s.Exit
	}
	if native == 2 {
		status = 2
	}
	if s.finished {
		return status
	}
	s.finished = true
	if s.refusal != nil {
		code, message := classify(s.refusal)
		Write(s.Err, s.Binary, code, message, s.refusal)
	} else if s.pending.Len() > 0 {
		if status != 0 {
			code := "command-failed"
			if status == 2 {
				code = "invalid-arguments"
			}
			Write(s.Err, s.Binary, code, strings.TrimSpace(s.pending.String()), map[string]any{"stderr": s.pending.String()})
		} else {
			_, _ = s.Err.Write(s.pending.Bytes())
		}
	} else if status != 0 && !s.Protocol {
		code := "command-failed"
		if status == 2 {
			code = "invalid-arguments"
		}
		Write(s.Err, s.Binary, code, "Command refused without a diagnostic; run "+s.Binary+" --help.", nil)
	}
	return status
}
func (w streamWriter) Write(p []byte) (int, error) {
	w.s.mu.Lock()
	defer w.s.mu.Unlock()
	var v map[string]any
	if json.Unmarshal(p, &v) == nil && v != nil && failure(v) {
		code, _ := classify(v)
		w.s.Exit = 1
		if code == "invalid-arguments" || code == "unknown-verb" || code == "unknown-help-topic" || code == "MALFORMED" {
			w.s.Exit = 2
		}
		w.s.refusal = v
		return len(p), nil
	}
	if w.stderr {

		const bound = 1 << 20
		if left := bound - w.s.pending.Len(); left > 0 {
			_, _ = w.s.pending.Write(p[:min(left, len(p))])
		}
		return len(p), nil
	}
	return w.s.Out.Write(p)
}
func failure(v map[string]any) bool {
	if ok, exists := v["ok"]; exists && ok == false {
		return true
	}
	if outcome, _ := v["outcome"].(string); outcome == "ERROR" || outcome == "REFUSED" {
		return true
	}
	if value, exists := v["error"]; exists && value != nil && value != "" {
		return true
	}
	return false
}
func classify(v map[string]any) (string, string) {
	code, _ := v["code"].(string)
	message, _ := v["error"].(string)
	if e, ok := v["error"].(map[string]any); ok {
		if code == "" {
			code, _ = e["code"].(string)
		}
		message, _ = e["message"].(string)
	}
	if code == "" {
		if codes, ok := v["codes"].([]any); ok && len(codes) > 0 {
			code, _ = codes[0].(string)
		}
	}
	if message == "" {
		if warnings, ok := v["warnings"].([]any); ok {
			for _, warning := range warnings {
				if text, ok := warning.(string); ok {
					if message != "" {
						message += "; "
					}
					message += text
				}
			}
		}
	}
	if message == "" {
		message, _ = v["message"].(string)
	}
	if code == "" {
		code = "command-failed"
	}
	return code, message
}
