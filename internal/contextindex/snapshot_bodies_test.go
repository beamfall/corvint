package contextindex

import (
	"bytes"
	"context"
	"encoding/gob"
	"reflect"
	"testing"
)

// writtenIndexes writes each test index's snapshot into a scratch store
// (IDX-SNAP-V0-808), so a repository run never touches the shared store.
func writtenIndexes(t *testing.T) map[string]*Index {
	t.Helper()
	t.Setenv("CORVINT_"+indexStoreSuffix, t.TempDir())
	indexes := symbolTermIndexes(t)
	for name, index := range indexes {
		if _, err := WriteSnapshot(index); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	return indexes
}

// TestDeferredGobBodiesEqualFullLoad_IDXSNAPV0811 proves a deferred /4 load
// answers like a full one: it hands out no body bytes until read, every
// source's Text equals the full load's, the task-context packets are
// identical for the shared queries, and no refusal is recorded
// (IDX-SNAP-V0-811).
func TestDeferredGobBodiesEqualFullLoad_IDXSNAPV0811(t *testing.T) {
	for name, index := range writtenIndexes(t) {
		full, hit, err := LoadSnapshot(context.Background(), index.Root)
		if err != nil || !hit {
			t.Fatalf("%s: full load hit=%v err=%v", name, hit, err)
		}
		deferred, hit, err := LoadSnapshotDeferred(context.Background(), index.Root)
		if err != nil || !hit {
			t.Fatalf("%s: deferred load hit=%v err=%v", name, hit, err)
		}
		lazy := 0
		for path, source := range deferred.Sources {
			if source.body.gob != nil {
				lazy++
				if source.Data != nil {
					t.Fatalf("%s: %s carries bytes before a read", name, path)
				}
			}
		}
		if lazy == 0 {
			t.Fatalf("%s: the deferred load handed out no lazy body", name)
		}
		for path, source := range full.Sources {
			fullText, fullValid, fullLoaded := source.Text()
			text, valid, loaded := deferred.Sources[path].Text()
			if text != fullText || valid != fullValid || loaded != fullLoaded {
				t.Fatalf("%s: %s reads differently through its lazy body", name, path)
			}
		}
		for _, text := range symbolTermQueries {
			want, wantErr := TaskContext(context.Background(), full, text, "", 20)
			got, gotErr := TaskContext(context.Background(), deferred, text, "", 20)
			if (wantErr == nil) != (gotErr == nil) || wantErr == nil && mustJSON(t, got) != mustJSON(t, want) {
				t.Fatalf("%s %q: deferred and full packets differ", name, text)
			}
		}
		if err := deferred.SnapshotRefusal(); err != nil {
			t.Fatalf("%s: refusal on an intact file: %v", name, err)
		}
		deferred.Release()
	}
}

// TestGobBodiesInlineAndRegionRoundTrip_IDXSNAPV0811: a body that does not
// rehash to its blob id stays inline and decodes to the same Source; a
// region body whose bytes changed fails its blob id on a full decode.
func TestGobBodiesInlineAndRegionRoundTrip_IDXSNAPV0811(t *testing.T) {
	wire := snapshotWireOf(taskContextFixture(t))
	values := append([]Source(nil), wire.Sources.Values...)
	values[0].BlobHash = "not-its-blob"
	wire.Sources.Values = values
	metas, region := gobSourceMetas(values, wire.ObjectFormat)
	if metas[0].InRegion || metas[0].Inline == nil || len(region) == 0 {
		t.Fatal("the mismatched body was not kept inline")
	}
	want := decodeSingleMessage(t, wire)
	got, _ := decodeChunked(t, wire, snapshotBodyFull)
	if !reflect.DeepEqual(got.Sources, want.Sources) {
		t.Fatal("inline and region bodies decode differently from the single message")
	}
	var buffer bytes.Buffer
	parts, err := encodeSnapshotBody(gob.NewEncoder(&buffer), wire)
	if err != nil {
		t.Fatal(err)
	}
	joined := bytes.Join(parts, nil)
	joined[0] ^= 0xff
	if _, err := decodeSnapshotBody(gob.NewDecoder(&buffer), snapshotBodyFull, &snapshotBodies{region: joined, objectFormat: wire.ObjectFormat}); err == nil {
		t.Fatal("a changed region body decoded")
	}
}
