package testsource

import (
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// source is one lexed test or page-object file (TSA-V0-010).
type source struct {
	path     string
	tokens   []appmap.JSToken
	comments []appmap.JSComment
	// match pairs every bracket with its partner; -1 when unbalanced.
	match []int
	// uncertain is true when the lexer could not place some source or a bracket is unbalanced,
	// so findings in this file may be incomplete.
	uncertain bool
	// uncertainLine is the first line known to make the file uncertain: where the lexer met source
	// it cannot place, or the first unbalanced bracket; 0 when no line is known (TSA-V0-648).
	uncertainLine int
	decls         []decl
	// defs indexes named function definitions, built on first use (TSA-V0-030).
	defs map[string][]helperDef
	// abstentions collects the advisory readings the validity rules withheld because what
	// decides them is declared in another file (TSA-V0-300).
	abstentions []testvalidity.RuleAbstention
	// exemptions collects the readings a justified annotation exempted, with its reason
	// (TSA-V0-870).
	exemptions []testvalidity.RuleExemption
	// assertNames are the local bindings of assertion modules and nodeTest whether the file imports
	// node:test, both read on first use (TSA-V0-649).
	assertNames map[string]bool
	nodeTest    bool
}

// decl is one test, describe or runtime skip/only call.
type decl struct {
	kind      string // "test", "describe" or "annotation"
	base      string
	modifiers map[string]bool
	title     string
	titled    bool
	at        int // first token of the callee
	open      int // the call's `(`
	close     int // its matching `)`
	args      [][2]int
	body      [2]int // callback token range [start, end); empty for an annotation
	line      int
	endLine   int
}

var (
	testRoots     = map[string]bool{"test": true, "it": true, "specify": true, "xit": true, "xtest": true, "fit": true}
	describeRoots = map[string]bool{"describe": true, "context": true, "suite": true, "xdescribe": true, "xcontext": true, "fdescribe": true}
	modifierNames = map[string]bool{"only": true, "skip": true, "fixme": true, "fail": true, "slow": true, "serial": true, "parallel": true}
)

func parseSource(name, text string) *source {
	tokens, comments, unsureLine := appmap.LexJSUnsureLine(text)
	s := &source{path: name, tokens: tokens, comments: comments, uncertain: unsureLine != 0, uncertainLine: unsureLine}
	doubt := func(line int) {
		s.uncertain = true
		if s.uncertainLine == 0 || (line != 0 && line < s.uncertainLine) {
			s.uncertainLine = line
		}
	}
	s.match = make([]int, len(tokens))
	stack := []int{}
	for i, t := range tokens {
		s.match[i] = -1
		if t.Kind != appmap.JSPunct {
			continue
		}
		switch t.Text {
		case "(", "[", "{":
			stack = append(stack, i)
		case ")", "]", "}":
			if len(stack) == 0 || closer(tokens[stack[len(stack)-1]].Text) != t.Text {
				doubt(t.Line)
				stack = stack[:0]
				continue
			}
			open := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			s.match[open], s.match[i] = i, open
		}
	}
	if len(stack) > 0 {
		doubt(tokens[stack[0]].Line)
	}
	s.decls = s.declarations()
	return s
}

func closer(open string) string {
	return map[string]string{"(": ")", "[": "]", "{": "}"}[open]
}

func (s *source) punct(i int, text string) bool {
	return i >= 0 && i < len(s.tokens) && s.tokens[i].Kind == appmap.JSPunct && s.tokens[i].Text == text
}

func (s *source) ident(i int, text string) bool {
	return i >= 0 && i < len(s.tokens) && s.tokens[i].Kind == appmap.JSIdent && (text == "" || s.tokens[i].Text == text)
}

// memberAccess reports that token i follows a `.` (or `?.`), so it names a property, not a root.
func (s *source) memberAccess(i int) bool { return s.punct(i-1, ".") }

// arrow reports a `=>` starting at token i.
func (s *source) arrow(i int) bool {
	return s.punct(i, "=") && s.punct(i+1, ">") && s.tokens[i+1].Glued
}

// declarations finds every test, describe and runtime skip/only call (TSA-V0-010).
func (s *source) declarations() []decl {
	out := []decl{}
	for i := range s.tokens {
		if !s.ident(i, "") || s.memberAccess(i) || s.ident(i-1, "function") {
			continue
		}
		root := s.tokens[i].Text
		if !testRoots[root] && !describeRoots[root] {
			continue
		}
		d := decl{base: root, at: i, modifiers: map[string]bool{}, line: s.tokens[i].Line}
		kind := "test"
		if describeRoots[root] {
			kind = "describe"
		}
		switch root[0] {
		case 'x':
			d.modifiers["skip"] = true
		case 'f':
			d.modifiers["only"] = true
		}
		j, other := i+1, false
		for s.punct(j, ".") && s.ident(j+1, "") {
			member := s.tokens[j+1].Text
			switch {
			case member == "describe" && testRoots[root]:
				kind = "describe"
			case modifierNames[member]:
				d.modifiers[member] = true
			default:
				other = true
			}
			j += 2
		}
		if other || !s.punct(j, "(") || s.match[j] < 0 {
			continue
		}
		d.open, d.close = j, s.match[j]
		d.endLine = s.tokens[d.close].Line
		d.args = s.splitArgs(d.open, d.close)
		callback := -1
		if len(d.args) >= 2 && (s.callback(d.args[len(d.args)-1]) || s.loneString(d.args[0])) {
			callback = len(d.args) - 1
		}
		if callback < 0 {
			// A call with no callback is Playwright's runtime form, `test.skip(condition, reason)`.
			if d.modifiers["skip"] || d.modifiers["fixme"] || d.modifiers["only"] {
				d.kind = "annotation"
				out = append(out, d)
			}
			continue
		}
		d.kind = kind
		d.body = d.args[callback]
		if s.loneString(d.args[0]) {
			t := s.tokens[d.args[0][0]]
			d.title, d.titled = titleText(t), t.Literal
		}
		out = append(out, d)
	}
	return out
}

// titleText is a title token as a reader sees it: a template with substitutions keeps its source
// form, `${expr}` included, instead of the lexer's internal placeholder byte (TSA-V0-651).
func titleText(t appmap.JSToken) string {
	if t.Kind == appmap.JSTemplate && !t.Literal && strings.HasPrefix(t.Source, "`") {
		return strings.TrimSuffix(strings.TrimPrefix(t.Source, "`"), "`")
	}
	return t.Text
}

// loneString reports an argument that is one string or template token.
func (s *source) loneString(arg [2]int) bool {
	if arg[1]-arg[0] != 1 {
		return false
	}
	kind := s.tokens[arg[0]].Kind
	return kind == appmap.JSString || kind == appmap.JSTemplate
}

// splitArgs splits a call's arguments at top-level commas.
func (s *source) splitArgs(open, close int) [][2]int {
	args, start := [][2]int{}, open+1
	for i := open + 1; i < close; i++ {
		if s.match[i] > i {
			i = s.match[i]
			continue
		}
		if s.punct(i, ",") {
			args = append(args, [2]int{start, i})
			start = i + 1
		}
	}
	if start < close {
		args = append(args, [2]int{start, close})
	}
	return args
}

// callback reports an argument that is a function or arrow function.
func (s *source) callback(arg [2]int) bool {
	i := arg[0]
	if s.ident(i, "async") {
		i++
	}
	if s.ident(i, "function") {
		return true
	}
	if s.punct(i, "(") && s.match[i] > 0 {
		return s.arrow(s.match[i] + 1)
	}
	return s.ident(i, "") && s.arrow(i+1)
}

// within reports token i inside the [start, end) range.
func within(i int, r [2]int) bool { return i >= r[0] && i < r[1] }

// describes returns the literal titles of the describes enclosing declaration d, outermost first.
func (s *source) describes(d decl) []string {
	titles := []string{}
	for _, outer := range s.decls {
		if outer.kind == "describe" && outer.at < d.at && within(d.at, outer.body) {
			titles = append(titles, outer.title)
		}
	}
	return titles
}

// isAssertion reports an assertion call at token i: an identifier beginning with expect, assert,
// verify or should that is called, or `expect.soft(` and `expect.poll(` (TSA-V0-013).
func (s *source) isAssertion(i int) bool {
	if !s.ident(i, "") || s.ident(i-1, "function") {
		return false
	}
	if s.punct(i+1, "(") || s.punct(i+1, ".") && s.tokens[i].Text == "expect" {
		lower := strings.ToLower(s.tokens[i].Text)
		for _, prefix := range []string{"expect", "assert", "verify", "should"} {
			if strings.HasPrefix(lower, prefix) {
				return true
			}
		}
	}
	return s.moduleAssertion(i)
}

// assertModules are the modules whose bindings are assertion functions or objects (TSA-V0-649).
var assertModules = map[string]bool{"assert": true, "node:assert": true, "assert/strict": true, "node:assert/strict": true}

// moduleAssertion reports a call through a binding of an assertion module, such as
// `assert.equal(...)`, `assert.strict.ok(...)` or `strictEqual(...)` after an import from
// `node:assert`, Chai's imported `assert`, or the `t.assert.equal(...)` of a node:test context in a
// file that imports `node:test` (TSA-V0-649).
func (s *source) moduleAssertion(i int) bool {
	if s.assertNames == nil {
		s.assertNames = map[string]bool{}
		for local, b := range s.importBindings() {
			if assertModules[b.spec] || b.spec == "chai" && b.imported == "assert" {
				s.assertNames[local] = true
			}
			s.nodeTest = s.nodeTest || b.spec == "node:test"
		}
	}
	call := func(k int) bool { // `(` or up to three `.member` steps then `(`
		for steps := 0; steps <= 3; steps, k = steps+1, k+2 {
			if s.punct(k, "(") {
				return true
			}
			if !s.punct(k, ".") || !s.ident(k+1, "") {
				return false
			}
		}
		return false
	}
	text := s.tokens[i].Text
	if !s.memberAccess(i) {
		return s.assertNames[text] && call(i+1)
	}
	return s.nodeTest && text == "assert" && s.ident(i-2, "") && !s.memberAccess(i-2) && s.punct(i+1, ".") && call(i+1)
}

// expectChain reads the `expect(...)[.member[(...)]]*` call chain starting at token i: the
// subject range, the matcher name, the matcher's argument range and the chain's end token.
type expectChain struct {
	subject [2]int
	matcher string
	margs   [2]int
	end     int
}

func (s *source) readExpect(i int) (expectChain, bool) {
	if !s.ident(i, "expect") || s.memberAccess(i) {
		return expectChain{}, false
	}
	j := i + 1
	for s.punct(j, ".") && s.ident(j+1, "") && (s.tokens[j+1].Text == "soft" || s.tokens[j+1].Text == "poll") {
		j += 2
	}
	if !s.punct(j, "(") || s.match[j] < 0 {
		return expectChain{}, false
	}
	c := expectChain{subject: [2]int{j + 1, s.match[j]}, end: s.match[j]}
	j = s.match[j] + 1
	for s.punct(j, ".") && s.ident(j+1, "") {
		member := s.tokens[j+1].Text
		j += 2
		c.end = j - 1
		if s.punct(j, "(") && s.match[j] > 0 {
			if member != "not" && member != "resolves" && member != "rejects" {
				c.matcher, c.margs = member, [2]int{j + 1, s.match[j]}
			}
			c.end = s.match[j]
			j = s.match[j] + 1
		}
	}
	return c, true
}

// render renders a token range canonically: strings quoted, words separated by one space, all
// other spacing dropped, so formatting differences do not change it (TSA-V0-020). A regular
// expression or a template with substitutions renders its source, so different patterns or
// substitution expressions never render alike.
func (s *source) render(start, end int) string {
	var b strings.Builder
	previousWord := false
	for i := start; i < end; i++ {
		t := s.tokens[i]
		word := t.Kind == appmap.JSIdent || t.Kind == appmap.JSNumber
		if word && previousWord {
			b.WriteByte(' ')
		}
		switch {
		case t.Source != "":
			b.WriteString(t.Source)
		case t.Kind == appmap.JSString:
			b.WriteString(strconv.Quote(t.Text))
		case t.Kind == appmap.JSTemplate:
			b.WriteString("`" + t.Text + "`")
		default:
			b.WriteString(t.Text)
		}
		previousWord = word
	}
	return b.String()
}
