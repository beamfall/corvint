// SPDX-License-Identifier: Apache-2.0

package noncodemap

import (
	"slices"
	"strings"
)

// Decision reasons, printed with the path or runner they concern.
const (
	// ReasonUnowned is a non-code path no runner attributes and no proven
	// entry covers (NCM-V1-001, R32, orchestrator ruling 2026-11-11).
	ReasonUnowned = "unowned-path"
	// ReasonRunnerRemoved is deleted runner configuration or an unavailable
	// runner (NCM-V1-001, R32).
	ReasonRunnerRemoved = "runner-removed"
	// ReasonNeverNarrowed is a path on the never-narrow list (NCM-V1-005).
	ReasonNeverNarrowed = "never-narrowed-path"
	// ReasonCorvintConfig is Corvint configuration under `.corvint/`
	// (NCM-V1-006).
	ReasonCorvintConfig = "corvint-config"
	// ReasonFlowsUnreadable is a flow file or link record whose changed flows
	// cannot be computed (NCM-V1-006).
	ReasonFlowsUnreadable = "flows-unreadable"
	// ReasonFlowLinked is a flow file or link record whose linked tests the
	// flow join selects; it adds no full run (NCM-V1-006).
	ReasonFlowLinked = "flow-linked"
	// ReasonPrivateLedger is one of the two private ledgers; it adds no test
	// (NCM-V1-006).
	ReasonPrivateLedger = "private-ledger"
	// ReasonDocumentation is a declared documentation path; it adds no test
	// (NCM-V1-001).
	ReasonDocumentation = "DECLARED_DOCUMENTATION"
	// ReasonProvenEntry is a non-code path a proven entry narrows to its
	// tests (NCM-V1-004).
	ReasonProvenEntry = "proven-entry"
)

// corvintDirectory holds Corvint's configuration (NCM-V1-006).
const corvintDirectory = ".corvint/"

// privateLedgers add no test (NCM-V1-006, AGENTS.md invariant 4).
var privateLedgers = map[string]bool{".corvint/self-observations.jsonl": true, ".corvint/unplanned-reads.jsonl": true}

// FlowFiles are the flow file and its deprecated form (NCM-V1-006).
var FlowFiles = map[string]bool{".corvint/flows.json": true, ".corvint/e2e-flows.json": true}

// Input is everything one decision is a function of. Paths are clean
// repository-relative slash paths; the plan supplies the sets.
type Input struct {
	// Dirty is the plan's changed paths.
	Dirty []string
	// Unowned is each path the plan reports as UNOWNED_DIRTY_PATH.
	Unowned []string
	// Documentation is each path of the plan's DECLARED_DOCUMENTATION group.
	Documentation []string
	// Deleted is each changed path absent from the tested tree.
	Deleted []string
	// Attributed reports a path a Go rule attributes: an enclosing package
	// directory (its testdata and go:embed inputs included), a package whose
	// files name it, or a declared read scope that bears on it. Such a path
	// is owned and keeps the selector's attribution; nil attributes nothing.
	Attributed func(string) bool
	// FlowsDirectory is the plan's declared flow intent directory, empty
	// without a flow join; FlowsJoined says the plan carries the join.
	FlowsDirectory string
	FlowsJoined    bool
	// FlowsInvalid, when not empty, says why the plan could not read the
	// flow declaration.
	FlowsInvalid string
	// Unavailable names each runner the plan reports unavailable, as
	// "<runner>: <why>".
	Unavailable []string
	// Map is the read map; Env the facts its entries' status needs.
	Map Map
	Env Env
}

// Line is one decided path, or one unavailable runner.
type Line struct {
	Reason string
	Path   string
	// Full is set when the line needs every test of every runner.
	Full bool
	// Tests is the proven entry's tests, on a ReasonProvenEntry line only.
	Tests []string
}

// EntryReport is one map entry that covers a changed path.
type EntryReport struct {
	Key    string
	Status string
	Detail string
}

// Decision is the per-path decision for one plan.
type Decision struct {
	Lines   []Line
	Entries []EntryReport
}

// Decide applies the delivered rules of docs/specs/non-code-map-v1.md to
// each changed path, in byte order, first match wins: the private ledgers;
// the flow file and its link records; declared documentation and every other
// path under `.corvint/`; the never-narrow list (a deleted runner
// configuration reads as a removed runner); declared documentation; and each
// UNOWNED_DIRTY_PATH, which a proven entry narrows, a Go rule's attribution
// keeps owned, and otherwise selects everything. Any other path keeps the
// selector's attribution and gets no line. Each unavailable runner adds a
// full line first.
func Decide(in Input) Decision {
	var decision Decision
	for _, runner := range sortedUnique(in.Unavailable) {
		decision.Lines = append(decision.Lines, Line{Reason: ReasonRunnerRemoved, Path: runner, Full: true})
	}
	unowned := set(in.Unowned)
	documentation := set(in.Documentation)
	deleted := set(in.Deleted)
	entries := in.Map.sortedEntries()
	reported := map[string]bool{}
	for _, p := range sortedUnique(in.Dirty) {
		line, ok := in.decidePath(p, unowned, documentation, deleted, entries, &decision, reported)
		if ok {
			decision.Lines = append(decision.Lines, line)
		}
	}
	slices.SortFunc(decision.Entries, func(a, b EntryReport) int { return strings.Compare(a.Key, b.Key) })
	return decision
}

func (in Input) decidePath(p string, unowned, documentation, deleted map[string]bool, entries []Entry, decision *Decision, reported map[string]bool) (Line, bool) {
	if privateLedgers[p] {
		return Line{Reason: ReasonPrivateLedger, Path: p}, true
	}
	if in.flowPath(p) {
		if in.FlowsInvalid != "" || (FlowFiles[p] && !in.FlowsJoined) {
			return Line{Reason: ReasonFlowsUnreadable, Path: p, Full: true}, true
		}
		return Line{Reason: ReasonFlowLinked, Path: p}, true
	}
	if strings.HasPrefix(p, corvintDirectory) {
		if documentation[p] {
			return Line{Reason: ReasonDocumentation, Path: p}, true
		}
		return Line{Reason: ReasonCorvintConfig, Path: p, Full: true}, true
	}
	if category, never := NeverNarrowed(p); never {
		if category == CategoryRunnerConfig && deleted[p] {
			return Line{Reason: ReasonRunnerRemoved, Path: p, Full: true}, true
		}
		return Line{Reason: ReasonNeverNarrowed, Path: p, Full: true}, true
	}
	if documentation[p] {
		return Line{Reason: ReasonDocumentation, Path: p}, true
	}
	if !unowned[p] {
		return Line{}, false
	}
	var proven *Entry
	for index, entry := range entries {
		if !entry.Covers(p) {
			continue
		}
		status, detail := Evaluate(entry, in.Env)
		if !reported[entry.Key()] {
			reported[entry.Key()] = true
			decision.Entries = append(decision.Entries, EntryReport{Key: entry.Key(), Status: status, Detail: detail})
		}
		if status == StatusProven && proven == nil {
			proven = &entries[index]
		}
	}
	switch {
	case proven != nil:
		return Line{Reason: ReasonProvenEntry, Path: p, Tests: slices.Clone(proven.Tests)}, true
	case in.Attributed != nil && in.Attributed(p):
		// Owned: the Go attribution selects its readers (orchestrator
		// ruling 2026-11-11 on NCM-V1-001).
		return Line{}, false
	}
	return Line{Reason: ReasonUnowned, Path: p, Full: true}, true
}

// flowPath reports the flow file, its deprecated form, or a link record in
// the declared intent directory when that directory lies under `.corvint/`.
func (in Input) flowPath(p string) bool {
	if FlowFiles[p] {
		return true
	}
	directory := strings.TrimSuffix(in.FlowsDirectory, "/")
	return strings.HasPrefix(directory+"/", corvintDirectory) && directory+"/" != corvintDirectory && strings.HasPrefix(p, directory+"/")
}

// fullReasons are the reasons of a line that needs every test, and the map
// refusal, as the selector's `FALLBACK <reason> ...` verdict prints them.
var fullReasons = []string{ReasonUnowned, ReasonRunnerRemoved, ReasonNeverNarrowed, ReasonCorvintConfig, ReasonFlowsUnreadable, CodeInvalid + ":"}

// NamesFullRun reports a selector verdict `FALLBACK <reason> ...` that this
// decision or the map refusal produced, so a later generic check keeps its
// named reason (NCM-V1-001).
func NamesFullRun(verdict string) bool {
	rest, ok := strings.CutPrefix(verdict, "FALLBACK ")
	if !ok {
		return false
	}
	reason, _, _ := strings.Cut(rest, " ")
	return slices.Contains(fullReasons, reason)
}

// FullRun returns the first line that needs every test, if any: an
// unavailable runner first, then paths in byte order.
func (decision Decision) FullRun() (Line, bool) {
	for _, line := range decision.Lines {
		if line.Full {
			return line, true
		}
	}
	return Line{}, false
}

// Audit renders the decision as selector audit lines: one
// `noncode <reason> <path>` line per decided path (a declared documentation
// path keeps the selector's own `documentation <path>` line instead), then
// one `data-map <key> <status>: <detail>` line per covering entry.
func (decision Decision) Audit() []string {
	var lines []string
	for _, line := range decision.Lines {
		if line.Reason == ReasonDocumentation {
			continue
		}
		text := "noncode " + line.Reason + " " + line.Path
		if line.Reason == ReasonProvenEntry {
			text += " -> " + strings.Join(line.Tests, " ")
		}
		lines = append(lines, text)
	}
	for _, entry := range decision.Entries {
		lines = append(lines, "data-map "+entry.Key+" "+entry.Status+": "+entry.Detail)
	}
	return lines
}

func set(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

func sortedUnique(values []string) []string {
	out := slices.Clone(values)
	slices.Sort(out)
	return slices.Compact(out)
}
