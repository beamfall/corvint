package store_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/release"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// importMapBytes is a taskman-import-map/0 for queueID whose entries map
// each [sourceQueueId, sourceItemId, local ticket] triple, in sorted order.
func importMapBytes(queueID string, entries ...[3]string) []byte {
	values := []wire.Value{}
	for _, e := range entries {
		values = append(values, obj("appliedSeq", str("1"), "sourceItemId", str(e[1]), "sourceQueueId", str(e[0]), "sourceRevisionSha256", wire.Null(), "ticketId", str(fixture.TicketID(e[2]))))
	}
	return wire.EncodeFile(obj("entries", wire.Array(values...), "profile", str(intent.ProfileImportMap), "queueId", str(queueID)))
}

// mappedRepo writes a queue (writer and fixture as given) whose importMapSha256 is
// digest, and the import map bytes when not nil, without initializing.
func mappedRepo(t *testing.T, writer string, isFixture bool, digest wire.Digest, importMap []byte) *intent.Repository {
	t.Helper()
	repo := fixture.TempRepo(t)
	q := fixture.QueueValue()
	q.Obj.Set("fixture", wire.Bool(isFixture))
	q.Obj.Set("canonicalWriter", str(writer))
	q.Obj.Set("importMapSha256", str(string(digest)))
	fixture.Write(t, filepath.Join(repo.IntentDir, "queue.json"), wire.EncodeFile(q))
	fixture.Write(t, filepath.Join(repo.IntentDir, "policy.json"), fixture.PolicyBytes())
	if importMap != nil {
		fixture.Write(t, filepath.Join(repo.IntentDir, intent.ImportMapFile), importMap)
	}
	resolved, err := intent.Resolve(repo.Root)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	return resolved
}

// mappedStore initializes a non-fixture queue that names importMap.
func mappedStore(t *testing.T, writer string, importMap []byte) *intent.Repository {
	t.Helper()
	repo := mappedRepo(t, writer, false, wire.Sum(importMap), importMap)
	report, err := store.Init(context.Background(), repo, operator(), "req-init", now(t))
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("init: %+v %v", report, err)
	}
	return repo
}

// TestCTSV0014_ImportMappedQueueTakesEveryWrite: a non-fixture queue whose
// importMapSha256 names its import map initializes with the map journaled at
// genesis, then commits a ticket create and edit, an import that honours the
// map, pause, unpause, a policy update and the writer cutover; the queue keeps
// its digest, the map projection is unchanged, and the journal audits
// CONSISTENT (CTS-V0-014, CTS-V0-015, CTS-V0-016, CTS-V0-017).
func TestCTSV0014_ImportMappedQueueTakesEveryWrite(t *testing.T) {
	t.Parallel()
	importMap := importMapBytes(fixture.QueueID, [3]string{"queue:beamfall:main", "BF-1", "BF-1"})
	repo := mappedStore(t, "ROADMAP", importMap)
	auditOK(t, repo)
	created := mutate(t, repo, envelope("create-a", mutation.OpCreate, "", "", createPayload("a")))
	if created.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("create: %+v", created)
	}
	edited := mutate(t, repo, envelope("edit-a", mutation.OpPrioritize, created.Ticket, "1", obj("priority", str("P1"), "order", str("0"))))
	if edited.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("edit: %+v", edited)
	}
	runImport(t, repo, importExport(importItem("BF-1", "one\n", nil), importItem("BF-2", "two\n", nil)))
	for _, op := range []string{transaction.Pause, transaction.Unpause} {
		if report := changeBarrier(t, repo, op, strings.ToLower(op)); report.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("%s: %+v", op, report)
		}
	}
	policy, err := store.PolicyUpdate(context.Background(), repo, operator(), policyRequest("policy-2", "1", policyVersion("2")), now(t))
	if err != nil || policy.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy: %+v %v", policy, err)
	}
	if report := cutover(t, repo, operator(), "decision-1"); report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("cutover: %+v", report)
	}
	auditOK(t, repo)
	raw, err := os.ReadFile(filepath.Join(repo.PrimaryWorktree, intent.Dir, "queue.json"))
	if err != nil {
		t.Fatal(err)
	}
	q, err := intent.DecodeQueue(raw)
	if err != nil || q.ImportMapSha256 == nil || *q.ImportMapSha256 != wire.Sum(importMap) {
		t.Fatalf("queue lost its import map digest: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(repo.PrimaryWorktree, intent.Dir, intent.ImportMapFile)); err != nil || string(got) != string(importMap) {
		t.Fatalf("import map projection changed: %v", err)
	}
}

// TestCTSV0014_ImportMappedQueueLeasesAndReleases: a mapped non-fixture
// queue takes release intent, the execution cutover, and a claim, heartbeat
// and lease release, and audits CONSISTENT (CTS-V0-014, CTS-V0-015).
func TestCTSV0014_ImportMappedQueueLeasesAndReleases(t *testing.T) {
	t.Parallel()
	s := leaseStoreOn(t, mappedStore(t, "NATIVE", importMapBytes(fixture.QueueID)))
	applyRelease(t, s.repo, releaseRequest("create", release.OpCreate, "", "Mapped"))
	if r := s.executionCutover(t, operator(), "qualified", suiteRun()); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("execution cutover: %+v", r)
	}
	id := s.planned(t, "one", "P1", "src/")
	claimed := s.lease(t, "claim-1", claimOf(id), 0, nil)
	if claimed.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("claim: %+v", claimed)
	}
	if r := s.lease(t, "beat-1", heartbeatOf(claimed), 1, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("heartbeat: %+v", r)
	}
	if r := s.lease(t, "release-1", releaseOf(claimed), 2, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("lease release: %+v", r)
	}
	auditOK(t, s.repo)
}

// TestCTSV0015_ImportMapBindingRefusesInit: INIT refuses VALIDATION_FAILED
// MALFORMED, creating no store, when the named map is absent, differs from
// importMapSha256, names another queue, or belongs to a fixture queue.
func TestCTSV0015_ImportMapBindingRefusesInit(t *testing.T) {
	t.Parallel()
	good := importMapBytes(fixture.QueueID)
	other := importMapBytes("queue:other:main")
	cases := []struct {
		name      string
		isFixture bool
		digest    wire.Digest
		importMap []byte
	}{
		{"absent", false, wire.Sum(good), nil},
		{"digest mismatch", false, wire.Sum([]byte("map")), good},
		{"other queue", false, wire.Sum(other), other},
		{"fixture queue", true, wire.Sum(good), good},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			repo := mappedRepo(t, "ROADMAP", c.isFixture, c.digest, c.importMap)
			report, err := store.Init(context.Background(), repo, operator(), "req-init", now(t))
			if err != nil {
				t.Fatal(err)
			}
			refusedWith(t, report, mutation.OutcomeValidationFailed, wire.CodeMalformed)
			if _, err := os.Lstat(repo.StateDir); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("refused init created state: %v", err)
			}
		})
	}
}

// TestCTSV0017_ImportRefusesMapIdentityConflicts: a mapped source item
// imports only to its mapped ticket, and a mapped ticket takes no other
// source item; either conflict refuses DUPLICATE_ID with nothing written.
func TestCTSV0017_ImportRefusesMapIdentityConflicts(t *testing.T) {
	t.Parallel()
	cases := map[string][3]string{
		"source mapped elsewhere": {"queue:beamfall:main", "BF-1", "OLD-1"},
		"ticket mapped elsewhere": {"queue:other:main", "X-9", "BF-1"},
	}
	for name, entry := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repo := mappedStore(t, "ROADMAP", importMapBytes(fixture.QueueID, entry))
			before := storeDigest(t, repo)
			report, err := store.Import(context.Background(), repo, operator(), fixture.QueueID, importExport(importItem("BF-1", "one\n", nil)), now(t))
			code := wire.CodeOf(err)
			if err == nil && report != nil && len(report.Batches) == 1 && len(report.Batches[0].Outcome.Codes) == 1 {
				code = report.Batches[0].Outcome.Codes[0]
			}
			if code != wire.CodeDuplicateID || storeDigest(t, repo) != before {
				t.Fatalf("code %q, want %q (%+v %v)", code, wire.CodeDuplicateID, report, err)
			}
		})
	}
}

// TestCTSV0018_ImportPlanNamesEveryMapConflict: import checks its whole plan
// against the import map before the first write, refusing DUPLICATE_ID with
// every conflicting source item named and the store digest unchanged; the
// per-batch check alone stopped at the first conflict (CTS-V0-018, V1-2401).
func TestCTSV0018_ImportPlanNamesEveryMapConflict(t *testing.T) {
	t.Parallel()
	repo := mappedStore(t, "ROADMAP", importMapBytes(fixture.QueueID,
		[3]string{"queue:beamfall:main", "BF-1", "OLD-1"},
		[3]string{"queue:other:main", "X-9", "BF-3"}))
	before := storeDigest(t, repo)
	export := importExport(importItem("BF-1", "one\n", nil), importItem("BF-2", "two\n", nil), importItem("BF-3", "three\n", nil))
	report, err := store.Import(context.Background(), repo, operator(), fixture.QueueID, export, now(t))
	if wire.CodeOf(err) != wire.CodeDuplicateID {
		t.Fatalf("code %q, want %q (%+v %v)", wire.CodeOf(err), wire.CodeDuplicateID, report, err)
	}
	for _, want := range []string{"2 import-map conflicts", "source item BF-1 of queue:beamfall:main", "source item BF-3 of queue:beamfall:main"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("refusal %q does not name %q", err, want)
		}
	}
	if len(report.Batches) != 0 || storeDigest(t, repo) != before {
		t.Fatalf("a conflicting plan wrote: %+v", report)
	}
}
