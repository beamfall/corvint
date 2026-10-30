package appmap

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
)

type reactParser struct {
	e        blobEntry
	data     []byte
	states   []rawState
	unknowns []Unknown
	ordinal  int
	anchor   Anchor
}

// reactImporter reads the module a relative import specifier names at the build revision for the
// one import hop of AMAP-V0-079; why names the unknown reason when it cannot.
type reactImporter func(spec string) (e blobEntry, data []byte, why string)

// reactImportExtensions are the module files a relative specifier without an extension may name.
var reactImportExtensions = []string{".ts", ".tsx", ".js", ".jsx"}

// readReactRouter compiles one React router file, following an imported route array through one
// literal named-import hop at the same revision (AMAP-V0-079). The read shares the router bound.
func readReactRouter(r repo, rev string, e blobEntry, data []byte, remaining *int) ([]rawState, []Unknown, error) {
	var hopErr error
	imp := func(spec string) (blobEntry, []byte, string) {
		base := path.Join(path.Dir(e.path), spec)
		if !safeRelative(base) {
			return blobEntry{}, nil, "unresolved-react-routes-import"
		}
		cands := []string{base}
		if !containsString(reactImportExtensions, path.Ext(base)) {
			cands = nil
			for _, ext := range reactImportExtensions {
				cands = append(cands, base+ext)
			}
			for _, ext := range reactImportExtensions {
				cands = append(cands, base+"/index"+ext)
			}
		}
		var found []blobEntry
		var bodies [][]byte
		for _, c := range cands {
			if *remaining == 0 {
				hopErr = bound("more than 64 router files including React route imports")
				return blobEntry{}, nil, ""
			}
			fe, fd, err := r.readOne(rev, c, maxRouterBytes)
			if err != nil {
				var coded *gokernel.Error
				if errors.As(err, &coded) && coded.Code == "appmap-bound-exceeded" {
					hopErr = err
					return blobEntry{}, nil, ""
				}
				continue
			}
			found, bodies = append(found, fe), append(bodies, fd)
		}
		switch len(found) {
		case 0:
			return blobEntry{}, nil, "unresolved-react-routes-import"
		case 1:
			*remaining--
			return found[0], bodies[0], ""
		}
		return blobEntry{}, nil, "ambiguous-react-routes-import"
	}
	states, unknowns := parseReactRouterWith(e, data, imp)
	if hopErr != nil {
		return nil, nil, hopErr
	}
	return states, unknowns, nil
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func parseReactRouter(e blobEntry, data []byte) ([]rawState, []Unknown) {
	return parseReactRouterWith(e, data, nil)
}

// reactDynamic reports a conditional, logical, loop or `.map` construct anywhere in the file: one
// could select or compute routes, so the file's states are not read as declared.
func reactDynamic(ts []token) bool {
	for i, t := range ts {
		if next(ts, i, "?") || next(ts, i, "&") && next(ts, i+1, "&") || next(ts, i, "|") && next(ts, i+1, "|") || t.kind == tokIdent && (t.text == "if" || t.text == "switch" || t.text == "for" || t.text == "while" || t.text == "map" && i > 0 && next(ts, i-1, ".")) {
			return true
		}
	}
	return false
}

func parseReactRouterWith(e blobEntry, data []byte, imp reactImporter) ([]rawState, []Unknown) {
	p := &reactParser{e: e, data: data, anchor: wholeFile(e, data)}
	ts, ok := lexRoutes(string(data), false)
	if !ok {
		return nil, []Unknown{routerUnknown(e, 1, "unsupported-react-syntax")}
	}
	found := false
	dynamicContext := reactDynamic(ts)
	placed := map[int]bool{} // states of a statically placed JSX Routes tree (AMAP-V0-190)
	for i := 0; i < len(ts); i++ {
		if ts[i].kind == tokIdent && (ts[i].text == "createBrowserRouter" || ts[i].text == "createRoutesFromElements") && next(ts, i+1, "(") {
			found = true
			end := closeParen(ts, i+1)
			if end <= i+2 || !next(ts, end, ")") {
				p.unknown(ts[i].line, "non-literal-react-routes")
				continue
			}
			if ts[i].text == "createBrowserRouter" && end == i+3 && ts[i+2].kind == tokIdent {
				if v, ref, why := reactRoutesBinding(ts, ts[i+2].text, i+2, false); why != "" {
					p.unknown(ts[i].line, why)
				} else if ref != nil {
					p.imported(ref, ts[i].line, imp)
				} else {
					p.routes(v, "", "", 0)
				}
			} else if ts[i].text == "createBrowserRouter" {
				v, after := parseValue(ts, i+2)
				if after != end || v.kind != "array" {
					p.unknown(ts[i].line, "non-literal-react-routes")
				} else {
					p.routes(v, "", "", 0)
				}
			} else {
				v, after, valid := reactJSX(ts, i+2, 0)
				if !valid || after != end {
					p.unknown(ts[i].line, "non-literal-react-children")
				} else {
					p.routes(jsValue{kind: "array", arr: []jsValue{v}}, "", "", 0)
				}
			}
			i = end
			continue
		}
		if next(ts, i, "<") && i+1 < len(ts) && ts[i+1].text == "Routes" {
			found = true
			v, after, valid := reactJSX(ts, i, 0)
			if valid {
				first := len(p.states)
				children, _ := v.get("children")
				p.routes(children, "", "", 0)
				if gated, guarded := reactGated(ts, i, after); !gated {
					for k := first; k < len(p.states); k++ {
						placed[k] = true
						if guarded {
							// Guard returns decide whether the tree renders, never which routes it declares (AMAP-V0-193).
							p.states[k].entry = &EntryDeclaration{URLDeclared: true, Constraints: []string{"render-guarded"}}
						}
					}
				}
				i = after - 1
			} else {
				p.unknown(ts[i].line, "non-literal-react-children")
				break
			}
		}
	}
	if dynamicContext && len(placed) < len(p.states) || dynamicContext && len(p.states) == 0 {
		p.unknown(1, "dynamic-route-context")
		for i := range p.states {
			if !placed[i] {
				p.states[i].reason = "dynamic-route-context"
			}
		}
	}
	if !found {
		p.unknown(1, "react-routes-not-found")
	}
	return p.states, p.unknowns
}

func (p *reactParser) unknown(line int, reason string) {
	p.unknowns = append(p.unknowns, routerUnknown(p.e, line, reason))
}

// imported compiles the array an `import { NAME } from './module'` binding names: the one top-level
// `export const NAME = [...]` of that module, read whole at the same revision, exactly as a same-file
// array (AMAP-V0-079). Its states are anchored to the declaring module and carry the router file as
// a name_from anchor, so a change to either file leaves derived screens not FRESH. No further hop.
func (p *reactParser) imported(ref *reactImportRef, line int, imp reactImporter) {
	if imp == nil || !strings.HasPrefix(ref.spec, "./") && !strings.HasPrefix(ref.spec, "../") {
		p.unknown(line, "imported-react-routes") // a package or aliased specifier is never resolved
		return
	}
	te, td, why := imp(ref.spec)
	if why != "" || td == nil {
		if why != "" {
			p.unknown(line, why)
		}
		return
	}
	q := &reactParser{e: te, data: td, anchor: wholeFile(te, td)}
	ts, ok := lexRoutes(string(td), false)
	if !ok {
		p.unknown(line, "unsupported-react-syntax")
		return
	}
	v, _, why := reactRoutesBinding(ts, ref.name, -1, true)
	if why != "" {
		q.unknown(1, why)
		p.unknowns = append(p.unknowns, q.unknowns...)
		p.unknown(line, "imported-react-routes")
		return
	}
	q.routes(v, "", "", 0)
	dynamic := reactDynamic(ts)
	if dynamic {
		q.unknown(1, "dynamic-route-context")
	}
	for i := range q.states {
		q.states[i].nameFrom = append(q.states[i].nameFrom, p.anchor)
		if dynamic {
			q.states[i].reason = "dynamic-route-context"
		}
	}
	p.states = append(p.states, q.states...)
	p.unknowns = append(p.unknowns, q.unknowns...)
}

// reactImportRef is one named import binding: the module's exported name and its specifier.
type reactImportRef struct{ name, spec string }

// reactNamedImport reads the binding at j as `import { NAME } from 'spec'` or
// `import { ORIGINAL as NAME } from 'spec'`; default, namespace, type and mixed imports read nil.
func reactNamedImport(ts []token, j int) *reactImportRef {
	k := j - 1
	for k >= 0 && !(ts[k].kind == tokIdent && ts[k].text == "import") {
		k--
	}
	if k < 0 || !next(ts, k+1, "{") {
		return nil
	}
	cb := j + 1
	for cb < len(ts) && !next(ts, cb, "}") {
		cb++
	}
	if cb+2 >= len(ts) || ts[cb+1].kind != tokIdent || ts[cb+1].text != "from" || !literal(ts[cb+2]) {
		return nil
	}
	name := ts[j].text
	open := func(i int) bool { return next(ts, i, "{", ",") }
	switch {
	case open(j-1) && next(ts, j+1, "}", ","):
	case j >= 3 && ts[j-1].kind == tokIdent && ts[j-1].text == "as" && ts[j-2].kind == tokIdent && open(j-3):
		name = ts[j-2].text
	default:
		return nil
	}
	if name == "default" || name == "type" {
		return nil
	}
	return &reactImportRef{name: name, spec: ts[cb+2].text}
}

func (p *reactParser) routes(v jsValue, parent, parentPath string, depth int) {
	if depth >= maxRouteDepth {
		p.unknown(v.line, "router-depth-exceeded")
		return
	}
	if v.kind != "array" {
		p.unknown(v.line, "non-literal-react-children")
		return
	}
	for _, node := range v.arr {
		if node.kind == "jsx-expression" {
			p.unknown(node.line, "non-literal-react-children") // its siblings stay read (AMAP-V0-190)
			continue
		}
		if node.kind != "object" {
			p.unknown(node.line, "non-literal-react-route")
			continue
		}
		p.ordinal++
		name := fmt.Sprintf("react:%s:%d", p.e.path, p.ordinal)
		reason := ""
		routePath := ""
		index := false
		redirect := false
		children, hasChildren := node.get("children")
		for _, pair := range node.obj {
			value := pair.value
			switch pair.key {
			case "":
				reason = "non-literal-react-route"
			case "lazy":
				reason = "lazy-route"
			case "id":
				if value.kind != "string" || value.str == "" {
					reason = "non-literal-name"
				} else {
					name = value.str
				}
			case "path":
				if value.kind != "string" {
					reason = "non-literal-path"
				} else {
					routePath = value.str
				}
			case "index":
				if value.kind != "bool" {
					reason = "non-literal-index"
				} else {
					index = value.str == "true"
				}
			case "children":
				if value.kind != "array" {
					reason = "non-literal-react-children"
				}
			case "element":
				redirect = value.kind == "navigate"
			case "Component", "errorElement", "ErrorBoundary", "loader", "action", "handle", "shouldRevalidate", "hydrateFallbackElement", "HydrateFallback":
				// These do not supply route paths. Their behavior is not inferred.
			default:
				reason = "unsupported-react-option"
			}
		}
		if index && (hasChildren || routePath != "") {
			reason = "invalid-index-route"
		}
		if !literalRoutePath(routePath) || strings.Contains(routePath, "*") && !strings.HasSuffix(routePath, "/*") && routePath != "*" {
			reason = "unsupported-route-pattern"
		}
		full := routeJoin(parentPath, routePath)
		if strings.HasPrefix(routePath, "/") {
			if parentPath != "" && parentPath != "/" && routePath != parentPath && !strings.HasPrefix(routePath, strings.TrimRight(parentPath, "/")+"/") {
				reason = "invalid-absolute-child"
			}
			full = routePath
		}
		if !literalRoutePath(full) {
			reason = "unsupported-route-pattern"
		}
		if len(p.states) > maxStates {
			return
		}
		// A childless route whose element is only a <Navigate> redirects; it declares no screen (AMAP-V0-194).
		p.states = append(p.states, rawState{name: name, parent: parent, parentSet: true, url: "^" + full, urlSet: true, abstract: hasChildren || redirect, redirect: redirect, reason: reason, anchor: p.anchor})
		if hasChildren && children.kind == "array" {
			p.routes(children, name, full, depth+1)
		}
	}
}

// reactJSX reads a Route tree, skipping presentation expressions but refusing computed route
// attributes and dynamic children. It never scans through a conditional to harvest its Route.
func reactJSX(ts []token, i, depth int) (jsValue, int, bool) {
	if depth >= maxRouteDepth || !next(ts, i, "<") || i+1 >= len(ts) {
		return jsValue{}, i, false
	}
	tag := ts[i+1].text
	if tag != "Route" && tag != "Routes" {
		return jsValue{}, i, false
	}
	v := jsValue{kind: "object", line: ts[i].line}
	i += 2
	seen := map[string]bool{}
	for i < len(ts) && !next(ts, i, ">") && !(next(ts, i, "/") && next(ts, i+1, ">")) {
		key := ts[i].text
		if ts[i].kind != tokIdent {
			return jsValue{}, i, false
		}
		i++
		value := jsValue{kind: "bool", str: "true", line: v.line}
		if next(ts, i, "=") {
			i++
			if i >= len(ts) {
				return jsValue{}, i, false
			}
			if literal(ts[i]) {
				if ts[i].kind != tokString || strings.ContainsAny(ts[i].code, "&\\\n\r\t") {
					return jsValue{}, i, false
				}
				value = jsValue{kind: "string", str: ts[i].text, line: ts[i].line}
				i++
			} else if next(ts, i, "{") {
				end := closeParen(ts, i)
				if end <= i || !next(ts, end, "}") {
					return jsValue{}, i, false
				}
				inner := append(append([]token{}, ts[i+1:end]...), token{kind: tokPunct, text: ","})
				var after int
				value, after = parseValue(inner, 0)
				if after != len(inner)-1 {
					value = jsValue{kind: "other", line: v.line}
				}
				if reactNavigateElement(ts[i+1 : end]) {
					value = jsValue{kind: "navigate", line: v.line}
				}
				i = end + 1
			} else {
				return jsValue{}, i, false
			}
		}
		if seen[key] {
			v.obj = append(v.obj, jsPair{key: ""})
		}
		seen[key] = true
		v.obj = append(v.obj, jsPair{key: key, value: value})
	}
	if next(ts, i, "/") && next(ts, i+1, ">") {
		return v, i + 2, true
	}
	if !next(ts, i, ">") {
		return jsValue{}, i, false
	}
	i++
	children := jsValue{kind: "array", line: v.line}
	for i < len(ts) {
		if next(ts, i, "<") && next(ts, i+1, "/") && i+3 < len(ts) && ts[i+2].text == tag && next(ts, i+3, ">") {
			if len(children.arr) > 0 || tag == "Routes" {
				v.obj = append(v.obj, jsPair{key: "children", value: children})
			}
			return v, i + 4, true
		}
		if next(ts, i, "{") {
			// A JSX expression child may compute or gate routes: it is never harvested, but it
			// no longer discards its literal siblings (AMAP-V0-190). A comment-only `{}` is empty.
			end := closeParen(ts, i)
			if end <= i || !next(ts, end, "}") {
				return jsValue{}, i, false
			}
			if end > i+1 {
				children.arr = append(children.arr, jsValue{kind: "jsx-expression", line: ts[i].line})
			}
			i = end + 1
			continue
		}
		child, after, ok := reactJSX(ts, i, depth+1)
		if !ok {
			return jsValue{}, i, false
		}
		children.arr = append(children.arr, child)
		i = after
	}
	return jsValue{}, i, false
}

// reactNavigateElement reports whether an attribute expression is exactly one self-closing
// `<Navigate ... />` element holding no other element: a redirect, never a screen (AMAP-V0-194).
// A member-qualified tag such as `<Navigate.Panel />` is another component, not a redirect.
func reactNavigateElement(ts []token) bool {
	n := len(ts)
	if n < 4 || !next(ts, 0, "<") || ts[1].kind != tokIdent || ts[1].text != "Navigate" || next(ts, 2, ".") || !next(ts, n-2, "/") || !next(ts, n-1, ">") {
		return false
	}
	for j := 2; j < n-2; j++ {
		if next(ts, j, "<") {
			return false
		}
	}
	return true
}

// reactRoutesBinding resolves `createBrowserRouter(NAME)` to the one literal array a top-level
// `const NAME = [...]` (optionally typed `: Type` or `: Type[]`) binds in the same file. Any other
// use of NAME may compute or mutate the array and stays UNKNOWN (`computed-react-routes`); a file
// holding any template substitution or an escaped identifier is not read either, since either could
// name NAME unseen (AMAP-V0-078). A NAME bound only by one literal named import returns the import
// (AMAP-V0-079); any other import form stays `imported-react-routes`. With exported set the file is
// the imported module: use is -1, the binding must be `export const NAME = [...]`, and an import of
// NAME there is a second hop, never followed.
func reactRoutesBinding(ts []token, name string, use int, exported bool) (jsValue, *reactImportRef, string) {
	decl, depth := -1, 0
	var ref *reactImportRef
	for j, t := range ts {
		if t.subst || t.kind == tokPunct && t.text == "\\" {
			return jsValue{}, nil, "computed-react-routes" // a ${} expression or escaped identifier could touch NAME unseen
		}
		if t.kind == tokPunct {
			switch t.text {
			case "{", "(", "[":
				depth++
			case "}", ")", "]":
				depth--
			}
			continue
		}
		if j == use || t.kind != tokIdent || t.text != name || j > 0 && next(ts, j-1, ".") && !(j > 1 && next(ts, j-2, ".")) {
			continue
		}
		if reactImported(ts, j) {
			if exported || ref != nil || decl >= 0 {
				return jsValue{}, nil, "imported-react-routes"
			}
			if ref = reactNamedImport(ts, j); ref == nil {
				return jsValue{}, nil, "imported-react-routes"
			}
			continue
		}
		eq := j + 1
		if next(ts, eq, ":") && eq+1 < len(ts) && ts[eq+1].kind == tokIdent {
			eq += 2
			if next(ts, eq, "[") && next(ts, eq+1, "]") {
				eq += 2
			}
		}
		if ref != nil || decl >= 0 || depth != 0 || j == 0 || ts[j-1].kind != tokIdent || ts[j-1].text != "const" || !next(ts, eq, "=") || next(ts, eq+1, "=", ">") ||
			exported && (j < 2 || ts[j-2].kind != tokIdent || ts[j-2].text != "export") {
			return jsValue{}, nil, "computed-react-routes"
		}
		decl = eq + 1
	}
	if ref != nil {
		return jsValue{}, ref, ""
	}
	if decl < 0 {
		return jsValue{}, nil, "non-literal-react-routes"
	}
	v, after := parseValue(ts, decl)
	if v.kind != "array" || after < len(ts) && !next(ts, after, ";") && !reactStatementStart(ts, after) {
		return jsValue{}, nil, "computed-react-routes"
	}
	return v, nil, ""
}

// reactStatementStart reports whether the token after an array literal without `;` begins a new
// statement: an identifier on a later line that cannot continue the expression. Anything else
// (`.concat(...)`, `[`, `(`, an operator, `as`) may extend the array and is not read.
func reactStatementStart(ts []token, k int) bool {
	t := ts[k]
	if t.kind != tokIdent || t.line == ts[k-1].line {
		return false
	}
	switch t.text {
	case "in", "instanceof", "as", "satisfies":
		return false
	}
	return true
}

// reactImported reports whether the identifier at j sits in an `import ... from` clause.
func reactImported(ts []token, j int) bool {
	for k := j - 1; k >= 0; k-- {
		t := ts[k]
		switch {
		case t.kind == tokIdent && t.text == "import":
			return true
		case t.kind == tokIdent || next(ts, k, ",", "{", "}", "*"):
		default:
			return false
		}
	}
	return false
}

// reactGated reports whether anything other than plain JSX nesting could decide whether the
// `<Routes>` element at ts[start:end] renders (AMAP-V0-190). It walks out through each enclosing
// bracket. At every level, the statement holding the element must carry no conditional, logical
// operator or loop keyword, and every enclosing bracket must be a `return (`, `=> (`, `= (` or
// JSX-child group, or the block body of a function: a call argument, array element, attribute or
// object value, or control-flow block is treated as gated. Anything unrecognised is gated.
// guarded reports that an enclosing function body returns earlier only through guard statements
// (AMAP-V0-193).
func reactGated(ts []token, start, end int) (gated, guarded bool) {
	var stack []int
	for j := 0; j < start; j++ {
		if ts[j].kind != tokPunct {
			continue
		}
		switch ts[j].text {
		case "(", "[", "{":
			stack = append(stack, j)
		case ")", "]", "}":
			if len(stack) == 0 {
				return true, false
			}
			stack = stack[:len(stack)-1]
		}
	}
	lo, hi := start, end-1
	for level := len(stack); ; level-- {
		open, close := -1, len(ts)
		if level > 0 {
			open = stack[level-1]
			close = closeParen(ts, open)
		}
		if reactLevelGated(ts, lo, hi, open, close) {
			return true, false
		}
		if level == 0 {
			return false, guarded
		}
		if reactOpenerGated(ts, open) {
			return true, false
		}
		exits, guards := reactBodyExitsEarly(ts, open, lo)
		if exits {
			return true, false
		}
		guarded = guarded || guards
		lo, hi = open, close
	}
}

// reactLevelGated scans the statement around ts[lo..hi] between the level's brackets.
func reactLevelGated(ts []token, lo, hi, open, close int) bool {
back:
	for j := lo - 1; j > open; j-- {
		t := ts[j]
		if t.kind == tokPunct {
			switch t.text {
			case ";":
				break back
			case ")", "]", "}":
				j = reactOpenOf(ts, j)
				if j < 0 {
					return true
				}
				continue
			case "?":
				return true
			case "&", "|":
				if j > 0 && next(ts, j-1, t.text) {
					return true
				}
			}
			continue
		}
		if t.kind != tokIdent {
			continue
		}
		switch t.text {
		case "if", "else", "switch", "case", "for", "while", "do":
			return true
		case "return":
			if j > 0 && (next(ts, j-1, ")") || ts[j-1].kind == tokIdent && ts[j-1].text == "else") {
				return true
			}
			break back
		case "function", "const", "let", "var", "export", "class", "import":
			break back
		}
	}
	for j := hi + 1; j < close; j++ {
		t := ts[j]
		if t.kind == tokPunct {
			switch t.text {
			case ";":
				return false
			case "(", "[", "{":
				j = closeParen(ts, j)
				continue
			case "?":
				return true
			case "&", "|":
				if next(ts, j+1, t.text) {
					return true
				}
			}
			continue
		}
		if t.kind == tokIdent {
			switch t.text {
			case "function", "const", "let", "var", "export", "class", "import", "return", "if", "for", "while", "switch", "type", "interface":
				return false
			}
		}
	}
	return false
}

// reactBodyExitsEarly reports whether the block opened at ts[open] could leave before reaching
// ts[lo] (AMAP-V0-190): a `throw`, a `return` inside a nested control-flow block, more than one
// `return` at the block's own level, or a `return` whose statement has already ended. Nested
// function bodies are skipped, because their exits do not leave this block. A JSX expression
// child brace holds no statements and is left to reactLevelGated. A guard statement at the
// block's own level (reactGuard) is not such an exit; guards reports that one was skipped
// (AMAP-V0-193).
func reactBodyExitsEarly(ts []token, open, lo int) (exits, guards bool) {
	if ts[open].text != "{" || next(ts, open-1, ">") && !(open >= 2 && next(ts, open-2, "=")) {
		return false, false
	}
	returns, depth, ended := 0, 0, false
	for j := open + 1; j < lo; j++ {
		t := ts[j]
		if depth == 0 && returns == 0 && t.kind == tokIdent && t.text == "if" {
			if end, guard := reactGuard(ts, j, lo); end > j {
				guards = guards || guard
				j = end
				continue
			}
		}
		if t.kind == tokPunct {
			switch t.text {
			case "{":
				if reactFunctionBody(ts, j) {
					j = closeParen(ts, j)
					continue
				}
				depth++
			case "(", "[":
				depth++
			case ")", "]", "}":
				depth--
			case ";":
				if depth == 0 && returns > 0 {
					ended = true
				}
			}
			continue
		}
		if t.kind != tokIdent {
			continue
		}
		switch t.text {
		case "throw":
			return true, false
		case "return":
			if depth != 0 || returns > 0 {
				return true, false
			}
			returns++
		}
	}
	return ended, guards
}

// reactGuard reads the `if` statement at ts[j], before ts[lo], as a guard: `if (COND) STATEMENT;`
// or `if (COND) { ... }` with no `else`, whose body holds a `return` or `throw` and no `<Routes>`
// or `<Route>` element, so it can only leave the block rendering something other than a route
// tree. An `if` whose body cannot leave is skipped as no exit. end is the statement's last token
// index, or j when the statement is not recognised, an `else` follows it, or its body holds a
// route element; the caller then applies the AMAP-V0-190 exit rules to it.
func reactGuard(ts []token, j, lo int) (end int, guard bool) {
	if !next(ts, j+1, "(") {
		return j, false
	}
	c := closeParen(ts, j+1)
	if c <= j+1 || c+1 >= lo || !next(ts, c, ")") {
		return j, false
	}
	end = -1
	if next(ts, c+1, "{") {
		end = closeParen(ts, c+1)
		if end <= c+1 || end >= lo || !next(ts, end, "}") {
			return j, false
		}
	} else {
		for k := c + 1; k < lo; k++ {
			if next(ts, k, "(", "[", "{") {
				k = closeParen(ts, k)
				continue
			}
			if next(ts, k, ";") {
				end = k
				break
			}
		}
		if end < 0 {
			return j, false // without `;` the statement's end is not read
		}
	}
	if end+1 < len(ts) && ts[end+1].kind == tokIdent && ts[end+1].text == "else" {
		return j, false
	}
	for k := c + 1; k < end; k++ {
		switch t := ts[k]; {
		case next(ts, k, "<") && k+1 < end && (ts[k+1].text == "Routes" || ts[k+1].text == "Route"):
			return j, false
		case t.kind == tokIdent && (t.text == "return" || t.text == "throw"):
			guard = true
		}
	}
	return end, guard
}

// reactFunctionBody reports whether the brace at ts[open] opens an arrow, function or method
// body. A brace after a control-flow head, or anything unrecognised, is not one, so its exits count.
func reactFunctionBody(ts []token, open int) bool {
	if open >= 2 && next(ts, open-1, ">") && next(ts, open-2, "=") {
		return true
	}
	if open < 1 || !next(ts, open-1, ")") {
		return false
	}
	o := reactOpenOf(ts, open-1)
	if o <= 0 || ts[o-1].kind != tokIdent {
		return false
	}
	switch ts[o-1].text {
	case "if", "for", "while", "switch", "catch", "with", "return":
		return false
	}
	return true
}

// reactOpenerGated reports whether the bracket at ts[open] could decide its content renders.
func reactOpenerGated(ts []token, open int) bool {
	if open == 0 || ts[open].text == "[" {
		return true
	}
	prev := ts[open-1]
	if ts[open].text == "(" {
		if prev.kind == tokIdent {
			return prev.text != "return"
		}
		if next(ts, open-1, ">") {
			return open < 2 || !next(ts, open-2, "=") // `=>` is lexed as `=` `>`: an arrow body
		}
		return !next(ts, open-1, "=", ",", "{", "(")
	}
	// A brace: a JSX expression child (after a tag's `>`), an arrow body, or a function body.
	if next(ts, open-1, ">") {
		return false
	}
	j := open - 1
	for j > 1 && ts[j].kind == tokIdent && next(ts, j-1, ".") {
		j -= 2 // a qualified return type annotation
	}
	if j > 0 && ts[j].kind == tokIdent && next(ts, j-1, ":") {
		j -= 2
	}
	if j < 0 || !next(ts, j, ")") {
		return true
	}
	o := reactOpenOf(ts, j)
	if o <= 0 {
		return true
	}
	switch p := ts[o-1]; {
	case p.kind == tokIdent && (p.text == "if" || p.text == "for" || p.text == "while" || p.text == "switch" || p.text == "catch" || p.text == "with"):
		return true
	case p.kind == tokIdent, next(ts, o-1, "=", "(", ",", ">"):
		return false
	}
	return true
}

// reactOpenOf returns the index of the bracket that ts[c] closes, or -1.
func reactOpenOf(ts []token, c int) int {
	depth := 0
	for j := c; j >= 0; j-- {
		if ts[j].kind != tokPunct {
			continue
		}
		switch ts[j].text {
		case ")", "]", "}":
			depth++
		case "(", "[", "{":
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return -1
}
