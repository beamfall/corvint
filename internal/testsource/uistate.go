package testsource

import (
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
)

// MaxHelperDepth bounds helper-following for the advisory rules (TSA-V0-029): the helper an
// assertion or test body calls is hop 1, a helper that helper returns or calls is hop 2, and a
// call that would need hop 4 is UNKNOWN.
const MaxHelperDepth = 3

// uiStateReads maps each one-shot Playwright UI-state read to the web-first matcher that retries
// it (TSA-V0-029).
var uiStateReads = map[string]string{"isVisible": "toBeVisible", "isHidden": "toBeHidden", "isEnabled": "toBeEnabled",
	"isDisabled": "toBeDisabled", "isChecked": "toBeChecked", "isEditable": "toBeEditable", "count": "toHaveCount",
	"textContent": "toHaveText", "innerText": "toHaveText", "allTextContents": "toHaveText", "allInnerTexts": "toHaveText",
	"inputValue": "toHaveValue", "getAttribute": "toHaveAttribute"}

// knownCalls are Playwright, Promise and built-in calls that are not page-object helpers: with no
// visible definition of the same name they are neither followed nor reported as UNKNOWN
// (TSA-V0-030).
var knownCalls = map[string]bool{}

func init() {
	for _, name := range strings.Fields(`locator getByText getByRole getByTestId getByLabel getByPlaceholder getByAltText
		getByTitle frameLocator contentFrame filter first last nth and or title url json text body status statusText ok
		headers allHeaders evaluate evaluateHandle screenshot content cookies storageState goto reload waitForResponse
		waitForRequest waitForEvent waitForURL waitForLoadState waitForFunction waitForSelector waitForTimeout newPage
		newContext fetch get post put patch delete head all allSettled race any resolve reject then catch finally
		boundingBox pdf click dblclick fill press check uncheck hover tap focus type selectOption setInputFiles
		toString trim toLowerCase toUpperCase includes startsWith endsWith map some every find reduce join split
		slice length Number String Boolean JSON parse stringify Array Object Math Date parseInt parseFloat require
		import expect test step describe Promise setTimeout console log`) {
		knownCalls[name] = true
	}
	for _, name := range strings.Fields(`if for while switch catch return typeof function new await async super do else with void delete in of`) {
		keywords[name] = true
	}
}

// keywords are JavaScript keywords that can precede `(` but never name a definition or a call.
var keywords = map[string]bool{}

// exactBuilders are the Playwright locator builders that take an `exact` option (TSA-V0-031).
var exactBuilders = map[string]bool{"getByText": true, "getByRole": true, "getByLabel": true, "getByPlaceholder": true,
	"getByTitle": true, "getByAltText": true}

// lintContext is what the advisory rules read beyond one file: every parsed source (linted files
// and the relatively imported modules read for helper resolution), each file's resolved relative
// imports, and the cross-variant split by test.
type lintContext struct {
	sources map[string]*source
	imports map[string][]string
	splits  map[string]VariantSplit
}

func newLintContext(sources map[string]*source) *lintContext {
	return &lintContext{sources: sources, imports: map[string][]string{}, splits: map[string]VariantSplit{}}
}

// helperDef is one function, method or arrow-function property definition.
type helperDef struct {
	src  *source
	line int
	body [2]int // a block's [`{`, `}`] or an arrow expression's [start, end)
	expr bool
}

// definitions indexes the named function, method and arrow-function definitions of a source.
func (s *source) definitions() map[string][]helperDef {
	if s.defs != nil {
		return s.defs
	}
	s.defs = map[string][]helperDef{}
	add := func(name int, body [2]int, expr bool) {
		text := s.tokens[name].Text
		s.defs[text] = append(s.defs[text], helperDef{src: s, line: s.tokens[name].Line, body: body, expr: expr})
	}
	for i := range s.tokens {
		if !s.ident(i, "") || s.memberAccess(i) || keywords[s.tokens[i].Text] {
			continue
		}
		switch {
		case s.ident(i-1, "function") && s.punct(i+1, "("):
			if body, ok := s.blockAfterParams(i + 1); ok {
				add(i, body, false)
			}
		case s.punct(i+1, "(") && s.memberStart(i-1):
			if body, ok := s.blockAfterParams(i + 1); ok {
				add(i, body, false)
			}
		case s.punct(i+1, "=") && !s.punct(i+2, "=") && !s.punct(i+2, ">"), s.punct(i+1, ":"):
			value := i + 2
			if s.punct(i+1, ":") && !s.functionValue(value) {
				if j := s.skipType(value); s.punct(j, "=") && !s.punct(j+1, ">") {
					value = j + 1
				}
			}
			if body, expr, ok := s.functionBody(value); ok {
				add(i, body, expr)
			}
		}
	}
	return s.defs
}

// memberStart reports a token after which a class or object member, or a statement, begins.
func (s *source) memberStart(j int) bool {
	if j < 0 || s.punct(j, "{") || s.punct(j, "}") || s.punct(j, ";") || s.punct(j, ",") {
		return true
	}
	for _, word := range []string{"async", "static", "public", "private", "protected", "override", "get"} {
		if s.ident(j, word) {
			return true
		}
	}
	return false
}

// blockAfterParams returns the `{...}` body after the parameter list opened at open, skipping a
// return-type annotation.
func (s *source) blockAfterParams(open int) ([2]int, bool) {
	if s.match[open] < 0 {
		return [2]int{}, false
	}
	j := s.match[open] + 1
	if s.punct(j, ":") {
		j = s.skipType(j + 1)
	}
	if s.punct(j, "{") && s.match[j] > j {
		return [2]int{j, s.match[j]}, true
	}
	return [2]int{}, false
}

// functionValue reports a function or arrow function starting at token v.
func (s *source) functionValue(v int) bool {
	_, _, ok := s.functionBody(v)
	return ok
}

// functionBody reads `[async] function (...) {}`, `[async] (...) [: T] => body` or
// `[async] x => body` at token v.
func (s *source) functionBody(v int) ([2]int, bool, bool) {
	if s.ident(v, "async") {
		v++
	}
	if s.ident(v, "function") {
		if s.ident(v+1, "") {
			v++
		}
		if s.punct(v+1, "(") {
			body, ok := s.blockAfterParams(v + 1)
			return body, false, ok
		}
		return [2]int{}, false, false
	}
	after := -1
	switch {
	case s.punct(v, "(") && s.match[v] > v:
		after = s.match[v] + 1
	case s.ident(v, ""):
		after = v + 1
	default:
		return [2]int{}, false, false
	}
	if s.punct(after, ":") {
		after = s.skipType(after + 1)
	}
	if !s.arrow(after) {
		return [2]int{}, false, false
	}
	start := after + 2
	if s.punct(start, "{") && s.match[start] > start {
		return [2]int{start, s.match[start]}, false, true
	}
	return [2]int{start, s.exprEnd(start, len(s.tokens))}, true, true
}

// skipType walks a type annotation from j to the first top-level `{`, `=` or `;`.
func (s *source) skipType(j int) int {
	angle := 0
	for limit := j + 64; j < len(s.tokens) && j < limit; j++ {
		switch {
		case s.punct(j, "<"):
			angle++
		case s.punct(j, ">") && angle > 0 && !s.arrow(j-1):
			angle--
		case angle == 0 && (s.punct(j, "{") || s.punct(j, "=") || s.punct(j, ";")):
			return j
		case (s.punct(j, "(") || s.punct(j, "[") || s.punct(j, "{")) && s.match[j] > j:
			j = s.match[j]
		}
	}
	return j
}

// exprEnd returns the end of the expression starting at start: a top-level `;` or `,`, an
// unmatched closer, or a line break that does not continue the expression.
func (s *source) exprEnd(start, limit int) int {
	for j := start; j < limit && j < len(s.tokens); j++ {
		if j > start && s.tokens[j].Line != s.tokens[j-1].Line && !s.continues(j) {
			return j
		}
		if s.tokens[j].Kind != appmap.JSPunct {
			continue
		}
		switch s.tokens[j].Text {
		case ";", ",", ")", "]", "}":
			return j
		case "(", "[", "{":
			if s.match[j] <= j {
				return j
			}
			j = s.match[j]
		}
	}
	return min(limit, len(s.tokens))
}

// continues reports that token j, first on its line, continues the previous line's expression.
func (s *source) continues(j int) bool {
	if t := s.tokens[j]; t.Kind == appmap.JSPunct && strings.Contains(".?:&|+-*/<>=", t.Text) {
		return true
	}
	if p := s.tokens[j-1]; p.Kind == appmap.JSPunct && strings.Contains("=&|+-*/<>?:.!(", p.Text) {
		return true
	}
	return s.ident(j-1, "await") || s.ident(j-1, "return") || s.ident(j-1, "new") || s.ident(j-1, "typeof")
}

// verdict is what helper-following found for one expression: a one-shot UI-state read, nothing,
// or UNKNOWN with the reason.
type verdict struct {
	kind   int // verdictNone, verdictRead or verdictUnknown
	read   string
	at     string
	via    []string
	reason string
}

const (
	verdictNone = iota
	verdictUnknown
	verdictRead
)

func better(a, b verdict) verdict {
	if b.kind > a.kind {
		return b
	}
	return a
}

func (s *source) at(i int) string { return s.path + ":" + strconv.Itoa(s.tokens[i].Line) }

// directRead returns the first `.read(` UI-state call in [start, end), or -1.
func (s *source) directRead(start, end int) int {
	for k := start; k < end; k++ {
		if s.ident(k, "") && uiStateReads[s.tokens[k].Text] != "" && s.memberAccess(k) && s.punct(k+1, "(") {
			return k
		}
	}
	return -1
}

// helperCalls returns the calls in [start, end) that may be page-object helpers. A UI-state read
// name is a helper only when it is called standalone, not as a member read.
func (s *source) helperCalls(start, end int) []int {
	out := []int{}
	for k := start; k < end; k++ {
		if s.ident(k, "") && s.punct(k+1, "(") && !keywords[s.tokens[k].Text] && !(uiStateReads[s.tokens[k].Text] != "" && s.memberAccess(k)) &&
			!matcherName(s.tokens[k].Text) && !s.ident(k-1, "new") && !s.ident(k-1, "function") {
			out = append(out, k)
		}
	}
	return out
}

// matcherName reports an expect matcher name such as toBe or toHaveText.
func matcherName(name string) bool {
	return len(name) > 2 && strings.HasPrefix(name, "to") && name[2] >= 'A' && name[2] <= 'Z'
}

// analyze follows an expression of src: a direct read wins, then each helper call it makes.
// block is the enclosing function body, used to follow a returned local variable.
func (c *lintContext) analyze(src *source, expr, block [2]int, depth int, visited map[string]bool) verdict {
	if k := src.directRead(expr[0], expr[1]); k >= 0 {
		return verdict{kind: verdictRead, read: src.tokens[k].Text, at: src.at(k)}
	}
	if inner := src.unwrap(expr); inner[1]-inner[0] == 1 && src.ident(inner[0], "") && block[1] > block[0] {
		if value, ok := src.localValue(block, src.tokens[inner[0]].Text); ok {
			return c.analyze(src, value, [2]int{}, depth, visited)
		}
	}
	out := verdict{}
	for _, k := range src.helperCalls(expr[0], expr[1]) {
		out = better(out, c.resolve(src, k, depth+1, visited))
		if out.kind == verdictRead {
			break
		}
	}
	return out
}

// unwrap strips leading `await`s and enclosing parentheses from an expression range.
func (s *source) unwrap(expr [2]int) [2]int {
	for expr[1] > expr[0] {
		switch {
		case s.ident(expr[0], "await"):
			expr[0]++
		case s.punct(expr[0], "(") && s.match[expr[0]] == expr[1]-1:
			expr = [2]int{expr[0] + 1, expr[1] - 1}
		default:
			return expr
		}
	}
	return expr
}

// localValue returns the initializer of `const|let|var name [: T] = value` declared in block outside
// any nested function.
func (s *source) localValue(block [2]int, name string) ([2]int, bool) {
	for k := block[0] + 1; k < block[1]; k++ {
		if end := s.nestedFunctionEnd(k); end >= k {
			k = end
			continue
		}
		if !(s.ident(k, "const") || s.ident(k, "let") || s.ident(k, "var")) || !s.ident(k+1, name) {
			continue
		}
		j := k + 2
		if s.punct(j, ":") {
			j = s.skipType(j + 1)
		}
		if s.punct(j, "=") && !s.punct(j+1, "=") && !s.punct(j+1, ">") {
			return [2]int{j + 1, s.exprEnd(j+1, block[1])}, true
		}
	}
	return [2]int{}, false
}

// nestedFunctionEnd returns the last token of a function, arrow function or method shorthand that
// starts at token k, or -1, so a scan of an enclosing body can skip it.
func (s *source) nestedFunctionEnd(k int) int {
	if s.ident(k, "") && !keywords[s.tokens[k].Text] && !s.memberAccess(k) && s.punct(k+1, "(") {
		if body, ok := s.blockAfterParams(k + 1); ok {
			return body[1]
		}
	}
	if body, expr, ok := s.functionBody(k); ok {
		if expr {
			return body[1] - 1
		}
		return body[1]
	}
	return -1
}

// candidates returns the definitions named name visible from src: its own and those of the
// modules it imports relatively, in path order.
func (c *lintContext) candidates(src *source, name string) []helperDef {
	defs := append([]helperDef{}, src.definitions()[name]...)
	for _, imported := range c.imports[src.path] {
		if other := c.sources[imported]; other != nil && other != src {
			defs = append(defs, other.definitions()[name]...)
		}
	}
	return defs
}

// resolve follows the helper called at token k of src to the definitions it may name
// (TSA-V0-029, TSA-V0-030).
func (c *lintContext) resolve(src *source, k, depth int, visited map[string]bool) verdict {
	name := src.tokens[k].Text
	defs := c.candidates(src, name)
	if len(defs) == 0 && knownCalls[name] {
		return verdict{}
	}
	if depth > MaxHelperDepth {
		return verdict{kind: verdictUnknown, reason: "helper-following stops after " + strconv.Itoa(MaxHelperDepth) + " hops at " + name + " (" + src.at(k) + ")"}
	}
	if len(defs) == 0 {
		return verdict{kind: verdictUnknown, reason: "no definition of " + name + " in " + src.path + " or the modules it imports relatively"}
	}
	out := verdict{}
	for _, d := range defs {
		key := d.src.path + ":" + strconv.Itoa(d.line) + ":" + name
		if visited[key] {
			continue
		}
		visited[key] = true
		v := verdict{}
		if d.expr {
			v = c.analyze(d.src, d.body, [2]int{}, depth, visited)
		} else {
			for r := d.body[0] + 1; r < d.body[1] && v.kind != verdictRead; r++ {
				// A return inside a nested function returns from that function, not from d.
				if end := d.src.nestedFunctionEnd(r); end >= r {
					r = end
					continue
				}
				if d.src.ident(r, "return") {
					v = better(v, c.analyze(d.src, [2]int{r + 1, d.src.exprEnd(r+1, d.body[1])}, d.body, depth, visited))
				}
			}
		}
		delete(visited, key)
		if v.kind == verdictRead {
			v.via = append([]string{name + " (" + d.src.path + ":" + strconv.Itoa(d.line) + ")"}, v.via...)
		}
		out = better(out, v)
		if out.kind == verdictRead {
			break
		}
	}
	return out
}

// toPassRanges returns the subject ranges of `expect(...).toPass()` chains, whose callbacks retry.
func (s *source) toPassRanges() [][2]int {
	out := [][2]int{}
	for i := range s.tokens {
		if chain, ok := s.readExpect(i); ok && chain.matcher == "toPass" {
			out = append(out, chain.subject)
		}
	}
	return out
}

// lintOneShot flags `expect(await X)` where X is, or returns through a resolved helper, a one-shot
// UI-state read; an unresolved helper yields an UNKNOWN finding (TSA-V0-029, TSA-V0-030).
func (s *source) lintOneShot(c *lintContext, i int, retrying [][2]int, emit func(Finding)) {
	if s.punct(i+1, ".") && s.ident(i+2, "poll") {
		return
	}
	chain, ok := s.readExpect(i)
	if !ok || chain.matcher == "toPass" {
		return
	}
	for _, r := range retrying {
		if within(i, r) {
			return
		}
	}
	awaited := false
	for k := chain.subject[0]; k < chain.subject[1]; k++ {
		awaited = awaited || s.ident(k, "await")
	}
	if !awaited {
		return
	}
	v := c.analyze(s, chain.subject, [2]int{}, 0, map[string]bool{})
	// The finding anchors on the expect line and spans the whole assertion, so a change on any of
	// its lines brings it into the changed-line scope.
	line, end := s.tokens[i].Line, s.tokens[chain.end].Line
	switch v.kind {
	case verdictRead:
		message := "expect(await ...) asserts a one-shot " + v.read + "() read (" + v.at + ")"
		if len(v.via) > 0 {
			message += " returned through " + strings.Join(v.via, " -> ")
		}
		emit(Finding{Rule: RuleOneShotUIState, Path: s.path, Line: line, EndLine: end, Advisory: true,
			Message: message + "; a slower renderer fails it, use the web-first " + uiStateReads[v.read] + " or expect.poll"})
	case verdictUnknown:
		emit(Finding{Rule: RuleOneShotUIState, Path: s.path, Line: line, EndLine: end, Advisory: true, Resolution: "UNKNOWN",
			Message: "cannot tell whether expect(await ...) asserts a one-shot UI-state read: " + v.reason})
	}
}

// The states of a locator builder's `exact` option (TSA-V0-031).
const (
	exactNo = iota
	exactYes
	exactUnknown
)

// exactOption reads the `exact` option of a locator builder call at k: absent or false, true, or
// UNKNOWN with the unresolved source when the options or the value are not literals.
func (s *source) exactOption(k int) (int, string) {
	if !s.ident(k, "") || !exactBuilders[s.tokens[k].Text] || !s.punct(k+1, "(") || s.match[k+1] < 0 {
		return exactNo, ""
	}
	args := s.splitArgs(k+1, s.match[k+1])
	if len(args) < 2 {
		return exactNo, ""
	}
	options := args[1]
	if !s.punct(options[0], "{") || s.match[options[0]] != options[1]-1 {
		return exactUnknown, "options " + s.render(options[0], options[1])
	}
	state, value := exactNo, ""
	// Properties apply in order, so the last `exact` or spread decides.
	for _, prop := range s.splitArgs(options[0], options[1]-1) {
		key := prop[0]
		switch {
		case s.punct(key, ".") || s.punct(key, "..."):
			state, value = exactUnknown, s.render(prop[0], prop[1])
		case !s.ident(key, "exact") && !(s.tokens[key].Kind == appmap.JSString && s.tokens[key].Literal && s.tokens[key].Text == "exact"):
		case prop[1]-key == 1:
			state, value = exactUnknown, "exact"
		case !s.punct(key+1, ":"):
		case prop[1]-key == 3 && s.ident(key+2, "true"):
			state, value = exactYes, ""
		case prop[1]-key == 3 && s.ident(key+2, "false"):
			state, value = exactNo, ""
		default:
			state, value = exactUnknown, s.render(prop[0], prop[1])
		}
	}
	return state, value
}

// exactUses is what following a range for exact-text locators found: every reachable exact
// locator, and every place helper-following could not tell (TSA-V0-030, TSA-V0-031).
type exactUses struct{ where, unknown []string }

func (u *exactUses) merge(o exactUses) {
	u.where = append(u.where, o.where...)
	u.unknown = append(u.unknown, o.unknown...)
}

// sorted returns u with each list sorted and deduplicated.
func (u exactUses) sorted() exactUses {
	return exactUses{where: uniqueSorted(u.where), unknown: uniqueSorted(u.unknown)}
}

func uniqueSorted(in []string) []string {
	out := append([]string{}, in...)
	sort.Strings(out)
	unique := out[:0]
	for i, v := range out {
		if i == 0 || v != out[i-1] {
			unique = append(unique, v)
		}
	}
	return unique
}

// exactLocal reads one locator builder call at k of src as an exact-text use.
func (s *source) exactLocal(k int) exactUses {
	switch state, value := s.exactOption(k); state {
	case exactYes:
		return exactUses{where: []string{s.tokens[k].Text + "(..., { exact: true }) at " + s.at(k)}}
	case exactUnknown:
		return exactUses{unknown: []string{"the exact option of " + s.tokens[k].Text + " at " + s.at(k) + " is not a literal (" + value + ")"}}
	}
	return exactUses{}
}

// exactIn collects the exact-text locators used in body of src, directly or through the helpers it
// calls; hop is the helper hop body belongs to.
func (c *lintContext) exactIn(src *source, body [2]int, hop int, visited map[string]bool) exactUses {
	out := exactUses{}
	for k := body[0]; k < body[1]; k++ {
		out.merge(src.exactLocal(k))
	}
	for _, k := range src.helperCalls(body[0], body[1]) {
		out.merge(c.exactCall(src, k, hop+1, visited))
	}
	return out
}

// exactCall follows the helper called at token k of src, as hop number hop, to every definition it
// may name; a missing definition or a hop past MaxHelperDepth is UNKNOWN, never silence.
func (c *lintContext) exactCall(src *source, k, hop int, visited map[string]bool) exactUses {
	name := src.tokens[k].Text
	defs := c.candidates(src, name)
	switch {
	case len(defs) == 0 && knownCalls[name]:
		return exactUses{}
	case hop > MaxHelperDepth:
		return exactUses{unknown: []string{"helper-following stops after " + strconv.Itoa(MaxHelperDepth) + " hops at " + name + " (" + src.at(k) + ")"}}
	case len(defs) == 0:
		return exactUses{unknown: []string{"no definition of " + name + " in " + src.path + " or the modules it imports relatively"}}
	}
	out := exactUses{}
	for _, d := range defs {
		key := d.src.path + ":" + strconv.Itoa(d.line) + ":" + name
		if visited[key] {
			continue
		}
		visited[key] = true
		out.merge(c.exactIn(d.src, d.body, hop, visited))
		delete(visited, key)
	}
	return out
}

// lintExactVariant names the exact-text locators of a test whose outcome splits by project at the
// analysed revision, one finding per line; a line whose helpers cannot be followed is UNKNOWN
// (TSA-V0-030, TSA-V0-031).
func (s *source) lintExactVariant(c *lintContext, d decl, emit func(Finding)) {
	split, ok := c.splits[s.testKey(d)]
	if !ok {
		return
	}
	cause := split.Passed[0] + " passes and " + split.Failed[0] + " fails at the same revision"
	type lineUses struct {
		line, end int
		uses      exactUses
	}
	lines := []*lineUses{}
	for k := d.body[0]; k < d.body[1]; k++ {
		uses := exactUses{}
		if local := s.exactLocal(k); len(local.where)+len(local.unknown) > 0 {
			uses = local
		} else if len(s.helperCalls(k, k+1)) == 1 {
			followed := c.exactCall(s, k, 1, map[string]bool{}).sorted()
			for _, w := range followed.where {
				uses.where = append(uses.where, s.tokens[k].Text+"() uses "+w)
			}
			for _, u := range followed.unknown {
				uses.unknown = append(uses.unknown, s.tokens[k].Text+"(): "+u)
			}
		}
		if len(uses.where)+len(uses.unknown) == 0 {
			continue
		}
		// The finding anchors on the call and spans it, so a change on any of its lines is in scope.
		line, end := s.tokens[k].Line, s.tokens[k].Line
		if close := s.match[k+1]; close > k {
			end = s.tokens[close].Line
		}
		if n := len(lines); n > 0 && lines[n-1].line == line {
			lines[n-1].uses.merge(uses)
			lines[n-1].end = max(lines[n-1].end, end)
			continue
		}
		lines = append(lines, &lineUses{line: line, end: end, uses: uses})
	}
	for _, l := range lines {
		uses := l.uses.sorted()
		if len(uses.where) == 0 {
			emit(Finding{Rule: RuleExactTextVariant, Path: s.path, Line: l.line, EndLine: l.end, Advisory: true, Resolution: "UNKNOWN",
				Message: cause + "; cannot tell whether an exact-text locator is a likely cause: " + strings.Join(uses.unknown, "; ")})
			continue
		}
		split.ExactText = true
		message := cause + "; the exact-text locator " + strings.Join(uses.where, ", ") +
			" is a likely cause: it cannot match when a variant renders extra inline content in the element"
		if len(uses.unknown) > 0 {
			message += " (not followed: " + strings.Join(uses.unknown, "; ") + ")"
		}
		emit(Finding{Rule: RuleExactTextVariant, Path: s.path, Line: l.line, EndLine: l.end, Advisory: true, Message: message})
	}
	if split.ExactText {
		c.splits[s.testKey(d)] = split
	}
}

// testKey identifies a declaration by path and literal title path, or "" when a title is not a
// literal.
func (s *source) testKey(d decl) string {
	titles := s.describes(d)
	for _, outer := range s.decls {
		if outer.kind == "describe" && outer.at < d.at && within(d.at, outer.body) && !outer.titled {
			return ""
		}
	}
	if !d.titled {
		return ""
	}
	return variantKey(s.path, append(titles, d.title))
}

func variantKey(file string, titles []string) string {
	return file + "\x00" + strings.Join(titles, "\x00")
}

// relativeImports returns the relative module specifiers a source imports, re-exports or requires.
func (s *source) relativeImports() []string {
	out := []string{}
	literal := func(k int) bool {
		return k < len(s.tokens) && s.tokens[k].Kind == appmap.JSString && s.tokens[k].Literal
	}
	for k := range s.tokens {
		spec := ""
		switch {
		case s.ident(k, "from") && literal(k+1):
			spec = s.tokens[k+1].Text
		case s.ident(k, "import") && literal(k+1):
			spec = s.tokens[k+1].Text
		case (s.ident(k, "require") || s.ident(k, "import")) && s.punct(k+1, "(") && literal(k+2) && s.punct(k+3, ")"):
			spec = s.tokens[k+2].Text
		}
		if spec == "." || spec == ".." || strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") {
			out = append(out, spec)
		}
	}
	return out
}

var resolveExtensions = []string{".ts", ".tsx", ".js", ".jsx", ".mts", ".cts", ".mjs", ".cjs"}

// importCandidates lists, in preference order, the repository paths a relative specifier may name.
func importCandidates(from, spec string) []string {
	base := path.Join(path.Dir(from), spec)
	if strings.HasPrefix(base, "../") || base == ".." {
		return nil
	}
	out := []string{}
	if ext := path.Ext(base); webExtensions[ext] {
		out = append(out, base)
		if ext == ".js" || ext == ".mjs" || ext == ".cjs" || ext == ".jsx" {
			stem := strings.TrimSuffix(base, ext)
			out = append(out, stem+".ts", stem+".tsx", stem+".mts", stem+".cts")
		}
	}
	for _, ext := range resolveExtensions {
		out = append(out, base+ext)
	}
	for _, ext := range resolveExtensions {
		out = append(out, base+"/index"+ext)
	}
	return out
}

// linkImports records, for each named source, the read modules its relative imports resolve to.
func (c *lintContext) linkImports(names []string) {
	for _, name := range names {
		seen := map[string]bool{}
		for _, spec := range c.sources[name].relativeImports() {
			for _, candidate := range importCandidates(name, spec) {
				if c.sources[candidate] != nil {
					if !seen[candidate] {
						seen[candidate] = true
						c.imports[name] = append(c.imports[name], candidate)
					}
					break
				}
			}
		}
		sort.Strings(c.imports[name])
	}
}

// readImports reads, at the revision, the modules the linted files import relatively, and theirs,
// up to MaxHelperDepth levels, so helpers resolve across files (TSA-V0-030). Every level draws on
// the command's one read budget.
func (c *lintContext) readImports(r Repo, revision string, budget *readBudget) error {
	frontier := make([]string, 0, len(c.sources))
	for name := range c.sources {
		frontier = append(frontier, name)
	}
	sort.Strings(frontier)
	for level := 0; level < MaxHelperDepth && len(frontier) > 0; level++ {
		wanted := map[string]bool{}
		for _, name := range frontier {
			for _, spec := range c.sources[name].relativeImports() {
				for _, candidate := range importCandidates(name, spec) {
					if c.sources[candidate] == nil {
						wanted[candidate] = true
					}
				}
			}
		}
		next := []string{}
		if len(wanted) > 0 {
			texts, _, err := r.readFilesWithin(revision, func(name string) bool { return wanted[name] }, budget)
			if err != nil {
				return err
			}
			for name, text := range texts {
				c.sources[name] = parseSource(name, text)
				next = append(next, name)
			}
		}
		c.linkImports(frontier)
		sort.Strings(next)
		frontier = next
	}
	c.linkImports(frontier)
	return nil
}
