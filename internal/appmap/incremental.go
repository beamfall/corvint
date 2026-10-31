package appmap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// Incremental reports how BuildFrom produced its map (AMAP-V0-211): Reused when the previous map
// was carried to the new revision, otherwise Reason names why the map was rebuilt in full.
type Incremental struct {
	From    string `json:"from"`
	Reused  bool   `json:"reused"`
	Changed int    `json:"changed"`
	Reason  string `json:"reason,omitempty"`
	// Rederived names the map sections BuildFrom re-derived while keeping the rest of the previous
	// map (AMAP-V0-240); absent when the map was carried whole or rebuilt in full.
	Rederived []string `json:"rederived,omitempty"`
}

// Reasons BuildFrom rebuilds in full (AMAP-V0-211, AMAP-V0-215, AMAP-V0-216, AMAP-V0-245).
const (
	RebuildAliasedRoots   = "aliased-roots"
	RebuildBuilderVersion = "builder-version-differs"
	RebuildRootDirty      = "aliased-root-dirty"
	RebuildManifestPath   = "manifest-path-differs"
	RebuildFromUnreadable = "from-revision-unavailable"
	RebuildChangedInput   = "changed-input"
	// RebuildIndexAdmission: an edited source enters or leaves the context index (its size bound,
	// a generated header or an LFS pointer), or the edit may cross the index's aggregate bound.
	RebuildIndexAdmission = "index-admission-changed"
	// RebuildConfigShaped: an edited source parses as JSON, so a tsconfig/jsconfig `extends` may
	// read it and the test joins' import resolution may change.
	RebuildConfigShaped = "config-shaped-source"
)

// BuildFrom compiles the map at revision from prev, a map of the same manifest at an earlier
// revision (AMAP-V0-211). When every path changed between the two revisions, in --root and in
// each aliased root prev pins (AMAP-V0-216), is one no build input reads, the map is prev carried
// to revision, which is the map a full build gives; any other change, a different root set, a map
// from another builder version (AMAP-V0-215), or a revision it cannot read rebuilds in full. Its
// result is the map BuildRoots returns, so the digest never depends on which path was taken.
func BuildFrom(ctx context.Context, root, manifestPath, revision string, roots Roots, prev *Map) (*Map, Incremental, error) {
	inc := Incremental{From: prev.Revision}
	r := repo{ctx: ctx, root: root}
	rev, err := r.resolve(revision)
	if err != nil {
		return nil, inc, orPlatform(err, &gokernel.Error{Code: "appmap-invalid-revision", Message: err.Error()})
	}
	var pins []RootPin
	var part *partialReuse
	inc.Reason, inc.Changed, pins, part = reuseBlocked(ctx, r, manifestPath, roots, prev, rev)
	if inc.Reason != "" {
		m, err := BuildRoots(ctx, root, manifestPath, rev, roots)
		return m, inc, err
	}
	if part != nil && part.sources {
		m, err := rederiveSources(r, rev, part, prev, pins)
		if err != nil {
			return nil, inc, err
		}
		inc.Rederived = []string{"routers", "test-joins"}
		if part.manifest.Flows != "" {
			inc.Rederived = append(inc.Rederived, "flows")
		}
		return m, inc, nil
	}
	if part != nil {
		m, err := rederiveFlows(r, rev, part.manifest, prev, pins)
		if err != nil {
			return nil, inc, err
		}
		inc.Rederived = []string{"flows"}
		return m, inc, nil
	}
	m := *prev
	m.Revision, m.Roots, m.Digest = rev, pins, ""
	if m.Digest, err = mapDigest(&m); err != nil {
		return nil, inc, err
	}
	inc.Reused = true
	return &m, inc, nil
}

// partialReuse is a change BuildFrom answers by re-deriving part of the map: only the flows
// (AMAP-V0-240), or, when sources is set, the routers, screens, test joins and flows while each
// test file's own entry is kept (AMAP-V0-245). tests is the root the test files are read from.
type partialReuse struct {
	manifest Manifest
	sources  bool
	tests    repo
}

// edit is a content change of one read path between two revisions: its blob ids and sizes before
// and after, and whether it is a flow intent (AMAP-V0-240) rather than a source (AMAP-V0-245).
type edit struct {
	path  string
	flow  bool
	added bool // a path new at the later revision (AMAP-V0-368); oids[0] is then the zero id
	oids  [2]string
	sizes [2]int
}

// reuseBlocked returns why prev cannot be carried from its revision to rev ("" when it can), how
// many paths changed across the roots it reads, the root pins the carried map has, and, when the
// only read changes are content edits of flow intents no other entry names (AMAP-V0-240) or of
// --root sources the test files' own entries cannot depend on (AMAP-V0-245), the part to re-derive.
func reuseBlocked(ctx context.Context, r repo, manifestPath string, roots Roots, prev *Map, rev string) (string, int, []RootPin, *partialReuse) {
	if !builderReuses(prev.Builder, builderIdentity) {
		return RebuildBuilderVersion, 0, nil, nil
	}
	if prev.Manifest.Repo != roots.ManifestRepo || prev.Manifest.Path != manifestPath {
		return RebuildManifestPath, 0, nil, nil
	}
	pinned := map[string]bool{}
	for _, p := range prev.Roots {
		pinned[p.Repo] = true
	}
	if roots.ManifestRepo != "" && !pinned[roots.ManifestRepo] {
		return RebuildAliasedRoots, 0, nil, nil
	}
	for alias := range roots.Revisions {
		// The manifest is unchanged or the map rebuilds, so the build reads the roots prev pins;
		// a flag revision for another root is refused by the full build, a workspace one unused.
		if !pinned[alias] && roots.Workspace == "" {
			return RebuildAliasedRoots, 0, nil, nil
		}
	}
	from, err := r.resolve(prev.Revision)
	if err != nil || from != prev.Revision {
		return RebuildFromUnreadable, 0, nil, nil
	}
	// Each root is a span from the revision prev read to the one a build reads now.
	type span struct {
		r        repo
		from, to string
		pin      int // index in prev.Roots; -1 for --root
	}
	spans := []span{{r: r, from: from, to: rev, pin: -1}}
	manifestAt := spans[0]
	for i, p := range prev.Roots {
		stated := roots.Revisions[p.Repo]
		if p.Stated != (stated != "") {
			return RebuildAliasedRoots, 0, nil, nil
		}
		ar, arev, err := openRoot(ctx, r.root, p.Repo, roots.Repos[p.Repo], stated)
		if err != nil {
			return RebuildAliasedRoots, 0, nil, nil // the full build refuses the same way
		}
		pfrom, err := ar.resolve(p.Revision)
		if err != nil || pfrom != p.Revision {
			return RebuildFromUnreadable, 0, nil, nil
		}
		spans = append(spans, span{r: ar, from: pfrom, to: arev, pin: i})
		if p.Repo == roots.ManifestRepo {
			manifestAt = spans[len(spans)-1]
		}
	}
	_, raw, err := manifestAt.r.readOne(manifestAt.from, manifestPath, maxManifestBytes)
	if err != nil {
		return RebuildFromUnreadable, 0, nil, nil
	}
	man, err := decodeManifest(raw)
	if err != nil {
		return RebuildFromUnreadable, 0, nil, nil
	}
	// Every root is judged against every input the manifest names, a superset of what the build
	// reads from any one root, so a path no input reads is unread in each.
	inputs := manifestInputs(man, manifestPath)
	pins := append([]RootPin(nil), prev.Roots...)
	changed := 0
	var edits []edit
	tests := r
	for _, s := range spans {
		var fs *flowScope
		var ss *sourceScope
		if s.pin < 0 {
			if man.Flows != "" {
				fs = flowScopeOf(man, manifestPath) // flows are read from --root only
			}
			ss = sourceScopeOf(man, manifestPath)
		}
		reason, n, es := spanBlocked(s.r, s.from, s.to, inputs, man.RouterGlobs, fs, ss)
		changed += n
		if reason != "" {
			return reason, changed, nil, nil
		}
		edits = append(edits, es...)
		if s.pin < 0 {
			continue
		}
		if prev.Roots[s.pin].Repo == man.Tests.Repo {
			tests = s.r
		}
		p := &pins[s.pin]
		if !p.Stated {
			// A HEAD-read root must be as clean as a full build requires (AMAP-V0-019).
			if dirty, err := s.r.dirty(pinPaths(man, manifestPath, p.Inputs)); err != nil || dirty {
				return RebuildRootDirty, changed, nil, nil
			}
		}
		p.Revision = s.to
	}
	if len(edits) == 0 {
		return "", changed, pins, nil
	}
	flowEdits, sources := []string{}, false
	for _, e := range edits {
		if e.flow {
			flowEdits = append(flowEdits, e.path)
		} else {
			sources = true
		}
	}
	// Tests read from an aliased root resolve their imports through that root's own index, which
	// an added --root path cannot change; a change to that root is its own span (AMAP-V0-372).
	if flowsNamedElsewhere(prev, flowEdits) || man.Tests.Repo == "" && addedNamed(prev, edits) {
		return RebuildChangedInput, changed, nil, nil
	}
	if reason := admissionBlocked(r, rev, edits); reason != "" {
		return reason, changed, nil, nil
	}
	return "", changed, pins, &partialReuse{manifest: man, sources: sources, tests: tests}
}

// admissionBlocked returns why the context index at rev may differ from the one at the earlier
// revision beyond the content of the edited paths ("" when it cannot): an edited path becomes or
// stops being an indexed source, a source edit is JSON-shaped (a config may extend it), or a
// grown source may cross the aggregate bound. Edited blobs over the 64 MiB read bound rebuild.
func admissionBlocked(r repo, rev string, edits []edit) string {
	entries, grew := []blobEntry{}, false
	for _, e := range edits {
		if !contextindex.SourcePathAdmitted(e.path) {
			grew = grew || e.added // the tree listing the index bounds grows
			continue
		}
		if e.added {
			// A new source enters the index; the aggregate bound is checked below and a
			// config-shaped one rebuilds, as for an edit (AMAP-V0-368).
			if e.sizes[1] > contextindex.SourceSizeBound {
				grew = true
				continue
			}
			grew = true
			entries = append(entries, blobEntry{path: e.path, oid: e.oids[1], size: e.sizes[1]}, blobEntry{path: e.path, oid: e.oids[1], size: e.sizes[1]})
			continue
		}
		before, after := e.sizes[0] > contextindex.SourceSizeBound, e.sizes[1] > contextindex.SourceSizeBound
		if before != after {
			return RebuildIndexAdmission
		}
		if before {
			continue // over the size bound on both sides: never a source
		}
		grew = grew || e.sizes[1] > e.sizes[0]
		entries = append(entries, blobEntry{path: e.path, oid: e.oids[0], size: e.sizes[0]}, blobEntry{path: e.path, oid: e.oids[1], size: e.sizes[1]})
	}
	data, err := r.blobs(entries, contextindex.SourceSizeBound, 64<<20)
	if err != nil {
		return RebuildChangedInput
	}
	for i := 0; i < len(entries); i += 2 {
		p, a, b := entries[i].path, data[entries[i].oid], data[entries[i+1].oid] // an added path pairs its blob with itself
		admitted := contextindex.SourceAdmitted(p, a)
		if admitted != contextindex.SourceAdmitted(p, b) {
			return RebuildIndexAdmission
		}
		if admitted && !strings.EqualFold(path.Ext(p), ".json") && (contextindex.WebConfigShaped(a) || contextindex.WebConfigShaped(b)) {
			return RebuildConfigShaped
		}
	}
	if grew && contextindex.CheckRevisionAdmission(r.ctx, r.root, rev) != nil {
		return RebuildIndexAdmission // the full build reports the refusal
	}
	return ""
}

// pinPaths lists the paths a root supplying inputs ("manifest", "tests") is read under, as
// BuildRoots computes them for its dirty check.
func pinPaths(m Manifest, manifestPath string, inputs []string) []string {
	paths := []string{}
	for _, in := range inputs {
		switch {
		case in == "manifest":
			paths = append(paths, manifestPath)
		case in == "tests" && m.Tests.Root == ".":
			t := m.Tests
			paths = append(append(append(append(paths, t.Specs...), t.PageObjects...), t.Workflows...), t.Scenarios...)
		case in == "tests":
			paths = append(paths, m.Tests.Root)
		}
	}
	return paths
}

// spanBlocked returns why a root's change from one revision to another is a build input ("" when
// it is not), how many paths changed, and the read edits among them. Only a content change of a
// regular file whose path, mode and listed size width stay put, that the context index never
// reads, and that no manifest input names, is outside the build: the index admits by suffix and
// resolves imports against every tracked path, so a removed or retyped path of any kind is a
// changed input, and an added one is unless sources.add admits it as a source edit (AMAP-V0-368). With flows set, the same content change of a `.json` file directly in that
// directory that no other manifest input names is returned as a flow edit instead (AMAP-V0-240);
// with sources set, the same content change of a path sources.edit admits is returned as a
// source edit (AMAP-V0-245).
func spanBlocked(r repo, from, rev string, inputs, globs []string, flows *flowScope, sources *sourceScope) (string, int, []edit) {
	if from == rev {
		return "", 0, nil
	}
	out, err := r.git(64<<20, nil, "diff-tree", "-r", "-z", "--no-renames", "--raw", "--no-abbrev", from, rev)
	if err != nil {
		return RebuildFromUnreadable, 0, nil
	}
	fields := bytes.Split(bytes.TrimSuffix(out, []byte{0}), []byte{0})
	if len(fields) == 1 && len(fields[0]) == 0 {
		return "", 0, nil
	}
	if len(fields)%2 != 0 {
		return RebuildFromUnreadable, 0, nil
	}
	changed := len(fields) / 2
	oids, edits := []string{}, []edit(nil)
	var added []int // indexes in edits of added paths, whose before side has no blob
	for i := 0; i < len(fields); i += 2 {
		meta, p := strings.Fields(strings.TrimPrefix(string(fields[i]), ":")), string(fields[i+1])
		if len(meta) == 5 && meta[4] == "A" && (meta[1] == "100644" || meta[1] == "100755") && sources.add(p, inputs, globs) {
			added = append(added, len(edits))
			edits = append(edits, edit{path: p, added: true, oids: [2]string{meta[2], meta[3]}})
			continue
		}
		if len(meta) != 5 || meta[4] != "M" || meta[0] != meta[1] || (meta[0] != "100644" && meta[0] != "100755") {
			return RebuildChangedInput, changed, nil
		}
		if readByBuild(p, inputs, globs) {
			flow := flows.edit(p, globs)
			if !flow && !sources.edit(p) {
				return RebuildChangedInput, changed, nil
			}
			edits = append(edits, edit{path: p, flow: flow, oids: [2]string{meta[2], meta[3]}})
		}
		oids = append(oids, meta[2], meta[3])
	}
	sizes := map[string]int{}
	if len(oids) > 0 {
		if sizes, err = r.blobSizes(oids); err != nil {
			return RebuildFromUnreadable, changed, nil
		}
	}
	for i := 0; i < len(oids); i += 2 {
		// `ls-tree -l` pads a size to seven columns; a wider size lengthens the listing the index
		// and router coverage bound.
		if sizeWidth(sizes[oids[i]]) != sizeWidth(sizes[oids[i+1]]) {
			return RebuildChangedInput, changed, nil
		}
	}
	for i := range edits {
		edits[i].sizes = [2]int{sizes[edits[i].oids[0]], sizes[edits[i].oids[1]]}
	}
	if len(added) > 0 {
		news := make([]string, 0, len(added))
		for _, i := range added {
			news = append(news, edits[i].oids[1])
		}
		if sizes, err = r.blobSizes(news); err != nil {
			return RebuildFromUnreadable, changed, nil
		}
		for _, i := range added {
			edits[i].sizes = [2]int{0, sizes[edits[i].oids[1]]}
		}
	}
	return "", changed, edits
}

// add reports whether a path added at the later revision is one the source re-derivation answers
// like an edit (AMAP-V0-368): a source edit (see edit) outside every manifest input, router glob
// and the test root, whose suffix is not one a web import resolves by probing (`.ts`, `.js`, ...),
// so only a specifier naming its whole base name could resolve to it (see addedNamed). When the
// tests are read from an aliased root, only the inputs read from --root count (AMAP-V0-372).
func (s *sourceScope) add(p string, inputs, globs []string) bool {
	if !s.edit(p) || webSuffix[strings.ToLower(path.Ext(p))] {
		return false
	}
	if s.rootInputs != nil {
		inputs = s.rootInputs
	}
	for _, in := range inputs {
		if in == "." || under(p, in) {
			return false
		}
	}
	for _, g := range globs {
		if globRoute(g, p) {
			return false
		}
	}
	return true
}

// addedNamed reports whether an added path may change how a kept test file's import resolves
// (AMAP-V0-368): some test import's specifier ends in the path's base name, cleaned or with a web
// suffix stripped (the specifier then names it, or makes it a second candidate), or a bare specifier is unresolved or resolved to a
// repository file, which a tsconfig/jsconfig `paths` mapping may point anywhere.
func addedNamed(prev *Map, edits []edit) bool {
	names := map[string]bool{}
	for _, e := range edits {
		if e.added {
			names[path.Base(e.path)] = true
		}
	}
	if len(names) == 0 {
		return false
	}
	for _, tf := range prev.Files {
		for _, imp := range tf.Imports {
			s := imp.Specifier
			bare := !strings.HasPrefix(s, ".") && !strings.HasPrefix(s, "/")
			// Resolution probes the cleaned target with a web suffix stripped (`theme.css.js`
			// probes `theme.css`), so compare that as well as the specifier's own base name.
			base := path.Base(path.Clean(s))
			stripped := strings.TrimSuffix(base, path.Ext(base))
			if !webSuffix[strings.ToLower(path.Ext(base))] {
				stripped = base
			}
			if names[path.Base(s)] || names[base] || names[stripped] || bare && (imp.Status == importUnresolved || imp.Status == importResolved) {
				return true
			}
		}
	}
	return false
}

// sourceScope is what a --root content edit may change without touching a test file's own map
// entry (AMAP-V0-245): every read path but the manifest, Git metadata, Corvint configuration, the
// Makefile and go.mod sources, any `.json` file (package manifests, tsconfig/jsconfig and their
// JSON extends targets) and, when tests are read from --root, the web sources under the test root.
type sourceScope struct {
	manifest  string
	testsRoot string // "" when the tests are read from an aliased root
	// rootInputs, set only when the tests are read from an aliased root, are the manifest inputs
	// read from --root: the manifest, routers, di_constants and flows, never a test input
	// (AMAP-V0-372).
	rootInputs []string
}

func sourceScopeOf(m Manifest, manifestPath string) *sourceScope {
	s := &sourceScope{manifest: manifestPath}
	if m.Tests.Repo == "" {
		s.testsRoot = m.Tests.Root
		return s
	}
	s.rootInputs = append([]string{manifestPath}, m.DIConstants...)
	if m.Flows != "" {
		s.rootInputs = append(s.rootInputs, m.Flows)
	}
	for _, rf := range m.Routers {
		s.rootInputs = append(s.rootInputs, rf.Path)
	}
	return s
}

// edit reports whether a content change of p is a source edit the partial re-derivation answers.
func (s *sourceScope) edit(p string) bool {
	if s == nil || p == s.manifest {
		return false
	}
	base := path.Base(p)
	if strings.HasPrefix(base, ".git") || under(p, ".corvint") || base == "Makefile" || base == "go.mod" ||
		strings.EqualFold(path.Ext(p), ".json") {
		return false
	}
	return s.testsRoot == "" || !under(p, s.testsRoot) || !webSuffix[strings.ToLower(path.Ext(p))]
}

// manifestInputs lists the paths and directories a manifest names as build inputs.
func manifestInputs(m Manifest, manifestPath string) []string {
	t := m.Tests
	in := append([]string{manifestPath, t.Root}, t.Specs...)
	in = append(append(append(append(in, t.PageObjects...), t.Workflows...), t.Scenarios...), m.DIConstants...)
	if m.Flows != "" {
		in = append(in, m.Flows)
	}
	for _, rf := range m.Routers {
		in = append(in, rf.Path)
	}
	return in
}

// flowScope is the flows directory of a manifest and every other input it names except the test
// root, which may contain the directory since tests read only JavaScript and TypeScript sources.
type flowScope struct {
	dir    string
	others []string
}

func flowScopeOf(m Manifest, manifestPath string) *flowScope {
	fs := &flowScope{dir: m.Flows}
	for _, in := range manifestInputs(m, manifestPath) {
		if in != m.Flows && in != m.Tests.Root {
			fs.others = append(fs.others, in)
		}
	}
	return fs
}

// edit reports whether p is a flow intent only the flows stage reads (AMAP-V0-240): a `.json`
// file directly in the flows directory that LoadIntentsAt loads (so not origins.json, which a
// tsconfig may extend), that no other manifest input or router glob names and that is not Git
// metadata or Corvint configuration.
func (fs *flowScope) edit(p string, globs []string) bool {
	if fs == nil || path.Dir(p) != fs.dir || path.Ext(p) != ".json" || !appflows.IntentCandidate(path.Base(p)) {
		return false
	}
	if strings.HasPrefix(path.Base(p), ".git") || under(p, ".corvint") {
		return false
	}
	for _, in := range fs.others {
		if in == "." || under(p, in) {
			return false
		}
	}
	for _, g := range globs {
		if globRoute(g, p) {
			return false
		}
	}
	return true
}

// readByBuild reports whether a build may read p: a context-index source, a path under a manifest
// input, a router_globs match, Git metadata or Corvint configuration.
func readByBuild(p string, inputs, globs []string) bool {
	base := path.Base(p)
	if contextindex.ImpactPathAdmitted(p) || base == "Makefile" || base == "go.mod" ||
		strings.HasPrefix(base, ".git") || under(p, ".corvint") {
		return true
	}
	for _, in := range inputs {
		if in == "." || under(p, in) {
			return true
		}
	}
	for _, g := range globs {
		if globRoute(g, p) {
			return true
		}
	}
	return false
}

func sizeWidth(n int) int {
	return max(7, len(fmt.Sprint(n)))
}

// blobSizes returns the size of each blob object id.
func (r repo) blobSizes(oids []string) (map[string]int, error) {
	var in bytes.Buffer
	for _, o := range oids {
		in.WriteString(o + "\n")
	}
	out, err := r.git(len(oids)*160+1024, in.Bytes(), "cat-file", "--batch-check=%(objectname) %(objecttype) %(objectsize)")
	if err != nil {
		return nil, err
	}
	sizes := map[string]int{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		var oid, kind string
		var size int
		if n, _ := fmt.Sscanf(line, "%s %s %d", &oid, &kind, &size); n != 3 || kind != "blob" {
			return nil, fmt.Errorf("blob size unreadable")
		}
		sizes[oid] = size
	}
	for _, o := range oids {
		if _, ok := sizes[o]; !ok {
			return nil, fmt.Errorf("blob size unreadable")
		}
	}
	return sizes, nil
}

// flowsNamedElsewhere reports whether any map entry other than the flows and what the flows stage
// adds (flow-step edges, step unknowns, each screen's flows, steps and preconditions) names one of
// the edited paths, so an entry that may depend on it is never kept stale (AMAP-V0-240).
func flowsNamedElsewhere(prev *Map, edits []string) bool {
	m := stripFlows(prev)
	m.Revision, m.Digest, m.Builder = "", "", ""
	raw, err := json.Marshal(m)
	if err != nil {
		return true
	}
	for _, p := range edits {
		if bytes.Contains(raw, []byte(p)) {
			return true
		}
	}
	return false
}

// stripFlows returns prev without what the flows stage derives: its flows, flow-step edges and
// step unknowns, and each screen's flows, steps and preconditions, as the map stands before the
// flows stage of a full build. prev itself is not changed.
func stripFlows(prev *Map) *Map {
	m := *prev
	m.Flows = []Flow{}
	m.Screens = make([]Screen, len(prev.Screens))
	for i, s := range prev.Screens {
		s.Flows, s.Steps, s.Preconditions = []string{}, []string{}, []Requirement{}
		m.Screens[i] = s
	}
	m.Edges = []Edge{}
	for _, e := range prev.Edges {
		if e.Basis != "flow-step" {
			m.Edges = append(m.Edges, e)
		}
	}
	m.Unknowns = []Unknown{}
	for _, u := range prev.Unknowns {
		if u.Kind != "step" {
			m.Unknowns = append(m.Unknowns, u)
		}
	}
	return &m
}

// rederiveFlows keeps every entry of prev but the flows and re-reads the flows at rev, giving the
// map a full build at rev gives when only flow intents changed (AMAP-V0-240).
func rederiveFlows(r repo, rev string, man Manifest, prev *Map, pins []RootPin) (*Map, error) {
	out := stripFlows(prev)
	out.Revision, out.Roots, out.Builder, out.Digest = rev, pins, builderIdentity, ""
	b := &builder{m: man, rev: rev, out: out, screens: map[string]*Screen{}, files: map[string]*TestFile{}}
	for i := range out.Screens {
		b.screens[out.Screens[i].ID] = &out.Screens[i]
	}
	b.idx = newScreenIndex(man.HashPrefix, man.BasePath, out.Screens)
	for i := range prev.Files {
		tf := prev.Files[i]
		b.files[tf.Path], b.order = &tf, append(b.order, tf.Path)
	}
	out.Files = []TestFile{}
	if err := b.flows(r.ctx, r, r.root); err != nil {
		return nil, err
	}
	return b.finish()
}

// routersNeedIndex reports whether the routers stage reads the context index: di_constants, or a
// router of the default dialect, whose constant tables resolve through it.
func routersNeedIndex(m Manifest) bool {
	if len(m.DIConstants) > 0 {
		return true
	}
	for _, rf := range m.Routers {
		if rf.Dialect != DialectRails && rf.Dialect != DialectReact {
			return true
		}
	}
	return false
}

// rederiveSources gives the map a full build at rev gives when only sources outside the test
// files changed (AMAP-V0-245): the routers, screens and their unknowns are derived again, each test
// file's own entry and the tests stage's unknowns are kept from prev (they read only the unchanged
// test files and an index whose source set and configs are unchanged), and the join and flows run
// again over them. The context index is built only when the routers read it.
func rederiveSources(r repo, rev string, part *partialReuse, prev *Map, pins []RootPin) (*Map, error) {
	man := part.manifest
	b := &builder{m: man, rev: rev, out: &Map{Schema: MapSchema, Builder: builderIdentity, App: man.App, Revision: rev, Roots: pins,
		HashPrefix: man.HashPrefix, BasePath: man.BasePath, Manifest: prev.Manifest,
		Edges: []Edge{}, Flows: []Flow{}, Files: []TestFile{}, Unknowns: []Unknown{}}}
	var ix *contextindex.Index
	if routersNeedIndex(man) {
		var err error
		if ix, err = contextindex.BuildRevisionContext(r.ctx, r.root, rev); err != nil {
			return nil, err
		}
	}
	if err := b.routers(r, ix); err != nil {
		return nil, err
	}
	if err := b.keepTests(part.tests, prev); err != nil {
		return nil, err
	}
	b.join()
	if man.Flows != "" {
		if err := b.flows(r.ctx, r, r.root); err != nil {
			return nil, err
		}
	}
	return b.finish()
}

// keepTests restores the tests stage's output from prev: each test file as tests() left it before
// the join (no screen binding; Join resolved when the join reported test-join for it), the facts,
// bindings and sources the join reads re-read from its pinned blob, its navigations placed again
// on the screens just derived, and the tests stage's unknowns.
func (b *builder) keepTests(tr repo, prev *Map) error {
	joined := map[string]bool{}
	for _, u := range prev.Unknowns {
		if testStageUnknown(u) {
			b.out.Unknowns = append(b.out.Unknowns, u)
		}
		if u.Kind == "test-join" {
			joined[u.Ref] = true
		}
	}
	b.files, b.facts, b.binds = map[string]*TestFile{}, map[string]fileFacts{}, map[string][]contextindex.WebImportBinding{}
	b.sources = map[string][]byte{}
	if len(prev.Files) == 0 {
		return nil
	}
	oids := make([]string, 0, len(prev.Files))
	for _, tf := range prev.Files {
		oids = append(oids, tf.Anchor.Blob)
	}
	sizes, err := tr.blobSizes(oids)
	if err != nil {
		return err
	}
	entries := make([]blobEntry, 0, len(prev.Files))
	for _, tf := range prev.Files {
		entries = append(entries, blobEntry{repo: tf.Anchor.Repo, path: tf.Path, oid: tf.Anchor.Blob, size: sizes[tf.Anchor.Blob]})
	}
	data, err := tr.blobs(entries, maxTestFileBytes, maxTestBytes+128*len(entries))
	if err != nil {
		return err
	}
	for i := range prev.Files {
		tf := prev.Files[i]
		text := string(data[tf.Anchor.Blob])
		tf.Screen, tf.ScreenBasis = "", ""
		if joined[tf.ID] {
			tf.Join = StatusResolved
		}
		facts := readFacts(text)
		tf.Gotos = b.gotos(facts)
		b.files[tf.Path], b.facts[tf.Path], b.binds[tf.Path], b.sources[tf.Path] = &tf, facts, contextindex.WebImportBindings(text), []byte(text)
		b.order = append(b.order, tf.Path)
	}
	return nil
}
