package testdiscovery

import (
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// MaxRSpecDepth bounds group and block nesting.
const MaxRSpecDepth = 64

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
	RSpecBlockExpr  = regexp.MustCompile(`\bexpect\s*\{\s*([a-z_]\w*[?!]?)\s*\}`)
	RSpecBlockOpen  = regexp.MustCompile(`\bexpect\s*(?:\{|do)$`)
	RSpecLoneName   = regexp.MustCompile(`^([a-z_]\w*[?!]?)$`)
	RSpecBlockClose = regexp.MustCompile(`^(?:\}|end\b)`)
	rspecString     = regexp.MustCompile(`'(?:[^'\\]|\\.)*'|"(?:[^"\\]|\\.)*"`)
	// RSpecPercent matches a percent literal (`%q(...)`, `%Q[...]`, `%w{...}`, `%i<...>`, `%(...)`)
	// with a bracket delimiter and no nested delimiter of the same kind.
	RSpecPercent    = regexp.MustCompile(`%[qQwWiIsr]?(?:\([^()]*\)|\[[^\[\]]*\]|\{[^{}]*\}|<[^<>]*>)|%[qQwWiIsr](?:\|[^|]*\||![^!]*!|/[^/]*/)`)
	rspecBlockParam = regexp.MustCompile(`(?:\bdo|\{)\s*\|([^|]*)\|`)
	RSpecIdentifier = regexp.MustCompile(`[A-Za-z_]\w*[?!]?`)
	RubyKeywords    = map[string]bool{"and": true, "begin": true, "break": true, "case": true, "class": true, "def": true, "do": true, "else": true, "elsif": true, "end": true, "ensure": true, "false": true, "for": true, "if": true, "in": true, "module": true, "next": true, "nil": true, "not": true, "or": true, "redo": true, "rescue": true, "retry": true, "return": true, "self": true, "super": true, "then": true, "true": true, "undef": true, "unless": true, "until": true, "when": true, "while": true, "yield": true}
)

type RSpecGroup struct {
	Parent int
	// Lets and fixtures are every let, let!, subject and subject! with its body and every
	// `fixtures` line of the group, for the seed-parity reading (TSA-V0-101).
	Lets     []RSpecLet
	Fixtures []RubyLine
	// Decls maps a declared name to "let", "subject" (an anonymous subject) or "let:NAME" (the
	// `subject` alias of a named subject, which RSpec defines as a let).
	Decls map[string]string
	Hooks []RSpecHook
	// Bodies maps a let or named subject name, and "subject", to its block lines (TSA-V0-121).
	Bodies map[string][]RubyLine
	// Line and head are the group's declaring line and its text, for TSA-V0-063..065.
	Line int
	Head string
	// Path is the file the group is declared in; loop marks a group declared inside a non-group
	// block (a loop or a conditional), whose RSpec index cannot be read (TSA-V0-091).
	Path  string
	Loop  bool
	Index int // the index of the declaring line in the reader's lines
}

// RSpecHook is one memoizing hook: a before(:each) hook, a let! or a subject!, with the names
// its block invokes. A let! or subject! hook calls its helper (own) by name, so its body's names
// count only while that helper is not overridden in a nested group.
type RSpecHook struct {
	Path  string
	Line  int
	Kind  string
	Own   string
	Names map[string]bool
	Body  []RubyLine // the hook's lines, for TSA-V0-066
}

type RSpecExample struct {
	Group int
	Path  string
	Line  int
	Title string
	Body  []RubyLine
	Loop  bool
	Index int // the index of the declaring line in the reader's lines
}

type RSpecReader struct {
	Path     string
	depth    int  // open blocks around the line being read
	tooDeep  bool // the last block call failed on nesting depth
	Lines    []RubyLine
	Groups   []RSpecGroup
	Flow     rspecFlow
	Examples []RSpecExample
	Helpers  []RSpecHelper
	Includes []RSpecInclude
	// OffGroupDefs are defs inside a module, class or other block: candidates the example
	// helper resolution cannot reach (TSA-V0-088).
	OffGroupDefs []RSpecHelper
	// Support is the bound support set helpers resolve against, or nil when none was given
	// (TSA-V0-088).
	Support *RSpecSupport
	// Flags is the flag lookup list stubs are classified against (TSA-V0-435).
	Flags testvalidity.FlagLookups
	// App is the committed application source assertion targets resolve in, or nil (TSA-V0-448).
	App *testvalidity.RubyApp
	// Reopened caches, per class and declaring file, whether other application source reopens
	// or patches the class (TSA-V0-447).
	Reopened map[string]bool
	// Excluded are the filter_run_excluding metadata filters of the bound support files
	// (TSA-V0-450).
	Excluded []RSpecExclusion
	// hookSites are every before, after and around hook with its group, for the static test
	// discovery (DCX-V1-375); the rules above read hooks only through groups[].hooks.
	hookSites []rspecHookSite
}

// rspecHookSite is one hook block: the group it is declared in, its index in lines, its block
// lines and whether it sits inside a non-group block.
type rspecHookSite struct {
	group, index int
	body         []RubyLine
	loop         bool
}

// NewRSpecReader returns a reader over one file's logical lines, with the file-level root group.
func NewRSpecReader(path, text string) *RSpecReader {
	lines := RubyLines(text)
	for i := range lines {
		lines[i].Path = path
	}
	return &RSpecReader{Path: path, Lines: lines, Groups: []RSpecGroup{{Parent: -1, Decls: map[string]string{}, Path: path}}}
}

// Frames of read that are not example groups.
const (
	frameOther   = -1 // any block that is not an example group: a loop, conditional, class or helper
	frameInclude = -2 // the customization block of an inline include, read as part of its group
)

// RSpecIncludeLine is a shared group inclusion (TSA-V0-089).
var RSpecIncludeLine = regexp.MustCompile(`^(?:RSpec\.)?(it_behaves_like|it_should_behave_like|include_examples|include_context)\b`)

// RSpecInclude is one shared group inclusion: its index in lines, the group it is called in, the
// group it creates (it_behaves_like with a block) or -1, its kind and the shared group's name.
type RSpecInclude struct {
	Index, Group, Nested int
	Line                 int
	Path                 string
	Kind, Name           string
	Literal, Loop        bool
}

func (r *RSpecReader) addInclude(group, i, nested int, loop bool) {
	text := r.Lines[i].Text
	kind := RSpecIncludeLine.FindStringSubmatch(text)[1]
	name, literal := SharedName(strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(text, "RSpec."), kind)))
	r.Includes = append(r.Includes, RSpecInclude{Index: i, Group: group, Nested: nested, Line: r.Lines[i].Line, Path: r.Lines[i].Path, Kind: kind, Name: name, Literal: literal, Loop: loop})
}

// rspecSharedName is the first argument of a shared group definition or inclusion: a string
// literal without interpolation, or a symbol.
var rspecSharedName = regexp.MustCompile(`^\(?\s*(?:'([^'\\]*)'|"([^"\\#]*)"|:(\w+[?!]?))\s*(?:[,)]|do\b|\{|$)`)

// SharedName returns the literal name an argument list starts with, or false when it is not a
// plain literal.
func SharedName(args string) (string, bool) {
	m := rspecSharedName.FindStringSubmatch(args)
	if m == nil {
		return "", false
	}
	return m[1] + m[2] + m[3], true
}

// Read walks the logical lines once, building groups, their declarations and hooks, and the
// examples. It returns an uncertainty reason, or "" when every block balanced.
func (r *RSpecReader) Read() string {
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
	for i := 0; i < len(r.Lines); i++ {
		r.depth = len(stack)
		text := r.Lines[i].Text
		if RubyEnd.MatchString(text) {
			if len(stack) == 0 {
				return ValidityRSpecUnbalanced
			}
			stack = stack[:len(stack)-1]
			continue
		}
		group := current()
		switch {
		case rspecGroupLine.MatchString(text) && RubyDo.MatchString(text):
			loop := inLoop() // read before the group's own frame hides the loop around it
			r.Groups = append(r.Groups, RSpecGroup{Parent: group, Decls: map[string]string{}, Line: r.Lines[i].Line, Head: text, Path: r.Lines[i].Path, Loop: loop, Index: i})
			stack = append(stack, frame{group: len(r.Groups) - 1})
			if RSpecIncludeLine.MatchString(text) {
				r.addInclude(group, i, len(r.Groups)-1, loop)
			}
		case RSpecIncludeLine.MatchString(text):
			r.addInclude(group, i, -1, inLoop())
			if RubyDo.MatchString(text) {
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
			r.Examples = append(r.Examples, RSpecExample{Group: group, Path: r.Lines[i].Path, Line: r.Lines[i].Line, Title: title, Body: body, Loop: inLoop(), Index: i})
			i = end
		case rspecDeclLine.MatchString(text):
			m := rspecDeclLine.FindStringSubmatch(text)
			body, end, ok := r.block(i)
			if !ok {
				return r.blockFailure()
			}
			kind, declared := strings.TrimSuffix(m[1], "!"), m[2]+m[3]
			g := &r.Groups[group]
			letName := declared
			if letName == "" && kind == "subject" {
				letName = "subject"
			}
			g.Lets = append(g.Lets, RSpecLet{Name: letName, Lazy: !strings.HasSuffix(m[1], "!"), Body: body})
			switch {
			case kind == "let" && declared != "":
				g.Decls[declared] = "let"
			case kind == "subject" && declared != "":
				g.Decls[declared], g.Decls["subject"] = "let", "let:"+declared
			case kind == "subject":
				g.Decls["subject"] = "subject"
			}
			if g.Bodies == nil {
				g.Bodies = map[string][]RubyLine{}
			}
			if declared != "" {
				g.Bodies[declared] = body
			}
			if kind == "subject" {
				g.Bodies["subject"] = body
			}
			if strings.HasSuffix(m[1], "!") {
				// let!(:x) adds `before { x }`; subject! adds `before { subject }`.
				own := declared
				if kind == "subject" {
					own = "subject"
				}
				g.Hooks = append(g.Hooks, RSpecHook{Path: r.Lines[i].Path, Line: r.Lines[i].Line, Kind: m[1], Own: own, Names: InvokedNames(body), Body: body})
			}
			i = end
		case rspecHookLine.MatchString(text) && (RubyDo.MatchString(text) || strings.Contains(text, "{")):
			body, end, ok := r.block(i)
			if !ok {
				return r.blockFailure()
			}
			r.hookSites = append(r.hookSites, rspecHookSite{group: group, index: i, body: body, loop: inLoop()})
			switch rspecHookLine.FindStringSubmatch(text)[1] {
			case "", "each", "example":
				r.Groups[group].Hooks = append(r.Groups[group].Hooks, RSpecHook{Path: r.Lines[i].Path, Line: r.Lines[i].Line, Kind: "before", Names: InvokedNames(body), Body: body})
			}
			i = end
		case rspecFixturesLine.MatchString(text) && !RubyOpens(text):
			r.Groups[group].Fixtures = append(r.Groups[group].Fixtures, r.Lines[i])
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
				r.Helpers = append(r.Helpers, RSpecHelper{Group: group, Path: r.Lines[i].Path, Body: body})
			} else if strings.HasPrefix(text, "def") {
				r.OffGroupDefs = append(r.OffGroupDefs, RSpecHelper{Group: -1, Path: r.Lines[i].Path, Body: body})
			}
			i = end
		case RubyOpens(text):
			stack = append(stack, frame{group: frameOther})
		}
		if len(stack) > MaxRSpecDepth {
			return ValidityRSpecTooDeep
		}
	}
	if len(stack) > 0 {
		return ValidityRSpecUnbalanced
	}
	return ""
}

// block returns the logical lines of the block opened on line i, the head line included, and the
// index of its last line. A line with no block (`it 'is pending'`, `def x = 1`) is its own block.
// ok is false when the block does not close or when its nesting, added to the r.depth blocks
// around it, exceeds maxRSpecDepth; blockFailure tells the two apart.
func (r *RSpecReader) block(i int) ([]RubyLine, int, bool) {
	r.tooDeep = false
	text := r.Lines[i].Text
	end := -1
	if RubyOpens(text) {
		if end = RubyBlockEnd(r.Lines, i); end >= len(r.Lines) {
			end = -1
		}
	} else {
		depth := 0
		for j := i; j < len(r.Lines) && end < 0; j++ {
			if depth = braceDepth(r.Lines[j].Text, depth); depth <= 0 {
				end = j
			}
		}
	}
	lines := r.Lines[i:]
	if end >= 0 {
		lines = r.Lines[i : end+1]
	}
	if r.depth+nestingDepth(lines) > MaxRSpecDepth {
		r.tooDeep = true
		return nil, 0, false
	}
	if end < 0 {
		return nil, 0, false
	}
	return lines, end, true
}

// blockFailure is the uncertainty reason of the last failed block call.
func (r *RSpecReader) blockFailure() string {
	if r.tooDeep {
		return ValidityRSpecTooDeep
	}
	return ValidityRSpecUnbalanced
}

// nestingDepth returns the deepest nesting of do/keyword blocks and braces within lines.
func nestingDepth(lines []RubyLine) int {
	depth, deepest := 0, 0
	for _, l := range lines {
		if RubyOpens(l.Text) {
			depth++
		} else if RubyEnd.MatchString(l.Text) {
			depth--
		}
		deepest = max(deepest, depth)
		for _, c := range StripRubyStrings(l.Text) {
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

// StripRubyStrings replaces the line's percent literals and quoted string literals with `""`.
func StripRubyStrings(text string) string {
	return rspecString.ReplaceAllString(RSpecPercent.ReplaceAllString(text, `""`), `""`)
}

// braceDepth adds the line's `{` and `}` outside string literals to depth.
func braceDepth(text string, depth int) int {
	for _, c := range StripRubyStrings(text) {
		switch c {
		case '{':
			depth++
		case '}':
			depth--
		}
	}
	return depth
}

// InvokedNames returns the bare identifiers a hook body calls: words outside string literals,
// not a method on a receiver (`.x`), a symbol (`:x`), an instance or global variable, a hash key
// (`x:`), a keyword, a block parameter or a local assigned in the body (TSA-V0-048).
func InvokedNames(body []RubyLine) map[string]bool {
	texts := make([]string, len(body))
	for k, l := range body {
		texts[k] = l.Text
	}
	text := StripRubyStrings(strings.Join(texts, "\n"))
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
	for _, loc := range RSpecIdentifier.FindAllStringIndex(text, -1) {
		name := text[loc[0]:loc[1]]
		if loc[0] > 0 && strings.ContainsRune(".:@$", rune(text[loc[0]-1])) || RubyKeywords[name] {
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
