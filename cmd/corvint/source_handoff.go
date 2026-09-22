package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

const (
	sourcePacketLimit = 65536
	sourceBlobLimit   = 1048576
	sourceViewLimit   = 16384
	sourceOutputLimit = 524288
	sourceLeadLimit   = 4096
	// sourceArgumentLimit bounds each CLI argument in UTF-8 bytes.
	sourceArgumentLimit = 4096
	sourceGitPath       = "/usr/bin/git"
	// sourceShutdownLimit bounds TERM, KILL, reap and stream drain for one
	// stopped Git command (procgroup.Spec.ShutdownTimeout).
	sourceShutdownLimit = 1500 * time.Millisecond
)

var (
	sourceGitOptions     = []string{"--no-pager", "--no-replace-objects", "--literal-pathspecs", "-c", "core.fsmonitor=false", "-c", "core.hooksPath=/dev/null"}
	sourceGitEnvironment = []string{"PATH=/usr/bin:/bin", "HOME=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_NO_LAZY_FETCH=1", "GIT_OPTIONAL_LOCKS=0", "LC_ALL=C"}
	sourceFencePattern   = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")
	sourceHTMLPattern    = regexp.MustCompile(`^\s*<(?:/?[A-Za-z]|[!?])`)
)

// sourceGitExecutableKey carries an in-process test substitute for
// /usr/bin/git through the context; no command-line path sets it.
type sourceGitExecutableKey struct{}

type sourceRefusal string

func (e sourceRefusal) Error() string { return string(e) }

func sourceRequire(ok bool, code string) error {
	if !ok {
		return sourceRefusal(code)
	}
	return nil
}

func runClaudeSourceHandoff(ctx context.Context, arguments []string, payload map[string]any) map[string]any {
	if len(arguments) != 2 || arguments[0] != "--packet-out" {
		return degradedAdapterOutput("invalid-source-handoff-arguments")
	}
	root := adapterGetenv(ctx, "CLAUDE_PROJECT_DIR")
	if root == "" {
		root, _ = os.Getwd()
	}
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return degradedAdapterOutput("project-root-unavailable")
	}
	normalized, reason := normalizeAdapterInput("claude-code", "user-prompt", payload, root)
	if reason != "" {
		return degradedAdapterOutput(reason)
	}
	lead, err := captureSourceHandoff(ctx, root, normalized["task"].(string), arguments[1])
	if err != nil {
		return renderClaudeContext("user-prompt", "NOT_PRODUCED "+sourceErrorCode(err), claudeGuidance(root, normalized["sessionIdSha256"].(string)))
	}
	if _, reason := invokeDogfoodEvent(ctx, root, "claude-code", "user-prompt", normalized, adapterOutputLimit); reason != "" {
		return degradedAdapterOutput(reason)
	}
	return renderClaudeContext("user-prompt", string(lead), claudeGuidance(root, normalized["sessionIdSha256"].(string)))
}

func captureSourceHandoff(ctx context.Context, root, task, destination string) ([]byte, error) {
	deadline, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	commit, err := sourceGitText(deadline, root, 8192, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	tree, err := sourceGitText(deadline, root, 8192, "rev-parse", commit+"^{tree}")
	if err != nil {
		return nil, err
	}
	common, err := sourceGitText(deadline, root, 8192, "rev-parse", "--git-common-dir")
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(root, common)
	}
	common, err = filepath.EvalSymlinks(common)
	if err != nil {
		return nil, sourceRefusal("repository-root")
	}
	if !filepath.IsAbs(destination) {
		return nil, sourceRefusal("packet-output-path")
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(destination))
	if err != nil {
		return nil, sourceRefusal("packet-output-path")
	}
	destination = filepath.Join(parent, filepath.Base(destination))
	if pathWithin(root, destination) || pathWithin(common, destination) {
		return nil, sourceRefusal("packet-output-in-repository")
	}

	var stdout, stderr bytes.Buffer
	status := runContext(deadline, []string{"--root", root, "context", "--task", task, "--limit", "8"}, bytes.NewReader(nil), &stdout, &stderr)
	if status != 0 || stdout.Len() > sourcePacketLimit {
		return nil, sourceRefusal("context-capture-failed")
	}
	raw := append([]byte(nil), stdout.Bytes()...)
	packet, err := decodeAdapterJSON(raw)
	if err != nil {
		return nil, sourceRefusal("packet-shape")
	}
	selectors, err := sourceSelectors(packet)
	if err != nil {
		return nil, err
	}
	if revision, _ := packet["revision"].(string); revision != tree {
		return nil, sourceRefusal("stale-tree")
	}
	sum := sha256.Sum256(raw)
	lead := map[string]any{
		"profile": "experimental-automatic-source-handoff/0", "tool": "context", "state": packet["state"],
		"revision": tree, "commit": commit, "packet": destination, "packet_sha256": hex.EncodeToString(sum[:]),
		"consumer": []string{"corvint", "adapter", "source-view"}, "coverage": packet["coverage"],
		"selectors": selectors, "omitted_selectors": 0, "metadata": "Complete raw packet in packet file; other fields omitted here.",
		"task_evidence_complete": "UNKNOWN", "capture_mutates": "caller-selected external packet only",
	}
	for {
		encoded, err := json.Marshal(lead)
		if err != nil {
			return nil, err
		}
		if len(encoded)+1 <= sourceLeadLimit {
			if len(selectors) == 0 {
				return nil, sourceRefusal("handoff-budget")
			}
			if strings.Contains(string(encoded), task) {
				return nil, sourceRefusal("context-prompt-echo")
			}
			current, gitErr := sourceGitText(deadline, root, 8192, "rev-parse", "HEAD")
			if gitErr != nil || current != commit {
				return nil, sourceRefusal("stale-commit")
			}
			stream, openErr := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if openErr != nil {
				return nil, sourceRefusal("packet-output-exists")
			}
			_, writeErr := stream.Write(raw)
			closeErr := stream.Close()
			if writeErr != nil || closeErr != nil {
				_ = os.Remove(destination)
				return nil, sourceRefusal("packet-publication-failed")
			}
			return append(encoded, '\n'), nil
		}
		selectors = selectors[:len(selectors)-1]
		lead["selectors"] = selectors
		lead["omitted_selectors"] = lead["omitted_selectors"].(int) + 1
	}
}

func sourceSelectors(packet map[string]any) ([]map[string]any, error) {
	results, ok := packet["results"].([]any)
	if !ok || len(results) == 0 || len(results) > 50 {
		return nil, sourceRefusal("packet-results")
	}
	selectors := []map[string]any{}
	for resultIndex, rawResult := range results {
		result, ok := rawResult.(map[string]any)
		if !ok {
			return nil, sourceRefusal("packet-evidence")
		}
		evidence, ok := result["evidence"].([]any)
		if !ok || len(selectors)+len(evidence) > 256 {
			return nil, sourceRefusal("packet-evidence-budget")
		}
		for evidenceIndex, rawHandle := range evidence {
			handle, ok := rawHandle.(map[string]any)
			if !ok || sourcePositiveInt(handle["line"]) == 0 || !sourceSafePath(handle["path"]) {
				return nil, sourceRefusal("packet-handle")
			}
			row := map[string]any{"result": resultIndex, "evidence": evidenceIndex, "path": handle["path"], "line": sourcePositiveInt(handle["line"])}
			for _, key := range []string{"blob_hash", "reason", "authority", "confidence"} {
				row[key] = handle[key]
			}
			row["kind"], row["action"] = result["kind"], result["action"]
			selectors = append(selectors, row)
		}
	}
	return selectors, nil
}

type sourceViewOptions struct {
	root, packet, digest, commit, lines, requirement string
	result, evidence, maxBytes                       int
}

func runSourceViewAdapter(ctx context.Context, arguments []string, stdout io.Writer) int {
	options, err := parseSourceViewOptions(arguments)
	result := map[string]any{"profile": "experimental-source-view/0", "ok": false, "mutates": false, "selector_complete": false, "task_evidence_complete": "UNKNOWN", "packet_metadata": nil}
	if err == nil {
		result, err = executeSourceView(ctx, options, result)
	}
	if err != nil {
		result["ok"], result["selector_complete"], result["error"] = false, false, map[string]any{"code": sourceErrorCode(err)}
		delete(result, "view")
	}
	raw, marshalErr := json.Marshal(result)
	if marshalErr != nil || len(raw)+1 > sourceOutputLimit {
		raw = []byte(`{"profile":"experimental-source-view/0","ok":false,"mutates":false,"selector_complete":false,"error":{"code":"output-budget"}}`)
	}
	enveloped, frameErr := repoenvelope.Frame(string(raw))
	if frameErr != nil {
		// AHI-004: repository data that carries the envelope terminator is
		// refused whole, never emitted, so no view or packet metadata leaks.
		enveloped, _ = repoenvelope.Frame(`{"profile":"experimental-source-view/0","ok":false,"mutates":false,"selector_complete":false,"error":{"code":"` + repoenvelope.CollisionCode + `"}}`)
	}
	_, _ = stdout.Write(append([]byte(enveloped), '\n'))
	if result["ok"] == true && frameErr == nil {
		return 0
	}
	return 1
}

func parseSourceViewOptions(arguments []string) (sourceViewOptions, error) {
	o := sourceViewOptions{maxBytes: sourceViewLimit, result: -1, evidence: -1}
	values := map[string]*string{"--root": &o.root, "--packet": &o.packet, "--packet-sha256": &o.digest, "--commit": &o.commit, "--lines": &o.lines, "--requirement": &o.requirement}
	ints := map[string]*int{"--result": &o.result, "--evidence": &o.evidence, "--max-bytes": &o.maxBytes}
	for len(arguments) > 0 {
		if len(arguments) < 2 {
			return o, sourceRefusal("malformed-input")
		}
		key, value := arguments[0], arguments[1]
		if len(key) > sourceArgumentLimit || len(value) > sourceArgumentLimit {
			return o, sourceRefusal("malformed-input")
		}
		if target := values[key]; target != nil {
			*target = value
		} else if target := ints[key]; target != nil {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return o, sourceRefusal("malformed-input")
			}
			*target = parsed
		} else {
			return o, sourceRefusal("malformed-input")
		}
		arguments = arguments[2:]
	}
	if o.root == "" || o.packet == "" || o.commit == "" || o.result < 0 || o.evidence < 0 || (o.lines == "") == (o.requirement == "") {
		return o, sourceRefusal("malformed-input")
	}
	return o, nil
}

func executeSourceView(ctx context.Context, o sourceViewOptions, result map[string]any) (map[string]any, error) {
	result["selection"] = map[string]any{"result": o.result, "evidence": o.evidence, "lines": nullableSourceString(o.lines), "requirement": nullableSourceString(o.requirement)}
	result["fallback"] = map[string]any{"argv": nil, "reason": "no-safe-immutable-handle"}
	if err := sourceRequire(o.maxBytes >= 1 && o.maxBytes <= sourceViewLimit, "view-budget"); err != nil {
		return result, err
	}
	file, err := openBoundedRegularFile(o.packet)
	if err != nil {
		return result, sourceRefusal("packet-file-type")
	}
	raw, err := io.ReadAll(io.LimitReader(file, sourcePacketLimit+1))
	_ = file.Close()
	if err != nil {
		return result, sourceRefusal("packet-file-type")
	}
	if len(raw) > sourcePacketLimit {
		return result, sourceRefusal("packet-budget")
	}
	return executeSourceViewBytes(ctx, o, result, raw)
}

// The Pi tool keeps the original packet in memory; file and memory routes use
// exactly the same immutable selector validation.
func executeSourceViewBytes(ctx context.Context, o sourceViewOptions, result map[string]any, raw []byte) (map[string]any, error) {
	result["selection"] = map[string]any{"result": o.result, "evidence": o.evidence, "lines": nullableSourceString(o.lines), "requirement": nullableSourceString(o.requirement)}
	result["fallback"] = map[string]any{"argv": nil, "reason": "no-safe-immutable-handle"}
	if o.result < 0 || o.evidence < 0 || o.maxBytes < 1 || o.maxBytes > sourceViewLimit || len(raw) > sourcePacketLimit {
		return result, sourceRefusal("malformed-input")
	}
	packet, err := decodeAdapterJSON(raw)
	if err != nil {
		return result, sourceRefusal("packet-shape")
	}
	result["packet_metadata"] = packet
	sum := sha256.Sum256(raw)
	packetDigest := hex.EncodeToString(sum[:])
	result["packet_sha256"] = packetDigest
	handle, err := selectSourceHandle(packet, o.result, o.evidence)
	if err != nil {
		return result, err
	}
	path, _ := handle["path"].(string)
	if !sourceSafePath(path) {
		return result, sourceRefusal("unsafe-path")
	}
	root, err := filepath.EvalSymlinks(o.root)
	if err != nil {
		return result, sourceRefusal("repository-root")
	}
	if !sourceHex(o.commit, 40) && !sourceHex(o.commit, 64) {
		return result, sourceRefusal("object-identity")
	}
	fallbackArgv := append(append([]string{sourceGitPath}, sourceGitOptions...), "-C", root, "show", o.commit+":"+path)
	result["fallback"] = map[string]any{"argv": fallbackArgv, "environment": sourceGitEnvironment, "reason": "raw-immutable-source"}
	if o.digest != "" && (!sourceHex(o.digest, 64) || o.digest != packetDigest) {
		return result, sourceRefusal("packet-digest-mismatch")
	}
	deadline, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if !sourceIsToplevel(deadline, root) {
		return result, sourceRefusal("repository-root")
	}
	format, _ := sourceGitText(deadline, root, 8192, "rev-parse", "--show-object-format")
	width := sourceObjectWidths[format]
	if width == 0 {
		return result, sourceRefusal("object-format")
	}
	revision, _ := packet["revision"].(string)
	blobHash, _ := handle["blob_hash"].(string)
	for _, oid := range []string{o.commit, revision, blobHash} {
		if !sourceHex(oid, width) {
			return result, sourceRefusal("object-identity")
		}
	}
	current, err := sourceGitText(deadline, root, 8192, "rev-parse", "HEAD")
	if err != nil || current != o.commit {
		return result, sourceRefusal("stale-commit")
	}
	tree, err := sourceGitText(deadline, root, 8192, "rev-parse", o.commit+"^{tree}")
	if err != nil || tree != packet["revision"] {
		return result, sourceRefusal("stale-tree")
	}
	entry, err := sourceGit(deadline, root, 8192, "ls-tree", "-z", tree, "--", path)
	if err != nil || len(entry) == 0 {
		return result, sourceRefusal("missing-path")
	}
	parts := bytes.Split(bytes.TrimSuffix(entry, []byte{0}), []byte{'\t'})
	fields := bytes.Fields(parts[0])
	if len(parts) != 2 || len(fields) != 3 || string(parts[1]) != path {
		return result, sourceRefusal("ambiguous-path")
	}
	if string(fields[0]) != "100644" && string(fields[0]) != "100755" || string(fields[1]) != "blob" {
		return result, sourceRefusal("nonregular-path")
	}
	blob := string(fields[2])
	if blob != handle["blob_hash"] {
		return result, sourceRefusal("stale-blob")
	}
	source, err := sourceGit(deadline, root, sourceBlobLimit+1, "cat-file", "blob", blob)
	if err != nil || len(source) > sourceBlobLimit {
		return result, sourceRefusal("blob-budget")
	}
	if sourceObjectID(format, source) != blob {
		return result, sourceRefusal("stale-blob")
	}
	view, err := extractSourceView(source, handle, o.lines, o.requirement, o.maxBytes)
	if err != nil {
		return result, err
	}
	current, err = sourceGitText(deadline, root, 8192, "rev-parse", "HEAD")
	if err != nil || current != o.commit {
		return result, sourceRefusal("stale-commit")
	}
	sourceSum := sha256.Sum256(source)
	result["source"] = map[string]any{"object_format": format, "commit": o.commit, "tree": tree, "path": path, "blob": blob, "bytes": len(source), "sha256": hex.EncodeToString(sourceSum[:])}
	result["view"], result["ok"], result["selector_complete"] = view, true, true
	return result, nil
}

func selectSourceHandle(packet map[string]any, row, evidence int) (map[string]any, error) {
	if packet["tool"] != "context" || packet["ok"] != true || packet["mutates"] != false || sourcePositiveInt(packet["schema_version"]) != 1 {
		return nil, sourceRefusal("packet-profile")
	}
	if packet["state"] != "READY" && packet["state"] != "BUDGETED" {
		return nil, sourceRefusal("packet-state")
	}
	if _, ok := packet["request"].(map[string]any); !ok {
		return nil, sourceRefusal("packet-shape")
	}
	if _, ok := packet["coverage"].(map[string]any); !ok {
		return nil, sourceRefusal("packet-shape")
	}
	if _, ok := packet["subject"]; !ok {
		return nil, sourceRefusal("packet-shape")
	}
	results, ok := packet["results"].([]any)
	if !ok || len(results) > 50 || row >= len(results) {
		return nil, sourceRefusal("packet-results")
	}
	total := 0
	for _, value := range results {
		item, ok := value.(map[string]any)
		items, evidenceOK := item["evidence"].([]any)
		if !ok || !evidenceOK {
			return nil, sourceRefusal("packet-evidence")
		}
		total += len(items)
	}
	if total > 256 {
		return nil, sourceRefusal("packet-evidence-budget")
	}
	items := results[row].(map[string]any)["evidence"].([]any)
	if evidence >= len(items) {
		return nil, sourceRefusal("selector-evidence")
	}
	handle, ok := items[evidence].(map[string]any)
	if !ok || sourcePositiveInt(handle["line"]) == 0 {
		return nil, sourceRefusal("packet-handle")
	}
	return handle, nil
}

// containsBareCR reports whether source holds a CR that no LF follows. The
// spec retains LF and CRLF bytes and refuses every other carriage return.
func containsBareCR(source []byte) bool {
	for offset := 0; ; {
		index := bytes.IndexByte(source[offset:], '\r')
		if index < 0 {
			return false
		}
		offset += index + 1
		if offset >= len(source) || source[offset] != '\n' {
			return true
		}
	}
}

func extractSourceView(source []byte, handle map[string]any, linesArg, requirement string, maxBytes int) (map[string]any, error) {
	if !utf8.Valid(source) || bytes.IndexByte(source, 0) >= 0 || containsBareCR(source) {
		return nil, sourceRefusal("unsupported-text")
	}
	lines := bytes.SplitAfter(source, []byte{'\n'})
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	start, end, rule := 0, 0, "caller-inclusive-lines"
	if requirement != "" {
		if !regexp.MustCompile(`^[A-Z][A-Z0-9]*(?:-[A-Z0-9]+)*-[0-9]+$`).MatchString(requirement) || len(requirement) > 96 || handle["authority"] != "repository-spec" || handle["confidence"] != "high" || handle["reason"] != "defines "+requirement {
			return nil, sourceRefusal("ambiguous-requirement")
		}
		prefix := []byte("- `" + requirement + "`:")
		matches := []int{}
		for index, line := range lines {
			if bytes.HasPrefix(line, prefix) {
				matches = append(matches, index)
			}
		}
		if len(matches) != 1 || matches[0]+1 != sourcePositiveInt(handle["line"]) {
			return nil, sourceRefusal("ambiguous-requirement")
		}
		if !sourcePlainPrefix(lines[:matches[0]]) {
			return nil, sourceRefusal("unsupported-requirement")
		}
		text := lines[matches[0]][len(prefix):]
		if !bytes.HasPrefix(text, []byte(" ")) || len(bytes.TrimSpace(text)) == 0 || sourceClauseToken(text) {
			return nil, sourceRefusal("unsupported-requirement")
		}
		start, end, rule = matches[0], len(lines), "before-next-top-level-bullet-or-atx-section"
		for index := start + 1; index < len(lines); index++ {
			trimmed := bytes.TrimSpace(lines[index])
			if bytes.HasPrefix(lines[index], []byte("- ")) || regexp.MustCompile(`^#{1,6} `).Match(lines[index]) {
				end = index
				break
			}
			if len(trimmed) > 0 && !bytes.HasPrefix(lines[index], []byte("  ")) || sourceHTMLPattern.Match(lines[index]) || sourceClauseToken(lines[index]) {
				return nil, sourceRefusal("unsupported-requirement")
			}
		}
		if end == len(lines) {
			return nil, sourceRefusal("incomplete-boundary")
		}
	} else {
		match := regexp.MustCompile(`^([0-9]+):([0-9]+)$`).FindStringSubmatch(linesArg)
		if match == nil {
			return nil, sourceRefusal("line-range")
		}
		first, _ := strconv.Atoi(match[1])
		last, _ := strconv.Atoi(match[2])
		if first < 1 || first > last || last > len(lines) {
			return nil, sourceRefusal("line-range")
		}
		start, end = first-1, last
	}
	byteStart := 0
	for _, line := range lines[:start] {
		byteStart += len(line)
	}
	selected := bytes.Join(lines[start:end], nil)
	if len(selected) > maxBytes {
		return nil, sourceRefusal("view-budget")
	}
	sum := sha256.Sum256(selected)
	return map[string]any{"text": string(selected), "byte_start": byteStart, "byte_end": byteStart + len(selected), "line_start": start + 1, "line_end": end, "bytes": len(selected), "sha256": hex.EncodeToString(sum[:]), "boundary_rule": rule}, nil
}

// sourcePlainPrefix reports whether the lines before a requirement clause
// leave it outside YAML frontmatter, a fenced block and HTML (ESV-V0-002).
// Any HTML comment delimiter or line-leading HTML in the prefix refuses.
func sourcePlainPrefix(lines [][]byte) bool {
	if len(lines) > 0 && string(bytes.TrimSpace(lines[0])) == "---" && !sourceFrontmatterClosed(lines[1:]) {
		return false
	}
	fence := ""
	for _, line := range lines {
		if bytes.Contains(line, []byte("<!--")) || bytes.Contains(line, []byte("-->")) || sourceHTMLPattern.Match(line) {
			return false
		}
		fence = sourceNextFence(fence, line)
	}
	return fence == ""
}

func sourceFrontmatterClosed(lines [][]byte) bool {
	for _, line := range lines {
		trimmed := string(bytes.TrimSpace(line))
		if trimmed == "---" || trimmed == "..." {
			return true
		}
	}
	return false
}

// sourceNextFence returns the open fence delimiter after line: a delimiter of
// at most three leading spaces opens a fence, and a same-character delimiter
// at least as long with no trailing info text closes it.
func sourceNextFence(open string, line []byte) string {
	marker := sourceFencePattern.FindSubmatchIndex(line)
	if marker == nil {
		return open
	}
	delimiter := string(line[marker[2]:marker[3]])
	if open == "" {
		return delimiter
	}
	if delimiter[0] == open[0] && len(delimiter) >= len(open) && len(bytes.TrimSpace(line[marker[1]:])) == 0 {
		return ""
	}
	return open
}

// sourceClauseToken reports a fence, HTML comment delimiter or tab inside a
// requirement clause line.
func sourceClauseToken(line []byte) bool {
	for _, token := range []string{"```", "~~~", "<!--", "-->", "\t"} {
		if bytes.Contains(line, []byte(token)) {
			return true
		}
	}
	return false
}

// sourceGit runs one sanitized Git read in an owned process group: cancellation
// sends TERM to the group, then KILL, and reaps descendants before returning.
func sourceGit(ctx context.Context, root string, limit int, arguments ...string) ([]byte, error) {
	directory, err := filepath.Abs(root)
	if err != nil {
		return nil, sourceRefusal("repository-root")
	}
	executable, _ := ctx.Value(sourceGitExecutableKey{}).(string)
	if executable == "" {
		executable = sourceGitPath
	}
	argv := append(append([]string{executable}, sourceGitOptions...), append([]string{"-C", directory}, arguments...)...)
	observation := procgroup.Run(ctx, procgroup.Spec{Argv: argv, Dir: directory, Env: sourceGitEnvironment, Timeout: 10 * time.Second, ShutdownTimeout: sourceShutdownLimit, OutputLimit: limit, StderrLimit: 8192})
	if observation.Err != nil || observation.ExitStatus != 0 {
		return nil, sourceRefusal("git-failed")
	}
	return observation.Stdout, nil
}

// sourceIsToplevel reports whether root resolves to its own Git toplevel.
func sourceIsToplevel(ctx context.Context, root string) bool {
	toplevel, err := sourceGitText(ctx, root, 8192, "rev-parse", "--show-toplevel")
	if err != nil {
		return false
	}
	toplevel, err = filepath.EvalSymlinks(toplevel)
	if err != nil {
		return false
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absolute, err = filepath.EvalSymlinks(absolute)
	return err == nil && toplevel == absolute
}

func sourceGitText(ctx context.Context, root string, limit int, arguments ...string) (string, error) {
	raw, err := sourceGit(ctx, root, limit, arguments...)
	return strings.TrimSuffix(string(raw), "\n"), err
}

func sourcePositiveInt(value any) int {
	switch v := value.(type) {
	case json.Number:
		n, err := strconv.Atoi(string(v))
		if err == nil && n > 0 {
			return n
		}
	case float64:
		if v > 0 && v == float64(int(v)) {
			return int(v)
		}
	case int:
		if v > 0 {
			return v
		}
	}
	return 0
}
func sourceSafePath(value any) bool {
	path, ok := value.(string)
	if !ok || path == "" || len([]byte(path)) > 4096 || strings.HasPrefix(path, "-") || strings.Contains(path, "\\") || filepath.IsAbs(path) {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	for _, char := range path {
		if char < 32 || char == 127 {
			return false
		}
	}
	return true
}

// sourceObjectWidths maps each supported Git object format to its hex OID width.
var sourceObjectWidths = map[string]int{"sha1": 40, "sha256": 64}

// sourceObjectID returns the Git blob object ID of source under a supported
// object format, recomputed over the `blob <size>NUL` header and the bytes.
func sourceObjectID(format string, source []byte) string {
	object := append([]byte("blob "+strconv.Itoa(len(source))+"\x00"), source...)
	if format == "sha1" {
		sum := sha1.Sum(object)
		return hex.EncodeToString(sum[:])
	}
	sum := sha256.Sum256(object)
	return hex.EncodeToString(sum[:])
}

func sourceHex(value string, width int) bool {
	if len(value) != width {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && strings.ToLower(value) == value
}
func sourceErrorCode(err error) string {
	var refusal sourceRefusal
	if errors.As(err, &refusal) {
		return string(refusal)
	}
	return "malformed-input"
}
func nullableSourceString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func pathWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
