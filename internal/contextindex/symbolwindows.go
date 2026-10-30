package contextindex

import (
	"encoding/binary"
	"errors"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// windowPostings is SymbolWindows' layout: the keys as termPostings keeps
// them, and for key k the ascending symbol positions as unsigned LEB128
// deltas in Data[Offsets[k]:Offsets[k+1]], the first delta from zero. A
// window's postings are dense in the symbol order, so a delta is mostly one
// byte where the fixed-width table spends four, and the walk decodes only the
// lists of the terms a query names.
type windowPostings struct {
	KeyBytes   string
	KeyOffsets []uint32
	Offsets    []uint32
	Data       []byte
}

// compressWindows re-lays a flattened posting table as deltas.
func compressWindows(postings termPostings) windowPostings {
	offsets := make([]uint32, 0, len(postings.Offsets))
	data := make([]byte, 0, len(postings.Sources))
	for key := 0; key < postings.keyCount(); key++ {
		offsets = append(offsets, uint32(len(data)))
		previous := uint32(0)
		for _, position := range postings.Sources[postings.Offsets[key]:postings.Offsets[key+1]] {
			data = binary.AppendUvarint(data, uint64(position-previous))
			previous = position
		}
	}
	offsets = append(offsets, uint32(len(data)))
	return windowPostings{KeyBytes: postings.KeyBytes, KeyOffsets: postings.KeyOffsets, Offsets: offsets, Data: data}
}

func (postings windowPostings) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, 4*4+len(postings.KeyBytes)+4*(len(postings.KeyOffsets)+len(postings.Offsets))+len(postings.Data))
	data = binary.LittleEndian.AppendUint32(data, uint32(len(postings.KeyBytes)))
	data = append(data, postings.KeyBytes...)
	for _, values := range [][]uint32{postings.KeyOffsets, postings.Offsets} {
		data = binary.LittleEndian.AppendUint32(data, uint32(len(values)))
		for _, value := range values {
			data = binary.LittleEndian.AppendUint32(data, value)
		}
	}
	data = binary.LittleEndian.AppendUint32(data, uint32(len(postings.Data)))
	return append(data, postings.Data...), nil
}

func (postings *windowPostings) UnmarshalBinary(data []byte) error {
	next := func(count int) ([]byte, error) {
		if count < 0 || len(data) < count {
			return nil, errors.New("symbol window table is truncated")
		}
		head := data[:count]
		data = data[count:]
		return head, nil
	}
	length := func() (int, error) {
		raw, err := next(4)
		if err != nil {
			return 0, err
		}
		return int(binary.LittleEndian.Uint32(raw)), nil
	}
	count, err := length()
	if err != nil {
		return err
	}
	keyBytes, err := next(count)
	if err != nil {
		return err
	}
	postings.KeyBytes = string(keyBytes)
	for _, target := range []*[]uint32{&postings.KeyOffsets, &postings.Offsets} {
		if count, err = length(); err != nil {
			return err
		}
		if count > len(data)/4 {
			return errors.New("symbol window table is truncated")
		}
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
	if count, err = length(); err != nil {
		return err
	}
	raw, err := next(count)
	if err != nil {
		return err
	}
	postings.Data = nil
	if count > 0 {
		postings.Data = append([]byte(nil), raw...)
	}
	if len(data) != 0 {
		return errors.New("symbol window table has trailing bytes")
	}
	return nil
}

func (postings *windowPostings) keyCount() int { return len(postings.KeyOffsets) - 1 }

func (postings *windowPostings) key(index int) string {
	return postings.KeyBytes[postings.KeyOffsets[index]:postings.KeyOffsets[index+1]]
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

// walk visits the symbol positions of one list in ascending order, and
// reports a list whose deltas do not decode; check refuses such a table.
func (postings *windowPostings) walk(low, high int, visit func(uint32)) bool {
	position := uint64(0)
	for data := postings.Data[low:high]; len(data) > 0; {
		delta, size := binary.Uvarint(data)
		if size <= 0 {
			return false
		}
		position += delta
		data = data[size:]
		visit(uint32(position))
	}
	return true
}

// check reports a table whose offsets or positions fall outside the index it
// was decoded for.
func (postings *windowPostings) check(symbolCount int) error {
	keyLength, dataLength := uint32(len(postings.KeyBytes)), uint32(len(postings.Data))
	if len(postings.KeyOffsets) != 0 && postings.KeyOffsets[0] != 0 {
		return errors.New("symbol window table keys do not start at the first byte")
	}
	for index, offset := range postings.KeyOffsets {
		if offset > keyLength || index > 0 && offset < postings.KeyOffsets[index-1] {
			return errors.New("symbol window table key offsets are not ascending within the keys")
		}
	}
	for index := 1; index < postings.keyCount(); index++ {
		if postings.key(index-1) >= postings.key(index) {
			return errors.New("symbol window table keys are not strictly ascending")
		}
	}
	if len(postings.KeyOffsets) != len(postings.Offsets) {
		return errors.New("symbol window table offsets do not match its keys")
	}
	for index, offset := range postings.Offsets {
		if offset > dataLength || index > 0 && offset < postings.Offsets[index-1] {
			return errors.New("symbol window table offsets are not ascending within the postings")
		}
	}
	var err error
	for key := 0; key < postings.keyCount(); key++ {
		decoded := postings.walk(int(postings.Offsets[key]), int(postings.Offsets[key+1]), func(position uint32) {
			if int(position) >= symbolCount {
				err = errors.New("symbol window table names a symbol outside the index")
			}
		})
		if !decoded {
			return errors.New("symbol window table postings do not decode")
		}
	}
	return err
}

// checkSymbolWindows refuses a decoded snapshot whose symbol window table
// does not fit its symbols, so a damaged table is a load miss and never a
// panic in the ranking. An index without the table passes.
func (index *Index) checkSymbolWindows() error {
	if index.Vocabulary == nil {
		return nil
	}
	return index.Vocabulary.SymbolWindows.check(len(index.Symbols))
}

// symbolContextText is the text evalRankSymbols scores a symbol's context
// terms from: its symbolContextWindow lines joined and capped at 2000 runes.
// It is one function so the table below and the scan it replaces tokenise the
// same string.
func symbolContextText(symbol Symbol, lines []string) string {
	start, end := symbolContextWindow(symbol, len(lines))
	if start >= end {
		return ""
	}
	return joinLinesCapped(lines[start:end], symbolContextRunes)
}

// symbolContextRunes is the rune cap on a symbol's context text.
const symbolContextRunes = 2000

// joinLinesCapped is truncateRunes(strings.Join(lines, "\n"), maximum)
// without joining or decoding past the cap (V1-3016): a window of long lines
// costs O(maximum), not O(window bytes). Like truncateRunes, a window within
// the cap is returned byte for byte and a longer one is re-encoded rune by
// rune, so an invalid byte becomes U+FFFD only on the truncating branch.
func joinLinesCapped(lines []string, maximum int) string {
	count := 0
	for index, line := range lines {
		if index > 0 {
			count++
		}
		for range line {
			if count++; count > maximum {
				return encodeLinesCapped(lines, maximum)
			}
		}
		if count > maximum {
			return encodeLinesCapped(lines, maximum)
		}
	}
	return strings.Join(lines, "\n")
}

func encodeLinesCapped(lines []string, maximum int) string {
	var text strings.Builder
	text.Grow(maximum)
	count := 0
	for index, line := range lines {
		if index > 0 {
			if count == maximum {
				break
			}
			text.WriteByte('\n')
			count++
		}
		for _, r := range line {
			if count == maximum {
				return text.String()
			}
			text.WriteRune(r)
			count++
		}
	}
	return text.String()
}

// buildSymbolWindows inverts terms(symbolContextText) over every symbol the
// ranking can prepare. Symbols under test paths are left out: evalSymbolChunk
// drops them before it reads the window, so their postings could never be
// read. Workers own contiguous symbol ranges and shards merge in worker
// order, so each posting list is ascending by construction.
func (index *Index) buildSymbolWindows() windowPostings {
	workers := min(runtime.NumCPU(), max(len(index.Symbols)/symbolsPerWorker, 1))
	builders := make([]postingBuilder, workers)
	var pending sync.WaitGroup
	for worker := range builders {
		builders[worker].ids = make(map[string]uint32, interningHint)
		pending.Add(1)
		low, high := worker*len(index.Symbols)/workers, (worker+1)*len(index.Symbols)/workers
		go func(builder *postingBuilder, low, high int) {
			defer pending.Done()
			sources := evalSourceLines{index: index}
			// One set per worker, cleared between symbols: a fresh terms() map
			// per symbol and its regrowth were the largest allocation site left
			// in the build (GPK-V0-088). collectTerms is the body of terms,
			// and a posting list does not depend on the order a set is walked.
			window := make(map[string]struct{}, 256)
			// Neighbouring symbols with one window (names declared on one
			// line) reuse its terms instead of tokenising it again (V1-3016).
			type windowKey struct {
				path       string
				start, end int
			}
			last, built := windowKey{}, false
			for position := low; position < high; position++ {
				symbol := index.Symbols[position]
				if isTestPath(symbol.Path) {
					continue
				}
				lines := sources.of(symbol.Path).lines
				start, end := symbolContextWindow(symbol, len(lines))
				if key := (windowKey{symbol.Path, start, end}); !built || key != last {
					clear(window)
					collectTerms(symbolContextText(symbol, lines), window)
					last, built = key, true
				}
				for term := range window {
					builder.add(term, uint32(position), 0, false)
				}
			}
		}(&builders[worker], low, high)
	}
	pending.Wait()
	return compressWindows(mergePostingBuilders(builders, false))
}

// evalScannedContextWindow makes evalPrepareSymbols scan the windows even when
// the snapshot carries SymbolWindows. Nothing in the product sets it; the
// receipt parity test flips it so one binary can emit the same receipt both
// ways and compare them.
var evalScannedContextWindow = false

// symbolWindowTerms is keepSetTerms(symbolContextText, queryTerms) for every
// symbol at once, read out of SymbolWindows: one posting walk per query term
// instead of one tokenisation per symbol. A symbol no query term reaches keeps
// a nil set, which every reader treats as empty. It is nil when the index
// carries no table, and evalSymbolChunk scans the windows instead.
func (index *Index) symbolWindowTerms(queryTerms map[string]struct{}) []map[string]struct{} {
	if evalScannedContextWindow || index.Vocabulary == nil || index.Vocabulary.SymbolWindows.keyCount() < 0 {
		return nil
	}
	table := &index.Vocabulary.SymbolWindows
	contexts := make([]map[string]struct{}, len(index.Symbols))
	for term := range queryTerms {
		low, high, ok := table.find(term)
		if !ok {
			continue
		}
		table.walk(low, high, func(position uint32) {
			if contexts[position] == nil {
				contexts[position] = make(map[string]struct{})
			}
			contexts[position][term] = struct{}{}
		})
	}
	return contexts
}
