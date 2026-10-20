package testsource

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// The RSpec readings of STUB-ON-PATH, STUB-NEVER-INVOKED and ASSERTED-VALUE-NOT-FROM-SUBJECT
// (TSA-V0-120..130). They trace values inside one example only: its body, the before, let! and
// subject! hooks on its group chain and the let and subject blocks it names. They never read the
// code under test, so every finding is advisory and an incomplete trace gives no finding.

const maxFlowDepth = 4

var (
	// rspecStubCall matches `allow(R).to receive(:m)`, `expect(R).to receive(:m)` and their
	// `_any_instance_of` forms; groups: 1 allow|expect, 2 _any_instance_of, 3 R, 4 m.
	rspecStubCall   = regexp.MustCompile(`\b(allow|expect)(_any_instance_of)?\s*\(\s*([A-Za-z_][\w:]*[?!]?)\s*\)\s*\.\s*to\s*\(?\s*receive\s*\(\s*:(\w+[?!]?)\s*\)`)
	rspecAndReturn  = regexp.MustCompile(`\.\s*and_return\s*\(`)
	rspecExpectArg  = regexp.MustCompile(`\bexpect\s*\(`)
	rspecIsExpected = regexp.MustCompile(`\bis_expected\s*\.\s*(?:to|not_to|to_not)\b`)
	// rspecMatcher reads `.to MATCHER` after an expect(...) or is_expected; group 1 is MATCHER.
	rspecMatcher   = regexp.MustCompile(`^\s*\.?\s*(?:to|not_to|to_not)\s*\(?\s*(\w+[?!]?)`)
	rspecWith      = regexp.MustCompile(`\.\s*with\s*\(`)
	rspecLocalSet  = regexp.MustCompile(`^([a-z_]\w*)\s*=\s*(?:[^=~>]|$)`)
	rspecBlockJob  = regexp.MustCompile(`\bexpect\s*\{`)
	rspecBlockJobM = regexp.MustCompile(`^\s*\.\s*to\s*\(?\s*have_enqueued_job\s*\(\s*([A-Z][\w:]*)\s*\)`)
	rspecConstPath = regexp.MustCompile(`^[A-Z][\w:]*$`)
	rspecNewCall   = regexp.MustCompile(`^([A-Z][\w:]*)\s*\.\s*new\b`)
	rspecVerified  = regexp.MustCompile(`^(?:instance_double|instance_spy|class_double)\s*\(?\s*['"]?([A-Z][\w:]*)`)
	rspecPlainDbl  = regexp.MustCompile(`^(?:double|spy)\b`)
	rspecBuilt     = regexp.MustCompile(`^(?:FactoryBot\s*\.\s*)?(?:build|build_stubbed)\b`)
	rspecCreated   = regexp.MustCompile(`^(?:FactoryBot\s*\.\s*)?create\b`)
	// rspecBuilder is a callee whose result the test builds from its arguments alone (TSA-V0-126).
	rspecBuilder = regexp.MustCompile(`^(?:(?:FactoryBot\s*\.\s*)?(?:create|build|build_stubbed|create_list|build_list|attributes_for)|double|instance_double|instance_spy|spy|Capybara\s*\.\s*string|Nokogiri\s*::\s*HTML5?(?:\s*\.\s*fragment)?|Nokogiri\s*::\s*XML|OpenStruct\s*\.\s*new|Struct\s*\.\s*new)\b`)
	// rspecPersisted is a read of persisted or queried state, which the subject can change.
	rspecPersisted = regexp.MustCompile(`\.\s*(?:reload|find\w*|where|pluck|exists\?|count|last|first|all|take|ids|sum|maximum|minimum)\b`)
	rspecSelfWords = regexp.MustCompile(`\b(?:self|super|yield)\b`)
	// rspecBoundaryConsts are the external-boundary roots: HTTP clients, third-party SDKs, the
	// clock, randomness, the process and file system, logging, queues and mailers (TSA-V0-123).
	rspecBoundaryConsts = map[string]bool{"Net": true, "HTTP": true, "HTTParty": true, "Faraday": true, "RestClient": true, "Excon": true,
		"Typhoeus": true, "Curl": true, "URI": true, "OpenURI": true, "Time": true, "Date": true, "DateTime": true, "Timecop": true,
		"Kernel": true, "SecureRandom": true, "Random": true, "Process": true, "File": true, "FileUtils": true, "Dir": true, "IO": true,
		"ENV": true, "Socket": true, "Resolv": true, "OpenSSL": true, "Logger": true, "Rails": true, "ActiveJob": true,
		"ActionMailer": true, "ActiveSupport": true, "Sidekiq": true, "Resque": true, "Redis": true, "Bunny": true, "Kafka": true,
		"Aws": true, "Google": true, "Stripe": true, "Twilio": true, "Octokit": true, "Slack": true, "Sentry": true, "Raven": true,
		"Honeybadger": true, "Bugsnag": true, "Faker": true, "Mail": true}
	// rspecQueueMethods are enqueue and delivery methods; a stub of one is a queue boundary.
	rspecQueueMethods = map[string]bool{"perform_later": true, "perform_async": true, "perform_in": true, "perform_at": true,
		"deliver_later": true, "deliver_now": true, "deliver": true, "enqueue": true, "publish": true, "broadcast": true}
	rspecEnqueueMethods = map[string]bool{"perform_later": true, "perform_async": true, "perform_in": true, "perform_at": true}
)

// rubyText is one logical line with its string and percent literals masked: masked has the same
// length as raw, with every character inside a literal replaced by \x01, so a pattern matched on
// masked never matches literal content and its positions slice raw. A `#{...}` interpolation in a
// double-quoted string stays code, so its names are read.
type rubyText struct {
	line        int
	raw, masked string
}

func maskRuby(l rubyLine) rubyText {
	b := []byte(l.text)
	for _, loc := range rspecPercent.FindAllStringIndex(l.text, -1) {
		for i := loc[0] + 1; i < loc[1]-1; i++ {
			b[i] = 1
		}
		b[loc[0]], b[loc[1]-1] = '"', '"'
	}
	// stack holds the open strings and interpolations: a quote byte, or 0 for an interpolation
	// whose brace depth is in depth.
	stack, depth := []byte{}, []int{}
	for i := 0; i < len(b); i++ {
		c := b[i]
		quote := byte(0)
		if len(stack) > 0 {
			quote = stack[len(stack)-1]
		}
		switch {
		case quote != 0 && c == '\\':
			b[i] = 1
			if i+1 < len(b) {
				i++
				b[i] = 1
			}
		case quote != 0 && c == quote:
			stack = stack[:len(stack)-1]
		case quote == '"' && c == '#' && i+1 < len(b) && b[i+1] == '{':
			i++
			stack, depth = append(stack, 0), append(depth, 1)
		case quote != 0:
			b[i] = 1
		case c == '\'' || c == '"':
			stack = append(stack, c)
		case len(stack) > 0 && c == '{':
			depth[len(depth)-1]++
		case len(stack) > 0 && c == '}':
			if depth[len(depth)-1]--; depth[len(depth)-1] == 0 {
				stack, depth = stack[:len(stack)-1], depth[:len(depth)-1]
			}
		}
	}
	return rubyText{line: l.line, raw: l.text, masked: string(b)}
}

// balanced returns the index of the closer matching the opener at open in masked, or -1.
func balanced(masked string, open int) int {
	depth := 0
	for i := open; i < len(masked); i++ {
		switch masked[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// squash drops the whitespace outside literals, so formatting does not change a comparison.
func (t rubyText) squash(from, to int) string {
	var b strings.Builder
	for i := from; i < to; i++ {
		if t.masked[i] == ' ' || t.masked[i] == '\t' {
			continue
		}
		b.WriteByte(t.raw[i])
	}
	return b.String()
}

// rubyWords returns the bare identifiers of masked text: not a method on a receiver, a symbol,
// an instance or global variable, a hash key or a keyword. Constants are kept.
func rubyWords(masked string) []string {
	out := []string{}
	for _, loc := range rspecIdentifier.FindAllStringIndex(masked, -1) {
		name := masked[loc[0]:loc[1]]
		if loc[0] > 0 && strings.ContainsRune(".:@$", rune(masked[loc[0]-1])) || rubyKeywords[name] {
			continue
		}
		rest := strings.TrimLeft(masked[loc[1]:], " \t")
		if strings.HasPrefix(rest, ":") && !strings.HasPrefix(rest, "::") {
			continue
		}
		out = append(out, name)
	}
	return out
}

func constant(word string) bool { return word != "" && unicode.IsUpper(rune(word[0])) }

// rubyAssertion is one `expect(ACTUAL).to MATCHER(ARGS)` or `is_expected.to MATCHER(ARGS)`.
type rubyAssertion struct {
	t              rubyText
	actual         [2]int // range in t; empty for is_expected
	self           bool
	negated        bool // not_to or to_not
	matcher        string
	expected       [2]int // the matcher's argument range, or empty
	with           [2]int // a `.with(...)` argument range after the matcher, or empty
	hasWith        bool
	actualConstant bool
}

func rubyAssertions(body []rubyLine) []rubyAssertion {
	out := []rubyAssertion{}
	for _, l := range body {
		t := maskRuby(l)
		m := t.masked
		for _, loc := range rspecExpectArg.FindAllStringIndex(m, -1) {
			if loc[0] > 0 && m[loc[0]-1] == '.' {
				continue
			}
			open := loc[1] - 1
			close := balanced(m, open)
			if close < 0 {
				continue
			}
			a := rubyAssertion{t: t, actual: [2]int{open + 1, close}}
			a.actualConstant = rspecConstPath.MatchString(strings.TrimSpace(m[open+1 : close]))
			if a.readMatcher(close + 1) {
				out = append(out, a)
			}
		}
		for _, loc := range rspecIsExpected.FindAllStringIndex(m, -1) {
			a := rubyAssertion{t: t, self: true}
			if a.readMatcher(loc[0] + len("is_expected")) {
				out = append(out, a)
			}
		}
	}
	return out
}

// readMatcher reads `.to MATCHER(ARGS)` or `.to MATCHER ARGS` from position from of the line and
// an optional `.with(...)` after it.
func (a *rubyAssertion) readMatcher(from int) bool {
	m := a.t.masked
	loc := rspecMatcher.FindStringSubmatchIndex(m[from:])
	if loc == nil {
		return false
	}
	a.matcher = m[from+loc[2] : from+loc[3]]
	a.negated = strings.Contains(m[from:from+loc[2]], "not")
	at := from + loc[1]
	rest := strings.TrimLeft(m[at:], " ")
	start := len(m) - len(rest)
	switch {
	case strings.HasPrefix(rest, "("):
		if close := balanced(m, start); close > 0 {
			a.expected = [2]int{start + 1, close}
			at = close + 1
		}
	case rest != "" && !strings.HasPrefix(rest, ".") && !strings.HasPrefix(rest, ")"):
		end := len(m)
		for _, stop := range []string{" do", "{"} {
			if k := strings.Index(m[start:], stop); k >= 0 && start+k < end {
				end = start + k
			}
		}
		a.expected = [2]int{start, end}
		at = end
	}
	if w := rspecWith.FindStringIndex(m[at:]); w != nil {
		if close := balanced(m, at+w[1]-1); close > 0 {
			a.with, a.hasWith = [2]int{at + w[1], close}, true
		}
	}
	return true
}

// rubyStub is one message stub or expectation the example's code path sees.
type rubyStub struct {
	t           rubyText
	expect      bool
	anyInstance bool
	recv        string
	method      string
	value       [2]int
	hasValue    bool
	end         int // the position in t after `receive(:m)`
}

func rubyStubs(lines []rubyLine) []rubyStub {
	out := []rubyStub{}
	for _, l := range lines {
		t := maskRuby(l)
		for _, loc := range rspecStubCall.FindAllStringSubmatchIndex(t.masked, -1) {
			s := rubyStub{t: t, expect: t.masked[loc[2]:loc[3]] == "expect", anyInstance: loc[4] >= 0,
				recv: t.masked[loc[6]:loc[7]], method: t.masked[loc[8]:loc[9]], end: loc[1]}
			if r := rspecAndReturn.FindStringIndex(t.masked[loc[1]:]); r != nil {
				open := loc[1] + r[1] - 1
				if close := balanced(t.masked, open); close > 0 {
					s.value, s.hasValue = [2]int{open + 1, close}, true
				}
			}
			out = append(out, s)
		}
	}
	return out
}

// rubyBinding is a name's binding: a local assignment in the example body or a let on the chain.
type rubyBinding struct {
	line  int
	value rubyText
	span  [2]int
}

// rspecFlow indexes the file for TSA-V0-120..130, built on first use: each line's masked text and
// words, the example owning each body line, and each group's subject names.
type rspecFlow struct {
	text     []rubyText
	words    [][]string
	owner    map[int]int
	subjects map[int]map[string]bool
}

// flowContext is one example's view for the TSA-V0-120..130 readings.
type flowContext struct {
	r        *rspecReader
	name     string
	e        rspecExample
	subject  map[string]bool // names that are, or are computed from, the subject (TSA-V0-122)
	scope    []int           // the r.flow.text indexes of every line outside the other examples' bodies
	action   int             // the first body line that names the subject, or 0
	asserted map[int]bool    // body lines holding an expectation
}

func (r *rspecReader) flowContext(name string, e rspecExample) *flowContext {
	c := &flowContext{r: r, name: name, e: e, subject: map[string]bool{}, asserted: map[int]bool{}}
	if r.flow.subjects == nil {
		r.flow.subjects = map[int]map[string]bool{}
	}
	if known, ok := r.flow.subjects[e.group]; ok {
		maps.Copy(c.subject, known)
	} else {
		c.groupSubjects()
		r.flow.subjects[e.group] = maps.Clone(c.subject)
	}
	for _, l := range e.body[1:] {
		t := maskRuby(l)
		if m := rspecLocalSet.FindStringSubmatchIndex(t.masked); m != nil && c.names(t.masked[m[3]:]) {
			c.subject[t.masked[m[2]:m[3]]] = true
		}
		if c.action == 0 && c.names(t.masked) {
			c.action = l.line
		}
		if rspecExpectation.MatchString(t.masked) {
			c.asserted[l.line] = true
		}
	}
	r.flowIndex()
	for i, t := range r.flow.text {
		if owner, ok := r.flow.owner[t.line]; !ok || owner == e.line {
			c.scope = append(c.scope, i)
		}
	}
	return c
}

// groupSubjects fills the subject names an example group sees: the subject's own names, named
// subjects and the lets computed from them. It depends only on the group, so flowContext reads it
// once per group.
func (c *flowContext) groupSubjects() {
	r, e := c.r, c.e
	for _, n := range []string{"subject", "described_class", "is_expected"} {
		c.subject[n] = true
	}
	for g := e.group; g >= 0; g = r.groups[g].parent {
		if v := r.groups[g].decls["subject"]; strings.HasPrefix(v, "let:") {
			c.subject[strings.TrimPrefix(v, "let:")] = true
		}
	}
	// A let whose block names the subject is computed from it; two passes reach a let of a let.
	for range 2 {
		for g := e.group; g >= 0; g = r.groups[g].parent {
			for _, declared := range slices.Sorted(maps.Keys(r.groups[g].bodies)) {
				body := r.groups[g].bodies[declared]
				if c.subject[declared] || r.declaring(e.group, declared) != g {
					continue
				}
				for _, l := range body[1:] {
					if c.names(maskRuby(l).masked) {
						c.subject[declared] = true
					}
				}
				if len(body) == 1 && c.names(maskRuby(body[0]).masked[len(declared):]) {
					c.subject[declared] = true
				}
			}
		}
	}
}

// flowIndex masks every line and reads its words once per file, and maps each example body line
// to its example, so the per-example readings stay linear in the file.
func (r *rspecReader) flowIndex() {
	if r.flow.text != nil {
		return
	}
	r.flow.owner = map[int]int{}
	for _, x := range r.examples {
		for _, l := range x.body {
			r.flow.owner[l.line] = x.line
		}
	}
	for _, l := range r.lines {
		t := maskRuby(l)
		r.flow.text = append(r.flow.text, t)
		r.flow.words = append(r.flow.words, rubyWords(t.masked))
	}
}

// names reports masked text that names the subject or a value computed from it.
func (c *flowContext) names(masked string) bool {
	for _, w := range rubyWords(masked) {
		if c.subject[w] {
			return true
		}
	}
	return rspecIsExpected.MatchString(masked)
}

func (c *flowContext) at(line int) string { return fmt.Sprintf("%s:%d", c.name, line) }

// binding returns name's binding seen from line before: the last local assignment in the example
// body above it, else the innermost let on the example's group chain whose block holds one value.
func (c *flowContext) binding(name string, before int) (rubyBinding, bool) {
	found, ok := rubyBinding{}, false
	for _, l := range c.e.body[1:] {
		if l.line >= before {
			break
		}
		t := maskRuby(l)
		if m := rspecLocalSet.FindStringSubmatchIndex(t.masked); m != nil && t.masked[m[2]:m[3]] == name {
			eq := strings.Index(t.masked[m[3]:], "=") + m[3]
			found, ok = rubyBinding{line: l.line, value: t, span: trimSpan(t.masked, eq+1, len(t.masked))}, true
		}
	}
	if ok {
		return found, true
	}
	g := c.r.declaring(c.e.group, name)
	if g < 0 || c.r.groups[g].decls[name] != "let" {
		return rubyBinding{}, false
	}
	body := c.r.groups[g].bodies[name]
	switch {
	case len(body) == 1:
		t := maskRuby(body[0])
		open := strings.Index(t.masked, "{")
		close := strings.LastIndex(t.masked, "}")
		if open < 0 || close <= open || balanced(t.masked, open) != close {
			return rubyBinding{}, false
		}
		return rubyBinding{line: t.line, value: t, span: trimSpan(t.masked, open+1, close)}, true
	case len(body) == 3 && rspecBlockClose.MatchString(body[2].text):
		t := maskRuby(body[1])
		return rubyBinding{line: t.line, value: t, span: [2]int{0, len(t.masked)}}, true
	}
	return rubyBinding{}, false
}

func trimSpan(s string, from, to int) [2]int {
	for from < to && s[from] == ' ' {
		from++
	}
	for to > from && s[to-1] == ' ' {
		to--
	}
	return [2]int{from, to}
}

// receiver classifies a stub's receiver (TSA-V0-121): "subject", "boundary", "first-party" or
// "unresolved"; class names it when known; fresh reports a binding to a new object; line is that
// binding's line.
type receiver struct {
	kind, class string
	fresh       bool
	line        int
}

func (c *flowContext) receiver(s rubyStub) receiver {
	classify := func(class string) string {
		if rspecBoundaryConsts[strings.SplitN(class, "::", 2)[0]] || rspecQueueMethods[s.method] {
			return "boundary"
		}
		return "first-party"
	}
	switch {
	case c.subject[s.recv] || s.recv == "self":
		return receiver{kind: "subject"}
	case constant(s.recv):
		return receiver{kind: classify(s.recv), class: s.recv}
	}
	b, ok := c.binding(s.recv, s.t.line)
	if !ok {
		return receiver{kind: "unresolved"}
	}
	value := b.value.masked[b.span[0]:b.span[1]]
	raw := b.value.raw[b.span[0]:b.span[1]]
	switch {
	case rspecNewCall.MatchString(value):
		class := rspecNewCall.FindStringSubmatch(value)[1]
		return receiver{kind: classify(class), class: class, fresh: true, line: b.line}
	case rspecVerified.MatchString(raw):
		class := rspecVerified.FindStringSubmatch(raw)[1]
		return receiver{kind: classify(class), class: class, fresh: true, line: b.line}
	case rspecPlainDbl.MatchString(value):
		return receiver{kind: "unresolved", fresh: true, line: b.line}
	case rspecBuilt.MatchString(value):
		return receiver{kind: classify(""), fresh: true, line: b.line}
	case rspecCreated.MatchString(value):
		return receiver{kind: classify(""), line: b.line}
	}
	return receiver{kind: "unresolved"}
}

// flowFindings returns example e's STUB-ON-PATH, STUB-NEVER-INVOKED and
// ASSERTED-VALUE-NOT-FROM-SUBJECT findings, in that rule order and each in line order
// (TSA-V0-120).
// A stub of a flag lookup is a condition, not a collaborator (TSA-V0-435), so neither stub rule
// reads it.
func (c *flowContext) flowFindings(test string) []testvalidity.SourceFinding {
	r, e := c.r, c.e
	finding := func(rule string, location, cause int, message string) testvalidity.SourceFinding {
		return testvalidity.SourceFinding{Rule: rule, Severity: testvalidity.SeverityAdvisory, Test: test, Title: e.title,
			Location: c.at(location), Cause: c.at(cause), Message: message}
	}
	lines := append([]rubyLine{}, e.body...)
	for g := e.group; g >= 0; g = r.groups[g].parent {
		for _, h := range r.groups[g].hooks {
			lines = append(lines, h.body...)
		}
	}
	stubs := rubyStubs(lines)
	slices.SortStableFunc(stubs, func(a, b rubyStub) int { return a.t.line - b.t.line })
	asserts := rubyAssertions(e.body[1:])
	onPath, never, notFrom := []testvalidity.SourceFinding{}, []testvalidity.SourceFinding{}, []testvalidity.SourceFinding{}
	for _, s := range stubs {
		rc := c.receiver(s)
		if _, flag := c.flagOf(s, rc); flag {
			continue
		}
		unused := !s.expect && !s.anyInstance && !constant(s.recv) && rc.fresh && rc.kind != "subject" && rc.kind != "boundary" && !c.mentioned(s.recv, rc.line)
		if rc.kind == "first-party" && !unused {
			if a, action, ok := c.onPath(s, asserts); ok {
				onPath = append(onPath, finding(testvalidity.RuleStubOnPath, s.t.line, a.t.line, fmt.Sprintf(
					"the first-party collaborator %s is stubbed, and the assertion at %s checks the stub's return value after the action at %s, so it observes the stub instead of the collaborator's behaviour",
					symbol(s, rc), c.at(a.t.line), c.at(action))))
			}
		}
		if unused {
			never = append(never, finding(testvalidity.RuleStubNeverInvoked, s.t.line, rc.line, fmt.Sprintf(
				"%s is stubbed on an object the test builds at %s and never hands to any code, so the action cannot invoke the stub; static reading only, a run that records calls would confirm it",
				symbol(s, rc), c.at(rc.line))))
		}
	}
	for _, a := range asserts {
		if f, ok := c.notFromSubject(a); ok {
			notFrom = append(notFrom, finding(testvalidity.RuleAssertedValueNotFromSubject, a.t.line, f.cause, f.message))
		}
	}
	for _, f := range c.blockEnqueues() {
		notFrom = append(notFrom, finding(testvalidity.RuleAssertedValueNotFromSubject, f.cause, f.cause, f.message))
	}
	slices.SortStableFunc(notFrom, func(a, b testvalidity.SourceFinding) int {
		return strings.Compare(lineKey(a.Location), lineKey(b.Location))
	})
	return append(append(onPath, never...), notFrom...)
}

// lineKey orders "file:line" locations of one file by line.
func lineKey(location string) string {
	line := location[strings.LastIndex(location, ":")+1:]
	return strings.Repeat("0", max(0, 10-len(line))) + line
}

func symbol(s rubyStub, rc receiver) string {
	if s.anyInstance || rc.class != "" && !constant(s.recv) {
		return rc.class + "#" + s.method
	}
	return s.recv + "." + s.method
}

// onPath returns the first assertion that checks stub s's value on the subject's result, or that
// calls the stub itself, with the action line (TSA-V0-122).
func (c *flowContext) onPath(s rubyStub, asserts []rubyAssertion) (rubyAssertion, int, bool) {
	call := regexp.MustCompile(`(?:^|[^\w.:])` + regexp.QuoteMeta(s.recv) + `\s*\.\s*` + regexp.QuoteMeta(s.method) + `(?:[^\w?!]|$)`)
	value, ident := "", false
	if s.hasValue {
		value = s.t.squash(s.value[0], s.value[1])
		ident = regexp.MustCompile(`^[a-z_]\w*$`).MatchString(value)
		switch value {
		case "", "nil", "true", "false", "[]", "{}", `""`, "''":
			value = ""
		}
	}
	for _, a := range asserts {
		if a.negated {
			continue
		}
		actual := ""
		if !a.self {
			actual = a.t.masked[a.actual[0]:a.actual[1]]
		}
		if !s.anyInstance && actual != "" && call.MatchString(actual) {
			return a, a.t.line, true
		}
		if value == "" || c.action == 0 || !(a.self || c.names(actual)) || a.expected[1] <= a.expected[0] {
			continue
		}
		expected := a.t.squash(a.expected[0], a.expected[1])
		if expected == value || ident && slices.Contains(rubyWords(a.t.masked[a.expected[0]:a.expected[1]]), value) {
			return a, c.action, true
		}
	}
	return rubyAssertion{}, 0, false
}

// mentioned reports a reference to name anywhere in scope other than its own binding line, a
// stub or have_received expectation on it (TSA-V0-124).
func (c *flowContext) mentioned(name string, binding int) bool {
	own := regexp.MustCompile(`\b(?:allow|expect)\s*\(\s*` + regexp.QuoteMeta(name) + `\s*\)\s*\.\s*(?:to|not_to|to_not)\s*\(?\s*(?:receive|have_received)\b`)
	set := regexp.MustCompile(`^` + regexp.QuoteMeta(name) + `\s*=`)
	for _, i := range c.scope {
		t := c.r.flow.text[i]
		if !slices.Contains(c.r.flow.words[i], name) {
			continue
		}
		m := own.ReplaceAllString(t.masked, "")
		if t.line == binding {
			m = set.ReplaceAllString(m, "")
		}
		if slices.Contains(rubyWords(m), name) {
			return true
		}
	}
	return false
}

type origin struct {
	cause   int
	message string
}

// notFromSubject reports an assertion whose actual value only the test built (TSA-V0-126), or an
// enqueue expectation the test's own enqueue meets (TSA-V0-128).
func (c *flowContext) notFromSubject(a rubyAssertion) (origin, bool) {
	if a.self || a.negated {
		return origin{}, false
	}
	actual := strings.TrimSpace(a.t.masked[a.actual[0]:a.actual[1]])
	if a.actualConstant {
		if a.matcher != "have_been_enqueued" && a.matcher != "have_enqueued_sidekiq_job" {
			return origin{}, false
		}
		want, has := a.with, a.hasWith
		if a.matcher == "have_enqueued_sidekiq_job" {
			want, has = a.expected, a.expected[1] > a.expected[0]
		}
		before := []rubyLine{}
		for _, l := range c.e.body[1:] {
			if l.line < a.t.line {
				before = append(before, l)
			}
		}
		return c.ownEnqueue(actual, before, a.t, want, has)
	}
	roots := map[string]int{}
	if !c.built(a.t, a.actual, a.t.line, 0, roots) || len(roots) == 0 {
		return origin{}, false
	}
	allowed := map[int]bool{}
	first := 0
	for _, line := range roots {
		allowed[line] = true
		if first == 0 || line < first {
			first = line
		}
	}
	for name := range roots {
		for _, i := range c.scope {
			t := c.r.flow.text[i]
			if allowed[t.line] || !slices.Contains(c.r.flow.words[i], name) {
				continue
			}
			if !c.asserted[t.line] || slices.Contains(rubyWords(assertionResidue(t)), name) {
				return origin{}, false
			}
		}
	}
	return origin{cause: first, message: fmt.Sprintf(
		"the asserted value %s comes only from values the test builds itself (from %s) and no code under test receives them, so the assertion passes whatever the subject does",
		strings.TrimSpace(a.t.raw[a.actual[0]:a.actual[1]]), c.at(first))}, true
}

// assertionResidue returns an expectation line's masked text with the leading name of each
// assertion's actual and expected value blanked: a value an assertion only reads. A name left in
// the residue is passed to a call, such as `expect(subject.update(invoice))`.
func assertionResidue(t rubyText) string {
	b := []byte(t.masked)
	blank := func(span [2]int) {
		if loc := rspecIdentifier.FindStringIndex(t.masked[span[0]:span[1]]); loc != nil && strings.TrimSpace(t.masked[span[0]:span[0]+loc[0]]) == "" {
			for i := span[0] + loc[0]; i < span[0]+loc[1]; i++ {
				b[i] = ' '
			}
		}
	}
	for _, a := range rubyAssertions([]rubyLine{{line: t.line, text: t.raw}}) {
		if !a.self {
			blank(a.actual)
		}
		blank(a.expected)
	}
	return string(b)
}

// built reports that span of t holds a value the test builds alone: literals, the builder calls
// of rspecBuilder over such values, and names bound to such values, with no subject, constant,
// persisted read or unresolved name (TSA-V0-126). roots collects each bound name's line.
func (c *flowContext) built(t rubyText, span [2]int, before, depth int, roots map[string]int) bool {
	text := strings.TrimSpace(t.masked[span[0]:span[1]])
	if text == "" || strings.Contains(t.raw[span[0]:span[1]], "#{") || depth > maxFlowDepth || rspecPersisted.MatchString(text) || rspecSelfWords.MatchString(text) || strings.Contains(text, "->") {
		return false
	}
	if loc := rspecBuilder.FindStringIndex(text); loc != nil {
		text = text[loc[1]:]
	}
	for _, w := range rubyWords(text) {
		if constant(w) || c.subject[w] {
			return false
		}
		b, ok := c.binding(w, before)
		if !ok || !c.built(b.value, b.span, b.line, depth+1, roots) {
			return false
		}
		roots[w] = b.line
	}
	return true
}

// ownEnqueue reports the test's own enqueue of job among lines whose arguments the expectation's
// `with` arguments repeat, or any own enqueue when the expectation names no arguments
// (TSA-V0-128).
func (c *flowContext) ownEnqueue(job string, lines []rubyLine, at rubyText, want [2]int, has bool) (origin, bool) {
	call := regexp.MustCompile(`(?:^|[^\w:.])` + regexp.QuoteMeta(job) + `(?:\s*\.\s*set\s*\([^()]*\))?\s*\.\s*(perform_later|perform_async|perform_in|perform_at)\b\s*(\()?`)
	expected := ""
	if has {
		expected = at.squash(want[0], want[1])
	}
	for _, l := range lines {
		t := maskRuby(l)
		for _, m := range call.FindAllStringSubmatchIndex(t.masked, -1) {
			args := ""
			if m[4] >= 0 {
				close := balanced(t.masked, m[4])
				if close < 0 {
					continue
				}
				args = t.squash(m[4]+1, close)
				if method := t.masked[m[2]:m[3]]; method == "perform_in" || method == "perform_at" {
					args = dropFirstArg(t, m[4]+1, close)
				}
			} else {
				args = t.squash(m[1], len(t.masked))
			}
			if !has || args == expected {
				return origin{cause: l.line, message: fmt.Sprintf(
					"the enqueue expectation on %s is met by the test's own enqueue at %s, so it passes whatever the subject enqueues",
					job, c.at(l.line))}, true
			}
		}
	}
	return origin{}, false
}

func dropFirstArg(t rubyText, from, to int) string {
	depth := 0
	for i := from; i < to; i++ {
		switch t.masked[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				return t.squash(i+1, to)
			}
		}
	}
	return ""
}

// blockEnqueues returns the TSA-V0-128 findings of single-line `expect { BLOCK }.to
// have_enqueued_job(Job)` expectations whose block enqueues Job itself.
func (c *flowContext) blockEnqueues() []origin {
	out := []origin{}
	for _, l := range c.e.body[1:] {
		t := maskRuby(l)
		for _, loc := range rspecBlockJob.FindAllStringIndex(t.masked, -1) {
			open := loc[1] - 1
			close := balanced(t.masked, open)
			if close < 0 {
				continue
			}
			m := rspecBlockJobM.FindStringSubmatchIndex(t.masked[close+1:])
			if m == nil {
				continue
			}
			job := t.masked[close+1+m[2] : close+1+m[3]]
			after := close + 1 + m[1]
			want, has := [2]int{}, false
			if w := rspecWith.FindStringIndex(t.masked[after:]); w != nil {
				if wc := balanced(t.masked, after+w[1]-1); wc > 0 {
					want, has = [2]int{after + w[1], wc}, true
				}
			}
			inner := rubyLine{line: l.line, text: l.text[open+1 : close]}
			if o, ok := c.ownEnqueue(job, []rubyLine{inner}, t, want, has); ok {
				out = append(out, o)
			}
		}
	}
	return out
}
