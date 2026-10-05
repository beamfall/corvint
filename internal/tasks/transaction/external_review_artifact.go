package transaction

import (
	"encoding/json"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ExternalArtifact is one admitted, journal-backed artifact link
// (ERG-V0-002): an evidence entry {sha256,bytes} of an executable gate
// result that a receipt posted fresh under evidence/<digest> and listed in
// the gate results of the attempt record the same receipt posted, where the
// gate result names that attempt, its generation and its candidate tree.
// planGateRun admits a gate result only after hashing its output against
// the entry, so the link proves the artifact was produced at that subject
// generation and tree and retained in the journal. A caller-supplied path or
// digest is never a link.
type ExternalArtifact struct {
	TicketID, AttemptID string
	Generation          wire.Size
	Tree                string
	Sha256              wire.Digest
	Bytes               wire.Size
}

// ExternalArtifactPosts returns the artifact links rc posts. Linking only
// admits, so an attempt post, gate result or evidence entry that cannot be
// read or decoded contributes no link rather than an error: a missing link
// refuses an EVIDENCE candidate, it never grants one. Only gate results this
// receipt posts fresh are read, so the cost is O(posts + fresh results).
func ExternalArtifactPosts(rc *snapshot.Receipt, blob ExternalReviewBlob) []ExternalArtifact {
	fresh := map[wire.Digest]bool{}
	for _, p := range rc.Post {
		if strings.HasPrefix(p.Path, "evidence/") && p.Sha256 != nil && p.Path == "evidence/"+string(*p.Sha256) {
			fresh[*p.Sha256] = true
		}
	}
	if len(fresh) == 0 || blob == nil {
		return nil
	}
	var out []ExternalArtifact
	for _, p := range rc.Post {
		if !strings.HasPrefix(p.Path, "attempts/") || p.Sha256 == nil {
			continue
		}
		raw, err := externalPostBytes(p, blob)
		if err != nil {
			continue
		}
		a, err := snapshot.DecodeAttempt(raw)
		if err != nil || a.CandidateTreeOid == nil {
			continue
		}
		for _, d := range a.GateResults {
			if !fresh[wire.Digest(d)] {
				continue
			}
			rec, ok := blob(wire.Digest(d))
			if !ok || wire.Sum(rec) != wire.Digest(d) {
				continue
			}
			g, err := snapshot.DecodeGateResult(rec)
			if err != nil || g.AttemptID != a.AttemptID || g.Generation != a.Generation || g.CandidateTreeOid != *a.CandidateTreeOid {
				continue
			}
			for _, e := range g.Evidence {
				out = append(out, ExternalArtifact{TicketID: a.TicketID.Raw, AttemptID: a.AttemptID, Generation: a.Generation, Tree: g.CandidateTreeOid, Sha256: e.Sha256, Bytes: e.Bytes})
			}
		}
	}
	return out
}

// externalLinked reports whether links hold an artifact {sha,bytes} of the
// subject attempt and generation at tree.
func externalLinked(links []ExternalArtifact, ticketID string, s snapshot.ExternalReviewSubject, tree string, sha wire.Digest, n wire.Size) bool {
	for _, l := range links {
		if l.TicketID == ticketID && l.AttemptID == s.AttemptID && l.Generation == s.Generation && l.Tree == tree && l.Sha256 == sha && l.Bytes == n {
			return true
		}
	}
	return false
}

// Acceptance report schemas of the issue 394 producer (`corvint tests
// accept`, internal/testacceptance). They are restated here so Tasks does
// not import the browser-executing producer.
const (
	acceptanceSchema      = "corvint-new-e2e-assessment/0"
	acceptanceFreshSchema = "corvint-new-e2e-assessment/1"
)

// ExternalAcceptanceVerdict maps an EVIDENCE artifact that is an issue 394
// acceptance report to the verdict it supports (owner decision 2026-10-04):
// accepted to PASS, rejected to RETURN, blocked to no verdict (""). report
// is false for any artifact that is not such a report, which then implies
// no verdict mapping. A report with another verdict is an error.
func ExternalAcceptanceVerdict(raw []byte) (verdict string, report bool, err error) {
	var r struct {
		Schema  string `json:"schema"`
		Verdict string `json:"verdict"`
	}
	if json.Unmarshal(raw, &r) != nil || (r.Schema != acceptanceSchema && r.Schema != acceptanceFreshSchema) {
		return "", false, nil
	}
	switch r.Verdict {
	case "accepted":
		return "PASS", true, nil
	case "rejected":
		return "RETURN", true, nil
	case "blocked":
		return "", true, nil
	}
	return "", true, wire.Errorf(wire.CodeMalformed, "/verdict", "acceptance report verdict %q is not accepted, rejected or blocked", r.Verdict)
}

// externalAcceptanceCheck enforces the issue 394 mapping on a RECORD whose
// EVIDENCE candidate artifact is an acceptance report: a blocked report
// supports no verdict (blocked is true), and any other report supports only
// its mapped one. detail is "" when the request agrees or the artifact is
// not a report.
func externalAcceptanceCheck(q snapshot.ExternalReviewRequest, artifact []byte) (detail string, blocked bool) {
	if q.Action != "RECORD" || q.Candidate.Kind != "EVIDENCE" {
		return "", false
	}
	want, report, err := ExternalAcceptanceVerdict(artifact)
	switch {
	case err != nil:
		return err.Error(), false
	case !report:
		return "", false
	case want == "":
		return "acceptance verdict blocked supports no review verdict", true
	case q.Verdict == nil || *q.Verdict != want:
		return "acceptance report supports only verdict " + want, false
	}
	return "", false
}
