package store_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// amendMap commits an IMPORT_PLAN amendment of repo's import map.
func amendMap(t *testing.T, repo *intent.Repository, actor mutation.Binding, request string, expected wire.Digest, next []byte) *store.Report {
	t.Helper()
	report, err := store.ImportMapAmend(context.Background(), repo, actor, store.ImportMapRequest{QueueID: fixture.QueueID, RequestID: request, ExpectedImportMapSha256: expected, ImportMap: next}, now(t))
	if err != nil {
		t.Fatalf("amend %s: %v", request, err)
	}
	return report
}

// TestCTSV0020_ImportMapAmendCommitsAndReplays: an OWNER amendment that
// names the journaled map commits one IMPORT_PLAN receipt posting the new
// map and the queue naming it; the projection follows, the journal audits
// CONSISTENT, an import honours the amended map, and the same request
// replays without a second receipt. Before CTS-V0-020 no write could post
// the map after genesis.
func TestCTSV0020_ImportMapAmendCommitsAndReplays(t *testing.T) {
	t.Parallel()
	old := importMapBytes(fixture.QueueID, [3]string{"queue:beamfall:main", "BF-1", "BF-1"})
	repo := mappedStore(t, "ROADMAP", old)
	runImport(t, repo, importExport(importItem("BF-1", "one\n", nil)))
	next := importMapBytes(fixture.QueueID,
		[3]string{"queue:beamfall:main", "BF-1", "BF-1"},
		[3]string{"queue:beamfall:main", "BF-2", "BF-2"},
		[3]string{"queue:other:main", "X-1", "X-1"})
	report := amendMap(t, repo, operator(), "amend-1", wire.Sum(old), next)
	if report.Outcome.Outcome != mutation.OutcomeCompleted || report.Receipt == "" {
		t.Fatalf("amend: %+v", report)
	}
	auditOK(t, repo)
	raw, err := os.ReadFile(filepath.Join(repo.PrimaryWorktree, intent.Dir, "queue.json"))
	if err != nil {
		t.Fatal(err)
	}
	if q, err := intent.DecodeQueue(raw); err != nil || q.ImportMapSha256 == nil || *q.ImportMapSha256 != wire.Sum(next) {
		t.Fatalf("queue does not name the amended map: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(repo.PrimaryWorktree, intent.Dir, intent.ImportMapFile)); err != nil || string(got) != string(next) {
		t.Fatalf("import map projection: %v", err)
	}
	receipt, err := os.ReadFile(filepath.Join(repo.StateDir, "receipts", report.Receipt))
	if err != nil || !strings.Contains(string(receipt), `"kind":"IMPORT_PLAN"`) {
		t.Fatalf("receipt %s: %v", report.Receipt, err)
	}
	before := storeDigest(t, repo)
	again := amendMap(t, repo, operator(), "amend-1", wire.Sum(old), next)
	if again.Outcome.Outcome != mutation.OutcomeCompleted || again.Receipt != "" || storeDigest(t, repo) != before {
		t.Fatalf("replay: %+v", again)
	}
	runImport(t, repo, importExport(importItem("BF-1", "one\n", nil), importItem("BF-2", "two\n", nil)))
	auditOK(t, repo)
}

// TestCTSV0021_ImportMapAmendRefusesConflicts: an amendment that moves a
// mapped source item to another ticket, drops a mapped ticket, maps one
// ticket twice, re-points an item an existing ticket imports, or claims an
// existing ticket that is not that item's import is refused DUPLICATE_ID
// naming the conflict; a stale expected map is a
// REVISION_CONFLICT, an OPERATOR binding UNAUTHORIZED and a map of another
// queue MALFORMED. Nothing is written in any case, and existing ticket
// identities stay as they were.
func TestCTSV0021_ImportMapAmendRefusesConflicts(t *testing.T) {
	t.Parallel()
	old := importMapBytes(fixture.QueueID, [3]string{"queue:beamfall:main", "BF-1", "BF-1"})
	repo := mappedStore(t, "ROADMAP", old)
	runImport(t, repo, importExport(importItem("BF-1", "one\n", nil), importItem("BF-2", "two\n", nil)))
	created := mutate(t, repo, envelope("create-a", mutation.OpCreate, "", "", createPayload("a")))
	if created.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("create: %+v", created)
	}
	native := strings.TrimPrefix(created.Ticket, "ticket:"+strings.TrimPrefix(fixture.QueueID, "queue:")+":")
	conflicts := map[string]struct {
		next []byte
		want string
	}{
		"moved item":     {importMapBytes(fixture.QueueID, [3]string{"queue:beamfall:main", "BF-1", "BF-9"}), "moves from ticket"},
		"dropped ticket": {importMapBytes(fixture.QueueID, [3]string{"queue:beamfall:main", "BF-3", "BF-3"}), "loses its mapping"},
		"ticket twice":   {importMapBytes(fixture.QueueID, [3]string{"queue:beamfall:main", "BF-1", "BF-1"}, [3]string{"queue:other:main", "X-1", "BF-1"}), "mapped twice"},
		"imported item":  {importMapBytes(fixture.QueueID, [3]string{"queue:beamfall:main", "BF-1", "BF-1"}, [3]string{"queue:beamfall:main", "BF-2", "BF-9"}), "is imported as ticket"},
		"other import":   {importMapBytes(fixture.QueueID, [3]string{"queue:beamfall:main", "BF-1", "BF-1"}, [3]string{"queue:other:main", "X-2", "BF-2"}), "is not that item's import"},
		"native ticket":  {importMapBytes(fixture.QueueID, [3]string{"queue:beamfall:main", "BF-1", "BF-1"}, [3]string{"queue:other:main", "X-3", native}), "is not that item's import"},
	}
	before := storeDigest(t, repo)
	for name, c := range conflicts {
		report := amendMap(t, repo, operator(), "amend-"+strings.ReplaceAll(name, " ", "-"), wire.Sum(old), c.next)
		if report.Outcome.Outcome != mutation.OutcomeValidationFailed || len(report.Outcome.Codes) != 1 || report.Outcome.Codes[0] != wire.CodeDuplicateID || !strings.Contains(report.Detail, c.want) {
			t.Fatalf("%s: %+v, want DUPLICATE_ID naming %q", name, report, c.want)
		}
	}
	added := importMapBytes(fixture.QueueID, [3]string{"queue:beamfall:main", "BF-1", "BF-1"}, [3]string{"queue:beamfall:main", "BF-2", "BF-2"})
	if r := amendMap(t, repo, operator(), "amend-stale", wire.Sum(added), added); r.Outcome.Outcome != mutation.OutcomeRevisionConflict {
		t.Fatalf("stale expected map: %+v", r)
	}
	if r := amendMap(t, repo, mutation.Binding{ID: "tester", Role: "OPERATOR"}, "amend-operator", wire.Sum(old), added); r.Outcome.Outcome != mutation.OutcomeUnauthorized {
		t.Fatalf("OPERATOR amendment: %+v", r)
	}
	_, err := store.ImportMapAmend(context.Background(), repo, operator(), store.ImportMapRequest{QueueID: fixture.QueueID, RequestID: "amend-other-queue", ExpectedImportMapSha256: wire.Sum(old), ImportMap: importMapBytes("queue:other:main")}, now(t))
	if wire.CodeOf(err) != wire.CodeMalformed {
		t.Fatalf("map of another queue: %v, want MALFORMED", err)
	}
	if storeDigest(t, repo) != before {
		t.Fatal("a refused amendment wrote")
	}
	auditOK(t, repo)
	if report := amendMap(t, repo, operator(), "amend-ok", wire.Sum(old), added); report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("an admissible amendment after the refusals: %+v", report)
	}
}

// TestCTSV0800_ImportMapAmendRefusesImportedSourceRename (V1-2479): an
// amendment that renames the source item of a ticket the store holds as an
// import is refused DUPLICATE_ID naming the ticket and its import source,
// with nothing written, so the ticket's unchanged re-import stays a no-change;
// a mapped ticket the store does not hold yet may still be renamed.
func TestCTSV0800_ImportMapAmendRefusesImportedSourceRename(t *testing.T) {
	t.Parallel()
	old := importMapBytes(fixture.QueueID, [3]string{"queue:beamfall:main", "BF-1", "BF-1"}, [3]string{"queue:beamfall:main", "BF-5", "BF-5"})
	repo := mappedStore(t, "ROADMAP", old)
	export := importExport(importItem("BF-1", "one\n", nil))
	runImport(t, repo, export)
	before := storeDigest(t, repo)
	renamed := importMapBytes(fixture.QueueID, [3]string{"queue:beamfall:main", "BF-5", "BF-5"}, [3]string{"queue:beamfall:main", "NEW-1", "BF-1"})
	report := amendMap(t, repo, operator(), "amend-rename-imported", wire.Sum(old), renamed)
	if report.Outcome.Outcome != mutation.OutcomeValidationFailed || len(report.Outcome.Codes) != 1 || report.Outcome.Codes[0] != wire.CodeDuplicateID || !strings.Contains(report.Detail, "BF-1 is the import of source item BF-1 of queue:beamfall:main; its source item cannot be renamed") {
		t.Fatalf("rename of an imported ticket's source item: %+v, want DUPLICATE_ID naming BF-1's import source", report)
	}
	if storeDigest(t, repo) != before {
		t.Fatal("a refused rename wrote")
	}
	if again := runImport(t, repo, export); again.Planned != 0 {
		t.Fatalf("re-import after the refused rename planned %d records, want none", again.Planned)
	}
	pending := importMapBytes(fixture.QueueID, [3]string{"queue:beamfall:main", "BF-1", "BF-1"}, [3]string{"queue:beamfall:main", "NEW-5", "BF-5"})
	if r := amendMap(t, repo, operator(), "amend-rename-pending", wire.Sum(old), pending); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("rename of a ticket the store does not hold yet: %+v", r)
	}
	auditOK(t, repo)
}

// TestCTSV0020_ImportMapAmendRefusesUnmappedQueue: a queue that names no
// import map has nothing to amend; the amendment is refused MALFORMED with
// nothing written (adding a map to an unmapped queue is out of scope).
func TestCTSV0020_ImportMapAmendRefusesUnmappedQueue(t *testing.T) {
	t.Parallel()
	repo, _ := initialized(t)
	before := storeDigest(t, repo)
	report, err := store.ImportMapAmend(context.Background(), repo, operator(), store.ImportMapRequest{QueueID: fixture.QueueID, RequestID: "amend-unmapped", ExpectedImportMapSha256: wire.Sum([]byte("none")), ImportMap: importMapBytes(fixture.QueueID)}, now(t))
	code := wire.CodeOf(err)
	if err == nil && report != nil && len(report.Outcome.Codes) == 1 {
		code = report.Outcome.Codes[0]
	}
	if code != wire.CodeMalformed || storeDigest(t, repo) != before {
		t.Fatalf("unmapped queue: %+v %v, want MALFORMED", report, err)
	}
}

// TestCTSV0024_RollbackDistinguishesInterruptedAmendment holds the two store
// states the CTS-V0-020 rollback guidance separates, neither of which needs a
// restore. A fully applied amendment leaves only an IMPORT_PLAN receipt, a
// kind the closed receipt set held before CTS-V0-020, read by the unchanged
// journal reader, so the store audits CONSISTENT. The non-fixture writer
// stages no descriptor, so no reader's stage decoder ever meets IMPORT_PLAN.
// An amendment interrupted after its receipt is linked but before its head
// is written is completed by the redo of the next write, here an ordinary
// ticket CREATE: redo and the journal reader are the code a pre-amendment
// writer runs, unchanged by CTS-V0-020.
func TestCTSV0024_RollbackDistinguishesInterruptedAmendment(t *testing.T) {
	old := importMapBytes(fixture.QueueID, [3]string{"queue:beamfall:main", "BF-1", "BF-1"})
	repo := mappedStore(t, "ROADMAP", old)
	next := importMapBytes(fixture.QueueID, [3]string{"queue:beamfall:main", "BF-1", "BF-1"}, [3]string{"queue:beamfall:main", "BF-2", "BF-2"})
	if r := amendMap(t, repo, operator(), "amend-applied", wire.Sum(old), next); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("amend: %+v", r)
	}
	active := filepath.Join(repo.StateDir, "staging", "active.json")
	if _, err := os.Lstat(active); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("applied amendment left a staging descriptor: %v", err)
	}
	if !slices.Contains(snapshot.ReceiptKinds, "IMPORT_PLAN") {
		t.Fatal("IMPORT_PLAN is not a journaled receipt kind")
	}
	auditOK(t, repo)

	third := importMapBytes(fixture.QueueID, [3]string{"queue:beamfall:main", "BF-1", "BF-1"}, [3]string{"queue:beamfall:main", "BF-2", "BF-2"}, [3]string{"queue:beamfall:main", "BF-3", "BF-3"})
	fault := errors.New("interrupted amendment")
	restore := store.SetPublishFaultForTest(func(a transaction.Artifact) error {
		if a.Role == "HEAD" {
			return fault
		}
		return nil
	})
	report, err := store.ImportMapAmend(context.Background(), repo, operator(), store.ImportMapRequest{QueueID: fixture.QueueID, RequestID: "amend-interrupted", ExpectedImportMapSha256: wire.Sum(next), ImportMap: third}, now(t))
	restore()
	if !errors.Is(err, fault) || report == nil || report.Receipt == "" {
		t.Fatalf("interrupted amendment: %+v %v", report, err)
	}
	if _, err := os.Lstat(active); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("interrupted amendment left a staging descriptor: %v", err)
	}
	if created := mutate(t, repo, envelope("create-after", mutation.OpCreate, "", "", createPayload("after"))); created.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("write after the interrupted amendment: %+v", created)
	}
	if got, err := os.ReadFile(filepath.Join(repo.PrimaryWorktree, intent.Dir, intent.ImportMapFile)); err != nil || string(got) != string(third) {
		t.Fatalf("import map after redo: %v", err)
	}
	auditOK(t, repo)
}
