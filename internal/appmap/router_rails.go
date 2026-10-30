package appmap

import (
	"errors"
	"github.com/Beamfall/corvint/internal/gokernel"
	"path"
	"strings"
)

type railsScope struct {
	prefix, names, shallowPath, shallowNames               string
	memberPath, memberName, collectionPath, collectionName string
	shallow                                                bool
	blocked                                                bool
	anchors                                                []Anchor
}

type railsParser struct {
	states   []rawState
	unknowns []Unknown
	draw     func(string, railsScope, int)
	anchors  map[string]Anchor
	helpers  map[string]int
}

func readRailsRouter(r repo, rev string, e blobEntry, data []byte, remaining *int) ([]rawState, []Unknown, error) {
	p := &railsParser{anchors: map[string]Anchor{}, helpers: map[string]int{}}
	seen := map[string]bool{}
	var drawErr error
	p.draw = func(name string, scope railsScope, line int) {
		file := path.Join(path.Dir(e.path), "routes", name+".rb")
		if !safeRelative(name) || strings.Contains(name, "/") {
			p.unknowns = append(p.unknowns, routerUnknown(e, line, "non-literal-draw"))
			return
		}
		if seen[file] {
			p.unknowns = append(p.unknowns, routerUnknown(e, line, "repeated-draw"))
			return
		}
		seen[file] = true
		if drawErr != nil {
			return
		}
		if *remaining == 0 {
			drawErr = bound("more than 64 router files including Rails draws")
			return
		}
		*remaining--
		pe, pd, err := r.readOne(rev, file, maxRouterBytes)
		if err != nil {
			var coded *gokernel.Error
			if errors.As(err, &coded) && coded.Code == "appmap-bound-exceeded" {
				drawErr = err
				return
			}
			p.unknowns = append(p.unknowns, routerUnknown(e, line, "draw-unavailable"))
			return
		}
		scope.anchors = append(append([]Anchor{}, scope.anchors...), p.anchors[e.path])
		p.parse(pe, pd, scope, 1)
	}
	p.parse(e, data, railsScope{}, 0)
	return p.states, p.unknowns, drawErr
}

// Ruby routes are admitted as literal, newline/semicolon-delimited DSL statements. Unsupported
// blocks are consumed with a blocked scope, never mined for apparently literal descendants.
func (p *railsParser) parse(e blobEntry, data []byte, initial railsScope, drawDepth int) {
	if p.anchors == nil {
		p.anchors = map[string]Anchor{}
	}
	if p.helpers == nil {
		p.helpers = map[string]int{}
	}
	p.anchors[e.path] = wholeFile(e, data)
	toks, ok := lexRoutes(string(data), true)
	if !ok {
		p.unknowns = append(p.unknowns, routerUnknown(e, 1, "unsupported-rails-syntax"))
		return
	}
	startStates := len(p.states)
	stack := []railsScope{initial}
	malformed := false
	unsafe := false
	for i := 0; i < len(toks); {
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
				malformed = true
				break
			}
			stack = stack[:len(stack)-1]
			continue
		}
		// As in the endpoint reader (AMAP-V0-351), a begin/if/unless/case/while/until region is
		// consumed to its matching `end` unread and a statement that provably declares no route is
		// skipped, each an unknown at its line instead of a refusal of the file (AMAP-V0-366).
		if ts[0].kind == tokIdent && railsRegionOpeners[ts[0].text] {
			after, inert, ok, transfer := railsRegion(toks, i-len(ts)-1)
			if transfer {
				unsafe = true
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
				p.unknowns = append(p.unknowns, routerUnknown(e, line, reason))
				i = after
				continue
			}
		}
		if railsInert(ts) {
			reason := "non-route-statement"
			if stack[len(stack)-1].blocked {
				reason = "unresolved-parent"
			}
			p.unknowns = append(p.unknowns, routerUnknown(e, line, reason))
			continue
		}
		// A block with block parameters is consumed unread, as in the endpoint reader (AMAP-V0-371).
		if railsBlockParams(ts) >= 0 {
			if after, _, ok, transfer := railsRegion(toks, i-len(ts)-1); ok && !transfer {
				reason := "rails-block-parameters"
				if stack[len(stack)-1].blocked {
					reason = "unresolved-parent"
				}
				p.unknowns = append(p.unknowns, routerUnknown(e, line, reason))
				i = after
				continue
			}
		}
		scope := stack[len(stack)-1]
		for j, t := range ts {
			if t.kind == tokIdent && t.text == "do" && j != len(ts)-1 {
				unsafe = true
			}
		}

		block := railsBlock(ts)
		if block {
			ts = ts[:len(ts)-1]
		}
		child := scope
		reason := ""
		if len(ts) == 0 {
			reason = "unsupported-rails-syntax"
		} else if scope.blocked {
			reason = "unresolved-parent"
		} else if railsWrapper(ts) && block && len(stack) == 1 && drawDepth == 0 {
			// The conventional Rails.application.routes.draw wrapper does not alter route scope.
		} else {
			verb := ts[0].text
			arg, opts, valid := railsArgs(ts[1:])
			switch verb {
			case "constraints", "defaults":
				reason = "rails-" + verb
			case "namespace", "scope":
				if guard := railsGuardOption(ts[1:]); guard != "" && block {
					// A constraints/defaults option narrows which requests the scope matches (a
					// format constraint may exclude HTML), so its screens stay unknown at this
					// line, as for the block form, and the rest of the file is read (AMAP-V0-366).
					reason = "rails-" + guard
					break
				}
				if !valid || !block || !railsOptions(opts, "path", "as", "shallow") {
					reason = "non-literal-rails-scope"
					break
				}
				segment := arg
				name := ""
				if verb == "namespace" {
					name = arg
				}
				if v, has := opts["path"]; has {
					segment = v.str
					valid = valid && v.kind == "string"
				}
				if v, has := opts["as"]; has {
					name = v.str
					valid = valid && v.kind == "string"
				}
				if !valid || verb == "namespace" && arg == "" || !literalRoutePath(segment) {
					reason = "non-literal-rails-scope"
					break
				}
				child.prefix = routeJoin(scope.prefix, segment)
				child.names = scope.names
				if name != "" {
					child.names += name + "_"
				}
				child.shallowPath, child.shallowNames = child.prefix, child.names
				if v, has := opts["shallow"]; has {
					if v.kind != "bool" {
						reason = "non-literal-rails-scope"
					} else {
						child.shallow = v.str == "true"
					}
				}
			case "resources", "resource":
				if !valid || arg == "" || !railsOptions(opts, "path", "as", "shallow") {
					reason = "non-literal-rails-resource"
					break
				}
				plural := arg
				if v, has := opts["as"]; has {
					if v.kind != "string" {
						reason = "non-literal-rails-resource"
						break
					}
					plural = v.str
				}
				if !routeLiteralName.MatchString(plural) {
					reason = "non-literal-name"
					break
				}
				singular := plural
				if verb == "resources" {
					singular = railsSingular(plural)
					if singular == "" {
						reason = "unsupported-rails-inflection"
						break
					}
				}
				segment := arg
				if v, has := opts["path"]; has {
					if v.kind != "string" {
						reason = "non-literal-rails-resource"
						break
					}
					segment = v.str
				}
				if !literalRoutePath(segment) {
					reason = "unsupported-route-pattern"
					break
				}
				shallow := scope.shallow
				if v, has := opts["shallow"]; has {
					if v.kind != "bool" {
						reason = "non-literal-rails-resource"
						break
					}
					shallow = v.str == "true"
				}
				collection := routeJoin(scope.prefix, segment)
				member := collection
				memberName := scope.names + singular
				if verb == "resources" {
					if shallow {
						member = routeJoin(scope.shallowPath, segment)
						memberName = scope.shallowNames + singular
					}
					member = routeJoin(member, ":id")
					p.add(e, data, line, scope.names+plural, collection, scope)
				}
				p.add(e, data, line, "new_"+scope.names+singular, routeJoin(collection, "new"), scope)
				p.add(e, data, line, memberName, member, scope)
				p.add(e, data, line, "edit_"+memberName, routeJoin(member, "edit"), scope)
				child.memberPath, child.memberName = member, memberName
				child.collectionPath, child.collectionName = collection, scope.names+plural
				child.prefix = collection
				if verb == "resources" {
					child.prefix = routeJoin(strings.TrimSuffix(member, "/:id"), ":"+singular+"_id")
				}
				child.names = memberName + "_"
				child.shallow = shallow
			case "member", "collection":
				if !valid || arg != "" || len(opts) > 0 || !block || scope.memberPath == "" {
					reason = "unsupported-rails-scope"
					break
				}
				if verb == "member" {
					child.prefix, child.names = scope.memberPath, scope.memberName
				} else {
					child.prefix, child.names = scope.collectionPath, scope.collectionName
				}
				// Special suffix for custom actions: preview_post, search_posts.
				child.names = "@" + child.names
			case "get", "post", "root", "mount":
				if !valid || block || !railsOptions(opts, "to", "as", "at") {
					reason = "non-literal-rails-route"
					break
				}
				if v, has := opts["to"]; has && v.kind != "string" {
					reason = "non-literal-rails-route"
					break
				}
				segment := arg
				name := ""
				if verb == "root" {
					segment = ""
					name = "root"
				}
				if verb == "mount" {
					v, has := opts["at"]
					if !has || v.kind != "string" {
						reason = "non-literal-rails-route"
						break
					}
					segment = v.str
				}
				if v, has := opts["as"]; has {
					if v.kind != "string" {
						reason = "non-literal-name"
						break
					}
					name = v.str
				}
				if name == "" && arg != "" && routeLiteralName.MatchString(arg) {
					name = arg
				}
				if name == "" {
					reason = "non-literal-name"
					break
				}
				if !literalRoutePath(segment) {
					reason = "unsupported-route-pattern"
					break
				}
				if strings.HasPrefix(scope.names, "@") {
					name = name + "_" + scope.names[1:]
				} else {
					name = scope.names + name
				}
				p.add(e, data, line, name, routeJoin(scope.prefix, segment), scope)
			case "draw":
				if !valid || arg == "" || len(opts) > 0 || block {
					reason = "non-literal-draw"
				} else if drawDepth > 0 {
					reason = "draw-depth-exceeded"
				} else if p.draw != nil {
					p.draw(arg, scope, line)
				} else {
					reason = "draw-unavailable"
				}
			default:
				reason = "unsupported-rails-statement"
			}
		}
		if reason == "unsupported-rails-statement" {
			unsafe = true
		}
		if reason != "" {
			p.unknowns = append(p.unknowns, routerUnknown(e, line, reason))
			child.blocked = true
		}
		if block {
			if len(stack) >= maxRouteDepth {
				malformed = true
				break
			}
			child.anchors = append(append([]Anchor{}, scope.anchors...), p.anchors[e.path])
			stack = append(stack, child)
		}
	}
	if len(stack) != 1 || malformed || unsafe {
		p.states = p.states[:startStates]
		p.unknowns = append(p.unknowns, routerUnknown(e, 1, "unsupported-rails-syntax"))
	}
}

// railsGuardOption returns "constraints" or "defaults" when an option of that name appears at the
// top level of a statement's arguments, else "".
func railsGuardOption(ts []token) string {
	depth := 0
	for j, t := range ts {
		switch {
		case isPunct(t, "(") || isPunct(t, "[") || isPunct(t, "{"):
			depth++
		case isPunct(t, ")") || isPunct(t, "]") || isPunct(t, "}"):
			depth--
		case depth <= 1 && t.kind == tokIdent && (t.text == "constraints" || t.text == "defaults") && next(ts, j+1, ":") && (j == 0 || next(ts, j-1, ",", "(")):
			return t.text
		}
	}
	return ""
}

func railsWrapper(ts []token) bool {
	var b strings.Builder
	for _, t := range ts {
		b.WriteString(t.text)
	}
	return b.String() == "Rails.application.routes.draw"
}

func railsSingular(s string) string {
	if !routeLiteralName.MatchString(s) || !strings.HasSuffix(s, "s") || strings.HasSuffix(s, "ss") || strings.HasSuffix(s, "ies") || strings.HasSuffix(s, "ses") || strings.HasSuffix(s, "xes") || strings.HasSuffix(s, "zes") || strings.HasSuffix(s, "ches") || strings.HasSuffix(s, "shes") || strings.HasSuffix(s, "ves") || strings.HasSuffix(s, "us") || s == "news" {
		return ""
	}
	return strings.TrimSuffix(s, "s")
}

func railsOptions(opts map[string]jsValue, allowed ...string) bool {
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

func railsArgs(ts []token) (string, map[string]jsValue, bool) {
	opts := map[string]jsValue{}
	arg := ""
	if len(ts) > 1 && next(ts, 0, "(") && next(ts, len(ts)-1, ")") {
		ts = ts[1 : len(ts)-1]
	}
	i := 0
	value := func() (jsValue, bool) {
		if i >= len(ts) {
			return jsValue{}, false
		}
		t := ts[i]
		i++
		if literal(t) {
			return jsValue{kind: "string", str: t.text}, true
		}
		if isPunct(t, ":") && i < len(ts) && ts[i].kind == tokIdent {
			v := jsValue{kind: "string", str: ts[i].text}
			i++
			return v, true
		}
		if t.kind == tokIdent && (t.text == "true" || t.text == "false") {
			return jsValue{kind: "bool", str: t.text}, true
		}
		return jsValue{}, false
	}
	for i < len(ts) {
		if i+1 < len(ts) && ts[i].kind == tokIdent && next(ts, i+1, ":") {
			k := ts[i].text
			i += 2
			v, ok := value()
			if !ok {
				return arg, opts, false
			}
			if _, dup := opts[k]; dup {
				return arg, opts, false
			}
			opts[k] = v
		} else {
			if arg != "" {
				return arg, opts, false
			}
			v, ok := value()
			if !ok || v.kind != "string" {
				return arg, opts, false
			}
			arg = v.str
		}
		if i < len(ts) {
			if !next(ts, i, ",") {
				return arg, opts, false
			}
			i++
		}
	}
	return arg, opts, true
}

func (p *railsParser) add(e blobEntry, data []byte, line int, name, url string, scope railsScope) {
	if !routeLiteralName.MatchString(name) || !literalRoutePath(url) || strings.HasSuffix(url, "*") {
		p.unknowns = append(p.unknowns, routerUnknown(e, line, "unsupported-route-pattern"))
		return
	}
	if len(p.states) > maxStates {
		return
	}
	// Helpers shared by HTTP verbs denote one screen only for an identical path.
	key := name + "\x00" + url
	if index, ok := p.helpers[key]; ok && index < len(p.states) && p.states[index].name == name && p.states[index].url == url {
		p.states[index].nameFrom = append(p.states[index].nameFrom, p.anchors[e.path])
		return
	}
	p.helpers[key] = len(p.states)
	p.states = append(p.states, rawState{splatNonEmpty: strings.Contains(url, "*"), name: name, url: url, urlSet: true, parentSet: true, anchor: p.anchors[e.path], nameFrom: append([]Anchor{}, scope.anchors...)})
}
