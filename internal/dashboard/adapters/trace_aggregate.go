package adapters

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/dashboard/model"
)

const traceStoreDomain = "trace-store/0"

type TraceMember struct {
	Revision      string `json:"revision"`
	ContentSHA256 string `json:"contentSha256"`
	ByteCount     string `json:"byteCount"`
}

type VerifiedTraceMember struct {
	Summary       TraceSummary
	ContentSHA256 string
	ByteCount     uint64
	Start         string
	End           string
}

type TraceAggregate struct {
	Members             []TraceMember
	ByteCount           uint64
	RetainedRows        uint64
	ContentSHA256       string
	ObservationStart    *string
	ObservationEnd      *string
	repositoryWitnesses []model.RepositoryWitness
}

func AggregateTraceMembers(snapshotHead model.RepositoryWitness, input []VerifiedTraceMember) (TraceAggregate, AdapterIssueCode) {
	head, ok := normalizeWitnesses([]model.RepositoryWitness{snapshotHead})
	if !ok || len(head) != 1 || head[0].Kind != "SNAPSHOT_HEAD" || head[0].ObjectType != "commit" || head[0].Revision != nil {
		return TraceAggregate{}, IssueSourceInvalidIdentity
	}
	members := append([]VerifiedTraceMember(nil), input...)
	sort.Slice(members, func(left, right int) bool { return members[left].Summary.Revision < members[right].Summary.Revision })
	result := TraceAggregate{Members: make([]TraceMember, 0, len(members))}
	witnesses := map[string]model.RepositoryWitness{witnessKey(head[0]): head[0]}
	traceIDs := make(map[string]struct{})
	var earliest, latest string
	for index, member := range members {
		if index > 0 && members[index-1].Summary.Revision == member.Summary.Revision {
			return TraceAggregate{}, IssueSourceInvalidIdentity
		}
		memberWitnesses, witnessOK := validateAggregateMemberWitnesses(head[0], member.Summary)
		if !witnessOK || !validObjectID(member.Summary.Revision, member.Summary.ObjectFormat) ||
			member.Summary.ObjectFormat != head[0].ObjectFormat ||
			!validObjectID(member.Summary.TreeRevision, member.Summary.ObjectFormat) ||
			!validSHA256Digest(member.ContentSHA256) || !validTimestamp(member.Start) ||
			!validTimestamp(member.End) || member.Start > member.End {
			return TraceAggregate{}, IssueSourceInvalidIdentity
		}
		if member.ByteCount > maxTraceStoreBytes || result.ByteCount > maxTraceStoreBytes-member.ByteCount ||
			member.Summary.RetainedRows > maxTraceRows-result.RetainedRows {
			return TraceAggregate{}, IssueTraceStoreBound
		}
		if !validTraceSummaryCounts(member.Summary) {
			return TraceAggregate{}, IssueSourceInvalidSchema
		}
		result.ByteCount += member.ByteCount
		result.RetainedRows += member.Summary.RetainedRows
		result.Members = append(result.Members, TraceMember{
			Revision: member.Summary.Revision, ContentSHA256: member.ContentSHA256,
			ByteCount: strconv.FormatUint(member.ByteCount, 10),
		})
		if earliest == "" || member.Start < earliest {
			earliest = member.Start
		}
		if latest == "" || member.End > latest {
			latest = member.End
		}
		for _, witness := range memberWitnesses {
			witnesses[witnessKey(witness)] = witness
		}
		for index, traceID := range member.Summary.traceIDs {
			if !lowerHex(traceID, 64) || (index > 0 && member.Summary.traceIDs[index-1] >= traceID) {
				return TraceAggregate{}, IssueSourceInvalidIdentity
			}
			if _, duplicate := traceIDs[traceID]; duplicate {
				return TraceAggregate{}, IssueVerifierRejected
			}
			traceIDs[traceID] = struct{}{}
		}
	}
	preimage := make([]map[string]any, len(result.Members))
	for index, member := range result.Members {
		preimage[index] = map[string]any{
			"revision": member.Revision, "contentSha256": member.ContentSHA256,
			"byteCount": member.ByteCount,
		}
	}
	encoded, err := contextindex.CanonicalJSON(preimage)
	if err != nil {
		return TraceAggregate{}, IssueVerifierRejected
	}
	digest := sha256.New()
	digest.Write([]byte(traceStoreDomain))
	digest.Write([]byte{0})
	digest.Write(encoded)
	result.ContentSHA256 = "sha256:" + hex.EncodeToString(digest.Sum(nil))
	if earliest != "" {
		result.ObservationStart = &earliest
		result.ObservationEnd = &latest
	}
	for _, witness := range witnesses {
		result.repositoryWitnesses = append(result.repositoryWitnesses, witness)
	}
	result.repositoryWitnesses, ok = normalizeWitnesses(result.repositoryWitnesses)
	if !ok {
		return TraceAggregate{}, IssueSourceInvalidIdentity
	}
	return result, ""
}

func validateAggregateMemberWitnesses(snapshotHead model.RepositoryWitness, summary TraceSummary) ([]model.RepositoryWitness, bool) {
	witnesses, ok := normalizeWitnesses(summary.repositoryWitnesses)
	if !ok {
		return nil, false
	}
	var headCount, revisionCount int
	for _, witness := range witnesses {
		if witness.ObjectFormat != summary.ObjectFormat {
			return nil, false
		}
		switch witness.Kind {
		case "SNAPSHOT_HEAD":
			if witnessKey(witness) != witnessKey(snapshotHead) {
				return nil, false
			}
			headCount++
		case "TRACE_REVISION":
			if witness.Revision == nil || witness.ObjectID != summary.Revision || *witness.Revision != summary.Revision {
				return nil, false
			}
			revisionCount++
		case "TRACE_PATH_OBJECT":
			if witness.Revision == nil || *witness.Revision != summary.Revision {
				return nil, false
			}
		}
	}
	return witnesses, headCount == 1 && revisionCount == 1
}

func witnessKey(witness model.RepositoryWitness) string {
	encoded, err := canonicalWitnessJSON(witness)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func validTraceSummaryCounts(summary TraceSummary) bool {
	if summary.RetainedRows != uint64(len(summary.traceIDs)) {
		return false
	}
	var total uint64
	previous := ""
	for _, outcome := range summary.Outcomes {
		if outcome.Count == 0 || (outcome.Outcome != "blocked" && outcome.Outcome != "failed" && outcome.Outcome != "passed") ||
			(previous != "" && previous >= outcome.Outcome) || total > summary.RetainedRows-outcome.Count {
			return false
		}
		total += outcome.Count
		previous = outcome.Outcome
	}
	return total == summary.RetainedRows
}

func validSHA256Digest(value string) bool {
	return len(value) == len("sha256:")+64 && value[:len("sha256:")] == "sha256:" && lowerHex(value[len("sha256:"):], 64)
}

func validTimestamp(value string) bool {
	parsed, err := time.Parse("2006-01-02T15:04:05.000000000Z", value)
	return err == nil && parsed.Location() == time.UTC && parsed.Format("2006-01-02T15:04:05.000000000Z") == value
}
