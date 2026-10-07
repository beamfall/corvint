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

var testIDAttribute = regexp.MustCompile(`^\[data-test(?:id|-id)?=["']?([^"'\]]+)["']?\]$`)

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
}

type rawGoto struct {
	line   int
	url    string
	reason string
}

type rawMethod struct {
	name       string
	start, end int
	noArgs     bool
}

type rawNew struct {
	class string
	line  int
}

var (
	methodLine = regexp.MustCompile(`^\s*(?:(?:public|private|protected|static|readonly|override|async)\s+)*(?:get\s+)?([A-Za-z_$][\w$]*)\s*(?:<[^>]*>)?\s*\([^)]*\)\s*(?::[^{=;]*)?\{\s*$`)
	arrowLine  = regexp.MustCompile(`^\s*(?:(?:public|private|protected|static|readonly)\s+)*([A-Za-z_$][\w$]*)\s*=\s*(?:async\s+)?\([^)]*\)\s*(?::[^{=;]*)?=>\s*\{\s*$`)
	funcLine   = regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?(?:async\s+)?function\s*\*?\s*([A-Za-z_$][\w$]*)\s*(?:<[^>]*>)?\s*\(`)
	classLine  = regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?(?:abstract\s+)?class\s+([A-Za-z_$][\w$]*)`)
	notMethod  = map[string]bool{"if": true, "for": true, "while": true, "switch": true, "catch": true, "function": true, "return": true, "constructor": true, "with": true}
)

// readFacts scans one test source lexically (AMAP-V0-005, AMAP-V0-007).
func readFacts(text string) fileFacts {
	toks, code := lexJS(text)
	f := fileFacts{}
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
				if name, named = roleName(toks, i+4, lit); !named {
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
			f.news = append(f.news, rawNew{class: t.text, line: t.line})
		}
	}
	lines := strings.Split(code, "\n")
	for n, l := range lines {
		if f.class == "" {
			if m := classLine.FindStringSubmatch(l); m != nil {
				f.class = m[1]
			}
		}
		name := ""
		for _, re := range []*regexp.Regexp{methodLine, arrowLine, funcLine} {
			if m := re.FindStringSubmatch(l); m != nil && !notMethod[m[1]] {
				name = m[1]
				break
			}
		}
		if name == "" {
			continue
		}
		if end := blockEnd(lines, n); end > n {
			f.methods = append(f.methods, rawMethod{name: name, start: n + 1, end: end + 1, noArgs: emptyParams(l, name)})
		}
	}
	return f
}

// emptyParams reports whether the first parameter list after name on a method header line is
// literally empty. A multi-line or defaulted list reads false.
func emptyParams(line, name string) bool {
	rest := line[strings.Index(line, name)+len(name):]
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
	out := []string{}
	for k := start; k < len(lines) && k < start+8; k++ {
		out = append(out, strings.TrimRight(lines[k], " \t\r"))
		if strings.Contains(lines[k], specifier) {
			break
		}
	}
	s := strings.Join(out, "\n")
	if len(s) > 512 {
		return ""
	}
	return s
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
