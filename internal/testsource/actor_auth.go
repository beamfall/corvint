package testsource

import (
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
)

// ActorAuthSchema names the actor-authorization report (TSA-V0-037).
const ActorAuthSchema = "corvint-tests-actor-authorization/0"

// Request statuses and the finding code (TSA-V0-036..038).
const (
	ActorUnauthorized     = "UNAUTHORIZED"
	ActorAuthorized       = "AUTHORIZED"
	ActorExempt           = "EXEMPT"
	ActorUnknown          = "UNKNOWN"
	ActorUnauthorizedCode = "ACTOR-UNAUTHORIZED"
)

// Default committed inputs for the Rails and CanCanCan-style slice (TSA-V0-034, TSA-V0-035).
const (
	DefaultRoutesPath  = "config/routes.rb"
	DefaultAbilityPath = "app/models/ability.rb"
	controllersRoot    = "app/controllers/"
)

// ActorAuthFrameworks is the closed set of input forms the check reads (TSA-V0-033..035).
var ActorAuthFrameworks = []string{
	"js-ts-fixture-factory-role-literal",
	"js-ts-test-login-and-request-literal",
	"rails-routes-literal",
	"rails-controller-authorize-literal",
	"cancancan-style-role-rule-sets",
}

// FixtureActor is one user a fixture seeds and returns under Key (TSA-V0-033).
type FixtureActor struct {
	Fixture string `json:"fixture"`
	Key     string `json:"key"`
	Role    string `json:"role,omitempty"`
	// RoleFrom is "literal" for a `role:` value in the factory call and "factory-default" for a
	// literal default in the factory's parameters.
	RoleFrom string `json:"roleFrom,omitempty"`
	Path     string `json:"path"`
	Line     int    `json:"line"`
	Reason   string `json:"reason,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

// RouteRef is the route a request reaches.
type RouteRef struct {
	Controller string `json:"controller"`
	Action     string `json:"action"`
	Path       string `json:"path"`
	Line       int    `json:"line"`
}

// AbilityCheck is the controller action's literal `authorize!` call.
type AbilityCheck struct {
	Action  string `json:"action"`
	Subject string `json:"subject"`
	Path    string `json:"path"`
	Line    int    `json:"line"`
}

// GrantRef is a rule that grants the ability, and the rule set that declares it.
type GrantRef struct {
	Role string `json:"role"`
	Path string `json:"path"`
	Line int    `json:"line"`
}

// ActorRequest is one request a test sends, with every resolved link and its status (TSA-V0-036).
type ActorRequest struct {
	Path         string         `json:"path"`
	Line         int            `json:"line"`
	Test         string         `json:"test"`
	Method       string         `json:"method"`
	URL          string         `json:"url"`
	Status       string         `json:"status"`
	Code         string         `json:"code,omitempty"`
	Reason       string         `json:"reason,omitempty"`
	Detail       string         `json:"detail,omitempty"`
	Actor        *FixtureActor  `json:"actor,omitempty"`
	Route        *RouteRef      `json:"route,omitempty"`
	Check        *AbilityCheck  `json:"check,omitempty"`
	GrantedBy    []GrantRef     `json:"grantedBy"`
	LowestRoles  []string       `json:"lowestRoles"`
	Alternatives []FixtureActor `json:"alternatives"`
	// segments is the request path split at `/`; "*" marks a segment that is one substitution.
	segments []string
	// pathUnknown marks a path that is not a literal, so no route is matched for it.
	pathUnknown bool
}

// ActorAuthReport is the deterministic actor-authorization result (TSA-V0-037).
type ActorAuthReport struct {
	Schema     string         `json:"schema"`
	Revision   string         `json:"revision"`
	Verdict    string         `json:"verdict"`
	Frameworks []string       `json:"frameworks"`
	Routes     string         `json:"routes"`
	Ability    string         `json:"ability"`
	Requests   []ActorRequest `json:"requests"`
	Actors     []FixtureActor `json:"actors"`
	Uncertain  []string       `json:"uncertain"`
}

// ActorAuthOptions select the revision and the Rails inputs; empty paths use the defaults.
type ActorAuthOptions struct {
	Revision, Routes, Ability string
}

// ActorAuthorization reads the committed tests, fixtures, routes, controllers and Ability class at
// a revision and reports each test request whose actor's role does not hold the ability the
// request needs (TSA-V0-033..039). It reads committed objects only.
func ActorAuthorization(r Repo, options ActorAuthOptions) (ActorAuthReport, error) {
	revision, err := r.Resolve(options.Revision)
	if err != nil {
		return ActorAuthReport{}, err
	}
	routes, ability := defaultPath(options.Routes, DefaultRoutesPath), defaultPath(options.Ability, DefaultAbilityPath)
	keep := func(name string) bool {
		if name == routes || name == ability {
			return true
		}
		if strings.HasPrefix(name, controllersRoot) {
			return strings.HasSuffix(name, "_controller.rb")
		}
		return webExtensions[path.Ext(name)] && !strings.Contains("/"+name, "/node_modules/")
	}
	texts, _, err := r.readFiles(revision, keep)
	if err != nil {
		return ActorAuthReport{}, err
	}
	report := AnalyzeActorAuthorization(texts, routes, ability)
	report.Revision = revision
	return report, nil
}

func defaultPath(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func locate(file string, line int) string { return file + ":" + strconv.Itoa(line) }

// fixtureInfo is one exported fixture builder: its actors by returned key, or why none resolve.
type fixtureInfo struct {
	path   string
	line   int
	actors map[string]FixtureActor
	reason string
	detail string
}

// seededUser is one user-factory call inside a fixture body.
type seededUser struct {
	variable string
	idents   map[string]bool
	actor    FixtureActor
}

// AnalyzeActorAuthorization runs the check over in-memory files keyed by path. It is the pure core
// of ActorAuthorization and is deterministic for the same input.
func AnalyzeActorAuthorization(files map[string]string, routesPath, abilityPath string) ActorAuthReport {
	report := ActorAuthReport{Schema: ActorAuthSchema, Frameworks: append([]string(nil), ActorAuthFrameworks...),
		Routes: routesPath, Ability: abilityPath, Requests: []ActorRequest{}, Actors: []FixtureActor{}, Uncertain: []string{}}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	sources := map[string]*source{}
	for _, name := range names {
		if webExtensions[path.Ext(name)] {
			s := parseSource(name, files[name])
			sources[name] = s
			if s.uncertain {
				report.Uncertain = append(report.Uncertain, name)
			}
		}
	}
	fixtures := map[string]*fixtureInfo{}
	roles := factoryDefaultRoles(sources, names)
	for _, name := range names {
		if s := sources[name]; s != nil && !IsTestFile(name) {
			s.collectFixtures(fixtures, roles)
		}
	}
	for _, name := range sortedKeys(fixtures) {
		for _, key := range sortedKeys(fixtures[name].actors) {
			report.Actors = append(report.Actors, fixtures[name].actors[key])
		}
	}
	routesText, haveRoutes := files[routesPath]
	abilityText, haveAbility := files[abilityPath]
	routes := parseRailsRoutes(routesPath, routesText)
	ability := parseAbility(abilityPath, abilityText)
	for _, name := range names {
		if s := sources[name]; s != nil && IsTestFile(name) {
			for _, request := range s.actorRequests(fixtures) {
				request.resolve(files, routes, haveRoutes, ability, haveAbility, report.Actors)
				report.Requests = append(report.Requests, request)
			}
		}
	}
	report.Verdict = "CLEAN"
	for _, r := range report.Requests {
		if r.Status == ActorUnauthorized {
			report.Verdict = "ACTOR_UNAUTHORIZED"
			break
		}
		if r.Status == ActorUnknown {
			report.Verdict = ActorUnknown
		}
	}
	return report
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// exportedFunctions finds `export [async] function NAME(...) {` and
// `export const NAME = [async] (...) => {` / `= [async] function (...) {` declarations, returning
// each name with its body brace and line.
func (s *source) exportedFunctions() []struct {
	name       string
	open, line int
} {
	out := []struct {
		name       string
		open, line int
	}{}
	for i := range s.tokens {
		if !s.ident(i, "export") {
			continue
		}
		j := i + 1
		name := ""
		switch {
		case s.ident(j, "async") && s.ident(j+1, "function") && s.ident(j+2, ""):
			name, j = s.tokens[j+2].Text, j+3
		case s.ident(j, "function") && s.ident(j+1, ""):
			name, j = s.tokens[j+1].Text, j+2
		case (s.ident(j, "const") || s.ident(j, "let")) && s.ident(j+1, "") && s.punct(j+2, "="):
			name, j = s.tokens[j+1].Text, j+3
			if s.ident(j, "async") {
				j++
			}
			if s.ident(j, "function") {
				j++
			}
		default:
			continue
		}
		if !s.punct(j, "(") || s.match[j] < 0 {
			continue
		}
		j = s.match[j] + 1
		if s.arrow(j) {
			j += 2
		}
		if s.punct(j, "{") && s.match[j] > j {
			out = append(out, struct {
				name       string
				open, line int
			}{name, j, s.tokens[i].Line})
		}
	}
	return out
}

// refs collects the identifiers in [start, end) that name a value: not a property after `.` and
// not an object key before `:`.
func (s *source) refs(start, end int) map[string]bool {
	out := map[string]bool{}
	for i := start; i < end; i++ {
		if s.ident(i, "") && !s.memberAccess(i) && !s.punct(i+1, ":") {
			out[s.tokens[i].Text] = true
		}
	}
	return out
}

// collectFixtures records every exported fixture builder in s whose body seeds users
// (TSA-V0-033).
func (s *source) collectFixtures(fixtures map[string]*fixtureInfo, roles map[string]*factoryRole) {
	for _, f := range s.exportedFunctions() {
		body := [2]int{f.open + 1, s.match[f.open]}
		users := s.seededUsers(body, f.name, roles)
		if len(users) == 0 {
			continue
		}
		info := &fixtureInfo{path: s.path, line: f.line, actors: map[string]FixtureActor{}}
		if previous, ok := fixtures[f.name]; ok {
			previous.actors, previous.reason = map[string]FixtureActor{}, "fixture-ambiguous"
			previous.detail = "fixture " + f.name + " is exported by " + previous.path + " and " + s.path
			continue
		}
		fixtures[f.name] = info
		returns := []int{}
		for i := body[0]; i < body[1]; i++ {
			if s.ident(i, "return") && s.punct(i+1, "{") && s.match[i+1] > i {
				returns = append(returns, i+1)
			}
			if s.match[i] > i {
				i = s.match[i]
			}
		}
		if len(returns) != 1 {
			info.reason = "fixture-return-not-literal"
			info.detail = fmt.Sprintf("%s fixture %s has %d top-level object returns", locate(s.path, f.line), f.name, len(returns))
			continue
		}
		open := returns[0]
		for _, prop := range s.splitArgs(open, s.match[open]) {
			if !s.ident(prop[0], "") || !(prop[1]-prop[0] == 1 || s.punct(prop[0]+1, ":")) {
				continue
			}
			key := s.tokens[prop[0]].Text
			value := prop
			if prop[1]-prop[0] > 1 {
				value = [2]int{prop[0] + 2, prop[1]}
			}
			refs := s.refs(value[0], value[1])
			if prop[1]-prop[0] == 1 {
				refs = map[string]bool{key: true}
			}
			var matched []seededUser
			for _, u := range users {
				if refs[u.variable] {
					matched = append(matched, u)
				}
			}
			if len(matched) == 0 {
				for _, u := range users {
					for ref := range refs {
						if u.idents[ref] {
							matched = append(matched, u)
							break
						}
					}
				}
			}
			switch len(matched) {
			case 0:
				continue
			case 1:
				actor := matched[0].actor
				actor.Fixture, actor.Key = f.name, key
				info.actors[key] = actor
			default:
				info.actors[key] = FixtureActor{Fixture: f.name, Key: key, Path: s.path, Line: s.tokens[prop[0]].Line,
					Reason: "actor-key-ambiguous", Detail: fmt.Sprintf("%s key %s refers to %d seeded users", locate(s.path, s.tokens[prop[0]].Line), key, len(matched))}
			}
		}
	}
}

// seededUsers finds the user-factory calls in a fixture body: a call whose callee chain names a
// user, or whose arguments carry a `role:` key (TSA-V0-033).
func (s *source) seededUsers(body [2]int, fixture string, roles map[string]*factoryRole) []seededUser {
	users := []seededUser{}
	for i := body[0]; i < body[1]; i++ {
		if !s.ident(i, "") || !s.punct(i+1, "(") || s.match[i+1] < 0 || s.ident(i-1, "function") {
			continue
		}
		open, close := i+1, s.match[i+1]
		roleKeys := []int{}
		for j := open + 1; j < close; j++ {
			if s.ident(j, "role") && s.punct(j+1, ":") && !s.memberAccess(j) {
				roleKeys = append(roleKeys, j)
			}
		}
		callee, start := []string{s.tokens[i].Text}, i
		for s.punct(start-1, ".") && s.ident(start-2, "") {
			start -= 2
			callee = append(callee, s.tokens[start].Text)
		}
		named := false
		for _, part := range callee {
			named = named || strings.Contains(strings.ToLower(part), "user")
		}
		if !named && len(roleKeys) == 0 {
			continue
		}
		u := seededUser{idents: s.refs(open+1, close), actor: FixtureActor{Fixture: fixture, Path: s.path, Line: s.tokens[i].Line}}
		k := start
		if s.ident(k-1, "await") {
			k--
		}
		if s.punct(k-1, "=") && s.ident(k-2, "") {
			u.variable = s.tokens[k-2].Text
		}
		switch {
		case len(roleKeys) > 1:
			u.actor.Reason, u.actor.Detail = "role-ambiguous", locate(s.path, s.tokens[i].Line)+" sets role more than once"
		case len(roleKeys) == 1:
			v := roleKeys[0] + 2
			if at := s.roleOverride(roleKeys[0], close); at >= 0 {
				u.actor.Reason, u.actor.Detail = "role-not-literal", locate(s.path, s.tokens[at].Line)+" a later spread or computed property can override role"
			} else if t := s.tokens[v]; (t.Kind == appmap.JSString || t.Kind == appmap.JSTemplate) && t.Literal && (s.punct(v+1, ",") || s.punct(v+1, "}")) {
				u.actor.Role, u.actor.RoleFrom = t.Text, "literal"
			} else {
				u.actor.Reason, u.actor.Detail = "role-not-literal", locate(s.path, s.tokens[v].Line)+" role is not a string literal"
			}
		case s.roleOverride(open, close) >= 0:
			at := s.roleOverride(open, close)
			u.actor.Reason, u.actor.Detail = "role-not-literal", locate(s.path, s.tokens[at].Line)+" a spread or computed property can set role"
		default:
			if r := roles[s.tokens[i].Text]; r != nil && r.found == 1 && !r.other {
				u.actor.Role, u.actor.RoleFrom, u.actor.Detail = r.role, "factory-default", r.at
			} else {
				u.actor.Reason, u.actor.Detail = "role-default-unresolved", locate(s.path, s.tokens[i].Line)+" "+s.tokens[i].Text+" has no role and no single literal default"
			}
		}
		users = append(users, u)
		i = close
	}
	return users
}

// roleOverride returns the first spread (`...x`) or computed property (`[k]: v`) after token from
// and before close, which could set role at run time, or -1 (TSA-V0-033).
func (s *source) roleOverride(from, close int) int {
	for j := from + 1; j < close; j++ {
		if s.punct(j, ".") && s.punct(j+1, ".") && s.punct(j+2, ".") {
			return j
		}
		if s.punct(j, "[") && s.match[j] > j && s.punct(s.match[j]+1, ":") && (s.punct(j-1, "{") || s.punct(j-1, ",")) {
			return j
		}
	}
	return -1
}

// factoryRole aggregates every definition of one factory name whose parameters give `role` a
// default: found counts single string-literal defaults and other records any other default.
type factoryRole struct {
	found    int
	role, at string
	other    bool
}

// factoryDefaultRoles indexes, in one pass over the sources, every factory definition whose
// parameters give `role` a default. A factory resolves only when it has exactly one default that is
// a single string literal ending at a `,`, `}` or `)` (`role = 'x'`, also inside a destructured
// parameter) and no other default (TSA-V0-033). Computing it once per analysis keeps each fixture's
// lookup independent of the number of source files (V1-1777).
func factoryDefaultRoles(sources map[string]*source, names []string) map[string]*factoryRole {
	out := map[string]*factoryRole{}
	for _, name := range names {
		s := sources[name]
		if s == nil {
			continue
		}
		for i := range s.tokens {
			if !s.ident(i, "") || s.memberAccess(i) {
				continue
			}
			open := -1
			switch {
			case s.ident(i-1, "function") && s.punct(i+1, "("):
				open = i + 1
			case s.punct(i+1, "=") && !s.tokens[i+1].Glued:
				j := i + 2
				if s.ident(j, "async") {
					j++
				}
				if s.ident(j, "function") {
					j++
				}
				if s.punct(j, "(") && s.match[j] > j && (s.arrow(s.match[j]+1) || s.ident(j-1, "function")) {
					open = j
				}
			}
			if open < 0 || s.match[open] < 0 {
				continue
			}
			for j := open + 1; j < s.match[open]; j++ {
				if !s.ident(j, "role") || !s.punct(j+1, "=") || s.punct(j+2, "=") || s.punct(j+2, ">") {
					continue
				}
				r := out[s.tokens[i].Text]
				if r == nil {
					r = &factoryRole{}
					out[s.tokens[i].Text] = r
				}
				t := s.tokens[min(j+2, len(s.tokens)-1)]
				if (t.Kind == appmap.JSString || t.Kind == appmap.JSTemplate) && t.Literal &&
					(s.punct(j+3, ",") || s.punct(j+3, "}") || s.punct(j+3, ")")) {
					r.found++
					r.role, r.at = t.Text, locate(s.path, t.Line)
				} else {
					r.other = true
				}
			}
		}
	}
	return out
}

// substitution is the lexer's marker for one template substitution in a template's Text.
const substitution = "\x00"

var requestMethods = map[string]string{"get": "GET", "post": "POST", "put": "PUT", "patch": "PATCH", "delete": "DELETE"}

// login is one login call and the fixture member it names.
type login struct {
	at      int
	fixture string
	key     string
	reason  string
	detail  string
}

// actorRequests finds each literal request a test sends and binds the actor it is sent as: the
// latest login call before it in the same test or outside every test (TSA-V0-033, TSA-V0-036).
func (s *source) actorRequests(fixtures map[string]*fixtureInfo) []ActorRequest {
	tests := []decl{}
	for _, d := range s.decls {
		if d.kind == "test" && d.body[1] > d.body[0] {
			tests = append(tests, d)
		}
	}
	inScope := func(i, at int, d decl) bool {
		if within(i, d.body) {
			return true
		}
		for _, other := range tests {
			if within(i, other.body) {
				return false
			}
		}
		return i < at
	}
	type binding struct {
		at       int
		variable string
		fixture  string
	}
	bindings := []binding{}
	logins := []login{}
	for i := range s.tokens {
		if (s.ident(i, "const") || s.ident(i, "let") || s.ident(i, "var")) && s.ident(i+1, "") && s.punct(i+2, "=") {
			j := i + 3
			if s.ident(j, "await") {
				j++
			}
			if s.ident(j, "") && s.punct(j+1, "(") && fixtures[s.tokens[j].Text] != nil {
				bindings = append(bindings, binding{at: i, variable: s.tokens[i+1].Text, fixture: s.tokens[j].Text})
			}
		}
		lower := strings.ToLower(s.tokens[i].Text)
		if s.ident(i, "") && s.punct(i+1, "(") && s.match[i+1] > i && !s.ident(i-1, "function") &&
			(strings.Contains(lower, "login") || strings.Contains(lower, "signin")) {
			l := login{at: i, reason: "actor-not-literal", detail: locate(s.path, s.tokens[i].Line) + " login argument is not a fixture member"}
			for j := i + 2; j < s.match[i+1]; j++ {
				if !s.ident(j, "") || s.memberAccess(j) || !s.punct(j+1, ".") || !s.ident(j+2, "") {
					continue
				}
				for k := len(bindings) - 1; k >= 0; k-- {
					if bindings[k].variable == s.tokens[j].Text {
						l = login{at: i, fixture: bindings[k].fixture, key: s.tokens[j+2].Text}
						break
					}
				}
				if l.fixture != "" {
					break
				}
			}
			logins = append(logins, l)
		}
	}
	out := []ActorRequest{}
	for _, d := range tests {
		exempt := s.assertsRejection(d)
		for i := d.body[0]; i < d.body[1]; i++ {
			method, ok := requestMethods[s.tokens[i].Text]
			if !ok || !s.ident(i, "") || !s.memberAccess(i) || !s.punct(i+1, "(") || s.match[i+1] < 0 {
				continue
			}
			args := s.splitArgs(i+1, s.match[i+1])
			literal := len(args) > 0 && s.loneString(args[0])
			if literal {
				t := s.tokens[args[0][0]]
				literal = strings.HasPrefix(t.Text, "/") && !(t.Kind == appmap.JSString && !t.Literal)
			}
			if !literal && !s.requestReceiver(i) {
				continue
			}
			request := ActorRequest{Path: s.path, Line: s.tokens[i].Line, Test: d.title, Method: method,
				GrantedBy: []GrantRef{}, LowestRoles: []string{}, Alternatives: []FixtureActor{}}
			if exempt != "" {
				request.Status, request.Reason, request.Detail = ActorExempt, "asserts-rejection", exempt
			}
			if literal {
				t := s.tokens[args[0][0]]
				request.Line, request.URL = t.Line, t.Text
				if t.Kind == appmap.JSTemplate && !t.Literal {
					request.URL = strings.Trim(t.Source, "`")
				}
				pathOnly, _, _ := strings.Cut(t.Text, "?")
				for _, segment := range strings.Split(strings.Trim(pathOnly, "/"), "/") {
					switch {
					case segment == "":
					case segment == substitution:
						request.segments = append(request.segments, "*")
					case strings.Contains(segment, substitution):
						request.pathUnknown = true
						request.setUnknown("path-not-literal", locate(s.path, t.Line)+" a path segment mixes text and a substitution")
					default:
						request.segments = append(request.segments, segment)
					}
				}
			} else {
				if len(args) > 0 {
					request.Line, request.URL = s.tokens[args[0][0]].Line, s.render(args[0][0], args[0][1])
				}
				request.pathUnknown = true
				request.setUnknown("path-not-literal", locate(s.path, request.Line)+" the request path is not a literal beginning with /")
			}
			chosen := -1
			for k, l := range logins {
				if l.at < i && inScope(l.at, d.at, d) {
					chosen = k
				}
			}
			switch {
			case chosen < 0:
				request.setUnknown("actor-unbound", locate(s.path, request.Line)+" no login call precedes the request")
			case logins[chosen].fixture == "":
				request.setUnknown(logins[chosen].reason, logins[chosen].detail)
			default:
				l := logins[chosen]
				info := fixtures[l.fixture]
				actor, ok := info.actors[l.key]
				switch {
				case info.reason != "":
					request.setUnknown(info.reason, info.detail)
				case !ok:
					request.setUnknown("actor-key-not-user", fmt.Sprintf("fixture %s returns no seeded user under %s", l.fixture, l.key))
				default:
					request.Actor = &actor
					if actor.Reason != "" {
						request.setUnknown(actor.Reason, actor.Detail)
					}
				}
			}
			out = append(out, request)
			i = s.match[i+1]
		}
	}
	return out
}

// requestReceiver reports a method call at token i on a receiver named like a request context
// (`request.put(`, `page.request.get(`), whose path argument must then resolve (TSA-V0-034).
func (s *source) requestReceiver(i int) bool {
	return s.memberAccess(i) && s.ident(i-2, "") && strings.Contains(strings.ToLower(s.tokens[i-2].Text), "request")
}

// setUnknown records the first unresolved link; an exempt request keeps its status.
func (r *ActorRequest) setUnknown(reason, detail string) {
	if r.Status == "" {
		r.Status, r.Reason, r.Detail = ActorUnknown, reason, detail
	}
}

// rejectionStatuses are the statuses whose assertion exempts a test (TSA-V0-039).
var rejectionStatuses = map[string]bool{"401": true, "403": true, "422": true}

// assertsRejection reports where a test asserts a 401, 403 or 422 status with an equality matcher
// that is not negated, on a subject that is a response status (`x.status()`, `x.status`,
// `x.statusCode`) or, for toHaveStatus, the response itself.
func (s *source) assertsRejection(d decl) string {
	for i := d.body[0]; i < d.body[1]; i++ {
		c, ok := s.readExpect(i)
		if !ok {
			continue
		}
		negated := false
		for j := c.subject[1]; j < c.margs[0]; j++ {
			negated = negated || s.ident(j, "not") && s.memberAccess(j)
		}
		if negated || c.matcher != "toHaveStatus" && !s.statusSubject(c.subject) {
			continue
		}
		switch c.matcher {
		case "toBe", "toEqual", "toStrictEqual", "toHaveStatus":
			if c.margs[1]-c.margs[0] == 1 && s.tokens[c.margs[0]].Kind == appmap.JSNumber && rejectionStatuses[s.tokens[c.margs[0]].Text] {
				return locate(s.path, s.tokens[i].Line) + " asserts status " + s.tokens[c.margs[0]].Text
			}
		}
	}
	return ""
}

// statusSubject reports an expect subject ending in `.status()`, `.status` or `.statusCode`.
func (s *source) statusSubject(subject [2]int) bool {
	end := subject[1]
	if end-subject[0] > 2 && s.punct(end-1, ")") && s.punct(end-2, "(") {
		end -= 2
	}
	return end > subject[0] && s.memberAccess(end-1) && (s.ident(end-1, "status") || s.ident(end-1, "statusCode"))
}

// resolve follows the request to its route, controller check and the actor role's grant, and sets
// the status (TSA-V0-034..038).
func (r *ActorRequest) resolve(files map[string]string, routes railsRoutes, haveRoutes bool, ability abilityModel, haveAbility bool, actors []FixtureActor) {
	if r.pathUnknown {
		return
	}
	if !haveRoutes {
		r.setUnknown("routes-unavailable", routes.path+" is not committed")
		return
	}
	route, ok := routes.match(r.Method, r.segments)
	if !ok {
		if len(routes.unparsed) > 0 {
			r.setUnknown("route-unparsed", fmt.Sprintf("no literal route matches; %s has unparsed statements at line %d", routes.path, routes.unparsed[0]))
		} else {
			r.setUnknown("route-not-found", "no route in "+routes.path+" matches "+r.Method+" "+r.URL)
		}
		return
	}
	r.Route = &RouteRef{Controller: route.controller, Action: route.action, Path: routes.path, Line: route.line}
	controller := controllersRoot + route.controller + "_controller.rb"
	text, ok := files[controller]
	if !ok {
		r.setUnknown("controller-unavailable", controller+" is not committed")
		return
	}
	check, reason, detail := controllerCheck(controller, text, route.action)
	if reason != "" {
		r.setUnknown(reason, detail)
		return
	}
	r.Check = &check
	if !haveAbility {
		r.setUnknown("ability-unavailable", ability.path+" is not committed")
		return
	}
	if r.Actor == nil {
		return
	}
	if r.Actor.Role == "" {
		r.setUnknown("role-undefined", locate(r.Actor.Path, r.Actor.Line)+" seeds an empty role")
		return
	}
	g := ability.evaluate(r.Actor.Role, check.Action, check.Subject)
	switch g.state {
	case "unknown":
		r.setUnknown(g.reason, g.detail)
	case "granted":
		if r.Status == "" {
			r.Status = ActorAuthorized
		}
	case "denied":
		r.lowest(ability, check, actors)
		if r.Status == "" {
			r.Status, r.Code = ActorUnauthorized, ActorUnauthorizedCode
			r.Detail = fmt.Sprintf("role %s does not hold %s on %s", r.Actor.Role, check.Action, check.Subject)
		}
	}
}

// lowest fills the granting rules, the lowest sufficient roles and the alternative fixture actors
// (TSA-V0-036). A lowest role is a granted role that inherits no other granted role.
func (r *ActorRequest) lowest(ability abilityModel, check AbilityCheck, actors []FixtureActor) {
	granted := map[string]GrantRef{}
	for _, role := range sortedKeys(ability.roles) {
		if g := ability.evaluate(role, check.Action, check.Subject); g.state == "granted" {
			granted[role] = g.rule
		}
	}
	seen := map[GrantRef]bool{}
	for _, role := range sortedKeys(granted) {
		order, _, _ := ability.closure(role)
		lowest := true
		for _, inherited := range order[1:] {
			if _, ok := granted[inherited]; ok {
				lowest = false
			}
		}
		if lowest {
			r.LowestRoles = append(r.LowestRoles, role)
			if !seen[granted[role]] {
				seen[granted[role]] = true
				r.GrantedBy = append(r.GrantedBy, granted[role])
			}
		}
	}
	for _, a := range actors {
		if _, ok := granted[a.Role]; ok && a.Reason == "" {
			r.Alternatives = append(r.Alternatives, a)
		}
	}
}
