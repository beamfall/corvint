// SPDX-License-Identifier: Apache-2.0

// Package docsenvelope is the published, versioned envelope between the core
// and the corvint-docs app (decision 0594 item 3, CMS-V0-003). The core never
// imports a documentation package: `corvint docs ...` and `corvint --corpus=FILE
// ...` are exec-only shims onto the corvint-docs executable, and the MCP docs
// tools reach the app through the `docs-tool` verb, whose stdout is one
// ToolResult. This package imports no documentation package and no core
// package, so both sides can share it.
package docsenvelope

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	// Binary is the documentation app's executable name.
	Binary = "corvint-docs"
	// CoreBinary is the core executable the app runs for `--corpus=FILE` joins.
	CoreBinary = "corvint"

	// ToolVerb is the app verb that answers one MCP docs tool call:
	// `corvint-docs docs-tool --root ABSOLUTE_ROOT TOOL < arguments.json`.
	ToolVerb = "docs-tool"
	// ToolSchema names the ToolResult document the verb prints.
	ToolSchema = "corvint-docs-tool/1"

	// ToolDraft and ToolConsume are the MCP tool names the verb accepts.
	ToolDraft   = "corvint.docs_draft"
	ToolConsume = "corvint.docs_consume"

	// DraftMaxBytes mirrors doccompiler.DraftMaxBytes, which the app's own
	// test pins; the core reads only this copy.
	DraftMaxBytes = 64 << 10
	// MaxArgumentBytes bounds the tool arguments the core forwards.
	MaxArgumentBytes = 3 * DraftMaxBytes
	// MaxResultBytes bounds the envelope the core reads back: the app bounds
	// a result at twice DraftMaxBytes; the rest is framing slack.
	MaxResultBytes = 3 * DraftMaxBytes
	// MaxPathRunes and MaxTaskBytes bound the tool's path and task arguments.
	MaxPathRunes = 1024
	MaxTaskBytes = 1024

	// NotInstalledCode is the coded gap a shim reports when the executable it
	// forwards to is absent (CMS-V0-003: a named NOT_INSTALLED gap).
	NotInstalledCode = "not-installed"
)

// ErrNotInstalled reports that Locate found no executable.
var ErrNotInstalled = errors.New(NotInstalledCode)

// Coded is one coded refusal: Message is empty for a transport Error.
type Coded struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

// ToolResult is the `docs-tool` verb's stdout document. Exactly one of
// Structured (with Text), Failure or Error is set: Failure is a business
// refusal the MCP server reports as isError, Error a transport failure.
type ToolResult struct {
	Schema     string         `json:"schema"`
	Tool       string         `json:"tool"`
	Structured map[string]any `json:"structured,omitempty"`
	Text       string         `json:"text,omitempty"`
	Failure    *Coded         `json:"failure,omitempty"`
	Error      *Coded         `json:"error,omitempty"`
}

// NotInstalledMessage is the gap message a shim emits for a missing executable.
func NotInstalledMessage(binary, forwarded string) string {
	return forwarded + " is served by the " + binary + " executable, which is NOT_INSTALLED: install " +
		binary + " beside this executable or on PATH"
}

// Locate finds the named sibling executable: first beside the running
// executable, then on PATH. It returns ErrNotInstalled when neither has it.
func Locate(name string) (string, error) {
	if self, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(self); err == nil {
			self = resolved
		}
		candidate := filepath.Join(filepath.Dir(self), name+executableSuffix)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", ErrNotInstalled
	}
	return path, nil
}
