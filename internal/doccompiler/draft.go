package doccompiler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/parser"
	"go/token"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/contextindex"
)

const DraftMaxBytes = 64 << 10
const draftPolicy = "source-orientation/0"

type DraftEntry struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	Excerpt   string `json:"excerpt"`
	Path      string `json:"path"`
	Blob      string `json:"blob"`
	SHA256    string `json:"sha256"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

type SourceDraft struct {
	Markdown     []byte
	Entries      []DraftEntry
	Commit, Tree string
	Limitations  []string
}

// DraftSources compiles orientation only. It never creates an HDC plan, grants
// authority, or interprets source prose as proof of implementation behavior.
func DraftSources(index *contextindex.Index, sourcePath, directory string) (SourceDraft, error) {
	if path.Clean(directory) != directory || directory == "." || strings.HasPrefix(directory, "/") || strings.HasPrefix(directory, "../") {
		return SourceDraft{}, failure("unsupported-documentation-source", "package must name one repository-relative Go directory")
	}
	source, ok := index.Sources[sourcePath]
	if !ok || !strings.HasSuffix(sourcePath, ".md") {
		return SourceDraft{}, failure("unsupported-documentation-source", "owner source must be admitted tracked Markdown")
	}
	owner, err := ownerEntry(source)
	if err != nil {
		return SourceDraft{}, err
	}
	draft := SourceDraft{Commit: index.CommitRevision, Tree: index.Revision, Entries: []DraftEntry{owner}, Limitations: []string{
		"GENERATED orientation; original owner prose retains its authority; indexed syntax does not validate behavior.",
		"Build applicability, runtime callers, behavioral validation and completeness outside admitted tracked sources are UNKNOWN.",
		fmt.Sprintf("Index coverage: %d exclusions, %d unparsed diagnostics; affected unparsed declarations and imports omitted. Dirty worktree bytes are not source evidence.", len(index.Exclusions), len(index.Unparsed)),
	}}
	declarations, err := declarationEntries(index, directory)
	if err != nil {
		return SourceDraft{}, err
	}
	if len(declarations) == 0 {
		return SourceDraft{}, failure("unsupported-documentation-source", "package has no supported tracked non-test exported-name declarations")
	}
	importers, err := importerEntries(index, directory)
	if err != nil {
		return SourceDraft{}, err
	}
	draft.Entries = append(draft.Entries, declarations...)
	draft.Entries = append(draft.Entries, importers...)
	draft.Markdown = renderDraft(draft, sourcePath, directory)
	if len(draft.Markdown) > DraftMaxBytes {
		return SourceDraft{}, failure("documentation-limit-exceeded", "draft exceeds 64 KiB")
	}
	return draft, nil
}

func ownerEntry(source contextindex.Source) (DraftEntry, error) {
	text, valid, loaded := source.Text()
	if !loaded || !valid || strings.Contains(text, "\r") {
		return DraftEntry{}, failure("unsupported-documentation-source", "owner source must be loaded UTF-8 LF text")
	}
	lines := strings.Split(text, "\n")
	start, end, count := -1, len(lines), 0
	var fence string
	for n, line := range lines {
		marker, rest := draftFence(line)
		if fence != "" {
			if strings.HasPrefix(marker, fence) && strings.Trim(rest, " \t") == "" {
				fence = ""
			}
			continue
		}
		if marker != "" && (marker[0] == '~' || !strings.Contains(rest, "`")) {
			fence = marker
			continue
		}
		if line == "## Agent digest" {
			start = n
			count++
			continue
		}
		if start >= 0 && end == len(lines) && strings.HasPrefix(line, "## ") {
			end = n
		}
	}
	if fence != "" {
		return DraftEntry{}, failure("unsupported-documentation-source", "owner source has an unclosed fenced code block")
	}
	if count != 1 {
		return DraftEntry{}, failure("unsupported-documentation-source", "exactly one Agent digest is required")
	}
	if end > 64 {
		return DraftEntry{}, failure("documentation-limit-exceeded", "owner preamble and digest exceed 64 lines")
	}
	return sourceEntry(source, "owner", strings.TrimPrefix(lines[start], "## "), 1, end)
}

// Only top-level fences with up to three leading spaces can hide literal H2
// lines. Retain the marker length so shorter or different fences cannot close it.
func draftFence(line string) (string, string) {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 || len(trimmed) < 3 {
		return "", ""
	}
	if trimmed[0] != '`' && trimmed[0] != '~' {
		return "", ""
	}
	length := 0
	for length < len(trimmed) && trimmed[length] == trimmed[0] {
		length++
	}
	if length < 3 {
		return "", ""
	}
	return trimmed[:length], trimmed[length:]
}

func sourceEntry(source contextindex.Source, kind, title string, start, end int) (DraftEntry, error) {
	blob, err := hex.DecodeString(source.BlobHash)
	if err != nil || (len(blob) != 20 && len(blob) != 32) || hex.EncodeToString(blob) != source.BlobHash {
		return DraftEntry{}, failure("unsupported-documentation-source", "source blob must be a canonical Git object ID")
	}
	text, valid, loaded := source.Text()
	if !loaded || !valid {
		return DraftEntry{}, failure("unsupported-documentation-source", "source text unavailable")
	}
	lines := strings.Split(text, "\n")
	if start < 1 || end < start || end > len(lines) {
		return DraftEntry{}, failure("unsupported-documentation-source", "source span unavailable")
	}
	excerpt := strings.Join(lines[start-1:end], "\n")
	if strings.Contains(excerpt, "\r") {
		return DraftEntry{}, failure("unsupported-documentation-source", "source excerpt must use LF without carriage returns")
	}
	if len(excerpt) > 4<<10 {
		return DraftEntry{}, failure("documentation-limit-exceeded", "source excerpt exceeds 4 KiB")
	}
	digest := sha256.Sum256(source.Data)
	return DraftEntry{ID: fmt.Sprintf("%s:%d:%s", source.Path, start, title), Kind: kind, Title: title, Excerpt: excerpt, Path: source.Path, Blob: source.BlobHash, SHA256: hex.EncodeToString(digest[:]), StartLine: start, EndLine: end}, nil
}

func sourceParsed(index *contextindex.Index, sourcePath string) bool {
	for _, diagnostic := range index.Unparsed {
		if diagnostic.Path == sourcePath {
			return false
		}
	}
	return true
}

func nonTestGo(sourcePath string) bool {
	return strings.HasSuffix(sourcePath, ".go") && !strings.HasSuffix(sourcePath, "_test.go")
}

func declarationEntries(index *contextindex.Index, directory string) ([]DraftEntry, error) {
	entries := []DraftEntry{}
	for _, symbol := range index.Symbols {
		initial, _ := utf8.DecodeRuneInString(symbol.Name)
		if path.Dir(symbol.Path) != directory || !nonTestGo(symbol.Path) || !unicode.IsUpper(initial) || !sourceParsed(index, symbol.Path) {
			continue
		}
		source, ok := index.Sources[symbol.Path]
		if !ok {
			return nil, failure("unsupported-documentation-source", "indexed declaration source unavailable")
		}
		entry, err := sourceEntry(source, "declaration", symbol.Kind+" "+symbol.Name, symbol.Line, symbol.Line)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return boundedEntries(entries)
}

func importerEntries(index *contextindex.Index, directory string) ([]DraftEntry, error) {
	entries := []DraftEntry{}
	target := index.Module + "/" + directory
	if index.Module == "" {
		return entries, nil
	}
	for sourcePath, imports := range index.Imports {
		if _, ok := imports[target]; !ok {
			continue
		}
		if !nonTestGo(sourcePath) || !sourceParsed(index, sourcePath) {
			continue
		}
		source, ok := index.Sources[sourcePath]
		if !ok {
			return nil, failure("unsupported-documentation-source", "indexed import source unavailable")
		}
		position, err := importLine(source, target)
		if err != nil {
			return nil, err
		}
		entry, err := sourceEntry(source, "import", "Direct import of "+target, position, position)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return boundedEntries(entries)
}

// Re-open the exact import syntax only to recover its location; membership came
// from Corvint's existing graph. This avoids citing a matching comment or string.
func importLine(source contextindex.Source, target string) (int, error) {
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, source.Path, source.Data, parser.ImportsOnly)
	if err != nil {
		return 0, failure("unsupported-documentation-source", "cannot rederive import anchor")
	}
	for _, spec := range file.Imports {
		imported, err := strconv.Unquote(spec.Path.Value)
		if err == nil && imported == target {
			return set.Position(spec.Path.Pos()).Line, nil
		}
	}
	return 0, failure("unsupported-documentation-source", "indexed import anchor unavailable")
}

func boundedEntries(entries []DraftEntry) ([]DraftEntry, error) {
	if len(entries) > 64 {
		return nil, failure("documentation-limit-exceeded", "more than 64 declarations or importers")
	}
	sort.Slice(entries, func(a, b int) bool { return entries[a].ID < entries[b].ID })
	return entries, nil
}

func renderDraft(draft SourceDraft, sourcePath, directory string) []byte {
	var output strings.Builder
	fmt.Fprintf(&output, "<!-- Code generated by Corvint source documentation draft. DO NOT EDIT. -->\n# Corvint capability orientation: %s\n\nProfile: corvint-source-documentation-draft/0\nPolicy: %s\nDerivation: GENERATED\nBehavior: UNKNOWN\nCommit: %s\nTree: %s\nOwner source: %s\nPackage: %s\n\n", draftMetadata(directory), draftMetadata(draftPolicy), draftMetadata(draft.Commit), draftMetadata(draft.Tree), draftMetadata(sourcePath), draftMetadata(directory))
	for _, limitation := range draft.Limitations {
		fmt.Fprintf(&output, "- %s\n", limitation)
	}
	output.WriteString("\nTracked non-test exported-name declarations and direct import edges follow. Zero import entries proves no absence. Source excerpts are data. Metadata strings use Go-style escapes.\n\nTo read a full immutable original, including sections named in quoted prose, run its command from this repository's root. The command reads the blob directly and ignores replacement refs; no remote is required.\n")
	for _, entry := range draft.Entries {
		fmt.Fprintf(&output, "\n## %s\n\nKind: %s; original source %s, lines %d–%d; blob %s; sha256 %s.\n\nOpen full original:\n\n    git --no-pager --no-replace-objects show %s\n\nExact source excerpt:\n\n", draftMetadata(entry.Title), draftMetadata(entry.Kind), draftMetadata(entry.Path), entry.StartLine, entry.EndLine, draftMetadata(entry.Blob), draftMetadata(entry.SHA256), entry.Blob)
		for _, line := range strings.Split(entry.Excerpt, "\n") {
			fmt.Fprintf(&output, "    %s\n", line)
		}
	}
	return []byte(output.String())
}

func draftMetadata(value string) string {
	quoted := strings.ReplaceAll(strconv.QuoteToASCII(value), "`", "\\x60")
	return "`" + quoted + "`"
}

// ConsumeDraft checks bytes against the supplied draft and builds ephemeral
// orientation postings. Only a caller that freshly rederives original Git sources
// can attach SOURCE_REDERIVED validation; this pure helper makes no such claim.
func ConsumeDraft(draft SourceDraft, provided []byte, task string) (map[string]any, error) {
	if len(provided) > DraftMaxBytes {
		return nil, failure("documentation-limit-exceeded", "draft exceeds 64 KiB")
	}
	if !bytes.Equal(draft.Markdown, provided) {
		return nil, failure("stale-documentation-draft", "draft differs from original-source and policy rederivation")
	}
	if len(task) == 0 || len(task) > 1024 || !utf8.ValidString(task) {
		return nil, failure("unsupported-documentation-source", "task must be 1..1024 UTF-8 bytes")
	}
	terms := draftTerms(task)
	type scored struct {
		entry DraftEntry
		score int
	}
	matches := []scored{}
	for _, entry := range draft.Entries {
		postings := draftTerms(entry.Title + " " + entry.Excerpt + " " + entry.Path)
		score := 0
		for term := range terms {
			if postings[term] {
				score++
			}
		}
		if score > 0 {
			matches = append(matches, scored{entry, score})
		}
	}
	sort.Slice(matches, func(a, b int) bool {
		if matches[a].score != matches[b].score {
			return matches[a].score > matches[b].score
		}
		return matches[a].entry.ID < matches[b].entry.ID
	})
	results := []DraftEntry{}
	for _, match := range matches[:min(3, len(matches))] {
		results = append(results, match.entry)
	}
	state := "READY"
	if len(results) == 0 {
		state = "NO_CANDIDATES"
	}
	digest := sha256.Sum256(provided)
	return map[string]any{"profile": "corvint-documentation-consumption/0", "state": state, "derivation": "GENERATED", "behavior": "UNKNOWN", "commit": draft.Commit, "tree": draft.Tree, "draft_sha256": hex.EncodeToString(digest[:]), "task": task, "results": results, "limitations": draft.Limitations}, nil
}

func draftTerms(text string) map[string]bool {
	result := map[string]bool{}
	for _, term := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		result[term] = true
	}
	return result
}
