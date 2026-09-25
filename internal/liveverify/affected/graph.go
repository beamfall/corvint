package affected

import (
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"fmt"
	"sort"
)

// Graph is the composed dependency graph over every participating language.
//
// A graph is immutable once built. Its digest covers every unit, every edge,
// and every frontier reason, so two graphs with the same digest select
// identically and a plan can name the exact universe it inspected.
type Graph struct {
	units      map[string]Unit
	order      []string
	owner      map[string]string
	dependents map[string][]string
	testUsers  map[string][]string
	testReach  map[string]bool
	unbounded  []string
	claimants  map[string]Language
	languages  []string
	frontier   []string
	frontierBy map[string][]string
	digest     string
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
}

// digestUnit projects one Unit; every member is always present.
type digestUnit struct {
	ID                string   `json:"id"`
	Sources           []string `json:"sources"`
	Tests             []string `json:"tests"`
	Imports           []string `json:"imports"`
	TestImports       []string `json:"testImports"`
	PathTokens        []string `json:"pathTokens"`
	PathTokensBounded bool     `json:"pathTokensBounded"`
	Embeds            bool     `json:"embeds"`
	UnboundedReads    string   `json:"unboundedReads"`
	LocatesRoot       bool     `json:"locatesRoot"`
	Frontier          []string `json:"frontier"`
}

// Build composes one graph from every supplied language plugin.
//
// Plugins are observed in the order given but the resulting graph is
// order-independent: identities, paths, and edges are all canonically sorted
// before the digest is taken.
func Build(root string, languages ...Language) (*Graph, error) {
	if len(languages) == 0 {
		return nil, fmt.Errorf("%w: no language plugins", ErrInvalidLanguage)
	}
	graph := &Graph{
		units:      make(map[string]Unit),
		owner:      make(map[string]string),
		dependents: make(map[string][]string),
		testUsers:  make(map[string][]string),
		claimants:  make(map[string]Language),
		frontierBy: make(map[string][]string),
	}
	seenLanguage := make(map[string]bool, len(languages))
	frontier := make(map[string]bool)
	defer holdWalks(root)()
	for _, language := range languages {
		name := language.Name()
		if name == "" || seenLanguage[name] {
			return nil, fmt.Errorf("%w: language name %q", ErrInvalidLanguage, name)
		}
		seenLanguage[name] = true
		graph.claimants[name] = language
		graph.languages = append(graph.languages, name)
		result, err := language.Units(root)
		if err != nil {
			return nil, fmt.Errorf("language %s: %w", name, err)
		}
		own := make(map[string]bool)
		if err := graph.admit(name, result, own); err != nil {
			return nil, err
		}
		graph.frontierBy[name] = sortedKeys(own)
		for reason := range own {
			frontier[reason] = true
		}
	}
	sort.Strings(graph.languages)
	graph.frontier = sortedKeys(frontier)
	graph.finish()
	digest, err := graph.computeDigest()
	if err != nil {
		return nil, err
	}
	graph.digest = digest
	return graph, nil
}

func (graph *Graph) admit(namespace string, result Result, frontier map[string]bool) error {
	for _, reason := range result.Frontier {
		if reason == "" {
			return fmt.Errorf("%w: empty frontier reason from %s", ErrInvalidLanguage, namespace)
		}
		frontier[reason] = true
	}
	for _, unit := range result.Units {
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
	graph.testUsers = graph.reverse(func(unit Unit) []string { return unit.TestImports })
	graph.testReach = graph.unitsReachingTests()
	graph.unbounded = graph.unboundedReaders()
}

// unboundedReaders lists, in id order, every unit whose reads no literal
// bounds (AFP-V0-012 rule (d)): its own UnboundedReads, or a dependency whose
// non-test code locates the root and so reads from it when this unit's code
// or tests call it.
func (graph *Graph) unboundedReaders() []string {
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
	reached := graph.traverse(locators)
	graph.testUsersOf(reached)
	for _, id := range own {
		reached[id] = Witness{}
	}
	ids := make([]string, 0, len(reached))
	for id := range reached {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
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
func (graph *Graph) unitsReachingTests() map[string]bool {
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
		for _, target := range graph.units[id].Imports {
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
		Languages: graph.languages,
		Frontier:  graph.frontier,
		Units:     units,
	}, json.Deterministic(true))
}

func projectUnit(unit Unit) digestUnit {
	return digestUnit{
		ID:                unit.ID,
		Sources:           unit.Sources,
		Tests:             unit.Tests,
		Imports:           unit.Imports,
		TestImports:       unit.TestImports,
		PathTokens:        unit.PathTokens,
		PathTokensBounded: unit.PathTokensBounded,
		Embeds:            unit.Embeds,
		UnboundedReads:    unit.UnboundedReads,
		LocatesRoot:       unit.LocatesRoot,
		Frontier:          unit.Frontier,
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
