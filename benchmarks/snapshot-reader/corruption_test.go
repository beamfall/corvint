package main

import (
	"bytes"
	"encoding/gob"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// TestPackCorruptionRefused mutates one byte inside a single checksummed
// block's trailing string content -- not its leading gob type/length
// framing -- so the mutated bytes still gob-decode cleanly (to a different
// value). This falsifies checksum removal specifically: if the checksum
// check were deleted, this corruption would decode without error and the
// test would catch that, unlike a corruption that also breaks gob's wire
// format (which would fail on decode alone, checksum or not).
func TestPackCorruptionRefused(t *testing.T) {
	dir := t.TempDir()
	corpus := GenerateCorpus(50, 7)
	if err := BuildPack(dir, corpus); err != nil {
		t.Fatalf("build pack: %v", err)
	}

	r, err := OpenPack(dir)
	if err != nil {
		t.Fatalf("open pack: %v", err)
	}
	sortedPaths := append([]string(nil), corpus.Paths...)
	sort.Strings(sortedPaths)
	target := sortedPaths[0]
	blk := r.manifest.Kinds[kindImports][0]
	dataFile := dataPath(dir, r.manifest.DataFile)
	r.Close()

	data, err := os.ReadFile(dataFile)
	if err != nil {
		t.Fatalf("read data file: %v", err)
	}
	// Flip a low bit in the last byte of the block: the last import
	// string's trailing character, not the block's leading type
	// descriptor or length varint.
	corruptOffset := blk.Offset + blk.Length - 1
	data[corruptOffset] ^= 0x01
	if err := os.WriteFile(dataFile, data, 0o644); err != nil {
		t.Fatalf("rewrite data file: %v", err)
	}

	// Confirm the mutated block is still valid gob for []string, proving
	// this corruption is the kind only a checksum check can catch.
	mutated := data[blk.Offset : blk.Offset+blk.Length]
	var decoded []string
	if err := gob.NewDecoder(bytes.NewReader(mutated)).Decode(&decoded); err != nil {
		t.Fatalf("mutated block must still gob-decode (test setup invalid): %v", err)
	}

	r2, err := OpenPack(dir)
	if err != nil {
		t.Fatalf("reopen pack: %v", err)
	}
	defer r2.Close()
	_, err = r2.ReadOnePath(target)
	if err == nil {
		t.Fatal("expected checksum failure reading a corrupted block, got nil error")
	}
	if !errors.Is(err, errChecksumMismatch) {
		t.Fatalf("expected errChecksumMismatch, got %v", err)
	}
}

// TestPackTruncationRefused truncates data.bin and asserts a whole read
// fails rather than silently returning a partial corpus.
func TestPackTruncationRefused(t *testing.T) {
	dir := t.TempDir()
	corpus := GenerateCorpus(50, 8)
	if err := BuildPack(dir, corpus); err != nil {
		t.Fatalf("build pack: %v", err)
	}

	r0, err := OpenPack(dir)
	if err != nil {
		t.Fatalf("open pack to learn data file name: %v", err)
	}
	dataFile := dataPath(dir, r0.manifest.DataFile)
	r0.Close()

	info, err := os.Stat(dataFile)
	if err != nil {
		t.Fatalf("stat data file: %v", err)
	}
	if err := os.Truncate(dataFile, info.Size()/2); err != nil {
		t.Fatalf("truncate data file: %v", err)
	}

	r, err := OpenPack(dir)
	if err != nil {
		t.Fatalf("reopen pack: %v", err)
	}
	defer r.Close()
	if _, err := r.ReadWhole(); err == nil {
		t.Fatal("expected a read failure against a truncated data file, got nil error")
	}
}

// TestPackManifestLastAtomicWrite asserts DNIP-IDX-008's publication order:
// a missing manifest means "no snapshot", never a partial read of whatever
// data blocks happen to already be on disk, and BuildPack's temp manifest
// is renamed away rather than left behind.
func TestPackManifestLastAtomicWrite(t *testing.T) {
	dir := t.TempDir()

	// Simulate a crash mid-publish: sections written, no manifest yet.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data-inflight.bin"), []byte("partial-data-not-yet-published"), 0o644); err != nil {
		t.Fatalf("write partial section file: %v", err)
	}
	if _, err := OpenPack(dir); !errors.Is(err, errNoSnapshot) {
		t.Fatalf("expected errNoSnapshot before manifest publish, got %v", err)
	}

	corpus := GenerateCorpus(20, 9)
	if err := BuildPack(dir, corpus); err != nil {
		t.Fatalf("build pack: %v", err)
	}
	if _, err := os.Stat(manifestPath(dir) + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("expected temp manifest to be renamed away, stat err: %v", err)
	}

	r, err := OpenPack(dir)
	if err != nil {
		t.Fatalf("open pack after publish: %v", err)
	}
	defer r.Close()
	if _, err := r.ReadWhole(); err != nil {
		t.Fatalf("read whole after publish: %v", err)
	}
}

// TestPackRepublishFailureKeepsOldSnapshotReadable asserts the other half
// of DNIP-IDX-008: a failed re-publish must not disturb an already-published
// snapshot. BuildPack writes each publish's sections to a freshly named file
// (os.CreateTemp, never a reused fixed name -- see pack.go), so a crash
// between writing new sections and renaming the new manifest into place
// leaves the old manifest resolving the old, untouched section file.
func TestPackRepublishFailureKeepsOldSnapshotReadable(t *testing.T) {
	dir := t.TempDir()
	original := GenerateCorpus(30, 11)
	if err := BuildPack(dir, original); err != nil {
		t.Fatalf("build original pack: %v", err)
	}

	r, err := OpenPack(dir)
	if err != nil {
		t.Fatalf("open original pack: %v", err)
	}
	originalDataFile := r.manifest.DataFile
	r.Close()

	// Simulate a crash partway through a re-publish: new sections are
	// written under their own new name, and the new manifest is written to
	// manifest.gob.tmp, but the process dies before the rename that would
	// publish it.
	if err := os.WriteFile(filepath.Join(dir, "data-republish-attempt.bin"), []byte("new-sections-not-yet-published"), 0o644); err != nil {
		t.Fatalf("write in-flight republish sections: %v", err)
	}
	if err := os.WriteFile(manifestPath(dir)+".tmp", []byte("new-manifest-not-yet-renamed"), 0o644); err != nil {
		t.Fatalf("write in-flight republish manifest: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, originalDataFile)); err != nil {
		t.Fatalf("original data file missing or truncated after failed republish: %v", err)
	}

	r2, err := OpenPack(dir)
	if err != nil {
		t.Fatalf("open pack after failed republish: %v", err)
	}
	defer r2.Close()
	got, err := r2.ReadWhole()
	if err != nil {
		t.Fatalf("read whole after failed republish: %v", err)
	}
	if !bytes.Equal(corpusDigest(got), corpusDigest(original)) {
		t.Fatal("pack contents changed after a failed republish")
	}
}

// v2Fixture builds a pack v2 of n files and returns its reader's manifest.
func v2Fixture(t *testing.T, n int, seed int64) (string, *Corpus, packV2Manifest) {
	t.Helper()
	dir := t.TempDir()
	corpus := GenerateCorpus(n, seed)
	if err := BuildPackV2(dir, corpus); err != nil {
		t.Fatalf("build pack v2: %v", err)
	}
	r, err := OpenPackV2(dir)
	if err != nil {
		t.Fatalf("open pack v2: %v", err)
	}
	defer r.Close()
	return dir, corpus, r.manifest
}

// TestPackV2CorruptionRefused is TestPackCorruptionRefused for pack v2: a
// one-byte change inside a block that still decodes (with its section
// descriptor replayed) must fail with errChecksumMismatch.
func TestPackV2CorruptionRefused(t *testing.T) {
	dir, corpus, m := v2Fixture(t, 50, 7)
	sortedPaths := append([]string(nil), corpus.Paths...)
	sort.Strings(sortedPaths)
	target := sortedPaths[0]

	r, err := OpenPackV2(dir)
	if err != nil {
		t.Fatalf("open pack v2: %v", err)
	}
	row, ok, err := r.lookup(target)
	if err != nil || !ok {
		t.Fatalf("lookup %q: ok=%v err=%v", target, ok, err)
	}
	blk := getV2Locator(row[m.KeyWidth+v2SecImports*v2LocatorWidth:])
	descriptor := r.descriptors[v2SecImports]
	r.Close()

	dataFile := dataPath(dir, m.DataFile)
	data, err := os.ReadFile(dataFile)
	if err != nil {
		t.Fatalf("read data file: %v", err)
	}
	data[blk.Offset+uint64(blk.Length)-1] ^= 0x01
	if err := os.WriteFile(dataFile, data, 0o644); err != nil {
		t.Fatalf("rewrite data file: %v", err)
	}
	var decoded []string
	if err := decodeV2Block(descriptor, data[blk.Offset:blk.Offset+uint64(blk.Length)], &decoded); err != nil {
		t.Fatalf("mutated block must still gob-decode (test setup invalid): %v", err)
	}

	r2, err := OpenPackV2(dir)
	if err != nil {
		t.Fatalf("reopen pack v2: %v", err)
	}
	defer r2.Close()
	if _, err := r2.ReadOnePath(target); !errors.Is(err, errChecksumMismatch) {
		t.Fatalf("expected errChecksumMismatch, got %v", err)
	}
}

// TestPackV2TruncationRefused truncates the data file to half its length;
// opening or reading the whole pack must fail, never return a partial corpus.
func TestPackV2TruncationRefused(t *testing.T) {
	dir, _, m := v2Fixture(t, 50, 8)
	dataFile := dataPath(dir, m.DataFile)
	info, err := os.Stat(dataFile)
	if err != nil {
		t.Fatalf("stat data file: %v", err)
	}
	if err := os.Truncate(dataFile, info.Size()/2); err != nil {
		t.Fatalf("truncate data file: %v", err)
	}
	r, err := OpenPackV2(dir)
	if err == nil {
		_, err = r.ReadWhole()
		r.Close()
	}
	if err == nil {
		t.Fatal("expected a failure against a truncated data file, got nil error")
	}
}

// TestPackV2ManifestLastAtomicWrite is TestPackManifestLastAtomicWrite for
// pack v2: no manifest means errNoSnapshot, and the temp manifest is
// renamed away by a completed publish.
func TestPackV2ManifestLastAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "data-inflight.bin"), []byte("partial-data-not-yet-published"), 0o644); err != nil {
		t.Fatalf("write partial section file: %v", err)
	}
	if _, err := OpenPackV2(dir); !errors.Is(err, errNoSnapshot) {
		t.Fatalf("expected errNoSnapshot before manifest publish, got %v", err)
	}
	if err := BuildPackV2(dir, GenerateCorpus(20, 9)); err != nil {
		t.Fatalf("build pack v2: %v", err)
	}
	if _, err := os.Stat(manifestV2Path(dir) + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("expected temp manifest to be renamed away, stat err: %v", err)
	}
	r, err := OpenPackV2(dir)
	if err != nil {
		t.Fatalf("open pack v2 after publish: %v", err)
	}
	defer r.Close()
	if _, err := r.ReadWhole(); err != nil {
		t.Fatalf("read whole after publish: %v", err)
	}
}

// TestPackV2RepublishFailureKeepsOldSnapshotReadable is the pack v2 twin of
// TestPackRepublishFailureKeepsOldSnapshotReadable.
func TestPackV2RepublishFailureKeepsOldSnapshotReadable(t *testing.T) {
	dir, original, _ := v2Fixture(t, 30, 11)
	if err := os.WriteFile(filepath.Join(dir, "data-republish-attempt.bin"), []byte("new-sections-not-yet-published"), 0o644); err != nil {
		t.Fatalf("write in-flight republish sections: %v", err)
	}
	if err := os.WriteFile(manifestV2Path(dir)+".tmp", []byte("new-manifest-not-yet-renamed"), 0o644); err != nil {
		t.Fatalf("write in-flight republish manifest: %v", err)
	}
	r, err := OpenPackV2(dir)
	if err != nil {
		t.Fatalf("open pack v2 after failed republish: %v", err)
	}
	defer r.Close()
	got, err := r.ReadWhole()
	if err != nil {
		t.Fatalf("read whole after failed republish: %v", err)
	}
	if !bytes.Equal(corpusDigest(got), corpusDigest(original)) {
		t.Fatal("pack v2 contents changed after a failed republish")
	}
}

// TestPackV2ManifestCorruptionRefused flips one byte of the manifest body:
// its trailing checksum must refuse it before any locator is trusted.
func TestPackV2ManifestCorruptionRefused(t *testing.T) {
	dir, _, _ := v2Fixture(t, 20, 12)
	b, err := os.ReadFile(manifestV2Path(dir))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	b[len(v2Magic)+v2DataNameWidth+1] ^= 0x01 // low byte of the key width
	if err := os.WriteFile(manifestV2Path(dir), b, 0o644); err != nil {
		t.Fatalf("rewrite manifest: %v", err)
	}
	if _, err := OpenPackV2(dir); !errors.Is(err, errChecksumMismatch) {
		t.Fatalf("expected errChecksumMismatch, got %v", err)
	}
}

// TestPackV2OversizedLengthRefusedBeforeAllocation rewrites a manifest that
// still passes its own checksum but claims a vocabulary block larger than
// v2MaxBlockBytes: the read must fail with errPackV2Malformed, not allocate.
func TestPackV2OversizedLengthRefusedBeforeAllocation(t *testing.T) {
	dir, _, m := v2Fixture(t, 20, 13)
	m.Vocabulary.Length = v2MaxBlockBytes + 1
	if err := os.WriteFile(manifestV2Path(dir), m.encode(), 0o644); err != nil {
		t.Fatalf("rewrite manifest: %v", err)
	}
	r, err := OpenPackV2(dir)
	if err != nil {
		t.Fatalf("open pack v2: %v", err)
	}
	defer r.Close()
	if _, err := r.ReadWhole(); !errors.Is(err, errPackV2Malformed) {
		t.Fatalf("expected errPackV2Malformed, got %v", err)
	}
}
