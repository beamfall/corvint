package affected

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"fmt"
	"io/fs"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Graph is the composed dependency graph over every participating language.
//
// A graph is immutable once built. Its digest covers every unit, every edge,
// and every frontier reason, so two graphs with the same digest select
// identically and a plan can name the exact universe it inspected.
type Graph struct {
	units map[string]Unit
	order []string
	owner map[string]string
	// embedded maps each file a unit's EmbedFiles lists to those units, in
	// unit order (AFP-V0-600).
	embedded   map[string][]string
	dependents map[string][]string
	advisory   map[string][]string
	routed     map[string][]string
	testUsers  map[string][]string
	execUsers  map[string][]string
	testReach  map[string]bool
	unbounded  []string
	// causes names, for each unbounded reader, the unit whose own reason
	// makes it one (AFP-V0-192).
	causes map[string]string
	// chains holds, for each unbounded reader, the unit ids from its cause to
	// itself along the import edges that make it one (AFP-V0-500).
	chains map[string][]string
	// locatorReach holds the root locators and the units their dependency
	// edges reach: on any dirty path the plan follows these units'
	// dependents (AFP-V0-581).
	locatorReach map[string]bool
	claimants    map[string]Language
	languages    []string
	frontier     []string
	frontierBy   map[string][]string
	// sourceBound and fixtureClosed hold each plugin's FrontierReach
	// declaration by reason (AFP-V0-511); a plugin absent from both is
	// path-open throughout. They classify plugin code, not the observed
	// repository, so they are no part of the digest.
	sourceBound   map[string]map[string]bool
	fixtureClosed map[string]map[string]bool
	// sites counts the repository's frontier sites by reason (AFP-V0-381).
	sites map[string]int
	// promoted marks each fixture unit whose own frontier no Go unit hosts, so
	// it was raised to its plugin's frontier again (AFP-V0-446).
	promoted map[string]bool
	digest   string
	// documentation is the repository's declared documentation entries
	// (AFP-V0-160), sorted; it is part of the digest when present.
	documentation []string
	// runtimeEdges is the repository's declared runtime edges (AFP-V0-819).
	runtimeEdges []RuntimeEdge
	// runtimeReaders is the repository's declared runtime-readers entries
	// (AFP-V0-602), sorted by path; it is part of the digest when present.
	runtimeReaders []RuntimeReader
	// unobserved maps each plugin to the directories it owns but builds no
	// unit for (AFP-V0-191), each list sorted.
	unobserved map[string][]string
	// runnable maps each plugin to the subset of its unobserved directories
	// its own runner can test as a root (AFP-V0-650), each list sorted.
	runnable map[string][]string
	// priorReadScopes is the read-scope declaration a change is measured
	// from, by package directory; nil means unknown (AFP-V0-050). It is a
	// selection input, not part of the observed graph or its digest.
	priorReadScopes map[string][]string
	// goAdditive holds the golang plugin's additive-change records, sorted
	// by unit (ADN-V0-002). Like priorReadScopes it is a selection input,
	// not part of the observed graph or its digest.
	goAdditive []GoAdditive
	// root is the disk checkout the graph was observed from, empty for an
	// fs.FS graph. Only AFP-V0-494's symlink hint reads it.
	root string
}

// WithReadScopeBase returns a copy of graph whose selection compares an edit
// to the read-scope declaration against prior, the declaration at the commit
// the change is measured from; an absent declaration is an empty map
// (AFP-V0-050). A nil prior is unknown and keeps AFP-V0-023's rule that the
// edit bears on every declared unit.
func (graph *Graph) WithReadScopeBase(prior map[string][]string) *Graph {
	copied := *graph
	copied.priorReadScopes = prior
	return &copied
}

// graphDigestDomain is hashed ahead of the digest body, so a graph digest
// cannot collide with a digest of the same bytes in another domain (AFP-V0-005).
const graphDigestDomain = "corvint-affected-graph/1\n"

// digestBody is the documented projection the graph digest covers
// (AFP-V0-005). Its member names are fixed here, not by Unit's struct tags, so
// a new internal field changes the digest only once it is added here.
type digestBody struct {
	Languages []string     `json:"languages"`
	Frontier  []string     `json:"frontier"`
	Units     []digestUnit `json:"units"`
	// Documentation appears only when the repository declares documentation
	// paths (AFP-V0-160), so graphs without the declaration keep their digest.
	Documentation []string `json:"documentation,omitempty"`
	// RuntimeReaders appears only when the repository declares runtime-readers
	// paths (AFP-V0-602), so graphs without the declaration keep their digest.
	RuntimeReaders []RuntimeReader `json:"runtimeReaders,omitempty"`
	// RuntimeEdges appears only when the repository declares runtime edges
	// (AFP-V0-819).
	RuntimeEdges []RuntimeEdge `json:"runtimeEdges,omitempty"`
	// Unobserved appears only when a plugin reports a directory it owns but
	// does not observe (AFP-V0-191), as "namespace:directory", sorted.
	Unobserved []string `json:"unobserved,omitempty"`
	// Runnable appears only when a plugin reports an unobserved directory
	// its own runner can test as a root (AFP-V0-650), in the same form.
	Runnable []string `json:"runnable,omitempty"`
}

// digestUnit projects one Unit; every member is always present except the
// declared read scope, which appears only for a declared unit, and the
// command edges, which appear only for a unit that runs a command, and the
// advisory and route-dispatch edges, which appear only for a unit that has
// one (RUBY-AFFECTED-V0-009, RUBY-AFFECTED-V0-049), the literal globs
// (AFP-V0-401), the exact embed targets (AFP-V0-600), the file-name
// placement certificate (AFP-V0-400), the declared binary consumers
// (AFP-V0-762) and the
// frontier's bearing condition (AFP-V0-581), so that graphs
// without them keep their digest.
type digestUnit struct {
	ID                   string   `json:"id"`
	Sources              []string `json:"sources"`
	Tests                []string `json:"tests"`
	Imports              []string `json:"imports"`
	TestImports          []string `json:"testImports"`
	Execs                []string `json:"execs,omitempty"`
	AdvisoryImports      []string `json:"advisoryImports,omitempty"`
	RouteDispatchImports []string `json:"routeDispatchImports,omitempty"`
	PathTokens           []string `json:"pathTokens"`
	PathGlobs            []string `json:"pathGlobs,omitempty"`
	PathTokensBounded    bool     `json:"pathTokensBounded"`
	PlacesFileNames      bool     `json:"placesFileNames,omitzero"`
	Embeds               bool     `json:"embeds"`
	EmbedFiles           []string `json:"embedFiles,omitempty"`
	UnboundedReads       string   `json:"unboundedReads"`
	LocatesRoot          bool     `json:"locatesRoot"`
	ReadScoped           bool     `json:"readScoped,omitzero"`
	ReadScope            []string `json:"readScope,omitempty"`
	Frontier             []string `json:"frontier"`
	FrontierBears        string   `json:"frontierBears,omitempty"`
	Callers              []string `json:"callers,omitempty"`
	ExecConsumers        []string `json:"execConsumers,omitempty"`
}

// Build composes one graph from every supplied language plugin.
//
// Plugins are observed in the order given but the resulting graph is
// order-independent: identities, paths, and edges are all canonically sorted
// before the digest is taken.
// SourceLanguage observes the same unit contract through an explicit reader.
type SourceLanguage interface {
	Language
	UnitsSource(*Source) (Result, error)
}

// BuildFS composes a graph exclusively from the supplied source filesystem.
// A provider without an immutable reader refuses rather than reading ambient files.
func BuildFS(source fs.FS, languages ...Language) (*Graph, error) {
	if source == nil {
		return nil, ErrInvalidLanguage
	}
	reader := FSSource(source)
	return build(context.Background(), languages, declarationsOf(reader), func(language Language) (Result, error) {
		explicit, ok := language.(SourceLanguage)
		if !ok {
			return Result{}, fmt.Errorf("%w: no source reader for %s", ErrInvalidLanguage, language.Name())
		}
		result, err := explicit.UnitsSource(reader)
		if fatal := reader.Err(); fatal != nil {
			return Result{}, fatal
		}
		return result, err
	})
}

func Build(ctx context.Context, root string, languages ...Language) (*Graph, error) {
	graph, _, err := buildDisk(ctx, DiskSourceContext(ctx, root), languages)
	return graph, err
}

// buildDisk is Build over an explicit disk reader, which may record what the
// observation read (AFP-V0-391). It also returns each plugin's result in
// language order, the input a cached graph is restored from.
func buildDisk(ctx context.Context, reader *Source, languages []Language) (*Graph, []Result, error) {
	root := reader.root
	results := make([]Result, 0, len(languages))
	graph, err := build(ctx, languages, declarationsOf(reader), func(language Language) (Result, error) {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		var result Result
		var err error
		if explicit, ok := language.(SourceLanguage); ok {
			result, err = explicit.UnitsSource(reader)
		} else {
			// Its reads bypass reader, so no stamp could pin them.
			reader.record.refuse("language " + language.Name() + " reads outside the recorded source")
			result, err = language.Units(ctx, root)
		}
		if cancelled := ctx.Err(); cancelled != nil {
			return Result{}, cancelled
		}
		if err == nil {
			results = append(results, result)
		}
		return result, err
	})
	if graph != nil {
		graph.root = root
	}
	return graph, results, err
}

// declarations is what the repository's own Corvint declarations add to the
// observed graph: the documentation entries (AFP-V0-160) and the
// runtime-readers entries (AFP-V0-602).
type declarations struct {
	documentation  []string
	runtimeReaders []RuntimeReader
	runtimeEdges   []RuntimeEdge
}

// declarationsOf reads both declarations through reader.
func declarationsOf(reader *Source) func() (declarations, error) {
	return func() (declarations, error) {
		documentation, err := readDocumentationPaths(reader)
		if err != nil {
			return declarations{}, err
		}
		runtimeReaders, err := readRuntimeReaders(reader)
		if err != nil {
			return declarations{}, err
		}
		runtimeEdges, err := readRuntimeEdges(reader)
		if err != nil {
			return declarations{}, err
		}
		return declarations{documentation: documentation, runtimeReaders: runtimeReaders, runtimeEdges: runtimeEdges}, nil
	}
}

func build(ctx context.Context, languages []Language, declared func() (declarations, error), observe func(Language) (Result, error)) (*Graph, error) {
	if len(languages) == 0 {
		return nil, fmt.Errorf("%w: no language plugins", ErrInvalidLanguage)
	}
	graph := &Graph{
		units:      make(map[string]Unit),
		owner:      make(map[string]string),
		embedded:   make(map[string][]string),
		dependents: make(map[string][]string),
		testUsers:  make(map[string][]string),
		execUsers:  make(map[string][]string),
		claimants:  make(map[string]Language),
		frontierBy: make(map[string][]string),
	}
	seenLanguage := make(map[string]bool, len(languages))
	frontier := make(map[string]bool)
	for _, language := range languages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := language.Name()
		if name == "" || seenLanguage[name] {
			return nil, fmt.Errorf("%w: language name %q", ErrInvalidLanguage, name)
		}
		seenLanguage[name] = true
		graph.claimants[name] = language
		graph.languages = append(graph.languages, name)
		if err := graph.declareFrontierReach(name, language); err != nil {
			return nil, err
		}
		result, err := observe(language)
		if err != nil {
			return nil, fmt.Errorf("language %s: %w", name, err)
		}
		own := make(map[string]bool)
		if err := graph.admit(ctx, name, result, own); err != nil {
			return nil, err
		}
		graph.frontierBy[name] = sortedKeys(own)
		for reason := range own {
			frontier[reason] = true
		}
	}
	sort.Strings(graph.languages)
	graph.promoteUnhostedFixtureFrontier(frontier)
	graph.frontier = sortedKeys(frontier)
	graph.sites = graph.frontierSites()
	repository, err := declared()
	if err != nil {
		return nil, err
	}
	graph.documentation = repository.documentation
	graph.runtimeReaders = repository.runtimeReaders
	graph.runtimeEdges = repository.runtimeEdges
	if err := graph.applyRuntimeEdges(); err != nil {
		return nil, err
	}
	graph.finish()
	digest, err := graph.computeDigest()
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	graph.digest = digest
	return graph, nil
}

// declareFrontierReach records a plugin's FrontierReach classification
// (AFP-V0-511). Every declared reason must be `<name>:<category>` with a
// non-empty category and no further field; anything else fails the build, so
// a malformed declaration can never narrow a plan. A plugin without the
// interface declares nothing and stays path-open.
func (graph *Graph) declareFrontierReach(name string, language Language) error {
	reach, declares := language.(FrontierReach)
	if !declares {
		return nil
	}
	record := func(kind string, reasons []string) (map[string]bool, error) {
		set := make(map[string]bool, len(reasons))
		for _, reason := range reasons {
			namespace, category, found := strings.Cut(reason, ":")
			if !found || namespace != name || category == "" || strings.Contains(category, ":") {
				return nil, fmt.Errorf("%w: language %s declares %s frontier reason %q", ErrInvalidLanguage, name, kind, reason)
			}
			set[reason] = true
		}
		return set, nil
	}
	sourceBound, err := record("source-bound", reach.SourceBoundFrontiers())
	if err != nil {
		return err
	}
	fixtureClosed, err := record("fixture-closed", reach.FixtureClosedFrontiers())
	if err != nil {
		return err
	}
	for reason := range fixtureClosed {
		if sourceBound[reason] {
			return fmt.Errorf("%w: language %s declares %q both source-bound and fixture-closed", ErrInvalidLanguage, name, reason)
		}
	}
	if graph.sourceBound == nil {
		graph.sourceBound = make(map[string]map[string]bool)
		graph.fixtureClosed = make(map[string]map[string]bool)
	}
	graph.sourceBound[name] = sourceBound
	graph.fixtureClosed[name] = fixtureClosed
	return nil
}

func (graph *Graph) admit(ctx context.Context, namespace string, result Result, frontier map[string]bool) error {
	previous := ""
	for _, directory := range result.Unobserved {
		if !ValidRelativePath(directory) || directory <= previous {
			return fmt.Errorf("%w: unobserved directory %q from %s", ErrInvalidLanguage, directory, namespace)
		}
		previous = directory
	}
	if len(result.Unobserved) != 0 {
		if graph.unobserved == nil {
			graph.unobserved = make(map[string][]string)
		}
		graph.unobserved[namespace] = append([]string(nil), result.Unobserved...)
	}
	previous = ""
	for _, directory := range result.Runnable {
		if directory <= previous || !slices.Contains(result.Unobserved, directory) {
			return fmt.Errorf("%w: runnable directory %q from %s", ErrInvalidLanguage, directory, namespace)
		}
		previous = directory
	}
	if len(result.Runnable) != 0 {
		if graph.runnable == nil {
			graph.runnable = make(map[string][]string)
		}
		graph.runnable[namespace] = append([]string(nil), result.Runnable...)
	}
	for _, reason := range result.Frontier {
		if reason == "" {
			return fmt.Errorf("%w: empty frontier reason from %s", ErrInvalidLanguage, namespace)
		}
		frontier[reason] = true
	}
	for _, unit := range result.Units {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := validUnit(unit, namespace); err != nil {
			return fmt.Errorf("language %s unit %q: %w", namespace, unit.ID, err)
		}
		if _, exists := graph.units[unit.ID]; exists {
			return fmt.Errorf("%w: %s", ErrDuplicateUnit, unit.ID)
		}
		if len(graph.units) >= MaxUnits {
			return fmt.Errorf("%w: more than %d units", ErrInvalidLanguage, MaxUnits)
		}
		for _, path := range append(append([]string(nil), unit.Sources...), unit.Tests...) {
			if previous, exists := graph.owner[path]; exists {
				return fmt.Errorf("%w: %s claimed by %s and %s", ErrDuplicateOwner, path, previous, unit.ID)
			}
			graph.owner[path] = unit.ID
		}
		for _, path := range unit.EmbedFiles {
			graph.embedded[path] = append(graph.embedded[path], unit.ID)
		}
		graph.units[unit.ID] = unit
		graph.order = append(graph.order, unit.ID)
	}
	return nil
}

// finish materializes the reverse edges, with test-only imports kept apart. An
// import naming a unit outside the graph is dropped: it is an external
// dependency, and an external dependency cannot be a dirty repository path.
func (graph *Graph) finish() {
	sort.Strings(graph.order)
	graph.dependents = graph.reverse(func(unit Unit) []string { return unit.Imports })
	graph.advisory = graph.reverse(func(unit Unit) []string { return unit.AdvisoryImports })
	graph.routed = graph.reverse(func(unit Unit) []string { return unit.RouteDispatchImports })
	graph.testUsers = graph.reverse(func(unit Unit) []string { return unit.TestImports })
	graph.execUsers = graph.reverse(func(unit Unit) []string { return unit.Execs })
	called := graph.callerEdges()
	graph.testReach = graph.unitsReachingTests(called)
	graph.unbounded, graph.causes, graph.chains, graph.locatorReach = graph.unboundedReaders()
}

// callerEdges adds the TSP-V0-005 edges of every unit's declared Callers to
// the reverse maps: the owner of a caller file that is one of its tests
// becomes a test user of the unit, any other owner a dependent. It returns
// the forward form of the importer edges, owner to called units, for the
// walk that finds the units reaching a test.
func (graph *Graph) callerEdges() map[string][]string {
	called := map[string][]string{}
	for _, id := range graph.order {
		for _, path := range graph.units[id].Callers {
			owner, owned := graph.owner[path]
			if !owned || owner == id {
				continue
			}
			edges := graph.dependents
			if slices.Contains(graph.units[owner].Tests, path) {
				edges = graph.testUsers
			} else if !slices.Contains(called[owner], id) {
				called[owner] = append(called[owner], id)
			}
			if !slices.Contains(edges[id], owner) {
				edges[id] = append(edges[id], owner)
				sort.Strings(edges[id])
			}
		}
	}
	return called
}

// unboundedReaders lists, in id order, every unit whose reads no literal
// bounds (AFP-V0-012 rule (d)): its own UnboundedReads, or a dependency whose
// non-test code locates the root and so reads from it when this unit's code
// or tests call it. A unit whose reads are declared is left out, whatever the
// cause (AFP-V0-023): its declared scope bounds the whole test process. It
// also maps each listed unit to the unit whose own reason makes it one: the
// unit itself when it has one, else the root locator its chain starts at, and
// to that chain: the cause first and the unit last, each unit importing (or
// its tests importing) the one before it. It also returns the units the
// locators' dependency traversal walks (AFP-V0-581).
func (graph *Graph) unboundedReaders() ([]string, map[string]string, map[string][]string, map[string]bool) {
	locators := make(map[string]Witness)
	own := make([]string, 0)
	for _, id := range graph.order {
		unit := graph.units[id]
		if unit.UnboundedReads != "" {
			own = append(own, id)
		}
		if unit.LocatesRoot {
			locators[id] = Witness{Via: []string{id}}
		}
	}
	reached := graph.traverse(locators, nil)
	walked := make(map[string]bool, len(reached))
	for id := range reached {
		walked[id] = true
	}
	graph.testUsersOf(reached, nil)
	for _, id := range own {
		reached[id] = Witness{}
	}
	ids := make([]string, 0, len(reached))
	causes := make(map[string]string, len(reached))
	chains := make(map[string][]string, len(reached))
	for id, witness := range reached {
		if graph.units[id].ReadScoped {
			continue
		}
		ids = append(ids, id)
		causes[id] = id
		chains[id] = []string{id}
		if graph.units[id].UnboundedReads == "" && witness.ViaLength() != 0 {
			causes[id] = witness.viaHead()
			chains[id] = witness.Chain()
		}
	}
	sort.Strings(ids)
	return ids, causes, chains, walked
}

// reverse maps every unit to the sorted units whose edges name it.
func (graph *Graph) reverse(edges func(Unit) []string) map[string][]string {
	reversed := make(map[string][]string)
	for _, id := range graph.order {
		for _, target := range edges(graph.units[id]) {
			if _, known := graph.units[target]; !known {
				continue
			}
			reversed[target] = append(reversed[target], id)
		}
	}
	return reversed
}

// unitsReachingTests names every unit that reaches, itself included, a unit
// declaring a test along reverse dependency edges: one walk over forward
// imports from every unit that declares a test.
func (graph *Graph) unitsReachingTests(called map[string][]string) map[string]bool {
	reach := make(map[string]bool)
	frontier := make([]string, 0, len(graph.order))
	for _, id := range graph.order {
		if len(graph.units[id].Tests) != 0 {
			reach[id] = true
			frontier = append(frontier, id)
		}
	}
	for len(frontier) != 0 {
		id := frontier[len(frontier)-1]
		frontier = frontier[:len(frontier)-1]
		unit := graph.units[id]
		for _, target := range slices.Concat(unit.Imports, unit.AdvisoryImports, unit.RouteDispatchImports, called[id]) {
			if _, known := graph.units[target]; !known || reach[target] {
				continue
			}
			reach[target] = true
			frontier = append(frontier, target)
		}
	}
	return reach
}

func (graph *Graph) computeDigest() (string, error) {
	body, err := graph.Canonical()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(append([]byte(graphDigestDomain), body...))
	return "affected-graph:sha256:" + hex.EncodeToString(sum[:]), nil
}

// Digest is the canonical identity of this graph.
func (graph *Graph) Digest() string { return graph.digest }

// Languages names the participating plugins, sorted.
func (graph *Graph) Languages() []string { return append([]string(nil), graph.languages...) }

// Frontier names everything no plugin could resolve exactly, sorted.
func (graph *Graph) Frontier() []string { return append([]string(nil), graph.frontier...) }

// UnitIDs lists every unit identity, sorted.
func (graph *Graph) UnitIDs() []string { return append([]string(nil), graph.order...) }

// UnboundedReaders lists, sorted, every unit whose reads no literal or declared
// scope bounds: the units rule (d) selects on any dirty path (AFP-V0-012,
// AFP-V0-023).
func (graph *Graph) UnboundedReaders() []string { return append([]string(nil), graph.unbounded...) }

// UnboundedChain returns why rule (d) selects an unbounded reader
// (AFP-V0-500): the unit ids from the unit whose own unbounded read is the
// cause to the reader itself, each importing (or its tests importing) the one
// before it, and the cause's own reason. A reader that is its own cause has a
// one-unit chain; a unit that is not an unbounded reader has none.
func (graph *Graph) UnboundedChain(id string) (chain []string, reads string) {
	chain, ok := graph.chains[id]
	if !ok {
		return nil, ""
	}
	return append([]string(nil), chain...), graph.units[chain[0]].UnboundedReads
}

// Unit returns one unit by identity.
func (graph *Graph) Unit(id string) (Unit, bool) {
	unit, ok := graph.units[id]
	return unit, ok
}

// OwnerOf returns the unit that declares a repository-relative path.
func (graph *Graph) OwnerOf(path string) (string, bool) {
	id, ok := graph.owner[path]
	return id, ok
}

// Canonical returns the deterministic JSON body the digest is taken over, after
// the domain tag. It is the persistable form: a graph rebuilt from repository
// authority produces byte-identical output.
func (graph *Graph) Canonical() ([]byte, error) {
	units := make([]digestUnit, 0, len(graph.order))
	for _, id := range graph.order {
		units = append(units, projectUnit(graph.units[id]))
	}
	return json.Marshal(digestBody{
		Languages:      graph.languages,
		Frontier:       graph.frontier,
		Units:          units,
		Documentation:  graph.documentation,
		RuntimeReaders: graph.runtimeReaders,
		RuntimeEdges:   graph.runtimeEdges,
		Unobserved:     namespacedEntries(graph.unobserved),
		Runnable:       namespacedEntries(graph.runnable),
	}, json.Deterministic(true))
}

// namespacedEntries is the digest projection of the unobserved or runnable
// directories: "namespace:directory" for every plugin, sorted; nil when there
// are none.
func namespacedEntries(byNamespace map[string][]string) []string {
	var entries []string
	for namespace, directories := range byNamespace {
		for _, directory := range directories {
			entries = append(entries, namespace+":"+directory)
		}
	}
	sort.Strings(entries)
	return entries
}

func projectUnit(unit Unit) digestUnit {
	return digestUnit{
		ID:                   unit.ID,
		Sources:              unit.Sources,
		Tests:                unit.Tests,
		Imports:              unit.Imports,
		TestImports:          unit.TestImports,
		Execs:                unit.Execs,
		AdvisoryImports:      unit.AdvisoryImports,
		RouteDispatchImports: unit.RouteDispatchImports,
		PathTokens:           unit.PathTokens,
		PathGlobs:            unit.PathGlobs,
		PathTokensBounded:    unit.PathTokensBounded,
		PlacesFileNames:      unit.PlacesFileNames,
		Embeds:               unit.Embeds,
		EmbedFiles:           unit.EmbedFiles,
		UnboundedReads:       unit.UnboundedReads,
		LocatesRoot:          unit.LocatesRoot,
		ReadScoped:           unit.ReadScoped,
		ReadScope:            unit.ReadScope,
		Frontier:             unit.Frontier,
		FrontierBears:        unit.FrontierBears,
		Callers:              unit.Callers,
		ExecConsumers:        unit.ExecConsumers,
	}
}

func sortedKeys(set map[string]bool) []string {
	values := make([]string, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

// ReachedUnitIDs names the complete reached universe before Select filters out
// units with no tests. Keep this walk aligned with Select; witness chains alone
// enumerate only one path and cannot establish the denominator of a diamond.
func (graph *Graph) ReachedUnitIDs(dirty []string) []string {
	reached, _, _, _, _, _, _, _ := graph.reach(NormalizePaths(dirty), true)
	ids := make([]string, 0, len(reached))
	for id := range reached {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// ReachedBeyondOwnTests returns, in one walk, the witness of every unit the
// dirty set reaches other than through a test-only Go unit's own changed test
// files (AFP-V0-036): such a unit appears only when an import edge, a reader,
// a declared read or another rule also reaches it. Every other unit has the
// witness Select would give it. The dirty paths are a superset of any subset
// a caller would plan instead, and every rule but that one only adds units as
// paths are added, so a unit absent here is absent from Select on any subset
// of the dirty paths too (STR-V0-072).
func (graph *Graph) ReachedBeyondOwnTests(dirty []string) map[string]Witness {
	reached, _, _, _, _, _, _, _ := graph.reach(NormalizePaths(dirty), false)
	// The walk holds chains by reference (V1-2979); copy them into Via as
	// MaterializeVia does, so each witness equals Select's.
	for id, witness := range reached {
		if witness.link != nil {
			witness.Via, witness.link = witness.Chain(), nil
			reached[id] = witness
		}
	}
	return reached
}

// UntestedCodeReached names, sorted, the units that declare no test file and
// that the dirty set reaches by a change to their own source or by an import
// path: the packages Select drops for want of a test, which only a compile
// check covers (AFP-V0-468). A unit reached only as a reader of the dirty path
// is left out: its own code did not change and nothing it imports did.
func (graph *Graph) UntestedCodeReached(dirty []string) []string {
	reached, _, _, _, _, _, _, _ := graph.reach(NormalizePaths(dirty), true)
	var ids []string
	for id, witness := range reached {
		if witness.Kind != WitnessDirectSource && witness.Kind != WitnessDependency {
			continue
		}
		if unit, ok := graph.units[id]; ok && len(unit.Tests) == 0 {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

// frontierSites counts every frontier site in the repository by reason
// (AFP-V0-381): each plugin and unit frontier entry is one site of the reason
// formed by its first two colon-separated fields, except a `reason:+N` entry,
// which counts the N sites its list left unnamed. Plugin-wide and unit
// frontiers are summed per list, so a count does not depend on the dirty set.
// It is nil when the repository has no frontier.
func (graph *Graph) frontierSites() map[string]int {
	var sites map[string]int
	add := func(entries []string) {
		for _, entry := range entries {
			if sites == nil {
				sites = make(map[string]int)
			}
			fields := strings.SplitN(entry, ":", 3)
			if len(fields) < 2 {
				sites[entry]++
				continue
			}
			reason := fields[0] + ":" + fields[1]
			if len(fields) == 3 && strings.HasPrefix(fields[2], "+") {
				if n, err := strconv.Atoi(fields[2][1:]); err == nil && n > 0 {
					sites[reason] += n
					continue
				}
			}
			sites[reason]++
		}
	}
	for _, name := range graph.languages {
		add(graph.frontierBy[name])
	}
	for id, unit := range graph.units {
		if !graph.promoted[id] {
			add(unit.Frontier)
		}
	}
	return sites
}
