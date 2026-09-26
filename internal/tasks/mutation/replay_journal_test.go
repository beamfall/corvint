package mutation_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

type indexFailure struct{ err error }

func (i indexFailure) Lookup(string) (mutation.IndexEntry, bool, error) {
	return mutation.IndexEntry{}, false, i.err
}

func TestTMV0006_AS03_LookupErrorRefusesApplyAndAdopt(t *testing.T) {
	rec := fixture.Ticket("A")
	ctx := newCtx(t, owner, nil, rec)
	ctx.Requests = indexFailure{wire.Errorf(wire.CodeUnsupportedFilesystem, "requests", "injected EIO")}
	env, err := mutation.Decode(envelope("request", owner, "A", "1", mutation.OpRefine, obj("title", str("new"))))
	if err != nil {
		t.Fatal(err)
	}
	for _, plan := range []*mutation.Plan{mutation.Apply(ctx, env), mutation.Adopt(ctx, "request", rec, rec.Encode())} {
		if plan.Outcome.Outcome != mutation.OutcomeStorageFailed || !plan.Outcome.HasCode(wire.CodeUnsupportedFilesystem) || plan.Post != nil || plan.QueuePost != nil || plan.Outcome.ReceiptSeq != nil || plan.Planned() {
			t.Fatalf("fresh plan or lost failure: %+v", plan)
		}
	}
}

type replayReadFailure struct {
	journal.Source
	path string
	err  error
}

func (s replayReadFailure) Read(p string, n int) ([]byte, error) {
	if p == s.path {
		return nil, s.err
	}
	return s.Source.Read(p, n)
}

func TestTMV0006_AS03_StreamedLedgerExactReplayConflictAndLateIO(t *testing.T) {
	repo := fixture.TempRepo(t)
	fixture.WriteIntent(t, repo)
	fixture.WriteState(t, repo)
	rec := fixture.Ticket("A")
	fixture.CommitPosts(t, repo, "MUTATION", "", map[string][]byte{"intent/tickets/A.json": rec.Encode()})
	ctx := newCtx(t, owner, nil, rec)
	raw := envelope("request", owner, "A", "1", mutation.OpRefine, obj("title", str("new")))
	env, err := mutation.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	plan := mutation.Apply(ctx, env)
	if !plan.Planned() {
		t.Fatal(plan.Outcome)
	}
	seq := wire.Size("3")
	out := plan.Outcome
	out.ReceiptSeq = &seq
	path, _ := snapshot.RequestPath("request")
	entry := obj("requestId", str("request"), "seq", str("3"), "mutationSha256", str(string(plan.MutationSha256)), "outcome", out.Value())
	fixture.CommitPosts(t, repo, "MUTATION", "request", map[string][]byte{path: wire.EncodeFile(entry), "intent/tickets/A.json": plan.Post.Encode()})
	// Bind the simulated receipt to its actual ticket afterimage.
	rp := filepath.Join(repo.StateDir, "receipts", "000000000003.json")
	receiptRaw, err := os.ReadFile(rp)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := wire.Parse(receiptRaw)
	if err != nil {
		t.Fatal(err)
	}
	receipt.Obj.Set("ticketId", str(rec.TicketID.Raw))
	receiptRaw = wire.EncodeFile(receipt)
	fixture.Write(t, rp, receiptRaw)
	hp := filepath.Join(repo.StateDir, "head.json")
	headRaw, err := os.ReadFile(hp)
	if err != nil {
		t.Fatal(err)
	}
	head, err := snapshot.DecodeHead(headRaw)
	if err != nil {
		t.Fatal(err)
	}
	last := wire.Sum(receiptRaw)
	head.LastReceiptSha256 = &last
	fixture.Write(t, hp, wire.EncodeFile(head.Value()))
	q, _ := wire.ParseQueueID("", fixture.QueueID)
	reader := journal.Reader{Source: journal.Native{StateDir: repo.StateDir, PrimaryWorktree: repo.Root}, QueueID: q, PrimaryWorktree: repo.Root}
	index := &journal.RequestIndex{Reader: reader}
	ctx.Requests = index
	replay := mutation.Apply(ctx, env)
	want := out
	want.Replayed = true
	if !wire.Equal(replay.Outcome.Value(), want.Value()) || replay.Planned() || replay.Post != nil {
		t.Fatalf("full replay differs: %+v", replay)
	}
	conflict, err := mutation.Decode(envelope("request", owner, "A", "1", mutation.OpRefine, obj("title", str("different"))))
	if err != nil {
		t.Fatal(err)
	}
	if got := mutation.Apply(ctx, conflict); got.Outcome.Outcome != mutation.OutcomeRequestIDConflict || got.Post != nil {
		t.Fatalf("conflict: %+v", got)
	}
	boom := errors.New("injected source error after prior successful audit")
	index.Reader.Source = replayReadFailure{Source: reader.Source, path: path, err: boom}
	for _, got := range []*mutation.Plan{mutation.Apply(ctx, env), mutation.Adopt(ctx, "unseen", rec, rec.Encode())} {
		if got.Outcome.Outcome != mutation.OutcomeStorageFailed || got.Post != nil || got.Outcome.ReceiptSeq != nil {
			t.Fatalf("I/O became fresh plan: %+v", got)
		}
	}
	index.Reader = reader
	if err := os.Remove(filepath.Join(repo.StateDir, path)); err != nil {
		t.Fatal(err)
	}
	if got := mutation.Apply(ctx, env); got.Outcome.Outcome != mutation.OutcomeStorageFailed || !got.Outcome.HasCode(wire.CodeJournalForked) {
		t.Fatal(got.Outcome)
	}
}

func TestTMV0002_AS01_RecordedRefusalSequence(t *testing.T) {
	seq := wire.Size("7")
	out := mutation.Outcome{RequestID: "request", Outcome: mutation.OutcomeBlocked, ReceiptSeq: &seq, Codes: []string{wire.CodePaused}}
	raw, err := out.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := mutation.DecodeOutcome(raw)
	if err != nil || decoded.ReceiptSeq == nil || *decoded.ReceiptSeq != seq {
		t.Fatalf("recorded refusal: %+v %v", decoded, err)
	}
	rev := wire.Count("1")
	out.ResultingRevision = &rev
	if _, err := out.Encode(); wire.CodeOf(err) != wire.CodeMalformed {
		t.Fatal("refusal revisions accepted", err)
	}
}
