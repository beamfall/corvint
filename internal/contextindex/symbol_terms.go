package contextindex

import (
	"encoding/binary"
	"errors"
	"runtime"
	"slices"
	"strings"
	"sync"
)

// symbolTermTable is the query-independent half of evalPrepareSymbols,
// computed once when the snapshot is written (IDX-SNAP-V0-807). For every
// symbol in Index.Symbols' order it records the compactText of its name and
// the terms(name) set, and over all symbols it counts the two frequency
// tables the ranking reads: symbols per compact name, and symbols whose
// terms(name) ∪ terms(path) holds each term. A query then builds those
// tables from the counts, prefilters every symbol with integer term ids, and
// prepares in full only a symbol that can still score.
type symbolTermTable struct {
	// symbols is len(Index.Symbols) at build; a table whose count differs from
	// the index it is attached to is never used.
	symbols         int
	names           []string // distinct compact names, sorted
	nameCounts      []uint32
	terms           []string // distinct terms over every symbol's name and path, sorted
	termCounts      []uint32
	symbolName      []uint32 // per symbol, its compact name's id
	nameTermOffsets []uint32 // per symbol i, nameTermIDs[offsets[i]:offsets[i+1]]
	nameTermIDs     []uint32 // ascending ids into terms
}

// buildSymbolTermTable computes the table with the same terms, compactText
// and path-term cache evalSymbolChunk.collect uses, chunked across the CPUs
// and merged in symbol order, so it is a function of the index alone.
func buildSymbolTermTable(index *Index) *symbolTermTable {
	type chunkResult struct {
		compact   []string
		nameTerms [][]string
		termCount map[string]uint32
	}
	symbols := index.Symbols
	workers := min(runtime.NumCPU(), max(len(symbols)/symbolsPerWorker, 1))
	chunks := make([]chunkResult, workers)
	var pending sync.WaitGroup
	for worker := range workers {
		pending.Add(1)
		go func(chunk *chunkResult, low, high int) {
			defer pending.Done()
			chunk.termCount = make(map[string]uint32)
			paths := evalSymbolTerms{}
			for _, symbol := range symbols[low:high] {
				named := terms(symbol.Name)
				for term := range named {
					chunk.termCount[term]++
				}
				for term := range paths.pathTermsOf(symbol.Path) {
					if _, ok := named[term]; !ok {
						chunk.termCount[term]++
					}
				}
				sorted := make([]string, 0, len(named))
				for term := range named {
					sorted = append(sorted, term)
				}
				slices.Sort(sorted)
				chunk.compact = append(chunk.compact, compactText(symbol.Name))
				chunk.nameTerms = append(chunk.nameTerms, sorted)
			}
		}(&chunks[worker], worker*len(symbols)/workers, (worker+1)*len(symbols)/workers)
	}
	pending.Wait()

	nameCount := make(map[string]uint32)
	termCount := make(map[string]uint32)
	for _, chunk := range chunks {
		for _, name := range chunk.compact {
			nameCount[name]++
		}
		for term, count := range chunk.termCount {
			termCount[term] += count
		}
	}
	table := &symbolTermTable{symbols: len(symbols)}
	nameID := sortedIDs(nameCount, &table.names, &table.nameCounts)
	termID := sortedIDs(termCount, &table.terms, &table.termCounts)
	table.symbolName = make([]uint32, 0, len(symbols))
	table.nameTermOffsets = make([]uint32, 1, len(symbols)+1)
	for _, chunk := range chunks {
		for position, name := range chunk.compact {
			table.symbolName = append(table.symbolName, nameID[name])
			for _, term := range chunk.nameTerms[position] {
				table.nameTermIDs = append(table.nameTermIDs, termID[term])
			}
			table.nameTermOffsets = append(table.nameTermOffsets, uint32(len(table.nameTermIDs)))
		}
	}
	return table
}

// sortedIDs fills keys and counts in key order and returns each key's id.
func sortedIDs(counts map[string]uint32, keys *[]string, values *[]uint32) map[string]uint32 {
	*keys = make([]string, 0, len(counts))
	for key := range counts {
		*keys = append(*keys, key)
	}
	slices.Sort(*keys)
	*values = make([]uint32, len(*keys))
	ids := make(map[string]uint32, len(*keys))
	for id, key := range *keys {
		(*values)[id] = counts[key]
		ids[key] = uint32(id)
	}
	return ids
}

// frequencies returns the nameFrequency and termFrequency maps
// evalPrepareSymbols counts, read out of the table.
func (table *symbolTermTable) frequencies() (map[string]int, map[string]int) {
	names := make(map[string]int, len(table.names))
	for id, name := range table.names {
		names[name] = int(table.nameCounts[id])
	}
	termsByCount := make(map[string]int, len(table.terms))
	for id, term := range table.terms {
		termsByCount[term] = int(table.termCounts[id])
	}
	return names, termsByCount
}

// usable reports whether the table describes index's symbols and the fast
// path's other inputs are present; otherwise the full pass runs.
func (table *symbolTermTable) usable(index *Index, windows []map[string]struct{}) bool {
	return table != nil && table.symbols == len(index.Symbols) && windows != nil && !evalRecomputePathTerms
}

// evalPreparePrecomputed is evalPrepareSymbols over a symbol term table. A
// symbol the ranking would drop before it scores is not prepared: the ranking
// keeps a candidate only when its name may match exactly, or two query terms
// meet terms(name) ∪ terms(path), or three meet its context window, and each
// of those is checked here with the table's ids and the window sets. Every
// other symbol is prepared exactly as collect prepares it, in symbol order.
// anyVersioned is evalAnyVersionedPath over every symbol the full pass would
// have prepared, so the version narrowing sees the same universe.
func evalPreparePrecomputed(index *Index, table *symbolTermTable, queryText string, queryTerms map[string]struct{}, intent evalIntent, windows []map[string]struct{}) (prepared []evalPreparedSymbol, nameFrequency, termFrequency map[string]int, anyVersioned bool) {
	nameFrequency, termFrequency = table.frequencies()
	if len(queryTerms) == 0 {
		return nil, nameFrequency, termFrequency, false
	}
	queryIDs := make(map[uint32]string, len(queryTerms))
	for term := range queryTerms {
		if id, ok := slices.BinarySearch(table.terms, term); ok {
			queryIDs[uint32(id)] = term
		}
	}
	compactQuery := compactText(queryText)
	exactCompact := make(map[uint32]bool)
	for id, name := range table.names {
		if len(name) >= 5 && table.nameCounts[id] == 1 && strings.Contains(compactQuery, name) {
			exactCompact[uint32(id)] = true
		}
	}

	type chunkResult struct {
		prepared     []evalPreparedSymbol
		anyVersioned bool
	}
	symbols := index.Symbols
	workers := min(runtime.NumCPU(), max(len(symbols)/symbolsPerWorker, 1))
	chunks := make([]chunkResult, workers)
	var pending sync.WaitGroup
	for worker := range workers {
		pending.Add(1)
		go func(chunk *chunkResult, low, high int) {
			defer pending.Done()
			symbolTermsOf := evalSymbolTerms{}
			lastPath, allowed, versioned := "", false, false
			var pathHits map[string]struct{}
			for position := low; position < high; position++ {
				symbol := symbols[position]
				if symbol.Path != lastPath || position == low {
					lastPath = symbol.Path
					allowed = !isTestPath(symbol.Path) && evalPathAllowed(symbol.Path, intent)
					if allowed {
						pathTerms := symbolTermsOf.pathTermsOf(symbol.Path)
						pathHits, versioned = make(map[string]struct{}), false
						for term := range queryTerms {
							if _, ok := pathTerms[term]; ok {
								pathHits[term] = struct{}{}
							}
						}
						for term := range pathTerms {
							if evalVersionTerm.MatchString(term) {
								versioned = true
								break
							}
						}
					}
				}
				if !allowed {
					continue
				}
				chunk.anyVersioned = chunk.anyVersioned || versioned
				overlap := len(pathHits)
				for _, id := range table.nameTermIDs[table.nameTermOffsets[position]:table.nameTermOffsets[position+1]] {
					if term, ok := queryIDs[id]; ok {
						if _, counted := pathHits[term]; !counted {
							overlap++
						}
					}
				}
				exact := exactCompact[table.symbolName[position]] || queryText == pythonLower(symbol.Name)
				if !exact && overlap < 2 && len(windows[position]) < 3 {
					continue
				}
				symbolTerms, nameTerms := symbolTermsOf.of(symbol.Name, symbol.Path)
				chunk.prepared = append(chunk.prepared, evalPreparedSymbol{
					symbol: symbol, terms: symbolTerms, context: windows[position], name: nameTerms,
					path: symbolTermsOf.pathTermsOf(symbol.Path), nameParts: evalOrderedTerms(symbol.Name),
					compactName: table.names[table.symbolName[position]],
				})
			}
		}(&chunks[worker], worker*len(symbols)/workers, (worker+1)*len(symbols)/workers)
	}
	pending.Wait()
	for _, chunk := range chunks {
		prepared = append(prepared, chunk.prepared...)
		anyVersioned = anyVersioned || chunk.anyVersioned
	}
	return prepared, nameFrequency, termFrequency, anyVersioned
}

// MarshalBinary lays the table out little-endian: the symbol count, then the
// names and terms each as a count, their joined bytes and end offsets, then
// the five uint32 arrays each behind its count.
func (table *symbolTermTable) MarshalBinary() ([]byte, error) {
	var data []byte
	put := func(value int) { data = binary.LittleEndian.AppendUint32(data, uint32(value)) }
	put(table.symbols)
	for _, list := range [][]string{table.names, table.terms} {
		put(len(list))
		for _, value := range list {
			put(len(value))
		}
		for _, value := range list {
			data = append(data, value...)
		}
	}
	for _, values := range [][]uint32{table.nameCounts, table.termCounts, table.symbolName, table.nameTermOffsets, table.nameTermIDs} {
		put(len(values))
		for _, value := range values {
			data = binary.LittleEndian.AppendUint32(data, value)
		}
	}
	return data, nil
}

var errSymbolTermTable = errors.New("symbol term table is malformed")

// UnmarshalBinary reads MarshalBinary's layout and refuses any count, offset
// or id that does not fit the table, so a decoded table is always safe to
// index.
func (table *symbolTermTable) UnmarshalBinary(data []byte) error {
	next := func() (int, bool) {
		if len(data) < 4 {
			return 0, false
		}
		value := int(binary.LittleEndian.Uint32(data))
		data = data[4:]
		return value, true
	}
	decoded := symbolTermTable{}
	var ok bool
	if decoded.symbols, ok = next(); !ok {
		return errSymbolTermTable
	}
	for _, target := range []*[]string{&decoded.names, &decoded.terms} {
		count, ok := next()
		if !ok || count > len(data)/4 {
			return errSymbolTermTable
		}
		lengths := make([]int, count)
		total := 0
		for position := range lengths {
			if lengths[position], ok = next(); !ok {
				return errSymbolTermTable
			}
			total += lengths[position]
		}
		if total > len(data) {
			return errSymbolTermTable
		}
		joined := string(data[:total])
		data = data[total:]
		*target = make([]string, count)
		offset := 0
		for position, length := range lengths {
			(*target)[position] = joined[offset : offset+length]
			offset += length
		}
	}
	for _, target := range []*[]uint32{&decoded.nameCounts, &decoded.termCounts, &decoded.symbolName, &decoded.nameTermOffsets, &decoded.nameTermIDs} {
		count, ok := next()
		if !ok || count > len(data)/4 {
			return errSymbolTermTable
		}
		*target = make([]uint32, count)
		for position := range *target {
			(*target)[position] = binary.LittleEndian.Uint32(data[4*position:])
		}
		data = data[4*count:]
	}
	if len(data) != 0 || !decoded.valid() {
		return errSymbolTermTable
	}
	*table = decoded
	return nil
}

func (table *symbolTermTable) valid() bool {
	if len(table.nameCounts) != len(table.names) || len(table.termCounts) != len(table.terms) ||
		len(table.symbolName) != table.symbols || len(table.nameTermOffsets) != table.symbols+1 || table.nameTermOffsets[0] != 0 {
		return false
	}
	for _, id := range table.symbolName {
		if int(id) >= len(table.names) {
			return false
		}
	}
	for position := 1; position < len(table.nameTermOffsets); position++ {
		if table.nameTermOffsets[position] < table.nameTermOffsets[position-1] {
			return false
		}
	}
	if int(table.nameTermOffsets[table.symbols]) != len(table.nameTermIDs) {
		return false
	}
	for _, id := range table.nameTermIDs {
		if int(id) >= len(table.terms) {
			return false
		}
	}
	return true
}
