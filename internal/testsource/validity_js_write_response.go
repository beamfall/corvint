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
func writeResponseFindings(r *jsReader, e jsDecl, finding func(rule, severity, location, cause, message string) testvalidity.SourceFinding) []testvalidity.SourceFinding {
	out := []testvalidity.SourceFinding{}
	if !e.HasBody {
		return out
	}
	exemplar := ""
	for _, step := range writeSteps(r, e.Body) {
		facts := jsReaderWriteStepFacts(r, step)
		if facts.action < 0 {
			continue
		}
		for _, w := range facts.waits {
			if w.asserted {
				continue
			}
			switch {
			case w.unresolved != "":
				out = append(out, finding(testvalidity.RuleWriteResponseUnresolved, testvalidity.SeverityAdvisory, jsReaderAt(r, facts.actionLine), jsReaderAt(r, w.line),
					"abstained: the request the step's user action triggers cannot be resolved ("+w.unresolved+"), so whether an unasserted write response is left is unknown"))
			case w.method != "":
				if exemplar == "" {
					exemplar = writeExemplar(r)
				}
				message := "the step's user action triggers a first-party " + w.method + " request awaited at " + jsReaderAt(r, w.line) +
					" but no assertion in the step binds its response (status or body), so a failed or misrouted write still passes"
				if exemplar != "-" {
					message += "; start the response wait before the action and assert its status after it, as " + exemplar + " does"
				}
				out = append(out, finding(testvalidity.RuleWriteResponseUnasserted, testvalidity.SeverityAdvisory, jsReaderAt(r, facts.actionLine), jsReaderAt(r, w.line), message))
			}
		}
	}
	return out
}

// writeExemplar returns `path:line` of the first in-file wait that starts before its step's
// action, resolves to a write and is asserted (TSA-V0-232), or "-" when there is none.
func writeExemplar(r *jsReader) string {
	for _, d := range r.Decls {
		if d.Kind != "test" || !d.HasBody {
			continue
		}
		for _, step := range writeSteps(r, d.Body) {
			facts := jsReaderWriteStepFacts(r, step)
			if facts.action < 0 {
				continue
			}
			for _, w := range facts.waits {
				if w.method != "" && w.asserted && w.early {
					return jsReaderAt(r, w.line)
				}
			}
		}
	}
	return "-"
}

// writeSteps splits an example body into its `X.step(title, fn)` bodies and the rest.
func writeSteps(r *jsReader, body [2]int) []writeStep {
	s := r.S
	rest := writeStep{span: body}
	steps := []writeStep{}
	for k := body[0]; k < body[1]; k++ {
		if !s.Ident(k, "step") || !s.MemberAccess(k) || !s.Punct(k+1, "(") || s.Match[k+1] < 0 {
			continue
		}
		args := s.SplitArgs(k+1, s.Match[k+1])
		for a := 1; a < len(args); a++ {
			if inner, _, ok := s.FunctionBody(args[a][0]); ok && inner[1] <= args[a][1] {
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

// writeStepFacts reads one step's first user action and its waits.
func jsReaderWriteStepFacts(r *jsReader, st writeStep) writeStepFacts {
	s := r.S
	facts := writeStepFacts{action: -1}
	for k := st.span[0]; k < st.span[1]; k = st.next(k) {
		if !s.Ident(k, "") || !s.MemberAccess(k) || !s.Punct(k+1, "(") || s.Match[k+1] < 0 {
			continue
		}
		name := s.Tokens[k].Text
		if writeActions[name] && facts.action < 0 {
			facts.action, facts.actionLine = k, s.Tokens[k].Line
		}
		if writeWaits[name] {
			facts.waits = append(facts.waits, readWait(r, st, k))
		}
	}
	for i := range facts.waits {
		w := &facts.waits[i]
		w.early = w.early && facts.action >= 0 && w.at < facts.action
	}
	return facts
}

// readWait resolves the wait call whose member name is at token k (TSA-V0-230, TSA-V0-231).
func readWait(r *jsReader, st writeStep, k int) writeWait {
	s := r.S
	w := writeWait{at: k, line: s.Tokens[k].Line}
	open := k + 1
	args := s.SplitArgs(open, s.Match[open])
	if len(args) == 0 {
		w.unresolved = "the wait names no request"
	} else {
		w.method, w.unresolved = waitMethod(r, args[0])
	}
	if w.unresolved == "" && w.method != "" && !writeMethods[w.method] {
		w.method = ""
	}
	names, early, thenArgs := waitBinding(r, k)
	w.early = early
	if thenArgs[1] > thenArgs[0] {
		// The callback's response parameter must be read after an assertion starts.
		param := thenArgs[0]
		if s.Ident(param, "async") {
			param++
		}
		if s.Punct(param, "(") {
			param++
		}
		asserting := false
		for j := param + 1; s.Ident(param, "") && j < thenArgs[1]; j++ {
			asserting = asserting || s.JSAssertion(j)
			if asserting && s.Ident(j, s.Tokens[param].Text) && !s.MemberAccess(j) {
				w.asserted = true
			}
		}
	}
	if len(names) > 0 && assertsAny(r, st, k, names) {
		w.asserted = true
	}
	return w
}

// waitMethod resolves the method a wait's first argument matches. A URL string or glob names no
// method, a predicate must compare `.method()` with literal methods and hold no negation, and an
// absolute URL is not known to be first party.
func waitMethod(r *jsReader, arg [2]int) (string, string) {
	s := r.S
	methods := map[string]bool{}
	method, negated := false, false
	for j := arg[0]; j < arg[1]; j++ {
		t := s.Tokens[j]
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
			if t.Text == "method" && s.MemberAccess(j) && s.Punct(j+1, "(") {
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
func waitBinding(r *jsReader, k int) ([]string, bool, [2]int) {
	s := r.S
	close := s.Match[k+1]
	thenArgs := [2]int{}
	if s.Punct(close+1, ".") && s.Ident(close+2, "then") && s.Punct(close+3, "(") && s.Match[close+3] > 0 {
		thenArgs = [2]int{close + 4, s.Match[close+3]}
	}
	root := k
	for s.MemberAccess(root) && s.Ident(root-2, "") {
		root -= 2
	}
	before := root - 1
	awaited := s.Ident(before, "await")
	if awaited {
		before--
	}
	if name, ok := assignedName(r, before); ok {
		return []string{name}, !awaited, thenArgs
	}
	// A `Promise.all([..., wait, ...])` element: bound through array destructuring.
	for j := root - 1; j >= 0; j-- {
		if s.Punct(j, "[") && s.Match[j] > k {
			slot := 0
			for q := j + 1; q < root; q++ {
				if s.Match[q] > q {
					q = s.Match[q]
					continue
				}
				if s.Punct(q, ",") {
					slot++
				}
			}
			p := j - 1
			if !s.Punct(p, "(") || !s.Ident(p-1, "all") || !s.Punct(p-2, ".") || !s.Ident(p-3, "Promise") {
				return nil, false, thenArgs
			}
			q := p - 4
			if s.Ident(q, "await") {
				q--
			}
			if !s.Punct(q, "=") || !s.Punct(q-1, "]") || s.Match[q-1] < 0 {
				return nil, true, thenArgs
			}
			open := s.Match[q-1]
			elems := s.SplitArgs(open, q-1)
			if slot < len(elems) && elems[slot][1]-elems[slot][0] == 1 && s.Ident(elems[slot][0], "") {
				return []string{s.Tokens[elems[slot][0]].Text}, true, thenArgs
			}
			return nil, true, thenArgs
		}
		if s.Punct(j, ";") || s.Punct(j, "{") || s.Punct(j, "}") {
			break
		}
	}
	return nil, false, thenArgs
}

// assignedName returns X when token i is the `=` of `X =` (not `==`, `=>` or a compound).
func assignedName(r *jsReader, i int) (string, bool) {
	s := r.S
	if !s.Punct(i, "=") || s.Punct(i-1, "=") || s.Punct(i-1, "!") || s.Punct(i+1, "=") || s.Punct(i+1, ">") {
		return "", false
	}
	if s.Ident(i-1, "") && !s.MemberAccess(i-1) {
		return s.Tokens[i-1].Text, true
	}
	return "", false
}

// assertsAny reports an assertion in step st, after token from, that reads one of names or a
// local assigned from an expression reading one (TSA-V0-231).
func assertsAny(r *jsReader, st writeStep, from int, names []string) bool {
	s := r.S
	bound := map[string]bool{}
	for _, n := range names {
		bound[n] = true
	}
	reads := func(a, b int) bool {
		for q := a; q < b; q++ {
			if s.Ident(q, "") && !s.MemberAccess(q) && bound[s.Tokens[q].Text] {
				return true
			}
		}
		return false
	}
	for k := from; k < st.span[1]; k = st.next(k) {
		if name, ok := assignedName(r, k); ok && !bound[name] {
			end := k + 1
			for end < st.span[1] && !s.Punct(end, ";") && s.Tokens[end].Line == s.Tokens[k].Line {
				if s.Match[end] > end {
					end = s.Match[end]
				}
				end++
			}
			if reads(k+1, end) {
				bound[name] = true
			}
			continue
		}
		if !s.JSAssertion(k) {
			continue
		}
		end := k + 1
		if c, ok := s.ReadExpect(k); ok {
			end = c.End + 1
		} else {
			for end < st.span[1] && !s.Punct(end, "(") {
				end++
			}
			if s.Match[end] > end {
				end = s.Match[end] + 1
			}
		}
		if reads(k, end) {
			return true
		}
	}
	return false
}
