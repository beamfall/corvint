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
	for _, name := range strings.Fields(`if for while switch catch return typeof function new await super do else with void delete in of`) {
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

// helperCalls returns the calls in [start, end) that may be page-object helpers.
func (s *source) helperCalls(start, end int) []int {
	out := []int{}
	for k := start; k < end; k++ {
		if s.ident(k, "") && s.punct(k+1, "(") && !keywords[s.tokens[k].Text] && uiStateReads[s.tokens[k].Text] == "" &&
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
	if expr[1]-expr[0] == 1 && src.ident(expr[0], "") && block[1] > block[0] {
		name := src.tokens[expr[0]].Text
		for k := block[0]; k < block[1]; k++ {
			if (src.ident(k, "const") || src.ident(k, "let") || src.ident(k, "var")) && src.ident(k+1, name) && src.punct(k+2, "=") {
				return c.analyze(src, [2]int{k + 3, src.exprEnd(k+3, block[1])}, [2]int{}, depth, visited)
			}
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
	line := s.tokens[i].Line
	switch v.kind {
	case verdictRead:
		message := "expect(await ...) asserts a one-shot " + v.read + "() read (" + v.at + ")"
		if len(v.via) > 0 {
			message += " returned through " + strings.Join(v.via, " -> ")
		}
		emit(Finding{Rule: RuleOneShotUIState, Path: s.path, Line: line, EndLine: line, Advisory: true,
			Message: message + "; a slower renderer fails it, use the web-first " + uiStateReads[v.read] + " or expect.poll"})
	case verdictUnknown:
		emit(Finding{Rule: RuleOneShotUIState, Path: s.path, Line: line, EndLine: line, Advisory: true, Resolution: "UNKNOWN",
			Message: "cannot tell whether expect(await ...) asserts a one-shot UI-state read: " + v.reason})
	}
}

// exactLocator reports a locator builder call at k whose options pass `exact: true`.
func (s *source) exactLocator(k int) bool {
	if !s.ident(k, "") || !exactBuilders[s.tokens[k].Text] || !s.punct(k+1, "(") || s.match[k+1] < 0 {
		return false
	}
	for j := k + 2; j < s.match[k+1]; j++ {
		if s.ident(j, "exact") && s.punct(j+1, ":") && s.ident(j+2, "true") {
			return true
		}
	}
	return false
}

// exactIn returns where an exact-text locator is used in [start, end) of src, directly or through
// helpers it calls, up to MaxHelperDepth hops (TSA-V0-031).
func (c *lintContext) exactIn(src *source, body [2]int, depth int, visited map[string]bool) string {
	for k := body[0]; k < body[1]; k++ {
		if src.exactLocator(k) {
			return src.tokens[k].Text + "(..., { exact: true }) at " + src.at(k)
		}
	}
	if depth >= MaxHelperDepth {
		return ""
	}
	for _, k := range src.helperCalls(body[0], body[1]) {
		for _, d := range c.candidates(src, src.tokens[k].Text) {
			key := d.src.path + ":" + strconv.Itoa(d.line)
			if visited[key] {
				continue
			}
			visited[key] = true
			if where := c.exactIn(d.src, d.body, depth+1, visited); where != "" {
				return where
			}
		}
	}
	return ""
}

// lintExactVariant names the exact-text locators of a test whose outcome splits by project at the
// analysed revision (TSA-V0-031).
func (s *source) lintExactVariant(c *lintContext, d decl, emit func(Finding)) {
	split, ok := c.splits[s.testKey(d)]
	if !ok {
		return
	}
	cause := split.Passed[0] + " passes and " + split.Failed[0] + " fails at the same revision"
	reported := false
	for k := d.body[0]; k < d.body[1]; k++ {
		where := ""
		switch {
		case s.exactLocator(k):
			where = s.tokens[k].Text + "(..., { exact: true })"
		case len(s.helperCalls(k, k+1)) == 1:
			for _, def := range c.candidates(s, s.tokens[k].Text) {
				if where = c.exactIn(def.src, def.body, 1, map[string]bool{}); where != "" {
					where = s.tokens[k].Text + "() uses " + where
					break
				}
			}
		}
		if where == "" {
			continue
		}
		reported = true
		line := s.tokens[k].Line
		emit(Finding{Rule: RuleExactTextVariant, Path: s.path, Line: line, EndLine: line, Advisory: true,
			Message: cause + "; the exact-text locator " + where + " is a likely cause: it cannot match when a variant renders extra inline content in the element"})
	}
	if reported {
		split.ExactText = true
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
// up to MaxHelperDepth levels, so helpers resolve across files (TSA-V0-030).
func (c *lintContext) readImports(r Repo, revision string) error {
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
			texts, _, err := r.readFiles(revision, func(name string) bool { return wanted[name] })
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
