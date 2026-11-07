package release

import (
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTRMV0532_AttestationArgvAdmitsLongPathElement (V1-4164): an attestation
// argv element is bounded by the 4,096-byte PathText bound, not the 128-byte
// Identifier bound, so a long absolute worktree path is admitted, and an
// element past 4,096 bytes is refused LIMIT_EXCEEDED naming the bound.
func TestTRMV0532_AttestationArgvAdmitsLongPathElement(t *testing.T) {
	long := "/" + strings.Repeat("d", 300) + "/script/release-checklist"
	argv := []string{long, "--pre-promotion"}
	gates := []Gate{{GateID: "checklist", Kind: "COMMAND", Required: true, Argv: argv}}
	id := "ticket:acme:main:AT-0001"
	r := &Record{QueueID: wire.QueueID{Raw: "queue:acme:main"}, ReleaseID: "v1", Revision: "1", Version: "v1", Title: "Release", TicketIDs: []wire.TicketID{{Raw: id}}, AcceptanceCriteria: []string{"criterion"}, RequiredGates: []string{"checklist"}, Attestations: []Attestation{}}
	r.Candidate = candidateFor(r, digest("policy"), id, digest("ticket"), 1, nil)
	r, err := Decode(Encode(r))
	if err != nil {
		t.Fatal(err)
	}
	cd := CandidateDigest(r.Candidate)
	obs := Observation{HeadCommit: r.Candidate.HeadCommit, SourceSha256: r.Candidate.SourceSha256, PolicySha256: r.Candidate.PolicySha256, Tickets: map[string]*ticket.Record{id: completed(id, 1)}, TicketDigests: map[string]wire.Digest{id: digest("ticket")}, PredecessorPromotions: map[string]wire.Digest{}}
	envelope := func(a []string) ([]byte, wire.Value) {
		o := wire.NewObject().Set("profile", wire.String(AttestationProfile)).Set("attestationId", wire.String("checklist")).Set("candidateSha256", wire.String(string(cd))).Set("gateId", wire.String("checklist")).Set("provenance", wire.String(External)).Set("actor", wire.String("owner")).Set("recordedAt", wire.String("2026-09-20T00:00:00Z")).Set("result", wire.String("PASS")).Set("criteria", wire.Strings([]string{"0"})).Set("evidence", wire.Strings([]string{string(digest("evidence"))})).Set("sourceIdentity", wire.String("ci")).Set("argv", wire.Strings(a))
		v := envelopeValue(OpExternalAttest, wire.String("1"), wire.ObjectValue(wire.NewObject().Set("attestation", wire.ObjectValue(o))))
		return wire.EncodeFile(v), v
	}
	raw, _ := envelope(argv)
	env, err := DecodeEnvelope(raw)
	if err != nil {
		t.Fatalf("a %d-byte argv element must be admitted: %v", len(long), err)
	}
	out, err := Apply(env, Actor{ID: "owner", Role: "OWNER"}, r, r.QueueID, nil, nil, "2026-09-20T00:01:00Z", nil, gates, obs)
	if err != nil || len(out.Attestations) != 1 || !slices.Equal(out.Attestations[0].Argv, argv) {
		t.Fatalf("long-path attestation: %+v %v", out, err)
	}
	if back, err := Decode(Encode(out)); err != nil || !slices.Equal(back.Attestations[0].Argv, argv) {
		t.Fatalf("long-path attestation round trip: %v", err)
	}
	raw, _ = envelope([]string{"/" + strings.Repeat("x", wire.MaxGateArgBytes)})
	if _, err := DecodeEnvelope(raw); wire.CodeOf(err) != wire.CodeLimitExceeded || !strings.Contains(err.Error(), "4096") {
		t.Fatalf("an argv element over 4096 bytes must be LIMIT_EXCEEDED naming the bound: %v", err)
	}
}
