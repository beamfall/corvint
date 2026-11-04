//go:build unix

package contextindex

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"runtime"
	"sync/atomic"
	"syscall"
	"testing"
)

// faultingMapping maps path shared and read-only, then truncates the file to
// zero, so every later read of the returned bytes faults (SIGBUS) on Linux
// and Darwin alike. The readers map private; a private mapping of a truncated
// file faults on Linux but on Darwin keeps reading the cached pages, so the
// tests substitute this mapping to reach the fault on every unix host
// (V1-3005).
func faultingMapping(t *testing.T, path string) []byte {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	mapping, err := syscall.Mmap(int(file.Fd()), 0, int(info.Size()), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		t.Skipf("shared mapping unavailable: %v", err)
	}
	t.Cleanup(func() { _ = syscall.Munmap(mapping) })
	if err := os.Truncate(path, 0); err != nil {
		t.Fatal(err)
	}
	return mapping
}

// V1-3005: a pack truncated under its mapping is a refused read on the
// decoding goroutine, in the verifyAll workers, and on a deferred body read;
// the process does not die of the fault.
func TestPackMappedReadsRefuseATruncatedFile(t *testing.T) {
	index, receipt := packFixture(t)
	identity := fixtureIdentity(index)

	file, err := openPackFile(receipt.PackPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.release()
	defer file.close()
	header, err := file.header()
	if err != nil {
		t.Fatal(err)
	}
	file.mapping = faultingMapping(t, receipt.PackPath)

	if _, err := file.identity(); !errors.Is(err, errSnapshotFault) {
		t.Fatalf("identity over a truncated mapping = %v, want errSnapshotFault", err)
	}
	var sections []*packSection
	for _, name := range packSectionsByLoad[loadFull] {
		entry, err := header.entry(name)
		if err != nil {
			t.Fatal(err)
		}
		sections = append(sections, &packSection{
			name: name, data: file.mapping[entry.Offset : entry.Offset+entry.Length],
			digests: make([]byte, packBlockCount(entry.Length)*32), verified: make([]atomic.Bool, packBlockCount(entry.Length)),
			refused: &file.refused,
		})
	}
	if err := verifyAll(sections); !errors.Is(err, errSnapshotFault) {
		t.Fatalf("verifyAll over a truncated mapping = %v, want errSnapshotFault", err)
	}
	body := packBody{section: sections[0], length: uint64(len(sections[0].data))}
	sections[0].verified = make([]atomic.Bool, len(sections[0].verified))
	if _, _, loaded := (Source{body: body}).Text(); loaded {
		t.Fatal("a deferred body read over a truncated mapping reported loaded")
	}
	refusal := (&Index{bodies: sections[0]}).SnapshotRefusal()
	if !errors.Is(refusal, ErrSnapshotRefused) || !errors.Is(refusal, errSnapshotFault) {
		t.Fatalf("SnapshotRefusal = %v, want ErrSnapshotRefused wrapping errSnapshotFault", refusal)
	}
	file.refused.Store(false)
	if _, err := decodePackSnapshot(file, identity, analyzerEngine(), loadFull); !errors.Is(err, errSnapshotFault) {
		t.Fatalf("decode of a truncated mapping = %v, want errSnapshotFault", err)
	}
	if !file.refused.Load() {
		t.Fatal("a faulted pack is not marked refused")
	}
}

// TestDeferredGobBodyTruncatedInPlaceIsRefused_IDXSNAPV0811: a snapshot file
// truncated in place under a deferred load's mapping makes the next body read
// a refusal (the fault is caught inside the guard), never a SIGBUS, and a
// fresh load misses. Lazy bodies exist only on a mapped load. One load reads
// its region through faultingMapping, so the read faults on Darwin as on Linux
// (V1-3005); another reads through its own private mapping, which refuses on
// Linux and on Darwin may keep serving the cached pages, so every body it
// serves must still equal the full load's (V1-4413, V1-4553).
func TestDeferredGobBodyTruncatedInPlaceIsRefused_IDXSNAPV0811(t *testing.T) {
	t.Setenv("CORVINT_"+indexStoreSuffix, t.TempDir())
	index := taskContextFixture(t)
	receipt, err := WriteSnapshot(index)
	if err != nil {
		t.Fatal(err)
	}
	full, hit, err := LoadSnapshot(context.Background(), index.Root)
	if err != nil || !hit {
		t.Fatalf("full load hit=%v err=%v", hit, err)
	}
	t.Cleanup(full.Release)
	faulted, private := deferredLoad(t, index.Root), deferredLoad(t, index.Root)
	data, err := os.ReadFile(receipt.Path)
	if err != nil {
		t.Fatal(err)
	}
	footerAt := len(data) - 32 - snapshotFooterBytes
	regionStart := footerAt - int(binary.LittleEndian.Uint64(data[footerAt:]))
	lazy := lazyBodies(t, faulted)
	if !bytes.Equal(lazy.region, data[regionStart:footerAt]) {
		t.Fatal("the lazy region is not the file's body region")
	}
	lazy.region = faultingMapping(t, receipt.Path)[regionStart:footerAt]
	loaded := 0
	for _, source := range faulted.Sources {
		if _, _, ok := source.Text(); ok {
			loaded++
		}
	}
	if !errors.Is(faulted.SnapshotRefusal(), ErrSnapshotRefused) {
		t.Fatalf("the truncated body read was not refused (%d loaded)", loaded)
	}
	loaded = 0
	for path, source := range private.Sources {
		text, valid, ok := source.Text()
		if !ok {
			continue
		}
		loaded++
		if wantText, wantValid, _ := full.Sources[path].Text(); text != wantText || valid != wantValid {
			t.Fatalf("%s: a body read after the truncate is not the full load's", path)
		}
	}
	refusal := private.SnapshotRefusal()
	if runtime.GOOS == "linux" && !errors.Is(refusal, ErrSnapshotRefused) {
		t.Fatalf("the truncated private-mapping read was not refused on linux (%d loaded)", loaded)
	}
	t.Logf("private mapping after the truncate on %s: %d verified bodies served, refusal %v", runtime.GOOS, loaded, refusal)
	if _, hit, err := LoadSnapshot(context.Background(), index.Root); err != nil || hit {
		t.Fatalf("a truncated snapshot loaded: hit=%v err=%v", hit, err)
	}
}

// deferredLoad is a deferred load of root, released at cleanup.
func deferredLoad(t *testing.T, root string) *Index {
	t.Helper()
	deferred, hit, err := LoadSnapshotDeferred(context.Background(), root)
	if err != nil || !hit {
		t.Fatalf("deferred load hit=%v err=%v", hit, err)
	}
	t.Cleanup(deferred.Release)
	return deferred
}

// lazyBodies is the shared lazy body store of a deferred load.
func lazyBodies(t *testing.T, deferred *Index) *gobBodies {
	t.Helper()
	for _, source := range deferred.Sources {
		if source.body.gob != nil {
			return source.body.gob
		}
	}
	t.Fatal("the deferred load handed out no lazy body")
	return nil
}

func TestSectionedMappedReadRefusesATruncatedFile(t *testing.T) {
	index, receipt := sectionedFixture(t)
	identity := fixtureIdentity(index)
	file, err := openSectionedFile(receipt.SectionedPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.release()
	defer file.close()
	header, err := file.header()
	if err != nil {
		t.Fatal(err)
	}
	file.mapping = faultingMapping(t, receipt.SectionedPath)
	// The bodies section is verified on its own goroutine during a decode.
	checked := make(chan error, 1)
	go func() {
		_, err := file.section(header, sectionBodies)
		checked <- err
	}()
	if err := <-checked; !errors.Is(err, errSnapshotFault) {
		t.Fatalf("bodies verification over a truncated mapping = %v, want errSnapshotFault", err)
	}
	if _, err := file.identity(); !errors.Is(err, errSnapshotFault) {
		t.Fatalf("identity over a truncated mapping = %v, want errSnapshotFault", err)
	}
	for _, load := range []snapshotLoad{loadCompact, loadEvent, loadFull} {
		if _, err := decodeSectionedSnapshot(file, identity, receipt.Engine, load); !errors.Is(err, errSnapshotFault) {
			t.Fatalf("load %d over a truncated mapping = %v, want errSnapshotFault", load, err)
		}
	}
}
