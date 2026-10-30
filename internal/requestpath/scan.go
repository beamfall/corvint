package requestpath

import (
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
)

// outboundPackages make network calls through any member (AMAP-V0-132). The list is closed; a
// package on neither list is reported UNKNOWN, never assumed local.
var outboundPackages = map[string]bool{"axios": true, "node-fetch": true, "cross-fetch": true, "got": true, "undici": true,
	"superagent": true, "ky": true, "needle": true, "request": true, "openid-client": true}

// nodeHTTP make network calls only through request and get.
var nodeHTTP = map[string]bool{"http": true, "https": true}

// localMembers of an outbound package build or inspect a client without a network call.
var localMembers = map[string]bool{"create": true, "defaults": true, "interceptors": true, "isAxiosError": true,
	"isCancel": true, "CancelToken": true, "Headers": true, "Request": true, "Response": true}

// localPackages make no network call on the request path.
var localPackages = map[string]bool{"express": true, "jsonwebtoken": true, "zod": true, "joi": true, "yup": true, "ajv": true,
	"validator": true, "express-validator": true, "lodash": true, "uuid": true, "crypto": true, "path": true, "url": true,
	"querystring": true, "util": true, "cookie": true, "cookie-parser": true, "http-errors": true, "dayjs": true,
	"date-fns": true, "config": true, "nconf": true, "dotenv": true, "buffer": true, "assert": true}

var configPackages = map[string]bool{"config": true, "nconf": true}

// packageRoot is the package a module specifier names: `@scope/name` or the first segment, without
// a `node:` prefix.
func packageRoot(module string) string {
	m := strings.TrimPrefix(module, "node:")
	parts := strings.Split(m, "/")
	if strings.HasPrefix(m, "@") && len(parts) > 1 {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

// classify reports whether calling member of package module is outbound, local, or not followed.
func classify(module, member string) string {
	root := packageRoot(module)
	switch {
	case nodeHTTP[root]:
		if member == "request" || member == "get" {
			return OutboundYes
		}
		return OutboundNoneFound
	case outboundPackages[root]:
		if localMembers[member] {
			return OutboundNoneFound
		}
		return OutboundYes
	case localPackages[root]:
		return OutboundNoneFound
	}
	return OutboundUnknown
}

var callKeywords = map[string]bool{"if": true, "for": true, "while": true, "switch": true, "catch": true, "function": true,
	"return": true, "typeof": true, "super": true, "import": true, "await": true, "new": true, "void": true, "delete": true,
	"in": true, "of": true, "instanceof": true, "throw": true, "yield": true, "case": true, "else": true, "do": true,
	"async": true, "const": true, "let": true, "var": true, "require": true}

// stepScan is the per-step scan state.
type stepScan struct {
	order   int
	visited map[*body]bool
	keys    map[string]bool
	config  map[string]bool
	calls   int
	unknown int
	bounded bool
	req     string          // the request parameter of the step function
	params  map[string]bool // the step function's parameters (Express req, res, next)
	unsure  map[*jsFile]bool
}

func (a *analyzer) runStep(order int, role string, p pending) {
	st := &stepScan{order: order, visited: map[*body]bool{}, keys: map[string]bool{}, config: map[string]bool{},
		params: map[string]bool{}, unsure: map[*jsFile]bool{}}
	file := ""
	if p.file != nil {
		file = p.file.path
	}
	step := Step{Order: order, Role: role, Name: p.name, Registered: p.registered, Location: Location{File: file, Line: p.line}}
	before := len(a.report.Unknowns)
	switch {
	case p.unknown != "":
		a.stepUnknown(st, UnknownMiddleware, p.unknown, file, p.line, p.name)
	case p.pkg != "":
		switch classify(p.pkg, "") {
		case OutboundYes:
			a.call(st, p.name, nil, file, p.line)
		case OutboundUnknown:
			a.stepUnknown(st, UnknownMiddleware, "package-not-followed", file, p.line, p.name+" ("+packageRoot(p.pkg)+")")
		}
	case p.fn != nil:
		if len(p.fn.params) > 0 {
			st.req = p.fn.params[0]
		}
		for _, n := range p.fn.params {
			st.params[n] = true
		}
		a.scan(st, p.fn, []string{p.name}, 0)
	}
	if p.args != nil {
		a.scan(st, p.args, []string{p.name}, 0)
	}
	step.Unknowns = len(a.report.Unknowns) - before
	step.EnvKeys = sortedSet(st.keys)
	step.ConfigKeys = sortedSet(st.config)
	switch {
	case st.calls > 0:
		step.Outbound = OutboundYes
	case st.unknown > 0:
		step.Outbound = OutboundUnknown
	default:
		step.Outbound = OutboundNoneFound
	}
	a.report.Steps = append(a.report.Steps, step)
}

func sortedSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// stepUnknown records an unknown that leaves the step's outbound state UNKNOWN unless a call is
// found.
func (a *analyzer) stepUnknown(st *stepScan, kind, reason, file string, line int, text string) {
	if kind == UnknownCall || kind == UnknownBound || kind == UnknownSource || kind == UnknownMiddleware {
		st.unknown++
	}
	a.unknown(st.order, kind, reason, file, line, text)
}

func (a *analyzer) call(st *stepScan, callee string, via []string, file string, line int) {
	st.calls++
	a.report.OutboundCalls = append(a.report.OutboundCalls, Call{Step: st.order, Callee: callee, Via: append([]string{}, via...), Location: Location{File: file, Line: line}})
}

func (a *analyzer) key(m map[string]*Key, stepKeys map[string]bool, st *stepScan, name string, defaulted bool, file string, line int) {
	k := m[name]
	if k == nil {
		k = &Key{Name: name, Defaulted: true, Steps: []int{}, ReadAt: []Location{}}
		m[name] = k
	}
	k.Defaulted = k.Defaulted && defaulted
	if len(k.Steps) == 0 || k.Steps[len(k.Steps)-1] != st.order {
		k.Steps = append(k.Steps, st.order)
	}
	loc := Location{File: file, Line: line}
	for _, r := range k.ReadAt {
		if r == loc {
			stepKeys[name] = true
			return
		}
	}
	k.ReadAt = append(k.ReadAt, loc)
	stepKeys[name] = true
}

// scan reads one function body or initializer for environment and configuration reads, calls and
// referenced top-level values, following calls it can resolve (AMAP-V0-131, AMAP-V0-132).
func (a *analyzer) scan(st *stepScan, b *body, via []string, depth int) {
	if st.visited[b] {
		return
	}
	st.visited[b] = true
	a.visits++
	if a.visits > maxVisits || depth > maxDepth {
		a.bounded = true
		if !st.bounded {
			st.bounded = true
			a.stepUnknown(st, UnknownBound, "bound-exceeded", b.file.path, b.line, b.name)
		}
		return
	}
	f := b.file
	if f.unsure && !st.unsure[f] {
		st.unsure[f] = true
		a.stepUnknown(st, UnknownSource, "lexer-unsure", f.path, 1, f.path)
	}
	toks := f.toks
	end := b.end
	if end > len(toks) {
		end = len(toks)
	}
	for j := b.start; j < end; j++ {
		t := toks[j]
		if t.Kind != appmap.JSIdent || (j > 0 && (isPunct(toks[j-1], ".") || isIdent(toks[j-1], "function"))) {
			continue
		}
		if t.Text == "process" && j+2 < end && isPunct(toks[j+1], ".") && isIdent(toks[j+2], "env") {
			j = a.envRead(st, f, b, j, end)
			continue
		}
		if callKeywords[t.Text] {
			continue
		}
		chain, next := memberChain(toks, j, end)
		called := next < end && isPunct(toks[next], "(")
		if j > 0 && isIdent(toks[j-1], "new") {
			j = next - 1
			continue
		}
		if called && len(chain) == 2 && (chain[1] == "get" || chain[1] == "has") && a.configRead(st, f, chain[0], toks, next, end) {
			j = next
			continue
		}
		if called {
			a.callChain(st, b, chain, via, toks[j].Line, depth)
		} else {
			a.reference(st, b, chain[0], via, depth)
		}
		j = next - 1
	}
}

// memberChain reads `a.b.c` from j; next is the index after it.
func memberChain(toks []tok, j, end int) ([]string, int) {
	chain := []string{toks[j].Text}
	k := j + 1
	for k+1 < end && (isPunct(toks[k], ".") || (isPunct(toks[k], "?") && k+2 < end && isPunct(toks[k+1], "."))) {
		n := k + 1
		if isPunct(toks[k], "?") {
			n = k + 2
		}
		if n >= end || toks[n].Kind != appmap.JSIdent {
			break
		}
		chain = append(chain, toks[n].Text)
		k = n + 1
	}
	return chain, k
}

// envRead reads `process.env...` at j and returns the last index it consumed (AMAP-V0-131, AMAP-V0-134).
func (a *analyzer) envRead(st *stepScan, f *jsFile, b *body, j, end int) int {
	toks := f.toks
	line := toks[j].Line
	k := j + 3
	name := ""
	switch {
	case k+1 < end && isPunct(toks[k], ".") && toks[k+1].Kind == appmap.JSIdent:
		name, k = toks[k+1].Text, k+2
	case k+2 < end && isPunct(toks[k], "[") && closeOf(toks, k) == k+2:
		inner := toks[k+1]
		switch {
		case isString(inner):
			name = inner.Text
		case inner.Kind == appmap.JSIdent:
			name = a.stringConst(f, inner.Text)
		}
		if name == "" {
			a.stepUnknown(st, UnknownEnvKey, "dynamic-key", f.path, line, "process.env["+inner.Text+"]")
			return k + 2
		}
		k += 3
	case k < end && isPunct(toks[k], "["):
		c := closeOf(toks, k)
		a.stepUnknown(st, UnknownEnvKey, "dynamic-key", f.path, line, "process.env["+tokenText(toks[k+1:min(c, len(toks))])+"]")
		return k // the subscript expression is scanned too
	default:
		if a.destructure(st, f, b, j, line) {
			return k - 1
		}
		a.stepUnknown(st, UnknownEnvKey, "env-object-escapes", f.path, line, "process.env")
		return k - 1
	}
	a.key(a.env, st.keys, st, name, defaultedAt(toks, k, end), f.path, line)
	return k - 1
}

// defaultedAt reports whether the read ending before k is followed by `||` or `??`.
func defaultedAt(toks []tok, k, end int) bool {
	return k+1 < end && ((isPunct(toks[k], "|") && isPunct(toks[k+1], "|")) || (isPunct(toks[k], "?") && isPunct(toks[k+1], "?")))
}

// destructure reads `{ A, B = 'x', C: c } = process.env` ending at j; a rest element escapes.
func (a *analyzer) destructure(st *stepScan, f *jsFile, b *body, j, line int) bool {
	toks := f.toks
	if j < 2 || !isPunct(toks[j-1], "=") || !isPunct(toks[j-2], "}") {
		return false
	}
	open := -1
	depth := 0
	for i := j - 2; i >= b.start && i >= 0; i-- {
		if isPunct(toks[i], "}") {
			depth++
		} else if isPunct(toks[i], "{") {
			depth--
			if depth == 0 {
				open = i
				break
			}
		}
	}
	if open < 0 {
		return false
	}
	expect := true
	for i := open + 1; i < j-2; i++ {
		t := toks[i]
		switch {
		case isPunct(t, ","):
			expect = true
		case isPunct(t, "."):
			if expect {
				a.stepUnknown(st, UnknownEnvKey, "env-object-escapes", f.path, line, "...rest = process.env")
				expect = false
			}
		case expect && (t.Kind == appmap.JSIdent || isString(t)):
			defaulted := false
			for n := i + 1; n < j-2 && !isPunct(toks[n], ","); n++ {
				if isPunct(toks[n], "=") {
					defaulted = true
				}
			}
			a.key(a.env, st.keys, st, t.Text, defaulted, f.path, line)
			expect = false
		}
	}
	return true
}

// stringConst resolves a top-level `const NAME = 'literal'` in f.
func (a *analyzer) stringConst(f *jsFile, name string) string {
	v := f.values[name]
	if v == nil || v.start < 0 || v.end != v.start+1 || v.start >= len(f.toks) || !isString(f.toks[v.start]) {
		return ""
	}
	return f.toks[v.start].Text
}

// configRead reads `<config>.get|has('key')` when recv resolves to a configuration package.
func (a *analyzer) configRead(st *stepScan, f *jsFile, recv string, toks []tok, open, end int) bool {
	t := a.resolveName(f, recv, 0)
	if t.pkg == "" || !configPackages[packageRoot(t.pkg)] {
		return false
	}
	line := toks[open].Line
	if open+2 < end && isString(toks[open+1]) && (isPunct(toks[open+2], ")") || isPunct(toks[open+2], ",")) {
		a.key(a.config, st.config, st, toks[open+1].Text, false, f.path, line)
		return true
	}
	c := closeOf(toks, open)
	a.stepUnknown(st, UnknownConfigKey, "dynamic-key", f.path, line, recv+".get("+tokenText(toks[open+1:min(c, len(toks))])+")")
	return true
}

// callChain follows or classifies one call.
func (a *analyzer) callChain(st *stepScan, b *body, chain, via []string, line, depth int) {
	f := b.file
	text := strings.Join(chain, ".")
	if chain[0] == "this" {
		if len(chain) == 2 && b.owner != nil && b.owner[chain[1]] != nil {
			if t := a.member(f, b.owner[chain[1]], 0); t.fn != nil {
				a.scan(st, t.fn, append(via, t.fn.name), depth+1)
				return
			}
		}
		a.stepUnknown(st, UnknownCall, "receiver-not-resolved", f.path, line, text)
		return
	}
	if chain[0] == st.req && st.req != "" && len(chain) >= 3 {
		a.stepUnknown(st, UnknownCall, "request-object-call", f.path, line, text)
		return
	}
	t := a.resolveName(f, chain[0], 0)
	for _, m := range chain[1:] {
		if t.fn != nil {
			return // a call on a function's property is not followed
		}
		t = a.memberOf(f, t, m)
	}
	switch {
	case t.fn != nil:
		a.scan(st, t.fn, append(via, t.fn.name), depth+1)
	case t.pkg != "":
		a.reference(st, b, chain[0], via, depth) // a top-level client's initializer (AMAP-V0-131)
		switch classify(t.pkg, t.pkgMember) {
		case OutboundYes:
			a.call(st, text, via, f.path, line)
		case OutboundUnknown:
			a.stepUnknown(st, UnknownCall, "package-not-followed", f.path, line, text+" ("+packageRoot(t.pkg)+")")
		}
	case t.unknown != "":
		a.stepUnknown(st, UnknownCall, t.unknown, f.path, line, text)
	case chain[0] == "fetch" && len(chain) == 1:
		a.call(st, "fetch", via, f.path, line)
	case st.params[chain[0]] && len(chain) <= 2, builtinGlobals[chain[0]]:
		// The Express req/res/next API of the step function, or a JavaScript built-in.
	default:
		if !a.reference(st, b, chain[0], via, depth) {
			a.stepUnknown(st, UnknownCall, "callee-not-resolved", f.path, line, text)
		}
	}
}

// builtinGlobals are JavaScript and Node.js built-ins that make no network call.
var builtinGlobals = map[string]bool{"JSON": true, "Object": true, "Array": true, "Math": true, "console": true, "Promise": true,
	"Number": true, "String": true, "Boolean": true, "Date": true, "Buffer": true, "Error": true, "TypeError": true,
	"RangeError": true, "parseInt": true, "parseFloat": true, "isNaN": true, "isFinite": true, "Symbol": true, "Map": true,
	"Set": true, "WeakMap": true, "WeakSet": true, "RegExp": true, "encodeURIComponent": true, "decodeURIComponent": true,
	"encodeURI": true, "decodeURI": true, "structuredClone": true, "setTimeout": true, "clearTimeout": true,
	"setImmediate": true, "queueMicrotask": true, "BigInt": true, "URL": true, "URLSearchParams": true,
	"TextEncoder": true, "TextDecoder": true, "atob": true, "btoa": true, "Reflect": true}

// reference scans a top-level value or a function a name refers to (a callback or an initializer
// read on the path) once per step. It reports false when name is not a top-level declaration or
// import of the file, such as a parameter, local or undeclared global.
func (a *analyzer) reference(st *stepScan, b *body, name string, via []string, depth int) bool {
	f := b.file
	if v := f.values[name]; v != nil && v.start >= 0 && v != b {
		if _, imported := f.imports[name]; !imported {
			a.scan(st, v, via, depth+1)
			return true
		}
	}
	if fn := f.funcs[name]; fn != nil {
		a.scan(st, fn, append(via, fn.name), depth+1)
		return true
	}
	if f.members[name] != nil || f.newOf[name] != "" {
		return true
	}
	bnd, ok := f.imports[name]
	if !ok {
		return false
	}
	if !relative(bnd.module) {
		return true
	}
	g, reason := a.module(f, bnd.module)
	if g == nil {
		a.stepUnknown(st, UnknownCall, reason, f.path, b.line, name+" ("+bnd.module+")")
		return true
	}
	local := g.exports[bnd.name]
	if local == "" && bnd.name == "module" {
		local = g.cjs
	}
	if local == "" {
		return true
	}
	if v := g.values[local]; v != nil && v.start >= 0 {
		a.scan(st, v, via, depth+1)
		return true
	}
	if fn := g.funcs[local]; fn != nil {
		a.scan(st, fn, append(via, fn.name), depth+1)
	}
	return true
}
