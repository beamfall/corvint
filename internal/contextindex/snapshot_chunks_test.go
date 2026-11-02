package contextindex

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/gob"
	"errors"
	"math"
	"os"
	"reflect"
	"testing"
)

// decodeSingleMessage is the format /2 body: the whole snapshotWire as one
// gob message, the reference the chunked body must equal.
func decodeSingleMessage(t *testing.T, wire *snapshotWire) *snapshotWire {
	t.Helper()
	var buffer bytes.Buffer
	if err := gob.NewEncoder(&buffer).Encode(wire); err != nil {
		t.Fatal(err)
	}
	decoded := &snapshotWire{}
	if err := gob.NewDecoder(&buffer).Decode(decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func decodeChunked(t *testing.T, wire *snapshotWire, depth snapshotBodyDepth) (*snapshotWire, int) {
	t.Helper()
	var buffer bytes.Buffer
	region, err := encodeSnapshotBody(gob.NewEncoder(&buffer), wire)
	if err != nil {
		t.Fatal(err)
	}
	decoder := gob.NewDecoder(&buffer)
	decoded, err := decodeSnapshotBody(decoder, depth, &snapshotBodies{region: bytes.Join(region, nil), objectFormat: wire.ObjectFormat})
	if err != nil {
		t.Fatal(err)
	}
	return decoded, buffer.Len()
}

// TestSnapshotChunkedBodyEqualsSingleMessage_IDXSNAPV0806 proves the format /3
// body decodes to exactly the snapshotWire, and so the Index, the /2 single
// message decodes to, at the production chunk bounds and at bounds small
// enough to split every source, symbol, field and term-table blob across many
// messages; the spine and event depths equal the matching subsets and leave
// the rest unread (IDX-SNAP-V0-806).
func TestSnapshotChunkedBodyEqualsSingleMessage_IDXSNAPV0806(t *testing.T) {
	fixture := taskContextFixture(t)
	indexes := map[string]*Index{"fixture": fixture, "empty": {}, "vocabulary only": {Vocabulary: &TermTable{}}}
	if root := os.Getenv("CORVINT_SNAPSHOT_PARITY_ROOT"); root != "" {
		built, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		indexes["repository"] = built
	}
	chunkBytes, symbolChunk := snapshotChunkBytes, snapshotSymbolChunk
	defer func() { snapshotChunkBytes, snapshotSymbolChunk = chunkBytes, symbolChunk }()
	for _, bounds := range [][2]int{{chunkBytes, symbolChunk}, {7, 3}, {1, 1}} {
		snapshotChunkBytes, snapshotSymbolChunk = bounds[0], bounds[1]
		for name, index := range indexes {
			wire := snapshotWireOf(index)
			want := decodeSingleMessage(t, wire)
			got, _ := decodeChunked(t, wire, snapshotBodyFull)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%s at bounds %v: chunked decode differs from the single message", name, bounds)
			}
			gotIndex, err := got.index()
			if err != nil {
				t.Fatal(err)
			}
			wantIndex, err := want.index()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(gotIndex, wantIndex) {
				t.Fatalf("%s at bounds %v: chunked index differs", name, bounds)
			}
			events, _ := decodeChunked(t, wire, snapshotBodyEvents)
			if !reflect.DeepEqual(events.eventSnapshot(), want.eventSnapshot()) || events.Vocabulary != nil || !reflect.DeepEqual(events.Tracked, want.Tracked) {
				t.Fatalf("%s at bounds %v: event decode differs", name, bounds)
			}
			spine, _ := decodeChunked(t, wire, snapshotBodySpine)
			if spine.ProfileID != want.ProfileID || spine.Sources.Values != nil || spine.Symbols != nil {
				t.Fatalf("%s at bounds %v: spine decode read past the spine", name, bounds)
			}
		}
	}
}

// TestSnapshotChunkedBodyRefusesMalformedCounts_IDXSNAPV0806: a spine whose
// counts disagree with the chunks that follow is an error (the miss), never a
// short or padded index.
func TestSnapshotChunkedBodyRefusesMalformedCounts_IDXSNAPV0806(t *testing.T) {
	wire := snapshotWireOf(taskContextFixture(t))
	for name, mutate := range map[string]func(*snapshotSpine){
		"extra source key":  func(spine *snapshotSpine) { spine.SourceKeys = append(spine.SourceKeys, "zz") },
		"missing symbol":    func(spine *snapshotSpine) { spine.SymbolCount++ },
		"negative fields":   func(spine *snapshotSpine) { spine.FieldCount = -1 },
		"keys without flag": func(spine *snapshotSpine) { spine.SourcesPresent = false },
	} {
		spine := snapshotSpineOf(wire)
		mutate(&spine)
		var forged bytes.Buffer
		encoder := gob.NewEncoder(&forged)
		if err := encoder.Encode(&spine); err != nil {
			t.Fatal(err)
		}
		region, err := encodeSnapshotChunks(encoder, wire)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := decodeSnapshotBody(gob.NewDecoder(&forged), snapshotBodyFull, &snapshotBodies{region: bytes.Join(region, nil), objectFormat: wire.ObjectFormat}); err == nil {
			t.Fatalf("%s: a spine that disagrees with its chunks decoded", name)
		}
	}
}

// TestSnapshotFormatTwoFileIsAMiss_IDXSNAPV0806: a format /2 file (one gob
// message, valid trailer) at the store path is the header-mismatch miss for
// every reader, so the caller rebuilds; it is never decoded as /3.
func TestSnapshotFormatTwoFileIsAMiss_IDXSNAPV0806(t *testing.T) {
	index := taskContextFixture(t)
	receipt, err := WriteSnapshot(index)
	if err != nil {
		t.Fatal(err)
	}
	if _, hit, err := LoadSnapshot(context.Background(), index.Root); err != nil || !hit {
		t.Fatalf("the /3 snapshot did not load: hit=%v err=%v", hit, err)
	}
	// /2 is snapshotFormat's single message; a /3 header (IDX-SNAP-V0-811)
	// over the same message is the same header-mismatch miss.
	for _, format := range []string{snapshotFormat, "corvint-index-snapshot/3"} {
		var payload bytes.Buffer
		encoder := gob.NewEncoder(&payload)
		if err := encoder.Encode(snapshotHeader{Format: format, ObjectFormat: index.ObjectFormat, Tree: index.Revision, Engine: snapshotEngineID()}); err != nil {
			t.Fatal(err)
		}
		if err := encoder.Encode(snapshotWireOf(index)); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(payload.Bytes())
		if err := os.WriteFile(receipt.Path, append(payload.Bytes(), digest[:]...), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, hit, err := LoadSnapshot(context.Background(), index.Root); err != nil || hit {
			t.Fatalf("a %s snapshot loaded: hit=%v err=%v", format, hit, err)
		}
		for _, compact := range []bool{false, true} {
			if _, hit, err := LoadEventSnapshot(context.Background(), index.Root, compact); err != nil || hit {
				t.Fatalf("a %s snapshot event-loaded compact=%v: hit=%v err=%v", format, compact, hit, err)
			}
		}
	}
}

// TestSnapshotChunkedBodyRefusesOversizedCounts_IDXSNAPV0812: a spine whose
// symbol or field count, or a table or term-table blob length, exceeds the
// message bytes is refused before anything is allocated for it: an error,
// never a makeslice panic the fault guard would rethrow (IDX-SNAP-V0-812).
func TestSnapshotChunkedBodyRefusesOversizedCounts_IDXSNAPV0812(t *testing.T) {
	wire := snapshotWireOf(taskContextFixture(t))
	wire.symbolTerms = buildSymbolTermTable(taskContextFixture(t))
	for name, mutate := range map[string]func(*snapshotSpine, int){
		"max symbols":           func(spine *snapshotSpine, _ int) { spine.SymbolCount = math.MaxInt },
		"symbols past messages": func(spine *snapshotSpine, size int) { spine.SymbolCount = 2*size + 1 },
		"max fields":            func(spine *snapshotSpine, _ int) { spine.FieldCount = math.MaxInt },
		"fields past messages":  func(spine *snapshotSpine, size int) { spine.FieldCount = 2*size + 1 },
		"max symbol terms":      func(spine *snapshotSpine, _ int) { spine.SymbolTermsLength = math.MaxInt },
		"max test links":        func(spine *snapshotSpine, size int) { spine.TestLinksLength = 2*size + 1 },
	} {
		var plain bytes.Buffer
		if _, err := encodeSnapshotBody(gob.NewEncoder(&plain), wire); err != nil {
			t.Fatal(err)
		}
		spine := snapshotSpineOf(wire)
		mutate(&spine, plain.Len())
		var forged bytes.Buffer
		encoder := gob.NewEncoder(&forged)
		if err := encoder.Encode(&spine); err != nil {
			t.Fatal(err)
		}
		region, err := encodeSnapshotChunks(encoder, wire)
		if err != nil {
			t.Fatal(err)
		}
		bodies := &snapshotBodies{region: bytes.Join(region, nil), objectFormat: wire.ObjectFormat, messageBytes: int64(forged.Len())}
		var decodeErr error
		panicked := guardMappedRead(func() error {
			_, decodeErr = decodeSnapshotBody(gob.NewDecoder(&forged), snapshotBodyFull, bodies)
			return nil
		})
		if panicked != nil || !errors.Is(decodeErr, errSnapshotChunks) {
			t.Fatalf("%s: decode err=%v guard=%v, want the malformed-chunks refusal", name, decodeErr, panicked)
		}
	}
	if _, err := decodeSnapshotBlob(nil, math.MaxInt, 1<<20); !errors.Is(err, errSnapshotChunks) {
		t.Fatalf("an oversized term-table blob length was not refused: %v", err)
	}
}
