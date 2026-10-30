package doccorpus

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// buildQueryIndexFast derives the query index BuildQueryIndex's reference
// derivation stores, with the record maps and the term maps built
// concurrently and the posting and unique-key-byte totals taken from the
// finished maps instead of charged one posting at a time (DCR-V1-419). While
// no bound is crossed the reference derivation stores every posting it
// charges, so each map then holds exactly the reference map and the totals
// are the reference totals. ok is false whenever the reference derivation
// could refuse: an empty or duplicate record identity, a key over its bound,
// a path at its fanout bound, or a posting total over its bound. The caller
// then runs the reference derivation, so every refusal keeps its code,
// message and observed count. err is only a context error.
func buildQueryIndexFast(parent context.Context, a *Artifact) (x *QueryIndex, ok bool, err error) {
	x = &QueryIndex{IDs: make(map[string]RecordRef, len(a.Subjects)+len(a.Claims)+len(a.Relations)), Paths: map[string][]RecordRef{}, Terms: map[string][]int{}, Claims: map[string][]int{}, Edges: map[string][]int{}, ClaimTerms: map[string][]int{}, Aliases: map[string][]int{}}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	var postings atomic.Int64
	var refused atomic.Bool
	// stored counts n stored postings and reports whether the derivation may
	// go on: past the posting bound the reference derivation refuses.
	stored := func(n int) bool {
		if postings.Add(int64(n)) > int64(indexPostingBound) {
			refused.Store(true)
			cancel()
		}
		return ctx.Err() == nil
	}
	terms := make([]map[string]bool, len(a.Subjects))
	pre := a.claimTerms.take(a.Claims)
	var wg sync.WaitGroup
	for _, derive := range []func() bool{
		func() bool { return indexRecordsFast(a, x, stored) },
		func() bool { return indexTermsFast(ctx, a, x, terms, stored, pre) },
	} {
		wg.Go(func() {
			if !derive() {
				refused.Store(true)
				cancel()
			}
		})
	}
	wg.Wait()
	if err := parent.Err(); err != nil {
		return nil, false, err
	}
	if refused.Load() {
		return nil, false, nil
	}
	total, keyBytes, ok := indexTotals(x)
	if !ok {
		return nil, false, nil
	}
	if total > indexPostingBound || keyBytes > indexKeyByteBound {
		return nil, false, nil
	}
	return x, true, nil
}

// indexTotals returns the posting total and the unique key bytes of x, or ok false when a key
// is over its bound. Each charged key is a key of the map its posting is stored in, and each
// stored posting is one entry of a posting list or of IDs. Identities are unique, and any other
// key counts once, in the first map in charging order that holds it, unless it is also an
// identity; so each map is counted on its own goroutine with lookups alone, and the sums are the
// ones a shared key set gives (DCR-V1-556).
func indexTotals(x *QueryIndex) (total, keyBytes int, ok bool) {
	lists := []map[string][]int{x.Terms, x.Claims, x.Edges, x.ClaimTerms, x.Aliases}
	// earlier reports whether key is an identity or a key of a map charged before map j, where
	// map 0 is Paths and map j > 0 is lists[j-1].
	earlier := func(j int, key string) bool {
		if _, id := x.IDs[key]; id {
			return true
		}
		if j == 0 {
			return false
		}
		if _, held := x.Paths[key]; held {
			return true
		}
		for _, m := range lists[:j-1] {
			if _, held := m[key]; held {
				return true
			}
		}
		return false
	}
	type sum struct {
		total, bytes int
		over         bool
	}
	sums := make([]sum, len(lists)+2)
	var wg sync.WaitGroup
	wg.Go(func() {
		s := &sums[len(lists)+1]
		s.total = len(x.IDs)
		for key := range x.IDs {
			s.over = s.over || len(key) > 65536
			s.bytes += len(key)
		}
	})
	wg.Go(func() {
		s := &sums[0]
		for key, refs := range x.Paths {
			s.total += len(refs)
			s.over = s.over || len(key) > 65536
			if !earlier(0, key) {
				s.bytes += len(key)
			}
		}
	})
	for i, m := range lists {
		wg.Go(func() {
			s := &sums[i+1]
			for key, list := range m {
				s.total += len(list)
				s.over = s.over || len(key) > 65536
				if !earlier(i+1, key) {
					s.bytes += len(key)
				}
			}
		})
	}
	wg.Wait()
	for _, s := range sums {
		if s.over {
			return 0, 0, false
		}
		total += s.total
		keyBytes += s.bytes
	}
	return total, keyBytes, true
}

// indexRecordsFast stores the identity, path, alias, claim-subject and edge
// postings in record order, as the reference derivation does, and reports
// false on any condition under which the reference derivation could refuse.
// The identities, the paths and the other three maps are distinct maps, so
// each is built on its own goroutine, in record order (DCR-V1-556); the
// paths are sorted as soon as they are complete. Each map is the one the
// sequential loop built, and any refusal of one still declines the whole
// fast derivation.
func indexRecordsFast(a *Artifact, x *QueryIndex, stored func(int) bool) bool {
	var ids, paths, links bool
	var wg sync.WaitGroup
	wg.Go(func() { ids = indexIDsFast(a, x, stored) })
	wg.Go(func() {
		if paths = indexPathsFast(a, x, stored); paths {
			sortPathRefs(x)
		}
	})
	links = indexLinksFast(a, x, stored)
	wg.Wait()
	return ids && paths && links
}

// eachIndexedRecord calls add on every indexed record, families in the
// reference derivation's order, until add reports false.
func eachIndexedRecord(a *Artifact, add func(id string, anchors []Anchor, ref RecordRef) bool) bool {
	for i := range a.Subjects {
		if !add(a.Subjects[i].ID, a.Subjects[i].Evidence.Anchors, RecordRef{"subject", i}) {
			return false
		}
	}
	for i := range a.Claims {
		if !add(a.Claims[i].ID, a.Claims[i].Evidence.Anchors, RecordRef{"claim", i}) {
			return false
		}
	}
	for i := range a.Relations {
		if !add(a.Relations[i].ID, a.Relations[i].Evidence.Anchors, RecordRef{"relation", i}) {
			return false
		}
	}
	for i := range a.Journeys {
		if !add(a.Journeys[i].ID, a.Journeys[i].Evidence.Anchors, RecordRef{"journey", i}) {
			return false
		}
	}
	for i := range a.Observations {
		if !add(a.Observations[i].Link.ID, a.Observations[i].Link.StepEvidence, RecordRef{"observation", i}) {
			return false
		}
	}
	for i := range a.SuppliedGaps {
		if !add(a.SuppliedGaps[i].ID, a.SuppliedGaps[i].Evidence.Anchors, RecordRef{"supplied_gap", i}) {
			return false
		}
	}
	return true
}

// indexIDsFast stores each record's identity, declining an empty or
// duplicate one.
func indexIDsFast(a *Artifact, x *QueryIndex, stored func(int) bool) bool {
	n := 0
	ok := eachIndexedRecord(a, func(id string, _ []Anchor, ref RecordRef) bool {
		if id == "" {
			return false
		}
		if _, exists := x.IDs[id]; exists {
			return false
		}
		x.IDs[id] = ref
		if n++; n == 4096 {
			n = 0
			return stored(4096)
		}
		return true
	})
	return ok && stored(n)
}

// indexPathsFast stores each record under each distinct path and symbol of
// its anchors, declining a key at its fanout bound.
func indexPathsFast(a *Artifact, x *QueryIndex, stored func(int) bool) bool {
	paths := []string{}
	return eachIndexedRecord(a, func(_ string, anchors []Anchor, ref RecordRef) bool {
		paths = paths[:0]
		for _, anchor := range anchors {
			for _, key := range [2]string{anchor.Path, anchor.Symbol} {
				if key != "" && !containsString(paths, key) {
					paths = append(paths, key)
				}
			}
		}
		for _, key := range paths {
			if len(x.Paths[key]) >= MaxCorpusRecords {
				return false
			}
			x.Paths[key] = append(x.Paths[key], ref)
		}
		return len(paths) == 0 || stored(len(paths))
	})
}

// indexLinksFast stores the subject aliases, the claims about each subject
// and the relations at each end.
func indexLinksFast(a *Artifact, x *QueryIndex, stored func(int) bool) bool {
	n := 0
	for i := range a.Subjects {
		for _, alias := range a.Subjects[i].AlternateIDs {
			x.Aliases[alias] = append(x.Aliases[alias], i)
		}
		n += len(a.Subjects[i].AlternateIDs)
	}
	for i := range a.Claims {
		x.Claims[a.Claims[i].Subject] = append(x.Claims[a.Claims[i].Subject], i)
	}
	n += len(a.Claims)
	for i := range a.Relations {
		r := &a.Relations[i]
		x.Edges[r.From] = append(x.Edges[r.From], i)
		n++
		if r.To != r.From {
			x.Edges[r.To] = append(x.Edges[r.To], i)
			n++
		}
	}
	return stored(n)
}

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// indexClaimBlock is the number of consecutive claims one worker derives
// into one block of claim-term postings.
const indexClaimBlock = 2048

// claimTermBlock is the claim-term derivation of the consecutive claims
// [lo, hi): each claim's own term postings in claim order, their count, and
// the text terms of each subject the claims are about.
type claimTermBlock struct {
	lo, hi   int
	postings map[string][]int
	n        int
	subjects map[string]map[string]struct{}
	// declarations holds the providers of the declaration-shaped claims
	// ranked by their symbol when the block was derived without the
	// manifest (DCS-V1-570).
	declarations map[string]struct{}
}

// claimTermHandoff passes the claim-term blocks a read derived to exactly one query index
// derivation of the artifact. Concurrent derivations of one artifact share it, so the blocks
// are taken under its lock; every other derivation tokenizes, and every derivation stores the
// same index. The blocks themselves are only read.
type claimTermHandoff struct {
	mu     sync.Mutex
	blocks []*claimTermBlock
	of     []Claim // the claims slice the blocks were derived from: a replaced slice voids them
}

// take returns the blocks, at most once, when they were derived from claims.
func (h *claimTermHandoff) take(claims []Claim) []*claimTermBlock {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	blocks, of := h.blocks, h.of
	h.blocks, h.of = nil, nil
	if len(of) != len(claims) || len(claims) > 0 && &of[0] != &claims[0] {
		return nil
	}
	return blocks
}

// deriveClaimTerms derives the claim-term block of claims, the first of
// which is claim lo.
func deriveClaimTerms(claims []Claim, lo int, native map[string]bool) *claimTermBlock {
	b := &claimTermBlock{lo: lo, hi: lo + len(claims), postings: make(map[string][]int, len(claims)), subjects: map[string]map[string]struct{}{}}
	text, id := map[string]struct{}{}, map[string]struct{}{}
	for k := range claims {
		c := &claims[k]
		i := lo + k
		// A space splits words and never starts or ends a camel-case
		// boundary, so the terms of a claim's ID and text are the union of
		// the terms of each, and the text is tokenized once.
		clear(text)
		clear(id)
		contextindex.CollectEvidenceTerms(c.Text, text)
		contextindex.CollectEvidenceTerms(rankedClaimID(c, native), id)
		if native == nil && declarationShaped(c) {
			if b.declarations == nil {
				b.declarations = map[string]struct{}{}
			}
			b.declarations[c.Provider] = struct{}{}
		}
		for term := range text {
			b.postings[term] = append(b.postings[term], i)
		}
		b.n += len(text)
		for term := range id {
			if _, ok := text[term]; !ok {
				b.postings[term] = append(b.postings[term], i)
				b.n++
			}
		}
		set := b.subjects[c.Subject]
		if set == nil {
			set = make(map[string]struct{}, len(text))
			b.subjects[c.Subject] = set
		}
		for term := range text {
			set[term] = struct{}{}
		}
	}
	return b
}

// claimTermBlocksCover reports whether blocks, in order, derive exactly the
// claims [0, n).
func claimTermBlocksCover(blocks []*claimTermBlock, n int) bool {
	next := 0
	for _, b := range blocks {
		if b == nil || b.lo != next || b.hi < b.lo {
			return false
		}
		next = b.hi
	}
	return next == n && len(blocks) > 0
}

// claimTermBlocksNative reports whether every declaration-shaped claim the
// blocks ranked by its symbol came from a `kind: native` provider, so the
// blocks a read derived before it had the manifest rank like the reference
// derivation (DCS-V1-570).
func claimTermBlocksNative(blocks []*claimTermBlock, native map[string]bool) bool {
	for _, b := range blocks {
		for provider := range b.declarations {
			if !native[provider] {
				return false
			}
		}
	}
	return true
}

// indexTermsFast fills each subject's term set from its identity and name
// and from the text of every claim about it, and stores each claim's own
// term postings in claim order. Claims are derived in blocks on worker
// goroutines, or taken from pre when a read derived blocks covering every
// claim while it decoded them (DCR-V1-434); each block's postings are in
// claim order, and the blocks are appended in order.
func indexTermsFast(ctx context.Context, a *Artifact, x *QueryIndex, terms []map[string]bool, stored func(int) bool, pre []*claimTermBlock) bool {
	subjectIDs := make(map[string]int, len(a.Subjects))
	for i := range a.Subjects {
		subjectIDs[a.Subjects[i].ID] = i
	}
	parallelBlocks(ctx, len(a.Subjects), 256, func(_, lo, hi int) {
		for i := lo; i < hi; i++ {
			list := contextindex.EvidenceTerms(a.Subjects[i].ID + " " + a.Subjects[i].Name)
			terms[i] = make(map[string]bool, len(list))
			for term := range list {
				terms[i][term] = true
			}
		}
	})
	if ctx.Err() != nil {
		return false
	}
	blocks := pre
	native := nativeProviders(a)
	if !claimTermBlocksCover(blocks, len(a.Claims)) || !claimTermBlocksNative(blocks, native) {
		blocks = make([]*claimTermBlock, (len(a.Claims)+indexClaimBlock-1)/indexClaimBlock)
		parallelBlocks(ctx, len(a.Claims), indexClaimBlock, func(b, lo, hi int) {
			blocks[b] = deriveClaimTerms(a.Claims[lo:hi], lo, native)
		})
	}
	if ctx.Err() != nil {
		return false
	}
	locks := make([]sync.Mutex, len(a.Subjects))
	parallelBlocks(ctx, len(blocks), 1, func(b, _, _ int) {
		for subject, set := range blocks[b].subjects {
			if j, ok := subjectIDs[subject]; ok {
				locks[j].Lock()
				for term := range set {
					terms[j][term] = true
				}
				locks[j].Unlock()
			}
		}
	})
	if ctx.Err() != nil {
		return false
	}
	// The subject terms are complete, so their postings are stored, in subject order, while the
	// claim-term postings are merged into their own map (DCR-V1-556).
	var subjectTerms sync.WaitGroup
	defer subjectTerms.Wait()
	subjectTerms.Go(func() {
		for i, set := range terms {
			for term := range set {
				x.Terms[term] = append(x.Terms[term], i)
			}
		}
	})
	// Each term's postings are allocated once at their final length, and a term that only one
	// block holds keeps that block's list; the blocks are not read again (DCR-V1-468).
	hint := 0
	if len(blocks) > 0 {
		hint = len(blocks[0].postings)
	}
	total := make(map[string]int, hint)
	for _, b := range blocks {
		for term, list := range b.postings {
			total[term] += len(list)
		}
	}
	x.ClaimTerms = make(map[string][]int, len(total))
	for _, b := range blocks {
		for term, list := range b.postings {
			held, ok := x.ClaimTerms[term]
			switch {
			case ok:
				x.ClaimTerms[term] = append(held, list...)
			case total[term] == len(list):
				x.ClaimTerms[term] = list
			default:
				x.ClaimTerms[term] = append(make([]int, 0, total[term]), list...)
			}
		}
		if !stored(b.n) {
			return false
		}
	}
	return true
}

// parallelBlocks runs derive over [0, n) in blocks of size consecutive
// indexes on up to GOMAXPROCS goroutines, and returns once every started
// block has finished. A cancelled context stops further blocks.
func parallelBlocks(ctx context.Context, n, size int, derive func(block, lo, hi int)) {
	blocks := (n + size - 1) / size
	var next atomic.Int64
	var wg sync.WaitGroup
	for range min(runtime.GOMAXPROCS(0), blocks) {
		wg.Go(func() {
			for ctx.Err() == nil {
				b := int(next.Add(1) - 1)
				if b >= blocks {
					return
				}
				derive(b, b*size, min(n, (b+1)*size))
			}
		})
	}
	wg.Wait()
}
