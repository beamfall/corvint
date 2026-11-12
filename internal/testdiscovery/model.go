package testdiscovery

import (
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// Source is one lexed test or page-object file (TSA-V0-010).
type Source struct {
	Path     string
	Tokens   []appmap.JSToken
	Comments []appmap.JSComment
	// Match pairs every bracket with its partner; -1 when unbalanced.
	Match []int
	// Uncertain is true when the lexer could not place some source or a bracket is unbalanced,
	// so findings in this file may be incomplete.
	Uncertain bool
	// UncertainLine is the first line known to make the file uncertain: where the lexer met source
	// it cannot place, or the first unbalanced bracket; 0 when no line is known (TSA-V0-648).
	UncertainLine int
	Decls         []Decl
	// Defs indexes named function definitions, built on first use (TSA-V0-030).
	Defs map[string][]HelperDef
	// Abstentions collects the advisory readings the validity rules withheld because what
	// decides them is declared in another file (TSA-V0-300).
	Abstentions []testvalidity.RuleAbstention
	// Exemptions collects the readings a justified annotation exempted, with its reason
	// (TSA-V0-870).
	Exemptions []testvalidity.RuleExemption
	// assertNames are the local bindings of assertion modules and nodeTest whether the file imports
	// node:test, both read on first use (TSA-V0-649).
	assertNames map[string]bool
	nodeTest    bool
}

// Decl is one test, describe or runtime skip/only call.
type Decl struct {
	Kind      string // "test", "describe" or "annotation"
	Base      string
	Modifiers map[string]bool
	Title     string
	Titled    bool
	At        int // first token of the callee
	Open      int // the call's `(`
	Close     int // its matching `)`
	Args      [][2]int
	Body      [2]int // callback token range [start, end); empty for an annotation
	Line      int
	EndLine   int
}

var (
	testRoots     = map[string]bool{"test": true, "it": true, "specify": true, "xit": true, "xtest": true, "fit": true}
	describeRoots = map[string]bool{"describe": true, "context": true, "suite": true, "xdescribe": true, "xcontext": true, "fdescribe": true}
	modifierNames = map[string]bool{"only": true, "skip": true, "fixme": true, "fail": true, "slow": true, "serial": true, "parallel": true}
)

func ParseSource(name, text string) *Source {
	tokens, comments, unsureLine := appmap.LexJSUnsureLine(text)
	s := &Source{Path: name, Tokens: tokens, Comments: comments, Uncertain: unsureLine != 0, UncertainLine: unsureLine}
	doubt := func(line int) {
		s.Uncertain = true
		if s.UncertainLine == 0 || (line != 0 && line < s.UncertainLine) {
			s.UncertainLine = line
		}
	}
	s.Match = make([]int, len(tokens))
	stack := []int{}
	for i, t := range tokens {
		s.Match[i] = -1
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
			s.Match[open], s.Match[i] = i, open
		}
	}
	if len(stack) > 0 {
		doubt(tokens[stack[0]].Line)
	}
	s.Decls = s.declarations()
	return s
}

func closer(open string) string {
	return map[string]string{"(": ")", "[": "]", "{": "}"}[open]
}

func (s *Source) Punct(i int, text string) bool {
	return i >= 0 && i < len(s.Tokens) && s.Tokens[i].Kind == appmap.JSPunct && s.Tokens[i].Text == text
}

func (s *Source) Ident(i int, text string) bool {
	return i >= 0 && i < len(s.Tokens) && s.Tokens[i].Kind == appmap.JSIdent && (text == "" || s.Tokens[i].Text == text)
}

// MemberAccess reports that token i follows a `.` (or `?.`), so it names a property, not a root.
func (s *Source) MemberAccess(i int) bool { return s.Punct(i-1, ".") }

// Arrow reports a `=>` starting at token i.
func (s *Source) Arrow(i int) bool {
	return s.Punct(i, "=") && s.Punct(i+1, ">") && s.Tokens[i+1].Glued
}

// declarations finds every test, describe and runtime skip/only call (TSA-V0-010).
func (s *Source) declarations() []Decl {
	out := []Decl{}
	for i := range s.Tokens {
		if !s.Ident(i, "") || s.MemberAccess(i) || s.Ident(i-1, "function") {
			continue
		}
		root := s.Tokens[i].Text
		if !testRoots[root] && !describeRoots[root] {
			continue
		}
		d := Decl{Base: root, At: i, Modifiers: map[string]bool{}, Line: s.Tokens[i].Line}
		kind := "test"
		if describeRoots[root] {
			kind = "describe"
		}
		switch root[0] {
		case 'x':
			d.Modifiers["skip"] = true
		case 'f':
			d.Modifiers["only"] = true
		}
		j, other := i+1, false
		for s.Punct(j, ".") && s.Ident(j+1, "") {
			member := s.Tokens[j+1].Text
			switch {
			case member == "describe" && testRoots[root]:
				kind = "describe"
			case modifierNames[member]:
				d.Modifiers[member] = true
			default:
				other = true
			}
			j += 2
		}
		if other || !s.Punct(j, "(") || s.Match[j] < 0 {
			continue
		}
		d.Open, d.Close = j, s.Match[j]
		d.EndLine = s.Tokens[d.Close].Line
		d.Args = s.SplitArgs(d.Open, d.Close)
		callback := -1
		if len(d.Args) >= 2 && (s.Callback(d.Args[len(d.Args)-1]) || s.LoneString(d.Args[0])) {
			callback = len(d.Args) - 1
		}
		if callback < 0 {
			// A call with no callback is Playwright's runtime form, `test.skip(condition, reason)`.
			if d.Modifiers["skip"] || d.Modifiers["fixme"] || d.Modifiers["only"] {
				d.Kind = "annotation"
				out = append(out, d)
			}
			continue
		}
		d.Kind = kind
		d.Body = d.Args[callback]
		if s.LoneString(d.Args[0]) {
			t := s.Tokens[d.Args[0][0]]
			d.Title, d.Titled = titleText(t), t.Literal
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

// LoneString reports an argument that is one string or template token.
func (s *Source) LoneString(arg [2]int) bool {
	if arg[1]-arg[0] != 1 {
		return false
	}
	kind := s.Tokens[arg[0]].Kind
	return kind == appmap.JSString || kind == appmap.JSTemplate
}

// SplitArgs splits a call's arguments at top-level commas.
func (s *Source) SplitArgs(open, close int) [][2]int {
	args, start := [][2]int{}, open+1
	for i := open + 1; i < close; i++ {
		if s.Match[i] > i {
			i = s.Match[i]
			continue
		}
		if s.Punct(i, ",") {
			args = append(args, [2]int{start, i})
			start = i + 1
		}
	}
	if start < close {
		args = append(args, [2]int{start, close})
	}
	return args
}

// Callback reports an argument that is a function or arrow function.
func (s *Source) Callback(arg [2]int) bool {
	i := arg[0]
	if s.Ident(i, "async") {
		i++
	}
	if s.Ident(i, "function") {
		return true
	}
	if s.Punct(i, "(") && s.Match[i] > 0 {
		return s.Arrow(s.Match[i] + 1)
	}
	return s.Ident(i, "") && s.Arrow(i+1)
}

// Within reports token i inside the [start, end) range.
func Within(i int, r [2]int) bool { return i >= r[0] && i < r[1] }

// IsAssertion reports an assertion call at token i: an identifier beginning with expect, assert,
// verify or should that is called, or `expect.soft(` and `expect.poll(` (TSA-V0-013).
func (s *Source) IsAssertion(i int) bool {
	if !s.Ident(i, "") || s.Ident(i-1, "function") {
		return false
	}
	if s.Punct(i+1, "(") || s.Punct(i+1, ".") && s.Tokens[i].Text == "expect" {
		lower := strings.ToLower(s.Tokens[i].Text)
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
func (s *Source) moduleAssertion(i int) bool {
	if s.assertNames == nil {
		s.assertNames = map[string]bool{}
		for local, b := range s.importBindings() {
			if assertModules[b.Spec] || b.Spec == "chai" && b.imported == "assert" {
				s.assertNames[local] = true
			}
			s.nodeTest = s.nodeTest || b.Spec == "node:test"
		}
	}
	call := func(k int) bool { // `(` or up to three `.member` steps then `(`
		for steps := 0; steps <= 3; steps, k = steps+1, k+2 {
			if s.Punct(k, "(") {
				return true
			}
			if !s.Punct(k, ".") || !s.Ident(k+1, "") {
				return false
			}
		}
		return false
	}
	text := s.Tokens[i].Text
	if !s.MemberAccess(i) {
		return s.assertNames[text] && call(i+1)
	}
	return s.nodeTest && text == "assert" && s.Ident(i-2, "") && !s.MemberAccess(i-2) && s.Punct(i+1, ".") && call(i+1)
}

// ExpectChain reads the `expect(...)[.member[(...)]]*` call chain starting at token i: the
// subject range, the matcher name, the matcher's argument range and the chain's end token.
type ExpectChain struct {
	Subject [2]int
	Matcher string
	Margs   [2]int
	End     int
}

func (s *Source) ReadExpect(i int) (ExpectChain, bool) {
	if !s.Ident(i, "expect") || s.MemberAccess(i) {
		return ExpectChain{}, false
	}
	j := i + 1
	for s.Punct(j, ".") && s.Ident(j+1, "") && (s.Tokens[j+1].Text == "soft" || s.Tokens[j+1].Text == "poll") {
		j += 2
	}
	if !s.Punct(j, "(") || s.Match[j] < 0 {
		return ExpectChain{}, false
	}
	c := ExpectChain{Subject: [2]int{j + 1, s.Match[j]}, End: s.Match[j]}
	j = s.Match[j] + 1
	for s.Punct(j, ".") && s.Ident(j+1, "") {
		member := s.Tokens[j+1].Text
		j += 2
		c.End = j - 1
		if s.Punct(j, "(") && s.Match[j] > 0 {
			if member != "not" && member != "resolves" && member != "rejects" {
				c.Matcher, c.Margs = member, [2]int{j + 1, s.Match[j]}
			}
			c.End = s.Match[j]
			j = s.Match[j] + 1
		}
	}
	return c, true
}

// Render renders a token range canonically: strings quoted, words separated by one space, all
// other spacing dropped, so formatting differences do not change it (TSA-V0-020). A regular
// expression or a template with substitutions renders its source, so different patterns or
// substitution expressions never render alike.
func (s *Source) Render(start, end int) string {
	var b strings.Builder
	previousWord := false
	for i := start; i < end; i++ {
		t := s.Tokens[i]
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
