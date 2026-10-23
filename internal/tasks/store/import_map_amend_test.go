package store_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

const (
	amendSource  = "queue:beamfall:main"
	amendRenamed = "queue:renamed:main"
	amendOther   = "queue:other:main"
)

// amendedStore initializes a mapped ROADMAP queue whose map names BF-1 and
// imports BF-1 (mapped) and BF-2 (not named by the map).
func amendedStore(t *testing.T) (*intent.Repository, []byte) {
	t.Helper()
	current := importMapBytes(fixture.QueueID, [3]string{amendSource, "BF-1", "BF-1"})
	repo := mappedStore(t, "ROADMAP", current)
	runImport(t, repo, importExport(importItem("BF-1", "one\n", nil), importItem("BF-2", "two\n", nil)))
	return repo, current
}

func amendRequest(id string, current, amended []byte) store.ImportMapRequest {
	return store.ImportMapRequest{QueueID: fixture.QueueID, RequestID: id, ExpectedImportMapSha256: wire.Sum(current), ImportMap: amended}
}

// ticketBytes reads every ticket projection.
func ticketBytes(t *testing.T, repo *intent.Repository) map[string]string {
	t.Helper()
	dir := filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, e := range entries {
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		out[e.Name()] = string(raw)
	}
	return out
}

// TestCTSV0021_AmendRecordsAnAmendedMapAndReplays proves a mapped store
// records an amended map through one OWNER IMPORT_MAP_AMEND receipt that
// posts the map and the queue binding it; the amendment renames a source
// queue, adds a source queue and names a stored ticket by its own source;
// every stored ticket keeps its bytes, the journal audits CONSISTENT, and an
// identical retry replays while a different payload under the id conflicts.
func TestCTSV0021_AmendRecordsAnAmendedMapAndReplays(t *testing.T) {
	t.Parallel()
	repo, current := amendedStore(t)
	amended := importMapBytes(fixture.QueueID,
		[3]string{amendRenamed, "BF-1", "BF-1"},
		[3]string{amendSource, "BF-2", "BF-2"},
		[3]string{amendOther, "X-1", "BF-9"})
	tickets := ticketBytes(t, repo)
	report, err := store.ImportMapAmend(context.Background(), repo, operator(), amendRequest("amend-1", current, amended), now(t))
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted || report.Receipt == "" {
		t.Fatalf("amend: %+v %v", report, err)
	}
	raw, err := os.ReadFile(filepath.Join(repo.StateDir, "receipts", report.Receipt))
	if err != nil {
		t.Fatal(err)
	}
	rc, err := snapshot.DecodeReceipt(raw)
	if err != nil || rc.Kind != "IMPORT_MAP_AMEND" || len(rc.Post) != 3 {
		t.Fatalf("receipt kind %q with %d posts: %v", rc.Kind, len(rc.Post), err)
	}
	var queue, importMap *wire.Digest
	for _, e := range rc.Post {
		switch e.Path {
		case "intent/queue.json":
			queue = e.Sha256
		case "intent/import-map.json":
			importMap = e.Sha256
		}
	}
	if queue == nil || importMap == nil || *importMap != wire.Sum(amended) {
		t.Fatalf("receipt posts: queue %v map %v", queue, importMap)
	}
	projected, err := os.ReadFile(filepath.Join(repo.PrimaryWorktree, intent.Dir, intent.ImportMapFile))
	if err != nil || string(projected) != string(amended) {
		t.Fatalf("map projection not replaced: %v", err)
	}
	qraw, err := os.ReadFile(filepath.Join(repo.PrimaryWorktree, intent.Dir, "queue.json"))
	if err != nil || wire.Sum(qraw) != *queue {
		t.Fatalf("queue projection differs from the receipt: %v", err)
	}
	q, err := intent.DecodeQueue(qraw)
	if err != nil || q.ImportMapSha256 == nil || *q.ImportMapSha256 != wire.Sum(amended) || q.CanonicalWriter != "ROADMAP" {
		t.Fatalf("queue after amendment: %+v %v", q, err)
	}
	after := ticketBytes(t, repo)
	if len(after) != len(tickets) {
		t.Fatalf("tickets %d, want %d", len(after), len(tickets))
	}
	for name, was := range tickets {
		if after[name] != was {
			t.Fatalf("ticket %s changed across the amendment", name)
		}
	}
	auditOK(t, repo)
	// The queue still takes writes and an import under the amended map.
	mutate(t, repo, envelope("after-amend", mutation.OpCreate, "", "", createPayload("after amend")))
	auditOK(t, repo)

	before := storeDigest(t, repo)
	replay, err := store.ImportMapAmend(context.Background(), repo, operator(), amendRequest("amend-1", current, amended), now(t))
	if err != nil || replay.Kind != "Replay" || !replay.Outcome.Replayed || replay.Receipt != "" || storeDigest(t, repo) != before {
		t.Fatalf("replay: %+v %v", replay, err)
	}
	other := importMapBytes(fixture.QueueID, [3]string{amendRenamed, "BF-1", "BF-1"})
	conflict, err := store.ImportMapAmend(context.Background(), repo, operator(), amendRequest("amend-1", amended, other), now(t))
	if err != nil || !conflict.Outcome.HasCode(wire.CodeRequestIDConflict) || storeDigest(t, repo) != before {
		t.Fatalf("conflicting reuse of the request id: %+v %v", conflict, err)
	}
}

// TestCTSV0022_AmendRefusesIdentityConflicts proves an amendment that drops a
// mapped ticket, remaps it to another source item, reassigns a source item the
// map assigns, or names a stored ticket by a source it does not hold is
// refused DUPLICATE_ID, naming the conflict, with nothing written.
func TestCTSV0022_AmendRefusesIdentityConflicts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		entries [][3]string
		want    string
	}{
		{"dropped", nil, "ticket " + fixture.TicketID("BF-1") + " is dropped from the map"},
		{"remapped", [][3]string{{amendSource, "BF-7", "BF-1"}}, "ticket " + fixture.TicketID("BF-1") + " is remapped"},
		{"reassigned", [][3]string{{amendSource, "BF-1", "BF-5"}, {amendRenamed, "BF-1", "BF-1"}}, "source item BF-1 of " + amendSource + " is reassigned from " + fixture.TicketID("BF-1") + " to " + fixture.TicketID("BF-5")},
		{"stored ticket with another source", [][3]string{{amendSource, "BF-1", "BF-1"}, {amendOther, "Z-1", "BF-2"}}, "ticket " + fixture.TicketID("BF-2") + " exists with another source"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			repo, current := amendedStore(t)
			before := storeDigest(t, repo)
			report, err := store.ImportMapAmend(context.Background(), repo, operator(), amendRequest("amend", current, importMapBytes(fixture.QueueID, c.entries...)), now(t))
			if err != nil {
				t.Fatal(err)
			}
			if report.Outcome.Outcome != mutation.OutcomeValidationFailed || !report.Outcome.HasCode(wire.CodeDuplicateID) || !strings.Contains(report.Detail, c.want) {
				t.Fatalf("outcome %+v detail %q, want DUPLICATE_ID naming %q", report.Outcome, report.Detail, c.want)
			}
			if storeDigest(t, repo) != before {
				t.Fatal("a conflicting amendment wrote")
			}
		})
	}
}

// TestCTSV0023_AmendRefusalsFailClosed proves the amendment writes nothing
// unless every precondition holds: a stale expected digest, a non-owner, a
// barrier, a NATIVE-written queue, an unmapped queue, an unchanged map and a
// map of another queue each refuse with their own typed outcome.
func TestCTSV0023_AmendRefusalsFailClosed(t *testing.T) {
	t.Parallel()
	good := func(current []byte) []byte {
		return importMapBytes(fixture.QueueID, [3]string{amendSource, "BF-1", "BF-1"}, [3]string{amendOther, "X-1", "BF-9"})
	}
	cases := []struct {
		name    string
		outcome string
		code    string
	}{
		{"stale expected digest", mutation.OutcomeRevisionConflict, ""},
		{"operator role", mutation.OutcomeUnauthorized, ""},
		{"barrier", mutation.OutcomeBlocked, wire.CodePaused},
		{"native writer", mutation.OutcomeBlocked, wire.CodeUnsupported},
		{"unmapped queue", mutation.OutcomeValidationFailed, wire.CodeMalformed},
		{"unchanged map", mutation.OutcomeBlocked, ""},
		{"another queue", "", wire.CodeMalformed},
		{"intent branch", mutation.OutcomeBlocked, wire.CodeIntentBranchMismatch},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var repo *intent.Repository
			current := importMapBytes(fixture.QueueID, [3]string{amendSource, "BF-1", "BF-1"})
			switch c.name {
			case "native writer":
				repo = mappedStore(t, "NATIVE", current)
			case "unmapped queue":
				repo, _ = initialized(t)
			default:
				repo = mappedStore(t, "ROADMAP", current)
			}
			request := amendRequest("amend", current, good(current))
			actor := operator()
			switch c.name {
			case "stale expected digest":
				request.ExpectedImportMapSha256 = wire.Sum([]byte("another map"))
			case "operator role":
				actor.Role = "OPERATOR"
			case "barrier":
				if r := changeBarrier(t, repo, transaction.Pause, "pause"); r.Outcome.Outcome != mutation.OutcomeCompleted {
					t.Fatalf("pause: %+v", r)
				}
			case "unmapped queue":
				request.ExpectedImportMapSha256 = wire.Sum(current)
			case "unchanged map":
				request.ImportMap = current
			case "intent branch":
				fixture.Write(t, filepath.Join(repo.CommonDir, "HEAD"), []byte("ref: refs/heads/elsewhere\n"))
			case "another queue":
				request.ImportMap = importMapBytes("queue:other:main", [3]string{amendSource, "BF-1", "BF-1"})
			}
			before := storeDigest(t, repo)
			report, err := store.ImportMapAmend(context.Background(), repo, actor, request, now(t))
			outcome, code := "", wire.CodeOf(err)
			if err == nil {
				outcome = report.Outcome.Outcome
				if len(report.Outcome.Codes) > 0 {
					code = report.Outcome.Codes[0]
				}
			}
			if outcome != c.outcome || code != c.code && !(c.code == "" && code == "") {
				t.Fatalf("outcome %q code %q (%+v, %v), want %q %q", outcome, code, report, err, c.outcome, c.code)
			}
			if storeDigest(t, repo) != before {
				t.Fatal("a refused amendment wrote")
			}
		})
	}
}

// TestCTSV0021_InterruptedAmendIsRedone proves an amendment interrupted after
// its receipt is published and before the head names it settles on the next
// request: the same request replays as committed, the store audits CONSISTENT
// and the amended map and its queue binding are in place. It installs the
// package's publish fault, so it does not run in parallel.
func TestCTSV0021_InterruptedAmendIsRedone(t *testing.T) {
	repo, current := amendedStore(t)
	amended := importMapBytes(fixture.QueueID,
		[3]string{amendRenamed, "BF-1", "BF-1"},
		[3]string{amendOther, "X-1", "BF-9"})
	fault := errors.New("injected head fault")
	restore := store.SetPublishFaultForTest(func(a transaction.Artifact) error {
		if a.Role == "HEAD" {
			return fault
		}
		return nil
	})
	report, err := store.ImportMapAmend(context.Background(), repo, operator(), amendRequest("amend-redo", current, amended), now(t))
	restore()
	if !errors.Is(err, fault) || report.Receipt == "" {
		t.Fatalf("interrupted amend: %+v %v", report, err)
	}
	again, err := store.ImportMapAmend(context.Background(), repo, operator(), amendRequest("amend-redo", current, amended), now(t))
	if err != nil || !again.Redone || again.Kind != "Replay" {
		t.Fatalf("redo: %+v %v", again, err)
	}
	auditOK(t, repo)
	q, err := os.ReadFile(filepath.Join(repo.PrimaryWorktree, intent.Dir, "queue.json"))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := intent.DecodeQueue(q)
	if err != nil || decoded.ImportMapSha256 == nil || *decoded.ImportMapSha256 != wire.Sum(amended) {
		t.Fatalf("queue after redo: %+v %v", decoded, err)
	}
}
