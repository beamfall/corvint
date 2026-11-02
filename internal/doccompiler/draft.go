package doccompiler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"slices"
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
	// DryRun marks a draft over untracked or inline owner bytes (SDD-V0-019):
	// NOT_ADMISSIBLE, never governing, and refused by ConsumeDraft.
	DryRun bool
	// OwnerRelation is NAMES_PACKAGE or OWNER_UNRELATED (SDD-V0-630).
	OwnerRelation string
	// Page of Pages (SDD-V0-632); Pages is 1 when every list fits one page.
	Page, Pages int
	// OwnerSection is the named owner heading, empty for the default (SDD-V0-633).
	OwnerSection string
}

// Owner relations (SDD-V0-630).
const (
	OwnerNamesPackage = "NAMES_PACKAGE"
	OwnerUnrelated    = "OWNER_UNRELATED"
)

const ownerUnrelatedLimitation = "OWNER_UNRELATED: the owner excerpt does not name this package; its prose may describe a different capability and governs nothing here."

// ownerRelation reports whether excerpt names directory as a whole path token:
// not preceded by a path character and followed by end, "/", a non-name byte,
// or a '.' that is not followed by a name byte.
func ownerRelation(excerpt, directory string) string {
	pathByte := func(b byte) bool {
		return b == '_' || b == '-' || b == '.' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= 0x80
	}
	for offset := 0; ; {
		found := strings.Index(excerpt[offset:], directory)
		if found < 0 {
			return OwnerUnrelated
		}
		start, end := offset+found, offset+found+len(directory)
		before := start == 0 || !(pathByte(excerpt[start-1]) || excerpt[start-1] == '/')
		// A '.' ends the token as prose punctuation unless a name byte follows it.
		after := end == len(excerpt) || excerpt[end] == '/' || !pathByte(excerpt[end]) ||
			excerpt[end] == '.' && (end+1 == len(excerpt) || !pathByte(excerpt[end+1]) || excerpt[end+1] == '.')
		if before && after {
			return OwnerNamesPackage
		}
		offset = start + 1
	}
}

// draftDryRunProfile names the dry-run page; IsDryRunDraft reads this exact line.
const draftDryRunProfile = "corvint-source-documentation-draft-dry-run/0"

// DraftOptions selects a draft page (SDD-V0-632). The zero value is page 1.
type DraftOptions struct {
	// Page is the 1-based page of declarations and importers; 0 means 1.
	Page int
	// OwnerSection names the owner's H2 section (SDD-V0-633); empty means
	// the default "Agent digest", whose excerpt keeps the preamble.
	OwnerSection string
}

// defaultOwnerSection is the Corvint owner heading drafted when no section is named.
const defaultOwnerSection = "Agent digest"

// draftPageSize bounds each of the declaration and importer lists per page.
const draftPageSize = 64

// DraftSources compiles orientation only. It never creates an HDC plan, grants
// authority, or interprets source prose as proof of implementation behavior.
func DraftSources(index *contextindex.Index, sourcePath, directory string) (SourceDraft, error) {
	return DraftSourcesWith(index, sourcePath, directory, DraftOptions{})
}

// DraftSourcesWith is DraftSources with explicit options.
func DraftSourcesWith(index *contextindex.Index, sourcePath, directory string, options DraftOptions) (SourceDraft, error) {
	if err := draftDirectory(directory); err != nil {
		return SourceDraft{}, err
	}
	source, ok := index.Sources[sourcePath]
	if !ok || !strings.HasSuffix(sourcePath, ".md") {
		return SourceDraft{}, failure("unsupported-documentation-source", "owner source must be admitted tracked Markdown")
	}
	owner, err := ownerEntry(source, options.OwnerSection)
	if err != nil {
		return SourceDraft{}, err
	}
	return compileDraft(index, owner, sourcePath, directory, false, options)
}

// DraftDryRun previews a draft whose owner Markdown is untracked or inline
// bytes (SDD-V0-019). Declarations and importers still come only from the
// admitted index. The owner entry carries no Git blob, the page is marked
// NOT_ADMISSIBLE, and ConsumeDraft and maintenance never accept it.
func DraftDryRun(index *contextindex.Index, sourcePath string, owner []byte, directory string) (SourceDraft, error) {
	return DraftDryRunWith(index, sourcePath, owner, directory, DraftOptions{})
}

// DraftDryRunWith is DraftDryRun with explicit options.
func DraftDryRunWith(index *contextindex.Index, sourcePath string, owner []byte, directory string, options DraftOptions) (SourceDraft, error) {
	if err := draftDirectory(directory); err != nil {
		return SourceDraft{}, err
	}
	if clean, err := cleanRelative(sourcePath); err != nil || clean != sourcePath || !strings.HasSuffix(sourcePath, ".md") {
		return SourceDraft{}, failure("unsupported-documentation-source", "dry-run owner source must name one repository-relative Markdown path")
	}
	if len(owner) > DraftMaxBytes {
		return SourceDraft{}, failure("documentation-limit-exceeded", "dry-run owner source exceeds the DraftMaxBytes limit of %d bytes (64 KiB): observed %d bytes", DraftMaxBytes, len(owner))
	}
	entry, err := ownerEntry(contextindex.Source{Path: sourcePath, Data: owner}, options.OwnerSection)
	if err != nil {
		return SourceDraft{}, err
	}
	return compileDraft(index, entry, sourcePath, directory, true, options)
}

// ReadDryRunOwner reads the working-tree bytes of one repository-relative
// owner file for DraftDryRun, refusing symlinks, non-regular files and more
// than DraftMaxBytes. It writes nothing.
func ReadDryRunOwner(root, sourcePath string) ([]byte, error) {
	if !strings.HasSuffix(sourcePath, ".md") || slices.ContainsFunc(strings.Split(sourcePath, "/"), func(part string) bool { return strings.EqualFold(part, ".git") }) {
		return nil, failure("unsupported-documentation-source", "dry-run owner source must name one repository-relative Markdown path")
	}
	data, _, err := readPinned(root, sourcePath, DraftMaxBytes)
	if err == nil {
		return data, nil
	}
	var problem *Error
	if errors.As(err, &problem) && problem.Code == "file-too-large" {
		return nil, failure("documentation-limit-exceeded", "dry-run owner source exceeds the DraftMaxBytes limit of %d bytes (64 KiB): observed more than %d bytes", DraftMaxBytes, DraftMaxBytes)
	}
	if problem != nil {
		return nil, failure("unsupported-documentation-source", "dry-run owner source unreadable: %s", problem.Message)
	}
	return nil, failure("unsupported-documentation-source", "dry-run owner source unreadable")
}

// IsDryRunDraft reports whether provided bytes carry the dry-run profile line.
func IsDryRunDraft(provided []byte) bool {
	return bytes.Contains(provided, []byte("\nProfile: "+draftDryRunProfile+"\n"))
}

// PrecheckDraft refuses, before any index is built, a draft whose package is
// not one module directory or whose owner source is not Markdown or not in the
// commit the index reads, with exactly the error DraftSources returns for it
// (SDD-V0-700). committed reports whether the commit holds the path; nil means
// only the index can decide.
func PrecheckDraft(sourcePath, directory string, committed func(string) bool) error {
	if err := draftDirectory(directory); err != nil {
		return err
	}
	if !strings.HasSuffix(sourcePath, ".md") || !committed(sourcePath) {
		return failure("unsupported-documentation-source", "owner source must be admitted tracked Markdown")
	}
	return nil
}

func draftDirectory(directory string) error {
	if path.Clean(directory) != directory || directory == "." || strings.HasPrefix(directory, "/") || strings.HasPrefix(directory, "../") {
		return failure("unsupported-documentation-source", "package must name one repository-relative module directory")
	}
	return nil
}

func compileDraft(index *contextindex.Index, owner DraftEntry, sourcePath, directory string, dryRun bool, options DraftOptions) (SourceDraft, error) {
	draft := SourceDraft{Commit: index.CommitRevision, Tree: index.Revision, Entries: []DraftEntry{owner}, DryRun: dryRun, Limitations: []string{
		"GENERATED orientation; original owner prose retains its authority; indexed syntax does not validate behavior.",
		"Build applicability, runtime callers, behavioral validation and completeness outside admitted tracked sources are UNKNOWN.",
		fmt.Sprintf("Index coverage: %d exclusions, %d unparsed diagnostics; affected unparsed declarations and imports omitted. Dirty worktree bytes are not source evidence.", len(index.Exclusions), len(index.Unparsed)),
	}}
	draft.OwnerRelation = ownerRelation(owner.Excerpt, directory)
	if options.OwnerSection != defaultOwnerSection {
		draft.OwnerSection = options.OwnerSection
	}
	if draft.OwnerRelation == OwnerUnrelated {
		draft.Limitations = append([]string{ownerUnrelatedLimitation}, draft.Limitations...)
	}
	if dryRun {
		draft.Limitations = append([]string{"DRY RUN: NOT_ADMISSIBLE. The owner source is untracked or inline bytes, not admitted immutable Git content; this draft governs nothing, cannot be cited as authority, and docs consume refuses it. Commit the owner source and draft again."}, draft.Limitations...)
	}
	languages, unsupported, unknown := directoryLanguages(index, directory)
	if refused := append(unsupported, unknown...); len(languages) == 0 && len(refused) != 0 {
		return SourceDraft{}, failure("unsupported-documentation-source", "unsupported language: the native index extracts no declarations from %s", strings.Join(refused, ", "))
	}
	draft.Limitations = append(draft.Limitations, languageLimitations(languages, unsupported)...)
	declarations, spanLimitations, err := declarationEntries(index, directory)
	if err != nil {
		return SourceDraft{}, err
	}
	draft.Limitations = append(draft.Limitations, spanLimitations...)
	if len(declarations) == 0 {
		return SourceDraft{}, failure("unsupported-documentation-source", "package has no supported tracked non-test exported-name declarations")
	}
	importers, err := importerEntries(index, directory)
	if err != nil {
		return SourceDraft{}, err
	}
	// SDD-V0-632: each list pages independently; an entry off this page is UNKNOWN here.
	page := max(options.Page, 1)
	draft.Pages = max(pageCount(len(declarations)), pageCount(len(importers)), 1)
	if options.Page < 0 || page > draft.Pages {
		return SourceDraft{}, failure("documentation-limit-exceeded", "page %d is outside the draft's pages 1..%d (%d declarations, %d importers, %d entries per list per page)", options.Page, draft.Pages, len(declarations), len(importers), draftPageSize)
	}
	draft.Page = page
	var truncated []string
	declarations, truncated = pageEntries("declarations", declarations, page, draft.Pages, truncated)
	importers, truncated = pageEntries("importers", importers, page, draft.Pages, truncated)
	draft.Limitations = append(draft.Limitations, truncated...)
	draft.Entries = append(draft.Entries, declarations...)
	draft.Entries = append(draft.Entries, importers...)
	draft.Markdown = renderDraft(draft, sourcePath, directory)
	if len(draft.Markdown) > DraftMaxBytes {
		return SourceDraft{}, failure("documentation-limit-exceeded", "draft exceeds the DraftMaxBytes limit of %d bytes (64 KiB): observed %d bytes", DraftMaxBytes, len(draft.Markdown))
	}
	return draft, nil
}

// ownerEntry excerpts the owner. The default Agent digest keeps the preamble
// through the digest; a named section (SDD-V0-633) is that section alone.
func ownerEntry(source contextindex.Source, section string) (DraftEntry, error) {
	if section == "" {
		section = defaultOwnerSection
	} else if err := ownerSectionHeading(section); err != nil {
		return DraftEntry{}, err
	}
	text, valid, loaded := source.Text()
	if !loaded || !valid || strings.Contains(text, "\r") {
		return DraftEntry{}, failure("unsupported-documentation-source", "owner source must be loaded UTF-8 LF text")
	}
	// A final LF terminates the last physical line; it does not add another one.
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
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
		if line == "## "+section {
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
		if section == defaultOwnerSection {
			return DraftEntry{}, failure("unsupported-documentation-source", "exactly one Agent digest is required")
		}
		return DraftEntry{}, failure("unsupported-documentation-source", "exactly one literal ## %s heading outside fenced code is required: observed %d", draftMetadata(section), count)
	}
	first := 0
	if section != defaultOwnerSection {
		first = start
	}
	if end-first > 64 {
		if section == defaultOwnerSection {
			return DraftEntry{}, failure("documentation-limit-exceeded", "owner preamble and digest exceed the limit of 64 lines: observed %d lines", end)
		}
		return DraftEntry{}, failure("documentation-limit-exceeded", "owner section exceeds the limit of 64 lines: observed %d lines", end-first)
	}
	if source.BlobHash == "" {
		return dryRunOwnerEntry(source, section, first, lines[first:end])
	}
	return sourceEntry(source, "owner", section, first+1, end)
}

// ownerSectionHeading admits 1..200 bytes of printable UTF-8 with no newline.
func ownerSectionHeading(section string) error {
	printable := utf8.ValidString(section) && !strings.ContainsFunc(section, func(r rune) bool { return !unicode.IsPrint(r) })
	if len(section) > 200 || !printable || strings.TrimSpace(section) != section {
		return failure("unsupported-documentation-source", "owner section must be 1..200 bytes of printable UTF-8 without a newline or surrounding spaces")
	}
	return nil
}

// dryRunOwnerEntry is the owner entry over bytes with no Git object: Blob stays
// empty so the page renders no immutable original route.
func dryRunOwnerEntry(source contextindex.Source, title string, first int, lines []string) (DraftEntry, error) {
	excerpt := strings.Join(lines, "\n")
	if len(excerpt) > 4<<10 {
		return DraftEntry{}, failure("documentation-limit-exceeded", "source excerpt exceeds the limit of %d bytes (4 KiB): observed %d bytes", 4<<10, len(excerpt))
	}
	digest := sha256.Sum256(source.Data)
	return DraftEntry{ID: fmt.Sprintf("%s:%d:%s", source.Path, first+1, title), Kind: "dry-run-owner", Title: title, Excerpt: excerpt, Path: source.Path, SHA256: hex.EncodeToString(digest[:]), StartLine: first + 1, EndLine: first + len(lines)}, nil
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
		return DraftEntry{}, failure("documentation-limit-exceeded", "source excerpt exceeds the limit of %d bytes (4 KiB): observed %d bytes", 4<<10, len(excerpt))
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

// draftLanguage is one language the native index extracts declarations from
// (contextindex: goSymbols, pythonSourceFacts and symbolExtractors). Indexed
// symbols carry no visibility, so exported reads the declaration's own line.
type draftLanguage struct {
	name     string
	exported func(name, line string) bool
	// byName is true when exported ignores the line, so the source is not
	// read for a declaration the name alone excludes.
	byName bool
}

var (
	goLanguage     = draftLanguage{"Go", func(name, _ string) bool { r, _ := utf8.DecodeRuneInString(name); return unicode.IsUpper(r) }, true}
	pythonLanguage = draftLanguage{"Python", func(name, _ string) bool { return !strings.HasPrefix(name, "_") }, true}
	rubyLanguage   = draftLanguage{"Ruby", func(name, _ string) bool { return !strings.HasPrefix(name, "_") }, true}
	tsLanguage     = draftLanguage{"TypeScript", webExported, false}
	jsLanguage     = draftLanguage{"JavaScript", webExported, false}

	draftLanguages = map[string]draftLanguage{
		".go": goLanguage, ".py": pythonLanguage, ".rb": rubyLanguage, ".rake": rubyLanguage,
		".ts": tsLanguage, ".tsx": tsLanguage,
		".js": jsLanguage, ".jsx": jsLanguage, ".mjs": jsLanguage, ".cjs": jsLanguage,
		// pub(crate), pub(super) and pub(in path) are not exported.
		".rs": {"Rust", func(_, line string) bool {
			trimmed := strings.TrimLeft(line, " \t")
			return strings.HasPrefix(trimmed, "pub ") || strings.HasPrefix(trimmed, "pub\t")
		}, false},
		".cs": {"C#", func(name, line string) bool { return modifierWords(name, line, false)["public"] }, false},
		".swift": {"Swift", func(name, line string) bool {
			words := modifierWords(name, line, true)
			return words["public"] || words["open"]
		}, false},
		".kt":  {"Kotlin", kotlinExported, false},
		".kts": {"Kotlin", kotlinExported, false},
	}

	// unsupportedLanguages names admitted code suffixes the index reads only
	// as text. Any other unlisted suffix that is not documentation or data is
	// named by its suffix alone.
	unsupportedLanguages = map[string]string{
		".java": "Java", ".scala": "Scala", ".pony": "Pony", ".sh": "Shell", ".sql": "SQL",
		".c": "C", ".h": "C/C++ header", ".cc": "C++", ".cpp": "C++", ".cxx": "C++",
		".hh": "C++ header", ".hpp": "C++ header", ".hxx": "C++ header",
		".mts": "TypeScript module (.mts)", ".cts": "TypeScript module (.cts)", ".m": "Objective-C/MATLAB",
		".vue": "Vue", ".svelte": "Svelte",
	}
	// nonCodeSuffixes never make a directory a module of an unsupported language.
	nonCodeSuffixes = map[string]bool{
		"": true, ".md": true, ".mdx": true, ".txt": true, ".rst": true, ".adoc": true, ".asciidoc": true,
		".json": true, ".yaml": true, ".yml": true, ".toml": true, ".mod": true, ".sum": true, ".lock": true,
		".html": true, ".htm": true, ".css": true, ".scss": true, ".less": true, ".pcss": true,
		".erb": true, ".haml": true, ".slim": true, ".hbs": true, ".tpl": true,
		".svg": true, ".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".ico": true,
	}
)

func webExported(_, line string) bool { return firstWord(line) == "export" }

// Kotlin declarations are public unless a narrower modifier says otherwise.
func kotlinExported(name, line string) bool {
	words := modifierWords(name, line, true)
	return !words["private"] && !words["internal"] && !words["protected"]
}

// modifierWords returns the words of the declaration prefix: the code before
// the first whole-word occurrence of the declared name, with string literals
// and comments blanked, so a body, literal or trailing comment cannot supply
// a visibility modifier. An occurrence inside a `[...]` attribute or after
// `@` or `.` names an attribute, annotation or qualifier, not the declaration.
// nested is true for Swift and Kotlin, whose block comments nest. A name the
// line does not show yields no words.
func modifierWords(name, line string, nested bool) map[string]bool {
	if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
		name = name[dot+1:]
	}
	code := codeOnlyLine(line, nested)
	for offset := 0; name != "" && offset < len(code); {
		found := strings.Index(code[offset:], name)
		if found < 0 {
			break
		}
		start, end := offset+found, offset+found+len(name)
		if !identifierByteAt(code, start-1) && !identifierByteAt(code, end) && !qualifiedAt(code, start) &&
			strings.Count(code[:start], "[") <= strings.Count(code[:start], "]") {
			return lineWords(code[:start])
		}
		offset = end
	}
	return map[string]bool{}
}

func qualifiedAt(code string, start int) bool {
	before := strings.TrimRight(code[:start], " \t")
	return strings.HasSuffix(before, "@") || strings.HasSuffix(before, ".")
}

func identifierByteAt(text string, position int) bool {
	if position < 0 || position >= len(text) {
		return false
	}
	character := text[position]
	return character == '_' || character >= 0x80 || unicode.IsLetter(rune(character)) || unicode.IsDigit(rune(character))
}

// codeOnlyLine blanks quoted literals and block comments and drops a line
// comment, keeping byte offsets. It is lexical: a construct opened on an
// earlier line is not seen, so the declaration line alone decides. A C#
// verbatim string (@"...", $@"...", @$"...") escapes a quote by doubling it,
// never with a backslash.
func codeOnlyLine(line string, nested bool) string {
	code := []byte(line)
	for position := 0; position < len(code); position++ {
		switch {
		case code[position] == '/' && position+1 < len(code) && code[position+1] == '/':
			return string(code[:position])
		case code[position] == '/' && position+1 < len(code) && code[position+1] == '*':
			stop, depth := len(code), 1
			for scan := position + 2; scan+1 < len(code); scan++ {
				if nested && code[scan] == '/' && code[scan+1] == '*' {
					depth++
					scan++
				} else if code[scan] == '*' && code[scan+1] == '/' {
					depth--
					scan++
					if depth == 0 {
						stop = scan + 1
						break
					}
				}
			}
			for blank := position; blank < stop; blank++ {
				code[blank] = ' '
			}
			position = stop - 1
		case code[position] == '"' || code[position] == '\'':
			quote, stop := code[position], len(code)
			verbatim := quote == '"' && (strings.HasSuffix(string(code[:position]), "@") || strings.HasSuffix(string(code[:position]), "@$"))
			for scan := position + 1; scan < len(code); scan++ {
				if verbatim && code[scan] == '"' && scan+1 < len(code) && code[scan+1] == '"' {
					scan++
					continue
				}
				if !verbatim && code[scan] == '\\' {
					scan++
					continue
				}
				if code[scan] == quote {
					stop = scan + 1
					break
				}
			}
			for blank := position; blank < stop && blank < len(code); blank++ {
				code[blank] = ' '
			}
			position = stop - 1
		}
	}
	return string(code)
}

func firstWord(line string) string {
	fields := strings.FieldsFunc(line, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' })
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func lineWords(line string) map[string]bool {
	words := map[string]bool{}
	for _, word := range strings.FieldsFunc(line, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' }) {
		words[word] = true
	}
	return words
}

func sourceLanguage(sourcePath string) (draftLanguage, bool) {
	// Case-sensitive, as contextindex dispatches its extractors.
	language, ok := draftLanguages[path.Ext(sourcePath)]
	return language, ok
}

// draftTestSource excludes test files by name in every drafted language; Go
// keeps exactly its historical _test.go rule.
func draftTestSource(sourcePath string) bool {
	if strings.HasSuffix(sourcePath, ".go") {
		return strings.HasSuffix(sourcePath, "_test.go")
	}
	base := path.Base(sourcePath)
	original := strings.TrimSuffix(base, path.Ext(base))
	name, stem := strings.ToLower(base), strings.ToLower(original)
	return strings.HasPrefix(name, "test_") || strings.HasSuffix(stem, "_test") || strings.HasSuffix(stem, "_spec") ||
		strings.Contains(name, ".test.") || strings.Contains(name, ".spec.") ||
		strings.HasSuffix(original, "Test") || strings.HasSuffix(original, "Tests")
}

// directoryLanguages names, in sorted order, the drafted languages of the
// directory's tracked non-test sources, the known code languages it holds that
// the native index extracts no declarations from, and its other unrecognised
// suffixes. Only a directory with no drafted language is refused over the
// latter, since a data file beside a module is not a language.
func directoryLanguages(index *contextindex.Index, directory string) (languages, unsupported, unknown []string) {
	seen, missing, other := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for tracked := range index.Tracked {
		if path.Dir(tracked) != directory {
			continue
		}
		if language, ok := sourceLanguage(tracked); ok {
			if !draftTestSource(tracked) {
				seen[language.name] = true
			}
			continue
		}
		suffix := strings.ToLower(path.Ext(tracked))
		if nonCodeSuffixes[suffix] {
			continue
		}
		if name, ok := unsupportedLanguages[suffix]; ok {
			missing[name] = true
		} else {
			other[suffix] = true
		}
	}
	for name := range seen {
		languages = append(languages, name)
	}
	for name := range missing {
		unsupported = append(unsupported, name)
	}
	for suffix := range other {
		unknown = append(unknown, suffix)
	}
	sort.Strings(languages)
	sort.Strings(unsupported)
	sort.Strings(unknown)
	return languages, unsupported, unknown
}

// languageLimitations is empty for a Go-only package, so its draft bytes are
// unchanged from the Go-only profile.
func languageLimitations(languages, unsupported []string) []string {
	limitations := []string{}
	nonGo := []string{}
	for _, name := range languages {
		if name != "Go" {
			nonGo = append(nonGo, name)
		}
	}
	if len(nonGo) != 0 {
		limitations = append(limitations, fmt.Sprintf("Non-Go languages drafted: %s. Their exported names are read lexically from the declaration line (export, pub, public/open, no narrower Kotlin modifier, no leading underscore); import edges are drafted only for Go packages, so importers of these sources are UNKNOWN.", strings.Join(nonGo, ", ")))
	}
	// A Go package beside cgo, assembly or script files keeps its Go-only bytes.
	if len(nonGo) != 0 && len(unsupported) != 0 {
		limitations = append(limitations, fmt.Sprintf("Sources omitted because the native index extracts no declarations from their language: %s.", strings.Join(unsupported, ", ")))
	}
	return limitations
}

func declarationEntries(index *contextindex.Index, directory string) ([]DraftEntry, []string, error) {
	entries := []DraftEntry{}
	lines := map[string][]string{}
	spans := map[string]map[int]goSpan{}
	clipped := 0
	for _, symbol := range index.Symbols {
		language, drafted := sourceLanguage(symbol.Path)
		if path.Dir(symbol.Path) != directory || !drafted || draftTestSource(symbol.Path) || !sourceParsed(index, symbol.Path) {
			continue
		}
		if language.byName && !language.exported(symbol.Name, "") {
			continue
		}
		source, ok := index.Sources[symbol.Path]
		if !ok {
			return nil, nil, failure("unsupported-documentation-source", "indexed declaration source unavailable")
		}
		if !language.byName && !language.exported(symbol.Name, declarationLine(lines, source, symbol.Line)) {
			continue
		}
		start, end := symbol.Line, symbol.Line
		if span, ok := goDeclarationSpans(spans, source)[symbol.Line]; ok {
			start, end = span.start, span.end
		}
		entry, err := sourceEntry(source, "declaration", symbol.Kind+" "+symbol.Name, start, end)
		if err != nil && (start != symbol.Line || end != symbol.Line) {
			// SDD-V0-631: a span over the excerpt bound or holding a carriage return cites
			// only its indexed line, which must itself pass; the fallback is counted below.
			clipped++
			entry, err = sourceEntry(source, "declaration", symbol.Kind+" "+symbol.Name, symbol.Line, symbol.Line)
		}
		if err != nil {
			return nil, nil, err
		}
		entries = append(entries, entry)
	}
	limitations := []string{}
	if len(entries) != 0 {
		packageDoc, omitted, err := packageDocEntry(index, directory)
		if err != nil {
			return nil, nil, err
		}
		if packageDoc != nil {
			entries = append(entries, *packageDoc)
		}
		if omitted != "" {
			limitations = append(limitations, fmt.Sprintf("The package doc comment in %s exceeds the 4 KiB excerpt bound and is omitted.", omitted))
		}
	}
	if clipped != 0 {
		limitations = append(limitations, fmt.Sprintf("%d declarations cite only their indexed line: their doc-comment and declaration span exceeds the 4 KiB excerpt bound or contains a carriage return.", clipped))
	}
	sort.Slice(entries, func(a, b int) bool { return entries[a].ID < entries[b].ID })
	return entries, limitations, nil
}

// goSpan is one declaration's doc-comment-through-surface line span (SDD-V0-631).
type goSpan struct{ start, end int }

// goDeclarationSpans parses each Go source once per draft and maps every
// declared name's line to its span. A non-Go or unparsable source maps nothing,
// so its declarations keep their indexed line.
func goDeclarationSpans(cache map[string]map[int]goSpan, source contextindex.Source) map[int]goSpan {
	if spans, ok := cache[source.Path]; ok {
		return spans
	}
	spans := map[int]goSpan{}
	cache[source.Path] = spans
	if path.Ext(source.Path) != ".go" {
		return spans
	}
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, source.Path, source.Data, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return spans
	}
	line := func(position token.Pos) int { return set.Position(position).Line }
	from := func(doc *ast.CommentGroup, fallback token.Pos) int {
		if doc != nil {
			return line(doc.Pos())
		}
		return line(fallback)
	}
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			end := line(declaration.End())
			if declaration.Body != nil {
				end = line(declaration.Body.Lbrace)
			}
			span := goSpan{from(declaration.Doc, declaration.Pos()), end}
			spans[line(declaration.Pos())] = span
			spans[line(declaration.Name.Pos())] = span
		case *ast.GenDecl:
			grouped := declaration.Lparen.IsValid()
			for _, spec := range declaration.Specs {
				var doc *ast.CommentGroup
				var names []*ast.Ident
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					doc, names = spec.Doc, []*ast.Ident{spec.Name}
				case *ast.ValueSpec:
					doc, names = spec.Doc, spec.Names
				default:
					continue
				}
				span := goSpan{from(doc, spec.Pos()), line(spec.End())}
				if grouped && doc == nil && declaration.Doc != nil && len(declaration.Specs) == 1 {
					// A single-spec group's comment documents its spec, as in go/doc.
					span.start = line(declaration.Doc.Pos())
				}
				if !grouped {
					span = goSpan{from(declaration.Doc, declaration.Pos()), line(declaration.End())}
				}
				for _, name := range names {
					spans[line(name.Pos())] = span
				}
			}
		}
	}
	return spans
}

// packageDocEntry is the first non-test Go file in path order, in exactly
// directory, whose package clause carries a doc comment (SDD-V0-631). It
// returns the omitted path instead when that comment exceeds the excerpt bound.
func packageDocEntry(index *contextindex.Index, directory string) (*DraftEntry, string, error) {
	paths := []string{}
	for sourcePath := range index.Sources {
		if path.Dir(sourcePath) == directory && nonTestGo(sourcePath) && sourceParsed(index, sourcePath) {
			paths = append(paths, sourcePath)
		}
	}
	sort.Strings(paths)
	for _, sourcePath := range paths {
		source := index.Sources[sourcePath]
		set := token.NewFileSet()
		file, err := parser.ParseFile(set, sourcePath, source.Data, parser.PackageClauseOnly|parser.ParseComments)
		if err != nil || file.Doc == nil {
			continue
		}
		entry, err := sourceEntry(source, "package-doc", "package "+file.Name.Name, set.Position(file.Doc.Pos()).Line, set.Position(file.Name.End()).Line)
		var problem *Error
		if errors.As(err, &problem) && problem.Code == "documentation-limit-exceeded" {
			return nil, sourcePath, nil
		}
		if err != nil {
			return nil, "", err
		}
		return &entry, "", nil
	}
	return nil, "", nil
}

// declarationLine splits each source once per draft, caching it in lines.
func declarationLine(lines map[string][]string, source contextindex.Source, line int) string {
	split, cached := lines[source.Path]
	if !cached {
		if text, valid, loaded := source.Text(); loaded && valid {
			split = strings.Split(text, "\n")
		}
		lines[source.Path] = split
	}
	if line < 1 || line > len(split) {
		return ""
	}
	return split[line-1]
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
	return entries, nil
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

func pageCount(entries int) int { return (entries + draftPageSize - 1) / draftPageSize }

// pageEntries returns one page of an ID-sorted list and, when entries fall
// outside that page, appends its TRUNCATED limitation.
func pageEntries(name string, entries []DraftEntry, page, pages int, truncated []string) ([]DraftEntry, []string) {
	sort.Slice(entries, func(a, b int) bool { return entries[a].ID < entries[b].ID })
	first := min((page-1)*draftPageSize, len(entries))
	last := min(first+draftPageSize, len(entries))
	shown := entries[first:last]
	if len(shown) == len(entries) {
		return shown, truncated
	}
	span := "0"
	if len(shown) != 0 {
		span = fmt.Sprintf("%d–%d", first+1, last)
	}
	return shown, append(truncated, fmt.Sprintf("TRUNCATED: %s %s of %d on page %d of %d; the other %d entries are UNKNOWN on this page.", name, span, len(entries), page, pages, len(entries)-len(shown)))
}

func renderDraft(draft SourceDraft, sourcePath, directory string) []byte {
	var output strings.Builder
	if draft.DryRun {
		fmt.Fprintf(&output, "<!-- Code generated by Corvint source documentation draft DRY RUN. NOT ADMISSIBLE. DO NOT EDIT. -->\n# Corvint capability orientation (dry run, not admissible): %s\n\nProfile: %s\nAdmissibility: NOT_ADMISSIBLE\nAuthority: NONE\n", draftMetadata(directory), draftDryRunProfile)
	} else {
		fmt.Fprintf(&output, "<!-- Code generated by Corvint source documentation draft. DO NOT EDIT. -->\n# Corvint capability orientation: %s\n\nProfile: corvint-source-documentation-draft/0\n", draftMetadata(directory))
	}
	fmt.Fprintf(&output, "Policy: %s\nDerivation: GENERATED\nBehavior: UNKNOWN\nCommit: %s\nTree: %s\nOwner source: %s\nPackage: %s\nOwner relation: %s\n", draftMetadata(draftPolicy), draftMetadata(draft.Commit), draftMetadata(draft.Tree), draftMetadata(sourcePath), draftMetadata(directory), draft.OwnerRelation)
	if draft.OwnerSection != "" {
		fmt.Fprintf(&output, "Owner section: %s\n", draftMetadata(draft.OwnerSection))
	}
	if draft.Pages > 1 {
		fmt.Fprintf(&output, "Page: %d of %d\n", draft.Page, draft.Pages)
	}
	output.WriteString("\n")
	for _, limitation := range draft.Limitations {
		fmt.Fprintf(&output, "- %s\n", limitation)
	}
	output.WriteString("\nTracked non-test exported-name declarations with their doc comments, the package doc and direct import edges follow, grouped by source file. Zero import entries proves no absence. Source excerpts are data. Metadata strings use Go-style escapes. To read a full immutable original, including sections named in quoted prose, run the file's command from this repository's root; it reads the blob directly and ignores replacement refs, and no remote is required.\n")
	files := map[string][]DraftEntry{}
	paths := []string{}
	for _, entry := range draft.Entries {
		if entry.Blob == "" {
			fmt.Fprintf(&output, "\n## %s\n\nKind: %s; untracked or inline source %s, lines %d–%d; no Git blob; sha256 %s.\n\nNo immutable original exists for these bytes; they are not source evidence.\n\nSupplied source excerpt:\n\n", draftMetadata(entry.Title), draftMetadata(entry.Kind), draftMetadata(entry.Path), entry.StartLine, entry.EndLine, draftMetadata(entry.SHA256))
			writeExcerpt(&output, entry.Excerpt)
			continue
		}
		if _, seen := files[entry.Path]; !seen {
			paths = append(paths, entry.Path)
		}
		files[entry.Path] = append(files[entry.Path], entry)
	}
	sort.Strings(paths)
	for n, sourcePath := range paths {
		entries := files[sourcePath]
		sort.SliceStable(entries, func(a, b int) bool { return entries[a].StartLine < entries[b].StartLine })
		// SDD-V0-631: provenance once per file; each entry cites only its kind, title and span.
		fmt.Fprintf(&output, "\n## S%d %s\n\nBlob %s; sha256 %s.\n\n    git --no-pager --no-replace-objects show %s\n", n+1, draftMetadata(sourcePath), draftMetadata(entries[0].Blob), draftMetadata(entries[0].SHA256), entries[0].Blob)
		for _, entry := range entries {
			if entry.Kind == "declaration" {
				// The excerpt itself shows the declared name; the title stays in the entry JSON.
				fmt.Fprintf(&output, "\nLines %d–%d:\n\n", entry.StartLine, entry.EndLine)
			} else {
				fmt.Fprintf(&output, "\n%s %s, lines %d–%d:\n\n", draftMetadata(entry.Kind), draftMetadata(entry.Title), entry.StartLine, entry.EndLine)
			}
			writeExcerpt(&output, entry.Excerpt)
		}
	}
	return []byte(output.String())
}

func writeExcerpt(output *strings.Builder, excerpt string) {
	for _, line := range strings.Split(excerpt, "\n") {
		fmt.Fprintf(output, "    %s\n", line)
	}
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
		return nil, failure("documentation-limit-exceeded", "draft exceeds the DraftMaxBytes limit of %d bytes (64 KiB): observed %d bytes", DraftMaxBytes, len(provided))
	}
	if draft.DryRun || IsDryRunDraft(provided) {
		return nil, failure("unsupported-documentation-source", "dry-run draft is not admissible: commit the owner source, draft again and consume that draft")
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
