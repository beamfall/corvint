package requestpath

import (
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
)

// tok aliases the application map's exported JavaScript/TypeScript token, so this reader shares
// one lexer with every other JS reader (AMAP-V0-130).
type tok = appmap.JSToken

// body is one function, method or initializer: the token range [start, end) of a file and the
// parameter names, in order, of a function.
type body struct {
	file   *jsFile
	start  int
	end    int
	line   int
	name   string
	params []string
	owner  map[string]*body // the object literal's or class's members, for `this.m()`
}

// binding is a name a module import or require introduces: the module specifier and the imported
// name ("default", "*" for a namespace, "module" for a CommonJS require of the whole module).
type binding struct {
	module string
	name   string
}

// jsFile is one parsed source file: its top-level declarations, imports and exports.
type jsFile struct {
	path    string
	toks    []tok
	unsure  bool
	funcs   map[string]*body            // top-level functions, function-valued consts and classes' constructors
	values  map[string]*body            // other top-level initializers (statement token ranges)
	members map[string]map[string]*body // top-level object literals and classes: member -> body
	newOf   map[string]string           // top-level `const x = new C(...)`: x -> C
	derived map[string]string           // top-level `const x = y.create(...)`: x -> y
	imports map[string]binding
	exports map[string]string // export name ("default" too) -> local name
	cjs     string            // `module.exports = NAME` or "" (an object literal is members["module.exports"])
}

func isPunct(t tok, p string) bool { return t.Kind == appmap.JSPunct && t.Text == p }
func isIdent(t tok, s string) bool { return t.Kind == appmap.JSIdent && t.Text == s }
func isString(t tok) bool {
	return (t.Kind == appmap.JSString || t.Kind == appmap.JSTemplate) && t.Literal
}

// closeOf returns the index of the bracket closing the one at i, or len(toks) when unbalanced.
func closeOf(toks []tok, i int) int {
	depth := 0
	for j := i; j < len(toks); j++ {
		if toks[j].Kind != appmap.JSPunct {
			continue
		}
		switch toks[j].Text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return len(toks)
}

var statementStarts = map[string]bool{"const": true, "let": true, "var": true, "function": true, "export": true,
	"import": true, "class": true, "module": true, "exports": true, "if": true, "return": true, "async": true}

// statementEnd returns the end (exclusive) of the expression starting at i: a `;` or `,` at its
// own depth, a bracket closing an enclosing one, or a statement keyword on a later line where the
// previous token cannot continue the expression.
func statementEnd(toks []tok, i int) int {
	depth := 0
	for j := i; j < len(toks); j++ {
		t := toks[j]
		if t.Kind == appmap.JSPunct {
			switch t.Text {
			case "(", "[", "{":
				depth++
			case ")", "]", "}":
				if depth == 0 {
					return j
				}
				depth--
			case ";", ",":
				if depth == 0 {
					return j
				}
			}
			continue
		}
		if depth == 0 && j > i && t.Line > toks[j-1].Line && t.Kind == appmap.JSIdent && statementStarts[t.Text] {
			prev := toks[j-1]
			if prev.Kind != appmap.JSPunct || prev.Text == ")" || prev.Text == "]" || prev.Text == "}" {
				return j
			}
		}
	}
	return len(toks)
}

// functionAt reads a function expression or arrow starting at i (after any `async`): its
// parameters and body range. ok is false when no function starts there.
func functionAt(f *jsFile, i int, name string) (*body, bool) {
	toks := f.toks
	if i < len(toks) && isIdent(toks[i], "async") {
		i++
	}
	if i >= len(toks) {
		return nil, false
	}
	params := []string{}
	if isIdent(toks[i], "function") {
		j := i + 1
		if j < len(toks) && isPunct(toks[j], "*") {
			j++
		}
		if j < len(toks) && toks[j].Kind == appmap.JSIdent {
			j++
		}
		j = skipTypeParams(toks, j)
		if j >= len(toks) || !isPunct(toks[j], "(") {
			return nil, false
		}
		end := closeOf(toks, j)
		params = paramNames(toks, j+1, end)
		k := skipToBrace(toks, end+1)
		if k >= len(toks) {
			return nil, false
		}
		return &body{file: f, start: j, end: closeOf(toks, k) + 1, line: toks[i].Line, name: name, params: params}, true
	}
	var arrow int
	start := i
	switch {
	case toks[i].Kind == appmap.JSIdent && i+2 < len(toks) && isPunct(toks[i+1], "=") && isPunct(toks[i+2], ">"):
		params, arrow = []string{toks[i].Text}, i+3
	case isPunct(toks[i], "(") || isPunct(toks[i], "<"):
		j := skipTypeParams(toks, i)
		if j >= len(toks) || !isPunct(toks[j], "(") {
			return nil, false
		}
		end := closeOf(toks, j)
		params = paramNames(toks, j+1, end)
		start = j
		k := end + 1
		if k < len(toks) && isPunct(toks[k], ":") { // a return type annotation
			for k < len(toks) && !(isPunct(toks[k], "=") && k+1 < len(toks) && isPunct(toks[k+1], ">")) {
				k++
			}
		}
		if k+1 >= len(toks) || !isPunct(toks[k], "=") || !isPunct(toks[k+1], ">") {
			return nil, false
		}
		arrow = k + 2
	default:
		return nil, false
	}
	if arrow >= len(toks) {
		return nil, false
	}
	if isPunct(toks[arrow], "{") {
		return &body{file: f, start: start, end: closeOf(toks, arrow) + 1, line: toks[i].Line, name: name, params: params}, true
	}
	return &body{file: f, start: start, end: statementEnd(toks, arrow), line: toks[i].Line, name: name, params: params}, true
}

func skipTypeParams(toks []tok, j int) int {
	if j < len(toks) && isPunct(toks[j], "<") {
		depth := 0
		for ; j < len(toks); j++ {
			if isPunct(toks[j], "<") {
				depth++
			} else if isPunct(toks[j], ">") {
				depth--
				if depth == 0 {
					return j + 1
				}
			}
		}
	}
	return j
}

func skipToBrace(toks []tok, j int) int {
	for ; j < len(toks); j++ {
		if isPunct(toks[j], "{") {
			return j
		}
		if isPunct(toks[j], ";") {
			return len(toks)
		}
	}
	return j
}

// paramNames lists the top-level identifier of each parameter in (start, end); a destructured or
// otherwise unnamed parameter reads "".
func paramNames(toks []tok, start, end int) []string {
	out := []string{}
	expect := true
	depth := 0
	for j := start; j < end && j < len(toks); j++ {
		t := toks[j]
		if t.Kind == appmap.JSPunct {
			switch t.Text {
			case "(", "[", "{", "<":
				if depth == 0 && expect {
					out, expect = append(out, ""), false
				}
				depth++
			case ")", "]", "}", ">":
				depth--
			case ",":
				if depth == 0 {
					expect = true
				}
			}
			continue
		}
		if depth == 0 && expect && t.Kind == appmap.JSIdent {
			out, expect = append(out, t.Text), false
		}
	}
	return out
}

func parseFile(path, text string) *jsFile {
	toks, _, unsure := appmap.LexJS(text)
	f := &jsFile{path: path, toks: expandTemplates(toks), unsure: unsure, funcs: map[string]*body{}, values: map[string]*body{},
		members: map[string]map[string]*body{}, newOf: map[string]string{}, derived: map[string]string{},
		imports: map[string]binding{}, exports: map[string]string{}}
	toks = f.toks
	for i := 0; i < len(toks); {
		next := f.topLevel(i)
		if next <= i {
			next = i + 1
		}
		i = next
	}
	return f
}

// expandTemplates splices each template literal's substitution expressions in as parenthesised
// tokens after the template, so a `${process.env.X}` read is code like any other.
func expandTemplates(toks []tok) []tok {
	out := make([]tok, 0, len(toks))
	for _, t := range toks {
		out = append(out, t)
		if t.Kind != appmap.JSTemplate || t.Source == "" {
			continue
		}
		for _, sub := range substitutions(t.Source) {
			inner, _, _ := appmap.LexJS(sub.text)
			out = append(out, tok{Kind: appmap.JSPunct, Text: "(", Line: t.Line + sub.line})
			for _, in := range expandTemplates(inner) {
				in.Line += t.Line + sub.line - 1
				out = append(out, in)
			}
			out = append(out, tok{Kind: appmap.JSPunct, Text: ")", Line: t.Line + sub.line})
		}
	}
	return out
}

type substitution struct {
	text string
	line int // newlines before the substitution within the template
}

func substitutions(source string) []substitution {
	out := []substitution{}
	for i := 0; i+1 < len(source); i++ {
		if source[i] != '$' || source[i+1] != '{' {
			continue
		}
		depth, j := 0, i+1
		for ; j < len(source); j++ {
			if source[j] == '{' {
				depth++
			} else if source[j] == '}' {
				depth--
				if depth == 0 {
					break
				}
			}
		}
		if j >= len(source) {
			break
		}
		out = append(out, substitution{text: source[i+2 : j], line: strings.Count(source[:i], "\n")})
		i = j
	}
	return out
}

// topLevel reads the statement at i when it is at depth 0 and returns the index after it.
func (f *jsFile) topLevel(i int) int {
	toks := f.toks
	t := toks[i]
	if t.Kind == appmap.JSPunct {
		if t.Text == "{" || t.Text == "(" || t.Text == "[" {
			return closeOf(toks, i) + 1
		}
		return i + 1
	}
	if t.Kind != appmap.JSIdent || (i > 0 && isPunct(toks[i-1], ".")) {
		return i + 1
	}
	exported := false
	if t.Text == "export" {
		exported = true
		i++
		if i >= len(toks) {
			return i
		}
		if isIdent(toks[i], "default") {
			return f.exportDefault(i + 1)
		}
		if isPunct(toks[i], "{") {
			end := closeOf(toks, i)
			for _, pair := range namePairs(toks, i+1, end) {
				f.exports[pair[1]] = pair[0]
			}
			return end + 1
		}
		t = toks[i]
	}
	switch t.Text {
	case "import":
		return f.importAt(i)
	case "async", "function":
		return f.functionDecl(i, exported)
	case "class":
		return f.classDecl(i, exported)
	case "const", "let", "var":
		return f.declaration(i, exported)
	case "module", "exports":
		return f.cjsExport(i)
	}
	return i + 1
}

// namePairs reads `a, b as c` (or `a, b: c` when colon) between start and end as [local, exported]
// pairs; for an import list the first is the imported name.
func namePairs(toks []tok, start, end int) [][2]string {
	out := [][2]string{}
	for j := start; j < end; j++ {
		if toks[j].Kind != appmap.JSIdent || isIdent(toks[j], "type") && j+1 < end && toks[j+1].Kind == appmap.JSIdent {
			continue
		}
		name, alias := toks[j].Text, toks[j].Text
		if j+2 < end && (isIdent(toks[j+1], "as") || isPunct(toks[j+1], ":")) && toks[j+2].Kind == appmap.JSIdent {
			alias = toks[j+2].Text
			j += 2
		}
		out = append(out, [2]string{name, alias})
		for j+1 < end && !isPunct(toks[j+1], ",") {
			j++
		}
	}
	return out
}

func (f *jsFile) exportDefault(i int) int {
	toks := f.toks
	if i >= len(toks) {
		return i
	}
	if isIdent(toks[i], "function") || isIdent(toks[i], "async") {
		if b, ok := functionAt(f, i, "default"); ok {
			name := "default"
			j := i
			for j < b.start && !isPunct(toks[j], "(") {
				if toks[j].Kind == appmap.JSIdent && !isIdent(toks[j], "async") && !isIdent(toks[j], "function") {
					name = toks[j].Text
				}
				j++
			}
			b.name = name
			f.funcs[name] = b
			f.exports["default"] = name
			return b.end
		}
	}
	if isIdent(toks[i], "class") {
		end := f.classDecl(i, false)
		if i+1 < len(toks) && toks[i+1].Kind == appmap.JSIdent {
			f.exports["default"] = toks[i+1].Text
		}
		return end
	}
	if toks[i].Kind == appmap.JSIdent && i+1 < len(toks) && (isPunct(toks[i+1], ";") || toks[i+1].Line > toks[i].Line) {
		f.exports["default"] = toks[i].Text
		return i + 1
	}
	return f.assign(i, "default", true)
}

func (f *jsFile) importAt(i int) int {
	toks := f.toks
	end := i + 1
	for end < len(toks) && !isIdent(toks[end], "from") && !isString(toks[end]) {
		end++
	}
	if end < len(toks) && isIdent(toks[end], "from") {
		end++
	}
	if end >= len(toks) || !isString(toks[end]) {
		return end
	}
	module := toks[end].Text
	for j := i + 1; j < end; j++ {
		switch {
		case isPunct(toks[j], "{"):
			c := closeOf(toks, j)
			for _, pair := range namePairs(toks, j+1, c) {
				f.imports[pair[1]] = binding{module: module, name: pair[0]}
			}
			j = c
		case isPunct(toks[j], "*") && j+2 < end && isIdent(toks[j+1], "as"):
			f.imports[toks[j+2].Text] = binding{module: module, name: "*"}
			j += 2
		case toks[j].Kind == appmap.JSIdent && !isIdent(toks[j], "type") && !isIdent(toks[j], "from"):
			f.imports[toks[j].Text] = binding{module: module, name: "default"}
		}
	}
	return end + 1
}

func (f *jsFile) functionDecl(i int, exported bool) int {
	toks := f.toks
	j := i
	if isIdent(toks[j], "async") {
		j++
	}
	if j >= len(toks) || !isIdent(toks[j], "function") {
		return i + 1
	}
	k := j + 1
	if k < len(toks) && isPunct(toks[k], "*") {
		k++
	}
	if k >= len(toks) || toks[k].Kind != appmap.JSIdent {
		return i + 1
	}
	name := toks[k].Text
	b, ok := functionAt(f, i, name)
	if !ok {
		return k + 1
	}
	f.funcs[name] = b
	if exported {
		f.exports[name] = name
	}
	return b.end
}

func (f *jsFile) classDecl(i int, exported bool) int {
	toks := f.toks
	if i+1 >= len(toks) || toks[i+1].Kind != appmap.JSIdent {
		return i + 1
	}
	name := toks[i+1].Text
	open := skipToBrace(toks, i+2)
	if open >= len(toks) {
		return open
	}
	end := closeOf(toks, open)
	f.members[name] = f.classMembers(open+1, end)
	if exported {
		f.exports[name] = name
	}
	return end + 1
}

// classMembers reads `name(params) { ... }` methods (any `async`, `static`, `get`, `set` or
// access modifier before them) and `name = arrow` fields between start and end.
func (f *jsFile) classMembers(start, end int) map[string]*body {
	toks := f.toks
	out := map[string]*body{}
	for j := start; j < end; j++ {
		t := toks[j]
		if t.Kind == appmap.JSPunct {
			if t.Text == "{" || t.Text == "(" || t.Text == "[" {
				j = closeOf(toks, j)
			}
			continue
		}
		if t.Kind != appmap.JSIdent || j+1 >= end {
			continue
		}
		if isPunct(toks[j+1], "(") || isPunct(toks[j+1], "<") {
			p := skipTypeParams(toks, j+1)
			if p >= end || !isPunct(toks[p], "(") {
				continue
			}
			c := closeOf(toks, p)
			open := skipToBrace(toks, c+1)
			if open >= end {
				continue
			}
			out[t.Text] = &body{file: f, start: p, end: closeOf(toks, open) + 1, line: t.Line, name: t.Text, params: paramNames(toks, p+1, c)}
			j = closeOf(toks, open)
			continue
		}
		if isPunct(toks[j+1], "=") && !(j+2 < end && isPunct(toks[j+2], "=")) {
			if b, ok := functionAt(f, j+2, t.Text); ok {
				out[t.Text] = b
				j = b.end - 1
			}
		}
	}
	for _, b := range out {
		b.owner = out
	}
	return out
}

// objectMembers reads an object literal's `name(params) {}`, `name: function/arrow`, `name: ref`
// and shorthand `name` members; a member naming another top-level function is resolved later.
func (f *jsFile) objectMembers(open int) map[string]*body {
	toks := f.toks
	end := closeOf(toks, open)
	out := map[string]*body{}
	for j := open + 1; j < end; j++ {
		t := toks[j]
		if t.Kind == appmap.JSPunct {
			if t.Text == "{" || t.Text == "(" || t.Text == "[" {
				j = closeOf(toks, j)
			}
			continue
		}
		if t.Kind != appmap.JSIdent && !isString(t) {
			continue
		}
		key := t.Text
		k := j
		if isIdent(t, "async") && j+1 < end && toks[j+1].Kind == appmap.JSIdent {
			k = j + 1
			key = toks[k].Text
		}
		switch {
		case k+1 < end && isPunct(toks[k+1], "("):
			c := closeOf(toks, k+1)
			open := skipToBrace(toks, c+1)
			if open < end {
				out[key] = &body{file: f, start: k + 1, end: closeOf(toks, open) + 1, line: t.Line, name: key, params: paramNames(toks, k+2, c)}
				j = closeOf(toks, open)
			}
		case k+1 < end && isPunct(toks[k+1], ":"):
			if b, ok := functionAt(f, k+2, key); ok {
				out[key] = b
				j = b.end - 1
			} else if k+2 < end && toks[k+2].Kind == appmap.JSIdent {
				out[key] = &body{file: f, name: toks[k+2].Text, start: -1}
				j = statementEnd(toks, k+2) - 1
			} else {
				j = statementEnd(toks, k+2) - 1
			}
		case k+1 <= end && (k+1 == end || isPunct(toks[k+1], ",")):
			out[key] = &body{file: f, name: key, start: -1} // shorthand: a reference to a top-level name
		}
	}
	for _, b := range out {
		b.owner = out
	}
	return out
}

func (f *jsFile) declaration(i int, exported bool) int {
	toks := f.toks
	j := i + 1
	if j >= len(toks) {
		return j
	}
	if isPunct(toks[j], "{") { // destructuring
		c := closeOf(toks, j)
		end := statementEnd(toks, c+1)
		if c+3 < len(toks) && isPunct(toks[c+1], "=") && isIdent(toks[c+2], "require") && c+5 < len(toks) && isString(toks[c+4]) {
			for _, pair := range namePairs(toks, j+1, c) {
				f.imports[pair[1]] = binding{module: toks[c+4].Text, name: pair[0]}
			}
			return end
		}
		stmt := &body{file: f, start: i, end: end, line: toks[i].Line}
		for _, pair := range namePairs(toks, j+1, c) {
			f.values[pair[1]] = stmt
		}
		return end
	}
	if toks[j].Kind != appmap.JSIdent {
		return j
	}
	name := toks[j].Text
	k := j + 1
	for k < len(toks) && !isPunct(toks[k], "=") && !isPunct(toks[k], ";") && toks[k].Line == toks[j].Line {
		k++ // a type annotation
	}
	if k >= len(toks) || !isPunct(toks[k], "=") {
		return k
	}
	e := f.assign(k+1, name, exported)
	// `const a = 1, b = process.env.B`: read each later declarator too.
	for e+3 < len(toks) && isPunct(toks[e], ",") && toks[e+1].Kind == appmap.JSIdent && isPunct(toks[e+2], "=") && !isPunct(toks[e+3], "=") {
		e = f.assign(e+3, toks[e+1].Text, exported)
	}
	return e
}

// assign reads the initializer at i bound to name.
func (f *jsFile) assign(i int, name string, exported bool) int {
	toks := f.toks
	if exported {
		f.exports[name] = name
	}
	if i >= len(toks) {
		return i
	}
	if b, ok := functionAt(f, i, name); ok {
		f.funcs[name] = b
		return b.end
	}
	end := statementEnd(toks, i)
	start := i
	if isIdent(toks[start], "await") {
		start++
	}
	switch {
	case isIdent(toks[start], "require") && start+3 < len(toks) && isPunct(toks[start+1], "(") && isString(toks[start+2]):
		f.imports[name] = binding{module: toks[start+2].Text, name: "module"}
		if start+4 < len(toks) && isPunct(toks[start+4], ".") && start+5 < len(toks) && toks[start+5].Kind == appmap.JSIdent {
			f.imports[name] = binding{module: toks[start+2].Text, name: toks[start+5].Text}
		}
	case isPunct(toks[start], "{"):
		f.members[name] = f.objectMembers(start)
	case isIdent(toks[start], "new") && start+1 < len(toks) && toks[start+1].Kind == appmap.JSIdent:
		f.newOf[name] = toks[start+1].Text
	case toks[start].Kind == appmap.JSIdent && start+3 < len(toks) && isPunct(toks[start+1], ".") && isIdent(toks[start+2], "create") && isPunct(toks[start+3], "("):
		f.derived[name] = toks[start].Text
	}
	f.values[name] = &body{file: f, start: i, end: end, line: toks[i].Line, name: name}
	return end
}

// cjsExport reads `module.exports = X`, `module.exports.name = X` and `exports.name = X`.
func (f *jsFile) cjsExport(i int) int {
	toks := f.toks
	j := i
	if isIdent(toks[j], "module") {
		if j+2 >= len(toks) || !isPunct(toks[j+1], ".") || !isIdent(toks[j+2], "exports") {
			return i + 1
		}
		j += 2
	}
	if j+1 >= len(toks) {
		return j
	}
	if isPunct(toks[j+1], "=") && j+2 < len(toks) && !isPunct(toks[j+2], "=") {
		v := j + 2
		if v < len(toks) && toks[v].Kind == appmap.JSIdent && v+1 < len(toks) && (isPunct(toks[v+1], ";") || toks[v+1].Line > toks[v].Line) {
			f.cjs = toks[v].Text
			return v + 1
		}
		if v < len(toks) && isPunct(toks[v], "{") {
			f.members["module.exports"] = f.objectMembers(v)
			return closeOf(toks, v) + 1
		}
		return f.assign(v, "module.exports", false)
	}
	if isPunct(toks[j+1], ".") && j+3 < len(toks) && toks[j+2].Kind == appmap.JSIdent && isPunct(toks[j+3], "=") {
		name := toks[j+2].Text
		v := j + 4
		if v < len(toks) && toks[v].Kind == appmap.JSIdent && v+1 < len(toks) && (isPunct(toks[v+1], ";") || toks[v+1].Line > toks[v].Line) {
			f.exports[name] = toks[v].Text
			return v + 1
		}
		f.exports[name] = "exports." + name
		return f.assign(v, "exports."+name, false)
	}
	return j + 1
}
