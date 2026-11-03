// SPDX-License-Identifier: Apache-2.0

// Package rubydialect is the Ruby language plugin's dialect algorithm bodies
// (docs/specs/language-plugins-v0.md, LGP-V0-025): the lexical scanner that
// blanks comments, strings, percent literals and heredocs, the impact profile
// built on it, the symbol-region normalizer and declaration extractor, and the
// repository verification command. It registers them into
// internal/langplugin/dialect under the dialect names the Ruby plugin declares
// and imports no core package (LGP-V0-020).
//
// There is no closed Ruby grammar in this repository the way there is for
// Python, so this is lexical: the blanking pass is the whole point. A scanner
// that matches `def` and `class` against raw lines reports the contents of SQL
// heredocs, RSpec docstrings and commented-out code as declarations, and once
// those false symbols are in the index nothing distinguishes them from real
// ones.
package rubydialect

import (
	"regexp"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/langplugin/decl"
	"github.com/Beamfall/corvint/internal/langplugin/dialect"
)

// plugin is the language plugin whose declarations this package implements.
const plugin = "ruby"

func init() {
	for _, d := range decl.Of(plugin).Dialects {
		switch d.Kind {
		case decl.ImpactDialect:
			dialect.RegisterImpact(d.Name, dialect.Impact{
				CodeLines: CodeLines,
				Probes: []dialect.Probe{
					{Code: "observed-reflective-dispatch", Pattern: dialect.Reflection},
					{Code: "autoload-unresolved", Pattern: autoload},
				},
				Loads:        loads,
				StaticLoad:   StaticLoad,
				Verification: VerificationCommand,
			})
		case decl.RegionDialect:
			dialect.RegisterRegion(d.Name, dialect.Region{
				Normalize: func(text string) ([]dialect.Line, bool) { return NormalizedLines(text), true },
				Symbols:   Symbols,
			})
		}
	}
}

var (
	loads    = regexp.MustCompile(`\b(?:require_relative|require|load)\b`)
	autoload = regexp.MustCompile(`\bautoload\b`)
)

// CodeLines blanks every comment, string, percent literal and heredoc body,
// preserving line and column positions.
func CodeLines(text string) []string {
	scanner := rubyScanner{}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = scanner.scanLine(line)
	}
	return lines
}

// StaticLoad recognizes only a complete quoted argument, parenthesized or
// not. A quote-first expression can still concatenate or interpolate a
// dynamic name, so a double-quoted `#{` is dynamic.
func StaticLoad(tail string) bool {
	tail = strings.TrimSpace(tail)
	parenthesized := strings.HasPrefix(tail, "(")
	if parenthesized {
		tail = strings.TrimSpace(tail[1:])
	}
	if len(tail) < 2 || (tail[0] != '\'' && tail[0] != '"') {
		return false
	}
	quote := tail[0]
	for i := 1; i < len(tail); i++ {
		if quote == '"' && strings.HasPrefix(tail[i:], "#{") {
			return false
		}
		if tail[i] == '\\' {
			i++
			continue
		}
		if tail[i] != quote {
			continue
		}
		rest := strings.TrimSpace(tail[i+1:])
		if parenthesized {
			return strings.HasPrefix(rest, ")")
		}
		return rest == "" || strings.HasPrefix(rest, "#") || strings.HasPrefix(rest, ";")
	}
	return false
}

// Symbols extracts the declarations of one Ruby source. Every path that stops
// short of the end of the file, or reaches the end inside a lexical state it
// could not resolve, says so in the Extraction.
func Symbols(text string, maxLines, maxSymbols int) dialect.Extraction {
	var out dialect.Extraction
	scanner := rubyScanner{}
	remaining := text
	lineNumber := 0
	for remaining != "" {
		lineNumber++
		if lineNumber > maxLines {
			out.Stop = dialect.LineCap
			return out
		}
		line, rest, _ := strings.Cut(remaining, "\n")
		remaining = rest
		for _, declaration := range rubyDeclarations(scanner.scanLine(line)) {
			if len(out.Declarations) >= maxSymbols {
				out.Stop = dialect.SymbolCap
				return out
			}
			out.Declarations = append(out.Declarations, dialect.Declaration{Kind: declaration.kind, Name: declaration.name, Line: lineNumber})
		}
	}
	// The file was walked to its end, so the only remaining way to have missed
	// declarations is to have been inside something that never closed. Both
	// states swallowed every line after the opener.
	out.UnterminatedComment = scanner.inBlockComment
	out.UnterminatedString = scanner.literal != nil || len(scanner.heredocs) > 0
	return out
}

// NormalizedLines returns the code lines with comments removed and whitespace
// outside literals collapsed; literal and heredoc text is kept byte for byte.
func NormalizedLines(text string) []dialect.Line {
	var out []dialect.Line
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
			out = append(out, dialect.Line{Line: offset + 1, Text: line})
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
			out = append(out, dialect.Line{Line: offset + 1, Text: normalized})
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

// Word helpers shared with core's constant-reference rule, which reads the
// same blanked lines this scanner produces.

// Word returns the identifier starting at index and the index after it.
func Word(text string, index int) (string, int) { return rubyWord(text, index) }

// WordByte reports whether character continues an identifier.
func WordByte(character byte) bool { return rubyWordByte(character) }

// UpperByte reports whether character is an ASCII capital.
func UpperByte(character byte) bool { return rubyUpperByte(character) }

// SkipSpaces steps over blanks and tabs, reporting whether it moved.
func SkipSpaces(code string, index int) (int, bool) { return rubySkipSpaces(code, index) }

// SkipSigil steps over a variable's `@`, `@@` or `$`.
func SkipSigil(code string, index int) int { return rubySkipSigil(code, index) }

// KeywordStart reports whether the word at index starts a token.
func KeywordStart(code string, index int) bool { return rubyKeywordStart(code, index) }

// OperatorMethods are the method names Ruby spells with punctuation, longest
// first. Callers must not modify it.
var OperatorMethods = rubyOperatorMethods
