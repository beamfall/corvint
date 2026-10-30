package testsource

import (
	"regexp"
	"strconv"
	"strings"
)

// This file reads the literal Rails and CanCanCan-style forms the actor-authorization check
// supports (TSA-V0-034, TSA-V0-035). Anything outside those forms is recorded as unparsed, so a
// conclusion that depends on it is UNKNOWN rather than authorized (TSA-V0-038).

// rubyLine is one logical Ruby statement: comments removed, continuation lines joined, and the
// 1-based line it starts on.
type rubyLine struct {
	line int
	text string
	// path is the file the line was read from; the RSpec reader sets it so that a shared group
	// read through an inclusion keeps its own file's locations (TSA-V0-090).
	path string
}

// rubyLines splits Ruby source into logical statements.
func rubyLines(text string) []rubyLine {
	out := []rubyLine{}
	pending, start := "", 0
	heredocs := []rubyHeredoc{}
	for n, raw := range strings.Split(text, "\n") {
		if len(heredocs) > 0 {
			// A heredoc body is string text, not code (TSA-V0-396).
			if heredocs[0].closes(raw) {
				heredocs = heredocs[1:]
			}
			continue
		}
		stripped := strings.TrimSpace(stripRubyComment(raw))
		heredocs = rubyHeredocs(stripped)
		if pending == "" {
			start = n + 1
		}
		if pending != "" {
			pending += " "
		}
		pending += stripped
		if strings.HasSuffix(pending, ",") || strings.HasSuffix(pending, "(") || strings.HasSuffix(pending, "[") || strings.HasSuffix(pending, "\\") {
			pending = strings.TrimSuffix(pending, "\\")
			continue
		}
		if pending != "" {
			out = append(out, rubyLine{line: start, text: pending})
		}
		pending = ""
	}
	if pending != "" {
		out = append(out, rubyLine{line: start, text: pending})
	}
	return out
}

// rubyHeredoc is one heredoc a line opens: its terminator, and whether the terminator may be
// indented (`<<~ID`, `<<-ID`) or must start the line (`<<ID`).
type rubyHeredoc struct {
	id       string
	indented bool
}

// rubyHeredocStart is `<<~ID`, `<<-ID`, a quoted `<<'ID'` or `<<"ID"`, or a bare upper-case
// `<<ID`; a shovel (`list << item`) has a space and a lower-case operand.
var rubyHeredocStart = regexp.MustCompile(`<<([~-]?)(?:(['"])([A-Za-z_]\w*)['"]|([A-Za-z_]\w*))`)

func rubyHeredocs(text string) []rubyHeredoc {
	out := []rubyHeredoc{}
	for _, m := range rubyHeredocStart.FindAllStringSubmatch(text, -1) {
		id := m[3] + m[4]
		if m[1] == "" && m[2] == "" && strings.ToUpper(id) != id {
			continue
		}
		out = append(out, rubyHeredoc{id: id, indented: m[1] != ""})
	}
	return out
}

func (h rubyHeredoc) closes(raw string) bool {
	raw = strings.TrimRight(raw, " \t\r")
	if h.indented {
		raw = strings.TrimSpace(raw)
	}
	return raw == h.id
}

// stripRubyComment drops a `#` comment outside a quoted string.
func stripRubyComment(line string) string {
	quote := byte(0)
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quote != 0 && c == '\\':
			i++
		case quote != 0 && c == quote:
			quote = 0
		case quote == 0 && (c == '\'' || c == '"'):
			quote = c
		case quote == 0 && c == '#':
			return line[:i]
		}
	}
	return line
}

var (
	rubyOpener = regexp.MustCompile(`^(def|class|module|if|unless|case|while|until|begin|for)\b`)
	rubyDo     = regexp.MustCompile(`\bdo(\s*\|[^|]*\|)?$`)
	rubyEnd    = regexp.MustCompile(`^end\b`)
	// rubyOneLine is a statement that opens and closes on one line, such as
	// `def read_data; build_report_io; end`; it opens no block (TSA-V0-084).
	rubyOneLine = regexp.MustCompile(`;\s*end$`)
	// rubyEndlessDef is an endless method, `def name = expr` or `def name(args) = expr`; it opens
	// no block. A setter (`def name=(v)`) and a comparison (`==`) are not one.
	rubyEndlessDef = regexp.MustCompile(`^def\s+[A-Za-z_]\w*[?!]?(?:\s+|\([^()]*\)\s*)=(?:[^=~]|$)`)
	// rubyValueOpener is if, unless, case, begin, while or until in value position, where it
	// opens a block its own `end` closes: after an assignment (`=`, `||=`, `+=`), a hash `=>`, a
	// label key (`key: if`), an append (`<< if`), an opening bracket, a comma, `&&` or `||`, a
	// line-start `else` (`else if`, which nests a block), and case or begin after `return`. A
	// modifier (`foo if bar`, `return if bar`) follows an expression or `return` and opens
	// nothing, and a keyword argument (`if: x`) is not the keyword (TSA-V0-396, V1-2078).
	rubyValueOpener = regexp.MustCompile(`(?:(?:=>?|[(\[,]|&&|\|\||\w:\s|<<\s|^else\s)\s*(if|unless|case|begin|while|until)|\breturn\s+(case|begin))(?:\s|[(;]|$)`)
	// rubyLineEnd is an `end` word; after a value opener it closes the block on the same line
	// unless it is a method or symbol (`.end`, `:end`, `end?`, `end:`), see rubyEndKeyword.
	rubyLineEnd = regexp.MustCompile(`\bend\b`)
)

func rubyOpens(text string) bool {
	if (rubyOpener.MatchString(text) || rubyDo.MatchString(text)) && !rubyOneLine.MatchString(text) && !rubyEndlessDef.MatchString(text) {
		return true
	}
	return rubyValueOpens(text)
}

// rubyValueOpens reports whether a line opens a block with a value-position keyword that the line
// does not also close (`x = if a then 1 else 2 end`); string literals are ignored (TSA-V0-396).
func rubyValueOpens(text string) bool {
	return rubyValueKeyword(text) != ""
}

// rubyValueKeyword returns the value-position keyword whose block a line opens and leaves open,
// or "" (TSA-V0-396).
func rubyValueKeyword(text string) string {
	text = stripRubyStrings(text)
	m := rubyValueOpener.FindStringSubmatchIndex(text)
	if m == nil || rubyEndKeyword(text[m[1]-1:]) {
		return ""
	}
	if m[2] >= 0 {
		return text[m[2]:m[3]]
	}
	return text[m[4]:m[5]]
}

// rubyEndKeyword reports whether text holds an `end` keyword, not a method call (`.end`, `end?`),
// a symbol (`:end`), a hash key (`end:`) or a variable (`@end`).
func rubyEndKeyword(text string) bool {
	for _, at := range rubyLineEnd.FindAllStringIndex(text, -1) {
		if at[0] > 0 && strings.ContainsRune(".:@$", rune(text[at[0]-1])) {
			continue
		}
		if at[1] < len(text) && strings.ContainsRune("?!:", rune(text[at[1]])) && !strings.HasPrefix(text[at[1]:], "::") {
			continue
		}
		return true
	}
	return false
}

// rubyBlockEnd returns the index of the `end` closing the block opened at lines[i], or len(lines).
func rubyBlockEnd(lines []rubyLine, i int) int {
	depth := 0
	for j := i; j < len(lines); j++ {
		if rubyOpens(lines[j].text) {
			depth++
		} else if rubyEnd.MatchString(lines[j].text) {
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return len(lines)
}

// railsRoute is one literal route: method, path segments (`:name` for a dynamic segment) and the
// controller action it reaches.
type railsRoute struct {
	method     string
	segments   []string
	controller string
	action     string
	line       int
}

type railsRoutes struct {
	path     string
	routes   []railsRoute
	unparsed []int
}

var (
	routesDraw     = regexp.MustCompile(`^Rails\.application\.routes\.draw do$`)
	routeNamespace = regexp.MustCompile(`^namespace\s+:(\w+)\s+do$`)
	routeResources = regexp.MustCompile(`^resources\s+:(\w+)(\s*,\s*(only|except):\s*\[([^\]]*)\])?(\s+do)?$`)
	routeVerb      = regexp.MustCompile(`^(get|post|put|patch|delete)\s+['"]([^'"]*)['"]\s*(,\s*to:|=>)\s*['"]([\w/]+)#(\w+)['"]$`)
	routeRoot      = regexp.MustCompile(`^root\s+(to:\s*)?['"]([\w/]+)#(\w+)['"]$`)
	rubySymbol     = regexp.MustCompile(`^:(\w+)$`)
)

// resourceActions is the Rails `resources` expansion, in Rails' own declaration order.
var resourceActions = []struct{ method, suffix, action string }{
	{"GET", "", "index"}, {"POST", "", "create"}, {"GET", "new", "new"}, {"GET", ":id/edit", "edit"},
	{"GET", ":id", "show"}, {"PATCH", ":id", "update"}, {"PUT", ":id", "update"}, {"DELETE", ":id", "destroy"},
}

type routeFrame struct {
	prefix, module string
	opaque         bool
}

// parseRailsRoutes reads the literal routes.rb forms (TSA-V0-034): `namespace :x do`,
// `resources :x` with `only:`/`except:` and nesting, `<verb> 'path', to: 'c#a'` (or `=>`) and
// `root`. Any other statement is unparsed; a block it opens is skipped.
func parseRailsRoutes(path, text string) railsRoutes {
	result := railsRoutes{path: path}
	stack := []routeFrame{}
	top := func() routeFrame {
		if len(stack) == 0 {
			return routeFrame{}
		}
		return stack[len(stack)-1]
	}
	join := func(prefix, segment string) string {
		return strings.Trim(strings.Trim(prefix, "/")+"/"+strings.Trim(segment, "/"), "/")
	}
	add := func(method, routePath, controller, action string, line int) {
		segments := []string{}
		for _, s := range strings.Split(strings.TrimSuffix(strings.Trim(routePath, "/"), "(.:format)"), "/") {
			if s != "" {
				segments = append(segments, s)
			}
		}
		result.routes = append(result.routes, railsRoute{method: method, segments: segments, controller: controller, action: action, line: line})
	}
	for _, l := range rubyLines(text) {
		frame := top()
		if rubyEnd.MatchString(l.text) {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			continue
		}
		if frame.opaque {
			if rubyOpens(l.text) {
				stack = append(stack, frame)
			}
			continue
		}
		if routesDraw.MatchString(l.text) {
			stack = append(stack, routeFrame{})
			continue
		}
		if m := routeNamespace.FindStringSubmatch(l.text); m != nil {
			stack = append(stack, routeFrame{prefix: join(frame.prefix, m[1]), module: join(frame.module, m[1])})
			continue
		}
		if m := routeResources.FindStringSubmatch(l.text); m != nil {
			name, filter, list := m[1], m[3], map[string]bool{}
			parsed := true
			for _, item := range strings.Split(m[4], ",") {
				if item = strings.TrimSpace(item); item == "" {
					continue
				}
				symbol := rubySymbol.FindStringSubmatch(item)
				if symbol == nil {
					parsed = false
					break
				}
				list[symbol[1]] = true
			}
			if !parsed {
				result.unparsed = append(result.unparsed, l.line)
				if m[5] != "" {
					stack = append(stack, routeFrame{opaque: true})
				}
				continue
			}
			controller := join(frame.module, name)
			for _, a := range resourceActions {
				if filter == "only" && !list[a.action] || filter == "except" && list[a.action] {
					continue
				}
				add(a.method, join(join(frame.prefix, name), a.suffix), controller, a.action, l.line)
			}
			if m[5] != "" {
				stack = append(stack, routeFrame{prefix: join(join(frame.prefix, name), ":"+singular(name)+"_id"), module: frame.module})
			}
			continue
		}
		if m := routeVerb.FindStringSubmatch(l.text); m != nil {
			add(strings.ToUpper(m[1]), join(frame.prefix, m[2]), join(frame.module, m[4]), m[5], l.line)
			continue
		}
		if m := routeRoot.FindStringSubmatch(l.text); m != nil {
			add("GET", frame.prefix, join(frame.module, m[2]), m[3], l.line)
			continue
		}
		result.unparsed = append(result.unparsed, l.line)
		if rubyOpens(l.text) {
			stack = append(stack, routeFrame{opaque: true})
		}
	}
	return result
}

func singular(name string) string {
	switch {
	case strings.HasSuffix(name, "ies"):
		return strings.TrimSuffix(name, "ies") + "y"
	case strings.HasSuffix(name, "s"):
		return strings.TrimSuffix(name, "s")
	}
	return name
}

// match returns the first route, in file order as Rails dispatches, that a request reaches. A
// request segment "*" stands for a template substitution and matches only a dynamic segment.
func (r railsRoutes) match(method string, segments []string) (railsRoute, bool) {
	for _, route := range r.routes {
		if route.method != method || len(route.segments) != len(segments) {
			continue
		}
		ok := true
		for i, s := range route.segments {
			if !strings.HasPrefix(s, ":") && s != segments[i] {
				ok = false
				break
			}
		}
		if ok {
			return route, true
		}
	}
	return railsRoute{}, false
}

var (
	authorizeCall     = regexp.MustCompile(`\bauthorize!`)
	authorizeLiteral  = regexp.MustCompile(`^authorize!\s*\(?\s*:(\w+)\s*,\s*([A-Z]\w*(?:::[A-Z]\w*)*)\s*\)?$`)
	implicitAuthorize = regexp.MustCompile(`^(load_and_authorize_resource|authorize_resource)\b`)
	rubyDef           = regexp.MustCompile(`^def\s+(self\.)?(\w+[!?]?)\s*(\(.*\))?$`)
	// rubyBranch starts a clause of the enclosing method or block; rubyExit can leave the method.
	rubyBranch = regexp.MustCompile(`^(rescue|ensure|else|elsif|when|in)\b`)
	rubyExit   = regexp.MustCompile(`\b(return|raise|fail|throw|exit|abort)\b`)
)

// controllerCheck finds the one literal `authorize! :action, Subject` in a controller action
// (TSA-V0-034). A check inside a nested block or clause, or after a statement that can leave the
// action early, is conditional. It returns the check, or an UNKNOWN reason code with its detail.
func controllerCheck(path, text, action string) (AbilityCheck, string, string) {
	lines := rubyLines(text)
	implicit := 0
	for _, l := range lines {
		if implicitAuthorize.MatchString(l.text) {
			implicit = l.line
		}
	}
	for i, l := range lines {
		m := rubyDef.FindStringSubmatch(l.text)
		if m == nil || m[1] != "" || m[2] != action {
			continue
		}
		end := rubyBlockEnd(lines, i)
		checks := []AbilityCheck{}
		depth, guard, conditional := 0, 0, ""
		for _, s := range lines[i+1 : min(end, len(lines))] {
			if authorizeCall.MatchString(s.text) {
				c := authorizeLiteral.FindStringSubmatch(s.text)
				if c == nil {
					return AbilityCheck{}, "check-not-literal", locate(path, s.line)
				}
				checks = append(checks, AbilityCheck{Action: c[1], Subject: c[2], Path: path, Line: s.line})
				switch {
				case depth > 0 && conditional == "":
					conditional = locate(path, s.line) + " authorize! is inside a nested block or clause"
				case guard > 0 && conditional == "":
					conditional = locate(path, s.line) + " authorize! follows an early exit at line " + strconv.Itoa(guard)
				}
				continue
			}
			switch {
			case rubyOpens(s.text):
				depth++
			case rubyEnd.MatchString(s.text):
				depth--
			case depth == 0 && rubyBranch.MatchString(s.text):
				depth++
			}
			if guard == 0 && rubyExit.MatchString(s.text) {
				guard = s.line
			}
		}
		switch {
		case len(checks) > 1:
			return AbilityCheck{}, "check-multiple", locate(path, checks[1].Line)
		case len(checks) == 1 && conditional != "":
			return AbilityCheck{}, "check-conditional", conditional
		case len(checks) == 1:
			return checks[0], "", ""
		case implicit > 0:
			return AbilityCheck{}, "check-implicit", locate(path, implicit)
		}
		return AbilityCheck{}, "check-not-found", locate(path, l.line)
	}
	return AbilityCheck{}, "action-not-found", path + " has no `def " + action + "`"
}

// abilityRule is one `can` or `cannot` statement.
type abilityRule struct {
	actions     []string
	subjects    []string
	cannot      bool
	conditional bool
	line        int
}

// abilityRole is one role rule set: a method named by the role.
type abilityRole struct {
	line     int
	rules    []abilityRule
	inherits []string
	unparsed []int
}

// abilityModel is a CanCanCan-style Ability class whose initializer dispatches to the method named
// by the user's role (TSA-V0-035).
type abilityModel struct {
	path     string
	dispatch bool
	global   abilityRole
	roles    map[string]*abilityRole
}

var (
	abilityClass     = regexp.MustCompile(`^class\s+Ability\b`)
	abilityDispatch  = regexp.MustCompile(`^(public_)?send\s*\(?\s*user\.role(\.to_sym)?\s*\)?$`)
	abilityRuleStart = regexp.MustCompile(`^(can|cannot)\b`)
	abilityRuleLine  = regexp.MustCompile(`^(can|cannot)\b\s*\(?\s*(.*?)\s*\)?$`)
	abilityBlock     = regexp.MustCompile(`(\bdo(\s*\|[^|]*\|)?|\{\s*\|[^|]*\|.*\})$`)
	rubyBareCall     = regexp.MustCompile(`^[a-z_]\w*$`)
	rubyConstant     = regexp.MustCompile(`^[A-Z]\w*(::[A-Z]\w*)*$`)
	rubyIgnored      = regexp.MustCompile(`^(include\s|extend\s|private$|protected$|public$|\w+\s*\|\|=)`)
	// rubyMentionsRule is a statement that names can or cannot without starting with it, such as
	// `return can ...` or `x = can ...`; it could grant or revoke, so it is unparsed.
	rubyMentionsRule = regexp.MustCompile(`\b(can|cannot)\b`)
	rubyKeyword      = regexp.MustCompile(`^(return|next|break|redo|retry|super|yield|raise|fail)$`)
)

// parseAbility reads the Ability class. Statements it cannot read are recorded per rule set; a
// nested block is skipped whole and recorded as unparsed, so its rules never grant.
func parseAbility(path, text string) abilityModel {
	model := abilityModel{path: path, roles: map[string]*abilityRole{}}
	lines := rubyLines(text)
	class := -1
	for i, l := range lines {
		if abilityClass.MatchString(l.text) {
			class = i
			break
		}
	}
	if class < 0 {
		return model
	}
	classEnd := rubyBlockEnd(lines, class)
	for i := class + 1; i < min(classEnd, len(lines)); i++ {
		m := rubyDef.FindStringSubmatch(lines[i].text)
		if m == nil {
			if rubyOpens(lines[i].text) {
				i = rubyBlockEnd(lines, i)
			}
			continue
		}
		end := min(rubyBlockEnd(lines, i), len(lines))
		target := &abilityRole{line: lines[i].line}
		if m[2] == "initialize" {
			target = &model.global
		} else if m[1] == "" && m[3] == "" {
			model.roles[m[2]] = target
		}
		for j := i + 1; j < end; j++ {
			l := lines[j]
			switch {
			case target == &model.global && abilityDispatch.MatchString(l.text):
				model.dispatch = true
			case abilityRuleStart.MatchString(l.text):
				readRule(target, l)
				if rubyOpens(l.text) {
					j = rubyBlockEnd(lines, j)
				}
			case rubyOpens(l.text):
				target.unparsed = append(target.unparsed, l.line)
				j = rubyBlockEnd(lines, j)
			case rubyMentionsRule.MatchString(l.text) || rubyKeyword.MatchString(l.text):
				target.unparsed = append(target.unparsed, l.line)
			case rubyBareCall.MatchString(l.text) && target != &model.global:
				target.inherits = append(target.inherits, l.text)
			case rubyIgnored.MatchString(l.text):
			default:
				target.unparsed = append(target.unparsed, l.line)
			}
		}
		i = end
	}
	return model
}

// readRule records one literal `can`/`cannot` statement; extra arguments or a block make it
// conditional, and a non-literal action or subject makes it unparsed.
func readRule(target *abilityRole, l rubyLine) {
	text := l.text
	conditional := abilityBlock.MatchString(text)
	if conditional {
		text = strings.TrimSpace(abilityBlock.ReplaceAllString(text, ""))
	}
	m := abilityRuleLine.FindStringSubmatch(text)
	if m == nil {
		target.unparsed = append(target.unparsed, l.line)
		return
	}
	args := splitRuby(m[2])
	if len(args) < 2 {
		target.unparsed = append(target.unparsed, l.line)
		return
	}
	actions, okActions := rubyList(args[0], func(s string) (string, bool) {
		if v := rubySymbol.FindStringSubmatch(s); v != nil {
			return v[1], true
		}
		return "", false
	})
	subjects, okSubjects := rubyList(args[1], func(s string) (string, bool) {
		if s == ":all" {
			return "all", true
		}
		return s, rubyConstant.MatchString(s)
	})
	if !okActions || !okSubjects {
		target.unparsed = append(target.unparsed, l.line)
		return
	}
	target.rules = append(target.rules, abilityRule{actions: actions, subjects: subjects, cannot: m[1] == "cannot",
		conditional: conditional || len(args) > 2, line: l.line})
}

// splitRuby splits arguments at top-level commas.
func splitRuby(text string) []string {
	out, depth, start := []string{}, 0, 0
	for i, c := range text {
		switch c {
		case '[', '(', '{':
			depth++
		case ']', ')', '}':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(text[start:i]))
				start = i + 1
			}
		}
	}
	return append(out, strings.TrimSpace(text[start:]))
}

// rubyList reads one literal item or a `[a, b]` list of them.
func rubyList(text string, read func(string) (string, bool)) ([]string, bool) {
	items := []string{text}
	if strings.HasPrefix(text, "[") && strings.HasSuffix(text, "]") {
		items = splitRuby(text[1 : len(text)-1])
	}
	out := []string{}
	for _, item := range items {
		v, ok := read(strings.TrimSpace(item))
		if !ok {
			return nil, false
		}
		out = append(out, v)
	}
	return out, len(out) > 0
}

// actionAliases are CanCanCan's default aliases: a rule on the key also grants the listed actions.
var actionAliases = map[string][]string{"read": {"index", "show"}, "create": {"new"}, "update": {"edit"}}

func (rule abilityRule) matches(action, subject string) bool {
	actionOK, subjectOK := false, false
	for _, a := range rule.actions {
		if a == action || a == "manage" {
			actionOK = true
		}
		for _, alias := range actionAliases[a] {
			actionOK = actionOK || alias == action
		}
	}
	for _, s := range rule.subjects {
		subjectOK = subjectOK || s == "all" || s == subject
	}
	return actionOK && subjectOK
}

// grant is one role's evaluation: granted with the rule that grants, denied, or unknown with a
// reason (TSA-V0-035, TSA-V0-038).
type grant struct {
	state  string // "granted", "denied" or "unknown"
	rule   GrantRef
	reason string
	detail string
}

// closure lists role and every rule set it inherits, depth first in declaration order. It reports
// an undefined role or an inheritance cycle.
func (m abilityModel) closure(role string) ([]string, string, string) {
	order, state := []string{}, map[string]int{}
	var walk func(string) (string, string)
	walk = func(name string) (string, string) {
		if state[name] == 1 {
			return "inheritance-cycle", m.path + " rule set " + name + " inherits itself"
		}
		if state[name] == 2 {
			return "", ""
		}
		r, ok := m.roles[name]
		if !ok {
			return "role-undefined", m.path + " defines no rule set " + name
		}
		state[name] = 1
		order = append(order, name)
		for _, parent := range r.inherits {
			if reason, detail := walk(parent); reason != "" {
				return reason, detail
			}
		}
		state[name] = 2
		return "", ""
	}
	reason, detail := walk(role)
	return order, reason, detail
}

// evaluate decides whether role holds action on subject.
func (m abilityModel) evaluate(role, action, subject string) grant {
	if !m.dispatch {
		return grant{state: "unknown", reason: "role-dispatch-not-literal", detail: m.path + " initialize does not call send(user.role)"}
	}
	order, reason, detail := m.closure(role)
	if reason != "" {
		return grant{state: "unknown", reason: reason, detail: detail}
	}
	sets := []struct {
		name string
		r    *abilityRole
	}{}
	for _, name := range order {
		sets = append(sets, struct {
			name string
			r    *abilityRole
		}{name, m.roles[name]})
	}
	sets = append(sets, struct {
		name string
		r    *abilityRole
	}{"initialize", &m.global})
	var granted *GrantRef
	conditional, unparsed := 0, 0
	for _, set := range sets {
		for _, rule := range set.r.rules {
			if !rule.matches(action, subject) {
				continue
			}
			switch {
			case rule.cannot:
				return grant{state: "unknown", reason: "rule-cannot", detail: locate(m.path, rule.line)}
			case rule.conditional:
				if conditional == 0 {
					conditional = rule.line
				}
			case granted == nil:
				granted = &GrantRef{Role: set.name, Path: m.path, Line: rule.line}
			}
		}
		if len(set.r.unparsed) > 0 && unparsed == 0 {
			unparsed = set.r.unparsed[0]
		}
	}
	switch {
	case granted != nil && unparsed > 0:
		// An unread statement in the evaluated rule sets could revoke the grant (a conditional
		// `cannot`, a guard), so the grant is not concluded.
		return grant{state: "unknown", reason: "statement-unparsed", detail: locate(m.path, unparsed)}
	case granted != nil:
		return grant{state: "granted", rule: *granted}
	case conditional > 0:
		return grant{state: "unknown", reason: "rule-conditional", detail: locate(m.path, conditional)}
	case unparsed > 0:
		return grant{state: "unknown", reason: "statement-unparsed", detail: locate(m.path, unparsed)}
	}
	return grant{state: "denied"}
}
