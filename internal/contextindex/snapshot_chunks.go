package contextindex

import (
	"encoding"
	"encoding/gob"
	"errors"
	"reflect"
)

// The gob snapshot body (format corvint-index-snapshot/3, IDX-SNAP-V0-806).
// Format /2 carried the whole index as one gob message of about 177 MB on
// this repository. encoding/gob reads a message over 10 MiB in 10 MiB pieces
// appended to a growing slice (internal/saferio.ReadData), so every load
// regrew and copied that buffer many times: 27 to 39% of a hook's or
// context's CPU. Format /3 carries the same snapshotWire as a sequence of
// messages, each well under that threshold: a spine with every small field,
// then the source bodies, symbols and fields in bounded chunks, then the term
// table's binary blobs in bounded chunks that the reader copies into one
// buffer of the exact length. Every message is written in a fixed order from
// key-sorted data, so the bytes stay a function of the index (IDX-SNAP-V0-106),
// and the decoded snapshotWire equals the /2 decode of the same index.

// gobSnapshotFormat is the header format of the gob snapshot only; the
// sectioned and pack encodings keep snapshotFormat.
const gobSnapshotFormat = "corvint-index-snapshot/4"

// snapshotChunkBytes bounds one chunk message's payload, well under gob's
// 10 MiB single-allocation threshold even with its framing.
var snapshotChunkBytes = 4 << 20

// snapshotSymbolChunk bounds the symbols or fields one message carries.
var snapshotSymbolChunk = 8192

// snapshotBodyDepth says how much of the body a reader needs; the messages
// after it are left undecoded (the trailer digest still covers them).
type snapshotBodyDepth int

const (
	snapshotBodySpine  snapshotBodyDepth = iota // ProfileID and the other small fields
	snapshotBodyEvents                          // also sources, symbols and fields
	snapshotBodyFull                            // also the term table
	// snapshotBodySources reads the spine and the sources only, every body
	// copied and verified: the IDX-SNAP-V0-011 probe (IDX-SNAP-V0-811).
	snapshotBodySources
)

// snapshotSpine is every snapshotWire field except the chunked ones, with the
// counts a reader needs to read those back.
type snapshotSpine struct {
	ObjectFormat, CommitRevision, Revision, ProfileID, Module string
	SourcesPresent                                            bool
	SourceKeys                                                []string
	Tracked, Skipped                                          wireSet
	Exclusions                                                []Exclusion
	UnsupportedSuffixCount                                    int
	Features, Scenarios, Documents                            wirePairs[wireRecord]
	Markers                                                   wirePairs[[]Marker]
	SymbolCount, FieldCount                                   int
	Imports                                                   wirePairs[[]string]
	Unparsed                                                  []Unparsed
	ApproximateImports                                        int
	ExtractionNotes                                           []ExtractionNote
	VocabularyPresent                                         bool
	// SymbolTermsLength is the byte length of the symbol term table blob
	// that follows the term table, 0 when there is none (IDX-SNAP-V0-807).
	SymbolTermsLength int
	// TestLinksLength is the byte length of the test-link table blob that
	// follows the symbol term table, 0 when there is none (IDX-SNAP-V0-809).
	TestLinksLength int
}

// snapshotVocabularySpine is a TermTable's Paths and the length of each of
// its binary blobs, -1 for a blob gob would omit (a zero value or nil graph).
type snapshotVocabularySpine struct {
	Paths       []string
	BlobLengths []int
}

func vocabularyBlobs(table *TermTable) []any {
	return []any{table.Terms, table.Words, table.PathTerms, table.SymbolWindows, table.IdentGraph}
}

func vocabularyTargets(table *TermTable) []any {
	if table.IdentGraph == nil {
		table.IdentGraph = &identGraph{}
	}
	return []any{&table.Terms, &table.Words, &table.PathTerms, &table.SymbolWindows, table.IdentGraph}
}

// encodeSnapshotBody writes wire as the message sequence and returns the
// body region's parts, in order, for the caller to write after the messages
// (IDX-SNAP-V0-811).
func encodeSnapshotBody(encoder *gob.Encoder, wire *snapshotWire) ([][]byte, error) {
	if len(wire.Sources.Keys) != len(wire.Sources.Values) {
		return nil, errWirePairs
	}
	spine := snapshotSpineOf(wire)
	if err := encoder.Encode(&spine); err != nil {
		return nil, err
	}
	return encodeSnapshotChunks(encoder, wire)
}

func snapshotSpineOf(wire *snapshotWire) snapshotSpine {
	return snapshotSpine{
		ObjectFormat: wire.ObjectFormat, CommitRevision: wire.CommitRevision, Revision: wire.Revision,
		ProfileID: wire.ProfileID, Module: wire.Module,
		SourcesPresent: wire.Sources.Present, SourceKeys: wire.Sources.Keys,
		Tracked: wire.Tracked, Skipped: wire.Skipped, Exclusions: wire.Exclusions,
		UnsupportedSuffixCount: wire.UnsupportedSuffixCount,
		Features:               wire.Features, Scenarios: wire.Scenarios, Documents: wire.Documents, Markers: wire.Markers,
		SymbolCount: len(wire.Symbols), FieldCount: len(wire.Fields), Imports: wire.Imports,
		SymbolTermsLength: len(symbolTermsBlob(wire)), TestLinksLength: len(testLinksBlob(wire)),
		Unparsed: wire.Unparsed, ApproximateImports: wire.ApproximateImports, ExtractionNotes: wire.ExtractionNotes,
		VocabularyPresent: wire.Vocabulary != nil,
	}
}

// testLinksBlob is the marshalled test-link table, nil without one.
func testLinksBlob(wire *snapshotWire) []byte {
	if wire.testLinks == nil {
		return nil
	}
	data, _ := wire.testLinks.MarshalBinary()
	return data
}

// symbolTermsBlob is the marshalled symbol term table, nil without one.
func symbolTermsBlob(wire *snapshotWire) []byte {
	if wire.symbolTerms == nil {
		return nil
	}
	data, _ := wire.symbolTerms.MarshalBinary()
	return data
}

// encodeSnapshotChunks writes the messages after the spine and returns the
// body region's parts.
func encodeSnapshotChunks(encoder *gob.Encoder, wire *snapshotWire) ([][]byte, error) {
	metas, region := gobSourceMetas(wire.Sources.Values, wire.ObjectFormat)
	for low := 0; low < len(metas); {
		high, size := low, 0
		for high < len(metas) && (high == low || high-low < snapshotSymbolChunk && size+len(metas[high].Inline) <= snapshotChunkBytes) {
			size += len(metas[high].Inline)
			high++
		}
		if err := encoder.Encode(metas[low:high]); err != nil {
			return nil, err
		}
		low = high
	}
	for _, symbols := range [][]Symbol{wire.Symbols, wire.Fields} {
		for low := 0; low < len(symbols); low += snapshotSymbolChunk {
			if err := encoder.Encode(symbols[low:min(low+snapshotSymbolChunk, len(symbols))]); err != nil {
				return nil, err
			}
		}
	}
	if wire.Vocabulary != nil {
		if err := encodeVocabularyChunks(encoder, wire.Vocabulary); err != nil {
			return nil, err
		}
	}
	if err := encodeBlobChunks(encoder, symbolTermsBlob(wire)); err != nil {
		return nil, err
	}
	return region, encodeBlobChunks(encoder, testLinksBlob(wire))
}

func encodeBlobChunks(encoder *gob.Encoder, data []byte) error {
	for low := 0; low < len(data); low += snapshotChunkBytes {
		if err := encoder.Encode(data[low:min(low+snapshotChunkBytes, len(data))]); err != nil {
			return err
		}
	}
	return nil
}

func encodeVocabularyChunks(encoder *gob.Encoder, table *TermTable) error {
	blobs := make([][]byte, 0, 5)
	vocabulary := snapshotVocabularySpine{Paths: table.Paths}
	for _, blob := range vocabularyBlobs(table) {
		// Gob omits a zero-valued field, and a nil graph, from a struct, so the
		// /2 decode left them zero; -1 keeps that distinction.
		value := reflect.ValueOf(blob)
		if value.IsZero() {
			vocabulary.BlobLengths = append(vocabulary.BlobLengths, -1)
			blobs = append(blobs, nil)
			continue
		}
		data, err := blob.(encoding.BinaryMarshaler).MarshalBinary()
		if err != nil {
			return err
		}
		vocabulary.BlobLengths = append(vocabulary.BlobLengths, len(data))
		blobs = append(blobs, data)
	}
	if err := encoder.Encode(&vocabulary); err != nil {
		return err
	}
	for _, data := range blobs {
		if err := encodeBlobChunks(encoder, data); err != nil {
			return err
		}
	}
	return nil
}

var errSnapshotChunks = errors.New("snapshot chunks are malformed")

// unboundedMessageBytes stands in for the message length when a caller does
// not know it (a test decoding a buffer it built).
const unboundedMessageBytes = 1 << 31

// countBound is the largest count or byte length a spine may declare: every
// symbol, field or blob byte takes at least one byte of the messages, so a
// count above the message length is malformed and refused before anything
// is allocated for it (IDX-SNAP-V0-812).
func (bodies *snapshotBodies) countBound() int {
	if bodies == nil || bodies.messageBytes <= 0 {
		return unboundedMessageBytes
	}
	return int(min(bodies.messageBytes, unboundedMessageBytes))
}

// preallocation caps a capacity taken from a bounded count, so even a
// within-bound count reserves no more than a modest slice up front.
func preallocation(count int) int { return min(count, 1<<16) }

// decodeSnapshotBody reads the message sequence up to depth, with the
// sources' bodies from bodies (nil for a body with no region). Malformed
// counts or chunk lengths are an error, which is the IDX-SNAP-V0-003 miss.
func decodeSnapshotBody(decoder *gob.Decoder, depth snapshotBodyDepth, bodies *snapshotBodies) (*snapshotWire, error) {
	var spine snapshotSpine
	if err := decoder.Decode(&spine); err != nil {
		return nil, err
	}
	bound := bodies.countBound()
	if spine.SymbolCount < 0 || spine.FieldCount < 0 || (!spine.SourcesPresent && len(spine.SourceKeys) != 0) ||
		spine.SymbolCount > bound || spine.FieldCount > bound || spine.SymbolTermsLength > bound || spine.TestLinksLength > bound {
		return nil, errSnapshotChunks
	}
	wire := &snapshotWire{
		ObjectFormat: spine.ObjectFormat, CommitRevision: spine.CommitRevision, Revision: spine.Revision,
		ProfileID: spine.ProfileID, Module: spine.Module,
		Sources: wirePairs[Source]{Present: spine.SourcesPresent, Keys: spine.SourceKeys},
		Tracked: spine.Tracked, Skipped: spine.Skipped, Exclusions: spine.Exclusions,
		UnsupportedSuffixCount: spine.UnsupportedSuffixCount,
		Features:               spine.Features, Scenarios: spine.Scenarios, Documents: spine.Documents, Markers: spine.Markers,
		Imports: spine.Imports, Unparsed: spine.Unparsed, ApproximateImports: spine.ApproximateImports,
		ExtractionNotes: spine.ExtractionNotes,
	}
	if depth == snapshotBodySpine {
		return wire, nil
	}
	metas := make([]wireSourceMeta, 0, len(spine.SourceKeys))
	for len(metas) < len(spine.SourceKeys) {
		var chunk []wireSourceMeta
		if err := decoder.Decode(&chunk); err != nil {
			return nil, err
		}
		if len(chunk) == 0 || len(metas)+len(chunk) > len(spine.SourceKeys) {
			return nil, errSnapshotChunks
		}
		metas = append(metas, chunk...)
	}
	if len(metas) != 0 {
		values, err := sourcesFromMetas(metas, bodies)
		if err != nil {
			return nil, err
		}
		wire.Sources.Values = values
	} else if bodies != nil && len(bodies.region) != 0 {
		return nil, errSnapshotChunks
	}
	if depth == snapshotBodySources {
		return wire, nil
	}
	for _, read := range []struct {
		count  int
		target *[]Symbol
	}{{spine.SymbolCount, &wire.Symbols}, {spine.FieldCount, &wire.Fields}} {
		if read.count == 0 {
			continue
		}
		*read.target = make([]Symbol, 0, preallocation(read.count))
		for len(*read.target) < read.count {
			var chunk []Symbol
			if err := decoder.Decode(&chunk); err != nil {
				return nil, err
			}
			if len(chunk) == 0 || len(*read.target)+len(chunk) > read.count {
				return nil, errSnapshotChunks
			}
			*read.target = append(*read.target, chunk...)
		}
	}
	if depth == snapshotBodyEvents {
		return wire, nil
	}
	if spine.VocabularyPresent {
		table, err := decodeVocabularyChunks(decoder, bound)
		if err != nil {
			return nil, err
		}
		wire.Vocabulary = table
	}
	if spine.SymbolTermsLength < 0 {
		return nil, errSnapshotChunks
	}
	if spine.SymbolTermsLength > 0 {
		data, err := decodeSnapshotBlob(decoder, spine.SymbolTermsLength, bound)
		if err != nil {
			return nil, err
		}
		wire.symbolTerms = &symbolTermTable{}
		if err := wire.symbolTerms.UnmarshalBinary(data); err != nil {
			return nil, err
		}
	}
	if spine.TestLinksLength < 0 {
		return nil, errSnapshotChunks
	}
	if spine.TestLinksLength > 0 {
		data, err := decodeSnapshotBlob(decoder, spine.TestLinksLength, bound)
		if err != nil {
			return nil, err
		}
		wire.testLinks = &testLinkTable{}
		if err := wire.testLinks.UnmarshalBinary(data); err != nil {
			return nil, err
		}
	}
	return wire, nil
}

func decodeVocabularyChunks(decoder *gob.Decoder, bound int) (*TermTable, error) {
	var vocabulary snapshotVocabularySpine
	if err := decoder.Decode(&vocabulary); err != nil {
		return nil, err
	}
	table := &TermTable{Paths: vocabulary.Paths}
	targets := vocabularyTargets(table)
	if len(vocabulary.BlobLengths) != len(targets) {
		return nil, errSnapshotChunks
	}
	graphPresent := false
	for position, length := range vocabulary.BlobLengths {
		if length == -1 {
			continue
		}
		if length < 0 {
			return nil, errSnapshotChunks
		}
		data, err := decodeSnapshotBlob(decoder, length, bound)
		if err != nil {
			return nil, err
		}
		if err := targets[position].(encoding.BinaryUnmarshaler).UnmarshalBinary(data); err != nil {
			return nil, err
		}
		graphPresent = graphPresent || position == len(targets)-1
	}
	if !graphPresent {
		table.IdentGraph = nil
	}
	return table, nil
}

// decodeSnapshotBlob reads one blob of length bytes from its chunk messages
// into a single buffer of exactly that length. Each chunk decodes in place
// when gob can reuse the destination, so the blob is copied once out of the
// message buffer.
func decodeSnapshotBlob(decoder *gob.Decoder, length, bound int) ([]byte, error) {
	if length < 0 || length > bound {
		return nil, errSnapshotChunks
	}
	data := make([]byte, length)
	for low := 0; low < length; {
		want := min(snapshotChunkBytes, length-low)
		chunk := data[low : low+want : low+want]
		if err := decoder.Decode(&chunk); err != nil {
			return nil, err
		}
		if len(chunk) != want {
			return nil, errSnapshotChunks
		}
		if &chunk[0] != &data[low] {
			copy(data[low:], chunk)
		}
		low += want
	}
	return data, nil
}
