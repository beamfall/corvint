package testsource

import (
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// WRITE-RESPONSE-UNASSERTED (TSA-V0-230..234; V1-1933 / GitHub #1194). A browser step whose user
// action triggers a first-party write request, statically resolved from the response or request
// wait the step awaits, is reported when no assertion in the same step binds that response.

var (
	// writeActions are the locator, page and keyboard calls that act for a user (TSA-V0-230).
	writeActions = map[string]bool{"click": true, "dblclick": true, "tap": true, "press": true, "check": true,
		"uncheck": true, "setChecked": true, "selectOption": true, "submit": true, "dispatchEvent": true}
	// writeWaits are the waits that name the request a step triggers (TSA-V0-230).
	writeWaits   = map[string]bool{"waitForResponse": true, "waitForRequest": true}
	httpMethods  = map[string]bool{"GET": true, "HEAD": true, "OPTIONS": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}
	writeMethods = map[string]bool{"POST": true, "PUT": true, "PATCH": true, "DELETE": true}
)

// writeStep is one step: a `test.step` callback body, or the example body without its steps.
type writeStep struct {
	span [2]int
	skip [][2]int
}

// writeWait is one response or request wait in a step and what the reader resolved about it.
type writeWait struct {
	at, line int
	// method is the resolved write method, "" when the request is read-only or unresolved.
	method     string
	unresolved string // why the request could not be resolved, or ""
	asserted   bool   // an assertion in the step binds the wait's result
	early      bool   // the wait starts before the step's first action without being awaited first
}

// writeStepFacts is what one step holds: its first user action and its waits.
type writeStepFacts struct {
	action, actionLine int
	waits              []writeWait
}

// writeResponseFindings returns the TSA-V0-230..233 findings of example e.
func (r *jsReader) writeResponseFindings(e jsDecl, finding func(rule, severity, location, cause, message string) testvalidity.SourceFinding) []testvalidity.SourceFinding {
	out := []testvalidity.SourceFinding{}
	if !e.hasBody {
		return out
	}
	exemplar := ""
	for _, step := range r.writeSteps(e.body) {
		facts := r.writeStepFacts(step)
		if facts.action < 0 {
			continue
		}
		for _, w := range facts.waits {
			if w.asserted {
				continue
			}
			switch {
			case w.unresolved != "":
				out = append(out, finding(testvalidity.RuleWriteResponseUnresolved, testvalidity.SeverityAdvisory, r.at(facts.actionLine), r.at(w.line),
					"abstained: the request the step's user action triggers cannot be resolved ("+w.unresolved+"), so whether an unasserted write response is left is unknown"))
			case w.method != "":
				if exemplar == "" {
					exemplar = r.writeExemplar()
				}
				message := "the step's user action triggers a first-party " + w.method + " request awaited at " + r.at(w.line) +
					" but no assertion in the step binds its response (status or body), so a failed or misrouted write still passes"
				if exemplar != "-" {
					message += "; start the response wait before the action and assert its status after it, as " + exemplar + " does"
				}
				out = append(out, finding(testvalidity.RuleWriteResponseUnasserted, testvalidity.SeverityAdvisory, r.at(facts.actionLine), r.at(w.line), message))
			}
		}
	}
	return out
}

// writeExemplar returns `path:line` of the first in-file wait that starts before its step's
// action, resolves to a write and is asserted (TSA-V0-232), or "-" when there is none.
func (r *jsReader) writeExemplar() string {
	for _, d := range r.decls {
		if d.kind != "test" || !d.hasBody {
			continue
		}
		for _, step := range r.writeSteps(d.body) {
			facts := r.writeStepFacts(step)
			if facts.action < 0 {
				continue
			}
			for _, w := range facts.waits {
				if w.method != "" && w.asserted && w.early {
					return r.at(w.line)
				}
			}
		}
	}
	return "-"
}

// writeSteps splits an example body into its `X.step(title, fn)` bodies and the rest.
func (r *jsReader) writeSteps(body [2]int) []writeStep {
	s := r.s
	rest := writeStep{span: body}
	steps := []writeStep{}
	for k := body[0]; k < body[1]; k++ {
		if !s.ident(k, "step") || !s.memberAccess(k) || !s.punct(k+1, "(") || s.match[k+1] < 0 {
			continue
		}
		args := s.splitArgs(k+1, s.match[k+1])
		for a := 1; a < len(args); a++ {
			if inner, _, ok := s.functionBody(args[a][0]); ok && inner[1] <= args[a][1] {
				steps = append(steps, writeStep{span: inner})
				rest.skip = append(rest.skip, inner)
				k = inner[1]
				break
			}
		}
	}
	return append([]writeStep{rest}, steps...)
}

// next returns the next token index in step after k, jumping over skipped ranges.
func (st writeStep) next(k int) int {
	k++
	for _, sk := range st.skip {
		if within(k, sk) {
			k = sk[1]
		}
	}
	return k
}

func (st writeStep) has(k int) bool {
	if !within(k, st.span) {
		return false
	}
	for _, sk := range st.skip {
		if within(k, sk) {
			return false
		}
	}
	return true
}

// writeStepFacts reads one step's first user action and its waits.
func (r *jsReader) writeStepFacts(st writeStep) writeStepFacts {
	s := r.s
	facts := writeStepFacts{action: -1}
	for k := st.span[0]; k < st.span[1]; k = st.next(k) {
		if !s.ident(k, "") || !s.memberAccess(k) || !s.punct(k+1, "(") || s.match[k+1] < 0 {
			continue
		}
		name := s.tokens[k].Text
		if writeActions[name] && facts.action < 0 {
			facts.action, facts.actionLine = k, s.tokens[k].Line
		}
		if writeWaits[name] {
			facts.waits = append(facts.waits, r.readWait(st, k))
		}
	}
	for i := range facts.waits {
		w := &facts.waits[i]
		w.early = w.early && facts.action >= 0 && w.at < facts.action
	}
	return facts
}

// readWait resolves the wait call whose member name is at token k (TSA-V0-230, TSA-V0-231).
func (r *jsReader) readWait(st writeStep, k int) writeWait {
	s := r.s
	w := writeWait{at: k, line: s.tokens[k].Line}
	open := k + 1
	args := s.splitArgs(open, s.match[open])
	if len(args) == 0 {
		w.unresolved = "the wait names no request"
	} else {
		w.method, w.unresolved = r.waitMethod(args[0])
	}
	if w.unresolved == "" && w.method != "" && !writeMethods[w.method] {
		w.method = ""
	}
	names, early, thenArgs := r.waitBinding(k)
	w.early = early
	if thenArgs[1] > thenArgs[0] {
		// The callback's response parameter must be read after an assertion starts.
		param := thenArgs[0]
		if s.ident(param, "async") {
			param++
		}
		if s.punct(param, "(") {
			param++
		}
		asserting := false
		for j := param + 1; s.ident(param, "") && j < thenArgs[1]; j++ {
			asserting = asserting || s.jsAssertion(j)
			if asserting && s.ident(j, s.tokens[param].Text) && !s.memberAccess(j) {
				w.asserted = true
			}
		}
	}
	if len(names) > 0 && r.assertsAny(st, k, names) {
		w.asserted = true
	}
	return w
}

// waitMethod resolves the method a wait's first argument matches. A URL string or glob names no
// method, a predicate must compare `.method()` with literal methods and hold no negation, and an
// absolute URL is not known to be first party.
func (r *jsReader) waitMethod(arg [2]int) (string, string) {
	s := r.s
	methods := map[string]bool{}
	method, negated := false, false
	for j := arg[0]; j < arg[1]; j++ {
		t := s.tokens[j]
		switch t.Kind {
		case appmap.JSString, appmap.JSTemplate:
			lower := strings.ToLower(t.Text)
			if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "//") {
				return "", "absolute URL, not known to be first party"
			}
			if t.Literal && httpMethods[strings.ToUpper(t.Text)] {
				methods[strings.ToUpper(t.Text)] = true
			}
		case appmap.JSIdent:
			if t.Text == "method" && s.memberAccess(j) && s.punct(j+1, "(") {
				method = true
			}
		case appmap.JSPunct:
			if t.Text == "!" {
				negated = true
			}
		}
	}
	if !method {
		return "", "the wait does not match a request method"
	}
	if negated || len(methods) == 0 {
		return "", "the request method is not compared with a literal"
	}
	write, read, first := 0, 0, ""
	for m := range methods {
		if writeMethods[m] {
			write++
			if first == "" || m < first {
				first = m
			}
		} else {
			read++
		}
	}
	if write > 0 && read > 0 {
		return "", "the wait matches both read and write methods"
	}
	if write == 0 {
		return "GET", ""
	}
	return first, ""
}

// waitBinding returns the names the wait's result is bound to, whether the wait starts unawaited
// (a promise variable or a `Promise.all` element), and the argument range of a `.then(...)` on it.
func (r *jsReader) waitBinding(k int) ([]string, bool, [2]int) {
	s := r.s
	close := s.match[k+1]
	thenArgs := [2]int{}
	if s.punct(close+1, ".") && s.ident(close+2, "then") && s.punct(close+3, "(") && s.match[close+3] > 0 {
		thenArgs = [2]int{close + 4, s.match[close+3]}
	}
	root := k
	for s.memberAccess(root) && s.ident(root-2, "") {
		root -= 2
	}
	before := root - 1
	awaited := s.ident(before, "await")
	if awaited {
		before--
	}
	if name, ok := r.assignedName(before); ok {
		return []string{name}, !awaited, thenArgs
	}
	// A `Promise.all([..., wait, ...])` element: bound through array destructuring.
	for j := root - 1; j >= 0; j-- {
		if s.punct(j, "[") && s.match[j] > k {
			slot := 0
			for q := j + 1; q < root; q++ {
				if s.match[q] > q {
					q = s.match[q]
					continue
				}
				if s.punct(q, ",") {
					slot++
				}
			}
			p := j - 1
			if !s.punct(p, "(") || !s.ident(p-1, "all") || !s.punct(p-2, ".") || !s.ident(p-3, "Promise") {
				return nil, false, thenArgs
			}
			q := p - 4
			if s.ident(q, "await") {
				q--
			}
			if !s.punct(q, "=") || !s.punct(q-1, "]") || s.match[q-1] < 0 {
				return nil, true, thenArgs
			}
			open := s.match[q-1]
			elems := s.splitArgs(open, q-1)
			if slot < len(elems) && elems[slot][1]-elems[slot][0] == 1 && s.ident(elems[slot][0], "") {
				return []string{s.tokens[elems[slot][0]].Text}, true, thenArgs
			}
			return nil, true, thenArgs
		}
		if s.punct(j, ";") || s.punct(j, "{") || s.punct(j, "}") {
			break
		}
	}
	return nil, false, thenArgs
}

// assignedName returns X when token i is the `=` of `X =` (not `==`, `=>` or a compound).
func (r *jsReader) assignedName(i int) (string, bool) {
	s := r.s
	if !s.punct(i, "=") || s.punct(i-1, "=") || s.punct(i-1, "!") || s.punct(i+1, "=") || s.punct(i+1, ">") {
		return "", false
	}
	if s.ident(i-1, "") && !s.memberAccess(i-1) {
		return s.tokens[i-1].Text, true
	}
	return "", false
}

// assertsAny reports an assertion in step st, after token from, that reads one of names or a
// local assigned from an expression reading one (TSA-V0-231).
func (r *jsReader) assertsAny(st writeStep, from int, names []string) bool {
	s := r.s
	bound := map[string]bool{}
	for _, n := range names {
		bound[n] = true
	}
	reads := func(a, b int) bool {
		for q := a; q < b; q++ {
			if s.ident(q, "") && !s.memberAccess(q) && bound[s.tokens[q].Text] {
				return true
			}
		}
		return false
	}
	for k := from; k < st.span[1]; k = st.next(k) {
		if name, ok := r.assignedName(k); ok && !bound[name] {
			end := k + 1
			for end < st.span[1] && !s.punct(end, ";") && s.tokens[end].Line == s.tokens[k].Line {
				if s.match[end] > end {
					end = s.match[end]
				}
				end++
			}
			if reads(k+1, end) {
				bound[name] = true
			}
			continue
		}
		if !s.jsAssertion(k) {
			continue
		}
		end := k + 1
		if c, ok := s.readExpect(k); ok {
			end = c.end + 1
		} else {
			for end < st.span[1] && !s.punct(end, "(") {
				end++
			}
			if s.match[end] > end {
				end = s.match[end] + 1
			}
		}
		if reads(k, end) {
			return true
		}
	}
	return false
}
