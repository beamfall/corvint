package witnesscollapse

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

type disjointSet struct {
	parent []int
	rank   []uint8
}

// Compile parses exact CEM bytes and collapses only explicitly declared common
// causes. It performs structural parsing, not repository or canonical CEM
// verification.
func Compile(rawCEM []byte, attributions []Attribution) (Report, error) {
	document, err := wire.ParseMap(rawCEM)
	if err != nil {
		return Report{}, err
	}
	if len(attributions) > MaxAttributions {
		return Report{}, failure("too-many-attributions", "memberships exceed the %d-item bound", MaxAttributions)
	}

	evidence := make(map[string]int, len(document.Evidence))
	evidenceIDs := make([]string, 0, len(document.Evidence))
	for index, item := range document.Evidence {
		evidence[item.ID] = index
		evidenceIDs = append(evidenceIDs, item.ID)
	}

	ordered := append([]Attribution(nil), attributions...)
	sort.Slice(ordered, func(left, right int) bool {
		a, b := ordered[left], ordered[right]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.CauseSubjectSHA256 != b.CauseSubjectSHA256 {
			return a.CauseSubjectSHA256 < b.CauseSubjectSHA256
		}
		if a.WitnessEvidenceID != b.WitnessEvidenceID {
			return a.WitnessEvidenceID < b.WitnessEvidenceID
		}
		return a.AttestationEvidenceID < b.AttestationEvidenceID
	})

	causes := make([]Cause, 0, min(len(ordered)/2, MaxCauses))
	for start := 0; start < len(ordered); {
		first := ordered[start]
		causeID, identityErr := CauseIdentity(first.Kind, first.CauseSubjectSHA256)
		if identityErr != nil {
			return Report{}, identityErr
		}
		end := start + 1
		for end < len(ordered) && ordered[end].Kind == first.Kind && ordered[end].CauseSubjectSHA256 == first.CauseSubjectSHA256 {
			end++
		}
		if len(causes) == MaxCauses {
			return Report{}, failure("too-many-causes", "common causes exceed the %d-item bound", MaxCauses)
		}
		members := make([]CauseMember, 0, end-start)
		for index := start; index < end; index++ {
			attribution := ordered[index]
			if _, ok := evidence[attribution.WitnessEvidenceID]; !ok {
				return Report{}, failure("unknown-witness", "membership references evidence absent from the CEM")
			}
			if _, ok := evidence[attribution.AttestationEvidenceID]; !ok {
				return Report{}, failure("unknown-attestation", "membership attestation is absent from the CEM")
			}
			if attribution.AttestationEvidenceID == attribution.WitnessEvidenceID {
				return Report{}, failure("self-attestation", "membership attestation must differ from its witness")
			}
			if index != start && ordered[index-1].WitnessEvidenceID == attribution.WitnessEvidenceID {
				return Report{}, failure("duplicate-membership", "common cause repeats a witness membership")
			}
			members = append(members, CauseMember{
				WitnessEvidenceID: attribution.WitnessEvidenceID, AttestationEvidenceID: attribution.AttestationEvidenceID,
			})
		}
		if len(members) < 2 {
			return Report{}, failure("non-common-cause", "common cause must name at least two distinct witnesses")
		}
		causes = append(causes, Cause{
			ID: causeID, Kind: first.Kind, SubjectSHA256: first.CauseSubjectSHA256, Members: members,
		})
		start = end
	}
	sort.Slice(causes, func(left, right int) bool { return causes[left].ID < causes[right].ID })

	sets := newDisjointSet(len(document.Evidence))
	for _, cause := range causes {
		first := evidence[cause.Members[0].WitnessEvidenceID]
		for _, member := range cause.Members[1:] {
			sets.union(first, evidence[member.WitnessEvidenceID])
		}
	}

	componentMembers := make(map[int][]string)
	for index, evidenceID := range evidenceIDs {
		root := sets.find(index)
		componentMembers[root] = append(componentMembers[root], evidenceID)
	}
	componentCauses := make(map[int][]string)
	for _, cause := range causes {
		root := sets.find(evidence[cause.Members[0].WitnessEvidenceID])
		componentCauses[root] = append(componentCauses[root], cause.ID)
	}

	groups := make([]CorrelationGroup, 0, len(componentCauses))
	witnessGroup := make([]string, len(document.Evidence))
	for root, members := range componentMembers {
		if len(members) < 2 {
			continue
		}
		sort.Strings(members)
		groupCauses := componentCauses[root]
		sort.Strings(groupCauses)
		groupID := groupIdentity(members, groupCauses)
		groups = append(groups, CorrelationGroup{
			ID: groupID, WitnessEvidenceIDs: append([]string(nil), members...),
			CauseIDs: append([]string(nil), groupCauses...),
		})
		for _, witness := range members {
			witnessGroup[evidence[witness]] = groupID
		}
	}
	sort.Slice(groups, func(left, right int) bool { return groups[left].ID < groups[right].ID })

	hunks := make([]HunkAssessment, 0, len(document.Hunks))
	for _, hunk := range document.Hunks {
		witnessSet := make(map[string]struct{}, len(hunk.Basis))
		for _, basis := range hunk.Basis {
			witnessSet[basis.EvidenceID] = struct{}{}
		}
		witnesses := sortedSetKeys(witnessSet)
		components := make(map[int]struct{}, len(witnesses))
		groupFrequency := make(map[string]int)
		for _, witness := range witnesses {
			index := evidence[witness]
			components[sets.find(index)] = struct{}{}
			if groupID := witnessGroup[index]; groupID != "" {
				groupFrequency[groupID]++
			}
		}
		applied := make([]string, 0, len(groupFrequency))
		for groupID, count := range groupFrequency {
			if count >= 2 {
				applied = append(applied, groupID)
			}
		}
		sort.Strings(applied)
		hunks = append(hunks, HunkAssessment{
			HunkID: hunk.ID, Disposition: hunk.Disposition,
			WitnessEvidenceIDs: witnesses, RawWitnessCount: len(witnesses),
			IndependenceUpperBound: len(components), AppliedCorrelationGroupIDs: applied,
		})
	}

	mapDigest := sha256.Sum256(rawCEM)
	return Report{
		Profile: Profile, DeliveryStage: DeliveryStage, Claim: Claim, Assurance: Assurance,
		CEM: CEMBinding{
			Spec: document.Spec, MapSHA256: hex.EncodeToString(mapDigest[:]),
			BaseRevision: document.BaseRevision, PatchSHA256: document.PatchSha256,
		},
		Causes: causes, Groups: groups, Hunks: hunks,
	}, nil
}

func sortedMapKeys[T any](values map[string]T) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func sortedSetKeys(values map[string]struct{}) []string { return sortedMapKeys(values) }

func newDisjointSet(size int) *disjointSet {
	result := &disjointSet{parent: make([]int, size), rank: make([]uint8, size)}
	for index := range result.parent {
		result.parent[index] = index
	}
	return result
}

func (sets *disjointSet) find(value int) int {
	parent := sets.parent[value]
	if parent != value {
		sets.parent[value] = sets.find(parent)
	}
	return sets.parent[value]
}

func (sets *disjointSet) union(left, right int) {
	leftRoot, rightRoot := sets.find(left), sets.find(right)
	if leftRoot == rightRoot {
		return
	}
	leftRank, rightRank := sets.rank[leftRoot], sets.rank[rightRoot]
	if leftRank < rightRank || leftRank == rightRank && rightRoot < leftRoot {
		leftRoot, rightRoot = rightRoot, leftRoot
		leftRank, rightRank = rightRank, leftRank
	}
	sets.parent[rightRoot] = leftRoot
	if leftRank == rightRank {
		sets.rank[leftRoot]++
	}
}
