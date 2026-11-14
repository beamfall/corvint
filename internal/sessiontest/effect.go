// SPDX-License-Identifier: Apache-2.0

package sessiontest

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/gateselect"
	"github.com/Beamfall/corvint/internal/testid"
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
// (RNC-V1-004, owner decision R32), which the AFP-V0-012 Go selector decides
// (attributionOf).
const (
	AttributionAll  = "attributed"
	AttributionNone = "unattributed"
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

// SelectionVerdict is the one verdict each of `corvint affected`,
// `corvint test` and the MCP plan tool prints (SLF-V1-007). Unknown is in
// plan order; Runners is sorted by runner. Reason is the FULL reason when
// Scope is full, the same one `corvint test` runs FULL for.
type SelectionVerdict struct {
	Scope   string          `json:"scope"`
	Reason  string          `json:"reason,omitempty"`
	Unknown []EffectEntry   `json:"unknown"`
	Runners []RunnerVerdict `json:"runners"`
}

// unknownAction is the one decision for one plan.unknown entry, which
// decideScope makes for Build and the selection verdict alike.
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
		return unknownAction{effect: EffectForcesFull, full: gateselect.UnownedPathFallback + ": " + entry.Detail,
			hint: "no runner instance owns this path and no Go selection rule attributes it, so every test runs (RNC-V1-004)"}
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

// How `corvint test` runs one plan.selected unit: a Go package, a declared
// consumer's own argv, a declared provider (STR-V0-8702), the runner
// contract in the files' owning instances (RNC-V1-019), its language's
// declared full command (STR-V0-004), or not at all (a make target or CI
// job, AZU-V0-001).
const (
	runNone = iota
	runGo
	runConsumer
	runProvider
	runContract
	runLanguage
)

// selectedRun is how `corvint test` runs the selected unit id, and the
// language it names; Build places each unit by it and the verdict's runner
// tiers read it, so a tier describes the execution that happens.
func selectedRun(input Input, id string) (int, string) {
	language, _, ok := strings.Cut(id, ":")
	switch {
	case strings.HasPrefix(id, "declared-consumer:"):
		return runConsumer, ""
	case input.RunnerProfile != ProfileCI && providerUnit(input.Declared, id):
		// The ci profile runs only Go (STR-V0-012), so there the unit names
		// its language's declared command as a check instead.
		return runProvider, language
	case !ok || language == driverNamespace:
		return runNone, ""
	case language == "go":
		return runGo, language
	}
	if _, ok := contractTests(input, id); ok {
		return runContract, language
	}
	return runLanguage, language
}

// selectedTier is the R16 tier of the execution selectedRun names: a Go
// package is line when line evidence narrowed the plan and file when not; a
// Playwright provider runs the selected files' tests, flow; a Vitest
// provider runs the whole suite of its declared directory, a superset of
// the selected files (providerInvocations), and a language's declared full
// command runs that language's whole suite, so both are full. A contract
// run runs the selected files in their owning instances, file, unless the
// instance runs its full command (contractFull). A unit no runner runs has
// no tier.
func selectedTier(input Input, id string) (string, string) {
	kind, language := selectedRun(input, id)
	switch kind {
	case runContract:
		tests, _ := contractTests(input, id)
		for _, test := range tests {
			if contractFull(input, test.instance) {
				return language, contract.TierFull
			}
		}
		return language, contract.TierFile
	case runGo:
		if len(input.LineUnits) != 0 {
			return language, contract.TierLine
		}
		return language, contract.TierFile
	case runProvider:
		if runner, _, _ := splitProviderUnit(id); runner == ProviderRunnerPlaywright {
			return language, contract.TierFlow
		}
		return language, contract.TierFull
	case runLanguage:
		return language, contract.TierFull
	}
	return "", ""
}

// contractFull reports whether Build runs instance's full command instead of
// contract selections in a narrowed plan: the kill switch names its runner
// (RNC-V1-008), or its inventory record is absent, stale or unreadable
// (RNC-V1-002, RNC-V1-019).
func contractFull(input Input, instance contract.Instance) bool {
	return input.KillSwitch.Full(instance.Runner) || input.FullInstances[instance.Key()] != ""
}

// scopeDecision is the complete scope decision for one plan, the one Build
// applies and the selection verdict reports (SLF-V1-007): the FULL reason,
// empty when the run is narrowed; the Go packages the AFP-V0-012 verdict
// runs; and one action per plan.unknown entry, in plan order.
type scopeDecision struct {
	full    string
	goRun   []string
	actions []unknownAction
}

// decideEntries is decideUnknown over every plan.unknown entry.
func decideEntries(unknown []Unknown, attribution, profile string) []unknownAction {
	actions := make([]unknownAction, 0, len(unknown))
	for _, entry := range unknown {
		actions = append(actions, decideUnknown(entry, attribution, profile))
	}
	return actions
}

// decideScope is the scope decision `corvint test` runs by. FULL, in order
// of precedence: no plan or no readable declaration (only the declared full
// scope runs); the `full` kill switch (RNC-V1-008); a runner the inventory
// or declaration names but the tree no longer declares (RNC-V1-004); the Go
// selector's fallback or a missing verdict (AFP-V0-012); the first plan.unknown entry that forces full; and a
// selected package outside the Go universe.
func decideScope(input Input) scopeDecision {
	decision := scopeDecision{actions: decideEntries(input.Unknown, attributionOf(input.GoSelection, input.Universe), input.RunnerProfile)}
	switch {
	case input.PlanError != "":
		decision.full = "no plan: " + input.PlanError
		return decision
	case input.Declared.Unreadable != "":
		decision.full = "the declared full scope is unreadable"
		return decision
	}
	switch verdict, detail, _ := strings.Cut(input.GoSelection, " "); verdict {
	case "FALLBACK":
		decision.full = detail
	case "run":
		decision.goRun = strings.Fields(detail)
	case "NOTHING":
	default:
		decision.full = "no AFP-V0-012 attribution verdict"
	}
	if len(input.Removed) != 0 {
		// RNC-V1-004, owner decision R32: a removed runner runs every test.
		decision.full = ReasonRunnerRemoved + ": " + strings.Join(input.Removed, ", ") + " is named by the inventory or " + CommandsPath + " but no longer declared, so every test runs"
	}
	if input.KillSwitch.All {
		// RNC-V1-008: `full` makes every runner run its full command.
		decision.full = input.KillSwitch.Describe()
	}
	for _, action := range decision.actions {
		if decision.full == "" && action.suite == "" && action.module == "" && action.check == "" {
			decision.full = action.full
		}
	}
	universe := map[string]bool{}
	for _, pkg := range input.Universe {
		universe[pkg] = true
	}
	for _, pkg := range decision.goRun {
		if decision.full == "" && !universe[pkg] {
			decision.full = "selected package outside the Go universe: " + pkg
		}
	}
	return decision
}

// verdict reports the decision (SLF-V1-007). Its scope is full exactly when
// the decision has a FULL reason, and then every named runner's tier is
// full; otherwise a runner whose suite an entry adds is full, a runner the
// kill switch names or with an instance whose inventory record is not
// current runs its full command, so is full (RNC-V1-008, RNC-V1-019), and
// every other runner has the tier of the execution its selections get
// (selectedTier), the least narrow winning. An entry's hint is the plan's
// when it has one and the effect's otherwise, so it is never empty.
func (decision scopeDecision) verdict(input Input) SelectionVerdict {
	verdict := SelectionVerdict{Scope: VerdictNarrowed, Unknown: []EffectEntry{}, Runners: []RunnerVerdict{}}
	if decision.full != "" {
		verdict.Scope, verdict.Reason = VerdictFull, decision.full
	}
	tiers := map[string]string{}
	raise := func(runner, tier string) {
		if current, ok := tiers[runner]; !ok || tierRank[tier] > tierRank[current] {
			tiers[runner] = tier
		}
	}
	for index, entry := range input.Unknown {
		action := decision.actions[index]
		hint := strings.TrimSpace(entry.Hint)
		if hint == "" {
			hint = action.hint
		}
		verdict.Unknown = append(verdict.Unknown, EffectEntry{Reason: entry.Reason, Detail: entry.Detail, Effect: action.effect, Hint: hint})
		switch {
		case action.suite != "":
			raise(action.suite, contract.TierFull)
		case action.module != "":
			raise("go", contract.TierFull)
		}
	}
	for _, id := range input.Selected {
		if runner, tier := selectedTier(input, id); runner != "" {
			raise(runner, tier)
		}
	}
	if input.KillSwitch.Full(testid.Go) && (len(input.Universe) != 0 || goSuite(input)) {
		// RNC-V1-008: the Go kill switch runs the whole root universe, or
		// the declared Go suite, and every nested module.
		raise("go", contract.TierFull)
	}
	for _, instance := range input.Instances {
		language, ok := runnerLanguages[instance.Runner]
		switch {
		case instance.Runner == testid.Go:
			if input.KillSwitch.Full(testid.Go) && instance.Root != "." && input.RunnerProfile != ProfileCI {
				// RNC-V1-008: a nested module's own suite runs whole.
				raise("go", contract.TierFull)
			}
		case !ok:
		case input.KillSwitch.Full(instance.Runner):
			// RNC-V1-008: its declared language command or declared:<instance>.
			raise(language, contract.TierFull)
		case input.RunnerProfile != ProfileCI && contractRunners[instance.Runner] && input.FullInstances[instance.Key()] != "":
			if _, declared := input.Declared.Command(language); !declared {
				// RNC-V1-019, TIV-V1-006: declared:<instance> runs whole.
				raise(language, contract.TierFull)
			}
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

// attributionOf reads the Go attribution of a plan's UNOWNED_DIRTY_PATH
// entries from the AFP-V0-012 verdict line over a Go universe: run or
// NOTHING means the selector found a Go rule for every unowned path, since
// it falls back when it does not.
func attributionOf(goSelection string, universe []string) string {
	verdict, _, _ := strings.Cut(goSelection, " ")
	if (verdict == "run" || verdict == "NOTHING") && len(universe) != 0 {
		return AttributionAll
	}
	return AttributionNone
}

// SelectionVerdict is the one verdict `corvint affected`, `corvint test` and
// the MCP plan tool print (SLF-V1-007): the scope decision Build applies to
// the same Input, so it states exactly the scope `corvint test` runs. Each
// surface fills the Input from the plan the same way; one with no AFP-V0-012
// Go verdict is FULL, the fail-closed reading.
func (input Input) SelectionVerdict() SelectionVerdict {
	return decideScope(input).verdict(input)
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
