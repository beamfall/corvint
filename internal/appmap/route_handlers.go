package appmap

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// Route-action check (AMAP-V0-100..107): every route a supported Rails router compiles resolves
// to a controller action defined by the handler class, an ancestor, an included module or an
// implicit view template; anything that cannot be decided statically abstains with a reason.
const (
	ActionsSchema          = "application-map-actions/0"
	RuleRouteActionMissing = "ROUTE-ACTION-MISSING"

	maxAncestorChain = 16
)

var actionControllerTerminals = map[string]bool{"ActionController::Base": true, "ActionController::API": true, "ActionController::Metal": true}

// railsApp is the committed Ruby and view source under one application root at the map revision.
type railsApp struct {
	root     string
	entries  map[string]blobEntry
	data     map[string][]byte // by path
	tooLarge map[string]bool
	parsed   map[string]*rubyFile
}

func appJoin(root, p string) string {
	if root == "." || root == "" {
		return p
	}
	return root + "/" + p
}

// appRootOf is the application root of a router file: two directories above it.
func appRootOf(router string) string { return path.Dir(path.Dir(router)) }

// appReadBudget is one command's read allowance, shared by every application root it loads and
// by the map's test files (AMAP-V0-107, AMAP-V0-116).
type appReadBudget struct{ files, bytes int }

// newAppReadBudget is the per-command budget; tests narrow it.
var newAppReadBudget = func() *appReadBudget { return &appReadBudget{files: maxTestFiles, bytes: maxTestBytes} }

// spend charges files and bytes already read to the budget.
func (b *appReadBudget) spend(files, size int) error {
	b.files, b.bytes = b.files-files, b.bytes-size
	if b.files < 0 || b.bytes < 0 {
		return bound("the command's application reads exceed its shared file or byte budget")
	}
	return nil
}

// loadRailsApp lists every file under the prefixes and reads those read accepts, within the
// command's shared read budget (AMAP-V0-107); a file over 1 MiB is listed but not read.
func loadRailsApp(r repo, rev, root string, prefixes []string, read func(string) bool, budget *appReadBudget) (*railsApp, error) {
	a := &railsApp{root: root, entries: map[string]blobEntry{}, data: map[string][]byte{}, tooLarge: map[string]bool{}, parsed: map[string]*rubyFile{}}
	full := make([]string, len(prefixes))
	for i, p := range prefixes {
		full[i] = appJoin(root, p)
	}
	entries, err := r.tree(rev, full, budget.files)
	if err != nil {
		return nil, err
	}
	wanted, total := []blobEntry{}, 0
	for _, e := range entries {
		a.entries[e.path] = e
		if !read(e.path) {
			continue
		}
		if e.size > maxTestFileBytes {
			a.tooLarge[e.path] = true
			continue
		}
		wanted = append(wanted, e)
		total += e.size
	}
	if err := budget.spend(len(entries), total); err != nil {
		return nil, err
	}
	blobs, err := r.blobs(wanted, maxTestFileBytes, maxTestBytes)
	if err != nil {
		return nil, err
	}
	for _, e := range wanted {
		if b := blobs[e.oid]; bytes.IndexByte(b, 0) < 0 {
			a.data[e.path] = b
		} else {
			a.tooLarge[e.path] = true
		}
	}
	return a, nil
}

// ruby returns the parsed file at an application-relative path, or the reason it is unavailable.
func (a *railsApp) ruby(rel string) (*rubyFile, string) {
	p := appJoin(a.root, rel)
	if f := a.parsed[p]; f != nil {
		return f, ""
	}
	if a.tooLarge[p] {
		return nil, "handler-file-unreadable"
	}
	data, ok := a.data[p]
	if !ok {
		return nil, "handler-file-not-found"
	}
	f := parseRubyStructure(p, data)
	a.parsed[p] = f
	return f, ""
}

type actionVerdict struct {
	status string // defined, missing, abstained
	basis  string
	reason string
	at     string
	file   string
}

type chainUncertainty struct{ reason, at string }

// resolveAction decides whether ctrl#action is an action (AMAP-V0-102..105).
func (a *railsApp) resolveAction(ctrl, action string) actionVerdict {
	rel := "app/controllers/" + ctrl + "_controller.rb"
	v := actionVerdict{file: appJoin(a.root, rel)}
	f, why := a.ruby(rel)
	if why != "" {
		v.status, v.reason, v.at = "abstained", why, v.file
		return v
	}
	if f.broken != 0 {
		v.status, v.reason, v.at = "abstained", "unsupported-ruby-syntax", f.path+":"+strconv.Itoa(f.broken)
		return v
	}
	constant := controllerConstant(ctrl)
	scope := f.scope(constant)
	if scope == nil || scope.kind != "class" {
		v.status, v.reason, v.at = "abstained", "handler-class-not-found", v.file
		return v
	}
	var doubt *chainUncertainty
	note := func(reason, at string) {
		if doubt == nil {
			doubt = &chainUncertainty{reason, at}
		}
	}
	visited := map[string]bool{}
	members := 0
	var found *rubyDef
	var foundBasis, visibility string
	prefixes := []string{ctrl}
	// walk visits the ancestors in Ruby's lookup order: a class or module, then its includes
	// (the last include statement first), then a class's superclass. The first definition or
	// visibility change (`private :x`, `public :x`) of the action decides its visibility; the walk
	// continues past it so a later member's fallback (`action_missing`) or doubt is still seen,
	// and so each controller class contributes its view prefix (AMAP-V0-103..105). It reports
	// whether the chain beyond s is unknown.
	var walk func(s *rubyScope, sf *rubyFile, basis string) bool
	walk = func(s *rubyScope, sf *rubyFile, basis string) bool {
		if visited[s.name] && s.kind == "module" {
			return false // Ruby skips a module already in the chain
		}
		if visited[s.name] || members >= maxAncestorChain {
			note("ancestor-depth-exceeded", sf.path+":"+strconv.Itoa(s.line))
			return true
		}
		visited[s.name] = true
		members++
		if s.kind == "class" && basis == "ancestor" {
			if name, ok := strings.CutSuffix(s.name, "Controller"); ok && name != "" {
				prefixes = append(prefixes, rubyUnderscore(name))
			}
		}
		if found == nil {
			for i := len(s.defs) - 1; i >= 0; i-- { // a later definition replaces an earlier one
				if d := s.defs[i]; d.instance && d.name == action {
					found, foundBasis = d, basis
					break
				}
			}
			if v, ok := s.hidden[action]; ok && visibility == "" {
				visibility = v
			}
			if found != nil && visibility == "" {
				visibility = found.visibility
			}
		}
		for _, dyn := range s.dynamic {
			note("dynamic-definition", sf.path+":"+strconv.Itoa(dyn.line))
		}
		for i := len(s.includes) - 1; i >= 0; i-- {
			inc := s.includes[i]
			at := sf.path + ":" + strconv.Itoa(inc.line)
			if inc.name == "" {
				note("unresolved-module", at)
				continue
			}
			ms, mf, reason := a.constant(inc.name, s.nesting, []string{"app/controllers/concerns", "app/controllers", "app/models/concerns"})
			if ms == nil || ms.kind != "module" {
				if reason == "" {
					reason = "unresolved-module"
				}
				note(reason, at+" "+inc.name)
				continue
			}
			if walk(ms, mf, "module") {
				return true
			}
		}
		if s.kind != "class" || s.super == "" {
			return false
		}
		at := sf.path + ":" + strconv.Itoa(s.superLine)
		if actionControllerTerminals[strings.TrimPrefix(s.super, "::")] {
			return false
		}
		if s.super == "?" {
			note("unresolved-ancestor", at)
			return true
		}
		ps, pf, reason := a.constant(s.super, s.nesting, []string{"app/controllers"})
		if ps == nil || ps.kind != "class" {
			if reason == "" {
				reason = "unresolved-ancestor"
			}
			note(reason, at+" "+s.super)
			return true
		}
		return walk(ps, pf, "ancestor")
	}
	walk(scope, f, "method")
	if found != nil && visibility == "public" {
		v.status, v.basis = "defined", foundBasis
		return v
	}
	if a.template(prefixes, action) {
		v.status, v.basis = "defined", "implicit-template"
		return v
	}
	if doubt != nil {
		v.status, v.reason, v.at = "abstained", doubt.reason, doubt.at
		return v
	}
	v.status = "missing"
	if found != nil {
		v.basis = "non-public-method"
	}
	return v
}

// constant resolves a constant reference lexically (innermost nesting first, then top level)
// through the Zeitwerk file convention under the given directories (AMAP-V0-103).
func (a *railsApp) constant(name string, nesting []string, dirs []string) (*rubyScope, *rubyFile, string) {
	candidates := []string{}
	if strings.HasPrefix(name, "::") {
		candidates = append(candidates, strings.TrimPrefix(name, "::"))
	} else {
		for i := 1; i < len(nesting); i++ {
			candidates = append(candidates, nesting[i]+"::"+name)
		}
		candidates = append(candidates, name)
	}
	reason := ""
	for _, c := range candidates {
		for _, dir := range dirs {
			f, why := a.ruby(dir + "/" + rubyUnderscore(c) + ".rb")
			if why == "handler-file-unreadable" {
				reason = why
			}
			if f == nil {
				continue
			}
			if f.broken != 0 {
				reason = "unsupported-ruby-syntax"
				continue
			}
			if s := f.scope(c); s != nil {
				return s, f, ""
			}
		}
	}
	return nil, nil, reason
}

// template reports whether app/views/<prefix>/<action>.* exists for any of the controller's view
// prefixes: its own path, then each ancestor controller's (AMAP-V0-104).
func (a *railsApp) template(prefixes []string, action string) bool {
	for _, vp := range prefixes {
		prefix := appJoin(a.root, "app/views/"+vp+"/"+action+".")
		for p := range a.entries {
			if strings.HasPrefix(p, prefix) && !strings.Contains(p[len(prefix):], "/") {
				return true
			}
		}
	}
	return false
}

// readManifestAt reads the map's manifest anchor at the map's revision of the root holding it:
// --root, or the aliased root repos binds (SKH-V0-329); a nil repos means the caller binds no
// aliased root (actions, blocker, branches, plan and the corpus MCP tools). A changed manifest refuses (AMAP-V0-100).
func readManifestAt(r repo, m *Map, repos map[string]string) (Manifest, error) {
	revErr := &gokernel.Error{Code: "appmap-invalid-revision", Message: "the map manifest is not readable from --root at the map revision"}
	if !safeRelative(m.Manifest.Path) {
		return Manifest{}, revErr
	}
	rev := m.Revision
	if alias := m.Manifest.Repo; alias != "" {
		if repos == nil {
			return Manifest{}, rootUnavailable(alias, "holds the map manifest, and this caller reads --root only")
		}
		ar, arev, err := mapRoot(r.ctx, m, alias, Options{Root: r.root, Repos: repos})
		if err != nil {
			return Manifest{}, err
		}
		r, rev = ar, arev
		revErr = &gokernel.Error{Code: "appmap-invalid-revision", Message: "the map manifest is not readable from root " + alias + " at the map's revision of it"}
	}
	e, raw, err := r.readOne(rev, m.Manifest.Path, maxRouterBytes)
	if err != nil || e.oid != m.Manifest.Blob {
		return Manifest{}, revErr
	}
	return decodeManifest(raw)
}

// compiledRoutes compiles every rails-routes/0 router of the manifest and reports the other
// routers as not applicable.
type compiledRoutes struct {
	routes     []railsRoute
	abstain    []routeNote
	skipped    []routeNote
	opaque     []opaqueRoute
	roots      map[string][]int // app root -> route indexes
	checked    []string
	notChecked []actionNotApplicable
}

func compileRailsRoutes(r repo, rev string, man Manifest) (*compiledRoutes, error) {
	out := &compiledRoutes{roots: map[string][]int{}, checked: []string{}, notChecked: []actionNotApplicable{}}
	remaining := maxRouterFiles - len(man.Routers)
	files := []string{}
	for _, rf := range man.Routers {
		if rf.Dialect != DialectRails {
			out.notChecked = append(out.notChecked, actionNotApplicable{Declaration: rf.Path, Reason: "no-server-handler"})
			continue
		}
		files = append(files, rf.Path)
	}
	reads, err := readRailsRouters(r, rev, files, &remaining)
	if err != nil {
		return nil, err
	}
	for _, rd := range reads {
		if rd.peerDrawn {
			out.checked = append(out.checked, rd.path)
			continue
		}
		p, root := rd.p, rd.root
		if !rd.rooted {
			root = appRootOf(rd.path)
		}
		for _, rt := range p.routes {
			out.roots[root] = append(out.roots[root], len(out.routes))
			out.routes = append(out.routes, rt)
		}
		out.abstain = append(out.abstain, p.abstain...)
		out.skipped = append(out.skipped, p.skipped...)
		out.opaque = append(out.opaque, p.opaque...)
		out.checked = append(out.checked, rd.path)
		if len(out.routes) > maxStates {
			return nil, bound(fmt.Sprintf("more than %d compiled routes", maxStates))
		}
	}
	return out, nil
}

type actionFinding struct {
	Rule        string `json:"rule"`
	Declaration string `json:"declaration"`
	Verb        string `json:"verb"`
	Path        string `json:"path"`
	Controller  string `json:"controller"`
	HandlerFile string `json:"handler_file"`
	Action      string `json:"action"`
	Basis       string `json:"basis,omitempty"`
}

type actionAbstention struct {
	Declaration string `json:"declaration"`
	Controller  string `json:"controller,omitempty"`
	Action      string `json:"action,omitempty"`
	Reason      string `json:"reason"`
	At          string `json:"at"`
}

type actionNotApplicable struct {
	Declaration string `json:"declaration"`
	Reason      string `json:"reason"`
}

// ActionsReport is the route-action document and its finding count, which decides the exit
// status (AMAP-V0-106).
type ActionsReport struct {
	Document []byte
	Findings int
}

// CheckActions runs the route-action check (AMAP-V0-100..107). It reads committed objects of
// --root at the map revision only and writes nothing.
func CheckActions(ctx context.Context, m *Map, o Options) (*ActionsReport, error) {
	budget, err := o.budget(DefaultFlowBudget)
	if err != nil {
		return nil, err
	}
	r := repo{ctx: ctx, root: o.Root}
	rev, err := r.resolve(m.Revision)
	if err != nil || rev != m.Revision {
		return nil, orPlatform(err, &gokernel.Error{Code: "appmap-invalid-revision", Message: "the map revision is not a commit in --root"})
	}
	man, err := readManifestAt(r, m, nil)
	if err != nil {
		return nil, err
	}
	compiled, err := compileRailsRoutes(r, rev, man)
	if err != nil {
		return nil, err
	}
	findings, abstentions := []any{}, []any{}
	defined := 0
	for _, n := range compiled.abstain {
		abstentions = append(abstentions, actionAbstention{Declaration: n.file + ":" + strconv.Itoa(n.line), Reason: n.reason, At: n.file + ":" + strconv.Itoa(n.line)})
	}
	roots := make([]string, 0, len(compiled.roots))
	for root := range compiled.roots {
		roots = append(roots, root)
	}
	sort.Strings(roots)
	reads := newAppReadBudget()
	for _, root := range roots {
		app, err := loadRailsApp(r, rev, root, []string{"app"}, func(p string) bool {
			return strings.HasSuffix(p, ".rb") && (strings.HasPrefix(p, appJoin(root, "app/controllers/")) || strings.HasPrefix(p, appJoin(root, "app/models/concerns/")))
		}, reads)
		if err != nil {
			return nil, err
		}
		for _, i := range compiled.roots[root] {
			rt := compiled.routes[i]
			v := app.resolveAction(rt.ctrl, rt.action)
			switch v.status {
			case "defined":
				defined++
			case "missing":
				findings = append(findings, actionFinding{Rule: RuleRouteActionMissing, Declaration: rt.declaration(), Verb: rt.verb, Path: rt.path,
					Controller: controllerConstant(rt.ctrl), HandlerFile: v.file, Action: rt.action, Basis: v.basis})
			default:
				abstentions = append(abstentions, actionAbstention{Declaration: rt.declaration(), Controller: controllerConstant(rt.ctrl), Action: rt.action, Reason: v.reason, At: v.at})
			}
		}
	}
	notApplicable := []any{}
	for _, n := range compiled.notChecked {
		notApplicable = append(notApplicable, n)
	}
	for _, n := range compiled.skipped {
		notApplicable = append(notApplicable, actionNotApplicable{Declaration: n.file + ":" + strconv.Itoa(n.line), Reason: n.reason})
	}
	head := []field{{"schema", ActionsSchema}, {"app", m.App}, {"map_revision", m.Revision}, {"map_digest", m.Digest},
		{"budget", budget}, {"full", o.Full}, {"rule", RuleRouteActionMissing}, {"routers_checked", compiled.checked},
		{"routers_not_checked", len(compiled.notChecked)}, {"routes", len(compiled.routes)}, {"defined", defined},
		{"missing", len(findings)}, {"abstained", len(abstentions)}, {"not_applicable_total", len(notApplicable)}}
	doc, err := render(head, []section{{"findings", findings}, {"abstentions", abstentions}, {"not_applicable", notApplicable}}, budget)
	if err != nil {
		return nil, err
	}
	return &ActionsReport{Document: doc, Findings: len(findings)}, nil
}
