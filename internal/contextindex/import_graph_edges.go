package contextindex

import (
	"encoding/json"
	"path"
	"slices"
	"strings"
)

// Convention and framework-resource edges of the pinned import graph
// (NGI-V0-306 to NGI-V0-308). Each edge names the referencing site; anything
// that cannot be resolved to exactly one indexed file stays an uncertainty
// record on the referencing path, never an edge.

// importGraphKindNames is the closed --kinds vocabulary, in rank order: two
// edges with the same importer and specifier are emitted in this order.
var importGraphKindNames = []string{"import", "type-import", "angular-template", "angular-style", "ruby-constant"}

// importGraphEdge is one non-import edge into a target path.
type importGraphEdge struct {
	from, specifier, kind string
	evidence              map[string]any
}

// graphExtras holds the precomputed non-import edges by target path and the
// uncertainty rows they leave by referencing path.
type graphExtras struct {
	edges    map[string][]importGraphEdge
	unknowns map[string][]map[string]any
}

func (extras *graphExtras) edge(target string, edge importGraphEdge) {
	extras.edges[target] = append(extras.edges[target], edge)
}

// unknown records row once per identical content on its path.
func (extras *graphExtras) unknown(row map[string]any) {
	name := row["path"].(string)
	data, _ := json.Marshal(row)
	for _, existing := range extras.unknowns[name] {
		if seen, _ := json.Marshal(existing); string(seen) == string(data) {
			return
		}
	}
	extras.unknowns[name] = append(extras.unknowns[name], row)
}

// sortedUnknowns orders a path's rows by line, then by their encoding.
func (extras *graphExtras) sortedUnknowns(name string) []map[string]any {
	rows := extras.unknowns[name]
	slices.SortStableFunc(rows, func(left, right map[string]any) int {
		if a, b := left["line"].(int), right["line"].(int); a != b {
			return a - b
		}
		l, _ := json.Marshal(left)
		r, _ := json.Marshal(right)
		return strings.Compare(string(l), string(r))
	})
	return rows
}

// sortGraphEdges orders edges by importer, specifier, kind rank and line, and
// keeps the first site of each importer, specifier and kind.
func sortGraphEdges(edges []importGraphEdge) []importGraphEdge {
	slices.SortStableFunc(edges, func(left, right importGraphEdge) int {
		if c := strings.Compare(left.from, right.from); c != 0 {
			return c
		}
		if c := strings.Compare(left.specifier, right.specifier); c != 0 {
			return c
		}
		if c := slices.Index(importGraphKindNames, left.kind) - slices.Index(importGraphKindNames, right.kind); c != 0 {
			return c
		}
		return left.evidence["line"].(int) - right.evidence["line"].(int)
	})
	return slices.CompactFunc(edges, func(left, right importGraphEdge) bool {
		return left.from == right.from && left.specifier == right.specifier && left.kind == right.kind
	})
}

func writeGraphEdge(write func(ImportGraphRecord) error, kinds map[string]bool, target string, edge importGraphEdge) error {
	if len(kinds) > 0 && !kinds[edge.kind] {
		return nil
	}
	return write(ImportGraphRecord{Type: "edge", From: edge.from, To: target, Kind: edge.kind, Evidence: edge.evidence})
}

func buildGraphExtras(index *Index, ruby *rubyAutoload) *graphExtras {
	extras := &graphExtras{edges: map[string][]importGraphEdge{}, unknowns: map[string][]map[string]any{}}
	for _, name := range sortedSourcePaths(index) {
		switch NonGoImpactLanguage(name) {
		case "javascript", "typescript":
			angularResourceEdges(index, name, extras)
		}
	}
	rubyConstantEdges(index, ruby, extras)
	return extras
}

// angularResourceEdges reads the templateUrl, styleUrl and styleUrls members
// of each @Component(...) decorator argument. Only a plain string literal is
// resolved, relative to the component file, to an indexed path (NGI-V0-306).
func angularResourceEdges(index *Index, name string, extras *graphExtras) {
	source := index.Sources[name]
	text, valid, loaded := source.Text()
	if !valid || !loaded || !strings.Contains(text, "@Component") {
		return
	}
	masked := maskWebDecoratorCode(text)
	language := NonGoImpactLanguage(name)
	lineAt := func(offset int) int { return strings.Count(text[:offset], "\n") + 1 }
	resolve := func(property, kind, literal string, offset int) {
		row := map[string]any{"path": name, "language": language, "code": "angular-resource-unresolved",
			"line": lineAt(offset), "property": property}
		target := path.Join(path.Dir(name), literal)
		if literal == "" || strings.HasPrefix(literal, "/") || strings.Contains(literal, "\\") || target == ".." || strings.HasPrefix(target, "../") {
			extras.unknown(row)
			return
		}
		if _, ok := index.Sources[target]; !ok {
			row["candidate"] = target
			extras.unknown(row)
			return
		}
		extras.edge(target, importGraphEdge{from: name, specifier: literal, kind: kind, evidence: map[string]any{
			"path": name, "line": lineAt(offset), "blob_hash": source.BlobHash, "specifier": literal,
			"authority": "syntax", "property": property}})
	}
	nonliteral := func(property string, offset int) {
		extras.unknown(map[string]any{"path": name, "language": language, "code": "angular-resource-nonliteral",
			"line": lineAt(offset), "property": property})
	}
	for start := 0; ; {
		at := strings.Index(masked[start:], "@Component")
		if at < 0 {
			return
		}
		at += start
		start = at + len("@Component")
		if start < len(masked) && webIdentifierByte(masked[start]) {
			continue
		}
		open := skipWebSpace(masked, start)
		if open >= len(masked) || masked[open] != '(' {
			continue
		}
		end := matchingWebBracket(masked, open)
		brace := skipWebSpace(masked, open+1)
		if brace >= end || masked[brace] != '{' {
			continue
		}
		objectEnd := matchingWebBracket(masked, brace)
		depth := 0
		for i := brace + 1; i < objectEnd; i++ {
			switch masked[i] {
			case '{', '(', '[':
				depth++
				continue
			case '}', ')', ']':
				depth--
				continue
			}
			if depth != 0 {
				continue
			}
			// A member name is an identifier or a quoted string without
			// escapes; a literal not followed by `:` is a value, skipped whole.
			property, keyEnd := "", i
			switch {
			case masked[i] == '\'' || masked[i] == '"' || masked[i] == '`':
				closing := strings.IndexByte(masked[i+1:objectEnd], masked[i])
				if closing < 0 {
					i = objectEnd
					continue
				}
				keyEnd = i + closing + 2
				if name, _, ok := webStringLiteral(text, i); ok && masked[i] != '`' {
					property = name
				}
			case webIdentifierByte(masked[i]) && !webIdentifierByte(masked[i-1]):
				for keyEnd < objectEnd && webIdentifierByte(masked[keyEnd]) {
					keyEnd++
				}
				property = masked[i:keyEnd]
			default:
				continue
			}
			colon := skipWebSpace(masked, keyEnd)
			i = keyEnd - 1
			if colon >= objectEnd || masked[colon] != ':' {
				continue
			}
			kind := ""
			switch property {
			case "templateUrl":
				kind = "angular-template"
			case "styleUrl", "styleUrls":
				kind = "angular-style"
			default:
				continue
			}
			valueEnd := webMemberEnd(masked, colon+1, objectEnd)
			value := skipWebRawSpace(text, colon+1, valueEnd)
			i = valueEnd - 1
			if property != "styleUrls" {
				if literal, after, ok := webStringLiteral(text, value); ok && skipWebRawSpace(text, after, valueEnd) == valueEnd {
					resolve(property, kind, literal, value)
				} else {
					nonliteral(property, value)
				}
				continue
			}
			if value >= valueEnd || text[value] != '[' {
				nonliteral(property, value)
				continue
			}
			arrayEnd := matchingWebBracket(masked, value)
			if skipWebRawSpace(text, min(arrayEnd+1, valueEnd), valueEnd) != valueEnd {
				nonliteral(property, value)
				continue
			}
			for element := value + 1; element < arrayEnd; {
				elementEnd := webMemberEnd(masked, element, arrayEnd)
				first := skipWebRawSpace(text, element, elementEnd)
				if first < elementEnd {
					if literal, after, ok := webStringLiteral(text, first); ok && skipWebRawSpace(text, after, elementEnd) == elementEnd {
						resolve(property, kind, literal, first)
					} else {
						nonliteral(property, first)
					}
				}
				element = elementEnd + 1
			}
		}
	}
}

// maskWebDecoratorCode blanks comments and literal contents, preserving every
// offset and newline. Unlike stripWebNonCode it keeps each outermost literal's
// quotes, so a quoted member name stays visible, and blanks a template literal
// whole, its `${}` substitutions and the literals nested in them included, so
// a decorator spelled inside one is never read as code (NGI-V0-306).
func maskWebDecoratorCode(text string) string {
	masked := []byte(text)
	blank := func(from, to int) {
		for i := from; i < to; i++ {
			if masked[i] != '\n' {
				masked[i] = ' '
			}
		}
	}
	for i := 0; i < len(text); {
		switch {
		case strings.HasPrefix(text[i:], "//"):
			end := len(text)
			if next := strings.IndexByte(text[i:], '\n'); next >= 0 {
				end = i + next
			}
			blank(i, end)
			i = end
		case strings.HasPrefix(text[i:], "/*"):
			end := len(text)
			if next := strings.Index(text[i+2:], "*/"); next >= 0 {
				end = i + next + 4
			}
			blank(i, end)
			i = end
		case text[i] == '\'' || text[i] == '"' || text[i] == '`':
			end, closed := webLiteralEnd(text, i)
			if closed {
				blank(i+1, end-1)
			} else {
				blank(i+1, end)
			}
			i = end
		default:
			i++
		}
	}
	return string(masked)
}

// webLiteralEnd is the offset just past the literal opening at text[at], and
// whether it closed before the end of text. A template's `${}` substitutions
// are skipped as code, nested literals included.
func webLiteralEnd(text string, at int) (int, bool) {
	quote := text[at]
	for i := at + 1; i < len(text); {
		switch {
		case text[i] == '\\':
			i += 2
		case text[i] == quote:
			return i + 1, true
		case quote == '`' && strings.HasPrefix(text[i:], "${"):
			i = webSubstitutionEnd(text, i+2)
		default:
			i++
		}
	}
	return len(text), false
}

// webSubstitutionEnd is the offset just past the `}` closing a template
// substitution whose code starts at text[at], or len(text).
func webSubstitutionEnd(text string, at int) int {
	depth := 0
	for i := at; i < len(text); {
		switch {
		case strings.HasPrefix(text[i:], "//"):
			next := strings.IndexByte(text[i:], '\n')
			if next < 0 {
				return len(text)
			}
			i += next
		case strings.HasPrefix(text[i:], "/*"):
			next := strings.Index(text[i+2:], "*/")
			if next < 0 {
				return len(text)
			}
			i += next + 4
		case text[i] == '\'' || text[i] == '"' || text[i] == '`':
			i, _ = webLiteralEnd(text, i)
		case text[i] == '{':
			depth++
			i++
		case text[i] == '}':
			if depth == 0 {
				return i + 1
			}
			depth--
			i++
		default:
			i++
		}
	}
	return len(text)
}

func webIdentifierByte(c byte) bool {
	return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func skipWebSpace(masked string, at int) int {
	for at < len(masked) && (masked[at] == ' ' || masked[at] == '\t' || masked[at] == '\n' || masked[at] == '\r') {
		at++
	}
	return at
}

// skipWebRawSpace skips raw whitespace and comments up to limit.
func skipWebRawSpace(text string, at, limit int) int {
	for at < limit {
		switch {
		case text[at] == ' ' || text[at] == '\t' || text[at] == '\n' || text[at] == '\r':
			at++
		case strings.HasPrefix(text[at:], "//"):
			next := strings.IndexByte(text[at:], '\n')
			if next < 0 || at+next > limit {
				return limit
			}
			at += next
		case strings.HasPrefix(text[at:], "/*"):
			next := strings.Index(text[at+2:], "*/")
			if next < 0 || at+next+4 > limit {
				return limit
			}
			at += next + 4
		default:
			return at
		}
	}
	return limit
}

// matchingWebBracket returns the offset of the bracket closing masked[open],
// or len(masked) when it is unbalanced.
func matchingWebBracket(masked string, open int) int {
	depth := 0
	for i := open; i < len(masked); i++ {
		switch masked[i] {
		case '{', '(', '[':
			depth++
		case '}', ')', ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return len(masked)
}

// webMemberEnd is the offset of the `,` ending a member or element at depth
// zero, or limit.
func webMemberEnd(masked string, at, limit int) int {
	depth := 0
	for i := at; i < limit; i++ {
		switch masked[i] {
		case '{', '(', '[':
			depth++
		case '}', ')', ']':
			depth--
		case ',':
			if depth == 0 {
				return i
			}
		}
	}
	return limit
}

// webStringLiteral reads a quoted literal without escapes or interpolation.
func webStringLiteral(text string, at int) (string, int, bool) {
	if at >= len(text) || (text[at] != '\'' && text[at] != '"' && text[at] != '`') {
		return "", at, false
	}
	quote := text[at]
	for i := at + 1; i < len(text); i++ {
		switch text[i] {
		case quote:
			return text[at+1 : i], i + 1, true
		case '\\', '\n':
			return "", at, false
		case '$':
			if quote == '`' && i+1 < len(text) && text[i+1] == '{' {
				return "", at, false
			}
		}
	}
	return "", at, false
}

// rubyConstantEdges resolves each Ruby constant reference to the files the
// autoload convention maps its constant to. Exactly one candidate under a
// complete resolution is a convention edge; zero or several candidates, or a
// resolution an unbalanced file or a bound left open, is an uncertainty row
// naming the constants and candidates. A constant the referencing file maps
// itself, defines locally without any mapped file, or that is an implicit
// namespace (a mapped directory with no file) adds neither (NGI-V0-307).
func rubyConstantEdges(index *Index, ruby *rubyAutoload, extras *graphExtras) {
	files := map[string][]string{}
	for _, name := range ruby.paths {
		if constant, ok := ruby.mapper.Constant(name); ok {
			files[constant] = append(files[constant], name)
		}
	}
	type pending struct {
		row   map[string]any
		count int
	}
	for _, name := range ruby.paths {
		source := ruby.sources[name]
		if source == nil || !source.readable {
			continue
		}
		own, _ := ruby.mapper.Constant(name)
		edges := map[string]bool{}
		rows := map[string]*pending{}
		order := []string{}
		for _, reference := range source.references {
			resolved, complete := ruby.resolutions(source, reference)
			complete = complete && source.balanced && !source.overflowed
			slices.Sort(resolved)
			resolved = slices.Compact(resolved)
			constants, candidates := []string{}, []string{}
			for _, constant := range resolved {
				mapped := files[constant]
				if constant == own || len(mapped) == 0 && (source.local[constant] || ruby.mapper.Known(constant)) {
					continue
				}
				constants = append(constants, constant)
				for _, candidate := range mapped {
					if candidate != name {
						candidates = append(candidates, candidate)
					}
				}
			}
			if len(constants) == 0 {
				continue
			}
			slices.Sort(candidates)
			candidates = slices.Compact(candidates)
			if complete && len(resolved) == 1 && len(candidates) == 1 {
				key := candidates[0] + "\x00" + constants[0]
				if !edges[key] {
					edges[key] = true
					extras.edge(candidates[0], importGraphEdge{from: name, specifier: constants[0], kind: "ruby-constant", evidence: map[string]any{
						"path": name, "line": reference.line, "blob_hash": index.Sources[name].BlobHash, "specifier": constants[0],
						"authority": "convention", "convention": "zeitwerk"}})
				}
				continue
			}
			code := "ruby-constant-ambiguous"
			if len(candidates) == 0 {
				code = "ruby-constant-unresolved"
			}
			key := code + "\x00" + strings.Join(constants, ",") + "\x00" + strings.Join(candidates, ",")
			if row, ok := rows[key]; ok {
				row.count++
				continue
			}
			rows[key] = &pending{row: map[string]any{"path": name, "language": "ruby", "code": code, "line": reference.line,
				"constants": constants, "candidates": candidates}, count: 1}
			order = append(order, key)
		}
		for _, key := range order {
			rows[key].row["count"] = rows[key].count
			extras.unknown(rows[key].row)
		}
	}
}
