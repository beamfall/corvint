package criterionexperiment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/verify"
	cw "github.com/Beamfall/corvint/internal/cem/wire"
	taskscli "github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	tw "github.com/Beamfall/corvint/internal/tasks/wire"
)

type Captures struct {
	Policy  string `json:"policy"`
	Ticket  string `json:"ticket"`
	Queue   string `json:"queue"`
	Attempt string `json:"attempt"`
}
type envelope struct {
	Profile  string            `json:"profile"`
	Outcome  string            `json:"outcome"`
	Snapshot json.RawMessage   `json:"snapshot"`
	Items    []json.RawMessage `json:"items"`
}
type snapIdentity struct {
	HeadSeq     *string         `json:"headSeq"`
	HeadReceipt *string         `json:"headReceiptSha256"`
	IntentTree  *string         `json:"intentTreeSha256"`
	Primary     *string         `json:"primaryWorktreeSha256"`
	Pending     bool            `json:"pendingRedo"`
	Barrier     json.RawMessage `json:"barrier"`
}

func parseEnvelope(raw string) (envelope, error) {
	var e envelope
	if len(raw) > MaxDocument {
		return e, fmt.Errorf("native read too large")
	}
	if _, err := tw.DecodeResult([]byte(raw)); err != nil {
		return e, err
	}
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		return e, err
	}
	if e.Profile != "taskman-command-result/0" || e.Outcome != "OK" || len(e.Items) != 1 {
		return e, fmt.Errorf("native read not OK")
	}
	var s snapIdentity
	if err := json.Unmarshal(e.Snapshot, &s); err != nil {
		return e, err
	}
	if s.HeadSeq == nil || s.HeadReceipt == nil || s.IntentTree == nil || s.Primary == nil || !cw.IsSha256(*s.HeadReceipt) || !cw.IsSha256(*s.IntentTree) || !cw.IsSha256(*s.Primary) || s.Pending || string(s.Barrier) != "null" {
		return e, fmt.Errorf("native snapshot incomplete or blocked")
	}
	return e, nil
}
func capture(repo, ticketID, attemptID string) (Captures, error) {
	var last error
	for tries := 0; tries < 3; tries++ {
		outputs := make([]string, 3)
		for i, args := range [][]string{{"ticket", "show", ticketID}, {"queue", "status"}, {"attempt", "show", attemptID}} {
			var b bytes.Buffer
			if taskscli.Run(taskscli.Env{Cwd: repo, Args: args, Stdout: &b}) != 0 {
				return Captures{}, fmt.Errorf("native read refused: %s", args[0])
			}
			if b.Len() > MaxDocument {
				return Captures{}, fmt.Errorf("native capture bound")
			}
			outputs[i] = b.String()
		}
		queueEnvelope, err := parseEnvelope(outputs[1])
		if err != nil {
			last = err
			continue
		}
		var observed snapIdentity
		if err = json.Unmarshal(queueEnvelope.Snapshot, &observed); err != nil {
			last = err
			continue
		}
		resolved, err := intent.Resolve(repo)
		if err != nil {
			return Captures{}, err
		}
		loaded, err := intent.LoadExpecting(resolved.PrimaryWorktree, tw.Digest(*observed.IntentTree))
		if err != nil {
			last = err
			continue
		}
		c := Captures{Ticket: outputs[0], Queue: outputs[1], Attempt: outputs[2], Policy: string(loaded.Policy.Raw)}
		if _, _, _, err := c.records(); err == nil {
			return c, nil
		} else {
			last = err
		}
	}
	return Captures{}, fmt.Errorf("native coherent snapshot unavailable: %w", last)
}
func (c Captures) records() (*ticket.Record, *snapshot.Attempt, string, error) {
	es := make([]envelope, 3)
	for i, raw := range []string{c.Ticket, c.Queue, c.Attempt} {
		e, err := parseEnvelope(raw)
		if err != nil {
			return nil, nil, "", err
		}
		es[i] = e
	}
	var first snapIdentity
	_ = json.Unmarshal(es[0].Snapshot, &first)
	for _, e := range es[1:] {
		var s snapIdentity
		_ = json.Unmarshal(e.Snapshot, &s)
		if !bytes.Equal(Encode(first), Encode(s)) {
			return nil, nil, "", fmt.Errorf("native snapshot moved")
		}
	}
	var item struct {
		Record json.RawMessage `json:"record"`
	}
	if err := json.Unmarshal(es[0].Items[0], &item); err != nil {
		return nil, nil, "", err
	}
	t, err := ticket.Decode(append(append([]byte{}, item.Record...), '\n'))
	if err != nil {
		return nil, nil, "", err
	}
	a, err := snapshot.DecodeAttempt(append(append([]byte{}, es[2].Items[0]...), '\n'))
	if err != nil {
		return nil, nil, "", err
	}
	var q struct {
		Policy       string          `json:"policySha256"`
		Barrier      json.RawMessage `json:"barrier"`
		WriteBarrier string          `json:"writeBarrier"`
		Cutover      bool            `json:"executionCutover"`
	}
	if err = json.Unmarshal(es[1].Items[0], &q); err != nil {
		return nil, nil, "", err
	}
	policy, err := intent.DecodePolicy([]byte(c.Policy))
	if err != nil {
		return nil, nil, "", err
	}
	if q.WriteBarrier != "NONE" || string(q.Barrier) != "null" || !q.Cutover || q.Policy != string(policy.PolicySha256()) || a.PolicySha256 != tw.Sum(policy.Raw) {
		return nil, nil, "", fmt.Errorf("queue policy blocked or changed")
	}
	if a.RuntimeID != snapshot.RuntimeExternalAgent || a.ConfigSha256 != a.PolicySha256 || a.TicketRevision != t.AcceptanceRevision {
		return nil, nil, "", fmt.Errorf("external attempt configuration/acceptance binding mismatch")
	}
	if a.TicketID.Raw != t.TicketID.Raw {
		return nil, nil, "", fmt.Errorf("attempt ticket mismatch")
	}
	return t, a, string(a.PolicySha256), nil
}
func authorityBinding(c Captures, tree string) (Binding, []string, *snapshot.Attempt, error) {
	t, a, policy, e := c.records()
	if e != nil {
		return Binding{}, nil, nil, e
	}
	hashes := make([]string, len(t.AcceptanceCriteria))
	for i, s := range t.AcceptanceCriteria {
		hashes[i] = Digest([]byte(s))
	}
	b := Binding{t.TicketID.Raw, string(t.AcceptanceRevision), CanonicalDigest(hashes), a.AttemptID, string(a.Generation), policy, string(a.ConfigSha256), tree}
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
func checkLive(ctx context.Context, repoPath string, repo *gitauth.Repository, p Plan) (Captures, error) {
	c, e := capture(repoPath, p.Binding.Ticket, p.Binding.Attempt)
	if e != nil {
		return c, e
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
	t, _, _, e := captures.records()
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
	if a.BaseCommit != p.Request.Base || b != p.Binding || a.CandidateTreeOid == nil || *a.CandidateTreeOid != p.Binding.CandidateTree || (a.Phase != "BUILT" && a.Phase != "CHECKING") || a.Lease == nil {
		return fmt.Errorf("live task/attempt stale or not submitted")
	}
	expiry, e := time.Parse(time.RFC3339Nano, string(a.Lease.ExpiresAt))
	if e != nil || !now.Before(expiry) {
		return fmt.Errorf("live lease expired")
	}
	return nil
}
