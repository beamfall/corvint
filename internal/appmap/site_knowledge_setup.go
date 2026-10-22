package appmap

import (
	"path"
	"sort"
	"strconv"
	"strings"
)

// Stage 3 setup records (SKH-V0-030, SKH-V0-031, SKH-V0-033, SKH-V0-034), read from the
// TypeScript and JavaScript sources stage 1 already tokenised: test.extend fixtures and their
// chains, test hooks, storageState reads and writes, route mocks, API seeding calls and cy.task.
// Every other SKH-V0-030 source (FactoryBot, fixture files, seeds, setup endpoints, setup projects)
// is not read yet and is named in the report's not_read list, never guessed.

// SetupSchema is the profile of `flows appmap setup`.
const SetupSchema = "site-knowledge-setup/0"

// Setup record kinds.
const (
	SetupFixture      = "fixture"
	SetupHook         = "hook"
	SetupStorageState = "storage-state"
	SetupRouteMock    = "route-mock"
	SetupAPISeed      = "api-seed"
	SetupTask         = "task"
)

// SetupNotRead names the SKH-V0-030 sources this profile does not read.
var SetupNotRead = []string{"factorybot", "fixture-files", "seeds", "setup-endpoints", "setup-projects"}

// skBuiltinFixtures are the Playwright fixtures every test object provides.
var skBuiltinFixtures = wordSet("page context browser browserName request baseURL headless channel isMobile " +
	"javaScriptEnabled locale timezoneId viewport storageState colorScheme contextOptions launchOptions playwright " +
	"video trace screenshot acceptDownloads hasTouch geolocation permissions offline httpCredentials userAgent " +
	"extraHTTPHeaders ignoreHTTPSErrors bypassCSP deviceScaleFactor proxy serviceWorkers testIdAttribute actionTimeout navigationTimeout")

var (
	skHookNames   = wordSet("beforeEach beforeAll afterEach afterAll before after")
	skSeedMethods = wordSet("get post put patch delete head fetch")
)

// SKSetup is one typed setup record. Target, Method and Blob are evidence read from committed
// source; a target that is not a literal is UNKNOWN with reason dynamic-value. storageState
// contents are never read or stored: a record keeps the path and, when the file is committed in
// the declared sources, its blob (SKH-V0-033).
type SKSetup struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`
	Name     string   `json:"name,omitempty"`
	Path     string   `json:"path"`
	Status   string   `json:"status"`
	Reason   string   `json:"reason,omitempty"`
	Target   string   `json:"target,omitempty"`
	Method   string   `json:"method,omitempty"`
	Params   []string `json:"params,omitempty"`
	Produces []string `json:"produces,omitempty"`
	// Test and Base name the fixture's extension variable and the test object it extends.
	Test  string `json:"test,omitempty"`
	Base  string `json:"base,omitempty"`
	Scope string `json:"scope,omitempty"`
	// Option and Auto copy the fixture's literal options.
	Option bool `json:"option,omitempty"`
	Auto   bool `json:"auto,omitempty"`
	// Cleanup is after-use when a fixture runs code after use(), after-each or after-all for a hook.
	Cleanup string `json:"cleanup,omitempty"`
	// Mocked marks a route mock: no plan or proof may treat it as real backend behaviour (SKH-V0-034).
	Mocked bool   `json:"mocked,omitempty"`
	Mode   string `json:"mode,omitempty"`
	Blob   string `json:"blob,omitempty"`
	// Within is the fixture, hook, helper or test whose body holds the record.
	Within string `json:"within,omitempty"`
	// Requires lists the fixture records a fixture's parameters resolve to along the extend chain;
	// Unresolved names parameters that are neither built in nor resolved.
	Requires   []string `json:"requires,omitempty"`
	Unresolved []string `json:"unresolved,omitempty"`
	Authority  string   `json:"authority"`
	Anchor     Anchor   `json:"anchor"`
}

// SKSetupUse links one test to the setup records that arrange it (SKH-V0-031): the fixtures it
// requests and their chains, hooks and storageState in its enclosing blocks, and records inside
// its body or the helpers it calls.
type SKSetupUse struct {
	Test       string   `json:"test"`
	Title      string   `json:"title"`
	Uses       []string `json:"uses"`
	Unresolved []string `json:"unresolved,omitempty"`
}

// skExtension is one `const NAME = BASE.extend({...})` declaration.
type skExtension struct {
	name, base string
	fixtures   map[string]int // fixture name -> setup index
}

type skSetupRange struct {
	from, to int
	id       string
}

type skSetupFile struct {
	f          *skFile
	extensions map[string]*skExtension
	records    []int // setup indexes declared in this file
	at         map[int]int
	bodies     []skSetupRange // fixture and hook bodies
}

type skSetupReader struct {
	m     *SKModel
	blobs map[string]string
	out   []SKSetup
	files map[string]*skSetupFile
}

// extractSetup reads the setup records of every read TS/JS file and links each test to them.
// blobs maps the declared sources' committed paths to their blob ids.
func (m *SKModel) extractSetup(blobs map[string]string) ([]SKSetup, []SKSetupUse) {
	r := &skSetupReader{m: m, blobs: blobs, out: []SKSetup{}, files: map[string]*skSetupFile{}}
	paths := make([]string, 0, len(m.files))
	for p, f := range m.files {
		if !f.ruby {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	for _, p := range paths {
		r.readFile(m.files[p])
	}
	for _, p := range paths {
		r.within(r.files[p])
	}
	for _, p := range paths {
		sf := r.files[p]
		for _, i := range sf.records {
			if s := &r.out[i]; s.Kind == SetupFixture {
				s.Requires, s.Unresolved = r.resolveFixtures(sf.f, s.Test, s.Params, s.Name)
			}
		}
	}
	uses := []SKSetupUse{}
	for _, p := range paths {
		uses = append(uses, r.link(r.files[p])...)
	}
	return r.out, uses
}

func (r *skSetupReader) add(sf *skSetupFile, tok int, s SKSetup) int {
	f := sf.f
	line := f.toks[tok].line
	s.Path, s.Authority = f.path, skAuthority
	if s.Status == "" {
		s.Status = StatusResolved
	}
	s.ID = "setup:" + f.path + "#" + skDigest(s.Kind, s.Name, s.Target, strconv.Itoa(line), strconv.Itoa(tok))
	s.Anchor = spanOf(f.entry, f.data, line, line)
	r.out = append(r.out, s)
	i := len(r.out) - 1
	sf.records = append(sf.records, i)
	sf.at[i] = tok
	return i
}

// skLiteralArg returns the literal value at toks[k] or reports a dynamic value.
func skLiteralArg(toks []token, k, limit int) (string, bool) {
	if k < limit && literal(toks[k]) && (k+1 >= limit || next(toks, k+1, ",", ")", "}")) {
		return toks[k].text, true
	}
	return "", false
}

func (r *skSetupReader) target(s *SKSetup, toks []token, k, limit int) {
	if v, ok := skLiteralArg(toks, k, limit); ok {
		s.Target = v
		return
	}
	s.Status, s.Reason = StatusUnknown, OpaqueDynamicValue
}

func (r *skSetupReader) readFile(f *skFile) {
	sf := &skSetupFile{f: f, extensions: map[string]*skExtension{}, at: map[int]int{}}
	r.files[f.path] = sf
	toks := f.toks
	for i := 0; i+2 < len(toks); i++ {
		t := toks[i]
		if t.kind != tokIdent {
			continue
		}
		member := i > 1 && next(toks, i-1, ".")
		switch {
		case !member && next(toks, i+1, ".") && toks[i+2].kind == tokIdent && toks[i+2].text == "extend":
			r.extension(sf, i)
		case skHookNames[t.text] && next(toks, i+1, "(") && (!member || (i > 1 && toks[i-2].kind == tokIdent)):
			r.hook(sf, i)
		case member && (t.text == "route" || t.text == "routeFromHAR") && next(toks, i+1, "("):
			s := SKSetup{Kind: SetupRouteMock, Name: t.text, Mocked: true}
			r.target(&s, toks, i+2, closeParen(toks, i+1))
			r.add(sf, i, s)
		case member && t.text == "intercept" && toks[i-2].text == "cy" && next(toks, i+1, "("):
			s := SKSetup{Kind: SetupRouteMock, Name: "cy.intercept", Mocked: true}
			end := closeParen(toks, i+1)
			if v, ok := skLiteralArg(toks, i+2, end); ok && strings.ToUpper(v) == v && next(toks, i+3, ",") {
				s.Method = v
				r.target(&s, toks, i+4, end)
			} else {
				r.target(&s, toks, i+2, end)
			}
			r.add(sf, i, s)
		case member && skSeedMethods[t.text] && next(toks, i+1, "(") && toks[i-2].kind == tokIdent && toks[i-2].text == "request":
			s := SKSetup{Kind: SetupAPISeed, Name: "request." + t.text, Method: strings.ToUpper(t.text)}
			if t.text == "fetch" {
				s.Method = ""
			}
			r.target(&s, toks, i+2, closeParen(toks, i+1))
			r.add(sf, i, s)
		case member && t.text == "request" && toks[i-2].text == "cy" && next(toks, i+1, "("):
			s := SKSetup{Kind: SetupAPISeed, Name: "cy.request"}
			end := closeParen(toks, i+1)
			if v, ok := skLiteralArg(toks, i+2, end); ok && strings.ToUpper(v) == v && next(toks, i+3, ",") {
				s.Method = v
				r.target(&s, toks, i+4, end)
			} else {
				r.target(&s, toks, i+2, end)
			}
			r.add(sf, i, s)
		case member && t.text == "task" && toks[i-2].text == "cy" && next(toks, i+1, "("):
			s := SKSetup{Kind: SetupTask, Name: "cy.task"}
			r.target(&s, toks, i+2, closeParen(toks, i+1))
			r.add(sf, i, s)
		case member && t.text == "storageState" && next(toks, i+1, "("):
			s := SKSetup{Kind: SetupStorageState, Name: "storageState", Mode: "writes"}
			end := closeParen(toks, i+1)
			s.Status, s.Reason = StatusUnknown, OpaqueDynamicValue
			if next(toks, i+2, "{") {
				for k := i + 3; k+2 < end; k++ {
					if toks[k].kind == tokIdent && toks[k].text == "path" && next(toks, k+1, ":") {
						s.Status, s.Reason = "", ""
						r.target(&s, toks, k+2, end)
						break
					}
				}
			}
			r.state(&s)
			r.add(sf, i, s)
		case !member && t.text == "storageState" && skObjectLiteral(toks, i):
			s := SKSetup{Kind: SetupStorageState, Name: "storageState", Mode: "reads"}
			if next(toks, i+1, ":") {
				r.target(&s, toks, i+2, len(toks))
			} else {
				s.Status, s.Reason = StatusUnknown, OpaqueDynamicValue
			}
			r.state(&s)
			r.add(sf, i, s)
		}
	}
}

// state pins a literal storageState path to its committed blob when the declared sources hold it.
func (r *skSetupReader) state(s *SKSetup) {
	if s.Target == "" {
		return
	}
	for _, c := range []string{path.Join(path.Dir(s.Path), s.Target), path.Clean(s.Target)} {
		if oid := r.blobs[c]; oid != "" {
			s.Blob = oid
			return
		}
	}
}

// skObjectLiteral reports whether the property name at toks[i] sits directly inside an object
// literal (a call argument or a value), not a parameter list, destructuring pattern or type.
func skObjectLiteral(toks []token, i int) bool {
	if !next(toks, i+1, ":", ",", "}") || !next(toks, i-1, "{", ",") {
		return false
	}
	depth := 0
	for k := i - 1; k >= 0; k-- {
		if toks[k].kind != tokPunct {
			continue
		}
		switch toks[k].text {
		case "}", ")", "]":
			depth++
		case "(", "[":
			if depth == 0 {
				return false
			}
			depth--
		case "{":
			if depth > 0 {
				depth--
				continue
			}
			if next(toks, k-1, "(") {
				if pc := closeParen(toks, k-1); next(toks, pc+1, "=") && next(toks, pc+2, ">") {
					return false // arrow parameters
				}
				return k > 1 && toks[k-2].kind == tokIdent && toks[k-2].text != "function"
			}
			return next(toks, k-1, "=", ":", ",", "[") || (k > 0 && toks[k-1].kind == tokIdent && toks[k-1].text == "return")
		}
	}
	return false
}

// skFunctionAt reads `async? (params) => {` or `async? function NAME? (params) {` at toks[k] and
// returns the parameter open paren and the body open brace.
func skFunctionAt(toks []token, k, limit int) (open, body int, ok bool) {
	if k < limit && toks[k].kind == tokIdent && toks[k].text == "async" {
		k++
	}
	if k < limit && toks[k].kind == tokIdent && toks[k].text == "function" {
		k++
		if k < limit && toks[k].kind == tokIdent {
			k++
		}
		if next(toks, k, "(") {
			pc := closeParen(toks, k)
			if b := skBodyOpen(toks, pc, limit); b > 0 {
				return k, b, true
			}
		}
		return 0, 0, false
	}
	if !next(toks, k, "(") {
		return 0, 0, false
	}
	pc := closeParen(toks, k)
	j := pc + 1
	if next(toks, j, ":") { // a return type annotation before =>
		for j < limit && !(next(toks, j, "=") && next(toks, j+1, ">")) {
			j++
		}
	}
	if next(toks, j, "=") && next(toks, j+1, ">") && next(toks, j+2, "{") {
		return k, j + 2, true
	}
	return 0, 0, false
}

// skDestructured returns the property names of the first parameter's destructuring pattern.
func skDestructured(toks []token, open int) []string {
	if !next(toks, open+1, "{") {
		return nil
	}
	end := closeParen(toks, open+1)
	names := []string{}
	for q := open + 2; q < end; q++ {
		if toks[q].kind == tokIdent && next(toks, q-1, "{", ",") && (next(toks, q+1, ",", "}", ":", "=") || q+1 == end) {
			names = append(names, toks[q].text)
		}
		if next(toks, q, "{", "[", "(") {
			q = closeParen(toks, q)
		}
	}
	return names
}

func (r *skSetupReader) extension(sf *skSetupFile, i int) {
	toks := sf.f.toks
	k := i + 3
	if next(toks, k, "<") { // type arguments
		for angle := 0; k < len(toks); k++ {
			if next(toks, k, "<") {
				angle++
			} else if next(toks, k, ">") && !next(toks, k-1, "=") {
				if angle--; angle == 0 {
					k++
					break
				}
			}
		}
	}
	if !next(toks, k, "(") || !next(toks, k+1, "{") {
		return
	}
	ext := &skExtension{base: toks[i].text, fixtures: map[string]int{}}
	if i > 1 && next(toks, i-1, "=") && toks[i-2].kind == tokIdent {
		ext.name = toks[i-2].text
		sf.extensions[ext.name] = ext
	}
	obj, end := k+1, closeParen(toks, k+1)
	for q := obj + 1; q < end; q++ {
		if (toks[q].kind != tokIdent && toks[q].kind != tokString) || !next(toks, q-1, "{", ",") {
			if next(toks, q, "{", "[", "(") {
				q = closeParen(toks, q)
			}
			continue
		}
		name, v := toks[q].text, q+2
		if toks[q].kind == tokIdent && toks[q].text == "async" && q+1 < end && toks[q+1].kind == tokIdent && next(toks, q+2, "(") {
			name, v = toks[q+1].text, q+2 // async method shorthand
		} else if !next(toks, q+1, ":") {
			if next(toks, q+1, "(") {
				v = q + 1 // method shorthand
			} else {
				continue
			}
		}
		s := SKSetup{Kind: SetupFixture, Name: name, Produces: []string{name}, Test: ext.name, Base: ext.base}
		fn := v
		if next(toks, v, "[") {
			fn = v + 1
			r.fixtureOptions(&s, toks, v)
		}
		if open, body, ok := skFunctionAt(toks, fn, end); ok {
			r.fixtureBody(&s, toks, open, body)
			idx := r.add(sf, q, s)
			sf.bodies = append(sf.bodies, skSetupRange{from: body, to: closeParen(toks, body), id: r.out[idx].ID})
			ext.fixtures[name] = idx
		} else if fn == v+1 || literal(toks[v]) || toks[v].kind == tokNumber {
			s.Option = true
			ext.fixtures[name] = r.add(sf, q, s)
		}
		q = skValueEnd(toks, v, end)
	}
}

// skValueEnd returns the index of the `,` or close that ends the object property value at v.
func skValueEnd(toks []token, v, end int) int {
	for k := v; k < end; k++ {
		if next(toks, k, ",") {
			return k
		}
		if next(toks, k, "{", "[", "(") {
			k = closeParen(toks, k)
		}
	}
	return end
}

// fixtureOptions reads `[value, { scope: 'worker', auto: true, option: true }]`.
func (r *skSetupReader) fixtureOptions(s *SKSetup, toks []token, open int) {
	end := closeParen(toks, open)
	for k := open + 1; k+2 < end; k++ {
		if toks[k].kind != tokIdent || !next(toks, k+1, ":") || !next(toks, k-1, "{", ",") {
			continue
		}
		switch v := toks[k+2]; toks[k].text {
		case "scope":
			if literal(v) {
				s.Scope = v.text
			}
		case "auto":
			s.Auto = v.kind == tokIdent && v.text == "true"
		case "option":
			s.Option = v.kind == tokIdent && v.text == "true"
		}
	}
}

// fixtureBody reads the dependencies, the use() call and any cleanup after it.
func (r *skSetupReader) fixtureBody(s *SKSetup, toks []token, open, body int) {
	s.Params = skDestructured(toks, open)
	pc := closeParen(toks, open)
	use := ""
	for k := open + 1; k < pc; k++ {
		if next(toks, k, ",") && k+1 < pc && toks[k+1].kind == tokIdent {
			use = toks[k+1].text
			break
		}
		if next(toks, k, "{", "[", "(") {
			k = closeParen(toks, k)
		}
	}
	end := closeParen(toks, body)
	for k := body + 1; use != "" && k < end; k++ {
		if toks[k].kind == tokIdent && toks[k].text == use && next(toks, k+1, "(") && !next(toks, k-1, ".") {
			for q := closeParen(toks, k+1) + 1; q < end; q++ {
				if !next(toks, q, ";", "}", ")") {
					s.Cleanup = "after-use"
					break
				}
			}
			break
		}
	}
}

func (r *skSetupReader) hook(sf *skSetupFile, i int) {
	toks := sf.f.toks
	end := closeParen(toks, i+1)
	for k := i + 2; k < end; k++ {
		open, body, ok := skFunctionAt(toks, k, end)
		if !ok {
			continue
		}
		s := SKSetup{Kind: SetupHook, Name: toks[i].text, Params: skDestructured(toks, open)}
		if i > 1 && next(toks, i-1, ".") {
			s.Test = toks[i-2].text
		}
		switch toks[i].text {
		case "afterEach":
			s.Cleanup = "after-each"
		case "afterAll", "after":
			s.Cleanup = "after-all"
		}
		idx := r.add(sf, i, s)
		sf.bodies = append(sf.bodies, skSetupRange{from: body, to: closeParen(toks, body), id: r.out[idx].ID})
		return
	}
}

// fileUnits lists every callable body of f with its record key.
func (r *skSetupReader) fileUnits(f *skFile) []skSetupRange {
	out := []skSetupRange{}
	for _, n := range f.forder {
		u := f.funcs[n]
		out = append(out, skSetupRange{from: u.from - 1, to: u.to, id: "function:" + f.path + "#" + n})
	}
	for _, cn := range f.corder {
		c := f.classes[cn]
		for _, n := range c.order {
			u := c.methods[n]
			out = append(out, skSetupRange{from: u.from - 1, to: u.to, id: "function:" + f.path + "#" + cn + "." + n})
		}
	}
	for _, u := range f.tests {
		out = append(out, skSetupRange{from: u.from - 1, to: u.to, id: skTestID(f.path, u)})
	}
	return out
}

func skTestID(p string, u *skUnit) string {
	return "test:" + p + "#" + skDigest(u.title, strconv.Itoa(u.start))
}

// within sets each record's innermost enclosing fixture, hook, helper or test body.
func (r *skSetupReader) within(sf *skSetupFile) {
	ranges := append(append([]skSetupRange{}, sf.bodies...), r.fileUnits(sf.f)...)
	for _, i := range sf.records {
		tok, best := sf.at[i], -1
		for j, g := range ranges {
			if g.from < tok && tok < g.to && g.id != r.out[i].ID && (best < 0 || g.to-g.from < ranges[best].to-ranges[best].from) {
				best = j
			}
		}
		if best >= 0 {
			r.out[i].Within = ranges[best].id
		}
	}
}

// extensionOf finds the extension a test-object name denotes in f: declared there or imported.
func (r *skSetupReader) extensionOf(f *skFile, name string) (*skSetupFile, *skExtension) {
	for depth := 0; depth <= maxSKHops; depth++ {
		sf := r.files[f.path]
		if sf == nil {
			return nil, nil
		}
		if ext := sf.extensions[name]; ext != nil {
			return sf, ext
		}
		imp, ok := f.imports[name]
		if !ok {
			return nil, nil
		}
		g := r.m.importFile(imp.base)
		if g == nil {
			return nil, nil
		}
		f, name = g, imp.name
	}
	return nil, nil
}

// fixture resolves one fixture name visible to the test object `test` in f, along the extend chain.
func (r *skSetupReader) fixture(f *skFile, test, name string) (int, bool) {
	for depth := 0; depth <= maxSKHops; depth++ {
		sf, ext := r.extensionOf(f, test)
		if ext == nil {
			return 0, false
		}
		if i, ok := ext.fixtures[name]; ok {
			return i, true
		}
		f, test = sf.f, ext.base
	}
	return 0, false
}

// resolveFixtures resolves fixture parameter names to records, following each record's own
// requirements, and names the rest.
func (r *skSetupReader) resolveFixtures(f *skFile, test string, names []string, self string) ([]string, []string) {
	ids, unresolved := []string{}, []string{}
	for _, n := range names {
		if n == self && test != "" {
			// A fixture that names itself overrides the base's fixture of that name.
			if sf, ext := r.extensionOf(f, test); ext != nil {
				if i, ok := r.fixture(sf.f, ext.base, n); ok {
					ids = append(ids, r.out[i].ID)
				}
			}
			continue
		}
		if i, ok := r.fixture(f, test, n); ok {
			ids = append(ids, r.out[i].ID)
		} else if !skBuiltinFixtures[n] {
			unresolved = append(unresolved, n)
		}
	}
	return nilIfEmpty(uniqueSorted(ids)), nilIfEmpty(uniqueSorted(unresolved))
}

func nilIfEmpty(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}

// enclosing returns the token range of the innermost block around toks[tok], or the whole file.
// An object-literal brace (`test.use({ storageState: ... })`) is not a block.
func skEnclosing(toks []token, tok int) (int, int) {
	block := func(k int) bool {
		if k == 0 {
			return true
		}
		p := toks[k-1]
		if p.kind == tokIdent {
			return p.text == "else" || p.text == "try" || p.text == "finally" || p.text == "do"
		}
		return p.kind == tokPunct && (p.text == ")" || p.text == ";" || p.text == "{" || p.text == "}" ||
			(p.text == ">" && k > 1 && next(toks, k-2, "=")))
	}
	depth := 0
	for k := tok - 1; k >= 0; k-- {
		if toks[k].kind != tokPunct {
			continue
		}
		switch toks[k].text {
		case "}", ")", "]":
			depth++
		case "{", "(", "[":
			if depth == 0 && toks[k].text == "{" && block(k) {
				return k, closeParen(toks, k)
			}
			if depth > 0 {
				depth--
			}
		}
	}
	return -1, len(toks)
}

func (r *skSetupReader) link(sf *skSetupFile) []SKSetupUse {
	f := sf.f
	uses := []SKSetupUse{}
	byID := map[string]int{}
	for i := range r.out {
		byID[r.out[i].ID] = i
	}
	for _, u := range f.tests {
		use := SKSetupUse{Test: skTestID(f.path, u), Title: u.title}
		ids := map[string]bool{}
		var follow func(id string, depth int)
		follow = func(id string, depth int) {
			if ids[id] || depth > maxSKHops {
				return
			}
			ids[id] = true
			for _, q := range r.out[byID[id]].Requires {
				follow(q, depth+1)
			}
			// Records inside a fixture or hook body arrange the tests that use it.
			for i := range r.out {
				if r.out[i].Within == id {
					follow(r.out[i].ID, depth+1)
				}
			}
		}
		req, unresolved := r.resolveFixtures(f, u.name, skDestructured(f.toks, u.open), "")
		for _, id := range req {
			follow(id, 0)
		}
		// Auto fixtures of the test object's chain run for every test.
		for cur, test, depth := f, u.name, 0; depth <= maxSKHops; depth++ {
			esf, ext := r.extensionOf(cur, test)
			if ext == nil {
				break
			}
			for _, i := range ext.fixtures {
				if r.out[i].Auto {
					follow(r.out[i].ID, 0)
				}
			}
			cur, test = esf.f, ext.base
		}
		// Hooks and storageState outside any body apply to the tests of their enclosing block.
		for _, i := range sf.records {
			s := r.out[i]
			if s.Within != "" || (s.Kind != SetupHook && s.Kind != SetupStorageState) {
				continue
			}
			if from, to := skEnclosing(f.toks, sf.at[i]); from < u.from && u.to <= to {
				follow(s.ID, 0)
			}
		}
		// Records inside the test body or the helpers it reaches.
		for _, v := range r.reach(u) {
			key := skTestID(v.file.path, v)
			if v != u {
				key = "function:" + v.file.path + "#" + v.name
				if v.class != nil {
					key = "function:" + v.file.path + "#" + v.class.name + "." + v.name
				}
			}
			for i := range r.out {
				if r.out[i].Within == key {
					follow(r.out[i].ID, 0)
				}
			}
		}
		for id := range ids {
			use.Uses = append(use.Uses, id)
		}
		sort.Strings(use.Uses)
		if use.Uses == nil {
			use.Uses = []string{}
		}
		use.Unresolved = unresolved
		uses = append(uses, use)
	}
	return uses
}

// reach returns u and every unit its resolved calls and setup helpers reach, bounded.
func (r *skSetupReader) reach(u *skUnit) []*skUnit {
	seen := map[*skUnit]bool{}
	out := []*skUnit{}
	var walk func(*skUnit, int)
	walk = func(v *skUnit, depth int) {
		if seen[v] || depth > maxSKHops {
			return
		}
		seen[v] = true
		out = append(out, v)
		names := append([]string{}, v.setup...)
		for _, s := range r.m.bodySteps(v) {
			if s.Kind == skCall {
				names = append(names, s.Call)
			}
		}
		for _, n := range names {
			if t, _, _ := r.m.resolveCall(v, n); t != nil {
				walk(t, depth+1)
			}
		}
	}
	walk(u, 0)
	return out
}
