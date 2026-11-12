package testsource

import (
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/testdiscovery"
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

// helperDef is testdiscovery.HelperDef, kept under its old name.
type helperDef = testdiscovery.HelperDef

// definitions indexes the named function, method and arrow-function definitions of a source.
func definitions(s *source) map[string][]helperDef {
	if s.Defs != nil {
		return s.Defs
	}
	s.Defs = map[string][]helperDef{}
	add := func(name int, body [2]int, expr bool) {
		text := s.Tokens[name].Text
		s.Defs[text] = append(s.Defs[text], helperDef{Src: s, Line: s.Tokens[name].Line, Body: body, Expr: expr})
	}
	for i := range s.Tokens {
		if !s.Ident(i, "") || s.MemberAccess(i) || keywords[s.Tokens[i].Text] {
			continue
		}
		switch {
		case s.Ident(i-1, "function") && s.Punct(i+1, "("):
			if body, ok := s.BlockAfterParams(i + 1); ok {
				add(i, body, false)
			}
		case s.Punct(i+1, "(") && memberStart(s, i-1):
			if body, ok := s.BlockAfterParams(i + 1); ok {
				add(i, body, false)
			}
		case s.Punct(i+1, "=") && !s.Punct(i+2, "=") && !s.Punct(i+2, ">"), s.Punct(i+1, ":"):
			value := i + 2
			if s.Punct(i+1, ":") && !functionValue(s, value) {
				if j := s.SkipType(value); s.Punct(j, "=") && !s.Punct(j+1, ">") {
					value = j + 1
				}
			}
			if body, expr, ok := s.FunctionBody(value); ok {
				add(i, body, expr)
			}
		}
	}
	return s.Defs
}

// memberStart reports a token after which a class or object member, or a statement, begins.
func memberStart(s *source, j int) bool {
	if j < 0 || s.Punct(j, "{") || s.Punct(j, "}") || s.Punct(j, ";") || s.Punct(j, ",") {
		return true
	}
	for _, word := range []string{"async", "static", "public", "private", "protected", "override", "get"} {
		if s.Ident(j, word) {
			return true
		}
	}
	return false
}

// functionValue reports a function or arrow function starting at token v.
func functionValue(s *source, v int) bool {
	_, _, ok := s.FunctionBody(v)
	return ok
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

func sourceAt(s *source, i int) string { return s.Path + ":" + strconv.Itoa(s.Tokens[i].Line) }

// directRead returns the first `.read(` UI-state call in [start, end), or -1.
func directRead(s *source, start, end int) int {
	for k := start; k < end; k++ {
		if s.Ident(k, "") && uiStateReads[s.Tokens[k].Text] != "" && s.MemberAccess(k) && s.Punct(k+1, "(") {
			return k
		}
	}
	return -1
}

// helperCalls returns the calls in [start, end) that may be page-object helpers. A UI-state read
// name is a helper only when it is called standalone, not as a member read.
func helperCalls(s *source, start, end int) []int {
	out := []int{}
	for k := start; k < end; k++ {
		if s.Ident(k, "") && s.Punct(k+1, "(") && !keywords[s.Tokens[k].Text] && !(uiStateReads[s.Tokens[k].Text] != "" && s.MemberAccess(k)) &&
			!matcherName(s.Tokens[k].Text) && !s.Ident(k-1, "new") && !s.Ident(k-1, "function") {
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
	if k := directRead(src, expr[0], expr[1]); k >= 0 {
		return verdict{kind: verdictRead, read: src.Tokens[k].Text, at: sourceAt(src, k)}
	}
	if inner := unwrap(src, expr); inner[1]-inner[0] == 1 && src.Ident(inner[0], "") && block[1] > block[0] {
		if value, ok := localValue(src, block, src.Tokens[inner[0]].Text); ok {
			return c.analyze(src, value, [2]int{}, depth, visited)
		}
	}
	out := verdict{}
	for _, k := range helperCalls(src, expr[0], expr[1]) {
		out = better(out, c.resolve(src, k, depth+1, visited))
		if out.kind == verdictRead {
			break
		}
	}
	return out
}

// unwrap strips leading `await`s and enclosing parentheses from an expression range.
func unwrap(s *source, expr [2]int) [2]int {
	for expr[1] > expr[0] {
		switch {
		case s.Ident(expr[0], "await"):
			expr[0]++
		case s.Punct(expr[0], "(") && s.Match[expr[0]] == expr[1]-1:
			expr = [2]int{expr[0] + 1, expr[1] - 1}
		default:
			return expr
		}
	}
	return expr
}

// localValue returns the initializer of `const|let|var name [: T] = value` declared in block outside
// any nested function.
func localValue(s *source, block [2]int, name string) ([2]int, bool) {
	for k := block[0] + 1; k < block[1]; k++ {
		if end := nestedFunctionEnd(s, k); end >= k {
			k = end
			continue
		}
		if !(s.Ident(k, "const") || s.Ident(k, "let") || s.Ident(k, "var")) || !s.Ident(k+1, name) {
			continue
		}
		j := k + 2
		if s.Punct(j, ":") {
			j = s.SkipType(j + 1)
		}
		if s.Punct(j, "=") && !s.Punct(j+1, "=") && !s.Punct(j+1, ">") {
			return [2]int{j + 1, s.ExprEnd(j+1, block[1])}, true
		}
	}
	return [2]int{}, false
}

// nestedFunctionEnd returns the last token of a function, arrow function or method shorthand that
// starts at token k, or -1, so a scan of an enclosing body can skip it.
func nestedFunctionEnd(s *source, k int) int {
	if s.Ident(k, "") && !keywords[s.Tokens[k].Text] && !s.MemberAccess(k) && s.Punct(k+1, "(") {
		if body, ok := s.BlockAfterParams(k + 1); ok {
			return body[1]
		}
	}
	if body, expr, ok := s.FunctionBody(k); ok {
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
	defs := append([]helperDef{}, definitions(src)[name]...)
	for _, imported := range c.imports[src.Path] {
		if other := c.sources[imported]; other != nil && other != src {
			defs = append(defs, definitions(other)[name]...)
		}
	}
	return defs
}

// resolve follows the helper called at token k of src to the definitions it may name
// (TSA-V0-029, TSA-V0-030).
func (c *lintContext) resolve(src *source, k, depth int, visited map[string]bool) verdict {
	name := src.Tokens[k].Text
	defs := c.candidates(src, name)
	if len(defs) == 0 && knownCalls[name] {
		return verdict{}
	}
	if depth > MaxHelperDepth {
		return verdict{kind: verdictUnknown, reason: "helper-following stops after " + strconv.Itoa(MaxHelperDepth) + " hops at " + name + " (" + sourceAt(src, k) + ")"}
	}
	if len(defs) == 0 {
		return verdict{kind: verdictUnknown, reason: "no definition of " + name + " in " + src.Path + " or the modules it imports relatively"}
	}
	out := verdict{}
	for _, d := range defs {
		key := d.Src.Path + ":" + strconv.Itoa(d.Line) + ":" + name
		if visited[key] {
			continue
		}
		visited[key] = true
		v := verdict{}
		if d.Expr {
			v = c.analyze(d.Src, d.Body, [2]int{}, depth, visited)
		} else {
			for r := d.Body[0] + 1; r < d.Body[1] && v.kind != verdictRead; r++ {
				// A return inside a nested function returns from that function, not from d.
				if end := nestedFunctionEnd(d.Src, r); end >= r {
					r = end
					continue
				}
				if d.Src.Ident(r, "return") {
					v = better(v, c.analyze(d.Src, [2]int{r + 1, d.Src.ExprEnd(r+1, d.Body[1])}, d.Body, depth, visited))
				}
			}
		}
		delete(visited, key)
		if v.kind == verdictRead {
			v.via = append([]string{name + " (" + d.Src.Path + ":" + strconv.Itoa(d.Line) + ")"}, v.via...)
		}
		out = better(out, v)
		if out.kind == verdictRead {
			break
		}
	}
	return out
}

// toPassRanges returns the subject ranges of `expect(...).toPass()` chains, whose callbacks retry.
func toPassRanges(s *source) [][2]int {
	out := [][2]int{}
	for i := range s.Tokens {
		if chain, ok := s.ReadExpect(i); ok && chain.Matcher == "toPass" {
			out = append(out, chain.Subject)
		}
	}
	return out
}

// lintOneShot flags `expect(await X)` where X is, or returns through a resolved helper, a one-shot
// UI-state read; an unresolved helper yields an UNKNOWN finding (TSA-V0-029, TSA-V0-030).
func lintOneShot(s *source, c *lintContext, i int, retrying [][2]int, emit func(Finding)) {
	if s.Punct(i+1, ".") && s.Ident(i+2, "poll") {
		return
	}
	chain, ok := s.ReadExpect(i)
	if !ok || chain.Matcher == "toPass" {
		return
	}
	for _, r := range retrying {
		if within(i, r) {
			return
		}
	}
	awaited := false
	for k := chain.Subject[0]; k < chain.Subject[1]; k++ {
		awaited = awaited || s.Ident(k, "await")
	}
	if !awaited {
		return
	}
	v := c.analyze(s, chain.Subject, [2]int{}, 0, map[string]bool{})
	// The finding anchors on the expect line and spans the whole assertion, so a change on any of
	// its lines brings it into the changed-line scope.
	line, end := s.Tokens[i].Line, s.Tokens[chain.End].Line
	switch v.kind {
	case verdictRead:
		message := "expect(await ...) asserts a one-shot " + v.read + "() read (" + v.at + ")"
		if len(v.via) > 0 {
			message += " returned through " + strings.Join(v.via, " -> ")
		}
		emit(Finding{Rule: RuleOneShotUIState, Path: s.Path, Line: line, EndLine: end, Advisory: true,
			Message: message + "; a slower renderer fails it, use the web-first " + uiStateReads[v.read] + " or expect.poll"})
	case verdictUnknown:
		emit(Finding{Rule: RuleOneShotUIState, Path: s.Path, Line: line, EndLine: end, Advisory: true, Resolution: "UNKNOWN",
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
func exactOption(s *source, k int) (int, string) {
	if !s.Ident(k, "") || !exactBuilders[s.Tokens[k].Text] || !s.Punct(k+1, "(") || s.Match[k+1] < 0 {
		return exactNo, ""
	}
	args := s.SplitArgs(k+1, s.Match[k+1])
	if len(args) < 2 {
		return exactNo, ""
	}
	options := args[1]
	if !s.Punct(options[0], "{") || s.Match[options[0]] != options[1]-1 {
		return exactUnknown, "options " + s.Render(options[0], options[1])
	}
	state, value := exactNo, ""
	// Properties apply in order, so the last `exact` or spread decides.
	for _, prop := range s.SplitArgs(options[0], options[1]-1) {
		key := prop[0]
		switch {
		case s.Punct(key, ".") || s.Punct(key, "..."):
			state, value = exactUnknown, s.Render(prop[0], prop[1])
		case !s.Ident(key, "exact") && !(s.Tokens[key].Kind == appmap.JSString && s.Tokens[key].Literal && s.Tokens[key].Text == "exact"):
		case prop[1]-key == 1:
			state, value = exactUnknown, "exact"
		case !s.Punct(key+1, ":"):
		case prop[1]-key == 3 && s.Ident(key+2, "true"):
			state, value = exactYes, ""
		case prop[1]-key == 3 && s.Ident(key+2, "false"):
			state, value = exactNo, ""
		default:
			state, value = exactUnknown, s.Render(prop[0], prop[1])
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
func exactLocal(s *source, k int) exactUses {
	switch state, value := exactOption(s, k); state {
	case exactYes:
		return exactUses{where: []string{s.Tokens[k].Text + "(..., { exact: true }) at " + sourceAt(s, k)}}
	case exactUnknown:
		return exactUses{unknown: []string{"the exact option of " + s.Tokens[k].Text + " at " + sourceAt(s, k) + " is not a literal (" + value + ")"}}
	}
	return exactUses{}
}

// exactIn collects the exact-text locators used in body of src, directly or through the helpers it
// calls; hop is the helper hop body belongs to.
func (c *lintContext) exactIn(src *source, body [2]int, hop int, visited map[string]bool) exactUses {
	out := exactUses{}
	for k := body[0]; k < body[1]; k++ {
		out.merge(exactLocal(src, k))
	}
	for _, k := range helperCalls(src, body[0], body[1]) {
		out.merge(c.exactCall(src, k, hop+1, visited))
	}
	return out
}

// exactCall follows the helper called at token k of src, as hop number hop, to every definition it
// may name; a missing definition or a hop past MaxHelperDepth is UNKNOWN, never silence.
func (c *lintContext) exactCall(src *source, k, hop int, visited map[string]bool) exactUses {
	name := src.Tokens[k].Text
	defs := c.candidates(src, name)
	switch {
	case len(defs) == 0 && knownCalls[name]:
		return exactUses{}
	case hop > MaxHelperDepth:
		return exactUses{unknown: []string{"helper-following stops after " + strconv.Itoa(MaxHelperDepth) + " hops at " + name + " (" + sourceAt(src, k) + ")"}}
	case len(defs) == 0:
		return exactUses{unknown: []string{"no definition of " + name + " in " + src.Path + " or the modules it imports relatively"}}
	}
	out := exactUses{}
	for _, d := range defs {
		key := d.Src.Path + ":" + strconv.Itoa(d.Line) + ":" + name
		if visited[key] {
			continue
		}
		visited[key] = true
		out.merge(c.exactIn(d.Src, d.Body, hop, visited))
		delete(visited, key)
	}
	return out
}

// lintExactVariant names the exact-text locators of a test whose outcome splits by project at the
// analysed revision, one finding per line; a line whose helpers cannot be followed is UNKNOWN
// (TSA-V0-030, TSA-V0-031).
func lintExactVariant(s *source, c *lintContext, d decl, emit func(Finding)) {
	split, ok := c.splits[testKey(s, d)]
	if !ok {
		return
	}
	cause := split.Passed[0] + " passes and " + split.Failed[0] + " fails at the same revision"
	type lineUses struct {
		line, end int
		uses      exactUses
	}
	lines := []*lineUses{}
	for k := d.Body[0]; k < d.Body[1]; k++ {
		uses := exactUses{}
		if local := exactLocal(s, k); len(local.where)+len(local.unknown) > 0 {
			uses = local
		} else if len(helperCalls(s, k, k+1)) == 1 {
			followed := c.exactCall(s, k, 1, map[string]bool{}).sorted()
			for _, w := range followed.where {
				uses.where = append(uses.where, s.Tokens[k].Text+"() uses "+w)
			}
			for _, u := range followed.unknown {
				uses.unknown = append(uses.unknown, s.Tokens[k].Text+"(): "+u)
			}
		}
		if len(uses.where)+len(uses.unknown) == 0 {
			continue
		}
		// The finding anchors on the call and spans it, so a change on any of its lines is in scope.
		line, end := s.Tokens[k].Line, s.Tokens[k].Line
		if close := s.Match[k+1]; close > k {
			end = s.Tokens[close].Line
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
			emit(Finding{Rule: RuleExactTextVariant, Path: s.Path, Line: l.line, EndLine: l.end, Advisory: true, Resolution: "UNKNOWN",
				Message: cause + "; cannot tell whether an exact-text locator is a likely cause: " + strings.Join(uses.unknown, "; ")})
			continue
		}
		split.ExactText = true
		message := cause + "; the exact-text locator " + strings.Join(uses.where, ", ") +
			" is a likely cause: it cannot match when a variant renders extra inline content in the element"
		if len(uses.unknown) > 0 {
			message += " (not followed: " + strings.Join(uses.unknown, "; ") + ")"
		}
		emit(Finding{Rule: RuleExactTextVariant, Path: s.Path, Line: l.line, EndLine: l.end, Advisory: true, Message: message})
	}
	if split.ExactText {
		c.splits[testKey(s, d)] = split
	}
}

// testKey identifies a declaration by path and literal title path, or "" when a title is not a
// literal.
func testKey(s *source, d decl) string {
	titles := describes(s, d)
	for _, outer := range s.Decls {
		if outer.Kind == "describe" && outer.At < d.At && within(d.At, outer.Body) && !outer.Titled {
			return ""
		}
	}
	if !d.Titled {
		return ""
	}
	return variantKey(s.Path, append(titles, d.Title))
}

func variantKey(file string, titles []string) string {
	return file + "\x00" + strings.Join(titles, "\x00")
}

// relativeImports returns the relative module specifiers a source imports, re-exports or requires.
func relativeImports(s *source) []string {
	out := []string{}
	literal := func(k int) bool {
		return k < len(s.Tokens) && s.Tokens[k].Kind == appmap.JSString && s.Tokens[k].Literal
	}
	for k := range s.Tokens {
		spec := ""
		switch {
		case s.Ident(k, "from") && literal(k+1):
			spec = s.Tokens[k+1].Text
		case s.Ident(k, "import") && literal(k+1):
			spec = s.Tokens[k+1].Text
		case (s.Ident(k, "require") || s.Ident(k, "import")) && s.Punct(k+1, "(") && literal(k+2) && s.Punct(k+3, ")"):
			spec = s.Tokens[k+2].Text
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
		for _, spec := range relativeImports(c.sources[name]) {
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
			for _, spec := range relativeImports(c.sources[name]) {
				for _, candidate := range importCandidates(name, spec) {
					if c.sources[candidate] == nil {
						wanted[candidate] = true
					}
				}
			}
		}
		next := []string{}
		if len(wanted) > 0 {
			texts, _, err := readFilesWithin(r, revision, func(name string) bool { return wanted[name] }, budget)
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
