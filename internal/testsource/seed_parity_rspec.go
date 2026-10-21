package testsource

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// The RSpec side of the seed-parity reading (TSA-V0-101): FactoryBot create, create_list, build
// and build_list calls and Rails YAML fixtures declared by `fixtures`, read in the example body,
// the before/let!/subject! hooks of its groups, and the lazy lets and subjects that scope invokes.

var (
	rspecFixturesLine = regexp.MustCompile(`^fixtures\b`)
	rspecSeedCall     = regexp.MustCompile(`(?:\bFactoryBot\.)?\b(create_list|build_list|create|build)\b`)
	rubyWord          = regexp.MustCompile(`[A-Za-z_]\w*[!?]?`)
	rspecAssertLine   = regexp.MustCompile(`\b(?:expect|is_expected|should|assert\w*|refute\w*)\b`)
	rspecSubjectUse   = regexp.MustCompile(`\bis_expected\b|(?:^|[\s{])should\b`)
	rubyAssignTail    = regexp.MustCompile(`(?:^|[\s;{(])@?([A-Za-z_]\w*)\s*=\s*$`)
	rubySeedSymbol    = regexp.MustCompile(`^:([A-Za-z_]\w*[?!]?)$`)
	rubyQuotedName    = regexp.MustCompile(`^(?:"([A-Za-z_]\w*)"|'([A-Za-z_]\w*)')$`)
	rubyKeyValue      = regexp.MustCompile(`^([A-Za-z_]\w*[?!]?):(\s+\S.*|[^:\s].*)$`)
	rubyRocket        = regexp.MustCompile(`^(?::([A-Za-z_]\w*[?!]?)|"([A-Za-z_]\w*)"|'([A-Za-z_]\w*)')\s*=>\s*(\S.*)$`)
	seedNumber        = regexp.MustCompile(`^-?\d+(?:\.\d+)?$`)
	// rubyConditional matches a line whose calls may not all run.
	rubyConditional = regexp.MustCompile(`\b(?:if|unless|while|until|rescue|and|or|then|else|elsif|when|case|return|next|break)\b|&&|\|\||\s\?\s`)
	yamlKey         = regexp.MustCompile(`^([A-Za-z_][\w.-]*|"[^"]*"|'[^']*'):(?:\s+(.*))?$`)
)

// rspecLet is one let, let!, subject or subject! declaration; name is "subject" for an anonymous
// subject.
type rspecLet struct {
	name string
	lazy bool
	body []rubyLine
}

// rspecSeed extracts the setup of the example declared on line (TSA-V0-101).
func rspecSeed(name, text string, line int, read fixtureReader) (seedExtract, error) {
	at := func(n int) string { return fmt.Sprintf("%s:%d", name, n) }
	x := seedExtract{test: at(line)}
	r := &rspecReader{lines: rubyLines(text), groups: []rspecGroup{{parent: -1, decls: map[string]string{}}}}
	if r.read() != "" {
		x.abstain(seedReasonSourceUncertain, x.test)
		return x, nil
	}
	found := -1
	for k, e := range r.examples {
		if e.line == line {
			found = k
		}
	}
	if found < 0 {
		x.abstain(seedReasonExampleNotFound, x.test)
		return x, nil
	}
	e := r.examples[found]
	chain := []int{}
	for g := e.group; g >= 0; g = r.groups[g].parent {
		chain = append(chain, g)
	}
	// scope maps each logical line that runs for the example to the let it declares, or "";
	// depth holds the block nesting at the line's start (TSA-V0-101).
	scope, depth := map[int]string{}, map[int]int{}
	add := func(body []rubyLine, own string) {
		depths := rubyBodyDepths(body)
		for _, l := range body {
			if _, ok := scope[l.line]; !ok {
				scope[l.line], depth[l.line] = own, depths[l.line]
			}
		}
	}
	// resolve finds the definition a name runs: the innermost let, or the named subject that
	// `subject` aliases.
	resolve := func(n string) (rspecLet, bool) {
		if c := r.canonical(e.group, n); strings.HasPrefix(c, "let:") {
			n = strings.TrimPrefix(c, "let:")
		}
		return r.innermostLet(chain, n)
	}
	invoked := map[string]bool{}
	queue := []string{}
	// calls holds, per bare name a non-asserting setup line invokes, the first such line
	// (TSA-V0-644).
	calls := map[string]string{}
	visit := func(body []rubyLine, params map[string]bool) {
		for n := range invokedNames(body) {
			if !invoked[n] {
				invoked[n] = true
				queue = append(queue, n)
			}
		}
		setup := []rubyLine{}
		for _, l := range body {
			if !rspecAssertLine.MatchString(maskRubyStrings(l.text)) {
				setup = append(setup, l)
			}
		}
		names := invokedNames(setup)
		for _, l := range setup {
			for _, n := range rubyStatementHeads(maskRubyStrings(l.text)) {
				if _, ok := calls[n]; !ok && names[n] && !params[n] {
					calls[n] = at(l.line)
				}
			}
		}
		for _, l := range body {
			if rspecSubjectUse.MatchString(maskRubyStrings(l.text)) && !invoked["subject"] {
				invoked["subject"] = true
				queue = append(queue, "subject")
			}
		}
	}
	add(e.body, "")
	visit(e.body, nil)
	for _, g := range chain {
		for _, h := range r.groups[g].hooks {
			body, own := h.body, ""
			if h.kind != "before" {
				// let!(:x) runs `before { x }`, and x is the innermost definition: an inner let
				// overriding an outer let! replaces its body.
				own = h.own
				if let, ok := resolve(h.own); ok {
					body, own = let.body, let.name
				}
			}
			add(body, own)
			visit(body, nil)
		}
	}
	helpers := r.seedHelpers(chain)
	followed := []string{}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		if let, ok := resolve(n); ok {
			if let.lazy {
				add(let.body, let.name)
				visit(let.body, nil)
			}
			continue
		}
		// A first-party helper the setup calls is followed into its factory calls; its body
		// nests one level under the def (TSA-V0-644).
		if h, ok := helpers[n]; ok {
			followed = append(followed, n)
			depths := rubyBodyDepths(h.body)
			for _, l := range h.body {
				if _, ok := scope[l.line]; !ok {
					scope[l.line], depth[l.line] = "", depths[l.line]+1
				}
			}
			visit(h.body, helperParams(h.body))
		}
	}
	// A helper is followed only when it runs exactly once, unconditionally, for the example; a
	// name the setup calls that is neither a let, a followed helper nor known RSpec, Rails or
	// Capybara vocabulary may seed records the reading cannot see, so the side abstains naming
	// it (TSA-V0-644).
	for _, n := range followed {
		if site, ok := r.helperSite(scope, depth, n, helpers[n], at); !ok {
			x.abstainHelper(n, site)
		}
	}
	unresolved := []string{}
	for n := range calls {
		if _, isHelper := helpers[n]; isHelper || seedCallVocabulary[n] || seedLike(n) || !(n[0] >= 'a' && n[0] <= 'z' || n[0] == '_') {
			continue
		}
		if _, ok := resolve(n); ok || r.canonical(e.group, n) != "" {
			continue
		}
		unresolved = append(unresolved, n)
	}
	slices.Sort(unresolved)
	for _, n := range unresolved {
		x.abstainHelper(n, calls[n])
	}
	fixtures := map[int]bool{}
	for _, g := range chain {
		for _, l := range r.groups[g].fixtures {
			fixtures[l.line] = true
		}
	}
	// ignored holds the lines that never run for the example.
	ignored := map[int]bool{}
	ignore := func(body []rubyLine) {
		for _, l := range body {
			ignored[l.line] = true
		}
	}
	for k, other := range r.examples {
		if k != found {
			ignore(other.body)
		}
	}
	for _, g := range r.groups {
		ignored[g.line] = true // a group's declaring line: `describe CreateOrder do` names, never seeds
		for _, h := range g.hooks {
			ignore(h.body)
		}
		for _, let := range g.lets {
			ignore(let.body)
		}
		ignore(g.fixtures)
	}
	for _, h := range r.helpers {
		if !invoked[helperName(h.body)] {
			ignore(h.body)
		}
	}
	tables := 0
	for _, l := range r.lines {
		masked := maskRubyStrings(l.text)
		words := seedWordSpans(masked)
		if len(words) == 0 {
			continue
		}
		own, inScope := scope[l.line]
		switch {
		case fixtures[l.line]:
			calls, reason, err := fixtureSeeds(l.text, at(l.line), read, &tables)
			if err != nil {
				return seedExtract{}, err
			}
			if reason != "" {
				x.abstain(reason, at(l.line))
				continue
			}
			x.calls = append(x.calls, calls...)
		case inScope:
			calls, reason := rubySeedCalls(l.text, masked, words, at(l.line), own, depth[l.line])
			if reason != "" {
				x.abstain(reason, at(l.line))
				continue
			}
			x.calls = append(x.calls, calls...)
		case !ignored[l.line]:
			x.abstain(seedReasonScopeUnresolved, at(l.line))
		}
	}
	for _, l := range e.body {
		masked := maskRubyStrings(l.text)
		if !rspecAssertLine.MatchString(masked) {
			continue
		}
		a := seedAssert{at: at(l.line), idents: map[string]bool{}}
		for _, w := range rubyWord.FindAllString(masked, -1) {
			a.idents[snake(w)] = true
		}
		x.asserts = append(x.asserts, a)
	}
	return x, nil
}

// rubyBodyDepths maps each line of a block body (head line first) to the nesting at its start
// relative to the block's own body: 0 runs with the block, more is nested in an inner block,
// lambda or keyword construct. A brace-delimited block starts at -1 so its own `{` reaches 0. A
// body whose keyword nesting cannot be followed marks every line nested.
func rubyBodyDepths(body []rubyLine) map[int]int {
	out := map[int]int{}
	if len(body) == 0 {
		return out
	}
	brace, kw := 0, 0
	if !rubyDo.MatchString(body[0].text) {
		brace = -1
	}
	for k, l := range body {
		if k > 0 && rubyEnd.MatchString(l.text) {
			kw--
		}
		if kw < 0 && k < len(body)-1 {
			for _, l := range body {
				out[l.line] = 1
			}
			return out
		}
		out[l.line] = kw + brace
		if k > 0 && rubyOpens(l.text) {
			kw++
		}
		m := maskRubyStrings(l.text)
		brace += strings.Count(m, "{") - strings.Count(m, "}")
	}
	return out
}

// innermostLet is the let named n declared by the nearest group of chain (innermost first).
func (r *rspecReader) innermostLet(chain []int, n string) (rspecLet, bool) {
	for _, g := range chain {
		lets := r.groups[g].lets
		for k := len(lets) - 1; k >= 0; k-- {
			if lets[k].name == n {
				return lets[k], true
			}
		}
	}
	return rspecLet{}, false
}

// maskRubyStrings blanks the contents of quoted strings, keeping every byte offset.
func maskRubyStrings(text string) string {
	b := []byte(text)
	quote := byte(0)
	for i := 0; i < len(b); i++ {
		c := b[i]
		switch {
		case quote != 0 && c == '\\':
			b[i] = ' '
			if i+1 < len(b) {
				b[i+1] = ' '
				i++
			}
		case quote != 0 && c == quote:
			quote = 0
		case quote != 0:
			b[i] = ' '
		case c == '\'' || c == '"':
			quote = c
		}
	}
	return string(b)
}

// seedWordSpans returns the [start, end) spans of the seed-like words of a masked line that are
// not symbols (`:create`) or hash keys (`create:`).
func seedWordSpans(masked string) [][2]int {
	out := [][2]int{}
	for _, loc := range rubyWord.FindAllStringIndex(masked, -1) {
		if loc[0] > 0 && masked[loc[0]-1] == ':' && (loc[0] < 2 || masked[loc[0]-2] != ':') {
			continue
		}
		rest := masked[loc[1]:]
		if strings.HasPrefix(rest, ":") && !strings.HasPrefix(rest, "::") {
			continue
		}
		if seedLike(masked[loc[0]:loc[1]]) {
			out = append(out, [2]int{loc[0], loc[1]})
		}
	}
	return out
}

// rubySeedCalls reads every FactoryBot call on one logical line. Any other seed-like word on the
// line abstains as an unsupported form.
func rubySeedCalls(text, masked string, words [][2]int, at, own string, depth int) ([]seedCall, string) {
	calls := []seedCall{}
	consumed := [][2]int{}
	for _, m := range rspecSeedCall.FindAllStringSubmatchIndex(masked, -1) {
		start, nameEnd := m[0], m[3]
		if start > 0 && strings.ContainsRune(".:@$", rune(masked[start-1])) {
			continue
		}
		if nameEnd < len(masked) && (masked[nameEnd] == '!' || masked[nameEnd] == '?') {
			continue
		}
		args, reason := rubyCallArgs(masked, nameEnd)
		if reason != "" {
			return nil, reason
		}
		if args[0] < 0 {
			continue // a bare word such as a local named build: not a call this reader reads
		}
		// A call nested in a block, a lambda or a condition may never run (TSA-V0-101).
		if depth+strings.Count(masked[:start], "{")-strings.Count(masked[:start], "}") != 0 || rubyConditional.MatchString(masked) {
			return nil, seedReasonExecutionUnresolved
		}
		consumed = append(consumed, [2]int{start, nameEnd})
		call, reason := rubySeedCall(masked[m[2]:m[3]], text, masked, args, at)
		if reason != "" {
			return nil, reason
		}
		call.bound = boundNames(own, text[:start])
		calls = append(calls, call)
	}
	for _, w := range words {
		ok := false
		for _, c := range consumed {
			ok = ok || w[0] >= c[0] && w[1] <= c[1]
		}
		if !ok {
			return nil, seedReasonFormUnsupported
		}
	}
	return calls, ""
}

// rubyCallArgs returns the [start, end) range of a call's arguments after the name ending at p,
// or [-1, -1] when the word is not followed by arguments. A call with a trailing block abstains.
func rubyCallArgs(masked string, p int) ([2]int, string) {
	if p < len(masked) && masked[p] == '(' {
		depth := 0
		for i := p; i < len(masked); i++ {
			switch masked[i] {
			case '(', '[', '{':
				depth++
			case ')', ']', '}':
				depth--
			}
			if depth == 0 {
				rest := strings.TrimLeft(masked[i+1:], " ")
				if strings.HasPrefix(rest, "{") || rest == "do" || strings.HasPrefix(rest, "do ") || strings.HasPrefix(rest, "do|") {
					return [2]int{}, seedReasonFormUnsupported
				}
				return [2]int{p + 1, i}, ""
			}
		}
		return [2]int{}, seedReasonFormUnsupported
	}
	rest := strings.TrimLeft(masked[p:], " ")
	if !strings.HasPrefix(rest, ":") && !strings.HasPrefix(rest, "'") && !strings.HasPrefix(rest, "\"") {
		return [2]int{-1, -1}, ""
	}
	start := len(masked) - len(rest)
	depth := 0
	for i := start; i < len(masked); i++ {
		switch masked[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth == 0 {
				return [2]int{start, i}, ""
			}
			depth--
		case ';':
			if depth == 0 {
				return [2]int{start, i}, ""
			}
		}
		if depth == 0 && strings.HasPrefix(masked[i:], " do") && (i+3 == len(masked) || !isWordByte(masked[i+3])) {
			return [2]int{}, seedReasonFormUnsupported
		}
	}
	return [2]int{start, len(masked)}, ""
}

func isWordByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// splitTopLevel splits [start, end) of a masked line at top-level commas.
func splitTopLevel(masked string, start, end int) [][2]int {
	out := [][2]int{}
	depth, from := 0, start
	for i := start; i < end; i++ {
		switch masked[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, [2]int{from, i})
				from = i + 1
			}
		}
	}
	if strings.TrimSpace(masked[from:end]) != "" {
		out = append(out, [2]int{from, end})
	}
	return out
}

// rubySeedCall reads one FactoryBot call's kind, count and attributes.
func rubySeedCall(method, text, masked string, args [2]int, at string) (seedCall, string) {
	parts := splitTopLevel(masked, args[0], args[1])
	arg := func(k int) string { return strings.TrimSpace(text[parts[k][0]:parts[k][1]]) }
	if len(parts) == 0 {
		return seedCall{}, seedReasonKindNotLiteral
	}
	kind, ok := rubyName(arg(0))
	if !ok {
		return seedCall{}, seedReasonKindNotLiteral
	}
	call := seedCall{kind: snake(kind), count: 1, attrs: map[string]string{}, at: at}
	next := 1
	if strings.HasSuffix(method, "_list") {
		if len(parts) < 2 {
			return seedCall{}, seedReasonCountNotLiteral
		}
		n, err := strconv.Atoi(arg(1))
		if err != nil || n < 0 {
			return seedCall{}, seedReasonCountNotLiteral
		}
		call.count, next = n, 2
	}
	for k := next; k < len(parts); k++ {
		a := arg(k)
		if m := rubySeedSymbol.FindStringSubmatch(a); m != nil {
			call.attrs["trait:"+snake(m[1])] = "true"
			continue
		}
		if m := rubyKeyValue.FindStringSubmatch(a); m != nil {
			call.attrs[snake(m[1])] = rubyValue(m[2])
			continue
		}
		if m := rubyRocket.FindStringSubmatch(a); m != nil {
			call.attrs[snake(m[1]+m[2]+m[3])] = rubyValue(m[4])
			continue
		}
		return seedCall{}, seedReasonAttributesNotRead
	}
	return call, ""
}

// rubyName reads a literal symbol or quoted word.
func rubyName(a string) (string, bool) {
	if m := rubySeedSymbol.FindStringSubmatch(a); m != nil {
		return m[1], true
	}
	if m := rubyQuotedName.FindStringSubmatch(a); m != nil {
		return m[1] + m[2], true
	}
	return "", false
}

// rubyValue normalises a Ruby literal: a symbol or string is a quoted string, nil is null, a
// number or boolean is itself; anything else is <expr>.
func rubyValue(v string) string {
	v = strings.TrimSpace(v)
	switch {
	case rubySeedSymbol.MatchString(v):
		return strconv.Quote(v[1:])
	case len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'' && !strings.ContainsAny(v[1:len(v)-1], `'\`):
		return strconv.Quote(v[1 : len(v)-1])
	case len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' && !strings.ContainsAny(v[1:len(v)-1], `"\#`):
		return strconv.Quote(v[1 : len(v)-1])
	case seedNumber.MatchString(v), v == "true", v == "false":
		return v
	case v == "nil":
		return "null"
	}
	return seedExpr
}

// boundNames is the let the call is in, else the local or instance variable it is assigned to.
func boundNames(own, before string) []string {
	if own != "" {
		return []string{snake(own)}
	}
	if m := rubyAssignTail.FindStringSubmatch(maskRubyStrings(before)); m != nil {
		return []string{snake(m[1])}
	}
	return nil
}

// fixtureSeeds reads a `fixtures :a, :b` line: one call per record of each named table's
// committed YAML file, spec/fixtures/ first, then test/fixtures/.
func fixtureSeeds(text, at string, read fixtureReader, tables *int) ([]seedCall, string, error) {
	masked := maskRubyStrings(text)
	start, end := len("fixtures"), len(masked)
	if rest := strings.TrimSpace(masked[start:]); strings.HasPrefix(rest, "(") && strings.HasSuffix(rest, ")") {
		start, end = strings.IndexByte(masked, '(')+1, strings.LastIndexByte(masked, ')')
	}
	out := []seedCall{}
	parts := splitTopLevel(masked, start, end)
	for _, part := range parts {
		table, ok := rubyName(strings.TrimSpace(text[part[0]:part[1]]))
		if !ok || table == "all" {
			return nil, seedReasonFixtureUnsupported, nil
		}
		if *tables++; *tables > maxSeedFixtures {
			return nil, seedReasonFixtureBoundReached, nil
		}
		var data []byte
		found := false
		for _, dir := range []string{"spec/fixtures/", "test/fixtures/"} {
			var err error
			data, found, err = read(dir + table + ".yml")
			switch {
			case errors.Is(err, errSeedFixtureTooLarge):
				return nil, seedReasonFixtureBoundReached, nil
			case errors.Is(err, errSeedFixtureUnreadable):
				return nil, seedReasonFixtureUnavailable, nil
			case err != nil:
				return nil, "", err
			}
			if found {
				break
			}
		}
		if !found {
			return nil, seedReasonFixtureUnavailable, nil
		}
		records, ok := fixtureRecords(string(data))
		if !ok {
			return nil, seedReasonFixtureUnsupported, nil
		}
		for _, attrs := range records {
			out = append(out, seedCall{kind: seedSingular(snake(table)), count: 1, attrs: attrs, at: at, bound: []string{snake(table)}})
		}
	}
	if len(parts) == 0 {
		return nil, seedReasonFixtureUnsupported, nil
	}
	return out, "", nil
}

// fixtureRecords reads a Rails fixture file of the plain shape: top-level record labels, each
// with one level of `key: value` attributes. ERB, tabs, anchors, aliases, merge keys, lists and
// flow collections at the record level are not read.
func fixtureRecords(text string) ([]map[string]string, bool) {
	records := []map[string]string{}
	var current map[string]string
	indent, nested := -1, false
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimRight(raw, " \r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || line == "---" {
			continue
		}
		if strings.Contains(line, "<%") || strings.Contains(line, "\t") {
			return nil, false
		}
		lead := len(line) - len(strings.TrimLeft(line, " "))
		m := yamlKey.FindStringSubmatch(trimmed)
		if lead == 0 {
			if m == nil || strings.TrimSpace(m[2]) != "" {
				return nil, false
			}
			current, indent, nested = map[string]string{}, -1, false
			records = append(records, current)
			continue
		}
		if current == nil {
			return nil, false
		}
		if indent < 0 {
			indent = lead
		}
		if lead > indent {
			if !nested {
				return nil, false
			}
			continue
		}
		if lead < indent || m == nil {
			return nil, false
		}
		key, value := strings.Trim(m[1], `"'`), yamlStripComment(strings.TrimSpace(m[2]))
		if key == "<<" || strings.HasPrefix(value, "&") || strings.HasPrefix(value, "*") {
			return nil, false
		}
		nested = value == "" || value == "|" || value == ">"
		current[snake(key)] = yamlValue(value)
	}
	return records, true
}

// yamlStripComment drops a ` #` comment, after a quoted scalar's closing quote too.
func yamlStripComment(v string) string {
	if v == "" {
		return v
	}
	if q := v[0]; q == '"' || q == '\'' {
		for i := 1; i < len(v); i++ {
			switch {
			case q == '"' && v[i] == '\\':
				i++
			case q == '\'' && v[i] == q && i+1 < len(v) && v[i+1] == q:
				i++
			case v[i] == q:
				if rest := v[i+1:]; strings.HasPrefix(strings.TrimLeft(rest, " "), "#") && strings.HasPrefix(rest, " ") {
					return v[:i+1]
				}
				return v
			}
		}
		return v
	}
	if i := strings.Index(v, " #"); i >= 0 {
		return strings.TrimSpace(v[:i])
	}
	return v
}

// yamlValue normalises a YAML scalar like rubyValue; a plain scalar is a string.
func yamlValue(v string) string {
	switch {
	case v == "" || v == "|" || v == ">" || strings.HasPrefix(v, "[") || strings.HasPrefix(v, "{"):
		return seedExpr
	case len(v) >= 2 && (v[0] == '"' && v[len(v)-1] == '"' || v[0] == '\'' && v[len(v)-1] == '\''):
		inner := v[1 : len(v)-1]
		if strings.ContainsAny(inner, `\"'`) {
			return seedExpr
		}
		return strconv.Quote(inner)
	case seedNumber.MatchString(v), v == "true", v == "false":
		return v
	case v == "null" || v == "~":
		return "null"
	}
	return strconv.Quote(v)
}

// seedHelpers maps each def an example of chain can call to its body: a def of an enclosing
// group, innermost first, or at file level (TSA-V0-644).
func (r *rspecReader) seedHelpers(chain []int) map[string]rspecHelper {
	out := map[string]rspecHelper{}
	for _, g := range append(append([]int{}, chain...), 0) {
		for _, h := range r.helpers {
			if h.group != g || len(h.body) == 0 {
				continue
			}
			if n := helperName(h.body); n != "" {
				if _, ok := out[n]; !ok {
					out[n] = h
				}
			}
		}
	}
	return out
}

// helperSite finds the one line that calls helper n among the lines that run for the example,
// outside the helper's own body. It reports false, with the first calling line, when the helper
// is called more than once, from a nested block or a conditional line, or not found.
func (r *rspecReader) helperSite(scope map[int]string, depth map[int]int, n string, h rspecHelper, at func(int) string) (string, bool) {
	own := map[int]bool{}
	for _, l := range h.body {
		own[l.line] = true
	}
	word := regexp.MustCompile(`(^|[^\w.:@$])` + regexp.QuoteMeta(n) + `($|[^\w?!:])`)
	site, count, plain := "", 0, true
	for _, l := range r.lines {
		if _, ok := scope[l.line]; !ok || own[l.line] {
			continue
		}
		masked := maskRubyStrings(l.text)
		for _, loc := range word.FindAllStringIndex(masked, -1) {
			if site == "" {
				site = at(l.line)
			}
			count++
			if depth[l.line]+strings.Count(masked[:loc[0]], "{")-strings.Count(masked[:loc[0]], "}") != 0 || rubyConditional.MatchString(masked) {
				plain = false
			}
		}
	}
	return site, count == 1 && plain
}

// rubyStatementHeads returns the word each statement of a masked line starts with: at the line
// start or after `;`, `{` or `do`, past block parameters and one `name =` assignment. An argument
// or attribute value is not a head: the reading already treats it as a value it does not know.
func rubyStatementHeads(masked string) []string {
	out := []string{}
	starts := []int{0}
	for i := 0; i < len(masked); i++ {
		switch {
		case masked[i] == ';' || masked[i] == '{':
			starts = append(starts, i+1)
		case strings.HasPrefix(masked[i:], "do") && (i == 0 || !isWordByte(masked[i-1])) && (i+2 == len(masked) || !isWordByte(masked[i+2])):
			starts = append(starts, i+2)
		}
	}
	for _, p := range starts {
		rest := strings.TrimLeft(masked[p:], " \t")
		if strings.HasPrefix(rest, "|") {
			if k := strings.IndexByte(rest[1:], '|'); k >= 0 {
				rest = strings.TrimLeft(rest[k+2:], " \t")
			}
		}
		if m := rubyHeadAssign.FindStringIndex(rest); m != nil {
			rest = rest[m[1]:]
		}
		if w := rubyWord.FindStringIndex(rest); w != nil && w[0] == 0 {
			out = append(out, rest[:w[1]])
		}
	}
	return out
}

// rubyHeadAssign matches a leading `name =` or `@name =` assignment, not `==`, `=~` or `=>`.
var rubyHeadAssign = regexp.MustCompile(`^@?[A-Za-z_]\w*\s*=(?:[^=~>]|$)\s*`)

// seedCallVocabulary are the bare RSpec, FactoryBot, Rails and Capybara names a setup line calls
// that never seed records the reading misses: the reading follows lets, subjects and helpers and
// reads factory calls itself (TSA-V0-644).
var seedCallVocabulary = map[string]bool{
	"let": true, "let!": true, "subject": true, "subject!": true, "before": true, "after": true, "around": true,
	"it": true, "specify": true, "example": true, "scenario": true, "describe": true, "context": true, "feature": true,
	"fixtures": true, "described_class": true, "is_expected": true, "expect": true, "allow": true,
	"allow_any_instance_of": true, "expect_any_instance_of": true, "receive": true, "receive_messages": true,
	"double": true, "instance_double": true, "class_double": true, "object_double": true, "spy": true,
	"instance_spy": true, "anything": true, "any_args": true, "hash_including": true, "stub_const": true,
	"travel_to": true, "travel": true, "travel_back": true, "freeze_time": true, "sign_in": true, "sign_out": true,
	"login_as": true, "logout": true, "puts": true, "p": true, "pp": true, "print": true, "sleep": true,
	"raise": true, "require": true, "require_relative": true, "lambda": true, "proc": true, "loop": true,
	"aggregate_failures": true, "pending": true, "skip": true, "get": true, "post": true, "put": true,
	"patch": true, "delete": true, "head": true, "follow_redirect!": true, "render": true, "visit": true,
	"click_on": true, "click_button": true, "click_link": true, "fill_in": true, "select": true, "check": true,
	"uncheck": true, "choose": true, "within": true, "find": true, "perform_enqueued_jobs": true,
	"attributes_for": true, "build_stubbed": true, "build_stubbed_list": true, "sequence": true,
	"generate": true, "file_fixture": true, "fixture_file_upload": true, "rand": true, "format": true,
}

// helperParams are the parameter names a def declares, with the def's own name: the body reads
// them as locals, not as calls (TSA-V0-644).
func helperParams(body []rubyLine) map[string]bool {
	out := map[string]bool{helperName(body): true}
	head := stripRubyStrings(body[0].text)
	if rubyOneLine.MatchString(head) {
		head, _, _ = strings.Cut(head, ";")
	}
	m := rspecDefHead.FindStringSubmatch(strings.TrimSpace(head))
	if m == nil {
		return out
	}
	for _, p := range strings.Split(m[2]+m[3], ",") {
		p = strings.TrimLeft(strings.TrimSpace(p), "*&")
		if k := strings.IndexAny(p, "=:"); k >= 0 {
			p = p[:k]
		}
		if p = strings.TrimSpace(p); p != "" {
			out[p] = true
		}
	}
	return out
}
