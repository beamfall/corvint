package attest

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"strings"
	"testing"
)

func signedCEMV3(t *testing.T, cem []byte, ceilings CEMCeilings) ([]byte, []byte) {
	t.Helper()
	statement, err := CEMStatementV3(stableMapName, cem, fixtureV2Target, ceilings)
	if err != nil {
		t.Fatalf("CEMStatementV3: %v", err)
	}
	envelope, _ := signedFixtureStatement(t, statement)
	return statement, envelope
}

func ceiling(value int64) *int64 { return &value }

// TestCAXV0001StatementAddsCountsAndAppliedCeilings checks CAX-V0-001: the
// cem/v3 statement is the cem/v2 statement under its own predicateType plus
// counts read from the map and exactly the applied ceilings, deterministic,
// and the cem/v2 statement bytes are unchanged by the revision.
func TestCAXV0001StatementAddsCountsAndAppliedCeilings(t *testing.T) {
	cem := stableFixture(t)
	v2, _ := signedCEMV2(t, cem)
	first, _ := signedCEMV3(t, cem, CEMCeilings{MaxUnknown: ceiling(0)})
	second, _ := signedCEMV3(t, cem, CEMCeilings{MaxUnknown: ceiling(0)})
	if !bytes.Equal(first, second) {
		t.Fatal("CEMStatementV3 is not deterministic")
	}
	wantPredicate := `"ceilings":{"maxUnknown":0},"counts":{"mechanical":0,"supported":2,"total":2,"unknown":0},`
	if !strings.Contains(string(first), `"predicate":{"baseRevision":`) || !strings.Contains(string(first), wantPredicate) ||
		!strings.Contains(string(first), `"predicateType":"`+CEMPredicateTypeV3+`"`) {
		t.Fatalf("cem/v3 statement = %s", first)
	}
	// Removing the two v3 members and restoring the type gives the v2 bytes.
	stripped := strings.Replace(strings.Replace(string(first), wantPredicate, "", 1), CEMPredicateTypeV3, CEMPredicateTypeV2, 1)
	if stripped != string(v2) {
		t.Fatalf("cem/v3 less its members =\n%s\nwant the cem/v2 bytes\n%s", stripped, v2)
	}
	none, _ := signedCEMV3(t, cem, CEMCeilings{})
	if !strings.Contains(string(none), `"ceilings":{},"counts":`) {
		t.Fatalf("cem/v3 without ceilings = %s", none)
	}
	if _, err := CEMStatementV3(stableMapName, cem, fixtureV2Target, CEMCeilings{MaxMechanical: ceiling(-1)}); err == nil {
		t.Error("CEMStatementV3 accepted a negative ceiling")
	}
	if _, err := CEMStatementV3(stableMapName, cem, fixtureV2Target, CEMCeilings{}); err != nil {
		t.Errorf("CEMStatementV3 without ceilings: %v", err)
	}
}

// TestCAXV0001RefusesExceededCeiling checks CAX-V0-001: no statement is
// produced for a map whose counts break an applied ceiling.
func TestCAXV0001RefusesExceededCeiling(t *testing.T) {
	cem := stableFixture(t)
	// The fixture has two supported hunks; a supported ceiling does not
	// exist, so exercise exceeded() directly for unknown and mechanical.
	issues := CEMCeilings{MaxUnknown: ceiling(0), MaxMechanical: ceiling(1)}.exceeded(CEMCounts{Total: 3, Unknown: 1, Mechanical: 2})
	if len(issues) != 2 || issues[0] != "unknown 1 > 0" || issues[1] != "mechanical 2 > 1" {
		t.Fatalf("exceeded = %v", issues)
	}
	if _, err := CEMStatementV3(stableMapName, cem, fixtureV2Target, CEMCeilings{MaxUnknown: ceiling(0), MaxMechanical: ceiling(0)}); err != nil {
		t.Fatalf("a map within its ceilings was refused: %v", err)
	}
}

// TestCAXV0001VerifiesCountsAgainstTheMap checks CAX-V0-001: the verifier
// accepts a cem/v3 envelope, reports its counts and ceilings, and refuses
// signed counts that differ from the supplied map.
func TestCAXV0001VerifiesCountsAgainstTheMap(t *testing.T) {
	cem := stableFixture(t)
	publicKey := cemFixtureKey().Public().(ed25519.PublicKey)
	_, envelope := signedCEMV3(t, cem, CEMCeilings{MaxMechanical: ceiling(4)})
	target := fixtureV2Target
	got, err := VerifyCEMPredicateWith(envelope, publicKey, CEMCheck{Map: cem, Artifact: caseArtifact(nil), Target: &target})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != CEMVerified || got.PredicateType != CEMPredicateTypeV3 || got.V2 == nil || got.V2.Counts == nil ||
		*got.V2.Counts != (CEMCounts{Total: 2, Supported: 2}) || got.V2.Ceilings == nil ||
		got.V2.Ceilings.MaxUnknown != nil || got.V2.Ceilings.MaxMechanical == nil || *got.V2.Ceilings.MaxMechanical != 4 ||
		got.V2.ArtifactsStatus != CEMVerified || got.V2.TargetStatus != CEMVerified {
		t.Fatalf("verification = %#v (%#v)", got, got.V2)
	}
	statement, _ := signedCEMV3(t, cem, CEMCeilings{})
	edited := strings.Replace(string(statement), `"supported":2,"total":2,"unknown":0`, `"supported":1,"total":2,"unknown":1`, 1)
	if edited == string(statement) {
		t.Fatal("the edit changed nothing")
	}
	forged, _ := signedFixtureStatement(t, []byte(edited))
	if _, err := VerifyCEMPredicate(forged, publicKey, cem); !errors.Is(err, ErrCEMCountsMismatch) {
		t.Fatalf("signed counts that differ from the map: err = %v, want ErrCEMCountsMismatch", err)
	}
}

// TestCAXV0001PredicateIsClosed checks CAX-V0-001: an unknown member, a
// missing counts or ceilings member, a missing count, counts that do not sum,
// and counts over a signed ceiling are refused even when signed.
func TestCAXV0001PredicateIsClosed(t *testing.T) {
	cem := stableFixture(t)
	publicKey := cemFixtureKey().Public().(ed25519.PublicKey)
	statement, _ := signedCEMV3(t, cem, CEMCeilings{})
	cases := map[string]func(string) string{
		"unknown predicate member": func(s string) string { return strings.Replace(s, `"ceilings":{}`, `"ceilings":{},"extra":1`, 1) },
		"unknown counts member":    func(s string) string { return strings.Replace(s, `"counts":{`, `"counts":{"skipped":0,`, 1) },
		"unknown ceilings member":  func(s string) string { return strings.Replace(s, `"ceilings":{}`, `"ceilings":{"maxSupported":9}`, 1) },
		"missing ceilings":         func(s string) string { return strings.Replace(s, `"ceilings":{},`, ``, 1) },
		"missing count":            func(s string) string { return strings.Replace(s, `"mechanical":0,`, ``, 1) },
		"counts do not sum":        func(s string) string { return strings.Replace(s, `"total":2`, `"total":3`, 1) },
		"negative count": func(s string) string {
			return strings.Replace(s, `"supported":2,"total":2,"unknown":0`, `"supported":3,"total":2,"unknown":-1`, 1)
		},
		"count over a signed ceiling": func(s string) string {
			s = strings.Replace(s, `"ceilings":{}`, `"ceilings":{"maxUnknown":0}`, 1)
			return strings.Replace(s, `"supported":2,"total":2,"unknown":0`, `"supported":1,"total":2,"unknown":1`, 1)
		},
		"negative ceiling": func(s string) string { return strings.Replace(s, `"ceilings":{}`, `"ceilings":{"maxUnknown":-1}`, 1) },
		"missing counts": func(s string) string {
			start := strings.Index(s, `"counts":{`)
			end := start + strings.Index(s[start:], `},`) + 2
			return s[:start] + s[end:]
		},
	}
	for name, edit := range cases {
		edited := edit(string(statement))
		if edited == string(statement) {
			t.Fatalf("%s: the edit changed nothing", name)
		}
		envelope, _ := signedFixtureStatement(t, []byte(edited))
		if _, err := VerifyCEMPredicate(envelope, publicKey, cem); err == nil {
			t.Errorf("%s: a signed statement verified", name)
		}
	}
	// A cem/v2 member set under the v3 type lacks counts and is refused.
	v2, _ := signedCEMV2(t, cem)
	envelope, _ := signedFixtureStatement(t, []byte(strings.Replace(string(v2), CEMPredicateTypeV2, CEMPredicateTypeV3, 1)))
	if _, err := VerifyCEMPredicate(envelope, publicKey, cem); err == nil {
		t.Error("a cem/v2 predicate under the cem/v3 type verified")
	}
}
