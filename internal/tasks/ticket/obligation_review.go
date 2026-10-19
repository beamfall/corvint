package ticket

// Reviewer-verdict witnesses (TOL-V0-073..078, GitHub #1170). A REVIEWED
// witness credits one obligation on K independent reviewer verdicts under an
// agreement policy. The outcome is a pure function of the payload's review,
// so the writer, every fold and the audit agree on it.
import (
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const (
	ObligationSourceReviewed = "REVIEWED"

	ObligationReviewCredited = "CREDITED"
	ObligationReviewHeld     = "HELD"

	VerdictCorrect = "correct"
	VerdictPartial = "partial"
	VerdictWrong   = "wrong"

	StubRiskNone     = "none"
	StubRiskBoundary = "boundary"
	StubRiskOnPath   = "on-path"

	MaxObligationReviewers     = 16
	MaxObligationReviewAuthors = 64

	// ObligationReviewerNotIndependentDetail prefixes the refusal of a
	// verdict whose reviewer is one of the ticket's derived authors
	// (TOL-V0-074).
	ObligationReviewerNotIndependentDetail = "OBLIGATION_REVIEWER_NOT_INDEPENDENT:"
)

var stubRiskRank = map[string]int{StubRiskNone: 0, StubRiskBoundary: 1, StubRiskOnPath: 2}

// ObligationVerdict is one reviewer's verdict on the cited example.
type ObligationVerdict struct {
	Reviewer string
	Verdict  string
	StubRisk string
	Reason   string
	// Finding is the optional TOL-V0-085 finding, "" when absent.
	Finding string
}

// ObligationReviewSubject is the cited example: a repository path, a
// 1-based line range and the sha256 of the path's blob at the witness commit.
type ObligationReviewSubject struct {
	Path       string
	StartLine  int64
	EndLine    int64
	BlobSha256 wire.Digest
}

// ObligationReview is the REVIEWED payload member (TOL-V0-074).
type ObligationReview struct {
	Reviewers   int64
	MaxStubRisk string
	Subject     ObligationReviewSubject
	Authors     []string
	Verdicts    []ObligationVerdict
}

func (r ObligationReview) Value() wire.Value {
	authors := make([]wire.Value, 0, len(r.Authors))
	for _, a := range r.Authors {
		authors = append(authors, wire.String(a))
	}
	verdicts := make([]wire.Value, 0, len(r.Verdicts))
	for _, v := range r.Verdicts {
		vv := noteObject("reviewer", wire.String(v.Reviewer), "verdict", wire.String(v.Verdict),
			"stubRisk", wire.String(v.StubRisk), "reason", wire.String(v.Reason))
		if v.Finding != "" {
			vv.Obj.Set("finding", wire.String(v.Finding))
		}
		verdicts = append(verdicts, vv)
	}
	return noteObject(
		"policy", noteObject("reviewers", wire.String(string(wire.CountOf(r.Reviewers))), "maxStubRisk", wire.String(r.MaxStubRisk)),
		"subject", noteObject("path", wire.String(r.Subject.Path), "startLine", wire.String(string(wire.CountOf(r.Subject.StartLine))),
			"endLine", wire.String(string(wire.CountOf(r.Subject.EndLine))), "blobSha256", wire.String(string(r.Subject.BlobSha256))),
		"authors", wire.Array(authors...), "verdicts", wire.Array(verdicts...))
}

// ReadObligationReview reads the closed review member and refuses a
// reviewer who is a listed author.
func ReadObligationReview(r *wire.Reader) *ObligationReview {
	r.Closed("policy", "subject", "authors", "verdicts")
	out := &ObligationReview{}
	pol := r.Field("policy")
	pol.Closed("reviewers", "maxStubRisk")
	out.Reviewers = pol.Field("reviewers").Count().Int()
	out.MaxStubRisk = pol.Field("maxStubRisk").Enum(StubRiskNone, StubRiskBoundary)
	if pol.Err() == nil && (out.Reviewers < 1 || out.Reviewers > MaxObligationReviewers) {
		pol.Field("reviewers").Fail(wire.CodeMalformed, "reviewers is 1..%d", MaxObligationReviewers)
	}
	sub := r.Field("subject")
	sub.Closed("path", "startLine", "endLine", "blobSha256")
	out.Subject = ObligationReviewSubject{Path: sub.Field("path").Path(), StartLine: sub.Field("startLine").Count().Int(),
		EndLine: sub.Field("endLine").Count().Int(), BlobSha256: sub.Field("blobSha256").Digest()}
	if sub.Err() == nil && (out.Subject.StartLine < 1 || out.Subject.EndLine < out.Subject.StartLine) {
		sub.Fail(wire.CodeMalformed, "subject lines are 1-based with startLine <= endLine")
	}
	out.Authors = r.Field("authors").Strings(MaxObligationReviewAuthors, false, (*wire.Reader).Label)
	items := r.Field("verdicts").Array(MaxObligationReviewers, true)
	if r.Err() != nil {
		return nil
	}
	if len(items) == 0 {
		r.Field("verdicts").Fail(wire.CodeMalformed, "verdicts needs at least one entry")
		return nil
	}
	authors := map[string]bool{}
	for _, a := range out.Authors {
		authors[a] = true
	}
	prev := ""
	for i, it := range items {
		it.Closed(wire.OptionalKeys(it.Value(), []string{"reviewer", "verdict", "stubRisk", "reason"}, "finding")...)
		v := ObligationVerdict{Reviewer: it.Field("reviewer").Label(), Verdict: it.Field("verdict").Enum(VerdictCorrect, VerdictPartial, VerdictWrong),
			StubRisk: it.Field("stubRisk").Enum(StubRiskNone, StubRiskBoundary, StubRiskOnPath), Reason: readNonblank(it.Field("reason"), MaxObligationReasonBytes)}
		if wire.Has(it.Value(), "finding") {
			v.Finding = it.Field("finding").Enum(VerdictFindingNoFalsifiableText)
		}
		if it.Err() == nil && v.Finding != "" && v.Verdict == VerdictCorrect {
			it.Field("finding").Fail(wire.CodeMalformed, "a %s finding needs a partial or wrong verdict", v.Finding)
		}
		if it.Err() != nil {
			return nil
		}
		if i > 0 && v.Reviewer <= prev {
			it.Field("reviewer").Fail(wire.CodeMalformed, "verdicts are sorted unique by reviewer")
			return nil
		}
		if authors[v.Reviewer] {
			it.Field("reviewer").Fail(wire.CodeMalformed, "%s reviewer %s is an author of the ticket", ObligationReviewerNotIndependentDetail, v.Reviewer)
			return nil
		}
		prev = v.Reviewer
		out.Verdicts = append(out.Verdicts, v)
	}
	return out
}

// Outcome is the TOL-V0-075 pure evaluation: CREDITED when at least K
// verdicts are given, all correct, with no stub risk above the policy's;
// otherwise HELD with the sorted failing axes.
func (r ObligationReview) Outcome() (string, []string) {
	var axes []string
	if int64(len(r.Verdicts)) < r.Reviewers {
		axes = append(axes, "reviewers")
	}
	stub, verdict := false, false
	for _, v := range r.Verdicts {
		if stubRiskRank[v.StubRisk] > stubRiskRank[r.MaxStubRisk] {
			stub = true
		}
		if v.Verdict != VerdictCorrect {
			verdict = true
		}
	}
	if stub {
		axes = append(axes, "stubRisk")
	}
	if verdict {
		axes = append(axes, "verdict")
	}
	sort.Strings(axes)
	if len(axes) == 0 {
		return ObligationReviewCredited, []string{}
	}
	return ObligationReviewHeld, axes
}

// ObligationReviewRecord is an entry's optional review member (TOL-V0-075):
// the latest reviewed witness's outcome and its whole review.
// TestRepository names the policy-declared repository the subject was read
// from, so staleness is judged in that repository and not the store's.
type ObligationReviewRecord struct {
	Outcome        string
	Axes           []string
	EventSha256    wire.Digest
	Commit         string
	Review         ObligationReview
	TestRepository *string
}

func (r ObligationReviewRecord) Value() wire.Value {
	axes := make([]wire.Value, 0, len(r.Axes))
	for _, a := range r.Axes {
		axes = append(axes, wire.String(a))
	}
	v := noteObject("outcome", wire.String(r.Outcome), "axes", wire.Array(axes...), "eventSha256", wire.String(string(r.EventSha256)),
		"commit", wire.String(r.Commit), "review", r.Review.Value())
	if r.TestRepository != nil {
		v.Obj.Set("testRepository", wire.String(*r.TestRepository))
	}
	return v
}

func readObligationReviewRecord(r *wire.Reader) *ObligationReviewRecord {
	r.Closed(wire.OptionalKeys(r.Value(), []string{"outcome", "axes", "eventSha256", "commit", "review"}, "testRepository")...)
	out := &ObligationReviewRecord{Outcome: r.Field("outcome").Enum(ObligationReviewCredited, ObligationReviewHeld),
		Axes:        r.Field("axes").Strings(3, false, func(x *wire.Reader) string { return x.Enum("reviewers", "stubRisk", "verdict") }),
		EventSha256: r.Field("eventSha256").Digest(), Commit: r.Field("commit").OID()}
	rv := ReadObligationReview(r.Field("review"))
	if r.Err() != nil || rv == nil {
		return nil
	}
	out.Review = *rv
	if wire.Has(r.Value(), "testRepository") {
		name := r.Field("testRepository").Label()
		if r.Err() != nil {
			return nil
		}
		out.TestRepository = &name
	}
	if outcome, axes := rv.Outcome(); outcome != out.Outcome || strings.Join(axes, ",") != strings.Join(out.Axes, ",") {
		r.Fail(wire.CodeMalformed, "review outcome does not match its verdicts")
		return nil
	}
	return out
}

// applyReviewedWitness folds one REVIEWED witness onto entry e (TOL-V0-075,
// 076).
func applyReviewedWitness(e *ObligationEntry, w *ObligationWitnessPayload, eventSha256 wire.Digest) error {
	if e.State == ObligationDeferred || (e.State == ObligationWitnessed && e.Evidence != nil && e.Evidence.Source != ObligationSourceReviewed) {
		return wire.Errorf(wire.CodeMalformed, "/request/payload/credits", "%s obligation %s is %s and is not credited by review", ObligationCreditMismatchDetail, e.ID, e.State)
	}
	if e.State == ObligationWitnessed && e.Review != nil && e.Review.Review.Subject.BlobSha256 == w.Review.Subject.BlobSha256 {
		return wire.Errorf(wire.CodeMalformed, "/request/payload/review/subject", "%s obligation %s is already witnessed by review of this subject blob", ObligationCreditMismatchDetail, e.ID)
	}
	outcome, axes := w.Review.Outcome()
	e.Review = &ObligationReviewRecord{Outcome: outcome, Axes: axes, EventSha256: eventSha256, Commit: w.Commit, Review: *w.Review, TestRepository: w.TestRepository}
	switch {
	case outcome == ObligationReviewCredited:
		e.State = ObligationWitnessed
		e.Reason = nil
		e.Evidence = &ObligationEvidence{Source: ObligationSourceReviewed, EventSha256: eventSha256, Commit: w.Commit, Provenance: w.Provenance, TestRepository: w.TestRepository}
	case e.State == ObligationWitnessed:
		e.State = ObligationOpen
		e.Evidence = nil
	}
	return nil
}
