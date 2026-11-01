package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// affectedByPathProfile is the --by-path document (AFP-V0-990).
const affectedByPathProfile = "affected-by-path/1"

// affectedByPathUnit is one selected unit of a --by-path document.
type affectedByPathUnit struct {
	UnitID    string `json:"unitId"`
	TestCount int    `json:"testCount"`
	Witness   string `json:"witness"`
}

// affectedByPathEntry is one named path: its own selection less the shared
// UNBOUNDED_READER set, how many of that set it also selects, and its
// unknowns.
type affectedByPathEntry struct {
	Path      string               `json:"path"`
	Scope     string               `json:"scope"`
	Shared    int                  `json:"sharedSelected"`
	Additions []affectedByPathUnit `json:"additions"`
	Unknown   []affected.Unknown   `json:"unknown"`
}

// affectedByPathReceipt separates the units every path selects through an
// unbounded reader from each path's own additions, over one graph build
// (AFP-V0-990).
type affectedByPathReceipt struct {
	Profile  string                `json:"profile"`
	Tool     string                `json:"tool"`
	Revision string                `json:"revision"`
	Scope    string                `json:"scope"`
	Shared   []affectedByPathUnit  `json:"sharedUnboundedReaders"`
	Paths    []affectedByPathEntry `json:"paths"`
	// Degradations is the plan's AFP-V0-846 self-binary freshness, as the
	// affected-plan/1 receipt carries it.
	Degradations []string `json:"degradations,omitempty"`
	Mutates      bool     `json:"mutates"`
	OK           bool     `json:"ok"`
}

// runAffectedByPath plans the union of the named paths once, then selects each
// path alone over the same graph: the union's UNBOUNDED_READER units are
// listed once, and each path lists only the units it adds (AFP-V0-990).
func runAffectedByPath(ctx context.Context, invocation affectedInvocation, stdout, stderr io.Writer) int {
	receipt, graph, err := compileAffectedGraph(ctx, invocation)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	// AFP-V0-846: a stale or unverified binary is named here as on the plan.
	receipt = degradeStaleAffected(ctx, invocation.Root, receipt, stdout, stderr).(affectedReceipt)
	result := affectedByPath(receipt, graph, invocation.PathList)
	result.Degradations = receipt.Degradations
	if invocation.Format == "text" {
		if _, err := io.WriteString(stdout, renderAffectedByPathText(result)); err != nil {
			emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write affected plan"})
			return 2
		}
		return 0
	}
	if err := gokernel.WriteCanonicalJSON(stdout, result); err != nil {
		if gokernel.IsCanonicalOutputFailure(err) {
			err = &gokernel.Error{Code: "output-failed", Message: "cannot write affected receipt"}
		}
		emitError(stderr, err)
		return 2
	}
	return 0
}

// affectedByPath builds the --by-path document: each path is selected alone
// over the graph the union receipt was selected over. The shared set is every
// unit each path selects and at least one selects as UNBOUNDED_READER, which
// any dirty path reaches; each path then lists only the rest.
func affectedByPath(receipt affectedReceipt, graph *affected.Graph, paths []string) affectedByPathReceipt {
	result := affectedByPathReceipt{Profile: affectedByPathProfile, Tool: "affected", Revision: receipt.Revision, Scope: receipt.Plan.Scope, Shared: []affectedByPathUnit{}, Paths: []affectedByPathEntry{}, OK: true}
	plans := make([]affected.Plan, len(paths))
	selectedBy, unbounded, tests := map[string]int{}, map[string]bool{}, map[string]int{}
	for index, path := range paths {
		plans[index] = affected.SelectLinked(graph, []string{path})
		for _, selection := range plans[index].Selected {
			selectedBy[selection.UnitID]++
			tests[selection.UnitID] = len(selection.Tests)
			if selection.Witness.Kind == affected.WitnessUnboundedReader {
				unbounded[selection.UnitID] = true
			}
		}
	}
	shared := map[string]bool{}
	for _, selection := range receipt.Plan.Selected {
		if unbounded[selection.UnitID] && selectedBy[selection.UnitID] == len(paths) {
			shared[selection.UnitID] = true
			result.Shared = append(result.Shared, affectedByPathUnit{UnitID: selection.UnitID, TestCount: tests[selection.UnitID], Witness: affected.WitnessUnboundedReader})
		}
	}
	for index, path := range paths {
		plan := plans[index]
		entry := affectedByPathEntry{Path: path, Scope: plan.Scope, Additions: []affectedByPathUnit{}, Unknown: plan.Unknown}
		for _, selection := range plan.Selected {
			if shared[selection.UnitID] {
				entry.Shared++
				continue
			}
			entry.Additions = append(entry.Additions, affectedByPathUnit{UnitID: selection.UnitID, TestCount: len(selection.Tests), Witness: selection.Witness.Kind})
		}
		result.Paths = append(result.Paths, entry)
	}
	return result
}

// renderAffectedByPathText is the --by-path document as lines.
func renderAffectedByPathText(result affectedByPathReceipt) string {
	var out strings.Builder
	fmt.Fprintf(&out, "affected by-path scope=%s paths=%d shared=%d revision=%s\n", result.Scope, len(result.Paths), len(result.Shared), result.Revision)
	for _, degradation := range result.Degradations {
		fmt.Fprintf(&out, "degraded %s\n", degradation)
	}
	for _, unit := range result.Shared {
		fmt.Fprintf(&out, "shared %s tests=%d %s\n", textWord(unit.UnitID), unit.TestCount, unit.Witness)
	}
	for _, entry := range result.Paths {
		fmt.Fprintf(&out, "path %s scope=%s shared=%d/%d additions=%d\n", textWord(entry.Path), entry.Scope, entry.Shared, len(result.Shared), len(entry.Additions))
		for _, unit := range entry.Additions {
			fmt.Fprintf(&out, "  added %s tests=%d %s\n", textWord(unit.UnitID), unit.TestCount, unit.Witness)
		}
		for _, unknown := range entry.Unknown {
			fmt.Fprintf(&out, "  unknown %s %s\n", unknown.Reason, textWord(unknown.Detail))
		}
	}
	return out.String()
}
