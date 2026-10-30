package testsource

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// The RSpec reading of RB-ASSERT-TARGET-NEVER-WRITTEN (TSA-V0-443..449). Unlike TSA-V0-430..442 it
// reads the code under test: the committed class an example's receiver is built from and the
// methods the example calls on it. It reports only a target it resolves completely; every form it
// cannot read gives no finding, never a finding.

const (
	maxAppSuperclasses = 4   // superclass hops from the receiver's class (TSA-V0-445)
	maxAppMethods      = 256 // method names visited from the exercised calls (TSA-V0-446)
)

var (
	// rspecReaderTarget is an assertion's actual that is one reader call on a local name.
	rspecReaderTarget = regexp.MustCompile(`^([a-z_]\w*)\.([a-z_]\w*)$`)
	// rspecNewValue is a binding built by `.new`; group 1 is the class or described_class.
	rspecNewValue = regexp.MustCompile(`^(described_class|(?:::)?[A-Z]\w*(?:::[A-Z]\w*)*)\s*\.\s*new\b`)
	// rspecOutsideChain is a declaration whose code the reader does not see on the example's
	// path: an around or before(:all) hook, or an included shared context or example group.
	rspecOutsideChain = regexp.MustCompile(`^(?:(?:prepend_|append_)?around\b|(?:prepend_|append_)?before\b\s*\(?\s*:(?:all|context|suite)\b|include_context\b|include_examples\b|it_behaves_like\b|it_should_behave_like\b)`)
	// rubyAssignOp follows an assignment target: `=`, `||=`, `&&=` or an operator assignment.
	rubyAssignOp     = `\s*(?:[-+*/%|&^]|\*\*|<<|>>|\|\||&&)?=(?:[^=~>]|$)`
	rubyAssignAfter  = regexp.MustCompile(`^` + rubyAssignOp)
	rubyClassDecl    = regexp.MustCompile(`^class\s+((?:::)?[A-Z]\w*(?:::[A-Z]\w*)*)\s*(?:<\s*((?:::)?[A-Z]\w*(?:::[A-Z]\w*)*)\s*)?$`)
	rubyModuleDecl   = regexp.MustCompile(`^module\s+((?:::)?[A-Z]\w*(?:::[A-Z]\w*)*)\s*$`)
	rubySingleton    = regexp.MustCompile(`^class\s*<<\s*self$`)
	rubyVisibleDef   = regexp.MustCompile(`^(?:private|protected|public)\s+(def\b.*)$`)
	rubyDefName      = regexp.MustCompile(`^def\s+(self\s*\.\s*)?([A-Za-z_]\w*[?!=]?)`)
	rubyAttrDecl     = regexp.MustCompile(`^(attr_accessor|attr_reader|attr_writer)\b\s*\(?(.*?)\)?$`)
	rubyAttrSymbol   = regexp.MustCompile(`^:([a-z_]\w*)$`)
	rubyCallbackDecl = regexp.MustCompile(`^(?:(?:before|after|around)_\w+|validate)\b`)
	rubyBlockValue   = regexp.MustCompile(`\{|->|\blambda\b|\bproc\b|\bProc\b|\bdo\b`)
	// rubyInertDecl is a class-level statement that defines no instance state: a visibility
	// change, a constant, a table setting, an association, a validation or a scope.
	rubyInertDecl = regexp.MustCompile(`^(?:(?:private|protected|public)(?:\s+:\w+[?!=]?(?:\s*,\s*:\w+[?!=]?)*)?$|[A-Z][A-Z0-9_]*\s*=[^=]|self\s*\.\s*(?:abstract_class|table_name|primary_key)\s*=[^=]|(?:belongs_to|has_many|has_one|has_and_belongs_to_many|validates|validates_\w+|scope)\b)`)
	// rubyDynamicCall is metaprogramming a write or a call can hide behind.
	rubyDynamicCall = regexp.MustCompile(`\b(?:send|public_send|__send__|instance_variable_set|instance_eval|instance_exec|class_eval|class_exec|module_eval|define_method|define_singleton_method|eval|binding|method_missing)\b`)
	rubySelfWord    = regexp.MustCompile(`\bself\b`)
	rubySuperWord   = regexp.MustCompile(`(?:^|[^.:\w])super\b`)
	rubySelfCall    = regexp.MustCompile(`\bself\s*\.\s*([A-Za-z_]\w*[?!]?)`)
	rubySymbolRef   = regexp.MustCompile(`(?:^|[^:\w]):([A-Za-z_]\w*[?!=]?)`)
	// rubyMassAssign are the record methods that write attributes from a hash.
	rubyMassAssign = map[string]bool{"assign_attributes": true, "attributes": true, "update": true, "update!": true,
		"update_attributes": true, "update_attributes!": true, "write_attribute": true, "update_column": true, "update_columns": true}
	rspecRecordBase = map[string]bool{"ApplicationRecord": true, "ActiveRecord::Base": true}
	// rubyClassNew is a class-level `new` or `allocate` definition, which can build the receiver
	// some other way than `initialize` (TSA-V0-447).
	rubyClassNew   = regexp.MustCompile(`^def\s+(?:self|[A-Z]\w*(?:::[A-Z]\w*)*)\s*\.\s*(?:new|allocate)\b`)
	rubyNewWord    = regexp.MustCompile(`\b(?:new|allocate)\b`)
	rubyBareAssign = regexp.MustCompile(`(?:^|[^=!<>])=(?:[^=~>]|$)`)
)

// appMethod is one instance method of a class on the receiver's chain.
type appMethod struct {
	class int // index in the chain, the receiver's class first
	path  string
	lines []rubyLine // the def line through its end
}

// appClass is one class of the receiver's chain read from committed application source.
type appClass struct {
	name, path string
	readers    map[string]rubyLine // attr_accessor and attr_reader names, with the declaring line
	kinds      map[string]string   // the declaring attr_* of each reader
	writers    map[string]bool     // attr_accessor and attr_writer names
	defLines   map[string][][]rubyLine
	callbacks  []string
	mentions   bool // a class-level statement other than attr_* names the reader
}

// appChain is the receiver's class and its superclasses; record marks a chain ending at an
// ActiveRecord base.
type appChain struct {
	classes []appClass
	record  bool
}

// accessorTarget is one asserted reader and the object it is read from.
type accessorTarget struct {
	a       rubyAssertion
	root    string
	reader  string
	aliases map[string]bool
	class   string
	local   int    // the body line of a local binding, or 0
	args    string // the masked `.new` arguments
	rawArgs string // the same arguments unmasked
}

// neverWritten returns example e's TSA-V0-444 findings in assertion line order (TSA-V0-443).
func (c *flowContext) neverWritten(test string) []testvalidity.SourceFinding {
	out := []testvalidity.SourceFinding{}
	if c.r.app == nil || c.e.path != c.name || c.outsideChain() {
		return out
	}
	for _, a := range rubyAssertions(c.exampleLines()) {
		if a.self {
			continue
		}
		actual := a.t.masked[a.actual[0]:a.actual[1]]
		m := rspecReaderTarget.FindStringSubmatch(a.t.squash(a.actual[0], a.actual[1]))
		if m == nil || strings.ContainsRune(actual, 1) || rubyKeywords[m[1]] || rubyKeywords[m[2]] {
			continue
		}
		target, ok := c.target(a, m[1], m[2])
		if !ok {
			continue
		}
		if f, ok := c.neverWrittenFinding(test, target); ok {
			out = append(out, f)
		}
	}
	return out
}

// outsideChain reports a declaration in the file whose code could exercise the receiver out of
// the reader's sight (TSA-V0-447).
func (c *flowContext) outsideChain() bool {
	for _, l := range c.r.lines {
		if rspecOutsideChain.MatchString(l.text) {
			return true
		}
	}
	return false
}

// describedClass is the innermost constant group description on e's chain.
func (c *flowContext) describedClass() string {
	for g := c.e.group; g > 0; g = c.r.groups[g].parent {
		if m := rspecGroupTitle.FindStringSubmatch(c.r.groups[g].head); m != nil && m[3] != "" {
			return strings.TrimPrefix(m[3], "::")
		}
	}
	return ""
}

// target resolves the assertion's root to the class its value is built from (TSA-V0-445): a let,
// named or anonymous subject, or local assignment whose whole value is `Class.new(...)` or
// `described_class.new(...)`, or the implicit subject of a described class.
func (c *flowContext) target(a rubyAssertion, root, reader string) (accessorTarget, bool) {
	out := accessorTarget{a: a, root: root, reader: reader, aliases: map[string]bool{root: true}}
	name := root
	if root == "subject" {
		g := c.r.declaring(c.e.group, "subject")
		if g < 0 {
			out.class = c.describedClass()
			return out, out.class != ""
		}
		v := c.r.groups[g].decls["subject"]
		if !strings.HasPrefix(v, "let:") {
			value, span, ok := blockValue(c.r.groups[g].bodies["subject"])
			if !ok {
				return out, false
			}
			return out, c.newValue(&out, value, span)
		}
		name = strings.TrimPrefix(v, "let:")
		out.aliases[name] = true
	} else if c.r.canonical(c.e.group, "subject") == "let:"+name {
		out.aliases["subject"] = true
	}
	b, ok := c.binding(name, a.t.line)
	if !ok {
		return out, false
	}
	for _, l := range c.e.body[1:] {
		if l.line == b.line {
			out.local = b.line
		}
	}
	return out, c.newValue(&out, b.value, b.span)
}

// blockValue is the one value of a let or subject block, as binding reads it.
func blockValue(body []rubyLine) (rubyText, [2]int, bool) {
	switch {
	case len(body) == 1:
		t := maskRuby(body[0])
		open := strings.Index(t.masked, "{")
		close := strings.LastIndex(t.masked, "}")
		if open < 0 || close <= open || balanced(t.masked, open) != close {
			return rubyText{}, [2]int{}, false
		}
		return t, trimSpan(t.masked, open+1, close), true
	case len(body) == 3 && rspecBlockClose.MatchString(body[2].text):
		t := maskRuby(body[1])
		return t, [2]int{0, len(t.masked)}, true
	}
	return rubyText{}, [2]int{}, false
}

// newValue reads a value that is exactly `Class.new` with an optional parenthesised argument list.
func (c *flowContext) newValue(out *accessorTarget, t rubyText, span [2]int) bool {
	m := t.masked[span[0]:span[1]]
	loc := rspecNewValue.FindStringSubmatchIndex(m)
	if loc == nil {
		return false
	}
	class := strings.TrimPrefix(m[loc[2]:loc[3]], "::")
	if class == "described_class" {
		class = c.describedClass()
	}
	rest := strings.TrimLeft(m[loc[1]:], " \t")
	switch {
	case rest == "":
	case strings.HasPrefix(rest, "("):
		open := len(m) - len(rest)
		if balanced(m, open) != len(m)-1 {
			return false
		}
		out.args = m[open+1 : len(m)-1]
		out.rawArgs = t.raw[span[0]+open+1 : span[0]+len(m)-1]
	default:
		return false
	}
	out.class = class
	return class != ""
}

// exercisedCall is one method the example calls on the receiver; asserted marks a call on a line
// holding an `expect(...)` or `is_expected` expectation, which reads the receiver rather than
// exercising it.
type exercisedCall struct {
	method   string
	line     int
	asserted bool
}

// exercised returns the methods the example's hooks and body call on the target before the
// assertion, and false when the target is read some other way: passed, stubbed, yielded or used in
// another let (TSA-V0-447).
func (c *flowContext) exercised(target accessorTarget) ([]exercisedCall, bool) {
	r := c.r
	for g := c.e.group; g >= 0; g = r.groups[g].parent {
		for name, body := range r.groups[g].bodies {
			if target.aliases[name] {
				continue
			}
			for _, l := range body {
				for _, w := range rubyWords(maskRuby(l).masked) {
					if target.aliases[w] {
						return nil, false
					}
				}
			}
		}
	}
	lines := []rubyLine{}
	chain := []int{}
	for g := c.e.group; g >= 0; g = r.groups[g].parent {
		chain = append(chain, g)
	}
	slices.Reverse(chain)
	for _, g := range chain {
		for _, h := range r.groups[g].hooks {
			lines = append(lines, h.body...)
		}
	}
	for _, l := range c.exampleLines() {
		if l.line <= target.a.t.line {
			lines = append(lines, l)
		}
	}
	asserted := map[int]bool{}
	for _, a := range rubyAssertions(lines) {
		asserted[a.t.line] = true
	}
	out := []exercisedCall{}
	for _, l := range lines {
		t := maskRuby(l)
		m := t.masked
		for _, loc := range rspecIdentifier.FindAllStringIndex(m, -1) {
			if !target.aliases[m[loc[0]:loc[1]]] || loc[0] > 0 && strings.ContainsRune(".:@$", rune(m[loc[0]-1])) {
				continue
			}
			rest := m[loc[1]:]
			if trimmed := strings.TrimLeft(rest, " \t"); strings.HasPrefix(trimmed, ":") && !strings.HasPrefix(trimmed, "::") {
				continue
			}
			if l.line == target.local && strings.TrimSpace(m[:loc[0]]) == "" && rubyAssignAfter.MatchString(rest) {
				continue
			}
			call := rspecTailCall.FindStringSubmatch(rest)
			if call == nil {
				return nil, false
			}
			method := call[1]
			if rubyAssignAfter.MatchString(rest[len(call[0]):]) {
				method += "="
			}
			if method != target.reader {
				out = append(out, exercisedCall{method: method, line: l.line, asserted: asserted[l.line]})
			}
		}
	}
	return out, true
}

// writesReader reports a write of reader in masked text: an instance variable or attribute
// assignment, a target of a multiple assignment (`@reader, x = ...`, `self.reader, x = ...`), a
// rescue target (`rescue => @reader`) or a `for` variable.
func writesReader(reader string) func(masked string) bool {
	q := regexp.QuoteMeta(reader)
	target := `(?:(?:^|[^@\w])@|\.\s*)` + q
	single := regexp.MustCompile(target + rubyAssignOp + `|=>\s*@` + q + `\b|\bfor\b.*(?:^|[^@\w])@` + q + `\b`)
	listed := regexp.MustCompile(target + `\s*[,)]`)
	return func(masked string) bool {
		if single.MatchString(masked) {
			return true
		}
		for _, loc := range listed.FindAllStringIndex(masked, -1) {
			if rubyBareAssign.MatchString(masked[loc[1]:]) {
				return true
			}
		}
		return false
	}
}

// namesReader is a symbol, hash key or string naming reader, which a mass assignment or a
// reflective write can use (TSA-V0-446).
func namesReader(reader string) func(t rubyText) bool {
	q := regexp.QuoteMeta(reader)
	code := regexp.MustCompile(`(?:^|[^:\w]):` + q + `(?:[^\w?!]|$)|(?:^|[^\w:])` + q + `:(?:[^:]|$)`)
	literal := regexp.MustCompile(`['"]@?` + q + `=?['"]`)
	return func(t rubyText) bool { return code.MatchString(t.masked) || literal.MatchString(t.raw) }
}

// neverWrittenFinding reads the target's class and the methods the example exercises, and returns
// the finding when the asserted reader is a plain accessor no reachable code writes.
func (c *flowContext) neverWrittenFinding(test string, target accessorTarget) (testvalidity.SourceFinding, bool) {
	none := testvalidity.SourceFinding{}
	writes, names := writesReader(target.reader), namesReader(target.reader)
	// A string literal among the `.new` arguments can name the reader in any spelling
	// (`"reader" => v`, `"reader": v`), so it withholds like a symbol key does.
	if names(rubyText{raw: target.rawArgs, masked: target.args}) || strings.ContainsAny(target.args, "\"'`\x01") || strings.Contains(target.args, "<<") {
		return none, false
	}
	// Without a complete support set an unseen helper could write the receiver.
	if c.r.supportGap() != "" || c.callsHelper(target) {
		return none, false
	}
	calls, ok := c.exercised(target)
	if !ok {
		return none, false
	}
	// A write of the reader in the example or its hooks, on any receiver, is a write.
	for g := c.e.group; g >= 0; g = c.r.groups[g].parent {
		for _, h := range c.r.groups[g].hooks {
			for _, l := range h.body {
				if writes(maskRuby(l).masked) {
					return none, false
				}
			}
		}
	}
	for _, l := range c.exampleLines() {
		if l.line <= target.a.t.line && writes(maskRuby(l).masked) {
			return none, false
		}
	}
	chain, ok := c.appChain(target.class, target.reader)
	// A record's `.new` with any argument assigns attributes through their setters.
	if !ok || chain.record && strings.TrimSpace(target.args) != "" {
		return none, false
	}
	declared := -1
	for i, class := range chain.classes {
		if class.mentions || len(class.defLines[target.reader]) > 0 {
			return none, false
		}
		if _, ok := class.readers[target.reader]; ok && declared < 0 {
			declared = i
		}
	}
	if declared < 0 {
		return none, false
	}
	methods := func(name string) []appMethod {
		out := []appMethod{}
		for i, class := range chain.classes {
			for _, lines := range class.defLines[name] {
				out = append(out, appMethod{class: i, path: class.path, lines: lines})
			}
		}
		return out
	}
	resolved := func(name string) bool {
		if len(methods(name)) > 0 {
			return true
		}
		for _, class := range chain.classes {
			if _, ok := class.readers[name]; ok {
				return true
			}
			if base, ok := strings.CutSuffix(name, "="); ok && class.writers[base] {
				return true
			}
		}
		return false
	}
	queue := []string{"initialize"}
	shown, cause := []string{}, 0
	for _, call := range calls {
		if !resolved(call.method) {
			return none, false
		}
		queue = append(queue, call.method)
		if call.asserted || len(methods(call.method)) == 0 || slices.Contains(shown, call.method) {
			continue
		}
		if shown = append(shown, call.method); cause == 0 {
			cause = call.line
		}
	}
	if len(shown) == 0 {
		return none, false
	}
	for _, class := range chain.classes {
		queue = append(queue, class.callbacks...)
	}
	visited := map[string]bool{}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if visited[name] {
			continue
		}
		if visited[name] = true; len(visited) > maxAppMethods {
			return none, false
		}
		for _, method := range methods(name) {
			for _, l := range method.lines {
				t := maskRuby(l)
				m := t.masked
				if writes(m) || names(t) {
					return none, false
				}
				if rubyDynamicCall.MatchString(m) {
					return none, false
				}
				for _, loc := range rubySelfWord.FindAllStringIndex(m, -1) {
					if loc[0] > 0 && strings.ContainsRune(".:@$", rune(m[loc[0]-1])) {
						continue
					}
					if !strings.HasPrefix(strings.TrimLeft(m[loc[1]:], " \t"), ".") {
						return none, false
					}
				}
				if rubySuperWord.MatchString(m) && !superResolved(chain, name, method.class) {
					return none, false
				}
				for _, w := range rubyWords(m) {
					if chain.record && rubyMassAssign[w] {
						return none, false
					}
					queue = append(queue, w)
				}
				for _, s := range rubySelfCall.FindAllStringSubmatchIndex(m, -1) {
					called := m[s[2]:s[3]]
					if chain.record && rubyMassAssign[called] {
						return none, false
					}
					queue = append(queue, called)
					if rubyAssignAfter.MatchString(m[s[1]:]) {
						queue = append(queue, called+"=")
					}
				}
				for _, s := range rubySymbolRef.FindAllStringSubmatch(m, -1) {
					queue = append(queue, s[1])
				}
			}
		}
	}
	if c.reopened(chain) {
		return none, false
	}
	class := chain.classes[declared]
	decl := class.readers[target.reader]
	exercised := make([]string, 0, len(shown))
	for _, method := range shown {
		exercised = append(exercised, chain.classes[0].name+"#"+method)
	}
	sibling, hint := "", ""
	if predicate := methods(target.reader + "?"); len(predicate) > 0 {
		p := predicate[0]
		sibling = fmt.Sprintf("; the class defines the sibling predicate %s#%s? at %s:%d, which may hold the state the assertion means",
			chain.classes[p.class].name, target.reader, p.path, p.lines[0].line)
		hint = fmt.Sprintf(" (for example %s.%s?)", target.root, target.reader)
	}
	return c.finding(testvalidity.RuleRBAssertTargetNeverWritten, testvalidity.SeverityAbstained, test, target.a.t.line, cause, fmt.Sprintf(
		"assertion target is never written on the exercised path: the assertion at %s (%s) reads %s.%s, the plain %s :%s that %s declares at %s:%d, and nothing reachable from %s, which the example exercises at %s, writes it (no @%s or attribute assignment, %s= call or mass assignment of :%s), so the assertion reads the accessor's initial value whatever that code does%s; hint: assert the state the code writes%s, or write the accessor on that path; the code under test is read statically and not run, so the example abstains instead of being refused",
		c.at(target.a.t.line), lineText(target.a.t), target.root, target.reader, class.kinds[target.reader], target.reader, class.name, class.path, decl.line,
		strings.Join(exercised, ", "), c.at(cause), target.reader, target.reader, target.reader, sibling, hint)), true
}

// superResolved reports whether `super` in method name of chain class index from reaches a known
// method: a later class's def, or a plain object's Object base.
func superResolved(chain appChain, name string, from int) bool {
	for i := from + 1; i < len(chain.classes); i++ {
		if len(chain.classes[i].defLines[name]) > 0 {
			return true
		}
	}
	return !chain.record
}

// callsHelper reports a hook or example line on the path, or a let or subject body of the
// example's groups, that calls or names a def of the spec file or its support set: a helper whose
// body the reader does not expand could write the receiver (TSA-V0-447).
func (c *flowContext) callsHelper(target accessorTarget) bool {
	helpers := map[string]bool{}
	for _, sr := range c.r.sharedReaders() {
		for _, h := range append(append([]rspecHelper{}, sr.offGroupDefs...), sr.helpers...) {
			if name := helperName(h.body); name != "" {
				helpers[name] = true
			}
		}
	}
	if len(helpers) == 0 {
		return false
	}
	lines := []rubyLine{}
	for g := c.e.group; g >= 0; g = c.r.groups[g].parent {
		for _, h := range c.r.groups[g].hooks {
			lines = append(lines, h.body...)
		}
		for _, body := range c.r.groups[g].bodies {
			lines = append(lines, body...)
		}
	}
	for _, l := range c.exampleLines() {
		if l.line <= target.a.t.line {
			lines = append(lines, l)
		}
	}
	for _, l := range lines {
		m := maskRuby(l).masked
		for _, w := range rubyWords(m) {
			if helpers[w] {
				return true
			}
		}
		for _, s := range rubySymbolRef.FindAllStringSubmatch(m, -1) {
			if helpers[s[1]] {
				return true
			}
		}
	}
	return false
}

// rubyPatchCall is a call that reopens or patches a class from outside its declaration.
const rubyPatchCall = `(?:class_eval|class_exec|module_eval|module_exec|include|prepend|extend|send|__send__|public_send|define_method|alias_method|attr_accessor|attr_writer|attr_reader|instance_variable_set|singleton_class)`

// reopened reports whether application source other than each chain class's one declaration
// reopens or patches it: a `class` or `module` line naming the constant, or a patch call on it,
// in any committed app/, lib/ or config/ file. Every listed file must read within the command's
// budget; one that does not counts as a reopening (TSA-V0-447).
func (c *flowContext) reopened(chain appChain) bool {
	app := c.r.app
	if app.ReadAll == nil || !app.ReadAll() {
		return true
	}
	if c.r.reopened == nil {
		c.r.reopened = map[string]bool{}
	}
	for _, class := range chain.classes {
		key := class.name + "\x00" + class.path
		done, ok := c.r.reopened[key]
		if !ok {
			name := regexp.QuoteMeta(class.name[strings.LastIndex(class.name, ":")+1:])
			decl := regexp.MustCompile(`(?m)^[ \t]*(?:class|module)[ \t]+(?:::)?(?:\w+::)*` + name + `(?:[^\w:]|$)|\b` + name + `\s*\.\s*` + rubyPatchCall + `\b`)
			for _, path := range app.Paths {
				text, read := app.Read(path)
				n := len(decl.FindAllStringIndex(text, -1))
				if path == class.path {
					n--
				}
				if done = !read || n != 0; done {
					break
				}
			}
			c.r.reopened[key] = done
		}
		if done {
			return true
		}
	}
	return false
}

// appChain reads the target class and its superclasses from committed application source
// (TSA-V0-445). A record chain needs a committed db/schema.rb that never names the reader.
func (c *flowContext) appChain(class, reader string) (appChain, bool) {
	app, prefix := c.r.app, specPrefix(c.name)
	out := appChain{}
	for hop := 0; ; hop++ {
		if hop > maxAppSuperclasses {
			return appChain{}, false
		}
		path, ok := appFile(app, prefix, class)
		if !ok {
			return appChain{}, false
		}
		text, ok := app.Read(path)
		if !ok {
			return appChain{}, false
		}
		read, super, ok := readAppClass(path, text, class, reader)
		if !ok {
			return appChain{}, false
		}
		out.classes = append(out.classes, read)
		if super == "" {
			return out, true
		}
		if rspecRecordBase[super] {
			out.record = true
			schema, ok := app.Read(prefix + "db/schema.rb")
			if !ok || regexp.MustCompile(`\b`+regexp.QuoteMeta(reader)+`\b`).MatchString(schema) {
				return appChain{}, false
			}
			return out, true
		}
		next, ok := superclassName(app, prefix, class, super)
		if !ok {
			return appChain{}, false
		}
		class = next
	}
}

// superclassName resolves a superclass constant written in class's namespace; a name both the
// namespace and the top level could mean gives none.
func superclassName(app *testvalidity.RubyApp, prefix, class, super string) (string, bool) {
	if strings.HasPrefix(super, "::") || !strings.Contains(class, "::") {
		return strings.TrimPrefix(super, "::"), true
	}
	nested := class[:strings.LastIndex(class, "::")] + "::" + super
	_, inner := appFile(app, prefix, nested)
	_, outer := appFile(app, prefix, super)
	switch {
	case inner && outer:
		return "", false
	case inner:
		return nested, true
	}
	return super, true
}

// appFile is the one committed file under prefix's app/<dir>/ or lib/ that Rails autoloading
// names class by, or false when there is none or more than one.
func appFile(app *testvalidity.RubyApp, prefix, class string) (string, bool) {
	parts := strings.Split(strings.TrimPrefix(class, "::"), "::")
	for i, p := range parts {
		parts[i] = underscore(p)
	}
	want := strings.Join(parts, "/") + ".rb"
	found := ""
	for _, path := range app.Paths {
		rest, ok := strings.CutPrefix(path, prefix)
		if !ok {
			continue
		}
		match := rest == "lib/"+want
		if under, ok := strings.CutPrefix(rest, "app/"); ok {
			if k := strings.Index(under, "/"); k > 0 && under[k+1:] == want {
				match = true
			}
		}
		if match {
			if found != "" {
				return "", false
			}
			found = path
		}
	}
	return found, found != ""
}

// underscore is a constant segment's Rails file name: `HTTPClient` is `http_client`.
func underscore(s string) string {
	var b strings.Builder
	r := []rune(s)
	for i, ch := range r {
		if unicode.IsUpper(ch) {
			if i > 0 && (unicode.IsLower(r[i-1]) || unicode.IsDigit(r[i-1]) || unicode.IsUpper(r[i-1]) && i+1 < len(r) && unicode.IsLower(r[i+1])) {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(ch))
			continue
		}
		b.WriteRune(ch)
	}
	return b.String()
}

// readAppClass reads the one declaration of class in a file: its attr_* declarations, instance
// methods and callback symbols, and its superclass. A class-level statement the reader does not
// know, an attr_* argument that is not a symbol, a callback with a block, or a second declaration
// gives false (TSA-V0-445, TSA-V0-447).
func readAppClass(path, text, class, reader string) (appClass, string, bool) {
	lines := rubyLines(text)
	type scope struct {
		name string
		end  int
	}
	stack := []scope{}
	start, end, super := -1, -1, ""
	class = strings.TrimPrefix(class, "::")
	for i := 0; i < len(lines); i++ {
		for len(stack) > 0 && i > stack[len(stack)-1].end {
			stack = stack[:len(stack)-1]
		}
		m := rubyClassDecl.FindStringSubmatch(lines[i].text)
		mod := rubyModuleDecl.FindStringSubmatch(lines[i].text)
		if m == nil && mod == nil {
			continue
		}
		own := ""
		if m != nil {
			own = m[1]
		} else {
			own = mod[1]
		}
		full := strings.TrimPrefix(own, "::")
		if !strings.HasPrefix(own, "::") && len(stack) > 0 {
			full = stack[len(stack)-1].name + "::" + full
		}
		e := rubyBlockEnd(lines, i)
		if e >= len(lines) {
			return appClass{}, "", false
		}
		if m != nil && full == class {
			if start >= 0 {
				return appClass{}, "", false
			}
			start, end, super = i, e, m[2]
		}
		stack = append(stack, scope{name: full, end: e})
	}
	if start < 0 {
		return appClass{}, "", false
	}
	out := appClass{name: class, path: path, readers: map[string]rubyLine{}, kinds: map[string]string{}, writers: map[string]bool{},
		defLines: map[string][][]rubyLine{}}
	names := namesReader(reader)
	for j := start + 1; j < end; j++ {
		text := lines[j].text
		if m := rubyVisibleDef.FindStringSubmatch(text); m != nil {
			text = m[1]
		}
		blockEnd := j
		if rubyOpens(text) {
			if blockEnd = rubyBlockEnd(lines, j); blockEnd >= end {
				return appClass{}, "", false
			}
		}
		switch {
		case rubyClassNew.MatchString(text):
			return appClass{}, "", false
		case rubyDefName.MatchString(text):
			m := rubyDefName.FindStringSubmatch(text)
			if m[1] == "" {
				if m[2] == "method_missing" || m[2] == "respond_to_missing?" {
					return appClass{}, "", false
				}
				out.defLines[m[2]] = append(out.defLines[m[2]], lines[j:blockEnd+1])
			}
		case rubyAttrDecl.MatchString(text):
			m := rubyAttrDecl.FindStringSubmatch(text)
			for _, arg := range strings.Split(m[2], ",") {
				sym := rubyAttrSymbol.FindStringSubmatch(strings.TrimSpace(arg))
				if sym == nil {
					return appClass{}, "", false
				}
				if m[1] != "attr_writer" {
					if _, ok := out.readers[sym[1]]; !ok {
						out.readers[sym[1]], out.kinds[sym[1]] = lines[j], m[1]
					}
				}
				if m[1] != "attr_reader" {
					out.writers[sym[1]] = true
				}
			}
		case rubySingleton.MatchString(text):
			// A singleton `new` or `allocate` can build the receiver without `initialize`.
			for k := j + 1; k < blockEnd; k++ {
				if rubyNewWord.MatchString(maskRuby(lines[k]).masked) {
					return appClass{}, "", false
				}
			}
		case rubyClassDecl.MatchString(text) || rubyModuleDecl.MatchString(text):
		case rubyCallbackDecl.MatchString(text):
			if rubyBlockValue.MatchString(maskRuby(lines[j]).masked) || blockEnd != j {
				return appClass{}, "", false
			}
			out.mentions = out.mentions || names(maskRuby(lines[j]))
			for _, s := range rubySymbolRef.FindAllStringSubmatch(maskRuby(lines[j]).masked, -1) {
				out.callbacks = append(out.callbacks, s[1])
			}
		case rubyInertDecl.MatchString(text):
			out.mentions = out.mentions || names(maskRuby(lines[j]))
		default:
			return appClass{}, "", false
		}
		j = blockEnd
	}
	return out, super, true
}
