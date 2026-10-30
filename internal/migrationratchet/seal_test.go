package migrationratchet

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// foreignDigest mimics a non-Go producer: same member order, but '<', '>'
// and '&' are written raw instead of <, > and &.
func foreignDigest(t *testing.T, value any) string {
	t.Helper()
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(bytes.TrimSuffix(buffer.Bytes(), []byte("\n")))
	return hex.EncodeToString(sum[:])
}

func TestSealRepairsForeignEncodingAndNamesTheField(t *testing.T) {
	profile := fixtureProfile()
	for i := range profile.Candidate.Records {
		if profile.Candidate.Records[i].Kind == "coverage-target" {
			profile.Candidate.Records[i].ID = "coverage:a>b"
			record := profile.Candidate.Records[i]
			record.SHA256 = ""
			profile.Candidate.Records[i].SHA256 = foreignDigest(t, record)
		}
	}
	slices.SortFunc(profile.Candidate.Records, compareRecords)
	profile.Candidate.ArtifactSHA256 = SnapshotDigest(profile.Candidate)
	_, err := Compare(profile)
	if err == nil || !strings.Contains(err.Error(), "candidate digest refused: records[") || !strings.Contains(err.Error(), "coverage-target coverage:a>b) expected ") {
		t.Fatalf("foreign record digest error = %v", err)
	}
	expected := RecordDigest(profile.Candidate.Records[slices.IndexFunc(profile.Candidate.Records, func(r Record) bool { return r.ID == "coverage:a>b" })])
	if !strings.HasSuffix(err.Error(), expected) {
		t.Fatalf("error does not name expected digest %s: %v", expected, err)
	}
	// Records out of order and stale self-digests are repaired by sealing
	// alone; no content digest, state or link changes.
	slices.Reverse(profile.Candidate.Records)
	profile.Candidate.Policy.SHA256 = strings.Repeat("0", 64)
	sealed := SealProfile(profile)
	receipt, err := Compare(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Verdict != "unknown" && receipt.Verdict != "fail" {
		t.Fatalf("renamed coverage target must stay visible, got %s", receipt.Verdict)
	}
	for i, record := range sealed.Candidate.Records {
		original := profile.Candidate.Records[slices.IndexFunc(profile.Candidate.Records, func(r Record) bool { return r.Kind == record.Kind && r.ID == record.ID })]
		if record.ContentSHA256 != original.ContentSHA256 || record.State != original.State || len(record.Links) != len(original.Links) {
			t.Fatalf("seal changed evidence of record %d", i)
		}
	}
	if again := SealProfile(sealed); Digest(again) != Digest(sealed) {
		t.Fatal("seal is not idempotent")
	}
}

func TestSealLeavesRuleArtifactBindingsReviewed(t *testing.T) {
	profile := fixtureProfile()
	profile.Candidate.Complete = false
	sealSnapshot(&profile.Candidate)
	profile.MigrationRule = reviewedRule(profile, []string{"partial"})
	profile.Candidate.Fresh = false
	sealed := SealProfile(profile)
	if sealed.MigrationRule.CandidateArtifactSHA256 == sealed.Candidate.ArtifactSHA256 {
		t.Fatal("seal re-bound a reviewed rule to a new artifact")
	}
	if _, err := Compare(sealed); err == nil {
		t.Fatal("rule bound to an older artifact was admitted after sealing")
	}
}

func TestPolicyAcceptsPartialAcrossComparisons(t *testing.T) {
	accept := func(p *Profile, accepted []string) {
		for _, snapshot := range []*Snapshot{&p.Baseline, &p.Candidate} {
			snapshot.Policy.AcceptIncomparable = accepted
			snapshot.Policy.SHA256 = PolicyDigest(snapshot.Policy)
			sealSnapshot(snapshot)
		}
	}
	profile := fixtureProfile()
	profile.Baseline.Complete, profile.Candidate.Complete = false, false
	sealSnapshot(&profile.Baseline)
	sealSnapshot(&profile.Candidate)
	if _, err := Compare(profile); err == nil || !strings.Contains(err.Error(), "incomparable partial refused") {
		t.Fatalf("partial without acceptance = %v", err)
	}
	accept(&profile, []string{"partial"})
	receipt, err := Compare(profile)
	if err != nil {
		t.Fatal(err)
	}
	found := slices.ContainsFunc(receipt.Identities, func(d IdentityDelta) bool {
		return d.Kind == "policy-acceptance" && d.CandidateID == "policy:migration" && slices.Equal(d.Changes, []string{"partial"})
	})
	if !found || !slices.ContainsFunc(receipt.Limitations, func(v string) bool { return strings.Contains(v, "accepted partial inputs") }) {
		t.Fatalf("policy acceptance not visible in receipt: %+v %+v", receipt.Identities, receipt.Limitations)
	}
	// A second candidate under the same policy needs no new rule.
	next := profile
	next.Baseline = profile.Candidate
	next.Candidate.Repository.Revision = hexID('5')
	sealSnapshot(&next.Candidate)
	if _, err := Compare(next); err != nil {
		t.Fatalf("second comparison under the same policy refused: %v", err)
	}
	// Stale stays refused when only partial is accepted.
	stale := profile
	stale.Candidate.Fresh = false
	sealSnapshot(&stale.Candidate)
	if _, err := Compare(stale); err == nil || !strings.Contains(err.Error(), "incomparable stale refused") {
		t.Fatalf("stale admitted by partial acceptance: %v", err)
	}
	// Acceptance on one side only is a policy change, not an acceptance.
	oneSided := fixtureProfile()
	oneSided.Candidate.Complete = false
	oneSided.Candidate.Policy.AcceptIncomparable = []string{"partial"}
	oneSided.Candidate.Policy.SHA256 = PolicyDigest(oneSided.Candidate.Policy)
	sealSnapshot(&oneSided.Candidate)
	if _, err := Compare(oneSided); err == nil || !strings.Contains(err.Error(), "incomparable policy,partial refused") {
		t.Fatalf("one-sided acceptance = %v", err)
	}
	for _, bad := range [][]string{{"schema"}, {"stale", "partial"}, {"partial", "partial"}, {}} {
		invalid := fixtureProfile()
		accept(&invalid, bad)
		if _, err := Compare(invalid); err == nil || !strings.Contains(err.Error(), "policy accept_incomparable refused") {
			t.Fatalf("accept %v = %v", bad, err)
		}
	}
	// An absent acceptance keeps every existing policy digest unchanged.
	plain := fixtureProfile()
	encoded, _ := Encode(plain.Baseline.Policy)
	if strings.Contains(string(encoded), "accept_incomparable") {
		t.Fatal("absent acceptance changed the policy encoding")
	}
}

func TestDescribeCoversDecoderAndRefusals(t *testing.T) {
	description := Describe()
	receipt, err := Compare(description.Example)
	if err != nil || receipt.Verdict != "pass" {
		t.Fatalf("example = %v %+v", err, receipt)
	}
	names := []string{}
	for _, field := range description.Objects["Policy"] {
		names = append(names, field.Name)
	}
	if !slices.Contains(names, "accept_incomparable") || len(description.Objects) != 14 || len(description.RecordKinds) != len(recordKinds) {
		t.Fatalf("description = %+v", description.Objects["Policy"])
	}
	file, err := parser.ParseFile(token.NewFileSet(), "compare.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		value, _ := strconv.Unquote(literal.Value)
		pattern := strings.NewReplacer("%s", "*", "%w", "*").Replace(value)
		switch {
		case value == "migration-ratchet %s %w":
		case strings.HasPrefix(value, "migration-ratchet "):
			if !slices.Contains(Refusals, pattern) {
				t.Errorf("refusal %q is not described", pattern)
			}
		case strings.HasPrefix(value, "policy ") && strings.HasSuffix(value, "refused"):
			if !slices.Contains(Refusals, "migration-ratchet * "+pattern) {
				t.Errorf("policy refusal %q is not described", pattern)
			}
		}
		return true
	})
}
