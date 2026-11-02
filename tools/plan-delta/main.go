// Command plan-delta shows how a declaration change moves an affected plan
// (TSP-V0-666, V1-4118). It reads two `corvint affected --format json`
// receipts of the same path list, one planned with the committed declarations
// and one with the edited declarations, and prints the scope, unknown and
// selected-unit delta between them as one JSON object. It only reads the two
// files.
//
// A declaration change plans UNKNOWN on its own, so replay it as a path plan:
//
//	corvint affected --path P... --format json > before.json   # clean tree
//	# edit .corvint/test-read-scopes.json (or another declaration), uncommitted
//	corvint affected --worktree --path P... --format json > after.json
//	go run ./tools/plan-delta before.json after.json
//
// It exits 1 naming the problem when either file is not a receipt or the two
// receipts plan different path lists, and 0 otherwise.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
)

type unknown struct {
	Reason string `json:"reason"`
	Detail string `json:"detail"`
}

type receipt struct {
	PathList []string `json:"path_list"`
	Plan     *struct {
		Scope    string `json:"scope"`
		Selected []struct {
			UnitID  string `json:"unitId"`
			Witness struct {
				Kind string `json:"kind"`
			} `json:"witness"`
		} `json:"selected"`
		Unknown []unknown `json:"unknown"`
	} `json:"plan"`
}

// unit is one selected unit and the witness kind that selected it.
type unit struct {
	UnitID  string `json:"unitId"`
	Witness string `json:"witness"`
}

// delta is the printed comparison. Removed units are the ones the edited
// declarations no longer select; Rewitnessed units stay selected for another
// reason.
type delta struct {
	Profile        string    `json:"profile"`
	PathList       []string  `json:"pathList"`
	ScopeBefore    string    `json:"scopeBefore"`
	ScopeAfter     string    `json:"scopeAfter"`
	SelectedBefore int       `json:"selectedBefore"`
	SelectedAfter  int       `json:"selectedAfter"`
	Removed        []unit    `json:"removed"`
	Added          []unit    `json:"added"`
	Rewitnessed    []unit    `json:"rewitnessed"`
	UnknownRemoved []unknown `json:"unknownRemoved"`
	UnknownAdded   []unknown `json:"unknownAdded"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "plan-delta:", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	if len(args) != 2 {
		return errors.New("usage: go run ./tools/plan-delta BEFORE.json AFTER.json")
	}
	before, err := read(args[0])
	if err != nil {
		return err
	}
	after, err := read(args[1])
	if err != nil {
		return err
	}
	if !slices.Equal(before.PathList, after.PathList) {
		return fmt.Errorf("the receipts plan different path lists: %v and %v", before.PathList, after.PathList)
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(compare(before, after))
}

func read(name string) (receipt, error) {
	body, err := os.ReadFile(name)
	if err != nil {
		return receipt{}, err
	}
	var parsed receipt
	if err := json.Unmarshal(body, &parsed); err != nil {
		return receipt{}, fmt.Errorf("%s: %w", name, err)
	}
	if parsed.Plan == nil || parsed.Plan.Scope == "" {
		return receipt{}, fmt.Errorf("%s: not a corvint affected --format json receipt (no plan.scope)", name)
	}
	return parsed, nil
}

func compare(before, after receipt) delta {
	units := func(r receipt) map[string]string {
		found := map[string]string{}
		for _, selected := range r.Plan.Selected {
			found[selected.UnitID] = selected.Witness.Kind
		}
		return found
	}
	was, is := units(before), units(after)
	result := delta{
		Profile: "corvint-plan-delta/0", PathList: before.PathList,
		ScopeBefore: before.Plan.Scope, ScopeAfter: after.Plan.Scope,
		SelectedBefore: len(was), SelectedAfter: len(is),
		Removed: []unit{}, Added: []unit{}, Rewitnessed: []unit{},
		UnknownRemoved: missing(before.Plan.Unknown, after.Plan.Unknown),
		UnknownAdded:   missing(after.Plan.Unknown, before.Plan.Unknown),
	}
	for id, kind := range was {
		if now, ok := is[id]; !ok {
			result.Removed = append(result.Removed, unit{id, kind})
		} else if now != kind {
			result.Rewitnessed = append(result.Rewitnessed, unit{id, kind + " -> " + now})
		}
	}
	for id, kind := range is {
		if _, ok := was[id]; !ok {
			result.Added = append(result.Added, unit{id, kind})
		}
	}
	for _, list := range [][]unit{result.Removed, result.Added, result.Rewitnessed} {
		slices.SortFunc(list, func(a, b unit) int {
			if a.UnitID < b.UnitID {
				return -1
			}
			if a.UnitID > b.UnitID {
				return 1
			}
			return 0
		})
	}
	return result
}

// missing returns the unknowns of from that are not in other, in order.
func missing(from, other []unknown) []unknown {
	found := []unknown{}
	for _, entry := range from {
		if !slices.Contains(other, entry) && !slices.Contains(found, entry) {
			found = append(found, entry)
		}
	}
	return found
}
