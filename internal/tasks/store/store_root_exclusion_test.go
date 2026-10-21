package store

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
)

// forgetLeaseAudits drops every retained lease audit, so the next lease write
// in this process runs a complete audit.
func forgetLeaseAudits() {
	leaseAudits.Lock()
	clear(leaseAudits.entries)
	leaseAudits.Unlock()
}

// programWriterRefs is the holder and waiter count of a store's program
// writer entry, or 0 when the store has none.
func programWriterRefs(root string) int {
	programWriters.Lock()
	defer programWriters.Unlock()
	if e := programWriters.byRoot[root]; e != nil {
		return e.refs
	}
	return 0
}

// V1-1108: the in-process program writer excludes per store root. A second
// store proceeds while one store's writer is held, and a second writer on the
// held store waits on the same entry. The entry counts its holder and waiter
// and is removed once both released, so the map does not grow per store.
func TestV11108_ProgramWriterExcludesPerStoreRoot(t *testing.T) {
	a := &intent.Repository{StateDir: t.TempDir() + "/a/.git/taskman"}
	sameA := &intent.Repository{StateDir: a.StateDir}
	b := &intent.Repository{StateDir: t.TempDir() + "/b/.git/taskman"}
	unlockA := lockProgramWriter(a)
	acquired := make(chan func(), 1)
	go func() { acquired <- lockProgramWriter(sameA) }()
	deadline := time.Now().Add(2 * time.Minute)
	for programWriterRefs(a.StateDir) != 2 {
		if time.Now().After(deadline) {
			t.Fatal("the second writer never registered on the held store's entry (hang guard)")
		}
		runtime.Gosched()
	}
	select {
	case <-acquired:
		t.Fatal("a second writer on the held store did not wait")
	default:
	}
	other := make(chan func(), 1)
	go func() { other <- lockProgramWriter(b) }()
	select {
	case unlockB := <-other:
		unlockB()
	case <-time.After(2 * time.Minute):
		t.Fatal("an independent store waited on another store's program writer (hang guard)")
	}
	if n := programWriterRefs(b.StateDir); n != 0 {
		t.Fatalf("released store kept an entry with %d refs", n)
	}
	unlockA()
	select {
	case unlock := <-acquired:
		unlock()
	case <-time.After(2 * time.Minute):
		t.Fatal("the waiting writer never acquired after release (hang guard)")
	}
	programWriters.Lock()
	_, kept := programWriters.byRoot[a.StateDir]
	programWriters.Unlock()
	if kept {
		t.Fatal("a store with no holder or waiter kept its program writer entry")
	}
}

// V1-1108: the lease audit cache keeps one audit per store root, so an audit
// retained for one store is not evicted by audits of other stores (up to the
// bound), and a root past the bound evicts only one other root's entry.
func TestV11108_LeaseAuditCacheKeepsOneAuditPerStoreRoot(t *testing.T) {
	leaseAudits.Lock()
	saved := leaseAudits.entries
	leaseAudits.entries = nil
	leaseAudits.Unlock()
	t.Cleanup(func() {
		leaseAudits.Lock()
		leaseAudits.entries = saved
		leaseAudits.Unlock()
	})
	proofs := map[string]*journal.Result{}
	for i := range leaseAuditRoots {
		root := fmt.Sprintf("/store-%d/.git/taskman", i)
		proofs[root] = &journal.Result{}
		retainLeaseAudit(leaseAuditKey{root: root}, proofs[root])
	}
	for root, proof := range proofs {
		if got := cachedLeaseAudit(root); got.proof != proof || got.key.root != root {
			t.Fatalf("%s: audit evicted by another store's audit", root)
		}
	}
	retainLeaseAudit(leaseAuditKey{root: "/extra/.git/taskman"}, &journal.Result{})
	kept := 0
	for root := range proofs {
		if cachedLeaseAudit(root).proof != nil {
			kept++
		}
	}
	if kept != leaseAuditRoots-1 || cachedLeaseAudit("/extra/.git/taskman").proof == nil {
		t.Fatalf("past the bound: kept %d of %d earlier roots", kept, leaseAuditRoots)
	}
	leaseAudits.Lock()
	n := len(leaseAudits.entries)
	leaseAudits.Unlock()
	if n != leaseAuditRoots {
		t.Fatalf("cache holds %d roots, bound %d", n, leaseAuditRoots)
	}
}
