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

// searchIDF is the BM25 inverse document frequency of a term held by df of n
// records; rarer terms weigh more.
func searchIDF(n, df int) float64 {
	return math.Log(1 + (float64(n)-float64(df)+0.5)/(float64(df)+0.5))
}

type searchHit struct {
	value any
	id    string
	score float64
}

// search ranks subjects, and with include:["claims"] claims, by a binary-tf
// BM25 score over their name and claim-text fields (DCP-V1-331, DCP-V1-328).
// Terms are summed in sorted order so the floating-point score is
// deterministic; equal scores order by record ID. A kind filter keeps only
// subjects of the named kinds and claims about them.
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
	exact := 0.0
	for _, term := range terms {
		if err := ctx.Err(); err != nil {
			return err
		}
		idf := searchIDF(len(a.Subjects), len(postings[term]))
		exact += nameFieldWeight * idf
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
	for i, s := range a.Subjects {
		if err := ctx.Err(); err != nil {
			return err
		}
		if keep(s.Kind) && strings.EqualFold(request.Query, s.Name) {
			// An exact name match outranks every partial match.
			scores[i] += exact + 1
		}
	}
	for i, score := range scores {
		hits = append(hits, searchHit{a.Subjects[i], a.Subjects[i].ID, score})
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
		return hits[i].id < hits[j].id
	})
	for _, h := range hits {
		addRecord(r, h.value)
	}
	return ctx.Err()
}

// subjectPostings returns, for each query term, the subject offsets whose
// name, id or claim text holds it, and a function naming the terms of one
// subject's name and id. With an index the postings are the index's; without
// one every subject is read once.
func subjectPostings(ctx context.Context, a *Artifact, x *QueryIndex, terms []string) (map[string][]int, func(int) map[string]bool, error) {
	names := map[int]map[string]bool{}
	nameTerms := func(i int) map[string]bool {
		if set, ok := names[i]; ok {
			return set
		}
		set := map[string]bool{}
		for term := range contextindex.EvidenceTerms(a.Subjects[i].ID + " " + a.Subjects[i].Name) {
			set[term] = true
		}
		names[i] = set
		return set
	}
	postings := map[string][]int{}
	if x != nil {
		for _, term := range terms {
			postings[term] = x.Terms[term]
		}
		return postings, nameTerms, nil
	}
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
	return postings, nameTerms, nil
}

// claimSearchHits scores claims by their ID and text terms with claim-text
// weight, adding an exact-text bonus above every partial match (DCP-V1-328).
func claimSearchHits(ctx context.Context, a *Artifact, x *QueryIndex, request Request, terms []string, keep func(string) bool) ([]searchHit, error) {
	about := func(c Claim) bool {
		if request.Kinds == nil {
			return true
		}
		ref, ok := x.IDs[c.Subject]
		return ok && ref.Kind == "subject" && keep(a.Subjects[ref.Offset].Kind)
	}
	scores := map[int]float64{}
	exact := 0.0
	for _, term := range terms {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		idf := searchIDF(len(a.Claims), len(x.ClaimTerms[term]))
		exact += claimFieldWeight * idf
		for _, i := range x.ClaimTerms[term] {
			if about(a.Claims[i]) {
				scores[i] += claimFieldWeight * idf
			}
		}
	}
	hits := make([]searchHit, 0, len(scores))
	for i, score := range scores {
		if strings.EqualFold(strings.TrimSpace(request.Query), strings.TrimSpace(a.Claims[i].Text)) {
			score += exact + 1
		}
		hits = append(hits, searchHit{a.Claims[i], a.Claims[i].ID, score})
	}
	return hits, nil
}
