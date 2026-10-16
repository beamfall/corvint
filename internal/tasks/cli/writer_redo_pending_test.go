package cli_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// redoWarning is the completed-redo warning a settling writer reports.
const redoWarning = "a receipt left pending by an interrupted run was completed"

// auditedHeadSeq is the committed head sequence by a complete receipt audit.
func auditedHeadSeq(t *testing.T, root string) int64 {
	t.Helper()
	x := atm(t, root, nil, "receipt", "audit")
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("receipt audit: %s", x.stdout)
	}
	n, err := wire.ParseCount("headSeq", field(x.res.Items[0], "headSeq").Str)
	if err != nil {
		t.Fatal(err)
	}
	return n.Int()
}

// holdPending runs write, which must commit exactly one receipt, then puts
// back head.json and every post that receipt published, so the receipt is
// linked in but head.json does not name it: a writer killed at §5.2 crash
// point C2 with no writer since. It returns the head sequence before the
// write. Two reads both report REDO_PENDING: reads never settle it.
func holdPending(t *testing.T, root string, write func()) int64 {
	t.Helper()
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	seq := auditedHeadSeq(t, root)
	headPath := filepath.Join(repo.StateDir, "head.json")
	destination := func(path string) string {
		if rest, ok := strings.CutPrefix(path, "intent/"); ok {
			return filepath.Join(repo.IntentRoot(), intent.Dir, rest)
		}
		return filepath.Join(repo.StateDir, path)
	}
	before := map[string][]byte{}
	for _, dir := range []string{repo.StateDir, filepath.Join(repo.IntentRoot(), intent.Dir)} {
		if err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			raw, err := os.ReadFile(p)
			before[p] = raw
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	write()
	if got := auditedHeadSeq(t, root); got != seq+1 {
		t.Fatalf("the held write committed %d receipts, want 1", got-seq)
	}
	name, err := snapshot.ReceiptName(uint64(seq + 1))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(repo.StateDir, "receipts", name))
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := snapshot.DecodeReceipt(raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, post := range receipt.Post {
		path := destination(post.Path)
		if old, ok := before[path]; ok {
			fixture.Write(t, path, old)
		} else if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Fatal(err)
		}
	}
	fixture.Write(t, headPath, before[headPath])
	for i := 0; i < 2; i++ {
		if _, err := cli.ObserveTickets(root); wire.CodeOf(err) != wire.CodeRedoPending {
			t.Fatalf("read %d: the rewound head does not hold a pending receipt: %v", i, err)
		}
	}
	return seq
}

// settledBy checks that x completed, reported the settled redo and committed
// the pending receipt and its own (want receipts after the held write's
// base), and that reads succeed afterwards.
func settledBy(t *testing.T, root string, x run, seq, want int64) {
	t.Helper()
	if x.code != 0 || x.res.Outcome != wire.OutcomeOK || hasCode(x.res, wire.CodeRedoPending) {
		t.Fatalf("%v over a held pending receipt: %s", x.res.Command, x.stdout)
	}
	if !bytes.Contains(x.stdout, []byte(redoWarning)) {
		t.Fatalf("%v does not report the redo: %s", x.res.Command, x.stdout)
	}
	if got := auditedHeadSeq(t, root); got != seq+want {
		t.Fatalf("%v: head seq %d, want %d", x.res.Command, got, seq+want)
	}
	if _, err := cli.ObserveTickets(root); err != nil {
		t.Fatalf("%v: read after settling: %v", x.res.Command, err)
	}
}

// heartbeatOnce commits one attempt heartbeat for the store's claimed attempt.
func heartbeatOnce(t *testing.T, root string, a attemptRef, request string) func() {
	return func() {
		t.Helper()
		if x := atm(t, root, nil, "attempt", "heartbeat", "--attempt", a.attempt, "--generation", a.generation, "--request-id", request); x.code != 0 {
			t.Fatalf("heartbeat %s: %s", request, x.stdout)
		}
	}
}

type attemptRef struct{ attempt, generation, ticket, receipt string }

func heldLeaseStore(t *testing.T) (string, attemptRef) {
	t.Helper()
	root, claimed := leaseCLIStore(t, 1, time.Now().UTC().Add(-12*time.Minute))
	t.Setenv("CORVINT_TASKS_ACTOR", "holder")
	c := claimed[0]
	return root, attemptRef{attempt: c.AttemptID, generation: string(c.Generation), ticket: c.Ticket, receipt: c.Receipt}
}

// TestCALV0260_AdministrativeWritersSettleHeldPendingReceipt reproduces
// V1-1353 for each command class, each on its own store: barrier (pause,
// unpause), policy update, ticket escalate, ticket answer, import and
// cutover. Each probed the store once before writing, only to learn the
// queue, and returned the probe's REDO_PENDING, although its store writer
// settles a validated pending receipt under the store lock. Now each settles
// the held receipt, commits its own and reports the completed redo.
func TestCALV0260_AdministrativeWritersSettleHeldPendingReceipt(t *testing.T) {
	t.Run("pause", func(t *testing.T) {
		root, a := heldLeaseStore(t)
		seq := holdPending(t, root, heartbeatOnce(t, root, a, "beat-pause"))
		settledBy(t, root, atm(t, root, nil, "pause", "--request-id", "pause-1"), seq, 2)
	})
	t.Run("unpause", func(t *testing.T) {
		// The pending receipt is the pause itself: unpause settles it first.
		root, _ := heldLeaseStore(t)
		seq := holdPending(t, root, func() {
			if x := atm(t, root, nil, "pause", "--request-id", "pause-1"); x.code != 0 {
				t.Fatalf("pause: %s", x.stdout)
			}
		})
		settledBy(t, root, atm(t, root, nil, "unpause", "--request-id", "unpause-1"), seq, 2)
	})
	t.Run("policy update", func(t *testing.T) {
		root, a := heldLeaseStore(t)
		path := bumpedPolicyFile(t, root, "2")
		seq := holdPending(t, root, heartbeatOnce(t, root, a, "beat-policy"))
		// With production read patience restored, the fence preview (a read
		// taken before the writer) is skipped over the pending receipt
		// rather than waiting the patience out: it reports itself not
		// observed, and the command does not pay the wait.
		old := snapshot.DefaultPatience
		snapshot.DefaultPatience = 5 * time.Second
		defer func() { snapshot.DefaultPatience = old }()
		start := time.Now()
		x := atm(t, root, nil, "policy", "update", "--request-id", "policy-2", "--expected-policy-version", "1", "--file", path)
		elapsed := time.Since(start)
		snapshot.DefaultPatience = old
		settledBy(t, root, x, seq, 2)
		if !bytes.Contains(x.stdout, []byte("handoff fence preview NOT_OBSERVED (REDO_PENDING)")) {
			t.Fatalf("policy update over a pending receipt claims a fence preview: %s", x.stdout)
		}
		if elapsed >= 5*time.Second {
			t.Fatalf("policy update waited %v of read patience before settling the receipt", elapsed)
		}
	})
	escalate := func(t *testing.T, root string, a attemptRef) run {
		return atm(t, root, nil, "ticket", "escalate", "--attempt", a.attempt, "--claim-receipt", a.receipt,
			"--kind", "decision", "--question", "Which store?", "--options", "x,y", "--request-id", "q-1")
	}
	t.Run("ticket escalate", func(t *testing.T) {
		root, a := heldLeaseStore(t)
		seq := holdPending(t, root, heartbeatOnce(t, root, a, "beat-escalate"))
		settledBy(t, root, escalate(t, root, a), seq, 2)
	})
	t.Run("ticket answer", func(t *testing.T) {
		root, a := heldLeaseStore(t)
		if x := escalate(t, root, a); x.code != 0 {
			t.Fatalf("escalate: %s", x.stdout)
		}
		seq := holdPending(t, root, heartbeatOnce(t, root, a, "beat-answer"))
		settledBy(t, root, atm(t, root, nil, "ticket", "answer", "--target", a.ticket, "--text", "x", "--request-id", "ans-1"), seq, 2)
	})
	// Import and cutover run on a store whose canonical writer is the roadmap.
	roadmap := func(t *testing.T) string {
		r := fixture.TempRepo(t)
		q := fixture.QueueValue()
		q.Obj.Set("canonicalWriter", wire.String("ROADMAP"))
		fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), wire.EncodeFile(q))
		fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
		if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("init: %+v", x.res)
		}
		return r.Root
	}
	export := func(t *testing.T) string {
		dir, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "export.jsonl")
		fixture.Write(t, path, []byte(importExportJSONL))
		return path
	}
	t.Run("import", func(t *testing.T) {
		// The pending receipt is a policy update.
		root := roadmap(t)
		policy := bumpedPolicyFile(t, root, "2")
		seq := holdPending(t, root, func() {
			if x := atm(t, root, nil, "policy", "update", "--request-id", "policy-2", "--expected-policy-version", "1", "--file", policy); x.code != 0 {
				t.Fatalf("policy update: %s", x.stdout)
			}
		})
		settledBy(t, root, atm(t, root, nil, "import", "--file", export(t)), seq, 2)
	})
	t.Run("import retry", func(t *testing.T) {
		// The pending receipt is the import's own last batch: the retry of
		// the same export settles it and plans nothing, and still reports it.
		root := roadmap(t)
		path := export(t)
		seq := holdPending(t, root, func() {
			if x := atm(t, root, nil, "import", "--file", path); x.code != 0 {
				t.Fatalf("import: %s", x.stdout)
			}
		})
		x := atm(t, root, nil, "import", "--file", path)
		settledBy(t, root, x, seq, 1)
		if w := field(x.res.Items[0], "written").Str; w != "0" {
			t.Fatalf("retried import wrote %s records, want 0: %s", w, x.stdout)
		}
	})
	t.Run("cutover", func(t *testing.T) {
		// The pending receipt is the import the cutover follows.
		root := roadmap(t)
		path := export(t)
		seq := holdPending(t, root, func() {
			if x := atm(t, root, nil, "import", "--file", path); x.code != 0 {
				t.Fatalf("import: %s", x.stdout)
			}
		})
		settledBy(t, root, atm(t, root, nil, "cutover", "--decision", "decision-redo"), seq, 2)
	})
}

// bumpedPolicyFile writes the store's current policy with policyVersion n to
// a temporary file outside the repository.
func bumpedPolicyFile(t *testing.T, root, n string) string {
	t.Helper()
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(repo.IntentRoot(), intent.Dir, "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	v, err := wire.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	v.Obj.Set("policyVersion", wire.String(n))
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "policy-"+n+".json")
	fixture.Write(t, path, wire.EncodeFile(v))
	return path
}

// TestCALV0262_WriterHelpNamesRedo: the verbose help of every writer that
// settles a pending receipt states that REDO_PENDING does not refuse it and
// that reads never settle one.
func TestCALV0262_WriterHelpNamesRedo(t *testing.T) {
	root := fixture.TempRepo(t).Root
	for _, verb := range [][]string{{"pause"}, {"unpause"}, {"policy", "update"}, {"import"}, {"cutover"},
		{"ticket", "escalate"}, {"ticket", "answer"}, {"run"}, {"health"}, {"pool", "cleanup"}} {
		x := atm(t, root, nil, append(verb, "--help", "--verbose")...)
		if x.code != 0 || len(x.res.Items) != 1 {
			t.Fatalf("%v --help: %s", verb, x.stdout)
		}
		note := field(x.res.Items[0], "redoPending").Str
		for _, want := range []string{"REDO_PENDING", "does not refuse", "settles", "under the store lock", "never settle"} {
			if !strings.Contains(note, want) {
				t.Fatalf("%v help redoPending lacks %q: %q", verb, want, note)
			}
		}
	}
}
