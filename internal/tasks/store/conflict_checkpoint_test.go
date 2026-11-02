// SPDX-License-Identifier: Apache-2.0

package store

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestLGLV0846ConflictRefusedFromCheckpoint: a request retained more than
// MaxWriterTail receipts before the writer checkpoint, sent again with other
// request bytes, declines the writer route exactly as at the base and is
// refused REQUEST_ID_CONFLICT by the complete route without a write. That
// refusal carries the recorded entry and ticket its audit bound, which equal
// what a full request-index lookup reports, so the CLI does not audit again.
func TestLGLV0846ConflictRefusedFromCheckpoint(t *testing.T) {
	t.Parallel()
	repo := writerStore(t, journal.MaxWriterTail+40)
	if gap := readWriterCheckpoint(repo).Seq.Uint64() - retainedRequestSeq(t, repo, "history-create-0"); gap <= journal.MaxWriterTail {
		t.Fatalf("fixture: request %d receipts before the checkpoint, want more than %d", gap, journal.MaxWriterTail)
	}
	write := writerCreate(repo, "history-create-0", WallClock())
	before := mutationPublished(t, repo)
	run := stagedWrite(write, nil)
	if run.err != nil || !strings.Contains(run.declined(), "not within the bound") {
		t.Fatalf("route: %+v %v %v", run.rep, run.err, run.stages)
	}
	if run.rep.Outcome.Outcome != mutation.OutcomeRequestIDConflict || !reflect.DeepEqual(run.rep.Outcome.Codes, []string{"REQUEST_ID_CONFLICT"}) {
		t.Fatalf("outcome %+v", run.rep.Outcome)
	}
	if after := mutationPublished(t, repo); after != before || run.rep.Receipt != "" {
		t.Fatalf("published %s (receipt %q), before %s", after, run.rep.Receipt, before)
	}
	entry, ticketID, ok := run.rep.ConflictBinding()
	if !ok {
		t.Fatal("the complete route's conflict carries no binding")
	}
	head, err := readHead(repo)
	if err != nil {
		t.Fatal(err)
	}
	idx := journal.RequestIndex{Reader: journalReader(repo, head)}
	want, found, err := idx.Lookup("history-create-0")
	if err != nil || !found || !reflect.DeepEqual(entry, want) || ticketID != idx.TicketID {
		t.Fatalf("binding %+v %q, lookup %+v %q %v %v", entry, ticketID, want, idx.TicketID, found, err)
	}
	// A replay is not a conflict and carries no binding.
	replay := stagedWrite(writerEnvelope(repo, historyCreate("history-create-0", "history history-create-0"), WallClock()), nil)
	if !replay.completed() || replay.rep.Kind != "Replay" {
		t.Fatalf("replay: %+v %v", replay.rep, replay.err)
	}
	if _, _, ok := replay.rep.ConflictBinding(); ok {
		t.Fatal("a replay carries a conflict binding")
	}
}

// TestLGLV0846MalformedShardRefusedAsBase: with a malformed file inside the
// retained request's existing requests/<shard>/ directory, a conflicting
// retry beyond the replay tail is refused exactly as the complete route
// refuses it at the base, by the shard scan, and never as REQUEST_ID_CONFLICT.
func TestLGLV0846MalformedShardRefusedAsBase(t *testing.T) {
	t.Parallel()
	repo := writerStore(t, journal.MaxWriterTail+40)
	rp, err := snapshot.RequestPath("history-create-0")
	if err != nil {
		t.Fatal(err)
	}
	stray := filepath.Join(repo.StateDir, filepath.Dir(rp), "zz-not-a-request.json")
	if err := os.WriteFile(stray, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	write := writerCreate(repo, "history-create-0", WallClock())
	before := mutationPublished(t, repo)
	run := stagedWrite(write, nil)
	if run.err == nil || run.rep != nil && run.rep.Outcome.Outcome == mutation.OutcomeRequestIDConflict {
		t.Fatalf("a malformed shard entry did not refuse first: %+v %v %v", run.rep, run.err, run.stages)
	}
	if after := mutationPublished(t, repo); after != before {
		t.Fatalf("published %s, before %s", after, before)
	}
	oracle := writerOracle(t, repo, write)
	sameDecision(t, run, oracle)
	if run.err.Error() != oracle.err.Error() {
		t.Fatalf("refusal %q, complete route %q", run.err, oracle.err)
	}
	t.Logf("refused %s: %v", wire.CodeOf(run.err), run.err)
}
