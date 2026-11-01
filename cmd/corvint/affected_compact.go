package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// affectedExcludedDigestPrefix tags the digest of the full exclusion list.
const affectedExcludedDigestPrefix = "affected-excluded:sha256:"

// affectedAdvisoryArguments names the member whose entries complete the
// compact advisory Go command (AFP-V0-038).
const affectedAdvisoryArguments = "provider.go.packages"

// affectedCompactReceipt is the default affected-plan/1 document (AFP-V0-035):
// the affected-plan/0 receipt with plan replaced by its compact projection.
type affectedCompactReceipt struct {
	Snapshot map[string]any        `json:"snapshot,omitempty"`
	PathList []string              `json:"path_list,omitempty"`
	Advice   affectedCompactAdvice `json:"advice"`
	Mutates  bool                  `json:"mutates"`
	OK       bool                  `json:"ok"`
	Plan     affectedCompactPlan   `json:"plan"`
	Profile  string                `json:"profile"`
	Provider affectedProvider      `json:"provider"`
	Range    affectedRange         `json:"range"`
	Revision string                `json:"revision"`
	Tool     string                `json:"tool"`
	Warning  string                `json:"warning,omitempty"`
	// Degradations is copied from the full receipt (AFP-V0-846).
	Degradations []string `json:"degradations,omitempty"`
	// Tiers ranks the selection for a bounded local run (AFP-V0-162); it is
	// omitted when nothing is selected.
	Tiers *affectedTiers `json:"tiers,omitempty"`
	// Summary restates the plan's scope and counts in one object (AFP-V0-510).
	Summary affectedSummary `json:"summary"`
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
	NonSource   []affected.NonSource       `json:"nonSource,omitempty"`
	// FrontierSites is the plan's repository frontier site count (AFP-V0-381).
	FrontierSites map[string]int `json:"frontierSites,omitempty"`
	// FrontierExcluded is the plan's frontier left out of unknown, with its
	// reason (AFP-V0-560).
	FrontierExcluded []affected.FrontierExclusion `json:"frontierExcluded,omitempty"`
	// FrontierSources is what raised each frontier entry of unknown (TSP-V0-724).
	FrontierSources map[string]string `json:"frontierSources,omitempty"`
	// GoAdditive is the plan's additive-change records (ADN-V0-001).
	GoAdditive []affected.GoAdditive `json:"goAdditive,omitempty"`
	// GoAdditiveExcluded is the plan's units left out by a narrowed
	// additive-change record (ADN-V0-005).
	GoAdditiveExcluded []affected.GoAdditiveExclusion `json:"goAdditiveExcluded,omitempty"`
	// ViaLinks maps every unit after the first on a selection's witness
	// chain to the unit before it, so each chain is stated once and shared
	// instead of copied into every selection (AFP-V0-640). It is omitted when
	// no chain has two units.
	ViaLinks map[string]string `json:"viaLinks,omitempty"`
}

// affectedCompactAdvice is the affected-plan/0 advice except that the
// advisory Go command names provider.go.packages instead of repeating the
// package list (AFP-V0-038).
type affectedCompactAdvice struct {
	Checks        []affectedCompactCheck  `json:"checks"`
	Note          string                  `json:"note"`
	Status        string                  `json:"status"`
	TestSelection map[string]any          `json:"test_selection,omitempty"`
	AppNavigation []affectedAppNavigation `json:"app_navigation,omitempty"`
	Unknown       []string                `json:"unknown"`
}

// affectedCompactCheck is an advice check. When Arguments is set, the
// runnable command is Command followed by every entry of the named member,
// each POSIX single-quoted and separated by one space: exactly the
// affected-plan/0 command.
type affectedCompactCheck struct {
	Arguments string `json:"arguments,omitempty"`
	Command   string `json:"command"`
	Kind      string `json:"kind"`
	Reason    string `json:"reason"`
	Source    string `json:"source"`
}

type affectedCompactSelection struct {
	UnitID    string                 `json:"unitId"`
	TestCount int                    `json:"testCount"`
	Witness   affectedCompactWitness `json:"witness"`
}

// affectedCompactWitness is a plan witness whose chain is a reference, not a
// copy (AFP-V0-640): ViaLength is the chain's length, and the chain is the
// selected unit followed by ViaLength-1 steps through plan.viaLinks, read in
// reverse. Via carries the whole chain only for a chain plan.viaLinks cannot
// state, one that disagrees with a link an earlier selection recorded.
type affectedCompactWitness struct {
	Kind      string                   `json:"kind"`
	DirtyPath string                   `json:"dirtyPath"`
	ViaLength int                      `json:"viaLength"`
	Via       []string                 `json:"via,omitempty"`
	Cause     *affected.UnboundedCause `json:"cause,omitempty"`
	Glob      string                   `json:"glob,omitempty"`
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
	return compactAffected(full)
}

// compactAffected is the typed affected-plan/1 projection, also embedded by
// review (RGV-V0-015).
func compactAffected(full affectedReceipt) (affectedCompactReceipt, error) {
	excluded, err := summarizeAffectedExclusions(full.Plan.Excluded)
	if err != nil {
		return affectedCompactReceipt{}, err
	}
	selected, links := compactAffectedSelections(full.Plan.Selected)
	return affectedCompactReceipt{
		Snapshot: full.Snapshot,
		PathList: full.PathList,
		Advice:   compactAffectedAdvice(full.Advice, full.Provider.Go.Packages, full.Plan.Unknown),
		Mutates:  full.Mutates,
		OK:       full.OK,
		Plan: affectedCompactPlan{
			GraphDigest:        full.Plan.GraphDigest,
			Dirty:              full.Plan.Dirty,
			Scope:              full.Plan.Scope,
			Selected:           selected,
			Excluded:           excluded,
			Unknown:            full.Plan.Unknown,
			NonSource:          full.Plan.NonSource,
			FrontierSites:      full.Plan.FrontierSites,
			FrontierExcluded:   full.Plan.FrontierExcluded,
			FrontierSources:    full.Plan.FrontierSources,
			GoAdditive:         full.Plan.GoAdditive,
			GoAdditiveExcluded: full.Plan.GoAdditiveExcluded,
			ViaLinks:           links,
		},
		Profile:      affectedCompactProfile,
		Provider:     full.Provider,
		Range:        full.Range,
		Revision:     full.Revision,
		Tool:         full.Tool,
		Warning:      full.Warning,
		Degradations: full.Degradations,
		Tiers:        tierAffectedSelections(full.Plan.Selected),
		Summary:      summarizeAffectedPlan(full.Plan, excluded.Count),
	}, nil
}

// compactAffectedSelections projects each selection, sharing witness chains
// through one link map so the document grows with the units on chains, not
// with the sum of chain lengths (AFP-V0-640, V1-2647). A unit's link is
// recorded with the chain prefix that ends at it; a later chain reaching that
// unit must carry the same prefix to reuse it, so a walk through the links
// always rebuilds exactly the chain the selection carries. A chain that does
// not end at its own unit, or disagrees with a recorded prefix, keeps its
// full via and records nothing.
func compactAffectedSelections(selections []affected.Selection) ([]affectedCompactSelection, map[string]string) {
	selected := make([]affectedCompactSelection, 0, len(selections))
	links := map[string]string{}
	prefixes := map[string]affected.ViaStep{}
	for _, selection := range selections {
		witness := selection.Witness
		compact := affectedCompactWitness{Kind: witness.Kind, DirtyPath: witness.DirtyPath, ViaLength: witness.ViaLength(), Cause: witness.Cause, Glob: witness.Glob}
		if !linkAffectedChain(selection.UnitID, witness, links, prefixes) {
			compact.Via = witness.Chain()
		}
		selected = append(selected, affectedCompactSelection{UnitID: selection.UnitID, TestCount: len(selection.Tests), Witness: compact})
	}
	if len(links) == 0 {
		links = nil
	}
	return selected, links
}

// linkAffectedChain records the witness chain in links, walking back from its
// last unit until it meets a unit whose recorded prefix it shares, and reports
// whether the links now rebuild the chain from unit. On a disagreement it
// removes what it recorded for this chain. Prefixes are compared as chain
// steps, so a chain the selector shares by reference is never copied
// (AFP-V0-641).
func linkAffectedChain(unit string, witness affected.Witness, links map[string]string, prefixes map[string]affected.ViaStep) bool {
	step, ok := witness.LastStep()
	if !ok || step.Unit() != unit {
		return false
	}
	added := make([]string, 0, 1)
	for {
		node := step.Unit()
		if recorded, seen := prefixes[node]; seen {
			if recorded.SamePrefix(step) {
				return true
			}
			for _, undo := range added {
				delete(prefixes, undo)
				delete(links, undo)
			}
			return false
		}
		prefixes[node] = step
		added = append(added, node)
		previous, more := step.Previous()
		if !more {
			return true
		}
		links[node] = previous.Unit()
		step = previous
	}
}

// expandAffectedVia rebuilds a compact witness chain: the selected unit and
// length-1 steps through links, in chain order (AFP-V0-640). It reports false
// when a step has no link.
func expandAffectedVia(links map[string]string, unit string, length int) ([]string, bool) {
	if length < 1 {
		return nil, false
	}
	chain := make([]string, length)
	chain[length-1] = unit
	for index := length - 2; index >= 0; index-- {
		previous, linked := links[chain[index+1]]
		if !linked {
			return nil, false
		}
		chain[index] = previous
	}
	return chain, true
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

// affectedPlanUnknownReference is the one advice.unknown line of the
// affected-plan/1 default that stands for the plan.unknown entries the
// `--full` advice repeats (AFP-V0-155).
const affectedPlanUnknownReference = "PLAN_UNKNOWN: %d plan.unknown entries; see plan.unknown"

// compactAffectedAdvice replaces the package list of the advisory Go command
// with a reference to provider.go.packages. A command that is not exactly the
// prefix followed by the quoted packages is kept whole, so nothing is lost.
// An advice.unknown line that is exactly a plan.unknown entry's "reason:
// detail" is replaced by one count line naming plan.unknown, which the
// default keeps whole; every advice-specific line is kept (AFP-V0-155).
func compactAffectedAdvice(advice affectedAdvice, packages []string, planUnknown []affected.Unknown) affectedCompactAdvice {
	checks := make([]affectedCompactCheck, 0, len(advice.Checks))
	full := adviceAdvisoryGoTest + " " + shellQuoteJoin(packages)
	for _, check := range advice.Checks {
		compact := affectedCompactCheck{Command: check.Command, Kind: check.Kind, Reason: check.Reason, Source: check.Source}
		if check.Kind == adviceKindAdvisory && check.Source == adviceSourcePlan && len(packages) > 0 && check.Command == full {
			compact.Command, compact.Arguments = adviceAdvisoryGoTest, affectedAdvisoryArguments
		}
		checks = append(checks, compact)
	}
	return affectedCompactAdvice{Checks: checks, Note: advice.Note, Status: advice.Status, TestSelection: advice.TestSelection, AppNavigation: advice.AppNavigation, Unknown: compactAdviceUnknown(advice.Unknown, planUnknown)}
}

func compactAdviceUnknown(unknown []string, planUnknown []affected.Unknown) []string {
	fromPlan := make(map[string]bool, len(planUnknown))
	for _, entry := range planUnknown {
		fromPlan[entry.Reason+": "+entry.Detail] = true
	}
	kept, repeated := make([]string, 0, len(unknown)), 0
	for _, line := range unknown {
		if fromPlan[line] {
			repeated++
			continue
		}
		kept = append(kept, line)
	}
	if repeated == 0 {
		return unknown
	}
	return sortedUniqueStrings(append(kept, fmt.Sprintf(affectedPlanUnknownReference, len(planUnknown))))
}
