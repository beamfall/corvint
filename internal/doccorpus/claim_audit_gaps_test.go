package doccorpus

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

// referenceClaimAuditGaps is claimAuditGaps before DCR-V1-557, which mapped every claim.
func referenceClaimAuditGaps(a *Artifact) []Gap {
	out := []Gap{}
	subjects := map[string]string{}
	for _, claim := range a.Claims {
		subjects[claim.ID] = claim.Subject
	}
	for _, j := range a.ClaimAudits {
		switch j.Status {
		case "stale":
			out = append(out, Gap{j.ID, "stale-claim-audit", strings.Join(j.Reasons, "; ")})
		case "unresolved":
			out = append(out, Gap{j.ID, "unresolved-claim-audit", strings.Join(j.Reasons, "; ")})
		case "current":
			if j.Verdict != "incorrect" {
				continue
			}
			label := "independent"
			if !j.Headline {
				label = "non-headline"
			}
			reason := "current " + label + " " + j.Method + " verdict " + j.ID + " judges claim " + j.ClaimID + " incorrect"
			out = append(out, Gap{j.ClaimID, "claim-audit-incorrect", reason})
			if s := subjects[j.ClaimID]; s != "" && s != j.ClaimID {
				out = append(out, Gap{s, "claim-audit-incorrect", reason})
			}
		}
	}
	return out
}

// TestDCRV1557ClaimAuditGapsMatchReference shows that mapping only the judged claims returns
// exactly the gaps of mapping every claim: duplicate claim IDs (the last still wins), empty and
// self subjects, judged IDs no claim has, and audits with no current incorrect verdict.
func TestDCRV1557ClaimAuditGapsMatchReference(t *testing.T) {
	rng := rand.New(rand.NewSource(557))
	statuses := []string{"current", "stale", "unresolved", "other"}
	verdicts := []string{"incorrect", "correct", ""}
	for round := range 400 {
		a := &Artifact{}
		ids := 1 + rng.Intn(6)
		for i := range rng.Intn(12) {
			id := fmt.Sprintf("claim:%d", rng.Intn(ids))
			subject := []string{"", id, fmt.Sprintf("flow:%d", i), "flow:shared"}[rng.Intn(4)]
			a.Claims = append(a.Claims, Claim{ID: id, Subject: subject})
		}
		for i := range rng.Intn(8) {
			a.ClaimAudits = append(a.ClaimAudits, ClaimAuditJoin{
				ID:       fmt.Sprintf("audit:%d", i),
				ClaimID:  fmt.Sprintf("claim:%d", rng.Intn(ids+2)),
				Status:   statuses[rng.Intn(len(statuses))],
				Verdict:  verdicts[rng.Intn(len(verdicts))],
				Method:   "review",
				Headline: rng.Intn(2) == 0,
				Reasons:  []string{"r"},
			})
		}
		if got, want := claimAuditGaps(a), referenceClaimAuditGaps(a); !reflect.DeepEqual(got, want) {
			t.Fatalf("round %d: gaps = %#v, want %#v", round, got, want)
		}
	}
}
