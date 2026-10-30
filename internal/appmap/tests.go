package appmap

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// Selector strength classes (AMAP-V0-007).
const (
	strengthStrong  = "strong"
	strengthMedium  = "medium"
	strengthWeak    = "weak"
	strengthUnknown = "unknown"
)

var selectorKinds = map[string]struct{ kind, strength string }{
	"getByTestId":      {"test-id", strengthStrong},
	"getByRole":        {"role", strengthMedium},
	"getByLabel":       {"label", strengthMedium},
	"getByPlaceholder": {"placeholder", strengthMedium},
	"getByAltText":     {"alt-text", strengthMedium},
	"getByTitle":       {"title", strengthMedium},
	"getByText":        {"text", strengthWeak},
	"locator":          {"css", strengthWeak},
}

// Only data-testid is the attribute getByTestId reads; data-test and data-test-id are other
// attributes, so they stay CSS and never share a test-id selector.
var testIDAttribute = regexp.MustCompile(`^\[data-testid=["']?([^"'\]]+)["']?\]$`)

// newSelector builds a content-addressed selector: the same kind, value and name keep one ID in
// every file and at every revision.
func newSelector(kind, value, name, strength string, line int) Selector {
	sum := sha256.Sum256([]byte(kind + "\x00" + value + "\x00" + name))
	return Selector{ID: "selector:" + hex.EncodeToString(sum[:8]), Kind: kind, Value: value, Name: name, Strength: strength, Line: line}
}

// fileFacts is the lexical reading of one test source.
type fileFacts struct {
	class     string
	selectors []Selector
	gotos     []rawGoto
	tests     []string
	asserts   int
	methods   []rawMethod
	news      []rawNew
	secrets   []int
	helpers   map[string]int // navigation helpers by name, the URL parameter's position (AMAP-V0-342)
	// helperForms and helperGaps are a helper's URL form and, for a function whose navigation the
	// reader does not follow, `line:reason` of its first such call (AMAP-V0-367).
	helperForms map[string]helperForm
	helperGaps  map[string]string
	calls       []rawCall
	// navBase, navMethods, navDeclared and thisCalls are the first class's `extends` name, its
	// methods that may navigate through a parameter, every method it declares and the
	// `this.m(...)` calls in its body (AMAP-V0-370).
	navBase     string
	navMethods  map[string][]navMethod
	navDeclared map[string]bool
	thisCalls   []rawCall
	// classDefault is true when the class is declared `export default class` (V1-0974).
	classDefault bool
	// defaultImports maps each local name a static import binds by its default clause to the
	// statement's specifier (AMAP-V0-394).
	defaultImports map[string]string
}

type rawGoto struct {
	line   int
	url    string
	reason string
}

type rawMethod struct {
	name       string
	start, end int
	callable   bool
	member     bool // declared directly in the class body
}

type rawNew struct {
	class string
	line  int
	at    int // index of the class token in lexJS's token stream
}

var (
	methodLine = regexp.MustCompile(`^\s*(?:(?:public|private|protected|static|readonly|override|async)\s+)*(?:get\s+)?([A-Za-z_$][\w$]*)\s*(?:<[^>]*>)?\s*\([^)]*\)\s*(?::[^{=;]*)?\{\s*$`)
	arrowLine  = regexp.MustCompile(`^\s*(?:(?:public|private|protected|static|readonly)\s+)*([A-Za-z_$][\w$]*)\s*=\s*(?:async\s+)?\([^)]*\)\s*(?::[^{=;]*)?=>\s*\{\s*$`)
	funcLine   = regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?(?:async\s+)?function\s*\*?\s*([A-Za-z_$][\w$]*)\s*(?:<[^>]*>)?\s*\(`)
	classLine  = regexp.MustCompile(`^\s*(?:export\s+)?(default\s+)?(?:abstract\s+)?class\s+([A-Za-z_$][\w$]*)`)
	notMethod  = map[string]bool{"if": true, "for": true, "while": true, "switch": true, "catch": true, "function": true, "return": true, "constructor": true, "with": true}
)

// readFacts scans one test source lexically (AMAP-V0-005, AMAP-V0-007).
func readFacts(text string) fileFacts {
	toks, code := lexJS(text)
	f := fileFacts{calls: plainCalls(toks)}
	f.helpers, f.helperForms, f.helperGaps = navHelpers(toks)
	var body []token
	f.navBase, f.navMethods, f.navDeclared, body = classNav(toks)
	f.thisCalls = thisCalls(body)
	f.defaultImports = defaultImports(toks)
	lit := func(t token) (string, bool) {
		if literal(t) {
			return t.text, true
		}
		return "", false
	}
	// whole reports whether toks[j] is a complete argument: an expression such as
	// '/home' + suffix is not a literal even though it starts with one.
	whole := func(j int) bool {
		return j+1 < len(toks) && (isPunct(toks[j+1], ")") || isPunct(toks[j+1], ","))
	}
	for i := 0; i+1 < len(toks); i++ {
		t := toks[i]
		if t.kind != tokIdent || !isPunct(toks[i+1], "(") {
			continue
		}
		dotted := i > 0 && isPunct(toks[i-1], ".")
		switch {
		case dotted && selectorKinds[t.text].kind != "":
			sk := selectorKinds[t.text]
			arg := token{}
			if i+2 < len(toks) {
				arg = toks[i+2]
			}
			value, ok := lit(arg)
			if !ok || !whole(i+2) {
				f.selectors = append(f.selectors, newSelector(sk.kind, "", "", strengthUnknown, t.line))
				continue
			}
			if secretscreen.MatchString(value) {
				f.secrets = append(f.secrets, t.line)
				continue
			}
			kind, strength, name := sk.kind, sk.strength, ""
			if t.text == "locator" {
				if m := testIDAttribute.FindStringSubmatch(value); m != nil {
					kind, strength, value = "test-id", strengthStrong, m[1]
				} else if strings.HasPrefix(value, "//") || strings.HasPrefix(value, "xpath=") {
					kind = "xpath"
				} else if strings.HasPrefix(value, "text=") {
					kind = "text"
				}
			}
			if t.text == "getByRole" && i+4 < len(toks) && isPunct(toks[i+3], ",") && !isPunct(toks[i+4], ")") && !isPunct(toks[i+4], "{") {
				// options passed by reference may carry any name
				f.selectors = append(f.selectors, newSelector(sk.kind, "", "", strengthUnknown, t.line))
				continue
			}
			if t.text == "getByRole" && i+4 < len(toks) && isPunct(toks[i+3], ",") && isPunct(toks[i+4], "{") {
				var named bool
				// The literal object must be the whole options argument; ({...} && options)
				// passes options, whose name the map cannot read.
				end := closeParen(toks, i+4)
				whole := next(toks, end+1, ")") || (next(toks, end+1, ",") && next(toks, end+2, ")"))
				if name, named = roleName(toks, i+4, lit); !named || !whole {
					f.selectors = append(f.selectors, newSelector(sk.kind, "", "", strengthUnknown, t.line))
					continue
				}
			}
			f.selectors = append(f.selectors, newSelector(kind, value, name, strength, t.line))
		case dotted && (t.text == "goto" || t.text == "waitForURL" || t.text == "toHaveURL"):
			g := rawGoto{line: t.line}
			if i+2 < len(toks) && (toks[i+2].kind == tokString || toks[i+2].kind == tokTemplate) && !toks[i+2].inexact && whole(i+2) {
				g.url = toks[i+2].text
				if secretscreen.MatchString(g.url) {
					g.url, g.reason = "", "secret-shaped"
				}
			} else {
				g.reason = "non-literal-url"
			}
			f.gotos = append(f.gotos, g)
		case t.text == "expect" && !dotted:
			f.asserts++
		case (t.text == "test" || t.text == "it") && !dotted:
			if i+2 < len(toks) {
				if title, ok := lit(toks[i+2]); ok && len(f.tests) < 256 && !secretscreen.MatchString(title) {
					f.tests = append(f.tests, title)
				}
			}
		}
		if t.text != "" && i > 0 && toks[i-1].kind == tokIdent && toks[i-1].text == "new" {
			f.news = append(f.news, rawNew{class: t.text, line: t.line, at: i})
		}
	}
	lines := strings.Split(code, "\n")
	// depth[n] is the brace depth at the start of line n, counted as blockEnd counts it, so a
	// member sits in the class body only at one level below the class line (V1-0968).
	depth := make([]int, len(lines))
	for n, d := 0, 0; n < len(lines); n++ {
		depth[n] = d
		d += strings.Count(lines[n], "{") - strings.Count(lines[n], "}")
	}
	classStart, classEnd, lastEnd := -1, -1, -1
	for n, l := range lines {
		if f.class == "" {
			if m := classLine.FindStringSubmatch(l); m != nil {
				f.class, f.classDefault, classStart, classEnd = m[2], m[1] != "", n, blockEnd(lines, n)
			}
		}
		name, member := "", false
		for k, re := range []*regexp.Regexp{methodLine, arrowLine, funcLine} {
			if m := re.FindStringSubmatch(l); m != nil && !notMethod[m[1]] {
				// A member is callable on the class only when it sits directly in the class body:
				// not before or after it, not inside another method, and not in an object literal
				// assigned to a class field (an object literal's method).
				name, member = m[1], k < 2 && n > classStart && n < classEnd && n > lastEnd && depth[n] == depth[classStart]+1
				break
			}
		}
		if name == "" {
			continue
		}
		if end := blockEnd(lines, n); end > n {
			f.methods = append(f.methods, rawMethod{name: name, start: n + 1, end: end + 1, callable: member && callable(l, name), member: member})
			lastEnd = max(lastEnd, end)
		}
	}
	return f
}

// defaultImports returns the local name each static `import Name from 'm'` or
// `import Name, {...} from 'm'` of toks binds to the default export of m, keyed to m; a type-only
// import binds no value, and a name two statements bind is dropped (AMAP-V0-394).
func defaultImports(toks []token) map[string]string {
	out, twice := map[string]string{}, map[string]bool{}
	for i := 0; i+1 < len(toks); i++ {
		if !word(toks, i, "import") || i > 0 && isPunct(toks[i-1], ".") || toks[i+1].kind != tokIdent || toks[i+1].text == "type" {
			continue
		}
		name, j := toks[i+1].text, i+2
		if next(toks, j, ",") {
			j++
			if next(toks, j, "{") {
				j = closeParen(toks, j) + 1
			} else if next(toks, j, "*") {
				j += 3 // `* as NS`
			}
		}
		if !word(toks, j, "from") || j+1 >= len(toks) || !literal(toks[j+1]) {
			continue
		}
		if _, dup := out[name]; dup {
			twice[name] = true
		}
		out[name] = toks[j+1].text
	}
	for name := range twice {
		delete(out, name)
	}
	return out
}

// hiddenMember matches the modifiers that make a class member unreachable as instance.name().
var hiddenMember = regexp.MustCompile(`\b(?:private|protected|static|get|set)\b`)

// callable reports whether a class member header declares a public instance method whose first
// parameter list is literally empty: no getter, setter, static, private or protected member, and no
// multi-line or defaulted parameter list.
func callable(line, name string) bool {
	at := strings.Index(line, name)
	if hiddenMember.MatchString(line[:at]) {
		return false
	}
	rest := line[at+len(name):]
	lp := strings.IndexByte(rest, '(')
	if lp < 0 {
		return false
	}
	rp := strings.IndexByte(rest[lp:], ')')
	return rp > 0 && strings.TrimSpace(rest[lp+1:lp+rp]) == ""
}

// roleName reads the literal name property of the getByRole options object opening at toks[open].
// It reports false when the object could set a name it cannot read: a spread, a computed or
// shorthand key, or a name whose value is not one exact literal (AMAP-V0-007).
func roleName(toks []token, open int, lit func(token) (string, bool)) (string, bool) {
	depth, name, named := 0, "", false
	for j := open; j < len(toks); j++ {
		keyPos := j > open && (isPunct(toks[j-1], "{") || isPunct(toks[j-1], ",")) && depth == 1
		switch {
		case keyPos && (isPunct(toks[j], ".") || isPunct(toks[j], "[")):
			return "", false
		case isPunct(toks[j], "{") || isPunct(toks[j], "(") || isPunct(toks[j], "["):
			depth++
		case isPunct(toks[j], "}") || isPunct(toks[j], ")") || isPunct(toks[j], "]"):
			depth--
			if depth == 0 {
				return name, true
			}
		case keyPos && (toks[j].text == "name" && (toks[j].kind == tokIdent || literal(toks[j]))):
			if named || j+3 >= len(toks) || !isPunct(toks[j+1], ":") || !(isPunct(toks[j+3], ",") || isPunct(toks[j+3], "}")) {
				return "", false
			}
			n, ok := lit(toks[j+2])
			if !ok || secretscreen.MatchString(n) {
				return "", false
			}
			name, named = n, true
		}
	}
	return "", false
}

// blockEnd returns the index of the line whose '}' closes the first '{' opened on line start.
func blockEnd(lines []string, start int) int {
	depth, opened := 0, false
	for n := start; n < len(lines); n++ {
		for _, c := range lines[n] {
			switch c {
			case '{':
				depth++
				opened = true
			case '}':
				depth--
				if opened && depth == 0 {
					return n
				}
			}
		}
	}
	return -1
}

// importStatus classifies a specifier the index did not resolve (AMAP-V0-005). A relative
// specifier is a first-party import that names no single file; a bare specifier is external only
// when it names a declared package dependency or a Node built-in. Anything else (a path alias
// whose resolution the index does not yet support, V1-0958) is unresolved, so the join through it
// stays UNKNOWN rather than silently complete.
func importStatus(specifier string, packages map[string]bool) string {
	if strings.HasPrefix(specifier, ".") || strings.HasPrefix(specifier, "/") {
		return importUnresolved
	}
	if strings.HasPrefix(specifier, "node:") || nodeBuiltins[specifier] {
		return importExternal
	}
	if packages[packageName(specifier)] {
		return importExternal
	}
	return importUnresolved
}

// packageName is the npm package a bare specifier names: `@scope/name` or `name`.
func packageName(specifier string) string {
	parts := strings.SplitN(specifier, "/", 3)
	if strings.HasPrefix(specifier, "@") && len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

var nodeBuiltins = map[string]bool{"assert": true, "buffer": true, "child_process": true, "crypto": true, "events": true,
	"fs": true, "fs/promises": true, "http": true, "https": true, "os": true, "path": true, "process": true, "stream": true,
	"url": true, "util": true, "zlib": true}

// importLine finds the first line of text whose code quotes specifier, for an import the index
// recorded without a position.
func importLine(text, specifier string) int {
	for n, l := range strings.Split(text, "\n") {
		if strings.Contains(l, `'`+specifier+`'`) || strings.Contains(l, `"`+specifier+`"`) {
			return n + 1
		}
	}
	return 0
}

// statementAt returns the import statement text starting at line, up to the line that quotes
// specifier, bounded to 8 lines and 512 bytes.
func statementAt(text string, line int, specifier string) string {
	lines := strings.Split(text, "\n")
	if line < 1 || line > len(lines) {
		return ""
	}
	start := line - 1
	for k := start; k >= 0 && k > start-8; k-- {
		if strings.Contains(lines[k], "import") {
			start = k
			break
		}
	}
	out, found := []string{}, false
	for k := start; k < len(lines) && k < start+8; k++ {
		out = append(out, strings.TrimRight(lines[k], " \t\r"))
		if strings.Contains(lines[k], specifier) {
			found = true
			break
		}
	}
	s := strings.Join(out, "\n")
	if !found || len(s) > 512 {
		return "" // an unfinished or oversized statement is not read
	}
	return oneStatement(s, specifier)
}

// oneStatement cuts s to the one import statement whose module string is specifier, reading s
// lexically: an import keyword, semicolon or specifier inside a string or comment never starts,
// ends or matches a statement, and another statement sharing a line is not copied with it. A
// statement with a comment inside, or one whose boundaries cannot be read (an unterminated string
// or comment, a stray comment end), is refused as "" so the caller emits it UNRESOLVED (V1-0968).
func oneStatement(s, specifier string) string {
	start, commented := -1, false
	for i := 0; i < len(s); {
		switch c := s[i]; {
		case strings.HasPrefix(s[i:], "//"):
			commented = commented || start >= 0
			if nl := strings.IndexByte(s[i:], '\n'); nl >= 0 {
				i += nl
			} else {
				i = len(s)
			}
		case strings.HasPrefix(s[i:], "/*"):
			e := strings.Index(s[i+2:], "*/")
			if e < 0 {
				return ""
			}
			commented = commented || start >= 0
			i += e + 4
		case strings.HasPrefix(s[i:], "*/"):
			return "" // s may begin inside a block comment
		case c == '\'' || c == '"' || c == '`':
			j := i + 1
			for ; j < len(s) && s[j] != c; j++ {
				if s[j] == '\\' {
					j++
				} else if s[j] == '\n' && c != '`' {
					return ""
				}
			}
			if j >= len(s) {
				return ""
			}
			if start >= 0 && s[i+1:j] == specifier {
				if commented {
					return ""
				}
				end := j + 1
				if rest := strings.TrimLeft(s[end:], " \t"); strings.HasPrefix(rest, ";") {
					end = len(s) - len(rest) + 1
				}
				return s[start:end]
			}
			i = j + 1
		case c == ';':
			start, commented = -1, false
			i++
		case strings.HasPrefix(s[i:], "import") && !identByte(s, i-1) && !identByte(s, i+len("import")):
			start, commented = i, false
			i += len("import")
		default:
			i++
		}
	}
	return ""
}

// identByte reports whether s[i] exists and can be part of a JavaScript identifier.
func identByte(s string, i int) bool {
	if i < 0 || i >= len(s) {
		return false
	}
	c := s[i]
	return c == '_' || c == '$' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

func fileID(p string) string          { return "file:" + p }
func methodID(p, name string) string  { return "method:" + p + "#" + name }
func refAt(p string, line int) string { return fmt.Sprintf("%s:%d", p, line) }
func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}
