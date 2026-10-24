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
	// tier is the hit's ranking tier, tierExact to tierTerms: an exact name
	// or claim-text match (DCP-V1-402), an identifier token (DCP-V1-541), a
	// phrase in a path, a name or only the ID (DCP-V1-475, DCP-V1-542), a
	// name a question holds (DCP-V1-543), or terms alone. An earlier tier
	// orders first, whatever the scores.
	tier int
	// terms counts the query words whose terms the hit holds (DCP-V1-540),
	// in a question only its content words (DCP-V1-565).
	terms int
	// claim orders a claim after a subject of equal score (DCP-V1-402).
	claim bool
	// text is the subject name or claim text whose length breaks a score
	// tie before the natural ID order (DCP-V1-477).
	text string
}

// normalized divides a family's raw score by the family's largest possible
// partial score for the query, so each family's partial hits score in [0, 1]
// and compare across families (DCP-V1-402). A query without terms has no
// partial hits.
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
// (DCP-V1-402). Exact matches order first whatever their term overlap,
// then subjects whose name field holds the query as a phrase (DCP-V1-475).
// Claim text also matches regular inflections of a query term
// (DCP-V1-476). A query word counts once, by the best of the terms it yields
// (DCP-V1-540), and the tiers of DCP-V1-541 to DCP-V1-543 order identifier
// tokens, path phrases, ID-only phrases and names a question holds. Groups
// are summed in sorted order so the floating-point score is deterministic.
// Inside tier terms a hit holding more query words orders first (DCP-V1-547),
// counting in a question only its content words (DCP-V1-565);
// equal scores order subjects before claims, then
// the shorter name or text, then by natural record ID order (DCP-V1-477). A kind filter keeps only subjects of the named kinds and claims about
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
	groups := queryGroups(request.Query, terms)
	covered := coveredGroups(request.Query, groups)
	hits := []searchHit{}
	scores := map[int]float64{}
	held := map[int]int{}
	maximum := 0.0
	for g, group := range groups {
		gain := map[int]float64{}
		top := 0.0
		for _, term := range group {
			if err := ctx.Err(); err != nil {
				return err
			}
			idf := searchIDF(len(a.Subjects), len(postings[term]))
			top = max(top, idf)
			for _, i := range postings[term] {
				if !keep(a.Subjects[i].Kind) {
					continue
				}
				w := claimFieldWeight
				if names(i)[term] {
					w = nameFieldWeight
				}
				gain[i] = max(gain[i], w*idf)
			}
		}
		maximum += nameFieldWeight * top
		for i, v := range gain {
			scores[i] += v
			if covered[g] {
				held[i]++
			}
		}
	}
	for i := range scores {
		scores[i] = normalized(scores[i], maximum)
	}
	tiers := map[int]int{}
	phrase, identifier := queryPhrase(request.Query)
	question := questionShaped(request.Query)
	var queryWords []string
	var capitalized map[string]bool
	folded := ""
	if question {
		queryWords, folded, capitalized = identifierWords(request.Query), strings.ToLower(request.Query), capitalizedWords(request.Query)
	}
	for i, s := range a.Subjects {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !keep(s.Kind) {
			continue
		}
		tier := tierTerms
		if strings.EqualFold(request.Query, s.Name) {
			// An exact name match outranks every partial match of either
			// family, even one sharing no query term after tokenization.
			tier = tierExact
		} else if phrase != nil && mayHoldPhrase(s, phrase) {
			// A phrase match is a candidate on its own, even when every
			// phrase word is a stop word or no query term scored it.
			if identifier && identifierToken(s, phrase) {
				tier = tierIdentifier
			} else {
				tier = phraseTier(s, phrase, identifier)
			}
		}
		if tier == tierTerms && question && namedInQuery(s, queryWords, folded, capitalized) {
			tier = tierNamed
		}
		if tier == tierTerms {
			continue
		}
		tiers[i] = tier
		if _, ok := scores[i]; !ok {
			scores[i] = 0
		}
	}
	for i, score := range scores {
		if err := ctx.Err(); err != nil {
			return err
		}
		s := a.Subjects[i]
		tier, ok := tiers[i]
		if !ok {
			tier = tierTerms
		}
		hits = append(hits, searchHit{value: s, id: s.ID, score: score, tier: tier, terms: held[i], text: s.Name})
	}
	if claims {
		claimHits, err := claimSearchHits(ctx, a, x, request, groups, covered, keep)
		if err != nil {
			return err
		}
		hits = append(hits, claimHits...)
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].tier != hits[j].tier {
			return hits[i].tier < hits[j].tier
		}
		if hits[i].tier == tierTerms && hits[i].terms != hits[j].terms {
			// Inside tier terms a hit holding more query words orders first,
			// whatever the per-word scores (DCP-V1-547).
			return hits[i].terms > hits[j].terms
		}
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		if hits[i].claim != hits[j].claim {
			return !hits[i].claim
		}
		if li, lj := specificity(hits[i].text), specificity(hits[j].text); li != lj {
			return li < lj
		}
		return naturalLess(hits[i].id, hits[j].id)
	})
	r.ranks = make(map[string]ResultRank, len(hits))
	group := 0
	for n, h := range hits {
		if n == 0 || h.tier != hits[n-1].tier || h.score != hits[n-1].score || h.tier == tierTerms && h.terms != hits[n-1].terms {
			group++
		}
		r.ranks[h.id] = ResultRank{Rank: n + 1, Group: group, Tier: tierNames[h.tier], Score: servedScore(h.score), Terms: h.terms}
		addRecord(r, h.value)
	}
	return ctx.Err()
}

// servedScore rounds a normalized score to six decimals for a receipt, so
// the served value does not depend on the last bits of a float sum
// (DCP-V1-479).
func servedScore(score float64) float64 {
	return math.Round(score*1e6) / 1e6
}

// subjectPostings returns, for each query term, the subject offsets whose
// name, id, claim text or (for a module or file subject) anchor path holds
// it, and a function naming the query terms in one subject's name field: its
// id, its name unless the name is prose, and a module or file subject's
// anchor paths (DCP-V1-400, DCP-V1-401). Only query terms are kept, and only
// for subjects the function is asked about, so a miss retains no subject
// vocabulary (DCP-V1-403). With an index the name, id and claim-text
// postings are the index's; without one every subject is read once. The
// index is never modified.
func subjectPostings(ctx context.Context, a *Artifact, x *QueryIndex, terms []string) (map[string][]int, func(int) map[string]bool, error) {
	query := make(map[string]bool, len(terms))
	for _, term := range terms {
		query[term] = true
	}
	// matching returns the query terms in subject i's name field, or nil.
	matching := func(i int) map[string]bool {
		s := a.Subjects[i]
		var set map[string]bool
		add := func(words map[string]struct{}) {
			for term := range words {
				if query[term] {
					if set == nil {
						set = map[string]bool{}
					}
					set[term] = true
				}
			}
		}
		add(contextindex.EvidenceTerms(s.ID))
		if name := contextindex.EvidenceTerms(s.Name); pathSubject(s) || len(name) <= proseNameTerms {
			add(name)
		}
		if pathSubject(s) {
			for _, p := range extraPaths(s) {
				add(contextindex.EvidenceTerms(p))
			}
		}
		return set
	}
	names := map[int]map[string]bool{}
	nameTerms := func(i int) map[string]bool {
		if set, ok := names[i]; ok {
			return set
		}
		set := matching(i)
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
		// Subjects are tokenized in blocks on worker goroutines; each block's
		// postings are in subject order and the blocks are appended in order
		// (DCP-V1-419).
		const block = 128
		blocks := make([]map[string][]int, (len(a.Subjects)+block-1)/block)
		parallelBlocks(ctx, len(a.Subjects), block, func(b, lo, hi int) {
			local := map[string][]int{}
			for i := lo; i < hi && ctx.Err() == nil; i++ {
				s := &a.Subjects[i]
				words := contextindex.EvidenceTerms(s.Name + " " + s.ID + " " + strings.Join(claimText[s.ID], " "))
				for _, term := range terms {
					if _, ok := words[term]; ok {
						local[term] = append(local[term], i)
					}
				}
			}
			blocks[b] = local
		})
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		for _, local := range blocks {
			for _, term := range terms {
				if list := local[term]; len(list) > 0 {
					postings[term] = append(postings[term], list...)
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
		set := matching(i)
		for _, term := range terms {
			if set[term] {
				extra[term] = append(extra[term], i)
			}
		}
		if set != nil {
			names[i] = set
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

// claimSearchHits scores claims by their ID and text terms, and regular
// inflections of them (DCP-V1-476), with claim-text weight, a query word
// counting once by its best term (DCP-V1-540) and, in a question, as covered
// only when it is a content word (DCP-V1-565), normalized by the claim
// family's largest possible partial score; an exact-text match orders first
// (DCP-V1-328, DCP-V1-402).
func claimSearchHits(ctx context.Context, a *Artifact, x *QueryIndex, request Request, groups [][]string, covered []bool, keep func(string) bool) ([]searchHit, error) {
	about := func(c Claim) bool {
		if request.Kinds == nil {
			return true
		}
		ref, ok := x.IDs[c.Subject]
		return ok && ref.Kind == "subject" && keep(a.Subjects[ref.Offset].Kind)
	}
	scores := map[int]float64{}
	count := map[int]int{}
	maximum := 0.0
	for g, group := range groups {
		gain := map[int]float64{}
		top := 0.0
		for _, term := range group {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			held := claimPostings(x, term)
			idf := searchIDF(len(a.Claims), len(held))
			top = max(top, idf)
			for _, i := range held {
				if about(a.Claims[i]) {
					gain[i] = max(gain[i], claimFieldWeight*idf)
				}
			}
		}
		maximum += claimFieldWeight * top
		for i, v := range gain {
			scores[i] += v
			if covered[g] {
				count[i]++
			}
		}
	}
	query := strings.TrimSpace(request.Query)
	exact := map[int]bool{}
	for i, c := range a.Claims {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if query != "" && strings.EqualFold(query, strings.TrimSpace(c.Text)) && about(c) {
			exact[i] = true
			if _, ok := scores[i]; !ok {
				scores[i] = 0
			}
		}
	}
	hits := make([]searchHit, 0, len(scores))
	for i, score := range scores {
		c := a.Claims[i]
		tier := tierTerms
		if exact[i] {
			tier = tierExact
		}
		hits = append(hits, searchHit{value: c, id: c.ID, score: normalized(score, maximum), tier: tier, terms: count[i], claim: true, text: c.Text})
	}
	return hits, nil
}
