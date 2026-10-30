package dogfoodflow

import (
	"fmt"
	"strconv"
	"strings"
)

// bootstrapIntentReason is the ocm prepare refusal a cited bootstrap intent
// hunk causes (OCM-V0-009).
const bootstrapIntentReason = "bootstrap-intent-not-unknown"

// bootstrapIntentHint is the fix for a plan that cites a hunk of an intent
// absent at BASE, shared by the pre-flight and the ocm-prepare row.
const bootstrapIntentHint = "an intent in DOGFOOD_INTENTS_FILE that this change adds (absent at BASE, such as a companion spec) is a bootstrap intent: OCM-V0-009 keeps each of its hunks unknown, the dogfood policy counts them as bootstrap unknowns, and the plan owes them no row; delete each named row from DOGFOOD_CITATIONS, or keep it as SELECTOR<TAB>unknown<TAB>insufficient-evidence<TAB>bootstrap intent added by this change, then rerun corvint dogfood inputs-check (DCW-V0-449)"

// precheckBootstrapIntentRows refuses, in the prepare pass and before any
// bind, a cite row whose hunk is on an intent path absent at BASE. The cite
// itself succeeds, so without this check the bind pass spends its run and
// only ocm prepare refuses bootstrap-intent-not-unknown (DCW-V0-449,
// V1-3108). plan is the plan planShape admitted, its ordinals already in this
// map's terms; unknown rows keep the hunk unknown and pass.
func (c *change) precheckBootstrapIntentRows(plan []byte) int {
	if !c.manifestValid || len(c.intents) == 0 {
		return 0
	}
	bootstrap := map[string]bool{}
	for _, path := range c.intents {
		if !c.gitSucceeds("cat-file", "-e", c.base+":"+path) {
			bootstrap[path] = true
		}
	}
	if len(bootstrap) == 0 {
		return 0
	}
	data, err := readPrefix(c.path(finishLaneSidecar), maxPlanBytes)
	if err != nil {
		return 0
	}
	hunks := mapHunks(data)
	byID := make(map[string]int, len(hunks))
	for index, hunk := range hunks {
		byID[hunk["id"]] = index
	}
	refusals := []string{}
	for index, line := range textLines(plan) {
		fields := strings.SplitN(line, "\t", 4)
		if len(fields) != 4 || unknownRow(fields) {
			continue
		}
		hunk := -1
		if ordinal.MatchString(fields[0]) {
			if value, err := strconv.Atoi(fields[0]); err == nil && value <= len(hunks) {
				hunk = value - 1
			}
		} else if found, ok := byID[fields[0]]; ok {
			hunk = found
		}
		if hunk < 0 || !bootstrap[hunks[hunk]["path"]] {
			continue
		}
		refusals = append(refusals, fmt.Sprintf("row %d hunk=%s path=%s cites %s as %s, but %s is an intent absent at BASE",
			index+1, fields[0], strconv.Quote(hunks[hunk]["path"]), fields[1]+":"+fields[2], fields[3], hunks[hunk]["path"]))
	}
	if len(refusals) == 0 {
		return 0
	}
	c.addStep(inputsPrecheckStep, "NOT_PRODUCED", bootstrapIntentReason)
	c.say("dogfood-change: FAIL %s ocm-prepare %s input=--citations %s rows=%d (ocm prepare would refuse it after the bind; nothing was bound, DCW-V0-449):\n",
		inputsPrecheckStep, bootstrapIntentReason, c.options.Citations, len(refusals))
	for _, refusal := range refusals[:min(len(refusals), maxNamedRefusals)] {
		c.say("  %s\n", refusal)
	}
	if len(refusals) > maxNamedRefusals {
		c.say("  (+%d more)\n", len(refusals)-maxNamedRefusals)
	}
	c.say("  fix: %s\n", bootstrapIntentHint)
	return 1
}
