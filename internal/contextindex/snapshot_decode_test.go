//go:build unix

package contextindex

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"reflect"
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
	for _, fresh := range []func() any{func() any { return &Index{} }, func() any { return &eventSnapshot{} }} {
		mapped, streamed := fresh(), fresh()
		mapping := mapReadOnly(file, info.Size())
		if mapping == nil {
			t.Fatal("snapshot file did not map")
		}
		if err := decodeMappedSnapshotValue(mapping, identity, engineID, mapped); err != nil {
			t.Fatalf("mapped decode: %v", err)
		}
		if err := decodeStreamedSnapshotValue(file, info.Size()-sha256.Size, identity, engineID, streamed); err != nil {
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
	if err := decodeSnapshotValue(changed, identity, engineID, &Index{}); err == nil || err.Error() != "snapshot digest mismatch" {
		t.Fatalf("changed trailer decoded: %v", err)
	}
}

// TestMappedSnapshotDecodeFaultIsAMiss pins IDX-SNAP-V0-105: a snapshot
// truncated under its mapping is an error, the IDX-SNAP-V0-003 miss, rather
// than a crash on either the decode or the hash goroutine.
func TestMappedSnapshotDecodeFaultIsAMiss(t *testing.T) {
	index := taskContextFixture(t)
	receipt, err := WriteSnapshot(index)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := readIdentity(context.Background(), index.Root)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(receipt.Path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	mapping := mapReadOnly(file, info.Size())
	if mapping == nil {
		t.Fatal("snapshot file did not map")
	}
	if err := file.Truncate(0); err != nil {
		t.Fatal(err)
	}
	if err := decodeMappedSnapshotValue(mapping, identity, snapshotEngineID(), &Index{}); !errors.Is(err, errSnapshotFault) {
		t.Fatalf("truncated mapping decoded: %v", err)
	}
}
