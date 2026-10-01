package criterionexperiment

import (
	"bytes"
	"context"
	"fmt"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/verify"
	cw "github.com/Beamfall/corvint/internal/cem/wire"
	tw "github.com/Beamfall/corvint/internal/tasks/wire"
	"time"
)

type Captures struct {
	Capture      tw.CriterionCapture
	Verification tw.CriterionVerification
}

func (c Captures) records() (tw.CriterionBinding, error) { return c.Verification.Binding, nil }
func authorityBinding(c Captures, tree string) (Binding, []string, tw.CriterionBinding, error) {
	a := c.Verification.Binding
	hashes := make([]string, len(a.AcceptanceCriteria))
	for i, s := range a.AcceptanceCriteria {
		hashes[i] = Digest([]byte(s))
	}
	b := Binding{a.TicketID.Raw, string(a.AcceptanceRevision), CanonicalDigest(hashes), a.AttemptID, string(a.Generation), string(a.PolicySHA256), string(a.ConfigSHA256), tree}
	return b, hashes, a, nil
}
func validateCEM(ctx context.Context, repo *gitauth.Repository, r Request, raw []byte) error {
	m, e := cw.ParseMap(raw)
	if e != nil {
		return e
	}
	if m.Spec != cw.Spec02 {
		return fmt.Errorf("only cem/0.2 supported")
	}
	_, bound, e := verify.Canonical(ctx, repo, m, verify.CanonicalOptions{ExpectedBase: r.Base, Target: r.Target, RawMapBytes: raw})
	if e != nil {
		return e
	}
	if !bound {
		return fmt.Errorf("CEM not canonically bound")
	}
	ids := map[string]bool{}
	for _, h := range m.Hunks {
		ids[h.ID] = true
	}
	for _, c := range r.Criteria {
		for _, h := range c.Hunks {
			if !ids[h] {
				return fmt.Errorf("criterion references unknown CEM hunk")
			}
		}
	}
	return nil
}
func checkLive(ctx context.Context, repoPath string, repo *gitauth.Repository, p Plan, tasks TasksVerifierConfig) (Captures, error) {
	c, e := capture(ctx, repoPath, p.Binding.Ticket, p.Binding.Attempt, tasks)
	if e != nil {
		return c, e
	}
	if c.Capture.Producer != p.TasksVerifier || c.Verification.Verifier != p.TasksVerifier {
		return c, fmt.Errorf("live Tasks identity changed")
	}
	if e = liveBinding(c, p, time.Now()); e != nil {
		return c, e
	}
	if e = repo.RequireCleanTarget(ctx, p.Request.Target); e != nil {
		return c, e
	}
	return c, nil
}

func checkAnchors(ctx context.Context, repo *gitauth.Repository, r Request, captures Captures) error {
	t, e := captures.records()
	if e != nil {
		return e
	}
	if len(t.AcceptanceCriteria) != len(r.Criteria) {
		return fmt.Errorf("complete criteria required")
	}
	for i, c := range r.Criteria {
		b, _, e := blob(ctx, repo, c.Authority.AnchorCommit, c.Authority.AnchorPath)
		if e != nil {
			return e
		}
		if Digest(b) != c.Authority.AnchorSha256 || !bytes.Contains(b, []byte(t.AcceptanceCriteria[i])) {
			return fmt.Errorf("reviewed authority anchor must retain exact accepted criterion")
		}
	}
	return nil
}

func liveBinding(c Captures, p Plan, now time.Time) error {
	b, _, a, e := authorityBinding(c, p.Binding.CandidateTree)
	if e != nil {
		return e
	}
	if a.BaseCommit != p.Request.Base || b != p.Binding || a.CandidateTreeOID == nil || *a.CandidateTreeOID != p.Binding.CandidateTree || (a.Phase != "BUILT" && a.Phase != "CHECKING") {
		return fmt.Errorf("live task/attempt stale or not submitted")
	}
	expiry, e := time.Parse(time.RFC3339Nano, string(a.LeaseExpiresAt))
	if e != nil || !now.Before(expiry) {
		return fmt.Errorf("live lease expired")
	}
	return nil
}
