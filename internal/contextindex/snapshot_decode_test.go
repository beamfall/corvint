//go:build unix

package contextindex

import (
	"context"
	"encoding/gob"
	"errors"
	"os"
	"reflect"
	"syscall"
	"testing"
)

// TestMappedSnapshotDecodeMatchesStreamedDecode pins IDX-SNAP-V0-105
// (V1-2237): the mapped decode returns the value the read(2) decode returns
// from the same file, nothing it returns aliases the released mapping, and a
// changed trailer is the same digest-mismatch miss.
func TestMappedSnapshotDecodeMatchesStreamedDecode(t *testing.T) {
	index := taskContextFixture(t)
	receipt, err := WriteSnapshot(index)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := readIdentity(context.Background(), index.Root)
	if err != nil {
		t.Fatal(err)
	}
	engineID := snapshotEngineID()
	file, err := os.Open(receipt.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []snapshotBodyDepth{snapshotBodySpine, snapshotBodyEvents, snapshotBodyFull} {
		var mapped, streamed *snapshotWire
		mappedBodies := &snapshotBodies{objectFormat: identity.objectFormat}
		streamedBodies := &snapshotBodies{objectFormat: identity.objectFormat}
		into := func(target **snapshotWire, bodies *snapshotBodies) func(*gob.Decoder) error {
			return func(decoder *gob.Decoder) (err error) {
				*target, err = decodeSnapshotBody(decoder, depth, bodies)
				return err
			}
		}
		mapping := mapSnapshot(file, info.Size())
		if mapping == nil {
			t.Fatal("snapshot file did not map")
		}
		if err := decodeMappedSnapshotValue(mapping, identity, engineID, mappedBodies, into(&mapped, mappedBodies)); err != nil {
			t.Fatalf("mapped decode: %v", err)
		}
		if err := decodeStreamedSnapshotValue(file, info.Size(), identity, engineID, streamedBodies, into(&streamed, streamedBodies)); err != nil {
			t.Fatalf("streamed decode: %v", err)
		}
		// The mapping is released; comparing reads every decoded string.
		if !reflect.DeepEqual(mapped, streamed) {
			t.Fatalf("mapped decode of %T differs from the streamed decode", mapped)
		}
	}

	data, err := os.ReadFile(receipt.Path)
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)-1] ^= 0xff
	if err := os.WriteFile(receipt.Path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := os.Open(receipt.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer changed.Close()
	if _, err := decodeSnapshotWire(changed, identity, engineID, snapshotBodyFull); err == nil || err.Error() != "snapshot digest mismatch" {
		t.Fatalf("changed trailer decoded: %v", err)
	}
}

// TestMappedSnapshotDecodeFaultIsAMiss pins IDX-SNAP-V0-105: a memory fault
// while reading the mapping, which a file truncated under it raises on hosts
// that do not keep its pages, is an error and so the IDX-SNAP-V0-003 miss,
// on the decode and the hash goroutine alike, never a crash. A PROT_NONE
// mapping faults on every host; any other panic is re-raised.
func TestMappedSnapshotDecodeFaultIsAMiss(t *testing.T) {
	mapping, err := syscall.Mmap(-1, 0, 1<<16, syscall.PROT_NONE, syscall.MAP_ANON|syscall.MAP_PRIVATE)
	if err != nil {
		t.Fatal(err)
	}
	identity := repositoryIdentity{objectFormat: "sha1"}
	faulting := &snapshotMapping{bytes: mapping}
	faulting.refs.Store(1)
	liveSnapshotMappings.Add(1)
	bodies := &snapshotBodies{objectFormat: identity.objectFormat}
	if err := decodeMappedSnapshotValue(faulting, identity, snapshotEngineID(), bodies, func(decoder *gob.Decoder) error {
		_, err := decodeSnapshotBody(decoder, snapshotBodyFull, bodies)
		return err
	}); !errors.Is(err, errSnapshotFault) {
		t.Fatalf("faulting mapping decoded: %v", err)
	}
	defer func() {
		if recovered := recover(); recovered != "not a fault" {
			t.Fatalf("non-fault panic became %v", recovered)
		}
	}()
	_ = guardMappedRead(func() error { panic("not a fault") })
}
