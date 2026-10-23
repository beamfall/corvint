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
// finished maps instead of charged one posting at a time (DCP-V1-419). While
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
	var wg sync.WaitGroup
	for _, derive := range []func() bool{
		func() bool { return indexRecordsFast(a, x, stored) },
		func() bool { return indexTermsFast(ctx, a, x, terms, stored) },
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
	for i, set := range terms {
		for term := range set {
			x.Terms[term] = append(x.Terms[term], i)
		}
	}
	// Each charged key is a key of the map its posting is stored in, and each
	// stored posting is one entry of a posting list or of IDs. Identities are
	// unique, so only the other keys need a set of their own.
	total, keyBytes := len(x.IDs), 0
	for key := range x.IDs {
		if len(key) > 65536 {
			return nil, false, nil
		}
		keyBytes += len(key)
	}
	keys := make(map[string]struct{}, len(x.Paths)+len(x.Terms)+len(x.ClaimTerms))
	addKey := func(key string) bool {
		if len(key) > 65536 {
			return false
		}
		if _, id := x.IDs[key]; id {
			return true
		}
		if _, seen := keys[key]; !seen {
			keys[key] = struct{}{}
			keyBytes += len(key)
		}
		return true
	}
	for key, refs := range x.Paths {
		total += len(refs)
		if !addKey(key) {
			return nil, false, nil
		}
	}
	for _, m := range []map[string][]int{x.Terms, x.Claims, x.Edges, x.ClaimTerms, x.Aliases} {
		for key, list := range m {
			total += len(list)
			if !addKey(key) {
				return nil, false, nil
			}
		}
	}
	if total > indexPostingBound || keyBytes > indexKeyByteBound {
		return nil, false, nil
	}
	sortPathRefs(x)
	return x, true, nil
}

// indexRecordsFast stores the identity, path, alias, claim-subject and edge
// postings in record order, as the reference derivation does, and reports
// false on any condition under which the reference derivation could refuse.
func indexRecordsFast(a *Artifact, x *QueryIndex, stored func(int) bool) bool {
	paths := []string{}
	add := func(id string, anchors []Anchor, ref RecordRef) bool {
		if id == "" {
			return false
		}
		if _, exists := x.IDs[id]; exists {
			return false
		}
		x.IDs[id] = ref
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
		return stored(1 + len(paths))
	}
	for i := range a.Subjects {
		s := &a.Subjects[i]
		if !add(s.ID, s.Evidence.Anchors, RecordRef{"subject", i}) {
			return false
		}
		for _, alias := range s.AlternateIDs {
			x.Aliases[alias] = append(x.Aliases[alias], i)
		}
		if !stored(len(s.AlternateIDs)) {
			return false
		}
	}
	for i := range a.Claims {
		c := &a.Claims[i]
		if !add(c.ID, c.Evidence.Anchors, RecordRef{"claim", i}) {
			return false
		}
		x.Claims[c.Subject] = append(x.Claims[c.Subject], i)
	}
	if !stored(len(a.Claims)) {
		return false
	}
	for i := range a.Relations {
		r := &a.Relations[i]
		if !add(r.ID, r.Evidence.Anchors, RecordRef{"relation", i}) {
			return false
		}
		x.Edges[r.From] = append(x.Edges[r.From], i)
		n := 1
		if r.To != r.From {
			x.Edges[r.To] = append(x.Edges[r.To], i)
			n++
		}
		if !stored(n) {
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

// indexTermsFast fills each subject's term set from its identity and name
// and from the text of every claim about it, and stores each claim's own
// term postings in claim order. Claims are derived in blocks on worker
// goroutines; each block's postings are in claim order, and the blocks are
// appended in order.
func indexTermsFast(ctx context.Context, a *Artifact, x *QueryIndex, terms []map[string]bool, stored func(int) bool) bool {
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
	locks := make([]sync.Mutex, len(a.Subjects))
	blocks := make([]map[string][]int, (len(a.Claims)+indexClaimBlock-1)/indexClaimBlock)
	counts := make([]int, len(blocks))
	parallelBlocks(ctx, len(a.Claims), indexClaimBlock, func(b, lo, hi int) {
		local := map[string][]int{}
		n := 0
		text, id := map[string]struct{}{}, map[string]struct{}{}
		for i := lo; i < hi && ctx.Err() == nil; i++ {
			c := &a.Claims[i]
			// A space splits words and never starts or ends a camel-case
			// boundary, so the terms of a claim's ID and text are the union of
			// the terms of each, and the text is tokenized once.
			clear(text)
			clear(id)
			contextindex.CollectEvidenceTerms(c.Text, text)
			contextindex.CollectEvidenceTerms(c.ID, id)
			for term := range text {
				local[term] = append(local[term], i)
			}
			n += len(text)
			for term := range id {
				if _, ok := text[term]; !ok {
					local[term] = append(local[term], i)
					n++
				}
			}
			if j, ok := subjectIDs[c.Subject]; ok {
				locks[j].Lock()
				for term := range text {
					terms[j][term] = true
				}
				locks[j].Unlock()
			}
		}
		blocks[b], counts[b] = local, n
	})
	if ctx.Err() != nil {
		return false
	}
	for b, local := range blocks {
		for term, list := range local {
			x.ClaimTerms[term] = append(x.ClaimTerms[term], list...)
		}
		if !stored(counts[b]) {
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
