package release

import (
	"bytes"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func digest(s string) wire.Digest { return wire.Sum([]byte(s)) }

func TestTMV0028_AS38_BindingArraysFollowDefinitionOrder(t *testing.T) {
	first, second := "ticket:acme:main:AT-0001", "ticket:acme:main:AT-0002"
	hi, lo := wire.Digest(strings.Repeat("f", 64)), wire.Digest(strings.Repeat("0", 64))
	r := &Record{QueueID: wire.QueueID{Raw: "queue:acme:main"}, ReleaseID: "v1", Revision: "1", Version: "v1", Title: "Release", TicketIDs: []wire.TicketID{{Raw: first}, {Raw: second}}, PredecessorIDs: []string{"a", "b"}, AcceptanceCriteria: []string{"criterion"}}
	r.Candidate = candidateFor(r, digest("policy"), first, hi, 1, []PredecessorBinding{{ReleaseID: "a", PromotionSha256: hi}, {ReleaseID: "b", PromotionSha256: lo}})
	r.Candidate.Tickets = append(r.Candidate.Tickets, TicketBinding{TicketID: wire.TicketID{Raw: second}, RecordSha256: lo, AcceptanceRevision: "1"})
	a := Attestation{AttestationID: "pass", CandidateSha256: CandidateDigest(r.Candidate), GateID: "manual", Provenance: Manual, Actor: "owner", RecordedAt: "2026-09-20T00:00:00Z", Result: "PASS", Criteria: []wire.Count{"0"}, Evidence: []wire.Digest{digest("evidence")}, SourceIdentity: "owner"}
	r.Attestations = []Attestation{a}
	r.Promotion = &Promotion{CandidateSha256: a.CandidateSha256, AttestationSha256s: []wire.Digest{AttestationDigest(a)}, Predecessors: append([]PredecessorBinding(nil), r.Candidate.Predecessors...), Actor: "owner", RecordedAt: a.RecordedAt}
	raw := Encode(r)
	decoded, err := Decode(raw)
	if err != nil || !bytes.Equal(Encode(decoded), raw) {
		t.Fatalf("ordered round trip: %v", err)
	}
	for _, kind := range []string{"duplicate-ticket", "reordered-ticket", "duplicate-predecessor", "reordered-predecessor", "duplicate-promotion-predecessor"} {
		t.Run(kind, func(t *testing.T) {
			copy, err := Decode(raw)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "duplicate-ticket":
				copy.Candidate.Tickets[1] = copy.Candidate.Tickets[0]
			case "reordered-ticket":
				copy.Candidate.Tickets[0], copy.Candidate.Tickets[1] = copy.Candidate.Tickets[1], copy.Candidate.Tickets[0]
			case "duplicate-predecessor":
				copy.Candidate.Predecessors[1] = copy.Candidate.Predecessors[0]
			case "reordered-predecessor":
				copy.Candidate.Predecessors[0], copy.Candidate.Predecessors[1] = copy.Candidate.Predecessors[1], copy.Candidate.Predecessors[0]
			case "duplicate-promotion-predecessor":
				copy.Promotion.Predecessors[1] = copy.Promotion.Predecessors[0]
			}
			if _, err := Decode(Encode(copy)); err == nil {
				t.Fatal("malformed binding accepted")
			}
		})
	}
}

func TestTMV0028_AS38_PromotionBindsOnlyCompatiblePassingEvidence(t *testing.T) {
	id := "ticket:acme:main:AT-0001"
	r := &Record{QueueID: wire.QueueID{Raw: "queue:acme:main"}, ReleaseID: "v1", Revision: "1", Version: "v1", Title: "Release", TicketIDs: []wire.TicketID{{Raw: id}}, AcceptanceCriteria: []string{"criterion"}}
	r.Candidate = candidateFor(r, digest("policy"), id, digest("ticket"), 1, nil)
	a := Attestation{AttestationID: "pass", CandidateSha256: CandidateDigest(r.Candidate), GateID: "manual", Provenance: Manual, Actor: "owner", RecordedAt: "2026-09-20T00:00:00Z", Result: "PASS", Criteria: []wire.Count{"0"}, Evidence: []wire.Digest{digest("evidence")}, SourceIdentity: "owner"}
	r.Attestations = []Attestation{a}
	for _, kind := range []string{"failed", "stale", "incompatible", "unknown"} {
		b := a
		b.AttestationID = kind
		switch kind {
		case "failed":
			b.Result = "FAIL"
		case "stale":
			b.CandidateSha256 = digest("old")
		case "incompatible":
			b.Provenance = External
		case "unknown":
			b.GateID = "absent"
		}
		r.Attestations = append(r.Attestations, b)
	}
	var err error
	sort.Slice(r.Attestations, func(i, j int) bool {
		return bytes.Compare(wire.EncodeFile(attestationValue(r.Attestations[i])), wire.EncodeFile(attestationValue(r.Attestations[j]))) < 0
	})
	r, err = Decode(Encode(r))
	if err != nil {
		t.Fatal(err)
	}
	env, err := DecodeEnvelope(wire.EncodeFile(envelopeValue(OpPromote, wire.String("1"), wire.ObjectValue(wire.NewObject()))))
	if err != nil {
		t.Fatal(err)
	}
	obs := Observation{HeadCommit: r.Candidate.HeadCommit, SourceSha256: r.Candidate.SourceSha256, PolicySha256: r.Candidate.PolicySha256, Tickets: map[string]*ticket.Record{id: completed(id, 1)}, TicketDigests: map[string]wire.Digest{id: digest("ticket")}}
	out, err := Apply(env, Actor{ID: "owner", Role: "OWNER"}, r, r.QueueID, nil, nil, "2026-09-20T00:01:00Z", nil, []Gate{{GateID: "manual", Kind: "MANUAL", Required: true}}, obs)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Promotion.AttestationSha256s) != 1 || out.Promotion.AttestationSha256s[0] != AttestationDigest(a) {
		t.Fatalf("bound irrelevant evidence: %+v", out.Promotion)
	}
}

func completed(id string, rev int64) *ticket.Record {
	return &ticket.Record{TicketID: wire.TicketID{Raw: id}, Status: ticket.StatusCompleted, AcceptanceRevision: wire.CountOf(rev)}
}

func candidateFor(r *Record, policy wire.Digest, ticketID string, td wire.Digest, rev int64, predecessors []PredecessorBinding) *Candidate {
	return &Candidate{
		RepositoryIdentity: "corvint-tasks", HeadCommit: "5117b9238f9ccc7caf01bc9a5e0ab1877ba76208",
		HeadTree: "6117b9238f9ccc7caf01bc9a5e0ab1877ba76208", SourceSha256: digest("source"),
		DefinitionSha256: DefinitionDigest(r), PolicySha256: policy,
		Tickets:      []TicketBinding{{TicketID: wire.TicketID{Raw: ticketID}, RecordSha256: td, AcceptanceRevision: wire.CountOf(rev)}},
		Predecessors: predecessors,
	}
}

func TestTMV0028_AS38_TwoReleaseReadinessAndInvalidation(t *testing.T) {
	policyDigest := digest("policy")
	policy := []Gate{{GateID: "manual", Kind: "MANUAL", Required: true}}
	t1, t2 := "ticket:acme:main:AT-0001", "ticket:acme:main:AT-0002"
	r1 := &Record{QueueID: wire.QueueID{Raw: "queue:acme:main"}, ReleaseID: "v0-9", Revision: "1", Version: "0.9", Title: "0.9", TicketIDs: []wire.TicketID{{Raw: t1}}, AcceptanceCriteria: []string{"release criterion"}}
	td1 := digest("ticket-1")
	r1.Candidate = candidateFor(r1, policyDigest, t1, td1, 1, nil)
	o1 := Observation{HeadCommit: r1.Candidate.HeadCommit, SourceSha256: r1.Candidate.SourceSha256, PolicySha256: policyDigest, Tickets: map[string]*ticket.Record{t1: completed(t1, 1)}, TicketDigests: map[string]wire.Digest{t1: td1}, PredecessorPromotions: map[string]wire.Digest{}}
	if got := Assess(r1, policy, o1); got.State != Blocked {
		t.Fatalf("missing gate must block: %+v", got)
	}
	r1.Attestations = []Attestation{{AttestationID: "a1", CandidateSha256: CandidateDigest(r1.Candidate), GateID: "manual", Provenance: Manual, Actor: "owner", RecordedAt: "2026-09-20T00:00:00Z", Result: "PASS", Criteria: []wire.Count{"0"}, Evidence: []wire.Digest{digest("evidence")}, SourceIdentity: "owner-observation"}}
	if got := Assess(r1, policy, o1); got.State != ReadyAttested {
		t.Fatalf("matching attestation must ready release: %+v", got)
	}
	r1.Promotion = &Promotion{CandidateSha256: CandidateDigest(r1.Candidate), AttestationSha256s: []wire.Digest{AttestationDigest(r1.Attestations[0])}, Actor: "owner", RecordedAt: "2026-09-20T00:01:00Z"}
	pd := PromotionDigest(r1.Promotion)
	r2 := &Record{QueueID: r1.QueueID, ReleaseID: "v1-0", Revision: "1", Version: "1.0", Title: "1.0", PredecessorIDs: []string{"v0-9"}, TicketIDs: []wire.TicketID{{Raw: t2}}, AcceptanceCriteria: []string{"ship 1.0"}}
	td2 := digest("ticket-2")
	r2.Candidate = candidateFor(r2, policyDigest, t2, td2, 2, []PredecessorBinding{{ReleaseID: "v0-9", PromotionSha256: pd}})
	o2 := Observation{HeadCommit: r2.Candidate.HeadCommit, SourceSha256: r2.Candidate.SourceSha256, PolicySha256: policyDigest, Tickets: map[string]*ticket.Record{t2: completed(t2, 2)}, TicketDigests: map[string]wire.Digest{t2: td2}, PredecessorPromotions: map[string]wire.Digest{"v0-9": pd}}
	r2.Attestations = []Attestation{{AttestationID: "a2", CandidateSha256: CandidateDigest(r2.Candidate), GateID: "manual", Provenance: Manual, Actor: "owner", RecordedAt: "2026-09-20T00:02:00Z", Result: "PASS", Criteria: []wire.Count{"0"}, Evidence: []wire.Digest{digest("evidence-2")}, SourceIdentity: "owner-observation"}}
	if got := Assess(r2, policy, o2); got.State != ReadyAttested {
		t.Fatalf("promoted predecessor must enable successor: %+v", got)
	}
	for name, mutate := range map[string]func(){
		"ticket":      func() { o2.TicketDigests[t2] = digest("changed") },
		"candidate":   func() { o2.SourceSha256 = digest("changed-source") },
		"policy":      func() { o2.PolicySha256 = digest("changed-policy") },
		"predecessor": func() { o2.PredecessorPromotions["v0-9"] = digest("changed-promotion") },
	} {
		t.Run(name, func(t *testing.T) {
			copy := o2
			copy.TicketDigests = cloneDigests(o2.TicketDigests)
			copy.PredecessorPromotions = cloneDigests(o2.PredecessorPromotions)
			o2 = copy
			mutate()
			if got := Assess(r2, policy, o2); got.State != Blocked {
				t.Fatalf("change must invalidate: %+v", got)
			}
			o2 = Observation{HeadCommit: r2.Candidate.HeadCommit, SourceSha256: r2.Candidate.SourceSha256, PolicySha256: policyDigest, Tickets: map[string]*ticket.Record{t2: completed(t2, 2)}, TicketDigests: map[string]wire.Digest{t2: td2}, PredecessorPromotions: map[string]wire.Digest{"v0-9": pd}}
		})
	}
}

func cloneDigests(in map[string]wire.Digest) map[string]wire.Digest {
	out := map[string]wire.Digest{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func envelopeValue(operation string, expected wire.Value, payload wire.Value) wire.Value {
	return wire.ObjectValue(wire.NewObject().
		Set("profile", wire.String(MutationProfile)).
		Set("requestId", wire.String("release-request")).
		Set("actor", wire.ObjectValue(wire.NewObject().Set("id", wire.String("owner")).Set("role", wire.String("OWNER")))).
		Set("queueId", wire.String("queue:acme:main")).
		Set("releaseId", wire.String("v1")).
		Set("expectedRevision", expected).
		Set("operation", wire.String(operation)).
		Set("payload", payload).
		Set("issuedAt", wire.String("2026-09-20T00:00:00Z")))
}

func TestTMV0028_AS38_ReleaseMutationCodecAndPolicyRemoval(t *testing.T) {
	payload := wire.ObjectValue(wire.NewObject().
		Set("version", wire.String("1-0")).
		Set("title", wire.String("Version 1.0")).
		Set("predecessorReleaseIds", wire.Array()).
		Set("ticketIds", wire.Strings([]string{"ticket:acme:main:AT-0001"})).
		Set("acceptanceCriteria", wire.Strings([]string{"release criterion"})).
		Set("requiredGates", wire.Array()))
	raw := wire.EncodeFile(envelopeValue(OpCreate, wire.Null(), payload))
	env, err := DecodeEnvelope(raw)
	if err != nil || env.Operation != OpCreate || env.ExpectedRevision != nil || env.Sha256() != wire.Sum(raw) {
		t.Fatalf("decode: %+v %v", env, err)
	}
	if !Authorized("OPERATOR", OpCreate, "", nil) {
		t.Fatal("default operator create authorization missing")
	}
	if Authorized("OPERATOR", OpCreate, "", map[string][]string{"OPERATOR": {OpUpdate}}) {
		t.Fatal("configured removal was bypassed")
	}
	if Authorized("OPERATOR", OpManualAttest, Manual, nil) || Authorized("OPERATOR", OpPromote, "", nil) {
		t.Fatal("operator gained owner-only release authority")
	}
	bad := envelopeValue(OpCreate, wire.String("1"), payload)
	if _, err := DecodeEnvelope(wire.EncodeFile(bad)); err == nil {
		t.Fatal("create accepted an expected revision")
	}
	bad.Obj.Set("unexpected", wire.Bool(true))
	if _, err := DecodeEnvelope(wire.EncodeFile(bad)); err == nil {
		t.Fatal("unknown envelope key accepted")
	}
}

func TestTMV0028_AS38_ReleaseGraphRejectsUnknownAndCycles(t *testing.T) {
	q := wire.QueueID{Raw: "queue:acme:main"}
	a := &Record{QueueID: q, ReleaseID: "a", PredecessorIDs: []string{"b"}}
	if err := ValidateGraph([]*Record{a}); err == nil {
		t.Fatal("unknown predecessor accepted")
	}
	b := &Record{QueueID: q, ReleaseID: "b", PredecessorIDs: []string{"a"}}
	if err := ValidateGraph([]*Record{a, b}); err == nil {
		t.Fatal("predecessor cycle accepted")
	}
}

func TestTMV0028_AS38_AttestationCompatibilityAndDerivedObligations(t *testing.T) {
	policyDigest := digest("policy")
	policy := []Gate{{GateID: "command", Kind: "COMMAND", Required: true}, {GateID: "manual", Kind: "MANUAL"}}
	id := "ticket:acme:main:AT-0001"
	r := &Record{QueueID: wire.QueueID{Raw: "queue:acme:main"}, ReleaseID: "v1", Revision: "1", Version: "1.0", Title: "1.0", TicketIDs: []wire.TicketID{{Raw: id}}, AcceptanceCriteria: []string{"criterion"}, RequiredGates: []string{"manual"}}
	td := digest("ticket")
	r.Candidate = candidateFor(r, policyDigest, id, td, 1, nil)
	o := Observation{HeadCommit: r.Candidate.HeadCommit, SourceSha256: r.Candidate.SourceSha256, PolicySha256: policyDigest, Tickets: map[string]*ticket.Record{id: completed(id, 1)}, TicketDigests: map[string]wire.Digest{id: td}, PredecessorPromotions: map[string]wire.Digest{}}
	cd := CandidateDigest(r.Candidate)
	r.Attestations = []Attestation{
		{AttestationID: "wrong-manual", CandidateSha256: cd, GateID: "command", Provenance: Manual, Result: "PASS", Criteria: []wire.Count{"0"}},
		{AttestationID: "wrong-external", CandidateSha256: cd, GateID: "manual", Provenance: External, Result: "PASS", Criteria: []wire.Count{"0"}},
	}
	if got := Assess(r, policy, o); got.State != Blocked {
		t.Fatalf("incompatible provenance must block: %+v", got)
	}
	r.Attestations = append(r.Attestations,
		Attestation{AttestationID: "command-ok", CandidateSha256: cd, GateID: "command", Provenance: External, Result: "PASS", Criteria: []wire.Count{"0"}},
		Attestation{AttestationID: "manual-ok", CandidateSha256: cd, GateID: "manual", Provenance: Manual, Result: "PASS"},
	)
	if got := Assess(r, policy, o); got.State != ReadyAttested {
		t.Fatalf("union and compatibility should pass: %+v", got)
	}
}
