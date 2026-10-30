package appmap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// Endpoint join (AMAP-V0-212..214): server endpoints read from Rails route files and JSON OpenAPI
// documents, client HTTP call sites read from declared source, and the join that places each
// endpoint on the screens whose source calls it. Nothing is guessed: a call whose URL or verb is not
// read, a file the reference hop cannot place, and a route the parser abstains on are unknown with
// a reason.

// EndpointsSchema names the endpoint join report (AMAP-V0-214).
const EndpointsSchema = "application-map-endpoints/0"

// EndpointsLimits is printed in every report (AMAP-V0-214).
const EndpointsLimits = "endpoints come from the rails-routes/0 files named with --rails-routes and the OpenAPI 2/3 documents (JSON, or block-style YAML) named with --openapi; " +
	"calls are $http, axios and fetch with a literal, template or `+`-joined URL in --source script files; a substitution may spell one or more whole path segments; " +
	"an axios instance created in the same file with a literal or absent baseURL prefixes its calls with that base, and an AngularJS $resource with a literal url template yields its default and literal declared actions; " +
	"JSX in .js, .jsx and .tsx files is read for its expression containers; " +
	"request interceptors, axios.defaults and other clients (instances with a non-literal baseURL or created elsewhere, resource factories without $resource, generated clients) are not read and are reported unknown; " +
	"a call's endpoints are every declared endpoint it may reach; its screens are those its file reaches through the reference hop"

const (
	maxEndpointDocs = 16
	maxOpenAPIBytes = 8 << 20
)

// EndpointsOptions names the server declarations and the client source an endpoint join reads, from
// --root at the map revision, or as ALIAS:PATH from the root the map pinned under ALIAS at the
// revision the map recorded for it (AMAP-V0-222).
type EndpointsOptions struct {
	RailsRoutes []string
	OpenAPI     []string
	Sources     []string
	// absent are --root routers the impact change adds or deletes and the map revision lacks;
	// the join reads them as declaring no endpoints (AMAP-V0-355).
	absent map[string]bool
}

type endpointRow struct {
	ID          string   `json:"id"`
	Repo        string   `json:"repo,omitempty"`
	Verb        string   `json:"verb"`
	Path        string   `json:"path"`
	Source      string   `json:"source"`
	Declaration string   `json:"declaration"`
	Handler     string   `json:"handler,omitempty"`
	Controller  string   `json:"controller,omitempty"`
	Operation   string   `json:"operation_id,omitempty"`
	Calls       []string `json:"calls"`
	Screens     []string `json:"screens"`
	segs        []string
	format      bool
	guard       string // a Rails route's constraints and defaults (AMAP-V0-355)
}

type endpointCall struct {
	Repo      string   `json:"repo,omitempty"`
	At        string   `json:"at"`
	Client    string   `json:"client"`
	Verb      string   `json:"verb"`
	URL       string   `json:"url"`
	Endpoints []string `json:"endpoints"`
	Screens   []string `json:"screens"`
	Placement string   `json:"placement,omitempty"`
	segs      []string
	base      string // an axios instance's literal baseURL, joined to each relative URL (AMAP-V0-358)
}

type endpointUnknown struct {
	Repo   string `json:"repo,omitempty"`
	At     string `json:"at"`
	Reason string `json:"reason"`
	Detail string `json:"detail,omitempty"`
}

// Endpoints joins server endpoints to client calls and screens (AMAP-V0-212..214). It reads every
// input from --root at the map revision and writes nothing.
func Endpoints(ctx context.Context, m *Map, eo EndpointsOptions, o Options) ([]byte, error) {
	budget, err := o.budget(DefaultFlowBudget)
	if err != nil {
		return nil, err
	}
	j, err := joinEndpoints(ctx, m, eo, o)
	if err != nil {
		return nil, err
	}
	endpoints, calls, unknown := j.endpoints, j.calls, j.unknown
	head := []field{{"schema", EndpointsSchema}, {"app", m.App}, {"map_revision", m.Revision}, {"map_digest", m.Digest},
		{"budget", budget}, {"full", o.Full}, {"limits", EndpointsLimits}, {"rails_routes", nonNil(eo.RailsRoutes)},
		{"openapi", nonNil(eo.OpenAPI)}, {"sources", eo.Sources}, {"source_files", j.scripts}, {"source_files_skipped", j.skipped},
		{"endpoints_total", len(endpoints)}, {"calls_total", len(calls)}, {"calls_unmatched", j.unmatched},
		{"unknown_total", len(unknown)}, {"complete", len(unknown) == 0}}
	endpointItems := make([]any, len(endpoints))
	for i, e := range endpoints {
		endpointItems[i] = e
	}
	callItems := make([]any, len(calls))
	for i, c := range calls {
		callItems[i] = c
	}
	unknownItems := make([]any, len(unknown))
	for i, u := range unknown {
		unknownItems[i] = u
	}
	return render(head, []section{{"unknown", unknownItems}, {"endpoints", endpointItems}, {"calls", callItems}}, budget)
}

// endpointJoin is one endpoint join: the endpoints, the calls, every unknown, and the source
// file counts.
type endpointJoin struct {
	endpoints                   []*endpointRow
	calls                       []*endpointCall
	unknown                     []endpointUnknown
	scripts, skipped, unmatched int
}

// joinEndpoints reads the declarations and the client source and joins them (AMAP-V0-212..214).
func joinEndpoints(ctx context.Context, m *Map, eo EndpointsOptions, o Options) (*endpointJoin, error) {
	query := func(msg string) error { return &gokernel.Error{Code: "appmap-invalid-query", Message: msg} }
	if len(eo.RailsRoutes)+len(eo.OpenAPI) == 0 || len(eo.RailsRoutes)+len(eo.OpenAPI) > maxEndpointDocs {
		return nil, query(fmt.Sprintf("endpoints requires 1 to %d --rails-routes or --openapi files", maxEndpointDocs))
	}
	if len(eo.Sources) == 0 || len(eo.Sources) > maxRefSources {
		return nil, query(fmt.Sprintf("endpoints requires 1 to %d --source directories", maxRefSources))
	}
	seen := map[string]bool{}
	for _, p := range append(append(append([]string{}, eo.RailsRoutes...), eo.OpenAPI...), eo.Sources...) {
		if _, rel := splitSource(p); !safeRelative(rel) || seen[p] {
			return nil, query(fmt.Sprintf("%q is not a unique repository-relative path of --root or ALIAS:PATH of a root the map read", p))
		}
		seen[p] = true
	}
	roots, err := openSources(ctx, m, eo.Sources, o)
	if err != nil {
		return nil, err
	}
	// Each root is opened once, at the map's revision of it (AMAP-V0-222).
	opened := map[string]*sourceRoot{}
	for _, sr := range roots {
		opened[sr.alias] = sr
	}
	open := func(alias string) (*sourceRoot, error) {
		if sr := opened[alias]; sr != nil {
			return sr, nil
		}
		r, rev, err := mapRoot(ctx, m, alias, o)
		if err != nil {
			return nil, err
		}
		opened[alias] = &sourceRoot{alias: alias, r: r, rev: rev}
		return opened[alias], nil
	}
	unknown, endpoints := []endpointUnknown{}, []*endpointRow{}
	routesByRoot, routeRoots := map[string][]string{}, []string{}
	for _, p := range eo.RailsRoutes {
		alias, file := splitSource(p)
		if eo.absent[p] {
			continue
		}
		if routesByRoot[alias] == nil {
			routeRoots = append(routeRoots, alias)
		}
		routesByRoot[alias] = append(routesByRoot[alias], file)
	}
	sort.Strings(routeRoots)
	routerFiles := maxRouterFiles - len(eo.RailsRoutes) // one draw allowance across every root (AMAP-V0-042)
	for _, alias := range routeRoots {
		sr, err := open(alias)
		if err != nil {
			return nil, err
		}
		rows, notes, err := railsEndpoints(sr.r, sr.rev, routesByRoot[alias], &routerFiles)
		if err != nil {
			return nil, err
		}
		for _, e := range rows {
			e.Repo = alias
		}
		for i := range notes {
			notes[i].Repo = alias
		}
		endpoints, unknown = append(endpoints, rows...), append(unknown, notes...)
	}
	for _, p := range eo.OpenAPI {
		alias, file := splitSource(p)
		sr, err := open(alias)
		if err != nil {
			return nil, err
		}
		_, raw, err := sr.r.readOne(sr.rev, file, maxOpenAPIBytes)
		if err != nil {
			return nil, err
		}
		rows, note, detail := openAPIEndpoints(file, raw)
		if note != "" {
			unknown = append(unknown, endpointUnknown{Repo: alias, At: file, Reason: note, Detail: detail})
			continue
		}
		for _, e := range rows {
			e.Repo = alias
		}
		endpoints = append(endpoints, rows...)
	}
	if len(endpoints) > maxStates {
		return nil, bound(fmt.Sprintf("more than %d endpoints", maxStates))
	}
	sort.SliceStable(endpoints, func(i, j int) bool {
		a, b := endpoints[i], endpoints[j]
		if a.Verb+" "+a.Path != b.Verb+" "+b.Path {
			return a.Verb+" "+a.Path < b.Verb+" "+b.Path
		}
		if a.Repo != b.Repo {
			return a.Repo < b.Repo
		}
		return a.Source+a.Declaration < b.Source+b.Declaration
	})
	ids := map[string]int{}
	for _, e := range endpoints {
		id := e.Verb + " " + strings.Join(e.segs, "/")
		if ids[id]++; ids[id] > 1 {
			id += "#" + strconv.Itoa(ids[id])
		}
		e.ID = id
	}

	entries, err := sourceEntries(roots, maxTestFiles)
	if err != nil {
		return nil, err
	}
	scripts, skipped, total := []blobEntry{}, 0, 0
	for _, e := range entries {
		switch {
		case !endpointScript(e.path):
		case e.size > maxTestFileBytes:
			skipped++
			unknown = append(unknown, endpointUnknown{Repo: e.repo, At: e.path, Reason: "source-over-bound", Detail: strconv.Itoa(maxTestFileBytes)})
		default:
			scripts = append(scripts, e)
			total += e.size
		}
	}
	if total > maxTestBytes {
		return nil, bound(fmt.Sprintf("declared source files exceed %d bytes", maxTestBytes))
	}
	data := map[string][]byte{}
	for _, sr := range roots {
		mine := []blobEntry{}
		for _, e := range scripts {
			if e.repo == sr.alias {
				mine = append(mine, e)
			}
		}
		got, err := sr.r.blobs(mine, maxTestFileBytes, maxTestBytes)
		if err != nil {
			return nil, err
		}
		for oid, b := range got {
			data[oid] = b
		}
	}
	calls := []*endpointCall{}
	for _, e := range scripts {
		found, notes := scanCalls(e.path, data[e.oid])
		for _, c := range found {
			c.Repo = e.repo
		}
		for i := range notes {
			notes[i].Repo = e.repo
		}
		calls = append(calls, found...)
		unknown = append(unknown, notes...)
	}
	r, rev := opened[""], ""
	if r != nil {
		rev = r.rev
	}

	// Place each calling file on screens: a router anchor directly, any other file through the
	// reference hop impact uses (ANL-V0-032); a file the hop cannot place is unknown. Screens are
	// anchored in --root and the hop reads --root alone, so a calling file in an aliased root is
	// never placed by path: it is unknown (AMAP-V0-222). One path may be read from two roots, so
	// placement is keyed by the root-qualified file.
	anchored := map[string][]string{}
	for _, s := range m.Screens {
		if s.Anchor.Repo == "" {
			anchored[s.Anchor.Path] = append(anchored[s.Anchor.Path], s.ID)
		}
	}
	placed, why := map[string][]string{}, map[string]string{}
	var g *referenceGraph
	graphDetail, graphRead := "", false
	place := func(alias, file string) {
		key := rootRef(alias, file)
		if _, done := placed[key]; done || why[key] != "" {
			return
		}
		if alias != "" {
			why[key] = "aliased-root-caller-not-placed"
			unknown = append(unknown, endpointUnknown{Repo: alias, At: file, Reason: why[key],
				Detail: "screens are anchored in --root and the reference hop does not cross roots"})
			return
		}
		if ids := anchored[file]; len(ids) > 0 {
			placed[file] = ids
			return
		}
		if !graphRead {
			g, graphDetail = loadReferenceGraph(r.r, rev)
			graphRead = true
		}
		if g == nil {
			why[file] = "reference-graph-bound"
			unknown = append(unknown, endpointUnknown{At: file, Reason: why[file], Detail: graphDetail})
			return
		}
		targets, reason, detail := templateHop(m, g, file)
		if reason != "" {
			why[file] = reason
			unknown = append(unknown, endpointUnknown{At: file, Reason: reason, Detail: detail})
			return
		}
		set := map[string]bool{}
		for _, t := range targets {
			for _, id := range t.screens {
				set[id] = true
			}
		}
		placed[file] = sortedKeys(set)
	}
	byID := map[string]*endpointRow{}
	for _, e := range endpoints {
		byID[e.ID] = e
	}
	unmatched := 0
	for _, c := range calls {
		file, _, _ := strings.Cut(c.At, ":")
		for _, e := range endpoints {
			if e.reaches(c.Verb, c.segs) {
				c.Endpoints = append(c.Endpoints, e.ID)
			}
		}
		place(c.Repo, file)
		key := rootRef(c.Repo, file)
		c.Screens, c.Placement = placed[key], why[key]
		if c.Screens == nil {
			c.Screens = []string{}
		}
		if len(c.Endpoints) == 0 {
			c.Endpoints = []string{}
			unmatched++
			unknown = append(unknown, endpointUnknown{Repo: c.Repo, At: c.At, Reason: "no-declared-endpoint", Detail: c.Verb + " " + c.URL})
		}
		for _, id := range c.Endpoints {
			e := byID[id]
			e.Calls = append(e.Calls, rootRef(c.Repo, c.At))
			e.Screens = append(e.Screens, c.Screens...)
		}
	}
	for _, e := range endpoints {
		set := map[string]bool{}
		for _, s := range e.Screens {
			set[s] = true
		}
		e.Screens = sortedKeys(set)
	}
	sort.SliceStable(unknown, func(i, j int) bool {
		if unknown[i].Repo != unknown[j].Repo {
			return unknown[i].Repo < unknown[j].Repo
		}
		if unknown[i].At != unknown[j].At {
			return unknown[i].At < unknown[j].At
		}
		return unknown[i].Reason < unknown[j].Reason
	})
	return &endpointJoin{endpoints: endpoints, calls: calls, unknown: unknown, scripts: len(scripts), skipped: skipped, unmatched: unmatched}, nil
}

// rootRef names at in the root it was read from: ALIAS:at for an aliased root, at for --root, the
// --source spelling (AMAP-V0-222).
func rootRef(alias, at string) string {
	if alias == "" {
		return at
	}
	return alias + ":" + at
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// endpointScript reports whether a source file is one the call scanner lexes.
func endpointScript(p string) bool {
	switch path.Ext(p) {
	case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs":
		return !strings.HasSuffix(p, ".d.ts")
	}
	return false
}

// railsEndpoints compiles each --rails-routes file (and its draws) to endpoints with their
// handler and the conventional controller file under the file's Rails application root; routes
// the parser abstains on, skips or cannot bound are unknown (AMAP-V0-212, AMAP-V0-353,
// AMAP-V0-354).
func railsEndpoints(r repo, rev string, files []string, remaining *int) ([]*endpointRow, []endpointUnknown, error) {
	out, notes := []*endpointRow{}, []endpointUnknown{}
	reads, err := readRailsRouters(r, rev, files, remaining)
	if err != nil {
		return nil, nil, err
	}
	for _, rd := range reads {
		if rd.peerDrawn {
			continue
		}
		p := rd.p
		if !rd.rooted {
			notes = append(notes, endpointUnknown{At: rd.path + ":1", Reason: "rails-app-root-unresolved",
				Detail: "no config/routes.rb or config/application.rb at or above " + path.Dir(rd.path)})
		}
		for _, rt := range p.routes {
			segs, ok := templateSegments(rt.path, ':', '*')
			if !ok {
				notes = append(notes, endpointUnknown{At: rt.declaration(), Reason: "unsupported-route-pattern", Detail: rt.path})
				continue
			}
			row := &endpointRow{Verb: rt.verb, Path: rt.path, Source: "rails", Declaration: rt.declaration(),
				Handler: rt.ctrl + "#" + rt.action, Calls: []string{}, segs: segs, format: true, guard: rt.guard}
			if rd.rooted {
				row.Controller = appJoin(rd.root, "app/controllers/"+rt.ctrl+"_controller.rb")
			}
			out = append(out, row)
		}
		for _, n := range append(append([]routeNote{}, p.abstain...), p.skipped...) {
			notes = append(notes, endpointUnknown{At: n.file + ":" + strconv.Itoa(n.line), Reason: n.reason})
		}
		for _, o := range p.opaque {
			notes = append(notes, endpointUnknown{At: o.file + ":" + strconv.Itoa(o.line), Reason: o.reason, Detail: strings.TrimSpace(o.verb + " " + o.path)})
		}
	}
	return out, notes, nil
}

// templateSegments normalizes a route template to segments: a literal, `{}` for a parameter and
// `{*}` for a trailing splat. param and splat are the dialect's markers (Rails `:id`, `*rest`);
// OpenAPI passes 0 and reads `{name}`.
func templateSegments(p string, param, splat byte) ([]string, bool) {
	if !strings.HasPrefix(p, "/") {
		return nil, false
	}
	parts := strings.Split(strings.TrimSuffix(p[1:], "/"), "/")
	if p == "/" {
		parts = nil
	}
	segs := []string{""}
	for i, s := range parts {
		switch {
		case s == "":
			return nil, false
		case param != 0 && s[0] == param, param == 0 && strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") && !strings.ContainsAny(s[1:len(s)-1], "{}"):
			segs = append(segs, "{}")
		case splat != 0 && s[0] == splat:
			if i != len(parts)-1 {
				return nil, false
			}
			segs = append(segs, "{*}")
		case strings.ContainsAny(s, "{}*:()?#"):
			return nil, false
		default:
			segs = append(segs, s)
		}
	}
	return segs, true
}

var openAPIMethods = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}

// openAPIEndpoints reads an OpenAPI 3 or Swagger 2 document's paths and methods, JSON or the YAML
// subset of AMAP-V0-219. A document it cannot read whole (YAML outside the subset, no paths, servers
// with different path prefixes) yields no endpoint, the reason and a detail (AMAP-V0-212).
func openAPIEndpoints(file string, raw []byte) ([]*endpointRow, string, string) {
	if t := bytes.TrimSpace(raw); len(t) == 0 || t[0] != '{' {
		converted, refusal := yamlToJSON(raw)
		if refusal != "" {
			return nil, "openapi-yaml-unsupported", refusal
		}
		raw = converted
	}
	var doc struct {
		Swagger  string                                `json:"swagger"`
		OpenAPI  string                                `json:"openapi"`
		BasePath string                                `json:"basePath"`
		Servers  []struct{ URL string }                `json:"servers"`
		Paths    map[string]map[string]json.RawMessage `json:"paths"`
	}
	if json.Unmarshal(raw, &doc) != nil || (doc.Swagger == "") == (doc.OpenAPI == "") || doc.Paths == nil {
		return nil, "openapi-unreadable", ""
	}
	prefix := strings.TrimSuffix(doc.BasePath, "/")
	for i, s := range doc.Servers {
		u := s.URL
		if k := strings.Index(u, "://"); k >= 0 {
			u = u[k+3:]
			if j := strings.IndexByte(u, '/'); j >= 0 {
				u = u[j:]
			} else {
				u = ""
			}
		}
		u = strings.TrimSuffix(u, "/")
		if strings.ContainsAny(u, "{}") || (u != "" && !strings.HasPrefix(u, "/")) {
			return nil, "openapi-server-url-not-literal", ""
		}
		if i > 0 && u != prefix {
			return nil, "openapi-servers-differ", ""
		}
		prefix = u
	}
	keys := make([]string, 0, len(doc.Paths))
	for k := range doc.Paths {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := []*endpointRow{}
	for _, k := range keys {
		if _, ref := doc.Paths[k]["$ref"]; ref {
			// A referenced path item's operations are not read, so the document is refused whole
			// rather than reported complete without them (AMAP-V0-221).
			return nil, "openapi-path-ref-not-read", k
		}
		segs, ok := templateSegments(prefix+k, 0, 0)
		if !ok {
			return nil, "openapi-path-not-template", ""
		}
		for _, method := range openAPIMethods {
			op, has := doc.Paths[k][method]
			if !has {
				continue
			}
			var meta struct {
				OperationID string `json:"operationId"`
			}
			_ = json.Unmarshal(op, &meta)
			out = append(out, &endpointRow{Verb: strings.ToUpper(method), Path: prefix + k, Source: "openapi",
				Declaration: file + "#/paths/" + strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1") + "/" + method,
				Operation:   meta.OperationID, Calls: []string{}, segs: segs})
		}
	}
	return out, "", ""
}

// reaches reports whether a call with verb and URL segments may reach the endpoint: the verb
// matches (any for a call whose verb is not read, any for a Rails `match` without `via`, and HEAD
// for a Rails GET) and every segment may be equal, a dynamic call segment reaching any endpoint
// segment. A Rails endpoint also takes a trailing `.format` on the last literal segment.
func (e *endpointRow) reaches(verb string, segs []string) bool {
	if verb != "*" && e.Verb != "ANY" {
		ok := false
		for _, v := range strings.Split(e.Verb, "|") {
			ok = ok || v == verb || (e.format && verb == "HEAD" && v == "GET")
		}
		if !ok {
			return false
		}
	}
	if segmentsReach(e.segs, segs) {
		return true
	}
	last := len(segs) - 1
	if !e.format || last < 1 || strings.Contains(segs[last], substMark) {
		return false
	}
	dot := strings.LastIndexByte(segs[last], '.')
	if dot <= 0 || !routeLiteralName.MatchString(segs[last][dot+1:]) {
		return false
	}
	trimmed := append(append([]string{}, segs[:last]...), segs[last][:dot])
	return segmentsReach(e.segs, trimmed)
}

func segmentsReach(tmpl, segs []string) bool {
	memo := map[[2]int]bool{}
	var reach func(i, j int) bool
	reach = func(i, j int) bool {
		if i == len(tmpl) {
			return j == len(segs)
		}
		if tmpl[i] == "{*}" {
			return len(segs) > j
		}
		if j >= len(segs) {
			return false
		}
		key := [2]int{i, j}
		if v, ok := memo[key]; ok {
			return v
		}
		ok := false
		t, s := tmpl[i], segs[j]
		if !strings.Contains(s, substMark) {
			ok = (t == s || t == "{}" && s != "" && !strings.Contains(s, ".")) && reach(i+1, j+1)
		} else {
			// A substitution may spell one or more segments (AMAP-V0-217); the literal text
			// before it must fit the first template segment it covers and the text after it the
			// last one.
			pre, post, _ := strings.Cut(s, substMark)
			post = post[strings.LastIndex(post, substMark)+1:]
			fits := func(t string, prefix bool) bool {
				if t == "{}" {
					return true
				}
				if prefix {
					return strings.HasPrefix(t, pre)
				}
				return strings.HasSuffix(t, post)
			}
			if t == "{}" || (strings.HasPrefix(t, pre) && strings.HasSuffix(t, post) && len(pre)+len(post) <= len(t)) {
				ok = reach(i+1, j+1)
			}
			for k := i + 1; !ok && k < len(tmpl) && fits(t, true); k++ {
				if tmpl[k] == "{*}" {
					ok = true
					break
				}
				ok = fits(tmpl[k], false) && reach(k+1, j+1)
			}
		}
		memo[key] = ok
		return ok
	}
	return reach(0, 0)
}

var callVerbs = map[string]string{"get": "GET", "post": "POST", "put": "PUT", "patch": "PATCH", "delete": "DELETE",
	"head": "HEAD", "options": "OPTIONS", "jsonp": "GET", "postForm": "POST", "putForm": "PUT", "patchForm": "PATCH"}

// clientConfig is the client API surface that is not a request: property reads and helpers.
var clientConfig = map[string]bool{"defaults": true, "interceptors": true, "pendingRequests": true, "isCancel": true,
	"CancelToken": true, "isAxiosError": true, "all": true, "spread": true, "Cancel": true, "AxiosError": true,
	"toFormData": true, "formToJSON": true, "getAdapter": true, "mergeConfig": true, "VERSION": true, "HttpStatusCode": true}

// scanCalls reads the $http, axios and fetch call sites of one script (AMAP-V0-213). A client name
// used any other way than a recognised call, a parameter, an import binding or an object key is an
// unknown, as is a call whose URL or configuration is not read.
func scanCalls(file string, src []byte) ([]*endpointCall, []endpointUnknown) {
	text := string(src)
	if marker := generatedClientMarker(text); marker != "" {
		return nil, []endpointUnknown{{At: file, Reason: "generated-client-not-modelled", Detail: marker}}
	}
	if !strings.Contains(text, "$http") && !strings.Contains(text, "axios") && !strings.Contains(text, "fetch") && !strings.Contains(text, "$resource") {
		return nil, nil
	}
	calls, notes := []*endpointCall{}, []endpointUnknown{}
	var toks []token
	if jsxScript(file) {
		toks = lexJSX(text)
	} else {
		toks, _ = lexJS(text)
	}
	if len(toks) > 0 && toks[0].unsure {
		return nil, []endpointUnknown{{At: file, Reason: "source-not-lexed"}}
	}
	note := func(line int, reason, detail string) {
		notes = append(notes, endpointUnknown{At: file + ":" + strconv.Itoa(line), Reason: reason, Detail: detail})
	}
	instances := axiosInstances(toks, note)
	for i, t := range toks {
		if t.kind != tokIdent {
			continue
		}
		if t.text == "$httpProvider" && next(toks, i+1, ".") && word(toks, i+2, "interceptors") {
			note(t.line, "request-interceptor-not-modelled", "$httpProvider.interceptors")
			continue
		}
		client, base := "", ""
		switch inst := instances[t.text]; {
		case t.text == "$http", t.text == "axios":
			client = t.text
		case t.text == "$resource":
			if !clientBinding(toks, i) {
				calls = append(calls, resourceCalls(file, toks, i, note)...)
			}
			continue
		case t.text == "fetch":
			afterDot := i > 0 && next(toks, i-1, ".")
			global := i > 1 && afterDot && toks[i-2].kind == tokIdent && (toks[i-2].text == "window" || toks[i-2].text == "globalThis" || toks[i-2].text == "self") && !(i > 2 && next(toks, i-3, "."))
			if !afterDot || global {
				client = "fetch"
			}
		case inst != nil && !(i > 0 && next(toks, i-1, ".")):
			if inst.reason != "" || i == inst.decl {
				continue // stated once at its declaration
			}
			client, base = "axios-instance", inst.base
		}
		if client == "" || clientBinding(toks, i) {
			continue
		}
		axiosLike := client == "axios" || client == "axios-instance"
		c := &endpointCall{At: file + ":" + strconv.Itoa(t.line), Client: client, base: base}
		reason, detail := "", ""
		switch {
		case client != "fetch" && next(toks, i+1, ".") && i+2 < len(toks) && toks[i+2].kind == tokIdent:
			method := toks[i+2].text
			switch {
			case clientConfig[method] && !next(toks, i+3, "("):
				if axiosLike && method == "defaults" {
					reason = "client-defaults-not-read"
					break
				}
				if method == "interceptors" {
					reason, detail = "request-interceptor-not-modelled", client+".interceptors"
					break
				}
				continue
			case clientConfig[method]:
				continue
			case !next(toks, i+3, "("):
				reason, detail = "client-reference-not-a-call", client+"."+method
			case method == "request" && axiosLike:
				reason = c.config(toks, i+4, "GET")
			case callVerbs[method] != "" && (method != "jsonp" || client == "$http") && (!strings.HasSuffix(method, "Form") || axiosLike):
				c.Verb = callVerbs[method]
				reason = c.url(toks, i+4)
				if reason == "" && axiosLike && i+4 < len(toks) {
					reason = c.noBaseURL(toks, i+4)
				}
			case method == "create" && client == "axios":
				continue // read by axiosInstances
			case method == "create":
				reason = "client-instance-not-modelled"
			default:
				reason, detail = "client-method-not-modelled", client+"."+method
			}
		case next(toks, i+1, "("):
			switch {
			case client != "fetch" && next(toks, i+2, "{"):
				reason = c.config(toks, i+2, "GET")
			default:
				c.Verb = "GET"
				if reason = c.url(toks, i+2); reason == "" {
					reason = c.init(toks, i+2)
				}
			}
		default:
			reason, detail = "client-reference-not-a-call", client
		}
		if reason != "" {
			if detail == "" && c.URL != "" {
				detail = c.URL
			}
			note(t.line, reason, detail)
			continue
		}
		calls = append(calls, c)
	}
	return calls, notes
}

// jsxScript reports whether a script may hold JSX: every scanned script but a TypeScript one,
// where `<T>x` is a type assertion (AMAP-V0-356).
func jsxScript(p string) bool {
	switch path.Ext(p) {
	case ".ts", ".mts", ".cts":
		return false
	}
	return true
}

// clientInstance is one `const|let|var NAME = axios.create(...)` in a script: its literal baseURL,
// or why calls through it are not read, and the index of NAME's declaring token.
type clientInstance struct {
	base, reason string
	decl         int
}

// axiosInertConfig are the axios.create configuration keys that change neither a request's URL
// nor its method; any other key (`method`, `url`, `adapter`, ...) leaves the instance a stated
// unknown (AMAP-V0-358).
var axiosInertConfig = map[string]bool{
	"headers": true, "timeout": true, "withCredentials": true, "responseType": true, "responseEncoding": true,
	"xsrfCookieName": true, "xsrfHeaderName": true, "params": true, "paramsSerializer": true,
	"maxContentLength": true, "maxBodyLength": true, "maxRedirects": true, "validateStatus": true,
	"decompress": true, "auth": true, "proxy": true, "transformResponse": true, "timeoutErrorMessage": true,
}

// axiosInstances reads the axios instances one script creates (AMAP-V0-358). An instance whose
// configuration is not a literal object with a literal or absent baseURL and otherwise only
// axiosInertConfig keys, whose initializer continues past the create call, a NAME bound more than
// once, and a create bound to no declared name are stated once at the create; an exported
// instance is read here and its use elsewhere is stated as client-instance-escapes.
func axiosInstances(toks []token, note func(line int, reason, detail string)) map[string]*clientInstance {
	out := map[string]*clientInstance{}
	for i, t := range toks {
		if t.kind != tokIdent || t.text != "axios" || !next(toks, i+1, ".") || !word(toks, i+2, "create") || !next(toks, i+3, "(") || i > 0 && next(toks, i-1, ".") {
			continue
		}
		if i < 3 || !next(toks, i-1, "=") || toks[i-2].kind != tokIdent || !(word(toks, i-3, "const") || word(toks, i-3, "let") || word(toks, i-3, "var")) {
			note(t.line, "client-instance-not-modelled", "axios.create")
			continue
		}
		name := toks[i-2].text
		inst := &clientInstance{decl: i - 2}
		closing := i + 4
		switch keys, ok := objectKeys(toks, i+4); {
		case next(toks, i+4, ")"):
		case !ok || !next(toks, skipValue(toks, i+4), ")"):
			inst.reason = "client-instance-not-modelled"
		default:
			closing = skipValue(toks, i+4)
			for key, k := range keys {
				switch {
				case key == "baseURL":
					if !literal(toks[k]) || !next(toks, k+1, ",", "}") {
						inst.reason = "client-instance-not-modelled"
					} else {
						inst.base = toks[k].text
					}
				case !axiosInertConfig[key]:
					inst.reason = "client-instance-not-modelled"
				}
			}
		}
		// The initializer must end at the create call: `axios.create().get(...)` binds a
		// request, not an instance.
		if f := closing + 1; f < len(toks) && !next(toks, f, ";", ",", "}") && (toks[f].line == toks[closing].line || toks[f].kind == tokPunct) {
			inst.reason = "client-instance-not-modelled"
		}
		if out[name] != nil {
			inst.reason = "client-instance-shadowed"
			out[name].reason = inst.reason
		}
		for k, u := range toks {
			if k != i-2 && u.kind == tokIdent && u.text == name && !(k > 0 && next(toks, k-1, ".")) &&
				(clientBinding(toks, k) || next(toks, k+1, "=") && !next(toks, k+2, "=")) {
				inst.reason = "client-instance-shadowed"
			}
		}
		if inst.reason != "" {
			note(t.line, inst.reason, name)
		} else if word(toks, i-4, "export") {
			note(t.line, "client-instance-escapes", name)
		}
		out[name] = inst
	}
	return out
}

// resourceActions are the actions every AngularJS $resource class has.
var resourceActions = []struct{ name, verb string }{{"get", "GET"}, {"query", "GET"}, {"save", "POST"}, {"remove", "DELETE"}, {"delete", "DELETE"}}

// resourceCalls reads one `$resource(url, paramDefaults, actions)` at toks[i] (AMAP-V0-357): a
// literal url template whose `:name` and `{name}` segments are parameters yields one call per
// distinct verb of the default actions and of each declared action with a literal method (and
// an optional literal url), all at the declaration. A trailing parameter may be omitted, so the
// URL without it is a call too; any other parameter that may be omitted, a non-literal url and an
// action not read are unknown.
func resourceCalls(file string, toks []token, i int, note func(line int, reason, detail string)) []*endpointCall {
	line := toks[i].line
	if !next(toks, i+1, "(") {
		note(line, "client-reference-not-a-call", "$resource")
		return nil
	}
	type action struct{ verb, url string }
	actions := []action{}
	if !literal(toks[min(i+2, len(toks)-1)]) || !next(toks, i+3, ",", ")") {
		note(line, "non-literal-url", "")
		return nil
	}
	url := strings.ReplaceAll(toks[i+2].text, substMark, "")
	for _, a := range resourceActions {
		actions = append(actions, action{a.verb, url})
	}
	if k := skipValue(toks, i+2); next(toks, k, ",") {
		if k = skipValue(toks, k+1); next(toks, k, ",") && !next(toks, k+1, ")") {
			keys, ok := objectKeys(toks, k+1)
			if !ok {
				note(line, "non-literal-resource-action", "actions")
			}
			names := []string{}
			for name := range keys {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				spec, ok := objectKeys(toks, keys[name])
				m, has := spec["method"]
				if !ok || !has || !literal(toks[m]) || !next(toks, m+1, ",", "}") {
					note(line, "non-literal-resource-action", name)
					continue
				}
				a := action{strings.ToUpper(toks[m].text), url}
				if u, has := spec["url"]; has {
					if !literal(toks[u]) || !next(toks, u+1, ",", "}") {
						note(line, "non-literal-resource-action", name)
						continue
					}
					a.url = strings.ReplaceAll(toks[u].text, substMark, "")
				}
				actions = append(actions, a)
			}
		}
	}
	out, seen := []*endpointCall{}, map[string]bool{}
	for _, a := range actions {
		segs := strings.Split(a.url, "/")
		params, read := []int{}, true
		for k, seg := range segs {
			switch {
			case k == 0: // a scheme or relative lead is refused by setURL
			case strings.HasPrefix(seg, ":") && routeLiteralName.MatchString(seg[1:]),
				strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") && routeLiteralName.MatchString(seg[1:len(seg)-1]):
				segs[k], params = substMark, append(params, k)
			case strings.ContainsAny(seg, ":{}"):
				read = false
			}
		}
		if !read {
			note(line, "non-literal-url", a.url)
			continue
		}
		variants := []string{strings.Join(segs, "/")}
		for _, k := range params {
			if k == len(segs)-1 {
				variants = append(variants, strings.Join(segs[:k], "/"))
			} else if !seen["omit "+a.url] {
				seen["omit "+a.url] = true
				note(line, "resource-param-may-be-omitted", a.url)
			}
		}
		for _, u := range variants {
			if u == "" {
				u = "/"
			}
			if seen[a.verb+" "+u] {
				continue
			}
			seen[a.verb+" "+u] = true
			c := &endpointCall{At: file + ":" + strconv.Itoa(line), Client: "$resource", Verb: a.verb}
			if reason := c.setURL(u); reason != "" {
				note(line, reason, c.URL)
				continue
			}
			out = append(out, c)
		}
	}
	return out
}

// generatedClientMarkers are the header comments API client generators write. A generated
// client builds its URLs from its own runtime and configuration, which are not read (AMAP-V0-218).
var generatedClientMarkers = []string{"openapi-generator", "OpenAPI Generator", "swagger-codegen", "Swagger Codegen",
	"NSwag", "Generated by orval", "@generated"}

// httpClientWords are the request APIs a generated file must name for its marker to matter.
var httpClientWords = []string{"fetch", "axios", "$http", "XMLHttpRequest", "HttpClient"}

// generatedClientMarker returns the generator marker in the first 2 KiB of a script that names a
// request API, or "".
func generatedClientMarker(text string) string {
	head := text
	if len(head) > 2048 {
		head = head[:2048]
	}
	for _, m := range generatedClientMarkers {
		if !strings.Contains(head, m) {
			continue
		}
		for _, w := range httpClientWords {
			if strings.Contains(text, w) {
				return m
			}
		}
	}
	return ""
}

// clientBinding reports whether the client name at toks[i] declares rather than uses it: an import
// binding, a parameter, an object key or a declared name.
func clientBinding(toks []token, i int) bool {
	if i > 0 && (word(toks, i-1, "import") || word(toks, i-1, "function") || word(toks, i-1, "const") || word(toks, i-1, "let") || word(toks, i-1, "var") || word(toks, i-1, "as")) {
		return true
	}
	if next(toks, i+1, ":") && i > 0 && next(toks, i-1, "{", ",") {
		return true
	}
	// A parameter: walk back over `name ,` pairs to the `(` and check what the list belongs to.
	j := i
	for j > 1 && next(toks, j-1, ",") && toks[j-2].kind == tokIdent {
		j -= 2
	}
	if j == 0 || !next(toks, j-1, "(") {
		return false
	}
	open := j - 1
	k := i + 1
	for k+1 < len(toks) && next(toks, k, ",") && toks[k+1].kind == tokIdent {
		k += 2
	}
	if !next(toks, k, ")") {
		return false
	}
	if open > 0 && (word(toks, open-1, "function") || (toks[open-1].kind == tokIdent && open > 1 && word(toks, open-2, "function"))) {
		return true
	}
	if next(toks, k+1, "=>") || (next(toks, k+1, "=") && next(toks, k+2, ">")) {
		return true
	}
	return open > 0 && toks[open-1].kind == tokIdent && !controlKeywords[toks[open-1].text] && next(toks, k+1, "{") && !(open > 1 && next(toks, open-2, "."))
}

// url reads the URL expression starting at toks[i]: string and template literals and plain
// names or members joined by `+`, ending at a `,`, `)` or `}`. It sets URL and segs or returns why the
// URL is not read.
func (c *endpointCall) url(toks []token, i int) string {
	var b strings.Builder
	for k := i; ; k++ {
		if k >= len(toks) {
			return "non-literal-url"
		}
		t := toks[k]
		switch {
		case literal(t):
			b.WriteString(strings.ReplaceAll(t.text, substMark, ""))
		case t.kind == tokTemplate && !t.inexact && !t.unsure:
			b.WriteString(t.text)
		case t.kind == tokIdent && !controlKeywords[t.text]:
			for k+2 < len(toks) && next(toks, k+1, ".") && toks[k+2].kind == tokIdent {
				k += 2
			}
			if next(toks, k+1, "(", "[") {
				return "non-literal-url"
			}
			b.WriteString(substMark)
		default:
			return "non-literal-url"
		}
		if next(toks, k+1, ",", ")", "}") {
			break
		}
		if !next(toks, k+1, "+") {
			return "non-literal-url"
		}
		k++
	}
	return c.setURL(b.String())
}

// setURL normalizes a read URL: it must start with `/` (an absolute, protocol-relative or relative
// URL, or one led by a substitution, is not placed on this server), and loses its query and
// fragment.
func (c *endpointCall) setURL(u string) string {
	if c.base != "" && !strings.HasPrefix(u, substMark) && !strings.HasPrefix(u, "//") && !strings.Contains(strings.SplitN(u, "/", 2)[0], ":") {
		u = strings.TrimRight(c.base, "/") + "/" + strings.TrimLeft(u, "/") // axios combineURLs
	}
	c.URL = strings.ReplaceAll(u, substMark, "{}")
	switch {
	case u == substMark:
		return "non-literal-url"
	case strings.HasPrefix(u, substMark):
		return "url-base-not-literal"
	case strings.HasPrefix(u, "//") || strings.Contains(strings.SplitN(u, "/", 2)[0], ":"):
		return "absolute-url"
	case !strings.HasPrefix(u, "/"):
		return "relative-url"
	}
	if k := strings.IndexAny(u, "?#"); k >= 0 {
		u = u[:k]
	}
	c.URL = strings.ReplaceAll(u, substMark, "{}")
	c.segs = strings.Split(strings.TrimSuffix(u, "/"), "/")
	if u == "/" {
		c.segs = []string{""}
	}
	return ""
}

// objectKeys reads the plain object literal at toks[i]: each key's value index. ok is false when the
// object has a spread, computed key, method or shorthand.
func objectKeys(toks []token, i int) (map[string]int, bool) {
	if !next(toks, i, "{") {
		return nil, false
	}
	keys := map[string]int{}
	for k := i + 1; k < len(toks); {
		if next(toks, k, "}") {
			return keys, true
		}
		t := toks[k]
		if (t.kind != tokIdent && !literal(t)) || !next(toks, k+1, ":") {
			return nil, false
		}
		keys[t.text] = k + 2
		k = skipValue(toks, k+2)
		if next(toks, k, ",") {
			k++
		}
	}
	return nil, false
}

// config reads a request configuration object at toks[i]: a literal `url`, a literal `method` or
// def, and no `baseURL`.
func (c *endpointCall) config(toks []token, i int, def string) string {
	keys, ok := objectKeys(toks, i)
	if !ok {
		return "non-literal-config"
	}
	if _, has := keys["baseURL"]; has {
		return "config-base-url"
	}
	u, has := keys["url"]
	if !has {
		return "non-literal-url"
	}
	c.Verb = def
	if k, has := keys["method"]; has {
		if !literal(toks[k]) || !next(toks, k+1, ",", "}") {
			c.Verb = "*"
		} else {
			c.Verb = strings.ToUpper(toks[k].text)
		}
	}
	return c.url(toks, u)
}

// init reads fetch's or axios's second argument after the URL at toks[i]: absent keeps GET, an
// object literal sets a literal method, anything else leaves the verb unread ("*").
func (c *endpointCall) init(toks []token, i int) string {
	k := skipValue(toks, i)
	if !next(toks, k, ",") || next(toks, k+1, ")") {
		return ""
	}
	keys, ok := objectKeys(toks, k+1)
	if !ok {
		c.Verb = "*"
		return ""
	}
	if _, has := keys["baseURL"]; has && (c.Client == "axios" || c.Client == "axios-instance") {
		return "config-base-url"
	}
	if m, has := keys["method"]; has {
		if literal(toks[m]) && next(toks, m+1, ",", "}") {
			c.Verb = strings.ToUpper(toks[m].text)
		} else {
			c.Verb = "*"
		}
	}
	return ""
}

// noBaseURL refuses an axios verb call whose configuration literal sets baseURL.
func (c *endpointCall) noBaseURL(toks []token, i int) string {
	k := skipValue(toks, i)
	for next(toks, k, ",") && !next(toks, k+1, ")") {
		if keys, ok := objectKeys(toks, k+1); ok {
			if _, has := keys["baseURL"]; has {
				return "config-base-url"
			}
		}
		k = skipValue(toks, k+1)
	}
	return ""
}
