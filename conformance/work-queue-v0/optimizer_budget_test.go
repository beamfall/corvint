//go:build unix

package workqueuev0_test

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/workqueue"
)

//go:embed testdata/optimizer_budget_cycles.json
var optimizerBudgetCycles []byte

func TestProposalLiteralMillionNodeBudgetWitness(t *testing.T) {
	t.Run("WQO-V0-044", func(t *testing.T) {
		var graph struct {
			Vertices int      `json:"vertices"`
			Edges    [][2]int `json:"edges"`
			Selected []int    `json:"selected"`
		}
		if err := json.Unmarshal(optimizerBudgetCycles, &graph); err != nil {
			t.Fatal(err)
		}
		if graph.Vertices != 64 || len(graph.Edges) != 63 || len(graph.Selected) != 28 {
			t.Fatal("frozen nine C7 cycles plus isolated vertex changed")
		}
		tickets := make([]workqueue.TicketSummary, graph.Vertices)
		for i := range tickets {
			tickets[i] = ticket(fmt.Sprintf("budget-%02d", i), workqueue.Rank(i))
		}
		groups := make([]workqueue.CollisionGroup, len(graph.Edges))
		for i, edge := range graph.Edges {
			base, offset := i/7*7, i%7
			if edge != [2]int{base + offset, base + (offset+1)%7} {
				t.Fatalf("edge %d is not the registered cycle edge: %v", i, edge)
			}
			groups[i] = workqueue.CollisionGroup{ID: budgetGroupID(i), MemberTicketIDs: []string{tickets[edge[0]].TicketID, tickets[edge[1]].TicketID}, Source: "ADAPTER"}
		}
		value := snapshot(tickets...)
		assertWitnessValidated(t, value)
		capacity := envelope(value)
		before, capacityBefore := value.Canonical(), capacity.Canonical()
		// Degree zero puts vertex 63 first; all cycle vertices have degree two.
		// Ascending rank then selects offsets 0,2,4 in each cycle. This literal
		// oracle does not reproduce the bounded maximum-search traversal.
		selected := map[int]bool{}
		for _, vertex := range graph.Selected {
			selected[vertex] = true
		}
		reasons := make([]string, graph.Vertices)
		for i := range reasons {
			reasons[i] = "SELECTED_COLLISION"
			if selected[i] {
				reasons[i] = "ELIGIBLE_AT_CHECKPOINT"
			}
		}
		var first []byte
		for attempt := 0; attempt < 2; attempt++ {
			proposal := proposeWitness(t, value, capacity, workqueue.CollisionClosure{Complete: true, Groups: groups})
			// Exactly 64 valid READY candidates excludes the >64 shortcut.
			// With the unchanged production 1,000,000-node bound, GREEDY can
			// therefore only be reached through actual search exhaustion.
			if proposal.WaveOptimality != "GREEDY" || proposal.State != "ELIGIBLE_AT" || len(proposal.Unknowns) != 0 || proposal.MutationAuthority {
				t.Fatalf("unexpected budget-fallback result: %#v", proposal)
			}
			assertWitnessReasons(t, proposal, tickets, reasons)
			for vertex, entry := range proposal.Entries {
				want := []string{}
				// Independently enumerated selected-neighbour edge witnesses.
				edges := map[int][]int{1: {0, 1}, 3: {2, 3}, 5: {4}, 6: {6}}
				if !selected[vertex] {
					for _, edge := range edges[vertex%7] {
						want = append(want, budgetGroupID(vertex/7*7+edge))
					}
				}
				if !reflect.DeepEqual(entry.CollisionGroupIDs, want) {
					t.Fatalf("vertex %d explanations = %v; want %v", vertex, entry.CollisionGroupIDs, want)
				}
			}
			if attempt == 0 {
				first = proposal.Canonical()
			} else if !bytes.Equal(first, proposal.Canonical()) {
				t.Fatal("reversed closure order changed the deterministic proposal")
			}
			for left, right := 0, len(groups)-1; left < right; left, right = left+1, right-1 {
				groups[left], groups[right] = groups[right], groups[left]
			}
		}
		if !bytes.Equal(before, value.Canonical()) || !bytes.Equal(capacityBefore, capacity.Canonical()) {
			t.Fatal("budget fallback mutated input facts or capacity")
		}
	})
}

func budgetGroupID(edge int) string {
	return fmt.Sprintf("collision:corvint:worklist:budget-%02d", edge)
}
