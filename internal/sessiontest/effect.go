// SPDX-License-Identifier: Apache-2.0

package sessiontest

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/gateselect"
	"github.com/Beamfall/corvint/internal/testrunner/contract"
)

// The effects one plan.unknown entry has on a run (SLF-V1-007): it is
// ignored, it adds one runner's declared suite, or it runs every test.
const (
	EffectIgnored    = "ignored"
	EffectAddsSuite  = "adds-suite:"
	EffectForcesFull = "forces-full"
)

// The selection verdict's scope: narrowed, or every test runs.
const (
	VerdictNarrowed = "narrowed"
	VerdictFull     = "full"
)

// The Go attribution of the plan's UNOWNED_DIRTY_PATH entries
// (RNC-V1-004, owner decision R32), which only the AFP-V0-012 Go selector
// decides: `corvint test` runs it, `corvint affected` and the MCP plan tool
// do not, so they pass AttributionUndecided and every such entry forces
// FULL there, the fail-closed reading.
const (
	AttributionUndecided = ""
	AttributionAll       = "attributed"
	AttributionNone      = "unattributed"
)

// EffectEntry is one plan.unknown entry with its effect and a hint that is
// never empty (SLF-V1-007).
type EffectEntry struct {
	Reason string `json:"reason"`
	Detail string `json:"detail"`
	Effect string `json:"effect"`
	Hint   string `json:"hint"`
}

// RunnerVerdict is one runner's tier on the owner decision R16 vocabulary:
// line, file, flow or full. A runner is named by its affected plan language
// namespace (go, typescript, python, ...), the name an adds-suite effect
// carries.
type RunnerVerdict struct {
	Runner string `json:"runner"`
	Tier   string `json:"tier"`
}

// SelectionVerdict is SelectionVerdictOf's output, the one each of
// `corvint affected`, `corvint test` and the MCP plan tool prints
// (SLF-V1-007). Unknown is in plan order; Runners is sorted by runner.
type SelectionVerdict struct {
	Scope   string          `json:"scope"`
	Unknown []EffectEntry   `json:"unknown"`
	Runners []RunnerVerdict `json:"runners"`
}

// VerdictInput is what SelectionVerdictOf reads: the plan's unknown
// entries and selected unit ids, the runner profile, the Go attribution of
// its unowned paths, and whether the plan's line evidence narrowed its Go
// units (GLS-V0-020).
type VerdictInput struct {
	Unknown       []Unknown
	Selected      []string
	RunnerProfile string
	Attribution   string
	LineNarrowed  bool
}

// unknownAction is the one decision for one plan.unknown entry, shared by
// SelectionVerdictOf and Build so that neither computes its own.
type unknownAction struct {
	effect string
	// suite is the language whose declared suite the entry adds; module the
	// nested Go module whose own suite it adds; check a still-required check
	// it names without widening the run; full the FULL reason.
	suite, module, check, full string
	hint                       string
}

// decideUnknown maps one plan.unknown entry to its action under the
// AFP-V0-028 allowlist `corvint test` applies after owner decision R32.
// An entry the allowlist does not know forces FULL.
func decideUnknown(entry Unknown, attribution, profile string) unknownAction {
	language, _, hasLanguage := strings.Cut(entry.Detail, ":")
	if entry.Reason == "NO_SELECTABLE_TEST" && hasLanguage && language != "go" && language != driverNamespace {
		// STR-V0-004: an unknown unit with no selectable test still needs
		// its language's declared full command.
		return unknownAction{effect: EffectAddsSuite + language, suite: language,
			hint: "no selectable test reaches this " + language + " unit, so the declared " + language + " suite runs"}
	}
	if directory, ok := moduleDirectory(entry); ok {
		if profile != ProfileCI {
			return unknownAction{effect: EffectAddsSuite + "go", module: directory,
				hint: "the nested Go module's own suite runs whole: go -C " + directory + " test ./... (AFP-V0-650)"}
		}
		check := ModuleCheck(directory)
		return unknownAction{effect: EffectIgnored, check: check, hint: "still required, not run: " + check}
	}
	if file, ok := binaryConsumer(entry); ok {
		check := BinaryConsumerCheck(file)
		return unknownAction{effect: EffectIgnored, check: check, hint: "still required, not run: " + check}
	}
	if entry.Reason == "UNOWNED_DIRTY_PATH" && attribution != AttributionAll {
		hint := "no runner instance owns this path, so every test runs (RNC-V1-004)"
		if attribution == AttributionUndecided {
			hint += "; corvint test keeps the run narrowed only when a Go selection rule attributes the path, which this command does not decide"
		}
		return unknownAction{effect: EffectForcesFull, full: gateselect.UnownedPathFallback + ": " + entry.Detail, hint: hint}
	}
	if allowedUnknown(entry) {
		return unknownAction{effect: EffectIgnored, hint: "allowlisted (AFP-V0-028): no narrowed run omits a reader of the change for it"}
	}
	if _, ok := gateselect.E2EFlowUnknown(entry.Reason, entry.Detail); ok {
		return unknownAction{effect: EffectIgnored, hint: "the flow-bound consumer it concerns stays selected or named as a check (AFU-V1-1652)"}
	}
	if language, ok := frontierSuite(entry); ok {
		return unknownAction{effect: EffectAddsSuite + language, suite: language,
			hint: "a " + language + " frontier may hide a reader of the change, so the declared " + language + " suite runs (AFP-V0-2750)"}
	}
	return unknownAction{effect: EffectForcesFull, full: "plan.unknown entry outside the AFP-V0-028 allowlist: " + entry.Reason + " " + entry.Detail,
		hint: "outside the AFP-V0-028 allowlist, so every test runs"}
}

// tierRank orders the R16 tiers from most to least narrow; a runner with
// selections at several tiers reports the least narrow.
var tierRank = map[string]int{contract.TierLine: 0, contract.TierFile: 1, contract.TierFlow: 2, contract.TierFull: 3}

// SelectionVerdictOf maps each plan.unknown entry to its effect and each
// runner to its tier (SLF-V1-007). It is pure. Any forces-full entry makes
// the scope full and every named runner's tier full; a runner whose suite an
// entry adds is full; otherwise a Go runner is line when line evidence
// narrowed the plan and file when not, a Playwright selection is flow, and
// every other selection is file. An entry's hint is the plan's when it has
// one and the effect's otherwise, so it is never empty.
func SelectionVerdictOf(input VerdictInput) SelectionVerdict {
	verdict := SelectionVerdict{Scope: VerdictNarrowed, Unknown: []EffectEntry{}, Runners: []RunnerVerdict{}}
	tiers := map[string]string{}
	raise := func(runner, tier string) {
		if current, ok := tiers[runner]; !ok || tierRank[tier] > tierRank[current] {
			tiers[runner] = tier
		}
	}
	for _, entry := range input.Unknown {
		action := decideUnknown(entry, input.Attribution, input.RunnerProfile)
		hint := strings.TrimSpace(entry.Hint)
		if hint == "" {
			hint = action.hint
		}
		verdict.Unknown = append(verdict.Unknown, EffectEntry{Reason: entry.Reason, Detail: entry.Detail, Effect: action.effect, Hint: hint})
		switch {
		case action.effect == EffectForcesFull:
			verdict.Scope = VerdictFull
		case action.suite != "":
			raise(action.suite, contract.TierFull)
		case action.module != "":
			raise("go", contract.TierFull)
		}
	}
	for _, id := range input.Selected {
		language, rest, ok := strings.Cut(id, ":")
		if !ok || language == driverNamespace || language == "declared-consumer" {
			continue
		}
		switch {
		case language == "go" && input.LineNarrowed:
			raise(language, contract.TierLine)
		case language == "typescript" && strings.HasPrefix(rest, "playwright:"):
			raise(language, contract.TierFlow)
		default:
			raise(language, contract.TierFile)
		}
	}
	for _, runner := range slices.Sorted(maps.Keys(tiers)) {
		tier := tiers[runner]
		if verdict.Scope == VerdictFull {
			tier = contract.TierFull
		}
		verdict.Runners = append(verdict.Runners, RunnerVerdict{Runner: runner, Tier: tier})
	}
	return verdict
}

// attributionOf reads the Go attribution from the AFP-V0-012 verdict line
// over a Go universe: run or NOTHING means the selector found a Go rule for
// every unowned path, since it falls back when it does not.
func attributionOf(input Input) string {
	verdict, _, _ := strings.Cut(input.GoSelection, " ")
	if (verdict == "run" || verdict == "NOTHING") && len(input.Universe) != 0 {
		return AttributionAll
	}
	return AttributionNone
}

// SelectionVerdict is SelectionVerdictOf over the inputs `corvint test`
// builds its manifest from, with the Go attribution its Go selector decided.
func (input Input) SelectionVerdict() SelectionVerdict {
	return SelectionVerdictOf(VerdictInput{
		Unknown: input.Unknown, Selected: input.Selected, RunnerProfile: input.RunnerProfile,
		Attribution: attributionOf(input), LineNarrowed: len(input.LineUnits) != 0,
	})
}

// EffectCounts counts the verdict's entries by effect, sorted by effect, as
// "<effect>=<n>" words for a one-line report.
func (verdict SelectionVerdict) EffectCounts() []string {
	counts := map[string]int{}
	for _, entry := range verdict.Unknown {
		counts[entry.Effect]++
	}
	words := []string{}
	for _, effect := range slices.Sorted(maps.Keys(counts)) {
		words = append(words, effect+"="+strconv.Itoa(counts[effect]))
	}
	return words
}
