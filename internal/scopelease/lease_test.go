package scopelease

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newRoot(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func mustAcquire(t *testing.T, root string, request Request) Lease {
	t.Helper()
	lease, conflicts, err := Acquire(root, request)
	if err != nil {
		t.Fatalf("Acquire: %v (conflicts %+v)", err, conflicts)
	}
	return lease
}

// SCL-V0-001
func TestAcquireWritesOneLeaseDocument(t *testing.T) {
	root := newRoot(t)
	lease := mustAcquire(t, root, Request{Holder: "agent-a", Paths: []string{"internal/scopelease/**"}, TTL: 30 * time.Minute})
	if len(lease.ID) != 16 {
		t.Fatalf("lease id = %q, want 16 hex characters", lease.ID)
	}
	stored, err := read(filepath.Join(Directory(root), lease.ID+".json"))
	if err != nil {
		t.Fatalf("read stored lease: %v", err)
	}
	if stored.Holder != "agent-a" || stored.SchemaVersion != SchemaVersion || len(stored.Paths) != 1 {
		t.Fatalf("stored lease = %+v", stored)
	}
	if _, err := time.Parse(timeLayout, stored.ExpiresAt); err != nil {
		t.Fatalf("expires_at %q: %v", stored.ExpiresAt, err)
	}
}

// SCL-V0-002
func TestOverlapDecisionIsConservative(t *testing.T) {
	cases := []struct {
		held, wanted string
		overlap      bool
	}{
		{"internal/scopelease", "internal/scopelease", true},
		{"internal/**", "internal/scopelease/lease.go", true},
		{"internal/scopelease/**", "internal/**", true},
		{"internal/a/**", "internal/b/**", false},
		{"internal/aa/x.go", "internal/a/x.go", false},
		{"*.go", "internal/a/x.go", true},
		{"docs/specs/scope-lease-v0.md", "docs/specs/snapshot-batch-v0.md", false},
		// Decision 0168: a case-insensitive volume names one file for both spellings.
		{"Internal/x", "internal/x", true},
		{"INTERNAL/**", "internal/scopelease/lease.go", true},
		// Decision 0168 addendum: APFS is normalization-insensitive too, so an
		// NFC ("café") and an NFD ("café") spelling of one directory
		// name one file. Neither can be proven distinct without a
		// normalization table, so the undecidable comparison overlaps.
		{"internal/café", "internal/café", true},
		// The same undecidable-comparison rule is coarse: any non-ASCII byte
		// in either compared literal prefix overlaps, even for two scopes
		// that share no path segment.
		{"docs/éclair", "src/über", true},
	}
	for _, item := range cases {
		if got := overlaps(item.held, item.wanted); got != item.overlap {
			t.Errorf("overlaps(%q, %q) = %v, want %v", item.held, item.wanted, got, item.overlap)
		}
	}
}

// SCL-V0-009: `**` is a whole segment matching zero or more segments; `*` never
// crosses `/`.
func TestCoversMatchesDoubleStarOnlyAsWholeSegment(t *testing.T) {
	cases := []struct {
		scope, path string
		covered     bool
	}{
		{"src/**", "src/a/b.go", true},
		{"src/**/x.go", "src/a/b/x.go", true},
		{"src/**/x.go", "src/x.go", true},
		{"src/a**", "src/a/b.go", false},
		{"src/*.go", "src/a/b.go", false},
		{"src/**/x.go", "src/a/y.go", false},
	}
	for _, item := range cases {
		if got := covers(Lease{Paths: []string{item.scope}}, item.path); got != item.covered {
			t.Errorf("covers(%q, %q) = %v, want %v", item.scope, item.path, got, item.covered)
		}
	}
}

// SCL-V0-009: many `**` segments against a deep path must not backtrack
// exponentially; the recursive matcher took 2.5 s for four of them.
func TestCoversManyDoubleStarsInBoundedTime(t *testing.T) {
	scope := strings.Repeat("**/", 12) + "absent.go"
	target := strings.TrimSuffix(strings.Repeat("a/", 200), "/")
	start := time.Now()
	if covers(Lease{Paths: []string{scope}}, target) {
		t.Fatalf("covers(%q, %q) = true", scope, target)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("covers took %v, want at most 100ms", elapsed)
	}
}

// SCL-V0-003
func TestAcquireRefusesOverlappingScopeAndTicket(t *testing.T) {
	root := newRoot(t)
	held := mustAcquire(t, root, Request{
		Holder: "agent-a", Paths: []string{"internal/**"}, Ticket: "AT-42", TTL: time.Hour,
	})
	_, conflicts, err := Acquire(root, Request{
		Holder: "agent-b", Paths: []string{"internal/scopelease/lease.go"}, Ticket: "AT-42", TTL: time.Hour,
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("Acquire error = %v, want ErrConflict", err)
	}
	reasons := map[string]string{}
	for _, conflict := range conflicts {
		if conflict.LeaseID != held.ID || conflict.Holder != "agent-a" {
			t.Fatalf("conflict does not name the live lease: %+v", conflict)
		}
		reasons[conflict.Reason] = conflict.Scope
	}
	if reasons["path-overlap"] != "internal/**" || reasons["ticket"] != "AT-42" {
		t.Fatalf("conflict reasons = %+v", reasons)
	}
	reports, err := List(root)
	if err != nil || len(reports) != 1 {
		t.Fatalf("refused acquire wrote a lease: reports=%d err=%v", len(reports), err)
	}
}

// SCL-V0-004
func TestReleaseRequiresMatchingHolder(t *testing.T) {
	root := newRoot(t)
	lease := mustAcquire(t, root, Request{Holder: "agent-a", Paths: []string{"docs/**"}, TTL: time.Hour})
	if _, err := Release(root, lease.ID, "agent-b"); !errors.Is(err, ErrHolderMismatch) {
		t.Fatalf("Release by another holder = %v, want ErrHolderMismatch", err)
	}
	if _, err := Release(root, lease.ID, "agent-a"); err != nil {
		t.Fatalf("Release by holder: %v", err)
	}
	if _, err := Status(root, lease.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Status after release = %v, want ErrNotFound", err)
	}
}

// SCL-V0-005
func TestRenewExtendsExpiryForHolderOnly(t *testing.T) {
	root := newRoot(t)
	lease := mustAcquire(t, root, Request{Holder: "agent-a", Paths: []string{"docs/**"}, TTL: time.Minute})
	if _, err := Renew(root, lease.ID, "agent-b", time.Hour); !errors.Is(err, ErrHolderMismatch) {
		t.Fatalf("Renew by another holder = %v, want ErrHolderMismatch", err)
	}
	renewed, err := Renew(root, lease.ID, "agent-a", time.Hour)
	if err != nil {
		t.Fatalf("Renew: %v", err)
	}
	before, _ := time.Parse(timeLayout, lease.ExpiresAt)
	after, _ := time.Parse(timeLayout, renewed.ExpiresAt)
	if !after.After(before) {
		t.Fatalf("renewed expiry %s does not extend %s", renewed.ExpiresAt, lease.ExpiresAt)
	}
}

// TestRenewNeverShortensALease pins decision 0254: renew extends, so a
// shorter TTL than the lease has left keeps the recorded expiry.
func TestRenewNeverShortensALease(t *testing.T) {
	root := newRoot(t)
	lease := mustAcquire(t, root, Request{Holder: "agent-a", Paths: []string{"docs/**"}, TTL: time.Hour})
	renewed, err := Renew(root, lease.ID, "agent-a", time.Second)
	if err != nil {
		t.Fatalf("Renew: %v", err)
	}
	if renewed.ExpiresAt != lease.ExpiresAt {
		t.Fatalf("renewed expiry %s, want the later recorded %s", renewed.ExpiresAt, lease.ExpiresAt)
	}
	if report, err := Status(root, lease.ID); err != nil || report.Lease.ExpiresAt != lease.ExpiresAt {
		t.Fatalf("stored expiry after renew = %+v (%v), want %s", report, err, lease.ExpiresAt)
	}
}

// SCL-V0-006
func TestStatusAndListReportExpiredWithoutWriting(t *testing.T) {
	root := newRoot(t)
	lease := mustAcquire(t, root, Request{Holder: "agent-a", Paths: []string{"docs/**"}, TTL: time.Minute})
	expired := lease
	expired.ExpiresAt = time.Now().UTC().Add(-time.Minute).Format(timeLayout)
	if err := write(Directory(root), expired); err != nil {
		t.Fatalf("write expired lease: %v", err)
	}
	directory := Directory(root)
	beforeInfo, err := os.Stat(directory)
	if err != nil {
		t.Fatalf("stat lease directory: %v", err)
	}
	beforeEntries, _ := os.ReadDir(directory)

	report, err := Status(root, lease.ID)
	if err != nil || report.State != "expired" {
		t.Fatalf("Status = %+v err=%v, want state expired", report, err)
	}
	reports, err := List(root)
	if err != nil || len(reports) != 1 || reports[0].State != "expired" {
		t.Fatalf("List = %+v err=%v, want one expired report", reports, err)
	}

	afterInfo, err := os.Stat(directory)
	if err != nil {
		t.Fatalf("stat lease directory: %v", err)
	}
	if !afterInfo.ModTime().Equal(beforeInfo.ModTime()) {
		t.Fatalf("read commands changed lease directory mtime")
	}
	afterEntries, _ := os.ReadDir(directory)
	if len(afterEntries) != len(beforeEntries) {
		t.Fatalf("read commands changed lease directory contents: %d -> %d", len(beforeEntries), len(afterEntries))
	}
}

// SCL-V0-006
func TestReadCommandsCreateNoLeaseDirectory(t *testing.T) {
	root := newRoot(t)
	reports, err := List(root)
	if err != nil || len(reports) != 0 {
		t.Fatalf("List on empty root = %+v err=%v", reports, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".corvint")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("List created local state: %v", err)
	}
}

// SCL-V0-007
func TestAcquireReapsExpiredLease(t *testing.T) {
	root := newRoot(t)
	lease := mustAcquire(t, root, Request{Holder: "agent-a", Paths: []string{"internal/**"}, TTL: time.Minute})
	expired := lease
	expired.ExpiresAt = time.Now().UTC().Add(-time.Minute).Format(timeLayout)
	if err := write(Directory(root), expired); err != nil {
		t.Fatalf("write expired lease: %v", err)
	}
	fresh := mustAcquire(t, root, Request{Holder: "agent-b", Paths: []string{"internal/**"}, TTL: time.Minute})
	if _, err := os.Stat(filepath.Join(Directory(root), lease.ID+".json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expired lease survived acquire: %v", err)
	}
	if fresh.Holder != "agent-b" {
		t.Fatalf("fresh lease = %+v", fresh)
	}
}

// SCL-V0-008
func TestConcurrentAcquireAdmitsExactlyOneHolder(t *testing.T) {
	root := newRoot(t)
	var group sync.WaitGroup
	results := make([]error, 2)
	group.Add(2)
	for index, holder := range []string{"agent-a", "agent-b"} {
		go func(index int, holder string) {
			defer group.Done()
			_, _, err := Acquire(root, Request{Holder: holder, Paths: []string{"internal/scopelease/**"}, TTL: time.Hour})
			results[index] = err
		}(index, holder)
	}
	group.Wait()
	successes := 0
	for _, err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrConflict):
		default:
			t.Fatalf("unexpected concurrent acquire error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent acquires succeeded %d times, want 1", successes)
	}
	reports, err := List(root)
	if err != nil || len(reports) != 1 {
		t.Fatalf("List after contention = %d leases, err=%v", len(reports), err)
	}
}

// SCL-V0-008
func TestStaleLockIsReclaimed(t *testing.T) {
	root := newRoot(t)
	directory := Directory(root)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create lease directory: %v", err)
	}
	lockPath := filepath.Join(directory, lockName)
	age := func(t *testing.T) {
		t.Helper()
		stale := time.Now().Add(-2 * StaleLockAge)
		if err := os.Chtimes(lockPath, stale, stale); err != nil {
			t.Fatalf("age lock: %v", err)
		}
	}

	// A lock a live writer holds is never taken over: two writers would then
	// both believe they hold it.
	unlock, err := lock(directory)
	if err != nil {
		t.Fatalf("hold lock: %v", err)
	}
	if _, _, err := Acquire(root, Request{Holder: "agent-a", Paths: []string{"docs/**"}, TTL: time.Hour}); !errors.Is(err, ErrLockBusy) {
		t.Fatalf("Acquire under a held lock = %v, want ErrLockBusy", err)
	}
	unlock()

	if err := os.WriteFile(lockPath, []byte(strconv.Itoa(exitedProcess(t))+"\n"), 0o600); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	age(t)
	if _, _, err := Acquire(root, Request{Holder: "agent-a", Paths: []string{"docs/**"}, TTL: time.Hour}); err != nil {
		t.Fatalf("Acquire over stale lock: %v", err)
	}
}

// SCL-V0-008
func TestStaleLockContentionGrantsAtMostOneLease(t *testing.T) {
	dead := exitedProcess(t)
	for round := 0; round < 40; round++ {
		root := newRoot(t)
		directory := Directory(root)
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatalf("create lease directory: %v", err)
		}
		lockPath := filepath.Join(directory, lockName)
		if err := os.WriteFile(lockPath, []byte(strconv.Itoa(dead)+"\n"), 0o600); err != nil {
			t.Fatalf("write stale lock: %v", err)
		}
		stale := time.Now().Add(-2 * StaleLockAge)
		if err := os.Chtimes(lockPath, stale, stale); err != nil {
			t.Fatalf("age lock: %v", err)
		}
		start := make(chan struct{})
		var group sync.WaitGroup
		var granted atomic.Int32
		for index := 0; index < 32; index++ {
			group.Add(1)
			go func(holder string) {
				defer group.Done()
				<-start
				_, _, err := Acquire(root, Request{Holder: holder, Paths: []string{"src/x.go"}, TTL: time.Hour})
				if err == nil {
					granted.Add(1)
					return
				}
				if !errors.Is(err, ErrConflict) && !errors.Is(err, ErrLockBusy) {
					t.Errorf("unexpected contended acquire error: %v", err)
				}
			}("agent-" + strconv.Itoa(index))
		}
		close(start)
		group.Wait()
		reports, err := List(root)
		if err != nil {
			t.Fatalf("List after contention: %v", err)
		}
		if granted.Load() > 1 || len(reports) > 1 {
			t.Fatalf("round %d: stale-lock contention granted %d leases with %d documents, want at most 1", round, granted.Load(), len(reports))
		}
	}
}

// SCL-V0-011
func TestUnreadableLeaseDocumentBlocksOnlyNewClaims(t *testing.T) {
	root := newRoot(t)
	held := mustAcquire(t, root, Request{Holder: "agent-a", Paths: []string{"src/a.go"}, TTL: time.Hour})
	const tornID = "0123456789abcdef"
	tornPath := filepath.Join(Directory(root), tornID+".json")
	if err := os.WriteFile(tornPath, []byte(`{"schema_version":1,"lease_id":"0123`), 0o600); err != nil {
		t.Fatalf("write torn lease: %v", err)
	}

	reports, err := List(root)
	if err != nil || len(reports) != 2 {
		t.Fatalf("List with a torn document = %+v, err=%v", reports, err)
	}
	// List sorts by identifier and the held ID is random, so the torn
	// report's position varies; find it by ID.
	tornStates := make([]string, 0, 1)
	for _, entry := range reports {
		if entry.Lease.ID == tornID {
			tornStates = append(tornStates, entry.State)
		}
	}
	if len(tornStates) != 1 || tornStates[0] != "unreadable" {
		t.Fatalf("List with a torn document = %+v, want one unreadable %s report", reports, tornID)
	}
	_, conflicts, err := Acquire(root, Request{Holder: "agent-b", Paths: []string{"src/b.go"}, TTL: time.Hour})
	if !errors.Is(err, ErrConflict) || len(conflicts) != 1 || conflicts[0].LeaseID != tornID || conflicts[0].Reason != "unreadable-lease" {
		t.Fatalf("Acquire beside a torn document = %v, conflicts %+v", err, conflicts)
	}
	checked, err := Check(root, []string{"src/a.go"})
	if err != nil || len(checked) != 1 || checked[0].Reason != "unreadable-lease" {
		t.Fatalf("Check beside a torn document = %+v, err=%v", checked, err)
	}
	if _, err := Renew(root, held.ID, "agent-a", time.Hour); err != nil {
		t.Fatalf("Renew beside a torn document: %v", err)
	}
	if _, err := Release(root, held.ID, "agent-a"); err != nil {
		t.Fatalf("Release beside a torn document: %v", err)
	}
	if _, err := os.Stat(tornPath); err != nil {
		t.Fatalf("torn document must survive reaping: %v", err)
	}

	if err := os.Remove(tornPath); err != nil {
		t.Fatalf("remove torn lease: %v", err)
	}
	mustAcquire(t, root, Request{Holder: "agent-b", Paths: []string{"src/b.go"}, TTL: time.Hour})
}

// exitedProcess returns the process id of a child that has already run and been
// reaped, which is the closest a test can get to a holder that crashed.
func exitedProcess(t *testing.T) int {
	t.Helper()
	command := exec.Command("go", "version")
	if err := command.Run(); err != nil {
		t.Fatalf("run a throwaway child: %v", err)
	}
	return command.Process.Pid
}

// SCL-V0-009
func TestCheckReportsUncoveredAndMultiplyCoveredPaths(t *testing.T) {
	root := newRoot(t)
	mustAcquire(t, root, Request{Holder: "agent-a", Paths: []string{"internal/scopelease/**"}, TTL: time.Hour})
	conflicts, err := Check(root, []string{"internal/scopelease/lease.go", "cmd/corvint/main.go"})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(conflicts) != 1 || conflicts[0].Reason != "uncovered" || conflicts[0].Scope != "cmd/corvint/main.go" {
		t.Fatalf("Check conflicts = %+v", conflicts)
	}
	// A second overlapping lease can only exist if one was written outside the
	// refusal path; Check must still report the double coverage.
	overlapping := Lease{
		SchemaVersion: SchemaVersion, ID: "00000000000000ff", Holder: "agent-b",
		Paths: []string{"internal/scopelease/lease.go"}, AcquiredAt: time.Now().UTC().Format(timeLayout),
		ExpiresAt: time.Now().UTC().Add(time.Hour).Format(timeLayout),
	}
	if err := write(Directory(root), overlapping); err != nil {
		t.Fatalf("write overlapping lease: %v", err)
	}
	conflicts, err = Check(root, []string{"internal/scopelease/lease.go"})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(conflicts) != 2 {
		t.Fatalf("Check multiple-coverage conflicts = %+v", conflicts)
	}
	for _, conflict := range conflicts {
		if conflict.Reason != "multiple-leases" {
			t.Fatalf("conflict reason = %q, want multiple-leases", conflict.Reason)
		}
	}
}

// SCL-V0-002: equivalent spellings of one scope normalize before overlap.
func TestAcquireNormalizesEquivalentPathSpellings(t *testing.T) {
	root := newRoot(t)
	mustAcquire(t, root, Request{Holder: "agent-a", Paths: []string{"internal/x/a.go"}, TTL: time.Hour})
	for _, spelling := range []string{"./internal/x/a.go", "internal//x/a.go", "internal/./x/a.go"} {
		_, conflicts, err := Acquire(root, Request{Holder: "agent-b", Paths: []string{spelling}, TTL: time.Hour})
		if !errors.Is(err, ErrConflict) || len(conflicts) != 1 || conflicts[0].Reason != "path-overlap" {
			t.Errorf("Acquire(%q) beside internal/x/a.go = %v, conflicts %+v; want one path-overlap", spelling, err, conflicts)
		}
	}
}

// SCL-V0-011: a document whose recorded id differs from its file name is
// unreadable, so reaping or releasing it can never remove another lease.
func TestMismatchedLeaseIdentifierCannotRemoveAnotherLease(t *testing.T) {
	root := newRoot(t)
	victim := mustAcquire(t, root, Request{Holder: "agent-a", Paths: []string{"src/a.go"}, TTL: time.Hour})
	const forgedID = "0123456789abcdef"
	forged := `{"schema_version":1,"lease_id":"` + victim.ID + `","holder":"agent-b","paths":["src/b.go"],"ticket":"","note":"","acquired_at":"2000-01-01T00:00:00Z","expires_at":"2000-01-01T00:00:01Z","revision":""}`
	forgedPath := filepath.Join(Directory(root), forgedID+".json")
	if err := os.WriteFile(forgedPath, []byte(forged), 0o600); err != nil {
		t.Fatalf("write forged lease: %v", err)
	}
	_, conflicts, err := Acquire(root, Request{Holder: "agent-c", Paths: []string{"src/c.go"}, TTL: time.Hour})
	if !errors.Is(err, ErrConflict) || len(conflicts) != 1 || conflicts[0].LeaseID != forgedID || conflicts[0].Reason != unreadableReason {
		t.Fatalf("Acquire beside a forged document = %v, conflicts %+v", err, conflicts)
	}
	if _, err := os.Stat(filepath.Join(Directory(root), victim.ID+".json")); err != nil {
		t.Fatalf("reaping a forged document removed the live lease it named: %v", err)
	}
	if _, err := os.Stat(forgedPath); err != nil {
		t.Fatalf("forged document must survive reaping: %v", err)
	}
}

// SCL-V0-008: a repository can commit .corvint or .corvint/leases as a symlink.
// Following it would create the lock, publish a lease, and reap documents
// outside the worktree, so acquire and release refuse before touching it.
func TestMutatingActionsRefuseSymlinkedLeaseDirectory(t *testing.T) {
	for _, link := range []string{".corvint", ".corvint/leases"} {
		t.Run(link, func(t *testing.T) {
			root := newRoot(t)
			outside := t.TempDir()
			linkPath := filepath.Join(root, filepath.FromSlash(link))
			if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, linkPath); err != nil {
				t.Fatal(err)
			}
			request := Request{Holder: "agent-a", Paths: []string{"internal/**"}, TTL: time.Minute}
			if _, _, err := Acquire(root, request); err == nil {
				t.Error("SCL-V0-008: acquire wrote through a symlinked lease directory")
			}
			if _, err := Release(root, "0123456789abcdef", "agent-a"); err == nil || errors.Is(err, ErrNotFound) {
				t.Errorf("SCL-V0-008: release through a symlinked lease directory = %v, want a refusal", err)
			}
			entries, err := os.ReadDir(outside)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("SCL-V0-008: outside directory gained entries: %v", entries)
			}
		})
	}
}

// SCL-V0-001
func TestAnOversizeLeaseDocumentIsNeverWrittenOrRead(t *testing.T) {
	root := newRoot(t)
	oversize := strings.Repeat("n", documentLimit)
	if _, _, err := Acquire(root, Request{Holder: "agent-a", Paths: []string{"src/a.go"}, Note: oversize, TTL: time.Hour}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Acquire with an oversize note = %v, want ErrInvalidRequest", err)
	}
	const foreignID = "0123456789abcdef"
	document := `{"schema_version":1,"lease_id":"` + foreignID + `","holder":"agent-b","paths":["src/b.go"],"ticket":"","note":"` +
		oversize + `","acquired_at":"2026-09-13T00:00:00Z","expires_at":"2999-01-01T00:00:00Z","revision":""}`
	if err := os.MkdirAll(Directory(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(Directory(root), foreignID+".json"), []byte(document), 0o600); err != nil {
		t.Fatal(err)
	}
	reports, err := List(root)
	if err != nil || len(reports) != 1 {
		t.Fatalf("List beside an oversize document: %d reports, err=%v, want one", len(reports), err)
	}
	if reports[0].Lease.ID != foreignID || reports[0].State != "unreadable" {
		t.Fatalf("List beside an oversize document = %s %s, want %s unreadable", reports[0].Lease.ID, reports[0].State, foreignID)
	}
}

func TestAcquireRecordsOnlyARevisionGitWouldResolve(t *testing.T) {
	const commit = "0123456789abcdef0123456789abcdef01234567"
	for name, files := range map[string]map[string]string{
		"detached":        {".git/HEAD": commit + "\n"},
		"loose branch":    {".git/HEAD": "ref: refs/heads/main\n", ".git/refs/heads/main": commit + "\n"},
		"packed only":     {".git/HEAD": "ref: refs/heads/main\n", ".git/packed-refs": commit + " refs/heads/main\n"},
		"escaping ref":    {".git/HEAD": "ref: ../../outside\n", "../outside": commit + "\n"},
		"linked worktree": {".git": "gitdir: ../gitdir\n", "../gitdir/HEAD": commit + "\n"},
	} {
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "repo")
			for relative, content := range files {
				target := filepath.Join(root, filepath.FromSlash(relative))
				if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want := commit
			if name != "detached" && name != "loose branch" {
				want = ""
			}
			lease := mustAcquire(t, root, Request{Holder: "agent-a", Paths: []string{"a.go"}, TTL: time.Minute})
			if lease.Revision != want {
				t.Fatalf("revision = %q, want %q", lease.Revision, want)
			}
		})
	}
}
