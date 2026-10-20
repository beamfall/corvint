package testsource

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// Vacuous RSpec example patterns (TSA-V0-084..088): a block handed to a helper that never yields,
// a negated change matcher over a reloaded record, an assignment as a satisfy predicate, and an
// example whose only expectations are negated raise_error. The reading stays line-based and
// abstains with a named reason whenever it cannot resolve the helper an example calls.

var (
	// rspecNeverYieldWords are the words that let a def reach its caller's block without a block
	// parameter: any of them makes the def unread for TSA-V0-084.
	rspecNeverYieldWords = map[string]bool{
		"yield": true, "block_given?": true, "iterator?": true, "super": true, "binding": true,
		"proc": true, "Proc": true, "lambda": true, "define_method": true, "instance_eval": true,
		"instance_exec": true, "class_eval": true, "class_exec": true, "module_eval": true, "eval": true,
		"send": true, "__send__": true, "public_send": true, "method_missing": true,
	}
	// rspecBlockDSL are calls whose block RSpec, Rails or Ruby runs; they are never resolved as
	// spec helpers.
	rspecBlockDSL = map[string]bool{
		"expect": true, "change": true, "satisfy": true, "allow": true, "receive": true, "and_wrap_original": true,
		"and_return": true, "and_yield": true, "aggregate_failures": true, "travel_to": true, "travel": true,
		"freeze_time": true, "perform_enqueued_jobs": true, "within": true, "lambda": true, "proc": true,
		"each": true, "map": true, "times": true, "tap": true, "then": true, "loop": true, "raise_error": true,
		"output": true, "yield_control": true, "yield_with_args": true, "let": true, "subject": true,
		"before": true, "after": true, "around": true, "it": true, "specify": true, "context": true,
		"describe": true, "assert_raises": true, "assert_difference": true, "assert_no_difference": true,
		"assert_changes": true, "assert_no_changes": true, "with_options": true, "using_session": true,
	}
	// rspecChangeOnReload is a negated change matcher whose block is a reloaded record.
	rspecChangeOnReload = regexp.MustCompile(`\.\s*(?:not_to|to_not)\s*\(?\s*change\s*\{\s*(?:\|[^|]*\|)?\s*(@{0,2}[a-z_]\w*(?:\.[a-z_]\w*[?!]?)*\.reload)\s*\}`)
	// rspecSatisfyBrace is a one-line satisfy block; group 1 is its body.
	rspecSatisfyBrace = regexp.MustCompile(`\bsatisfy\b\s*(?:\([^()]*\))?\s*\{\s*(?:\|[^|]*\|)?(.*)\}`)
	rspecSatisfyDo    = regexp.MustCompile(`\bsatisfy\b\s*(?:\([^()]*\))?\s*do(?:\s*\|[^|]*\|)?$`)
	// rspecAssignment is a plain or `||=` assignment statement; group 1 is the assigned expression.
	rspecAssignment = regexp.MustCompile(`^@{0,2}[A-Za-z_]\w*(?:\[[^\]]*\]|\.[A-Za-z_]\w*)*\s*(?:\|\|)?=([^=~>].*)$`)
	// rspecTruthyLiteral is an expression Ruby always reads as true: a number, a string (stripped
	// to ""), a symbol, true, or an array or hash literal.
	rspecTruthyLiteral = regexp.MustCompile(`^(?:-?\d[\d_]*(?:\.\d+)?|""|:[A-Za-z_]\w*[?!]?|true|\[.*\]|\{.*\})$`)
	// rspecNegatedRaise is a `not_to raise_error` expectation.
	rspecNegatedRaise = regexp.MustCompile(`\.\s*(?:not_to|to_not)\s*\(?\s*raise_error\b`)
	// rspecRaiseTitle is a title that names the absence of an error as the behaviour.
	rspecRaiseTitle = regexp.MustCompile(`(?i)rais|error|exception|fail|crash|blow|explod|succeed|success|work|run|handle|accept`)
)

// rspecSupport is the support set a reader resolves helpers against: the readers of the bound
// support files by path, the files that did not read with their unread readers, and the reason
// the set is unbound.
type rspecSupport struct {
	readers  map[string]*rspecReader
	unread   []string
	unreadBy map[string]*rspecReader
	unbound  string
}

func newRSpecSupport(support testvalidity.RubySupport) *rspecSupport {
	s := &rspecSupport{readers: map[string]*rspecReader{}, unreadBy: map[string]*rspecReader{}, unbound: support.Unbound}
	paths := make([]string, 0, len(support.Files))
	for path := range support.Files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		r := newRSpecReader(path, support.Files[path])
		if r.read() != "" {
			s.unread = append(s.unread, path)
			s.unreadBy[path] = r
			continue
		}
		s.readers[path] = r
	}
	return s
}

// sharedDefinitions returns the shared groups named name in the readers, as (reader, group)
// pairs, and whether a non-literal shared definition could also be named name.
func sharedDefinitions(readers []*rspecReader, name string) ([]sharedDefinition, bool) {
	out := []sharedDefinition{}
	unsure := false
	for _, r := range readers {
		for g, group := range r.groups {
			word := declarator(group.head)
			if !sharedDefinitionWords[word] {
				continue
			}
			got, literal := sharedName(strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(group.head, "RSpec."), word)))
			switch {
			case !literal:
				unsure = true
			case got == name:
				out = append(out, sharedDefinition{reader: r, group: g})
			}
		}
	}
	return out, unsure
}

type sharedDefinition struct {
	reader *rspecReader
	group  int
}

// helperResolution is the definition an example's helper call resolves to, or why it does not.
type helperResolution struct {
	helper rspecHelper
	found  bool
	// abstain is RB-HELPER-UNRESOLVED or RB-HELPER-AMBIGUOUS, cause the include or file behind it.
	abstain, cause string
}

// resolveAcross resolves name for an example of group g: at each level of the chain, innermost
// first, the group's own defs and the defs of the shared contexts it includes inline compete, and
// the later one in the group wins. A level whose candidates sit in different files (a spliced
// shared body and its customization block) has no line order to compare, so it abstains, as does
// an include whose shared group cannot be bound to exactly one literal definition in a complete
// support set, or whose shared group includes another (TSA-V0-088).
func (r *rspecReader) resolveAcross(g int, name string) helperResolution {
	readers := r.sharedReaders()
	type candidate struct {
		helper rspecHelper
		path   string
		line   int
	}
	for ; g >= 0; g = r.groups[g].parent {
		candidates := []candidate{}
		for _, h := range r.helpers {
			if h.group == g && helperName(h.body) == name {
				candidates = append(candidates, candidate{h, h.path, h.body[0].line})
			}
		}
		for _, inc := range r.includes {
			inline := inc.kind == "include_context" || inc.kind == "include_examples"
			if inc.nested != g && !(inline && inc.group == g) {
				continue
			}
			cause := fmt.Sprintf("%s:%d", inc.path, inc.line)
			if !inc.literal {
				return helperResolution{abstain: testvalidity.RuleRBHelperUnresolved, cause: cause}
			}
			defs, unsure := sharedDefinitions(readers, inc.name)
			switch {
			case len(defs) > 1 || len(defs) == 1 && unsure:
				return helperResolution{abstain: testvalidity.RuleRBHelperAmbiguous, cause: cause}
			case len(defs) == 0 || r.support.sharedGap(inc.name) != "":
				return helperResolution{abstain: testvalidity.RuleRBHelperUnresolved, cause: cause}
			}
			d := defs[0]
			for _, nested := range d.reader.includes {
				if nested.group == d.group && (nested.kind == "include_context" || nested.kind == "include_examples") {
					return helperResolution{abstain: testvalidity.RuleRBHelperUnresolved, cause: fmt.Sprintf("%s:%d", nested.path, nested.line)}
				}
			}
			// Within one shared context the later def wins; the include line orders it in the group.
			last, ok := rspecHelper{}, false
			for _, h := range d.reader.helpers {
				if h.group == d.group && helperName(h.body) == name && (!ok || h.body[0].line > last.body[0].line) {
					last, ok = h, true
				}
			}
			if ok {
				candidates = append(candidates, candidate{last, inc.path, inc.line})
			}
		}
		if len(candidates) == 0 {
			continue
		}
		best := candidates[0]
		for _, c := range candidates[1:] {
			if c.path != best.path {
				return helperResolution{abstain: testvalidity.RuleRBHelperUnresolved, cause: fmt.Sprintf("%s:%d", c.path, c.line)}
			}
			if c.line > best.line {
				best = c
			}
		}
		return helperResolution{helper: best.helper, found: true}
	}
	// Unreached defs: a module or class method, or a def in a support file, may be the helper
	// through an inclusion the reader does not follow.
	for _, sr := range readers {
		for _, h := range append(append([]rspecHelper{}, sr.offGroupDefs...), sr.helpers...) {
			if helperName(h.body) == name && (sr != r || h.group < 0) {
				return helperResolution{abstain: testvalidity.RuleRBHelperUnresolved, cause: fmt.Sprintf("%s:%d", h.path, h.body[0].line)}
			}
		}
	}
	return helperResolution{}
}

// supportGap names why the support set could hold a definition the reader did not see: the reason
// it is unbound, or the first support file that did not read; "" when it is complete.
func (r *rspecReader) supportGap() string {
	switch {
	case r.support == nil:
		return "rspec-support-not-given"
	case r.support.unbound != "":
		return r.support.unbound
	case len(r.support.unread) > 0:
		return r.support.unread[0]
	}
	return ""
}

// rspecSharedWord finds each shared group definition anywhere on a line; its arguments follow it.
var rspecSharedWord = regexp.MustCompile(`\b(?:shared_examples_for|shared_examples|shared_context)\b`)

// sharedGap names why a shared group called name could be defined where the reader did not look:
// the reason the set is unbound or not given, or the first support file, other than the skipped
// paths, that did not read and has a shared definition naming name or a non-literal name. A file
// that did not read but defines no such group does not hold the inclusion back (TSA-V0-397).
func (s *rspecSupport) sharedGap(name string, skip ...string) string {
	switch {
	case s == nil:
		return "rspec-support-not-given"
	case s.unbound != "":
		return s.unbound
	}
	for _, path := range s.unread {
		if slices.Contains(skip, path) {
			continue
		}
		for _, l := range s.unreadBy[path].lines {
			// Every definition on the line counts, not only the first.
			for _, m := range rspecSharedWord.FindAllStringIndex(l.text, -1) {
				if got, literal := sharedName(strings.TrimSpace(l.text[m[1]:])); !literal || got == name {
					return path
				}
			}
		}
	}
	return ""
}

// sharedReaders returns this file's reader and the bound support readers.
func (r *rspecReader) sharedReaders() []*rspecReader {
	out := []*rspecReader{r}
	if r.support == nil {
		return out
	}
	paths := make([]string, 0, len(r.support.readers))
	for path := range r.support.readers {
		if path != r.path {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	for _, path := range paths {
		out = append(out, r.support.readers[path])
	}
	return out
}

// neverYields reports whether a def takes no block parameter and nothing in its body can reach
// the caller's block (TSA-V0-084). A def it cannot read, such as one with a nested def, a string
// interpolation or a heredoc, is never reported.
func neverYields(body []rubyLine) bool {
	head := stripRubyStrings(body[0].text)
	rest := []string{}
	switch {
	case rubyOneLine.MatchString(head):
		parts := strings.Split(head, ";")
		head = strings.TrimSpace(parts[0])
		rest = parts[1 : len(parts)-1]
	case rubyEndlessDef.MatchString(strings.TrimSpace(head)):
		head = strings.TrimSpace(head) // its expression is read from the params below
	case len(body) < 2:
		return false
	default:
		for _, l := range body[1 : len(body)-1] {
			if rspecHiddenText.MatchString(l.text) {
				return false
			}
			rest = append(rest, stripRubyStrings(l.text))
		}
	}
	if rspecHiddenText.MatchString(body[0].text) {
		return false
	}
	m := rspecDefHead.FindStringSubmatch(head)
	if m == nil {
		return false
	}
	params := strings.TrimSpace(m[2] + m[3])
	if strings.HasPrefix(params, "=") { // an endless def: its expression is its body
		rest, params = append(rest, params[1:]), ""
	}
	if strings.Contains(params, "&") || strings.Contains(params, "...") {
		return false
	}
	// A default argument such as `value = yield` runs when the caller omits it.
	for _, t := range append(rest, params) {
		t = strings.TrimSpace(t)
		if rspecDefLine.MatchString(t) {
			return false
		}
		for _, loc := range rspecIdentifier.FindAllStringIndex(t, -1) {
			if rspecNeverYieldWords[t[loc[0]:loc[1]]] {
				return false
			}
		}
	}
	return true
}

// blockCalls returns the names called with a literal block in an example body, each with the
// line of its first such call, in order of first appearance.
func blockCalls(body []rubyLine) []blockCall {
	out := []blockCall{}
	seen := map[string]bool{}
	for k, l := range body {
		t := stripRubyStrings(l.text)
		if k == 0 {
			// The head only holds calls after a one-line example's opening brace.
			open := strings.Index(t, "{")
			if open < 0 {
				continue
			}
			t = t[open+1:]
		}
		for _, loc := range rspecIdentifier.FindAllStringIndex(t, -1) {
			name := t[loc[0]:loc[1]]
			if seen[name] || rspecBlockDSL[name] || loc[0] > 0 && strings.ContainsRune(".:@$", rune(t[loc[0]-1])) {
				continue
			}
			if after, ok := afterArguments(t[loc[1]:]); ok && (strings.HasPrefix(after, "{") || rspecDoOpener.MatchString(after)) {
				seen[name] = true
				out = append(out, blockCall{name: name, line: l.line})
			}
		}
	}
	return out
}

type blockCall struct {
	name string
	line int
}

// vacuousFindings returns the TSA-V0-084..088 findings of one example.
func (r *rspecReader) vacuousFindings(test string, e rspecExample) []testvalidity.SourceFinding {
	out := []testvalidity.SourceFinding{}
	at := func(line int) string { return fmt.Sprintf("%s:%d", e.path, line) }
	finding := func(rule, severity, location, cause, message string) testvalidity.SourceFinding {
		return testvalidity.SourceFinding{Rule: rule, Severity: severity, Test: test, Title: e.title, Location: location, Cause: cause, Message: message}
	}
	self := at(e.line)
	for _, call := range blockCalls(e.body) {
		res := r.resolveAcross(e.group, call.name)
		switch {
		case res.found && neverYields(res.helper.body):
			def := fmt.Sprintf("%s:%d", res.helper.path, res.helper.body[0].line)
			out = append(out, finding(testvalidity.RuleRBBlockNeverYielded, testvalidity.SeverityInvalid, at(call.line), def,
				fmt.Sprintf("%s is called with a block, but its definition at %s takes no block parameter and never yields, so Ruby discards the block and every expectation inside it is dead code; yield to the block or call the expectations outside it", call.name, def)))
		case res.abstain != "" && r.neverYieldingCandidate(call.name):
			out = append(out, finding(res.abstain, testvalidity.SeverityAbstained, at(call.line), res.cause,
				fmt.Sprintf("%s is called with a block and a definition of it never yields, but the reader cannot bind the call to exactly one definition through %s, so it abstains instead of judging the block", call.name, res.cause)))
		case !res.found && res.abstain == "" && r.supportGap() != "":
			gap := r.supportGap()
			out = append(out, finding(testvalidity.RuleRBHelperUnresolved, testvalidity.SeverityAbstained, at(call.line), at(call.line),
				fmt.Sprintf("%s is called with a block, but no read file defines it and the support set is incomplete (%s), so a never-yielding definition could hide there; the reader abstains instead of judging the block", call.name, gap)))
		}
	}
	for _, l := range e.body {
		t := stripRubyStrings(l.text)
		if m := rspecChangeOnReload.FindStringSubmatch(t); m != nil {
			out = append(out, finding(testvalidity.RuleRBChangeOnIdentity, testvalidity.SeverityAdvisory, at(l.line), self,
				fmt.Sprintf("change { %s } observes the record itself; RSpec compares its before and after hash, which for an ActiveRecord model is its primary key, so an attribute change may not register and the negated expectation may always pass; observe the attribute instead (change { %s.ATTRIBUTE })", m[1], m[1])))
		}
	}
	if line := assignmentPredicate(e.body); line != 0 {
		out = append(out, finding(testvalidity.RuleRBAssignmentInPredicate, testvalidity.SeverityInvalid, at(line), self,
			"the satisfy block ends in an assignment of a truthy literal, which returns that literal, so the predicate is truthy whatever the subject holds; compare with == instead"))
	}
	if count, _ := expectations(e.path, e.body); count > 0 && !rspecRaiseTitle.MatchString(e.title) {
		negated := 0
		for _, l := range e.body {
			negated += len(rspecNegatedRaise.FindAllStringIndex(stripRubyStrings(l.text), -1))
		}
		if negated == count {
			out = append(out, finding(testvalidity.RuleRBRaiseErrorOnly, testvalidity.SeverityAdvisory, self, self,
				"the example's only expectations are not_to raise_error, so it proves the code ran without raising, not the outcome its title names; assert the outcome"))
		}
	}
	return out
}

// neverYieldingCandidate reports whether any bound definition of name never yields or never calls
// its block parameter, so an unresolved call could be vacuous.
func (r *rspecReader) neverYieldingCandidate(name string) bool {
	for _, sr := range r.sharedReaders() {
		for _, h := range append(append([]rspecHelper{}, sr.offGroupDefs...), sr.helpers...) {
			if helperName(h.body) == name && (neverYields(h.body) || helperDefect(h.body).reported) {
				return true
			}
		}
	}
	return r.support == nil || r.support.unbound != "" || len(r.support.unread) > 0
}

// truthyAssignment reports whether a statement assigns a truthy literal, so it returns a truthy
// value whatever the subject holds; an assignment of any other expression may be a real
// predicate (`valid = x.even?`) and is not reported.
func truthyAssignment(statement string) bool {
	m := rspecAssignment.FindStringSubmatch(statement)
	return m != nil && rspecTruthyLiteral.MatchString(strings.TrimSpace(m[1]))
}

// assignmentPredicate returns the line of a satisfy block whose last statement assigns a truthy
// literal, or 0 (TSA-V0-086).
func assignmentPredicate(body []rubyLine) int {
	for k, l := range body {
		t := stripRubyStrings(l.text)
		if m := rspecSatisfyBrace.FindStringSubmatch(t); m != nil {
			inner := m[1]
			if end := closingBrace("{" + inner + "}"); end > 0 {
				inner = inner[:end-1]
			}
			parts := strings.Split(inner, ";")
			if truthyAssignment(strings.TrimSpace(parts[len(parts)-1])) {
				return l.line
			}
			continue
		}
		if rspecSatisfyDo.MatchString(t) {
			end := rubyBlockEnd(body, k)
			if end < len(body) && end-k >= 2 && truthyAssignment(strings.TrimSpace(stripRubyStrings(body[end-1].text))) {
				return l.line
			}
		}
	}
	return 0
}
