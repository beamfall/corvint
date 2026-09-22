package adapters

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/dashboard/model"
)

func TestAggregateTraceMembersUsesFrozenIdentityAndBounds(t *testing.T) {
	secondRevision := strings.Repeat("d", 40)
	first := TraceSummary{
		Revision: testRevision, TreeRevision: testTree, ObjectFormat: "sha1", RetainedRows: 1,
		Outcomes:            []TraceOutcomeCount{{Outcome: "passed", Count: 1}},
		repositoryWitnesses: aggregateMemberWitnesses(testRevision),
		traceIDs:            []string{strings.Repeat("1", 64)},
	}
	second := TraceSummary{
		Revision: secondRevision, TreeRevision: strings.Repeat("e", 40), ObjectFormat: "sha1", RetainedRows: 1,
		Outcomes:            []TraceOutcomeCount{{Outcome: "passed", Count: 1}},
		repositoryWitnesses: aggregateMemberWitnesses(secondRevision),
		traceIDs:            []string{strings.Repeat("2", 64)},
	}
	aggregate, code := AggregateTraceMembers(snapshotHeadWitness(), []VerifiedTraceMember{
		{Summary: second, ContentSHA256: "sha256:" + strings.Repeat("2", 64), ByteCount: 9, Start: "2026-08-23T12:00:01.000000000Z", End: "2026-08-23T12:00:03.000000000Z"},
		{Summary: first, ContentSHA256: "sha256:" + strings.Repeat("1", 64), ByteCount: 7, Start: "2026-08-23T12:00:00.000000000Z", End: "2026-08-23T12:00:02.000000000Z"},
	})
	if code != "" {
		t.Fatal(code)
	}
	if aggregate.ByteCount != 16 || aggregate.ObservationStart == nil || *aggregate.ObservationStart != "2026-08-23T12:00:00.000000000Z" || aggregate.ObservationEnd == nil || *aggregate.ObservationEnd != "2026-08-23T12:00:03.000000000Z" {
		t.Fatalf("aggregate = %+v", aggregate)
	}
	if aggregate.Members[0].Revision != testRevision || aggregate.Members[0].ByteCount != "7" || !validSHA256Digest(aggregate.ContentSHA256) {
		t.Fatalf("members/hash = %+v %s", aggregate.Members, aggregate.ContentSHA256)
	}
	wantWitnesses, ok := normalizeWitnesses(append(aggregateMemberWitnesses(testRevision), aggregateMemberWitnesses(secondRevision)...))
	if !ok || !reflect.DeepEqual(aggregate.repositoryWitnesses, wantWitnesses) {
		t.Fatalf("witnesses = %v, want %v", aggregate.repositoryWitnesses, wantWitnesses)
	}
}

func TestAggregateTraceMembersRejectsCrossFileTraceIDDuplicates(t *testing.T) {
	duplicateID := strings.Repeat("1", 64)
	first := TraceSummary{
		Revision: testRevision, TreeRevision: testTree, ObjectFormat: "sha1", RetainedRows: 1,
		Outcomes: []TraceOutcomeCount{{Outcome: "passed", Count: 1}},
		traceIDs: []string{duplicateID}, repositoryWitnesses: aggregateMemberWitnesses(testRevision),
	}
	secondRevision := strings.Repeat("d", 40)
	second := TraceSummary{
		Revision: secondRevision, TreeRevision: strings.Repeat("e", 40), ObjectFormat: "sha1", RetainedRows: 1,
		Outcomes: []TraceOutcomeCount{{Outcome: "passed", Count: 1}},
		traceIDs: []string{duplicateID}, repositoryWitnesses: aggregateMemberWitnesses(secondRevision),
	}
	members := []VerifiedTraceMember{
		{Summary: first, ContentSHA256: "sha256:" + strings.Repeat("1", 64), ByteCount: 1, Start: "2026-08-23T12:00:00.000000000Z", End: "2026-08-23T12:00:00.000000000Z"},
		{Summary: second, ContentSHA256: "sha256:" + strings.Repeat("2", 64), ByteCount: 1, Start: "2026-08-23T12:00:00.000000000Z", End: "2026-08-23T12:00:00.000000000Z"},
	}
	if _, code := AggregateTraceMembers(snapshotHeadWitness(), members); code != IssueVerifierRejected {
		t.Fatalf("cross-file duplicate code = %s", code)
	}
}

func TestAggregateTraceMembersEnforcesStoreWideRowLimit(t *testing.T) {
	ids := make([]string, maxTraceRows+1)
	for index := range ids {
		ids[index] = fmt.Sprintf("%064x", index+1)
	}
	summary := TraceSummary{
		Revision: testRevision, TreeRevision: testTree, ObjectFormat: "sha1",
		RetainedRows: uint64(len(ids)), traceIDs: ids, repositoryWitnesses: aggregateMemberWitnesses(testRevision),
	}
	member := VerifiedTraceMember{
		Summary: summary, ContentSHA256: "sha256:" + strings.Repeat("1", 64), ByteCount: 1,
		Start: "2026-08-23T12:00:00.000000000Z", End: "2026-08-23T12:00:00.000000000Z",
	}
	if _, code := AggregateTraceMembers(snapshotHeadWitness(), []VerifiedTraceMember{member}); code != IssueTraceStoreBound {
		t.Fatalf("aggregate row bound code = %s", code)
	}
}

func TestAggregateTraceMembersEmptyIsMeasuredEmptyAggregate(t *testing.T) {
	aggregate, code := AggregateTraceMembers(snapshotHeadWitness(), nil)
	if code != "" || aggregate.ByteCount != 0 || aggregate.ObservationStart != nil || aggregate.ObservationEnd != nil || len(aggregate.Members) != 0 || !validSHA256Digest(aggregate.ContentSHA256) {
		t.Fatalf("empty aggregate = %+v, %s", aggregate, code)
	}
	if !reflect.DeepEqual(aggregate.repositoryWitnesses, []model.RepositoryWitness{snapshotHeadWitness()}) {
		t.Fatalf("empty witnesses = %+v", aggregate.repositoryWitnesses)
	}
}

func TestAggregateTraceMembersRejectsDuplicatesAndBadBounds(t *testing.T) {
	summary := TraceSummary{Revision: testRevision, TreeRevision: testTree, ObjectFormat: "sha1", repositoryWitnesses: aggregateMemberWitnesses(testRevision)}
	valid := VerifiedTraceMember{
		Summary: summary, ContentSHA256: "sha256:" + strings.Repeat("1", 64), ByteCount: 1,
		Start: "2026-08-23T12:00:00.000000000Z", End: "2026-08-23T12:00:00.000000000Z",
	}
	if _, code := AggregateTraceMembers(snapshotHeadWitness(), []VerifiedTraceMember{valid, valid}); code != IssueSourceInvalidIdentity {
		t.Fatalf("duplicate code = %s", code)
	}
	valid.ByteCount = maxTraceStoreBytes + 1
	if _, code := AggregateTraceMembers(snapshotHeadWitness(), []VerifiedTraceMember{valid}); code != IssueTraceStoreBound {
		t.Fatalf("bound code = %s", code)
	}
}

func TestAggregateTraceMembersRejectsContradictoryNormalizedSummary(t *testing.T) {
	summary := TraceSummary{
		Revision: testRevision, TreeRevision: testTree, ObjectFormat: "sha1", RetainedRows: 1,
		traceIDs:            []string{strings.Repeat("1", 64)},
		Outcomes:            []TraceOutcomeCount{{Outcome: "passed", Count: 2}},
		repositoryWitnesses: aggregateMemberWitnesses(testRevision),
	}
	member := VerifiedTraceMember{
		Summary: summary, ContentSHA256: "sha256:" + strings.Repeat("1", 64), ByteCount: 1,
		Start: "2026-08-23T12:00:00.000000000Z", End: "2026-08-23T12:00:00.000000000Z",
	}
	if _, code := AggregateTraceMembers(snapshotHeadWitness(), []VerifiedTraceMember{member}); code != IssueSourceInvalidSchema {
		t.Fatalf("contradictory summary code = %s", code)
	}
}

func TestAggregateTraceMembersClassifiesContradictionsBeforeEmission(t *testing.T) {
	valid := VerifiedTraceMember{
		Summary: TraceSummary{
			Revision: testRevision, TreeRevision: testTree, ObjectFormat: "sha1", RetainedRows: 1,
			Outcomes: []TraceOutcomeCount{{Outcome: "passed", Count: 1}}, traceIDs: []string{strings.Repeat("1", 64)},
			repositoryWitnesses: aggregateMemberWitnesses(testRevision),
		},
		ContentSHA256: "sha256:" + strings.Repeat("1", 64), ByteCount: 1,
		Start: "2026-08-23T12:00:00.000000000Z", End: "2026-08-23T12:00:00.000000000Z",
	}
	tests := []struct {
		name   string
		mutate func(*VerifiedTraceMember)
		want   AdapterIssueCode
	}{
		{"digest identity", func(member *VerifiedTraceMember) { member.ContentSHA256 = "sha256:bad" }, IssueSourceInvalidIdentity},
		{"timestamp identity", func(member *VerifiedTraceMember) { member.End = "2026-08-23T12:00:00Z" }, IssueSourceInvalidIdentity},
		{"byte bound", func(member *VerifiedTraceMember) { member.ByteCount = maxTraceStoreBytes + 1 }, IssueTraceStoreBound},
		{"row bound", func(member *VerifiedTraceMember) { member.Summary.RetainedRows = maxTraceRows + 1 }, IssueTraceStoreBound},
		{"count schema", func(member *VerifiedTraceMember) { member.Summary.Outcomes[0].Count = 2 }, IssueSourceInvalidSchema},
		{"trace identity", func(member *VerifiedTraceMember) { member.Summary.traceIDs[0] = strings.Repeat("A", 64) }, IssueSourceInvalidIdentity},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			member := valid
			member.Summary.Outcomes = append([]TraceOutcomeCount(nil), valid.Summary.Outcomes...)
			member.Summary.traceIDs = append([]string(nil), valid.Summary.traceIDs...)
			test.mutate(&member)
			if _, code := AggregateTraceMembers(snapshotHeadWitness(), []VerifiedTraceMember{member}); code != test.want {
				t.Fatalf("code = %s, want %s", code, test.want)
			}
		})
	}
}

func TestAggregateTraceMembersRejectsWrongSnapshotWitness(t *testing.T) {
	summary := TraceSummary{
		Revision: testRevision, TreeRevision: testTree, ObjectFormat: "sha1",
		repositoryWitnesses: aggregateMemberWitnesses(testRevision),
	}
	member := VerifiedTraceMember{
		Summary: summary, ContentSHA256: "sha256:" + strings.Repeat("1", 64), ByteCount: 1,
		Start: "2026-08-23T12:00:00.000000000Z", End: "2026-08-23T12:00:00.000000000Z",
	}
	wrongHead := snapshotHeadWitness()
	wrongHead.ObjectID = strings.Repeat("f", 40)
	if _, code := AggregateTraceMembers(wrongHead, []VerifiedTraceMember{member}); code != IssueTraceStoreBound && code != IssueSourceInvalidIdentity {
		t.Fatalf("wrong snapshot code = %s", code)
	}
}

func aggregateMemberWitnesses(revision string) []model.RepositoryWitness {
	return []model.RepositoryWitness{
		snapshotHeadWitness(),
		{Kind: "TRACE_REVISION", ObjectFormat: "sha1", ObjectID: revision, ObjectType: "commit", Revision: pointer(revision)},
	}
}
