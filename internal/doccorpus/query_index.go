package doccorpus

import (
	"context"
	"errors"
	"github.com/Beamfall/corvint/internal/contextindex"
	"sort"
	"strings"
)

// MaxIndexPostings and MaxIndexKeyBytes bound the derived query index: about
// twenty postings and 160 unique key bytes for each record the /2 family
// bounds admit (DCP-V1-193).
const MaxIndexPostings = 8000000
const MaxIndexKeyBytes = 64 << 20

// indexPostingBound and indexKeyByteBound are the bounds BuildQueryIndex
// enforces; tests lower them to reach the refusal without a huge fixture.
var indexPostingBound, indexKeyByteBound = MaxIndexPostings, MaxIndexKeyBytes

// IndexedOperation reports whether op answers from the derived query index,
// which the query builds when the artifact carries none (DCP-V1-195).
func IndexedOperation(op string) bool {
	switch op {
	case "gaps", "concept", "claims", "flow", "dependencies", "recommend-tests", "navigation", "vocabulary", "intent":
		return true
	}
	return false
}

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
	// ClaimTerms maps each evidence term of a claim's ID and text to the
	// claim offsets holding it, in artifact order (DCP-V1-313); a native
	// declaration claim contributes its symbol name in place of its ID
	// (DCS-V1-570). It is derived on every admission and is not part of the
	// companion encoding.
	ClaimTerms map[string][]int `json:"-"`
	// Aliases maps each subject alternate id to the subject offsets declaring
	// it (DCP-V1-330); derived like ClaimTerms.
	Aliases map[string][]int `json:"-"`
}

// BuildAdmitted is Build for an artifact that will be written: a corpus whose
// query index would exceed its bounds is refused with the capacity code,
// naming the bound and the observed count, instead of being written for the
// typed reads to refuse (DCP-V1-196). Open keeps using Build, so an artifact
// written before still opens for the reads that need no index.
func BuildAdmitted(ctx context.Context, root string, m Manifest) (*Artifact, error) {
	a, err := Build(ctx, root, m)
	if err != nil {
		return nil, err
	}
	if err := AdmitIndex(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// AdmitIndex is BuildAdmitted's index check for a corpus built by another
// writer (republish, flow docs): it returns the typed refusal the indexed reads
// would return for the artifact's query index (the capacity code, or
// corpus-refused for a record family, key, identity or path-fanout bound;
// DCP-V1-244, V1-2002), or the context's error when the check was interrupted
// before it finished, and nil otherwise (DCP-V1-196, V1-2003).
func AdmitIndex(ctx context.Context, a *Artifact) error {
	var coded *Error
	_, err := BuildQueryIndex(ctx, a)
	if errors.As(err, &coded) {
		return err
	}
	return ctx.Err()
}

// SetIndexBoundsForTest lowers the query-index bounds so a test in another
// package can reach the DCP-V1-196 refusal without a huge fixture; the
// returned function restores them. Production code never calls it.
func SetIndexBoundsForTest(postings, keyBytes int) (restore func()) {
	oldPostings, oldKeyBytes := indexPostingBound, indexKeyByteBound
	indexPostingBound, indexKeyByteBound = postings, keyBytes
	return func() { indexPostingBound, indexKeyByteBound = oldPostings, oldKeyBytes }
}

// BuildQueryIndex derives the query index of a. buildQueryIndexReference is
// the reference derivation: buildQueryIndexFast answers whenever it can show
// the reference stores the same index and refuses nothing (DCR-V1-419), and
// the reference runs otherwise, so every refusal keeps its code, message and
// observed count.
func BuildQueryIndex(ctx context.Context, a *Artifact) (*QueryIndex, error) {
	if len(a.Subjects) > MaxCorpusRecords || len(a.Claims) > MaxCorpusRecords || len(a.Relations) > MaxCorpusRecords || len(a.Journeys) > MaxCorpusJourneys || len(a.Observations) > MaxCorpusRecords || len(a.SuppliedGaps) > MaxCorpusRecords {
		return nil, fail("indexed record family bound exceeded")
	}
	if x, ok, err := buildQueryIndexFast(ctx, a); ok || err != nil {
		return x, err
	}
	return buildQueryIndexReference(ctx, a)
}

// buildQueryIndexReference is the serial derivation: each posting is charged
// as it is stored, in artifact order.
func buildQueryIndexReference(ctx context.Context, a *Artifact) (*QueryIndex, error) {
	x := &QueryIndex{IDs: map[string]RecordRef{}, Paths: map[string][]RecordRef{}, Terms: map[string][]int{}, Claims: map[string][]int{}, Edges: map[string][]int{}, ClaimTerms: map[string][]int{}, Aliases: map[string][]int{}}
	// Each posting is charged once. Past a bound the index stops storing
	// postings but keeps counting, so the refusal names the observed totals
	// (DCP-V1-194).
	postings, keyBytes := 0, 0
	over := false
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
		over = over || postings > indexPostingBound || keyBytes > indexKeyByteBound
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
			if !over {
				x.Paths[key] = append(x.Paths[key], ref)
			}
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
		for _, alias := range s.AlternateIDs {
			if err := charge(alias); err != nil {
				return nil, err
			}
			if !over {
				x.Aliases[alias] = append(x.Aliases[alias], i)
			}
		}
		terms[i] = map[string]bool{}
		for term := range contextindex.EvidenceTerms(s.ID + " " + s.Name) {
			if err := charge(term); err != nil {
				return nil, err
			}
			terms[i][term] = true
		}
	}
	// V1-2001: one reused set holds each claim's terms in turn.
	claimTerms, ownTerms := map[string]struct{}{}, map[string]struct{}{}
	native := nativeProviders(a)
	for i, c := range a.Claims {
		if err := add(c, RecordRef{"claim", i}); err != nil {
			return nil, err
		}
		if err := charge(c.Subject); err != nil {
			return nil, err
		}
		if !over {
			x.Claims[c.Subject] = append(x.Claims[c.Subject], i)
		}
		// DCP-V1-313: the claim's own ID and text terms rank docs_claims
		// queries; each posting is charged like every other.
		clear(ownTerms)
		contextindex.CollectEvidenceTerms(rankedClaimID(&c, native)+" "+c.Text, ownTerms)
		for term := range ownTerms {
			if err := charge(term); err != nil {
				return nil, err
			}
			if !over {
				x.ClaimTerms[term] = append(x.ClaimTerms[term], i)
			}
		}
		if j, ok := subjectIDs[c.Subject]; ok {
			clear(claimTerms)
			contextindex.CollectEvidenceTerms(c.Text, claimTerms)
			for term := range claimTerms {
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
			if !over {
				x.Edges[key] = append(x.Edges[key], i)
			}
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
	// DCP-V1-214: supplied gaps are indexed by ID and anchor; a corpus without
	// them derives the same index.
	for i, g := range a.SuppliedGaps {
		if err := add(g, RecordRef{"supplied_gap", i}); err != nil {
			return nil, err
		}
	}
	if postings > indexPostingBound {
		return nil, capacityExceeded("indexed postings", postings, indexPostingBound)
	}
	if keyBytes > indexKeyByteBound {
		return nil, capacityExceeded("indexed unique key bytes", keyBytes, indexKeyByteBound)
	}
	// Every term posting was charged when its subject's set first held it.
	for i, set := range terms {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for term := range set {
			x.Terms[term] = append(x.Terms[term], i)
		}
	}
	sortPathRefs(x)
	return x, nil
}

// sortPathRefs orders each path's references by kind, then offset.
func sortPathRefs(x *QueryIndex) {
	for key, refs := range x.Paths {
		sort.Slice(refs, func(i, j int) bool {
			if refs[i].Kind != refs[j].Kind {
				return refs[i].Kind < refs[j].Kind
			}
			return refs[i].Offset < refs[j].Offset
		})
		x.Paths[key] = refs
	}
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
	case "supplied_gap":
		return a.SuppliedGaps[ref.Offset]
	}
	return nil
}

// declarationClaimSuffix ends the ID of a native declaration claim, which is
// its symbol subject's ID, spelling the declaring file's path, plus this
// suffix.
const declarationClaimSuffix = ":declaration"

// rankedClaimID is the part of a claim's ID whose terms rank it
// (DCS-V1-570): for a native declaration claim, the name of the symbol it
// declares, since the rest of its ID spells the file path and would let a
// claim whose text is "def call" match every word of that path; for any
// other claim, the whole ID. A claim is a native declaration claim only
// when native reports its provider as a `kind: native` provider and it has
// the shape compile gives such claims, so a records provider's claim of the
// same shape keeps its ID terms. A nil native admits every provider; the
// caller then checks the providers declarationShaped reports.
func rankedClaimID(c *Claim, native map[string]bool) string {
	if declarationShaped(c) && (native == nil || native[c.Provider]) {
		return c.Evidence.Anchors[0].Symbol
	}
	return c.ID
}

// declarationShaped reports whether c has the shape of a native declaration
// claim: its symbol subject's ID plus declarationClaimSuffix, with exactly
// one anchor naming a symbol.
func declarationShaped(c *Claim) bool {
	return len(c.Evidence.Anchors) == 1 && c.Evidence.Anchors[0].Symbol != "" && len(c.ID) == len(c.Subject)+len(declarationClaimSuffix) && strings.HasPrefix(c.ID, c.Subject) && strings.HasSuffix(c.ID, declarationClaimSuffix)
}

// nativeProviders is the set of the artifact's `kind: native` provider IDs.
func nativeProviders(a *Artifact) map[string]bool {
	native := map[string]bool{}
	for _, p := range a.Manifest.Providers {
		if p.Kind == "native" {
			native[p.ID] = true
		}
	}
	return native
}
