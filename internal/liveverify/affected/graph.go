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
	testReach  map[string]bool
	claimants  map[string]Language
	languages  []string
	frontier   []string
	digest     string
}

type graphBody struct {
	Languages []string `json:"languages"`
	Frontier  []string `json:"frontier"`
	Units     []Unit   `json:"units"`
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
		claimants:  make(map[string]Language),
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
		if err := graph.admit(name, result, frontier); err != nil {
			return nil, err
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

// finish materializes the reverse edges. An import naming a unit outside the
// graph is dropped: it is an external dependency, and an external dependency
// cannot be a dirty repository path.
func (graph *Graph) finish() {
	sort.Strings(graph.order)
	for _, id := range graph.order {
		for _, target := range graph.units[id].Imports {
			if _, known := graph.units[target]; !known {
				continue
			}
			graph.dependents[target] = append(graph.dependents[target], id)
		}
	}
	for target := range graph.dependents {
		sort.Strings(graph.dependents[target])
	}
	graph.testReach = graph.unitsReachingTests()
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
	units := make([]Unit, 0, len(graph.order))
	for _, id := range graph.order {
		units = append(units, graph.units[id])
	}
	body, err := json.Marshal(graphBody{
		Languages: graph.languages,
		Frontier:  graph.frontier,
		Units:     units,
	}, json.Deterministic(true))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
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

// Canonical returns the deterministic JSON body the digest is taken over. It is
// the persistable form: a graph rebuilt from repository authority produces
// byte-identical output.
func (graph *Graph) Canonical() ([]byte, error) {
	units := make([]Unit, 0, len(graph.order))
	for _, id := range graph.order {
		units = append(units, graph.units[id])
	}
	return json.Marshal(graphBody{
		Languages: graph.languages,
		Frontier:  graph.frontier,
		Units:     units,
	}, json.Deterministic(true))
}

func sortedKeys(set map[string]bool) []string {
	values := make([]string, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}
