package appflows

import (
	"context"
	"strings"
)

type flowStability struct {
	state, detail string
	aggregates    map[runJoin][]flowAggregate
	records       map[runJoin][]TestRunEvidence
}

type flowAggregate struct {
	declaration RunAggregate
	report      RunAggregateReport
	err         error
}

// Read the registry at the same immutable revision as the intents, even if HEAD moves meanwhile.
func readFlowStability(ctx context.Context, root, name string, evidence []TestRunEvidence, at head) *flowStability {
	if name == "" {
		return nil
	}
	s := &flowStability{aggregates: map[runJoin][]flowAggregate{}, records: joinRecords(evidence)}
	if !safePath(name) {
		s.state, s.detail = GapStabilityInvalid, "registry path must be repository-relative"
		return s
	}
	raw, err := committedFile(ctx, root, at.commit, name)
	if err != nil || raw == nil {
		s.state, s.detail = GapStabilityMissing, "registry unavailable at evaluated revision"
		return s
	}
	var registry RunRegistry
	if err := Decode(raw, &registry); err != nil {
		s.state, s.detail = GapStabilityInvalid, RegistryInvalid
		return s
	}
	thresholds, err := registryThresholds(registry)
	if err != nil {
		s.state, s.detail = GapStabilityInvalid, err.Error()
		return s
	}
	seen := map[string]bool{}
	for _, a := range registry.Aggregates {
		if seen[a.ID] {
			s.state, s.detail = GapStabilityInvalid, RegistryInvalid
			return s
		}
		seen[a.ID] = true
		report, err := aggregateRuns(a, thresholds, s.records)
		key := runJoin{testKey: a.TestKey, project: a.Project}
		s.aggregates[key] = append(s.aggregates[key], flowAggregate{a, report, err})
	}
	return s
}

func (s *flowStability) qualify(key, project string, matching []TestRunEvidence, controls []string, at head) (string, string) {
	if s == nil {
		return GapStabilityMissing, ""
	}
	if s.state != "" {
		return s.state, s.detail
	}
	projects := []string{project}
	if project == "" {
		projects = nil
		for _, r := range matching {
			if r.Authority != AuthorityStatic && r.Source == (RunSource{Commit: at.commit, Tree: at.tree, Clean: true}) {
				projects = append(projects, r.Project)
			}
		}
		projects = sortedSet(projects)
	}
	for _, p := range projects {
		aggregates := s.aggregates[runJoin{testKey: key, project: p}]
		if len(aggregates) == 0 {
			return GapStabilityMissing, "no aggregate for test/project"
		}
		for _, a := range aggregates {
			if a.err != nil {
				state := GapStabilityInvalid
				if strings.HasPrefix(a.err.Error(), RegistryIncomplete) || strings.HasPrefix(a.err.Error(), RegistryMissingRecord) {
					state = GapStabilityIncomplete
				}
				return state, a.err.Error()
			}
			if a.report.Source != (RunSource{Commit: at.commit, Tree: at.tree, Clean: true}) {
				return GapStabilityStale, a.report.ID
			}
			// Thresholds never override the flow's cleanup, control or flake requirements, including manual runs.
			for _, c := range a.declaration.Contributions {
				r := s.records[runJoin{c.RunID, key, p}][0]
				if state := evidenceState([]TestRunEvidence{r}, controls, at); state != "passed" {
					return state, a.report.ID
				}
			}
			if a.report.Verdict != stabilityVerdictClean {
				return GapStabilityFailed, a.report.ID
			}
		}
	}
	return "verified", ""
}
