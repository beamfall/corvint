package journal

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// countingSource records which state paths one audit read.
type countingSource struct {
	Native
	reads map[string]int
}

func (s *countingSource) Read(p string, n int) ([]byte, error) {
	s.reads[p]++
	return s.Native.Read(p, n)
}

// checkpointed commits history, takes the checkpoint a complete audit
// supports, and returns it after a codec round trip.
func checkpointed(t *testing.T, repo *fixture.Repo, r Reader) *Checkpoint {
	t.Helper()
	full, err := r.Audit()
	if err != nil {
		t.Fatal(err)
	}
	cp := full.Checkpoint()
	if cp == nil || full.Mode != ModeFull {
		t.Fatalf("complete settled audit yields no checkpoint: mode %s", full.Mode)
	}
	decoded, err := DecodeCheckpoint(cp.Encode())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded.Encode(), cp.Encode()) {
		t.Fatal("checkpoint codec is not a round trip")
	}
	return decoded
}

func ticketPath(id string) string { return "intent/tickets/" + id + ".json" }

func TestCALV0059_CheckpointCodecAndDerivation(t *testing.T) {
	repo, r := setup(t)
	tk := fixture.Ticket("A")
	appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("A"): tk.Encode()}, "", true, true, false)
	cp := checkpointed(t, repo, r)
	if cp.Seq != "2" || cp.QueueID != r.QueueID || cp.PrimaryWorktree != repo.Root {
		t.Fatalf("checkpoint identity: %+v", cp)
	}
	var sawTicket bool
	for _, e := range cp.Entries {
		if strings.HasPrefix(e.Path, "requests/") {
			t.Fatalf("request path retained: %s", e.Path)
		}
		sawTicket = sawTicket || (e.Path == ticketPath("A") && e.Seq == "2" && e.Sha256 != nil && *e.Sha256 == wire.Sum(tk.Encode()))
	}
	if !sawTicket {
		t.Fatal("latest ticket afterimage missing")
	}
	if _, err := os.Stat(CheckpointPath(repo.StateDir)); !os.IsNotExist(err) {
		t.Fatalf("audit wrote a checkpoint: %v", err)
	}
	if rel, err := filepath.Rel(repo.StateDir, CheckpointPath(repo.StateDir)); err != nil || !strings.HasPrefix(rel, "..") {
		t.Fatalf("checkpoint path is inside the state directory: %s", rel)
	}

	// Only a complete, settled, full audit yields a checkpoint.
	r.Checkpoint = cp
	fast, err := r.Audit()
	if err != nil || fast.Mode != ModeCheckpoint || fast.Checkpoint() != nil {
		t.Fatalf("checkpoint derived from a checkpoint: %v mode %s", err, fast.Mode)
	}
	fixture.PlantReceipt(t, repo, 3)
	pending, err := r.Audit()
	requireCode(t, err, wire.CodeRedoPending)
	if pending.Mode != ModeFull || pending.Checkpoint() != nil {
		t.Fatalf("pending audit yields a checkpoint: mode %s", pending.Mode)
	}

	edit := func(f func(o *wire.Object)) []byte {
		v, err := wire.Parse(cp.Encode())
		if err != nil {
			t.Fatal(err)
		}
		f(v.Obj)
		return wire.EncodeFile(v)
	}
	entries := func(o *wire.Object) []wire.Value { v, _ := o.Get("entries"); return v.Arr }
	for name, raw := range map[string][]byte{
		"profile":      edit(func(o *wire.Object) { o.Set("profile", str("taskman-audit-checkpoint/1")) }),
		"unknown key":  edit(func(o *wire.Object) { o.Set("extra", str("x")) }),
		"zero seq":     edit(func(o *wire.Object) { o.Set("seq", str("0")) }),
		"entry ahead":  edit(func(o *wire.Object) { entries(o)[0].Obj.Set("seq", str("3")) }),
		"entry order":  edit(func(o *wire.Object) { e := entries(o); e[0], e[1] = e[1], e[0] }),
		"request path": edit(func(o *wire.Object) { entries(o)[0].Obj.Set("path", str("requests/aa/x.json")) }),
		"outside path": edit(func(o *wire.Object) { entries(o)[0].Obj.Set("path", str("../head.json")) }),
		"not json":     []byte("{"),
	} {
		if _, err := DecodeCheckpoint(raw); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestCALV0061_CheckpointTailEqualsFullAudit(t *testing.T) {
	repo, r := setup(t)
	a := fixture.Ticket("A")
	appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("A"): a.Encode()}, "", true, true, false)
	c := fixture.Ticket("C")
	appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("C"): c.Encode()}, "", true, true, false)
	cp := checkpointed(t, repo, r)

	// No tail: the checkpoint names the head receipt.
	check := func(want string, paths ...string) *Result {
		t.Helper()
		full, err := Reader{Source: r.Source, QueueID: r.QueueID, PrimaryWorktree: r.PrimaryWorktree}.Audit(paths...)
		if err != nil {
			t.Fatal(err)
		}
		src := &countingSource{Native: r.Source.(Native), reads: map[string]int{}}
		fast, err := Reader{Source: src, QueueID: r.QueueID, PrimaryWorktree: r.PrimaryWorktree, Checkpoint: cp}.Audit(paths...)
		if err != nil {
			t.Fatal(err)
		}
		if fast.Mode != ModeCheckpoint || fast.StructuralConsistency != ModeCheckpoint || fast.ProjectionAgreement != "AGREES" || fast.Pending || fast.StagingPresent {
			t.Fatalf("fast result: %s %s %s", fast.Mode, fast.StructuralConsistency, fast.ProjectionAgreement)
		}
		if fast.LastSeq != full.LastSeq || fast.LastSeq != wire.Size(want) || fast.LastReceiptSha256 != full.LastReceiptSha256 || fast.Identity.HeadSha256 != full.Identity.HeadSha256 || fast.Identity.IntentTreeSha256 != full.Identity.IntentTreeSha256 || fast.SemanticCoverage != full.SemanticCoverage {
			t.Fatalf("fast seq %s full seq %s want %s", fast.LastSeq, full.LastSeq, want)
		}
		if len(fast.Records) != len(full.Records) {
			t.Fatalf("records: fast %d full %d", len(fast.Records), len(full.Records))
		}
		for p, f := range full.Records {
			g := fast.Records[p]
			if g.Seq != f.Seq || !equalDigest(g.Sha256, f.Sha256) || !bytes.Equal(g.Raw, f.Raw) {
				t.Fatalf("%s: fast seq %s full seq %s", p, g.Seq, f.Seq)
			}
		}
		for p := range src.reads {
			if !strings.HasPrefix(p, "receipts/") {
				continue
			}
			var seq uint64
			for _, c := range strings.TrimSuffix(strings.TrimPrefix(p, "receipts/"), ".json") {
				seq = seq*10 + uint64(c-'0')
			}
			if seq < cp.Seq.Uint64() {
				t.Fatalf("fast audit read prefix receipt %s", p)
			}
		}
		return fast
	}
	check("3", ticketPath("A"), ticketPath("C"), "reservations.json", "intent/queue.json")

	// Tail: later receipts replace, delete and add paths; a request joins.
	a.Title = "tail"
	appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("A"): a.Encode()}, "", true, true, false)
	c.Title = "tail too"
	appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("C"): c.Encode()}, "", true, true, false)
	b := fixture.Ticket("B")
	appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("B"): b.Encode()}, "", true, true, false)
	fast := check("6", ticketPath("A"), ticketPath("B"), ticketPath("C"), "reservations.json", "intent/queue.json")
	if !bytes.Equal(fast.Records[ticketPath("A")].Raw, a.Encode()) || !bytes.Equal(fast.Records[ticketPath("B")].Raw, b.Encode()) || fast.Records[ticketPath("C")].Seq != "5" {
		t.Fatal("tail not applied")
	}

	// SelectState retains the lease state without naming every attempt.
	state, err := Reader{Source: r.Source, QueueID: r.QueueID, PrimaryWorktree: r.PrimaryWorktree, Checkpoint: cp, SelectState: true}.Audit()
	if err != nil || state.Mode != ModeCheckpoint {
		t.Fatalf("%v mode %s", err, state.Mode)
	}
	if _, ok := state.Records["reservations.json"]; !ok {
		t.Fatal("state not selected")
	}
}

func TestCALV0061_CheckpointFallsBackToFullAudit(t *testing.T) {
	// Each case leaves the checkpoint unusable or the journal unsettled; the
	// read must give exactly the complete audit's answer.
	cases := map[string]struct {
		edit func(t *testing.T, repo *fixture.Repo, cp *Checkpoint)
		want string // "" means OK in FULL mode
	}{
		"other queue": {edit: func(t *testing.T, repo *fixture.Repo, cp *Checkpoint) {
			cp.QueueID, _ = wire.ParseQueueID("", "01ARZ3NDEKTSV4RRFFQ69G5FAW")
		}},
		"other worktree": {edit: func(t *testing.T, repo *fixture.Repo, cp *Checkpoint) { cp.PrimaryWorktree = repo.Root + "-x" }},
		"other init":     {edit: func(t *testing.T, repo *fixture.Repo, cp *Checkpoint) { cp.InitSha256 = wire.Sum([]byte("x")) }},
		"ahead of head":  {edit: func(t *testing.T, repo *fixture.Repo, cp *Checkpoint) { cp.Seq = "9" }},
		"receipt digest": {edit: func(t *testing.T, repo *fixture.Repo, cp *Checkpoint) { cp.ReceiptSha256 = wire.Sum([]byte("x")) }},
		"stale entry": {edit: func(t *testing.T, repo *fixture.Repo, cp *Checkpoint) {
			d := wire.Sum([]byte("x"))
			cp.Entries[0].Sha256 = &d
		}},
		"extra entry": {edit: func(t *testing.T, repo *fixture.Repo, cp *Checkpoint) {
			d := wire.Sum([]byte("x"))
			cp.Entries = append(cp.Entries, CheckpointEntry{Path: "pools.json", Seq: "1", Sha256: &d})
		}},
		"missing entry": {edit: func(t *testing.T, repo *fixture.Repo, cp *Checkpoint) { cp.Entries = cp.Entries[1:] }},
		"pending receipt": {want: wire.CodeRedoPending, edit: func(t *testing.T, repo *fixture.Repo, cp *Checkpoint) {
			fixture.PlantReceipt(t, repo, 4)
		}},
		"staging": {want: wire.CodeMalformed, edit: func(t *testing.T, repo *fixture.Repo, cp *Checkpoint) {
			fixture.Write(t, filepath.Join(repo.StateDir, "staging", "x"), []byte("x"))
		}},
		"intent diverged": {want: wire.CodeIntentDiverged, edit: func(t *testing.T, repo *fixture.Repo, cp *Checkpoint) {
			fixture.Write(t, filepath.Join(repo.IntentDir, "tickets", "A.json"), fixture.Ticket("Z").Encode())
		}},
		"stray intent": {want: wire.CodeIntentDiverged, edit: func(t *testing.T, repo *fixture.Repo, cp *Checkpoint) {
			fixture.Write(t, filepath.Join(repo.IntentDir, "tickets", "C.json"), fixture.Ticket("C").Encode())
		}},
		"state diverged": {want: wire.CodeJournalForked, edit: func(t *testing.T, repo *fixture.Repo, cp *Checkpoint) {
			fixture.Write(t, filepath.Join(repo.StateDir, "reservations.json"), []byte("{\"other\":\"1\"}\n"))
		}},
		"tail receipt malformed": {want: wire.CodeJournalForked, edit: func(t *testing.T, repo *fixture.Repo, cp *Checkpoint) {
			rewrite(t, repo, 3, func(v wire.Value) { v.Obj.Set("prev", str(string(wire.Sum([]byte("wrong"))))) })
		}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo, r := setup(t)
			a := fixture.Ticket("A")
			appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("A"): a.Encode()}, "", true, true, false)
			cp := checkpointed(t, repo, r)
			a.Title = "tail"
			appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("A"): a.Encode()}, "", true, true, false)
			tc.edit(t, repo, cp)
			full, fullErr := r.Audit(ticketPath("A"))
			r.Checkpoint = cp
			got, err := r.Audit(ticketPath("A"))
			requireCode(t, err, tc.want)
			requireCode(t, fullErr, tc.want)
			if got == nil || full == nil {
				if got != full {
					t.Fatal("fallback differs from full audit")
				}
				return
			}
			if got.Mode != ModeFull || got.StructuralConsistency != full.StructuralConsistency || got.Pending != full.Pending || got.LastSeq != full.LastSeq {
				t.Fatalf("fallback differs from full audit: mode %q structure %s/%s seq %s/%s", got.Mode, got.StructuralConsistency, full.StructuralConsistency, got.LastSeq, full.LastSeq)
			}
		})
	}
}

func TestCALV0061_CheckpointScopeAndMovement(t *testing.T) {
	repo, r := setup(t)
	a := fixture.Ticket("A")
	appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("A"): a.Encode()}, "", true, true, false)
	cp := checkpointed(t, repo, r)
	r.Checkpoint = cp

	// Writer, barrier-removal and reconciliation audits stay complete.
	if res, err := r.BarrierRemoval(); err != nil || res.Mode != ModeFull {
		t.Fatalf("barrier-removal audit used the checkpoint: %v mode %s", err, res.Mode)
	}
	if res, err := r.Reconciliation(fixture.TicketID("A")); err != nil || res.Mode != ModeFull {
		t.Fatalf("reconciliation audit used the checkpoint: %v mode %s", err, res.Mode)
	}
	if res, err := r.AuditForWrite(); err != nil || res.Mode != ModeFull {
		t.Fatalf("writer audit used the checkpoint: %v mode %s", err, res.Mode)
	}

	// A head that moves between the two captures is retried, not returned.
	moved := false
	r.afterCapture = func() {
		if moved {
			return
		}
		moved = true
		a.Title = "moved"
		appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("A"): a.Encode()}, "", true, true, false)
	}
	res, err := r.Audit(ticketPath("A"))
	if err != nil || res.Mode != ModeCheckpoint || res.LastSeq != "3" || !bytes.Equal(res.Records[ticketPath("A")].Raw, a.Encode()) {
		t.Fatalf("movement not retried: %v mode %s", err, res.Mode)
	}
}

// The checkpoint read does not list receipts/, requests/ or evidence/, so it
// cannot see what only a complete inventory shows. `receipt audit` and every
// mutation still run the complete audit, which refuses these.
func TestCALV0061_CheckpointLimitsStayWithFullAudit(t *testing.T) {
	for name, plant := range map[string]func(t *testing.T, repo *fixture.Repo){
		"receipt beyond head+1": func(t *testing.T, repo *fixture.Repo) { fixture.PlantReceipt(t, repo, 6) },
		"altered prefix receipt": func(t *testing.T, repo *fixture.Repo) {
			path := filepath.Join(repo.StateDir, "receipts", "000000000001.json")
			fixture.Write(t, path, append(read(t, path), ' '))
		},
	} {
		t.Run(name, func(t *testing.T) {
			repo, r := setup(t)
			a := fixture.Ticket("A")
			appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("A"): a.Encode()}, "", true, true, false)
			appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("B"): fixture.Ticket("B").Encode()}, "", true, true, false)
			cp := checkpointed(t, repo, r)
			plant(t, repo)
			if _, err := r.Audit(); err == nil {
				t.Fatal("complete audit accepted the journal")
			}
			r.Checkpoint = cp
			if res, err := r.Audit(); err != nil || res.Mode != ModeCheckpoint || res.StructuralConsistency == "CONSISTENT" {
				t.Fatalf("checkpoint read: %v", err)
			}
			if res, err := r.AuditForWrite(); err == nil || res == nil || res.Mode != ModeFull {
				t.Fatalf("writer audit accepted the journal: %v", err)
			}
		})
	}
}
