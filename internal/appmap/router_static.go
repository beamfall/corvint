package appmap

import "strings"

// staticMutators are the array methods that change an array in place; a use of the export through
// one of them makes the list computed (AMAP-V0-980).
var staticMutators = map[string]bool{"push": true, "pop": true, "shift": true, "unshift": true, "splice": true,
	"sort": true, "reverse": true, "fill": true, "copyWithin": true, "length": true}

// parseStaticRoutes reads a static-routes/0 file (AMAP-V0-980): the one top-level
// `export const NAME = [...]` the manifest's export names, optionally typed and followed by
// `as const` or `satisfies T`. Each string element that is an absolute literal path with no
// parameter or splat declares one resolved, top-level screen named `static:PATH`, anchored at the
// declaration; a repeated path declares it once. Any other element reads non-literal-static-route
// or unsupported-static-route. A file with no such declaration, more than one, a template
// substitution or escaped identifier anywhere, or a use of NAME that may change it (an assignment
// or a mutating array method) declares nothing and reads computed-static-routes or
// missing-static-routes.
func parseStaticRoutes(e blobEntry, data []byte, export string) ([]rawState, []Unknown) {
	ts, ok := lexRoutes(string(data), false)
	if !ok {
		return nil, []Unknown{routerUnknown(e, 1, "unsupported-static-routes-syntax")}
	}
	decl, eq, depth := -1, -1, 0
	for j, t := range ts {
		if t.subst || t.escaped || t.kind == tokPunct && t.text == "\\" {
			return nil, []Unknown{routerUnknown(e, t.line, "computed-static-routes")}
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
		if t.kind != tokIdent || t.text != export || next(ts, j-1, ".") {
			continue
		}
		if j >= 2 && depth == 0 && word(ts, j-1, "const") && word(ts, j-2, "export") {
			if decl >= 0 {
				return nil, []Unknown{routerUnknown(e, t.line, "computed-static-routes")}
			}
			decl, eq = j, staticInitializer(ts, j+1)
			continue
		}
		// Any other use may only read the list.
		if next(ts, j+1, "=") && !next(ts, j+2, "=") || next(ts, j+1, ".") && j+2 < len(ts) && staticMutators[ts[j+2].text] {
			return nil, []Unknown{routerUnknown(e, t.line, "computed-static-routes")}
		}
	}
	if decl < 0 {
		return nil, []Unknown{routerUnknown(e, 1, "missing-static-routes")}
	}
	if eq < 0 || !next(ts, eq+1, "[") {
		return nil, []Unknown{routerUnknown(e, ts[decl].line, "computed-static-routes")}
	}
	v, after := parseValue(ts, eq+1)
	if v.kind != "array" {
		return nil, []Unknown{routerUnknown(e, ts[decl].line, "computed-static-routes")}
	}
	end := after
	if word(ts, end, "as") && word(ts, end+1, "const") {
		end += 2
	} else if a := assertedEnd(ts, end); a > end {
		end = a
	}
	if end < len(ts) && !next(ts, end, ";") && !reactStatementStart(ts, end) {
		return nil, []Unknown{routerUnknown(e, ts[decl].line, "computed-static-routes")}
	}
	start := ts[decl].line
	last := ts[after-1].line
	anchor := spanOf(e, data, start, last)
	var states []rawState
	var unknowns []Unknown
	seen := map[string]bool{}
	for _, el := range v.arr {
		switch {
		case el.kind != "string" || el.subst:
			unknowns = append(unknowns, routerUnknown(e, staticLine(el, start), "non-literal-static-route"))
		case !strings.HasPrefix(el.str, "/") || strings.ContainsAny(el.str, ":*") || !literalRoutePath(el.str):
			unknowns = append(unknowns, routerUnknown(e, staticLine(el, start), "unsupported-static-route"))
		case !seen[el.str]:
			seen[el.str] = true
			states = append(states, rawState{name: "static:" + el.str, parentSet: true, url: "^" + el.str, urlSet: true, anchor: anchor})
		}
		if len(states) > maxStates {
			break
		}
	}
	return states, unknowns
}

// staticInitializer returns the index of the `=` that starts the initializer of the declaration
// whose name is at ts[i-1], stepping over a `: Type` annotation at bracket depth 0; -1 when none.
func staticInitializer(ts []token, i int) int {
	if next(ts, i, "=") {
		return i
	}
	if !next(ts, i, ":") {
		return -1
	}
	depth := 0
	for k := i + 1; k < len(ts); k++ {
		switch {
		case next(ts, k, "{", "(", "[", "<"):
			depth++
		case next(ts, k, "}", ")", "]", ">"):
			depth--
		case depth == 0 && next(ts, k, "=") && !next(ts, k+1, ">"):
			return k
		case depth == 0 && next(ts, k, ";"):
			return -1
		}
	}
	return -1
}

func staticLine(v jsValue, fallback int) int {
	if v.line > 0 {
		return v.line
	}
	return fallback
}

// dropShadowedStatic removes each static-routes/0 state whose path a router of another dialect
// also declares as a resolved, non-abstract state, so a prerendered route the application router
// already declares stays one screen (AMAP-V0-980). It runs after mounts are placed.
func dropShadowedStatic(routers []RouterFile, raws []rawState, spans [][2]int) ([]rawState, [][2]int) {
	routed := map[string]bool{}
	static := false
	for i, span := range spans {
		if routers[i].Dialect == DialectStatic {
			static = true
			continue
		}
		for _, s := range raws[span[0]:span[1]] {
			if !s.abstract && !s.redirect && s.reason == "" {
				routed[s.url] = true
			}
		}
	}
	if !static || len(routed) == 0 {
		return raws, spans
	}
	out := make([]rawState, 0, len(raws))
	kept := make([][2]int, len(spans))
	for i, span := range spans {
		begin := len(out)
		for _, s := range raws[span[0]:span[1]] {
			if routers[i].Dialect == DialectStatic && routed[s.url] {
				continue
			}
			out = append(out, s)
		}
		kept[i] = [2]int{begin, len(out)}
	}
	return out, kept
}
