package appmap

import (
	"fmt"
	"strings"
)

type reactParser struct {
	e        blobEntry
	data     []byte
	states   []rawState
	unknowns []Unknown
	ordinal  int
	anchor   Anchor
}

func parseReactRouter(e blobEntry, data []byte) ([]rawState, []Unknown) {
	p := &reactParser{e: e, data: data, anchor: wholeFile(e, data)}
	ts, ok := lexRoutes(string(data), false)
	if !ok {
		return nil, []Unknown{routerUnknown(e, 1, "unsupported-react-syntax")}
	}
	found := false
	dynamicContext := false
	for i := 0; i < len(ts); i++ {
		t := ts[i]
		if next(ts, i, "?") || next(ts, i, "&") && next(ts, i+1, "&") || next(ts, i, "|") && next(ts, i+1, "|") || t.kind == tokIdent && (t.text == "if" || t.text == "switch" || t.text == "for" || t.text == "while" || t.text == "map" && i > 0 && next(ts, i-1, ".")) {
			dynamicContext = true
		}

		if ts[i].kind == tokIdent && (ts[i].text == "createBrowserRouter" || ts[i].text == "createRoutesFromElements") && next(ts, i+1, "(") {
			found = true
			end := closeParen(ts, i+1)
			if end <= i+2 || !next(ts, end, ")") {
				p.unknown(ts[i].line, "non-literal-react-routes")
				continue
			}
			if ts[i].text == "createBrowserRouter" && end == i+3 && ts[i+2].kind == tokIdent {
				if v, why := reactRoutesBinding(ts, i+2); why != "" {
					p.unknown(ts[i].line, why)
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
				children, _ := v.get("children")
				p.routes(children, "", "", 0)
				i = after - 1
			} else {
				p.unknown(ts[i].line, "non-literal-react-children")
				break
			}
		}
	}
	if dynamicContext {
		p.unknown(1, "dynamic-route-context")
		for i := range p.states {
			p.states[i].reason = "dynamic-route-context"
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
		if node.kind != "object" {
			p.unknown(node.line, "non-literal-react-route")
			continue
		}
		p.ordinal++
		name := fmt.Sprintf("react:%s:%d", p.e.path, p.ordinal)
		reason := ""
		routePath := ""
		index := false
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
			case "element", "Component", "errorElement", "ErrorBoundary", "loader", "action", "handle", "shouldRevalidate", "hydrateFallbackElement", "HydrateFallback":
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
		p.states = append(p.states, rawState{name: name, parent: parent, parentSet: true, url: "^" + full, urlSet: true, abstract: hasChildren, reason: reason, anchor: p.anchor})
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
		child, after, ok := reactJSX(ts, i, depth+1)
		if !ok {
			return jsValue{}, i, false
		}
		children.arr = append(children.arr, child)
		i = after
	}
	return jsValue{}, i, false
}

// reactRoutesBinding resolves `createBrowserRouter(NAME)` to the one literal array a top-level
// `const NAME = [...]` (optionally typed `: Type` or `: Type[]`) binds in the same file. Any other
// use of NAME may compute or mutate the array and stays UNKNOWN (`computed-react-routes`); a NAME
// bound by an import is not read across files (`imported-react-routes`); a file holding any template
// substitution or an escaped identifier is not read either, since either could name NAME unseen
// (AMAP-V0-078).
func reactRoutesBinding(ts []token, use int) (jsValue, string) {
	name, decl, depth := ts[use].text, -1, 0
	for j, t := range ts {
		if t.subst || t.kind == tokPunct && t.text == "\\" {
			return jsValue{}, "computed-react-routes" // a ${} expression or escaped identifier could touch NAME unseen
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
		if j == use || t.kind != tokIdent || t.text != name || j > 0 && next(ts, j-1, ".") {
			continue
		}
		if reactImported(ts, j) {
			return jsValue{}, "imported-react-routes"
		}
		eq := j + 1
		if next(ts, eq, ":") && eq+1 < len(ts) && ts[eq+1].kind == tokIdent {
			eq += 2
			if next(ts, eq, "[") && next(ts, eq+1, "]") {
				eq += 2
			}
		}
		if decl >= 0 || depth != 0 || j == 0 || ts[j-1].kind != tokIdent || ts[j-1].text != "const" || !next(ts, eq, "=") || next(ts, eq+1, "=", ">") {
			return jsValue{}, "computed-react-routes"
		}
		decl = eq + 1
	}
	if decl < 0 {
		return jsValue{}, "non-literal-react-routes"
	}
	v, after := parseValue(ts, decl)
	if v.kind != "array" || after < len(ts) && !next(ts, after, ";") && !reactStatementStart(ts, after) {
		return jsValue{}, "computed-react-routes"
	}
	return v, ""
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
