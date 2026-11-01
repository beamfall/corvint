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
const gobSnapshotFormat = "corvint-index-snapshot/3"

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

// encodeSnapshotBody writes wire as the /3 message sequence.
func encodeSnapshotBody(encoder *gob.Encoder, wire *snapshotWire) error {
	if len(wire.Sources.Keys) != len(wire.Sources.Values) {
		return errWirePairs
	}
	spine := snapshotSpineOf(wire)
	if err := encoder.Encode(&spine); err != nil {
		return err
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
		SymbolTermsLength: len(symbolTermsBlob(wire)),
		Unparsed:          wire.Unparsed, ApproximateImports: wire.ApproximateImports, ExtractionNotes: wire.ExtractionNotes,
		VocabularyPresent: wire.Vocabulary != nil,
	}
}

// symbolTermsBlob is the marshalled symbol term table, nil without one.
func symbolTermsBlob(wire *snapshotWire) []byte {
	if wire.symbolTerms == nil {
		return nil
	}
	data, _ := wire.symbolTerms.MarshalBinary()
	return data
}

// encodeSnapshotChunks writes the messages after the spine.
func encodeSnapshotChunks(encoder *gob.Encoder, wire *snapshotWire) error {
	for low := 0; low < len(wire.Sources.Values); {
		high, size := low, 0
		for high < len(wire.Sources.Values) && (high == low || size+len(wire.Sources.Values[high].Data) <= snapshotChunkBytes) {
			size += len(wire.Sources.Values[high].Data)
			high++
		}
		if err := encoder.Encode(wire.Sources.Values[low:high]); err != nil {
			return err
		}
		low = high
	}
	for _, symbols := range [][]Symbol{wire.Symbols, wire.Fields} {
		for low := 0; low < len(symbols); low += snapshotSymbolChunk {
			if err := encoder.Encode(symbols[low:min(low+snapshotSymbolChunk, len(symbols))]); err != nil {
				return err
			}
		}
	}
	if wire.Vocabulary != nil {
		if err := encodeVocabularyChunks(encoder, wire.Vocabulary); err != nil {
			return err
		}
	}
	return encodeBlobChunks(encoder, symbolTermsBlob(wire))
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

// decodeSnapshotBody reads the /3 message sequence up to depth. Malformed
// counts or chunk lengths are an error, which is the IDX-SNAP-V0-003 miss.
func decodeSnapshotBody(decoder *gob.Decoder, depth snapshotBodyDepth) (*snapshotWire, error) {
	var spine snapshotSpine
	if err := decoder.Decode(&spine); err != nil {
		return nil, err
	}
	if spine.SymbolCount < 0 || spine.FieldCount < 0 || (!spine.SourcesPresent && len(spine.SourceKeys) != 0) {
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
	if len(spine.SourceKeys) != 0 {
		wire.Sources.Values = make([]Source, 0, len(spine.SourceKeys))
	}
	for len(wire.Sources.Values) < len(spine.SourceKeys) {
		var chunk []Source
		if err := decoder.Decode(&chunk); err != nil {
			return nil, err
		}
		if len(chunk) == 0 || len(wire.Sources.Values)+len(chunk) > len(spine.SourceKeys) {
			return nil, errSnapshotChunks
		}
		wire.Sources.Values = append(wire.Sources.Values, chunk...)
	}
	for _, read := range []struct {
		count  int
		target *[]Symbol
	}{{spine.SymbolCount, &wire.Symbols}, {spine.FieldCount, &wire.Fields}} {
		if read.count == 0 {
			continue
		}
		*read.target = make([]Symbol, 0, read.count)
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
		table, err := decodeVocabularyChunks(decoder)
		if err != nil {
			return nil, err
		}
		wire.Vocabulary = table
	}
	if spine.SymbolTermsLength < 0 {
		return nil, errSnapshotChunks
	}
	if spine.SymbolTermsLength > 0 {
		data, err := decodeSnapshotBlob(decoder, spine.SymbolTermsLength)
		if err != nil {
			return nil, err
		}
		wire.symbolTerms = &symbolTermTable{}
		if err := wire.symbolTerms.UnmarshalBinary(data); err != nil {
			return nil, err
		}
	}
	return wire, nil
}

func decodeVocabularyChunks(decoder *gob.Decoder) (*TermTable, error) {
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
		data, err := decodeSnapshotBlob(decoder, length)
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
func decodeSnapshotBlob(decoder *gob.Decoder, length int) ([]byte, error) {
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
