package doccorpus

import (
	"context"
	"maps"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// Field weights of the docs_search ranking (DCP-V1-331). A query term in a
// subject's name or id counts nameFieldWeight times a term found only in its
// claim text, so a name match outranks a claim-text-only match of the same
// term.
const (
	nameFieldWeight  = 3.0
	claimFieldWeight = 1.0
)

// proseNameTerms is the most distinct terms a subject name may hold and still
// weigh as a name; a longer name is prose, such as a documentation paragraph,
// and its terms weigh as claim text (DCP-V1-401).
const proseNameTerms = 12

// pathSubject reports whether a subject's anchor paths are part of its name
// field: module and file subjects (DCP-V1-400).
func pathSubject(s Subject) bool { return s.Kind == "module" || s.Kind == "file" }

// searchIDF is the BM25 inverse document frequency of a term held by df of n
// records; rarer terms weigh more.
func searchIDF(n, df int) float64 {
	return math.Log(1 + (float64(n)-float64(df)+0.5)/(float64(df)+0.5))
}

type searchHit struct {
	value any
	id    string
	score float64
	// claim orders a claim after a subject of equal score (DCP-V1-402).
	claim bool
}

// normalized divides a family's raw score by the family's largest possible
// partial score for the query, so each family's partial hits score in [0, 1]
// and an exact match, adding 1, outranks every partial hit of both families
// (DCP-V1-402). A query without terms has no partial hits.
func normalized(score, maximum float64) float64 {
	if maximum <= 0 {
		return 0
	}
	return score / maximum
}

// search ranks subjects, and with include:["claims"] claims, by a binary-tf
// BM25 score over their name and claim-text fields (DCP-V1-331, DCP-V1-328),
// with module and file path segments in the name field and prose names
// weighed as text (DCP-V1-400, DCP-V1-401). Each family's score is
// normalized by its largest possible partial score before the families merge
// (DCP-V1-402). Terms are summed in sorted order so the floating-point score
// is deterministic; equal scores order subjects before claims, then by record
// ID. A kind filter keeps only subjects of the named kinds and claims about
// them.
func search(ctx context.Context, a *Artifact, request Request, r *Receipt) error {
	terms := slices.Sorted(maps.Keys(contextindex.EvidenceTerms(request.Query)))
	claims := SearchIncludesClaims(request)
	x := a.RuntimeIndex
	if x == nil && claims {
		index, err := BuildQueryIndex(ctx, a)
		if err != nil {
			return err
		}
		x = index
	}
	keep := func(kind string) bool { return request.Kinds == nil || slices.Contains(request.Kinds, kind) }
	postings, names, err := subjectPostings(ctx, a, x, terms)
	if err != nil {
		return err
	}
	hits := []searchHit{}
	scores := map[int]float64{}
	maximum := 0.0
	for _, term := range terms {
		if err := ctx.Err(); err != nil {
			return err
		}
		idf := searchIDF(len(a.Subjects), len(postings[term]))
		maximum += nameFieldWeight * idf
		for _, i := range postings[term] {
			if !keep(a.Subjects[i].Kind) {
				continue
			}
			w := claimFieldWeight
			if names(i)[term] {
				w = nameFieldWeight
			}
			scores[i] += w * idf
		}
	}
	for i := range scores {
		scores[i] = normalized(scores[i], maximum)
	}
	for i, s := range a.Subjects {
		if err := ctx.Err(); err != nil {
			return err
		}
		if keep(s.Kind) && strings.EqualFold(request.Query, s.Name) {
			// An exact name match outranks every partial match of either family.
			scores[i]++
		}
	}
	for i, score := range scores {
		hits = append(hits, searchHit{a.Subjects[i], a.Subjects[i].ID, score, false})
	}
	if claims {
		claimHits, err := claimSearchHits(ctx, a, x, request, terms, keep)
		if err != nil {
			return err
		}
		hits = append(hits, claimHits...)
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		if hits[i].claim != hits[j].claim {
			return !hits[i].claim
		}
		return hits[i].id < hits[j].id
	})
	for _, h := range hits {
		addRecord(r, h.value)
	}
	return ctx.Err()
}

// subjectPostings returns, for each query term, the subject offsets whose
// name, id, claim text or (for a module or file subject) anchor path holds
// it, and a function naming the terms of one subject's name field: its id,
// its name unless the name is prose, and a module or file subject's anchor
// paths (DCP-V1-400, DCP-V1-401). With an index the name, id and claim-text
// postings are the index's; without one every subject is read once. The
// index is never modified.
func subjectPostings(ctx context.Context, a *Artifact, x *QueryIndex, terms []string) (map[string][]int, func(int) map[string]bool, error) {
	names := map[int]map[string]bool{}
	nameTerms := func(i int) map[string]bool {
		if set, ok := names[i]; ok {
			return set
		}
		s := a.Subjects[i]
		set := map[string]bool{}
		for term := range contextindex.EvidenceTerms(s.ID) {
			set[term] = true
		}
		name := contextindex.EvidenceTerms(s.Name)
		if pathSubject(s) || len(name) <= proseNameTerms {
			for term := range name {
				set[term] = true
			}
		}
		if pathSubject(s) {
			for _, p := range extraPaths(s) {
				for term := range contextindex.EvidenceTerms(p) {
					set[term] = true
				}
			}
		}
		names[i] = set
		return set
	}
	postings := map[string][]int{}
	if x != nil {
		for _, term := range terms {
			postings[term] = x.Terms[term]
		}
	} else {
		claimText := map[string][]string{}
		for _, c := range a.Claims {
			if err := ctx.Err(); err != nil {
				return nil, nil, err
			}
			claimText[c.Subject] = append(claimText[c.Subject], c.Text)
		}
		for i, s := range a.Subjects {
			if err := ctx.Err(); err != nil {
				return nil, nil, err
			}
			words := contextindex.EvidenceTerms(s.Name + " " + s.ID + " " + strings.Join(claimText[s.ID], " "))
			for _, term := range terms {
				if _, ok := words[term]; ok {
					postings[term] = append(postings[term], i)
				}
			}
		}
	}
	// DCP-V1-400: a module or file subject also matches the terms of an
	// anchor path that is not its name.
	extra := map[string][]int{}
	for i, s := range a.Subjects {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if !pathSubject(s) || !slices.ContainsFunc(s.Evidence.Anchors, func(a Anchor) bool { return a.Path != "" && a.Path != s.Name }) {
			continue
		}
		set := nameTerms(i)
		for _, term := range terms {
			if set[term] {
				extra[term] = append(extra[term], i)
			}
		}
	}
	for term, add := range extra {
		merged := slices.Concat(postings[term], add)
		slices.Sort(merged)
		postings[term] = slices.Compact(merged)
	}
	return postings, nameTerms, nil
}

// extraPaths lists a subject's distinct anchor paths other than its name, in
// anchor order.
func extraPaths(s Subject) []string {
	var out []string
	for _, anchor := range s.Evidence.Anchors {
		if anchor.Path != "" && anchor.Path != s.Name && !slices.Contains(out, anchor.Path) {
			out = append(out, anchor.Path)
		}
	}
	return out
}

// claimSearchHits scores claims by their ID and text terms with claim-text
// weight, normalized by the claim family's largest possible partial score;
// an exact-text match adds 1 (DCP-V1-328, DCP-V1-402).
func claimSearchHits(ctx context.Context, a *Artifact, x *QueryIndex, request Request, terms []string, keep func(string) bool) ([]searchHit, error) {
	about := func(c Claim) bool {
		if request.Kinds == nil {
			return true
		}
		ref, ok := x.IDs[c.Subject]
		return ok && ref.Kind == "subject" && keep(a.Subjects[ref.Offset].Kind)
	}
	scores := map[int]float64{}
	maximum := 0.0
	for _, term := range terms {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		idf := searchIDF(len(a.Claims), len(x.ClaimTerms[term]))
		maximum += claimFieldWeight * idf
		for _, i := range x.ClaimTerms[term] {
			if about(a.Claims[i]) {
				scores[i] += claimFieldWeight * idf
			}
		}
	}
	hits := make([]searchHit, 0, len(scores))
	for i, score := range scores {
		score = normalized(score, maximum)
		if strings.EqualFold(strings.TrimSpace(request.Query), strings.TrimSpace(a.Claims[i].Text)) {
			score++
		}
		hits = append(hits, searchHit{a.Claims[i], a.Claims[i].ID, score, true})
	}
	return hits, nil
}
