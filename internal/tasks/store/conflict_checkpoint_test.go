// SPDX-License-Identifier: Apache-2.0

package store

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
)

// TestLGLV0846ConflictRefusedFromCheckpoint: a request retained more than
// MaxWriterTail receipts before the writer checkpoint, sent again with other
// request bytes, is refused REQUEST_ID_CONFLICT by the writer route without
// the complete route's audit and without a write. It decides exactly as the
// complete route does, and its bound entry and ticket are those a full
// request-index lookup reports. The same request bytes are still a replay
// the route declines to the complete route (TestCALV0190_WriterReplayBeyondBoundDeclines).
func TestLGLV0846ConflictRefusedFromCheckpoint(t *testing.T) {
	t.Parallel()
	repo := writerStore(t, journal.MaxWriterTail+40)
	if gap := readWriterCheckpoint(repo).Seq.Uint64() - retainedRequestSeq(t, repo, "history-create-0"); gap <= journal.MaxWriterTail {
		t.Fatalf("fixture: request %d receipts before the checkpoint, want more than %d", gap, journal.MaxWriterTail)
	}
	write := writerCreate(repo, "history-create-0", WallClock())
	before := mutationPublished(t, repo)
	run := stagedWrite(write, nil)
	if run.err != nil || run.declined() != "" || run.fast() || !strings.Contains(strings.Join(run.stages, "\n"), "fast.conflict=") {
		t.Fatalf("route: %+v %v %v", run.rep, run.err, run.stages)
	}
	if run.rep.Outcome.Outcome != mutation.OutcomeRequestIDConflict || !reflect.DeepEqual(run.rep.Outcome.Codes, []string{"REQUEST_ID_CONFLICT"}) {
		t.Fatalf("outcome %+v", run.rep.Outcome)
	}
	if after := mutationPublished(t, repo); after != before || run.rep.Receipt != "" {
		t.Fatalf("published %s (receipt %q), before %s", after, run.rep.Receipt, before)
	}
	oracle := writerOracle(t, repo, write)
	sameDecision(t, run, oracle)
	if run.rep.Ticket != oracle.rep.Ticket || run.rep.Detail != oracle.rep.Detail || run.rep.OriginalReceipt != oracle.rep.OriginalReceipt || run.rep.Kind != oracle.rep.Kind {
		t.Fatalf("report %+v, complete route %+v", run.rep, oracle.rep)
	}
	entry, ticketID, ok := run.rep.ConflictBinding()
	if !ok {
		t.Fatal("the writer-route refusal carries no binding")
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
	if _, _, ok := oracle.rep.ConflictBinding(); ok {
		t.Fatal("the complete route's refusal carries a writer binding")
	}
}
