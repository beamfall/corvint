package testsource

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// The phase reading of `corvint tests teardown` (TSA-V0-112): every call an example makes, placed
// in setup, body or teardown, with the order teardown runs in. JS/TS reuses the TSA-V0-010 lexer
// and declaration model; RSpec reuses the logical-line Ruby reader of TSA-V0-034.

const (
	phaseSetup    = "setup"
	phaseBody     = "body"
	phaseTeardown = "teardown"
)

// tdCall is one call an example makes.
type tdCall struct {
	name  string
	line  int
	phase string
	// seq orders calls inside one setup/body run and inside one teardown unit.
	seq int
	// unit names the teardown code the call sits in: 0 for finally/ensure code in the body, else
	// one per hook. rank orders units: finally/ensure first, then hooks from the inner group out.
	unit, rank int
	// mentions holds the receiver root and the argument names the call passes.
	mentions map[string]bool
	// assigned is the variable the call's value is assigned to, when it is.
	assigned string
	// conditional marks a call that may not run each time its code runs: it sits in a nested
	// function, a branch, a loop, a rescue/catch, a ternary or after a short-circuit operator.
	conditional bool
	// at is the call's token index (JS/TS only).
	at int
}

// tdExample is one example and the calls it runs.
type tdExample struct {
	title  string
	titled bool
	line   int
	calls  []tdCall
	// unread holds the calls of an applying hook the reading does not place (beforeAll, around, ...).
	unread []tdCall
	// opaque is set when an applying hook could not be read at all (an RSpec hook without a block).
	opaque bool
}

// tdFile is one read test file.
type tdFile struct {
	framework string
	uncertain bool
	reason    string
	examples  []tdExample
}

// runsBefore reports whether teardown call a runs before b, and whether that order is known.
func runsBefore(a, b tdCall) (before, known bool) {
	switch {
	case a.unit == b.unit:
		return a.seq < b.seq, true
	case a.rank != b.rank:
		return a.rank < b.rank, true
	}
	return false, false
}

// --- JS/TS ---

var (
	jsSetupHooks    = map[string]bool{"beforeEach": true}
	jsTeardownHooks = map[string]bool{"afterEach": true}
	jsUnreadHooks   = map[string]bool{"beforeAll": true, "afterAll": true}
	jsNotCalls      = map[string]bool{"if": true, "for": true, "while": true, "switch": true, "catch": true, "function": true,
		"return": true, "typeof": true, "await": true, "async": true, "super": true, "import": true, "new": true, "void": true}
)

type jsHook struct {
	name   string
	at     int
	body   [2]int
	groups []int // describe declarations enclosing the hook
}

// jsAssertion reports a call that only asserts: expect, assert* or a matcher (toBe, toEqual, ...).
func jsAssertion(name string) bool {
	if name == "expect" || strings.HasPrefix(name, "assert") {
		return true
	}
	return len(name) > 2 && strings.HasPrefix(name, "to") && unicode.IsUpper(rune(name[2]))
}

func readJSTeardown(name, text string) tdFile {
	s := parseSource(name, text)
	f := tdFile{framework: "jsts", uncertain: s.uncertain}
	if s.uncertain {
		f.reason = "file-uncertain"
	}
	groups := func(i int) []int {
		out := []int{}
		for k, d := range s.decls {
			if d.kind == "describe" && within(i, d.body) {
				out = append(out, k)
			}
		}
		return out
	}
	inTest := func(i int) bool {
		for _, d := range s.decls {
			if d.kind == "test" && within(i, d.body) {
				return true
			}
		}
		return false
	}
	hooks := []jsHook{}
	for i, t := range s.tokens {
		hook := jsSetupHooks[t.Text] || jsTeardownHooks[t.Text] || jsUnreadHooks[t.Text]
		if !s.ident(i, "") || !hook || (s.memberAccess(i) && !(s.ident(i-2, "test") && !s.memberAccess(i-2))) {
			continue
		}
		if !s.punct(i+1, "(") || s.match[i+1] < 0 || inTest(i) {
			continue
		}
		for _, arg := range s.splitArgs(i+1, s.match[i+1]) {
			if s.callback(arg) {
				hooks = append(hooks, jsHook{name: t.Text, at: i, body: s.callbackBody(arg), groups: groups(i)})
				break
			}
		}
	}
	for _, d := range s.decls {
		if d.kind != "test" || d.body[1] <= d.body[0] {
			continue
		}
		enclosing := groups(d.at)
		ex := tdExample{title: d.title, titled: d.titled, line: d.line}
		applying := []jsHook{}
		for _, h := range hooks {
			if subset(h.groups, enclosing) {
				applying = append(applying, h)
			}
		}
		sort.SliceStable(applying, func(a, b int) bool { return len(applying[a].groups) < len(applying[b].groups) })
		add := ex.adder()
		for _, h := range applying {
			if jsSetupHooks[h.name] {
				add(s.tdCalls(h.body), phaseSetup, 0, 0)
			}
		}
		// finally code keeps its source position: it runs where it stands, not at the example's end.
		finals := s.finallyBlocks(d.body)
		for _, c := range s.tdCalls(s.callbackBody(d.body)) {
			c.phase = phaseBody
			for _, r := range finals {
				if within(c.at, r) {
					c.phase = phaseTeardown
				}
			}
			add([]tdCall{c}, "", 0, 0)
		}
		for k := len(applying) - 1; k >= 0; k-- {
			h := applying[k]
			switch {
			case jsTeardownHooks[h.name]:
				add(s.tdCalls(h.body), phaseTeardown, 1+h.at, 1+len(enclosing)-len(h.groups))
			case jsUnreadHooks[h.name]:
				ex.unread = append(ex.unread, s.tdCalls(h.body)...)
			}
		}
		f.examples = append(f.examples, ex)
	}
	return f
}

// adder returns a function that appends calls to the example in run order, giving each the phase
// ("" keeps the call's own), the teardown unit and rank, and the next sequence number.
func (ex *tdExample) adder() func(calls []tdCall, phase string, unit, rank int) {
	seq := 0
	return func(calls []tdCall, phase string, unit, rank int) {
		for _, c := range calls {
			if phase != "" {
				c.phase = phase
			}
			c.unit, c.rank, c.seq = unit, rank, seq
			seq++
			ex.calls = append(ex.calls, c)
		}
	}
}

func subset(inner, outer []int) bool {
	for _, k := range inner {
		found := false
		for _, o := range outer {
			found = found || o == k
		}
		if !found {
			return false
		}
	}
	return true
}

// finallyBlocks returns the `finally { }` blocks inside a range.
func (s *source) finallyBlocks(r [2]int) [][2]int {
	out := [][2]int{}
	for i := r[0]; i < r[1]; i++ {
		if s.ident(i, "finally") && s.punct(i+1, "{") && s.match[i+1] > i+1 {
			out = append(out, [2]int{i + 2, s.match[i+1]})
			i = s.match[i+1]
		}
	}
	return out
}

// callbackBody returns the token range of a callback argument's function body, inside its braces,
// or the expression of a brace-less arrow; the whole argument when neither is found.
func (s *source) callbackBody(arg [2]int) [2]int {
	for i := arg[0]; i < arg[1]; i++ {
		switch {
		case s.punct(i, "(") && s.match[i] > i:
			i = s.match[i]
		case s.arrow(i):
			if s.punct(i+2, "{") && s.match[i+2] > i+2 {
				return [2]int{i + 3, s.match[i+2]}
			}
			return [2]int{i + 2, arg[1]}
		case s.punct(i, "{") && s.match[i] > i:
			return [2]int{i + 1, s.match[i]}
		}
	}
	return arg
}

// conditional reports that the call at token i may not run each time the range r runs: it sits in
// a function nested in r, an if/else, a loop, a switch, a catch, a ternary or after a short-circuit
// operator (TSA-V0-112). try and finally blocks and plain blocks run.
func (s *source) conditional(i int, r [2]int) bool {
	statement := true // still inside the statement that holds the call
	for j := i - 1; j >= r[0]; j-- {
		if s.punct(j, ")") || s.punct(j, "]") || s.punct(j, "}") {
			open := s.match[j]
			if open < 0 || open > j {
				return true // unbalanced: execution cannot be established
			}
			if statement && s.punct(j, ")") && (s.ident(open-1, "if") || s.ident(open-1, "for") || s.ident(open-1, "while")) {
				return true // a brace-less if or loop
			}
			if s.punct(j, "}") {
				statement = false
			}
			j = open
			continue
		}
		switch {
		case s.punct(j, ";"):
			statement = false
		case s.punct(j, "{"):
			before := j - 1
			switch {
			case s.punct(before, ">") && s.arrow(before-1),
				s.punct(before, ")"), // if/for/while/switch/catch, or a function or method body
				s.ident(before, "else"), s.ident(before, "do"), s.ident(before, "catch"):
				return true
			}
			statement = true // a block, try/finally or object literal: its own statement continues
		case !statement:
		case s.arrow(j), s.ident(j, "else"), s.ident(j, "do"):
			return true
		case s.punct(j, "?") && !s.punct(j+1, "."):
			return true // a ternary or ??
		case (s.punct(j, "&") && s.punct(j-1, "&")) || (s.punct(j, "|") && s.punct(j-1, "|")):
			return true
		}
	}
	return false
}

// tdCalls lists the calls in a range in token order.
func (s *source) tdCalls(r [2]int) []tdCall {
	out := []tdCall{}
	for i := r[0]; i < r[1]; i++ {
		if !s.ident(i, "") || !s.punct(i+1, "(") || s.match[i+1] < 0 {
			continue
		}
		name := s.tokens[i].Text
		if jsNotCalls[name] || s.ident(i-1, "function") || jsAssertion(name) {
			continue
		}
		c := tdCall{name: name, line: s.tokens[i].Line, mentions: map[string]bool{}, at: i, conditional: s.conditional(i, r)}
		root := i
		for s.punct(root-1, ".") {
			k := root - 2
			if s.punct(k, "?") {
				k--
			}
			if !s.ident(k, "") {
				root = -1
				break
			}
			root = k
		}
		if root >= 0 && root != i {
			c.mentions[s.tokens[root].Text] = true
		}
		for _, arg := range s.splitArgs(i+1, s.match[i+1]) {
			if s.callback(arg) {
				continue
			}
			for k := arg[0]; k < arg[1]; k++ {
				if s.ident(k, "") && !s.memberAccess(k) {
					c.mentions[s.tokens[k].Text] = true
				}
			}
		}
		if root >= 0 {
			k := root - 1
			if s.ident(k, "await") {
				k--
			}
			if s.punct(k, "=") && s.ident(k-1, "") && !s.memberAccess(k-1) {
				c.assigned = s.tokens[k-1].Text
			}
		}
		out = append(out, c)
	}
	return out
}

// --- RSpec ---

var (
	tdRubyGroup   = regexp.MustCompile(`^(?:RSpec\.)?(?:[xf]?describe|[xf]?context|feature)\b`)
	tdRubyExample = regexp.MustCompile(`^(?:[xf]?it|[xf]?specify|[xf]?example|[xf]?scenario)\b`)
	tdRubyTitle   = regexp.MustCompile(`^\w+\s*\(?\s*(?:'([^'\\]*)'|"([^"\\]*)")`)
	// tdRubyHook: group 1 is the hook, group 2 the rest of the line (its arguments and block).
	tdRubyHook   = regexp.MustCompile(`^(before|prepend_before|append_before|after|prepend_after|append_after|around)\b(.*)$`)
	tdRubyScope  = regexp.MustCompile(`^(?:\(\s*:(\w+)\s*\)|:(\w+))?$`)
	tdRubyRescue = regexp.MustCompile(`^rescue\b`)
	tdRubyElse   = regexp.MustCompile(`^else\b`)
	// tdRubyMaybe: a statement modifier, a short-circuit operator, a ternary, a lambda, an inline
	// block or a one-line do block, any of which may keep a call on the line from running.
	tdRubyMaybe  = regexp.MustCompile(`\s(?:if|unless|while|until|and|or)\s|&&|\|\||\s\?\s|->|\blambda\b|\bproc\b|Proc\.new|[\w)\]?!]\s*\{|\bdo\b.*\bend\b`)
	tdRubyLet    = regexp.MustCompile(`^(?:let!?|subject!?)\s*(?:\(\s*:(\w+[?!]?)\s*\)|\s+:(\w+[?!]?))?\s*(?:do\b|\{)`)
	tdRubyEnsure = regexp.MustCompile(`^ensure\b`)
	tdRubyString = regexp.MustCompile(`'(?:[^'\\]|\\.)*'|"(?:[^"\\]|\\.)*"`)
	tdRubyToken  = regexp.MustCompile(`@{0,2}[A-Za-z_]\w*[?!]?|::|\S`)
	tdRubyNames  = map[string]bool{"and": true, "begin": true, "break": true, "case": true, "class": true, "def": true, "do": true,
		"else": true, "elsif": true, "end": true, "ensure": true, "false": true, "for": true, "if": true, "in": true, "module": true,
		"next": true, "nil": true, "not": true, "or": true, "redo": true, "rescue": true, "retry": true, "return": true, "self": true,
		"super": true, "then": true, "true": true, "undef": true, "unless": true, "until": true, "when": true, "while": true, "yield": true,
		"lambda": true, "proc": true, "raise": true}
	rubyAssertionNames    = map[string]bool{"expect": true, "is_expected": true, "to": true, "not_to": true, "to_not": true, "eq": true, "eql": true, "equal": true, "be": true, "include": true, "match": true, "raise_error": true, "change": true, "satisfy": true, "contain_exactly": true}
	rubyAssertionPrefixes = []string{"be_", "have_", "assert", "refute", "match_"}
)

func tdRubyAssertion(name string) bool {
	if rubyAssertionNames[name] {
		return true
	}
	for _, p := range rubyAssertionPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

type rbHook struct {
	kind  string // phaseSetup, phaseTeardown, "unread" or "opaque" (no block to read)
	body  []rubyLine
	depth int
	index int
	let   string
}

type rbWalker struct {
	lines     []rubyLine
	effects   sideEffects
	uncertain bool
	hooks     int
	examples  []tdExample
}

func readRSpecTeardown(text string, effects sideEffects) tdFile {
	w := &rbWalker{lines: rubyLines(text), effects: effects}
	w.group(0, len(w.lines), 0, nil)
	f := tdFile{framework: "rspec", uncertain: w.uncertain, examples: w.examples}
	if w.uncertain {
		f.reason = "file-uncertain"
	}
	return f
}

// block returns the body of the block opened on line i and the index of its last line.
func (w *rbWalker) block(i int) ([]rubyLine, int) {
	text := w.lines[i].text
	if rubyDo.MatchString(text) {
		end := rubyBlockEnd(w.lines, i)
		if end >= len(w.lines) {
			w.uncertain = true
			return w.lines[i+1:], len(w.lines)
		}
		return w.lines[i+1 : end], end
	}
	stripped := tdRubyString.ReplaceAllString(text, `""`)
	open := strings.Index(stripped, "{")
	if open < 0 {
		return nil, i
	}
	depth := 0
	for k := open; k < len(stripped); k++ {
		switch stripped[k] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return []rubyLine{{line: w.lines[i].line, text: strings.TrimSpace(stripped[open+1 : k])}}, i
			}
		}
	}
	body := []rubyLine{{line: w.lines[i].line, text: strings.TrimSpace(stripped[open+1:])}}
	for j := i + 1; j < len(w.lines); j++ {
		for _, c := range tdRubyString.ReplaceAllString(w.lines[j].text, `""`) {
			switch c {
			case '{':
				depth++
			case '}':
				depth--
			}
		}
		if depth <= 0 {
			return body, j
		}
		body = append(body, w.lines[j])
	}
	w.uncertain = true
	return body, len(w.lines)
}

// skip returns the last line of a block opened on line i, or i.
func (w *rbWalker) skip(i int) int {
	text := w.lines[i].text
	if rubyOpens(text) || strings.Count(text, "{") > strings.Count(text, "}") {
		_, end := w.block(i)
		return end
	}
	return i
}

func (w *rbWalker) group(lo, hi, depth int, outer []rbHook) {
	hooks := append([]rbHook{}, outer...)
	for i := lo; i < hi; i++ {
		text := w.lines[i].text
		if m := tdRubyHook.FindStringSubmatch(text); m != nil && !strings.HasPrefix(strings.TrimSpace(m[2]), "=") {
			kind, opens := hookKind(m[1], m[2])
			end, body := i, []rubyLine(nil)
			if opens {
				body, end = w.block(i)
			}
			w.hooks++
			hooks = append(hooks, rbHook{kind: kind, body: body, depth: depth, index: w.hooks})
			i = end
			continue
		}
		if m := tdRubyLet.FindStringSubmatch(text); m != nil {
			body, end := w.block(i)
			name := m[1] + m[2]
			if name == "" {
				name = "subject"
			}
			hooks = append(hooks, rbHook{kind: phaseSetup, body: body, depth: depth, let: name})
			i = end
			continue
		}
		i = w.skip(i)
	}
	for i := lo; i < hi; i++ {
		text := w.lines[i].text
		switch {
		case tdRubyGroup.MatchString(text) && rubyDo.MatchString(text):
			end := rubyBlockEnd(w.lines, i)
			if end > hi {
				w.uncertain = true
				end = hi
			}
			w.group(i+1, end, depth+1, hooks)
			i = end
		case tdRubyGroup.MatchString(text):
			// A brace group whose `{` ends its line and whose `}` starts its closing line is read
			// like a do group; any other group form is not read and the file is uncertain.
			end, ok := w.braceGroup(i, hi)
			if !ok {
				w.uncertain = true
				i = w.skip(i)
				continue
			}
			w.group(i+1, end, depth+1, hooks)
			i = end
		case tdRubyExample.MatchString(text):
			body, end := w.block(i)
			w.example(text, w.lines[i].line, body, depth, hooks)
			i = end
		default:
			i = w.skip(i)
		}
	}
}

// hookKind classifies a hook from its name and the rest of its line: an around hook, a hook with a
// scope other than each/example or with metadata arguments is unread, and a hook without a block
// is opaque.
func hookKind(name, rest string) (string, bool) {
	stripped := tdRubyString.ReplaceAllString(rest, `""`)
	args := stripped
	if loc := rubyDo.FindStringIndex(stripped); loc != nil {
		args = stripped[:loc[0]]
	} else if open := strings.Index(stripped, "{"); open >= 0 {
		args = stripped[:open]
	} else {
		return "opaque", false
	}
	kind := phaseSetup
	if strings.HasSuffix(name, "after") {
		kind = phaseTeardown
	}
	m := tdRubyScope.FindStringSubmatch(strings.TrimSpace(args))
	switch {
	case name == "around", m == nil:
		return "unread", true
	case m[1]+m[2] != "" && m[1]+m[2] != "each" && m[1]+m[2] != "example":
		return "unread", true
	}
	return kind, true
}

// braceGroup returns the closing line of a multi-line brace group opened on line i.
func (w *rbWalker) braceGroup(i, hi int) (int, bool) {
	text := tdRubyString.ReplaceAllString(w.lines[i].text, `""`)
	if !strings.HasSuffix(text, "{") || strings.Count(text, "{") != strings.Count(text, "}")+1 {
		return 0, false
	}
	depth := 1
	for j := i + 1; j < hi; j++ {
		line := tdRubyString.ReplaceAllString(w.lines[j].text, `""`)
		depth += strings.Count(line, "{") - strings.Count(line, "}")
		if depth <= 0 {
			rest := strings.TrimSpace(strings.TrimPrefix(line, "}"))
			return j, depth == 0 && strings.HasPrefix(line, "}") && (rest == "" || rest == ")")
		}
	}
	return 0, false
}

// blockCalls lists the calls of block lines in order. Each call gets the phase, or teardown inside
// the ensure clause when ensure is set, and is conditional when its line sits in an if/unless/case/
// loop/def/do block or a rescue clause, or carries a modifier, operator or inline block that may
// keep it from running (TSA-V0-112).
func (w *rbWalker) blockCalls(lines []rubyLine, phase string, ensure bool) []tdCall {
	out := []tdCall{}
	stack := []bool{} // per open block: whether its lines may not run
	ensureAt, rescueAt := -1, -1
	for _, l := range lines {
		text := tdRubyString.ReplaceAllString(l.text, `""`)
		switch {
		case rubyEnd.MatchString(text):
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			if ensureAt > len(stack) {
				ensureAt = -1
			}
			if rescueAt > len(stack) {
				rescueAt = -1
			}
			continue
		case tdRubyEnsure.MatchString(text):
			rescueAt = -1
			if ensure {
				ensureAt = len(stack)
			}
			continue
		case tdRubyRescue.MatchString(text):
			rescueAt = len(stack)
			continue
		case tdRubyElse.MatchString(text) && rescueAt == len(stack):
			rescueAt = -1
			continue
		}
		conditional := rescueAt >= 0 || tdRubyMaybe.MatchString(text)
		for _, c := range stack {
			conditional = conditional || c
		}
		p := phase
		if ensureAt >= 0 {
			p = phaseTeardown
		}
		calls := w.calls([]rubyLine{l}, p == phaseTeardown)
		for k := range calls {
			calls[k].phase, calls[k].conditional = p, conditional
		}
		out = append(out, calls...)
		if rubyOpens(text) {
			// begin, statement or value (`cleanup = begin`), runs its body unconditionally.
			stack = append(stack, !strings.HasPrefix(text, "begin") && rubyValueKeyword(text) != "begin")
		}
	}
	return out
}

func (w *rbWalker) example(text string, line int, body []rubyLine, depth int, hooks []rbHook) {
	ex := tdExample{line: line}
	if m := tdRubyTitle.FindStringSubmatch(text); m != nil {
		ex.title, ex.titled = m[1]+m[2], true
	}
	hooks = append([]rbHook{}, hooks...)
	sort.SliceStable(hooks, func(a, b int) bool { return hooks[a].depth < hooks[b].depth })
	add := ex.adder()
	for _, h := range hooks {
		ex.opaque = ex.opaque || h.kind == "opaque"
		if h.kind == phaseSetup {
			calls := w.blockCalls(h.body, phaseSetup, false)
			for k := range calls {
				if h.let != "" && calls[k].assigned == "" {
					if _, create := w.effects.creates[calls[k].name]; create {
						calls[k].assigned = h.let
					}
				}
			}
			add(calls, phaseSetup, 0, 0)
		}
	}
	// ensure code keeps its source position, as JS finally code does.
	add(w.blockCalls(body, phaseBody, true), "", 0, 0)
	for k := len(hooks) - 1; k >= 0; k-- {
		h := hooks[k]
		switch h.kind {
		case phaseTeardown:
			add(w.blockCalls(h.body, phaseTeardown, false), "", h.index, 1+depth-h.depth)
		case "unread":
			ex.unread = append(ex.unread, w.calls(h.body, false)...)
		}
	}
	w.examples = append(w.examples, ex)
}

// calls lists the calls of Ruby statements: a name followed by `(`, a declared name, and in
// teardown a bare name that starts a statement.
func (w *rbWalker) calls(lines []rubyLine, teardown bool) []tdCall {
	out := []tdCall{}
	for _, l := range lines {
		text := tdRubyString.ReplaceAllString(l.text, `""`)
		locs := tdRubyToken.FindAllStringIndex(text, -1)
		toks := make([]string, len(locs))
		for k, loc := range locs {
			toks[k] = text[loc[0]:loc[1]]
		}
		for k, tok := range toks {
			if !tdRubyName(tok) || strings.HasPrefix(tok, "@") || tdRubyNames[tok] || tdRubyAssertion(tok) {
				continue
			}
			start, end := locs[k][0], locs[k][1]
			if start > 0 && text[start-1] == ':' && (start < 2 || text[start-2] != ':') {
				continue // a symbol
			}
			if end < len(text) && text[end] == ':' && (end+1 >= len(text) || text[end+1] != ':') {
				continue // a hash key
			}
			root := k
			for root >= 2 && toks[root-1] == "." && tdRubyName(toks[root-2]) {
				root -= 2
			}
			paren := end < len(text) && text[end] == '('
			// In teardown a bare name starting a statement, or a method on a receiver that
			// starts one, is a call too.
			bare := teardown && ((k == 0 && (len(toks) == 1 || (toks[1] != "=" && toks[1] != "." && toks[1] != "+" && toks[1] != "-"))) || (k > 0 && root == 0))
			if !paren && !w.effects.declared(tok) && !bare {
				continue
			}
			c := tdCall{name: tok, line: l.line, mentions: map[string]bool{}}
			if root != k {
				c.mentions[toks[root]] = true
			}
			argsEnd := len(toks)
			if paren {
				depth := 0
				for j := k + 1; j < len(toks); j++ {
					if toks[j] == "(" {
						depth++
					} else if toks[j] == ")" {
						depth--
						if depth == 0 {
							argsEnd = j
							break
						}
					}
				}
			}
			for j := k + 1; j < argsEnd; j++ {
				if toks[j] == "do" || toks[j] == "{" {
					break
				}
				if tdRubyName(toks[j]) && !tdRubyNames[toks[j]] && toks[j-1] != "." && toks[j-1] != ":" {
					c.mentions[toks[j]] = true
				}
			}
			if root == 2 && toks[1] == "=" && tdRubyName(toks[0]) {
				c.assigned = toks[0]
			}
			out = append(out, c)
		}
	}
	return out
}

func tdRubyName(tok string) bool {
	t := strings.TrimLeft(tok, "@")
	return t != "" && (t[0] == '_' || unicode.IsLetter(rune(t[0])))
}
