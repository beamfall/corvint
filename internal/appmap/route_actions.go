package appmap

import (
	"errors"
	"path"
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
}

type routeActionParser struct {
	routes  []railsRoute
	abstain []routeNote
	skipped []routeNote
	draw    func(name string, scope actionScope, line int)
}

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

// readRouteActions compiles one rails-routes/0 router file and its one-level draws.
func readRouteActions(r repo, rev string, e blobEntry, data []byte, remaining *int) (*routeActionParser, error) {
	p := &routeActionParser{}
	seen := map[string]bool{}
	var drawErr error
	p.draw = func(name string, scope actionScope, line int) {
		file := path.Join(path.Dir(e.path), "routes", name+".rb")
		switch {
		case !safeRelative(name) || strings.Contains(name, "/"):
			p.abstain = append(p.abstain, routeNote{e.path, line, "non-literal-draw"})
			return
		case seen[file]:
			p.abstain = append(p.abstain, routeNote{e.path, line, "repeated-draw"})
			return
		case drawErr != nil:
			return
		case *remaining == 0:
			drawErr = bound("more than 64 router files including Rails draws")
			return
		}
		seen[file] = true
		*remaining--
		pe, pd, err := r.readOne(rev, file, maxRouterBytes)
		if err != nil {
			var coded *gokernel.Error
			if errors.As(err, &coded) && coded.Code == "appmap-bound-exceeded" {
				drawErr = err
				return
			}
			p.abstain = append(p.abstain, routeNote{e.path, line, "draw-unavailable"})
			return
		}
		p.parse(pe.path, pd, scope, 1)
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
	start, startAbstain := len(p.routes), len(p.abstain)
	stack := []actionScope{initial}
	broken := false
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
		if len(ts) == 1 && ts[0].text == "end" {
			if len(stack) == 1 {
				broken = true
				break
			}
			stack = stack[:len(stack)-1]
			continue
		}
		for j, t := range ts {
			if t.kind == tokIdent && (t.text == "do" && j != len(ts)-1 || t.text == "end") {
				broken = true
			}
		}
		block := ts[len(ts)-1].kind == tokIdent && ts[len(ts)-1].text == "do"
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
		p.routes = p.routes[:start]
		p.abstain = append(p.abstain[:startAbstain], routeNote{file, 1, "unsupported-rails-syntax"})
	}
}

// statement compiles one route DSL statement; it returns the scope a following block opens and
// the abstention reason, if any (AMAP-V0-101).
func (p *routeActionParser) statement(file string, line int, ts []token, block bool, scope actionScope, drawDepth int) (actionScope, string) {
	verb := ts[0].text
	arg, opts, valid := railsActionArgs(ts[1:])
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
		if !valid || !block || !railsOptionsOf(opts, "path", "as", "module", "shallow") || verb == "namespace" && arg == "" {
			return child, "non-literal-rails-scope"
		}
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
		if !ok1 || !ok2 || !ok3 || segment != "" && !literalRoutePath(segment) {
			return child, "non-literal-rails-scope"
		}
		child.prefix = routeJoin(scope.prefix, segment)
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
		if !valid || arg == "" || !railsOptionsOf(opts, "only", "except", "controller", "module", "path", "as", "shallow", "param") {
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
		if !valid || block || !railsOptionsOf(opts, "to", "as", "on", "controller", "action", "via", "path") {
			return child, "non-literal-rails-route"
		}
		if v, has := opts["to"]; has && (v.kind == "redirect" || v.kind == "constant") {
			p.skipped = append(p.skipped, routeNote{file, line, map[string]string{"redirect": "redirect", "constant": "rack-endpoint"}[v.kind]})
			return child, ""
		}
		for _, k := range []string{"to", "as", "on", "controller", "action", "path"} {
			if _, ok := str(k); !ok {
				return child, "non-literal-rails-route"
			}
		}
		return child, p.verbRoute(file, line, verb, arg, opts, scope)
	case "mount":
		p.skipped = append(p.skipped, routeNote{file, line, "mounted-application"})
		return child, ""
	case "draw":
		switch {
		case !valid || arg == "" || len(opts) > 0 || block:
			return child, "non-literal-draw"
		case drawDepth > 0:
			return child, "draw-depth-exceeded"
		}
		p.draw(arg, scope, line)
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
		p.routes = append(p.routes, rt)
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
	p.routes = append(p.routes, rt)
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
		opts[key] = v
	}
	return arg, opts, true
}

func routeValue(ts []token) (routeOpt, bool) {
	switch {
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
