package appmap

import (
	"errors"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// This file compiles Rails route declarations to their handlers (controller path and action) for
// the route-action check and the branch query (AMAP-V0-101). It is separate from the screen
// parser in router_rails.go, which models only GET screens and does not record handlers.

// railsRoute is one compiled route: declaration, verb, path template, helper name and handler.
type railsRoute struct {
	file   string
	line   int
	verb   string
	path   string
	name   string // the helper name without _path/_url; empty when Rails would not name it
	ctrl   string // controller path, e.g. admin/photos
	action string
	basis  string
	guard  string // the enclosing and own constraints/defaults, as read source (AMAP-V0-355)
}

func (rt railsRoute) declaration() string { return rt.file + ":" + strconv.Itoa(rt.line) }

// controllerConstant is the class a controller path names (admin/photos -> Admin::PhotosController).
func controllerConstant(ctrl string) string { return rubyCamelize(ctrl + "_controller") }

type routeNote struct {
	file   string
	line   int
	reason string
}

type routeOpt struct {
	kind string // string, symbol, bool, nil, list, redirect, constant
	str  string
	list []string
	raw  string // the value's tokens as read (railsRaw)
}

type actionResource struct {
	ctrl                                         string
	memberPath, memberName                       string
	collectionPath, collectionName               string
	nestedPrefix, nestedNames, shallowMemberPath string
}

type actionScope struct {
	prefix, names, module     string
	shallowPath, shallowNames string
	shallow                   bool
	res                       *actionResource
	on                        string
	blocked                   bool
	// optional is the enclosing optional scope segment `(:name)`: the prefix before it and the
	// prefix with it, or optionals > 1 when more than one encloses the scope (AMAP-V0-352).
	optionalBefore, optionalWith string
	optionals                    int
	// guard is every enclosing constraints/defaults block and option, as read source; a change
	// to it changes the endpoints it encloses (AMAP-V0-355).
	guard string
}

// add records a route; under one optional scope segment it also records the route without that
// segment, unnamed so the helper table keeps one row per name (AMAP-V0-352).
func (p *routeActionParser) add(rt railsRoute, scope actionScope) {
	rt.guard = scope.guard
	p.routes = append(p.routes, rt)
	if rest, ok := strings.CutPrefix(rt.path, scope.optionalWith); scope.optionals == 1 && ok && (rest == "" || rest[0] == '/') {
		v := rt
		v.path, v.name = routeJoin(scope.optionalBefore, rest), ""
		p.routes = append(p.routes, v)
	}
}

// railsOptionalSegment is a scope path that is one optional dynamic segment, `(:locale)`.
var railsOptionalSegment = regexp.MustCompile(`^\(/?:([a-z_][a-z0-9_]*)\)$`)

// opaqueRoute is a skipped route (rack endpoint, redirect or mounted application) whose literal
// path is known; a mount covers every path under it, and one whose path is unknown covers every
// path (AMAP-V0-142).
type opaqueRoute struct {
	routeNote
	verb, path string // verb is empty for a mount
}

type routeActionParser struct {
	routes  []railsRoute
	abstain []routeNote
	skipped []routeNote
	opaque  []opaqueRoute
	drawn   []string // files read through a draw, in read order
	draw    func(from, name string, scope actionScope, line, depth int)
}

// maxRailsDrawDepth bounds nested draws: a drawn file may draw again to this depth (AMAP-V0-353).
const maxRailsDrawDepth = 4

var pluralActions = []struct{ action, verb, where string }{
	{"index", "GET", "collection"}, {"create", "POST", "collection"}, {"new", "GET", "new"},
	{"edit", "GET", "edit"}, {"show", "GET", "member"}, {"update", "PATCH|PUT", "member"}, {"destroy", "DELETE", "member"},
}

// controllerPath joins a controller name to the enclosing module path; a leading `/` names an
// absolute controller that the module does not prefix, as Rails does.
func controllerPath(module, name string) string {
	if strings.HasPrefix(name, "/") {
		return strings.Trim(name, "/")
	}
	return module + strings.Trim(name, "/")
}

// railsRouterRead is one listed Rails router file compiled with its draws. root is its Rails
// application root when rooted; peerDrawn marks a listed file another listed file draws, which
// is read only through that draw so its routes keep the drawing scope (AMAP-V0-353).
type railsRouterRead struct {
	path, root        string
	rooted, peerDrawn bool
	p                 *routeActionParser
}

// railsAppRoot is the nearest directory at or above the router file's directory that holds
// config/routes.rb or config/application.rb at rev (AMAP-V0-354).
func railsAppRoot(r repo, rev, router string) (string, bool, error) {
	dirs, ask := []string{}, []string{}
	for d := path.Dir(router); ; d = path.Dir(d) {
		dirs = append(dirs, d)
		ask = append(ask, appJoin(d, "config/routes.rb"), appJoin(d, "config/application.rb"))
		if d == "." || d == "/" {
			break
		}
	}
	types, err := r.objectTypes(rev, ask)
	if err != nil {
		return "", false, err
	}
	for i, d := range dirs {
		if types[2*i] == "blob" || types[2*i+1] == "blob" {
			return d, true, nil
		}
	}
	return "", false, nil
}

// readRailsRouters compiles each listed Rails router file with its draws, resolving each file's
// application root; a file without one draws relative to its grandparent directory.
func readRailsRouters(r repo, rev string, files []string, remaining *int) ([]railsRouterRead, error) {
	out := make([]railsRouterRead, 0, len(files))
	drawnBy := map[string]map[string]bool{}
	for _, f := range files {
		e, data, err := r.readOne(rev, f, maxRouterBytes)
		if err != nil {
			return nil, err
		}
		root, rooted, err := railsAppRoot(r, rev, f)
		if err != nil {
			return nil, err
		}
		drawRoot := root
		if !rooted {
			drawRoot = appRootOf(f)
		}
		p, err := readRouteActions(r, rev, e, data, drawRoot, remaining)
		if err != nil {
			return nil, err
		}
		for _, d := range p.drawn {
			if drawnBy[d] == nil {
				drawnBy[d] = map[string]bool{}
			}
			drawnBy[d][f] = true
		}
		out = append(out, railsRouterRead{path: f, root: root, rooted: rooted, p: p})
	}
	for i := range out {
		for g := range drawnBy[out[i].path] {
			if g != out[i].path && !drawnBy[g][out[i].path] {
				out[i].peerDrawn = true
			}
		}
	}
	return out, nil
}

// readRouteActions compiles one rails-routes/0 router file and its draws. A draw reads
// `<appRoot>/config/routes/<name>.rb`, as Rails' draw paths do, under the drawing statement's
// scope; a drawn file may draw again up to maxRailsDrawDepth (AMAP-V0-353).
func readRouteActions(r repo, rev string, e blobEntry, data []byte, appRoot string, remaining *int) (*routeActionParser, error) {
	p := &routeActionParser{}
	seen := map[string]bool{e.path: true}
	var drawErr error
	p.draw = func(from, name string, scope actionScope, line, depth int) {
		file := appJoin(appRoot, "config/routes/"+name+".rb")
		literalName := safeRelative(name)
		for _, seg := range strings.Split(name, "/") {
			literalName = literalName && routeLiteralName.MatchString(seg)
		}
		switch {
		case !literalName:
			p.abstain = append(p.abstain, routeNote{from, line, "non-literal-draw"})
			return
		case seen[file]:
			p.abstain = append(p.abstain, routeNote{from, line, "repeated-draw"})
			return
		case drawErr != nil:
			return
		case *remaining == 0:
			drawErr = bound("more than 64 router files including Rails draws")
			return
		}
		seen[file] = true
		p.drawn = append(p.drawn, file)
		*remaining--
		pe, pd, err := r.readOne(rev, file, maxRouterBytes)
		if err != nil {
			var coded *gokernel.Error
			if errors.As(err, &coded) && coded.Code == "appmap-bound-exceeded" {
				drawErr = err
				return
			}
			p.abstain = append(p.abstain, routeNote{from, line, "draw-unavailable"})
			return
		}
		p.parse(pe.path, pd, scope, depth)
	}
	p.parse(e.path, data, actionScope{}, 0)
	return p, drawErr
}

func (p *routeActionParser) parse(file string, data []byte, initial actionScope, drawDepth int) {
	toks, ok := lexRoutes(string(data), true)
	if !ok {
		p.abstain = append(p.abstain, routeNote{file, 1, "unsupported-rails-syntax"})
		return
	}
	start, startAbstain, startOpaque := len(p.routes), len(p.abstain), len(p.opaque)
	stack := []actionScope{initial}
	broken := false
	brokenAt := 1 // the line of the statement that refused the file (AMAP-V0-371)
	for i := 0; i < len(toks) && !broken; {
		end := i
		for end < len(toks) && !next(toks, end, ";") {
			end++
		}
		ts := toks[i:end]
		i = end + 1
		if len(ts) == 0 {
			continue
		}
		line := ts[0].line
		brokenAt = line
		if len(ts) == 1 && ts[0].text == "end" {
			if len(stack) == 1 {
				broken = true
				break
			}
			stack = stack[:len(stack)-1]
			continue
		}
		// A begin/if/unless/case/while/until region is consumed to its matching `end` and never
		// mined for routes; a statement that provably declares none is skipped. Both stay unknown
		// at their line instead of refusing the file (AMAP-V0-351).
		if ts[0].kind == tokIdent && railsRegionOpeners[ts[0].text] {
			after, inert, ok, transfer := railsRegion(toks, i-len(ts)-1)
			if transfer {
				broken = true
				break
			}
			if ok {
				reason := "rails-region-not-read"
				switch {
				case stack[len(stack)-1].blocked:
					reason = "unresolved-parent"
				case inert:
					reason = "non-route-statement"
				}
				p.abstain = append(p.abstain, routeNote{file, line, reason})
				i = after
				continue
			}
		}
		if railsInert(ts) {
			reason := "non-route-statement"
			if stack[len(stack)-1].blocked {
				reason = "unresolved-parent"
			}
			p.abstain = append(p.abstain, routeNote{file, line, reason})
			continue
		}
		// A block with block parameters (`%w[a b].each do |ns|`, `direct :x do |m|`) is consumed to
		// its matching `end` unread, an unknown at its line (AMAP-V0-371).
		if railsBlockParams(ts) >= 0 {
			after, _, ok, transfer := railsRegion(toks, i-len(ts)-1)
			if ok && !transfer {
				reason := "rails-block-parameters"
				if stack[len(stack)-1].blocked {
					reason = "unresolved-parent"
				}
				p.abstain = append(p.abstain, routeNote{file, line, reason})
				i = after
				continue
			}
		}
		for j, t := range ts {
			if t.kind == tokIdent && (t.text == "do" && j != len(ts)-1 || t.text == "end") {
				broken = true
			}
		}
		block := railsBlock(ts)
		if block {
			ts = ts[:len(ts)-1]
			if len(ts) > 0 && next(ts, len(ts)-1, "|") {
				broken = true
			}
		}
		scope := stack[len(stack)-1]
		child := scope
		reason := ""
		switch {
		case len(ts) == 0:
			reason = "unsupported-rails-syntax"
		case scope.blocked:
			reason = "unresolved-parent"
		case railsWrapper(ts) && block && len(stack) == 1 && drawDepth == 0:
		default:
			child, reason = p.statement(file, line, ts, block, scope, drawDepth)
		}
		if reason != "" {
			p.abstain = append(p.abstain, routeNote{file, line, reason})
			child.blocked = true
		}
		if block {
			if len(stack) >= maxRouteDepth {
				broken = true
				break
			}
			stack = append(stack, child)
		}
	}
	if broken || len(stack) != 1 {
		if !broken {
			brokenAt = 1 // a block left open: no one statement refused
		}
		p.routes, p.opaque = p.routes[:start], p.opaque[:startOpaque]
		p.abstain = append(p.abstain[:startAbstain], routeNote{file, brokenAt, "unsupported-rails-syntax"})
	}
}

var railsRegionOpeners = map[string]bool{"begin": true, "if": true, "unless": true, "case": true, "while": true, "until": true}

// railsRouteWords are the route DSL and mapper-extension words a statement may name to declare a
// route; a statement naming one (not as a method of another receiver or a symbol) is never inert.
var railsRouteWords = map[string]bool{"get": true, "post": true, "patch": true, "put": true, "delete": true, "match": true,
	"root": true, "resources": true, "resource": true, "namespace": true, "scope": true, "member": true, "collection": true,
	"mount": true, "draw": true, "constraints": true, "defaults": true, "concern": true, "concerns": true, "direct": true,
	"resolve": true, "devise_for": true, "devise_scope": true, "instance_eval": true, "send": true, "public_send": true,
	"eval": true, "load": true, "self": true, "yield": true}

// railsNamesRoute reports whether ts names a route word, passes the mapper or opens a block.
func railsNamesRoute(ts []token) bool {
	for j, t := range ts {
		if t.kind == tokIdent && (t.text == "do" || railsRouteWords[t.text] && (j == 0 || !next(ts, j-1, ".", ":"))) {
			return true
		}
	}
	return false
}

// railsInert reports whether one statement provably declares no route (AMAP-V0-351): a require of
// a library (not a relative or routes path), a `puts`/`warn`/`raise`/`p`/`pp` call, a region
// keyword line (`rescue`, `else`, `elsif`, `ensure`, `when`, `then`), a local assignment, or a call
// on a constant receiver other than `Rails.application`, each naming no route word, opening no
// block and evaluating only an inert expression (railsInertExpr): `ignored = install_routes` may
// call a route-defining helper, so it is not inert.
func railsInert(ts []token) bool {
	if len(ts) == 0 || ts[0].kind != tokIdent || railsNamesRoute(ts) {
		return false
	}
	switch w := ts[0].text; {
	case w == "require":
		return len(ts) == 2 && literal(ts[1]) && !strings.HasPrefix(ts[1].text, ".") && !strings.HasPrefix(ts[1].text, "/") && !strings.Contains(ts[1].text, "routes")
	case w == "rescue":
		if n := len(ts); n >= 4 && isPunct(ts[n-3], "=") && isPunct(ts[n-2], ">") && ts[n-1].kind == tokIdent {
			ts = ts[:n-3]
		}
		return railsInertExpr(ts[1:])
	case w == "puts" || w == "warn" || w == "raise" || w == "p" || w == "pp" ||
		w == "else" || w == "elsif" || w == "ensure" || w == "when" || w == "then":
		return railsInertExpr(ts[1:])
	case w[0] >= 'A' && w[0] <= 'Z':
		return len(ts) >= 3 && next(ts, 1, ".", ":") && railsInertExpr(ts)
	case len(ts) >= 3 && isPunct(ts[1], "=") && !next(ts, 2, "=", ">", "~"):
		return railsInertExpr(ts[2:])
	}
	return false
}

// railsInertExpr reports whether an expression provably calls nothing that may declare a route:
// it holds only literals (no interpolation), punctuation, constants, `true`/`false`/`nil`/`and`/
// `or`/`not`, symbols and method names on a receiver (after `.`, `:` or `::`), never `self`, a bare
// lower-case name (a local or a call of a helper) or `Rails.application`.
func railsInertExpr(ts []token) bool {
	for j, t := range ts {
		switch {
		case t.kind == tokString || t.kind == tokTemplate:
			if !literal(t) {
				return false
			}
		case t.kind == tokIdent:
			w := t.text
			switch {
			case w == "self":
				return false
			case w == "Rails" && next(ts, j+1, ".") && j+2 < len(ts) && ts[j+2].text == "application":
				return false
			case w[0] >= 'A' && w[0] <= 'Z', w == "true", w == "false", w == "nil", w == "and", w == "or", w == "not":
			case j > 0 && next(ts, j-1, ".", ":"):
			default:
				return false
			}
		}
	}
	return true
}

// railsRegion reads the region a begin/if/unless/case/while/until statement at toks[i] opens up
// to its matching `end`: the token index after it, whether every statement in it is inert (an
// opener's own condition counts as inert when it names no route and is an inert expression), and
// whether the end was found. The end is found only when every keyword `end` in the region is a
// statement by itself and every keyword `do` ends its statement; an opener inside an expression
// (`x = if ...`, `case`, `begin`) or a one-line block leaves it not found (AMAP-V0-366). transfer
// is set when the region holds `next`, `break`, `return`, `redo`, `retry` or `throw`, which may
// decide whether the statements after it run at all, so the caller refuses the file.
func railsRegion(toks []token, i int) (after int, inert, ok, transfer bool) {
	depth := 0
	inert = true
	for first := true; i < len(toks); first = false {
		end := i
		for end < len(toks) && !next(toks, end, ";") {
			end++
		}
		ts := toks[i:end]
		i = end + 1
		if len(ts) == 0 {
			continue
		}
		opens, closes := 0, 0
		for j := range ts {
			if !rubyKeyword(ts, j) {
				continue
			}
			switch w := ts[j].text; {
			case railsTransfers[w]:
				transfer = true
			case w == "end":
				if len(ts) != 1 {
					return 0, false, false, transfer
				}
				closes++
			case w == "do":
				if j != len(ts)-1 && railsBlockParams(ts) != j {
					return 0, false, false, transfer
				}
				if !(railsRegionOpeners[ts[0].text] && (ts[0].text == "while" || ts[0].text == "until")) {
					opens++
				}
			case railsRegionOpeners[w] && j == 0:
				opens++
			case w == "case" || w == "begin":
				return 0, false, false, transfer
			case railsRegionOpeners[w] && (ts[j-1].kind == tokPunct && !isPunct(ts[j-1], ")") && !isPunct(ts[j-1], "]") && !isPunct(ts[j-1], "}")):
				return 0, false, false, transfer // `x = if ...` opens an expression region
			}
		}
		switch {
		case len(ts) == 1 && closes == 1:
		case first || opens > 0 && railsRegionOpeners[ts[0].text]:
			inert = inert && !railsNamesRoute(ts[1:]) && railsInertExpr(ts[1:])
		default:
			inert = inert && railsInert(ts)
		}
		if depth += opens - closes; depth <= 0 {
			return i, inert, depth == 0, transfer
		}
	}
	return 0, false, false, transfer
}

// railsBlockParams returns the index of the keyword `do` when the statement ts ends in a block
// with block parameters, `do |a, b|` (AMAP-V0-371), or -1. The parameters are names, `,`, `*`,
// `&` and parentheses.
func railsBlockParams(ts []token) int {
	n := len(ts)
	if n < 4 || !isPunct(ts[n-1], "|") {
		return -1
	}
	for j := n - 2; j >= 1; j-- {
		t := ts[j]
		switch {
		case isPunct(t, "|"):
			if j == n-2 || !(ts[j-1].kind == tokIdent && ts[j-1].text == "do" && rubyKeyword(ts, j-1)) {
				return -1
			}
			return j - 1
		case t.kind == tokIdent || next(ts, j, ",", "*", "&", "(", ")"):
		default:
			return -1
		}
	}
	return -1
}

// railsTransfers are the Ruby words that transfer control out of a region (see railsRegion).
var railsTransfers = map[string]bool{"next": true, "break": true, "return": true, "redo": true, "retry": true, "throw": true}

// rubyKeyword reports whether ts[j] is a bare word, not a method name after `.`, a symbol or
// constant path after `:`, nor a hash key before `:`.
func rubyKeyword(ts []token, j int) bool {
	return ts[j].kind == tokIdent && !(j > 0 && next(ts, j-1, ".", ":")) && !(next(ts, j+1, ":") && !next(ts, j+2, ":"))
}

// railsBlock reports whether a statement ends in the keyword `do`, not the symbol `:do`.
func railsBlock(ts []token) bool {
	return len(ts) > 0 && ts[len(ts)-1].kind == tokIdent && ts[len(ts)-1].text == "do" && rubyKeyword(ts, len(ts)-1)
}

// statement compiles one route DSL statement; it returns the scope a following block opens and
// the abstention reason, if any (AMAP-V0-101).
func (p *routeActionParser) statement(file string, line int, ts []token, block bool, scope actionScope, drawDepth int) (actionScope, string) {
	verb := ts[0].text
	arg, opts, valid := railsActionArgs(ts[1:])
	if verb == "constraints" || verb == "defaults" {
		scope.guard += verb + "(" + railsRaw(ts[1:]) + ");"
	} else {
		for _, k := range []string{"constraints", "defaults"} {
			if v, has := opts[k]; has {
				scope.guard += k + ":" + v.raw + ";"
			}
		}
	}
	child := scope
	str := func(k string) (string, bool) {
		v, has := opts[k]
		if !has {
			return "", true
		}
		return v.str, v.kind == "string" || v.kind == "symbol"
	}
	switch verb {
	case "constraints", "defaults":
		if !block {
			return child, "rails-" + verb
		}
		return child, ""
	case "namespace", "scope":
		segments := railsSegmentConstraints(arg, opts)
		if !valid || !block || !railsOptionsOf(opts, append([]string{"path", "as", "module", "shallow", "constraints", "defaults"}, segments...)...) ||
			!railsHashOptions(opts) || verb == "namespace" && arg == "" {
			return child, "non-literal-rails-scope"
		}
		for _, k := range segments { // `scope "(:locale)", locale: /en|fr/` (AMAP-V0-371)
			scope.guard += k + ":" + opts[k].raw + ";"
		}
		child.guard = scope.guard
		segment, name, module := arg, "", ""
		if verb == "namespace" {
			name, module = arg, arg
		}
		var ok1, ok2, ok3 bool
		if v, has := opts["path"]; has {
			segment, ok1 = v.str, v.kind == "string" || v.kind == "symbol"
		} else {
			ok1 = true
		}
		if v, has := opts["as"]; has {
			name, ok2 = v.str, v.kind == "string" || v.kind == "symbol"
		} else {
			ok2 = true
		}
		if v, has := opts["module"]; has {
			module, ok3 = v.str, v.kind == "string" || v.kind == "symbol"
		} else {
			ok3 = true
		}
		optional := railsOptionalSegment.FindStringSubmatch(segment)
		if optional != nil && ok1 {
			segment = ":" + optional[1]
		}
		if !ok1 || !ok2 || !ok3 || segment != "" && !literalRoutePath(segment) {
			return child, "non-literal-rails-scope"
		}
		child.prefix = routeJoin(scope.prefix, segment)
		if optional != nil {
			if child.optionals++; child.optionals > 1 {
				p.abstain = append(p.abstain, routeNote{file, line, "rails-optional-segments"})
			}
			child.optionalBefore, child.optionalWith = scope.prefix, child.prefix
		}
		if name != "" {
			child.names = scope.names + name + "_"
		}
		if module != "" {
			child.module = scope.module + strings.Trim(module, "/") + "/"
		}
		child.shallowPath, child.shallowNames = child.prefix, child.names
		child.res, child.on = nil, ""
		if v, has := opts["shallow"]; has {
			if v.kind != "bool" {
				return child, "non-literal-rails-scope"
			}
			child.shallow = v.str == "true"
		}
		return child, ""
	case "resources", "resource":
		if !valid || arg == "" || !railsOptionsOf(opts, "only", "except", "controller", "module", "path", "as", "shallow", "param", "constraints", "defaults") || !railsHashOptions(opts) {
			return child, "non-literal-rails-resource"
		}
		return p.resource(file, line, verb == "resources", arg, opts, scope)
	case "member", "collection":
		if !valid || arg != "" || len(opts) > 0 || !block || scope.res == nil {
			return child, "unsupported-rails-scope"
		}
		child.on = verb
		return child, ""
	case "get", "post", "patch", "put", "delete", "match", "root":
		if !valid || block || !railsOptionsOf(opts, "to", "as", "on", "controller", "action", "via", "path", "constraints", "defaults") || !railsHashOptions(opts) {
			return child, "non-literal-rails-route"
		}
		if v, has := opts["to"]; has && (v.kind == "redirect" || v.kind == "constant") {
			note := routeNote{file, line, map[string]string{"redirect": "redirect", "constant": "rack-endpoint"}[v.kind]}
			p.skipped = append(p.skipped, note)
			segment := arg
			if v, has := opts["path"]; has {
				segment = v.str
			}
			switch {
			case scope.res == nil && verb == "root":
				p.opaque = append(p.opaque, opaqueRoute{note, "GET", routeJoin(scope.prefix, "")})
			case scope.res == nil && verb != "match" && segment != "" && literalRoutePath(segment):
				p.opaque = append(p.opaque, opaqueRoute{note, strings.ToUpper(verb), routeJoin(scope.prefix, segment)})
			default:
				p.opaque = append(p.opaque, opaqueRoute{note, "", "/"}) // an unknown path may be any path
			}
			return child, ""
		}
		for _, k := range []string{"to", "as", "on", "controller", "action", "path"} {
			if _, ok := str(k); !ok {
				return child, "non-literal-rails-route"
			}
		}
		return child, p.verbRoute(file, line, verb, arg, opts, scope)
	case "mount":
		note := routeNote{file, line, "mounted-application"}
		p.skipped = append(p.skipped, note)
		at, ok := mountPath(ts[1:])
		if !ok || scope.res != nil {
			at, scope.prefix = "/", "" // an unknown mount path may be any path
		}
		p.opaque = append(p.opaque, opaqueRoute{note, "", routeJoin(scope.prefix, at)})
		return child, ""
	case "draw":
		switch {
		case !valid || arg == "" || len(opts) > 0 || block:
			return child, "non-literal-draw"
		case drawDepth >= maxRailsDrawDepth:
			return child, "draw-depth-exceeded"
		}
		p.draw(file, arg, scope, line, drawDepth+1)
		return child, ""
	}
	return child, "unsupported-rails-statement"
}

func (p *routeActionParser) resource(file string, line int, plural bool, arg string, opts map[string]routeOpt, scope actionScope) (actionScope, string) {
	child := scope
	text := func(k, def string) (string, bool) {
		v, has := opts[k]
		if !has {
			return def, true
		}
		return v.str, v.kind == "string" || v.kind == "symbol"
	}
	segment, ok1 := text("path", arg)
	name, ok2 := text("as", arg)
	ctrlName, ok3 := text("controller", "")
	module, ok4 := text("module", "")
	param, ok5 := text("param", "id")
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !routeLiteralName.MatchString(arg) || !routeLiteralName.MatchString(param) {
		return child, "non-literal-rails-resource"
	}
	if !literalRoutePath(segment) {
		return child, "unsupported-route-pattern"
	}
	shallow := scope.shallow
	if v, has := opts["shallow"]; has {
		if v.kind != "bool" {
			return child, "non-literal-rails-resource"
		}
		shallow = v.str == "true"
	}
	if ctrlName == "" {
		ctrlName = arg
		if !plural {
			ctrlName = railsPlural(arg)
			if ctrlName == "" {
				return child, "unsupported-rails-inflection"
			}
		}
	}
	ctrl := scope.module
	if module != "" {
		ctrl += strings.Trim(module, "/") + "/"
	}
	ctrl = controllerPath(ctrl, ctrlName)
	selected := map[string]bool{}
	for _, a := range pluralActions {
		selected[a.action] = plural || a.action != "index"
	}
	for _, k := range []string{"only", "except"} {
		v, has := opts[k]
		if !has {
			continue
		}
		list := v.list
		switch v.kind {
		case "string", "symbol":
			list = []string{v.str}
		case "list":
		default:
			return child, "non-literal-rails-resource"
		}
		named := map[string]bool{}
		for _, a := range list {
			named[a] = true
		}
		for a := range selected {
			if k == "only" && !named[a] || k == "except" && named[a] {
				selected[a] = false
			}
		}
	}
	singular := name
	if plural {
		singular = railsSingular(name)
	}
	collection := routeJoin(scope.prefix, segment)
	member, memberName := collection, ""
	if singular != "" {
		memberName = scope.names + singular
	}
	if plural {
		base := collection
		if shallow {
			base = routeJoin(scope.shallowPath, segment)
			if singular != "" {
				memberName = scope.shallowNames + singular
			}
		}
		member = routeJoin(base, ":"+param)
	}
	collectionName := scope.names + name
	if !plural {
		collectionName = memberName
	}
	for _, a := range pluralActions {
		if !selected[a.action] {
			continue
		}
		rt := railsRoute{file: file, line: line, verb: a.verb, ctrl: ctrl, action: a.action, basis: map[bool]string{true: "resources", false: "resource"}[plural]}
		switch a.where {
		case "collection":
			rt.path, rt.name = collection, collectionName
		case "new":
			rt.path = routeJoin(collection, "new")
			if memberName != "" {
				rt.name = "new_" + scope.names + singular
			}
		case "edit":
			rt.path = routeJoin(member, "edit")
			if memberName != "" {
				rt.name = "edit_" + memberName
			}
		default:
			rt.path, rt.name = member, memberName
		}
		p.add(rt, scope)
	}
	res := &actionResource{ctrl: ctrl, memberPath: member, memberName: memberName, collectionPath: collection, collectionName: collectionName}
	if plural {
		parent := singular
		if parent == "" {
			parent = "parent"
		}
		res.nestedPrefix = routeJoin(collection, ":"+parent+"_id")
	} else {
		res.nestedPrefix = collection
	}
	if memberName != "" {
		res.nestedNames = memberName + "_"
	}
	child.res, child.on = res, ""
	child.prefix, child.names = res.nestedPrefix, res.nestedNames
	child.shallow = shallow
	return child, ""
}

func (p *routeActionParser) verbRoute(file string, line int, verb, arg string, opts map[string]routeOpt, scope actionScope) string {
	ctrl, action := "", ""
	if v, has := opts["to"]; has {
		c, a, found := strings.Cut(v.str, "#")
		if !found || c == "" || !routeLiteralName.MatchString(a) {
			return "no-handler"
		}
		ctrl, action = controllerPath(scope.module, c), a
	}
	if c, has := opts["controller"]; has {
		ctrl = controllerPath(scope.module, c.str)
	}
	if a, has := opts["action"]; has {
		action = a.str
	}
	if verb == "root" && ctrl == "" && strings.Contains(arg, "#") {
		c, a, _ := strings.Cut(arg, "#")
		ctrl, action, arg = controllerPath(scope.module, c), a, ""
	}
	on := scope.on
	if v, has := opts["on"]; has {
		if v.str != "member" && v.str != "collection" || scope.res == nil {
			return "unsupported-rails-scope"
		}
		on = v.str
	}
	segment := arg
	if v, has := opts["path"]; has {
		segment = v.str
	}
	name := ""
	rt := railsRoute{file: file, line: line, verb: strings.ToUpper(verb), basis: "explicit"}
	switch {
	case verb == "root":
		rt.verb, rt.path, name, rt.basis = "GET", routeJoin(scope.prefix, ""), scope.names+"root", "root"
	case scope.res != nil:
		if action == "" && routeLiteralName.MatchString(arg) {
			action = arg
		}
		if ctrl == "" {
			ctrl = scope.res.ctrl
		}
		if segment == "" {
			segment = action
		}
		rt.basis = "resource-route"
		switch on {
		case "member":
			rt.path = routeJoin(scope.res.memberPath, segment)
			if scope.res.memberName != "" {
				name = action + "_" + scope.res.memberName
			}
		case "collection":
			rt.path = routeJoin(scope.res.collectionPath, segment)
			if scope.res.collectionName != "" {
				name = action + "_" + scope.res.collectionName
			}
		default:
			rt.path = routeJoin(scope.res.nestedPrefix, segment)
			if scope.res.nestedNames != "" {
				name = scope.res.nestedNames + action
			}
		}
	default:
		rt.path = routeJoin(scope.prefix, segment)
		parts := strings.Split(strings.Trim(arg, "/"), "/")
		literal := arg != ""
		for _, s := range parts {
			literal = literal && routeLiteralName.MatchString(s)
		}
		if ctrl == "" && literal && len(parts) >= 2 {
			ctrl, action = scope.module+strings.Join(parts[:len(parts)-1], "/"), parts[len(parts)-1]
		}
		if literal {
			name = scope.names + strings.Join(parts, "_")
		}
	}
	if v, has := opts["as"]; has {
		name = scope.names + v.str
	}
	if ctrl == "" || action == "" || !routeLiteralName.MatchString(action) {
		return "no-handler"
	}
	if !literalRoutePath(rt.path) {
		return "unsupported-route-pattern"
	}
	if verb == "match" {
		rt.verb = "ANY"
		if v, has := opts["via"]; has {
			list := v.list
			if v.kind == "symbol" || v.kind == "string" {
				list = []string{v.str}
			}
			if len(list) == 0 {
				return "non-literal-rails-route"
			}
			for i := range list {
				list[i] = strings.ToUpper(list[i])
			}
			if list[0] != "ALL" {
				rt.verb = strings.Join(list, "|")
			}
		}
	}
	if !routeLiteralName.MatchString(name) {
		name = ""
	}
	rt.ctrl, rt.action, rt.name = ctrl, action, name
	p.add(rt, scope)
	return ""
}

// railsPlural pluralises a singular resource name for its controller (`resource :profile` routes
// to ProfilesController); names whose plural is not a bare `s` are refused.
func railsPlural(s string) string {
	if !routeLiteralName.MatchString(s) {
		return ""
	}
	for _, suffix := range []string{"s", "x", "z", "ch", "sh", "y", "f", "fe", "o"} {
		if strings.HasSuffix(s, suffix) {
			return ""
		}
	}
	return s + "s"
}

// railsHashOptions reports whether the `constraints:` and `defaults:` options, when given, are hash
// literals. Neither is evaluated: a constrained route is read as reachable by every call its path
// matches, which over-approximates the join (AMAP-V0-352).
func railsHashOptions(opts map[string]routeOpt) bool {
	for _, k := range []string{"constraints", "defaults"} {
		if v, has := opts[k]; has && v.kind != "hash" {
			return false
		}
	}
	return true
}

// railsSegmentConstraints returns, sorted, the option keys of a scope statement that constrain
// one of its path's dynamic segments by a regexp (`scope "(:locale)", locale: /en|fr/`), which
// Rails reads as a segment constraint; the path is the `path:` option or the positional argument
// (AMAP-V0-371).
func railsSegmentConstraints(arg string, opts map[string]routeOpt) []string {
	p := arg
	if v, has := opts["path"]; has {
		p = v.str
	}
	var out []string
	for k, v := range opts {
		if v.kind != "regex" {
			continue
		}
		for _, seg := range strings.Split(p, "/") {
			if strings.Trim(seg, "()/") == ":"+k {
				out = append(out, k)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

func railsOptionsOf(opts map[string]routeOpt, allowed ...string) bool {
	for k := range opts {
		found := false
		for _, a := range allowed {
			found = found || k == a
		}
		if !found {
			return false
		}
	}
	return true
}

// railsActionArgs reads a route statement's positional argument and literal options: strings,
// symbols, booleans, `nil`, symbol or string arrays (`%i[]`/`%w[]` included), `redirect(...)`
// and constants. A `"path" => "c#a"` pair is the positional path with a `to:` option.
func railsActionArgs(ts []token) (string, map[string]routeOpt, bool) {
	opts := map[string]routeOpt{}
	arg := ""
	if len(ts) > 1 && next(ts, 0, "(") && next(ts, len(ts)-1, ")") {
		ts = ts[1 : len(ts)-1]
	}
	elements := [][]token{}
	depth, startEl := 0, 0
	for i, t := range ts {
		switch {
		case next(ts, i, "(", "[", "{"):
			depth++
		case next(ts, i, ")", "]", "}"):
			depth--
		case depth == 0 && isPunct(t, ","):
			elements = append(elements, ts[startEl:i])
			startEl = i + 1
		}
	}
	if len(ts) > 0 {
		elements = append(elements, ts[startEl:])
	}
	for n, el := range elements {
		if len(el) == 0 {
			return arg, opts, false
		}
		key, value := "", el
		rocket := -1
		for i := 0; i+1 < len(el); i++ {
			if isPunct(el[i], "=") && isPunct(el[i+1], ">") {
				rocket = i
				break
			}
		}
		switch {
		case len(el) >= 2 && el[0].kind == tokIdent && isPunct(el[1], ":") && (!next(el, 2, ":") || el[0].text[0] < 'A' || el[0].text[0] > 'Z'):
			// `key: :sym` lexes like `Const::Name`; only a capitalised name is a constant path.
			key, value = el[0].text, el[2:]
		case rocket > 0:
			left, ok := routeValue(el[:rocket])
			if !ok || left.kind != "string" && left.kind != "symbol" {
				return arg, opts, false
			}
			key, value = left.str, el[rocket+2:]
			if n == 0 && left.kind == "string" {
				if arg != "" {
					return arg, opts, false
				}
				arg, key = left.str, "to"
			}
		}
		v, ok := routeValue(value)
		if !ok {
			return arg, opts, false
		}
		if key == "" {
			if arg != "" || n != 0 || v.kind != "string" && v.kind != "symbol" {
				return arg, opts, false
			}
			arg = v.str
			continue
		}
		if _, dup := opts[key]; dup {
			return arg, opts, false
		}
		v.raw = railsRaw(value)
		opts[key] = v
	}
	return arg, opts, true
}

// railsRaw spells tokens as read, a string by its decoded text and a regexp by its source, so
// that two declarations compare equal only when their tokens do.
func railsRaw(ts []token) string {
	parts := make([]string, len(ts))
	for i, t := range ts {
		switch {
		case t.kind == tokString:
			parts[i] = strconv.Quote(t.text)
		case t.kind == tokTemplate && t.text == "opaque-regex":
			parts[i] = "regex" + strconv.Quote(t.code)
		default:
			parts[i] = t.text
		}
	}
	return strings.Join(parts, " ")
}

func routeValue(ts []token) (routeOpt, bool) {
	switch {
	case len(ts) == 1 && ts[0].kind == tokTemplate && ts[0].text == "opaque-regex":
		return routeOpt{kind: "regex"}, true
	case len(ts) >= 2 && isPunct(ts[0], "{") && isPunct(ts[len(ts)-1], "}"):
		return routeOpt{kind: "hash"}, railsHashLiteral(ts[1 : len(ts)-1])
	case len(ts) == 1 && literal(ts[0]):
		return routeOpt{kind: "string", str: ts[0].text}, true
	case len(ts) == 2 && isPunct(ts[0], ":") && ts[1].kind == tokIdent:
		return routeOpt{kind: "symbol", str: ts[1].text}, true
	case len(ts) == 2 && isPunct(ts[0], ":") && literal(ts[1]):
		return routeOpt{kind: "symbol", str: ts[1].text}, true
	case len(ts) == 1 && ts[0].kind == tokIdent && (ts[0].text == "true" || ts[0].text == "false"):
		return routeOpt{kind: "bool", str: ts[0].text}, true
	case len(ts) == 1 && ts[0].kind == tokIdent && ts[0].text == "nil":
		return routeOpt{kind: "nil"}, true
	case len(ts) >= 2 && ts[0].kind == tokIdent && ts[0].text == "redirect" && isPunct(ts[1], "("):
		return routeOpt{kind: "redirect"}, true
	case len(ts) >= 1 && ts[0].kind == tokIdent && ts[0].text[0] >= 'A' && ts[0].text[0] <= 'Z':
		return routeOpt{kind: "constant"}, true
	case len(ts) >= 2 && isPunct(ts[0], "[") && isPunct(ts[len(ts)-1], "]"):
		list := []string{}
		inner := ts[1 : len(ts)-1]
		for i := 0; i < len(inner); {
			switch {
			case isPunct(inner[i], ":") && i+1 < len(inner) && inner[i+1].kind == tokIdent:
				list = append(list, inner[i+1].text)
				i += 2
			case literal(inner[i]):
				list = append(list, inner[i].text)
				i++
			default:
				return routeOpt{}, false
			}
			if i < len(inner) {
				if !isPunct(inner[i], ",") {
					return routeOpt{}, false
				}
				i++
			}
		}
		return routeOpt{kind: "list", list: list}, true
	case len(ts) >= 4 && isPunct(ts[0], "%") && ts[1].kind == tokIdent && (ts[1].text == "i" || ts[1].text == "w") &&
		(isPunct(ts[2], "[") && isPunct(ts[len(ts)-1], "]") || isPunct(ts[2], "(") && isPunct(ts[len(ts)-1], ")")):
		list := []string{}
		for _, t := range ts[3 : len(ts)-1] {
			if t.kind != tokIdent {
				return routeOpt{}, false
			}
			list = append(list, t.text)
		}
		return routeOpt{kind: "list", list: list}, true
	}
	return routeOpt{}, false
}

// railsHashLiteral reports whether ts is the body of a hash literal whose keys are names, strings
// or symbols and whose values are literal route values (regexps and nested hashes included); it
// is read for its shape only, never evaluated (AMAP-V0-352).
func railsHashLiteral(ts []token) bool {
	start, depth := 0, 0
	for i := 0; i <= len(ts); i++ {
		if i < len(ts) {
			switch {
			case next(ts, i, "(", "[", "{"):
				depth++
				continue
			case next(ts, i, ")", "]", "}"):
				depth--
				continue
			case depth != 0 || !isPunct(ts[i], ","):
				continue
			}
		}
		el := ts[start:i]
		start = i + 1
		if len(el) == 0 {
			if i == len(ts) && len(ts) > 0 {
				return true // a trailing comma
			}
			return len(ts) == 0
		}
		var value []token
		switch {
		case len(el) >= 3 && el[0].kind == tokIdent && isPunct(el[1], ":") && !(isPunct(el[2], ":") && el[0].text[0] >= 'A' && el[0].text[0] <= 'Z'):
			value = el[2:]
		default:
			for j := 0; j+1 < len(el); j++ {
				if isPunct(el[j], "=") && isPunct(el[j+1], ">") {
					if k, ok := routeValue(el[:j]); ok && (k.kind == "string" || k.kind == "symbol") {
						value = el[j+2:]
					}
					break
				}
			}
		}
		if v, ok := routeValue(value); !ok || v.kind == "redirect" || v.kind == "constant" {
			return false
		}
	}
	return true
}

// mountPath reads the literal path of `mount App => "/p"` or `mount App, at: "/p"`.
func mountPath(ts []token) (string, bool) {
	for i := 0; i+1 < len(ts); i++ {
		var rest []token
		switch {
		case isPunct(ts[i], "=") && isPunct(ts[i+1], ">"):
			rest = ts[i+2:]
		case ts[i].kind == tokIdent && ts[i].text == "at" && isPunct(ts[i+1], ":") && i > 0 && isPunct(ts[i-1], ","):
			rest = ts[i+2:]
		default:
			continue
		}
		for j, t := range rest {
			if isPunct(t, ",") {
				rest = rest[:j]
				break
			}
		}
		v, ok := routeValue(rest)
		return v.str, ok && v.kind == "string" && literalRoutePath(v.str)
	}
	return "", false
}
