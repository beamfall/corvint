//go:build unix

package contextindex

import (
	"errors"
	"os"
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
