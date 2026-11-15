// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"context"
	"sync"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// partialGraph holds the Go-only partial observation of one root that a
// single selection re-observes twice: the record-addition admission
// (AFP-V0-3423) and the qualifying proof (AFP-V0-9500). It lives on the
// repository index, so only selections that already read one index of the
// same root, module and tree (AFP-V0-822) share it, never another root or
// process, and it keeps only a successful build: a failed or cancelled build
// is retried by the next caller under its own context (AFP-V0-11134). The graph is immutable after its build; each caller derives
// its own copy through the graph's With methods.
type partialGraph struct {
	mu    sync.Mutex
	root  string
	graph *affected.Graph
	built int // successful builds, read by tests only
}

// buildPartialGraph is affected.BuildPartial with the Go plugin; tests
// replace it to count or fail builds.
var buildPartialGraph = func(ctx context.Context, root string) (*affected.Graph, error) {
	return affected.BuildPartial(ctx, root, golang.New())
}

// observe returns the partial graph of root, built once per index. A nil
// receiver or another root builds a fresh graph and keeps nothing.
func (memo *partialGraph) observe(ctx context.Context, root string) (*affected.Graph, error) {
	if memo == nil {
		return buildPartialGraph(ctx, root)
	}
	memo.mu.Lock()
	defer memo.mu.Unlock()
	if memo.graph != nil {
		if memo.root == root {
			return memo.graph, nil
		}
		return buildPartialGraph(ctx, root)
	}
	graph, err := buildPartialGraph(ctx, root)
	if err != nil {
		return nil, err
	}
	memo.root, memo.graph = root, graph
	memo.built++
	return graph, nil
}

// partialGraphOf returns index's memo, nil for a nil index.
func partialGraphOf(index *repositoryIndex) *partialGraph {
	if index == nil {
		return nil
	}
	return &index.partial
}
