package appmap

import (
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// mountResolver resolves an import specifier written in importer to a tracked path; the state is
// contextindex.WebImportRepository when it does.
type mountResolver func(importer, specifier string) (string, contextindex.WebImportState)

// inferringMounts reports whether a build may infer a React mount (AMAP-V0-979): at least two
// declared react-router/0 routers carry no mount, so one can render the other.
func inferringMounts(routers []RouterFile) bool {
	n := 0
	for _, r := range routers {
		if r.Dialect == DialectReact && r.Mount == nil {
			n++
		}
	}
	return n >= 2
}

// mountBinding is one top-level name of a router file bound to a module specifier.
type mountBinding struct {
	spec string
	line int
}

// mountCandidate is one host splat route whose element renders an unmounted router.
type mountCandidate struct {
	host int    // router index of the host
	path string // the host route's path as its router declares it
}

// inferMounts returns routers with a mount inferred for each unmounted React router that exactly
// one non-redirect terminal splat route of another declared React router renders (AMAP-V0-979):
// the route's element names a component, the host file binds that name once at top level by a
// static import or by a `const` whose initializer holds exactly one dynamic `import("spec")`
// (React.lazy and its wrappers), and spec resolves from the host file to the unmounted router's
// path. The host path must be declared once in its router, so mountRouters places the routes as
// for a declared mount. A router two hosts render, or whose inferred mount would close a cycle,
// stays unmounted and reads ambiguous-react-mount; a host route whose element names a binding
// whose specifier does not resolve, and that renders no router, reads unresolved-react-mount-import.
// The declared routers are not modified.
func inferMounts(routers []RouterFile, raws []rawState, spans [][2]int, sources [][]byte, resolve mountResolver) ([]RouterFile, []Unknown) {
	if resolve == nil || !inferringMounts(routers) {
		return routers, nil
	}
	target := map[string]int{}
	for i, r := range routers {
		if r.Dialect == DialectReact && r.Mount == nil {
			target[r.Path] = i
		}
	}
	var unknowns []Unknown
	found := map[int][]mountCandidate{}
	for a, host := range routers {
		if host.Dialect != DialectReact {
			continue
		}
		var bindings map[string][]mountBinding
		declared := map[string]int{}
		for k := spans[a][0]; k < spans[a][1]; k++ {
			if !raws[k].redirect {
				declared[raws[k].url]++
			}
		}
		for k := spans[a][0]; k < spans[a][1]; k++ {
			s := raws[k]
			p := strings.TrimPrefix(s.url, "^")
			if s.redirect || s.element == "" || declared[s.url] != 1 || !strings.HasPrefix(p, "/") || !strings.HasSuffix(p, "/*") || !literalRoutePath(p) {
				continue
			}
			if bindings == nil {
				bindings = topLevelModuleBindings(sources[a])
			}
			rendered := map[int]bool{}
			var unresolved *mountBinding
			for _, name := range elementTags(s.element) {
				bs := bindings[name]
				if len(bs) != 1 {
					continue
				}
				switch path, state := resolve(host.Path, bs[0].spec); {
				case state == contextindex.WebImportRepository:
					if t, ok := target[path]; ok && t != a {
						rendered[t] = true
					}
				case state == contextindex.WebImportUnresolved && unresolved == nil:
					unresolved = &bs[0]
				}
			}
			switch {
			case len(rendered) == 1:
				for t := range rendered {
					found[t] = append(found[t], mountCandidate{host: a, path: p})
				}
			case len(rendered) > 1:
				unknowns = append(unknowns, Unknown{Kind: "router", Ref: host.Path, Path: host.Path, Reason: "ambiguous-react-mount", Detail: "route " + p + " renders more than one declared router"})
			case unresolved != nil:
				u := routerUnknown(blobEntry{path: host.Path}, unresolved.line, "unresolved-react-mount-import")
				u.Detail = "route " + p + ": specifier " + unresolved.spec
				unknowns = append(unknowns, u)
			}
		}
	}
	out := append([]RouterFile(nil), routers...)
	parent := map[string]string{}
	for _, r := range out {
		if r.Mount != nil {
			parent[r.Path] = r.Mount.Router
		}
	}
	for t := range out {
		cands := found[t]
		if len(cands) == 0 {
			continue
		}
		hostPath := routers[cands[0].host].Path
		if len(cands) > 1 || closesCycle(parent, out[t].Path, hostPath) {
			unknowns = append(unknowns, Unknown{Kind: "router", Ref: out[t].Path, Path: out[t].Path, Reason: "ambiguous-react-mount", Detail: "rendered by more than one splat route, or by a router it renders"})
			continue
		}
		parent[out[t].Path] = hostPath
		out[t].Mount = &RouterMount{Router: hostPath, Path: cands[0].path}
	}
	return out, unknowns
}

// closesCycle reports whether mounting child under host would make a router its own ancestor.
func closesCycle(parent map[string]string, child, host string) bool {
	seen := map[string]bool{}
	for cur := host; cur != ""; cur = parent[cur] {
		if cur == child || seen[cur] {
			return true
		}
		seen[cur] = true
	}
	return false
}

// elementTags lists the component names an element's canonical token text opens as JSX tags:
// an identifier right after `<` that no `.` qualifies.
func elementTags(element string) []string {
	ts, ok := lexRoutes(element, false)
	if !ok {
		return nil
	}
	var names []string
	for j := 0; j+1 < len(ts); j++ {
		if next(ts, j, "<") && ts[j+1].kind == tokIdent && !ts[j+1].escaped && !next(ts, j+2, ".") {
			names = append(names, ts[j+1].text)
		}
	}
	return names
}

// topLevelModuleBindings reads the names a file binds at top level to a module specifier: each
// default or named binding of a static `import ... from "spec"` (type-only bindings excluded), and
// each `const NAME = ...` whose initializer holds exactly one dynamic `import("spec")`. A name bound
// more than once, or also by another top-level declaration, maps to every binding, so callers
// requiring exactly one ignore it.
func topLevelModuleBindings(data []byte) map[string][]mountBinding {
	ts, ok := lexRoutes(string(data), false)
	out := map[string][]mountBinding{}
	if !ok {
		return out
	}
	depth := 0
	for i := 0; i < len(ts); i++ {
		t := ts[i]
		switch {
		case next(ts, i, "{", "(", "["):
			depth++
			continue
		case next(ts, i, "}", ")", "]"):
			depth--
			continue
		}
		if depth != 0 || t.kind != tokIdent || next(ts, i-1, ".") {
			continue
		}
		switch t.text {
		case "import":
			if next(ts, i+1, "(") {
				continue
			}
			end, names, spec := staticImportBindings(ts, i)
			for _, n := range names {
				out[n] = append(out[n], mountBinding{spec: spec, line: t.line})
			}
			if end > i {
				i = end
			}
		case "const", "let", "var", "function", "class":
			if i+1 >= len(ts) || ts[i+1].kind != tokIdent {
				continue
			}
			name := ts[i+1].text
			if t.text != "const" || !next(ts, i+2, "=") {
				out[name] = append(out[name], mountBinding{line: t.line}) // a declaration that binds no module
				continue
			}
			end, specs := initializerImports(ts, i+3)
			if len(specs) == 1 {
				out[name] = append(out[name], mountBinding{spec: specs[0], line: t.line})
			} else {
				out[name] = append(out[name], mountBinding{line: t.line})
			}
			i = end - 1
		}
	}
	for name, bs := range out {
		for _, b := range bs {
			if b.spec == "" {
				out[name] = append(bs[:0:0], b, b) // never exactly one usable binding
				break
			}
		}
	}
	return out
}

// staticImportBindings reads `import CLAUSE from "spec"` starting at the `import` token i and
// returns the index of the specifier, the names CLAUSE binds as values and the specifier; a
// side-effect import, a type-only import or a clause it cannot read binds none.
func staticImportBindings(ts []token, i int) (int, []string, string) {
	from := -1
	for k := i + 1; k < len(ts) && k < i+512; k++ {
		if ts[k].kind == tokIdent && ts[k].text == "from" && k+1 < len(ts) && literal(ts[k+1]) {
			from = k
			break
		}
		if next(ts, k, ";") || ts[k].kind == tokString {
			return k, nil, ""
		}
	}
	if from < 0 {
		return i, nil, ""
	}
	clause := ts[i+1 : from]
	spec := ts[from+1].text
	if len(clause) > 0 && clause[0].kind == tokIdent && clause[0].text == "type" && (len(clause) == 1 || clause[1].kind != tokPunct || clause[1].text == "{") {
		return from + 1, nil, ""
	}
	var names []string
	inBraces := false
	for k := 0; k < len(clause); k++ {
		c := clause[k]
		switch {
		case next(clause, k, "{"):
			inBraces = true
		case next(clause, k, "}"):
			inBraces = false
		case next(clause, k, "*"):
			k += 2 // `* as NS`: a namespace, never a component tag
		case c.kind != tokIdent || c.escaped:
		case inBraces && c.text == "type" && k+1 < len(clause) && clause[k+1].kind == tokIdent:
			k++ // `type X` and `type X as Y`: type-only
			if k+2 < len(clause) && clause[k+1].text == "as" {
				k += 2
			}
		case inBraces && k+2 < len(clause) && clause[k+1].kind == tokIdent && clause[k+1].text == "as":
			names = append(names, clause[k+2].text)
			k += 2
		case !inBraces && c.text == "as":
		default:
			names = append(names, c.text)
		}
	}
	return from + 1, names, spec
}

// initializerImports reads a `const` initializer from token i to the `;` or new statement that
// ends it at bracket depth 0, returning the index after it and every dynamic `import("spec")`
// specifier it holds; a dynamic import of anything but a literal adds an empty specifier.
func initializerImports(ts []token, i int) (int, []string) {
	var specs []string
	depth := 0
	k := i
	for ; k < len(ts); k++ {
		switch {
		case next(ts, k, "{", "(", "["):
			depth++
		case next(ts, k, "}", ")", "]"):
			depth--
			if depth < 0 {
				return k, specs
			}
		case depth == 0 && next(ts, k, ";", ","):
			return k + 1, specs
		case depth == 0 && k > i && reactStatementStart(ts, k) && !next(ts, k-1, "=>", ".", "=", "(", ",", "?", ":", "&&", "||"):
			return k, specs
		}
		if ts[k].kind == tokIdent && ts[k].text == "import" && next(ts, k+1, "(") {
			if k+3 < len(ts) && literal(ts[k+2]) && next(ts, k+3, ")", ",") {
				specs = append(specs, ts[k+2].text)
			} else {
				specs = append(specs, "")
			}
		}
	}
	return k, specs
}
