package contextindex

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/langplugin/decl"
	"github.com/Beamfall/corvint/internal/langplugin/dialect"
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
// The suffixes are the region dialects language plugins declare (LGP-V0-015).
func SymbolRegionLanguage(name string) string {
	return decl.DialectFor(decl.RegionDialect, name)
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
	region, ok := dialect.LookupRegion(SymbolRegionLanguage(name))
	if !ok {
		return nil, false
	}
	lines, ok := region.Normalize(text)
	if !ok {
		return nil, false
	}
	out := make([]NormalizedLine, len(lines))
	for i, line := range lines {
		out[i] = NormalizedLine(line)
	}
	return out, true
}

func webNormalizedLines(text string) ([]dialect.Line, bool) {
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
	var out []dialect.Line
	for index := range lines {
		if lines[index].Len() > 0 {
			out = append(out, dialect.Line{Line: index + 1, Text: lines[index].String()})
		}
	}
	return out, true
}

// ImplementedDialects lists the dialects of kind this package implements,
// sorted, so a test can fail when a language plugin declares one it does not
// (LGP-V0-016).
func ImplementedDialects(kind decl.DialectKind) []string {
	var out []string
	switch kind {
	case decl.ImpactDialect:
		out = dialect.ImpactNames()
	case decl.ModuleDialect:
		for _, name := range decl.Dialects(decl.ModuleDialect) {
			if _, ok := moduleDisclosures[decl.RuleOf(decl.ModuleDialect, name)]; ok {
				out = append(out, name)
			}
		}
	case decl.RegionDialect:
		out = dialect.RegionNames()
	}
	sort.Strings(out)
	return out
}
