package testsource

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// The RSpec reader behind MEMOIZED-BLOCK-EXPECTATION (TSA-V0-048..050) and BLOCK-NOT-CALLED
// (TSA-V0-053..056). It is not a Ruby parser: it reads logical lines (rubyLines) for example-group
// nesting, the subject/let/let!/before declarations and the `def` helpers of each group and the
// block expectations of each example, and abstains on anything it cannot balance.

// maxRSpecDepth bounds group and block nesting.
const maxRSpecDepth = 64

var (
	rspecGroupLine   = regexp.MustCompile(`^(?:RSpec\.)?(?:[xf]?describe|[xf]?context|feature|shared_examples|shared_examples_for|shared_context|it_behaves_like|it_should_behave_like)\b`)
	rspecExampleLine = regexp.MustCompile(`^(?:[xf]?it|[xf]?specify|[xf]?example|[xf]?scenario|its|focus|skip|pending)\b`)
	rspecTitle       = regexp.MustCompile(`^\w+\s*\(?\s*(?:'([^'\\]*)'|"([^"\\]*)")`)
	// rspecHookLine matches a before hook; group 1 is its scope symbol, when it names one.
	rspecHookLine = regexp.MustCompile(`^(?:before|prepend_before|append_before)\b\s*\(?\s*(?::(\w+))?`)
	// rspecSkippedLine opens a block that can hold neither a declaration nor a memoizing call
	// the rule reads: a method, an around or after hook.
	rspecSkippedLine = regexp.MustCompile(`^(?:def|around|after|prepend_after|append_after)\b`)
	// rspecDeclLine matches let, let!, subject and subject!; group 2 is the declared name.
	rspecDeclLine   = regexp.MustCompile(`^(let!?|subject!?)\s*(?:\(\s*:(\w+[?!]?)\s*\)|\s+:(\w+[?!]?))?\s*(?:do\b|\{)`)
	rspecBlockExpr  = regexp.MustCompile(`\bexpect\s*\{\s*([a-z_]\w*[?!]?)\s*\}`)
	rspecBlockOpen  = regexp.MustCompile(`\bexpect\s*(?:\{|do)$`)
	rspecLoneName   = regexp.MustCompile(`^([a-z_]\w*[?!]?)$`)
	rspecBlockClose = regexp.MustCompile(`^(?:\}|end\b)`)
	rspecString     = regexp.MustCompile(`'(?:[^'\\]|\\.)*'|"(?:[^"\\]|\\.)*"`)
	// rspecPercent matches a percent literal (`%q(...)`, `%Q[...]`, `%w{...}`, `%i<...>`, `%(...)`)
	// with a bracket delimiter and no nested delimiter of the same kind.
	rspecPercent    = regexp.MustCompile(`%[qQwWiIsr]?(?:\([^()]*\)|\[[^\[\]]*\]|\{[^{}]*\}|<[^<>]*>)|%[qQwWiIsr](?:\|[^|]*\||![^!]*!|/[^/]*/)`)
	rspecBlockParam = regexp.MustCompile(`(?:\bdo|\{)\s*\|([^|]*)\|`)
	rspecIdentifier = regexp.MustCompile(`[A-Za-z_]\w*[?!]?`)
	rubyKeywords    = map[string]bool{"and": true, "begin": true, "break": true, "case": true, "class": true, "def": true, "do": true, "else": true, "elsif": true, "end": true, "ensure": true, "false": true, "for": true, "if": true, "in": true, "module": true, "next": true, "nil": true, "not": true, "or": true, "redo": true, "rescue": true, "retry": true, "return": true, "self": true, "super": true, "then": true, "true": true, "undef": true, "unless": true, "until": true, "when": true, "while": true, "yield": true}
)

type rspecGroup struct {
	parent int
	// lets and fixtures are every let, let!, subject and subject! with its body and every
	// `fixtures` line of the group, for the seed-parity reading (TSA-V0-101).
	lets     []rspecLet
	fixtures []rubyLine
	// decls maps a declared name to "let", "subject" (an anonymous subject) or "let:NAME" (the
	// `subject` alias of a named subject, which RSpec defines as a let).
	decls map[string]string
	hooks []rspecHook
	// bodies maps a let or named subject name, and "subject", to its block lines (TSA-V0-121).
	bodies map[string][]rubyLine
	// line and head are the group's declaring line and its text, for TSA-V0-063..065.
	line int
	head string
	// path is the file the group is declared in; loop marks a group declared inside a non-group
	// block (a loop or a conditional), whose RSpec index cannot be read (TSA-V0-091).
	path  string
	loop  bool
	index int // the index of the declaring line in the reader's lines
}

// rspecHook is one memoizing hook: a before(:each) hook, a let! or a subject!, with the names
// its block invokes. A let! or subject! hook calls its helper (own) by name, so its body's names
// count only while that helper is not overridden in a nested group.
type rspecHook struct {
	path  string
	line  int
	kind  string
	own   string
	names map[string]bool
	body  []rubyLine // the hook's lines, for TSA-V0-066
}

type rspecExample struct {
	group int
	path  string
	line  int
	title string
	body  []rubyLine
	loop  bool
	index int // the index of the declaring line in the reader's lines
}

type rspecReader struct {
	path     string
	depth    int  // open blocks around the line being read
	tooDeep  bool // the last block call failed on nesting depth
	lines    []rubyLine
	groups   []rspecGroup
	flow     rspecFlow
	examples []rspecExample
	helpers  []rspecHelper
	includes []rspecInclude
	// offGroupDefs are defs inside a module, class or other block: candidates the example
	// helper resolution cannot reach (TSA-V0-088).
	offGroupDefs []rspecHelper
	// support is the bound support set helpers resolve against, or nil when none was given
	// (TSA-V0-088).
	support *rspecSupport
	// flags is the flag lookup list stubs are classified against (TSA-V0-435).
	flags testvalidity.FlagLookups
	// app is the committed application source assertion targets resolve in, or nil (TSA-V0-448).
	app *testvalidity.RubyApp
	// reopened caches, per class and declaring file, whether other application source reopens
	// or patches the class (TSA-V0-447).
	reopened map[string]bool
	// excluded are the filter_run_excluding metadata filters of the bound support files
	// (TSA-V0-450).
	excluded []rspecExclusion
	// hookSites are every before, after and around hook with its group, for the static test
	// discovery (DCX-V1-375); the rules above read hooks only through groups[].hooks.
	hookSites []rspecHookSite
}

// rspecHookSite is one hook block: the group it is declared in, its index in lines, its block
// lines and whether it sits inside a non-group block.
type rspecHookSite struct {
	group, index int
	body         []rubyLine
	loop         bool
}

// rspecValidity reports every `expect { NAME }` block expectation whose NAME is the subject or a
// let that a memoizing hook of the example's own or an enclosing group already invoked
// (TSA-V0-048, TSA-V0-049), then every BLOCK-NOT-CALLED helper and the examples that call it
// (TSA-V0-053..055). It abstains, with no findings, when the file's blocks do not balance or nest
// too deeply (TSA-V0-050).
func rspecValidity(name, text string) ValidityReport {
	report := ValidityReport{Findings: []testvalidity.SourceFinding{}}
	r := newRSpecReader(name, text)
	if reason := r.read(); reason != "" {
		report.Uncertain, report.Reason = true, reason
		return report
	}
	for _, e := range r.examples {
		report.Findings = append(report.Findings, r.memoizedFindings(e)...)
	}
	report.Findings = append(report.Findings, r.blockNotCalled()...)
	return report
}

// newRSpecReader returns a reader over one file's logical lines, with the file-level root group.
func newRSpecReader(path, text string) *rspecReader {
	lines := rubyLines(text)
	for i := range lines {
		lines[i].path = path
	}
	return &rspecReader{path: path, lines: lines, groups: []rspecGroup{{parent: -1, decls: map[string]string{}, path: path}}}
}

// Frames of read that are not example groups.
const (
	frameOther   = -1 // any block that is not an example group: a loop, conditional, class or helper
	frameInclude = -2 // the customization block of an inline include, read as part of its group
)

// rspecIncludeLine is a shared group inclusion (TSA-V0-089).
var rspecIncludeLine = regexp.MustCompile(`^(?:RSpec\.)?(it_behaves_like|it_should_behave_like|include_examples|include_context)\b`)

// rspecInclude is one shared group inclusion: its index in lines, the group it is called in, the
// group it creates (it_behaves_like with a block) or -1, its kind and the shared group's name.
type rspecInclude struct {
	index, group, nested int
	line                 int
	path                 string
	kind, name           string
	literal, loop        bool
}

func (r *rspecReader) addInclude(group, i, nested int, loop bool) {
	text := r.lines[i].text
	kind := rspecIncludeLine.FindStringSubmatch(text)[1]
	name, literal := sharedName(strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(text, "RSpec."), kind)))
	r.includes = append(r.includes, rspecInclude{index: i, group: group, nested: nested, line: r.lines[i].line, path: r.lines[i].path, kind: kind, name: name, literal: literal, loop: loop})
}

// rspecSharedName is the first argument of a shared group definition or inclusion: a string
// literal without interpolation, or a symbol.
var rspecSharedName = regexp.MustCompile(`^\(?\s*(?:'([^'\\]*)'|"([^"\\#]*)"|:(\w+[?!]?))\s*(?:[,)]|do\b|\{|$)`)

// sharedName returns the literal name an argument list starts with, or false when it is not a
// plain literal.
func sharedName(args string) (string, bool) {
	m := rspecSharedName.FindStringSubmatch(args)
	if m == nil {
		return "", false
	}
	return m[1] + m[2] + m[3], true
}

// memoizedFindings returns the MEMOIZED-BLOCK-EXPECTATION findings of one example (TSA-V0-048).
func (r *rspecReader) memoizedFindings(e rspecExample) []testvalidity.SourceFinding {
	out := []testvalidity.SourceFinding{}
	for _, target := range blockExpectations(e.body) {
		canonical := r.canonical(e.group, target.name)
		if canonical == "" {
			continue
		}
		hook, kind, ok := r.memoizingHook(e.group, canonical)
		if !ok {
			continue
		}
		test := fmt.Sprintf("%s:%d", e.path, e.line)
		cause := fmt.Sprintf("%s:%d", hook.path, hook.line)
		out = append(out, testvalidity.SourceFinding{
			Rule: testvalidity.RuleMemoizedBlockExpectation, Severity: testvalidity.SeverityInvalid,
			Test: test, Title: e.title, Location: fmt.Sprintf("%s:%d", e.path, target.line), Cause: cause,
			Message: fmt.Sprintf("expect { %s } re-reads a value the %s at %s already memoized, so the block runs nothing and a change, raise_error or have_received expectation on it is vacuous; call the action inside the block (for example expect { %s.perform }) or stop invoking %s in that hook", target.name, kind, cause, target.name, target.name),
		})
	}
	return out
}

// read walks the logical lines once, building groups, their declarations and hooks, and the
// examples. It returns an uncertainty reason, or "" when every block balanced.
func (r *rspecReader) read() string {
	// group >= 0 for an example group, frameInclude for the customization block of an inline
	// include (read as part of the enclosing group), frameOther for any other block.
	type frame struct{ group int }
	stack := []frame{}
	inLoop := func() bool {
		for k := len(stack) - 1; k >= 0 && stack[k].group < 0; k-- {
			if stack[k].group == frameOther {
				return true
			}
		}
		return false
	}
	current := func() int {
		for k := len(stack) - 1; k >= 0; k-- {
			if stack[k].group >= 0 {
				return stack[k].group
			}
		}
		return 0
	}
	for i := 0; i < len(r.lines); i++ {
		r.depth = len(stack)
		text := r.lines[i].text
		if rubyEnd.MatchString(text) {
			if len(stack) == 0 {
				return validityRSpecUnbalanced
			}
			stack = stack[:len(stack)-1]
			continue
		}
		group := current()
		switch {
		case rspecGroupLine.MatchString(text) && rubyDo.MatchString(text):
			loop := inLoop() // read before the group's own frame hides the loop around it
			r.groups = append(r.groups, rspecGroup{parent: group, decls: map[string]string{}, line: r.lines[i].line, head: text, path: r.lines[i].path, loop: loop, index: i})
			stack = append(stack, frame{group: len(r.groups) - 1})
			if rspecIncludeLine.MatchString(text) {
				r.addInclude(group, i, len(r.groups)-1, loop)
			}
		case rspecIncludeLine.MatchString(text):
			r.addInclude(group, i, -1, inLoop())
			if rubyDo.MatchString(text) {
				stack = append(stack, frame{group: frameInclude})
			}
		case rspecExampleLine.MatchString(text):
			body, end, ok := r.block(i)
			if !ok {
				return r.blockFailure()
			}
			title := ""
			if m := rspecTitle.FindStringSubmatch(text); m != nil {
				title = m[1] + m[2]
			}
			r.examples = append(r.examples, rspecExample{group: group, path: r.lines[i].path, line: r.lines[i].line, title: title, body: body, loop: inLoop(), index: i})
			i = end
		case rspecDeclLine.MatchString(text):
			m := rspecDeclLine.FindStringSubmatch(text)
			body, end, ok := r.block(i)
			if !ok {
				return r.blockFailure()
			}
			kind, declared := strings.TrimSuffix(m[1], "!"), m[2]+m[3]
			g := &r.groups[group]
			letName := declared
			if letName == "" && kind == "subject" {
				letName = "subject"
			}
			g.lets = append(g.lets, rspecLet{name: letName, lazy: !strings.HasSuffix(m[1], "!"), body: body})
			switch {
			case kind == "let" && declared != "":
				g.decls[declared] = "let"
			case kind == "subject" && declared != "":
				g.decls[declared], g.decls["subject"] = "let", "let:"+declared
			case kind == "subject":
				g.decls["subject"] = "subject"
			}
			if g.bodies == nil {
				g.bodies = map[string][]rubyLine{}
			}
			if declared != "" {
				g.bodies[declared] = body
			}
			if kind == "subject" {
				g.bodies["subject"] = body
			}
			if strings.HasSuffix(m[1], "!") {
				// let!(:x) adds `before { x }`; subject! adds `before { subject }`.
				own := declared
				if kind == "subject" {
					own = "subject"
				}
				g.hooks = append(g.hooks, rspecHook{path: r.lines[i].path, line: r.lines[i].line, kind: m[1], own: own, names: invokedNames(body), body: body})
			}
			i = end
		case rspecHookLine.MatchString(text) && (rubyDo.MatchString(text) || strings.Contains(text, "{")):
			body, end, ok := r.block(i)
			if !ok {
				return r.blockFailure()
			}
			r.hookSites = append(r.hookSites, rspecHookSite{group: group, index: i, body: body, loop: inLoop()})
			switch rspecHookLine.FindStringSubmatch(text)[1] {
			case "", "each", "example":
				r.groups[group].hooks = append(r.groups[group].hooks, rspecHook{path: r.lines[i].path, line: r.lines[i].line, kind: "before", names: invokedNames(body), body: body})
			}
			i = end
		case rspecFixturesLine.MatchString(text) && !rubyOpens(text):
			r.groups[group].fixtures = append(r.groups[group].fixtures, r.lines[i])
		case rspecSkippedLine.MatchString(text):
			body, end, ok := r.block(i)
			if !ok {
				return r.blockFailure()
			}
			if !strings.HasPrefix(text, "def") {
				r.hookSites = append(r.hookSites, rspecHookSite{group: group, index: i, body: body, loop: inLoop()})
			}
			// A def directly in an example group or at file level is a helper an example can call;
			// one inside a class, module or other block is not read (TSA-V0-053).
			if strings.HasPrefix(text, "def") && (len(stack) == 0 || stack[len(stack)-1].group >= 0) {
				r.helpers = append(r.helpers, rspecHelper{group: group, path: r.lines[i].path, body: body})
			} else if strings.HasPrefix(text, "def") {
				r.offGroupDefs = append(r.offGroupDefs, rspecHelper{group: -1, path: r.lines[i].path, body: body})
			}
			i = end
		case rubyOpens(text):
			stack = append(stack, frame{group: frameOther})
		}
		if len(stack) > maxRSpecDepth {
			return validityRSpecTooDeep
		}
	}
	if len(stack) > 0 {
		return validityRSpecUnbalanced
	}
	return ""
}

// block returns the logical lines of the block opened on line i, the head line included, and the
// index of its last line. A line with no block (`it 'is pending'`, `def x = 1`) is its own block.
// ok is false when the block does not close or when its nesting, added to the r.depth blocks
// around it, exceeds maxRSpecDepth; blockFailure tells the two apart.
func (r *rspecReader) block(i int) ([]rubyLine, int, bool) {
	r.tooDeep = false
	text := r.lines[i].text
	end := -1
	if rubyOpens(text) {
		if end = rubyBlockEnd(r.lines, i); end >= len(r.lines) {
			end = -1
		}
	} else {
		depth := 0
		for j := i; j < len(r.lines) && end < 0; j++ {
			if depth = braceDepth(r.lines[j].text, depth); depth <= 0 {
				end = j
			}
		}
	}
	lines := r.lines[i:]
	if end >= 0 {
		lines = r.lines[i : end+1]
	}
	if r.depth+nestingDepth(lines) > maxRSpecDepth {
		r.tooDeep = true
		return nil, 0, false
	}
	if end < 0 {
		return nil, 0, false
	}
	return lines, end, true
}

// blockFailure is the uncertainty reason of the last failed block call.
func (r *rspecReader) blockFailure() string {
	if r.tooDeep {
		return validityRSpecTooDeep
	}
	return validityRSpecUnbalanced
}

// nestingDepth returns the deepest nesting of do/keyword blocks and braces within lines.
func nestingDepth(lines []rubyLine) int {
	depth, deepest := 0, 0
	for _, l := range lines {
		if rubyOpens(l.text) {
			depth++
		} else if rubyEnd.MatchString(l.text) {
			depth--
		}
		deepest = max(deepest, depth)
		for _, c := range stripRubyStrings(l.text) {
			switch c {
			case '{':
				depth++
				deepest = max(deepest, depth)
			case '}':
				depth--
			}
		}
	}
	return deepest
}

// stripRubyStrings replaces the line's percent literals and quoted string literals with `""`.
func stripRubyStrings(text string) string {
	return rspecString.ReplaceAllString(rspecPercent.ReplaceAllString(text, `""`), `""`)
}

// braceDepth adds the line's `{` and `}` outside string literals to depth.
func braceDepth(text string, depth int) int {
	for _, c := range stripRubyStrings(text) {
		switch c {
		case '{':
			depth++
		case '}':
			depth--
		}
	}
	return depth
}

// invokedNames returns the bare identifiers a hook body calls: words outside string literals,
// not a method on a receiver (`.x`), a symbol (`:x`), an instance or global variable, a hash key
// (`x:`), a keyword, a block parameter or a local assigned in the body (TSA-V0-048).
func invokedNames(body []rubyLine) map[string]bool {
	texts := make([]string, len(body))
	for k, l := range body {
		texts[k] = l.text
	}
	text := stripRubyStrings(strings.Join(texts, "\n"))
	locals := map[string]bool{}
	for _, m := range rspecBlockParam.FindAllStringSubmatch(text, -1) {
		for _, p := range strings.Split(m[1], ",") {
			locals[strings.Trim(strings.TrimSpace(p), "*&")] = true
		}
	}
	type word struct {
		name   string
		assign bool
	}
	words := []word{}
	for _, loc := range rspecIdentifier.FindAllStringIndex(text, -1) {
		name := text[loc[0]:loc[1]]
		if loc[0] > 0 && strings.ContainsRune(".:@$", rune(text[loc[0]-1])) || rubyKeywords[name] {
			continue
		}
		rest := strings.TrimLeft(text[loc[1]:], " \t")
		if strings.HasPrefix(rest, ":") && !strings.HasPrefix(rest, "::") {
			continue
		}
		assign := strings.HasPrefix(rest, "=") && !strings.HasPrefix(rest, "==") && !strings.HasPrefix(rest, "=~") && !strings.HasPrefix(rest, "=>")
		words = append(words, word{name: name, assign: assign})
	}
	for _, w := range words {
		if w.assign {
			locals[w.name] = true
		}
	}
	names := map[string]bool{}
	for _, w := range words {
		if !locals[w.name] {
			names[w.name] = true
		}
	}
	return names
}

type blockTarget struct {
	line int
	name string
}

// blockExpectations returns each `expect { NAME }` or multi-line `expect do NAME end` block
// expectation in an example's lines whose whole block is one bare name.
func blockExpectations(body []rubyLine) []blockTarget {
	out := []blockTarget{}
	for k, l := range body {
		for _, m := range rspecBlockExpr.FindAllStringSubmatch(stripRubyStrings(l.text), -1) {
			out = append(out, blockTarget{line: l.line, name: m[1]})
		}
		if rspecBlockOpen.MatchString(l.text) && k+2 < len(body) && rspecBlockClose.MatchString(body[k+2].text) {
			if m := rspecLoneName.FindStringSubmatch(body[k+1].text); m != nil {
				out = append(out, blockTarget{line: l.line, name: m[1]})
			}
		}
	}
	return out
}

// canonical resolves name from group g, innermost declaration first: "let:NAME" for a let or a
// named subject, and for `subject` when the innermost subject is named NAME; "subject" for an
// anonymous or undeclared `subject`; "" for a name no group in the chain declares, which is not a
// memoized helper. Distinct named subjects stay distinct helpers.
func (r *rspecReader) canonical(g int, name string) string {
	if rubyKeywords[name] {
		return ""
	}
	if d := r.declaring(g, name); d >= 0 {
		switch v := r.groups[d].decls[name]; v {
		case "let":
			return "let:" + name
		default:
			return v
		}
	}
	if name == "subject" {
		return "subject"
	}
	return ""
}

// declaring returns the innermost group on g's chain that declares name, or -1.
func (r *rspecReader) declaring(g int, name string) int {
	for ; g >= 0; g = r.groups[g].parent {
		if r.groups[g].decls[name] != "" {
			return g
		}
	}
	return -1
}

// memoizingHook returns the first hook, innermost group first, then in source order, of group g
// or an enclosing group that invokes the canonical helper, and its kind. A hook in a sibling group
// is never on the chain.
func (r *rspecReader) memoizingHook(g int, canonical string) (rspecHook, string, bool) {
	for start := g; g >= 0; g = r.groups[g].parent {
		for _, h := range r.groups[g].hooks {
			if h.own != "" {
				if r.canonical(start, h.own) == canonical {
					return h, h.kind, true
				}
				if d := r.declaring(start, h.own); d >= 0 && d != g {
					continue // the helper is overridden below this hook's group; its body never runs
				}
			}
			for name := range h.names {
				if r.canonical(start, name) == canonical {
					return h, h.kind, true
				}
			}
		}
	}
	return rspecHook{}, "", false
}
