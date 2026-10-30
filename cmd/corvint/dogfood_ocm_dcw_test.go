package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/lrfrepo"
)

// V1-0147 exercises structural test-claim linkage, not full DCW clause adequacy.
// All maps and claims come from the built producer. The fixture archives this
// executing test's source so its claim anchors are real tests, not invented JSON.
func TestDogfoodDCWLinksSurviveRebind(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, _ := portableDogfoodRepo(t)
	const intent = "docs/specs/daily-change-evidence-workflow-v0.md"
	const testPath = "evidence/dogfood_ocm_dcw_test.go"
	for destination, source := range map[string]string{
		intent: "../../" + intent, testPath: "dogfood_ocm_dcw_test.go",
	} {
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		cemWrite(t, root, destination, string(data))
	}
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "archive DCW intent and executable claim source")
	base := cemGit(t, root, "rev-parse", "HEAD")
	cemWrite(t, root, "fixture/fixture.go", "package fixture\nfunc Answer() int { return 2 }\n// Linkage fixture revision one.\n")
	cemGit(t, root, "commit", "-qam", "first content change")

	inputsDir := t.TempDir()
	intents := filepath.Join(inputsDir, "intents")
	citations := filepath.Join(inputsDir, "citations.tsv")
	links := filepath.Join(inputsDir, "links.tsv")
	cemWrite(t, inputsDir, "intents", intent+"\n")
	// Cite the actual requirement span without pinning this test to doc line numbers.
	spec, err := os.ReadFile(filepath.Join(root, intent))
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(spec), "- `DCW-V0-013`:")
	end := strings.Index(string(spec), "- `DCW-V0-016`:")
	if start < 0 || end <= start {
		t.Fatal("DCW requirement span missing")
	}
	span := fmt.Sprintf("%d:%d", strings.Count(string(spec[:start]), "\n")+1, strings.Count(string(spec[:end]), "\n"))
	cemWrite(t, inputsDir, "citations.tsv", "1\t"+intent+"\t"+span+"\tspecification\n")
	requirements := []string{"DCW-V0-013", "DCW-V0-014", "DCW-V0-015"}
	selectors := []string{"ordered-rebind", "input-remediation", "stale-evidence"}
	var plan strings.Builder
	for i, id := range requirements {
		plan.WriteString(intent + "\t" + id + "\t1\t" + testPath + "\ttest:TestDogfoodDCWLinksSurviveRebind/case:dcw-v0-" + selectors[i] + "\n")
	}
	cemWrite(t, inputsDir, "links.tsv", plan.String())
	inputs := []string{"DOGFOOD_TASK=Retain DCW test claim linkage across rebinding.",
		"DOGFOOD_VERIFY=go test ./fixture", "DOGFOOD_OUTCOME=passed",
		"DOGFOOD_INTENTS_FILE=" + intents, "DOGFOOD_CITATIONS=" + citations,
		"DOGFOOD_OCM_LINKS=" + links}
	change := func(want int) string {
		t.Helper()
		code, stdout, stderr := run.exec(t, root, inputs, "dogfood", "change", base)
		if code != want {
			t.Fatalf("change exit=%d want=%d stdout=%s stderr=%s", code, want, stdout, stderr)
		}
		return stderr
	}
	bind := func() {
		t.Helper()
		cemGit(t, root, "add", ".corvint/change.cem.json")
		cemGit(t, root, "commit", "-qm", "bind change evidence")
	}

	if !t.Run("DCW-V0-014 input remediation", func(t *testing.T) {
		stderr := change(1)
		if !strings.Contains(stderr, "\n  local-outcome: record-index-failed\n") || !strings.Contains(stderr, "fix:") {
			t.Fatalf("missing uncommitted-sidecar refusal/remediation: %s", stderr)
		}
	}) {
		return
	}
	// The first producer pass really linked all three requirements, even though
	// the untracked CEM prevents local completion. Commit it and replay the plan.
	first := assertDCWLinks(t, root, base, requirements, testPath)
	bind()
	change(0)
	assertDCWLinks(t, root, base, requirements, testPath)
	// OCM-V0-020: the map cites the three linked requirements, so the summary
	// counts those three, not the DCW intent's whole requirements section.
	if out := run.ok(t, root, "dogfood", "check", base); !strings.Contains(out, " ocm=ready-for-review requirements=3 linked=3 unlinked=0 ") {
		t.Fatalf("check summary does not count the cited, linked requirements: %s", out)
	}
	oldTarget := cemGit(t, root, "rev-parse", "HEAD")

	if !t.Run("DCW-V0-015 stale evidence", func(t *testing.T) {
		cemWrite(t, root, "fixture/fixture.go", "package fixture\nfunc Answer() int { return 2 }\n// Linkage fixture revision two.\n")
		cemGit(t, root, "commit", "-qam", "later content change")
		if cemGit(t, root, "rev-parse", "HEAD") == oldTarget {
			t.Fatal("later commit did not advance HEAD")
		}
		code, stdout, stderr := run.exec(t, root, nil, "dogfood", "check", base)
		if code != 1 || !strings.Contains(stderr, "dogfood-report-drift") || strings.Contains(stdout, "dogfood-check: PASS") {
			t.Fatalf("stale check exit=%d stdout=%s stderr=%s", code, stdout, stderr)
		}
	}) {
		return
	}
	if !t.Run("DCW-V0-013 ordered rebind", func(t *testing.T) {
		// DOGFOOD.md steps 3–7: regenerate/cite the changed CEM, commit it,
		// then rerun with the SAME saved link plan on the clean new target.
		change(1)
		bind()
		change(0)
		rebound := assertDCWLinks(t, root, base, requirements, testPath)
		if !reflect.DeepEqual(first, rebound) {
			t.Fatalf("test claim links changed after rebind: before=%v after=%v", first, rebound)
		}
		run.ok(t, root, "dogfood", "check", base)
		if dirty := cemGit(t, root, "status", "--porcelain"); dirty != "" {
			t.Fatalf("rebind left dirty worktree: %s", dirty)
		}
	}) {
		return
	}
}

func assertDCWLinks(t *testing.T, root, base string, requirements []string, testPath string) map[string][]string {
	t.Helper()
	target := cemGit(t, root, "rev-parse", "HEAD")
	checked, err := lrfrepo.ReadOCM(context.Background(), root, lrfrepo.OCMReadOptions{
		OCMPath: ".corvint/change.ocm.001.json", CEMPath: ".corvint/change.cem.json",
		ExpectedBase: base, Target: target, ExpectedBaseGiven: true, TargetGiven: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked.State != "ready-for-review" || checked.Verification["valid"] != true {
		t.Fatalf("OCM state=%s verification=%v", checked.State, checked.Verification)
	}
	var document struct {
		TargetRevision string
		Obligations    []struct {
			ID, Disposition, Reason string
			HunkIDs, ClaimIDs       []string
		}
		Claims []struct{ ID, Path, BlobOID, Selector string }
	}
	if err := json.Unmarshal(checked.OCMRaw, &document); err != nil {
		t.Fatal(err)
	}
	if document.TargetRevision != target || len(document.Claims) != len(requirements) {
		t.Fatalf("target=%s want=%s claims=%v", document.TargetRevision, target, document.Claims)
	}
	claims := map[string]string{}
	blob := cemGit(t, root, "rev-parse", target+":"+testPath)
	for _, claim := range document.Claims {
		if claim.Path != testPath || claim.BlobOID != blob {
			t.Fatalf("claim not pinned to test source: %+v", claim)
		}
		claims[claim.ID] = claim.Selector
	}
	linked := map[string][]string{}
	for _, obligation := range document.Obligations {
		if obligation.Disposition != "linked" {
			continue
		}
		if obligation.Reason != "change-and-test-linked" || len(obligation.HunkIDs) != 1 || len(obligation.ClaimIDs) != 1 || claims[obligation.ClaimIDs[0]] == "" {
			t.Fatalf("incomplete obligation links: %+v", obligation)
		}
		linked[obligation.ID] = obligation.ClaimIDs
	}
	if len(linked) != len(requirements) {
		t.Fatalf("linked=%v want=%v", linked, requirements)
	}
	for i, id := range requirements {
		if len(linked[id]) != 1 {
			t.Fatalf("missing test claim for %s: %v", id, linked)
		}
		want := "test:TestDogfoodDCWLinksSurviveRebind/case:dcw-v0-" + []string{"ordered-rebind", "input-remediation", "stale-evidence"}[i]
		if got := claims[linked[id][0]]; got != want {
			t.Fatalf("%s links %q, want %q", id, got, want)
		}
	}
	return linked
}
