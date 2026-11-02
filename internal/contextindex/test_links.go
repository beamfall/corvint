package contextindex

import (
	"encoding/binary"
	"errors"
	"slices"
)

// testLinkTable is testLinkMaps flattened into sorted arrays, built when the
// gob snapshot is written so a packet's test linker answers its lookups by
// binary search instead of building every map from all symbols
// (IDX-SNAP-V0-809). It is built from buildTestLinkMaps itself, so every
// lookup returns what the map would, in the same order, except
// declaredByToken, whose names are sorted where the map kept Go's iteration
// order; that list is only searched for definers to credit with one shared
// label, so its order reaches no output.
type testLinkTable struct {
	symbols int
	paths   []string // every definer or test path, sorted
	names   []string // retained declared names, sorted
	// definers of name i: paths[definerIDs[definerOffsets[i]:definerOffsets[i+1]]], in symbol order.
	definerOffsets, definerIDs []uint32
	// declaredBy of path p: names[byPathIDs[byPathOffsets[p]:byPathOffsets[p+1]]], ascending.
	byPathOffsets, byPathIDs []uint32
	tokens                   []string // first tokens of retained names, sorted
	// declaredByToken of token t: names[byTokenIDs[byTokenOffsets[t]:byTokenOffsets[t+1]]], ascending.
	byTokenOffsets, byTokenIDs []uint32
	testTokens                 []string // first tokens of test names, sorted
	// testNames of token t: entries testOffsets[t]..testOffsets[t+1] of testPathIDs and testNameValues, in symbol order.
	testOffsets, testPathIDs []uint32
	testNameValues           []string
}

func buildTestLinkTable(symbols []Symbol) *testLinkTable {
	maps := buildTestLinkMaps(symbols)
	table := &testLinkTable{symbols: len(symbols)}
	pathSet := map[string]struct{}{}
	for _, definers := range maps.declared {
		for _, definer := range definers {
			pathSet[definer] = struct{}{}
		}
	}
	for _, entries := range maps.testNames {
		for _, entry := range entries {
			pathSet[entry.path] = struct{}{}
		}
	}
	table.paths = linkSortedKeys(pathSet)
	pathID := linkIDs(table.paths)
	table.names = linkSortedKeys(maps.declared)
	nameID := linkIDs(table.names)
	table.definerOffsets = []uint32{0}
	for _, name := range table.names {
		for _, definer := range maps.declared[name] {
			table.definerIDs = append(table.definerIDs, pathID[definer])
		}
		table.definerOffsets = append(table.definerOffsets, uint32(len(table.definerIDs)))
	}
	table.byPathOffsets = []uint32{0}
	for _, path := range table.paths {
		for _, name := range maps.declaredBy[path] {
			table.byPathIDs = append(table.byPathIDs, nameID[name])
		}
		table.byPathOffsets = append(table.byPathOffsets, uint32(len(table.byPathIDs)))
	}
	table.tokens = linkSortedKeys(maps.declaredByToken)
	table.byTokenOffsets = []uint32{0}
	for _, token := range table.tokens {
		ids := make([]uint32, 0, len(maps.declaredByToken[token]))
		for _, name := range maps.declaredByToken[token] {
			ids = append(ids, nameID[name])
		}
		slices.Sort(ids)
		table.byTokenIDs = append(table.byTokenIDs, ids...)
		table.byTokenOffsets = append(table.byTokenOffsets, uint32(len(table.byTokenIDs)))
	}
	table.testTokens = linkSortedKeys(maps.testNames)
	table.testOffsets = []uint32{0}
	for _, token := range table.testTokens {
		for _, entry := range maps.testNames[token] {
			table.testPathIDs = append(table.testPathIDs, pathID[entry.path])
			table.testNameValues = append(table.testNameValues, entry.name)
		}
		table.testOffsets = append(table.testOffsets, uint32(len(table.testPathIDs)))
	}
	for _, list := range table.stringLists() {
		if len(*list) == 0 {
			*list = nil
		}
	}
	for _, values := range table.arrays() {
		if len(*values) == 0 {
			*values = nil
		}
	}
	return table
}

func linkSortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func linkIDs(sorted []string) map[string]uint32 {
	ids := make(map[string]uint32, len(sorted))
	for id, value := range sorted {
		ids[value] = uint32(id)
	}
	return ids
}

// usable reports whether the table was built for index's symbols.
func (table *testLinkTable) usable(index *Index) bool {
	return table != nil && table.symbols == len(index.Symbols)
}

func (table *testLinkTable) strings(values []string, ids []uint32) []string {
	if len(ids) == 0 {
		return nil
	}
	out := make([]string, len(ids))
	for position, id := range ids {
		out[position] = values[id]
	}
	return out
}

func (table *testLinkTable) declared(name string) []string {
	id, ok := slices.BinarySearch(table.names, name)
	if !ok {
		return nil
	}
	return table.strings(table.paths, table.definerIDs[table.definerOffsets[id]:table.definerOffsets[id+1]])
}

func (table *testLinkTable) declaredBy(path string) []string {
	id, ok := slices.BinarySearch(table.paths, path)
	if !ok {
		return nil
	}
	return table.strings(table.names, table.byPathIDs[table.byPathOffsets[id]:table.byPathOffsets[id+1]])
}

func (table *testLinkTable) declaredByToken(token string) []string {
	id, ok := slices.BinarySearch(table.tokens, token)
	if !ok {
		return nil
	}
	return table.strings(table.names, table.byTokenIDs[table.byTokenOffsets[id]:table.byTokenOffsets[id+1]])
}

func (table *testLinkTable) testEntry(position int) testName {
	name := table.testNameValues[position]
	remainder, _ := testNameRemainder(name)
	return testName{path: table.paths[table.testPathIDs[position]], name: name, tokens: nameTokens(remainder)}
}

func (table *testLinkTable) testNames(token string) []testName {
	id, ok := slices.BinarySearch(table.testTokens, token)
	if !ok {
		return nil
	}
	entries := make([]testName, 0, table.testOffsets[id+1]-table.testOffsets[id])
	for position := table.testOffsets[id]; position < table.testOffsets[id+1]; position++ {
		entries = append(entries, table.testEntry(int(position)))
	}
	return entries
}

func (table *testLinkTable) testNamesAt(path string) []testName {
	names := make([]testName, 0)
	id, ok := slices.BinarySearch(table.paths, path)
	if !ok {
		return names
	}
	for position, pathID := range table.testPathIDs {
		if int(pathID) == id {
			names = append(names, table.testEntry(position))
		}
	}
	return names
}

// MarshalBinary lays the table out little-endian: the symbol count, the five
// string lists (each a count, the lengths, the joined bytes), then the nine
// uint32 arrays (each a count and its values).
func (table *testLinkTable) MarshalBinary() ([]byte, error) {
	var data []byte
	put := func(value int) { data = binary.LittleEndian.AppendUint32(data, uint32(value)) }
	put(table.symbols)
	for _, list := range table.stringLists() {
		put(len(*list))
		for _, value := range *list {
			put(len(value))
		}
		for _, value := range *list {
			data = append(data, value...)
		}
	}
	for _, values := range table.arrays() {
		put(len(*values))
		for _, value := range *values {
			data = binary.LittleEndian.AppendUint32(data, value)
		}
	}
	return data, nil
}

func (table *testLinkTable) stringLists() []*[]string {
	return []*[]string{&table.paths, &table.names, &table.tokens, &table.testTokens, &table.testNameValues}
}

func (table *testLinkTable) arrays() []*[]uint32 {
	return []*[]uint32{&table.definerOffsets, &table.definerIDs, &table.byPathOffsets, &table.byPathIDs,
		&table.byTokenOffsets, &table.byTokenIDs, &table.testOffsets, &table.testPathIDs}
}

var errTestLinkTable = errors.New("test link table is malformed")

// UnmarshalBinary reads MarshalBinary's layout and refuses any count, offset
// or id that does not fit, so lookups on a decoded table never index out of
// range.
func (table *testLinkTable) UnmarshalBinary(data []byte) error {
	next := func() (int, bool) {
		if len(data) < 4 {
			return 0, false
		}
		value := int(binary.LittleEndian.Uint32(data))
		data = data[4:]
		return value, true
	}
	decoded := testLinkTable{}
	var ok bool
	if decoded.symbols, ok = next(); !ok {
		return errTestLinkTable
	}
	for _, target := range decoded.stringLists() {
		count, ok := next()
		if !ok || count > len(data)/4 {
			return errTestLinkTable
		}
		lengths := make([]int, count)
		total := 0
		for position := range lengths {
			if lengths[position], ok = next(); !ok {
				return errTestLinkTable
			}
			total += lengths[position]
		}
		if total > len(data) {
			return errTestLinkTable
		}
		joined := string(data[:total])
		data = data[total:]
		if count == 0 {
			*target = nil
			continue
		}
		*target = make([]string, count)
		offset := 0
		for position, length := range lengths {
			(*target)[position] = joined[offset : offset+length]
			offset += length
		}
	}
	for _, target := range decoded.arrays() {
		count, ok := next()
		if !ok || count > len(data)/4 {
			return errTestLinkTable
		}
		if count == 0 {
			*target = nil
			continue
		}
		*target = make([]uint32, count)
		for position := range *target {
			(*target)[position] = binary.LittleEndian.Uint32(data[4*position:])
		}
		data = data[4*count:]
	}
	if len(data) != 0 || !decoded.valid() {
		return errTestLinkTable
	}
	*table = decoded
	return nil
}

func validRanges(offsets, ids []uint32, groups, bound int) bool {
	if len(offsets) != groups+1 || offsets[0] != 0 || int(offsets[groups]) != len(ids) {
		return false
	}
	for position := 1; position < len(offsets); position++ {
		if offsets[position] < offsets[position-1] {
			return false
		}
	}
	for _, id := range ids {
		if int(id) >= bound {
			return false
		}
	}
	return true
}

func (table *testLinkTable) valid() bool {
	return validRanges(table.definerOffsets, table.definerIDs, len(table.names), len(table.paths)) &&
		validRanges(table.byPathOffsets, table.byPathIDs, len(table.paths), len(table.names)) &&
		validRanges(table.byTokenOffsets, table.byTokenIDs, len(table.tokens), len(table.names)) &&
		validRanges(table.testOffsets, table.testPathIDs, len(table.testTokens), len(table.paths)) &&
		len(table.testNameValues) == len(table.testPathIDs) &&
		slices.IsSorted(table.paths) && slices.IsSorted(table.names) && slices.IsSorted(table.tokens) && slices.IsSorted(table.testTokens)
}
