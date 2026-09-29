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
			if a.Schema == SchemaV2 {
				unknowns = append(unknowns, "no documented subject for supplied path outside corpus subject inventory")
			} else {
				unknowns = append(unknowns, "no documented subject for "+p)
			}
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
	if len(a.BehaviorContracts) > 0 {
		unknowns = append(unknowns, "behavior contract joins retain full-relevant-suite fallback; inspect corpus gaps")
	}
	omitted := map[string]int{}
	if a.Schema == SchemaV2 {
		omitted["subjects"] = max(0, len(subjects)-MaxResults)
		omitted["relations"] = max(0, len(relations)-MaxResults)
		omitted["journeys"] = max(0, len(journeys)-MaxResults)
		omitted["observations"] = max(0, len(observations)-MaxResults)
		subjects = subjects[:min(len(subjects), MaxResults)]
		relations = relations[:min(len(relations), MaxResults)]
		journeys = journeys[:min(len(journeys), MaxResults)]
		observations = observations[:min(len(observations), MaxResults)]
		for _, n := range omitted {
			if n > 0 {
				unknowns = append(unknowns, "impact page omitted records; inventory pagination retains full corpus; full checks remain required")
				break
			}
		}
	}
	result := map[string]any{"schema": "corvint-corpus-impact/1", "artifact_sha256": a.SHA256, "repository": a.Manifest.Repository, "freshness": freshness, "subjects": subjects, "relations": relations, "journeys": journeys, "observations": observations, "unknowns": unknowns, "selection": map[string]any{"narrowing_allowed": false, "fallback": "full-repository-checks", "reason": "documentation evidence does not establish exhaustive test or journey coverage"}}
	if a.Schema == SchemaV2 {
		if len(unknowns) > MaxResults {
			result["unknowns_omitted"] = len(unknowns) - MaxResults
			unknowns = unknowns[:MaxResults]
			result["unknowns"] = unknowns
		}
		result["omitted"] = omitted
		for {
			raw, err := Encode(result)
			if err == nil && len(raw) <= 1<<20 {
				break
			}
			largest := ""
			size := 0
			for _, key := range []string{"subjects", "relations", "journeys", "observations"} {
				n := 0
				switch key {
				case "subjects":
					n = len(subjects)
				case "relations":
					n = len(relations)
				case "journeys":
					n = len(journeys)
				case "observations":
					n = len(observations)
				}
				if n > size {
					largest = key
					size = n
				}
			}
			if size == 0 {
				break
			}
			keep := size / 2
			omitted[largest] += size - keep
			switch largest {
			case "subjects":
				subjects = subjects[:keep]
				result[largest] = subjects
			case "relations":
				relations = relations[:keep]
				result[largest] = relations
			case "journeys":
				journeys = journeys[:keep]
				result[largest] = journeys
			case "observations":
				observations = observations[:keep]
				result[largest] = observations
			}
		}
	}
	return result
}
