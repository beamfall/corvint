// SPDX-License-Identifier: Apache-2.0

package contextindex

import "strings"

// CSharpBody is a conservative, whole-line interior of a C# method body.
// Start is inclusive and End exclusive; signatures and brace lines are omitted.
type CSharpBody struct {
	Name       string
	Start, End int
}

// CSharpBodies reuses the C# declaration dialect and its literal/comment mask.
// Only flat, block-bodied methods with a single-line signature are admitted.
// Unsupported constructs, preprocessing, unbalanced syntax and capped dialect
// walks return false, never guessed declaration extents as executable regions.
// Unnamed methods and other members have no regions: edits there stay file-wide.
func CSharpBodies(data []byte) ([]CSharpBody, bool) {
	if !textDecodable(data) {
		return nil, false
	}
	text := normalizeLineEndings(string(data))
	symbols, notes := csharpSymbols(Source{Path: "source.cs"}, text)
	if len(notes) != 0 {
		return nil, false
	}
	lines := strings.Split(text, "\n")
	masked := make([]string, len(lines))
	state := newBraceState()
	for i, line := range lines {
		// The index dialect deliberately tolerates a broken plain quoted
		// literal up to its line break. Evidence regions need a stricter guard.
		probe := *state
		if !csharpBodyLiteralsComplete(line, &probe) {
			return nil, false
		}
		masked[i] = codeOnly(line, state, csharpDialect)
		if strings.HasPrefix(strings.TrimSpace(masked[i]), "#") {
			return nil, false
		}
	}
	if _, open := state.open(); open || state.depth != 0 {
		return nil, false
	}
	var out []CSharpBody
	for _, s := range symbols {
		if s.Kind != "func" {
			continue
		}
		i := s.Line - 1
		signature := masked[i]
		open := strings.IndexByte(signature, '{')
		if open < 0 {
			if i+1 >= len(masked) || strings.TrimSpace(masked[i+1]) != "{" {
				return nil, false
			}
			i++
		} else {
			if strings.TrimSpace(signature[open:]) != "{" {
				return nil, false
			}
			signature = signature[:open]
		}
		// This deliberately excludes expression bodies, attributes on the
		// declaration line, initializers, multiline signatures and nested bodies.
		if strings.ContainsAny(signature, "{};=[]") || strings.Contains(signature, "=>") || strings.Count(signature, "(") != 1 || strings.Count(signature, ")") != 1 {
			return nil, false
		}
		end := i + 1
		for ; end < len(masked); end++ {
			line := strings.TrimSpace(masked[end])
			if line == "}" {
				break
			}
			if strings.ContainsAny(line, "{}") || strings.Contains(line, "=>") {
				return nil, false
			}
		}
		if end == len(masked) {
			return nil, false
		}
		if end > i+1 {
			out = append(out, CSharpBody{Name: s.Name, Start: i + 2, End: end + 1})
		}
	}
	return out, true
}

func csharpBodyLiteralsComplete(line string, state *braceState) bool {
	for position := 0; position < len(line); {
		switch {
		case state.commentDepth > 0:
			position = consumeBlockComment(line, position, state, csharpDialect)
		case state.rawHashes >= 0:
			position = consumeRawString(line, position, state)
		case state.inTriple:
			position = consumeTriple(line, position, state)
		case state.inVerbatim:
			position = consumeVerbatim(line, position, state)
		default:
			if (line[position] == '"' && !hasAt(line, position, `"""`)) || line[position] == '\'' {
				quote := line[position]
				end := position + 1
				for end < len(line) && line[end] != quote {
					if line[end] == '\\' {
						end++
					}
					end++
				}
				if end >= len(line) {
					return false
				}
			}
			next, _, done := scanCode(line, position, state, csharpDialect)
			if done {
				return true
			}
			position = next
		}
	}
	return true
}
