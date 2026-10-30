package proofabstraction

import (
	"bytes"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type fixture struct {
	source              Envelope
	proved              string
	refuted             string
	conflicted          string
	unknown             string
	critical            string
	refutedOnlyEvidence string
}

func TestPPAV0006CompilesExplicitOmission(t *testing.T) {
	item := sourceFixture(t)
	before := cloneEnvelope(item.source)
	request := requestFor(t, item.source, map[string]ActionKind{item.refuted: ActionOmit})
	destination, certificate, err := Compile(item.source, request)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(item.source, request, destination, certificate); err != nil {
		t.Fatal(err)
	}
	if destination.Completeness != CompletenessPartial || len(destination.Claims) != len(item.source.Claims)-1 {
		t.Fatalf("destination completeness/claim count = %s/%d", destination.Completeness, len(destination.Claims))
	}
	if _, exists := claimsByID(destination.Claims)[item.refuted]; exists {
		t.Fatal("omitted claim remains in destination")
	}
	if len(certificate.OmittedClaims) != 1 || certificate.OmittedClaims[0].SourceClaimID != item.refuted {
		t.Fatalf("omitted claims = %#v", certificate.OmittedClaims)
	}
	if stateForEvidence(t, certificate, item.refutedOnlyEvidence) != EvidenceOmitted {
		t.Fatal("refuted-only evidence was not accounted as omitted")
	}
	if len(certificate.FrontierAccounting.Added) != 1 {
		t.Fatalf("added frontier = %#v", certificate.FrontierAccounting.Added)
	}
	if !reflect.DeepEqual(item.source, before) {
		t.Fatal("compile mutated the source envelope")
	}
}

func TestPPAV0006CompilesCanonicalDemotion(t *testing.T) {
	item := sourceFixture(t)
	request := requestFor(t, item.source, map[string]ActionKind{item.proved: ActionDemoteToUnknown})
	destination, certificate, err := Compile(item.source, request)
	if err != nil {
		t.Fatal(err)
	}
	claim := claimsByID(destination.Claims)[item.proved]
	want := demoteClaim(claimsByID(item.source.Claims)[item.proved])
	if !reflect.DeepEqual(claim, want) {
		t.Fatalf("demoted claim = %#v, want %#v", claim, want)
	}
	if destination.Completeness != CompletenessPartial || len(certificate.OmittedClaims) != 0 {
		t.Fatalf("demotion result = %s, omissions %#v", destination.Completeness, certificate.OmittedClaims)
	}
	if err := Verify(item.source, request, destination, certificate); err != nil {
		t.Fatal(err)
	}
}

func TestPPAV0007KeepsCriticalConflictUnknownAndFrontierSticky(t *testing.T) {
	item := sourceFixture(t)
	tests := []struct {
		name string
		id   string
		code Code
	}{
		{"critical", item.critical, CriticalClaimWeakened},
		{"conflict", item.conflicted, ConflictHidden},
		{"unknown", item.unknown, UnknownHidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := requestFor(t, item.source, map[string]ActionKind{test.id: ActionOmit})
			_, _, err := Compile(item.source, request)
			assertCode(t, err, test.code)
		})
	}

	frontierSource := addSourceFrontier(t, item.source, item.refuted, FrontierSourceExclusion)
	request := requestFor(t, frontierSource, map[string]ActionKind{item.refuted: ActionDemoteToUnknown})
	_, _, err := Compile(frontierSource, request)
	assertCode(t, err, FrontierRemoved)
}

func TestPPAV0005RequiresTotalExactActionSet(t *testing.T) {
	item := sourceFixture(t)
	request := requestFor(t, item.source, nil)
	request.Actions = request.Actions[:len(request.Actions)-1]
	_, _, err := Compile(item.source, request)
	assertCode(t, err, OmissionUnaccounted)

	request = requestFor(t, item.source, nil)
	request.Actions[0].ClaimID = claimPrefix + strings.Repeat("f", 64)
	sort.Slice(request.Actions, func(left, right int) bool { return request.Actions[left].ClaimID < request.Actions[right].ClaimID })
	_, _, err = Compile(item.source, request)
	assertCode(t, err, ClaimAdded)
}

func TestPPAV0011RejectsStrengtheningMutations(t *testing.T) {
	item := sourceFixture(t)
	request := requestFor(t, item.source, map[string]ActionKind{item.refuted: ActionOmit})
	destination, certificate, err := Compile(item.source, request)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("added claim", func(t *testing.T) {
		mutated := cloneEnvelope(destination)
		foreign := mustClaim(t, "9", false, VerdictProved, AuthorityOwningVerifier, []EvidenceClass{EvidenceVerified}, []string{mutated.Evidence[0].ID}, []string{}, []string{})
		mutated.Claims = append(mutated.Claims, foreign)
		sort.Slice(mutated.Claims, func(left, right int) bool { return mutated.Claims[left].ID < mutated.Claims[right].ID })
		mutated = mustBindEnvelope(t, mutated)
		candidate := bindCertificateForDestination(t, certificate, mutated)
		assertCode(t, Verify(item.source, request, mutated, candidate), ClaimAdded)
	})

	t.Run("criticality changed", func(t *testing.T) {
		mutated := cloneEnvelope(destination)
		index := claimIndex(t, mutated, item.critical)
		mutated.Claims[index].Critical = false
		mutated = mustBindEnvelope(t, mutated)
		candidate := bindCertificateForDestination(t, certificate, mutated)
		assertCode(t, Verify(item.source, request, mutated, candidate), CriticalClaimWeakened)
	})

	t.Run("authority changed", func(t *testing.T) {
		mutated := cloneEnvelope(destination)
		index := claimIndex(t, mutated, item.proved)
		mutated.Claims[index].AuthorityClass = AuthorityAdvisory
		mutated = mustBindEnvelope(t, mutated)
		candidate := bindCertificateForDestination(t, certificate, mutated)
		assertCode(t, Verify(item.source, request, mutated, candidate), AuthorityChanged)
	})

	t.Run("evidence added", func(t *testing.T) {
		mutated := cloneEnvelope(destination)
		added := mustEvidence(t, "cem/0.2", "foreign", strings.Repeat("7", 64))
		mutated.Evidence = append(mutated.Evidence, added)
		sort.Slice(mutated.Evidence, func(left, right int) bool { return mutated.Evidence[left].ID < mutated.Evidence[right].ID })
		index := claimIndex(t, mutated, item.proved)
		mutated.Claims[index].Evidence = append(mutated.Claims[index].Evidence, added.ID)
		sort.Strings(mutated.Claims[index].Evidence)
		mutated = mustBindEnvelope(t, mutated)
		candidate := bindCertificateForDestination(t, certificate, mutated)
		assertCode(t, Verify(item.source, request, mutated, candidate), EvidenceAdded)
	})

	t.Run("snapshot changed", func(t *testing.T) {
		mutated := cloneEnvelope(destination)
		mutated.Repository.Revision = strings.Repeat("8", 40)
		mutated = mustBindEnvelope(t, mutated)
		candidate := bindCertificateForDestination(t, certificate, mutated)
		candidate.Repository = mutated.Repository
		candidate = mustBindCertificate(t, candidate)
		assertCode(t, Verify(item.source, request, mutated, candidate), SnapshotChanged)
	})

	t.Run("policy changed", func(t *testing.T) {
		mutated := cloneEnvelope(destination)
		mutated.PolicySHA256 = strings.Repeat("9", 64)
		mutated = mustBindEnvelope(t, mutated)
		candidate := bindCertificateForDestination(t, certificate, mutated)
		candidate.PolicySHA256 = mutated.PolicySHA256
		candidate = mustBindCertificate(t, candidate)
		assertCode(t, Verify(item.source, request, mutated, candidate), PolicyChanged)
	})

	t.Run("source frontier removed", func(t *testing.T) {
		mutated := cloneEnvelope(destination)
		mutated.Frontier = filterFrontier(mutated.Frontier, func(value Frontier) bool { return value.Kind == FrontierAbstractedClaim })
		mutated = mustBindEnvelope(t, mutated)
		candidate := bindCertificateForDestination(t, certificate, mutated)
		assertCode(t, Verify(item.source, request, mutated, candidate), FrontierRemoved)
	})

	t.Run("omission frontier removed", func(t *testing.T) {
		mutated := cloneEnvelope(destination)
		mutated.Frontier = filterFrontier(mutated.Frontier, func(value Frontier) bool { return value.Kind != FrontierAbstractedClaim })
		mutated = mustBindEnvelope(t, mutated)
		candidate := bindCertificateForDestination(t, certificate, mutated)
		assertCode(t, Verify(item.source, request, mutated, candidate), OmissionUnaccounted)
	})

	t.Run("completeness strengthened", func(t *testing.T) {
		mutated := cloneEnvelope(destination)
		mutated.Completeness = CompletenessComplete
		mutated = mustBindEnvelope(t, mutated)
		candidate := bindCertificateForDestination(t, certificate, mutated)
		candidate.CompletenessRelation = CompletenessEqual
		candidate = mustBindCertificate(t, candidate)
		assertCode(t, Verify(item.source, request, mutated, candidate), CompletenessStrengthened)
	})

	t.Run("certificate accounting changed", func(t *testing.T) {
		candidate := certificate
		candidate.EvidenceAccounting = append([]EvidenceAccounting{}, certificate.EvidenceAccounting...)
		candidate.EvidenceAccounting[0].State = EvidenceOmitted
		candidate = mustBindCertificate(t, candidate)
		assertCode(t, Verify(item.source, request, destination, candidate), CertificateMismatch)
	})
}

func TestPPAV0011RejectsRehashedCertificateForgeries(t *testing.T) {
	item := sourceFixture(t)

	t.Run("foreign identical relation", func(t *testing.T) {
		request := requestFor(t, item.source, map[string]ActionKind{item.refuted: ActionOmit})
		destination, certificate, err := Compile(item.source, request)
		if err != nil {
			t.Fatal(err)
		}
		candidate := certificate
		candidate.ClaimRelations = append([]ClaimRelation{}, certificate.ClaimRelations...)
		foreignID := claimPrefix + strings.Repeat("f", 64)
		candidate.ClaimRelations = append(candidate.ClaimRelations, ClaimRelation{
			SourceClaimID:      foreignID,
			DestinationClaimID: foreignID,
			Relation:           RelationIdentical,
		})
		sort.Slice(candidate.ClaimRelations, func(left, right int) bool {
			return candidate.ClaimRelations[left].SourceClaimID < candidate.ClaimRelations[right].SourceClaimID
		})
		candidate = mustBindCertificate(t, candidate)
		assertCode(t, Verify(item.source, request, destination, candidate), CertificateMismatch)
	})

	t.Run("demotion reason changed", func(t *testing.T) {
		request := requestFor(t, item.source, map[string]ActionKind{item.proved: ActionDemoteToUnknown})
		destination, certificate, err := Compile(item.source, request)
		if err != nil {
			t.Fatal(err)
		}
		candidate := certificate
		candidate.ClaimRelations = append([]ClaimRelation{}, certificate.ClaimRelations...)
		for index := range candidate.ClaimRelations {
			if candidate.ClaimRelations[index].SourceClaimID == item.proved {
				reason := ReasonAudience
				candidate.ClaimRelations[index].Reason = &reason
			}
		}
		candidate = mustBindCertificate(t, candidate)
		assertCode(t, Verify(item.source, request, destination, candidate), CertificateMismatch)

		changedRequest := request
		changedRequest.Actions = append([]Action{}, request.Actions...)
		reason := ReasonAudience
		changedRequest.Actions[claimActionIndex(t, changedRequest, item.proved)].Reason = &reason
		assertCode(t, Verify(item.source, changedRequest, destination, certificate), CertificateMismatch)
	})
}

func TestPPAV0003CanonicalWireRoundTrip(t *testing.T) {
	item := sourceFixture(t)
	request := requestFor(t, item.source, map[string]ActionKind{item.proved: ActionDemoteToUnknown})
	destination, certificate, err := Compile(item.source, request)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		marshal func() ([]byte, error)
		decode  func([]byte) error
	}{
		{"envelope", func() ([]byte, error) { return MarshalEnvelope(destination) }, func(raw []byte) error { _, err := DecodeEnvelope(raw); return err }},
		{"request", func() ([]byte, error) { return MarshalRequest(request) }, func(raw []byte) error { _, err := DecodeRequest(raw); return err }},
		{"certificate", func() ([]byte, error) { return MarshalCertificate(certificate) }, func(raw []byte) error { _, err := DecodeCertificate(raw); return err }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw, err := test.marshal()
			if err != nil {
				t.Fatal(err)
			}
			if err := test.decode(raw); err != nil {
				t.Fatal(err)
			}
			assertCode(t, test.decode(append([]byte(" "), raw...)), Noncanonical)
			assertCode(t, test.decode(append(raw, '\n')), Noncanonical)
		})
	}
}

func TestPPAV0003FreezesIdentityPreimages(t *testing.T) {
	item := sourceFixture(t)
	request := requestFor(t, item.source, map[string]ActionKind{item.proved: ActionDemoteToUnknown, item.refuted: ActionOmit})
	destination, certificate, err := Compile(item.source, request)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"proof-state:sha256:f04efb8e1a53737d9756bcb226d3f666d4bf8390744e63f1d63369ae4bec2954",
		"proof-state:sha256:a16a01e4a91cf19c31b73ccc038d95d5e40ef4cb652585b8fbf360f7a27eec99",
		"ppa-certificate:sha256:f40b0302cb67b294f99ffd0e7b31a5c49656e0298f23ac59a9a4356fc078f930",
	}
	got := []string{item.source.EnvelopeID, destination.EnvelopeID, certificate.CertificateID}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("identity tuple = %#v, want %#v", got, want)
	}
}

func TestPPAV0003RejectsUnknownDuplicateNullAndOversizeWire(t *testing.T) {
	item := sourceFixture(t)
	raw, err := MarshalEnvelope(item.source)
	if err != nil {
		t.Fatal(err)
	}
	unknown := bytes.Replace(raw, []byte(`"profile":`), []byte(`"extra":true,"profile":`), 1)
	_, err = DecodeEnvelope(unknown)
	assertCode(t, err, Noncanonical)
	duplicate := bytes.Replace(raw, []byte(`"profile":`), []byte(`"profile":"corvint-proof-state/0-experimental","profile":`), 1)
	_, err = DecodeEnvelope(duplicate)
	assertCode(t, err, Noncanonical)
	nullClaims := bytes.Replace(raw, []byte(`"claims":[`), []byte(`"claims":null,"discard":[`), 1)
	_, err = DecodeEnvelope(nullClaims)
	assertCode(t, err, Noncanonical)
	_, err = DecodeEnvelope(bytes.Repeat([]byte{'x'}, MaxArtifactBytes+1))
	assertCode(t, err, LimitExceeded)

	mutated := cloneEnvelope(item.source)
	mutated.Claims[0].StatementSHA256 = strings.Repeat("0", 64)
	_, err = MarshalEnvelope(mutated)
	assertCode(t, err, ClaimIdentityChanged)
}

func TestPPAV0011IsByteDeterministic(t *testing.T) {
	item := sourceFixture(t)
	request := requestFor(t, item.source, map[string]ActionKind{item.proved: ActionDemoteToUnknown, item.refuted: ActionOmit})
	firstDestination, firstCertificate, err := Compile(item.source, request)
	if err != nil {
		t.Fatal(err)
	}
	firstDestinationBytes, _ := MarshalEnvelope(firstDestination)
	firstCertificateBytes, _ := MarshalCertificate(firstCertificate)
	for iteration := 0; iteration < 100; iteration++ {
		destination, certificate, compileErr := Compile(item.source, request)
		if compileErr != nil {
			t.Fatal(compileErr)
		}
		destinationBytes, _ := MarshalEnvelope(destination)
		certificateBytes, _ := MarshalCertificate(certificate)
		if !bytes.Equal(destinationBytes, firstDestinationBytes) || !bytes.Equal(certificateBytes, firstCertificateBytes) {
			t.Fatalf("iteration %d produced different bytes", iteration)
		}
	}
}

func TestPPAV0015AcceptsAllSafeTwoClaimProjections(t *testing.T) {
	item := sourceFixture(t)
	actions := []ActionKind{ActionRetain, ActionDemoteToUnknown, ActionOmit}
	for _, provedAction := range actions {
		for _, refutedAction := range actions {
			request := requestFor(t, item.source, map[string]ActionKind{item.proved: provedAction, item.refuted: refutedAction})
			destination, certificate, err := Compile(item.source, request)
			if err != nil {
				t.Fatalf("%s/%s: %v", provedAction, refutedAction, err)
			}
			if err := Verify(item.source, request, destination, certificate); err != nil {
				t.Fatalf("%s/%s: %v", provedAction, refutedAction, err)
			}
		}
	}
}

func FuzzPPAV0003DecodeEnvelope(f *testing.F) {
	item := sourceFixture(f)
	raw, err := MarshalEnvelope(item.source)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(raw)
	f.Add([]byte(`{}`))
	f.Fuzz(func(t *testing.T, candidate []byte) {
		_, _ = DecodeEnvelope(candidate)
	})
}

func BenchmarkCompileProjection(b *testing.B) {
	item := sourceFixture(b)
	request := requestFor(b, item.source, map[string]ActionKind{item.proved: ActionDemoteToUnknown, item.refuted: ActionOmit})
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		if _, _, err := Compile(item.source, request); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkVerifyProjection(b *testing.B) {
	item := sourceFixture(b)
	request := requestFor(b, item.source, map[string]ActionKind{item.proved: ActionDemoteToUnknown, item.refuted: ActionOmit})
	destination, certificate, err := Compile(item.source, request)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		if err := Verify(item.source, request, destination, certificate); err != nil {
			b.Fatal(err)
		}
	}
}

func sourceFixture(t testing.TB) fixture {
	evidence := []Evidence{
		mustEvidence(t, "cem/0.2", "source-a", strings.Repeat("1", 64)),
		mustEvidence(t, "cem/0.2", "source-b", strings.Repeat("2", 64)),
		mustEvidence(t, "test-receipt/0", "source-c", strings.Repeat("3", 64)),
		mustEvidence(t, "test-receipt/0", "source-d", strings.Repeat("4", 64)),
		mustEvidence(t, "test-receipt/0", "source-e", strings.Repeat("5", 64)),
		mustEvidence(t, "policy/0", "source-f", strings.Repeat("6", 64)),
	}
	sort.Slice(evidence, func(left, right int) bool { return evidence[left].ID < evidence[right].ID })
	evidenceIDs := make([]string, len(evidence))
	for index, value := range evidence {
		evidenceIDs[index] = value.ID
	}
	provedEvidence := sortedCopy(evidenceIDs[:2])
	claims := []Claim{
		mustClaim(t, "a", false, VerdictProved, AuthorityOwningVerifier, []EvidenceClass{EvidenceVerified}, provedEvidence, []string{}, []string{}),
		mustClaim(t, "b", false, VerdictRefuted, AuthorityOwningVerifier, []EvidenceClass{EvidenceVerified}, []string{}, []string{evidenceIDs[2]}, []string{}),
		mustClaim(t, "c", false, VerdictConflicted, AuthorityRepositoryAccepted, []EvidenceClass{EvidenceDeclared, EvidenceObserved}, []string{evidenceIDs[3]}, []string{evidenceIDs[4]}, []string{}),
		mustClaim(t, "d", false, VerdictUnknown, AuthorityNone, []EvidenceClass{}, []string{}, []string{}, []string{"DYNAMIC_BOUNDARY"}),
		mustClaim(t, "e", true, VerdictProved, AuthorityRepositoryAccepted, []EvidenceClass{EvidenceDeclared}, []string{evidenceIDs[5]}, []string{}, []string{}),
	}
	idsByStatement := make(map[string]string)
	for _, value := range claims {
		idsByStatement[value.StatementSHA256] = value.ID
	}
	sort.Slice(claims, func(left, right int) bool { return claims[left].ID < claims[right].ID })
	unknownID := idsByStatement[digestCharacter("d")]
	frontier := mustFrontier(t, Frontier{ClaimID: unknownID, Kind: FrontierSourceUnknown, Reason: "DYNAMIC_BOUNDARY", Recovery: RecoverySourceVerifier})
	source := mustBindEnvelope(t, Envelope{
		Profile:       EnvelopeProfile,
		Repository:    Repository{ObjectFormat: "sha1", Revision: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40)},
		PolicySHA256:  strings.Repeat("c", 64),
		PurposeSHA256: strings.Repeat("d", 64),
		Completeness:  CompletenessComplete,
		Claims:        claims,
		Evidence:      evidence,
		Frontier:      []Frontier{frontier},
	})
	return fixture{
		source:              source,
		proved:              idsByStatement[digestCharacter("a")],
		refuted:             idsByStatement[digestCharacter("b")],
		conflicted:          idsByStatement[digestCharacter("c")],
		unknown:             unknownID,
		critical:            idsByStatement[digestCharacter("e")],
		refutedOnlyEvidence: evidenceIDs[2],
	}
}

func mustEvidence(t testing.TB, profile, externalID, content string) Evidence {
	t.Helper()
	value, err := BindEvidence(Evidence{SourceProfile: profile, ExternalID: externalID, ContentSHA256: content})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustClaim(t testing.TB, statement string, critical bool, verdict Verdict, authority AuthorityClass, classes []EvidenceClass, evidence, counter, unknown []string) Claim {
	t.Helper()
	sort.Slice(classes, func(left, right int) bool { return classes[left] < classes[right] })
	sort.Strings(evidence)
	sort.Strings(counter)
	sort.Strings(unknown)
	value, err := BindClaim(Claim{
		StatementSHA256: digestCharacter(statement), ScopeSHA256: strings.Repeat("f", 64), Critical: critical,
		Verdict: verdict, AuthorityClass: authority, EvidenceClasses: classes,
		Evidence: evidence, CounterEvidence: counter, UnknownReasons: unknown,
	})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustFrontier(t testing.TB, value Frontier) Frontier {
	t.Helper()
	bound, err := BindFrontier(value)
	if err != nil {
		t.Fatal(err)
	}
	return bound
}

func mustBindEnvelope(t testing.TB, value Envelope) Envelope {
	t.Helper()
	bound, err := BindEnvelope(value)
	if err != nil {
		t.Fatal(err)
	}
	return bound
}

func mustBindCertificate(t testing.TB, value Certificate) Certificate {
	t.Helper()
	value.CertificateID = ""
	id, err := certificateID(value)
	if err != nil {
		t.Fatal(err)
	}
	value.CertificateID = id
	if err := validateCertificate(value); err != nil {
		t.Fatal(err)
	}
	return value
}

func requestFor(t testing.TB, source Envelope, changes map[string]ActionKind) Request {
	t.Helper()
	actions := make([]Action, 0, len(source.Claims))
	for _, claim := range source.Claims {
		kind := ActionRetain
		if selected, ok := changes[claim.ID]; ok {
			kind = selected
		}
		var reason *Reason
		if kind != ActionRetain {
			value := ReasonBudget
			reason = &value
		}
		actions = append(actions, Action{ClaimID: claim.ID, Action: kind, Reason: reason})
	}
	return Request{Profile: RequestProfile, SourceEnvelopeID: source.EnvelopeID, PurposeSHA256: strings.Repeat("e", 64), Actions: actions}
}

func addSourceFrontier(t testing.TB, source Envelope, claimID string, kind FrontierKind) Envelope {
	t.Helper()
	value := cloneEnvelope(source)
	value.Frontier = append(value.Frontier, mustFrontier(t, Frontier{ClaimID: claimID, Kind: kind, Reason: "EXPLICIT_GAP", Recovery: RecoverySourceVerifier}))
	sort.Slice(value.Frontier, func(left, right int) bool { return value.Frontier[left].ID < value.Frontier[right].ID })
	return mustBindEnvelope(t, value)
}

func bindCertificateForDestination(t testing.TB, value Certificate, destination Envelope) Certificate {
	t.Helper()
	value.DestinationEnvelopeID = destination.EnvelopeID
	value.PurposeSHA256 = destination.PurposeSHA256
	return mustBindCertificate(t, value)
}

func cloneEnvelope(value Envelope) Envelope {
	sourceClaims := value.Claims
	value.Claims = make([]Claim, len(sourceClaims))
	for index, claim := range sourceClaims {
		value.Claims[index] = cloneClaim(claim)
	}
	value.Evidence = append([]Evidence{}, value.Evidence...)
	value.Frontier = cloneFrontiers(value.Frontier)
	return value
}

func filterFrontier(values []Frontier, keep func(Frontier) bool) []Frontier {
	result := make([]Frontier, 0, len(values))
	for _, value := range values {
		if keep(value) {
			result = append(result, value)
		}
	}
	return result
}

func claimIndex(t testing.TB, value Envelope, id string) int {
	t.Helper()
	for index, claim := range value.Claims {
		if claim.ID == id {
			return index
		}
	}
	t.Fatalf("claim %s not found", id)
	return -1
}

func claimActionIndex(t testing.TB, value Request, id string) int {
	t.Helper()
	for index, action := range value.Actions {
		if action.ClaimID == id {
			return index
		}
	}
	t.Fatalf("claim action %s not found", id)
	return -1
}

func stateForEvidence(t testing.TB, value Certificate, id string) EvidenceState {
	t.Helper()
	for _, row := range value.EvidenceAccounting {
		if row.EvidenceID == id {
			return row.State
		}
	}
	t.Fatalf("evidence %s not found", id)
	return ""
}

func digestCharacter(value string) string { return strings.Repeat(value, 64) }

func assertCode(t testing.TB, err error, want Code) {
	t.Helper()
	got, ok := ErrorCode(err)
	if !ok || got != want {
		t.Fatalf("error = %v (%q), want %q", err, got, want)
	}
}
