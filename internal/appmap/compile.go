package appmap

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
)

var webSuffix = map[string]bool{".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".mjs": true, ".cjs": true}

// Roots binds manifest root aliases to operator-declared roots (AMAP-V0-017): Repos maps each
// alias to its ABSOLUTE_ROOT, and ManifestRepo names the alias whose HEAD holds the manifest
// (empty: --root at the evaluated revision). Revisions maps an alias to a stated revision read in
// place of its HEAD (AMAP-V0-200).
type Roots struct {
	Repos        map[string]string
	ManifestRepo string
	Revisions    map[string]string
	// Workspace is the corvint-workspace/0 file Resolve bound these roots from, if any
	// (WSD-V0-004): its revisions are defaults, so one for an alias this build reads nothing
	// from is unused rather than refused.
	Workspace string

	workspaceFlag string // --workspace FILE
	noWorkspace   bool   // --no-workspace
}

// Build compiles the application-map/0 artifact for the manifest committed at revision
// (AMAP-V0-001..009, AMAP-V0-015). It reads committed objects only and writes nothing.
func Build(ctx context.Context, root, manifestPath, revision string) (*Map, error) {
	return BuildRoots(ctx, root, manifestPath, revision, Roots{})
}

// BuildRoots is Build with aliased roots (AMAP-V0-017, AMAP-V0-018). Routers and flows are read
// from root at revision; the manifest and the tests may each come from an aliased root at its
// pinned HEAD. Without aliases it is Build byte for byte.
func BuildRoots(ctx context.Context, root, manifestPath, revision string, roots Roots) (*Map, error) {
	r := repo{ctx: ctx, root: root}
	rev, err := r.resolve(revision)
	if err != nil {
		return nil, orPlatform(err, &gokernel.Error{Code: "appmap-invalid-revision", Message: err.Error()})
	}
	if !safeRelative(manifestPath) {
		return nil, invalidManifest("--manifest must be a repository-relative path")
	}
	type opened struct {
		r      repo
		rev    string
		inputs map[string][]string // input name -> paths read from this root
	}
	aliased := map[string]*opened{}
	at := func(alias string) (repo, string, error) {
		if alias == "" {
			return r, rev, nil
		}
		if o := aliased[alias]; o != nil {
			return o.r, o.rev, nil
		}
		ar, arev, err := openRoot(ctx, root, alias, roots.Repos[alias], roots.Revisions[alias])
		if err != nil {
			return repo{}, "", err
		}
		aliased[alias] = &opened{r: ar, rev: arev, inputs: map[string][]string{}}
		return ar, arev, nil
	}
	mr, mrev, err := at(roots.ManifestRepo)
	if err != nil {
		return nil, err
	}
	me, raw, err := mr.readOne(mrev, manifestPath, maxManifestBytes)
	if err != nil {
		return nil, invalidManifest("manifest: %v", err)
	}
	m, err := decodeManifest(raw)
	if err != nil {
		return nil, err
	}
	tr, trev, err := at(m.Tests.Repo)
	if err != nil {
		return nil, err
	}
	// A declared directory absent at the revision would silently drop its files from the join.
	dirs := append([]string{}, m.Tests.Specs...)
	if m.Tests.Root != "." {
		dirs = append([]string{m.Tests.Root}, dirs...) // "." is the repository itself (AMAP-V0-075)
	}
	dirs = append(append(append(dirs, m.Tests.PageObjects...), m.Tests.Workflows...), m.Tests.Scenarios...)
	if m.Tests.WebRoot != "" && m.Tests.WebRoot != "." {
		dirs = append(dirs, m.Tests.WebRoot) // "." is the repository itself
	}
	if m.Flows != "" && m.Tests.Repo == "" {
		dirs = append(dirs, m.Flows)
	}
	missing, err := tr.missingDirs(trev, dirs)
	if err == nil && len(missing) == 0 && m.Flows != "" && m.Tests.Repo != "" {
		missing, err = r.missingDirs(rev, []string{m.Flows})
	}
	if err != nil {
		return nil, invalidManifest("declared directories unreadable: %v", err)
	}
	if len(missing) > 0 {
		return nil, invalidManifest("declared directory %q is not a directory at the evaluated revision", missing[0])
	}
	if o := aliased[roots.ManifestRepo]; o != nil {
		o.inputs["manifest"] = []string{manifestPath}
	}
	if o := aliased[m.Tests.Repo]; o != nil {
		o.inputs["tests"] = []string{m.Tests.Root}
		if m.Tests.Root == "." {
			// A whole-repository root reads only the declared directories, so only they can be dirty (AMAP-V0-076).
			t := m.Tests
			o.inputs["tests"] = append(append(append(append([]string{}, t.Specs...), t.PageObjects...), t.Workflows...), t.Scenarios...)
		}
	}
	b := &builder{m: m, rev: rev, out: &Map{Schema: MapSchema, Builder: builderIdentity, App: m.App, Revision: rev, HashPrefix: m.HashPrefix, BasePath: m.BasePath, Manifest: wholeFile(me, raw),
		Edges: []Edge{}, Flows: []Flow{}, Files: []TestFile{}, Unknowns: []Unknown{}}}
	// An aliased root is read at HEAD, so an uncommitted change under what the map reads from it
	// would make the map silently disagree with that checkout (AMAP-V0-019). A stated revision is
	// read from committed objects only, so its checkout state cannot change the map (AMAP-V0-200).
	for alias := range roots.Revisions {
		if aliased[alias] == nil && roots.Workspace == "" {
			return nil, rootUnavailable(alias, "has a --repo-revision but the manifest reads nothing from it")
		}
	}
	for _, alias := range sortedKeys(aliased) {
		o, paths := aliased[alias], []string{}
		for _, name := range sortedKeys(o.inputs) {
			paths = append(paths, o.inputs[name]...)
		}
		if roots.Revisions[alias] != "" {
			b.out.Roots = append(b.out.Roots, RootPin{Repo: alias, Revision: o.rev, Inputs: sortedKeys(o.inputs), Stated: true})
			continue
		}
		dirty, err := o.r.dirty(paths)
		if err != nil {
			return nil, rootUnavailable(alias, "worktree status is unreadable")
		}
		if dirty {
			return nil, &gokernel.Error{Code: "appmap-root-dirty", Message: fmt.Sprintf("root %s has uncommitted changes under %s; commit or stash them", alias, strings.Join(paths, ", "))}
		}
		b.out.Roots = append(b.out.Roots, RootPin{Repo: alias, Revision: o.rev, Inputs: sortedKeys(o.inputs)})
	}
	// Routers are read from the primary root, so their constant tables resolve through its index;
	// tests read from an aliased root are joined through that root's own index.
	ix, err := contextindex.BuildRevisionContext(ctx, root, rev)
	if err != nil {
		return nil, err
	}
	if err = b.routers(r, ix); err != nil {
		return nil, err
	}
	if m.Tests.Repo != "" {
		if ix, err = contextindex.BuildRevisionContext(ctx, tr.root, trev); err != nil {
			return nil, err
		}
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
	effects *effectReader
}

func (b *builder) unknown(u Unknown) { b.out.Unknowns = append(b.out.Unknowns, u) }

// testUnknown reports an unknown about a test file, naming the root the tests were read from.
func (b *builder) testUnknown(u Unknown) {
	u.Repo = b.m.Tests.Repo
	b.unknown(u)
}

func (b *builder) routers(r repo, ix *contextindex.Index) error {
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
	consts, err := newConstTable(ix, b.m.DIConstants)
	if err != nil {
		return err
	}
	consts.closureLimit = b.m.DIClosureLimit
	raws := []rawState{}
	spans := make([][2]int, len(ordered)) // each router's states in raws, for mounts (AMAP-V0-192)
	remainingRouters := maxRouterFiles - len(ordered)
	for i, e := range ordered {
		var states []rawState
		var unknowns []Unknown
		switch b.m.Routers[i].Dialect {
		case DialectRails:
			states, unknowns, err = readRailsRouter(r, b.rev, e, data[e.oid], &remainingRouters)
		case DialectReact:
			states, unknowns, err = readReactRouter(r, b.rev, e, data[e.oid], &remainingRouters)
		default:
			states, unknowns = parseRouter(e, data[e.oid], consts.forRouter(e, data[e.oid]))
		}
		if err != nil {
			return err
		}
		if err := consts.err(); err != nil {
			return err
		}
		spans[i] = [2]int{len(raws), len(raws) + len(states)}
		raws = append(raws, states...)
		b.out.Unknowns = append(b.out.Unknowns, unknowns...)
		if len(raws) > maxStates {
			return bound(fmt.Sprintf("more than %d router states", maxStates))
		}
	}
	if err := b.routerCoverage(r); err != nil {
		return err
	}
	b.out.Unknowns = append(b.out.Unknowns, consts.diag...)
	if err := mountRouters(b.m.Routers, raws, spans); err != nil {
		return err
	}
	screens, unknowns := resolveScreens(b.m.App, raws)
	b.out.Screens = screens
	b.out.Unknowns = append(b.out.Unknowns, unknowns...)
	b.screens = map[string]*Screen{}
	for i := range b.out.Screens {
		b.screens[b.out.Screens[i].ID] = &b.out.Screens[i]
	}
	b.idx = newScreenIndex(b.m.HashPrefix, b.m.BasePath, screens)
	b.idx.rankReact(b.m.App, b.m.Routers, raws, spans)
	for _, k := range b.idx.ambiguousKeys() {
		b.unknown(Unknown{Kind: "template", Ref: k, Reason: "ambiguous-template"})
	}
	return nil
}

// role names the manifest directory a test file sits in; the deepest declared directory wins.
// In a directory declared as both specs and page objects, a file Playwright's default testMatch
// names is a spec and every other file a page object (AMAP-V0-780).
func (b *builder) role(p string) string {
	best, role, flat := "", roleSupport, false
	t := b.m.Tests
	for _, g := range []struct {
		dirs []string
		role string
	}{{t.Specs, roleSpec}, {t.PageObjects, rolePageObject}, {t.Workflows, roleWorkflow}, {t.Scenarios, roleScenario}} {
		for _, d := range g.dirs {
			switch {
			case under(p, d) && len(d) > len(best):
				best, role, flat = d, g.role, false
			case d == best && role == roleSpec && g.role == rolePageObject:
				flat = true
			}
		}
	}
	if flat && !playwrightTestName(path.Base(p)) {
		return rolePageObject
	}
	return role
}

// playwrightTestName reports whether a base name matches Playwright's default testMatch,
// `NAME.spec.EXT` or `NAME.test.EXT` with a JavaScript or TypeScript suffix.
func playwrightTestName(base string) bool {
	ext := path.Ext(base)
	if !webSuffix[strings.ToLower(ext)] {
		return false
	}
	stem := strings.TrimSuffix(base, ext)
	return strings.HasSuffix(stem, ".spec") || strings.HasSuffix(stem, ".test")
}

// bareConfigImport resolves a bare specifier through the tsconfig/jsconfig governing importer
// (`baseUrl`, `paths`, `extends`) and returns the tracked file it names, or "" when the specifier
// is relative, absolute or resolves to no single repository file. refused reports a bare
// specifier the resolver reads as unresolved (an unreadable config, an ambiguous pattern, no
// declared package) that is not a Node built-in: it stays unresolved even when some package.json
// declares its package, never an external certainty. A package reading keeps the map's own
// importStatus rule (AMAP-V0-195).
func bareConfigImport(resolver *contextindex.WebImportResolver, importer, specifier string) (target string, refused bool) {
	if strings.HasPrefix(specifier, ".") || strings.HasPrefix(specifier, "/") {
		return "", false
	}
	switch res := resolver.Resolve(importer, specifier); res.State {
	case contextindex.WebImportRepository:
		return res.Target, false
	case contextindex.WebImportUnresolved:
		return "", !strings.HasPrefix(specifier, "node:") && !nodeBuiltins[specifier]
	}
	return "", false
}

// testTarget is the test file an import resolves to as an edge of the test import graph: only a
// resolved Node import is one. A browser-context reference's Resolved names the module the page
// loads, never a test edge, even when it lies inside tests.root (AMAP-V0-781).
func (imp Import) testTarget() string {
	if imp.Status != importResolved {
		return ""
	}
	return imp.Resolved
}

// browserCalls are the member calls whose callback Playwright runs in the page (AMAP-V0-781).
var browserCalls = map[string]bool{"evaluate": true, "evaluateHandle": true, "addInitScript": true}

// testLoads is what the module loads of one test source read.
type testLoads struct {
	computed        []int            // Node-context loads whose argument is not one exact literal (AMAP-V0-195)
	browserComputed []int            // the same inside a browser-context callback (AMAP-V0-781)
	browserOnly     map[string]bool  // literal specifiers every occurrence of which is browser-context
	browserLines    map[string][]int // the lines each browser-only specifier is loaded on
}

// readLoads finds the `import(...)` and `require(...)` calls of a test source. A call lexically
// inside the first argument of a `.evaluate(`, `.evaluateHandle(` or `.addInitScript(` member call
// runs in the page, so it is a browser-context reference, not a load the index can resolve for the
// test file. A literal specifier is browser-only when no occurrence of it lies outside such a
// call, so a static or Node-context import of the same module keeps the AMAP-V0-005 rules.
func readLoads(text string) testLoads {
	toks, _ := lexJS(text)
	inBrowser := make([]bool, len(toks))
	for i := range toks {
		if toks[i].kind == tokIdent && browserCalls[toks[i].text] && property(toks, i) && next(toks, i+1, "(") {
			// Only the first argument, the callback or script the page runs, is browser
			// context; the later arguments are evaluated by Node before the call.
			for j, end := i+2, skipValue(toks, i+2); j < end && j < len(toks); j++ {
				inBrowser[j] = true
			}
		}
	}
	l := testLoads{computed: []int{}, browserComputed: []int{}, browserOnly: map[string]bool{}, browserLines: map[string][]int{}}
	node := map[string]bool{}
	for i := range toks {
		if literal(toks[i]) && !inBrowser[i] {
			node[toks[i].text] = true
		}
		if !(word(toks, i, "import") || word(toks, i, "require")) || property(toks, i) || !next(toks, i+1, "(") {
			continue
		}
		switch spec := moduleArg(toks, i+2); {
		case spec == "" && inBrowser[i]:
			l.browserComputed = append(l.browserComputed, toks[i].line)
		case spec == "":
			l.computed = append(l.computed, toks[i].line)
		case inBrowser[i]:
			l.browserLines[spec] = append(l.browserLines[spec], toks[i].line)
		}
	}
	for spec := range l.browserLines {
		if node[spec] {
			delete(l.browserLines, spec)
		} else {
			l.browserOnly[spec] = true
		}
	}
	return l
}

// browserTarget resolves a browser-context specifier: a root-relative one against tests.web_root,
// with its own suffix or the first web suffix or index file that names an indexed source
// (AMAP-V0-782). It returns the target or the browser-reference unknown reason.
func (b *builder) browserTarget(ix *contextindex.Index, specifier string) (target, reason string) {
	if !strings.HasPrefix(specifier, "/") || strings.HasPrefix(specifier, "//") {
		return "", "browser-specifier-unread"
	}
	if b.m.Tests.WebRoot == "" {
		return "", "browser-root-undeclared"
	}
	base := path.Join(b.m.Tests.WebRoot, strings.TrimPrefix(specifier, "/"))
	if !under(base, b.m.Tests.WebRoot) {
		return "", "browser-target-missing"
	}
	candidates := []string{base}
	for _, ext := range []string{".ts", ".tsx", ".js", ".jsx", ".mjs"} {
		candidates = append(candidates, base+ext)
	}
	for _, ext := range []string{".ts", ".tsx", ".js", ".jsx", ".mjs"} {
		candidates = append(candidates, base+"/index"+ext)
	}
	for _, c := range candidates {
		if _, ok := ix.Sources[c]; ok {
			return c, ""
		}
	}
	return "", "browser-target-missing"
}

func (b *builder) tests(ix *contextindex.Index) error {
	rootDir := b.m.Tests.Root
	for _, ex := range ix.Exclusions {
		if under(ex.Path, rootDir) && webSuffix[strings.ToLower(path.Ext(ex.Path))] {
			b.testUnknown(Unknown{Kind: "file", Ref: fileID(ex.Path), Reason: "excluded-by-index", Path: ex.Path})
		}
	}
	unparsed := map[string]bool{}
	for _, u := range ix.Unparsed {
		if under(u.Path, rootDir) {
			unparsed[u.Path] = true
		}
	}
	packages := declaredPackages(ix)
	// One resolver per build applies the tests root's tsconfig/jsconfig `baseUrl`, `paths` and
	// `extends` to bare specifiers, the index's own resolver rather than a second one (AMAP-V0-195).
	resolver := contextindex.NewWebImportResolver(ix)
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
			b.testUnknown(Unknown{Kind: "file", Ref: fileID(p), Reason: "unparsed-imports", Path: p})
			continue
		}
		if len(text) > maxTestFileBytes {
			return bound(fmt.Sprintf("%s exceeds %d bytes", p, maxTestFileBytes))
		}
		if total += len(text); total > maxTestBytes {
			return bound(fmt.Sprintf("test sources exceed %d bytes", maxTestBytes))
		}
		e := blobEntry{repo: b.m.Tests.Repo, path: p, oid: src.BlobHash, size: len(text)}
		data := []byte(text)
		facts := readFacts(text)
		binds := contextindex.WebImportBindings(text)
		tf := &TestFile{ID: fileID(p), Path: p, Role: b.role(p), Class: facts.class, Join: StatusResolved, Anchor: wholeFile(e, data),
			Imports: []Import{}, Methods: []Method{}, Gotos: []Goto{}, Tests: append([]string{}, facts.tests...),
			Assertions: facts.asserts, Selectors: append([]Selector{}, facts.selectors...)}
		if facts.classDefault {
			tf.ClassExport = ClassExportDefault
		}
		if unparsed[p] {
			tf.Join = StatusUnknown
			b.testUnknown(Unknown{Kind: "file", Ref: tf.ID, Reason: "unparsed-imports", Path: p})
		}
		specifiers := []string{}
		for s := range ix.Imports[p] {
			specifiers = append(specifiers, s)
		}
		sort.Strings(specifiers)
		imps := []Import{}
		for _, s := range specifiers {
			// One Import per statement: separate statements from one module bind different names,
			// and each borrowed statement must carry exactly the names it binds.
			byStmt := map[string]int{}
			for _, bd := range binds {
				if bd.Module != s {
					continue
				}
				stmt := statementAt(text, bd.Line, s)
				k, seen := byStmt[stmt]
				if !seen {
					k = len(imps)
					byStmt[stmt] = k
					imps = append(imps, Import{Specifier: s, Statement: stmt, Line: bd.Line, Names: []string{}})
				}
				imps[k].Names = append(imps[k].Names, bd.Local)
			}
			if len(byStmt) == 0 {
				line := importLine(text, s)
				imps = append(imps, Import{Specifier: s, Statement: statementAt(text, line, s), Line: line, Names: []string{}})
			}
		}
		sort.SliceStable(imps, func(i, j int) bool { return imps[i].Line < imps[j].Line })
		loads := readLoads(text)
		for _, imp := range imps {
			s := imp.Specifier
			detail := "specifier " + s
			if loads.browserOnly[s] {
				// The page loads it through the dev server; it is no edge of the test import graph.
				imp.Status = importBrowser
				if lines := loads.browserLines[s]; len(lines) > 0 {
					imp.Line = lines[0]
				}
				target, reason := b.browserTarget(ix, s)
				if target != "" {
					imp.Resolved = target
				} else {
					b.testUnknown(Unknown{Kind: "browser-reference", Ref: tf.ID, Reason: reason, Path: p, Line: imp.Line, Detail: detail})
				}
				tf.Imports = append(tf.Imports, imp)
				continue
			}
			if target := contextindex.ResolveWebImport(ix, p, s); target != "" {
				imp.Status, imp.Resolved = importResolved, target
			} else if res, refused := bareConfigImport(resolver, p, s); res != "" {
				imp.Status, imp.Resolved = importResolved, res
			} else if refused {
				imp.Status = importUnresolved
				if importStatus(s, packages) == importExternal {
					// Some package.json declares the package, yet the governing config (an
					// unindexed `extends`, an ambiguous pattern) or the importer's manifest chain
					// leaves its resolution unknown, so it is not read as external.
					detail += " (config resolution unknown)"
				}
			} else {
				imp.Status = importStatus(s, packages)
			}
			if imp.Status == importResolved && !under(imp.Resolved, rootDir) && webSuffix[strings.ToLower(path.Ext(imp.Resolved))] {
				// A first-party module outside tests.root is not read, yet it may import a page
				// object, so the chain through it is not known to be complete.
				tf.Join = StatusUnknown
				b.testUnknown(Unknown{Kind: "import", Ref: tf.ID, Reason: "import-outside-tests", Path: p, Line: imp.Line})
			}
			if imp.Status == importUnresolved {
				tf.Join = StatusUnknown
				b.testUnknown(Unknown{Kind: "import", Ref: tf.ID, Reason: "unresolved-import", Path: p, Line: imp.Line, Detail: detail})
			}
			tf.Imports = append(tf.Imports, imp)
		}
		for _, line := range loads.computed {
			tf.Join = StatusUnknown
			b.testUnknown(Unknown{Kind: "import", Ref: tf.ID, Reason: "unresolved-import", Path: p, Line: line, Detail: "computed specifier"})
		}
		for _, line := range loads.browserComputed {
			b.testUnknown(Unknown{Kind: "browser-reference", Ref: tf.ID, Reason: "browser-computed-specifier", Path: p, Line: line, Detail: "computed specifier"})
		}
		for _, rm := range facts.methods {
			meth := Method{ID: methodID(p, rm.name), Name: rm.name, Anchor: spanOf(e, data, rm.start, rm.end), Callable: rm.callable, Selectors: []Selector{}}
			for _, s := range facts.selectors {
				if s.Line >= rm.start && s.Line <= rm.end {
					meth.Selectors = append(meth.Selectors, s)
				}
			}
			tf.Methods = append(tf.Methods, meth)
		}
		tf.Gotos = b.gotos(facts)
		for _, line := range facts.secrets {
			b.testUnknown(Unknown{Kind: "selector", Ref: refAt(p, line), Reason: "secret-shaped", Path: p, Line: line})
		}
		b.files[p], b.facts[p], b.binds[p], b.sources[p] = tf, facts, binds, data
		b.order = append(b.order, p)
	}
	return nil
}

// gotos places a test file's navigations on the screens the routers derived.
func (b *builder) gotos(facts fileFacts) []Goto {
	out := []Goto{}
	for _, g := range facts.gotos {
		gt := Goto{Line: g.line, URL: strings.ReplaceAll(g.url, substMark, "${}"), Status: StatusUnknown, Reason: g.reason}
		if g.reason == "" {
			if id, reason := b.idx.byURL(g.url); id != "" {
				gt.Screen, gt.Status = id, StatusResolved
			} else {
				gt.Reason = reason
			}
		}
		out = append(out, gt)
	}
	return out
}

// testStageUnknown reports whether the tests stage, rather than the routers or the join, emits u:
// the file, import and selector unknowns tests() reports. The join reports import-depth-exceeded.
func testStageUnknown(u Unknown) bool {
	switch u.Kind {
	case "file", "selector", "browser-reference":
		return true
	case "import":
		return u.Reason != "import-depth-exceeded"
	}
	return false
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

// unresolvedScreenDetail names a declared screen's own blocking unknown: its id, status and reason.
func unresolvedScreenDetail(s *Screen) string {
	d := s.ID + " " + s.Status
	if s.Reason != "" {
		d += ": " + s.Reason
	}
	return d
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
			switch s := b.screens[id]; {
			case s != nil && s.Status == StatusResolved:
				tf.Screen, tf.ScreenBasis = id, "declared"
			case s != nil:
				// The state exists but its screen is itself unresolved: name that blocking unknown, so
				// the remedy does not ask to respell a correct state (AMAP-V0-359).
				b.testUnknown(Unknown{Kind: "page-object", Ref: tf.ID, Reason: "unresolved-screen", Path: p,
					Detail: unresolvedScreenDetail(s)})
			default:
				b.testUnknown(Unknown{Kind: "page-object", Ref: tf.ID, Reason: "unknown-state", Path: p})
			}
			continue
		}
		targets := map[string]bool{}
		var unresolved []string
		for _, g := range tf.Gotos {
			if g.Screen != "" {
				targets[g.Screen] = true
			} else {
				unresolved = append(unresolved, p+":"+strconv.Itoa(g.Line)+":"+g.Reason)
			}
		}
		switch {
		case len(unresolved) > 0:
			// a target the map cannot place may be a second screen, so no single binding is known;
			// retain helper candidates and name every direct blocker and helper gap (AMAP-V0-450).
			var gaps []string
			tf.HelperGotos, gaps = b.helperGotos(tf)
			b.testUnknown(Unknown{Kind: "page-object", Ref: tf.ID, Reason: "page-object-unresolved-target", Path: p, Detail: strings.Join(append(unresolved, gaps...), ",")})
		case len(targets) == 1:
			for id := range targets {
				tf.Screen, tf.ScreenBasis = id, "page-object-url"
			}
		case len(targets) == 0:
			var gaps []string
			tf.HelperGotos, gaps = b.helperGotos(tf)
			detail := strings.Join(gaps, ",")
			switch { // the unbound detail is never empty (AMAP-V0-370)
			case detail != "":
			case len(tf.HelperGotos) > 0:
				detail = "helper-candidates:" + strconv.Itoa(len(tf.HelperGotos))
			default:
				detail = "no-navigation"
			}
			b.testUnknown(Unknown{Kind: "page-object", Ref: tf.ID, Reason: "page-object-unbound", Path: p, Detail: detail})
		default:
			b.testUnknown(Unknown{Kind: "page-object", Ref: tf.ID, Reason: "page-object-ambiguous", Path: p})
		}
	}
	for p := range declared {
		if b.files[p] == nil {
			b.testUnknown(Unknown{Kind: "page-object", Ref: fileID(p), Reason: "page-object-unbound", Path: p})
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
			b.testUnknown(Unknown{Kind: "import", Ref: tf.ID, Reason: "import-depth-exceeded", Path: p})
		}
		if !complete && tf.Join == StatusResolved {
			tf.Join = StatusUnknown
			b.testUnknown(Unknown{Kind: "test-join", Ref: tf.ID, Reason: "unresolved-import", Path: p})
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
				t := imp.testTarget()
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
			if t := imp.testTarget(); imp.Specifier == bd.Module && t != "" {
				classes[bd.Local] = t
			}
		}
	}
	type use struct {
		screen   string
		line, at int
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
		uses = append(uses, use{b.files[target].Screen, n.line, n.at})
	}
	if len(uses) < 2 {
		return
	}
	e := blobEntry{repo: tf.Anchor.Repo, path: tf.Path, oid: tf.Anchor.Blob}
	if b.effects == nil {
		b.effects = newEffectReader(b)
	}
	toks := b.effects.toks(tf.Path)
	inst := instances(toks, classes)
	for i := 1; i < len(uses); i++ {
		anchor := spanOf(e, b.sources[tf.Path], uses[i-1].line, uses[i].line)
		// The whole spec is cited: its imports and instance bindings decide which methods are read.
		eff := stepEffect{kind: EffectNavigation, from: []Anchor{spanOf(e, b.sources[tf.Path], 1, strings.Count(string(b.sources[tf.Path]), "\n")+1)}}
		eff.join(b.effects.span(tf.Path, toks, uses[i-1].at, uses[i].at, inst, classes, "", 0))
		// Both constructions that bound the step run their page objects' constructors; the closing
		// construction's arguments run in this step too (the opening one's lie inside the span).
		for _, u := range []use{uses[i-1], uses[i]} {
			eff.join(b.effects.construct(classes[toks[u.at].text], 1, tf.Path, u.line))
		}
		if end := closeParen(toks, uses[i].at+1); !next(toks, uses[i].at+1, "(") || !isPunct(toks[end], ")") {
			eff.join(effectAt(EffectUnknown, tf.Path, uses[i].line, "new"))
		} else {
			eff.join(b.effects.span(tf.Path, toks, uses[i].at+1, end, inst, classes, "", 0))
		}
		edge := Edge{From: uses[i-1].screen, To: uses[i].screen, Basis: "test-sequence", Source: tf.ID, Anchor: anchor,
			Effect: eff.kind, EffectFrom: eff.from}
		if eff.kind != EffectNavigation {
			edge.EffectRef, edge.EffectCall = eff.ref, eff.call
		}
		b.out.Edges = append(b.out.Edges, edge)
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
		tests := map[string][]string{}
		for _, l := range intent.Links {
			if l.Basis == "declared" && l.Target.Type == "test" {
				tests[l.From] = append(tests[l.From], l.Target.TestKey)
			}
		}
		prev := ""
		for _, fs := range intent.Steps {
			st := Step{ID: "step:" + intent.FlowID + "/" + fs.StepID, Action: fs.Action, Status: StatusUnknown, Reuse: []string{}}
			if keys := tests[fs.StepID]; len(keys) > 0 {
				st.Tests = uniqueSorted(keys)
			}
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
	c.Digest, c.Builder = "", ""
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
