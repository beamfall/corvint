package ticket

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func sampleReview(k int64, maxStub string, verdicts ...ObligationVerdict) ObligationReview {
	return ObligationReview{Reviewers: k, MaxStubRisk: maxStub, Authors: []string{"author-1"},
		Subject:  ObligationReviewSubject{Path: "e2e/login.spec.ts", StartLine: 3, EndLine: 9, BlobSha256: wire.Digest(strings.Repeat("a", 64))},
		Verdicts: verdicts}
}

func readReview(v wire.Value) (*ObligationReview, error) {
	r := wire.NewReader(v, "/review")
	out := ReadObligationReview(r)
	return out, r.Err()
}

// TestTOLV0074_ReviewCodec: the review member round-trips canonically; an
// author reviewer, unsorted verdicts, an out-of-range K, an on-path policy
// and a blank reason are refused.
func TestTOLV0074_ReviewCodec(t *testing.T) {
	rv := sampleReview(2, StubRiskBoundary, ObligationVerdict{"rev-a", VerdictCorrect, StubRiskNone, "ok", ""}, ObligationVerdict{"rev-b", VerdictPartial, StubRiskOnPath, "stubbed", ""})
	got, err := readReview(rv.Value())
	if err != nil || !wire.Equal(got.Value(), rv.Value()) {
		t.Fatalf("round trip: %v %s", err, wire.Encode(rv.Value()))
	}
	bad := map[string]ObligationReview{
		ObligationReviewerNotIndependentDetail: sampleReview(1, StubRiskNone, ObligationVerdict{"author-1", VerdictCorrect, StubRiskNone, "self", ""}),
		"sorted unique":                        sampleReview(1, StubRiskNone, ObligationVerdict{"rev-b", VerdictCorrect, StubRiskNone, "x", ""}, ObligationVerdict{"rev-a", VerdictCorrect, StubRiskNone, "y", ""}),
		"reviewers is 1..16":                   sampleReview(17, StubRiskNone, ObligationVerdict{"rev-a", VerdictCorrect, StubRiskNone, "x", ""}),
		"maxStubRisk":                          sampleReview(1, StubRiskOnPath, ObligationVerdict{"rev-a", VerdictCorrect, StubRiskNone, "x", ""}),
		"non-whitespace":                       sampleReview(1, StubRiskNone, ObligationVerdict{"rev-a", VerdictCorrect, StubRiskNone, "   ", ""}),
	}
	for want, rv := range bad {
		if _, err := readReview(rv.Value()); err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s: got %v", want, err)
		}
	}
}

// TestTOLV0075_ReviewOutcome: the outcome is CREDITED only for K correct
// verdicts within the stub-risk policy; otherwise HELD with sorted axes.
func TestTOLV0075_ReviewOutcome(t *testing.T) {
	ok := ObligationVerdict{"rev-a", VerdictCorrect, StubRiskNone, "ok", ""}
	ok2 := ObligationVerdict{"rev-b", VerdictCorrect, StubRiskBoundary, "ok", ""}
	cases := []struct {
		rv   ObligationReview
		want string
	}{
		{sampleReview(2, StubRiskBoundary, ok, ok2), "CREDITED:"},
		{sampleReview(2, StubRiskNone, ok, ok2), "HELD:stubRisk"},
		{sampleReview(3, StubRiskBoundary, ok, ok2), "HELD:reviewers"},
		{sampleReview(2, StubRiskBoundary, ok, ObligationVerdict{"rev-b", VerdictPartial, StubRiskOnPath, "x", ""}), "HELD:stubRisk,verdict"},
		{sampleReview(1, StubRiskBoundary, ObligationVerdict{"rev-a", VerdictWrong, StubRiskNone, "x", ""}), "HELD:verdict"},
	}
	for i, c := range cases {
		outcome, axes := c.rv.Outcome()
		if got := outcome + ":" + strings.Join(axes, ","); got != c.want {
			t.Fatalf("case %d: got %s want %s", i, got, c.want)
		}
	}
}

// TestTOLV0076_ReviewFold: a credited review witnesses the entry; a HELD
// review of a review-witnessed entry returns it to OPEN; DEFERRED, another
// source's witness and a same-blob re-review are refused; a changed blob
// admits a fresh review.
func TestTOLV0076_ReviewFold(t *testing.T) {
	ok := ObligationVerdict{"rev-a", VerdictCorrect, StubRiskNone, "ok", ""}
	credited := sampleReview(1, StubRiskNone, ok)
	w := &ObligationWitnessPayload{Source: ObligationSourceReviewed, Commit: strings.Repeat("1", 40), Review: &credited}
	e := &ObligationEntry{ID: "AC-1", State: ObligationOpen}
	if err := applyReviewedWitness(e, w, wire.Digest(strings.Repeat("e", 64))); err != nil || e.State != ObligationWitnessed || e.Evidence.Source != ObligationSourceReviewed {
		t.Fatalf("credit: %v %+v", err, e)
	}
	if err := applyReviewedWitness(e, w, wire.Digest(strings.Repeat("f", 64))); err == nil || !strings.Contains(err.Error(), ObligationCreditMismatchDetail) {
		t.Fatalf("same-blob re-review: %v", err)
	}
	held := sampleReview(1, StubRiskNone, ObligationVerdict{"rev-a", VerdictPartial, StubRiskNone, "partial", ""})
	held.Subject.BlobSha256 = wire.Digest(strings.Repeat("b", 64))
	if err := applyReviewedWitness(e, &ObligationWitnessPayload{Source: ObligationSourceReviewed, Commit: w.Commit, Review: &held}, wire.Digest(strings.Repeat("c", 64))); err != nil ||
		e.State != ObligationOpen || e.Evidence != nil || e.Review.Outcome != ObligationReviewHeld {
		t.Fatalf("held after credit: %v %+v", err, e)
	}
	for _, pre := range []*ObligationEntry{{ID: "AC-2", State: ObligationDeferred},
		{ID: "AC-3", State: ObligationWitnessed, Evidence: &ObligationEvidence{Source: ObligationSourceDeclared}}} {
		if err := applyReviewedWitness(pre, w, wire.Digest(strings.Repeat("d", 64))); err == nil {
			t.Fatalf("%s %s admitted", pre.ID, pre.State)
		}
	}
	entry := ObligationEntry{ID: "AC-1", Title: "t", State: e.State, Review: e.Review, UpdatedAt: "2026-10-18T00:00:00Z", UpdatedByID: "owner", UpdatedByRole: "OWNER"}
	r := wire.NewReader(entry.Value(), "/entry")
	if back := ReadObligationEntry(r); r.Err() != nil || !wire.Equal(back.Value(), entry.Value()) {
		t.Fatalf("entry review round trip: %v", r.Err())
	}
	// The review record keeps the test repository the subject was read
	// from, so staleness is judged there (Codex #1189 finding 1).
	app := "app"
	moved := sampleReview(1, StubRiskNone, ok)
	moved.Subject.BlobSha256 = wire.Digest(strings.Repeat("9", 64))
	if err := applyReviewedWitness(e, &ObligationWitnessPayload{Source: ObligationSourceReviewed, Commit: w.Commit, Review: &moved, TestRepository: &app},
		wire.Digest(strings.Repeat("8", 64))); err != nil || e.Review.TestRepository == nil || *e.Review.TestRepository != app {
		t.Fatalf("review test repository: %v %+v", err, e.Review)
	}
	entry.State, entry.Review, entry.Evidence = e.State, e.Review, e.Evidence
	r = wire.NewReader(entry.Value(), "/entry")
	if back := ReadObligationEntry(r); r.Err() != nil || back.Review.TestRepository == nil || !wire.Equal(back.Value(), entry.Value()) {
		t.Fatalf("entry review test-repository round trip: %v", r.Err())
	}
}
