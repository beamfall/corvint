package appmap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// ImpactSchema names the change-to-navigation projection (ANL-V0-015).
const ImpactSchema = "application-map-impact/0"

const maxImpactPaths = 4096

// Verdicts: an unmapped UI path anywhere forces the whole app's suite (fail closed).
const (
	VerdictTargeted     = "targeted"
	VerdictFullSuite    = "full-suite-required"
	VerdictNoNavigation = "no-navigation-impact"
)

type ImpactScreen struct {
	ID    string   `json:"id"`
	Paths []string `json:"paths"`
	Basis string   `json:"basis"`
}
type ImpactEdge struct {
	ID      string `json:"id"`
	From    string `json:"from"`
	To      string `json:"to"`
	Status  string `json:"status"`
	Reprove string `json:"reprove_with,omitempty"`
	Flipped bool   `json:"flipped"`
}
type ImpactUnknown struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
	// Detail names what kept the path unknown when the reference hop knows it: the referrer
	// file and line of an unprovable mention, or the file, total or walked-node bound hit
	// (ANL-V0-072).
	Detail string `json:"detail,omitempty"`
	// Remedy names the declaration change that would resolve the unknown, or states that none is
	// inferred (AMAP-V0-210).
	Remedy *Remedy `json:"remedy,omitempty"`
}

// ImpactExempt is one changed path a declared exemption counts under ignored_paths (ANL-V0-810).
type ImpactExempt struct {
	Path     string `json:"path"`
	Pattern  string `json:"pattern"`
	Reason   string `json:"reason"`
	Declared string `json:"declared_at"`
}

type ImpactReport struct {
	Schema       string   `json:"schema"`
	App          string   `json:"app"`
	MapRevision  string   `json:"map_revision"`
	Base         string   `json:"base"`
	Head         string   `json:"head"`
	MapDigest    string   `json:"map_digest"`
	LedgerDigest string   `json:"ledger_digest,omitempty"`
	TestRoot     *RootPin `json:"test_root,omitempty"` // tests root pin from the map (ANL-V0-067); absent for one root
	Authority    string   `json:"authority"`
	Verdict      string   `json:"verdict"`
	Changed      int      `json:"changed_paths"`
	// Ignored counts the changed non-UI paths a declared manifest exemption covers (ANL-V0-048,
	// ANL-V0-810); Exempt lists each with its declaring manifest line.
	Ignored int            `json:"ignored_paths"`
	Exempt  []ImpactExempt `json:"exempt,omitempty"`
	// RecordsRevision and RecordsDigest pin the observed-ui-records/0 document whose bindings
	// Observed lists (OUE-V0-073); all three are absent when no records were supplied.
	RecordsRevision string           `json:"records_revision,omitempty"`
	RecordsDigest   string           `json:"records_digest,omitempty"`
	Observed        []ImpactObserved `json:"observed,omitempty"`
	Screens         []ImpactScreen   `json:"screens"`
	Edges           []ImpactEdge     `json:"edges"`
	SpecFiles       []string         `json:"spec_files"`
	TestKeys        []string         `json:"test_keys"`
	Unknown         []ImpactUnknown  `json:"unknown"`
	Omitted         map[string]int   `json:"omitted"`
	Digest          string           `json:"digest"`
}

var impactUIExtensions = map[string]bool{".html": true, ".htm": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
	".mjs": true, ".cjs": true, ".vue": true, ".svelte": true, ".css": true, ".scss": true, ".sass": true, ".less": true,
	".pug": true, ".hbs": true, ".mdx": true}

// Impact maps the paths changed between base and the evaluated revision onto screens (router
// anchors and their lineage descendants, attributed test files and flow anchors), the edges
// touching them and the tests that cross those edges. A changed UI path the map does not place is
// walked through the literal references to it up to router anchors and test files (ANL-V0-032);
// one the walk cannot prove is listed as unknown and forces full-suite-required (ANL-V0-015,
// ANL-V0-016). A changed non-UI path is unknown too, and a changed test-support file selects the
// specs importing it or reads unknown, so every changed path is placed or unknown (ANL-V0-048,
// ANL-V0-049).
func Impact(ctx context.Context, m *Map, l *EdgeLedger, base string, o Options) ([]byte, error) {
	return ImpactWithRecords(ctx, m, l, nil, base, o)
}

// ImpactWithRecords is Impact that also reads the page-object, workflow and spec bindings of one
// observed-ui-records/0 document (OUE-V0-070..074); a nil document reads as Impact.
func ImpactWithRecords(ctx context.Context, m *Map, l *EdgeLedger, rec *UIRecords, base string, o Options) ([]byte, error) {
	cap, err := o.budget(DefaultFlowBudget)
	if err != nil {
		return nil, err
	}
	r := repo{ctx: ctx, root: o.Root}
	head := o.Revision
	if head == "" {
		head = "HEAD"
	}
	headOID, err := r.resolve(head)
	if err != nil {
		return nil, orPlatform(err, verifyError("appmap-invalid-query", "evaluated revision is not a commit"))
	}
	if err = checkLedgerBinding(m, l, headOID); err != nil {
		return nil, err
	}
	if err = checkRecordsBinding(m, rec); err != nil {
		return nil, err
	}
	baseOID, err := r.resolve(base)
	if err != nil {
		return nil, orPlatform(err, verifyError("appmap-invalid-query", "--base is not a commit"))
	}
	out, err := r.git(8<<20, nil, "diff", "--name-only", "-z", "--no-renames", "--no-ext-diff", baseOID, headOID, "--")
	if err != nil {
		return nil, err
	}
	var changed []string
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			changed = append(changed, p)
		}
	}
	if len(changed) > maxImpactPaths {
		return nil, budgetError("more than %d changed paths", maxImpactPaths)
	}
	sort.Strings(changed)
	rep := ImpactReport{Schema: ImpactSchema, App: m.App, MapRevision: m.Revision, Base: baseOID, Head: headOID, MapDigest: m.Digest,
		Authority: AuthorityLearned, Changed: len(changed), Screens: []ImpactScreen{}, Edges: []ImpactEdge{}, SpecFiles: []string{},
		TestKeys: []string{}, Unknown: []ImpactUnknown{}, Omitted: map[string]int{"screens": 0, "edges": 0, "spec_files": 0, "test_keys": 0, "unknown": 0}}
	if l != nil {
		rep.LedgerDigest = l.Digest
	}
	rep.TestRoot = testRootPin(m)
	if rec != nil {
		rep.RecordsRevision, rep.RecordsDigest = rec.Revision, rec.Digest
		rep.Omitted["observed"] = 0
	}
	direct, hit := map[string][]string{}, map[string]*ImpactScreen{}
	touch := func(id, p, basis string) {
		s := hit[id]
		if s == nil {
			s = &ImpactScreen{ID: id, Basis: basis}
			hit[id] = s
		}
		if len(s.Paths) == 0 || s.Paths[len(s.Paths)-1] != p {
			s.Paths = append(s.Paths, p)
		}
	}
	specs := map[string]bool{}
	unplaced, support := []string{}, []string{}
	ep, err := impactEndpoints(ctx, m, o, r, baseOID, headOID, changed)
	if err != nil {
		return nil, err
	}
	exempt := impactExemptions(r, m)
	for _, p := range changed {
		placed := false
		if p == m.Manifest.Path && m.Manifest.Repo == "" {
			rep.Unknown = append(rep.Unknown, ImpactUnknown{Path: p, Reason: "manifest-changed"})
			continue
		}
		for i := range m.Screens {
			s := &m.Screens[i]
			for _, a := range append(append([]Anchor{s.Anchor}, s.NameFrom...), s.ParentFrom...) {
				if a.Repo == "" && a.Path == p {
					direct[s.ID] = append(direct[s.ID], p)
					touch(s.ID, p, "router-anchor")
					placed = true
				}
			}
			for _, a := range s.Specs {
				if (a.File == p || contains(a.Via, p)) && primaryTestFile(m, a.File) {
					touch(s.ID, p, "test-attribution")
					specs[a.File] = true
					placed = true
				}
			}
		}
		for _, f := range m.Files {
			if f.Path == p && f.Anchor.Repo == "" {
				placed = true
				if f.Screen != "" {
					touch(f.Screen, p, "test-file")
				}
				if f.Role == roleSpec {
					specs[p] = true
				} else {
					support = append(support, p)
				}
			}
		}
		for _, fl := range m.Flows {
			if fl.Anchor.Repo == "" && fl.Anchor.Path == p {
				placed = true
				for _, st := range fl.Steps {
					if st.Screen != "" {
						touch(st.Screen, p, "flow-anchor")
					}
				}
			}
		}
		switch {
		case placed:
		case impactUIExtensions[strings.ToLower(path.Ext(p))]:
			unplaced = append(unplaced, p)
		case ep != nil && ep.place(p, touch, &rep.Unknown):
		case exempt.covers(p, &rep):
		default:
			// A non-UI path (a server model, a worker, a config file) can still break any screen,
			// and no declared exemption covers it, so it is unknown rather than ignored (ANL-V0-048).
			rep.Unknown = append(rep.Unknown, ImpactUnknown{Path: p, Reason: "non-ui-path"})
		}
	}
	var importers map[string][]*TestFile
	incomplete := false
	// importing selects the specs importing the test file via and reports whether that selection
	// may miss one (ANL-V0-049).
	importing := func(via string) (found []string, partial bool) {
		if importers == nil {
			incomplete, importers = testGraphIncomplete(m), testImporters(m)
		}
		found = importingSpecs(importers, via)
		for _, f := range found {
			specs[f] = true
		}
		return found, incomplete
	}
	for _, p := range support {
		switch found, partial := importing(p); {
		case partial:
			rep.Unknown = append(rep.Unknown, ImpactUnknown{Path: p, Reason: "test-import-graph-incomplete"})
		case len(found) == 0:
			rep.Unknown = append(rep.Unknown, ImpactUnknown{Path: p, Reason: "test-support-unreached"})
		}
	}
	if len(unplaced) > 0 {
		g, unread := loadReferenceGraph(r, headOID)
		for _, p := range unplaced {
			targets, why, detail := []hopTarget(nil), "reference-graph-bound", unread
			if g != nil {
				targets, why, detail = templateHop(m, g, p)
			}
			if why != "" {
				rep.Unknown = append(rep.Unknown, ImpactUnknown{Path: p, Reason: why, Detail: detail})
				continue
			}
			partial := false
			for _, t := range targets {
				for _, id := range t.screens {
					direct[id] = append(direct[id], p)
					touch(id, p, "template-reference:"+t.via)
				}
				for _, f := range t.specs {
					specs[f] = true
				}
				// A hop ending at a test file that is not a spec (a bound page object) also selects
				// the specs importing it, as a changed test-support file does (ANL-V0-049).
				if f := primaryTestFileAt(m, t.via); f != nil && f.Role != roleSpec {
					_, q := importing(t.via)
					partial = partial || q
				}
			}
			if partial {
				rep.Unknown = append(rep.Unknown, ImpactUnknown{Path: p, Reason: "test-import-graph-incomplete"})
			}
		}
	}
	// Observed bindings only add screens and tests (OUE-V0-071, OUE-V0-072).
	observedKeys := map[string]bool{}
	for _, ob := range observedBindings(m, rec, changed) {
		for _, id := range ob.Screens {
			touch(id, ob.Path, BasisObservedBinding)
		}
		for _, k := range ob.Tests {
			observedKeys[k] = true
		}
		rep.Observed = append(rep.Observed, ob)
	}
	for i := range m.Screens {
		s := &m.Screens[i]
		seen := map[string]bool{s.ID: true}
		for cur := m.screen(s.Parent); cur != nil && !seen[cur.ID]; cur = m.screen(cur.Parent) {
			seen[cur.ID] = true
			if ps, ok := direct[cur.ID]; ok {
				for _, p := range ps {
					touch(s.ID, p, "lineage:"+cur.ID)
				}
				break
			}
		}
	}
	for _, s := range hit {
		sort.Strings(s.Paths)
		rep.Screens = append(rep.Screens, *s)
	}
	sort.Slice(rep.Screens, func(i, j int) bool { return rep.Screens[i].ID < rep.Screens[j].ID })
	for _, s := range m.Screens {
		if hit[s.ID] != nil {
			for _, a := range s.Specs {
				specs[a.File] = true
			}
		}
	}
	keys := observedKeys
	if l != nil {
		for _, e := range l.Edges {
			if hit[e.From] == nil && hit[e.To] == nil {
				continue
			}
			ie := ImpactEdge{ID: e.ID, From: e.From, To: e.To, Status: e.Status, Flipped: e.Status == "UNVERIFIED_AT_HEAD"}
			if e.Evidence != nil && e.Evidence.Verification.Receipt != nil {
				ie.Reprove = e.Evidence.Verification.Receipt.TestKey
				keys[ie.Reprove] = true
			}
			for _, k := range e.RedundantCrossers {
				keys[k] = true
			}
			rep.Edges = append(rep.Edges, ie)
		}
	} else {
		for _, e := range m.Edges {
			if hit[e.From] != nil || hit[e.To] != nil {
				rep.Edges = append(rep.Edges, ImpactEdge{ID: prefixEdgeID(e.From, e.To), From: e.From, To: e.To, Status: "NOT_IN_LEDGER"})
			}
		}
	}
	for _, e := range m.Edges {
		if e.Basis == "test-sequence" && (hit[e.From] != nil || hit[e.To] != nil) {
			for _, f := range m.Files {
				if f.ID == e.Source && f.Role == roleSpec {
					specs[f.Path] = true
				}
			}
		}
	}
	sort.Slice(rep.Edges, func(i, j int) bool { return rep.Edges[i].ID < rep.Edges[j].ID })
	for f := range specs {
		if f != "" {
			rep.SpecFiles = append(rep.SpecFiles, f)
		}
	}
	for k := range keys {
		rep.TestKeys = append(rep.TestKeys, k)
	}
	sort.Strings(rep.SpecFiles)
	sort.Strings(rep.TestKeys)
	for i := range rep.Unknown {
		rep.Unknown[i].Remedy = impactRemedy(m, rep.Unknown[i])
	}
	switch {
	case len(rep.Unknown) > 0:
		rep.Verdict = VerdictFullSuite
	case len(rep.Screens) > 0:
		rep.Verdict = VerdictTargeted
	default:
		rep.Verdict = VerdictNoNavigation
	}
	complete, _ := json.Marshal(rep)
	rep.Digest = digest(complete)
	for {
		raw, err := json.Marshal(rep)
		if err != nil {
			return nil, err
		}
		raw = append(raw, '\n')
		if len(raw) <= cap {
			return raw, nil
		}
		switch {
		case len(rep.TestKeys) > 0:
			rep.TestKeys = rep.TestKeys[:len(rep.TestKeys)-1]
			rep.Omitted["test_keys"]++
		case len(rep.SpecFiles) > 0:
			rep.SpecFiles = rep.SpecFiles[:len(rep.SpecFiles)-1]
			rep.Omitted["spec_files"]++
		case len(rep.Edges) > 0:
			rep.Edges = rep.Edges[:len(rep.Edges)-1]
			rep.Omitted["edges"]++
		case len(rep.Observed) > 0:
			rep.Observed = rep.Observed[:len(rep.Observed)-1]
			rep.Omitted["observed"]++
		case len(rep.Screens) > 0:
			rep.Screens = rep.Screens[:len(rep.Screens)-1]
			rep.Omitted["screens"]++
		case len(rep.Unknown) > 1:
			rep.Unknown = rep.Unknown[:len(rep.Unknown)-1]
			rep.Omitted["unknown"]++
		default:
			return nil, budgetError("impact report header exceeds --budget")
		}
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// checkLedgerBinding refuses a ledger from another app, map digest or evaluated revision, or one
// trimmed to a byte cap (a nil ledger passes).
func checkLedgerBinding(m *Map, l *EdgeLedger, evaluated string) error {
	if l == nil {
		return nil
	}
	if l.App != m.App || l.MapDigest != m.Digest || l.Revision != evaluated {
		return verifyError("appmap-invalid-query", "the edge ledger was not built from this map at the evaluated revision")
	}
	for _, n := range l.Omitted {
		if n > 0 {
			return verifyError("appmap-invalid-query", "the edge ledger is trimmed; rebuild it with --full")
		}
	}
	return nil
}

// primaryTestFile is true when file is an analysed test file of the --root repository itself.
func primaryTestFile(m *Map, file string) bool {
	for _, f := range m.Files {
		if f.Path == file && f.Anchor.Repo == "" {
			return true
		}
	}
	return false
}

// LoadImpactReport reads one complete application-map-impact/0 report for a consumer such as
// `corvint affected` (ANL-V0-031): a bounded regular file, strictly decoded, untrimmed, whose
// verdict follows from its own lists and whose digest matches its content. Anything else refuses.
func LoadImpactReport(name string) (*ImpactReport, error) {
	raw, err := readReceipt(name)
	if err != nil {
		return nil, verifyError("appmap-invalid-query", "--appmap-impact must name a regular JSON file of at most 4 MiB")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var r ImpactReport
	if dec.Decode(&r) != nil || dec.Decode(new(json.RawMessage)) != io.EOF || r.Schema != ImpactSchema || r.Authority != AuthorityLearned || r.App == "" || r.Base == "" || r.Head == "" {
		return nil, verifyError("appmap-invalid-query", "invalid application-map-impact/0 report")
	}
	for _, n := range r.Omitted {
		if n != 0 {
			return nil, verifyError("appmap-invalid-query", "the impact report is trimmed; rebuild it with --full")
		}
	}
	want := VerdictNoNavigation
	switch {
	case len(r.Unknown) > 0:
		want = VerdictFullSuite
	case len(r.Screens) > 0:
		want = VerdictTargeted
	}
	got := r.Digest
	r.Digest = ""
	complete, err := json.Marshal(r)
	if err != nil || r.Verdict != want || got != digest(complete) {
		return nil, verifyError("appmap-invalid-query", "the impact report does not match its digest or verdict")
	}
	r.Digest = got
	return &r, nil
}

// testImporters indexes the primary-repository test import graph backwards: each resolved import
// target maps to the test files importing it.
func testImporters(m *Map) map[string][]*TestFile {
	importers := map[string][]*TestFile{}
	for i := range m.Files {
		f := &m.Files[i]
		if f.Anchor.Repo != "" {
			continue
		}
		for _, imp := range f.Imports {
			if imp.Status == importResolved {
				importers[imp.Resolved] = append(importers[imp.Resolved], f)
			}
		}
	}
	return importers
}

// importingSpecs walks the test import graph backwards from a changed test-support file (page
// object, workflow, scenario, helper) and returns every spec that imports it directly or through
// other test files (ANL-V0-049). The walk is unbounded but finite: each file is visited once.
func importingSpecs(importers map[string][]*TestFile, p string) []string {
	seen, queue, out := map[string]bool{p: true}, []string{p}, []string{}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, f := range importers[cur] {
			if seen[f.Path] {
				continue
			}
			seen[f.Path] = true
			if f.Role == roleSpec {
				out = append(out, f.Path)
			}
			queue = append(queue, f.Path)
		}
	}
	sort.Strings(out)
	return out
}

// primaryTestFileAt returns the primary-repository test file the map records at p, or nil.
func primaryTestFileAt(m *Map, p string) *TestFile {
	for i := range m.Files {
		if f := &m.Files[i]; f.Path == p && f.Anchor.Repo == "" {
			return f
		}
	}
	return nil
}

// testGraphIncomplete reports whether the primary-repository test import graph may hold an edge
// the map did not record: a test file the index excluded or could not parse, an import it could
// not resolve (a computed `import(...)` or `require(...)` included), a first-party import outside
// tests.root that the map does not read, or an import resolved to a code module the map does not
// record (such as a `.mts` or `.cts` file). Any such file may import a changed test-support file,
// so the importing specs found are not all of them.
func testGraphIncomplete(m *Map) bool {
	recorded := map[string]bool{}
	for _, f := range m.Files {
		if f.Anchor.Repo == "" {
			recorded[f.Path] = true
		}
	}
	for _, f := range m.Files {
		if f.Anchor.Repo != "" {
			continue
		}
		for _, imp := range f.Imports {
			if imp.Status == importResolved && !recorded[imp.Resolved] && codeSuffix[strings.ToLower(path.Ext(imp.Resolved))] {
				return true
			}
		}
	}
	for _, u := range m.Unknowns {
		if u.Repo != "" {
			continue
		}
		switch {
		case u.Kind == "file":
			return true
		case u.Kind == "import" && (u.Reason == "unresolved-import" || u.Reason == "import-outside-tests"):
			return true
		}
	}
	return false
}

// testRootPin copies the map's pin of the aliased root that supplied the tests, or nil.
func testRootPin(m *Map) *RootPin {
	for _, p := range m.Roots {
		for _, in := range p.Inputs {
			if in == "tests" {
				pin := p
				pin.Inputs = append([]string(nil), p.Inputs...)
				return &pin
			}
		}
	}
	return nil
}

// impactJoin is the endpoint join impact reads (AMAP-V0-220): endpoints by controller file and by
// declaring file, the Rails endpoint differences between base and head by changed declaration
// file (AMAP-V0-355), and why the join may miss a caller.
type impactJoin struct {
	byController, byDeclaration map[string][]*endpointRow
	railsDiff                   map[string][]railsEndpointDiff
	railsPartial                string
	incomplete                  string
}

// railsEndpointDiff is one Rails endpoint present at only one side of the impact range, with the
// screens of the read client calls that reach it.
type railsEndpointDiff struct {
	kind, id string // kind is added, removed, changed or reordered
	screens  []string
}

func impactEndpoints(ctx context.Context, m *Map, o Options, r repo, baseOID, headOID string, changed []string) (*impactJoin, error) {
	if o.Endpoints == nil {
		return nil, nil
	}
	// A --root router the change adds (a map at base) or deletes (a map at head) is absent at the
	// map revision; the join reads it as declaring nothing and the comparison below reads both
	// sides. A router absent and unchanged still refuses (AMAP-V0-355).
	eo := *o.Endpoints
	inChange := map[string]bool{}
	for _, p := range changed {
		inChange[p] = true
	}
	local := []string{}
	for _, p := range eo.RailsRoutes {
		if alias, _ := splitSource(p); alias == "" && inChange[p] {
			local = append(local, p)
		}
	}
	if len(local) > 0 {
		if types, err := r.objectTypes(m.Revision, local); err == nil {
			for i, p := range local {
				if types[i] == "" {
					if eo.absent == nil {
						eo.absent = map[string]bool{}
					}
					eo.absent[p] = true
				}
			}
		}
	}
	j, err := joinEndpoints(ctx, m, eo, o)
	if err != nil {
		return nil, err
	}
	ij := &impactJoin{byController: map[string][]*endpointRow{}, byDeclaration: map[string][]*endpointRow{}}
	// The changed paths are --root's, so only an endpoint declared in --root is indexed: a
	// same-named path never stands for an aliased root's controller or declaration (AMAP-V0-222).
	rails, railsDecl := []string{}, map[string]bool{}
	for _, p := range append(append([]string{}, o.Endpoints.RailsRoutes...), o.Endpoints.OpenAPI...) {
		if alias, _ := splitSource(p); alias == "" {
			ij.byDeclaration[p] = []*endpointRow{}
		}
	}
	for _, p := range o.Endpoints.RailsRoutes {
		if alias, _ := splitSource(p); alias == "" {
			rails, railsDecl[p] = append(rails, p), true
		}
	}
	for _, e := range j.endpoints {
		if e.Repo != "" {
			continue
		}
		if e.Controller != "" {
			ij.byController[e.Controller] = append(ij.byController[e.Controller], e)
		}
		file := e.Declaration
		if k := strings.IndexAny(file, ":#"); k >= 0 {
			file = file[:k]
		}
		ij.byDeclaration[file] = append(ij.byDeclaration[file], e)
		if e.Source == "rails" {
			railsDecl[file] = true
		}
	}
	// A changed Rails declaration file, a listed router or a file under its routes/ directory, is
	// compared at base and head instead of standing for every endpoint it declares (AMAP-V0-355).
	railsChanged := []string{}
	for _, p := range changed {
		drawn := false
		for _, f := range rails {
			drawn = drawn || path.Base(f) == "routes.rb" && strings.HasPrefix(p, path.Join(path.Dir(f), "routes")+"/") && path.Ext(p) == ".rb"
		}
		if railsDecl[p] || drawn {
			railsChanged = append(railsChanged, p)
		}
	}
	if len(railsChanged) > 0 {
		if err := ij.compareRails(r, baseOID, headOID, rails, railsChanged, j.calls); err != nil {
			return nil, err
		}
	}
	open, first := 0, ""
	for _, u := range j.unknown {
		if u.Reason != "no-declared-endpoint" && u.Reason != "non-route-statement" {
			if open == 0 {
				first = rootRef(u.Repo, u.At) + " " + u.Reason
			}
			open++
		}
	}
	if open > 0 {
		ij.incomplete = fmt.Sprintf("%d endpoint join unknowns, first %s", open, first)
	}
	return ij, nil
}

// compareRails compiles the --root Rails routers at base and at head and records each endpoint
// (verb, path and handler) present at only one side, each present at both whose constraints or
// defaults differ (changed), and each of two overlapping endpoints whose declaration order flips
// (reordered), against its declaring file when that file changed, else against every changed
// declaration file. A router absent at a side reads as no endpoints; any route unknown at either
// side other than a non-route statement makes the comparison partial.
func (ij *impactJoin) compareRails(r repo, baseOID, headOID string, rails, changed []string, calls []*endpointCall) error {
	ij.railsDiff = map[string][]railsEndpointDiff{}
	for _, p := range changed {
		ij.railsDiff[p] = []railsEndpointDiff{}
	}
	order := map[string][]string{} // each side's keys in first-declaration order
	side := func(rev string) (map[string][]*endpointRow, error) {
		types, err := r.objectTypes(rev, rails)
		if err != nil {
			return nil, err
		}
		files := []string{}
		for i, f := range rails {
			if types[i] == "blob" {
				files = append(files, f)
			}
		}
		rows, notes := []*endpointRow{}, []endpointUnknown{}
		if len(files) > 0 {
			remaining := maxRouterFiles - len(files)
			if rows, notes, err = railsEndpoints(r, rev, files, &remaining); err != nil {
				return nil, err
			}
		}
		for _, n := range notes {
			if n.Reason != "non-route-statement" && ij.railsPartial == "" {
				ij.railsPartial = n.At + " " + n.Reason
			}
		}
		out := map[string][]*endpointRow{}
		for _, e := range rows {
			k := e.Verb + " " + e.Path + " " + e.Handler
			if out[k] == nil {
				order[rev] = append(order[rev], k)
			}
			out[k] = append(out[k], e)
		}
		return out, nil
	}
	base, err := side(baseOID)
	if err != nil {
		return err
	}
	head, err := side(headOID)
	if err != nil {
		return err
	}
	record := func(kind string, rows []*endpointRow, n int) {
		for _, e := range rows[len(rows)-n:] {
			d := railsEndpointDiff{kind: kind, id: e.Verb + " " + strings.Join(e.segs, "/")}
			seen := map[string]bool{}
			for _, c := range calls {
				if e.reaches(c.Verb, c.segs) {
					for _, s := range c.Screens {
						if !seen[s] {
							seen[s] = true
							d.screens = append(d.screens, s)
						}
					}
				}
			}
			file := e.Declaration
			if k := strings.LastIndexByte(file, ':'); k >= 0 {
				file = file[:k]
			}
			if _, ok := ij.railsDiff[file]; ok {
				ij.railsDiff[file] = append(ij.railsDiff[file], d)
				continue
			}
			for _, p := range changed {
				ij.railsDiff[p] = append(ij.railsDiff[p], d)
			}
		}
	}
	keys := []string{}
	for k := range base {
		keys = append(keys, k)
	}
	for k := range head {
		if base[k] == nil {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		switch b, h := base[k], head[k]; {
		case len(h) > len(b):
			record("added", h, len(h)-len(b))
		case len(b) > len(h):
			record("removed", b, len(b)-len(h))
		case !slices.Equal(railsGuards(b), railsGuards(h)):
			record("changed", h, len(h))
		}
	}
	// Rails matches the first declared route, so two endpoints that may match one request and
	// swap their order change which one serves it.
	headAt := map[string]int{}
	for n, k := range order[headOID] {
		headAt[k] = n
	}
	common := []string{}
	for _, k := range order[baseOID] {
		if _, ok := headAt[k]; ok {
			common = append(common, k)
		}
	}
	reordered := map[string]bool{}
	for x := range common {
		for y := x + 1; y < len(common); y++ {
			a, b := common[x], common[y]
			if headAt[a] > headAt[b] && railsOverlap(head[a][0], head[b][0]) {
				reordered[a], reordered[b] = true, true
			}
		}
	}
	for _, k := range keys {
		if reordered[k] {
			record("reordered", head[k], len(head[k]))
		}
	}
	return nil
}

func railsGuards(rows []*endpointRow) []string {
	out := []string{}
	for _, e := range rows {
		out = append(out, e.guard)
	}
	sort.Strings(out)
	return out
}

// railsOverlap reports whether one request may match both Rails endpoints: their verbs meet and
// their segments agree, a parameter agreeing with anything and a splat with any remainder.
func railsOverlap(a, b *endpointRow) bool {
	verbs := func(e *endpointRow) []string {
		v := strings.Split(e.Verb, "|")
		if slices.Contains(v, "GET") {
			v = append(v, "HEAD")
		}
		return v
	}
	meet := a.Verb == "ANY" || b.Verb == "ANY"
	for _, v := range verbs(a) {
		meet = meet || slices.Contains(verbs(b), v)
	}
	if !meet {
		return false
	}
	for i := 0; ; i++ {
		switch {
		case i == len(a.segs) || i == len(b.segs):
			return len(a.segs) == len(b.segs)
		case a.segs[i] == "{*}" || b.segs[i] == "{*}":
			return true
		case a.segs[i] != b.segs[i] && a.segs[i] != "{}" && b.segs[i] != "{}":
			return false
		}
	}
}

// place puts a changed controller or endpoint declaration on the screens calling its endpoints and
// reports whether p was one. A changed Rails declaration places only the endpoints added or
// removed between base and head (AMAP-V0-355); a caller the join may miss, an endpoint no read
// call reaches, and a declaration whose endpoints are not fully compared stay unknown.
func (ij *impactJoin) place(p string, touch func(id, p, basis string), unknown *[]ImpactUnknown) bool {
	if diffs, ok := ij.railsDiff[p]; ok {
		if ij.railsPartial != "" {
			*unknown = append(*unknown, ImpactUnknown{Path: p, Reason: "endpoint-declaration-changed",
				Detail: "the base and head endpoint comparison is partial, first " + ij.railsPartial})
		}
		for _, d := range diffs {
			basis := "endpoint:" + d.id // added, changed or reordered
			if d.kind == "removed" {
				basis = "endpoint-removed:" + d.id
			}
			for _, s := range d.screens {
				touch(s, p, basis)
			}
			if len(d.screens) == 0 {
				*unknown = append(*unknown, ImpactUnknown{Path: p, Reason: "endpoint-not-called", Detail: d.kind + " " + d.id})
			}
		}
		if ij.incomplete != "" && len(diffs) > 0 {
			*unknown = append(*unknown, ImpactUnknown{Path: p, Reason: "endpoint-callers-incomplete", Detail: ij.incomplete})
		}
		return true
	}
	eps, controller := ij.byController[p]
	if !controller {
		if eps, controller = ij.byDeclaration[p]; !controller {
			return false
		}
		*unknown = append(*unknown, ImpactUnknown{Path: p, Reason: "endpoint-declaration-changed",
			Detail: fmt.Sprintf("%d endpoints declared at the map revision", len(eps))})
	}
	for _, e := range eps {
		for _, s := range e.Screens {
			touch(s, p, "endpoint:"+e.ID)
		}
		if len(e.Calls) == 0 {
			*unknown = append(*unknown, ImpactUnknown{Path: p, Reason: "endpoint-not-called", Detail: e.ID})
		}
	}
	if ij.incomplete != "" {
		*unknown = append(*unknown, ImpactUnknown{Path: p, Reason: "endpoint-callers-incomplete", Detail: ij.incomplete})
	}
	return true
}

type impactExemptRule struct {
	ImpactExemptEntry
	declared string
}

type impactExemptRules []impactExemptRule

// impactExemptions reads the exemptions the --root manifest declares at the map revision, with the
// manifest line declaring each (ANL-V0-810). A manifest held by an aliased root, or one that is
// unreadable at the map's blob, declares none here, so its non-UI paths stay unknown.
func impactExemptions(r repo, m *Map) impactExemptRules {
	if m.Manifest.Repo != "" || !safeRelative(m.Manifest.Path) {
		return nil
	}
	e, raw, err := r.readOne(m.Revision, m.Manifest.Path, maxManifestBytes)
	if err != nil || e.oid != m.Manifest.Blob {
		return nil
	}
	man, err := decodeManifest(raw)
	if err != nil {
		return nil
	}
	key := bytes.Index(raw, []byte(`"impact_exempt"`))
	out := impactExemptRules{}
	for _, x := range man.ImpactExempt {
		at := key
		if enc, err := json.Marshal(x.Path); err == nil && key >= 0 {
			if k := bytes.Index(raw[key:], enc); k >= 0 {
				at = key + k
			}
		}
		line := 1
		if at > 0 {
			line += bytes.Count(raw[:at], []byte{'\n'})
		}
		out = append(out, impactExemptRule{x, m.Manifest.Path + ":" + strconv.Itoa(line)})
	}
	return out
}

// impactUnexemptable names template and script extensions the classifier does not read as UI:
// such a path may still render a screen, so no exemption covers it (ANL-V0-810).
var impactUnexemptable = map[string]bool{".mts": true, ".cts": true, ".erb": true, ".haml": true, ".slim": true,
	".ejs": true, ".njk": true, ".twig": true, ".liquid": true, ".mustache": true, ".handlebars": true, ".jinja": true,
	".jinja2": true, ".j2": true, ".jsp": true, ".php": true, ".cshtml": true, ".razor": true, ".astro": true,
	".coffee": true, ".styl": true, ".tpl": true}

// covers counts p under ignored_paths when a declared exemption matches it (ANL-V0-810).
func (rules impactExemptRules) covers(p string, rep *ImpactReport) bool {
	if impactUnexemptable[strings.ToLower(path.Ext(p))] {
		return false
	}
	for _, x := range rules {
		if globRoute(x.Path, p) {
			rep.Ignored++
			rep.Exempt = append(rep.Exempt, ImpactExempt{Path: p, Pattern: x.Path, Reason: x.Reason, Declared: x.declared})
			return true
		}
	}
	return false
}
