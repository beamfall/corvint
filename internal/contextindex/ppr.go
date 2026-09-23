package contextindex

import (
	"slices"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/runtimeenv"
)

// The `graph` slot (TCP-V0-031..034) ranks files by personalized PageRank
// over the identifier graph (identgraph.go), seeded by the task's anchors. It
// runs only under `CORVINT_CONTEXT_GRAPH=on`; unset, the compiler never
// consults the graph and the packet is byte-identical to the one before it.
const (
	contextGraphRelation = "graph"
	// contextGraphCap is the rows the slot may place; they take the packet's
	// last positions and so displace only lexical tail rows.
	contextGraphCap = 5
	// contextGraphSeedCap bounds the anchors the walk restarts at.
	contextGraphSeedCap = 16
	// contextGraphCandidates bounds the ranked candidates materialised for
	// the `withheld` count.
	contextGraphCandidates = 50
	contextGraphMaxHops    = 3
	contextGraphMaxPushes  = 200_000
	contextGraphAlpha      = 0.15
	contextGraphEpsilon    = 1e-5
	contextGraphScore      = 250
)

// contextGraphSeedKinds are the rows that anchor the walk: the relations the
// task's own paths and identifiers establish. Lexical, test, co-change and
// sibling rows are ranking guesses, not anchors, and seed nothing.
var contextGraphSeedKinds = []string{"mentioned", "pair", "definition", "reverse-import", "reference"}

type contextGraphSeed struct {
	node   uint32
	anchor string
}

func configureContextGraph(compiler *taskContextCompiler) *taskContextCompiler {
	compiler.graphRanking = runtimeenv.Value("CONTEXT_GRAPH") == "on"
	return compiler
}

// identGraph returns the snapshot's graph, deriving it once for an index
// compiled without one, as vocabulary() does for the term table.
func (index *Index) identGraph() *identGraph {
	table := index.vocabulary()
	if table.IdentGraph == nil {
		table.IdentGraph = index.buildIdentGraph(table)
	}
	return table.IdentGraph
}

// placeGraphRows admits up to contextGraphCap graph rows and places them at
// the last positions under limit that follow every relation row, so they
// replace lexical tail rows and never outrank a relation row. A candidate a
// lexical or documentation row holds at or past the first position the
// graph rows could take is taken over, not skipped: that row would otherwise
// be the one the graph rows push past the limit.
func (compiler *taskContextCompiler) placeGraphRows(rows []contextRow, limit int) []contextRow {
	if !compiler.graphRanking {
		return rows
	}
	compiler.markRan(contextGraphRelation)
	candidates, state := compiler.graphCandidates(rows)
	if state != "" {
		compiler.markState(state, contextGraphRelation)
		return rows
	}
	floor := graphFloor(rows)
	admitted := compiler.takeGraph(candidates, graphHeld(rows, max(limit-contextGraphCap, floor)))
	rows = slices.DeleteFunc(rows, func(row contextRow) bool {
		return slices.ContainsFunc(admitted, func(graphRow contextRow) bool { return graphRow.path == row.path })
	})
	at := min(max(limit-len(admitted), floor), len(rows))
	return slices.Concat(rows[:at], admitted, rows[at:])
}

// graphHeld maps every row's path to whether a graph row may replace it: a
// lexical or documentation row from position zone. Reserved and relation
// rows are never replaced.
func graphHeld(rows []contextRow, zone int) map[string]bool {
	held := make(map[string]bool, len(rows))
	for position, row := range rows {
		held[row.path] = position >= zone && (row.kind == "lexical" || row.kind == "documentation")
	}
	return held
}

// graphFloor is the position after the last row a relation admitted.
func graphFloor(rows []contextRow) int {
	floor := 0
	for position, row := range rows {
		if row.kind != "lexical" && row.kind != "documentation" {
			floor = position + 1
		}
	}
	return floor
}

// takeGraph records every candidate and admits up to the cap, never the
// subject or a held path other than a replaceable one. It registers no
// corroboration: the graph runs after the ranking that corroboration feeds.
func (compiler *taskContextCompiler) takeGraph(candidates []contextRow, held map[string]bool) []contextRow {
	admitted := make([]contextRow, 0, contextGraphCap)
	for _, candidate := range candidates {
		compiler.candidates[contextGraphRelation] = append(compiler.candidates[contextGraphRelation], candidate.path)
		replaceable, present := held[candidate.path]
		if present && !replaceable || candidate.path == compiler.subject {
			continue
		}
		if len(admitted) == contextGraphCap {
			compiler.slotOmitted = true
			continue
		}
		compiler.chosen[candidate.path] = struct{}{}
		admitted = append(admitted, candidate)
	}
	return admitted
}

// graphCandidates ranks the files within contextGraphMaxHops of a seed by
// personalized PageRank, or names the abstention state: `no-seed` when the
// task anchors nothing the graph carries, `graph-bounded` when the graph or
// the walk passed its bound (TCP-V0-033).
func (compiler *taskContextCompiler) graphCandidates(rows []contextRow) ([]contextRow, string) {
	graph := compiler.index.identGraph()
	if graph.Bounded {
		return nil, "graph-bounded"
	}
	table := compiler.index.vocabulary()
	seeds := compiler.graphSeeds(rows, table)
	if len(seeds) == 0 {
		return nil, "no-seed"
	}
	rank, converged := personalizedPageRank(graph, seeds, contextGraphMaxPushes)
	if !converged {
		return nil, "graph-bounded"
	}
	parents := graphHops(graph, seeds)
	ranked := rankGraphNodes(rank, parents, seeds, table)
	candidates := make([]contextRow, 0, len(ranked))
	for _, node := range ranked {
		candidates = append(candidates, compiler.graphRow(graph, table, seeds, parents, node))
	}
	return candidates, ""
}

// graphSeeds is the subject, then each anchored row's path, in packet order,
// deduplicated and capped.
func (compiler *taskContextCompiler) graphSeeds(rows []contextRow, table *TermTable) []contextGraphSeed {
	seeds := make([]contextGraphSeed, 0, contextGraphSeedCap)
	seen := map[uint32]struct{}{}
	add := func(candidate, anchor string) {
		node, ok := table.sourceID(candidate)
		if _, dup := seen[uint32(node)]; !ok || dup || len(seeds) == contextGraphSeedCap {
			return
		}
		seen[uint32(node)] = struct{}{}
		seeds = append(seeds, contextGraphSeed{node: uint32(node), anchor: anchor})
	}
	if compiler.subject != "" {
		add(compiler.subject, "subject")
	}
	for _, row := range rows {
		if slices.Contains(contextGraphSeedKinds, row.kind) {
			add(row.path, row.kind)
		}
	}
	return seeds
}

// personalizedPageRank is the forward-push approximation (Andersen, Chung
// and Lang) with restart probability contextGraphAlpha to the seeds, uniform
// over them: a node is pushed while its residual is at least
// contextGraphEpsilon times its weighted degree. The queue is FIFO over a
// fixed seed and CSR order, so the result is deterministic. It reports false
// past maxPushes (contextGraphMaxPushes) rather than rank a partial walk.
func personalizedPageRank(graph *identGraph, seeds []contextGraphSeed, maxPushes int) (map[uint32]float64, bool) {
	rank, residual := map[uint32]float64{}, map[uint32]float64{}
	queued := map[uint32]bool{}
	queue := make([]uint32, 0, len(seeds))
	for _, seed := range seeds {
		residual[seed.node] += 1 / float64(len(seeds))
		queue = append(queue, seed.node)
		queued[seed.node] = true
	}
	for pushes := 0; len(queue) > 0; pushes++ {
		if pushes == maxPushes {
			return nil, false
		}
		node := queue[0]
		queue, queued[node] = queue[1:], false
		degree := graph.degree(node)
		if degree == 0 || residual[node] < contextGraphEpsilon*degree {
			continue
		}
		mass := residual[node]
		rank[node] += float64(contextGraphAlpha * mass)
		residual[node] = 0
		spread := float64((1 - contextGraphAlpha) * mass / degree)
		low, high := graph.edges(node)
		for edge := low; edge < high; edge++ {
			target := graph.Targets[edge]
			residual[target] += float64(spread * float64(graph.Weights[edge]))
			if !queued[target] && residual[target] >= contextGraphEpsilon*graph.degree(target) {
				queue = append(queue, target)
				queued[target] = true
			}
		}
	}
	return rank, true
}

func (graph *identGraph) degree(node uint32) float64 {
	low, high := graph.edges(node)
	total := uint64(0)
	for edge := low; edge < high; edge++ {
		total += uint64(graph.Weights[edge])
	}
	return float64(total)
}

// graphHops is a breadth-first walk from the seeds in seed order, to
// contextGraphMaxHops, recording each reached node's parent edge: the hop
// path a row's reason names is the shortest one, ties to the earlier seed
// and the lower CSR edge.
func graphHops(graph *identGraph, seeds []contextGraphSeed) map[uint32]int {
	parents := map[uint32]int{}
	frontier := make([]uint32, 0, len(seeds))
	for _, seed := range seeds {
		parents[seed.node] = -1
		frontier = append(frontier, seed.node)
	}
	for hop := 0; hop < contextGraphMaxHops; hop++ {
		next := make([]uint32, 0)
		for _, node := range frontier {
			low, high := graph.edges(node)
			for edge := low; edge < high; edge++ {
				target := graph.Targets[edge]
				if _, reached := parents[target]; reached {
					continue
				}
				parents[target] = edge
				next = append(next, target)
			}
		}
		frontier = next
	}
	return parents
}

// rankGraphNodes orders the reached, ranked, non-seed nodes by rank, then
// path, capped at contextGraphCandidates.
func rankGraphNodes(rank map[uint32]float64, parents map[uint32]int, seeds []contextGraphSeed, table *TermTable) []uint32 {
	ranked := make([]uint32, 0, len(rank))
	for node, value := range rank {
		edge, reached := parents[node]
		if reached && edge >= 0 && value > 0 {
			ranked = append(ranked, node)
		}
	}
	sort.Slice(ranked, func(left, right int) bool {
		if rank[ranked[left]] != rank[ranked[right]] {
			return rank[ranked[left]] > rank[ranked[right]]
		}
		return table.Paths[ranked[left]] < table.Paths[ranked[right]]
	})
	return ranked[:min(len(ranked), contextGraphCandidates)]
}

// graphRow states the seed anchor and every hop from it (TCP-V0-032).
func (compiler *taskContextCompiler) graphRow(graph *identGraph, table *TermTable, seeds []contextGraphSeed, parents map[uint32]int, node uint32) contextRow {
	hops := make([]string, 0, contextGraphMaxHops)
	line := compiler.graphLine(graph, parents[node], table.Paths[node])
	current := node
	for parents[current] >= 0 {
		edge := parents[current]
		from := graphEdgeSource(graph, edge)
		name, refers := graph.name(graph.Labels[edge])
		hop := "`" + table.Paths[from] + "` defines `" + name + "`, which `" + table.Paths[current] + "` names"
		if refers {
			hop = "`" + table.Paths[from] + "` names `" + name + "`, which `" + table.Paths[current] + "` defines"
		}
		hops = append(hops, hop)
		current = from
	}
	slices.Reverse(hops)
	seed := seeds[slices.IndexFunc(seeds, func(seed contextGraphSeed) bool { return seed.node == current })]
	reason := "graph from seed `" + table.Paths[current] + "` (" + seed.anchor + "): " + strings.Join(hops, "; ")
	return contextRow{
		kind: contextGraphRelation, path: table.Paths[node], score: contextGraphScore, line: line,
		summary:    "ranked by personalized PageRank over identifier definitions and references from the task's anchors",
		reason:     reason,
		confidence: "low", authority: SyntaxAuthority,
	}
}

// graphLine is the definition line of the last hop's name in the row's own
// file when the row defines it, else line 1.
func (compiler *taskContextCompiler) graphLine(graph *identGraph, edge int, candidate string) int {
	name, refers := graph.name(graph.Labels[edge])
	if !refers {
		return 1
	}
	for _, symbol := range compiler.index.Symbols {
		if symbol.Name == name && symbol.Path == candidate {
			return max(symbol.Line, 1)
		}
	}
	return 1
}

// graphEdgeSource is the node whose CSR range holds edge.
func graphEdgeSource(graph *identGraph, edge int) uint32 {
	return uint32(sort.Search(len(graph.Offsets)-1, func(node int) bool { return int(graph.Offsets[node+1]) > edge }))
}

// graphAction is the row's one-sentence action (decision 0028).
func graphAction(row contextRow) string {
	return "Read this file only if the hop path matters (" + row.reason + "); a graph hop from an anchor is a ranking, not a relation."
}

// contextRelations is TCP-V0-011's relation order, with `graph` last only
// when the slot is enabled, so the default packet's receipt is unchanged.
func (compiler *taskContextCompiler) contextRelations() []string {
	if !compiler.graphRanking {
		return contextRelationOrder
	}
	return append(slices.Clone(contextRelationOrder), contextGraphRelation)
}
