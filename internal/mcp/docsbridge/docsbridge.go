// SPDX-License-Identifier: Apache-2.0

// Package docsbridge exposes the experimental source-documentation
// draft/consume tools (docs/specs/source-documentation-draft-v0.md) to the
// core MCP servers without linking a documentation package: every call runs
// the corvint-docs app's `docs-tool` verb and reads back one
// docsenvelope.ToolResult (decision 0594 item 3, CMS-V0-003). The tool
// descriptors, argument bounds, root binding and the four-way result shape
// are unchanged from the in-process bridge, so corvint-mcp's wire bytes are
// the same; the native implementation lives in internal/docstools.
package docsbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/docsenvelope"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/rootbind"
)

const (
	// ToolDraft and ToolConsume are the served tool names; docsenvelope carries the same
	// strings for the app, and the bridge's test pins them equal.
	ToolDraft   = "corvint.docs_draft"
	ToolConsume = "corvint.docs_consume"

	draftMaxBytes   = docsenvelope.DraftMaxBytes
	maxArgumentSize = docsenvelope.MaxArgumentBytes
	maxPathRunes    = docsenvelope.MaxPathRunes
	maxTaskBytes    = docsenvelope.MaxTaskBytes

	// waitDelay bounds how long a cancelled app process may hold its pipes.
	waitDelay = 5 * time.Second
)

// Error is a closed, sanitized transport-level failure: malformed arguments,
// an unknown tool name, cancellation, or an app process that answered with no
// envelope. It never carries repository paths or process output.
type Error struct{ Code string }

func (failure *Error) Error() string { return failure.Code }

// ToolFailure is a business-level refusal, reported as an MCP tool error
// (isError: true), never as a successful call. Code and Message are exactly
// the CLI's `docs` stderr envelope fields for the same input
// (unsupported-documentation-source, documentation-limit-exceeded,
// stale-documentation-draft, repository-unavailable), or the not-installed
// gap when no corvint-docs executable is present.
type ToolFailure struct{ Code, Message string }

// Registry binds every call to one canonical local repository root, matching
// internal/mcp/bridge.Registry's identity and Git-directory checks exactly.
type Registry struct {
	root         string
	rootIdentity os.FileInfo
	git          *rootbind.Binding
	locate       func() (string, error)
}

func New(root string) (*Registry, *Error) {
	if !validRoot(root) {
		return nil, failure("invalid-root")
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil || !filepath.IsAbs(resolved) || filepath.Clean(resolved) != resolved {
		return nil, failure("invalid-root")
	}
	rootIdentity, err := os.Stat(resolved)
	if err != nil || !rootIdentity.IsDir() {
		return nil, failure("invalid-root")
	}
	git, ok := rootbind.Bind(resolved) // shared with corvint-mcp: .git directory or linked-worktree gitfile
	if !ok {
		return nil, failure("invalid-root")
	}
	return &Registry{root: resolved, rootIdentity: rootIdentity, git: git, locate: locateApp}, nil
}

func locateApp() (string, error) { return docsenvelope.Locate(docsenvelope.Binary) }

// Tools is the exact delivered surface: the two experimental docs tools only.
func (registry *Registry) Tools() []bridge.ToolDescriptor {
	if registry == nil {
		return nil
	}
	pathSchema := map[string]any{
		"type": "string", "minLength": 1, "maxLength": maxPathRunes,
		"pattern": bridge.SubjectPathRule.Pattern(), // MCPV0-954
	}
	return []bridge.ToolDescriptor{
		{
			Name:        ToolConsume,
			Description: "Freshly rederive the canonical source-documentation draft from original immutable sources, reject byte drift, and return the same bounded task match as the CLI (docs/specs/source-documentation-draft-v0.md, experimental).",
			Annotations: readAnnotations(),
			InputSchema: objectSchema(map[string]any{
				"source":  pathSchema,
				"package": pathSchema,
				"task":    map[string]any{"type": "string", "minLength": 1, "maxLength": maxTaskBytes},
				"draft":   map[string]any{"type": "string", "minLength": 1, "maxLength": draftMaxBytes},
			}, []any{"source", "package", "task", "draft"}),
		},
		{
			Name:        ToolDraft,
			Description: "Draft source-pinned Markdown orientation from an owner Markdown source and a tracked module directory in any language the native index extracts declarations from (others are refused by name), identical to `corvint docs draft` (docs/specs/source-documentation-draft-v0.md, experimental). dry_run true previews over an untracked working-tree owner file or inline owner_markdown; that result is NOT_ADMISSIBLE, governs nothing, and docs_consume refuses it (SDD-V0-019).",
			Annotations: readAnnotations(),
			InputSchema: objectSchema(map[string]any{
				"source":         pathSchema,
				"package":        pathSchema,
				"dry_run":        map[string]any{"type": "boolean"},
				"owner_markdown": map[string]any{"type": "string", "minLength": 1, "maxLength": draftMaxBytes},
			}, []any{"source", "package"}),
		},
	}
}

func readAnnotations() bridge.ToolAnnotations {
	return bridge.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: false, IdempotentHint: true, OpenWorldHint: false}
}

// Call dispatches one tool by name. It returns exactly one of: a successful
// structured/text pair, a ToolFailure (the caller must report isError: true),
// or a transport Error (invalid arguments, unknown tool, cancellation, or an
// app process that did not answer with an envelope).
func (registry *Registry) Call(ctx context.Context, name string, arguments []byte) (structured map[string]any, text string, toolFailure *ToolFailure, transportErr *Error) {
	if registry == nil || registry.root == "" || registry.rootIdentity == nil || registry.git == nil || registry.locate == nil {
		return nil, "", nil, failure("invalid-registry")
	}
	if err := ctx.Err(); err != nil {
		return nil, "", nil, failure("cancelled")
	}
	if len(arguments) == 0 || len(arguments) > maxArgumentSize || !utf8.Valid(arguments) {
		return nil, "", nil, failure("invalid-arguments")
	}
	if name != ToolDraft && name != ToolConsume {
		return nil, "", nil, failure("unsupported-tool")
	}
	if !registry.sameRoot() {
		return nil, "", nil, failure("repository-unavailable")
	}
	executable, err := registry.locate()
	if err != nil {
		return nil, "", &ToolFailure{Code: docsenvelope.NotInstalledCode, Message: docsenvelope.NotInstalledMessage(docsenvelope.Binary, name)}, nil
	}
	result, transport := registry.run(ctx, executable, name, arguments)
	if transport != nil {
		return nil, "", nil, transport
	}
	switch {
	case result.Error != nil:
		return nil, "", nil, failure(result.Error.Code)
	case result.Failure != nil:
		return nil, "", &ToolFailure{Code: result.Failure.Code, Message: result.Failure.Message}, nil
	case result.Structured == nil:
		return nil, "", nil, failure("internal-error")
	}
	return result.Structured, result.Text, nil, nil
}

// run executes one `docs-tool` call and decodes its envelope. The app answers
// every refusal inside the envelope with exit 0; a non-zero exit, oversized or
// unparseable stdout is a transport failure, and a caller cancellation stays
// cancelled.
func (registry *Registry) run(ctx context.Context, executable, name string, arguments []byte) (docsenvelope.ToolResult, *Error) {
	var result docsenvelope.ToolResult
	command := exec.CommandContext(ctx, executable, docsenvelope.ToolVerb, "--root", registry.root, name)
	command.Stdin = bytes.NewReader(arguments)
	command.Stderr = io.Discard
	command.WaitDelay = waitDelay
	var stdout bytes.Buffer
	command.Stdout = &boundedWriter{limit: docsenvelope.MaxResultBytes, buffer: &stdout}
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return result, failure("cancelled")
		}
		return result, failure("internal-error")
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || result.Schema != docsenvelope.ToolSchema || result.Tool != name {
		if ctx.Err() != nil {
			return result, failure("cancelled")
		}
		return result, failure("internal-error")
	}
	return result, nil
}

// boundedWriter refuses output past its limit, which fails the app process
// (and so the call) instead of buffering an unbounded result.
type boundedWriter struct {
	limit  int
	buffer *bytes.Buffer
}

var errOutputBound = errors.New("docs-tool output exceeds bound")

func (writer *boundedWriter) Write(p []byte) (int, error) {
	if len(p) > writer.limit-writer.buffer.Len() {
		return 0, errOutputBound
	}
	return writer.buffer.Write(p)
}

func (registry *Registry) sameRoot() bool {
	current, err := os.Stat(registry.root)
	if err != nil || !current.IsDir() || !os.SameFile(registry.rootIdentity, current) {
		return false
	}
	return registry.git.Same(registry.root)
}

func validRoot(root string) bool {
	if root == "" || len(root) > 4096 || !utf8.ValidString(root) || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return false
	}
	for _, character := range root {
		if character == 0 || unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func objectSchema(properties map[string]any, required []any) map[string]any {
	return map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type":    "object", "properties": properties, "required": required, "additionalProperties": false,
	}
}

func failure(code string) *Error { return &Error{Code: code} }
