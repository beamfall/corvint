package contextindex

import (
	"regexp"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/projectprofile"
)

// The dependency-link stage of confident-symbol selection, ported from the
// oracle's `_web_dependency_links` and its helpers (`src/context_corvint.py`).
//
// The frontier stage picks the strongest declaration per path by vocabulary
// alone. It has no way to say that the second result is the function the first
// one calls, so on a web repository it fills the packet with same-vocabulary
// neighbours instead of the call chain the task is actually about. This stage
// walks the root result's direct calls two hops deep and promotes the two
// strongest linked declarations to ranks 2 and 3, each carrying the import and
// call sites that bind it to the root.
//
// Resolution reuses the reverse-import rules' specifier admission and target
// resolution (`reverseimports.go`), so the `@/` alias comes from the project
// profile rather than the one repository root the oracle hardcodes (DR-0012).

// webImportStatement is the oracle's static named-import pattern. RE2 has no
// lookahead, so the oracle's `(?!['"])` guard on the clause's first character
// is spelled as consuming one non-quote character: the two agree except on an
// import whose clause is empty, which is not a valid statement.
var webImportStatement = regexp.MustCompile(`(?ms)^[ \t]*import[ \t]+([^'"].*?)[ \t]+from[ \t]*['"]([^'"]+)['"]`)

// webImportAfterSemicolon is the same statement written after a code `;`, so
// every import of a line such as `import { a } from './a'; import { b } from
// './b'` binds where the oracle read only the first (GPK-V0-087, V1-2128;
// known-divergent DR-0044). It is tried only right after a `;` the web lexer
// reads as code, never one inside a string, template or comment.
var webImportAfterSemicolon = regexp.MustCompile(`(?s)\A[ \t]*import[ \t]+([^'"].*?)[ \t]+from[ \t]*['"]([^'"]+)['"]`)

var (
	webNamedClause = regexp.MustCompile(`(?s)\{(.*?)\}`)
	webAliasSplit  = regexp.MustCompile(`\s+as\s+`)
	webIdentifier  = regexp.MustCompile(`^[A-Za-z_$][\w$]*$`)
)

type webBinding struct {
	local, module, exported string
	line                    int
}

// webImportBindings returns the local names a file binds through static named
// imports, in the oracle's `sorted(bindings.items())` order. A type-only clause
// or a type-only item binds no value and is skipped. A statement after a code
// `;` that the oracle pattern did not already cover is read too, but only adds
// a local the oracle reading left unbound: it never rebinds one (GPK-V0-087).
func webImportBindings(text string) []webBinding {
	bindings := make(map[string]webBinding)
	statements := webImportStatement.FindAllStringSubmatchIndex(text, -1)
	for _, match := range statements {
		addWebImportBindings(bindings, text, match, false)
	}
	// covered is the end of the last statement already read at or before start: a `;` inside a
	// statement's own text opens no second statement.
	covered, next := 0, 0
	for _, start := range webCodeSemicolons(text) {
		for ; next < len(statements) && statements[next][0] < start; next++ {
			covered = max(covered, statements[next][1])
		}
		if start < covered {
			continue
		}
		match := webImportAfterSemicolon.FindStringSubmatchIndex(text[start:])
		if match == nil {
			continue
		}
		for at := range match {
			match[at] += start
		}
		addWebImportBindings(bindings, text, match, true)
		covered = max(covered, match[1])
	}
	ordered := make([]webBinding, 0, len(bindings))
	for _, binding := range bindings {
		ordered = append(ordered, binding)
	}
	sort.Slice(ordered, func(left, right int) bool { return ordered[left].local < ordered[right].local })
	return ordered
}

// addWebImportBindings records the named bindings of one matched statement.
// keep leaves an already bound local as it is.
func addWebImportBindings(bindings map[string]webBinding, text string, match []int, keep bool) {
	clause := TrimPythonSpace(text[match[2]:match[3]])
	module := text[match[4]:match[5]]
	if strings.HasPrefix(clause, "type ") {
		return
	}
	named := webNamedClause.FindStringSubmatch(clause)
	if named == nil {
		return
	}
	line := strings.Count(text[:match[0]], "\n") + 1
	for _, raw := range strings.Split(named[1], ",") {
		item := TrimPythonSpace(raw)
		if item == "" || strings.HasPrefix(item, "type ") {
			continue
		}
		parts := webAliasSplit.Split(item, -1)
		exported, local := "", ""
		switch len(parts) {
		case 1:
			exported, local = parts[0], parts[0]
		case 2:
			exported, local = parts[0], parts[1]
		default:
			continue
		}
		if _, bound := bindings[local]; bound && keep {
			continue
		}
		if webIdentifier.MatchString(exported) && webIdentifier.MatchString(local) {
			bindings[local] = webBinding{local, module, exported, line}
		}
	}
}

// webCodeSemicolons returns the offset just past every `;` the web lexer reads
// as code, in order. A `;` inside a string, template literal, regex or comment
// is not one.
func webCodeSemicolons(text string) []int {
	lexer := &webLexer{text: text}
	offsets := []int{}
	for {
		token, ok := lexer.next()
		if !ok {
			return offsets
		}
		if token.kind == webPunct && token.text == ";" {
			offsets = append(offsets, lexer.position)
		}
	}
}

// resolvedWebSource resolves one specifier to the single indexed source it
// names, or to the empty string when it names none or several.
func resolvedWebSource(index *Index, importerPath, imported string) string {
	profile := projectprofile.ByID(index.ProfileID)
	if !webSpecifierAdmitted(imported, profile) {
		return ""
	}
	target := webImportTarget(importerPath, imported, profile)
	candidates := []string{target}
	for _, suffix := range webSuffixOrder {
		candidates = append(candidates, target+suffix)
	}
	for _, suffix := range webSuffixOrder {
		candidates = append(candidates, strings.TrimRight(target, "/")+"/index"+suffix)
	}
	resolved := ""
	for _, candidate := range candidates {
		if _, tracked := index.Sources[candidate]; !tracked {
			continue
		}
		if resolved != "" && resolved != candidate {
			return ""
		}
		resolved = candidate
	}
	return resolved
}

// webSuffixOrder is the oracle's `_WEB_SUFFIXES` tuple. Order is observable:
// the candidate list is built from it and a target matching two of them
// resolves to nothing at all.
var webSuffixOrder = []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"}

// stripWebNonCode blanks comments and string, character and template literals
// while preserving every line position, so a call site's line number survives
// and a name written inside a comment or a literal is not read as one.
func stripWebNonCode(text string) string {
	var output strings.Builder
	output.Grow(len(text))
	const (
		code = iota
		lineComment
		blockComment
		literal
	)
	state, quote := code, byte(0)
	for index := 0; index < len(text); {
		character := text[index]
		following := byte(0)
		if index+1 < len(text) {
			following = text[index+1]
		}
		switch state {
		case lineComment:
			if character == '\n' {
				output.WriteByte('\n')
				state = code
			} else {
				output.WriteByte(' ')
			}
			index++
		case blockComment:
			if character == '*' && following == '/' {
				output.WriteString("  ")
				state, index = code, index+2
				continue
			}
			output.WriteByte(blankOrNewline(character))
			index++
		case literal:
			if character == '\\' {
				output.WriteByte(' ')
				if index+1 < len(text) {
					output.WriteByte(blankOrNewline(following))
					index += 2
				} else {
					index++
				}
				continue
			}
			if character == quote {
				output.WriteByte(' ')
				state = code
			} else {
				output.WriteByte(blankOrNewline(character))
			}
			index++
		default:
			switch {
			case character == '/' && following == '/':
				output.WriteString("  ")
				state, index = lineComment, index+2
			case character == '/' && following == '*':
				output.WriteString("  ")
				state, index = blockComment, index+2
			case character == '`':
				// A template literal is blanked whole: its `${}` substitutions and
				// every literal nested in them are string data here, so a backtick
				// inside a substitution never ends the outer template (GPK-V0-095).
				end := evalWebTemplateEnd(text, index)
				for _, blanked := range []byte(text[index:end]) {
					output.WriteByte(blankOrNewline(blanked))
				}
				index = end
			case character == '\'' || character == '"':
				output.WriteByte(' ')
				quote, state, index = character, literal, index+1
			default:
				output.WriteByte(character)
				index++
			}
		}
	}
	return output.String()
}

// evalWebTemplateEnd returns the offset just past the template literal that
// opens at start, or len(text) when it is unterminated. A backslash escapes the
// next byte, and `${` opens a substitution that evalWebSubstitutionEnd skips.
func evalWebTemplateEnd(text string, start int) int {
	for index := start + 1; index < len(text); {
		switch {
		case text[index] == '\\':
			index += 2
		case text[index] == '`':
			return index + 1
		case text[index] == '$' && index+1 < len(text) && text[index+1] == '{':
			index = evalWebSubstitutionEnd(text, index+2)
		default:
			index++
		}
	}
	return len(text)
}

// evalWebSubstitutionEnd returns the offset just past the `}` that closes a
// template substitution whose body starts at start, or len(text) when none
// does. Braces nest; comments and quoted or template literals inside the body
// are skipped, so a brace or backtick they hold neither closes it nor opens one.
func evalWebSubstitutionEnd(text string, start int) int {
	depth := 1
	for index := start; index < len(text); {
		character := text[index]
		following := byte(0)
		if index+1 < len(text) {
			following = text[index+1]
		}
		switch {
		case character == '/' && following == '/':
			index = evalWebLineCommentEnd(text, index+2)
		case character == '/' && following == '*':
			if close := strings.Index(text[index+2:], "*/"); close >= 0 {
				index += close + 4
			} else {
				return len(text)
			}
		case character == '`':
			index = evalWebTemplateEnd(text, index)
		case character == '\'' || character == '"':
			index = evalWebQuotedEnd(text, index)
		case character == '{':
			depth, index = depth+1, index+1
		case character == '}':
			if depth--; depth == 0 {
				return index + 1
			}
			index++
		default:
			index++
		}
	}
	return len(text)
}

// evalWebLineCommentEnd returns the offset of the ECMAScript line terminator
// (LF, CR, U+2028 or U+2029) that ends a `//` comment whose text starts at
// start, or len(text) when none does. Regular-expression literals are not read
// (a GPK-V0-095 non-goal), so a `//` inside one is still read as a comment.
func evalWebLineCommentEnd(text string, start int) int {
	for index := start; index < len(text); index++ {
		switch {
		case text[index] == '\n' || text[index] == '\r':
			return index
		case strings.HasPrefix(text[index:], "\u2028") || strings.HasPrefix(text[index:], "\u2029"):
			return index
		}
	}
	return len(text)
}

// evalWebQuotedEnd returns the offset just past the quoted literal that opens
// at start, ending, like the mask's literal state, at the next unescaped
// matching quote, or len(text) when there is none.
func evalWebQuotedEnd(text string, start int) int {
	for index := start + 1; index < len(text); index++ {
		switch text[index] {
		case '\\':
			index++
		case text[start]:
			return index + 1
		}
	}
	return len(text)
}

func blankOrNewline(character byte) byte {
	if character == '\n' {
		return '\n'
	}
	return ' '
}
