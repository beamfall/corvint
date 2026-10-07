package appmap

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
)

var webSuffix = map[string]bool{".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".mjs": true, ".cjs": true}

// Build compiles the application-map/0 artifact for the manifest committed at revision
// (AMAP-V0-001..009, AMAP-V0-015). It reads committed objects only and writes nothing.
func Build(ctx context.Context, root, manifestPath, revision string) (*Map, error) {
	r := repo{ctx: ctx, root: root}
	rev, err := r.resolve(revision)
	if err != nil {
		return nil, &gokernel.Error{Code: "appmap-invalid-revision", Message: err.Error()}
	}
	if !safeRelative(manifestPath) {
		return nil, invalidManifest("--manifest must be a repository-relative path")
	}
	me, raw, err := r.readOne(rev, manifestPath, maxManifestBytes)
	if err != nil {
		return nil, invalidManifest("manifest: %v", err)
	}
	m, err := decodeManifest(raw)
	if err != nil {
		return nil, err
	}
	// A declared directory absent at the revision would silently drop its files from the join.
	dirs := append([]string{m.Tests.Root}, m.Tests.Specs...)
	dirs = append(append(append(dirs, m.Tests.PageObjects...), m.Tests.Workflows...), m.Tests.Scenarios...)
	if m.Flows != "" {
		dirs = append(dirs, m.Flows)
	}
	missing, err := r.missingDirs(rev, dirs)
	if err != nil {
		return nil, invalidManifest("declared directories unreadable: %v", err)
	}
	if len(missing) > 0 {
		return nil, invalidManifest("declared directory %q is not a directory at the evaluated revision", missing[0])
	}
	b := &builder{m: m, rev: rev, out: &Map{Schema: MapSchema, App: m.App, Revision: rev, HashPrefix: m.HashPrefix, Manifest: wholeFile(me, raw),
		Edges: []Edge{}, Flows: []Flow{}, Files: []TestFile{}, Unknowns: []Unknown{}}}
	if err = b.routers(r); err != nil {
		return nil, err
	}
	ix, err := contextindex.BuildRevisionContext(ctx, root, rev)
	if err != nil {
		return nil, err
	}
	if err = b.tests(ix); err != nil {
		return nil, err
	}
	b.join()
	if m.Flows != "" {
		if err = b.flows(ctx, r, root); err != nil {
			return nil, err
		}
	}
	return b.finish()
}

type builder struct {
	m       Manifest
	rev     string
	out     *Map
	idx     *screenIndex
	screens map[string]*Screen
	files   map[string]*TestFile
	facts   map[string]fileFacts
	binds   map[string][]contextindex.WebImportBinding
	sources map[string][]byte
	order   []string
}

func (b *builder) unknown(u Unknown) { b.out.Unknowns = append(b.out.Unknowns, u) }

func (b *builder) routers(r repo) error {
	paths := []string{}
	for _, rf := range b.m.Routers {
		paths = append(paths, rf.Path)
	}
	entries, err := r.tree(b.rev, paths, maxRouterFiles)
	if err != nil {
		return err
	}
	byPath := map[string]blobEntry{}
	for _, e := range entries {
		byPath[e.path] = e
	}
	ordered := []blobEntry{}
	for _, p := range paths {
		e, ok := byPath[p]
		if !ok {
			return invalidManifest("router %s is absent at the evaluated revision", p)
		}
		ordered = append(ordered, e)
	}
	data, err := r.blobs(ordered, maxRouterBytes, maxRouterFiles*maxRouterBytes)
	if err != nil {
		return err
	}
	raws := []rawState{}
	for _, e := range ordered {
		states, unknowns := parseRouter(e, data[e.oid])
		raws = append(raws, states...)
		b.out.Unknowns = append(b.out.Unknowns, unknowns...)
		if len(raws) > maxStates {
			return bound(fmt.Sprintf("more than %d router states", maxStates))
		}
	}
	screens, unknowns := resolveScreens(b.m.App, raws)
	b.out.Screens = screens
	b.out.Unknowns = append(b.out.Unknowns, unknowns...)
	b.screens = map[string]*Screen{}
	for i := range b.out.Screens {
		b.screens[b.out.Screens[i].ID] = &b.out.Screens[i]
	}
	b.idx = newScreenIndex(b.m.HashPrefix, screens)
	for _, k := range b.idx.ambiguousKeys() {
		b.unknown(Unknown{Kind: "template", Ref: k, Reason: "ambiguous-template"})
	}
	return nil
}

// role names the manifest directory a test file sits in; the deepest declared directory wins.
func (b *builder) role(p string) string {
	best, role := "", roleSupport
	t := b.m.Tests
	for _, g := range []struct {
		dirs []string
		role string
	}{{t.Specs, roleSpec}, {t.PageObjects, rolePageObject}, {t.Workflows, roleWorkflow}, {t.Scenarios, roleScenario}} {
		for _, d := range g.dirs {
			if under(p, d) && len(d) > len(best) {
				best, role = d, g.role
			}
		}
	}
	return role
}

func (b *builder) tests(ix *contextindex.Index) error {
	rootDir := b.m.Tests.Root
	for _, ex := range ix.Exclusions {
		if under(ex.Path, rootDir) && webSuffix[strings.ToLower(path.Ext(ex.Path))] {
			b.unknown(Unknown{Kind: "file", Ref: fileID(ex.Path), Reason: "excluded-by-index", Path: ex.Path})
		}
	}
	unparsed := map[string]bool{}
	for _, u := range ix.Unparsed {
		if under(u.Path, rootDir) {
			unparsed[u.Path] = true
		}
	}
	packages := declaredPackages(ix)
	paths := []string{}
	for p := range ix.Sources {
		if under(p, rootDir) && webSuffix[strings.ToLower(path.Ext(p))] {
			paths = append(paths, p)
		}
	}
	if len(paths) > maxTestFiles {
		return bound(fmt.Sprintf("more than %d test files under %s", maxTestFiles, rootDir))
	}
	sort.Strings(paths)
	b.files, b.facts, b.binds = map[string]*TestFile{}, map[string]fileFacts{}, map[string][]contextindex.WebImportBinding{}
	b.sources = map[string][]byte{}
	total := 0
	for _, p := range paths {
		src := ix.Sources[p]
		text, valid, loaded := src.Text()
		if !loaded || !valid {
			b.unknown(Unknown{Kind: "file", Ref: fileID(p), Reason: "unparsed-imports", Path: p})
			continue
		}
		if len(text) > maxTestFileBytes {
			return bound(fmt.Sprintf("%s exceeds %d bytes", p, maxTestFileBytes))
		}
		if total += len(text); total > maxTestBytes {
			return bound(fmt.Sprintf("test sources exceed %d bytes", maxTestBytes))
		}
		e := blobEntry{path: p, oid: src.BlobHash, size: len(text)}
		data := []byte(text)
		facts := readFacts(text)
		binds := contextindex.WebImportBindings(text)
		tf := &TestFile{ID: fileID(p), Path: p, Role: b.role(p), Class: facts.class, Join: StatusResolved, Anchor: wholeFile(e, data),
			Imports: []Import{}, Methods: []Method{}, Gotos: []Goto{}, Tests: append([]string{}, facts.tests...),
			Assertions: facts.asserts, Selectors: append([]Selector{}, facts.selectors...)}
		if unparsed[p] {
			tf.Join = StatusUnknown
			b.unknown(Unknown{Kind: "file", Ref: tf.ID, Reason: "unparsed-imports", Path: p})
		}
		specifiers := []string{}
		for s := range ix.Imports[p] {
			specifiers = append(specifiers, s)
		}
		sort.Strings(specifiers)
		for _, s := range specifiers {
			imp := Import{Specifier: s, Names: []string{}}
			for _, bd := range binds {
				if bd.Module == s {
					imp.Names = append(imp.Names, bd.Local)
					imp.Line = bd.Line
				}
			}
			if imp.Line == 0 {
				imp.Line = importLine(text, s)
			}
			imp.Statement = statementAt(text, imp.Line, s)
			if target := contextindex.ResolveWebImport(ix, p, s); target != "" {
				imp.Status, imp.Resolved = importResolved, target
			} else {
				imp.Status = importStatus(s, packages)
			}
			if imp.Status == importResolved && !under(imp.Resolved, rootDir) && webSuffix[strings.ToLower(path.Ext(imp.Resolved))] {
				// A first-party module outside tests.root is not read, yet it may import a page
				// object, so the chain through it is not known to be complete.
				tf.Join = StatusUnknown
				b.unknown(Unknown{Kind: "import", Ref: tf.ID, Reason: "import-outside-tests", Path: p, Line: imp.Line})
			}
			if imp.Status == importUnresolved {
				tf.Join = StatusUnknown
				b.unknown(Unknown{Kind: "import", Ref: tf.ID, Reason: "unresolved-import", Path: p, Line: imp.Line})
			}
			tf.Imports = append(tf.Imports, imp)
		}
		for _, rm := range facts.methods {
			meth := Method{ID: methodID(p, rm.name), Name: rm.name, Anchor: spanOf(e, data, rm.start, rm.end), Selectors: []Selector{}}
			for _, s := range facts.selectors {
				if s.Line >= rm.start && s.Line <= rm.end {
					meth.Selectors = append(meth.Selectors, s)
				}
			}
			tf.Methods = append(tf.Methods, meth)
		}
		for _, g := range facts.gotos {
			gt := Goto{Line: g.line, URL: strings.ReplaceAll(g.url, substMark, "${}"), Status: StatusUnknown, Reason: g.reason}
			if g.reason == "" {
				if id, reason := b.idx.byURL(g.url); id != "" {
					gt.Screen, gt.Status = id, StatusResolved
				} else {
					gt.Reason = reason
				}
			}
			tf.Gotos = append(tf.Gotos, gt)
		}
		for _, line := range facts.secrets {
			b.unknown(Unknown{Kind: "selector", Ref: refAt(p, line), Reason: "secret-shaped", Path: p, Line: line})
		}
		b.files[p], b.facts[p], b.binds[p], b.sources[p] = tf, facts, binds, data
		b.order = append(b.order, p)
	}
	return nil
}

// declaredPackages collects every dependency name the repository's package manifests declare at
// the evaluated revision. An unreadable manifest contributes nothing, so imports of its packages
// stay unresolved rather than external.
func declaredPackages(ix *contextindex.Index) map[string]bool {
	out := map[string]bool{}
	for p, src := range ix.Sources {
		if path.Base(p) != "package.json" {
			continue
		}
		text, valid, loaded := src.Text()
		if !loaded || !valid {
			continue
		}
		var doc map[string]json.RawMessage
		if json.Unmarshal([]byte(text), &doc) != nil {
			continue
		}
		for _, field := range []string{"dependencies", "devDependencies", "peerDependencies", "optionalDependencies"} {
			var deps map[string]json.RawMessage
			if json.Unmarshal(doc[field], &deps) == nil {
				for name := range deps {
					out[name] = true
				}
			}
		}
	}
	return out
}

// join binds page objects to screens and attributes specs, workflows and scenarios to screens
// through the import graph (AMAP-V0-005, AMAP-V0-006, AMAP-V0-008).
func (b *builder) join() {
	declared := map[string]string{}
	for _, d := range b.m.PageObjectScreens {
		declared[d.Path] = d.State
	}
	for _, p := range b.order {
		tf := b.files[p]
		if tf.Role != rolePageObject {
			continue
		}
		if state, ok := declared[p]; ok {
			id := screenID(b.m.App, state)
			if s := b.screens[id]; s != nil && s.Status == StatusResolved {
				tf.Screen, tf.ScreenBasis = id, "declared"
			} else {
				b.unknown(Unknown{Kind: "page-object", Ref: tf.ID, Reason: "unknown-state", Path: p})
			}
			continue
		}
		targets, unresolved := map[string]bool{}, false
		for _, g := range tf.Gotos {
			if g.Screen != "" {
				targets[g.Screen] = true
			} else {
				unresolved = true
			}
		}
		switch {
		case unresolved:
			// a target the map cannot place may be a second screen, so no single binding is known
			b.unknown(Unknown{Kind: "page-object", Ref: tf.ID, Reason: "page-object-unresolved-target", Path: p})
		case len(targets) == 1:
			for id := range targets {
				tf.Screen, tf.ScreenBasis = id, "page-object-url"
			}
		case len(targets) == 0:
			b.unknown(Unknown{Kind: "page-object", Ref: tf.ID, Reason: "page-object-unbound", Path: p})
		default:
			b.unknown(Unknown{Kind: "page-object", Ref: tf.ID, Reason: "page-object-ambiguous", Path: p})
		}
	}
	for p := range declared {
		if b.files[p] == nil {
			b.unknown(Unknown{Kind: "page-object", Ref: fileID(p), Reason: "page-object-unbound", Path: p})
		}
	}
	for _, p := range b.order {
		tf := b.files[p]
		if tf.Role == rolePageObject && tf.Screen != "" {
			s := b.screens[tf.Screen]
			s.PageObjects = append(s.PageObjects, p)
		}
		if tf.Role != roleSpec && tf.Role != roleWorkflow {
			continue
		}
		reached, complete, truncated := b.closure(p)
		if truncated {
			b.unknown(Unknown{Kind: "import", Ref: tf.ID, Reason: "import-depth-exceeded", Path: p})
		}
		if !complete && tf.Join == StatusResolved {
			tf.Join = StatusUnknown
			b.unknown(Unknown{Kind: "test-join", Ref: tf.ID, Reason: "unresolved-import", Path: p})
		}
		screens := map[string]Attribution{}
		scenarios := []string{}
		for _, target := range sortedKeys(reached) {
			rf := b.files[target]
			if rf.Role == rolePageObject && rf.Screen != "" {
				if _, seen := screens[rf.Screen]; !seen {
					screens[rf.Screen] = Attribution{File: p, Basis: "import", Via: reached[target]}
				}
			}
			if rf.Role == roleScenario {
				scenarios = append(scenarios, target)
			}
		}
		for _, g := range tf.Gotos {
			if _, seen := screens[g.Screen]; g.Screen != "" && !seen {
				screens[g.Screen] = Attribution{File: p, Basis: "goto", Via: []string{refAt(p, g.Line)}}
			}
		}
		for _, id := range sortedKeys(screens) {
			s := b.screens[id]
			if tf.Role == roleSpec {
				s.Specs = append(s.Specs, screens[id])
				s.Scenarios = append(s.Scenarios, scenarios...)
			} else {
				s.Workflows = append(s.Workflows, p)
			}
		}
		b.sequence(tf)
	}
	for i := range b.out.Screens {
		s := &b.out.Screens[i]
		s.Scenarios = uniqueSorted(s.Scenarios)
	}
}

// closure walks resolved imports breadth-first from p, at most maxImportHops deep, and returns
// each reached test file with the import chain that reached it. complete is false when any file
// on the way has an unresolved import.
func (b *builder) closure(p string) (map[string][]string, bool, bool) {
	reached := map[string][]string{}
	complete := b.files[p].Join == StatusResolved
	frontier := []string{p}
	chains := map[string][]string{p: {}}
	truncated := false
	for depth := 0; len(frontier) > 0; depth++ {
		nextFrontier := []string{}
		for _, cur := range frontier {
			for _, imp := range b.files[cur].Imports {
				t := imp.Resolved
				if t == "" || b.files[t] == nil {
					continue
				}
				if _, seen := chains[t]; seen {
					continue
				}
				if depth >= maxImportHops {
					truncated = true
					continue
				}
				chain := append(append([]string{}, chains[cur]...), t)
				chains[t], reached[t] = chain, chain
				if b.files[t].Join != StatusResolved {
					complete = false
				}
				nextFrontier = append(nextFrontier, t)
			}
		}
		sort.Strings(nextFrontier)
		frontier = nextFrontier
	}
	return reached, complete && !truncated, truncated
}

// sequence adds test-sequence edges: the screens of the page objects a spec or workflow
// constructs, in source order (AMAP-V0-008).
func (b *builder) sequence(tf *TestFile) {
	classes := map[string]string{}
	for _, bd := range b.binds[tf.Path] {
		for _, imp := range tf.Imports {
			if imp.Specifier == bd.Module && imp.Resolved != "" {
				classes[bd.Local] = imp.Resolved
			}
		}
	}
	type use struct {
		screen string
		line   int
	}
	uses := []use{}
	for _, n := range b.facts[tf.Path].news {
		target := classes[n.class]
		if target == "" || b.files[target] == nil || b.files[target].Screen == "" {
			continue
		}
		if len(uses) > 0 && uses[len(uses)-1].screen == b.files[target].Screen {
			continue
		}
		uses = append(uses, use{b.files[target].Screen, n.line})
	}
	e := blobEntry{path: tf.Path, oid: tf.Anchor.Blob}
	for i := 1; i < len(uses); i++ {
		anchor := spanOf(e, b.sources[tf.Path], uses[i-1].line, uses[i].line)
		b.out.Edges = append(b.out.Edges, Edge{From: uses[i-1].screen, To: uses[i].screen, Basis: "test-sequence", Source: tf.ID, Anchor: anchor})
	}
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// flows projects the AFU-V1 intents' navigation steps onto the screen graph (AMAP-V0-004).
func (b *builder) flows(ctx context.Context, r repo, root string) error {
	set, err := appflows.LoadIntentsAt(ctx, root, b.m.Flows, b.rev)
	if err != nil {
		return &gokernel.Error{Code: "appmap-invalid-flows", Message: err.Error()}
	}
	entries, err := r.tree(b.rev, []string{b.m.Flows}, appflows.MaxFlows+2)
	if err != nil {
		return err
	}
	anchors := map[string]Anchor{}
	wanted := []blobEntry{}
	for _, e := range entries {
		if path.Dir(e.path) == b.m.Flows && strings.HasSuffix(e.path, ".json") {
			wanted = append(wanted, e)
		}
	}
	data, err := r.blobs(wanted, appflows.MaxBytes, len(wanted)*(appflows.MaxBytes+128)+1)
	if err != nil {
		return err
	}
	for _, e := range wanted {
		anchors[e.path] = wholeFile(e, data[e.oid])
	}
	reuse := b.reuseIndex()
	for _, intent := range set.Flows {
		fl := Flow{ID: "flow:" + intent.FlowID, FlowID: intent.FlowID, Preconditions: append([]string{}, intent.Preconditions...),
			Outcomes: []Outcome{}, Steps: []Step{}, Anchor: anchors[set.IntentPath(intent.FlowID)]}
		for _, o := range intent.Outcomes {
			fl.Outcomes = append(fl.Outcomes, Outcome{ID: o.OutcomeID, Behavior: o.Behavior, Matcher: o.Matcher, Locator: o.Locator, Value: o.Value})
		}
		nav := map[string]appflows.NavStep{}
		requires := []string{}
		if intent.Navigation != nil {
			for _, s := range intent.Navigation.Steps {
				nav[s.StepID] = s
			}
			for _, pf := range intent.Navigation.PreconditionFlows {
				requires = append(requires, "flow:"+pf)
			}
		}
		prev := ""
		for _, fs := range intent.Steps {
			st := Step{ID: "step:" + intent.FlowID + "/" + fs.StepID, Action: fs.Action, Status: StatusUnknown, Reuse: []string{}}
			ns, ok := nav[fs.StepID]
			if !ok || intent.Kind != "ui" {
				st.Reason = "no-navigation-step"
				b.unknown(Unknown{Kind: "step", Ref: st.ID, Reason: st.Reason, Path: fl.Anchor.Path})
				fl.Steps = append(fl.Steps, st)
				prev = "" // an unplaced step breaks the chain: no edge is inferred across it
				continue
			}
			var sel Selector
			if ns.Locator.TestID != "" {
				sel = newSelector("test-id", ns.Locator.TestID, "", strengthStrong, 0)
			} else {
				sel = newSelector("role", ns.Locator.Role, ns.Locator.Name, strengthMedium, 0)
			}
			st.Selector = &sel
			st.Reuse = append(st.Reuse, reuse[sel.ID]...)
			if id, reason := b.idx.byTemplate(ns.State); id != "" {
				st.Screen, st.Status = id, StatusResolved
				s := b.screens[id]
				s.Steps = append(s.Steps, st.ID)
				s.Flows = append(s.Flows, fl.ID)
				for _, pre := range append(append([]string{}, intent.Preconditions...), requires...) {
					s.Preconditions = append(s.Preconditions, Requirement{Flow: fl.ID, Text: pre})
				}
				if prev != "" && prev != id {
					b.out.Edges = append(b.out.Edges, Edge{From: prev, To: id, Basis: "flow-step", Source: st.ID, Anchor: fl.Anchor})
				}
				prev = id
			} else {
				st.Reason = reason
				b.unknown(Unknown{Kind: "step", Ref: st.ID, Reason: reason, Path: fl.Anchor.Path})
				prev = ""
			}
			fl.Steps = append(fl.Steps, st)
		}
		b.out.Flows = append(b.out.Flows, fl)
	}
	for i := range b.out.Screens {
		s := &b.out.Screens[i]
		s.Flows = uniqueSorted(s.Flows)
		sort.SliceStable(s.Preconditions, func(x, y int) bool {
			if s.Preconditions[x].Flow != s.Preconditions[y].Flow {
				return s.Preconditions[x].Flow < s.Preconditions[y].Flow
			}
			return s.Preconditions[x].Text < s.Preconditions[y].Text
		})
		s.Preconditions = uniqueRequirements(s.Preconditions)
	}
	return nil
}

func uniqueRequirements(in []Requirement) []Requirement {
	out := []Requirement{}
	for i, r := range in {
		if i == 0 || r != in[i-1] {
			out = append(out, r)
		}
	}
	return out
}

// reuseIndex maps each readable selector ID to the code that already performs it: page-object and
// workflow methods first, then the spec lines that use it outside any method.
func (b *builder) reuseIndex() map[string][]string {
	methods, lines := map[string][]string{}, map[string][]string{}
	for _, p := range b.order {
		tf := b.files[p]
		inMethod := map[int]bool{}
		for _, m := range tf.Methods {
			for _, s := range m.Selectors {
				inMethod[s.Line] = true
				if s.Strength != strengthUnknown {
					methods[s.ID] = append(methods[s.ID], m.ID)
				}
			}
		}
		for _, s := range tf.Selectors {
			// An unreadable selector has no value to compare, so it never counts as reuse.
			if !inMethod[s.Line] && s.Strength != strengthUnknown {
				lines[s.ID] = append(lines[s.ID], refAt(p, s.Line))
			}
		}
	}
	out := map[string][]string{}
	for id, ms := range methods {
		out[id] = uniqueSorted(ms)
	}
	for id, ls := range lines {
		out[id] = append(out[id], uniqueSorted(ls)...)
	}
	return out
}

// finish orders every list and seals the artifact digest (AMAP-V0-009).
func (b *builder) finish() (*Map, error) {
	for _, p := range b.order {
		b.out.Files = append(b.out.Files, *b.files[p])
	}
	sort.SliceStable(b.out.Edges, func(i, j int) bool {
		x, y := b.out.Edges[i], b.out.Edges[j]
		if x.From != y.From {
			return x.From < y.From
		}
		if x.To != y.To {
			return x.To < y.To
		}
		if x.Basis != y.Basis {
			return x.Basis < y.Basis
		}
		return x.Source < y.Source
	})
	sort.SliceStable(b.out.Unknowns, func(i, j int) bool { return unknownLess(b.out.Unknowns[i], b.out.Unknowns[j]) })
	dedup := []Unknown{}
	for i, u := range b.out.Unknowns {
		if i == 0 || u != b.out.Unknowns[i-1] {
			dedup = append(dedup, u)
		}
	}
	b.out.Unknowns = dedup
	d, err := mapDigest(b.out)
	if err != nil {
		return nil, err
	}
	b.out.Digest = d
	return b.out, nil
}

func unknownLess(x, y Unknown) bool {
	if x.Kind != y.Kind {
		return x.Kind < y.Kind
	}
	if x.Ref != y.Ref {
		return x.Ref < y.Ref
	}
	if x.Reason != y.Reason {
		return x.Reason < y.Reason
	}
	if x.Path != y.Path {
		return x.Path < y.Path
	}
	return x.Line < y.Line
}

// mapDigest is the SHA-256 of the artifact's canonical JSON with an empty digest member.
func mapDigest(m *Map) (string, error) {
	c := *m
	c.Digest = ""
	raw, err := json.Marshal(&c)
	if err != nil {
		return "", err
	}
	return digest(raw), nil
}

// Encode writes the artifact as one JSON line.
func Encode(m *Map) ([]byte, error) {
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	if len(raw) > maxMapBytes {
		return nil, bound(fmt.Sprintf("application map exceeds %d bytes", maxMapBytes))
	}
	return append(raw, '\n'), nil
}
