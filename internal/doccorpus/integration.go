package doccorpus

// Impact connects exact path/symbol anchors to subjects and explicit directed
// relations. It does not infer a dependency from shared citations.
func Impact(a *Artifact, paths []string, freshness string) map[string]any {
	subjects := []Subject{}
	relations := []Relation{}
	journeys := []Journey{}
	observations := []Observation{}
	unknowns := []string{}
	selected := map[string]bool{}
	for _, p := range paths {
		found := false
		for _, s := range a.Subjects {
			if subjectPath(s, p) {
				if !selected[s.ID] {
					subjects = append(subjects, s)
					selected[s.ID] = true
				}
				found = true
			}
		}
		if !found {
			unknowns = append(unknowns, "no documented subject for "+p)
		}
	}
	// One explicitly declared edge is a bounded frontier, not transitive closure.
	reached := map[string]bool{}
	for id := range selected {
		reached[id] = true
	}
	for _, r := range a.Relations {
		if selected[r.From] || selected[r.To] {
			relations = append(relations, r)
			reached[r.From] = true
			reached[r.To] = true
		}
	}
	for _, s := range a.Subjects {
		if reached[s.ID] && !selected[s.ID] {
			subjects = append(subjects, s)
		}
	}
	for _, j := range a.Journeys {
		if reached[j.Subject] {
			journeys = append(journeys, j)
		}
	}
	for _, o := range a.Observations {
		if reached[o.Link.Subject] {
			observations = append(observations, o)
		}
	}
	if !a.HasCapability("relations") {
		unknowns = append(unknowns, "relationship capability absent")
	}
	if !a.HasCapability("journeys") {
		unknowns = append(unknowns, "journey evidence not collected")
	}
	if freshness != "fresh" {
		unknowns = append(unknowns, "corpus evidence is not fresh")
	}
	if len(paths) == 0 {
		unknowns = append(unknowns, "no exact changed source identity supplied")
	}
	// No accepted closed-world source-to-test adequacy contract exists. Even a
	// complete inventory cannot justify skipping the repository's required gate.
	unknowns = append(unknowns, "source-to-test and journey adequacy is open; run mandatory repository checks")
	return map[string]any{"schema": "corvint-corpus-impact/1", "artifact_sha256": a.SHA256, "repository": a.Manifest.Repository, "freshness": freshness, "subjects": subjects, "relations": relations, "journeys": journeys, "observations": observations, "unknowns": unknowns, "selection": map[string]any{"narrowing_allowed": false, "fallback": "full-repository-checks", "reason": "documentation evidence does not establish exhaustive test or journey coverage"}}
}
