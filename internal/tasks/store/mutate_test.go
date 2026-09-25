package store_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const issued = "2026-09-07T12:00:00Z"

func obj(kv ...any) wire.Value {
	o := wire.NewObject()
	for i := 0; i < len(kv); i += 2 {
		o.Set(kv[i].(string), kv[i+1].(wire.Value))
	}
	return wire.ObjectValue(o)
}

func str(s string) wire.Value { return wire.String(s) }

// envelope builds one canonical taskman-mutation/0. targetId and
// expectedRevision are null exactly for CREATE.
func envelope(requestID, operation, target, expected string, payload wire.Value) []byte {
	tid, rev := wire.Null(), wire.Null()
	if target != "" {
		tid, rev = str(target), str(expected)
	}
	return wire.EncodeFile(obj(
		"profile", str(mutation.Profile),
		"requestId", str(requestID),
		"actor", obj("id", str("tester"), "role", str("OWNER")),
		"queueId", str(fixture.QueueID),
		"targetId", tid,
		"expectedRevision", rev,
		"operation", str(operation),
		"payload", payload,
		"issuedAt", str(issued),
	))
}

// createPayload is the closed §3.3 CREATE payload with an explicit title.
func createPayload(title string) wire.Value {
	return obj(
		"acceptanceCriteria", wire.Strings([]string{"it exists"}),
		"body", wire.Null(),
		"capabilities", wire.Strings(nil),
		"dependencies", wire.Array(),
		"dueDate", wire.Null(),
		"effects", obj("coverage", str("QUALIFIED"), "externalUnbounded", wire.Bool(false), "resources", wire.Array(), "touchPaths", wire.Strings(nil)),
		"estimateMinutes", wire.Null(),
		"executionClass", str("AUTONOMOUS"),
		"kind", str("FEATURE"),
		"labels", wire.Strings(nil),
		"milestone", wire.Null(),
		"order", str("0"),
		"owner", wire.Null(),
		"priority", str("P2"),
		"requiredGates", wire.Strings(nil),
		"requirementRefs", wire.Strings(nil),
		"source", obj("kind", str("NATIVE"), "sourceItemId", wire.Null(), "sourceQueueId", str(fixture.QueueID), "sourceRevisionSha256", wire.Null()),
		"supersededBy", wire.Null(),
		"supersedes", wire.Null(),
		"title", str(title),
	)
}

func mutate(t *testing.T, repo *intent.Repository, env []byte) *store.Report {
	t.Helper()
	report, err := store.Mutate(context.Background(), repo, operator(), env, now(t))
	if err != nil {
		t.Fatalf("mutate: %v", err)
	}
	return report
}

// storeDigest hashes every file of the intent store and the state dir, so a
// refusal can be shown to have written nothing at all.
func storeDigest(t *testing.T, repo *intent.Repository) wire.Digest {
	t.Helper()
	var paths []string
	for _, root := range []string{filepath.Join(repo.PrimaryWorktree, intent.Dir), repo.StateDir} {
		err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				paths = append(paths, p)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	sort.Strings(paths)
	var all []byte
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		all = append(all, p...)
		all = append(all, raw...)
	}
	return wire.Sum(all)
}

// TestTMV0005_AS02_CreateCommitsATicket is the TCP-02b wiring itself: a
// CREATE reaches the journal, publishes the projection and allocates a
// serial, and the ticket is then readable from the intent store.
func TestTMV0005_AS02_CreateCommitsATicket(t *testing.T) {
	repo, _ := initialized(t)
	report := mutate(t, repo, envelope("req-create", mutation.OpCreate, "", "", createPayload("First ticket")))
	if report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("outcome = %s (%v) %s", report.Outcome.Outcome, report.Outcome.Codes, report.Detail)
	}
	if report.Receipt != "000000000002.json" {
		t.Errorf("receipt = %q, want 000000000002.json", report.Receipt)
	}
	if report.Ticket == "" {
		t.Fatal("the report names no ticket, so a CREATE cannot learn its allocated id")
	}
	loaded, err := intent.Load(repo.PrimaryWorktree)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	record, ok := loaded.Inventory.Get(report.Ticket)
	if !ok {
		t.Fatalf("%s is not in the intent store after its receipt committed", report.Ticket)
	}
	if record.Title != "First ticket" || record.Revision != "1" {
		t.Errorf("record = %q rev %s, want \"First ticket\" rev 1", record.Title, record.Revision)
	}
	original, err := intent.DecodeQueue(fixture.QueueBytes())
	if err != nil {
		t.Fatalf("decode fixture queue: %v", err)
	}
	if loaded.Queue.NextSerial == original.NextSerial {
		t.Errorf("nextSerial is still %s, so the allocation was not published", original.NextSerial)
	}
}

// TestTMV0005_AS02_RefineChainsFromTheCommittedRevision covers the second
// half of AS-02: a refine names the revision the create produced and the
// post record advances exactly one revision.
func TestTMV0005_AS02_RefineChainsFromTheCommittedRevision(t *testing.T) {
	repo, _ := initialized(t)
	created := mutate(t, repo, envelope("req-create", mutation.OpCreate, "", "", createPayload("First ticket")))
	refined := mutate(t, repo, envelope("req-refine", mutation.OpRefine, created.Ticket, "1", obj("title", str("Refined"))))
	if refined.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("refine outcome = %s (%v) %s", refined.Outcome.Outcome, refined.Outcome.Codes, refined.Detail)
	}
	if refined.Outcome.ResultingRevision == nil || *refined.Outcome.ResultingRevision != "2" {
		t.Errorf("resulting revision = %v, want 2", refined.Outcome.ResultingRevision)
	}
}

// TestTMV0005_AS02_StaleExpectedRevisionLeavesTheStoreByteIdentical is the
// AS-02 conflict: a mutation naming a superseded revision is refused and
// nothing anywhere is written, not even a receipt.
func TestTMV0005_AS02_StaleExpectedRevisionLeavesTheStoreByteIdentical(t *testing.T) {
	repo, _ := initialized(t)
	created := mutate(t, repo, envelope("req-create", mutation.OpCreate, "", "", createPayload("First ticket")))
	mutate(t, repo, envelope("req-refine", mutation.OpRefine, created.Ticket, "1", obj("title", str("Refined"))))

	before := storeDigest(t, repo)
	stale := mutate(t, repo, envelope("req-stale", mutation.OpRefine, created.Ticket, "1", obj("title", str("Stale"))))
	if stale.Outcome.Outcome == mutation.OutcomeCompleted {
		t.Fatal("a stale expectedRevision was accepted")
	}
	if stale.Receipt != "" {
		t.Errorf("a refused mutation wrote receipt %q", stale.Receipt)
	}
	if after := storeDigest(t, repo); after != before {
		t.Error("the store changed although the mutation was refused")
	}
}

// TestTMV0006_AS03_IdenticalRetryReplays is the first half of AS-03 against
// the real request index: the same envelope bytes replay the original
// outcome and write no second receipt.
func TestTMV0006_AS03_IdenticalRetryReplays(t *testing.T) {
	repo, _ := initialized(t)
	env := envelope("req-create", mutation.OpCreate, "", "", createPayload("First ticket"))
	first := mutate(t, repo, env)
	before := storeDigest(t, repo)

	again := mutate(t, repo, env)
	if !again.Outcome.Replayed {
		t.Fatalf("retry was not replayed: kind %s outcome %s", again.Kind, again.Outcome.Outcome)
	}
	if again.Receipt != "" {
		t.Errorf("a replay wrote receipt %q", again.Receipt)
	}
	if *again.Outcome.ResultingRevision != *first.Outcome.ResultingRevision {
		t.Error("the replayed outcome differs from the original")
	}
	if after := storeDigest(t, repo); after != before {
		t.Error("a replay changed the store")
	}
}

// TestTMV0006_AS03_SameRequestIDDifferentBytesConflicts is the second half:
// the request id is the idempotency key, so reusing it for other bytes is a
// conflict rather than a second commit.
func TestTMV0006_AS03_SameRequestIDDifferentBytesConflicts(t *testing.T) {
	repo, _ := initialized(t)
	mutate(t, repo, envelope("req-create", mutation.OpCreate, "", "", createPayload("First ticket")))
	before := storeDigest(t, repo)

	other := mutate(t, repo, envelope("req-create", mutation.OpCreate, "", "", createPayload("Different ticket")))
	if !other.Outcome.HasCode(wire.CodeRequestIDConflict) {
		t.Fatalf("codes = %v, want %s", other.Outcome.Codes, wire.CodeRequestIDConflict)
	}
	if after := storeDigest(t, repo); after != before {
		t.Error("a conflicting request changed the store")
	}
}

// TestTMV0004_AS05_HoldsArchiveAndRestore runs the AS-05 sequence against
// the real journal and checks that each step carries its own §3.1 receipt
// kind: a tombstone is an ARCHIVE, and undoing it is a RESTORE.
func TestTMV0004_AS05_HoldsArchiveAndRestore(t *testing.T) {
	repo, _ := initialized(t)
	created := mutate(t, repo, envelope("req-create", mutation.OpCreate, "", "", createPayload("First ticket")))
	id := created.Ticket

	steps := []struct {
		request, operation, expected string
		payload                      wire.Value
	}{
		{"req-hold", mutation.OpHold, "1", obj("holdId", str("blocked"), "reason", str("waiting"))},
		{"req-release", mutation.OpReleaseHold, "2", obj("holdId", str("blocked"))},
		{"req-archive", mutation.OpArchive, "3", obj("reason", str("superseded"))},
		{"req-restore", mutation.OpRestore, "4", obj("reason", str("still needed"))},
	}
	for _, step := range steps {
		report := mutate(t, repo, envelope(step.request, step.operation, id, step.expected, step.payload))
		if report.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("%s: outcome %s (%v) %s", step.operation, report.Outcome.Outcome, report.Outcome.Codes, report.Detail)
		}
	}
	loaded, err := intent.Load(repo.PrimaryWorktree)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	record, ok := loaded.Inventory.Get(id)
	if !ok {
		t.Fatalf("%s was not retained through archive and restore", id)
	}
	if record.ArchivedFrom != nil {
		t.Errorf("archivedFrom = %v after a restore, want null", record.ArchivedFrom)
	}
	if record.Revision != "5" {
		t.Errorf("revision = %s after four mutations of a created record, want 5", record.Revision)
	}
}

// TestTMV0009_AS11_MutationLeavesNoUnassignedStageSlot checks the §5.6
// cleanup for a transaction that both links and renames: a slot consumed by
// a rename is gone, and every other slot it used is removed.
func TestTMV0009_AS11_MutationLeavesNoUnassignedStageSlot(t *testing.T) {
	repo, _ := initialized(t)
	mutate(t, repo, envelope("req-create", mutation.OpCreate, "", "", createPayload("First ticket")))
	entries, err := os.ReadDir(filepath.Join(repo.StateDir, "staging"))
	if err != nil {
		t.Fatalf("staging: %v", err)
	}
	if len(entries) != 0 {
		names := []string{}
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("staging still holds %v", names)
	}
}

// TestTMV0009_AS11_RedoCompletesAPendingReceipt covers §5.2 crash point C2:
// a receipt was linked in — the commit point passed — but the process died
// before its post files and head were written. The next transaction must
// finish it, because the receipt is a self-contained redo record.
func TestTMV0009_AS11_RedoCompletesAPendingReceipt(t *testing.T) {
	repo, _ := initialized(t)
	head := filepath.Join(repo.StateDir, "head.json")
	genesis, err := os.ReadFile(head)
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	created := mutate(t, repo, envelope("req-create", mutation.OpCreate, "", "", createPayload("First ticket")))
	local := created.Ticket[len(created.Ticket)-7:]
	projection := filepath.Join(repo.PrimaryWorktree, intent.Dir, intent.TicketsDir, local+".json")

	// Rewind to the instant after the receipt was linked in: the receipt
	// stays, its projection and the advanced head do not.
	if err := os.WriteFile(head, genesis, 0o600); err != nil {
		t.Fatalf("rewind head: %v", err)
	}
	if err := os.Remove(projection); err != nil {
		t.Fatalf("rewind projection: %v", err)
	}

	next := mutate(t, repo, envelope("req-second", mutation.OpCreate, "", "", createPayload("Second ticket")))
	if !next.Redone {
		t.Error("the pending receipt was not redone")
	}
	if next.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("outcome after redo = %s (%v) %s", next.Outcome.Outcome, next.Outcome.Codes, next.Detail)
	}
	if _, err := os.Stat(projection); err != nil {
		t.Errorf("redo did not republish the pending projection: %v", err)
	}
	loaded, err := intent.Load(repo.PrimaryWorktree)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if _, ok := loaded.Inventory.Get(created.Ticket); !ok {
		t.Errorf("%s was lost by the redo", created.Ticket)
	}
	if _, ok := loaded.Inventory.Get(next.Ticket); !ok {
		t.Errorf("%s was not committed after the redo", next.Ticket)
	}
}

// TestTMV0009_AS35_RedoDoesNotOverwriteAnEditedProjection is the third-value
// half of the redo rule: a projection someone edited by hand is reported as
// diverged, never silently replaced by the pending receipt's bytes.
func TestTMV0009_AS35_RedoDoesNotOverwriteAnEditedProjection(t *testing.T) {
	repo, _ := initialized(t)
	head := filepath.Join(repo.StateDir, "head.json")
	genesis, err := os.ReadFile(head)
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	created := mutate(t, repo, envelope("req-create", mutation.OpCreate, "", "", createPayload("First ticket")))
	local := created.Ticket[len(created.Ticket)-7:]
	projection := filepath.Join(repo.PrimaryWorktree, intent.Dir, intent.TicketsDir, local+".json")

	if err := os.WriteFile(head, genesis, 0o600); err != nil {
		t.Fatalf("rewind head: %v", err)
	}
	edited := []byte("{\"profile\":\"taskman-ticket/0\"}\n")
	if err := os.WriteFile(projection, edited, 0o600); err != nil {
		t.Fatalf("edit projection: %v", err)
	}

	_, err = store.Mutate(context.Background(), repo, operator(), envelope("req-second", mutation.OpCreate, "", "", createPayload("Second ticket")), now(t))
	if err == nil {
		t.Fatal("redo accepted a projection holding neither the pre nor the post state")
	}
	if wire.CodeOf(err) != wire.CodeIntentDiverged {
		t.Errorf("code = %s, want %s", wire.CodeOf(err), wire.CodeIntentDiverged)
	}
	after, readErr := os.ReadFile(projection)
	if readErr != nil || string(after) != string(edited) {
		t.Error("the edited projection was overwritten")
	}
}
