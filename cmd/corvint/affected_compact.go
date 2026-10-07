package main

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// affectedExcludedDigestPrefix tags the digest of the full exclusion list.
const affectedExcludedDigestPrefix = "affected-excluded:sha256:"

// affectedCompactReceipt is the default affected-plan/1 document (AFP-V0-035):
// the affected-plan/0 receipt with plan replaced by its compact projection.
type affectedCompactReceipt struct {
	Snapshot map[string]any      `json:"snapshot,omitempty"`
	Advice   affectedAdvice      `json:"advice"`
	Mutates  bool                `json:"mutates"`
	OK       bool                `json:"ok"`
	Plan     affectedCompactPlan `json:"plan"`
	Profile  string              `json:"profile"`
	Provider affectedProvider    `json:"provider"`
	Range    affectedRange       `json:"range"`
	Revision string              `json:"revision"`
	Tool     string              `json:"tool"`
}

// affectedCompactPlan keeps every plan member except that each selection
// carries a test count instead of its test files and the exclusions are one
// summary (AFP-V0-035).
type affectedCompactPlan struct {
	GraphDigest string                     `json:"graphDigest"`
	Dirty       []string                   `json:"dirty"`
	Scope       string                     `json:"scope"`
	Selected    []affectedCompactSelection `json:"selected"`
	Excluded    affectedExcludedSummary    `json:"excluded"`
	Unknown     []affected.Unknown         `json:"unknown"`
}

type affectedCompactSelection struct {
	UnitID    string           `json:"unitId"`
	TestCount int              `json:"testCount"`
	Witness   affected.Witness `json:"witness"`
}

// affectedExcludedSummary counts the exclusions and binds them by digest to
// the plan.excluded array `--full` emits. Groups state each distinct
// reason, universe and invalidation once, with its count.
type affectedExcludedSummary struct {
	Count  int                     `json:"count"`
	Digest string                  `json:"digest"`
	Groups []affectedExcludedGroup `json:"groups"`
}

type affectedExcludedGroup struct {
	Count        int    `json:"count"`
	Invalidation string `json:"invalidation"`
	Reason       string `json:"reason"`
	Universe     string `json:"universe"`
}

// compactAffectedReceipt projects a full affected-plan/0 receipt onto the
// compact affected-plan/1 wire; any other receipt passes through unchanged.
func compactAffectedReceipt(receipt any) (any, error) {
	full, ok := receipt.(affectedReceipt)
	if !ok {
		return receipt, nil
	}
	excluded, err := summarizeAffectedExclusions(full.Plan.Excluded)
	if err != nil {
		return nil, err
	}
	selected := make([]affectedCompactSelection, 0, len(full.Plan.Selected))
	for _, selection := range full.Plan.Selected {
		selected = append(selected, affectedCompactSelection{UnitID: selection.UnitID, TestCount: len(selection.Tests), Witness: selection.Witness})
	}
	return affectedCompactReceipt{
		Snapshot: full.Snapshot,
		Advice:   full.Advice,
		Mutates:  full.Mutates,
		OK:       full.OK,
		Plan: affectedCompactPlan{
			GraphDigest: full.Plan.GraphDigest,
			Dirty:       full.Plan.Dirty,
			Scope:       full.Plan.Scope,
			Selected:    selected,
			Excluded:    excluded,
			Unknown:     full.Plan.Unknown,
		},
		Profile:  affectedCompactProfile,
		Provider: full.Provider,
		Range:    full.Range,
		Revision: full.Revision,
		Tool:     full.Tool,
	}, nil
}

// summarizeAffectedExclusions digests the canonical JSON of the full
// exclusion array and groups it by reason, universe and invalidation.
func summarizeAffectedExclusions(exclusions []affected.Exclusion) (affectedExcludedSummary, error) {
	if exclusions == nil {
		exclusions = []affected.Exclusion{}
	}
	encoded, err := gokernel.CanonicalJSON(exclusions)
	if err != nil {
		return affectedExcludedSummary{}, err
	}
	sum := sha256.Sum256(encoded)
	counts := map[affectedExcludedGroup]int{}
	for _, exclusion := range exclusions {
		counts[affectedExcludedGroup{Invalidation: exclusion.Invalidation, Reason: exclusion.Reason, Universe: exclusion.Universe}]++
	}
	groups := make([]affectedExcludedGroup, 0, len(counts))
	for group, count := range counts {
		group.Count = count
		groups = append(groups, group)
	}
	sort.Slice(groups, func(i, j int) bool {
		a, b := groups[i], groups[j]
		if a.Reason != b.Reason {
			return a.Reason < b.Reason
		}
		if a.Universe != b.Universe {
			return a.Universe < b.Universe
		}
		return a.Invalidation < b.Invalidation
	})
	return affectedExcludedSummary{Count: len(exclusions), Digest: affectedExcludedDigestPrefix + hex.EncodeToString(sum[:]), Groups: groups}, nil
}
