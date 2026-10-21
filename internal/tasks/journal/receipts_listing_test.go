//go:build darwin || linux

package journal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
)

// listingHarness is a settled writer checkpoint with a short tail and a
// counter of receipts/ listings.
type listingHarness struct {
	repo     *fixture.Repo
	r        Reader
	wc       *WriterCheckpoint
	receipts string
	next     string
	listings *int
	attempts *int
	// captures records the cumulative listing count after each attempt's
	// first capture, so a first capture that reused a listing shows as no
	// listing of its own.
	captures *[]int
	during   func() // runs after each attempt's first capture
}

func newListingHarness(t *testing.T) *listingHarness {
	t.Helper()
	repo, r := setup(t)
	const history, tail = 4, 2
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
	h := &listingHarness{repo: repo, r: r, wc: wc, receipts: filepath.Join(repo.StateDir, "receipts"), listings: new(int), attempts: new(int), captures: new([]int)}
	h.next, _ = snapshot.ReceiptName(last + 1)
	h.r.afterCapture = func() {
		*h.attempts++
		*h.captures = append(*h.captures, *h.listings)
		if h.during != nil {
			h.during()
		}
	}
	old := afterReadNames
	t.Cleanup(func() { afterReadNames = old })
	afterReadNames = func(p string) {
		if p == "receipts" {
			*h.listings++
		}
	}
	return h
}

func (h *listingHarness) audit(t *testing.T) error {
	t.Helper()
	*h.listings, *h.attempts, *h.captures = 0, 0, nil
	res, _, err := h.r.AuditForWriter(h.wc, "", true)
	if err == nil && res.Mode != ModeWriter {
		t.Fatalf("writer audit ran in mode %s", res.Mode)
	}
	for i, n := range *h.captures {
		if want := 2*i + 1; n != want {
			t.Fatalf("attempt %d's first capture ended at %d cumulative listings, want %d: every first capture lists exactly once", i+1, n, want)
		}
	}
	return err
}

// TestCALV0224_ForeignReceiptDuringWalkMovesQuiescentListing covers the
// concurrent-writer side of CAL-V0-224 as JSG-V0-007 amends it: a
// settled-looking receipt that a foreign process adds after the first
// listing, with head.json untouched, is in the second capture's exact
// listing, so the attempt moves instead of accepting it unseen; the retried
// attempt lists the directory twice and declines on the extra name (4
// listings in 2 attempts). Mutation: a second capture that reuses the first
// listing accepts the first attempt with the foreign receipt unseen.
func TestCALV0224_ForeignReceiptDuringWalkMovesQuiescentListing(t *testing.T) {
	h := newListingHarness(t)
	if err := h.audit(t); err != nil || *h.listings != 2 || *h.attempts != 1 {
		t.Fatalf("settled audit: %v with %d listings in %d attempts, want two and one", err, *h.listings, *h.attempts)
	}
	h.during = func() {
		if *h.attempts == 1 {
			fixture.Write(t, filepath.Join(h.receipts, h.next), []byte("x"))
		}
	}
	err := h.audit(t)
	if err == nil || !strings.Contains(err.Error(), "receipt names differ from head") {
		t.Fatalf("receipt added during the walk: %v, want the retried attempt's decline", err)
	}
	if *h.attempts != 2 || *h.listings != 4 {
		t.Fatalf("%d listings in %d attempts after a receipt was added during the walk, want 4 in 2", *h.listings, *h.attempts)
	}
}

// TestCALV0224_ListingNeverOutlivesItsAttempt covers the crash-recovery
// side: every capture lists exactly even when nothing changed, and the
// temporary name a killed writer left behind under restored directory
// stamps is found by that listing and declines the fast route. Mutation: a
// listing retained across attempts makes the second audit list once.
func TestCALV0224_ListingNeverOutlivesItsAttempt(t *testing.T) {
	h := newListingHarness(t)
	for i := 0; i < 2; i++ {
		if err := h.audit(t); err != nil || *h.listings != 2 {
			t.Fatalf("audit %d: %v with %d listings, want exactly two", i+1, err, *h.listings)
		}
	}
	fixture.Write(t, filepath.Join(h.receipts, h.next+".tmp-1-x"), []byte("x"))
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(h.receipts, past, past); err != nil {
		t.Fatal(err)
	}
	err := h.audit(t)
	if err == nil || !strings.Contains(err.Error(), "not plainly settled") {
		t.Fatalf("orphaned temporary name under restored stamps: %v, want a decline", err)
	}
	if *h.listings != 2 {
		t.Fatalf("%d listings with an orphaned temporary name, want exactly two", *h.listings)
	}
}
