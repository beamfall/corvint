package affected

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// RuntimeEdgesPath is the repository's declaration of the runtime edges no
// plugin models, such as an end-to-end test that drives a server another
// language builds (AFP-V0-819).
const RuntimeEdgesPath = ".corvint/runtime-edges.json"

// RuntimeEdgesProfile is the declaration's closed profile name.
const RuntimeEdgesProfile = "corvint-runtime-edges/0"

const (
	maxRuntimeEdgesBytes   = 64 << 10
	maxRuntimeEdgesEntries = 256
	maxRuntimeEdgeReason   = 512
)

// RuntimeEdge is one declared edge: every unit with a test file at Tests (a
// repository-relative file, or a directory when it ends in `/`) runs the
// built binary of the unit Runs names, for the project's stated Reason.
type RuntimeEdge struct {
	Tests  string `json:"tests"`
	Runs   string `json:"runs"`
	Reason string `json:"reason"`
}

type runtimeEdgesFile struct {
	Profile string        `json:"profile"`
	Edges   []RuntimeEdge `json:"edges"`
}

// CrossLanguageFrontier is implemented by a plugin some of whose frontier
// reasons stand for a runtime edge from another plugin's code into its units,
// such as a TypeScript end-to-end test that drives a Go server (AFP-V0-818).
// Each reason is `<plugin>:<category>`, as for FrontierReach.
type CrossLanguageFrontier interface {
	CrossLanguageFrontiers() []string
}

// readRuntimeEdges returns the declared edges: nil with no declaration, an
// error for one that is unreadable, oversized, not a regular file, or invalid
// in any member. Edges are strictly ascending by tests then runs, and each
// carries a non-empty printable reason.
func readRuntimeEdges(root *Source) ([]RuntimeEdge, error) {
	if root == nil {
		return nil, nil
	}
	body, err := root.ReadBounded(RuntimeEdgesPath, maxRuntimeEdgesBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", RuntimeEdgesPath, err)
	}
	var declared runtimeEdgesFile
	if err := json.UnmarshalRead(bytes.NewReader(body), &declared, json.RejectUnknownMembers(true)); err != nil {
		return nil, fmt.Errorf("%s: %w", RuntimeEdgesPath, err)
	}
	if declared.Profile != RuntimeEdgesProfile || declared.Edges == nil || len(declared.Edges) > maxRuntimeEdgesEntries {
		return nil, fmt.Errorf("%s: profile must be %s with at most %d edges", RuntimeEdgesPath, RuntimeEdgesProfile, maxRuntimeEdgesEntries)
	}
	for index, edge := range declared.Edges {
		if !ValidReadScopeEntry(edge.Tests) || strings.TrimSuffix(edge.Tests, "/") == "" {
			return nil, fmt.Errorf("%s: tests %q", RuntimeEdgesPath, edge.Tests)
		}
		if namespace, name, found := strings.Cut(edge.Runs, ":"); !found || namespace == "" || name == "" || !utf8.ValidString(edge.Runs) || strings.ContainsFunc(edge.Runs, unicode.IsSpace) {
			return nil, fmt.Errorf("%s: runs %q", RuntimeEdgesPath, edge.Runs)
		}
		if edge.Reason == "" || len(edge.Reason) > maxRuntimeEdgeReason || !utf8.ValidString(edge.Reason) || strings.ContainsFunc(edge.Reason, unicode.IsControl) {
			return nil, fmt.Errorf("%s: reason of %q", RuntimeEdgesPath, edge.Tests)
		}
		if index > 0 {
			previous := declared.Edges[index-1]
			if edge.Tests < previous.Tests || edge.Tests == previous.Tests && edge.Runs <= previous.Runs {
				return nil, fmt.Errorf("%s: edges must ascend by tests then runs: %q", RuntimeEdgesPath, edge.Tests)
			}
		}
	}
	return declared.Edges, nil
}

// runtimeEdgeHolds reports whether a declared tests entry holds path.
func runtimeEdgeHolds(entry, path string) bool {
	return path == entry || strings.HasSuffix(entry, "/") && strings.HasPrefix(path, entry)
}

// applyRuntimeEdges adds each declared edge's Runs to the Execs of every unit
// with a test file the edge holds, so the unit is selected with a BINARY_EXEC
// witness whenever a change reaches the unit it runs (AFP-V0-037, AFP-V0-819).
// A test file holds the edge, so an edge the unit's code does not already
// run is test-only (TestOnlyExecs, AFP-V0-2991): it adds the unit one level
// past what reaches the run unit and walks no further.
// An edge whose Runs names no unit of the graph, or whose tests hold no test
// file, fails a full build: a stale declaration must not silently stop
// selecting. A partial graph, built from a subset of the plugins, cannot tell
// a stale edge from one whose other side an absent plugin observes, so it
// skips such an edge instead.
func (graph *Graph) applyRuntimeEdges(partial, widen bool) error {
	for _, edge := range graph.runtimeEdges {
		_, exists := graph.units[edge.Runs]
		if !exists {
			if partial {
				continue
			}
			if !widen {
				return &unknownRuntimeUnit{runs: edge.Runs}
			}
		}
		var holders []string
		for _, id := range graph.order {
			if id != edge.Runs && slices.ContainsFunc(graph.units[id].Tests, func(path string) bool { return runtimeEdgeHolds(edge.Tests, path) }) {
				holders = append(holders, id)
			}
		}
		if len(holders) == 0 {
			if partial {
				continue
			}
			return fmt.Errorf("%s: tests %q holds no test file", RuntimeEdgesPath, edge.Tests)
		}
		if !exists {
			graph.degradedRuntimeEdges = append(graph.degradedRuntimeEdges, edge)
			continue
		}
		for _, id := range holders {
			unit := graph.units[id]
			if !slices.Contains(unit.Execs, edge.Runs) {
				unit.Execs = append(slices.Clone(unit.Execs), edge.Runs)
				slices.Sort(unit.Execs)
				unit.TestOnlyExecs = append(slices.Clone(unit.TestOnlyExecs), edge.Runs)
				slices.Sort(unit.TestOnlyExecs)
				graph.units[id] = unit
			}
		}
	}
	return nil
}

// crossLanguageEntries returns the frontier entries of a plugin left out by
// AFP-V0-560 that its CrossLanguageFrontier names and that no declared runtime
// edge covers (AFP-V0-818). An entry is covered when its site, the path after
// its reason, is a file a declared edge holds; an entry without a site path is
// never covered.
func (graph *Graph) crossLanguageEntries(name string) []string {
	cross, declares := graph.claimants[name].(CrossLanguageFrontier)
	if !declares {
		return nil
	}
	reasons := cross.CrossLanguageFrontiers()
	var entries []string
	for _, entry := range graph.frontierBy[name] {
		namespace, rest, _ := strings.Cut(entry, ":")
		category, site, _ := strings.Cut(rest, ":")
		if !slices.Contains(reasons, namespace+":"+category) {
			continue
		}
		file, _, _ := strings.Cut(site, ":")
		if file != "" && slices.ContainsFunc(graph.runtimeEdges, func(edge RuntimeEdge) bool { return runtimeEdgeHolds(edge.Tests, file) }) {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

// unknownRuntimeUnit preserves the strict AFP-V0-819 refusal text while
// distinguishing it from malformed declarations and unrelated plugin failures.
type unknownRuntimeUnit struct{ runs string }

func (err *unknownRuntimeUnit) Error() string {
	return fmt.Sprintf("%s: runs %q names no unit", RuntimeEdgesPath, err.runs)
}

// IsUnknownRuntimeUnit admits only the typed strict runtime-unit refusal.
func IsUnknownRuntimeUnit(err error) bool {
	var unknown *unknownRuntimeUnit
	return errors.As(err, &unknown)
}

// BuildWithRuntimeEdgeWidening revalidates the complete graph and widens only
// unresolved runtime targets with known test holders (CIN-V1-940). Build and
// BuildPartial retain their existing contracts. This path is deliberately
// uncached: a failed strict build never publishes a successful cache entry.
func BuildWithRuntimeEdgeWidening(ctx context.Context, root string, languages ...Language) (*Graph, error) {
	graph, _, err := buildDisk(ctx, DiskSourceContext(ctx, root), languages, false, true)
	return graph, err
}

func runtimeEdgeDegradation(edge RuntimeEdge) string {
	return fmt.Sprintf("runtime:unresolved-edge: tests=%q runs=%q: runs names no unit; reason=%q", edge.Tests, edge.Runs, edge.Reason)
}

// registerRuntimeWidening makes the degradation an explicit unbounded cause,
// shared by selection, graph inspection and independent witness verification.
// A declared read scope or name-only read cannot bound an unresolved runtime
// target, so none of those narrower proofs applies to this closure.
func (graph *Graph) registerRuntimeWidening() {
	graph.runtimeWidening = graph.runtimeEdgeWidening()
	for id := range graph.runtimeWidening {
		graph.unbounded = append(graph.unbounded, id)
		graph.causes[id] = id
		graph.chains[id] = []string{id}
		delete(graph.namesOnly, id)
		delete(graph.runnerOnly, id)
	}
	slices.Sort(graph.unbounded)
	graph.unbounded = slices.Compact(graph.unbounded)
}

// runtimeEdgeWidening computes the full reverse closure, including terminal
// test/exec edges and file readers. Unlike normal change propagation it never
// stops at a test-only edge or a reader: uncertainty must cover that whole area.
// Every round adds a unit, bounding the closure by the observed unit count.
// It runs on the fresh graph before change-status or semantic pruning inputs.
func (graph *Graph) runtimeEdgeWidening() map[string]Witness {
	if len(graph.degradedRuntimeEdges) == 0 {
		return nil
	}
	widened := map[string]Witness{}
	for _, edge := range graph.degradedRuntimeEdges {
		cause := runtimeEdgeDegradation(edge)
		reached := map[string]Witness{}
		for _, id := range graph.order {
			if slices.ContainsFunc(graph.units[id].Tests, func(p string) bool { return runtimeEdgeHolds(edge.Tests, p) }) {
				reached[id] = Witness{Via: []string{id}}
			}
		}
		expanded := map[string]bool{}
		for {
			var paths []string
			var next []string
			for _, id := range slices.Sorted(maps.Keys(reached)) {
				if !expanded[id] {
					next = append(next, id)
				}
			}
			if len(next) == 0 {
				break
			}
			for _, id := range next {
				expanded[id] = true
				unit := graph.units[id]
				paths = append(paths, unit.Sources...)
				paths = append(paths, unit.Tests...)
				for _, reverse := range []map[string][]string{graph.dependents, graph.advisory, graph.routed, graph.testUsers, graph.execUsers} {
					for _, user := range reverse[id] {
						if _, exists := reached[user]; !exists {
							reached[user] = Witness{Via: []string{user}}
						}
					}
				}
			}
			paths = NormalizePaths(paths)
			for _, p := range paths {
				for _, user := range graph.embedded[p] {
					if _, exists := reached[user]; !exists {
						reached[user] = Witness{Via: []string{user}}
					}
				}
			}
			graph.readers(reached, paths)
			graph.scopedReadersOf(reached, paths)
			graph.existenceReadersOf(reached, paths)
			graph.testEmbedReadersOf(reached, paths)
			graph.importGraphReadersOf(reached, paths)
		}
		for _, id := range slices.Sorted(maps.Keys(reached)) {
			if _, exists := widened[id]; !exists {
				widened[id] = Witness{Kind: WitnessUnboundedReader, Via: []string{id}, Cause: &UnboundedCause{Unit: id, Reads: cause}}
			}
		}
	}
	return widened
}
