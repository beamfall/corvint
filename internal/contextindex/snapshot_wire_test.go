package contextindex

import (
	"bytes"
	"context"
	"encoding/gob"
	"os"
	"reflect"
	"testing"
)

// TestSnapshotBytesAreDeterministic: builds of one tree, cold or reusing the
// prior snapshot, write byte-identical gob snapshot and pack files
// (IDX-SNAP-V0-106, V1-2290).
func TestSnapshotBytesAreDeterministic(t *testing.T) {
	t.Run("IDX-SNAP-V0-106", func(t *testing.T) {
		t.Setenv(snapshotFormatEnv, packFormatValue)
		root := taskContextFixture(t).Root
		var first SnapshotReceipt
		var gobBytes, packBytes []byte
		for build := range 4 {
			index, err := Build(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			receipt, err := WriteSnapshot(index)
			if err != nil {
				t.Fatal(err)
			}
			gobFile, err := os.ReadFile(receipt.Path)
			if err != nil {
				t.Fatal(err)
			}
			packFile, err := os.ReadFile(receipt.PackPath)
			if err != nil {
				t.Fatal(err)
			}
			if build == 0 {
				first, gobBytes, packBytes = receipt, gobFile, packFile
			} else if receipt.Path != first.Path || !bytes.Equal(gobFile, gobBytes) || !bytes.Equal(packFile, packBytes) {
				t.Fatalf("build %d wrote different bytes: gob equal=%v pack equal=%v", build, bytes.Equal(gobFile, gobBytes), bytes.Equal(packFile, packBytes))
			}
			// Odd builds start cold; even builds reuse the file just written.
			if build%2 == 0 {
				for _, path := range []string{receipt.Path, receipt.PackPath} {
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
	})
}

// TestSnapshotWireRoundTripsMaps: the key-sorted wire form encodes an index
// with nested record fields to the same bytes every time and decodes to the
// same maps, keeping nil apart from empty (IDX-SNAP-V0-106).
func TestSnapshotWireRoundTripsMaps(t *testing.T) {
	t.Run("IDX-SNAP-V0-106", func(t *testing.T) {
		fields := map[string]any{"title": "x", "tags": []any{"b", "a", true, nil}, "names": []string{"z", "y"},
			"nested": map[string]any{"k2": "v", "k1": []any{map[string]any{"deep": "d", "alpha": "a"}}}, "empty": map[string]any{}}
		index := &Index{
			ObjectFormat: "sha1", Revision: "tree", CommitRevision: "commit", Module: "m",
			Sources:   map[string]Source{"b.go": {Path: "b.go", Data: []byte("b")}, "a.go": {Path: "a.go", Data: []byte("a")}, "c.go": {Path: "c.go"}},
			Tracked:   map[string]struct{}{"a.go": {}, "b.go": {}, "c.go": {}, "d.bin": {}},
			Skipped:   map[string]struct{}{},
			Features:  map[string]Record{"F2": {ID: "F2", Fields: fields}, "F1": {ID: "F1", Fields: map[string]any{}}, "F3": {ID: "F3"}},
			Documents: map[string]Record{"D": {ID: "D", Fields: map[string]any{"a": "1", "b": "2", "c": "3"}}},
			Markers:   map[string][]Marker{"b.go": {{Path: "b.go", Line: 2}}, "a.go": {{Path: "a.go", Line: 1}}, "c.go": nil},
			Imports:   map[string]map[string]struct{}{"b.go": {"x": {}, "y": {}, "z": {}}, "a.go": {}},
		}
		var encoded []byte
		for attempt := range 8 {
			var buffer bytes.Buffer
			if err := gob.NewEncoder(&buffer).Encode(snapshotWireOf(index)); err != nil {
				t.Fatal(err)
			}
			if attempt == 0 {
				encoded = buffer.Bytes()
			} else if !bytes.Equal(buffer.Bytes(), encoded) {
				t.Fatalf("encoding %d differs from the first", attempt)
			}
		}
		var wire snapshotWire
		if err := gob.NewDecoder(bytes.NewReader(encoded)).Decode(&wire); err != nil {
			t.Fatal(err)
		}
		decoded, err := wire.index()
		if err != nil {
			t.Fatal(err)
		}
		// Gob decodes an empty inner marker slice as nil, as before.
		if !reflect.DeepEqual(decoded, index) {
			t.Fatalf("round trip differs\nwant %#v\n got %#v", index, decoded)
		}
		if decoded.Scenarios != nil || decoded.Skipped == nil || decoded.Features["F1"].Fields == nil || decoded.Features["F3"].Fields != nil {
			t.Fatal("round trip lost nil versus empty")
		}
		if _, err := (&snapshotWire{Sources: wirePairs[Source]{Present: true, Keys: []string{"a"}}}).index(); err == nil {
			t.Fatal("pairs with a missing value decoded")
		}
	})
}
