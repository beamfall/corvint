package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/contextsummary"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// Experimental evidence summaries and handle expansion (ESV-V0-008..010,
// TCP-V0-024, decision 0364): two opt-in consumers of the unchanged context
// wire. Neither alters the packet the default invocation prints.
const (
	contextExpandView          = "experimental-evidence-expand/0"
	contextSummaryDefaultBytes = contextsummary.DefaultBytes
	contextSummaryMinBytes     = contextsummary.MinBytes
	contextExpandDefaultBytes  = 65536
	contextHandleLimit         = 4352
	contextHandlePrefix        = contextsummary.HandlePrefix
)

var contextHandleRange = regexp.MustCompile(`^([1-9][0-9]{0,8})-([1-9][0-9]{0,8})$`)

// summarizeContextPacket projects the exact default stdout bytes of one
// context invocation into a summary of at most budget bytes including its LF
// (ESV-V0-008); the projection is contextsummary's, shared with MCP.
func summarizeContextPacket(raw []byte, budget int) ([]byte, error) {
	return contextsummary.Summarize(raw, budget, contextsummary.CLIContinuation)
}

// contextHandle is one parsed `cv1:TREE:BLOB:RANGE:PATH` evidence handle.
type contextHandle struct {
	tree, blob, rangeText, path string
	first, last                 int
}

func contextHandleError(code, message string) error {
	return &gokernel.Error{Code: code, Message: message}
}

// parseContextHandle validates the handle's grammar before any repository read
// (ESV-V0-009): the tree is a full object ID, the blob a full or abbreviated
// (at least seven hex digits) object ID, the range `all` or START-END one-based
// inclusive, and the path the source view's safe relative POSIX path.
func parseContextHandle(text string) (contextHandle, error) {
	invalid := func(reason string) (contextHandle, error) {
		return contextHandle{}, contextHandleError("invalid-handle", reason)
	}
	if len(text) > contextHandleLimit || !utf8.ValidString(text) {
		return invalid(fmt.Sprintf("a handle is valid UTF-8 of at most %d bytes", contextHandleLimit))
	}
	body, ok := strings.CutPrefix(text, contextHandlePrefix)
	fields := strings.SplitN(body, ":", 4)
	if !ok || len(fields) != 4 {
		return invalid("a handle has the form cv1:TREE:BLOB:RANGE:PATH")
	}
	handle := contextHandle{tree: fields[0], blob: fields[1], rangeText: fields[2], path: fields[3]}
	if !sourceHex(handle.tree, 40) && !sourceHex(handle.tree, 64) {
		return invalid("TREE must be a full lower-case hex object ID")
	}
	if len(handle.blob) < 7 || len(handle.blob) > 64 || strings.Trim(handle.blob, "0123456789abcdef") != "" {
		return invalid("BLOB must be at least seven lower-case hex digits of an object ID")
	}
	if !sourceSafePath(handle.path) {
		return invalid("PATH must be a relative POSIX path without empty, dot or dot-dot components, backslashes, controls or a leading dash")
	}
	if handle.rangeText == "all" {
		return handle, nil
	}
	match := contextHandleRange.FindStringSubmatch(handle.rangeText)
	if match == nil {
		return invalid("RANGE must be all or START-END with one-based line numbers of at most nine digits")
	}
	handle.first, _ = strconv.Atoi(match[1])
	handle.last, _ = strconv.Atoi(match[2])
	if handle.first > handle.last {
		return invalid("RANGE start must not exceed its end")
	}
	return handle, nil
}

// expandContextHandle returns the handle's exact immutable source bytes from
// Git objects, never the worktree, after verifying the pinned tree is still
// HEAD's tree and the bytes recompute the blob's native object ID. Every
// failure refuses whole; current content is never substituted.
func expandContextHandle(ctx context.Context, root, text string, maxBytes int) (map[string]any, error) {
	if maxBytes < 1 || maxBytes > sourceBlobLimit {
		return nil, contextHandleError("expand-budget", fmt.Sprintf("--max-bytes must be an integer from 1 to %d", sourceBlobLimit))
	}
	handle, err := parseContextHandle(text)
	if err != nil {
		return nil, err
	}
	deadline, cancel := context.WithTimeout(ctx, sourceDeadline(ctx))
	defer cancel()
	result, err := expandPinnedHandle(deadline, root, text, handle, maxBytes)
	if err != nil && deadline.Err() != nil {
		// A Git read cut short by the bound fails like a missing or invalid
		// object; name the cause instead (ESV-V0-013, V1-1412).
		cause := fmt.Sprintf("the %s source-read bound expired before the expansion finished", sourceDeadline(ctx))
		if errors.Is(deadline.Err(), context.Canceled) {
			cause = "the expansion was cancelled before it finished"
		}
		return nil, contextHandleError("deadline-exceeded", cause)
	}
	return result, err
}

// expandPinnedHandle runs expandContextHandle's Git reads under its deadline.
func expandPinnedHandle(deadline context.Context, root, text string, handle contextHandle, maxBytes int) (map[string]any, error) {
	if !sourceIsToplevel(deadline, root) {
		return nil, contextHandleError("repository-root", "the root is not its Git toplevel")
	}
	format, _ := sourceGitText(deadline, root, 8192, "rev-parse", "--show-object-format")
	width := sourceObjectWidths[format]
	if width == 0 || len(handle.tree) != width || len(handle.blob) > width {
		return nil, contextHandleError("invalid-handle", "the handle's object IDs do not have this repository's object format width")
	}
	if err := contextHandleCurrent(deadline, root, handle.tree); err != nil {
		return nil, err
	}
	entry, err := sourceGit(deadline, root, 8192, "ls-tree", "-z", handle.tree, "--", handle.path)
	if err != nil || len(entry) == 0 {
		return nil, contextHandleError("missing-handle", "the pinned tree has no entry at the handle's path")
	}
	parts := bytes.Split(bytes.TrimSuffix(entry, []byte{0}), []byte{'\t'})
	fields := bytes.Fields(parts[0])
	if len(parts) != 2 || len(fields) != 3 || string(parts[1]) != handle.path {
		return nil, contextHandleError("ambiguous-handle", "the pinned tree does not list exactly one entry at the handle's path")
	}
	if string(fields[0]) != "100644" && string(fields[0]) != "100755" || string(fields[1]) != "blob" {
		return nil, contextHandleError("invalid-handle", fmt.Sprintf("the handle's path is a %s (mode %s) in the pinned tree; only a regular file blob expands", contextEntryKind(string(fields[0])), fields[0]))
	}
	blob := string(fields[2])
	candidates, err := sourceGitText(deadline, root, 8192, "rev-parse", "--disambiguate="+handle.blob)
	if err != nil {
		return nil, contextHandleError("git-failed", "cannot resolve the handle's blob identity")
	}
	matches := strings.Fields(candidates)
	if len(matches) == 0 {
		return nil, contextHandleError("missing-handle", "no object in the repository has the handle's blob identity")
	}
	if len(matches) > 1 {
		return nil, contextHandleError("ambiguous-handle", fmt.Sprintf("the abbreviated blob identity names %d objects; give more digits", len(matches)))
	}
	if matches[0] != blob {
		return nil, contextHandleError("invalid-handle", "the handle's blob is not the pinned tree's blob at its path")
	}
	source, err := sourceGit(deadline, root, sourceBlobLimit+1, "cat-file", "blob", blob)
	if errors.Is(err, errSourceOutputOverflow) || err == nil && len(source) > sourceBlobLimit {
		return nil, contextHandleError("expand-budget", fmt.Sprintf("the blob is larger than %d bytes", sourceBlobLimit))
	}
	if err != nil {
		return nil, contextHandleError("git-failed", "cannot read the handle's blob")
	}
	if sourceObjectID(format, source) != blob {
		return nil, contextHandleError("source-digest-mismatch", "the read bytes do not recompute the blob's object ID")
	}
	selection, err := contextHandleSelection(handle, source, maxBytes)
	if err != nil {
		return nil, err
	}
	if err := contextHandleCurrent(deadline, root, handle.tree); err != nil {
		return nil, err
	}
	sourceSum := sha256.Sum256(source)
	return map[string]any{
		"schema_version": 1, "tool": "context", "ok": true, "mutates": false, "view": contextExpandView,
		"handle":    text,
		"source":    map[string]any{"object_format": format, "tree": handle.tree, "path": handle.path, "blob": blob, "blob_verified": true, "bytes": len(source), "sha256": hex.EncodeToString(sourceSum[:])},
		"selection": selection,
	}, nil
}

// contextEntryKinds names the tree entries a summary row can point at but
// --expand cannot open: the packet carries no mode, so a symlink row keeps its
// handle and its expansion says why it refuses (V1-0156).
var contextEntryKinds = map[string]string{"120000": "symbolic link", "160000": "gitlink", "040000": "directory"}

func contextEntryKind(mode string) string {
	if kind, ok := contextEntryKinds[mode]; ok {
		return kind
	}
	return "non-regular entry"
}

// contextHandleCurrent refuses a handle pinned to a tree other than HEAD's.
func contextHandleCurrent(ctx context.Context, root, tree string) error {
	current, err := sourceGitText(ctx, root, 8192, "rev-parse", "--verify", "HEAD^{tree}")
	if err != nil {
		return contextHandleError("git-failed", "cannot read HEAD's tree")
	}
	if current != tree {
		return contextHandleError("stale-handle", "the handle is pinned to tree "+tree+" but HEAD's tree is "+current+"; rerun context for a current handle")
	}
	return nil
}

// contextHandleSelection cuts the handle's exact line range out of source.
func contextHandleSelection(handle contextHandle, source []byte, maxBytes int) (map[string]any, error) {
	lines := bytes.SplitAfter(source, []byte{'\n'})
	if len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	first, last := 1, len(lines)
	if handle.rangeText != "all" {
		first, last = handle.first, handle.last
	}
	if last > len(lines) {
		return nil, contextHandleError("invalid-handle", fmt.Sprintf("line range %s exceeds the blob's %d lines", handle.rangeText, len(lines)))
	}
	byteStart := 0
	for _, line := range lines[:first-1] {
		byteStart += len(line)
	}
	selected := bytes.Join(lines[first-1:last], nil)
	if len(selected) > maxBytes {
		return nil, contextHandleError("expand-budget", fmt.Sprintf("the selection is %d bytes, over --max-bytes %d; narrow RANGE or raise --max-bytes up to %d", len(selected), maxBytes, sourceBlobLimit))
	}
	if !utf8.Valid(selected) {
		return nil, contextHandleError("unsupported-text", "the selected bytes are not UTF-8 and cannot be returned exactly as JSON text")
	}
	sum := sha256.Sum256(selected)
	text, redacted := screenSourceLines(source, lines, first, last)
	selection := map[string]any{
		"range": handle.rangeText, "complete": byteStart == 0 && len(selected) == len(source),
		"line_start": first, "line_end": last, "byte_start": byteStart, "byte_end": byteStart + len(selected),
		"bytes": len(selected), "sha256": hex.EncodeToString(sum[:]), "text": text,
	}
	if redacted != nil {
		selection["redacted_lines"] = redacted
	}
	return selection, nil
}

// runContextExpand prints one expansion or refuses on stderr with nothing on stdout.
func runContextExpand(ctx context.Context, options taskContextOptions, stdout, stderr io.Writer) int {
	result, err := expandContextHandle(ctx, options.root, options.expand, options.maxBytes)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	encoded, err := gokernel.CanonicalJSON(result)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if _, err := stdout.Write(append(encoded, '\n')); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write the expanded evidence"})
		return 2
	}
	return 0
}
