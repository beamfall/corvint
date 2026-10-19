package testsource

// Stand-in route catalogs by language (docs/specs/test-source-analysis-v0.md, TSA-V0-213,
// TSA-V0-355..360): JS/TS, Go, Flask, Sinatra/Rails and WireMock mappings. A route's payload and
// query fields come from its own handler only: the registration call's arguments and, for a named
// handler, that function's body; never the code after the registration.

import (
	"encoding/json"
	"path"
	"regexp"
	"sort"
	"strings"
)

var (
	jsNamedHandler   = regexp.MustCompile(`,\s*([A-Za-z_$][A-Za-z0-9_$]*)\s*\)$`)
	goHandleFunc     = regexp.MustCompile(`\.(?:HandleFunc|Handle)\(\s*"(?:(GET|POST|PUT|PATCH|DELETE)\s+)?(/[^"]*)"`)
	goMethodRoute    = regexp.MustCompile(`\b[A-Za-z_][A-Za-z0-9_]*\.(GET|POST|PUT|PATCH|DELETE|Get|Post|Put|Patch|Delete)\(\s*"(/[^"]*)"`)
	goMethods        = regexp.MustCompile(`^\s*\.Methods\(\s*"([A-Z]+)"`)
	goNamedHandler   = regexp.MustCompile(`,\s*(?:[A-Za-z_][A-Za-z0-9_]*\.)?([A-Za-z_][A-Za-z0-9_]*)\s*\)$`)
	goPayloadField   = regexp.MustCompile("json:\"([A-Za-z_][A-Za-z0-9_]*)|\\b(?:PostFormValue|FormValue|PostForm)\\(\\s*\"([^\"]+)\"")
	goQueryField     = regexp.MustCompile(`\.Query\(\)\.Get\(\s*"([^"]+)"|\.Query\(\)\[\s*"([^"]+)"|\b(?:Query|QueryParam|DefaultQuery)\(\s*"([^"]+)"`)
	pyRoute          = regexp.MustCompile(`(?m)^([ \t]*)@[A-Za-z_][A-Za-z0-9_]*\.(route|get|post|put|patch|delete)\(\s*['"]([^'"]+)['"]([^\n]*)$`)
	pyMethodsList    = regexp.MustCompile(`methods\s*=\s*[\[(]([^\])]*)[\])]`)
	pyDef            = regexp.MustCompile(`(?m)^([ \t]*)(?:async\s+)?def\s`)
	pyAlias          = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\s*=\s*request\.(?:get_json\([^)]*\)|json)`)
	pyPayloadField   = regexp.MustCompile(`\brequest\.(?:json|form|get_json\([^)]*\))\s*(?:\[\s*['"]([^'"]+)['"]|\.get\(\s*['"]([^'"]+)['"])`)
	pyQueryField     = regexp.MustCompile(`\brequest\.args\s*(?:\[\s*['"]([^'"]+)['"]|\.get\(\s*['"]([^'"]+)['"])`)
	rbRoute          = regexp.MustCompile(`(?m)^([ \t]*)(get|post|put|patch|delete)\s*\(?\s*['"]([^'"]+)['"]([^\n]*)$`)
	rbController     = regexp.MustCompile(`(?:to:\s*|=>\s*)['"]([a-z_/]+)#([a-z_]+)['"]`)
	rbParamField     = regexp.MustCompile(`\bparams\s*(?:\[\s*:([A-Za-z_][A-Za-z0-9_]*)\s*\]|\[\s*['"]([^'"]+)['"]\s*\]|\.fetch\(\s*:([A-Za-z_][A-Za-z0-9_]*))`)
	rbPermitFields   = regexp.MustCompile(`\.permit\(([^)]*)\)`)
	rbSymbol         = regexp.MustCompile(`:([A-Za-z_][A-Za-z0-9_]*)`)
	wiremockJSONPath = regexp.MustCompile(`^\$\.([A-Za-z_][A-Za-z0-9_]*)`)
	bracePathParam   = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)(?::[^}]*)?\}|<(?:[a-z]+:)?([A-Za-z_][A-Za-z0-9_]*)>`)
)

// standInOtherLanguage are source languages a stand-in may be written in whose routes are not read;
// each such file is a named abstention (TSA-V0-359).
var standInOtherLanguage = map[string]bool{".java": true, ".kt": true, ".scala": true, ".groovy": true, ".cs": true,
	".php": true, ".rs": true, ".ex": true, ".exs": true, ".clj": true, ".swift": true}

// standInRouteFile is a stand-in file whose routes are read: JS/TS, Go, Python and Ruby sources
// other than tests, and WireMock mapping JSON under a `mappings` directory (TSA-V0-355..358).
func standInRouteFile(name string) bool {
	base, ext := path.Base(name), path.Ext(name)
	switch ext {
	case ".go":
		return !strings.HasSuffix(base, "_test.go")
	case ".py":
		return !strings.HasPrefix(base, "test_") && !strings.HasSuffix(base, "_test.py")
	case ".rb":
		return !strings.HasSuffix(base, "_spec.rb") && !strings.HasSuffix(base, "_test.rb")
	case ".json":
		return strings.Contains("/"+path.Dir(name)+"/", "/mappings/")
	}
	return webExtensions[ext] && !IsTestFile(name)
}

// routes reads one stand-in file's routes by its language.
func routes(name, text string, texts map[string]string) []StandInRoute {
	switch path.Ext(name) {
	case ".go":
		return goRoutes(name, text)
	case ".py":
		return pythonRoutes(name, text)
	case ".rb":
		return rubyRoutes(name, text, texts)
	case ".json":
		return wiremockRoutes(name, text)
	}
	return jsRoutes(name, text)
}

// newRoute classifies a route and its scope (TSA-V0-213).
func newRoute(method, route, name string, line int, payload, query []string) StandInRoute {
	r := StandInRoute{Method: strings.ToUpper(method), Path: route, Source: name, Line: line, Payload: payload, Query: query, Teardown: []string{}}
	r.Kind = routeKind(r.Path)
	if r.Kind != RouteService {
		r.Scope = ScopeServiceWide
		for _, key := range append(pathParams(r.Path), r.Payload...) {
			if scopeKey.MatchString(key) {
				r.Scope = scopeKeyedPrefix + key
				break
			}
		}
	}
	return r
}

func lineAt(text string, offset int) int { return 1 + strings.Count(text[:offset], "\n") }

// jsRoutes reads Express-style registrations. A route's fields come from the registration call's
// balanced arguments and, when the last argument names a handler, that function's body; an
// unbalanced call reads its own line only (TSA-V0-360).
func jsRoutes(name, text string) []StandInRoute {
	out := []StandInRoute{}
	for _, m := range routeRegistration.FindAllStringSubmatchIndex(text, -1) {
		body := callBody(text, m[0])
		if h := jsNamedHandler.FindStringSubmatch(body); h != nil {
			body += handlerBody(text, regexp.MustCompile(`\bfunction\s+`+regexp.QuoteMeta(h[1])+`\s*\(|\b(?:const|let|var)\s+`+regexp.QuoteMeta(h[1])+`\s*=`))
		}
		out = append(out, newRoute(text[m[2]:m[3]], text[m[4]:m[5]], name, lineAt(text, m[0]), fieldNames(body, true), fieldNames(body, false)))
	}
	return out
}

// callBody is the registration call from start through its balanced closing parenthesis; the
// first `(` after start opens it. An unbalanced call yields the registration's line only.
func callBody(text string, start int) string {
	open := strings.IndexByte(text[start:], '(')
	if open >= 0 {
		if end := balancedEnd(text, start+open); end > 0 {
			return text[start:end]
		}
	}
	end := strings.IndexByte(text[start:], '\n')
	if end < 0 {
		return text[start:]
	}
	return text[start : start+end]
}

// handlerBody is the balanced `{...}` body of the first definition the pattern finds, or "".
// Only a parameter list and `=>` may sit between the definition and its `{`, so an
// expression-bodied arrow never borrows the body of later, unrelated code (TSA-V0-360).
func handlerBody(text string, definition *regexp.Regexp) string {
	loc := definition.FindStringIndex(text)
	if loc == nil {
		return ""
	}
	open := strings.IndexByte(text[loc[1]:], '{')
	if open < 0 {
		return ""
	}
	head := handlerHeadExpr
	if strings.HasSuffix(text[loc[0]:loc[1]], "(") {
		head = handlerParamsExpr
	}
	if !head.MatchString(text[loc[1] : loc[1]+open]) {
		return ""
	}
	if end := balancedEnd(text, loc[1]+open); end > 0 {
		return text[loc[1]+open : end]
	}
	return ""
}

var (
	// handlerParamsExpr is the rest of a `function name(` declaration up to its body.
	handlerParamsExpr = regexp.MustCompile(`^[^(){};=]*\)\s*$`)
	// handlerHeadExpr is a `const name =` value up to its body: a function expression or an arrow.
	handlerHeadExpr = regexp.MustCompile(`^\s*(?:async\s+)?(?:function\s*[\w$]*\s*\([^(){};=]*\)\s*|\([^(){};=]*\)\s*=>\s*|[A-Za-z_$][\w$]*\s*=>\s*)$`)
)

// balancedEnd is the offset after the bracket that closes the one at open, skipping quoted
// strings and `//` and `/* */` comments, or -1 when it does not close.
func balancedEnd(text string, open int) int {
	closer := map[byte]byte{'(': ')', '{': '}', '[': ']'}[text[open]]
	depth := 0
	for i := open; i < len(text); i++ {
		c := text[i]
		switch {
		case c == '"' || c == '\'' || c == '`':
			j := i + 1
			for j < len(text) && text[j] != c {
				if text[j] == '\\' && c != '`' {
					j++
				}
				j++
			}
			if j >= len(text) {
				return -1
			}
			i = j
		case c == '/' && i+1 < len(text) && text[i+1] == '/':
			for i < len(text) && text[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(text) && text[i+1] == '*':
			end := strings.Index(text[i+2:], "*/")
			if end < 0 {
				return -1
			}
			i += end + 3
		case c == text[open]:
			depth++
		case c == closer:
			if depth--; depth == 0 {
				return i + 1
			}
		}
	}
	return -1
}

// goRoutes reads net/http, gorilla/mux, gin, echo and chi registrations (TSA-V0-355).
func goRoutes(name, text string) []StandInRoute {
	out := []StandInRoute{}
	add := func(method, route string, start int) {
		body := callBody(text, start)
		if m := goMethods.FindStringSubmatch(text[start+len(body):]); m != nil && method == "" {
			method = m[1]
		}
		if method == "" {
			method = "ALL"
		}
		if h := goNamedHandler.FindStringSubmatch(body); h != nil && !strings.Contains(body, "func(") {
			body += handlerBody(text, regexp.MustCompile(`\bfunc\s+(?:\([^)]*\)\s*)?`+regexp.QuoteMeta(h[1])+`\s*\(`))
		}
		payload, query := goFields(body)
		out = append(out, newRoute(method, route, name, lineAt(text, start), payload, query))
	}
	for _, m := range goHandleFunc.FindAllStringSubmatchIndex(text, -1) {
		method := ""
		if m[2] >= 0 {
			method = text[m[2]:m[3]]
		}
		add(method, text[m[4]:m[5]], m[0])
	}
	for _, m := range goMethodRoute.FindAllStringSubmatchIndex(text, -1) {
		add(text[m[2]:m[3]], text[m[4]:m[5]], m[0])
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Line < out[j].Line })
	return out
}

func goFields(body string) ([]string, []string) {
	payload, query := map[string]bool{}, map[string]bool{}
	for _, m := range goPayloadField.FindAllStringSubmatch(body, -1) {
		payload[m[1]+m[2]] = true
	}
	for _, m := range goQueryField.FindAllStringSubmatch(body, -1) {
		query[m[1]+m[2]+m[3]] = true
	}
	return sortedKeys(payload), sortedKeys(query)
}

// indentedBody is the block after the line at offset: the following lines indented deeper than
// indent, blank lines included, up to the first line that is not.
func indentedBody(text string, offset int, indent string) string {
	next := strings.IndexByte(text[offset:], '\n')
	if next < 0 {
		return ""
	}
	start, end := offset+next+1, offset+next+1
	for end < len(text) {
		lineEnd := strings.IndexByte(text[end:], '\n')
		line := text[end:]
		if lineEnd >= 0 {
			line = text[end : end+lineEnd]
		}
		if strings.TrimSpace(line) != "" && len(line)-len(strings.TrimLeft(line, " \t")) <= len(indent) {
			break
		}
		if lineEnd < 0 {
			end = len(text)
			break
		}
		end += lineEnd + 1
	}
	return text[start:end]
}

// pythonRoutes reads Flask and FastAPI-style decorators; a route's fields come from the decorated
// function's indented body (TSA-V0-356).
func pythonRoutes(name, text string) []StandInRoute {
	out := []StandInRoute{}
	for _, m := range pyRoute.FindAllStringSubmatchIndex(text, -1) {
		decorator, route, tail := text[m[4]:m[5]], text[m[6]:m[7]], text[m[8]:m[9]]
		methods := []string{decorator}
		if decorator == "route" {
			methods = []string{"GET"}
			if list := pyMethodsList.FindStringSubmatch(tail); list != nil {
				methods = nil
				for _, part := range strings.Split(list[1], ",") {
					if part = strings.Trim(strings.TrimSpace(part), `'"`); part != "" {
						methods = append(methods, part)
					}
				}
			}
		}
		body := ""
		if def := pyDef.FindStringSubmatchIndex(text[m[1]:]); def != nil {
			at := m[1] + def[0]
			body = indentedBody(text, at, text[m[1]+def[2]:m[1]+def[3]])
		}
		payload, query := map[string]bool{}, map[string]bool{}
		for _, f := range pyPayloadField.FindAllStringSubmatch(body, -1) {
			payload[f[1]+f[2]] = true
		}
		for _, alias := range pyAlias.FindAllStringSubmatch(body, -1) {
			field := regexp.MustCompile(`\b` + regexp.QuoteMeta(alias[1]) + `\s*(?:\[\s*['"]([^'"]+)['"]|\.get\(\s*['"]([^'"]+)['"])`)
			for _, f := range field.FindAllStringSubmatch(body, -1) {
				payload[f[1]+f[2]] = true
			}
		}
		for _, f := range pyQueryField.FindAllStringSubmatch(body, -1) {
			query[f[1]+f[2]] = true
		}
		for _, method := range methods {
			out = append(out, newRoute(method, route, name, lineAt(text, m[0]), sortedKeys(payload), sortedKeys(query)))
		}
	}
	return out
}

// rubyRoutes reads Sinatra blocks and Rails `config/routes.rb` entries; a Rails route's fields come
// from the named controller action under the same stand-in (TSA-V0-357). Params are payload fields
// on a write and query fields on a GET.
func rubyRoutes(name, text string, texts map[string]string) []StandInRoute {
	out := []StandInRoute{}
	for _, m := range rbRoute.FindAllStringSubmatchIndex(text, -1) {
		indent, method, route, tail := text[m[2]:m[3]], text[m[4]:m[5]], text[m[6]:m[7]], text[m[8]:m[9]]
		body := ""
		switch {
		case rbController.MatchString(tail):
			c := rbController.FindStringSubmatch(tail)
			root := strings.TrimSuffix(path.Dir(name), "config")
			controller := path.Clean(path.Join(root, "app/controllers", c[1]+"_controller.rb"))
			if source, ok := texts[controller]; ok {
				if def := regexp.MustCompile(`(?m)^([ \t]*)def\s+` + regexp.QuoteMeta(c[2]) + `\b`).FindStringSubmatchIndex(source); def != nil {
					body = indentedBody(source, def[0], source[def[2]:def[3]])
				}
			}
		case strings.Contains(tail, " do") || strings.HasSuffix(strings.TrimSpace(tail), "do"):
			body = indentedBody(text, m[0], indent)
		case strings.Contains(tail, "{"):
			body = tail
		}
		if !strings.HasPrefix(route, "/") {
			route = "/" + route
		}
		fields := map[string]bool{}
		for _, f := range rbParamField.FindAllStringSubmatch(body, -1) {
			fields[f[1]+f[2]+f[3]] = true
		}
		for _, p := range rbPermitFields.FindAllStringSubmatch(body, -1) {
			for _, s := range rbSymbol.FindAllStringSubmatch(p[1], -1) {
				fields[s[1]] = true
			}
		}
		payload, query := sortedKeys(fields), []string{}
		if method == "get" {
			payload, query = []string{}, payload
		}
		out = append(out, newRoute(method, route, name, lineAt(text, m[0]), payload, query))
	}
	return out
}

// wiremockRoutes reads WireMock stub mappings: one mapping or a `mappings` array. A mapping's
// payload fields are its `matchesJsonPath` body patterns and its query fields its
// `queryParameters` keys (TSA-V0-358). WireMock's own `/__admin` API is not listed.
func wiremockRoutes(name, text string) []StandInRoute {
	type request struct {
		Method          string                     `json:"method"`
		URL             string                     `json:"url"`
		URLPath         string                     `json:"urlPath"`
		URLPattern      string                     `json:"urlPattern"`
		URLPathPattern  string                     `json:"urlPathPattern"`
		QueryParameters map[string]json.RawMessage `json:"queryParameters"`
		BodyPatterns    []struct {
			MatchesJSONPath json.RawMessage `json:"matchesJsonPath"`
		} `json:"bodyPatterns"`
	}
	type mapping struct {
		Request *request `json:"request"`
	}
	var doc struct {
		mapping
		Mappings []mapping `json:"mappings"`
	}
	if json.Unmarshal([]byte(text), &doc) != nil {
		return []StandInRoute{}
	}
	all := doc.Mappings
	if doc.Request != nil {
		all = append([]mapping{doc.mapping}, all...)
	}
	out, offset := []StandInRoute{}, 0
	for _, m := range all {
		if m.Request == nil {
			continue
		}
		// Each mapping with a request advances the citation, including one with no route.
		line := 1
		if at := strings.Index(text[offset:], `"request"`); at >= 0 {
			offset += at
			line = lineAt(text, offset)
			offset++
		}
		r := m.Request
		route := r.URL + r.URLPath + r.URLPattern + r.URLPathPattern
		if route == "" {
			continue
		}
		if before, _, cut := strings.Cut(r.URL, "?"); cut {
			route = before
		}
		payload := []string{}
		for _, p := range r.BodyPatterns {
			var expression string
			if json.Unmarshal(p.MatchesJSONPath, &expression) != nil {
				var object struct {
					Expression string `json:"expression"`
				}
				_ = json.Unmarshal(p.MatchesJSONPath, &object)
				expression = object.Expression
			}
			if f := wiremockJSONPath.FindStringSubmatch(expression); f != nil {
				payload = append(payload, f[1])
			}
		}
		sort.Strings(payload)
		query := make([]string, 0, len(r.QueryParameters))
		for k := range r.QueryParameters {
			query = append(query, k)
		}
		sort.Strings(query)
		method := r.Method
		if method == "" || method == "ANY" {
			method = "ALL"
		}
		out = append(out, newRoute(method, route, name, line, payload, query))
	}
	return out
}
