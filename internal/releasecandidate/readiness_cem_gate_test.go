package releasecandidate

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/companionrelease"
)

func cemGateOutcome(t *testing.T, revision, verdict, reason string) string {
	t.Helper()
	raw, err := companionrelease.EncodeStableGateOutcome(companionrelease.StableGateOutcome{Profile: companionrelease.StableGateOutcomeProfile, Revision: revision, DeclarationSha256: strings.Repeat("d", 64), Records: []string{strings.Repeat("e", 64)}, Verdict: verdict, Reason: reason})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// TestCEMV1614ReadinessBindsTheCEMGateOutcome pins the cem-release-gate row:
// a 1.0.0-rc.3 record binds a READY outcome over the candidate commit by
// digest, and a missing outcome, a REFUSED one, one for another commit, a
// malformed one or a non-PASS row status refuses the candidate, at build and
// at verification. The rc.1 and rc.2 catalogues carry no such row.
func TestCEMV1614ReadinessBindsTheCEMGateOutcome(t *testing.T) {
	fixture := newReadinessFixtureVersion(t, cleanGoMod, "1.0.0-rc.3")
	options := func(evidence map[string]ReadinessEvidence) ReadinessOptions {
		return ReadinessOptions{CandidateDirectory: fixture.candidate, SourceRoot: fixture.source, Evidence: evidence}
	}
	if _, _, err := BuildReadinessRecord(t.Context(), options(nil)); err == nil || !strings.Contains(err.Error(), "needs the cem-release-gate outcome") {
		t.Fatalf("a candidate without a gate outcome: %v", err)
	}
	ready := fixture.file(t, "ready.json", cemGateOutcome(t, fixture.commit, companionrelease.StableGateReady, ""))
	refused := fixture.file(t, "refused.json", cemGateOutcome(t, fixture.commit, companionrelease.StableGateRefused, "release-external-incomplete: independent-external-consumer"))
	for name, evidence := range map[string]ReadinessEvidence{
		"refused":        {Status: "PASS", Path: refused},
		"another commit": {Status: "PASS", Path: fixture.file(t, "other.json", cemGateOutcome(t, strings.Repeat("a", 40), companionrelease.StableGateReady, ""))},
		"malformed":      {Status: "PASS", Path: fixture.file(t, "bad.json", "{}\n")},
		"failed row":     {Status: "FAIL", Path: ready},
		"not run":        {Status: "NOT_RUN", Reason: "skipped"},
	} {
		if _, _, err := BuildReadinessRecord(t.Context(), options(map[string]ReadinessEvidence{cemGateRow: evidence})); err == nil {
			t.Fatalf("%s gate outcome admitted", name)
		}
	}
	record, raw, err := BuildReadinessRecord(t.Context(), options(map[string]ReadinessEvidence{cemGateRow: {Status: "PASS", Path: ready}}))
	if err != nil {
		t.Fatal(err)
	}
	row := readinessRowByID(record, cemGateRow)
	if row.Status != "PASS" || row.SHA256 != digest([]byte(cemGateOutcome(t, fixture.commit, companionrelease.StableGateReady, ""))) || index(record, cemGateRow) != len(record.Rows)-1 {
		t.Fatalf("gate row = %#v at %d", row, index(record, cemGateRow))
	}
	if _, err := VerifyReadinessRecord(t.Context(), raw, fixture.candidate, map[string]string{cemGateRow: ready}); err != nil {
		t.Fatalf("a READY outcome record was refused at verification: %v", err)
	}
	if _, err := VerifyReadinessRecord(t.Context(), raw, fixture.candidate, nil); err == nil {
		t.Fatal("verification without the gate outcome admitted")
	}
	forged := resealReadiness(t, *record, func(r *ReadinessRecord) {
		r.Rows = append([]ReadinessRow(nil), r.Rows...)
		r.Rows[len(r.Rows)-1].SHA256 = digest([]byte(cemGateOutcome(t, fixture.commit, companionrelease.StableGateRefused, "release-external-incomplete: independent-external-consumer")))
	})
	if _, err := VerifyReadinessRecord(t.Context(), forged, fixture.candidate, map[string]string{cemGateRow: refused}); err == nil || !strings.Contains(err.Error(), "REFUSED") {
		t.Fatalf("a record rebound to a REFUSED outcome: %v", err)
	}
	dropped := resealReadiness(t, *record, func(r *ReadinessRecord) { r.Rows = append([]ReadinessRow(nil), r.Rows[:len(r.Rows)-1]...) })
	if _, err := VerifyReadinessRecord(t.Context(), dropped, fixture.candidate, nil); err == nil {
		t.Fatal("an rc.3 record without the gate row admitted")
	}
	for _, version := range []string{firstStableCandidate, "1.0.0-rc.2"} {
		for _, rule := range readinessRules(version) {
			if rule.id == cemGateRow {
				t.Fatalf("%s catalogue gained the gate row", version)
			}
		}
	}
	if rules := readinessRules("1.0.0"); rules[len(rules)-1].id != cemGateRow {
		t.Fatal("the 1.0.0 catalogue lacks the gate row")
	}
}
