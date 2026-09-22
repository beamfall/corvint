package playwrightminimize

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"
)

var (
	gitOIDPattern = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)
	digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

func BuildPlan(request Request) (Plan, error) {
	if err := validateRequest(request); err != nil {
		return Plan{}, err
	}
	plan := Plan{Schema: Schema, Request: cloneRequest(request), Blockers: qualificationBlockers(request.Qualification)}
	appendBaselineTrials(&plan)
	remainingGroups := (request.Limits.MaxTrials - len(plan.Trials)) / request.Limits.Repetitions
	ordered, orderedEnumerated := orderedCandidates(request.Predecessors, remainingGroups)
	load, loadEnumerated := loadCandidates(request.LoadMembers, remainingGroups)
	orderedTake, loadTake := fairCandidateAllocation(len(ordered), len(load), remainingGroups)
	appendCandidates(&plan, TrialOrdered, ordered[:orderedTake], request.OriginalFailure.Identity.WorkerTopology)
	appendCandidates(&plan, TrialLoad, load[:loadTake], request.LoadTopology)
	plan.OrderedComplete = orderedEnumerated && orderedTake == len(ordered)
	plan.LoadComplete = loadEnumerated && loadTake == len(load)
	plan.Complete = plan.OrderedComplete && plan.LoadComplete
	plan.Digest = planDigest(plan)
	return plan, nil
}

func validateRequest(request Request) error {
	if strings.TrimSpace(request.Target) == "" || request.Limits.MaxTrials < 2*request.Limits.Repetitions || request.Limits.MaxTrials > 4096 || request.Limits.Repetitions < 1 || request.Limits.Repetitions > 16 || request.Limits.WallClock <= 0 || request.Limits.WallClock > 24*time.Hour {
		return errors.New("invalid minimization bounds")
	}
	if len(request.Predecessors) > 8 || len(request.LoadMembers) > 8 || !uniqueMembers(request.Target, request.Predecessors) || !uniqueMembers(request.Target, request.LoadMembers) {
		return errors.New("invalid minimization universe")
	}
	if err := validateFixedIdentity(request.OriginalFailure.Identity.Fixed); err != nil {
		return err
	}
	if !reflect.DeepEqual(request.OriginalFailure.Identity.Fixed, request.IsolatedPass.Identity.Fixed) {
		return errors.New("baseline identity mismatch")
	}
	if request.OriginalFailure.Outcome != OutcomeFailed || len(request.OriginalFailure.Failures) == 0 || request.IsolatedPass.Outcome != OutcomePassed || len(request.IsolatedPass.Failures) != 0 {
		return errors.New("invalid baseline outcomes")
	}
	if !digestPattern.MatchString(request.OriginalFailure.ReceiptDigest) || !digestPattern.MatchString(request.IsolatedPass.ReceiptDigest) || request.OriginalFailure.ReceiptDigest == request.IsolatedPass.ReceiptDigest {
		return errors.New("invalid baseline receipt digest")
	}
	expectedOrder := append(slices.Clone(request.Predecessors), request.Target)
	if !slices.Equal(request.OriginalFailure.Identity.Order, expectedOrder) || !slices.Equal(request.IsolatedPass.Identity.Order, []string{request.Target}) {
		return errors.New("baseline order mismatch")
	}
	if !validTopology(request.OriginalFailure.Identity.WorkerTopology) || !validTopology(request.IsolatedPass.Identity.WorkerTopology) || !validTopology(request.LoadTopology) {
		return errors.New("invalid worker topology")
	}
	if strings.TrimSpace(request.ResetPolicy.Name) == "" || !digestPattern.MatchString(request.ResetPolicy.Digest) {
		return errors.New("invalid reset policy")
	}
	for _, failure := range request.OriginalFailure.Failures {
		if !validFailure(failure) {
			return errors.New("invalid baseline failure evidence")
		}
	}
	return nil
}

func fairCandidateAllocation(ordered, load, limit int) (int, int) {
	orderedTake, loadTake := 0, 0
	for orderedTake+loadTake < limit {
		progress := false
		if orderedTake < ordered {
			orderedTake++
			progress = true
		}
		if orderedTake+loadTake < limit && loadTake < load {
			loadTake++
			progress = true
		}
		if !progress {
			break
		}
	}
	return orderedTake, loadTake
}

func validateFixedIdentity(identity FixedIdentity) error {
	if !gitOIDPattern.MatchString(identity.TestRevision) || !gitOIDPattern.MatchString(identity.ApplicationRevision) {
		return errors.New("invalid test or application revision")
	}
	for _, digest := range []string{identity.ConfigDigest, identity.FixtureDigest, identity.SeedIdentity, identity.ApplicationAttestationDigest} {
		if !digestPattern.MatchString(digest) {
			return errors.New("invalid identity digest")
		}
	}
	for _, value := range []string{identity.Runner, identity.RunnerVersion, identity.Browser, identity.BrowserVersion, identity.Project, identity.FixtureSchema} {
		if strings.TrimSpace(value) == "" {
			return errors.New("incomplete run identity")
		}
	}
	return nil
}

func validTopology(topology WorkerTopology) bool {
	return topology.Workers > 0 && strings.TrimSpace(topology.Policy) != ""
}

func uniqueMembers(target string, members []string) bool {
	seen := map[string]bool{target: true}
	for _, member := range members {
		if strings.TrimSpace(member) == "" || seen[member] {
			return false
		}
		seen[member] = true
	}
	return true
}

func qualificationBlockers(qualification Qualification) []string {
	var blockers []string
	if !qualification.RunnerQualified || !digestPattern.MatchString(qualification.RunnerReceiptDigest) {
		blockers = append(blockers, "runner-qualification-missing")
	}
	if !qualification.StabilityQualified || !digestPattern.MatchString(qualification.StabilityReceiptDigest) {
		blockers = append(blockers, "stability-identity-missing")
	}
	if !qualification.ApplicationAttested || !digestPattern.MatchString(qualification.ApplicationReceiptDigest) {
		blockers = append(blockers, "application-attestation-missing")
	}
	return blockers
}

func appendBaselineTrials(plan *Plan) {
	request := plan.Request
	for repetition := 1; repetition <= request.Limits.Repetitions; repetition++ {
		appendTrial(plan, TrialReproduction, "original", request.Predecessors, request.OriginalFailure.Identity, repetition)
	}
	for repetition := 1; repetition <= request.Limits.Repetitions; repetition++ {
		appendTrial(plan, TrialIsolation, "isolated", nil, request.IsolatedPass.Identity, repetition)
	}
}

func appendCandidates(plan *Plan, kind TrialKind, candidates [][]string, topology WorkerTopology) {
	for candidateIndex, context := range candidates {
		candidateID := fmt.Sprintf("%s-%03d", kind, candidateIndex+1)
		order := append(slices.Clone(context), plan.Request.Target)
		identity := RunIdentity{Fixed: plan.Request.OriginalFailure.Identity.Fixed, Order: order, WorkerTopology: topology}
		for repetition := 1; repetition <= plan.Request.Limits.Repetitions; repetition++ {
			appendTrial(plan, kind, candidateID, context, identity, repetition)
		}
	}
}

func appendTrial(plan *Plan, kind TrialKind, candidateID string, context []string, identity RunIdentity, repetition int) {
	ordinal := len(plan.Trials) + 1
	plan.Trials = append(plan.Trials, Trial{
		ID: fmt.Sprintf("trial-%04d", ordinal), Ordinal: ordinal, CandidateID: candidateID,
		Repetition: repetition, Kind: kind, Context: slices.Clone(context), Identity: cloneIdentity(identity), ResetPolicy: plan.Request.ResetPolicy,
	})
}

func orderedCandidates(predecessors []string, limit int) ([][]string, bool) {
	if len(predecessors) == 0 {
		return nil, true
	}
	var candidates [][]string
	complete := enumeratePermutations(predecessors, limit+1, func(candidate []string) bool {
		candidates = append(candidates, candidate)
		return len(candidates) <= limit
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
		complete = false
	}
	return candidates, complete
}

func loadCandidates(members []string, limit int) ([][]string, bool) {
	if len(members) == 0 {
		return nil, true
	}
	var candidates [][]string
	complete := enumerateCombinations(members, limit+1, func(candidate []string) bool {
		candidates = append(candidates, candidate)
		return len(candidates) <= limit
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
		complete = false
	}
	return candidates, complete
}

func enumeratePermutations(members []string, limit int, visit func([]string) bool) bool {
	for size := 1; size <= len(members); size++ {
		used := make([]bool, len(members))
		candidate := make([]string, 0, size)
		var walk func() bool
		walk = func() bool {
			if len(candidate) == size {
				return visit(slices.Clone(candidate))
			}
			for i, member := range members {
				if used[i] {
					continue
				}
				used[i] = true
				candidate = append(candidate, member)
				if !walk() {
					return false
				}
				candidate = candidate[:len(candidate)-1]
				used[i] = false
			}
			return true
		}
		if !walk() {
			return false
		}
	}
	_ = limit
	return true
}

func enumerateCombinations(members []string, limit int, visit func([]string) bool) bool {
	for size := 1; size <= len(members); size++ {
		candidate := make([]string, 0, size)
		var walk func(int) bool
		walk = func(start int) bool {
			if len(candidate) == size {
				return visit(slices.Clone(candidate))
			}
			for i := start; i < len(members); i++ {
				candidate = append(candidate, members[i])
				if !walk(i + 1) {
					return false
				}
				candidate = candidate[:len(candidate)-1]
			}
			return true
		}
		if !walk(0) {
			return false
		}
	}
	_ = limit
	return true
}

func planDigest(plan Plan) string {
	plan.Digest = ""
	encoded, _ := json.Marshal(plan)
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func cloneRequest(request Request) Request {
	request.Predecessors = slices.Clone(request.Predecessors)
	request.LoadMembers = slices.Clone(request.LoadMembers)
	request.OriginalFailure.Identity = cloneIdentity(request.OriginalFailure.Identity)
	request.OriginalFailure.Failures = slices.Clone(request.OriginalFailure.Failures)
	request.IsolatedPass.Identity = cloneIdentity(request.IsolatedPass.Identity)
	return request
}

func cloneIdentity(identity RunIdentity) RunIdentity {
	identity.Order = slices.Clone(identity.Order)
	return identity
}
