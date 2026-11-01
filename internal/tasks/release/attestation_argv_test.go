package release

import (
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTRMV0803_CommandGateAttestationBindsPolicyArgv (V1-0232): an
// attestation of a COMMAND gate is written and counted only with the policy
// gate argv, argv is refused on any other gate, and an attestation without
// argv keeps its digest.
func TestTRMV0803_CommandGateAttestationBindsPolicyArgv(t *testing.T) {
	argv := []string{"script/release-checklist", "--pre-promotion"}
	gates := []Gate{{GateID: "checklist", Kind: "COMMAND", Required: true, Argv: argv}, {GateID: "manual", Kind: "MANUAL"}}
	id := "ticket:acme:main:AT-0001"
	r := &Record{QueueID: wire.QueueID{Raw: "queue:acme:main"}, ReleaseID: "v1", Revision: "1", Version: "v1", Title: "Release", TicketIDs: []wire.TicketID{{Raw: id}}, AcceptanceCriteria: []string{"criterion"}, RequiredGates: []string{"checklist"}, Attestations: []Attestation{}}
	r.Candidate = candidateFor(r, digest("policy"), id, digest("ticket"), 1, nil)
	r, err := Decode(Encode(r))
	if err != nil {
		t.Fatal(err)
	}
	cd := CandidateDigest(r.Candidate)
	obs := Observation{HeadCommit: r.Candidate.HeadCommit, SourceSha256: r.Candidate.SourceSha256, PolicySha256: r.Candidate.PolicySha256, Tickets: map[string]*ticket.Record{id: completed(id, 1)}, TicketDigests: map[string]wire.Digest{id: digest("ticket")}, PredecessorPromotions: map[string]wire.Digest{}}
	owner := Actor{ID: "owner", Role: "OWNER"}
	attest := func(op, gate string, provenance string, a []string) (*Record, error) {
		o := wire.NewObject().Set("profile", wire.String(AttestationProfile)).Set("attestationId", wire.String(gate)).Set("candidateSha256", wire.String(string(cd))).Set("gateId", wire.String(gate)).Set("provenance", wire.String(provenance)).Set("actor", wire.String("owner")).Set("recordedAt", wire.String("2026-09-20T00:00:00Z")).Set("result", wire.String("PASS")).Set("criteria", wire.Strings([]string{"0"})).Set("evidence", wire.Strings([]string{string(digest("evidence"))})).Set("sourceIdentity", wire.String("ci"))
		if a != nil {
			o.Set("argv", wire.Strings(a))
		}
		env, err := DecodeEnvelope(wire.EncodeFile(envelopeValue(op, wire.String("1"), wire.ObjectValue(wire.NewObject().Set("attestation", wire.ObjectValue(o))))))
		if err != nil {
			return nil, err
		}
		return Apply(env, owner, r, r.QueueID, nil, nil, "2026-09-20T00:01:00Z", nil, gates, obs)
	}
	refused := func(name string, out *Record, err error) {
		t.Helper()
		if out != nil || err == nil || !strings.Contains(err.Error(), "TRM-V0-803") {
			t.Fatalf("%s: want a TRM-V0-803 refusal, got %+v %v", name, out, err)
		}
	}
	var out *Record
	out, err = attest(OpExternalAttest, "checklist", External, nil)
	refused("absent argv", out, err)
	out, err = attest(OpExternalAttest, "checklist", External, []string{"script/release-checklist"})
	refused("different argv", out, err)
	out, err = attest(OpManualAttest, "manual", Manual, []string{"make"})
	refused("argv on a MANUAL gate", out, err)
	out, err = attest(OpExternalAttest, "absent", External, []string{"make"})
	refused("argv on an unknown gate", out, err)

	out, err = attest(OpExternalAttest, "checklist", External, argv)
	if err != nil || len(out.Attestations) != 1 || !slices.Equal(out.Attestations[0].Argv, argv) {
		t.Fatalf("matching argv: %+v %v", out, err)
	}
	back, err := Decode(Encode(out))
	if err != nil || !slices.Equal(back.Attestations[0].Argv, argv) || AttestationDigest(back.Attestations[0]) != AttestationDigest(out.Attestations[0]) {
		t.Fatalf("argv round trip: %+v %v", back, err)
	}
	if got := Assess(out, gates, obs); got.State != ReadyAttested {
		t.Fatalf("matching argv not counted: %+v", got)
	}
	env, err := DecodeEnvelope(wire.EncodeFile(envelopeValue(OpPromote, wire.String(string(out.Revision)), wire.ObjectValue(wire.NewObject()))))
	if err != nil {
		t.Fatal(err)
	}
	promoted, err := Apply(env, owner, out, out.QueueID, nil, nil, "2026-09-20T00:02:00Z", nil, gates, obs)
	if err != nil || len(promoted.Promotion.AttestationSha256s) != 1 || promoted.Promotion.AttestationSha256s[0] != AttestationDigest(out.Attestations[0]) {
		t.Fatalf("promotion: %+v %v", promoted, err)
	}

	// A stored attestation from before the policy argv (or before argv
	// existed) no longer counts; readiness names why.
	for name, stale := range map[string][]string{"argv-less": nil, "mismatched": {"script/release-checklist"}} {
		old := cloneRecord(out)
		old.Attestations[0].Argv = stale
		got := Assess(old, gates, obs)
		if got.State == ReadyAttested || !slices.Contains(got.Missing, "gate:checklist") || !slices.Contains(got.Missing, "gate-argv:checklist") {
			t.Fatalf("%s attestation counted: %+v", name, got)
		}
		if passingAttestation(old.Attestations[0], cd, gates) {
			t.Fatalf("%s attestation would be bound by promotion", name)
		}
	}

	// An attestation without argv encodes exactly as before TRM-V0-803.
	plain := out.Attestations[0]
	plain.Argv = nil
	if strings.Contains(string(wire.EncodeFile(attestationValue(plain))), "argv") {
		t.Fatal("argv-less attestation gained an argv key")
	}
	withEmpty := attestationValue(plain)
	withEmpty.Obj.Set("argv", wire.Array())
	rd := wire.NewReader(withEmpty, "attestation")
	if decodeAttestation(rd); rd.Err() == nil {
		t.Fatal("empty argv decoded")
	}
}
