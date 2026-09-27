package store_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func cutover(t *testing.T, repo *intent.Repository, actor mutation.Binding, decision string) *store.Report {
	t.Helper()
	report, err := store.Cutover(context.Background(), repo, actor, fixture.QueueID, decision, now(t))
	if err != nil {
		t.Fatalf("cutover: %v", err)
	}
	return report
}

func codes(t *testing.T, repo *intent.Repository, id string) []string {
	t.Helper()
	st, err := intent.Load(repo.PrimaryWorktree)
	if err != nil {
		t.Fatal(err)
	}
	view, ok := st.Inventory.View(id, st.Context())
	if !ok {
		t.Fatalf("no view of %s", id)
	}
	out := []string{}
	for _, b := range view.Blockers {
		out = append(out, b.Code)
	}
	return out
}

func has(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// TestCALV0004_CutoverSwitchesWriterInOneReceipt: one AUTHORITY_SWITCH
// receipt named by the decision makes the queue NATIVE with a CUTOVER write
// barrier; imported records keep their bytes and stop reading CUTOVER_MISSING.
func TestCALV0004_CutoverSwitchesWriterInOneReceipt(t *testing.T) {
	repo := importStore(t, "ROADMAP")
	runImport(t, repo, importExport(importItem("BF-1", "## BF-1\nfirst\n", nil)))
	id := fixture.TicketID("BF-1")
	_, before := readImported(t, repo, "BF-1")
	if !has(codes(t, repo, id), wire.CodeCutoverMissing) {
		t.Fatal("imported record eligible before cutover")
	}

	report := cutover(t, repo, operator(), "decision-0500")
	if report.Kind != "Transaction" || report.Outcome.Outcome != mutation.OutcomeCompleted || report.Receipt == "" {
		t.Fatalf("cutover: %+v", report)
	}
	raw, err := os.ReadFile(filepath.Join(repo.StateDir, "receipts", report.Receipt))
	if err != nil {
		t.Fatal(err)
	}
	rc, err := snapshot.DecodeReceipt(raw)
	if err != nil || rc.Kind != "AUTHORITY_SWITCH" || rc.RequestID == nil || *rc.RequestID != "decision-0500" || rc.TicketID != nil {
		t.Fatalf("receipt: %+v %v", rc, err)
	}
	st, err := intent.Load(repo.PrimaryWorktree)
	if err != nil {
		t.Fatal(err)
	}
	if st.Queue.CanonicalWriter != "NATIVE" || st.Queue.ForeignAdapterID != nil || st.Queue.WriteBarrier.Reason != "CUTOVER" || st.Queue.WriteBarrier.Since == nil || *st.Queue.WriteBarrier.Since != now(t) {
		t.Fatalf("queue after cutover: %+v", st.Queue)
	}
	if _, after := readImported(t, repo, "BF-1"); string(after) != string(before) {
		t.Fatal("cutover rewrote an imported record")
	}
	if has(codes(t, repo, id), wire.CodeCutoverMissing) {
		t.Fatal("imported record still CUTOVER_MISSING after cutover")
	}
	if _, err = (journal.Reader{Source: journal.Native{StateDir: repo.StateDir, PrimaryWorktree: repo.PrimaryWorktree}, QueueID: st.Queue.QueueID, PrimaryWorktree: repo.PrimaryWorktree}).Audit(); err != nil {
		t.Fatalf("audit after cutover: %v", err)
	}

	if again := cutover(t, repo, operator(), "decision-0500"); again.Kind != "Replay" || !again.Outcome.Replayed {
		t.Fatalf("same decision did not replay: %+v", again)
	}
	if other := cutover(t, repo, operator(), "decision-0501"); other.Kind != "Refused" || other.Outcome.Outcome != mutation.OutcomeBlocked {
		t.Fatalf("second switch not refused: %+v", other)
	}
}

// TestCALV0005_ImportAfterCutoverRefusesAndWritesNothing: a native queue
// never takes shadow records, so a later import cannot overwrite a record.
func TestCALV0005_ImportAfterCutoverRefusesAndWritesNothing(t *testing.T) {
	repo := importStore(t, "ROADMAP")
	runImport(t, repo, importExport(importItem("BF-1", "## BF-1\nfirst\n", nil)))
	cutover(t, repo, operator(), "decision-0500")
	tree, err := intent.TreeDigest(repo.PrimaryWorktree)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Import(context.Background(), repo, operator(), fixture.QueueID, importExport(importItem("BF-1", "## BF-1\nchanged\n", nil)), now(t)); err == nil {
		t.Fatal("import into a NATIVE queue accepted")
	}
	if after, _ := intent.TreeDigest(repo.PrimaryWorktree); after.Sha256 != tree.Sha256 {
		t.Fatal("refused import changed the intent tree")
	}
}

// TestCALV0004_CutoverRefusals: an OPERATOR binding and a present barrier
// each refuse, and the queue stays ROADMAP-written.
func TestCALV0004_CutoverRefusals(t *testing.T) {
	repo := importStore(t, "ROADMAP")
	if got := cutover(t, repo, mutation.Binding{ID: "tester", Role: "OPERATOR"}, "decision-0500"); got.Kind != "Refused" || got.Outcome.Outcome != mutation.OutcomeUnauthorized {
		t.Fatalf("operator cutover: %+v", got)
	}
	changeBarrier(t, repo, transaction.Pause, "pause")
	if got := cutover(t, repo, operator(), "decision-0501"); got.Kind != "Refused" || !has(got.Outcome.Codes, wire.CodePaused) {
		t.Fatalf("paused cutover: %+v", got)
	}
	st, err := intent.Load(repo.PrimaryWorktree)
	if err != nil {
		t.Fatal(err)
	}
	if st.Queue.CanonicalWriter != "ROADMAP" || st.Queue.WriteBarrier.Reason != "NONE" {
		t.Fatalf("refused cutover changed the queue: %+v", st.Queue)
	}
}
