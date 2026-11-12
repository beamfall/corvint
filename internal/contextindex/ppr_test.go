package contextindex

const graphFixtureTask = "Why does `AlphaWidget` in alpha/alpha.go break the widget pipeline?"

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
