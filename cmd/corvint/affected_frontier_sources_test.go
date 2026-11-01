package main

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TSP-V0-724: the default projection keeps plan.frontierSources, and the text
// form names each frontier entry's source on its unknown line.
func TestCompactAndTextPlanKeepFrontierSources_TSP_V0_724(t *testing.T) {
	t.Parallel()
	sources := map[string]string{"shell:foreign-invocation-unmodelled": "shell:script/build.sh"}
	unknown := []affected.Unknown{{Reason: affected.UnknownLanguageFrontier, Detail: "shell:foreign-invocation-unmodelled"}, {Reason: affected.UnknownUnownedDirtyPath, Detail: "notes.txt"}}
	full := affectedReceipt{Plan: affected.Plan{Selected: []affected.Selection{}, Excluded: []affected.Exclusion{}, Unknown: unknown, FrontierSources: sources}}
	compact, err := compactAffected(full)
	if err != nil {
		t.Fatal(err)
	}
	if got := compact.Plan.FrontierSources["shell:foreign-invocation-unmodelled"]; got != "shell:script/build.sh" || len(compact.Plan.FrontierSources) != 1 {
		t.Fatalf("frontierSources=%v want %v", compact.Plan.FrontierSources, sources)
	}
	text := renderAffectedText(compact)
	if !strings.Contains(text, "unknown LANGUAGE_FRONTIER shell:foreign-invocation-unmodelled source shell:script/build.sh\n") {
		t.Fatalf("text lacks the frontier source:\n%s", text)
	}
	if !strings.Contains(text, "unknown UNOWNED_DIRTY_PATH notes.txt\n") {
		t.Fatalf("text changed a non-frontier unknown:\n%s", text)
	}
}
