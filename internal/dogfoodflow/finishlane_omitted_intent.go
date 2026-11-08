package dogfoodflow

import (
	"strings"

	"github.com/Beamfall/corvint/internal/lrfrepo"
)

// noteOmittedOverBoundSpecs names, on a passing inputs-check, each spec the
// change adds or modifies under docs/specs/ that derives more obligations than
// one OCM map admits and that neither DOGFOOD_INTENTS_FILE lists nor a
// deferred plan row records (DWC-V0-3731, V1-2275). Such a spec cannot be an
// intent (DCW-V0-961 refuses it there), so without a deferred row the binding
// would leave it out silently; the NOTE names it and the deferral that records
// the omission in the CEM. The 256 bound is unchanged, and the NOTE changes no
// exit status.
func (l *finishLane) noteOmittedOverBoundSpecs() {
	changed, status, bounded := l.boundedGit("diff", "--name-only", "--no-renames", "--diff-filter=AM", l.base, l.target, "--", "docs/specs/*.md")
	if status != 0 || !bounded {
		return
	}
	named := map[string]bool{}
	if source := l.options.Change.IntentsFile; source != "" && isRegular(source) {
		if data, err := readPrefix(source, maxIntentManifestBytes+1); err == nil {
			for _, path := range readLines(data) {
				named[path] = true
			}
		}
	}
	if source := l.options.Change.Citations; source != "" && isRegular(source) {
		if data, err := readPrefix(source, maxPlanBytes); err == nil {
			for _, line := range textLines(data) {
				if fields := strings.Split(line, "\t"); len(fields) >= 2 && fields[0] == deferredRowKind {
					named[fields[1]] = true
				}
			}
		}
	}
	for _, spec := range nonEmpty(strings.Split(changed, "\n")) {
		if named[spec] {
			continue
		}
		if _, err := lrfrepo.IntentObligationCounts(l.ctx, l.root, l.target, []string{spec}); overMapBound(err) {
			l.line("inputs-check NOTE intent-over-bound-omitted path=%s: the change edits this spec, which derives more obligations than one OCM map admits (%d), so it cannot be an intent and the binding would leave it out silently; record the omission with the plan row deferred<TAB>%s<TAB>REASON (DCW-V0-370) and cite the BASE requirement rows the change depends on (DWC-V0-3731)", spec, lrfrepo.MaxOCMObligations, spec)
		}
	}
}
