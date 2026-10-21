//go:build darwin || linux

package journal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
)

// TestCALV0224_LivenessProbeNeverListsReceipts covers the V1-0887
// amendment of CAL-V0-224 (decision 0549): over a quiescent receipts/, a
// liveness writer audit never lists it, at two history sizes, reads exactly
// the checkpoint's receipt and the tail, declines on a pending receipt after
// head, and moves the attempt when a name is renamed after its first capture
// under a restored modification time; the retry finds the directory freshly
// changed, lists it exactly and declines the stray name. A temporary or
// stray name the probe does not name is not observed (the recorded failure
// mode); the listing writer audit still declines it. Mutations: listing in
// the probe branch fails the zero-listing count; dropping the next-name
// probe accepts the pending receipt; dropping the second capture's stamp
// comparison keeps the first attempt.
func TestCALV0224_LivenessProbeNeverListsReceipts(t *testing.T) {
	const tail = 3
	for _, history := range []int{4, 48} {
		t.Run(fmt.Sprint(history), func(t *testing.T) {
			repo, r := setup(t)
			receipts := filepath.Join(repo.StateDir, "receipts")
			for i := 0; i < history; i++ {
				id := fmt.Sprintf("H%d", i)
				appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath(id): fixture.Ticket(id).Encode()}, "", true, true, false)
			}
			full, observed, err := r.AuditForWriteObserved()
			if err != nil || full.Mode != ModeFull {
				t.Fatalf("complete audit: %v", err)
			}
			wc, err := full.WriterCheckpoint(observed.Files)
			if err != nil || wc == nil {
				t.Fatalf("writer checkpoint: %v", err)
			}
			var last uint64
			for i := 0; i < tail; i++ {
				id := fmt.Sprintf("T%d", i)
				last = appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath(id): fixture.Ticket(id).Encode()}, "", true, true, false)
			}
			listings, reads, attempts := 0, 0, 0
			oldNames, oldRead := afterReadNames, afterNativeRead
			t.Cleanup(func() { afterReadNames, afterNativeRead = oldNames, oldRead })
			afterReadNames = func(p string) {
				if p == "receipts" {
					listings++
				}
			}
			afterNativeRead = func(p string) {
				if strings.HasPrefix(p, "receipts/") {
					reads++
				}
			}
			var during func()
			r.afterCapture = func() {
				attempts++
				if during != nil {
					during()
				}
			}
			live := func() error {
				listings, reads, attempts = 0, 0, 0
				res, _, err := r.AuditForLiveness(wc, "")
				if err == nil && res.Mode != ModeWriter {
					t.Fatalf("liveness audit ran in mode %s", res.Mode)
				}
				return err
			}
			awaitQuiescent(t, receipts)
			if err := live(); err != nil || listings != 0 || reads != tail+1 || attempts != 1 {
				t.Fatalf("settled liveness audit: %v with %d listings, %d receipt reads in %d attempts, want none, %d and one", err, listings, reads, attempts, tail+1)
			}

			next, _ := snapshot.ReceiptName(last + 1)
			pending := filepath.Join(receipts, next)
			fixture.Write(t, pending, []byte("x"))
			awaitQuiescent(t, receipts)
			if err := live(); err == nil || !strings.Contains(err.Error(), "a receipt follows head") || listings != 0 {
				t.Fatalf("pending receipt after head: %v with %d listings, want the probe's decline and none", err, listings)
			}
			if err := os.Remove(pending); err != nil {
				t.Fatal(err)
			}

			// A name renamed after the first capture, with the directory's
			// modification time restored, keeps the directory's mode, size and
			// modification time, which the inventory digest binds, and changes
			// only its change time: the second capture's stamp comparison moves
			// the attempt. The retry finds the directory freshly changed, so
			// it lists exactly and declines the name after head.
			from, _ := snapshot.ReceiptName(last + 2)
			to, _ := snapshot.ReceiptName(last + 3)
			fixture.Write(t, filepath.Join(receipts, from), []byte("x"))
			info, err := os.Lstat(receipts)
			if err != nil {
				t.Fatal(err)
			}
			mtime := info.ModTime()
			awaitQuiescent(t, receipts)
			during = func() {
				if attempts == 1 {
					if err := os.Rename(filepath.Join(receipts, from), filepath.Join(receipts, to)); err != nil {
						t.Fatal(err)
					}
					if err := os.Chtimes(receipts, mtime, mtime); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := live(); err == nil || !strings.Contains(err.Error(), "receipt names differ from head") || attempts != 2 || listings != 2 {
				t.Fatalf("name renamed during the walk: %v in %d attempts with %d listings, want a moved first attempt and a retry whose two captures list and decline", err, attempts, listings)
			}
			during = nil
			if err := os.Remove(filepath.Join(receipts, to)); err != nil {
				t.Fatal(err)
			}

			// The recorded failure mode: a temporary name below head is not
			// observed by the probe; the next listing writer audit declines.
			stray, _ := snapshot.ReceiptName(last)
			fixture.Write(t, filepath.Join(receipts, stray+".tmp-1-x"), []byte("x"))
			awaitQuiescent(t, receipts)
			if err := live(); err != nil {
				t.Fatalf("liveness audit with a temporary name below head: %v, want the probe to pass", err)
			}
			if _, _, err := r.AuditForWriter(wc, "", false); err == nil || !strings.Contains(err.Error(), "not plainly settled") {
				t.Fatalf("listing writer audit with a temporary name: %v, want a decline", err)
			}
		})
	}
}

// TestCALV0224_LivenessProbeNeedsQuiescentDirectory covers decision 0549's
// tightening of the CAL-V0-224 probe: equal directory stamps cannot exclude
// a rename within one timestamp tick at an unchanged size, so a liveness
// audit probes only a receipts/ whose change time was already
// receiptsQuiescence old at its first capture. Right after an append, both
// captures list exactly, and a rename of the name after head to a later one
// during the walk (the H to H+2 collision at an unchanged size) is never
// committed over.
// Mutation: dropping the quiescence check probes the fresh directory, whose
// zero-listing count and acceptance of the renamed name fail here.
func TestCALV0224_LivenessProbeNeedsQuiescentDirectory(t *testing.T) {
	repo, r := setup(t)
	receipts := filepath.Join(repo.StateDir, "receipts")
	for i := 0; i < 4; i++ {
		id := fmt.Sprintf("H%d", i)
		appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath(id): fixture.Ticket(id).Encode()}, "", true, true, false)
	}
	full, observed, err := r.AuditForWriteObserved()
	if err != nil || full.Mode != ModeFull {
		t.Fatalf("complete audit: %v", err)
	}
	wc, err := full.WriterCheckpoint(observed.Files)
	if err != nil || wc == nil {
		t.Fatalf("writer checkpoint: %v", err)
	}
	last := appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("T"): fixture.Ticket("T").Encode()}, "", true, true, false)
	listings := 0
	oldNames := afterReadNames
	t.Cleanup(func() { afterReadNames = oldNames })
	afterReadNames = func(p string) {
		if p == "receipts" {
			listings++
		}
	}
	if _, _, err := r.AuditForLiveness(wc, ""); err != nil || listings != 2 {
		t.Fatalf("liveness audit over a freshly changed receipts/: %v with %d listings, want both captures to list", err, listings)
	}

	from, _ := snapshot.ReceiptName(last)
	to, _ := snapshot.ReceiptName(last + 2)
	info, err := os.Lstat(receipts)
	if err != nil {
		t.Fatal(err)
	}
	mtime := info.ModTime()
	renamed := false
	r.afterCapture = func() {
		if renamed {
			return
		}
		renamed = true
		if err := os.Rename(filepath.Join(receipts, from), filepath.Join(receipts, to)); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(receipts, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	listings = 0
	if res, _, err := r.AuditForLiveness(wc, ""); err == nil || listings == 0 {
		t.Fatalf("head receipt renamed to head+2 within one tick: %+v %v with %d listings, want a listed decline", res, err, listings)
	}
}
