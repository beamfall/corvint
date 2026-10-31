package dogfoodflow

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/Beamfall/corvint/internal/dogfoodocm"
	"github.com/Beamfall/corvint/internal/lrfrepo"
)

// aggregateObligationBound is dogfoodocm.MaxObligations. Since that bound is
// derived from the 16-scope count and a counted intent is within its map's
// 256-obligation bound, a well-formed manifest cannot exceed it; tests lower
// it to reach the refusal (V1-2971).
var aggregateObligationBound = dogfoodocm.MaxObligations

// precheckIntentObligations refuses a prepare or full pass before any step runs,
// and before the previous run's evidence is cleared, when the intents in
// DOGFOOD_INTENTS_FILE derive more obligations at HEAD than the fixed OCM
// aggregate bound admits. The aggregate would refuse too-many-obligations
// only after every per-intent prepare and status had run, which on a 583-path
// range took 41.7 minutes; this reads only the intent blobs, in seconds, and
// names the same subset fix (OCM-V0-028, V1-2893). A manifest it cannot read
// as a valid intent list, or an intent prepare itself would refuse, is left to
// the existing steps, which stay authoritative; the bound never widens.
// finish-lane's prepare pass runs it, so an over-bound manifest stops before
// any bind; its bind pass reads the same unchanged inputs at the same HEAD
// (DCW-V0-378) and skips it (DCW-V0-379).
func (c *change) precheckIntentObligations() {
	if c.scope == passBind {
		return
	}
	intents := c.manifestIntents()
	if len(intents) == 0 {
		return
	}
	shares, err := lrfrepo.IntentObligationCounts(c.ctx, c.root, c.target, intents)
	if overMapBound(err) {
		c.refuseOverBoundIntents(intents)
	}
	if err != nil || len(shares) != len(intents) {
		return
	}
	total := 0
	perScope := make([]string, len(shares))
	for index, share := range shares {
		total += share
		perScope[index] = fmt.Sprintf("%d=%d", index+1, share)
	}
	if total <= aggregateObligationBound {
		return
	}
	c.say("%s: REFUSE too-many-obligations\n", c.prefix)
	c.say("  the %d intent scopes in DOGFOOD_INTENTS_FILE derive %d obligations at HEAD, over the fixed OCM aggregate bound %d; per scope %s; no step ran and the previous evidence is kept (OCM-V0-021, OCM-V0-028)\n",
		len(intents), total, aggregateObligationBound, strings.Join(perScope, ", "))
	c.say("  fix: %s\n", fitHint(intents, aggregateObligationBound, shares))
	c.exit(2)
}

// manifestIntents is the intent list validateIntentManifest would freeze, or
// nil when the manifest is absent, malformed, over a bound, declares no intent
// or names a record change's own entries, which prepare no OCM scope; those
// cases keep their own ocm-aggregate reasons.
func (c *change) manifestIntents() []string {
	source := c.options.IntentsFile
	if source == "" || !isRegular(source) || isSymlink(source) {
		return nil
	}
	data, err := readPrefix(source, maxIntentManifestBytes+1)
	if err != nil || len(data) == 0 || len(data) > maxIntentManifestBytes || data[len(data)-1] != '\n' || bytes.Equal(data, noIntentManifest) {
		return nil
	}
	paths := readLines(data)
	previous := ""
	for _, path := range paths {
		if !canonicalIntentPath(path) || (previous != "" && previous >= path) {
			return nil
		}
		previous = path
	}
	if len(paths) > maxAggregateScopes || c.recordIntent(paths) {
		return nil
	}
	return paths
}

// overMapBound reports the derivation refusal of an intent whose Requirements
// section alone exceeds one OCM map's obligation bound.
func overMapBound(err error) bool {
	var refusal *lrfrepo.Error
	return errors.As(err, &refusal) && refusal.Code == "too-many-obligations"
}

// refuseOverBoundIntents refuses, before any step runs, a manifest naming an
// intent that one OCM map cannot hold: its count would fail, and ocm-prepare
// would refuse too-many-obligations only after the bind commit (DCW-V0-961).
// Each intent is derived alone, so the refusal names every one over the bound.
func (c *change) refuseOverBoundIntents(intents []string) {
	over := []string{}
	for _, intent := range intents {
		if _, err := lrfrepo.IntentObligationCounts(c.ctx, c.root, c.target, []string{intent}); overMapBound(err) {
			over = append(over, intent)
		}
	}
	if len(over) == 0 {
		return
	}
	c.say("%s: REFUSE too-many-obligations\n", c.prefix)
	c.say("  intent %s in DOGFOOD_INTENTS_FILE derives more obligations at HEAD than one OCM map admits (%d); no step ran and the previous evidence is kept (DCW-V0-961)\n",
		strings.Join(over, ", "), lrfrepo.MaxOCMObligations)
	c.say("  fix: drop it from DOGFOOD_INTENTS_FILE and cite the BASE lines of it the change depends on, or list a companion spec that holds the change's requirements (OCM-V0-024), then rerun\n")
	c.exit(2)
}
