package contextindex

import (
	"path"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// NormalizedLine is one source line in the comment- and whitespace-insensitive
// form that symbol-anchor re-resolution compares (DCP-V1-140). Line is the
// 1-based physical line the text starts on.
type NormalizedLine struct {
	Line int
	Text string
}

// SymbolRegionLanguage names the normalizer family for a path: "ruby" for a
// Ruby source (`.rb`, or a `.rake` task file, IDX-SNAP-V0-109), "web" for a
// TypeScript or JavaScript source, and "" for every other language, which has
// no region normalizer.
func SymbolRegionLanguage(name string) string {
	switch path.Ext(name) {
	case ".rb", ".rake":
		return "ruby"
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		return "web"
	}
	return ""
}

// NormalizedCodeLines returns a source's code lines with comments removed,
// indentation and trailing space dropped, and every other run of whitespace
// outside a literal collapsed to one space. Literal and heredoc text is kept
// byte for byte, so an edit inside a string is still a change. Lines left with
// no code are omitted, except a line inside a multi-line literal or heredoc.
// It reuses the index's own Ruby scanner and web lexer and adds no grammar; ok
// is false for an unsupported language, non-text bytes, or a web source that
// ends inside a comment or template, so a caller never compares a partial walk.
func NormalizedCodeLines(name string, data []byte) ([]NormalizedLine, bool) {
	if !textDecodable(data) {
		return nil, false
	}
	text := normalizeLineEndings(string(data))
	switch SymbolRegionLanguage(name) {
	case "ruby":
		return rubyNormalizedLines(text), true
	case "web":
		return webNormalizedLines(text)
	}
	return nil, false
}

func rubyNormalizedLines(text string) []NormalizedLine {
	var out []NormalizedLine
	scanner := rubyScanner{}
	for offset, line := range strings.Split(text, "\n") {
		heredoc := len(scanner.heredocs) > 0
		continued := scanner.literal != nil
		block := scanner.inBlockComment
		var spans [][2]int
		scanner.spans, scanner.comment = &spans, -1
		scanner.scanLine(line)
		scanner.spans = nil
		switch {
		case heredoc:
			out = append(out, NormalizedLine{Line: offset + 1, Text: line})
			continue
		case block || scanner.inBlockComment:
			continue
		}
		cut := len(line)
		if scanner.comment >= 0 {
			cut = scanner.comment
		}
		normalized := collapseOutside(line[:cut], spans)
		if normalized != "" || continued {
			out = append(out, NormalizedLine{Line: offset + 1, Text: normalized})
		}
	}
	return out
}

// collapseOutside keeps the bytes inside the literal spans verbatim and
// collapses every whitespace run outside them to one space, dropping the runs
// at either end of the line.
func collapseOutside(line string, spans [][2]int) string {
	sort.Slice(spans, func(i, j int) bool { return spans[i][0] < spans[j][0] })
	var b strings.Builder
	pending := false
	emit := func(text string) {
		if pending && b.Len() > 0 {
			b.WriteByte(' ')
		}
		pending = false
		b.WriteString(text)
	}
	next := 0
	for index := 0; index < len(line); {
		for next < len(spans) && spans[next][1] <= index {
			next++
		}
		if next < len(spans) && spans[next][0] <= index {
			end := min(spans[next][1], len(line))
			emit(line[index:end])
			index = end
			continue
		}
		switch line[index] {
		case ' ', '\t', '\r', '\f', '\v':
			pending = true
		default:
			emit(line[index : index+1])
		}
		index++
	}
	return b.String()
}

func webNormalizedLines(text string) ([]NormalizedLine, bool) {
	starts := []int{0}
	for index := 0; index < len(text); index++ {
		if text[index] == '\n' {
			starts = append(starts, index+1)
		}
	}
	lines := make([]strings.Builder, len(starts))
	lexer := &webLexer{text: text}
	pending := false
	for lexer.position < len(text) {
		character, width := utf8.DecodeRuneInString(text[lexer.position:])
		switch {
		case unicode.IsSpace(character):
			lexer.position += width
			pending = true
		case lexer.hasAt("//"):
			lexer.skipLineComment()
			pending = true
		case lexer.hasAt("/*"):
			lexer.skipBlockComment()
			pending = true
		default:
			start := lexer.position
			if _, ok := lexer.next(); !ok || lexer.position <= start {
				return nil, false
			}
			line := sort.SearchInts(starts, start+1) - 1
			if pending && lines[line].Len() > 0 {
				lines[line].WriteByte(' ')
			}
			pending = false
			lines[line].WriteString(text[start:lexer.position])
		}
	}
	if lexer.unterminated != "" {
		return nil, false
	}
	var out []NormalizedLine
	for index := range lines {
		if lines[index].Len() > 0 {
			out = append(out, NormalizedLine{Line: index + 1, Text: lines[index].String()})
		}
	}
	return out, true
}
