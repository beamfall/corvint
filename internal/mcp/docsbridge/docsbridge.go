// Package docsbridge exposes the experimental source-documentation draft/consume
// implementation (docs/specs/source-documentation-draft-v0.md) as read-only MCP
// tools. It calls exactly the native compiler the corvint CLI's `docs draft`
// and `docs consume` commands call (internal/doccompiler and
// internal/contextindex.BuildContext); it never shells out to corvint and
// never duplicates the compiler's claims, provenance, or limit logic.
package docsbridge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/doccompiler"
	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
)

const (
	ToolDraft   = "corvint.docs_draft"
	ToolConsume = "corvint.docs_consume"

	resultSchema    = "corvint-mcp-docsbridge-result/0"
	draftMaxBytes   = doccompiler.DraftMaxBytes
	maxArgumentSize = 3 * draftMaxBytes
	maxPathRunes    = 1024
	maxTaskBytes    = 1024
)

// Error is a closed, sanitized transport-level failure: malformed arguments,
// an unknown tool name, or cancellation. It never carries repository paths or
// underlying process output.
type Error struct{ Code string }

func (failure *Error) Error() string { return failure.Code }

// ToolFailure is a business-level refusal from the native compiler, reported
// as an MCP tool error (isError: true), never as a successful call. Code and
// Message are exactly the CLI's `docs` stderr envelope fields for the same
// input (unsupported-documentation-source, documentation-limit-exceeded,
// stale-documentation-draft, repository-unavailable).
type ToolFailure struct{ Code, Message string }

// Registry binds every call to one canonical local repository root, matching
// internal/mcp/bridge.Registry's identity and Git-directory checks exactly.
type Registry struct {
	root         string
	rootIdentity os.FileInfo
	gitIdentity  os.FileInfo
	build        func(context.Context, string, string) (*contextindex.Index, error)
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
	gitMarker, err := os.Lstat(filepath.Join(resolved, ".git"))
	if err != nil || !gitMarker.IsDir() || gitMarker.Mode()&os.ModeSymlink != 0 {
		return nil, failure("invalid-root")
	}
	return &Registry{root: resolved, rootIdentity: rootIdentity, gitIdentity: gitMarker, build: contextindex.BuildContext}, nil
}

// Tools is the exact delivered surface: the two experimental docs tools only.
func (registry *Registry) Tools() []bridge.ToolDescriptor {
	if registry == nil {
		return nil
	}
	pathSchema := map[string]any{
		"type": "string", "minLength": 1, "maxLength": maxPathRunes,
		"pattern": `^(?!/)(?!.*(?:^|/)[.]{1,2}(?:/|$))(?!.*//)(?!.*\\).+$`,
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
			Description: "Draft source-pinned Markdown orientation from an owner Markdown source and a tracked Go package directory, identical to `corvint docs draft` (docs/specs/source-documentation-draft-v0.md, experimental).",
			Annotations: readAnnotations(),
			InputSchema: objectSchema(map[string]any{
				"source":  pathSchema,
				"package": pathSchema,
			}, []any{"source", "package"}),
		},
	}
}

func readAnnotations() bridge.ToolAnnotations {
	return bridge.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: false, IdempotentHint: true, OpenWorldHint: false}
}

type draftInput struct {
	Source  string `json:"source"`
	Package string `json:"package"`
}

type consumeInput struct {
	Source  string `json:"source"`
	Package string `json:"package"`
	Task    string `json:"task"`
	Draft   string `json:"draft"`
}

// Call dispatches one tool by name. It returns exactly one of: a successful
// structured/text pair, a ToolFailure (the caller must report isError: true),
// or a transport Error (invalid arguments, unknown tool, or cancellation).
func (registry *Registry) Call(ctx context.Context, name string, arguments []byte) (structured map[string]any, text string, toolFailure *ToolFailure, transportErr *Error) {
	if registry == nil || registry.root == "" || registry.rootIdentity == nil || registry.gitIdentity == nil || registry.build == nil {
		return nil, "", nil, failure("invalid-registry")
	}
	if err := ctx.Err(); err != nil {
		return nil, "", nil, failure("cancelled")
	}
	if len(arguments) == 0 || len(arguments) > maxArgumentSize || !utf8.Valid(arguments) {
		return nil, "", nil, failure("invalid-arguments")
	}
	ctx = gitstatus.WithIsolation(ctx)
	switch name {
	case ToolDraft:
		return registry.callDraft(ctx, arguments)
	case ToolConsume:
		return registry.callConsume(ctx, arguments)
	default:
		return nil, "", nil, failure("unsupported-tool")
	}
}

func (registry *Registry) callDraft(ctx context.Context, arguments []byte) (map[string]any, string, *ToolFailure, *Error) {
	var input draftInput
	if err := decodeClosed(arguments, &input); err != nil {
		return nil, "", malformedArguments(), nil
	}
	if !validRelativePath(input.Source) || !validRelativePath(input.Package) {
		return nil, "", nil, failure("invalid-arguments")
	}
	if !registry.sameRoot() {
		return nil, "", nil, failure("repository-unavailable")
	}
	index, err := registry.build(ctx, registry.root, "")
	if err != nil {
		return nil, "", nil, transportOrToolFailure(ctx, err)
	}
	draft, err := doccompiler.DraftSources(index, input.Source, input.Package)
	if err != nil {
		if problem, ok := asDoccompilerError(err); ok {
			return nil, "", &ToolFailure{Code: problem.Code, Message: problem.Message}, nil
		}
		return nil, "", nil, transportOrToolFailure(ctx, err)
	}
	digest := sha256.Sum256(draft.Markdown)
	object := map[string]any{
		"schema": resultSchema, "tool": ToolDraft, "mutates": false, "state": "READY",
		"derivation": "GENERATED", "behavior": "UNKNOWN",
		"commit": draft.Commit, "tree": draft.Tree,
		"source": input.Source, "package": input.Package,
		"markdown": string(draft.Markdown), "markdown_sha256": hex.EncodeToString(digest[:]),
		"limitations": draft.Limitations,
	}
	if err := boundResult(object); err != nil {
		return nil, "", nil, err
	}
	return object, string(draft.Markdown), nil, nil
}

func (registry *Registry) callConsume(ctx context.Context, arguments []byte) (map[string]any, string, *ToolFailure, *Error) {
	var input consumeInput
	if err := decodeClosed(arguments, &input); err != nil {
		return nil, "", malformedArguments(), nil
	}
	if !validRelativePath(input.Source) || !validRelativePath(input.Package) {
		return nil, "", nil, failure("invalid-arguments")
	}
	if !validTask(input.Task) || len(input.Draft) == 0 || len(input.Draft) > draftMaxBytes || !utf8.ValidString(input.Draft) {
		return nil, "", malformedArguments(), nil
	}
	if !registry.sameRoot() {
		return nil, "", nil, failure("repository-unavailable")
	}
	index, err := registry.build(ctx, registry.root, "")
	if err != nil {
		return nil, "", nil, transportOrToolFailure(ctx, err)
	}
	draft, err := doccompiler.DraftSources(index, input.Source, input.Package)
	if err != nil {
		if problem, ok := asDoccompilerError(err); ok {
			return nil, "", &ToolFailure{Code: problem.Code, Message: problem.Message}, nil
		}
		return nil, "", nil, transportOrToolFailure(ctx, err)
	}
	result, err := doccompiler.ConsumeDraft(draft, []byte(input.Draft), input.Task)
	if err != nil {
		if problem, ok := asDoccompilerError(err); ok {
			return nil, "", &ToolFailure{Code: problem.Code, Message: problem.Message}, nil
		}
		return nil, "", nil, transportOrToolFailure(ctx, err)
	}
	// This call freshly built the immutable source index and reconstructed the
	// exact provided draft above; byte equality alone cannot grant this label,
	// matching the CLI's own attachment of SOURCE_REDERIVED after rederivation.
	result["validation"] = "SOURCE_REDERIVED"
	if err := boundResult(result); err != nil {
		return nil, "", nil, err
	}
	encoded, encodeErr := gokernel.CanonicalJSON(result)
	if encodeErr != nil {
		return nil, "", nil, failure("internal-error")
	}
	return result, string(encoded), nil, nil
}

func (registry *Registry) sameRoot() bool {
	current, err := os.Stat(registry.root)
	if err != nil || !current.IsDir() || !os.SameFile(registry.rootIdentity, current) {
		return false
	}
	gitMarker, err := os.Lstat(filepath.Join(registry.root, ".git"))
	return err == nil && gitMarker.IsDir() && gitMarker.Mode()&os.ModeSymlink == 0 && os.SameFile(registry.gitIdentity, gitMarker)
}

func boundResult(object map[string]any) *Error {
	encoded, err := gokernel.CanonicalJSON(object)
	if err != nil {
		return failure("internal-error")
	}
	if len(encoded) > draftMaxBytes*2 {
		return failure("documentation-limit-exceeded")
	}
	return nil
}

func asDoccompilerError(err error) (*doccompiler.Error, bool) {
	var problem *doccompiler.Error
	return problem, errors.As(err, &problem)
}

func transportOrToolFailure(ctx context.Context, err error) *Error {
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return failure("cancelled")
	}
	code := errorCode(err)
	if strings.HasPrefix(code, "repository-") || code == "unsupported-git-object-format" || strings.HasPrefix(err.Error(), "Git ") {
		return failure("repository-unavailable")
	}
	return failure("internal-error")
}

func errorCode(err error) string {
	var contextFailure *contextindex.Error
	if errors.As(err, &contextFailure) {
		return contextFailure.Code
	}
	var kernelFailure *gokernel.Error
	if errors.As(err, &kernelFailure) {
		return kernelFailure.Code
	}
	return ""
}

func decodeClosed(arguments []byte, target any) error {
	trimmed := bytes.TrimSpace(arguments)
	if len(trimmed) < 2 || trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' {
		return errors.New("arguments must be one JSON object")
	}
	return json.Unmarshal(arguments, target, json.RejectUnknownMembers(true))
}

func validTask(task string) bool {
	return len(task) > 0 && len(task) <= maxTaskBytes && utf8.ValidString(task)
}

func validRelativePath(value string) bool {
	if value == "" || utf8.RuneCountInString(value) > maxPathRunes || !utf8.ValidString(value) ||
		filepath.IsAbs(value) || filepath.ToSlash(filepath.Clean(value)) != value || strings.Contains(value, "\\") {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
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

// malformedArguments reports a well-formed JSON-RPC call whose tool arguments
// do not match the docs tool's argument shape (wrong field type, unknown
// member, or invalid task/draft content). SDD-V0-006 reserves the JSON-RPC
// InvalidParams protocol error for exactly an invalid/escaping path or an
// unsupported tool name; every other business-level refusal, malformed
// arguments included, must be an MCP tool result with isError: true.
func malformedArguments() *ToolFailure {
	return &ToolFailure{Code: "invalid-arguments", Message: "arguments do not match the tool's input schema"}
}
