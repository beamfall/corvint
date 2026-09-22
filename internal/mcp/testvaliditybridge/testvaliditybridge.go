// Package testvaliditybridge exposes the shared test-validity projection as
// the single read-only tool of the experimental MCP test-validity profile
// (docs/specs/mcp-test-validity-profile-v0.md). It calls exactly the builder
// `corvint test-validity` calls (internal/testvaliditydoc); it never shells
// out, never runs a test, and never duplicates projection logic.
package testvaliditybridge

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

const (
	ToolTestValidity = "corvint.test_validity"

	resultSchema = "corvint-mcp-test-validity-result/0"
	// MaxResultBytes bounds the canonical result so the text block and the
	// duplicated structuredContent fit one 1 MiB MCP frame.
	MaxResultBytes  = 256 << 10
	maxArgumentSize = 4 << 10
	maxPathRunes    = 1024
	maxGitFileBytes = 4 << 10
)

// Error is a closed, sanitized transport-level failure. It never carries a
// repository path or file content.
type Error struct{ Code string }

func (failure *Error) Error() string { return failure.Code }

// ToolFailure is a business-level refusal, reported as an MCP tool error
// (isError: true), never as a successful call.
type ToolFailure struct{ Code, Message string }

// Registry binds every call to one canonical local repository root.
type Registry struct {
	root         string
	rootIdentity os.FileInfo
	gitIdentity  os.FileInfo
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
	gitMarker, ok := worktreeMarker(resolved)
	if !ok {
		return nil, failure("invalid-root")
	}
	return &Registry{root: resolved, rootIdentity: rootIdentity, gitIdentity: gitMarker}, nil
}

// Tools is the exact delivered surface: one tool.
func (registry *Registry) Tools() []bridge.ToolDescriptor {
	if registry == nil {
		return nil
	}
	return []bridge.ToolDescriptor{{
		Name:        ToolTestValidity,
		Description: "Project the test-level results of one corvint-js-test-provider document at a repository-relative path, or with discover:true the newest retained provider document under .corvint/test-evidence, through the shared five-axis test-validity shape, identical to `corvint test-validity --receipt FILE` or `--discover`; with neither every axis is UNSUPPORTED (docs/specs/mcp-test-validity-profile-v0.md, experimental).",
		Annotations: bridge.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: false, IdempotentHint: true, OpenWorldHint: false},
		InputSchema: map[string]any{
			"$schema": "https://json-schema.org/draft/2020-12/schema",
			"type":    "object",
			"properties": map[string]any{
				"receipt": map[string]any{
					"type": "string", "minLength": 1, "maxLength": maxPathRunes,
					"pattern": `^(?!/)(?!.*(?:^|/)[.]{1,2}(?:/|$))(?!.*(?:^|/)[.]git(?:/|$))(?!.*//)(?!.*\\).+$`,
				},
				"discover": map[string]any{"type": "boolean"},
			},
			"required": []any{}, "additionalProperties": false,
		},
	}}
}

// Receipt stays raw so a present null is distinguishable from an absent member.
type input struct {
	Receipt  jsontext.Value `json:"receipt"`
	Discover jsontext.Value `json:"discover"`
}

// Call dispatches one tool by name. It returns exactly one of: a successful
// structured/text pair, a ToolFailure, or a transport Error.
func (registry *Registry) Call(ctx context.Context, name string, arguments []byte) (map[string]any, string, *ToolFailure, *Error) {
	if registry == nil || registry.root == "" || registry.rootIdentity == nil || registry.gitIdentity == nil {
		return nil, "", nil, failure("invalid-registry")
	}
	if ctx.Err() != nil {
		return nil, "", nil, failure("cancelled")
	}
	if name != ToolTestValidity {
		return nil, "", nil, failure("unsupported-tool")
	}
	if len(arguments) == 0 || len(arguments) > maxArgumentSize || !utf8.Valid(arguments) {
		return nil, "", nil, failure("invalid-arguments")
	}
	receipt, discover, err := decodeArguments(arguments)
	if err != nil || discover && receipt != nil {
		return nil, "", &ToolFailure{Code: "invalid-arguments", Message: "arguments do not match the tool's input schema"}, nil
	}
	if receipt != nil && !validReceiptPath(*receipt) {
		return nil, "", nil, failure("invalid-arguments")
	}
	if !registry.sameRoot() {
		return nil, "", nil, failure("repository-unavailable")
	}
	if discover {
		return registry.discover()
	}
	document := testvaliditydoc.Unsupported()
	var receiptField any
	if receipt != nil {
		projected, refusal := registry.project(*receipt)
		if refusal != nil {
			return nil, "", refusal, nil
		}
		document, receiptField = projected, *receipt
	}
	return result(document, receiptField)
}

// discover runs the shared retained-evidence discovery (MTV-V0-009,
// LPCV-V0-053) on the pinned worktree root.
func (registry *Registry) discover() (map[string]any, string, *ToolFailure, *Error) {
	document, err := testvaliditydoc.Discover(registry.root)
	var refusal *testvaliditydoc.DiscoveryError
	if errors.As(err, &refusal) {
		return nil, "", &ToolFailure{Code: refusal.Code, Message: refusal.Message}, nil
	}
	if err != nil {
		return nil, "", nil, failure("internal-error")
	}
	return result(document, nil)
}

func (registry *Registry) project(relative string) (testvaliditydoc.Document, *ToolFailure) {
	data, refusal := registry.readConfined(relative)
	if refusal != nil {
		return testvaliditydoc.Document{}, refusal
	}
	receipt, err := testvaliditydoc.Decode(data)
	if err != nil {
		// The decode error can quote receipt member names, which are repository
		// text; this unframed message stays fixed (MCPV0-016). The CLI keeps the detail.
		return testvaliditydoc.Document{}, &ToolFailure{Code: "invalid-test-validity-receipt", Message: "receipt is not an accepted test-validity provider document"}
	}
	return testvaliditydoc.Project(receipt), nil
}

// readConfined reads at most MaxInputBytes of one regular file whose fully
// resolved path stays inside the repository root and outside its .git.
func (registry *Registry) readConfined(relative string) ([]byte, *ToolFailure) {
	unreadable := &ToolFailure{Code: "invalid-test-validity-receipt", Message: "receipt is not a readable regular file"}
	resolved, err := filepath.EvalSymlinks(filepath.Join(registry.root, filepath.FromSlash(relative)))
	if err != nil {
		return nil, unreadable
	}
	inside, err := filepath.Rel(registry.root, resolved)
	// The root itself is inside the worktree but never a regular file (MTV-V0-003).
	if inside == "." {
		return nil, unreadable
	}
	if err != nil || !validReceiptPath(filepath.ToSlash(inside)) || namesGitDirectory(inside) {
		return nil, &ToolFailure{Code: "receipt-outside-repository", Message: "receipt resolves outside the repository worktree"}
	}
	return registry.readResolved(inside)
}

// namesGitDirectory reports a resolved segment that a case-insensitive
// filesystem would open as .git, such as ".GIT" on default Darwin volumes.
func namesGitDirectory(inside string) bool {
	for _, part := range strings.Split(filepath.ToSlash(inside), "/") {
		if strings.EqualFold(part, ".git") {
			return true
		}
	}
	return false
}

func (registry *Registry) readResolved(inside string) ([]byte, *ToolFailure) {
	unreadable := &ToolFailure{Code: "invalid-test-validity-receipt", Message: "receipt is not a readable regular file"}
	root, err := os.OpenRoot(registry.root)
	if err != nil {
		return nil, unreadable
	}
	defer root.Close()
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(registry.rootIdentity, opened) {
		return nil, unreadable
	}
	data, err := testvaliditydoc.ReadFile(root, inside)
	if errors.Is(err, testvaliditydoc.ErrInputTooLarge) {
		return nil, &ToolFailure{Code: "invalid-test-validity-receipt", Message: "receipt exceeds 4 MiB"}
	}
	if err != nil {
		return nil, unreadable
	}
	return data, nil
}

func result(document testvaliditydoc.Document, receipt any) (map[string]any, string, *ToolFailure, *Error) {
	raw, err := json.Marshal(document)
	if err != nil {
		return nil, "", nil, failure("internal-error")
	}
	var documentObject map[string]any
	if err := json.Unmarshal(raw, &documentObject); err != nil {
		return nil, "", nil, failure("internal-error")
	}
	object := map[string]any{
		"schema": resultSchema, "tool": ToolTestValidity, "mutates": false,
		"receipt": receipt, "document": documentObject,
	}
	encoded, err := gokernel.CanonicalJSON(object)
	if err != nil {
		return nil, "", nil, failure("internal-error")
	}
	if len(encoded) > MaxResultBytes {
		return nil, "", &ToolFailure{Code: "test-validity-limit-exceeded", Message: "projected result exceeds 256 KiB"}, nil
	}
	return object, string(encoded), nil, nil
}

func (registry *Registry) sameRoot() bool {
	current, err := os.Stat(registry.root)
	if err != nil || !current.IsDir() || !os.SameFile(registry.rootIdentity, current) {
		return false
	}
	gitMarker, ok := worktreeMarker(registry.root)
	return ok && os.SameFile(registry.gitIdentity, gitMarker)
}

// worktreeMarker returns root's non-symlink .git entry when it is a directory,
// or a regular linked-worktree file whose `gitdir:` names an existing
// directory (MTV-V0-003).
func worktreeMarker(root string) (os.FileInfo, bool) {
	marker, err := os.Lstat(filepath.Join(root, ".git"))
	if err != nil {
		return nil, false
	}
	if marker.IsDir() {
		return marker, true
	}
	return marker, marker.Mode().IsRegular() && linkedGitDirectoryExists(root)
}

// linkedGitDirectoryExists reads the .git file through the no-follow,
// nonblocking receipt reader, so a swapped symlink or FIFO cannot redirect or
// stall it.
func linkedGitDirectoryExists(root string) bool {
	worktree, err := os.OpenRoot(root)
	if err != nil {
		return false
	}
	defer worktree.Close()
	data, err := testvaliditydoc.ReadFile(worktree, ".git")
	target, found := strings.CutPrefix(strings.TrimRight(string(data), "\r\n"), "gitdir: ")
	if err != nil || len(data) > maxGitFileBytes || !found || target == "" {
		return false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	info, err := os.Stat(target)
	return err == nil && info.IsDir()
}

// decodeArguments closes the input schema: member names match exactly (no
// case folding), unknown or duplicate members fail, a present receipt must be
// a JSON string and a present discover a JSON boolean, so null is a wrong type
// rather than an absent member.
func decodeArguments(arguments []byte) (*string, bool, error) {
	trimmed := bytes.TrimSpace(arguments)
	if len(trimmed) < 2 || trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' {
		return nil, false, errors.New("arguments must be one JSON object")
	}
	var request input
	if err := jsonv2.Unmarshal(trimmed, &request, jsonv2.RejectUnknownMembers(true)); err != nil {
		return nil, false, err
	}
	discover := false
	if request.Discover != nil {
		if kind := request.Discover.Kind(); kind != 't' && kind != 'f' {
			return nil, false, errors.New("discover must be a boolean")
		}
		discover = request.Discover.Kind() == 't'
	}
	if request.Receipt == nil {
		return nil, discover, nil
	}
	if request.Receipt.Kind() != '"' {
		return nil, false, errors.New("receipt must be a string")
	}
	var receipt string
	if err := jsonv2.Unmarshal(request.Receipt, &receipt); err != nil {
		return nil, false, err
	}
	return &receipt, discover, nil
}

// validReceiptPath admits a clean, relative, slash-separated path with no
// empty, dot, dot-dot, or .git segment and no control character.
func validReceiptPath(value string) bool {
	if value == "" || utf8.RuneCountInString(value) > maxPathRunes || !utf8.ValidString(value) ||
		filepath.IsAbs(value) || strings.HasPrefix(value, "/") || strings.Contains(value, "\\") {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." || part == ".git" {
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

func failure(code string) *Error { return &Error{Code: code} }
