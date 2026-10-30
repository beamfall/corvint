package doccorpus

import (
	"context"
	"slices"
)

// addDocumentingOwners adds, for each subject in anchored, every relation of
// type documents whose To is that subject, and the relation's From subject:
// one incoming documents hop, so a path operation names the subjects that
// own the path with the relation that justifies each (DCS-V1-594). Outgoing
// documents relations, other relation types and further hops add nothing.
// Records whose IDs are in have are not added again. It returns the owner
// subject IDs it found, in a deterministic order.
func addDocumentingOwners(ctx context.Context, a *Artifact, anchored []string, have map[string]bool, r *Receipt) ([]string, error) {
	targets := map[string]bool{}
	for _, id := range anchored {
		targets[id] = true
	}
	var relations []int
	if x := a.RuntimeIndex; x != nil {
		for _, id := range slices.Sorted(slices.Values(anchored)) {
			for _, i := range x.Edges[id] {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				if rel := a.Relations[i]; rel.Type == "documents" && rel.To == id {
					relations = append(relations, i)
				}
			}
		}
	} else {
		for i, rel := range a.Relations {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if rel.Type == "documents" && targets[rel.To] {
				relations = append(relations, i)
			}
		}
	}
	slices.Sort(relations)
	relations = slices.Compact(relations)
	owners, owner := []string{}, map[string]bool{}
	for _, i := range relations {
		rel := a.Relations[i]
		if !have[rel.ID] {
			have[rel.ID] = true
			addRecord(r, rel)
		}
		if !owner[rel.From] {
			owner[rel.From] = true
			owners = append(owners, rel.From)
		}
	}
	// An owner is added only when it is a subject of the corpus; a dangling
	// From keeps its relation as the only evidence.
	subjects := map[string]Subject{}
	if a.RuntimeIndex != nil {
		for _, id := range owners {
			if s, ok := indexedSubject(a, id); ok {
				subjects[id] = s
			}
		}
	} else if len(owners) > 0 {
		for _, s := range a.Subjects {
			if owner[s.ID] {
				subjects[s.ID] = s
			}
		}
	}
	found := []string{}
	for _, id := range owners {
		s, ok := subjects[id]
		if !ok {
			continue
		}
		found = append(found, id)
		if !have[id] {
			have[id] = true
			addRecord(r, s)
		}
	}
	return found, ctx.Err()
}
