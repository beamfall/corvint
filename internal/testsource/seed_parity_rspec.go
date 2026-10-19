package testsource

import (
	"fmt"
	"regexp"
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
	yamlKey           = regexp.MustCompile(`^([A-Za-z_][\w.-]*|"[^"]*"|'[^']*'):(?:\s+(.*))?$`)
)

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
	// scope maps each logical line that runs for the example to the let it declares, or "".
	scope := map[int]string{}
	add := func(body []rubyLine, own string) {
		for _, l := range body {
			if _, ok := scope[l.line]; !ok {
				scope[l.line] = own
			}
		}
	}
	invoked := map[string]bool{}
	queue := []string{}
	visit := func(body []rubyLine) {
		for n := range invokedNames(body) {
			if !invoked[n] {
				invoked[n] = true
				queue = append(queue, n)
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
	visit(e.body)
	for _, g := range chain {
		for _, h := range r.groups[g].hooks {
			own := ""
			if h.kind != "before" {
				own = h.own
			}
			add(h.body, own)
			visit(h.body)
		}
	}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		if let, ok := r.innermostLet(chain, n); ok && let.lazy {
			add(let.body, n)
			visit(let.body)
		}
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
			calls, reason := rubySeedCalls(l.text, masked, words, at(l.line), own)
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
func rubySeedCalls(text, masked string, words [][2]int, at, own string) ([]seedCall, string) {
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
			if data, found, err = read(dir + table + ".yml"); err != nil {
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

func yamlStripComment(v string) string {
	if v == "" || v[0] == '"' || v[0] == '\'' {
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
