package main

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TSP-V0-942 (V1-3349): --format text restates each frontier bearer on one
// `bears` line naming the entry, its source and the unit or plugin, after the
// unknown lines; the JSON plan carries them as plan.frontierBearers.
func TestAffectedTextNamesFrontierBearers_TSP_V0_942(t *testing.T) {
	var receipt affectedCompactReceipt
	receipt.Plan.Unknown = []affected.Unknown{{Reason: affected.UnknownLanguageFrontier, Detail: "shell:foreign-invocation-unmodelled"}}
	receipt.Plan.FrontierBearers = []affected.FrontierBearer{
		{Detail: "shell:foreign-invocation-unmodelled", Via: affected.FrontierBearerReachedUnit, Unit: "shell:script/a.sh", Witness: affected.WitnessPathLiteralReader, DirtyPath: "docs/x.md"},
		{Detail: "go:build-constraint-variants", Via: affected.FrontierBearerUnownedPath, Language: "go", DirtyPath: "Makefile"},
		{Detail: "shell:foreign-invocation-unmodelled", Via: affected.FrontierBearerOmitted, Count: 3},
	}
	text := renderAffectedText(receipt)
	for _, want := range []string{
		"\nbears shell:foreign-invocation-unmodelled REACHED_UNIT unit shell:script/a.sh witness PATH_LITERAL_READER dirty docs/x.md\n",
		"\nbears go:build-constraint-variants UNOWNED_DIRTY_PATH language go dirty Makefile\n",
		"\nbears shell:foreign-invocation-unmodelled OMITTED count 3\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text lacks %q:\n%s", want, text)
		}
	}
	if strings.Index(text, "\nunknown ") > strings.Index(text, "\nbears ") {
		t.Fatalf("bears lines precede the unknown lines:\n%s", text)
	}
}
