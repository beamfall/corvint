package contextindex

import (
	"encoding/binary"
	"errors"
	"runtime"
	"sort"
	"sync"
)

// TermTable is the packet's vocabulary, built once per tree beside the other
// tables and carried in the snapshot: every readable source's lowercased
// camel-split tokens with their occurrence counts (Terms), its identifier
// words as written (Words), and the tokens of its path (PathTerms), each
// inverted so a task term costs one binary search and one posting walk
// instead of one pass over every source body (decision 0033). A source id is
// its position in Paths. A source over contextMaxBytes, or one whose bytes are
// not text, has no posting, which is the same absence the scan it replaces
// produced.
type TermTable struct {
	Paths     []string
	Terms     termPostings
	Words     termPostings
	PathTerms termPostings
	// SymbolWindows inverts the ranking's symbol context windows: for key k,
	// Sources[Offsets[k]:Offsets[k+1]] are the positions in Index.Symbols,
	// ascending, whose window text (symbolwindows.go) yields k under terms().
	// The full compile builds it for the snapshot; an index compiled for one
	// query leaves it empty and scans the windows instead.
	SymbolWindows windowPostings
	// IdentGraph is the identifier definition/reference graph over Paths
	// (identgraph.go, TCP-V0-030), built with SymbolWindows by the full
	// compile.
	IdentGraph *identGraph

	// lengths and averageLength are each source's body token count and their
	// mean, derived once from the counted postings for BM25 length
	// normalisation. They are unexported so neither gob nor the snapshot
	// carries them: the snapshot format is unchanged.
	lengthsReady  bool
	lengths       []uint32
	averageLength float64
}

// documentLengths returns each source's body token count and the mean over
// every source, computed on first use from Terms.Counts. Like vocabulary(),
// it assumes one goroutine reads the index at a time.
func (table *TermTable) documentLengths() ([]uint32, float64) {
	if !table.lengthsReady {
		lengths := make([]uint32, len(table.Paths))
		var total uint64
		for index, source := range table.Terms.Sources {
			lengths[source] += table.Terms.Counts[index]
			total += uint64(table.Terms.Counts[index])
		}
		table.lengths = lengths
		if len(lengths) > 0 {
			table.averageLength = float64(total) / float64(len(lengths))
		}
		table.lengthsReady = true
	}
	return table.lengths, table.averageLength
}

// termPostings is one inverted table in four flat slices so the snapshot
// decode is a handful of bulk reads rather than one allocation per key: the
// keys concatenated in sorted order with their offsets, and for key k the
// sources Sources[Offsets[k]:Offsets[k+1]] in ascending id order, with Counts
// parallel to Sources when the table counts occurrences.
type termPostings struct {
	KeyBytes   string
	KeyOffsets []uint32
	Offsets    []uint32
	Sources    []uint32
	Counts     []uint32
}

// MarshalBinary lays the five slices out fixed-width little-endian, each
// behind its element count, so gob moves the table as one byte string: its
// element-wise integer decoding costs about 250 ms for the 3.6 million
// postings of the Beamfall tree, the bulk copy under 10 ms.
func (postings termPostings) MarshalBinary() ([]byte, error) {
	size := 5*4 + len(postings.KeyBytes) + 4*(len(postings.KeyOffsets)+len(postings.Offsets)+len(postings.Sources)+len(postings.Counts))
	data := make([]byte, 0, size)
	data = binary.LittleEndian.AppendUint32(data, uint32(len(postings.KeyBytes)))
	data = append(data, postings.KeyBytes...)
	for _, values := range [][]uint32{postings.KeyOffsets, postings.Offsets, postings.Sources, postings.Counts} {
		data = binary.LittleEndian.AppendUint32(data, uint32(len(values)))
		for _, value := range values {
			data = binary.LittleEndian.AppendUint32(data, value)
		}
	}
	return data, nil
}

func (postings *termPostings) UnmarshalBinary(data []byte) error {
	next := func(count int) ([]byte, error) {
		if len(data) < count {
			return nil, errors.New("term table is truncated")
		}
		head := data[:count]
		data = data[count:]
		return head, nil
	}
	length, err := next(4)
	if err != nil {
		return err
	}
	keyBytes, err := next(int(binary.LittleEndian.Uint32(length)))
	if err != nil {
		return err
	}
	postings.KeyBytes = string(keyBytes)
	for _, target := range []*[]uint32{&postings.KeyOffsets, &postings.Offsets, &postings.Sources, &postings.Counts} {
		length, err := next(4)
		if err != nil {
			return err
		}
		count := int(binary.LittleEndian.Uint32(length))
		raw, err := next(4 * count)
		if err != nil {
			return err
		}
		values := make([]uint32, count)
		for index := range values {
			values[index] = binary.LittleEndian.Uint32(raw[4*index:])
		}
		if count == 0 {
			values = nil
		}
		*target = values
	}
	if len(data) != 0 {
		return errors.New("term table has trailing bytes")
	}
	return nil
}

func (postings *termPostings) keyCount() int { return len(postings.KeyOffsets) - 1 }

func (postings *termPostings) key(index int) string {
	return postings.KeyBytes[postings.KeyOffsets[index]:postings.KeyOffsets[index+1]]
}

// find returns the posting range of key, or false.
func (postings *termPostings) find(key string) (int, int, bool) {
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

// check refuses a table whose offsets would index outside their arrays, whose
// postings name a source outside the sourceCount paths of the vocabulary, or
// which is counted and does not carry one count per source. Every decoder of a
// derived snapshot runs it once at open, so key, find and the ranking walks
// index without a bound check of their own (IDX-SNAP-V0-003).
func (postings termPostings) check(sourceCount int, counted bool) error {
	if len(postings.Offsets) != len(postings.KeyOffsets) {
		return errors.New("term table offsets do not match its keys")
	}
	keyLength := uint32(len(postings.KeyBytes))
	for index, offset := range postings.KeyOffsets {
		if offset > keyLength || index > 0 && offset < postings.KeyOffsets[index-1] {
			return errors.New("term table key offsets are not ascending within the keys")
		}
	}
	postingCount := uint32(len(postings.Sources))
	for index, offset := range postings.Offsets {
		if offset > postingCount || index > 0 && offset < postings.Offsets[index-1] {
			return errors.New("term table offsets are not ascending within the postings")
		}
	}
	sourceLimit := uint32(sourceCount)
	for _, source := range postings.Sources {
		if source >= sourceLimit {
			return errors.New("term table names a source outside the vocabulary paths")
		}
	}
	if counted && len(postings.Counts) != len(postings.Sources) {
		return errors.New("counted term table does not carry one count per source")
	}
	if len(postings.Counts) != 0 && len(postings.Counts) != len(postings.Sources) {
		return errors.New("term table counts do not match its sources")
	}
	return nil
}

// check refuses a decoded table any of whose three postings fails
// termPostings.check against its Paths; Terms is the counted one.
func (table *TermTable) check() error {
	if table == nil {
		return nil
	}
	for _, item := range []struct {
		postings termPostings
		counted  bool
	}{{table.Terms, true}, {table.Words, false}, {table.PathTerms, false}} {
		if err := item.postings.check(len(table.Paths), item.counted); err != nil {
			return err
		}
	}
	return table.IdentGraph.check(len(table.Paths))
}

// sourceID returns a path's position in Paths, or false for a path the table
// does not carry.
func (table *TermTable) sourceID(path string) (int, bool) {
	index := sort.SearchStrings(table.Paths, path)
	if index == len(table.Paths) || table.Paths[index] != path {
		return 0, false
	}
	return index, true
}

// sourceTerms is one source's contribution before inversion.
type sourceTerms struct {
	terms     map[string]uint32
	words     map[string]struct{}
	pathTerms []string
}

// buildTermTable tokenises every readable source on all CPUs, then inverts
// the results in path order so each posting list is ascending by
// construction.
func buildTermTable(sources map[string]Source) *TermTable {
	paths := make([]string, 0, len(sources))
	for path := range sources {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	perSource := make([]sourceTerms, len(paths))
	workers := min(runtime.NumCPU(), max(len(paths)/sourcesPerWorker, 1))
	var pending sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		pending.Add(1)
		low, high := worker*len(paths)/workers, (worker+1)*len(paths)/workers
		go func(low, high int) {
			defer pending.Done()
			tokeniser := newTokeniser()
			for index := low; index < high; index++ {
				perSource[index] = tokeniseSource(paths[index], sources[paths[index]], tokeniser)
			}
		}(low, high)
	}
	pending.Wait()
	table := &TermTable{Paths: paths}
	table.Terms = invertCounts(perSource)
	table.Words = invertPresence(perSource, func(entry sourceTerms) map[string]struct{} { return entry.words })
	table.PathTerms = invertPresence(perSource, func(entry sourceTerms) map[string]struct{} { return stringSetOf(entry.pathTerms) })
	return table
}

func tokeniseSource(path string, source Source, tokeniser *tokeniser) sourceTerms {
	entry := sourceTerms{pathTerms: lexicalTerms(path)}
	text, ok := sourceTextBounded(source)
	if !ok {
		return entry
	}
	entry.terms = tokeniser.terms.count(text)
	entry.words = tokeniser.words.scan(text)
	return entry
}

// posting accumulators keyed by an interned key id.
type postingBuilder struct {
	ids     map[string]uint32
	keys    []string
	sources [][]uint32
	counts  [][]uint32
}

func (builder *postingBuilder) add(key string, source uint32, count uint32, counted bool) {
	id, ok := builder.ids[key]
	if !ok {
		id = uint32(len(builder.keys))
		builder.ids[key] = id
		builder.keys = append(builder.keys, key)
		builder.sources = append(builder.sources, nil)
		if counted {
			builder.counts = append(builder.counts, nil)
		}
	}
	builder.sources[id] = append(builder.sources[id], source)
	if counted {
		builder.counts[id] = append(builder.counts[id], count)
	}
}

func invertCounts(perSource []sourceTerms) termPostings {
	workers := min(runtime.NumCPU(), max(len(perSource)/sourcesPerWorker, 1))
	return invertCountsWithWorkers(perSource, workers)
}

func invertCountsWithWorkers(perSource []sourceTerms, workers int) termPostings {
	return invertPostings(perSource, workers, true, func(builder *postingBuilder, source uint32, entry sourceTerms) {
		for key, count := range entry.terms {
			builder.add(key, source, count, true)
		}
	})
}

func invertPresence(perSource []sourceTerms, pick func(sourceTerms) map[string]struct{}) termPostings {
	workers := min(runtime.NumCPU(), max(len(perSource)/sourcesPerWorker, 1))
	return invertPresenceWithWorkers(perSource, workers, pick)
}

func invertPresenceWithWorkers(perSource []sourceTerms, workers int, pick func(sourceTerms) map[string]struct{}) termPostings {
	return invertPostings(perSource, workers, false, func(builder *postingBuilder, source uint32, entry sourceTerms) {
		for key := range pick(entry) {
			builder.add(key, source, 0, false)
		}
	})
}

// invertPostings gives each worker a contiguous source range. Merging shards
// in worker order therefore keeps every posting list in ascending source order
// without depending on goroutine completion or map iteration order.
func invertPostings(perSource []sourceTerms, workers int, counted bool, add func(*postingBuilder, uint32, sourceTerms)) termPostings {
	builders := make([]postingBuilder, workers)
	var pending sync.WaitGroup
	for worker := range builders {
		builders[worker].ids = make(map[string]uint32, interningHint)
		pending.Add(1)
		low, high := worker*len(perSource)/workers, (worker+1)*len(perSource)/workers
		go func(builder *postingBuilder, low, high int) {
			defer pending.Done()
			for source := low; source < high; source++ {
				add(builder, uint32(source), perSource[source])
			}
		}(&builders[worker], low, high)
	}
	pending.Wait()
	return mergePostingBuilders(builders, counted)
}

// postingKeyShard locates one shard's local posting list for a key discovered
// while indexing mergePostingBuilders' shards.
type postingKeyShard struct{ shard, local int }

// mergePostingBuilders writes every shard's posting lists straight into one
// exactly-sized output instead of copying them into an intermediate merged
// postingBuilder first (whose per-key slices grew by repeated append) and
// copying that again in a second pass. Shards cover contiguous ascending
// source ranges (invertPostings), so concatenating a key's shard lists in
// shard order -- exactly what the loop below does -- already yields the
// ascending source order the format requires; no intermediate merge is
// needed to establish it. The merged vocabulary is at least the largest
// shard's, so sizing the key map to that shard up front skips most of its
// regrowth (the shards' own maps use interningHint for the same reason).
func mergePostingBuilders(builders []postingBuilder, counted bool) termPostings {
	hint := 0
	for _, builder := range builders {
		hint = max(hint, len(builder.keys))
	}
	// ids and refs mirror the discarded postingBuilder's own id/keys pair: one
	// map lookup-or-insert per (shard, key), then a plain slice-index append,
	// exactly the cost the old merge already paid. What it no longer appends
	// is the posting payload itself -- refs records where each key's data
	// lives (shard, local id) instead of copying the uint32s there and then
	// copying them again in flatten.
	ids := make(map[string]int, hint)
	keys := make([]string, 0, hint)
	refs := make([][]postingKeyShard, 0, hint)
	totalSources, totalKeyBytes := 0, 0
	for shard := range builders {
		builder := &builders[shard]
		for local, key := range builder.keys {
			id, ok := ids[key]
			if !ok {
				id = len(keys)
				ids[key] = id
				keys = append(keys, key)
				refs = append(refs, nil)
				totalKeyBytes += len(key)
			}
			refs[id] = append(refs[id], postingKeyShard{shard, local})
			totalSources += len(builder.sources[local])
		}
	}
	order := make([]int, len(keys))
	for index := range order {
		order[index] = index
	}
	sort.Slice(order, func(left, right int) bool { return keys[order[left]] < keys[order[right]] })

	keyBytes := make([]byte, totalKeyBytes)
	keyOffsets := make([]uint32, len(order)+1)
	offsets := make([]uint32, len(order)+1)
	sources := make([]uint32, totalSources)
	var counts []uint32
	if counted {
		counts = make([]uint32, totalSources)
	}
	keyPos, sourcePos := 0, 0
	for index, id := range order {
		keyOffsets[index] = uint32(keyPos)
		offsets[index] = uint32(sourcePos)
		keyPos += copy(keyBytes[keyPos:], keys[id])
		for _, ref := range refs[id] {
			builder := &builders[ref.shard]
			n := copy(sources[sourcePos:], builder.sources[ref.local])
			if counted {
				copy(counts[sourcePos:], builder.counts[ref.local])
			}
			sourcePos += n
		}
	}
	keyOffsets[len(order)] = uint32(keyPos)
	offsets[len(order)] = uint32(sourcePos)
	return termPostings{KeyBytes: string(keyBytes), KeyOffsets: keyOffsets, Offsets: offsets, Sources: sources, Counts: counts}
}

func isASCIIAlnum(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func isASCIILower(c byte) bool { return c >= 'a' && c <= 'z' }
func isASCIIUpper(c byte) bool { return c >= 'A' && c <= 'Z' }

func isWordByte(c byte) bool { return isASCIIAlnum(c) || c == '_' }

// countTerms is lexicalTerms with occurrence counts and without the
// allocation: a term is a maximal ASCII alphanumeric run, split where a
// lowercase letter meets an uppercase one (contextCamel), lowercased, of at
// least two bytes (contextToken and the length rule in lexicalTerms).
func countTerms(text string) map[string]uint32 {
	return newTermCounter().count(text)
}

// scanWords is the identifier vocabulary of a source: every maximal run of
// ASCII word bytes of at least three, as written (contextIdentifier's shape).
func scanWords(text string) map[string]struct{} {
	return newTermCounter().scan(text)
}

// tokeniser is one tokenising worker's reusable state: a term counter and
// a word counter, each interning across the worker's sources.
type tokeniser struct {
	terms *termCounter
	words *termCounter
}

func newTokeniser() *tokeniser {
	return &tokeniser{terms: newTermCounter(), words: newTermCounter()}
}

// termCounter is the working state behind countTerms and scanWords, owned
// by one tokenising worker and reused across its sources. Counting straight
// into a fresh map cost a map assignment per token and a regrow as each
// source's vocabulary appeared -- 0.27 s of a 1.86 s build in
// mapassign_faststr and rehash across the two scans, plus a string
// allocation per term for countTerms. The counter interns each key once per
// worker (a lookup on a byte slice or substring allocates nothing), counts
// by key id in a slice, and materialises the per-source map at its exact
// size with keys the interner already owns. The maps handed back are
// unchanged in content, so the postings inverted from them are the same
// bytes.
type termCounter struct {
	ids     map[string]uint32
	keys    []string
	counts  []uint32
	touched []uint32
	buffer  []byte
}

// interningHint sizes a worker's interner for a source window's vocabulary
// up front; the remaining mapassign cost was the interner regrowing.
const interningHint = 1 << 13

func newTermCounter() *termCounter {
	return &termCounter{ids: make(map[string]uint32, interningHint)}
}

// count is countTerms: maximal ASCII alphanumeric runs, split where a
// lowercase letter meets an uppercase one, lowercased, at least two bytes.
func (counter *termCounter) count(text string) map[string]uint32 {
	for index := 0; index < len(text); index++ {
		c := text[index]
		if !isASCIIAlnum(c) {
			counter.flush()
			continue
		}
		if isASCIIUpper(c) {
			if index > 0 && isASCIILower(text[index-1]) {
				counter.flush()
			}
			c += 'a' - 'A'
		}
		counter.buffer = append(counter.buffer, c)
	}
	counter.flush()
	return counter.take()
}

// scan is scanWords: every maximal run of ASCII word bytes of at least
// three, as written. Each word is a substring of text, so interning it
// allocates nothing.
func (counter *termCounter) scan(text string) map[string]struct{} {
	start := -1
	for index := 0; index <= len(text); index++ {
		inWord := index < len(text) && isWordByte(text[index])
		if inWord {
			if start < 0 {
				start = index
			}
			continue
		}
		if start >= 0 && index-start >= 3 {
			counter.add(counter.intern(text[start:index]))
		}
		start = -1
	}
	return counter.takePresence()
}

// flush counts the buffered term when it is at least two bytes long.
func (counter *termCounter) flush() {
	if len(counter.buffer) >= 2 {
		counter.add(counter.internBuffer())
	}
	counter.buffer = counter.buffer[:0]
}

// internBuffer is intern for the byte buffer, converting to a string only
// the first time this worker meets the term.
func (counter *termCounter) internBuffer() uint32 {
	if id, ok := counter.ids[string(counter.buffer)]; ok {
		return id
	}
	return counter.intern(string(counter.buffer))
}

func (counter *termCounter) intern(key string) uint32 {
	if id, ok := counter.ids[key]; ok {
		return id
	}
	id := uint32(len(counter.keys))
	counter.ids[key] = id
	counter.keys = append(counter.keys, key)
	counter.counts = append(counter.counts, 0)
	return id
}

func (counter *termCounter) add(id uint32) {
	if counter.counts[id] == 0 {
		counter.touched = append(counter.touched, id)
	}
	counter.counts[id]++
}

// take materialises the current source's counts as the map the inverter
// reads, sized exactly, and resets the touched ids for the next source.
func (counter *termCounter) take() map[string]uint32 {
	terms := make(map[string]uint32, len(counter.touched))
	for _, id := range counter.touched {
		terms[counter.keys[id]] = counter.counts[id]
		counter.counts[id] = 0
	}
	counter.touched = counter.touched[:0]
	return terms
}

// takePresence is take for a presence set.
func (counter *termCounter) takePresence() map[string]struct{} {
	words := make(map[string]struct{}, len(counter.touched))
	for _, id := range counter.touched {
		words[counter.keys[id]] = struct{}{}
		counter.counts[id] = 0
	}
	counter.touched = counter.touched[:0]
	return words
}
