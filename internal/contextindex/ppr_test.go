package contextindex

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"
)

const graphFixtureTask = "Why does `AlphaWidget` in alpha/alpha.go break the widget pipeline?"

func graphUnexamined(packet map[string]any) map[string]any {
	for _, item := range mapsFromAny(packet["coverage"].(map[string]any)["unexamined"]) {
		if item["relation"] == contextGraphRelation {
			return item
		}
	}
	return nil
}

func TestContextGraphSlotRanksFromTheTaskAnchors(t *testing.T) {
	index := identGraphFixture(t)
	t.Setenv("CORVINT_CONTEXT_GRAPH", "on")
	packet, err := TaskContext(context.Background(), index, graphFixtureTask, "", 5)
	if err != nil {
		t.Fatal(err)
	}
	results := mapsFromAny(packet["results"])
	t.Run("TCP-V0-031", func(t *testing.T) {
		found := contextRowsByKind(t, packet)
		if !slices.Equal(found[contextGraphRelation], []string{"beta/beta.go", "gamma/gamma.go"}) {
			t.Fatalf("graph rows = %v, want beta then gamma", found[contextGraphRelation])
		}
		seenGraph := false
		for _, item := range results {
			kind := item["kind"].(string)
			seenGraph = seenGraph || kind == contextGraphRelation
			if seenGraph && kind != contextGraphRelation && kind != "lexical" && kind != "documentation" {
				t.Fatalf("a %s row follows a graph row: the graph must not outrank a relation", kind)
			}
		}
		if state := graphUnexamined(packet); state == nil || state["state"] != "examined" {
			t.Fatalf("graph receipt = %v", state)
		}
	})
	t.Run("TCP-V0-032", func(t *testing.T) {
		for _, item := range results {
			if item["id"] != "gamma/gamma.go" {
				continue
			}
			reason := item["evidence"].([]any)[0].(map[string]any)["reason"].(string)
			want := "graph from seed `alpha/alpha.go` (mentioned): `alpha/alpha.go` defines `Quexel`, which `beta/beta.go` names; `beta/beta.go` defines `Brindle`, which `gamma/gamma.go` names"
			if reason != want {
				t.Fatalf("reason = %q\nwant %q", reason, want)
			}
			if !strings.Contains(item["action"].(string), reason) {
				t.Fatalf("action does not carry the hop path: %q", item["action"])
			}
			return
		}
		t.Fatal("no gamma row")
	})
	t.Run("TCP-V0-034", func(t *testing.T) {
		for _, limit := range []int{1, 3, 4} {
			bounded, err := TaskContext(context.Background(), index, graphFixtureTask, "", limit)
			if err != nil {
				t.Fatal(err)
			}
			rows := mapsFromAny(bounded["results"])
			if len(rows) > limit || rows[0]["id"] != "alpha/alpha.go" {
				t.Fatalf("limit %d: %d rows, first %v", limit, len(rows), rows[0]["id"])
			}
		}
		if packet["mutates"] != false {
			t.Fatalf("mutates = %v", packet["mutates"])
		}
	})
}

func TestContextGraphSlotAbstains(t *testing.T) {
	t.Run("TCP-V0-033", func(t *testing.T) {
		index := identGraphFixture(t)
		t.Setenv("CORVINT_CONTEXT_GRAPH", "on")
		packet, err := TaskContext(context.Background(), index, "the widget pipeline stalls", "", 20)
		if err != nil {
			t.Fatal(err)
		}
		if found := contextRowsByKind(t, packet); len(found[contextGraphRelation]) != 0 || graphUnexamined(packet)["state"] != "no-seed" {
			t.Fatalf("no anchor: graph rows %v, receipt %v", found[contextGraphRelation], graphUnexamined(packet))
		}
		index.Vocabulary.IdentGraph = &identGraph{Nodes: index.Vocabulary.IdentGraph.Nodes, Bounded: true, Offsets: []uint32{0}}
		packet, err = TaskContext(context.Background(), index, graphFixtureTask, "", 20)
		if err != nil {
			t.Fatal(err)
		}
		if found := contextRowsByKind(t, packet); len(found[contextGraphRelation]) != 0 || graphUnexamined(packet)["state"] != "graph-bounded" {
			t.Fatalf("bounded graph: graph rows %v, receipt %v", found[contextGraphRelation], graphUnexamined(packet))
		}
		graph := identGraphFixture(t).Vocabulary.IdentGraph
		if _, converged := personalizedPageRank(graph, []contextGraphSeed{{node: 0, anchor: "mentioned"}}, 0); converged {
			t.Fatal("a walk past its push bound ranked")
		}
		unsupported, err := TaskContext(context.Background(), identGraphFixture(t), "Does `httpRetry` call `retry_loop` inside the widget?", "", 20)
		if err != nil {
			t.Fatal(err)
		}
		if len(unsupported["results"].([]any)) != 0 {
			t.Fatalf("TCP-V0-016 abstention changed under the graph flag: %v", contextRowsByKind(t, unsupported))
		}
	})
}

func TestContextGraphDefaultBytes(t *testing.T) {
	t.Run("TCP-V0-034", func(t *testing.T) {
		index := recipeFixtureIndex(t)
		golden, err := os.ReadFile("testdata/context-recipe-default-golden.json")
		if err != nil {
			t.Fatal(err)
		}
		for _, flag := range []string{"", "off", "unknown"} {
			t.Setenv("CORVINT_CONTEXT_GRAPH", flag)
			if newTaskContextCompiler(index, recipeFixtureTask, "").graphRanking {
				t.Fatalf("flag %q enabled the graph slot", flag)
			}
			packet := recipePacket(t, index, "", recipeFixtureTask)
			encoded, err := json.MarshalIndent(packet, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(append(encoded, '\n'), golden) || graphUnexamined(packet) != nil {
				t.Fatalf("flag %q changed golden packet bytes", flag)
			}
		}
	})
}

// pprGraph is a symmetric CSR graph over undirected weighted edges.
func pprGraph(nodes uint32, edges [][3]uint32) *identGraph {
	adjacency := make([][][2]uint32, nodes)
	for _, edge := range edges {
		adjacency[edge[0]] = append(adjacency[edge[0]], [2]uint32{edge[1], edge[2]})
		adjacency[edge[1]] = append(adjacency[edge[1]], [2]uint32{edge[0], edge[2]})
	}
	graph := &identGraph{Nodes: nodes, Offsets: []uint32{0}}
	for _, targets := range adjacency {
		for _, target := range targets {
			graph.Targets = append(graph.Targets, target[0])
			graph.Weights = append(graph.Weights, target[1])
		}
		graph.Offsets = append(graph.Offsets, uint32(len(graph.Targets)))
	}
	return graph
}

// pprStar is a unit-weight hub, node 0, joined to each of its leaves: the
// shape the audit found rescanning the hub once per leaf push.
func pprStar(leaves uint32) *identGraph {
	edges := make([][3]uint32, 0, leaves)
	for leaf := uint32(1); leaf <= leaves; leaf++ {
		edges = append(edges, [3]uint32{0, leaf, 1})
	}
	return pprGraph(leaves+1, edges)
}

func pprLeafSeeds(count uint32) []contextGraphSeed {
	seeds := make([]contextGraphSeed, 0, count)
	for leaf := uint32(1); leaf <= count; leaf++ {
		seeds = append(seeds, contextGraphSeed{node: leaf, anchor: "mentioned"})
	}
	return seeds
}

// TCP-V0-031 (V1-0372): the graph is immutable for the walk, so a node's
// weighted degree is read once; a rescan of a hub's adjacency on every leaf
// push is repeated work that grows with the hub's degree.
func TestPersonalizedPageRankScansEachDegreeOnce(t *testing.T) {
	scan := graphDegree
	t.Cleanup(func() { graphDegree = scan })
	scans := map[uint32]int{}
	graphDegree = func(graph *identGraph, node uint32) float64 {
		scans[node]++
		return scan(graph, node)
	}
	if _, converged := personalizedPageRank(pprStar(2000), pprLeafSeeds(contextGraphSeedCap), contextGraphMaxPushes); !converged {
		t.Fatal("star walk did not converge")
	}
	if scans[0] != 1 {
		t.Fatalf("hub degree scanned %d times in one walk, want once", scans[0])
	}
	for node, count := range scans {
		if count != 1 {
			t.Fatalf("node %d degree scanned %d times in one walk, want once", node, count)
		}
	}
}

// unmemoizedPersonalizedPageRank is personalizedPageRank as it stood before
// V1-0372, kept verbatim as the equivalence oracle.
func unmemoizedPersonalizedPageRank(graph *identGraph, seeds []contextGraphSeed, maxPushes int) (map[uint32]float64, bool) {
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

// V1-0372: memoizing the degree changes no rank bit, no convergence verdict
// and no push-bound refusal.
func TestPersonalizedPageRankMatchesTheUnmemoizedWalk(t *testing.T) {
	ring := make([][3]uint32, 0)
	for node := uint32(0); node < 64; node++ {
		ring = append(ring, [3]uint32{node, (node + 1) % 64, node%9 + 1})
		if chord := (node*7 + 3) % 64; chord != node {
			ring = append(ring, [3]uint32{node, chord, (node*13)%5 + 1})
		}
		if node%8 != 0 {
			ring = append(ring, [3]uint32{node - node%8, node, 3})
		}
	}
	for _, testCase := range []struct {
		name  string
		graph *identGraph
		seeds []contextGraphSeed
	}{
		{"star seeded at the hub", pprStar(2000), []contextGraphSeed{{node: 0, anchor: "subject"}}},
		{"star seeded at leaves", pprStar(2000), pprLeafSeeds(contextGraphSeedCap)},
		{"weighted hubs on a ring", pprGraph(64, ring), []contextGraphSeed{{node: 5, anchor: "subject"}, {node: 40, anchor: "mentioned"}, {node: 5, anchor: "pair"}}},
		{"isolated seed", pprGraph(3, [][3]uint32{{1, 2, 4}}), []contextGraphSeed{{node: 0, anchor: "subject"}, {node: 1, anchor: "mentioned"}}},
	} {
		for _, maxPushes := range []int{0, 1, 7, contextGraphMaxPushes} {
			wantRank, wantConverged := unmemoizedPersonalizedPageRank(testCase.graph, testCase.seeds, maxPushes)
			rank, converged := personalizedPageRank(testCase.graph, testCase.seeds, maxPushes)
			if converged != wantConverged || !maps.Equal(rank, wantRank) || (rank == nil) != (wantRank == nil) {
				t.Fatalf("%s at %d pushes: converged %v rank %v, want %v %v", testCase.name, maxPushes, converged, rank, wantConverged, wantRank)
			}
		}
	}
}
