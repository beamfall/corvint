package testplan

import (
	"container/heap"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// MaxEdges bounds the precedence edges one input may build (TCN-V0-007).
const MaxEdges = 1 << 20

// planned is one compatible set in its independent order.
type planned struct {
	order []*Variation
}

// orderer counts every precedence edge the planner builds, and every edge it examines again while
// removing cycles, across one input.
type orderer struct {
	edges int
}

func boundExceeded(message string) error {
	return &gokernel.Error{Code: "test-plan-bound-exceeded", Message: message}
}

// plan splits one context's non-destructive variations into compatible sets with an independent
// order (TCN-V0-007). Variations are visited in ID order; each joins the first set none of whose
// members requires a different value for a key it requires. A cycle's greatest ID is removed and
// every removed variation is planned again as a further batch by this same rule.
func (o *orderer) plan(batch []*Variation) ([]planned, error) {
	out := []planned{}
	for len(batch) > 0 {
		sets := compatibleSets(batch)
		batch = nil
		for _, set := range sets {
			order, removed, err := o.order(set)
			if err != nil {
				return nil, err
			}
			out = append(out, planned{order: order})
			batch = append(batch, removed...)
		}
		sort.Slice(batch, func(i, j int) bool { return batch[i].ID < batch[j].ID })
	}
	return out, nil
}

func compatibleSets(batch []*Variation) [][]*Variation {
	type set struct {
		members  []*Variation
		requires map[string]string
	}
	sets := []*set{}
	for _, v := range batch {
		var target *set
		for _, s := range sets {
			if compatible(s.requires, v) {
				target = s
				break
			}
		}
		if target == nil {
			target = &set{requires: map[string]string{}}
			sets = append(sets, target)
		}
		target.members = append(target.members, v)
		for _, fact := range v.Requires {
			key, value, _ := strings.Cut(fact, "=")
			target.requires[key] = value
		}
	}
	out := make([][]*Variation, len(sets))
	for i, s := range sets {
		out[i] = s.members
	}
	return out
}

func compatible(requires map[string]string, v *Variation) bool {
	for _, fact := range v.Requires {
		key, value, _ := strings.Cut(fact, "=")
		if prior, ok := requires[key]; ok && prior != value {
			return false
		}
	}
	return true
}

// order builds the set's precedence graph (B before A whenever A changes a key B requires),
// removes the greatest ID of every strongly connected component until none is left, and returns
// the smallest-ID-first topological order of the rest with the removed variations.
func (o *orderer) order(set []*Variation) ([]*Variation, []*Variation, error) {
	n := len(set)
	requirers := map[string][]int{}
	for i, v := range set {
		for _, fact := range v.Requires {
			key, _, _ := strings.Cut(fact, "=")
			requirers[key] = append(requirers[key], i)
		}
	}
	// succ[b] lists every a that b must precede; edges are unique.
	succ := make([][]int, n)
	stamp := make([]int, n)
	for i := range stamp {
		stamp[i] = -1
	}
	for a, v := range set {
		for _, fact := range v.Changes {
			key, _, _ := strings.Cut(fact, "=")
			for _, b := range requirers[key] {
				if b == a || stamp[b] == a {
					continue
				}
				stamp[b] = a
				succ[b] = append(succ[b], a)
				if o.edges++; o.edges > MaxEdges {
					return nil, nil, boundExceeded("the input needs more than 1048576 precedence edges")
				}
			}
		}
	}
	alive := make([]bool, n)
	for i := range alive {
		alive[i] = true
	}
	removed := []*Variation{}
	// Only the members of a component that had a cycle can still be in one after a removal.
	candidates := make([]int, n)
	for i := range candidates {
		candidates[i] = i
	}
	for len(candidates) > 0 {
		components, err := o.components(succ, alive, candidates)
		if err != nil {
			return nil, nil, err
		}
		candidates = nil
		for _, c := range components {
			if len(c) < 2 {
				continue
			}
			greatest := c[0]
			for _, i := range c {
				if set[i].ID > set[greatest].ID {
					greatest = i
				}
			}
			alive[greatest] = false
			removed = append(removed, set[greatest])
			for _, i := range c {
				if i != greatest {
					candidates = append(candidates, i)
				}
			}
		}
	}
	return topological(set, succ, alive), removed, nil
}

// components returns the strongly connected components among candidates (Tarjan, iterative),
// counting every edge it examines toward the input's precedence-edge bound.
func (o *orderer) components(succ [][]int, alive []bool, candidates []int) ([][]int, error) {
	in := map[int]bool{}
	for _, c := range candidates {
		in[c] = true
	}
	index := map[int]int{}
	low := map[int]int{}
	onStack := map[int]bool{}
	stack := []int{}
	out := [][]int{}
	next := 0
	type frame struct{ node, edge int }
	sort.Ints(candidates)
	for _, root := range candidates {
		if _, seen := index[root]; seen {
			continue
		}
		calls := []frame{{root, 0}}
		index[root], low[root] = next, next
		next++
		stack = append(stack, root)
		onStack[root] = true
		for len(calls) > 0 {
			top := &calls[len(calls)-1]
			v := top.node
			if top.edge < len(succ[v]) {
				w := succ[v][top.edge]
				top.edge++
				if !alive[w] || !in[w] {
					continue
				}
				if o.edges++; o.edges > MaxEdges {
					return nil, boundExceeded("the input needs more than 1048576 precedence edges")
				}
				if _, seen := index[w]; !seen {
					index[w], low[w] = next, next
					next++
					stack = append(stack, w)
					onStack[w] = true
					calls = append(calls, frame{w, 0})
				} else if onStack[w] && index[w] < low[v] {
					low[v] = index[w]
				}
				continue
			}
			calls = calls[:len(calls)-1]
			if len(calls) > 0 {
				parent := calls[len(calls)-1].node
				if low[v] < low[parent] {
					low[parent] = low[v]
				}
			}
			if low[v] == index[v] {
				component := []int{}
				for {
					w := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					onStack[w] = false
					component = append(component, w)
					if w == v {
						break
					}
				}
				out = append(out, component)
			}
		}
	}
	return out, nil
}

// topological is Kahn's order over the live nodes, always taking the smallest available ID.
func topological(set []*Variation, succ [][]int, alive []bool) []*Variation {
	indegree := make([]int, len(set))
	for b := range set {
		if !alive[b] {
			continue
		}
		for _, a := range succ[b] {
			if alive[a] {
				indegree[a]++
			}
		}
	}
	ready := &idHeap{set: set}
	for i := range set {
		if alive[i] && indegree[i] == 0 {
			heap.Push(ready, i)
		}
	}
	order := []*Variation{}
	for ready.Len() > 0 {
		b := heap.Pop(ready).(int)
		order = append(order, set[b])
		for _, a := range succ[b] {
			if !alive[a] {
				continue
			}
			if indegree[a]--; indegree[a] == 0 {
				heap.Push(ready, a)
			}
		}
	}
	return order
}

type idHeap struct {
	set   []*Variation
	items []int
}

func (h *idHeap) Len() int           { return len(h.items) }
func (h *idHeap) Less(i, j int) bool { return h.set[h.items[i]].ID < h.set[h.items[j]].ID }
func (h *idHeap) Swap(i, j int)      { h.items[i], h.items[j] = h.items[j], h.items[i] }
func (h *idHeap) Push(x any)         { h.items = append(h.items, x.(int)) }
func (h *idHeap) Pop() any {
	last := h.items[len(h.items)-1]
	h.items = h.items[:len(h.items)-1]
	return last
}
