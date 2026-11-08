// SPDX-License-Identifier: Apache-2.0

// Package shaderdialect extracts bounded structural shader evidence from caller
// bytes. It never invokes a compiler or claims line coverage.
package shaderdialect

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/langplugin/dialect"
)

const (
	MaxBytes  = 1 << 20
	MaxTokens = 65536
	MaxBodies = 4096
)

// Include is a literal preprocessor dependency. Angled includes search only
// explicitly supplied include directories, never an ambient SDK or filesystem.
type Include struct {
	Name   string
	Angled bool
}

type token struct {
	text         string
	line, column int
}

// Range is a one-based, half-open line/column span. It mirrors the
// test-evidence region shape field for field, so the shader runner converts it
// without this dialect importing the evidence record.
type Range struct {
	Start, StartColumn, End, EndColumn int32
}

// Body is one shader function body with its span.
type Body struct {
	ID, Parent string
	Range      Range
}

// Source retains no partial complete result: Reason names any unsupported,
// malformed or capped input. Bodies use physical, one-based half-open ranges.
type Source struct {
	Includes     []Include
	Bodies       []Body
	Declarations []dialect.Declaration
	Lines        []dialect.Line
	Reason       string
}

func init() {
	dialect.RegisterRegion("shader", dialect.Region{Normalize: Normalize, Symbols: Symbols})
}

// Parse supports literal includes and ordinary C-style free functions/kernels,
// including Metal attributes and HLSL entry semantics. Conditional preprocessing,
// macros, aggregates and continuations are unsupported and explicitly incomplete.
func Parse(text string) Source {
	fail := func(reason string) Source { return Source{Reason: reason} }
	if len(text) > MaxBytes {
		return fail("shader-byte-cap")
	}
	if !utf8.ValidString(text) || strings.ContainsRune(text, 0) {
		return fail("shader-invalid-text")
	}
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	if strings.Contains(text, "\\\n") {
		return fail("shader-continuation-unsupported")
	}
	tokens, lines, reason := lex(text)
	if reason != "" {
		return fail(reason)
	}
	out := Source{Lines: lines}
	// Directives are deliberately interpreted before structural extraction. A
	// macro cannot masquerade as a function or hide an include from the graph.
	var code []token
	for i := 0; i < len(tokens); {
		if tokens[i].text != "#" {
			code = append(code, tokens[i])
			i++
			continue
		}
		start := i
		for i < len(tokens) && tokens[i].line == tokens[start].line {
			i++
		}
		row := tokens[start:i]
		if start > 0 && tokens[start-1].line == row[0].line {
			return fail("shader-directive-malformed")
		}
		if len(row) < 2 {
			return fail("shader-directive-malformed")
		}
		switch row[1].text {
		case "include":
			if len(row) == 3 && strings.HasPrefix(row[2].text, "\"") && strings.HasSuffix(row[2].text, "\"") {
				name := row[2].text[1 : len(row[2].text)-1]
				if name == "" || strings.ContainsAny(name, "\\\"\t ") {
					return fail("shader-include-malformed")
				}
				out.Includes = append(out.Includes, Include{Name: name})
			} else if len(row) >= 5 && row[2].text == "<" && row[len(row)-1].text == ">" {
				var name strings.Builder
				for j := 3; j < len(row); j++ {
					if row[j].column != row[j-1].column+len(row[j-1].text) {
						return fail("shader-include-malformed")
					}
				}
				for _, t := range row[3 : len(row)-1] {
					name.WriteString(t.text)
				}
				if name.Len() == 0 {
					return fail("shader-include-malformed")
				}
				out.Includes = append(out.Includes, Include{Name: name.String(), Angled: true})
			} else {
				return fail("shader-include-unresolved")
			}
		case "version", "extension":
			if len(row) < 3 {
				return fail("shader-directive-malformed")
			}
		case "pragma":
			if len(row) != 3 || row[2].text != "once" {
				return fail("shader-directive-unsupported")
			}
		default:
			return fail("shader-directive-unsupported")
		}
	}
	start := 0
	for i := 0; i < len(code); i++ {
		switch code[i].text {
		case ";":
			start = i + 1
		case "}":
			return fail("shader-unbalanced-body")
		case "{":
			name := ""
			nameLine := 0
			square, paren := 0, 0
			for j := start; j < i; j++ {
				switch code[j].text {
				case "[":
					square++
				case "]":
					square--
					if square < 0 {
						return fail("shader-signature-malformed")
					}
				case "(":
					if square == 0 && paren == 0 {
						if name != "" || j <= start+1 || !identifier(code[j-1].text) {
							return fail("shader-signature-unsupported")
						}
						name = code[j-1].text
						nameLine = code[j-1].line
					}
					paren++
				case ")":
					paren--
					if paren < 0 {
						return fail("shader-signature-malformed")
					}
				case "=":
					return fail("shader-aggregate-unsupported")
				}
			}
			if name == "" || square != 0 || paren != 0 {
				return fail("shader-signature-unsupported")
			}
			// The whole body is one conservative executable region. Nested statements
			// belong to it; no coverage or enclosing-body precision is invented.
			stack := []string{"{"}
			end := i
			for end++; end < len(code) && len(stack) > 0; end++ {
				t := code[end].text
				switch t {
				case "{", "(", "[":
					stack = append(stack, t)
				case "}", ")", "]":
					want := map[string]string{"}": "{", ")": "(", "]": "["}[t]
					if stack[len(stack)-1] != want {
						return fail("shader-unbalanced-body")
					}
					stack = stack[:len(stack)-1]
				}
			}
			if len(stack) != 0 {
				return fail("shader-unbalanced-body")
			}
			close := code[end-1]
			out.Declarations = append(out.Declarations, dialect.Declaration{Kind: "function", Name: name, Line: nameLine})
			out.Bodies = append(out.Bodies, Body{ID: fmt.Sprintf("%s:%d:%d", name, code[i].line, code[i].column), Range: Range{Start: int32(code[i].line), StartColumn: int32(code[i].column), End: int32(close.line), EndColumn: int32(close.column + 1)}})
			if len(out.Bodies) > MaxBodies {
				return fail("shader-body-cap")
			}
			i, start = end-1, end
		}
	}
	if start != len(code) {
		return fail("shader-trailing-declaration")
	}
	return out
}

func identifier(s string) bool {
	if s == "" || !(s[0] == '_' || s[0] >= 'a' && s[0] <= 'z' || s[0] >= 'A' && s[0] <= 'Z') {
		return false
	}
	for i := 1; i < len(s); i++ {
		if !word(s[i]) {
			return false
		}
	}
	return true
}
func word(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func lex(text string) ([]token, []dialect.Line, string) {
	var tokens []token
	line, col := 1, 1
	advance := func(s string) {
		for i := 0; i < len(s); i++ {
			if s[i] == '\n' {
				line++
				col = 1
			} else {
				col++
			}
		}
	}
	for i := 0; i < len(text); {
		start, l, c := i, line, col
		switch {
		case strings.ContainsRune(" \t\n\v\f", rune(text[i])):
			i++
		case strings.HasPrefix(text[i:], "//"):
			for i < len(text) && text[i] != '\n' {
				i++
			}
		case strings.HasPrefix(text[i:], "/*"):
			end := strings.Index(text[i+2:], "*/")
			if end < 0 {
				return nil, nil, "shader-unterminated-comment"
			}
			i += end + 4
		default:
			if text[i] == '"' || text[i] == '\'' {
				quote := text[i]
				i++
				for i < len(text) && text[i] != quote {
					if text[i] == '\n' {
						return nil, nil, "shader-unterminated-string"
					}
					if text[i] == '\\' {
						i++
					}
					i++
				}
				if i >= len(text) {
					return nil, nil, "shader-unterminated-string"
				}
				i++
			} else if text[i] >= '0' && text[i] <= '9' || text[i] == '.' && i+1 < len(text) && text[i+1] >= '0' && text[i+1] <= '9' {
				i++
				for i < len(text) {
					if word(text[i]) || text[i] == '.' {
						i++
						continue
					}
					if (text[i] == '+' || text[i] == '-') && strings.ContainsRune("eEpP", rune(text[i-1])) {
						i++
						continue
					}
					break
				}
			} else if word(text[i]) {
				for i < len(text) && word(text[i]) {
					i++
				}
			} else if strings.ContainsRune("#{}()[];,:.+-*/%=!<>&|^~?", rune(text[i])) {
				// Maximal munch preserves operator adjacency: x++ and x+ +y must
				// never normalize to the same token sequence.
				width := 1
				for _, op := range []string{"<<=", ">>=", "++", "--", "&&", "||", "<=", ">=", "==", "!=", "<<", ">>", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "::", "->"} {
					if strings.HasPrefix(text[i:], op) {
						width = len(op)
						break
					}
				}
				i += width
			} else {
				return nil, nil, "shader-token-unsupported"
			}
			tokens = append(tokens, token{text[start:i], l, c})
			if len(tokens) > MaxTokens {
				return nil, nil, "shader-token-cap"
			}
		}
		advance(text[start:i])
	}
	var lines []dialect.Line
	for i := 0; i < len(tokens); {
		l := tokens[i].line
		var b strings.Builder
		for i < len(tokens) && tokens[i].line == l {
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(tokens[i].text)
			i++
		}
		lines = append(lines, dialect.Line{Line: l, Text: b.String()})
	}
	return tokens, lines, ""
}

// Normalize preserves literal bytes and physical line identity while removing
// comments and whitespace differences. Unsupported syntax cannot compare equal.
func Normalize(text string) ([]dialect.Line, bool) {
	source := Parse(text)
	return source.Lines, source.Reason == ""
}

// Symbols exposes free functions/kernels through the existing dialect hook.
// Stop remains nonempty on any unsupported input, never a complete empty walk.
func Symbols(text string, maxLines, maxSymbols int) dialect.Extraction {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	source := Parse(text)
	out := dialect.Extraction{}
	if source.Reason != "" {
		out.Stop = dialect.Stop(source.Reason)
		return out
	}
	if maxLines <= 0 || strings.Count(text, "\n")+1 > maxLines {
		out.Stop = dialect.LineCap
		return out
	}
	if maxSymbols <= 0 || len(source.Bodies) > maxSymbols {
		out.Stop = dialect.SymbolCap
		return out
	}
	out.Declarations = source.Declarations
	return out
}
