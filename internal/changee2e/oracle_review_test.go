package changee2e

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeAcceptance(t *testing.T, dir, base, head string, entries ...map[string]string) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{"schema": OracleAcceptanceSchema, "base": base, "head": head, "acceptances": entries})
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "acceptance.json")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// CCE-V0-051 (V1-2413): a CHANGED_IN_DIFF outcome keeps the oracle-self-certified cap unless an
// acceptance document recorded outside the repository names its oracle digest and an acceptor
// other than the head author. Scoring stays false either way.
func TestCCEV0051OracleAcceptance(t *testing.T) {
	root, base := fixtureRepo(t)
	head := commitBehaviourChange(t, root)
	sc := compiledScenario(t, root, base, head)
	sc.OracleOrigin = OriginChangedInDiff
	mismatch := assertionMessage(sc, `"Pending"`)
	sides := map[string]fakeSide{head: {states: repeat("passed")}, base: {states: repeat("failed"), message: mismatch, line: sc.AssertionLine}}
	digest := oracleDigest(sc)
	for _, tc := range []struct {
		name, by, digest string
		want, status     string
	}{
		{"independent acceptance lifts the cap", "Reviewer <reviewer@example.invalid>", digest, VerdictDiscriminates, "accepted"},
		{"acceptance by email alone is matched against the head author", "fixture@example.invalid", digest, VerdictInconclusive, "acceptor-is-head-author"},
		{"acceptance by the head author keeps the cap", "Fixture", digest, VerdictInconclusive, "acceptor-is-head-author"},
		{"stale oracle digest keeps the cap", "Reviewer", "sha256:" + strings.Repeat("1", 64), VerdictInconclusive, "oracle-digest-mismatch"},
		{"no acceptance keeps the cap", "", "", VerdictInconclusive, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var entries []map[string]string
			if tc.by != "" {
				entries = append(entries, map[string]string{"assertion_id": sc.AssertionID, "oracle_digest": tc.digest, "accepted_by": tc.by, "reason": "the booked label is the intended outcome"})
			}
			acc, err := readOracleAcceptance(bg, root, base, head, writeAcceptance(t, t.TempDir(), base, head, entries...))
			if err != nil {
				t.Fatal(err)
			}
			e := (&fakeRunner{sides: sides, controls: "killed"}).env(root)
			e.acceptances = acc
			p := e.proveScenario(bg, base, head, sc, 3)
			if p.Verdict != tc.want || p.Scoring || p.OracleDigest != digest {
				t.Fatalf("verdict %s (%s) scoring %v digest %s, want %s unscored with %s", p.Verdict, p.Note, p.Scoring, p.OracleDigest, tc.want, digest)
			}
			if tc.status == "" {
				if p.OracleAcceptance != nil || !strings.HasPrefix(p.Note, ReasonOracleSelfCertified) {
					t.Fatalf("acceptance %+v note %q", p.OracleAcceptance, p.Note)
				}
				return
			}
			if a := p.OracleAcceptance; a == nil || a.Status != tc.status || !strings.HasPrefix(a.DocumentDigest, "sha256:") {
				t.Fatalf("acceptance %+v, want status %s", a, tc.status)
			}
			wantNote := ReasonOracleAccepted + " by " + tc.by
			if tc.status != "accepted" {
				wantNote = "acceptance not applied: " + tc.status
			}
			if !strings.Contains(p.Note, wantNote) {
				t.Fatalf("note %q, want %q", p.Note, wantNote)
			}
		})
	}

	// An admitted-at-base scenario ignores an acceptance: it is never capped and never lifted.
	admitted := sc
	admitted.OracleOrigin = OriginAdmittedAtBase
	acc, err := readOracleAcceptance(bg, root, base, head, writeAcceptance(t, t.TempDir(), base, head,
		map[string]string{"assertion_id": sc.AssertionID, "oracle_digest": digest, "accepted_by": "Reviewer", "reason": "r"}))
	if err != nil {
		t.Fatal(err)
	}
	e := (&fakeRunner{sides: sides, controls: "killed"}).env(root)
	e.acceptances = acc
	if p := e.proveScenario(bg, base, head, admitted, 3); p.OracleAcceptance != nil || p.OracleDigest != "" || !p.Scoring {
		t.Fatalf("admitted scenario: acceptance %+v digest %q scoring %v", p.OracleAcceptance, p.OracleDigest, p.Scoring)
	}
}

// CCE-V0-051: the acceptance document is refused inside the repository, with unknown members, a
// wrong schema or revisions, or an incomplete or duplicate entry.
func TestCCEV0051OracleAcceptanceRefusals(t *testing.T) {
	root, base := fixtureRepo(t)
	head := commitBehaviourChange(t, root)
	ok := map[string]string{"assertion_id": "a", "oracle_digest": "sha256:00", "accepted_by": "R", "reason": "r"}
	outside := t.TempDir()
	write := func(body string) string {
		p := filepath.Join(outside, "doc.json")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	for name, path := range map[string]string{
		"inside the repository": writeAcceptance(t, root, base, head, ok),
		"wrong base":            writeAcceptance(t, t.TempDir(), head, head, ok),
		"wrong schema":          write(`{"schema":"e2e-oracle-acceptance/9","base":"` + base + `","head":"` + head + `","acceptances":[]}`),
		"unknown member":        write(`{"schema":"` + OracleAcceptanceSchema + `","base":"` + base + `","head":"` + head + `","acceptances":[],"extra":1}`),
		"trailing document":     write(`{"schema":"` + OracleAcceptanceSchema + `","base":"` + base + `","head":"` + head + `","acceptances":[]} {}`),
		"incomplete entry":      writeAcceptance(t, t.TempDir(), base, head, map[string]string{"assertion_id": "a", "oracle_digest": "sha256:00", "accepted_by": "R"}),
		"duplicate entry":       writeAcceptance(t, t.TempDir(), base, head, ok, ok),
		"missing file":          filepath.Join(outside, "absent.json"),
	} {
		var r *Refusal
		if _, err := readOracleAcceptance(bg, root, base, head, path); !errors.As(err, &r) || r.Code != "e2e-oracle-acceptance-invalid" {
			t.Fatalf("%s: %v, want e2e-oracle-acceptance-invalid", name, err)
		}
	}
	// Prove refuses before any run when the document is invalid.
	var r *Refusal
	_, err := Prove(bg, ProveOptions{Root: root, Base: base, Head: head, Scenarios: []Scenario{compiledScenario(t, root, base, head)},
		OracleAcceptance: writeAcceptance(t, root, base, head, ok)})
	if !errors.As(err, &r) || r.Code != "e2e-oracle-acceptance-invalid" {
		t.Fatalf("Prove: %v, want e2e-oracle-acceptance-invalid", err)
	}
}

// CCE-V0-052 (V1-2413): an outcome locator that does not resolve at head gets a proposed
// replacement read from a data-testid rename in the diff, with the line pair as evidence. The
// proposal is never applied: the verdict is the ladder's and the scenario is unchanged.
func TestCCEV0052LocatorRepairProposal(t *testing.T) {
	root, base := fixtureRepo(t)
	behaviour := overlayText(t, "app/components/teesheet.head.html")
	renamed := strings.Replace(*behaviour, `data-testid="slot-status"`, `data-testid="slot-state"`, 1)
	if renamed == *behaviour {
		t.Fatal("the fixture has no slot-status test id")
	}
	booked := commitBehaviourChange(t, root)
	sc := compiledScenario(t, root, base, booked)
	head := commitFiles(t, root, map[string]*string{"app/components/teesheet.html": &renamed}, "rename")
	if sc.Locator != "slot-status" {
		t.Fatalf("scenario locator %q", sc.Locator)
	}
	unresolved := assertionMessage(sc, "<element(s) not found>")
	f := &fakeRunner{sides: map[string]fakeSide{head: {states: repeat("failed"), message: unresolved, line: sc.AssertionLine}, base: {states: repeat("passed")}}, controls: "killed"}
	p := f.env(root).proveScenario(bg, base, head, sc, 3)
	r := p.LocatorRepair
	if r == nil || r.Proposed != "slot-state" || r.Applied || r.Unproposed != "" || len(r.Evidence) != 1 {
		t.Fatalf("repair %+v", r)
	}
	if ev := r.Evidence[0]; ev.Path != "app/components/teesheet.html" || ev.BaseLine != 7 || ev.HeadLine != 7 {
		t.Fatalf("evidence %+v", ev)
	}
	if p.Verdict != VerdictRegressionDetected || sc.Locator != "slot-status" {
		t.Fatalf("verdict %s locator %s: the repair must not be applied", p.Verdict, sc.Locator)
	}

	// A value mismatch at head proposes nothing.
	f.sides[head] = fakeSide{states: repeat("failed"), message: assertionMessage(sc, `"Pending"`), line: sc.AssertionLine}
	if p := f.env(root).proveScenario(bg, base, head, sc, 3); p.LocatorRepair != nil {
		t.Fatalf("value mismatch proposed %+v", p.LocatorRepair)
	}

	// Two different renames of the locator are ambiguous; a diff without one proposes nothing.
	other := "<p data-testid=\"slot-status\">x</p>\n"
	b2 := commitFiles(t, root, map[string]*string{"app/components/teesheet.html": behaviour, "app/components/other.html": &other}, "second use")
	otherRenamed := "<p data-testid='slot-label'>x</p>\n"
	h2 := commitFiles(t, root, map[string]*string{"app/components/teesheet.html": &renamed, "app/components/other.html": &otherRenamed}, "rename both uses")
	if r := proposeLocatorRepair(bg, root, b2, h2, "slot-status"); r.Unproposed != "repair-ambiguous" || r.Proposed != "" || len(r.Evidence) != 2 || r.Applied {
		t.Fatalf("ambiguous repair %+v", r)
	}
	if r := proposeLocatorRepair(bg, root, base, booked, "slot-status"); r.Unproposed != "no-rename-in-diff" || r.Proposed != "" {
		t.Fatalf("no-rename repair %+v", r)
	}
}
