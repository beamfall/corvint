package store_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
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
