package contextindex

import (
	"fmt"
	"math"
	"reflect"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"
)

// frameCandidatesScan is the pre-index frameCandidates, kept as the
// reference: every term-table path is tested against the anchor.
func (linker *testLinker) frameCandidatesScan(anchor string) []*testCandidate {
	index := linker.compiler.index
	view := *index
	view.Imports = map[string]map[string]struct{}{anchor: index.Imports[anchor]}
	found := map[string]*testCandidate{}
	for _, candidate := range linker.table.Paths {
		if contextIsTest(candidate) || isDocumentationSuffix(candidate) {
			continue
		}
		entry := &testCandidate{path: candidate, anchor: anchor}
		relation := pairRelation(anchor, candidate, contextStem(anchor), true)
		if relation != "" && relation != "module directory member" {
			entry.mirrored = relation
			entry.weight += mirroredWeight(relation)
		}
		if len(reverseImporters(&view, candidate)) != 0 {
			entry.imports = true
			entry.weight++
		}
		found[candidate] = entry
	}
	text, loaded := sourceTextBounded(index.Sources[anchor])
	if loaded {
		for _, word := range keys(scanWords(text)) {
			low, high, present := linker.table.Words.find(word)
			if !present || high-low > contextTestMentionPostings {
				continue
			}
			idf := math.Log(float64(len(linker.table.Paths)+1) / float64(high-low))
			for _, definer := range linker.declaredOf(word) {
				if candidate := found[definer]; candidate != nil {
					candidate.mention(word, idf)
				}
			}
		}
	}
	result := make([]*testCandidate, 0, len(found))
	for _, candidate := range found {
		if candidate.signals() > 0 {
			result = append(result, candidate)
		}
	}
	return result
}

// TestFrameIndexEqualsFullScan_IDXSNAPV0810 proves the frame index returns,
// for every tracked path as an anchor, exactly the candidates the full scan
// did, with every signal and weight; on a repository index the indexed pass
// over all anchors must finish within 30 s (IDX-SNAP-V0-810).
func TestFrameIndexEqualsFullScan_IDXSNAPV0810(t *testing.T) {
	byPath := func(candidates []*testCandidate) []*testCandidate {
		slices.SortFunc(candidates, func(left, right *testCandidate) int { return compareStrings(left.path, right.path) })
		return candidates
	}
	for name, index := range symbolTermIndexes(t) {
		linker := newTaskContextCompiler(index, "frame index", "").newTestLinker()
		anchors := make([]string, 0, len(linker.tracked))
		for path := range linker.tracked {
			anchors = append(anchors, path)
		}
		slices.Sort(anchors)
		start := time.Now()
		indexed := make([][]*testCandidate, len(anchors))
		for position, anchor := range anchors {
			indexed[position] = byPath(linker.frameCandidates(anchor))
		}
		elapsed := time.Since(start)
		if len(anchors) > 500 && elapsed > 30*time.Second {
			t.Fatalf("%s: the indexed pass over %d anchors took %v", name, len(anchors), elapsed)
		}
		// The reference scan is read-only once the index exists, so the
		// anchors are compared on every CPU.
		mismatch := make([]string, len(anchors))
		forEachAnchor(len(anchors), func(position int) {
			want := byPath(linker.frameCandidatesScan(anchors[position]))
			if !reflect.DeepEqual(indexed[position], want) {
				mismatch[position] = fmt.Sprintf("%s: %d indexed, %d scanned", anchors[position], len(indexed[position]), len(want))
			}
		})
		nonEmpty := 0
		for position := range anchors {
			if mismatch[position] != "" {
				t.Fatalf("%s: frame candidates differ for %s", name, mismatch[position])
			}
			if len(indexed[position]) != 0 {
				nonEmpty++
			}
		}
		t.Logf("%s: %d anchors, %d with candidates, indexed pass %v", name, len(anchors), nonEmpty, elapsed)
	}
}

// forEachAnchor runs visit for 0..count-1 on every CPU.
func forEachAnchor(count int, visit func(int)) {
	next := make(chan int)
	var workers sync.WaitGroup
	for range runtime.NumCPU() {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for position := range next {
				visit(position)
			}
		}()
	}
	for position := range count {
		next <- position
	}
	close(next)
	workers.Wait()
}
