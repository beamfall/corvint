// SPDX-License-Identifier: Apache-2.0
// Package clierror provides opt-in CLI refusal helpers and preserves native streams.
package clierror

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Envelope is an opt-in JSON CLI refusal. Native commands own their exit codes.
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
	envelope["binary"], envelope["code"], envelope["error"], envelope["help"], envelope["ok"] = binary, code, message, binary+" --help", false
	if details != nil {
		envelope["details"] = details
	}
	_ = json.NewEncoder(w).Encode(envelope)
}

// Streams preserves each native command's output and exit contract. Existing JSON
// refusals are already envelopes; process adaptation must not rewrite their shape,
// redirect stdout, buffer diagnostics, or synthesize a failure from plain text.
type Streams struct {
	Binary   string
	Out, Err io.Writer
	Protocol bool
}

func (s *Streams) Stdout() io.Writer     { return s.Out }
func (s *Streams) Stderr() io.Writer     { return s.Err }
func (s *Streams) Diagnostic() io.Writer { return s.Err }
func (s *Streams) Finish()               {}
func (s *Streams) Status(native int) int { return native }
