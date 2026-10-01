package doccorpus

import (
	"context"
	"github.com/Beamfall/corvint/internal/contextindex"
	"sort"
)

const MaxIndexPostings = 1000000
const MaxIndexKeyBytes = 16 << 20

type RecordRef struct {
	Kind   string `json:"kind"`
	Offset int    `json:"offset"`
}

// QueryIndex contains only offsets into an immutable validated artifact. It is
// derived independently on admission; its bytes never establish source truth.
type QueryIndex struct {
	IDs    map[string]RecordRef   `json:"ids"`
	Paths  map[string][]RecordRef `json:"paths"`
	Terms  map[string][]int       `json:"terms"`
	Claims map[string][]int       `json:"claims"`
	Edges  map[string][]int       `json:"edges"`
}

func BuildQueryIndex(ctx context.Context, a *Artifact) (*QueryIndex, error) {
	if len(a.Subjects) > MaxCorpusRecords || len(a.Claims) > MaxCorpusRecords || len(a.Relations) > MaxCorpusRecords || len(a.Journeys) > MaxCorpusJourneys || len(a.Observations) > MaxRecords {
		return nil, fail("indexed record family bound exceeded")
	}
	x := &QueryIndex{IDs: map[string]RecordRef{}, Paths: map[string][]RecordRef{}, Terms: map[string][]int{}, Claims: map[string][]int{}, Edges: map[string][]int{}}
	postings, keyBytes := 0, 0
	keys := map[string]bool{}
	charge := func(key string) error {
		if len(key) > 65536 {
			return fail("indexed key bound exceeded")
		}
		if !keys[key] {
			keyBytes += len(key)
			keys[key] = true
		}
		postings++
		if postings > MaxIndexPostings || keyBytes > MaxIndexKeyBytes {
			return fail("indexed postings or key byte bound exceeded")
		}
		return ctx.Err()
	}
	add := func(v any, ref RecordRef) error {
		id := recordID(v)
		if id == "" {
			return fail("indexed record identity missing")
		}
		if _, exists := x.IDs[id]; exists {
			return fail("duplicate indexed record identity")
		}
		if err := charge(id); err != nil {
			return err
		}
		x.IDs[id] = ref
		paths := map[string]bool{}
		for _, anchor := range recordAnchors(v) {
			for _, key := range []string{anchor.Path, anchor.Symbol} {
				if key != "" {
					paths[key] = true
				}
			}
		}
		for key := range paths {
			if len(x.Paths[key]) >= MaxCorpusRecords {
				return fail("indexed path fanout bound exceeded")
			}
			if err := charge(key); err != nil {
				return err
			}
			x.Paths[key] = append(x.Paths[key], ref)
		}
		return nil
	}
	subjectIDs := map[string]int{}
	terms := make([]map[string]bool, len(a.Subjects))
	for i, s := range a.Subjects {
		if err := add(s, RecordRef{"subject", i}); err != nil {
			return nil, err
		}
		subjectIDs[s.ID] = i
		terms[i] = map[string]bool{}
		for term := range contextindex.EvidenceTerms(s.ID + " " + s.Name) {
			terms[i][term] = true
		}
	}
	for i, c := range a.Claims {
		if err := add(c, RecordRef{"claim", i}); err != nil {
			return nil, err
		}
		if err := charge(c.Subject); err != nil {
			return nil, err
		}
		x.Claims[c.Subject] = append(x.Claims[c.Subject], i)
		if j, ok := subjectIDs[c.Subject]; ok {
			for term := range contextindex.EvidenceTerms(c.Text) {
				if !terms[j][term] {
					if err := charge(term); err != nil {
						return nil, err
					}
					terms[j][term] = true
				}
			}
		}
	}
	for i, r := range a.Relations {
		if err := add(r, RecordRef{"relation", i}); err != nil {
			return nil, err
		}
		endpoints := []string{r.From}
		if r.To != r.From {
			endpoints = append(endpoints, r.To)
		}
		for _, key := range endpoints {
			if err := charge(key); err != nil {
				return nil, err
			}
			x.Edges[key] = append(x.Edges[key], i)
		}
	}
	for i, j := range a.Journeys {
		if err := add(j, RecordRef{"journey", i}); err != nil {
			return nil, err
		}
	}
	for i, o := range a.Observations {
		if err := add(o, RecordRef{"observation", i}); err != nil {
			return nil, err
		}
	}
	for i, set := range terms {
		for term := range set {
			if err := charge(term); err != nil {
				return nil, err
			}
			x.Terms[term] = append(x.Terms[term], i)
		}
	}
	for key, refs := range x.Paths {
		sort.Slice(refs, func(i, j int) bool {
			if refs[i].Kind != refs[j].Kind {
				return refs[i].Kind < refs[j].Kind
			}
			return refs[i].Offset < refs[j].Offset
		})
		x.Paths[key] = refs
	}
	return x, nil
}
func (a *Artifact) indexedRecord(ref RecordRef) any {
	switch ref.Kind {
	case "subject":
		return a.Subjects[ref.Offset]
	case "claim":
		return a.Claims[ref.Offset]
	case "relation":
		return a.Relations[ref.Offset]
	case "journey":
		return a.Journeys[ref.Offset]
	case "observation":
		return a.Observations[ref.Offset]
	}
	return nil
}
