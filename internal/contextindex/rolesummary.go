package contextindex

import (
	"iter"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/runtimeenv"
)

// TCP-V0-039..042 (decision 0370): a file's role line is the first sentence
// of its package or module doc comment, else of its first top-level doc
// comment, read from the pinned blob at the indexed revision. Nothing is
// generated or persisted: the same blob always yields the same bytes. With
// `CORVINT_CONTEXT_ROLES=on` the role lines of the highest-scoring lexical
// sources are a fifth lexical field.
const (
	roleSummaryMaxBytes  = 160
	roleSummaryScanBytes = 64 << 10
	roleCandidateCap     = 512
	roleGain             = 1.0
)

// roleSummary is one file's role line and the 1-based line range of the
// comment it was read from.
type roleSummary struct {
	text       string
	start, end int
}

// roleHit is one source whose role line carries task terms.
type roleHit struct {
	source  uint32
	path    string
	summary roleSummary
	terms   []string
}

// commentBlock is a comment's content lines with the markers removed.
type commentBlock struct {
	lines      []string
	start, end int
}

// roleBlockReaders lists, per file suffix, the doc comments a role line may
// come from, in preference order and lazily, so extraction stops at the first
// qualifying one; a suffix absent here has no role line.
var roleBlockReaders = map[string]func([]string) iter.Seq[commentBlock]{
	".go": goRoleBlocks, ".py": pythonRoleBlocks, ".rs": rustRoleBlocks,
	".js": docMarkerBlocks, ".jsx": docMarkerBlocks, ".mjs": docMarkerBlocks, ".cjs": docMarkerBlocks,
	".ts": docMarkerBlocks, ".tsx": docMarkerBlocks, ".java": docMarkerBlocks, ".kt": docMarkerBlocks,
	".scala": docMarkerBlocks, ".swift": docMarkerBlocks, ".cs": docMarkerBlocks, ".c": docMarkerBlocks,
	".h": docMarkerBlocks, ".cc": docMarkerBlocks, ".cpp": docMarkerBlocks, ".hpp": docMarkerBlocks,
	".php": docMarkerBlocks, ".dart": docMarkerBlocks,
}

// roleRejected marks a paragraph that is a licence or generator header, not
// a statement of the file's role.
var roleRejected = []string{"copyright", "spdx-license-identifier", "licensed under", "code generated", "do not edit"}

func configureContextRoles(compiler *taskContextCompiler) *taskContextCompiler {
	compiler.roles = runtimeenv.Value("CONTEXT_ROLES") == "on"
	return compiler
}

// extractRoleSummary returns the role line of the file at name whose text is
// text, reading at most roleSummaryScanBytes.
func extractRoleSummary(name, text string) (roleSummary, bool) {
	reader, ok := roleBlockReaders[strings.ToLower(path.Ext(name))]
	if !ok {
		return roleSummary{}, false
	}
	scanned := text[:min(len(text), roleSummaryScanBytes)]
	for block := range reader(strings.Split(scanned, "\n")) {
		if line, ok := roleLine(block.lines); ok {
			return roleSummary{text: line, start: block.start, end: block.end}, true
		}
	}
	return roleSummary{}, false
}

// roleLine is the first sentence of the block's first paragraph, whitespace
// collapsed, control characters dropped, cut to roleSummaryMaxBytes on a
// rune boundary. A paragraph carrying a roleRejected marker has no line.
func roleLine(lines []string) (string, bool) {
	joined := strings.Join(strings.Fields(strings.Join(firstParagraph(lines), " ")), " ")
	joined = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, joined)
	if roleRejectedParagraph(joined) {
		return "", false
	}
	if end := strings.Index(joined, ". "); end >= 0 {
		joined = joined[:end+1]
	}
	if len(joined) > roleSummaryMaxBytes {
		cut := roleSummaryMaxBytes
		for cut > 0 && !utf8.RuneStart(joined[cut]) {
			cut--
		}
		joined = strings.TrimSpace(joined[:cut])
	}
	return joined, len(anchorWords(joined)) > 0
}

func roleRejectedParagraph(paragraph string) bool {
	lower := strings.ToLower(paragraph)
	for _, marker := range roleRejected {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// firstParagraph is the content lines from the first non-empty one up to an
// empty line, a tag line (`@param`) or a code fence; Markdown heading lines
// are skipped, and `@file` and `@fileoverview` keep their text.
func firstParagraph(lines []string) []string {
	paragraph := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		for _, tag := range []string{"@fileoverview", "@file"} {
			if rest, ok := strings.CutPrefix(line, tag); ok && (rest == "" || rest[0] == ' ') {
				line = strings.TrimSpace(rest)
			}
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		stop := line == "" || strings.HasPrefix(line, "@") || strings.HasPrefix(line, "```")
		if stop && len(paragraph) > 0 {
			return paragraph
		}
		if !stop {
			paragraph = append(paragraph, line)
		}
	}
	return paragraph
}

// goRoleBlocks: the comment immediately above the package clause, then each
// comment immediately above a column-0 declaration.
func goRoleBlocks(lines []string) iter.Seq[commentBlock] {
	return func(yield func(commentBlock) bool) {
		for index, line := range lines {
			if !strings.HasPrefix(line, "package ") {
				continue
			}
			if block, ok := commentAbove(lines, index); ok && !yield(block) {
				return
			}
			break
		}
		for index, line := range lines {
			if !goDeclaration(line) {
				continue
			}
			if block, ok := commentAbove(lines, index); ok && !yield(block) {
				return
			}
		}
	}
}

func goDeclaration(line string) bool {
	for _, keyword := range []string{"func ", "type ", "var ", "const "} {
		if strings.HasPrefix(line, keyword) {
			return true
		}
	}
	return false
}

// commentAbove is the column-0 `//` run or `/* */` block ending on the line
// before index, directive lines (`//go:build`) left out.
func commentAbove(lines []string, index int) (commentBlock, bool) {
	if index == 0 {
		return commentBlock{}, false
	}
	if strings.HasSuffix(strings.TrimSpace(lines[index-1]), "*/") {
		return blockAbove(lines, index-1)
	}
	start := index
	for start > 0 && strings.HasPrefix(lines[start-1], "//") {
		start--
	}
	if start == index {
		return commentBlock{}, false
	}
	content := make([]string, 0, index-start)
	for _, line := range lines[start:index] {
		body := strings.TrimPrefix(line, "//")
		if isGoDirective(body) {
			continue
		}
		content = append(content, body)
	}
	return commentBlock{lines: content, start: start + 1, end: index}, true
}

// blockAbove is the column-0 `/* */` block closed by the `*/` ending
// lines[end]. That line holds no code before a `/*`, and the backward scan
// stops, refusing, at any line that closes or opens another comment, so each
// line is scanned for at most one declaration.
func blockAbove(lines []string, end int) (commentBlock, bool) {
	if strings.Contains(lines[end], "/*") {
		if !oneLineBlock(lines[end]) {
			return commentBlock{}, false
		}
		return markerBlock(lines, end, end), true
	}
	for start := end - 1; start >= 0; start-- {
		line := lines[start]
		if strings.Contains(line, "*/") {
			return commentBlock{}, false
		}
		if strings.HasPrefix(line, "/*") {
			return markerBlock(lines, start, end), true
		}
		if strings.Contains(line, "/*") {
			return commentBlock{}, false
		}
	}
	return commentBlock{}, false
}

// oneLineBlock reports a line that is one column-0 `/* ... */` comment and
// nothing else.
func oneLineBlock(line string) bool {
	body := strings.TrimRight(line, " \t\r")
	closer := strings.Index(body, "*/")
	return strings.HasPrefix(body, "/*") && closer >= 2 && closer == len(body)-2
}

// isGoDirective reports a `//go:build`-shaped line (`//` then
// `[a-z0-9]+:`) or a `//+build` line, which go/doc leaves out of a doc comment.
func isGoDirective(body string) bool {
	prefix, _, ok := strings.Cut(body, ":")
	return strings.HasPrefix(body, "+build") || ok && prefix != "" && strings.Trim(prefix, "abcdefghijklmnopqrstuvwxyz0123456789") == ""
}

// pythonRoleBlocks: the module docstring, the first statement after blank
// lines and `#` comments.
func pythonRoleBlocks(lines []string) iter.Seq[commentBlock] {
	return func(yield func(commentBlock) bool) {
		for index, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			if block, ok := docstring(lines, index); ok {
				yield(block)
			}
			return
		}
	}
}

// docstring reads a string literal statement starting on lines[index], with
// an optional `r`/`u` prefix, triple or single quoted.
func docstring(lines []string, index int) (commentBlock, bool) {
	text := strings.TrimLeft(lines[index], "rRuU")
	for _, quote := range []string{`"""`, `'''`, `"`, `'`} {
		body, ok := strings.CutPrefix(text, quote)
		if !ok {
			continue
		}
		content := make([]string, 0)
		for end := index; end < len(lines) && (end == index || len(quote) == 3); end++ {
			if end > index {
				body = lines[end]
			}
			before, _, closed := strings.Cut(body, quote)
			content = append(content, before)
			if closed {
				return commentBlock{lines: content, start: index + 1, end: end + 1}, true
			}
		}
		return commentBlock{}, false
	}
	return commentBlock{}, false
}

// rustRoleBlocks: the first column-0 `//!` run, then the doc-marker blocks.
func rustRoleBlocks(lines []string) iter.Seq[commentBlock] {
	return func(yield func(commentBlock) bool) {
		for index := 0; index < len(lines); index++ {
			if !strings.HasPrefix(lines[index], "//!") {
				continue
			}
			start := index
			content := make([]string, 0)
			for ; index < len(lines) && strings.HasPrefix(lines[index], "//!"); index++ {
				content = append(content, strings.TrimPrefix(lines[index], "//!"))
			}
			if !yield(commentBlock{lines: content, start: start + 1, end: index}) {
				return
			}
			break
		}
		docMarkerBlocks(lines)(yield)
	}
}

// docMarkerBlocks: every column-0 `/** */` block and `///` run, in file
// order. The explicit doc marker keeps a plain `/*` licence block out.
func docMarkerBlocks(lines []string) iter.Seq[commentBlock] {
	return func(yield func(commentBlock) bool) {
		for index := 0; index < len(lines); index++ {
			line := lines[index]
			switch {
			case strings.HasPrefix(line, "/**") && !strings.HasPrefix(line, "/**/"):
				end, closed := blockEnd(lines, index)
				if !closed || !yield(markerBlock(lines, index, end)) {
					return
				}
				index = end
			case strings.HasPrefix(line, "///") && !strings.HasPrefix(line, "////"):
				start := index
				content := make([]string, 0)
				for ; index < len(lines) && strings.HasPrefix(lines[index], "///"); index++ {
					content = append(content, strings.TrimPrefix(lines[index], "///"))
				}
				if !yield(commentBlock{lines: content, start: start + 1, end: index}) {
					return
				}
			}
		}
	}
}

// blockEnd is the line holding the `*/` that closes the block comment
// opened at the start of lines[start].
func blockEnd(lines []string, start int) (int, bool) {
	for end := start; end < len(lines); end++ {
		searched := lines[end]
		if end == start {
			searched = searched[2:]
		}
		if strings.Contains(searched, "*/") {
			return end, true
		}
	}
	return 0, false
}

// markerBlock strips `/*`, `/**`, `/*!`, `*/` and a leading `*` from the
// lines of a block comment spanning lines[start..end].
func markerBlock(lines []string, start, end int) commentBlock {
	content := make([]string, 0, end-start+1)
	for index := start; index <= end; index++ {
		line := strings.TrimSpace(lines[index])
		if index == start {
			line = strings.TrimLeft(strings.TrimPrefix(line, "/*"), "*!")
		}
		line, _, _ = strings.Cut(line, "*/")
		if index != start {
			line = strings.TrimPrefix(line, "*")
		}
		content = append(content, line)
	}
	return commentBlock{lines: content, start: start + 1, end: end + 1}
}

// roleHits reads the role line of each of the roleCandidateCap highest
// scoring sources and keeps those whose line carries task terms, in
// candidate order. It is empty unless `CORVINT_CONTEXT_ROLES=on`.
func (compiler *taskContextCompiler) roleHits(table *TermTable, scores []float64) []roleHit {
	if !compiler.roles {
		return nil
	}
	terms := stringSetOf(compiler.terms)
	hits := make([]roleHit, 0)
	for _, source := range roleCandidates(scores) {
		name := table.Paths[source]
		text, ok := sourceTextBounded(compiler.index.Sources[name])
		if !ok {
			continue
		}
		summary, ok := extractRoleSummary(name, text)
		if !ok {
			continue
		}
		if matched := roleTerms(summary.text, terms); len(matched) > 0 {
			hits = append(hits, roleHit{source: source, path: name, summary: summary, terms: matched})
		}
	}
	return hits
}

// roleCandidates is the sources scoring above zero, highest first, ties by
// source id, at most roleCandidateCap.
func roleCandidates(scores []float64) []uint32 {
	candidates := make([]uint32, 0)
	for source, score := range scores {
		if score > 0 {
			candidates = append(candidates, uint32(source))
		}
	}
	sort.SliceStable(candidates, func(left, right int) bool {
		return scores[candidates[left]] > scores[candidates[right]]
	})
	return candidates[:min(len(candidates), roleCandidateCap)]
}

// roleTerms is the sorted task terms the role line carries, tokenised as
// the body Terms table tokenises a source.
func roleTerms(line string, terms map[string]struct{}) []string {
	matched := make([]string, 0)
	for term := range countTerms(line) {
		if _, ok := terms[term]; ok {
			matched = append(matched, term)
		}
	}
	sort.Strings(matched)
	return matched
}

// roleReason is the TCP-V0-041 evidence prefix of a lexical row.
func roleReason(hit *roleHit) string {
	return "role: " + strconv.Quote(hit.summary.text) + " (" + hit.path + ":" + strconv.Itoa(hit.summary.start) + "-" +
		strconv.Itoa(hit.summary.end) + ") matches `" + strings.Join(hit.terms, "`, `") + "`; "
}
