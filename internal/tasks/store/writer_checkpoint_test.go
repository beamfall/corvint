package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// writerRun is one write and the mutation stages it reported.
type writerRun struct {
	rep    *Report
	err    error
	stages []string
}

// fast reports whether the writer-checkpoint route committed the write.
func (r writerRun) fast() bool {
	for _, s := range r.stages {
		if strings.HasPrefix(s, "fast.apply=") || strings.HasPrefix(s, "fast.lease.commit=") {
			return true
		}
	}
	return false
}

// declined is the reason the writer-checkpoint route gave up, if it did.
func (r writerRun) declined() string {
	for _, s := range r.stages {
		if why, ok := strings.CutPrefix(s, "fast.declined: "); ok {
			return why
		}
	}
	return ""
}

func (r writerRun) completed() bool {
	return r.err == nil && r.rep != nil && r.rep.Outcome.Outcome == mutation.OutcomeCompleted
}

// stagedWrite runs write with a stage hook that records every stage and
// calls hook, when set, with each.
func stagedWrite(write func(context.Context) (*Report, error), hook func(string)) writerRun {
	var run writerRun
	ctx := context.WithValue(context.Background(), mutationStageKey{}, func(stage string) {
		run.stages = append(run.stages, stage)
		if hook != nil {
			hook(stage)
		}
	})
	leaseAudits.Lock()
	leaseAudits.proof = nil
	leaseAudits.Unlock()
	run.rep, run.err = write(ctx)
	return run
}

// at returns a stage hook that runs change once, at the first stage named
// name (a writer stage reports name=duration).
func at(name string, change func()) func(string) {
	done := false
	return func(stage string) {
		if !done && (stage == name || strings.HasPrefix(stage, name+"=")) {
			done = true
			change()
		}
	}
}

func writerCreate(repo *intent.Repository, id string, now wire.Timestamp) func(context.Context) (*Report, error) {
	return writerEnvelope(repo, historyCreate(id, "writer "+id), now)
}

func writerEnvelope(repo *intent.Repository, raw []byte, now wire.Timestamp) func(context.Context) (*Report, error) {
	return func(ctx context.Context) (*Report, error) { return Mutate(ctx, repo, historyActor, raw, now) }
}

func writerMutate(t *testing.T, repo *intent.Repository, id string, hook func(string)) writerRun {
	t.Helper()
	return stagedWrite(writerCreate(repo, id, WallClock()), hook)
}

// writerStore is historyStore with the writer-checkpoint threshold lowered
// so the route serves it, after one complete-route write that retained the
// first writer checkpoint.
func writerStore(t *testing.T, receipts int) *intent.Repository {
	t.Helper()
	repo := historyStore(t, receipts)
	old := minWriterCheckpointSeq
	minWriterCheckpointSeq = 2
	t.Cleanup(func() { minWriterCheckpointSeq = old })
	if run := writerMutate(t, repo, "writer-seed", nil); !run.completed() || run.fast() {
		t.Fatalf("seed: %+v %v %v", run.rep, run.err, run.stages)
	}
	wc := readWriterCheckpoint(repo)
	if wc == nil || wc.Seq.Uint64() != uint64(receipts) || wc.FullSeq != uint64(receipts) {
		t.Fatalf("complete route retained %+v, want a checkpoint at %d", wc, receipts)
	}
	return repo
}

func headSeqOf(t *testing.T, repo *intent.Repository) uint64 {
	t.Helper()
	head, err := readHead(repo)
	if err != nil {
		t.Fatal(err)
	}
	return head.LastSeq.Uint64()
}

func readCheckpointSeq(t *testing.T, repo *intent.Repository) uint64 {
	t.Helper()
	raw, err := os.ReadFile(journal.CheckpointPath(repo.StateDir))
	if err != nil {
		t.Fatal(err)
	}
	cp, err := journal.DecodeCheckpoint(raw)
	if err != nil {
		t.Fatal(err)
	}
	return cp.Seq.Uint64()
}

// writerPad appends n synthetic receipts, each posting one request and one
// edited ticket afterimage, outside every writer (historyStoreAt's padding).
func writerPad(t *testing.T, repo *intent.Repository, n int) {
	t.Helper()
	dir := filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets")
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("tickets: %v", err)
	}
	for i := 0; i < n; i++ {
		seq := headSeqOf(t, repo) + 1
		name := entries[int(seq)%len(entries)].Name()
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		v, err := wire.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		v.Obj.Set("body", wire.String(fmt.Sprintf("pad %d", seq)))
		id := fmt.Sprintf("writer-pad-%d", seq)
		reqPath, _ := snapshot.RequestPath(id)
		historyAppend(t, repo, map[string][]byte{reqPath: historyRequest(id, seq), "intent/tickets/" + name: wire.EncodeFile(v)}, id)
	}
}

// writerCheckpointRewrite decodes the retained writer checkpoint, edits it
// and retains the re-encoded bytes (a valid trailer over forged content).
func writerCheckpointRewrite(t *testing.T, repo *intent.Repository, edit func(*journal.WriterCheckpoint)) {
	t.Helper()
	wc := readWriterCheckpoint(repo)
	if wc == nil {
		t.Fatal("no writer checkpoint")
	}
	edit(wc)
	historyWrite(t, journal.WriterCheckpointPath(repo.StateDir), wc.Encode())
}

func rewriteFile(t *testing.T, p string, edit func([]byte) []byte) func() {
	t.Helper()
	old, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	historyWrite(t, p, edit(append([]byte(nil), old...)))
	return func() { historyWrite(t, p, old) }
}

func appendLF(raw []byte) []byte { return append(raw, '\n') }

// forkReceipt replaces receipt seq with a different well-formed receipt (the
// same transaction under another actor id), so every later receipt's link to
// it breaks.
func forkReceipt(t *testing.T, repo *intent.Repository, seq uint64) func() {
	t.Helper()
	name, _ := snapshot.ReceiptName(seq)
	return rewriteFile(t, filepath.Join(repo.StateDir, "receipts", name), func(raw []byte) []byte {
		v, err := wire.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		actor, ok := v.Obj.Get("actor")
		if !ok || actor.Obj == nil {
			t.Fatalf("receipt %d has no actor", seq)
		}
		actor.Obj.Set("id", wire.String("forger"))
		return wire.EncodeFile(v)
	})
}

// CAL-V0-115 (proposed): the writer checkpoint is derived state. A missing
// (removed), corrupt, torn, foreign or forged checkpoint, and one the head
// has outrun by more than MaxWriterTail receipts, sends the write through
// the complete audit, which serves the valid store and retains a fresh
// checkpoint at the head it audited; a stale temporary file is harmless.
// CAL-V0-119 (proposed): a fast write advances the read checkpoint to the
// head it observed, as the complete route does.
func TestCALV0115_WriterCheckpointFallsBackToCompleteAudit(t *testing.T) {
	repo := writerStore(t, 70)
	other := historyStore(t, 70)
	if run := writerMutate(t, other, "foreign-seed", nil); !run.completed() {
		t.Fatalf("foreign seed: %+v %v", run.rep, run.err)
	}
	foreign, err := os.ReadFile(journal.WriterCheckpointPath(other.StateDir))
	if err != nil {
		t.Fatal(err)
	}
	wcPath := journal.WriterCheckpointPath(repo.StateDir)
	run := writerMutate(t, repo, "writer-fast", nil)
	if !run.completed() || !run.fast() {
		t.Fatalf("fast write: %+v %v %v", run.rep, run.err, run.stages)
	}
	if got, want := readCheckpointSeq(t, repo), headSeqOf(t, repo)-1; got != want {
		t.Fatalf("read checkpoint at %d, want the fast writer's observed head %d", got, want)
	}
	cases := []struct {
		name   string
		change func(t *testing.T)
	}{
		{"removed", func(t *testing.T) {
			if err := os.Remove(wcPath); err != nil {
				t.Fatal(err)
			}
		}},
		{"corrupt", func(t *testing.T) {
			rewriteFile(t, wcPath, func(raw []byte) []byte { raw[len(raw)/2] ^= 0xff; return raw })
		}},
		{"torn", func(t *testing.T) { rewriteFile(t, wcPath, func(raw []byte) []byte { return raw[:len(raw)/2] }) }},
		{"foreign", func(t *testing.T) { historyWrite(t, wcPath, foreign) }},
		{"forged-receipt-digest", func(t *testing.T) {
			writerCheckpointRewrite(t, repo, func(wc *journal.WriterCheckpoint) { wc.ReceiptSha256 = wire.Sum([]byte("forged")) })
		}},
		{"forged-ahead-of-head", func(t *testing.T) {
			writerCheckpointRewrite(t, repo, func(wc *journal.WriterCheckpoint) {
				wc.Seq = wire.SizeOf(headSeqOf(t, repo) + 1)
			})
		}},
		{"tail-beyond-bound", func(t *testing.T) { writerPad(t, repo, journal.MaxWriterTail+1) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.change(t)
			before := headSeqOf(t, repo)
			run := writerMutate(t, repo, "fallback-"+tc.name, nil)
			if !run.completed() || run.fast() || run.declined() == "" {
				t.Fatalf("write: %+v %v %v", run.rep, run.err, run.stages)
			}
			t.Logf("declined: %s", run.declined())
			wc := readWriterCheckpoint(repo)
			if wc == nil || wc.Seq.Uint64() != before || wc.FullSeq != before {
				t.Fatalf("complete route retained %+v, want a checkpoint at %d", wc, before)
			}
			if got := readCheckpointSeq(t, repo); got != before {
				t.Fatalf("read checkpoint at %d, want %d", got, before)
			}
			if next := writerMutate(t, repo, "after-"+tc.name, nil); !next.completed() || !next.fast() {
				t.Fatalf("next write: %+v %v %v", next.rep, next.err, next.stages)
			}
		})
	}
	t.Run("stale-temporary", func(t *testing.T) {
		historyWrite(t, wcPath+".tmp", []byte("torn"))
		if run := writerMutate(t, repo, "stale-temporary", nil); !run.completed() || !run.fast() {
			t.Fatalf("write: %+v %v %v", run.rep, run.err, run.stages)
		}
	})
	head, err := writerGuards(repo, "MUTATE")
	if err != nil {
		t.Fatal(err)
	}
	if proof, err := journalReader(repo, head).Audit(); err != nil || proof.StructuralConsistency != "CONSISTENT" {
		t.Fatalf("final audit: %+v %v", proof, err)
	}
}

// CAL-V0-116 (proposed): no writer accepts a checkpoint whose complete audit
// is WriterFullBound receipts or more behind the head, even when its tail is
// short: the write runs the complete audit, which re-derives FullSeq.
func TestCALV0116_WriterFullBoundDeclines(t *testing.T) {
	receipts := journal.WriterFullBound + 8
	repo := writerStore(t, receipts)
	writerCheckpointRewrite(t, repo, func(wc *journal.WriterCheckpoint) { wc.FullSeq = 8 })
	before := headSeqOf(t, repo)
	run := writerMutate(t, repo, "full-bound", nil)
	if !run.completed() || run.fast() || !strings.Contains(run.declined(), "writer bound") {
		t.Fatalf("write: %+v %v %v", run.rep, run.err, run.stages)
	}
	if wc := readWriterCheckpoint(repo); wc == nil || wc.FullSeq != before {
		t.Fatalf("complete route retained %+v, want FullSeq %d", wc, before)
	}
}

// storeFiles digests every file under root except the writer checkpoint,
// which the two routes legitimately retain at different heads.
func storeFiles(t *testing.T, root, wcPath string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		switch {
		case p == wcPath || p == wcPath+".tmp":
			return nil
		case d.Type()&fs.ModeSymlink != 0:
			target, err := os.Readlink(p)
			out[rel] = "link " + target
			return err
		case d.IsDir():
			out[rel] = "dir"
			return nil
		}
		raw, err := os.ReadFile(p)
		sum := sha256.Sum256(raw)
		out[rel] = hex.EncodeToString(sum[:])
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// writerParity runs write on the writer-checkpoint route, restores the
// store, runs it again with the writer checkpoint removed, and requires the
// same report and byte-identical journal, intent tree, private state and
// read checkpoint (CAL-V0-116, CAL-V0-119). The store keeps the complete
// route's result.
func writerParity(t *testing.T, repo *intent.Repository, write func(context.Context) (*Report, error)) *Report {
	t.Helper()
	parent := filepath.Dir(repo.PrimaryWorktree)
	wcPath := journal.WriterCheckpointPath(repo.StateDir)
	backup := filepath.Join(t.TempDir(), "store")
	copyTree(t, parent, backup)
	fast := stagedWrite(write, nil)
	if !fast.completed() || !fast.fast() {
		t.Fatalf("fast route: %+v %v %v", fast.rep, fast.err, fast.stages)
	}
	fastFiles := storeFiles(t, parent, wcPath)
	if err := os.RemoveAll(parent); err != nil {
		t.Fatal(err)
	}
	copyTree(t, backup, parent)
	if err := os.Remove(wcPath); err != nil {
		t.Fatal(err)
	}
	complete := stagedWrite(write, nil)
	if !complete.completed() || complete.fast() {
		t.Fatalf("complete route: %+v %v %v", complete.rep, complete.err, complete.stages)
	}
	completeFiles := storeFiles(t, parent, wcPath)
	for p, d := range completeFiles {
		if fastFiles[p] != d {
			t.Errorf("%s: fast %q, complete %q", p, fastFiles[p], d)
		}
	}
	for p := range fastFiles {
		if _, ok := completeFiles[p]; !ok {
			t.Errorf("%s: only the fast route wrote it", p)
		}
	}
	if !reflect.DeepEqual(*fast.rep, *complete.rep) {
		t.Fatalf("reports differ:\nfast     %+v\ncomplete %+v", *fast.rep, *complete.rep)
	}
	if t.Failed() {
		t.FailNow()
	}
	return complete.rep
}

// CAL-V0-116 and CAL-V0-119 (proposed): a create, a first and a superseding
// note, and the claim, renew, heartbeat and release writes the route serves
// commit the same bytes, retain the same read checkpoint and report the same
// outcome as the complete route.
func TestCALV0116_WriterRouteParity(t *testing.T) {
	repo := writerStore(t, 70)
	// Both routes mint the claim's attempt ID from the same fixed source.
	entropy := attemptEntropy
	attemptEntropy = fixedEntropy(7)
	t.Cleanup(func() { attemptEntropy = entropy })
	holdLeasePolicy(t, repo)
	if run := writerMutate(t, repo, "parity-reseed", nil); !run.completed() {
		t.Fatalf("reseed: %+v %v", run.rep, run.err)
	}
	root := filepath.Join(filepath.Dir(repo.PrimaryWorktree), "worktree")
	holdGit(t, root, "init", "-q", "-b", "main")
	holdGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base")
	now := WallClock()
	// Each parity pair shares one clock reading, taken after the last write.
	created := writerParity(t, repo, writerCreate(repo, "parity-create", now))
	note := func(id, supersedes, text string) []byte {
		s := wire.String
		return wire.EncodeFile(historyObject(
			"profile", s(mutation.Profile),
			"requestId", s(id),
			"actor", historyObject("id", s(historyActor.ID), "role", s(historyActor.Role)),
			"queueId", s(fixture.QueueID),
			"targetId", s(created.Ticket),
			"expectedRevision", wire.Null(),
			"operation", s(mutation.OpNoteSet),
			"payload", historyObject("supersedes", s(supersedes), "text", s(text)),
			"issuedAt", s("2026-09-07T12:00:00Z"),
		))
	}
	// A note receipt in the tail declines the next writer, whose complete
	// audit re-bases the checkpoint past it (the read checkpoint's
	// CAL-V0-061 rule, which the writer audit shares).
	rebase := func(id string) {
		t.Helper()
		run := writerMutate(t, repo, id, nil)
		if !run.completed() || run.fast() || !strings.Contains(run.declined(), "note transition pre-state precedes the checkpoint") {
			t.Fatalf("write after a note: %+v %v %v", run.rep, run.err, run.stages)
		}
	}
	writerParity(t, repo, writerEnvelope(repo, note("parity-note-1", "0", "First."), now))
	rebase("parity-after-note-1")
	now = WallClock()
	writerParity(t, repo, writerEnvelope(repo, note("parity-note-2", "1", "Second."), now))
	rebase("parity-after-note-2")
	now = WallClock()
	lease := func(id string, l transaction.LeaseRequest) func(context.Context) (*Report, error) {
		return func(ctx context.Context) (*Report, error) {
			return Lease(ctx, repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: id, Root: root, Lease: l}, now)
		}
	}
	claim := writerParity(t, repo, lease("parity-claim", transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: created.Ticket, Holder: "agent-1", LeaseMinutes: "60"}))
	writerParity(t, repo, lease("parity-renew", transaction.LeaseRequest{Verb: transaction.LeaseRenew, AttemptID: claim.AttemptID, Generation: claim.Generation, LeaseMinutes: "60"}))
	writerParity(t, repo, lease("parity-heartbeat", transaction.LeaseRequest{Verb: transaction.LeaseHeartbeat, AttemptID: claim.AttemptID, Generation: claim.Generation}))
	writerParity(t, repo, lease("parity-release", transaction.LeaseRequest{Verb: transaction.LeaseRelease, AttemptID: claim.AttemptID, Generation: claim.Generation}))
}

// fixedEntropy reads as an endless run of one byte.
type fixedEntropy byte

func (f fixedEntropy) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(f)
	}
	return len(p), nil
}

// writerOracle reruns write with the writer checkpoint removed, so the
// complete route alone decides it, then puts the checkpoint back.
func writerOracle(t *testing.T, repo *intent.Repository, write func(context.Context) (*Report, error)) writerRun {
	t.Helper()
	wcPath := journal.WriterCheckpointPath(repo.StateDir)
	raw, err := os.ReadFile(wcPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(wcPath); err != nil {
		t.Fatal(err)
	}
	run := stagedWrite(write, nil)
	if run.fast() {
		t.Fatalf("oracle used the writer route: %v", run.stages)
	}
	historyWrite(t, wcPath, raw)
	return run
}

// sameDecision requires the two runs to end alike: the same refusal code,
// or the same outcome, kind and detail.
func sameDecision(t *testing.T, got, want writerRun) {
	t.Helper()
	if (got.err == nil) != (want.err == nil) || wire.CodeOf(got.err) != wire.CodeOf(want.err) {
		t.Fatalf("refusal %v, complete route %v", got.err, want.err)
	}
	if got.err == nil && (got.rep.Outcome.Outcome != want.rep.Outcome.Outcome || got.rep.Kind != want.rep.Kind || !reflect.DeepEqual(got.rep.Outcome.Codes, want.rep.Outcome.Codes)) {
		t.Fatalf("report %+v, complete route %+v", got.rep, want.rep)
	}
}

// CAL-V0-116 (proposed): the route declines every request that may replay,
// a request file no receipt posted, and a fork at or after the checkpoint
// receipt; the complete route then decides exactly as it does with no
// checkpoint, and a refusal publishes nothing.
func TestCALV0116_WriterRouteCounterexamples(t *testing.T) {
	repo := writerStore(t, 70)
	if run := writerMutate(t, repo, "tail-request", nil); !run.completed() || !run.fast() {
		t.Fatalf("tail write: %+v %v %v", run.rep, run.err, run.stages)
	}
	wcSeq := readWriterCheckpoint(repo).Seq.Uint64()
	receipt := func(seq uint64) string {
		name, _ := snapshot.ReceiptName(seq)
		return filepath.Join(repo.StateDir, "receipts", name)
	}
	stray, _ := snapshot.RequestPath("stray-own-request")
	cases := []struct {
		name   string
		write  func(context.Context) (*Report, error)
		change func(t *testing.T) func()
		refuse bool
	}{
		{"prefix-request-replay", writerEnvelope(repo, historyCreate("history-create-0", "history history-create-0"), WallClock()), nil, false},
		{"prefix-request-conflict", writerCreate(repo, "history-create-1", WallClock()), nil, true},
		{"tail-request-replay", writerCreate(repo, "tail-request", WallClock()), nil, false},
		{"stray-own-request", writerCreate(repo, "stray-own-request", WallClock()), func(t *testing.T) func() {
			p := filepath.Join(repo.StateDir, stray)
			historyWrite(t, p, []byte("{}\n"))
			return func() { os.Remove(p) }
		}, true},
		{"fork-at-checkpoint-receipt", writerCreate(repo, "fork-at-checkpoint", WallClock()), func(t *testing.T) func() {
			return forkReceipt(t, repo, wcSeq)
		}, true},
		{"fork-in-tail", writerCreate(repo, "fork-in-tail", WallClock()), func(t *testing.T) func() {
			return forkReceipt(t, repo, wcSeq+1)
		}, true},
		{"torn-tail-receipt", writerCreate(repo, "torn-tail", WallClock()), func(t *testing.T) func() {
			return rewriteFile(t, receipt(wcSeq+1), appendLF)
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			undo := func() {}
			if tc.change != nil {
				undo = tc.change(t)
			}
			before := mutationPublished(t, repo)
			run := stagedWrite(tc.write, nil)
			if run.fast() || run.declined() == "" {
				t.Fatalf("writer route served it: %+v %v %v", run.rep, run.err, run.stages)
			}
			if refused := run.err != nil || run.rep.Outcome.Outcome != mutation.OutcomeCompleted; refused != tc.refuse {
				t.Fatalf("refused %v, want %v: %+v %v", refused, tc.refuse, run.rep, run.err)
			}
			if after := mutationPublished(t, repo); after != before {
				t.Fatalf("published %s, before %s", after, before)
			}
			sameDecision(t, run, writerOracle(t, repo, tc.write))
			t.Logf("declined %q; decided %v %+v", run.declined(), run.err, run.rep.Outcome)
			undo()
		})
	}
}

// CAL-V0-116 (proposed): the fast route's own tamper coverage, mirroring the
// CAL-V0-070 cases where the tamper point exists on this route. An intent
// edit after its observation or its model is caught by the pre-effect
// binding, and the complete route refuses it as it does with no checkpoint;
// a receipt planted by a writer outside the lock moves the head, so the
// route declines and the complete route serves the write after it. The
// lease route is held to the same. (CAL-V0-012's racing writer inside the
// clock callback is not reachable here: this route samples the clock under
// the lock.)
func TestCALV0116_WriterRouteTamperAtFastStages(t *testing.T) {
	repo := writerStore(t, 70)
	holdLeasePolicy(t, repo)
	root := filepath.Join(filepath.Dir(repo.PrimaryWorktree), "worktree")
	holdGit(t, root, "init", "-q", "-b", "main")
	holdGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base")
	_, ticketFile := mutationBoundaryFiles(t, repo)
	entries, err := os.ReadDir(filepath.Dir(ticketFile))
	if err != nil {
		t.Fatal(err)
	}
	claimTarget := fixture.TicketID(strings.TrimSuffix(entries[len(entries)-1].Name(), ".json"))
	ensure := func(t *testing.T) {
		if readWriterCheckpoint(repo) == nil {
			if run := writerMutate(t, repo, fmt.Sprintf("reseed-%d", headSeqOf(t, repo)), nil); !run.completed() {
				t.Fatalf("reseed: %+v %v", run.rep, run.err)
			}
		}
	}
	claim := func(id string) func(context.Context) (*Report, error) {
		return func(ctx context.Context) (*Report, error) {
			return Lease(ctx, repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: id, Root: root, Lease: transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: claimTarget, Holder: "agent-1", LeaseMinutes: "60"}}, WallClock())
		}
	}
	for _, tc := range []struct {
		name, stage string
		write       func(context.Context) (*Report, error)
	}{
		{"mutate-ticket-edited-after-observe", "fast.observe", writerCreate(repo, "tamper-observe", WallClock())},
		{"mutate-ticket-edited-after-model", "fast.model", writerCreate(repo, "tamper-model", WallClock())},
		{"lease-ticket-edited-after-observe", "fast.lease.observe", claim("tamper-lease-observe")},
		{"lease-ticket-edited-after-model", "fast.lease.model", claim("tamper-lease-model")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ensure(t)
			before := mutationPublished(t, repo)
			undo := func() {}
			run := stagedWrite(tc.write, at(tc.stage, func() { undo = rewriteFile(t, ticketFile, appendLF) }))
			if run.fast() || !strings.Contains(strings.Join(run.stages, "\n"), tc.stage+"=") || run.err == nil || wire.CodeOf(run.err) != wire.CodeIntentDiverged {
				undo()
				t.Fatalf("write: %+v %v %v", run.rep, run.err, run.stages)
			}
			if after := mutationPublished(t, repo); after != before {
				undo()
				t.Fatalf("published %s, before %s", after, before)
			}
			sameDecision(t, run, writerOracle(t, repo, tc.write))
			undo()
		})
	}
	for _, tc := range []struct {
		name, stage string
		write       func(context.Context) (*Report, error)
	}{
		{"mutate-receipt-planted-after-model", "fast.model", writerCreate(repo, "planted-model", WallClock())},
		{"lease-receipt-planted-after-model", "fast.lease.model", claim("planted-lease-model")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ensure(t)
			var planted uint64
			// The planted receipt models a concurrent writer, so the write
			// carries the live clock every command-line writer has, and the
			// receipt lands in a later second than the write sampled.
			live := func(ctx context.Context) (*Report, error) { return tc.write(WithClock(ctx, WallClock)) }
			run := stagedWrite(live, at(tc.stage, func() {
				for sampled := WallClock(); WallClock() == sampled; {
					time.Sleep(10 * time.Millisecond)
				}
				writerPad(t, repo, 1)
				planted = headSeqOf(t, repo)
			}))
			if !run.completed() || run.fast() || run.declined() == "" && !strings.HasPrefix(tc.name, "lease") {
				t.Fatalf("write: %+v %v %v", run.rep, run.err, run.stages)
			}
			if planted == 0 || headSeqOf(t, repo) <= planted {
				t.Fatalf("head %d, planted %d", headSeqOf(t, repo), planted)
			}
		})
	}
	head, err := writerGuards(repo, "MUTATE")
	if err != nil {
		t.Fatal(err)
	}
	if proof, err := journalReader(repo, head).Audit(); err != nil || proof.StructuralConsistency != "CONSISTENT" {
		t.Fatalf("final audit: %+v %v", proof, err)
	}
}

// CAL-V0-116 (proposed): a fast write rechecks the intent branch before
// effects, as the complete route does (commitLease for a lease, the queue's
// intent branch for a mutation). A branch switch after modelling that leaves
// the intent tree unchanged declines the route, publishes nothing and is
// decided by the complete route exactly as with no checkpoint.
func TestCALV0116_FastWriteRechecksIntentBranch(t *testing.T) {
	repo := writerStore(t, 70)
	holdLeasePolicy(t, repo)
	if run := writerMutate(t, repo, "branch-reseed", nil); !run.completed() || readWriterCheckpoint(repo) == nil {
		t.Fatalf("reseed: %+v %v", run.rep, run.err)
	}
	root := filepath.Join(filepath.Dir(repo.PrimaryWorktree), "worktree")
	holdGit(t, root, "init", "-q", "-b", "main")
	holdGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base")
	_, ticketFile := mutationBoundaryFiles(t, repo)
	entries, err := os.ReadDir(filepath.Dir(ticketFile))
	if err != nil {
		t.Fatal(err)
	}
	claimTarget := fixture.TicketID(strings.TrimSuffix(entries[len(entries)-1].Name(), ".json"))
	claim := func(ctx context.Context) (*Report, error) {
		return Lease(ctx, repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: "branch-lease", Root: root, Lease: transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: claimTarget, Holder: "agent-1", LeaseMinutes: "60"}}, WallClock())
	}
	for _, tc := range []struct {
		name, stage string
		write       func(context.Context) (*Report, error)
	}{
		{"lease", "fast.lease.model", claim},
		{"mutate", "fast.model", writerCreate(repo, "branch-mutate", WallClock())},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := mutationPublished(t, repo)
			undo := func() {}
			defer func() { undo() }()
			run := stagedWrite(tc.write, at(tc.stage, func() {
				undo = rewriteFile(t, repo.IntentHEAD(), func([]byte) []byte { return []byte("ref: refs/heads/other\n") })
			}))
			if run.fast() || !strings.Contains(strings.Join(run.stages, "\n"), tc.stage+"=") || run.completed() {
				t.Fatalf("write after a branch switch: %+v %v %v", run.rep, run.err, run.stages)
			}
			if after := mutationPublished(t, repo); after != before {
				t.Fatalf("published %s, before %s", after, before)
			}
			sameDecision(t, run, writerOracle(t, repo, tc.write))
		})
	}
}

// CAL-V0-116 (proposed): a writer resume binds a tail post of a ticket last
// posted before its checkpoint to the note reference the checkpoint carries,
// as the complete audit binds it to the walked reference (ON-V0-006). The
// reference survives a complete re-base and both writer advances (walked in
// the tail, and carried from the base). An unchanged reference stays on the
// route; a reference changed without its note event declines it, publishes
// nothing, and the complete route refuses.
func TestCALV0116_TailNoteReferenceChangeWithoutEvent(t *testing.T) {
	repo := writerStore(t, 70)
	created := writerMutate(t, repo, "note-target", nil)
	if !created.completed() {
		t.Fatalf("create: %+v %v", created.rep, created.err)
	}
	s := wire.String
	note := wire.EncodeFile(historyObject(
		"profile", s(mutation.Profile),
		"requestId", s("note-set"),
		"actor", historyObject("id", s(historyActor.ID), "role", s(historyActor.Role)),
		"queueId", s(fixture.QueueID),
		"targetId", s(created.rep.Ticket),
		"expectedRevision", wire.Null(),
		"operation", s(mutation.OpNoteSet),
		"payload", historyObject("supersedes", s("0"), "text", s("Kept.")),
		"issuedAt", s("2026-09-07T12:00:00Z"),
	))
	if run := stagedWrite(writerEnvelope(repo, note, WallClock()), nil); !run.completed() {
		t.Fatalf("note: %+v %v", run.rep, run.err)
	}
	// The complete route re-bases the checkpoint at the head it audited,
	// past the note.
	if err := os.Remove(journal.WriterCheckpointPath(repo.StateDir)); err != nil {
		t.Fatal(err)
	}
	if run := writerMutate(t, repo, "note-rebase", nil); !run.completed() || run.fast() {
		t.Fatalf("rebase: %+v %v %v", run.rep, run.err, run.stages)
	}
	full := readWriterCheckpoint(repo)
	if full == nil || full.Seq.Uint64() != headSeqOf(t, repo)-1 {
		t.Fatalf("rebase retained %+v", full)
	}
	dir := filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets")
	local := created.rep.Ticket[strings.LastIndex(created.rep.Ticket, ":")+1:]
	rel := "intent/tickets/" + local + ".json"
	raw, err := os.ReadFile(filepath.Join(dir, local+".json"))
	if err != nil {
		t.Fatal(err)
	}
	rec, err := ticket.Decode(raw)
	if err != nil || rec.OperatorNote == nil {
		t.Fatalf("note target %s: %v", rel, err)
	}
	post := func(id, path string, body []byte) {
		t.Helper()
		rp, err := snapshot.RequestPath(id)
		if err != nil {
			t.Fatal(err)
		}
		historyAppend(t, repo, map[string][]byte{path: body, rp: historyRequest(id, headSeqOf(t, repo)+1)}, id)
	}
	post("note-same", rel, raw)
	if run := writerMutate(t, repo, "after-same", nil); !run.completed() || !run.fast() {
		t.Fatalf("write after an unchanged reference: %+v %v %v", run.rep, run.err, run.stages)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) < 2 {
		t.Fatalf("tickets: %v", err)
	}
	other := entries[0].Name()
	if other == local+".json" {
		other = entries[1].Name()
	}
	// Each advance pads the tail with another ticket's posts, then re-bases
	// on the writer audit: first with the noted ticket walked in the tail,
	// then with its reference carried from the base.
	for _, id := range []string{"advance-walked", "advance-carried"} {
		prev := readWriterCheckpoint(repo)
		for i := 0; i < journal.WriterAdvanceTail; i++ {
			body, err := os.ReadFile(filepath.Join(dir, other))
			if err != nil {
				t.Fatal(err)
			}
			v, err := wire.Parse(body)
			if err != nil {
				t.Fatal(err)
			}
			v.Obj.Set("body", wire.String(fmt.Sprintf("%s %d", id, i)))
			post(fmt.Sprintf("%s-pad-%d", id, i), "intent/tickets/"+other, wire.EncodeFile(v))
		}
		if run := writerMutate(t, repo, id, nil); !run.completed() || !run.fast() {
			t.Fatalf("%s: %+v %v %v", id, run.rep, run.err, run.stages)
		}
		if wc := readWriterCheckpoint(repo); wc == nil || wc.Seq.Uint64() <= prev.Seq.Uint64() || wc.FullSeq != full.FullSeq {
			t.Fatalf("%s retained %+v after %+v", id, wc, prev)
		}
	}
	rec.OperatorNote = nil
	post("note-dropped", rel, rec.Encode())
	before := mutationPublished(t, repo)
	write := writerCreate(repo, "after-dropped", WallClock())
	run := stagedWrite(write, nil)
	if run.fast() || !strings.Contains(run.declined(), "note reference differs from the writer checkpoint") || run.completed() || wire.CodeOf(run.err) != wire.CodeJournalForked {
		t.Fatalf("write after a dropped reference: %+v %v %v", run.rep, run.err, run.stages)
	}
	if after := mutationPublished(t, repo); after != before {
		t.Fatalf("published %s, before %s", after, before)
	}
	sameDecision(t, run, writerOracle(t, repo, write))
}

// CAL-V0-116 (proposed), the accepted scope: a fast writer does not re-read
// request afterimages before its checkpoint, so it commits over a tampered
// prefix request. The complete audit receipt audit runs refuses that store,
// the scheduled complete audit removes the writer checkpoint, and every
// later write then refuses through the complete route.
func TestCALV0116_PrefixTamperIsLeftToCompleteAudits(t *testing.T) {
	repo := writerStore(t, 70)
	requestFile, _ := mutationBoundaryFiles(t, repo)
	rewriteFile(t, requestFile, func([]byte) []byte { return []byte("{}\n") })
	if run := writerMutate(t, repo, "over-prefix-tamper", nil); !run.completed() || !run.fast() {
		t.Fatalf("fast write: %+v %v %v", run.rep, run.err, run.stages)
	}
	head, err := readHead(repo)
	if err != nil {
		t.Fatal(err)
	}
	_, auditErr := journalReader(repo, head).Audit()
	if wire.CodeOf(auditErr) != wire.CodeJournalForked {
		t.Fatalf("complete audit: %v", auditErr)
	}
	refreshWriterCheckpoint(context.Background(), repo)
	if _, err := os.Lstat(journal.WriterCheckpointPath(repo.StateDir)); !os.IsNotExist(err) {
		t.Fatalf("scheduled complete audit kept the writer checkpoint: %v", err)
	}
	before := mutationPublished(t, repo)
	run := writerMutate(t, repo, "after-refresh", nil)
	if run.fast() || wire.CodeOf(run.err) != wire.CodeJournalForked {
		t.Fatalf("write after refresh: %+v %v %v", run.rep, run.err, run.stages)
	}
	if after := mutationPublished(t, repo); after != before {
		t.Fatalf("published %s, before %s", after, before)
	}
}

// CAL-V0-117 (proposed): a refresh publishes only if no newer refresh
// invalidated the checkpoint after its audit began. An older refresh whose
// audit passed before a prefix tamper cannot reinstall the checkpoint a
// newer refresh removed on finding the tamper, so the next writer still
// takes the complete route and refuses the fork.
func TestCALV0117_OlderRefreshCannotUndoInvalidation(t *testing.T) {
	repo := writerStore(t, 70)
	if run := writerMutate(t, repo, "before-refreshes", nil); !run.completed() || !run.fast() {
		t.Fatalf("fast write: %+v %v %v", run.rep, run.err, run.stages)
	}
	requestFile, _ := mutationBoundaryFiles(t, repo)
	wcPath := journal.WriterCheckpointPath(repo.StateDir)
	var stages []string
	hook := at("refresh.audited", func() {
		rewriteFile(t, requestFile, func([]byte) []byte { return []byte("{}\n") })
		refreshWriterCheckpoint(context.Background(), repo)
		if _, err := os.Lstat(wcPath); !os.IsNotExist(err) {
			t.Errorf("newer refresh kept the writer checkpoint: %v", err)
		}
	})
	ctx := context.WithValue(context.Background(), mutationStageKey{}, func(stage string) {
		stages = append(stages, stage)
		hook(stage)
	})
	refreshWriterCheckpoint(ctx, repo)
	if _, err := os.Lstat(wcPath); !os.IsNotExist(err) {
		t.Fatalf("older refresh reinstalled the writer checkpoint: %v %v", err, stages)
	}
	if !strings.Contains(strings.Join(stages, "\n"), "refresh.superseded") {
		t.Fatalf("older refresh stages %v", stages)
	}
	before := mutationPublished(t, repo)
	run := writerMutate(t, repo, "after-refreshes", nil)
	if run.fast() || wire.CodeOf(run.err) != wire.CodeJournalForked {
		t.Fatalf("write after both refreshes: %+v %v %v", run.rep, run.err, run.stages)
	}
	if after := mutationPublished(t, repo); after != before {
		t.Fatalf("published %s, before %s", after, before)
	}
}

// CAL-V0-117 (proposed): the scheduled complete audit runs outside the lock
// and binds what it retains to the receipt head it audited. Writers that
// commit between its audit and its lock leave the read checkpoint they
// retained and are walked as tail by the next writer; a receipt changed in
// that window fails the next writer's rebinding, and the complete route
// refuses the fork.
func TestCALV0117_RefreshWriteInterleave(t *testing.T) {
	repo := writerStore(t, 70)
	if run := writerMutate(t, repo, "before-refresh", nil); !run.completed() || !run.fast() {
		t.Fatalf("fast write: %+v %v %v", run.rep, run.err, run.stages)
	}
	audited := headSeqOf(t, repo)
	var between []writerRun
	ctx := context.WithValue(context.Background(), mutationStageKey{}, at("refresh.audited", func() {
		for i := 0; i < 2; i++ {
			between = append(between, writerMutate(t, repo, fmt.Sprintf("between-%d", i), nil))
		}
	}))
	refreshWriterCheckpoint(ctx, repo)
	for _, run := range between {
		if !run.completed() || !run.fast() {
			t.Fatalf("interleaved write: %+v %v %v", run.rep, run.err, run.stages)
		}
	}
	wc := readWriterCheckpoint(repo)
	if wc == nil || wc.Seq.Uint64() != audited || wc.FullSeq != audited {
		t.Fatalf("refresh retained %+v, want a checkpoint bound to receipt %d", wc, audited)
	}
	if got := readCheckpointSeq(t, repo); got != audited+1 {
		t.Fatalf("read checkpoint at %d, want the last interleaved writer's %d", got, audited+1)
	}
	if run := writerMutate(t, repo, "after-refresh", nil); !run.completed() || !run.fast() {
		t.Fatalf("write after refresh: %+v %v %v", run.rep, run.err, run.stages)
	}

	// A receipt changed between the refresh's audit and its lock, behind a
	// writer that committed in the same window (so the head receipt, which
	// every writer's guard checks, is intact).
	audited = headSeqOf(t, repo)
	undo := func() {}
	ctx = context.WithValue(context.Background(), mutationStageKey{}, at("refresh.audited", func() {
		if run := writerMutate(t, repo, "between-fork", nil); !run.completed() || !run.fast() {
			t.Errorf("interleaved write: %+v %v %v", run.rep, run.err, run.stages)
		}
		undo = forkReceipt(t, repo, audited)
	}))
	refreshWriterCheckpoint(ctx, repo)
	defer undo()
	if wc := readWriterCheckpoint(repo); wc == nil || wc.Seq.Uint64() != audited {
		t.Fatalf("refresh retained %+v, want a checkpoint bound to receipt %d", wc, audited)
	}
	before := mutationPublished(t, repo)
	run := writerMutate(t, repo, "after-fork", nil)
	if run.fast() || run.declined() == "" || wire.CodeOf(run.err) != wire.CodeJournalForked {
		t.Fatalf("write over the fork: %+v %v %v", run.rep, run.err, run.stages)
	}
	if after := mutationPublished(t, repo); after != before {
		t.Fatalf("published %s, before %s", after, before)
	}
	sameDecision(t, run, writerOracle(t, repo, writerCreate(repo, "after-fork", WallClock())))
}

// CAL-V0-116 and CAL-V0-117 (proposed): a writer whose tail reaches
// WriterAdvanceTail re-bases the checkpoint on it and keeps FullSeq; the
// write that takes the head WriterRefreshInterval receipts past FullSeq runs
// the scheduled complete audit after its lock is released.
func TestCALV0117_WriterAdvanceAndScheduledRefresh(t *testing.T) {
	repo := writerStore(t, 70)
	full := readWriterCheckpoint(repo).FullSeq
	for headSeqOf(t, repo)+1+journal.WriterAdvanceTail+1-full < journal.WriterRefreshInterval {
		writerPad(t, repo, journal.WriterAdvanceTail)
		observed := headSeqOf(t, repo)
		run := writerMutate(t, repo, fmt.Sprintf("advance-%d", observed), nil)
		if !run.completed() || !run.fast() {
			t.Fatalf("write: %+v %v %v", run.rep, run.err, run.stages)
		}
		if wc := readWriterCheckpoint(repo); wc == nil || wc.Seq.Uint64() != observed || wc.FullSeq != full {
			t.Fatalf("advanced to %+v, want seq %d full %d", wc, observed, full)
		}
	}
	writerPad(t, repo, int(full+journal.WriterRefreshInterval-headSeqOf(t, repo)-1))
	run := writerMutate(t, repo, "refresh-due", nil)
	if !run.completed() || !run.fast() || !strings.Contains(strings.Join(run.stages, "\n"), "refresh.audited") {
		t.Fatalf("write: %+v %v %v", run.rep, run.err, run.stages)
	}
	head := headSeqOf(t, repo)
	if wc := readWriterCheckpoint(repo); wc == nil || wc.FullSeq != head || wc.Seq.Uint64() != head {
		t.Fatalf("refresh retained %+v, want a complete checkpoint at %d", wc, head)
	}
	if got := readCheckpointSeq(t, repo); got != head {
		t.Fatalf("read checkpoint at %d, want %d", got, head)
	}
}

// CAL-V0-117 (proposed): a refusing refresh that stops after it replaced the
// invalidation token and before it removed the writer checkpoint leaves a
// checkpoint no writer uses. The checkpoint is bound to the token it was
// published under, so the next write declines the fast route and the
// complete route refuses the corruption the refresh found.
func TestCALV0117_InvalidationSurvivesStopBeforeRemoval(t *testing.T) {
	repo := writerStore(t, 70)
	if run := writerMutate(t, repo, "before-stop", nil); !run.completed() || !run.fast() {
		t.Fatalf("fast write: %+v %v %v", run.rep, run.err, run.stages)
	}
	requestFile, _ := mutationBoundaryFiles(t, repo)
	rewriteFile(t, requestFile, func([]byte) []byte { return []byte("{}\n") })
	type stopped struct{}
	ctx := context.WithValue(context.Background(), mutationStageKey{}, at("refresh.invalidated", func() { panic(stopped{}) }))
	func() {
		defer func() {
			if r := recover(); r != (stopped{}) {
				t.Fatalf("refresh did not stop at the invalidation boundary: %v", r)
			}
		}()
		refreshWriterCheckpoint(ctx, repo)
	}()
	if _, err := os.Lstat(journal.WriterCheckpointPath(repo.StateDir)); err != nil {
		t.Fatalf("the stop removed the writer checkpoint: %v", err)
	}
	if token := writerInvalidation(repo); !token.ok || token.value == "" {
		t.Fatalf("the stop left no invalidation token: %+v", token)
	}
	before := mutationPublished(t, repo)
	run := writerMutate(t, repo, "after-stop", nil)
	if run.fast() || !strings.Contains(run.declined(), "invalidated") || wire.CodeOf(run.err) != wire.CodeJournalForked {
		t.Fatalf("write after the stop: %+v %v %v", run.rep, run.err, run.stages)
	}
	if after := mutationPublished(t, repo); after != before {
		t.Fatalf("published %s, before %s", after, before)
	}
}
