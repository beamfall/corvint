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

// amendImportMap records next as the store's import map, read as expected.
func amendImportMap(t *testing.T, repo *intent.Repository, actor mutation.Binding, id string, expected wire.Digest, next []byte) *store.Report {
	t.Helper()
	report, err := store.ImportMapUpdate(context.Background(), repo, actor, store.ImportMapRequest{QueueID: fixture.QueueID, RequestID: id, ExpectedImportMapSha256: expected, ImportMap: next}, now(t))
	if err != nil {
		t.Fatalf("import map update %s: %v", id, err)
	}
	return report
}

// importedMappedStore is a mapped store whose map names BF-1, after an
// import of BF-1 and BF-2 from queue:beamfall:main.
func importedMappedStore(t *testing.T) (*intent.Repository, []byte) {
	t.Helper()
	current := importMapBytes(fixture.QueueID, [3]string{"queue:beamfall:main", "BF-1", "BF-1"})
	repo := mappedStore(t, "ROADMAP", current)
	runImport(t, repo, importExport(importItem("BF-1", "one\n", nil), importItem("BF-2", "two\n", nil)))
	return repo, current
}

// TestCTSV0020_ImportMapAmendmentRecordsAndReplays: a store initialized from
// an import map records an amended map (a renamed source queue and an added
// entry) through one journaled IMPORT_MAP_UPDATE; the queue names the new map,
// both projections follow, ticket records are unchanged, the journal audits
// CONSISTENT, a retry replays without writing, the same request id with other
// bytes is REQUEST_ID_CONFLICT, a stale expected digest is REVISION_CONFLICT,
// and later writes still keep the (new) map immutable (CTS-V0-020, CTS-V0-022).
func TestCTSV0020_ImportMapAmendmentRecordsAndReplays(t *testing.T) {
	t.Parallel()
	repo, current := importedMappedStore(t)
	ticketsBefore := map[string][]byte{}
	for _, local := range []string{"BF-1", "BF-2"} {
		_, raw := readImported(t, repo, local)
		ticketsBefore[local] = raw
	}
	next := importMapBytes(fixture.QueueID,
		[3]string{"queue:beamfall:renamed", "BF-1", "BF-1"},
		[3]string{"queue:beamfall:renamed", "BF-2", "BF-2"})
	report := amendImportMap(t, repo, operator(), "amend-1", wire.Sum(current), next)
	if report.Outcome.Outcome != mutation.OutcomeCompleted || report.Receipt == "" {
		t.Fatalf("amendment: %+v", report)
	}
	auditOK(t, repo)
	raw, err := os.ReadFile(filepath.Join(repo.PrimaryWorktree, intent.Dir, "queue.json"))
	if err != nil {
		t.Fatal(err)
	}
	q, err := intent.DecodeQueue(raw)
	if err != nil || q.ImportMapSha256 == nil || *q.ImportMapSha256 != wire.Sum(next) {
		t.Fatalf("queue does not name the amended map: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(repo.PrimaryWorktree, intent.Dir, intent.ImportMapFile)); err != nil || string(got) != string(next) {
		t.Fatalf("import map projection: %v", err)
	}
	for local, before := range ticketsBefore {
		if _, after := readImported(t, repo, local); string(after) != string(before) {
			t.Fatalf("ticket %s changed across the amendment", local)
		}
	}
	digest := storeDigest(t, repo)
	if again := amendImportMap(t, repo, operator(), "amend-1", wire.Sum(current), next); again.Outcome.Outcome != mutation.OutcomeCompleted || !again.Outcome.Replayed || storeDigest(t, repo) != digest {
		t.Fatalf("retry did not replay: %+v", again)
	}
	other := importMapBytes(fixture.QueueID, [3]string{"queue:beamfall:other", "BF-1", "BF-1"})
	refusedWith(t, amendImportMap(t, repo, operator(), "amend-1", wire.Sum(current), other), mutation.OutcomeRequestIDConflict, wire.CodeRequestIDConflict)
	if stale := amendImportMap(t, repo, operator(), "amend-2", wire.Sum(current), other); stale.Outcome.Outcome != mutation.OutcomeRevisionConflict {
		t.Fatalf("stale expected digest: %+v", stale)
	}
	if storeDigest(t, repo) != digest {
		t.Fatal("a refused amendment wrote")
	}
	created := mutate(t, repo, envelope("create-after", mutation.OpCreate, "", "", createPayload("after")))
	if created.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("create after the amendment: %+v", created)
	}
	runImport(t, repo, importExport(importItem("BF-3", "three\n", nil)))
	auditOK(t, repo)
}

// TestCTSV0021_ImportMapAmendmentKeepsTicketIdentities: an amendment that
// would change an existing ticket identity is refused VALIDATION_FAILED
// DUPLICATE_ID naming the conflict, with nothing written: a mapped ticket
// dropped, a source item moved to another ticket, a ticket mapped twice, or a
// ticket record's own source item mapped to another ticket (CTS-V0-021).
func TestCTSV0021_ImportMapAmendmentKeepsTicketIdentities(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		entries [][3]string
		want    string
	}{
		{"dropped", nil, "ticket " + fixture.TicketID("BF-1") + " (source item BF-1 of queue:beamfall:main) is dropped"},
		{"moved", [][3]string{{"queue:beamfall:main", "BF-1", "OTHER-1"}}, "source item BF-1 of queue:beamfall:main moves from " + fixture.TicketID("BF-1")},
		{"mapped twice", [][3]string{{"queue:beamfall:main", "BF-1", "BF-1"}, {"queue:beamfall:renamed", "BF-1", "BF-1"}}, "is mapped to both"},
		{"record source elsewhere", [][3]string{{"queue:beamfall:main", "BF-1", "BF-1"}, {"queue:beamfall:main", "BF-2", "BF-9"}}, "is ticket " + fixture.TicketID("BF-2") + "'s own source"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			repo, current := importedMappedStore(t)
			before := storeDigest(t, repo)
			report := amendImportMap(t, repo, operator(), "amend", wire.Sum(current), importMapBytes(fixture.QueueID, c.entries...))
			refusedWith(t, report, mutation.OutcomeValidationFailed, wire.CodeDuplicateID)
			if !strings.Contains(report.Detail, c.want) {
				t.Fatalf("refusal %q does not name %q", report.Detail, c.want)
			}
			if storeDigest(t, repo) != before {
				t.Fatal("a refused amendment wrote")
			}
		})
	}
}

// TestCTSV0022_ImportMapAmendmentFailsClosed: only an OWNER amends, and only
// a queue that already names a journaled map; each refusal writes nothing
// (CTS-V0-022).
func TestCTSV0022_ImportMapAmendmentFailsClosed(t *testing.T) {
	t.Parallel()
	repo, current := importedMappedStore(t)
	before := storeDigest(t, repo)
	next := importMapBytes(fixture.QueueID, [3]string{"queue:beamfall:renamed", "BF-1", "BF-1"})
	if r := amendImportMap(t, repo, mutation.Binding{ID: "tester", Role: "OPERATOR"}, "amend-operator", wire.Sum(current), next); r.Outcome.Outcome != mutation.OutcomeUnauthorized {
		t.Fatalf("OPERATOR amendment: %+v", r)
	}
	if storeDigest(t, repo) != before {
		t.Fatal("a refused amendment wrote")
	}
	plain, _ := initialized(t)
	plainBefore := storeDigest(t, plain)
	refusedWith(t, amendImportMap(t, plain, operator(), "amend-unmapped", wire.Sum(current), next), mutation.OutcomeBlocked, wire.CodeUnsupported)
	if storeDigest(t, plain) != plainBefore {
		t.Fatal("an amendment of an unmapped queue wrote")
	}
}
