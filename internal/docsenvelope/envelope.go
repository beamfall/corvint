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
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/versionline"
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
// It names the command that installs the core set (CMS-V0-930); a missing
// corvint itself gets only the manual install, because the updater replaces an
// installed corvint and cannot place a first one.
func NotInstalledMessage(binary, forwarded string) string {
	next := InstallHint()
	if binary == CoreBinary {
		next = manualInstall
	}
	return forwarded + " is served by the " + binary + " executable, which is NOT_INSTALLED: " + next +
		", or put " + binary + " beside this executable or on PATH"
}

const manualInstall = "install the core set from its release archives as docs/INSTALL.md describes"

// InstallHint is the CMS-V0-930 next step of the not-installed and
// version-mismatch refusals: the updater command that installs the whole core
// set at one version into the running executable's directory, or the manual
// install where that command would refuse.
func InstallHint() string {
	dir := ""
	if self, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(self); err == nil {
			self = resolved
		}
		dir = filepath.Dir(self)
	}
	return installHint(dir, ProbeVersion)
}

// installHint is InstallHint for the executables in dir. The updater replaces
// a corvint it can order against releases, so it names the manual install
// when dir holds no corvint or holds an unnumbered one (build 0 or none, a
// local build the updater refuses to replace, UPD-V0-010). An unknown dir
// keeps the command without --bin-dir.
func installHint(dir string, probe func(context.Context, string) (versionline.Line, error)) string {
	command := "install the core set with: " + InstallCommand(dir) +
		" (or the manual core-set archive install in docs/INSTALL.md)"
	if dir == "" {
		return command
	}
	core := filepath.Join(dir, CoreBinary)
	if info, err := os.Stat(core); err != nil || !info.Mode().IsRegular() {
		return manualInstall + " (the updater replaces an installed corvint, and none is beside this executable)"
	}
	if line, err := probe(context.Background(), core); err != nil || line.Build == "" || line.Build == "0" {
		return manualInstall + " (the updater does not replace the unnumbered corvint beside this executable)"
	}
	return command
}

// InstallCommand is the updater command that installs the core set into dir,
// without --bin-dir when dir is empty. A dir that needs shell quoting is
// single-quoted.
func InstallCommand(dir string) string {
	command := "corvint-update apply --allow-network --component core"
	if dir == "" {
		return command
	}
	return command + " --bin-dir " + shellWord(dir)
}

func shellWord(s string) string {
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("/._-", r)) {
			return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
		}
	}
	return s
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
