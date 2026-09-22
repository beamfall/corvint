package witnesscollapse

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

func TestCWCX0001ExactCEMBinding(t *testing.T) {
	raw, _ := testCEM(t, 2, false)
	withLF := append(append([]byte(nil), raw...), '\n')
	first, err := Compile(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Compile(withLF, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.CEM.MapSHA256 == second.CEM.MapSHA256 {
		t.Fatal("exact CEM byte binding ignored trailing whitespace")
	}
	if first.CEM.Spec != wire.Spec01 || first.CEM.BaseRevision == "" || first.CEM.PatchSHA256 == "" {
		t.Fatalf("incomplete CEM binding: %+v", first.CEM)
	}
}

func TestCWCX0002CauseDomainSeparation(t *testing.T) {
	subject := digestText("shared-subject")
	parserID, err := CauseIdentity(CauseParser, subject)
	if err != nil {
		t.Fatal(err)
	}
	oracleID, err := CauseIdentity(CauseOracle, subject)
	if err != nil {
		t.Fatal(err)
	}
	if parserID == oracleID {
		t.Fatal("cause kind was not domain-separated")
	}
	if _, err := CauseIdentity("model", subject); CodeOf(err) != "invalid-cause-kind" {
		t.Fatalf("unsupported kind error=%v", err)
	}
	if _, err := CauseIdentity(CauseSource, "ABC"); CodeOf(err) != "invalid-cause-subject" {
		t.Fatalf("invalid digest error=%v", err)
	}
}

func TestCWCX0003RejectsHostileMemberships(t *testing.T) {
	raw, evidence := testCEM(t, 3, false)
	subject := digestText("generator")
	valid := []Attribution{
		attribution(evidence[0], evidence[2], CauseGenerator, subject),
		attribution(evidence[1], evidence[2], CauseGenerator, subject),
	}
	cases := []struct {
		name string
		rows []Attribution
		code string
	}{
		{"unknown witness", []Attribution{{WitnessEvidenceID: "evidence:sha256:" + strings.Repeat("f", 64), Kind: CauseGenerator, CauseSubjectSHA256: subject, AttestationEvidenceID: evidence[0]}}, "unknown-witness"},
		{"unknown attestation", []Attribution{{WitnessEvidenceID: evidence[0], Kind: CauseGenerator, CauseSubjectSHA256: subject, AttestationEvidenceID: "evidence:sha256:" + strings.Repeat("f", 64)}}, "unknown-attestation"},
		{"self attestation", []Attribution{{WitnessEvidenceID: evidence[0], Kind: CauseGenerator, CauseSubjectSHA256: subject, AttestationEvidenceID: evidence[0]}, valid[1]}, "self-attestation"},
		{"duplicate membership", append(append([]Attribution(nil), valid...), valid[0]), "duplicate-membership"},
		{"singleton cause", valid[:1], "non-common-cause"},
		{"invalid kind", []Attribution{{WitnessEvidenceID: evidence[0], Kind: "model", CauseSubjectSHA256: subject, AttestationEvidenceID: evidence[0]}}, "invalid-cause-kind"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			report, err := Compile(raw, test.rows)
			if CodeOf(err) != test.code {
				t.Fatalf("error=%v code=%q want %q", err, CodeOf(err), test.code)
			}
			if !reflect.DeepEqual(report, Report{}) {
				t.Fatalf("failure returned partial report: %+v", report)
			}
		})
	}
}

func TestCWCX0004FiveCopiesCollapseToOne(t *testing.T) {
	raw, evidence := testCEM(t, 5, true)
	rows := sharedAttributions(evidence, CauseGenerator, digestText("one-generator"))
	report, err := Compile(raw, rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Causes) != 1 || len(report.Groups) != 1 {
		t.Fatalf("causes=%d groups=%d", len(report.Causes), len(report.Groups))
	}
	supported := report.Hunks[0]
	if supported.RawWitnessCount != 5 || supported.IndependenceUpperBound != 1 {
		t.Fatalf("supported assessment=%+v", supported)
	}
	if len(supported.AppliedCorrelationGroupIDs) != 1 || supported.AppliedCorrelationGroupIDs[0] != report.Groups[0].ID {
		t.Fatalf("applied groups=%v", supported.AppliedCorrelationGroupIDs)
	}
	if report.Profile != Profile || report.DeliveryStage != "experimental" || report.Claim != "UNPROVEN" || report.Assurance != "declared-common-causes-only" {
		t.Fatalf("promotion labels changed: %+v", report)
	}
}

func TestCWCX0004TransitiveAndDisconnectedCollapse(t *testing.T) {
	raw, evidence := testCEM(t, 4, false)
	rows := []Attribution{
		attribution(evidence[0], evidence[3], CauseGenerator, digestText("generator-a")),
		attribution(evidence[1], evidence[3], CauseGenerator, digestText("generator-a")),
		attribution(evidence[1], evidence[3], CausePremise, digestText("premise-b")),
		attribution(evidence[2], evidence[3], CausePremise, digestText("premise-b")),
	}
	report, err := Compile(raw, rows)
	if err != nil {
		t.Fatal(err)
	}
	wantTransitive := append([]string(nil), evidence[:3]...)
	sort.Strings(wantTransitive)
	if len(report.Groups) != 1 || !reflect.DeepEqual(report.Groups[0].WitnessEvidenceIDs, wantTransitive) {
		t.Fatalf("transitive groups=%+v", report.Groups)
	}
	if report.Hunks[0].IndependenceUpperBound != 2 {
		t.Fatalf("transitive component plus disconnected singleton=%+v", report.Hunks[0])
	}

	disconnected := []Attribution{
		attribution(evidence[0], evidence[2], CauseSource, digestText("source-a")),
		attribution(evidence[1], evidence[2], CauseSource, digestText("source-a")),
		attribution(evidence[2], evidence[0], CauseOracle, digestText("oracle-b")),
		attribution(evidence[3], evidence[0], CauseOracle, digestText("oracle-b")),
	}
	report, err = Compile(raw, disconnected)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Groups) != 2 || report.Hunks[0].IndependenceUpperBound != 2 {
		t.Fatalf("disconnected groups=%+v hunk=%+v", report.Groups, report.Hunks[0])
	}
}

func TestCWCX0005UniqueWitnessUpperBoundAndDisposition(t *testing.T) {
	raw, evidence := testCEM(t, 5, true)
	report, err := Compile(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Hunks[0].RawWitnessCount != 5 || report.Hunks[0].IndependenceUpperBound != 5 {
		t.Fatalf("repeated relation counted twice or undeclared independence promoted: %+v", report.Hunks[0])
	}
	if len(report.Groups) != 0 || len(report.Causes) != 0 {
		t.Fatalf("undeclared causes invented: %+v", report)
	}
	if got := report.Hunks[1]; got.Disposition != "unknown" || got.RawWitnessCount != 0 || got.IndependenceUpperBound != 0 {
		t.Fatalf("unknown hunk promoted: %+v", got)
	}
	if got := report.Hunks[2]; got.Disposition != "mechanical" || got.RawWitnessCount != 0 || got.IndependenceUpperBound != 0 {
		t.Fatalf("mechanical hunk promoted: %+v", got)
	}
	if len(evidence) != 5 {
		t.Fatal("fixture evidence changed")
	}
}

func TestCWCX0007PermutationGoldenAndVerification(t *testing.T) {
	raw, evidence := testCEM(t, 5, true)
	rows := sharedAttributions(evidence, CauseGenerator, digestText("one-generator"))
	first, err := Compile(raw, rows)
	if err != nil {
		t.Fatal(err)
	}
	for left, right := 0, len(rows)-1; left < right; left, right = left+1, right-1 {
		rows[left], rows[right] = rows[right], rows[left]
	}
	second, err := Compile(raw, rows)
	if err != nil {
		t.Fatal(err)
	}
	firstBytes, firstDigest, err := Canonical(first)
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, secondDigest, err := Canonical(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstBytes, secondBytes) || firstDigest != secondDigest {
		t.Fatal("attribution order changed canonical report")
	}
	golden, err := os.ReadFile("testdata/five-copies.report.json")
	if err != nil {
		t.Fatalf("golden unavailable: %v\n%s", err, firstBytes)
	}
	if !bytes.Equal(firstBytes, golden) {
		t.Fatalf("golden mismatch\n%s", firstBytes)
	}
	if err := Verify(raw, first); err != nil {
		t.Fatalf("valid report rejected: %v", err)
	}
	tampered := first
	tampered.Claim = "VALIDATED"
	if err := Verify(raw, tampered); CodeOf(err) != "report-mismatch" {
		t.Fatalf("tampered report error=%v", err)
	}
	if err := Verify(append(append([]byte(nil), raw...), '\n'), first); CodeOf(err) != "report-mismatch" {
		t.Fatalf("wrong CEM binding error=%v", err)
	}
	selfAttested := first
	selfAttested.Causes = append([]Cause(nil), first.Causes...)
	selfAttested.Causes[0].Members = append([]CauseMember(nil), first.Causes[0].Members...)
	selfAttested.Causes[0].Members[0].AttestationEvidenceID = selfAttested.Causes[0].Members[0].WitnessEvidenceID
	if err := Verify(raw, selfAttested); CodeOf(err) != "self-attestation" {
		t.Fatalf("self-attested report error=%v", err)
	}
}

func TestCWCX0008ReportExcludesSourceBodiesAndCausePreimages(t *testing.T) {
	raw, evidence := testCEM(t, 2, false)
	secret := "private-generator-name"
	report, err := Compile(raw, sharedAttributions(evidence, CauseGenerator, digestText(secret)))
	if err != nil {
		t.Fatal(err)
	}
	document, _, err := Canonical(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{secret, "evidence/0.txt", "evidence/1.txt"} {
		if bytes.Contains(document, []byte(forbidden)) {
			t.Fatalf("canonical report leaked %q", forbidden)
		}
	}
}

func TestCWCX0009AttributionBound(t *testing.T) {
	raw, _ := testCEM(t, 2, false)
	rows := make([]Attribution, MaxAttributions+1)
	if _, err := Compile(raw, rows); CodeOf(err) != "too-many-attributions" {
		t.Fatalf("bound error=%v", err)
	}
}

func TestCWCX0011MaximumAllocationRatchet(t *testing.T) {
	if raceInstrumented {
		t.Skip("race instrumentation changes allocation accounting")
	}
	raw, rows := benchmarkFixture(t, false)
	allocations := testing.AllocsPerRun(1, func() {
		report, err := Compile(raw, rows)
		if err != nil {
			t.Fatal(err)
		}
		runtime.KeepAlive(report)
	})
	if allocations > 250_000 {
		t.Fatalf("maximum disconnected compile allocations=%.0f want <=250000", allocations)
	}
}

func testCEM(t testing.TB, evidenceCount int, repeatedRelation bool) ([]byte, []string) {
	t.Helper()
	evidence := make([]any, 0, evidenceCount)
	basis := make([]any, 0, evidenceCount+1)
	ids := make([]string, 0, evidenceCount)
	for index := 0; index < evidenceCount; index++ {
		blobOID := digestText("blob-" + string(rune(index)))[0:40]
		spanDigest := digestText("span-" + string(rune(index)))
		span := wire.Span{Start: 0, End: 1}
		path := "evidence/" + hex.EncodeToString([]byte{byte(index)}) + ".txt"
		id := wire.EvidenceIdentity(blobOID, path, span, spanDigest)
		ids = append(ids, id)
		evidence = append(evidence, map[string]any{
			"id": id, "blobOid": blobOID, "path": path,
			"span": map[string]any{"start": 0, "end": 1}, "spanSha256": spanDigest,
		})
		basis = append(basis, map[string]any{"evidenceId": id, "relation": "implementation"})
	}
	if repeatedRelation && evidenceCount > 0 {
		basis = append(basis, map[string]any{"evidenceId": ids[0], "relation": "specification"})
	}
	hunks := make([]any, 0, (len(basis)+wire.MaxBases-1)/wire.MaxBases+2)
	for start := 0; start < len(basis); start += wire.MaxBases {
		end := start + wire.MaxBases
		if end > len(basis) {
			end = len(basis)
		}
		hunks = append(hunks, map[string]any{
			"id": prefixedDigest(wire.HunkPrefix, "supported-"+string(rune(start))), "path": "change.go",
			"oldRange":    map[string]any{"start": start + 1, "count": 1},
			"newRange":    map[string]any{"start": start + 1, "count": 1},
			"disposition": "supported", "reason": "evidence-backed", "basis": basis[start:end],
		})
	}
	hunks = append(hunks,
		map[string]any{
			"id": prefixedDigest(wire.HunkPrefix, "unknown"), "path": "change.go",
			"oldRange":    map[string]any{"start": evidenceCount + 1, "count": 1},
			"newRange":    map[string]any{"start": evidenceCount + 1, "count": 1},
			"disposition": "unknown", "reason": "no-evidence", "basis": []any{},
		},
		map[string]any{
			"id": prefixedDigest(wire.HunkPrefix, "mechanical"), "path": "change.go",
			"oldRange":    map[string]any{"start": evidenceCount + 2, "count": 1},
			"newRange":    map[string]any{"start": evidenceCount + 2, "count": 1},
			"disposition": "mechanical", "reason": "whitespace-only", "basis": []any{},
		},
	)
	value := map[string]any{
		"spec": wire.Spec01, "baseRevision": strings.Repeat("a", 40),
		"patchSha256": digestText("patch"), "evidence": evidence, "hunks": hunks,
	}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw, ids
}

func attribution(evidenceID, attestationID string, kind CauseKind, subject string) Attribution {
	return Attribution{
		WitnessEvidenceID: evidenceID, Kind: kind, CauseSubjectSHA256: subject,
		AttestationEvidenceID: attestationID,
	}
}

func sharedAttributions(evidence []string, kind CauseKind, subject string) []Attribution {
	result := make([]Attribution, 0, len(evidence))
	for index, evidenceID := range evidence {
		attestationID := evidence[(index+1)%len(evidence)]
		result = append(result, attribution(evidenceID, attestationID, kind, subject))
	}
	return result
}

func digestText(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func prefixedDigest(prefix, value string) string { return prefix + digestText(value) }
