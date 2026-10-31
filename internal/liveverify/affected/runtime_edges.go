package affected

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
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
// An edge whose Runs names no unit of the graph, or whose tests hold no test
// file, fails the build: a stale declaration must not silently stop selecting.
func (graph *Graph) applyRuntimeEdges() error {
	for _, edge := range graph.runtimeEdges {
		if _, exists := graph.units[edge.Runs]; !exists {
			return fmt.Errorf("%s: runs %q names no unit", RuntimeEdgesPath, edge.Runs)
		}
		held := false
		for _, id := range graph.order {
			unit := graph.units[id]
			if id == edge.Runs || !slices.ContainsFunc(unit.Tests, func(path string) bool { return runtimeEdgeHolds(edge.Tests, path) }) {
				continue
			}
			held = true
			if !slices.Contains(unit.Execs, edge.Runs) {
				unit.Execs = append(slices.Clone(unit.Execs), edge.Runs)
				slices.Sort(unit.Execs)
				graph.units[id] = unit
			}
		}
		if !held {
			return fmt.Errorf("%s: tests %q holds no test file", RuntimeEdgesPath, edge.Tests)
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
