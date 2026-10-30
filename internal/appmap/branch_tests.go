package appmap

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

// Branch query (AMAP-V0-110..116): for cited Ruby lines, each branch's reaching tests grouped by
// boundary and, where a boundary is not reached, the guard's model inputs joined to the setup
// writers that can satisfy them.
const (
	BranchesSchema = "application-map-branches/0"

	// BranchesLimits is printed in every report (AMAP-V0-116).
	BranchesLimits = "static naming evidence only: a test reaches a branch when it names its method or routed path, not when it executes it; " +
		"setup candidates are a write-name join of model attributes and do not prove the guard becomes true"

	maxCites        = 16
	maxCiteLines    = 400
	maxGuardBytes   = 200
	maxBranchTests  = 5
	maxInputWriters = 8
)

var boundaries = []string{"unit", "request", "e2e"}

type branchCite struct {
	path       string
	start, end int
}

func parseCite(s string) (branchCite, bool) {
	i := strings.LastIndexByte(s, ':')
	if i <= 0 {
		return branchCite{}, false
	}
	c := branchCite{path: s[:i]}
	a, b, ranged := strings.Cut(s[i+1:], "-")
	var err1, err2 error
	c.start, err1 = strconv.Atoi(a)
	c.end = c.start
	if ranged {
		c.end, err2 = strconv.Atoi(b)
	}
	if err1 != nil || err2 != nil || c.start < 1 || c.end < c.start || c.end-c.start+1 > maxCiteLines ||
		!safeRelative(c.path) || !strings.HasSuffix(c.path, ".rb") {
		return branchCite{}, false
	}
	return c, true
}

// rubyBranch is one branch read from cited lines (AMAP-V0-111).
type rubyBranch struct {
	line      int
	kind      string
	guard     string // original text
	masked    string // masked guard, for input resolution
	constant  string
	method    string
	singleton bool
	model     string // the model whose file holds the branch, if any
}

var branchStart = map[string]bool{"if": true, "unless": true, "elsif": true, "when": true}

// readBranches finds the branches on lines start..end of a parsed file.
func readBranches(f *rubyFile, raw []string, start, end int, modelFile string) []rubyBranch {
	out := []rubyBranch{}
	for ln := start; ln <= end && ln <= len(f.code); ln++ {
		code := f.code[ln-1]
		original := ""
		if ln-1 < len(raw) {
			original = raw[ln-1]
		}
		offset := 0
		for _, stmt := range strings.Split(code, ";") {
			base := offset
			offset += len(stmt) + 1
			lead := len(stmt) - len(strings.TrimLeft(stmt, " \t"))
			for wi, w := range rubyWordsOf(stmt) {
				if !w.keyword || !branchStart[w.text] {
					continue
				}
				modifier := wi > 0
				if modifier && (w.text == "elsif" || w.text == "when" || strings.ContainsRune("=(,[{|&!", rune(codeBefore(code, original, base+w.at)))) {
					continue // an expression-position `if` opens a block; elsif/when only start statements
				}
				if !modifier && w.at != lead {
					continue
				}
				from, to := base+w.at+len(w.text), base+len(stmt)
				guard := sliceCols(original, from, to)
				masked := strings.TrimSpace(stmt[w.at+len(w.text):])
				guard = strings.TrimSpace(guard)
				for _, suffix := range []string{" then", " do"} {
					if strings.HasSuffix(masked, suffix) {
						masked = strings.TrimSpace(strings.TrimSuffix(masked, suffix))
						guard = strings.TrimSpace(strings.TrimSuffix(guard, suffix))
					}
				}
				b := rubyBranch{line: ln, kind: w.text, guard: guard, masked: masked, model: modelFile}
				if modifier {
					b.kind = "modifier-" + w.text
				}
				d, s := f.enclosing(ln)
				if s != nil {
					b.constant = s.name
				}
				if d != nil {
					b.method, b.singleton = d.name, !d.instance
				}
				out = append(out, b)
				break
			}
		}
	}
	return out
}

func sliceCols(s string, from, to int) string {
	if from > len(s) {
		return ""
	}
	if to > len(s) {
		to = len(s)
	}
	return s[from:to]
}

// testRefs is the reaching evidence one test file offers.
type testRefs struct {
	path     string
	boundary string
	text     string
	requests []testCall // verb + literal path or helper
	visits   []testCall
}

type testCall struct {
	verb, path, helper string
	line               int
}

var (
	requestCall = regexp.MustCompile(`\b(get|post|patch|put|delete)\s*\(?\s*(?:["'` + "`" + `]([^"'` + "`" + `\s]+)["'` + "`" + `]|([a-z_]\w*)_(?:path|url)\b)`)
	visitCall   = regexp.MustCompile(`\b(visit|goto|waitForURL|toHaveURL)\s*\(?\s*(?:["'` + "`" + `]([^"'` + "`" + `\s]+)["'` + "`" + `]|([a-z_]\w*)_(?:path|url)\b)`)
)

func boundaryOf(p string) string {
	for _, seg := range strings.Split(p, "/") {
		switch seg {
		case "requests", "controllers", "integration", "api":
			return "request"
		case "system", "features", "e2e", "acceptance":
			return "e2e"
		}
	}
	return "unit"
}

func lineAt(text string, offset int) int { return strings.Count(text[:offset], "\n") + 1 }

// scanTestRefs reads request and visit calls; a call inside a comment or string is not one.
func scanTestRefs(p, boundary, text string) testRefs {
	t := testRefs{path: p, boundary: boundary, text: text}
	language := "web"
	if strings.HasSuffix(p, ".rb") {
		language = "ruby"
	}
	code := strings.Join(contextindex.FlowCodeLines(language, text), "\n")
	isCode := func(i int) bool { return len(code) == len(text) && code[i] == text[i] }
	for _, m := range requestCall.FindAllStringSubmatchIndex(text, -1) {
		if !isCode(m[0]) {
			continue
		}
		c := testCall{verb: strings.ToUpper(text[m[2]:m[3]]), line: lineAt(text, m[0])}
		if m[4] >= 0 {
			c.path = text[m[4]:m[5]]
		} else {
			c.helper = text[m[6]:m[7]]
		}
		t.requests = append(t.requests, c)
	}
	for _, m := range visitCall.FindAllStringSubmatchIndex(text, -1) {
		if !isCode(m[0]) {
			continue
		}
		c := testCall{verb: "GET", line: lineAt(text, m[0])}
		if m[4] >= 0 {
			c.path = text[m[4]:m[5]]
		} else {
			c.helper = text[m[6]:m[7]]
		}
		t.visits = append(t.visits, c)
	}
	return t
}

// pathMatches reports whether a literal test path matches a route template: an origin, query and
// fragment are dropped, `:param` segments and interpolated segments match any non-empty segment.
func pathMatches(template, literal string) bool {
	if i := strings.Index(literal, "://"); i >= 0 {
		rest := literal[i+3:]
		j := strings.IndexByte(rest, '/')
		if j < 0 {
			return template == "/"
		}
		literal = rest[j:]
	}
	if i := strings.IndexAny(literal, "?#"); i >= 0 && !strings.HasPrefix(literal[i:], "#{") {
		literal = literal[:i]
	}
	if !strings.HasPrefix(literal, "/") {
		return false
	}
	ts := strings.Split(strings.Trim(template, "/"), "/")
	ls := strings.Split(strings.Trim(literal, "/"), "/")
	if len(ts) != len(ls) {
		return false
	}
	for i := range ts {
		dynamic := strings.Contains(ls[i], "${") || strings.Contains(ls[i], "#{")
		switch {
		case strings.HasPrefix(ts[i], ":"):
			if ls[i] == "" {
				return false
			}
		case dynamic || ts[i] != ls[i]:
			return false
		}
	}
	return true
}

func (c testCall) matches(rt railsRoute) bool {
	if rt.verb != "ANY" && !containsField(rt.verb, c.verb) {
		return false
	}
	if c.helper != "" {
		return rt.name != "" && c.helper == rt.name
	}
	return pathMatches(rt.path, c.path)
}

func containsField(list, v string) bool {
	for _, s := range strings.Split(list, "|") {
		if s == v {
			return true
		}
	}
	return false
}

type reachRow struct {
	Test  string `json:"test"`
	Basis string `json:"basis"`
	Route string `json:"route,omitempty"`
}

type boundaryRow struct {
	Boundary string     `json:"boundary"`
	Status   string     `json:"status"`
	Note     string     `json:"note,omitempty"`
	Count    int        `json:"count"`
	Tests    []reachRow `json:"tests"`
}

type guardInput struct {
	Model     string        `json:"model"`
	Attribute string        `json:"attribute"`
	Expr      string        `json:"expression"`
	Writers   []setupWriter `json:"writers"`
	More      int           `json:"writers_omitted,omitempty"`
}

type unresolvedInput struct {
	Expr   string `json:"expression"`
	Reason string `json:"reason"`
}

type setupRow struct {
	Status     string            `json:"status"`
	Inputs     []guardInput      `json:"inputs"`
	Unresolved []unresolvedInput `json:"unresolved"`
}

type branchRow struct {
	At            string        `json:"at"`
	Kind          string        `json:"kind"`
	Guard         string        `json:"guard,omitempty"`
	GuardRedacted bool          `json:"guard_redacted,omitempty"`
	Constant      string        `json:"constant,omitempty"`
	Method        string        `json:"method,omitempty"`
	Boundaries    []boundaryRow `json:"boundaries"`
	Setup         *setupRow     `json:"setup,omitempty"`
}

type noBranchRow struct {
	Cite   string `json:"cite"`
	Reason string `json:"reason"`
}

var (
	guardChain      = regexp.MustCompile(`(@?[A-Za-z_]\w*[?!]?(?:\.[A-Za-z_]\w*[?!]?)*)(\s*[(\[])?`)
	guardPredicates = map[string]bool{"present?": true, "blank?": true, "nil?": true, "any?": true, "empty?": true, "zero?": true}
	guardKeywords   = map[string]bool{"and": true, "or": true, "not": true, "nil": true, "true": true, "false": true, "then": true, "defined?": true}
	requestRoots    = map[string]bool{"params": true, "session": true, "cookies": true, "request": true, "ENV": true}
)

// guardInputs resolves a guard's inputs to model attributes (AMAP-V0-113).
func (a *railsApp) guardInputs(b rubyBranch) ([][2]string, []unresolvedInput) {
	resolved, unresolved := [][2]string{}, []unresolvedInput{}
	seen := map[string]bool{}
	text := b.masked
	for _, m := range guardChain.FindAllStringSubmatchIndex(text, -1) {
		if m[0] > 0 && (text[m[0]-1] == ':' || text[m[0]-1] == '.' || rubyIdentByte(text[m[0]-1])) {
			continue // a symbol, a continuation or the tail of a number
		}
		expr := text[m[2]:m[3]]
		if seen[expr] {
			continue
		}
		seen[expr] = true
		parts := strings.Split(expr, ".")
		root := parts[0]
		called := m[4] >= 0
		switch {
		case guardKeywords[root] && len(parts) == 1:
			continue
		case requestRoots[root]:
			unresolved = append(unresolved, unresolvedInput{expr, "request-input"})
			continue
		case root[0] >= 'A' && root[0] <= 'Z':
			if len(parts) > 1 || called {
				unresolved = append(unresolved, unresolvedInput{expr, "unsupported-expression"})
			}
			continue // a bare constant is a literal
		case called:
			unresolved = append(unresolved, unresolvedInput{expr, "unsupported-expression"})
			continue
		}
		if n := len(parts); n > 1 && guardPredicates[parts[n-1]] {
			parts = parts[:n-1]
		}
		switch {
		case len(parts) == 2 && !strings.HasSuffix(parts[0], "?") && !strings.HasSuffix(parts[1], "!"):
			attr := strings.TrimSuffix(parts[1], "?")
			if parts[0] == "self" && b.model != "" {
				resolved = append(resolved, [2]string{b.model, attr})
			} else if model, ok := a.modelOf(parts[0]); ok {
				resolved = append(resolved, [2]string{model, attr})
			} else {
				unresolved = append(unresolved, unresolvedInput{expr, "unresolved-receiver"})
			}
		case len(parts) == 1 && b.model != "" && !strings.HasSuffix(parts[0], "!") && !strings.HasPrefix(parts[0], "@"):
			resolved = append(resolved, [2]string{b.model, strings.TrimSuffix(parts[0], "?")})
		case len(parts) == 1:
			unresolved = append(unresolved, unresolvedInput{expr, "unresolved-receiver"})
		default:
			unresolved = append(unresolved, unresolvedInput{expr, "unsupported-expression"})
		}
	}
	if b.kind == "when" {
		unresolved = append(unresolved, unresolvedInput{"case subject", "unsupported-expression"})
	}
	return resolved, unresolved
}

// branchSetup joins a branch's inputs to the writer index (AMAP-V0-115).
func branchSetup(app *railsApp, idx *setupWriterIndex, b rubyBranch) *setupRow {
	resolved, unresolved := app.guardInputs(b)
	row := &setupRow{Inputs: []guardInput{}, Unresolved: unresolved}
	writers := 0
	for _, in := range resolved {
		all := idx.writersOf(in[0], in[1])
		writers += len(all)
		gi := guardInput{Model: in[0], Attribute: in[1], Expr: in[0] + "#" + in[1], Writers: all}
		if len(all) > maxInputWriters {
			gi.Writers, gi.More = all[:maxInputWriters], len(all)-maxInputWriters
		}
		row.Inputs = append(row.Inputs, gi)
	}
	switch {
	case len(unresolved) > 0:
		row.Status = "unresolved-guard"
	case writers > 0:
		row.Status = "candidates"
	case len(resolved) == 0:
		row.Status = "no-input"
	default:
		row.Status = "none-found"
	}
	return row
}

// reachingRoutes are the routes whose request reaches a controller method: its own routes, the
// routes of actions that call it (one hop) and, for a callback, the routes of the actions its
// registrations apply to. maybe holds the routes a conditional or unread registration may reach.
func reachingRoutes(f *rubyFile, b rubyBranch, routes []railsRoute) (reaching []railsRoute, via string, maybe []railsRoute) {
	byAction := map[string][]railsRoute{}
	for _, rt := range routes {
		if controllerConstant(rt.ctrl) == b.constant {
			byAction[rt.action] = append(byAction[rt.action], rt)
		}
	}
	if len(byAction) == 0 || b.method == "" || b.singleton {
		return nil, "", nil
	}
	if direct := byAction[b.method]; len(direct) > 0 {
		return direct, "routed-action", nil
	}
	scope := f.scope(b.constant)
	if scope == nil {
		return nil, "", nil
	}
	if state, registered := callbackStates(f, scope, b.method, byAction); registered {
		for action, rs := range byAction {
			switch state[action] {
			case callbackApplies:
				reaching = append(reaching, rs...)
			case callbackMaybe:
				maybe = append(maybe, rs...)
			}
		}
		return reaching, "callback", maybe
	}
	out := []railsRoute{}
	for _, d := range scope.defs {
		if !d.instance || len(byAction[d.name]) == 0 {
			continue
		}
		for ln := d.line + 1; ln < d.end && ln <= len(f.code); ln++ {
			hit := false
			for _, w := range rubyWordsOf(f.code[ln-1]) {
				hit = hit || w.keyword && w.text == b.method
			}
			if hit {
				out = append(out, byAction[d.name]...)
				break
			}
		}
	}
	if len(out) > 0 {
		return out, "called-by-routed-action", nil
	}
	return nil, "", nil
}

const (
	callbackNone = iota
	callbackMaybe
	callbackApplies
)

var (
	callbackRegister = regexp.MustCompile(`^(?:prepend_|append_)?(?:before|after|around)_action$`)
	callbackSkip     = regexp.MustCompile(`^skip_(?:before|after|around)_action$`)
)

// callbackStates replays a controller's own callback statements for one method in declaration
// order across every reopening of the class: a registration replaces the method's earlier one, a
// skip removes it for its actions, and `only:`/`except:` restrict both. A statement with `if:` or
// `unless:`, an unread option, or one continued past its line leaves its actions possible
// (AMAP-V0-112). registered is false when no statement names the method.
func callbackStates(f *rubyFile, scope *rubyScope, method string, byAction map[string][]railsRoute) (map[string]int, bool) {
	state := map[string]int{}
	registered := false
	for _, part := range scope.parts {
		for ln := part.line; ln <= part.end && ln <= len(f.code) && ln <= len(f.raw); ln++ {
			words := rubyWordsOf(f.code[ln-1])
			if len(words) < 2 || !words[0].keyword {
				continue
			}
			register, skip := callbackRegister.MatchString(words[0].text), callbackSkip.MatchString(words[0].text)
			if !register && !skip {
				continue
			}
			named, mentioned, actions, sure := readCallback(f.raw[ln-1], words[0].text, method, byAction)
			if !named && !mentioned {
				continue
			}
			registered = registered || register
			for action := range byAction {
				in := actions[action]
				switch {
				case !named && in && register:
					// An unread statement may register the method or read it in a condition.
					state[action] = max(state[action], callbackMaybe)
				case !named && in:
					state[action] = min(state[action], callbackMaybe)
				case register && sure:
					state[action] = callbackNone
					if in {
						state[action] = callbackApplies
					}
				case register:
					state[action] = callbackNone
					if in {
						state[action] = callbackMaybe
					}
				case in && sure:
					state[action] = callbackNone
				case in:
					state[action] = min(state[action], callbackMaybe)
				}
			}
		}
	}
	return state, registered
}

// readCallback reads one callback statement: whether the method is a positional callback name,
// whether it appears elsewhere in the statement, the actions the statement applies to, and whether
// the statement was read completely and unconditionally.
func readCallback(line, keyword, method string, byAction map[string][]railsRoute) (named, mentioned bool, actions map[string]bool, sure bool) {
	all := func() map[string]bool {
		out := map[string]bool{}
		for a := range byAction {
			out[a] = true
		}
		return out
	}
	ts, balanced := lexRoutes(line, true)
	for len(ts) > 0 && isPunct(ts[len(ts)-1], ";") {
		ts = ts[:len(ts)-1]
	}
	if len(ts) == 0 || ts[0].kind != tokIdent || ts[0].text != keyword {
		return false, false, nil, false
	}
	ts = ts[1:]
	for _, t := range ts {
		mentioned = mentioned || (t.kind == tokIdent || t.kind == tokString) && t.text == method
	}
	if !balanced || len(ts) == 0 || isPunct(ts[len(ts)-1], ",") || isPunct(ts[len(ts)-1], "\\") {
		return false, mentioned, all(), false
	}
	if isPunct(ts[0], "(") && isPunct(ts[len(ts)-1], ")") {
		ts = ts[1 : len(ts)-1]
	}
	elements := [][]token{}
	depth, start := 0, 0
	for i, t := range ts {
		switch {
		case next(ts, i, "(", "[", "{"):
			depth++
		case next(ts, i, ")", "]", "}"):
			depth--
		case depth == 0 && isPunct(t, ","):
			elements = append(elements, ts[start:i])
			start = i + 1
		}
	}
	elements = append(elements, ts[start:])
	sure = true
	var only, except []string
	options := false
	for _, el := range elements {
		if len(el) >= 3 && el[0].kind == tokIdent && isPunct(el[1], ":") {
			options = true
			v, ok := routeValue(el[2:])
			names := []string{v.str}
			if v.kind == "list" {
				names = v.list
			}
			switch {
			case el[0].text == "only" && ok && (v.kind == "symbol" || v.kind == "string" || v.kind == "list"):
				only = append(only, names...)
			case el[0].text == "except" && ok && (v.kind == "symbol" || v.kind == "string" || v.kind == "list"):
				except = append(except, names...)
			case el[0].text == "prepend" || el[0].text == "raise":
			default:
				sure = false
			}
			continue
		}
		v, ok := routeValue(el)
		if options || !ok || v.kind != "symbol" && v.kind != "string" {
			sure = false
			continue
		}
		named = named || v.str == method
	}
	if !named {
		return false, mentioned, all(), false
	}
	actions = all()
	if only != nil {
		actions = map[string]bool{}
		for _, a := range only {
			actions[a] = true
		}
	}
	for _, a := range except {
		delete(actions, a)
	}
	return true, mentioned, actions, sure
}

// requestReach adds the first request of a test that matches one of the routes.
func requestReach(t testRefs, routes []railsRoute, via string, add func(reachRow)) bool {
	for _, c := range t.requests {
		for _, rt := range routes {
			if c.matches(rt) {
				basis := via + "-literal-path"
				if c.helper != "" {
					basis = via + "-path-helper"
				}
				add(reachRow{Test: t.path + ":" + strconv.Itoa(c.line), Basis: basis, Route: rt.verb + " " + rt.path})
				return true
			}
		}
	}
	return false
}

func sortRoutes(rs []railsRoute) {
	sort.Slice(rs, func(i, j int) bool {
		if rs[i].file != rs[j].file {
			return rs[i].file < rs[j].file
		}
		if rs[i].line != rs[j].line {
			return rs[i].line < rs[j].line
		}
		return rs[i].action < rs[j].action
	})
}

var unitMethodRef = func(method string) *regexp.Regexp {
	return regexp.MustCompile(`(?:[.#]|:)` + regexp.QuoteMeta(method) + `(?:[^\w?!]|$)`)
}

// reach groups the tests that reach a branch by boundary (AMAP-V0-112).
func reach(f *rubyFile, b rubyBranch, routes []railsRoute, tests []testRefs) []boundaryRow {
	rows := map[string]*boundaryRow{}
	possible := map[string][]reachRow{}
	for _, bd := range boundaries {
		rows[bd] = &boundaryRow{Boundary: bd, Tests: []reachRow{}}
	}
	add := func(bd string, r reachRow) {
		rows[bd].Count++
		if len(rows[bd].Tests) < maxBranchTests {
			rows[bd].Tests = append(rows[bd].Tests, r)
		}
	}
	reaching, via, maybe := reachingRoutes(f, b, routes)
	sortRoutes(reaching)
	sortRoutes(maybe)
	sameController := []railsRoute{}
	for _, rt := range routes {
		if controllerConstant(rt.ctrl) == b.constant && containsField(rt.verb, "GET") {
			sameController = append(sameController, rt)
		}
	}
	sortRoutes(sameController)
	var constRef, methodRef *regexp.Regexp
	if b.constant != "" && b.method != "" {
		last := b.constant[strings.LastIndex(b.constant, ":")+1:]
		constRef = regexp.MustCompile(`\b` + regexp.QuoteMeta(last) + `\b`)
		methodRef = unitMethodRef(b.method)
	}
	for _, t := range tests {
		switch t.boundary {
		case "unit":
			if constRef != nil {
				if loc := constRef.FindStringIndex(t.text); loc != nil {
					if mloc := methodRef.FindStringIndex(t.text); mloc != nil {
						add("unit", reachRow{Test: t.path + ":" + strconv.Itoa(lineAt(t.text, mloc[0])), Basis: "names-constant-and-method"})
					}
				}
			}
		case "request":
			if !requestReach(t, reaching, via, func(r reachRow) { add("request", r) }) {
				requestReach(t, maybe, "conditional-"+via, func(r reachRow) { possible["request"] = append(possible["request"], r) })
			}
		}
		if t.boundary != "e2e" {
			continue
		}
		reached := false
	visits:
		for _, c := range t.visits {
			for _, rt := range reaching {
				if containsField(rt.verb, "GET") || rt.verb == "ANY" {
					if c.matches(rt) {
						add("e2e", reachRow{Test: t.path + ":" + strconv.Itoa(c.line), Basis: via + "-visit", Route: "GET " + rt.path})
						reached = true
						break visits
					}
				}
			}
		}
		if reached {
			continue
		}
	conditional:
		for _, c := range t.visits {
			for _, rt := range maybe {
				if (containsField(rt.verb, "GET") || rt.verb == "ANY") && c.matches(rt) {
					possible["e2e"] = append(possible["e2e"], reachRow{Test: t.path + ":" + strconv.Itoa(c.line), Basis: "conditional-" + via + "-visit", Route: "GET " + rt.path})
					reached = true
					break conditional
				}
			}
		}
		if reached {
			continue
		}
	screens:
		for _, c := range t.visits {
			for _, rt := range sameController {
				if c.matches(rt) {
					possible["e2e"] = append(possible["e2e"], reachRow{Test: t.path + ":" + strconv.Itoa(c.line), Basis: "same-controller-screen", Route: "GET " + rt.path})
					break screens
				}
			}
		}
	}
	out := []boundaryRow{}
	for _, bd := range boundaries {
		r := rows[bd]
		switch {
		case r.Count > 0:
			r.Status = "reached"
		case len(possible[bd]) > 0:
			r.Status, r.Count = "possible", len(possible[bd])
			r.Tests = possible[bd]
			if len(r.Tests) > maxBranchTests {
				r.Tests = r.Tests[:maxBranchTests]
			}
		default:
			r.Status, r.Note = "none", "none at "+bd
		}
		out = append(out, *r)
	}
	return out
}

// BranchesReport is the branch query's capped document (AMAP-V0-116).
type BranchesReport struct {
	Document []byte
}

// QueryBranches runs the branch query (AMAP-V0-110..116). It reads committed objects of --root
// at the map revision only and writes nothing.
func QueryBranches(ctx context.Context, m *Map, cites []string, o Options) (*BranchesReport, error) {
	query := func(msg string) error { return &gokernel.Error{Code: "appmap-invalid-query", Message: msg} }
	budget, err := o.budget(DefaultFlowBudget)
	if err != nil {
		return nil, err
	}
	if len(cites) == 0 || len(cites) > maxCites {
		return nil, query(fmt.Sprintf("branches requires 1 to %d --cite ranges", maxCites))
	}
	parsed, seen := []branchCite{}, map[string]bool{}
	for _, s := range cites {
		c, ok := parseCite(s)
		if !ok || seen[s] {
			return nil, query(fmt.Sprintf("--cite %q is not a unique PATH:START[-END] of at most %d lines in a repository-relative .rb file", s, maxCiteLines))
		}
		seen[s] = true
		parsed = append(parsed, c)
	}
	r := repo{ctx: ctx, root: o.Root}
	rev, err := r.resolve(m.Revision)
	if err != nil || rev != m.Revision {
		return nil, &gokernel.Error{Code: "appmap-invalid-revision", Message: "the map revision is not a commit in --root"}
	}
	man, err := readManifestAt(r, m, nil)
	if err != nil {
		return nil, err
	}
	compiled, err := compileRailsRoutes(r, rev, man)
	if err != nil {
		return nil, err
	}
	// Each cite reads against the deepest application root that contains it; "." when no Rails
	// router declares one.
	roots := []string{}
	for root := range compiled.roots {
		roots = append(roots, root)
	}
	if len(roots) == 0 {
		roots = append(roots, ".")
	}
	sort.Slice(roots, func(i, j int) bool {
		if len(roots[i]) != len(roots[j]) {
			return len(roots[i]) > len(roots[j])
		}
		return roots[i] < roots[j]
	})
	rootOf := func(p string) string {
		for _, root := range roots {
			if root == "." || strings.HasPrefix(p, root+"/") {
				return root
			}
		}
		return ""
	}
	citeData := map[string][]byte{}
	for _, c := range parsed {
		if rootOf(c.path) == "" {
			return nil, query(fmt.Sprintf("--cite %s is outside every application root", c.path))
		}
		if _, ok := citeData[c.path]; ok {
			continue
		}
		_, data, err := r.readOne(rev, c.path, maxTestFileBytes)
		if err != nil {
			var coded *gokernel.Error
			if errors.As(err, &coded) && coded.Code == "appmap-bound-exceeded" {
				return nil, err
			}
			return nil, query(fmt.Sprintf("--cite %s is not a file at the map revision", c.path))
		}
		citeData[c.path] = data
	}
	for _, c := range parsed {
		if lines := strings.Count(string(citeData[c.path]), "\n") + 1; c.end > lines {
			return nil, query(fmt.Sprintf("--cite %s:%d-%d is outside its %d lines", c.path, c.start, c.end, lines))
		}
	}
	mapTests, unread, err := readTestFiles(r, m)
	if err != nil {
		return nil, err
	}
	// One budget covers the map's test files and every root a cite needs; other roots are unread.
	reads, needed, mapBytes := newAppReadBudget(), map[string]bool{}, 0
	for _, data := range mapTests {
		mapBytes += len(data)
	}
	if err := reads.spend(len(mapTests), mapBytes); err != nil {
		return nil, err
	}
	for _, c := range parsed {
		needed[rootOf(c.path)] = true
	}
	apps := map[string]*railsApp{}
	indexes := map[string]*setupWriterIndex{}
	appTests := map[string][]testRefs{}
	appRoutes := map[string][]railsRoute{}
	for _, root := range roots {
		if !needed[root] {
			continue
		}
		app, err := loadRailsApp(r, rev, root, []string{"app", "spec", "test", "factories"}, func(p string) bool {
			rel := strings.TrimPrefix(p, appJoin(root, ""))
			if root == "." {
				rel = p
			}
			return strings.HasSuffix(p, ".rb") || strings.HasSuffix(p, ".yml") && (strings.HasPrefix(rel, "spec/fixtures/") || strings.HasPrefix(rel, "test/fixtures/"))
		}, reads)
		if err != nil {
			return nil, err
		}
		apps[root] = app
		for _, i := range compiled.roots[root] {
			appRoutes[root] = append(appRoutes[root], compiled.routes[i])
		}
		indexes[root] = newSetupWriterIndex(app, appRoutes[root])
		tests := []testRefs{}
		paths := make([]string, 0, len(app.data))
		for p := range app.data {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		for _, p := range paths {
			rel := p
			if root != "." {
				rel = strings.TrimPrefix(p, root+"/")
			}
			if strings.HasPrefix(rel, "spec/") && strings.HasSuffix(p, "_spec.rb") || strings.HasPrefix(rel, "test/") && strings.HasSuffix(p, "_test.rb") {
				tests = append(tests, scanTestRefs(p, boundaryOf(rel), string(app.data[p])))
			}
		}
		mapPaths := make([]string, 0, len(mapTests))
		for p := range mapTests {
			mapPaths = append(mapPaths, p)
		}
		sort.Strings(mapPaths)
		for _, p := range mapPaths {
			tests = append(tests, scanTestRefs(p, "e2e", string(mapTests[p])))
		}
		appTests[root] = tests
	}
	branchItems, noBranch := []any{}, []any{}
	total := 0
	for _, c := range parsed {
		root := rootOf(c.path)
		app := apps[root]
		data := citeData[c.path]
		f := parseRubyStructure(c.path, data)
		cite := c.path + ":" + strconv.Itoa(c.start)
		if c.end != c.start {
			cite += "-" + strconv.Itoa(c.end)
		}
		if f.broken != 0 {
			noBranch = append(noBranch, noBranchRow{Cite: cite, Reason: "unsupported-ruby-syntax"})
			continue
		}
		modelFile := ""
		if rel := strings.TrimPrefix(c.path, appJoin(root, "")); root == "." && strings.HasPrefix(c.path, "app/models/") || root != "." && strings.HasPrefix(rel, "app/models/") {
			if root == "." {
				rel = c.path
			}
			name := strings.TrimSuffix(strings.TrimPrefix(rel, "app/models/"), ".rb")
			if !strings.HasPrefix(name, "concerns/") {
				modelFile = rubyCamelize(name)
			}
		}
		found := readBranches(f, strings.Split(string(data), "\n"), c.start, c.end, modelFile)
		if len(found) == 0 {
			noBranch = append(noBranch, noBranchRow{Cite: cite, Reason: "no-branch"})
			continue
		}
		for _, b := range found {
			total++
			row := branchRow{At: c.path + ":" + strconv.Itoa(b.line), Kind: b.kind, Constant: b.constant, Method: b.method}
			if secretscreen.MatchString(b.guard) {
				row.GuardRedacted = true
			} else {
				row.Guard = truncateBytes(b.guard, maxGuardBytes)
			}
			row.Boundaries = reach(f, b, appRoutes[root], appTests[root])
			for _, bd := range row.Boundaries {
				if bd.Status != "reached" {
					row.Setup = branchSetup(app, indexes[root], b)
					if row.GuardRedacted {
						for i := range row.Setup.Unresolved {
							row.Setup.Unresolved[i].Expr = "redacted"
						}
					}
					break
				}
			}
			branchItems = append(branchItems, row)
		}
	}
	endpoints, factories, fixtures := 0, 0, 0
	for _, idx := range indexes {
		endpoints += idx.endpoints
		for _, w := range idx.writers {
			switch w.Kind {
			case "factory", "trait":
				factories++
			case "fixture":
				fixtures++
			}
		}
	}
	testsRead := 0
	for _, t := range appTests {
		testsRead += len(t)
	}
	head := []field{{"schema", BranchesSchema}, {"app", m.App}, {"map_revision", m.Revision}, {"map_digest", m.Digest},
		{"budget", budget}, {"full", o.Full}, {"limits", BranchesLimits}, {"cites", cites}, {"branches_total", total},
		{"test_files_read", testsRead}, {"test_files_unread", unread}, {"routes", len(compiled.routes)},
		{"setup_endpoints", endpoints}, {"factories_and_traits", factories}, {"fixtures", fixtures}}
	doc, err := render(head, []section{{"branches", branchItems}, {"no_branch", noBranch}}, budget)
	if err != nil {
		return nil, err
	}
	return &BranchesReport{Document: doc}, nil
}

func truncateBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && s[n]&0xC0 == 0x80 {
		n--
	}
	return s[:n]
}
