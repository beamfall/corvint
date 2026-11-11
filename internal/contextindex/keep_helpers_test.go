package contextindex

import (
	"sort"
)

// The ranking that read these accessors was removed by the context-proven-only
// cut; the snapshot, symbol-window, identifier-graph and test-link tests still
// use them to inspect the derived structures the index keeps building.

func sortedStringKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// find returns the byte range of key's deltas in Data, or false.
func (postings *windowPostings) find(key string) (int, int, bool) {
	count := postings.keyCount()
	if count <= 0 {
		return 0, 0, false
	}
	index := sort.Search(count, func(candidate int) bool { return postings.key(candidate) >= key })
	if index == count || postings.key(index) != key {
		return 0, 0, false
	}
	return int(postings.Offsets[index]), int(postings.Offsets[index+1]), true
}

// edges returns node's CSR range.
func (graph *identGraph) edges(node uint32) (int, int) {
	return int(graph.Offsets[node]), int(graph.Offsets[node+1])
}

// name returns an edge label's identifier and whether the edge's source
// node references it (true) or defines it (false).
func (graph *identGraph) name(label uint32) (string, bool) {
	id := label >> 1
	return graph.NameBytes[graph.NameOffsets[id]:graph.NameOffsets[id+1]], label&identGraphReferenceBit != 0
}

// usable reports whether the table was built for index's symbols.
func (table *testLinkTable) usable(index *Index) bool {
	return table != nil && table.symbols == len(index.Symbols)
}

// walkTermTable reads every key, posting range and per-source count the way
// the removed ranking did, so a table that should have been refused panics here.
func walkTermTable(table *TermTable) {
	for _, postings := range []*termPostings{&table.Terms, &table.Words, &table.PathTerms} {
		for index := 0; index < postings.keyCount(); index++ {
			if low, high, ok := postings.find(postings.key(index)); ok {
				_ = postings.Sources[low:high]
			}
		}
	}
	lengths := make([]uint32, len(table.Paths))
	for index, source := range table.Terms.Sources {
		lengths[source] += table.Terms.Counts[index]
	}
}
