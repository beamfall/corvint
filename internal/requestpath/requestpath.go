// Package requestpath reads, from committed JavaScript/TypeScript source, the environment and
// configuration keys an Express-style route's request path reads, the outbound network calls it
// makes, and the order of its guards (docs/specs/application-map-v0.md, AMAP-V0-130..139). It is a
// lexical, bounded reader: it never runs code, never reads the working tree or the network, and a
// key or call it cannot resolve statically is reported UNKNOWN with a reason, never left out.
package requestpath

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
)

// Schema names the report (AMAP-V0-130).
const Schema = "corvint-request-path/0"

// Bounds (AMAP-V0-137).
const (
	MaxFiles     = 64
	MaxFileBytes = 1 << 20
	maxVisits    = 512
	maxDepth     = 24
)

// Step roles and outbound states (AMAP-V0-131, AMAP-V0-132).
const (
	RoleGuard   = "GUARD"
	RoleHandler = "HANDLER"

	OutboundYes       = "YES"
	OutboundNoneFound = "NONE_FOUND"
	OutboundUnknown   = "UNKNOWN"
)

// Unknown kinds (AMAP-V0-134). The set is closed.
const (
	UnknownEnvKey     = "ENV_KEY"
	UnknownConfigKey  = "CONFIG_KEY"
	UnknownCall       = "CALL"
	UnknownMiddleware = "MIDDLEWARE"
	UnknownMount      = "MOUNT"
	UnknownSource     = "SOURCE"
	UnknownBound      = "BOUND"
)

// FindingOutboundGuardFirst is the guard-order finding (AMAP-V0-133).
const FindingOutboundGuardFirst = "OUTBOUND_GUARD_BEFORE_CHEAP_GUARD"

// Source reads committed files at one revision.
type Source interface {
	// Read returns the first candidate that is a committed regular file and its text; found is
	// false when none is. An error is a read that failed or exceeded MaxFileBytes.
	Read(candidates []string) (found string, text string, ok bool, err error)
}

// Error is a refusal; Code is stable.
type Error struct{ Code, Message string }

func (e *Error) Error() string { return e.Message }

func refuse(code, format string, args ...any) error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Query selects the request path: a route registered in File, or one named handler there.
type Query struct {
	File    string
	Method  string // with Path; upper case
	Path    string
	Handler string
}

// Location is a file and 1-based line.
type Location struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

// Key is one environment or configuration key read on the path. Defaulted is true when every read
// supplies a fallback (`||` or `??`) at the read.
type Key struct {
	Name      string     `json:"name"`
	Defaulted bool       `json:"defaulted"`
	Steps     []int      `json:"steps"`
	ReadAt    []Location `json:"readAt"`
}

// Call is one outbound network call: the callee as written and the function chain from the step.
type Call struct {
	Step   int      `json:"step"`
	Callee string   `json:"callee"`
	Via    []string `json:"via"`
	Location
}

// Unknown is one key, call, middleware or mount the reader could not resolve.
type Unknown struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Text   string `json:"text"`
	Step   int    `json:"step,omitempty"`
	Location
}

// Step is one guard or the handler, in run order.
type Step struct {
	Order      int      `json:"order"`
	Role       string   `json:"role"`
	Name       string   `json:"name"`
	Registered string   `json:"registered"` // "route" or "use"
	Outbound   string   `json:"outbound"`
	EnvKeys    []string `json:"envKeys"`
	ConfigKeys []string `json:"configKeys"`
	Unknowns   int      `json:"unknowns"`
	Location
}

// Finding is one guard-order finding.
type Finding struct {
	Finding       string   `json:"finding"`
	OutboundGuard int      `json:"outboundGuard"`
	CheapGuards   []int    `json:"cheapGuards"`
	StandInFor    []string `json:"standInFor"`
	Message       string   `json:"message"`
}

// Report is the request-path report.
type Report struct {
	Schema        string    `json:"schema"`
	Revision      string    `json:"revision"`
	File          string    `json:"file"`
	Route         string    `json:"route,omitempty"`
	Handler       string    `json:"handler,omitempty"`
	Steps         []Step    `json:"steps"`
	EnvKeys       []Key     `json:"envKeys"`
	ConfigKeys    []Key     `json:"configKeys"`
	OutboundCalls []Call    `json:"outboundCalls"`
	Unknowns      []Unknown `json:"unknowns"`
	GuardOrder    []Finding `json:"guardOrder"`
	FilesRead     []string  `json:"filesRead"`
	PreRun        *PreRun   `json:"preRun,omitempty"`
}

type analyzer struct {
	src     Source
	files   map[string]*jsFile
	visits  int
	bounded bool
	report  *Report
	env     map[string]*Key
	config  map[string]*Key
}

// Analyze reads one request path at revision (named in the report only).
func Analyze(src Source, revision string, q Query) (*Report, error) {
	a := &analyzer{src: src, files: map[string]*jsFile{}, env: map[string]*Key{}, config: map[string]*Key{},
		report: &Report{Schema: Schema, Revision: revision, File: q.File, Steps: []Step{}, EnvKeys: []Key{}, ConfigKeys: []Key{},
			OutboundCalls: []Call{}, Unknowns: []Unknown{}, GuardOrder: []Finding{}}}
	f, err := a.load([]string{q.File})
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, refuse("request-path-file-not-found", "%s is not a committed regular file at %s", q.File, revision)
	}
	if f.unsure {
		a.unknown(0, UnknownSource, "lexer-unsure", f.path, 1, f.path)
	}
	var steps []pending
	if q.Handler != "" {
		a.report.Handler = q.Handler
		t := a.resolveName(f, q.Handler, 0)
		switch {
		case t.fn != nil:
		case t.unknown == "bound-exceeded":
			return nil, refuse("request-path-bound-exceeded", "the module declaring %s exceeds a read bound", q.Handler)
		case t.unknown == "read-failed":
			return nil, refuse("request-path-source-unavailable", "the module declaring %s could not be read", q.Handler)
		default:
			return nil, refuse("request-path-handler-not-found", "no function %s is declared or imported in %s", q.Handler, q.File)
		}
		steps = []pending{{name: q.Handler, registered: "route", file: t.fn.file, fn: t.fn, line: t.fn.line}}
	} else {
		a.report.Route = q.Method + " " + q.Path
		steps, err = a.routeSteps(f, q)
		if err != nil {
			return nil, err
		}
	}
	for i, p := range steps {
		role := RoleGuard
		if i == len(steps)-1 {
			role = RoleHandler
		}
		a.runStep(i+1, role, p)
	}
	a.finish()
	return a.report, nil
}

// pending is one step before its scan: its function or inline body, any factory call's argument
// tokens, or the reason it cannot be resolved.
type pending struct {
	name       string
	registered string
	file       *jsFile
	line       int
	fn         *body
	args       *body
	pkg        string // a middleware a package provides
	unknown    string
}

func relative(module string) bool {
	return strings.HasPrefix(module, "./") || strings.HasPrefix(module, "../")
}

var extensions = []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts"}

func (a *analyzer) load(candidates []string) (*jsFile, error) {
	for _, c := range candidates {
		if f, ok := a.files[c]; ok && f != nil {
			return f, nil
		}
	}
	if len(a.files) >= MaxFiles {
		a.bounded = true
		return nil, refuse("request-path-bound-exceeded", "more than %d files on the path", MaxFiles)
	}
	found, text, ok, err := a.src.Read(candidates)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	f := parseFile(found, text)
	a.files[found] = f
	return f, nil
}

// module resolves a relative module specifier from file f; reason names why it did not.
func (a *analyzer) module(f *jsFile, spec string) (*jsFile, string) {
	base := path.Clean(path.Join(path.Dir(f.path), spec))
	if strings.HasPrefix(base, "../") || base == ".." {
		return nil, "module-outside-repository"
	}
	candidates := []string{base}
	stem := base
	for _, js := range []string{".js", ".jsx", ".mjs", ".cjs"} {
		if strings.HasSuffix(base, js) {
			stem = strings.TrimSuffix(base, js) // a TypeScript ESM import names the emitted .js file
		}
	}
	for _, e := range extensions {
		candidates = append(candidates, stem+e)
	}
	for _, e := range extensions {
		candidates = append(candidates, base+"/index"+e)
	}
	g, err := a.load(candidates)
	if err != nil {
		if e, ok := err.(*Error); ok && e.Code == "request-path-bound-exceeded" {
			return nil, "bound-exceeded"
		}
		return nil, "read-failed"
	}
	if g == nil {
		return nil, "module-not-found"
	}
	if g.unsure {
		a.unknown(0, UnknownSource, "lexer-unsure", g.path, 1, g.path)
	}
	return g, ""
}

// target is what a name resolves to.
type target struct {
	fn        *body
	members   map[string]*body
	namespace *jsFile
	pkg       string
	pkgMember string
	unknown   string
}

func (a *analyzer) resolveName(f *jsFile, name string, depth int) target {
	if depth > maxDepth {
		return target{unknown: "bound-exceeded"}
	}
	if b := f.funcs[name]; b != nil {
		return target{fn: b}
	}
	if m := f.members[name]; m != nil {
		return target{members: m}
	}
	if c, ok := f.newOf[name]; ok {
		t := a.resolveName(f, c, depth+1)
		if t.pkg != "" {
			return target{pkg: t.pkg, pkgMember: t.pkgMember}
		}
		return target{members: t.members, unknown: t.unknown}
	}
	if b, ok := f.imports[name]; ok {
		return a.resolveImport(f, b, depth+1)
	}
	if d, ok := f.derived[name]; ok {
		if t := a.resolveName(f, d, depth+1); t.pkg != "" {
			return target{pkg: t.pkg} // the client itself; its members are classified on call
		}
	}
	return target{}
}

func (a *analyzer) resolveImport(f *jsFile, b binding, depth int) target {
	if !relative(b.module) {
		return target{pkg: b.module, pkgMember: b.name}
	}
	g, reason := a.module(f, b.module)
	if g == nil {
		return target{unknown: reason}
	}
	switch b.name {
	case "*":
		return target{namespace: g}
	case "module":
		if t := a.resolveExport(g, "default", depth+1); t.fn != nil || t.members != nil {
			return t
		}
		return target{namespace: g}
	}
	return a.resolveExport(g, b.name, depth+1)
}

func (a *analyzer) resolveExport(g *jsFile, name string, depth int) target {
	if depth > maxDepth {
		return target{unknown: "bound-exceeded"}
	}
	if local, ok := g.exports[name]; ok {
		if local == name && name == "default" {
			if b := g.funcs["default"]; b != nil {
				return target{fn: b}
			}
			if m := g.members["default"]; m != nil {
				return target{members: m}
			}
		}
		if t := a.resolveName(g, local, depth+1); t.fn != nil || t.members != nil || t.pkg != "" || t.unknown != "" {
			return t
		}
	}
	whole := g.cjs // CommonJS: `module.exports = NAME`, or the object, function or instance assigned
	if whole == "" {
		whole = "module.exports"
	}
	t := a.resolveName(g, whole, depth+1)
	if name == "default" {
		if t.fn != nil || t.members != nil || t.pkg != "" || t.unknown != "" {
			return t
		}
		return target{unknown: "export-not-found"}
	}
	if t.members != nil && t.members[name] != nil {
		return a.member(g, t.members[name], depth+1)
	}
	return target{unknown: "export-not-found"}
}

// member resolves an object member: a body, or a reference to a top-level name.
func (a *analyzer) member(f *jsFile, b *body, depth int) target {
	if b.start >= 0 {
		return target{fn: b}
	}
	return a.resolveName(b.file, b.name, depth)
}

var httpMethods = map[string]bool{"get": true, "post": true, "put": true, "patch": true, "delete": true, "options": true, "head": true, "all": true}

// routeSteps finds the one registration of q's route in f and the `use` middleware registered
// before it on the same receiver (AMAP-V0-131).
func (a *analyzer) routeSteps(f *jsFile, q Query) ([]pending, error) {
	toks := f.toks
	type registration struct{ recv, open int }
	var found []registration
	for i := 0; i+4 < len(toks); i++ {
		if toks[i].Kind != appmap.JSIdent || (i > 0 && isPunct(toks[i-1], ".")) || !isPunct(toks[i+1], ".") ||
			!isPunct(toks[i+3], "(") || !strings.EqualFold(toks[i+2].Text, q.Method) || !httpMethods[toks[i+2].Text] {
			continue
		}
		if isString(toks[i+4]) && toks[i+4].Text == q.Path {
			found = append(found, registration{recv: i, open: i + 3})
		}
	}
	if len(found) == 0 {
		return nil, refuse("request-path-route-not-found", "no literal %s %s registration in %s", q.Method, q.Path, q.File)
	}
	if len(found) > 1 {
		return nil, refuse("request-path-route-ambiguous", "%d literal %s %s registrations in %s", len(found), q.Method, q.Path, q.File)
	}
	r := found[0]
	recv := toks[r.recv].Text
	a.receiver(f, recv, toks[r.recv].Line)
	steps := []pending{}
	for i := 0; i+3 < r.recv; i++ {
		if !isIdent(toks[i], recv) || (i > 0 && isPunct(toks[i-1], ".")) || !isPunct(toks[i+1], ".") || !isIdent(toks[i+2], "use") || !isPunct(toks[i+3], "(") {
			continue
		}
		args := splitArgs(toks, i+3)
		if len(args) > 0 && isString(toks[args[0][0]]) {
			prefix := toks[args[0][0]].Text
			if !(prefix == "/" || q.Path == prefix || strings.HasPrefix(q.Path, strings.TrimSuffix(prefix, "/")+"/")) {
				continue
			}
			args = args[1:]
		}
		for _, arg := range args {
			steps = append(steps, a.middleware(f, arg, "use")...)
		}
	}
	args := splitArgs(toks, r.open)[1:]
	for _, arg := range args {
		steps = append(steps, a.middleware(f, arg, "route")...)
	}
	if len(steps) == 0 || len(args) == 0 {
		return nil, refuse("request-path-route-not-found", "the %s %s registration in %s names no handler", q.Method, q.Path, q.File)
	}
	return steps, nil
}

// receiver records a MOUNT unknown unless the receiver is an application created by `express()`
// in this file: middleware a parent router or another file registers is not read.
func (a *analyzer) receiver(f *jsFile, recv string, line int) {
	if v := f.values[recv]; v != nil && v.start >= 0 && v.start < len(f.toks) {
		t := a.resolveName(f, f.toks[v.start].Text, 0)
		if t.pkg == "express" && t.pkgMember != "Router" && v.start+2 < len(f.toks) && isPunct(f.toks[v.start+1], "(") && isPunct(f.toks[v.start+2], ")") {
			return
		}
		if t.pkg == "express" {
			a.unknown(0, UnknownMount, "parent-router-not-read", f.path, line, recv)
			return
		}
	}
	a.unknown(0, UnknownMount, "receiver-origin-not-read", f.path, line, recv)
}

// splitArgs returns the token ranges [start, end) of the call arguments whose `(` is at open.
func splitArgs(toks []tok, open int) [][2]int {
	end := closeOf(toks, open)
	out := [][2]int{}
	start := open + 1
	depth := 0
	for j := open + 1; j <= end && j < len(toks); j++ {
		t := toks[j]
		if t.Kind == appmap.JSPunct {
			switch t.Text {
			case "(", "[", "{":
				depth++
			case ")", "]", "}":
				depth--
			}
			if (t.Text == "," && depth == 0) || j == end {
				if j > start {
					out = append(out, [2]int{start, j})
				}
				start = j + 1
			}
		}
	}
	return out
}

func (a *analyzer) middleware(f *jsFile, arg [2]int, registered string) []pending {
	toks := f.toks
	s, e := arg[0], arg[1]
	line := toks[s].Line
	text := tokenText(toks[s:e])
	if isPunct(toks[s], "[") {
		out := []pending{}
		for _, inner := range splitArgs(toks, s) {
			out = append(out, a.middleware(f, inner, registered)...)
		}
		return out
	}
	if b, ok := functionAt(f, s, "<inline>"); ok && b.end >= e {
		return []pending{{name: fmt.Sprintf("<inline@%d>", line), registered: registered, file: f, line: line, fn: b}}
	}
	names := []string{}
	j := s
	for j < e && toks[j].Kind == appmap.JSIdent {
		names = append(names, toks[j].Text)
		if j+1 < e && isPunct(toks[j+1], ".") {
			j += 2
			continue
		}
		j++
		break
	}
	p := pending{name: text, registered: registered, file: f, line: line}
	if len(names) == 0 || (j < e && !(isPunct(toks[j], "(") && closeOf(toks, j) == e-1)) {
		p.unknown = "middleware-expression-not-resolved"
		return []pending{p}
	}
	t := a.resolveName(f, names[0], 0)
	for _, m := range names[1:] {
		t = a.memberOf(f, t, m)
	}
	if j < e { // a factory call: its body and its argument tokens both run on the path
		p.args = &body{file: f, start: j, end: e, line: line}
	}
	switch {
	case t.fn != nil:
		p.fn = t.fn
	case t.pkg != "":
		p.pkg = t.pkg
	case t.unknown != "":
		p.unknown = t.unknown
	default:
		p.unknown = "middleware-not-resolved"
	}
	return []pending{p}
}

func (a *analyzer) memberOf(f *jsFile, t target, m string) target {
	switch {
	case t.namespace != nil:
		return a.resolveExport(t.namespace, m, 0)
	case t.members != nil:
		if b := t.members[m]; b != nil {
			return a.member(f, b, 0)
		}
		return target{unknown: "member-not-found"}
	case t.pkg != "":
		if t.pkgMember != "" && t.pkgMember != "default" && t.pkgMember != "module" && t.pkgMember != "*" {
			return t // `https.request.call(...)` stays a call of request
		}
		return target{pkg: t.pkg, pkgMember: m}
	case t.unknown != "":
		return t
	}
	return target{}
}

func tokenText(ts []tok) string {
	var b strings.Builder
	for i, t := range ts {
		if i > 0 && t.Kind == appmap.JSIdent && ts[i-1].Kind == appmap.JSIdent {
			b.WriteByte(' ')
		}
		if t.Kind == appmap.JSString {
			b.WriteString("'" + t.Text + "'")
		} else {
			b.WriteString(t.Text)
		}
		if b.Len() > 120 {
			return b.String()[:120] + "..."
		}
	}
	return b.String()
}

func (a *analyzer) unknown(step int, kind, reason, file string, line int, text string) {
	a.report.Unknowns = append(a.report.Unknowns, Unknown{Kind: kind, Reason: reason, Text: text, Step: step, Location: Location{File: file, Line: line}})
}

func (a *analyzer) finish() {
	r := a.report
	r.EnvKeys = sortedKeys(a.env)
	r.ConfigKeys = sortedKeys(a.config)
	for p, f := range a.files {
		if f != nil {
			r.FilesRead = append(r.FilesRead, p)
		}
	}
	sort.Strings(r.FilesRead)
	if a.bounded {
		a.unknown(0, UnknownBound, "bound-exceeded", r.File, 1, fmt.Sprintf("more than %d files or %d function visits", MaxFiles, maxVisits))
	}
	sort.SliceStable(r.Unknowns, func(i, j int) bool {
		x, y := r.Unknowns[i], r.Unknowns[j]
		if x.Step != y.Step {
			return x.Step < y.Step
		}
		if x.File != y.File {
			return x.File < y.File
		}
		return x.Line < y.Line
	})
	r.GuardOrder = guardOrder(r)
}

func sortedKeys(m map[string]*Key) []Key {
	out := make([]Key, 0, len(m))
	for _, k := range m {
		out = append(out, *k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// guardOrder flags each guard with an outbound call that runs before a later guard the reader
// shows makes no outbound call and has no unknowns (AMAP-V0-133).
func guardOrder(r *Report) []Finding {
	out := []Finding{}
	for i, s := range r.Steps {
		if s.Role != RoleGuard || s.Outbound != OutboundYes {
			continue
		}
		cheap := []int{}
		for _, later := range r.Steps[i+1:] {
			if later.Role == RoleGuard && later.Outbound == OutboundNoneFound && later.Unknowns == 0 {
				cheap = append(cheap, later.Order)
			}
		}
		if len(cheap) == 0 {
			continue
		}
		callees := []string{}
		seen := map[string]bool{}
		for _, c := range r.OutboundCalls {
			if c.Step == s.Order && !seen[c.Callee] {
				seen[c.Callee] = true
				callees = append(callees, c.Callee)
			}
		}
		out = append(out, Finding{Finding: FindingOutboundGuardFirst, OutboundGuard: s.Order, CheapGuards: cheap, StandInFor: callees,
			Message: fmt.Sprintf("guard %d (%s) makes an outbound call before guard(s) %v that make none: a test aimed at a later guard or the handler needs a stand-in for %s", s.Order, s.Name, cheap, strings.Join(callees, ", "))})
	}
	return out
}
