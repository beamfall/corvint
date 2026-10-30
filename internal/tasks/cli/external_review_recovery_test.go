package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func ergReceiptPath(t *testing.T, repo *intent.Repository, seq uint64) string {
	t.Helper()
	name, err := snapshot.ReceiptName(seq)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(repo.StateDir, "receipts", name)
}

func ergReadValue(t *testing.T, path string) wire.Value {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	v, err := wire.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// ergBlobAttemptPost rewrites receipt seq so that its attempt post is
// blob-backed (record null, blobSha256 naming evidence/<sha>) instead of
// inline: the encoding the journal and redo accept for any post. It returns
// the rewritten receipt bytes; the caller relinks the chain.
func ergBlobAttemptPost(t *testing.T, repo *intent.Repository, seq uint64) []byte {
	t.Helper()
	path := ergReceiptPath(t, repo, seq)
	rv := ergReadValue(t, path)
	posts, _ := rv.Obj.Get("post")
	n := 0
	for _, p := range posts.Arr {
		at, _ := p.Obj.Get("path")
		rec, _ := p.Obj.Get("record")
		if !strings.HasPrefix(at.Str, "attempts/") || rec.Obj == nil {
			continue
		}
		raw := wire.EncodeFile(rec)
		sum := wire.Sum(raw)
		fixture.Write(t, filepath.Join(repo.StateDir, "evidence", string(sum)), raw)
		p.Obj.Set("record", wire.Null()).Set("blobSha256", wire.String(string(sum)))
		n++
	}
	if n == 0 {
		t.Fatalf("receipt %d posts no inline attempt", seq)
	}
	out := wire.EncodeFile(rv)
	fixture.Write(t, path, out)
	return out
}

// TestERGV0006_BlobBackedSubmissionSupersedes covers a newer submission whose
// BUILT attempt record is posted blob-backed: the dispatcher observation
// reads the older PASS as STALE, the writer refuses a verdict on the older
// subject, and receipt audit resolves the blob (ERG-V0-006).
func TestERGV0006_BlobBackedSubmissionSupersedes(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	root, tree := ergStore(t)
	id := planTicket(t, root, "reviewed", "P1", `["src/"]`)
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	runOK := func(args ...string) run {
		t.Helper()
		r := atm(t, root, nil, args...)
		if r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s", args, r.stdout)
		}
		return r
	}
	headSeq := func() uint64 {
		t.Helper()
		n, err := wire.ParseCount("headSeq", field(runOK("receipt", "audit").res.Items[0], "headSeq").Str)
		if err != nil {
			t.Fatal(err)
		}
		return uint64(n.Int())
	}
	c := runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim-a")
	attempt, generation := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	runOK("submit", "--attempt", attempt, "--generation", generation, "--tree", tree, "--request-id", "submit-a")
	subject := wire.SizeOf(headSeq())
	runOK("gate", "record", id, "--gate", ergGate, "--verdict", "PASS", "--subject-receipt", string(subject),
		"--expected-generation", "0", "--expected-revision", "0", "--request-id", "review-1")
	runOK("release", "--attempt", attempt, "--generation", generation, "--reason", wire.CodeHandoff, "--request-id", "release-a")
	c = runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim-b")
	runOK("submit", "--attempt", field(c.res.Items[0], "attemptId").Str, "--generation", field(c.res.Items[0], "generation").Str, "--tree", tree, "--request-id", "submit-b")
	b := headSeq()
	built := ergBlobAttemptPost(t, repo, b)
	editJSONFile(t, filepath.Join(repo.StateDir, "head.json"), func(v wire.Value) { v.Obj.Set("lastReceiptSha256", wire.String(string(wire.Sum(built)))) })

	runOK("receipt", "audit")
	if _, state := ergGateView(t, root, id); state != "STALE" {
		t.Fatalf("a blob-backed newer submission did not stale the PASS: %s", state)
	}
	if x := atm(t, root, nil, "gate", "record", id, "--gate", ergGate, "--verdict", "PASS", "--subject-receipt", string(subject),
		"--expected-generation", "1", "--expected-revision", "1", "--request-id", "review-2"); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("a verdict on a subject superseded by a blob-backed submission was accepted: %s", x.stdout)
	}

	// Scaling: the fold answers supersession from a per-stage index, so the
	// cost of a query does not grow with the number of folded submissions.
	rc, err := snapshot.DecodeReceipt(built)
	if err != nil {
		t.Fatal(err)
	}
	blob := func(d wire.Digest) ([]byte, bool) {
		raw, err := os.ReadFile(filepath.Join(repo.StateDir, "evidence", string(d)))
		return raw, err == nil
	}
	query := func(n uint64) time.Duration {
		var fold transaction.ExternalReviewReceiptAudit
		for seq := uint64(1); seq <= n; seq++ {
			syn := *rc
			syn.Seq = wire.SizeOf(seq)
			syn.Post = ergRebuiltAt(t, repo, rc.Post, syn.Seq)
			if err := fold.Step(&syn, wire.Sum([]byte{byte(seq)}), blob); err != nil {
				t.Fatal(err)
			}
		}
		if !fold.Superseded(id, []string{"implement"}, wire.SizeOf(n-1)) || fold.Superseded(id, []string{"implement"}, wire.SizeOf(n)) {
			t.Fatalf("index after %d submissions answers wrongly", n)
		}
		best := time.Duration(1 << 62)
		for round := 0; round < 5; round++ {
			start := time.Now()
			for i := 0; i < 20000; i++ {
				fold.Superseded(id, []string{"implement", "integrate"}, wire.SizeOf(n))
			}
			if d := time.Since(start); d < best {
				best = d
			}
		}
		return best
	}
	small, large := query(16), query(4096)
	if large > 16*small+5*time.Millisecond {
		t.Fatalf("supersession cost grows with history: %v at 16 submissions, %v at 4096", small, large)
	}
}

// ergRebuiltAt returns posts with every blob-backed attempt record moved to
// phaseSinceSeq seq, each new record published under its own digest.
func ergRebuiltAt(t *testing.T, repo *intent.Repository, posts []snapshot.PostEntry, seq wire.Size) []snapshot.PostEntry {
	t.Helper()
	out := append([]snapshot.PostEntry{}, posts...)
	for i, p := range out {
		if !strings.HasPrefix(p.Path, "attempts/") || p.BlobSha256 == nil {
			continue
		}
		v := ergReadValue(t, filepath.Join(repo.StateDir, "evidence", string(*p.BlobSha256)))
		v.Obj.Set("phaseSinceSeq", wire.String(string(seq)))
		raw := wire.EncodeFile(v)
		sum := wire.Sum(raw)
		fixture.Write(t, filepath.Join(repo.StateDir, "evidence", string(sum)), raw)
		out[i].Sha256, out[i].BlobSha256 = &sum, &sum
	}
	return out
}

// TestERGV0006_RecoveryChecksEveryAuthorStage replays a review receipt that a
// correct writer would refuse: it is recorded on subject A, rewound, and
// re-appended as a pending receipt after a newer submission B. Redo refuses
// it as superseded when B is in any author stage of the gate's retained
// definition (not only A's stage) or is posted blob-backed, and redoes it
// when B's stage is not an author stage (ERG-V0-006/-009).
func TestERGV0006_RecoveryChecksEveryAuthorStage(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	cases := []struct {
		name   string
		stage  string
		blob   bool
		refuse bool
	}{
		{"other-author-stage", "integrate", false, true},
		{"blob-backed", "implement", true, true},
		{"non-author-stage-control", "review", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, tree := ergStore(t)
			ergPolicyUpdateStages(t, root, "3", []string{"implement", "integrate"})
			id := planTicket(t, root, "reviewed", "P1", `["src/"]`)
			local, err := wire.ParseTicketID("", id)
			if err != nil {
				t.Fatal(err)
			}
			repo, err := intent.Resolve(root)
			if err != nil {
				t.Fatal(err)
			}
			runOK := func(args ...string) run {
				t.Helper()
				r := atm(t, root, nil, args...)
				if r.res.Outcome != wire.OutcomeOK {
					t.Fatalf("%v: %s", args, r.stdout)
				}
				return r
			}
			headSeq := func() uint64 {
				t.Helper()
				n, err := wire.ParseCount("headSeq", field(runOK("receipt", "audit").res.Items[0], "headSeq").Str)
				if err != nil {
					t.Fatal(err)
				}
				return uint64(n.Int())
			}
			c := runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim-a")
			attempt, generation := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
			runOK("submit", "--attempt", attempt, "--generation", generation, "--tree", tree, "--request-id", "submit-a")
			subject := headSeq()
			runOK("release", "--attempt", attempt, "--generation", generation, "--reason", wire.CodeHandoff, "--request-id", "release-a")

			headPath := filepath.Join(repo.StateDir, "head.json")
			ticketPath := filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets", local.Local+".json")
			preTicket, err := os.ReadFile(ticketPath)
			if err != nil {
				t.Fatal(err)
			}
			preState := ergFiles(t, repo.StateDir)
			// While A is still the current candidate the writer records
			// the verdict; the receipt is then rewound as if never written.
			runOK("gate", "record", id, "--gate", ergGate, "--verdict", "RETURN", "--subject-receipt", string(wire.SizeOf(subject)),
				"--expected-generation", "0", "--expected-revision", "0", "--request-id", "review-1", "--reason", "TESTS:missing case")
			r := headSeq()
			review := ergReadValue(t, ergReceiptPath(t, repo, r))
			ergRewind(t, repo.StateDir, preState)
			fixture.Write(t, ticketPath, preTicket)

			c = runOK("claim", id, "--holder", "tester", "--stage", tc.stage, "--request-id", "claim-b")
			if field(c.res.Items[0], "attemptId").Str == attempt && field(c.res.Items[0], "generation").Str == generation {
				t.Fatalf("claim B is attempt A's generation")
			}
			runOK("submit", "--attempt", field(c.res.Items[0], "attemptId").Str, "--generation", field(c.res.Items[0], "generation").Str, "--tree", tree, "--request-id", "submit-b")
			r = headSeq()
			bRaw, err := os.ReadFile(ergReceiptPath(t, repo, r))
			if err != nil {
				t.Fatal(err)
			}
			if tc.blob {
				bRaw = ergBlobAttemptPost(t, repo, r)
				editJSONFile(t, headPath, func(v wire.Value) { v.Obj.Set("lastReceiptSha256", wire.String(string(wire.Sum(bRaw)))) })
			}
			bValue, err := wire.Parse(bRaw)
			if err != nil {
				t.Fatal(err)
			}

			// Re-append the review receipt after B, chained, at the next
			// sequence and generation, with its ticket premise current.
			next := wire.SizeOf(r + 1)
			current, err := os.ReadFile(ticketPath)
			if err != nil {
				t.Fatal(err)
			}
			bGen, _ := bValue.Obj.Get("headGeneration")
			gen, err := strconv.ParseUint(bGen.Str, 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			review.Obj.Set("seq", wire.String(string(next))).Set("prev", wire.String(string(wire.Sum(bRaw)))).Set("headGeneration", wire.String(string(wire.SizeOf(gen+1))))
			pres, _ := review.Obj.Get("pre")
			for _, p := range pres.Arr {
				if at, _ := p.Obj.Get("path"); strings.HasPrefix(at.Str, "intent/tickets/") {
					p.Obj.Set("sha256", wire.String(string(wire.Sum(current))))
				}
			}
			posts, _ := review.Obj.Get("post")
			for _, p := range posts.Arr {
				if at, _ := p.Obj.Get("path"); strings.HasPrefix(at.Str, "requests/") {
					rec, _ := p.Obj.Get("record")
					rec.Obj.Set("seq", wire.String(string(next)))
					outcome, _ := rec.Obj.Get("outcome")
					outcome.Obj.Set("receiptSeq", wire.String(string(next)))
					p.Obj.Set("sha256", wire.String(string(wire.Sum(wire.EncodeFile(rec)))))
				}
			}
			fixture.Write(t, ergReceiptPath(t, repo, r+1), wire.EncodeFile(review))
			ergReceiptForge(t, repo, r+1, func(e *snapshot.ExternalReviewEvent) { e.ReceiptSeq = next })
			pendingHead, err := os.ReadFile(headPath)
			if err != nil {
				t.Fatal(err)
			}

			x := atm(t, root, nil, "ticket", "create", "--request-id", "trigger", "--issued-at", "2026-10-04T12:02:00Z", "--payload",
				strings.Replace(createPayloadJSON, `"title":"Console ticket"`, `"title":"trigger"`, 1))
			if !tc.refuse {
				if x.res.Outcome != wire.OutcomeOK {
					t.Fatalf("control redo: %s", x.stdout)
				}
				// B is a newer generation of the same attempt, so the redone
				// RETURN reads STALE; what matters is that it was redone.
				if v, state := ergGateView(t, root, id); v.Revision != "1" || v.Head == "" || state == dispatch.GateNone {
					t.Fatalf("control: the redone verdict is not observed: %s %+v", state, v)
				}
				return
			}
			if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeJournalForked) || !bytes.Contains(x.stdout, []byte("superseded")) {
				t.Fatalf("a verdict on a superseded subject was redone: %s", x.stdout)
			}
			for path, want := range map[string][]byte{headPath: pendingHead, ticketPath: current} {
				if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, want) {
					t.Fatalf("%s changed after the refused redo: %v", path, err)
				}
			}
			if _, err := cli.ObserveTickets(root); wire.CodeOf(err) != wire.CodeRedoPending {
				t.Fatalf("the refused receipt is no longer pending: %v", err)
			}
		})
	}
}

// ergFiles snapshots every regular file under dir.
func ergFiles(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		out[path] = raw
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// ergRewind restores dir to the snapshot, as if the writes since were never
// made, except that retained evidence blobs stay.
func ergRewind(t *testing.T, dir string, snap map[string][]byte) {
	t.Helper()
	for path, raw := range ergFiles(t, dir) {
		want, ok := snap[path]
		switch {
		case !ok && strings.Contains(path, string(filepath.Separator)+"evidence"+string(filepath.Separator)):
		case !ok:
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		case !bytes.Equal(raw, want):
			fixture.Write(t, path, want)
		}
	}
}
