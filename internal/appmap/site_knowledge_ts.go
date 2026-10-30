package appmap

import (
	"path"
	"slices"
	"strings"
)

// The TypeScript reader of site knowledge (SKH-V0-010, SKH-V0-020): Playwright and Cypress page
// objects written as classes or as modules of functions that take the page, and the test bodies
// that drive them. It reads tokens only; it never evaluates code.

type skImport struct{ base, name string }

var (
	skModifiers = wordSet("public private protected static readonly override async abstract declare")
	// Methods that read state; they add no step.
	skReadMethods = wordSet("textContent innerText innerHTML count isVisible isHidden isEnabled isDisabled isChecked " +
		"isEditable getAttribute inputValue boundingBox all allTextContents allInnerTexts its invoke then as url title " +
		"content screenshot waitForTimeout elementHandle elementHandles")
	// Interactions the closed IR cannot express: kept as opaque, never dropped (SKH-V0-020).
	skOpaqueMethods = wordSet("blur clear dragTo dispatchEvent evaluate evaluateHandle " +
		"scrollIntoViewIfNeeded trigger rightclick reload goBack goForward keyboard mouse touchscreen scrollTo")
	// Page methods that arrange the run rather than act on the UI: stage 3 setup (SKH-V0-030).
	skSetupPageMethods = wordSet("route unroute routeFromHAR addInitScript exposeFunction exposeBinding setExtraHTTPHeaders " +
		"setViewportSize emulateMedia on once context request intercept task fixture session clearCookies clearLocalStorage setCookie")
	// Producers of a locator, beyond the Playwright getBy family in selectorKinds.
	skCypressProducers = map[string]struct{ kind, strength string }{
		"get": {"css", strengthWeak}, "find": {"css", strengthWeak}, "contains": {"text", strengthWeak},
	}
	skLegacySelectorFirst = wordSet("click dblclick fill type check uncheck selectOption setInputFiles tap hover focus press waitForSelector")
	skControl             = wordSet("if for while switch try do")
	skUnsupportedImports  = map[string]string{"selenium-webdriver": "selenium-js", "webdriverio": "webdriverio",
		"protractor": "protractor", "nightwatch": "nightwatch", "testcafe": "testcafe", "puppeteer": "puppeteer"}
	skTestCalls  = wordSet("test it specify")
	skQuietRoots = wordSet("console test expect Math JSON Object Array String Number Date")
)

// readSKTS reads one TypeScript or JavaScript source.
func readSKTS(s skSource, m *SKModel) *skFile {
	toks, _ := lexJS(string(s.data))
	f := &skFile{path: s.entry.path, entry: s.entry, data: s.data, toks: toks, classes: map[string]*skClass{},
		funcs: map[string]*skUnit{}, imports: map[string]skImport{}, nested: map[string]bool{}, shadowed: map[string]bool{}, consts: map[string]string{}}
	for i := 0; i+1 < len(toks); i++ {
		if toks[i].kind == tokIdent && toks[i].text == "cy" && next(toks, i+1, ".") && (i == 0 || !next(toks, i-1, ".")) {
			f.cypress = true
			break
		}
	}
	if form := f.readImports(); form != "" {
		m.Unknowns = append(m.Unknowns, SKUnknown{Path: f.path, Form: form, Construct: "import", Knowledge: "page-objects",
			Reason: "form-unsupported"})
		return nil
	}
	busy := make([]bool, len(toks))
	depth := 0
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if t.kind == tokPunct {
			switch t.text {
			case "{", "(", "[":
				depth++
			case "}", ")", "]":
				depth--
			}
			continue
		}
		if depth != 0 && t.kind == tokIdent && t.text == "function" && skStmtStart(toks, i) {
			// A helper declared inside a describe callback is a spec-local helper (SKH-V0-012).
			if end := f.readFunction(i, true); end > i {
				i = end
			}
			continue
		}
		if depth != 0 || t.kind != tokIdent || (i > 0 && next(toks, i-1, ".")) {
			continue
		}
		end := -1
		switch t.text {
		case "class":
			end = f.readClass(m, i)
		case "function":
			end = f.readFunction(i, false)
		case "const", "let", "var":
			end = f.readArrow(i)
		}
		if end > i {
			for k := i; k <= end && k < len(busy); k++ {
				busy[k] = true
			}
			i = end
		}
	}
	f.readTests(busy)
	for _, n := range f.forder {
		u := f.funcs[n]
		u.root = len(u.pageArgs) > 0 || (f.cypress && f.usesCy(u.from, u.to))
	}
	return f
}

func (f *skFile) usesCy(from, to int) bool {
	for k := from; k+1 < to; k++ {
		if f.toks[k].kind == tokIdent && f.toks[k].text == "cy" && next(f.toks, k+1, ".") {
			return true
		}
	}
	return false
}

// readImports records relative imports and reports an unsupported framework import.
func (f *skFile) readImports() string {
	toks := f.toks
	for i := 0; i < len(toks); i++ {
		if toks[i].kind != tokIdent || toks[i].text != "import" || (i > 0 && next(toks, i-1, ".")) {
			continue
		}
		names := map[string]string{}
		j := i + 1
		if j < len(toks) && toks[j].kind == tokIdent && toks[j].text == "type" {
			j++
		}
		if j < len(toks) && toks[j].kind == tokIdent && toks[j].text != "from" {
			names[toks[j].text] = "default"
			j++
			if next(toks, j, ",") {
				j++
			}
		}
		if next(toks, j, "{") {
			end := closeParen(toks, j)
			for k := j + 1; k < end; k++ {
				if toks[k].kind != tokIdent || toks[k].text == "type" {
					continue
				}
				exported, local := toks[k].text, toks[k].text
				if k+2 < end && toks[k+1].kind == tokIdent && toks[k+1].text == "as" && toks[k+2].kind == tokIdent {
					local = toks[k+2].text
					k += 2
				}
				names[local] = exported
			}
			j = end + 1
		} else if next(toks, j, "*") {
			j += 3
		}
		if j+1 >= len(toks) || toks[j].kind != tokIdent || toks[j].text != "from" || !literal(toks[j+1]) {
			if j < len(toks) && literal(toks[j]) { // side-effect import
				if form := skUnsupportedImports[toks[j].text]; form != "" {
					return form
				}
			}
			continue
		}
		spec := toks[j+1].text
		if form := skUnsupportedImports[strings.SplitN(spec, "/", 2)[0]]; form != "" {
			return form
		}
		if !strings.HasPrefix(spec, ".") {
			continue
		}
		base := path.Join(path.Dir(f.path), spec)
		for local, exported := range names {
			f.imports[local] = skImport{base: base, name: exported}
		}
	}
	return ""
}

// skParamEnd returns the index of the `,` or close paren that ends the parameter at k. Angle
// brackets count as nesting inside a type annotation (`x: Record<string, T>`).
func skParamEnd(toks []token, k, close int) int {
	depth, typed := 0, false
	for ; k < close; k++ {
		t := toks[k]
		if t.kind != tokPunct {
			continue
		}
		switch {
		case t.text == "(" || t.text == "[" || t.text == "{":
			depth++
		case t.text == ")" || t.text == "]" || t.text == "}":
			depth--
		case t.text == ":" && depth == 0:
			typed = true
		case t.text == "=" && depth == 0:
			typed = false
		case typed && t.text == "<":
			depth++
		case typed && t.text == ">" && !(k > 0 && toks[k-1].text == "=" && t.glued):
			depth--
		case t.text == "," && depth == 0:
			return k
		}
	}
	return close
}

// skParams reads a parameter list between the parens at open and close.
func skParams(toks []token, open, close int) ([]string, map[string]bool) {
	names, page := []string{}, map[string]bool{}
	for k := open + 1; k < close; {
		end := skParamEnd(toks, k, close)
		j := k
		for j < end && toks[j].kind == tokIdent && skModifiers[toks[j].text] {
			j++
		}
		switch {
		case j < end && next(toks, j, "{"):
			names = append(names, "{}")
			inner := closeParen(toks, j)
			for q := j + 1; q < inner; q++ {
				if toks[q].kind == tokIdent && (next(toks, q+1, ",", "}", "=") || q+1 == inner) && !next(toks, q-1, ":") {
					if toks[q].text == "page" {
						page["page"] = true
					}
				}
			}
		case j < end && toks[j].kind == tokIdent:
			name := toks[j].text
			names = append(names, name)
			q := j + 1
			if next(toks, q, "?") {
				q++
			}
			if name == "page" || (next(toks, q, ":") && q+1 < end && toks[q+1].kind == tokIdent && toks[q+1].text == "Page") {
				page[name] = true
			}
		default:
			names = append(names, "")
		}
		k = end + 1
	}
	return names, page
}

// skBodyOpen finds the '{' that opens a function body after its parameter list closes at close.
func skBodyOpen(toks []token, close, limit int) int {
	// After a return type annotation, a brace inside type arguments or in a type position is an
	// object type, not the body: `): Promise<{ a: Locator }> {` opens its body at the last brace.
	typed, angle := next(toks, close+1, ":"), 0
	for k := close + 1; k < limit && k < len(toks); k++ {
		switch {
		case next(toks, k, ";"):
			return -1
		case !typed:
			if next(toks, k, "{") {
				return k
			}
		case next(toks, k, "<"):
			angle++
		case next(toks, k, ">") && !next(toks, k-1, "="):
			angle = max(angle-1, 0)
		case next(toks, k, "(") || next(toks, k, "["):
			k = closeParen(toks, k)
		case next(toks, k, "{"):
			if angle == 0 && !next(toks, k-1, ":") && !next(toks, k-1, "|") && !next(toks, k-1, "&") && !next(toks, k-1, ",") {
				return k
			}
			k = closeParen(toks, k)
		}
	}
	return -1
}

func (f *skFile) newUnit(c *skClass, name string, open, close, body int) *skUnit {
	toks := f.toks
	end := closeParen(toks, body)
	params, page := skParams(toks, open, close)
	return &skUnit{file: f, class: c, name: name, params: params, pageArgs: page, from: body + 1, to: end, open: open,
		start: toks[open].line, end: toks[end].line}
}

// readFunction reads `function NAME(params) { body }` at toks[i].
func (f *skFile) readFunction(i int, nested bool) int {
	toks := f.toks
	j := i + 1
	if next(toks, j, "*") {
		j++
	}
	if j+1 >= len(toks) || toks[j].kind != tokIdent {
		return -1
	}
	name := toks[j].text
	open := j + 1
	for open < len(toks) && !next(toks, open, "(") {
		open++
	}
	if open >= len(toks) {
		return -1
	}
	close := closeParen(toks, open)
	body := skBodyOpen(toks, close, len(toks))
	if body < 0 {
		return close
	}
	u := f.newUnit(nil, name, open, close, body)
	u.start = toks[i].line
	if !nested {
		f.addFunc(u)
	} else if f.funcs[name] == nil && !f.shadowed[name] {
		f.addFunc(u)
		f.nested[name] = true
	} else if f.nested[name] {
		// Two nested helpers share a name in different scopes: neither is a guess.
		delete(f.funcs, name)
		f.shadowed[name] = true
		for k, n := range f.forder {
			if n == name {
				f.forder = append(f.forder[:k], f.forder[k+1:]...)
				break
			}
		}
	}
	return closeParen(toks, body)
}

// skStmtStart reports whether toks[i] (`function`) starts a declaration statement rather than
// a function expression.
func skStmtStart(toks []token, i int) bool {
	k := i - 1
	if k >= 0 && toks[k].kind == tokIdent && toks[k].text == "async" {
		k--
	}
	return k < 0 || next(toks, k, "{") || next(toks, k, "}") || next(toks, k, ";")
}

func (f *skFile) addFunc(u *skUnit) {
	if f.funcs[u.name] == nil {
		f.forder = append(f.forder, u.name)
	}
	f.funcs[u.name] = u
}

// readArrow reads `const NAME = [async] (params) => { body }` or `= [async] function (...) {...}`.
func (f *skFile) readArrow(i int) int {
	toks := f.toks
	if i+2 >= len(toks) || toks[i+1].kind != tokIdent {
		return -1
	}
	name := toks[i+1].text
	j := i + 2
	if next(toks, j, ":") { // a type annotation
		for j < len(toks) && !next(toks, j, "=") && !next(toks, j, ";") && toks[j].line == toks[i].line {
			j++
		}
	}
	if !next(toks, j, "=") || next(toks, j+1, "=", ">") {
		return -1
	}
	j++
	if toks[i].text == "const" && j < len(toks) && literal(toks[j]) && (j+1 == len(toks) || next(toks, j+1, ";") || toks[j+1].line != toks[j].line) {
		f.consts[name] = toks[j].text // a module-level string constant templates may read
		return j
	}
	if j < len(toks) && toks[j].kind == tokIdent && toks[j].text == "async" {
		j++
	}
	if j < len(toks) && toks[j].kind == tokIdent && toks[j].text == "function" {
		j++
		if j < len(toks) && toks[j].kind == tokIdent {
			j++
		}
	}
	if !next(toks, j, "(") {
		return -1
	}
	close := closeParen(toks, j)
	body := skBodyOpen(toks, close, len(toks))
	if body < 0 || (!next(toks, body-1, ">") && !next(toks, body-1, ")") && toks[body-1].kind != tokIdent) {
		return -1
	}
	u := f.newUnit(nil, name, j, close, body)
	u.start = toks[i].line
	f.addFunc(u)
	return closeParen(toks, body)
}

// readClass reads one class declaration at toks[i] and returns its closing brace index.
func (f *skFile) readClass(m *SKModel, i int) int {
	toks := f.toks
	if i+1 >= len(toks) || toks[i+1].kind != tokIdent {
		return -1
	}
	c := &skClass{name: toks[i+1].text, start: toks[i].line, props: map[string]SKBinding{}, roots: map[string]bool{"page": true},
		methods: map[string]*skUnit{}, file: f}
	j := i + 2
	open := -1
	for ; j < len(toks); j++ {
		if next(toks, j, "{") {
			open = j
			break
		}
		if toks[j].kind == tokIdent && toks[j].text == "extends" && j+1 < len(toks) && toks[j+1].kind == tokIdent {
			c.extends = toks[j+1].text
		}
	}
	if open < 0 {
		return -1
	}
	close := closeParen(toks, open)
	c.end = toks[close].line
	if f.cypress {
		c.form = FormCypressClass
	} else {
		c.form = FormPlaywrightClass
	}
	f.readMembers(c, open, close)
	if !f.pageLike(c, open, close) {
		// A class that holds no page, locator or base page (a reporter, a data helper) is not a
		// page object; it is neither listed nor an unknown.
		return close
	}
	if len(f.classes) > 0 {
		m.Unknowns = append(m.Unknowns, SKUnknown{Path: f.path, Line: c.start, Form: c.form, Construct: "class " + c.name,
			Knowledge: "page-objects", Reason: "second-class-in-file"})
		return close
	}
	f.classes[c.name] = c
	f.corder = append(f.corder, c.name)
	return close
}

// pageLike reports whether a class can model a page: it extends a class, binds a locator, holds
// a page property, takes a page argument, or (Cypress) uses cy.
func (f *skFile) pageLike(c *skClass, open, close int) bool {
	if c.extends != "" || len(c.props) > 0 || len(c.roots) > 1 || (f.cypress && f.usesCy(open, close)) {
		return true
	}
	for _, u := range c.methods {
		if len(u.pageArgs) > 0 {
			return true
		}
	}
	toks := f.toks
	for k := open; k+2 < close; k++ {
		if toks[k].kind == tokIdent && toks[k].text == "this" && next(toks, k+1, ".") && toks[k+2].text == "page" {
			return true
		}
	}
	return false
}

// readMembers reads one class body: methods, getters and arrow fields become methods; fields,
// getters and constructor assignments that hold a locator become bindings (SKH-V0-011).
func (f *skFile) readMembers(c *skClass, open, close int) {
	toks := f.toks
	var ctor *skUnit
	for j := open + 1; j < close; {
		if next(toks, j, ";", ",") {
			j++
			continue
		}
		if next(toks, j, "@") { // decorator
			j++
			if j < close && toks[j].kind == tokIdent {
				j++
			}
			if next(toks, j, "(") {
				j = closeParen(toks, j) + 1
			}
			continue
		}
		for j+1 < close && toks[j].kind == tokIdent && skModifiers[toks[j].text] && toks[j+1].kind == tokIdent {
			j++
		}
		if j >= close || toks[j].kind != tokIdent {
			j++
			continue
		}
		name := toks[j].text
		getter := false
		if (name == "get" || name == "set") && j+1 < close && toks[j+1].kind == tokIdent {
			getter = name == "get"
			j++
			name = toks[j].text
		}
		k := j + 1
		if next(toks, k, "<") { // generic parameters
			for k < close && !next(toks, k, "(") {
				k++
			}
		}
		if next(toks, k, "(") {
			pclose := closeParen(toks, k)
			body := skBodyOpen(toks, pclose, close)
			if body < 0 {
				j = pclose + 1
				continue
			}
			end := closeParen(toks, body)
			u := f.newUnit(c, name, k, pclose, body)
			u.start = toks[j].line
			switch {
			case name == "constructor":
				ctor = u
				for q := k + 1; q < pclose; q++ {
					if toks[q].kind == tokIdent && skModifiers[toks[q].text] && q+1 < pclose && toks[q+1].kind == tokIdent && u.pageArgs[toks[q+1].text] {
						c.roots[toks[q+1].text] = true
					}
				}
			case getter:
				if r := f.findReturn(body+1, end); r >= 0 {
					f.bindLocator(c, name, r, skStmtEnd(toks, r, end), u.pageArgs, toks[j].line)
				}
			default:
				c.addMethod(u)
			}
			j = end + 1
			continue
		}
		// A field: NAME[?!][: Type][= init]
		for next(toks, k, "?", "!") {
			k++
		}
		end := skStmtEnd(toks, k, close)
		if next(toks, k, ":") {
			typ := ""
			if k+1 < end && toks[k+1].kind == tokIdent {
				typ = toks[k+1].text
			}
			if typ == "Page" {
				c.roots[name] = true
			}
			for k < end && !(next(toks, k, "=") && !next(toks, k+1, ">")) {
				k++
			}
		}
		if next(toks, k, "=") && k+1 < end {
			init := k + 1
			if u := f.arrowMember(c, name, init, close); u != nil {
				c.addMethod(u)
				j = closeParen(toks, u.from-1) + 1
				continue
			}
			f.bindLocator(c, name, init, end, nil, toks[j].line)
		}
		j = end
		if j < close && next(toks, j, ";") {
			j++
		}
		if j == k {
			j++
		}
	}
	if ctor != nil {
		f.ctorBindings(c, ctor)
	}
}

func (c *skClass) addMethod(u *skUnit) {
	if c.methods[u.name] == nil {
		c.order = append(c.order, u.name)
	}
	c.methods[u.name] = u
}

// arrowMember reads `NAME = [async] (params) => { body }` as a method.
func (f *skFile) arrowMember(c *skClass, name string, i, limit int) *skUnit {
	toks := f.toks
	if i < limit && toks[i].kind == tokIdent && toks[i].text == "async" {
		i++
	}
	if !next(toks, i, "(") {
		return nil
	}
	pclose := closeParen(toks, i)
	if !next(toks, pclose+1, "=") && !next(toks, pclose+1, ":") {
		return nil
	}
	body := skBodyOpen(toks, pclose, limit)
	if body < 0 || !next(toks, body-1, ">") {
		return nil
	}
	return f.newUnit(c, name, i, pclose, body)
}

// ctorBindings reads `this.X = <locator>` and `this.X = page` in a constructor body.
func (f *skFile) ctorBindings(c *skClass, u *skUnit) {
	toks := f.toks
	for q := u.from; q+3 < u.to; q++ {
		if toks[q].kind != tokIdent || toks[q].text != "this" || !next(toks, q+1, ".") || toks[q+2].kind != tokIdent ||
			!next(toks, q+3, "=") || next(toks, q+4, "=", ">") {
			continue
		}
		name := toks[q+2].text
		end := skStmtEnd(toks, q+4, u.to)
		if end == q+5 && toks[q+4].kind == tokIdent && u.pageArgs[toks[q+4].text] {
			c.roots[name] = true
		} else {
			f.bindLocator(c, name, q+4, end, u.pageArgs, toks[q].line)
		}
		q = end
	}
}

func (f *skFile) findReturn(from, to int) int {
	for q := from; q < to; q++ {
		if f.toks[q].kind == tokIdent && f.toks[q].text == "return" {
			return q + 1
		}
	}
	return -1
}

// bindLocator records a property binding when toks[a:b] is a locator expression.
func (f *skFile) bindLocator(c *skClass, name string, a, b int, pageArgs map[string]bool, line int) {
	x := &skCtx{f: f, c: c, pageArgs: pageArgs, locals: map[string]skLoc{}, inst: map[string]string{}, lits: map[string]string{}}
	loc, ok := x.locatorExpr(a, b)
	if !ok || loc.slot != "" {
		return
	}
	c.props[name] = SKBinding{Name: name, Locator: loc.sel, Anchor: spanOf(f.entry, f.data, line, f.toks[b-1].line)}
}

// readTests finds test(...) and it(...) calls outside classes and functions.
func (f *skFile) readTests(busy []bool) {
	toks := f.toks
	for i := 0; i+2 < len(toks); i++ {
		if busy[i] || toks[i].kind != tokIdent || !skTestCalls[toks[i].text] || (i > 0 && next(toks, i-1, ".")) {
			continue
		}
		j := i + 1
		if next(toks, j, ".") && j+1 < len(toks) && toks[j+1].kind == tokIdent && (toks[j+1].text == "only" || toks[j+1].text == "fixme" || toks[j+1].text == "skip") {
			j += 2
		}
		if !next(toks, j, "(") || j+1 >= len(toks) || (toks[j+1].kind != tokString && toks[j+1].kind != tokTemplate) {
			continue
		}
		close := closeParen(toks, j)
		title := toks[j+1].text
		// The callback is the last function argument: find `(params) =>` or `function (params)`.
		fn := -1
		for k := j + 2; k < close; k++ {
			if next(toks, k, "(") {
				pc := closeParen(toks, k)
				if next(toks, pc+1, "=") && next(toks, pc+2, ">") && next(toks, pc+3, "{") {
					fn = k
				}
				if k > 0 && toks[k-1].kind == tokIdent && toks[k-1].text == "function" && next(toks, pc+1, "{") {
					fn = k
				}
				if fn >= 0 {
					break
				}
				k = pc
			}
		}
		if fn < 0 {
			continue
		}
		pc := closeParen(toks, fn)
		body := skBodyOpen(toks, pc, close)
		if body < 0 {
			continue
		}
		u := f.newUnit(nil, toks[i].text, fn, pc, body)
		u.title, u.start, u.end = title, toks[i].line, toks[close].line
		if f.cypress {
			u.pageArgs["cy"] = true
		}
		f.tests = append(f.tests, u)
		i = close
	}
}

// skStmtEnd returns the index just past the statement that starts at a: a ';' at depth 0, a line
// break that cannot continue the expression, or the enclosing block's close.
func skStmtEnd(toks []token, a, limit int) int {
	depth := 0
	for k := a; k < limit; k++ {
		t := toks[k]
		if k > a && depth == 0 && t.line > toks[k-1].line && !skContinues(toks[k-1], t) {
			return k
		}
		if t.kind != tokPunct {
			continue
		}
		switch t.text {
		case "{", "(", "[":
			depth++
		case "}", ")", "]":
			if depth == 0 {
				return k
			}
			depth--
		case ";":
			if depth == 0 {
				return k
			}
		}
	}
	return limit
}

func skContinues(prev, cur token) bool {
	if prev.kind == tokPunct && strings.Contains(".,([{=+-*/%&|?:<>!", prev.text) {
		return true
	}
	if cur.kind == tokPunct && strings.Contains(".?)],=+-*/%&|:<>", cur.text) {
		return true
	}
	return cur.kind == tokIdent && (cur.text == "as" || cur.text == "satisfies" || cur.text == "instanceof" ||
		cur.text == "else" || cur.text == "catch" || cur.text == "finally")
}

// skLoc is a locator value: exact, or read from a parameter slot.
type skLoc struct {
	sel  Selector
	slot string
}

// skCtx is the reading context of one body.
type skCtx struct {
	m        *SKModel
	f        *skFile
	c        *skClass
	u        *skUnit
	pageArgs map[string]bool
	params   map[string]bool
	locals   map[string]skLoc
	inst     map[string]string
	lits     map[string]string
	steps    []SKStep
}

type skSeg struct {
	name        string
	call        bool
	open, close int
}

// chain reads `root(.name | ?.name | (args) | ! | [i])*` from toks[a]; ok is false unless it spans
// exactly toks[a:b].
func (x *skCtx) chain(a, b int) (string, bool, skSeg, []skSeg, bool) {
	toks := x.f.toks
	if a >= b || toks[a].kind != tokIdent {
		return "", false, skSeg{}, nil, false
	}
	root := toks[a].text
	var rootCall skSeg
	called := false
	k := a + 1
	if next(toks, k, "(") {
		c := closeParen(toks, k)
		rootCall, called = skSeg{name: root, call: true, open: k, close: c}, true
		k = c + 1
	}
	segs := []skSeg{}
	for k < b {
		switch {
		case next(toks, k, "!") && !next(toks, k+1, "="):
			k++
		case next(toks, k, "?") && next(toks, k+1, "."):
			k++
		case next(toks, k, ".") && k+1 < b && toks[k+1].kind == tokIdent:
			s := skSeg{name: toks[k+1].text}
			k += 2
			if next(toks, k, "(") {
				s.call, s.open, s.close = true, k, closeParen(toks, k)
				k = s.close + 1
			}
			segs = append(segs, s)
		case next(toks, k, "["):
			k = closeParen(toks, k) + 1
			segs = append(segs, skSeg{name: "[]"})
		default:
			return root, called, rootCall, segs, false
		}
	}
	return root, called, rootCall, segs, true
}

// args splits the arguments of the call whose parens are at open and close.
func (x *skCtx) args(open, close int) [][2]int {
	out := [][2]int{}
	for k := open + 1; k < close; {
		end := skipValue(x.f.toks, k)
		if end > close {
			end = close
		}
		out = append(out, [2]int{k, end})
		k = end + 1
	}
	return out
}

// arg reads argument n of a call as a literal, a parameter slot, or dynamic.
func (x *skCtx) arg(s skSeg, n int) (SKArg, bool) {
	as := x.args(s.open, s.close)
	if n >= len(as) {
		return SKArg{}, false
	}
	return x.value(as[n][0], as[n][1]), true
}

func (x *skCtx) value(a, b int) SKArg {
	toks := x.f.toks
	if b-a != 1 {
		switch {
		case b-a == 2 && next(toks, a, "-") && toks[a+1].kind == tokNumber:
			return SKArg{Literal: "-" + toks[a+1].text}
		case b-a == 2 && next(toks, a, "[") && next(toks, a+1, "]"):
			return SKArg{Literal: "[]"}
		case b-a == 2 && next(toks, a, "{") && next(toks, a+1, "}"):
			return SKArg{Literal: "{}"}
		}
		return SKArg{Dynamic: true}
	}
	t := toks[a]
	switch {
	case t.kind == tokTemplate && t.subst:
		return x.template(t)
	case literal(t):
		return SKArg{Literal: t.text}
	case t.regex != "":
		return SKArg{Literal: t.regex}
	case t.kind == tokNumber:
		return SKArg{Literal: t.text}
	case t.kind == tokIdent && (t.text == "true" || t.text == "false"):
		return SKArg{Literal: t.text}
	case t.kind == tokIdent && x.params[t.text]:
		return SKArg{Slot: t.text}
	case t.kind == tokIdent && x.lits[t.text] != "":
		return SKArg{Literal: x.lits[t.text]}
	}
	return SKArg{Dynamic: true}
}

// template reads a template literal whose every ${} substitution is one parameter or literal
// constant; anything else is a dynamic value.
func (x *skCtx) template(t token) SKArg {
	if t.inexact || t.unsure {
		return SKArg{Dynamic: true}
	}
	var subs []string
	for rest := t.code; ; {
		i := strings.Index(rest, "${")
		if i < 0 {
			break
		}
		j := strings.IndexByte(rest[i:], '}')
		if j < 0 {
			return SKArg{Dynamic: true}
		}
		subs = append(subs, strings.TrimSpace(rest[i+2:i+j]))
		rest = rest[i+j+1:]
	}
	parts := strings.Split(t.text, substMark)
	if len(parts) != len(subs)+1 {
		return SKArg{Dynamic: true}
	}
	out := SKArg{Literal: parts[0]}
	for k, name := range subs {
		switch {
		case x.params[name]:
			out.Literal += "${" + name + "}"
			if !slices.Contains(out.Template, name) {
				out.Template = append(out.Template, name)
			}
		case x.lits[name] != "":
			out.Literal += x.lits[name]
		default:
			return SKArg{Dynamic: true}
		}
		out.Literal += parts[k+1]
	}
	if len(out.Template) == 0 {
		out.Template = nil
	}
	return out
}

// produce builds the locator a producer segment names.
func (x *skCtx) produce(s skSeg) (skLoc, bool) {
	k, ok := selectorKinds[s.name]
	if !ok {
		k, ok = skCypressProducers[s.name]
	}
	if !ok || !s.call {
		return skLoc{}, false
	}
	toks := x.f.toks
	line := toks[s.open].line
	a, has := x.arg(s, 0)
	if !has {
		return skLoc{sel: newSelector(k.kind, "", "", strengthUnknown, line)}, true
	}
	kind, strength := k.kind, k.strength
	value := a.Literal
	if (kind == "css") && a.Literal != "" {
		if mm := testIDAttribute.FindStringSubmatch(a.Literal); mm != nil {
			kind, strength, value = "test-id", strengthStrong, mm[1]
		}
	}
	name := ""
	if kind == "role" {
		if as := x.args(s.open, s.close); len(as) > 1 && next(toks, as[1][0], "{") {
			name, _ = roleName(toks, as[1][0], func(t token) (string, bool) { return t.text, literal(t) })
		}
	}
	if a.Slot != "" {
		return skLoc{sel: newSelector(kind, "", name, strengthUnknown, line), slot: a.Slot}, true
	}
	if a.Dynamic || len(a.Template) > 0 {
		return skLoc{sel: newSelector(kind, "", "", strengthUnknown, line)}, true
	}
	return skLoc{sel: newSelector(kind, value, name, strength, line)}, true
}

// locatorExpr evaluates toks[a:b] as an expression naming one locator, with no action.
func (x *skCtx) locatorExpr(a, b int) (skLoc, bool) {
	if a < b && x.f.toks[a].kind == tokIdent && x.f.toks[a].text == "await" {
		a++
	}
	root, called, _, segs, ok := x.chain(a, b)
	if !ok || called {
		return skLoc{}, false
	}
	state, loc, rest := x.start(root, segs)
	if state == "" {
		return skLoc{}, false
	}
	for _, s := range rest {
		if l, ok := x.produce(s); ok {
			loc, state = l, "locator"
			continue
		}
		if skRefiners[s.name] || s.name == "[]" {
			continue
		}
		return skLoc{}, false
	}
	return loc, state == "locator"
}

// start resolves a chain's root: the page, a bound locator, or nothing.
func (x *skCtx) start(root string, segs []skSeg) (string, skLoc, []skSeg) {
	switch {
	case root == "this" && x.c != nil && len(segs) > 0 && !segs[0].call:
		if b, ok := x.c.props[segs[0].name]; ok {
			return "locator", skLoc{sel: b.Locator}, segs[1:]
		}
		if x.c.roots[segs[0].name] {
			return "page", skLoc{}, segs[1:]
		}
	case x.pageArgs[root] || root == "cy":
		return "page", skLoc{}, segs
	case x.locals[root].sel.ID != "":
		return "locator", x.locals[root], segs
	}
	return "", skLoc{}, segs
}

func (x *skCtx) anchor(a, b int) Anchor {
	toks := x.f.toks
	if b <= a {
		b = a + 1
	}
	return spanOf(x.f.entry, x.f.data, toks[a].line, toks[b-1].line)
}

func (x *skCtx) add(s SKStep) { x.steps = append(x.steps, s) }

func (x *skCtx) opaque(reason string, call string, a, b int) {
	x.add(SKStep{Kind: StepOpaque, Reason: reason, Call: call, Anchor: x.anchor(a, b)})
}

// valued builds a step that carries one argument value.
func (x *skCtx) valued(kind string, loc *skLoc, a SKArg, at Anchor) SKStep {
	s := SKStep{Kind: kind, Anchor: at}
	if loc != nil {
		sel := loc.sel
		s.Locator, s.LocatorSlot = &sel, loc.slot
	}
	switch {
	case a.Dynamic:
		return SKStep{Kind: StepOpaque, Reason: OpaqueDynamicValue, Call: kind, Anchor: at}
	case a.Slot != "":
		s.Slots = []string{a.Slot}
	default:
		v, screened := screenValue(a.Literal, s.Locator)
		s.Screened, s.Slots = screened, a.Template
		switch kind {
		case StepVisit:
			s.Target = v
		case StepAssert:
			s.Expected = v
		default:
			s.Value = v
		}
	}
	return s
}

// tsSteps reads a unit's body into ordered steps (SKH-V0-020).
func (m *SKModel) tsSteps(u *skUnit) []SKStep {
	x := &skCtx{m: m, f: u.file, c: u.class, u: u, pageArgs: map[string]bool{}, params: map[string]bool{},
		locals: map[string]skLoc{}, inst: map[string]string{}, lits: map[string]string{}}
	for n, v := range u.file.consts {
		x.lits[n] = v
	}
	for p := range u.pageArgs {
		x.pageArgs[p] = true
	}
	for _, p := range u.params {
		if p != "" && p != "{}" && !u.pageArgs[p] {
			x.params[p] = true
		}
	}
	x.block(u.from, u.to)
	return x.steps
}

// block reads the statements of toks[a:b].
func (x *skCtx) block(a, b int) {
	toks := x.f.toks
	for k := a; k < b; {
		if next(toks, k, ";") || next(toks, k, "}") {
			k++
			continue
		}
		end := skStmtEnd(toks, k, b)
		if end <= k {
			end = k + 1
		}
		x.statement(k, end)
		k = end
	}
}

func (x *skCtx) hasAction(a, b int) bool {
	toks := x.f.toks
	for k := a; k < b; k++ {
		if toks[k].kind != tokIdent {
			continue
		}
		if toks[k].text == "expect" || (next(toks, k-1, ".") && next(toks, k+1, "(") && skActionKinds[toks[k].text] != "") {
			return true
		}
	}
	return false
}

func (x *skCtx) statement(a, b int) {
	toks := x.f.toks
	t := toks[a]
	if t.kind == tokIdent && skControl[t.text] {
		if x.hasAction(a, b) {
			x.opaque(OpaqueParseUnsupported, "", a, b)
		}
		return
	}
	if t.kind == tokIdent && (t.text == "function" || (t.text == "async" && a+1 < b && toks[a+1].text == "function")) {
		return // a nested helper declaration is not a step; calls to it inline (SKH-V0-012)
	}
	awaited := false
	for a < b && toks[a].kind == tokIdent && (toks[a].text == "await" || toks[a].text == "return" || toks[a].text == "void") {
		awaited = awaited || toks[a].text == "await"
		a++
	}
	if a >= b {
		return
	}
	if toks[a].kind == tokIdent && (toks[a].text == "const" || toks[a].text == "let" || toks[a].text == "var") {
		x.declaration(a, b)
		return
	}
	x.expression(a, b, awaited)
}

func (x *skCtx) declaration(a, b int) {
	toks := x.f.toks
	if a+2 < b && toks[a+1].kind == tokIdent && next(toks, a+2, "=") {
		name, init := toks[a+1].text, a+3
		if init < b && toks[init].kind == tokIdent && toks[init].text == "new" && init+1 < b && toks[init+1].kind == tokIdent {
			x.inst[name] = toks[init+1].text
			return
		}
		if init+1 == b && literal(toks[init]) {
			x.lits[name] = toks[init].text
			return
		}
		if loc, ok := x.locatorExpr(init, b); ok {
			x.locals[name] = loc
			return
		}
		a = init
	} else {
		for a < b && !(next(toks, a, "=") && !next(toks, a+1, ">")) {
			a++
		}
		a++
	}
	awaited := false
	for a < b && toks[a].kind == tokIdent && toks[a].text == "await" {
		awaited = true
		a++
	}
	if a < b {
		x.expression(a, b, awaited)
	}
}

func (x *skCtx) expression(a, b int, awaited bool) {
	toks := x.f.toks
	if toks[a].kind == tokIdent && toks[a].text == "expect" {
		x.expect(a, b)
		return
	}
	root, called, rootCall, segs, ok := x.chain(a, b)
	if !ok {
		if awaited || x.hasAction(a, b) {
			x.opaque(OpaqueParseUnsupported, "", a, b)
		}
		return
	}
	at := x.anchor(a, b)
	// test.step('name', async () => { ... }) reads its body in place.
	if root == "test" && len(segs) == 1 && segs[0].name == "step" && segs[0].call {
		for k := segs[0].open; k < segs[0].close; k++ {
			if next(toks, k, "=") && next(toks, k+1, ">") && next(toks, k+2, "{") {
				x.block(k+3, closeParen(toks, k+2))
				return
			}
		}
		x.opaque(OpaqueParseUnsupported, "test.step", a, b)
		return
	}
	if called {
		x.callStep(root, rootCall, at, a, b)
		return
	}
	if root == "this" && x.c != nil && len(segs) > 0 && segs[0].call {
		x.add(SKStep{Kind: skCall, Call: "this." + segs[0].name, Args: x.callArgs(segs[0]), Anchor: at})
		return
	}
	if cls, ok := x.inst[root]; ok && len(segs) > 0 {
		if segs[0].call {
			x.add(SKStep{Kind: skCall, Call: cls + "." + segs[0].name, Args: x.callArgs(segs[0]), Anchor: at})
			return
		}
		if c, _ := x.m.findClass(x.f, cls); c != nil {
			if bnd, ok := c.props[segs[0].name]; ok {
				x.actions("locator", skLoc{sel: bnd.Locator}, segs[1:], at, a, b)
				return
			}
		}
		x.opaque(OpaqueUnresolvedCall, cls+"."+segs[0].name, a, b)
		return
	}
	state, loc, rest := x.start(root, segs)
	if state == "" {
		if skQuietRoots[root] && !x.hasAction(a, b) {
			return
		}
		if root == "request" || root == "fetch" {
			x.u.setup = append(x.u.setup, root)
			return
		}
		if awaited || x.hasAction(a, b) {
			name := root
			if len(segs) > 0 {
				name += "." + segs[0].name
			}
			x.opaque(OpaqueUnresolvedCall, name, a, b)
		}
		return
	}
	x.actions(state, loc, rest, at, a, b)
}

func (x *skCtx) callArgs(s skSeg) []SKArg {
	out := []SKArg{}
	for _, r := range x.args(s.open, s.close) {
		if r[1]-r[0] == 1 && x.f.toks[r[0]].kind == tokIdent && x.pageArgs[x.f.toks[r[0]].text] {
			out = append(out, SKArg{Slot: "page"})
			continue
		}
		out = append(out, x.value(r[0], r[1]))
	}
	return out
}

// callStep records a direct function call: a page function is a call step inlined later; a
// non-page helper is setup knowledge; an unresolved one is opaque.
func (x *skCtx) callStep(root string, s skSeg, at Anchor, a, b int) {
	target, _, _ := x.m.resolveCall(x.u, root)
	switch {
	case target == nil:
		if skQuietRoots[root] || root == "require" {
			return
		}
		x.add(SKStep{Kind: StepOpaque, Reason: OpaqueUnresolvedCall, Call: root, Anchor: at})
	case target.root:
		args := x.callArgs(s)
		// Positional arguments bind to the callee's parameters; the page argument stays in place.
		x.add(SKStep{Kind: skCall, Call: root, Args: args, Anchor: at})
	default:
		x.u.setup = append(x.u.setup, root)
	}
}

// actions applies a chain's remaining segments from a page or locator state.
func (x *skCtx) actions(state string, loc skLoc, segs []skSeg, at Anchor, a, b int) {
	for _, s := range segs {
		if l, ok := x.produce(s); ok && (state == "page" || state == "locator") {
			loc, state = l, "locator"
			continue
		}
		if skRefiners[s.name] || s.name == "[]" {
			continue
		}
		if skReadMethods[s.name] {
			return
		}
		if state == "page" && skSetupPageMethods[s.name] {
			x.u.setup = append(x.u.setup, "page."+s.name)
			return
		}
		if state == "page" && s.name == skKeyboard && !s.call {
			state = skKeyboard
			continue
		}
		if state == skKeyboard {
			x.keyboard(s, at)
			return
		}
		if skOpaqueMethods[s.name] {
			x.add(SKStep{Kind: StepOpaque, Reason: OpaqueParseUnsupported, Call: s.name, Anchor: at})
			return
		}
		kind := skActionKinds[s.name]
		if kind == "" || !s.call {
			if state == "page" && x.pageArgs["cy"] {
				x.add(SKStep{Kind: StepOpaque, Reason: OpaqueUnresolvedCall, Call: "cy." + s.name, Anchor: at})
			} else {
				x.add(SKStep{Kind: StepOpaque, Reason: OpaqueParseUnsupported, Call: s.name, Anchor: at})
			}
			return
		}
		x.action(kind, state, &loc, s, at)
		if kind == StepVisit && state == "page" {
			continue
		}
	}
}

// keyboard reads page.keyboard.press(key) as a press and page.keyboard.type|insertText(text) as a
// fill, both typed to the keyboard target; every other keyboard call stays opaque (SKH-V0-024).
func (x *skCtx) keyboard(s skSeg, at Anchor) {
	kind := map[string]string{"press": StepPress, "type": StepFill, "insertText": StepFill}[s.name]
	a, has := x.arg(s, 0)
	if kind == "" || !s.call || !has {
		x.add(SKStep{Kind: StepOpaque, Reason: OpaqueParseUnsupported, Call: skKeyboard + "." + s.name, Anchor: at})
		return
	}
	st := x.valued(kind, nil, a, at)
	if st.Kind != StepOpaque {
		st.Target = skKeyboard
	}
	x.add(st)
}

// action emits one mapped action.
func (x *skCtx) action(kind, state string, loc *skLoc, s skSeg, at Anchor) {
	n := 0
	if state == "page" {
		switch {
		case kind == StepVisit || s.name == "waitForURL":
			a, _ := x.arg(s, 0)
			if s.name == "waitForURL" {
				x.add(x.valued(StepWaitFor, nil, a, at))
				return
			}
			x.add(x.valued(StepVisit, nil, a, at))
			return
		case s.name == "waitForLoadState" || s.name == "wait":
			a, has := x.arg(s, 0)
			if !has {
				x.add(SKStep{Kind: StepWaitFor, Anchor: at})
				return
			}
			x.add(x.valued(StepWaitFor, nil, a, at))
			return
		case skLegacySelectorFirst[s.name] || s.name == "waitForSelector":
			// page.click('css') and friends name the element by their first argument.
			a, _ := x.arg(s, 0)
			sel := skLoc{sel: newSelector("css", a.Literal, "", skKindStrength("css", a.Literal), x.f.toks[s.open].line), slot: a.Slot}
			if mm := testIDAttribute.FindStringSubmatch(a.Literal); mm != nil {
				sel.sel = newSelector("test-id", mm[1], "", strengthStrong, x.f.toks[s.open].line)
			}
			if a.Dynamic || len(a.Template) > 0 {
				x.add(SKStep{Kind: StepOpaque, Reason: OpaqueDynamicValue, Call: kind, Anchor: at})
				return
			}
			loc, n = &sel, 1
		default:
			x.add(SKStep{Kind: StepOpaque, Reason: OpaqueParseUnsupported, Call: s.name, Anchor: at})
			return
		}
	}
	switch kind {
	case StepClick, StepCheck, StepWaitFor, StepFocus, StepHover:
		st := SKStep{Kind: kind, Anchor: at}
		sel := loc.sel
		st.Locator, st.LocatorSlot = &sel, loc.slot
		if s.name == "uncheck" || (s.name == "setChecked" && x.argIs(s, n, "false")) {
			st.Value = "false"
		}
		x.add(st)
	case StepAssert:
		a, has := x.arg(s, n)
		st := SKStep{Kind: StepAssert, Anchor: at}
		if has && a.Literal != "" {
			st.Matcher = a.Literal
		} else if has {
			x.add(SKStep{Kind: StepOpaque, Reason: OpaqueDynamicValue, Call: StepAssert, Anchor: at})
			return
		}
		sel := loc.sel
		st.Locator, st.LocatorSlot = &sel, loc.slot
		if e, ok := x.arg(s, n+1); ok {
			x.add(x.withExpected(st, e))
			return
		}
		x.add(st)
	default: // fill, select, upload, press
		a, has := x.arg(s, n)
		if !has {
			a = SKArg{Literal: ""}
		}
		x.add(x.valued(kind, loc, a, at))
	}
}

func (x *skCtx) argIs(s skSeg, n int, lit string) bool {
	as := x.args(s.open, s.close)
	return n < len(as) && as[n][1]-as[n][0] == 1 && x.f.toks[as[n][0]].text == lit
}

func (x *skCtx) withExpected(st SKStep, e SKArg) SKStep {
	switch {
	case e.Dynamic:
		return SKStep{Kind: StepOpaque, Reason: OpaqueDynamicValue, Call: st.Kind, Anchor: st.Anchor}
	case e.Slot != "":
		st.Slots = []string{e.Slot}
	default:
		v, screened := screenValue(e.Literal, st.Locator)
		st.Expected, st.Screened, st.Slots = v, screened, e.Template
	}
	return st
}

// expect reads `expect[.soft](X).[not.]matcher(arg)`.
func (x *skCtx) expect(a, b int) {
	toks := x.f.toks
	at := x.anchor(a, b)
	k := a + 1
	if next(toks, k, ".") && k+1 < b && toks[k+1].kind == tokIdent {
		if toks[k+1].text != "soft" {
			x.add(SKStep{Kind: StepOpaque, Reason: OpaqueParseUnsupported, Call: "expect." + toks[k+1].text, Anchor: at})
			return
		}
		k += 2
	}
	if !next(toks, k, "(") {
		x.add(SKStep{Kind: StepOpaque, Reason: OpaqueParseUnsupported, Call: "expect", Anchor: at})
		return
	}
	subjClose := closeParen(toks, k)
	q := subjClose + 1
	matcher := ""
	for q+1 < b && next(toks, q, ".") && toks[q+1].kind == tokIdent {
		if matcher != "" {
			matcher += "."
		}
		matcher += toks[q+1].text
		q += 2
		if next(toks, q, "(") {
			break
		}
	}
	if matcher == "" || !next(toks, q, "(") {
		x.add(SKStep{Kind: StepOpaque, Reason: OpaqueParseUnsupported, Call: "expect", Anchor: at})
		return
	}
	st := SKStep{Kind: StepAssert, Matcher: matcher, Anchor: at}
	// The subject is the first argument; a second is the failure message.
	if subj := x.args(k, subjClose); len(subj) > 0 {
		sa, sb := subj[0][0], subj[0][1]
		if loc, ok := x.locatorExpr(sa, sb); ok {
			sel := loc.sel
			st.Locator, st.LocatorSlot = &sel, loc.slot
		} else if sb-sa == 1 && toks[sa].kind == tokIdent && x.pageArgs[toks[sa].text] {
			st.Target = "page"
		} else {
			st.Target = "value" // a test-local value, not a page element
		}
	}
	mc := closeParen(toks, q)
	as := x.args(q, mc)
	switch len(as) {
	case 0:
		x.add(st)
	default:
		x.add(x.withExpected(st, x.value(as[0][0], as[0][1])))
	}
}
