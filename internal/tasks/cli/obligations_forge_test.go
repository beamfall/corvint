package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// tolForgeWorkerGeneration rewrites the head receipt, a WORKER witness, so
// that its retained request names generation gen: the request, event,
// request entry, ticket reference (head and lastRaise), receipt and journal
// head are all rehashed and the projection rewritten, so every byte is
// self-consistent and only the attempt provenance is untrue.
func tolForgeWorkerGeneration(t *testing.T, root, gen string) {
	t.Helper()
	tolForgeHeadWitness(t, root, func(payload wire.Value) { payload.Obj.Set("generation", wire.String(gen)) }, func(ref wire.Value) {
		if raise, ok := ref.Obj.Get("lastRaise"); ok && raise.Obj != nil {
			raise.Obj.Set("generation", wire.String(gen))
		}
	})
}

// tolForgeHeadWitness rewrites the head receipt's retained witness payload
// with edit (and the record reference with editRef when not nil), rehashing
// every dependent byte as tolForgeWorkerGeneration describes.
func tolForgeHeadWitness(t *testing.T, root string, edit func(payload wire.Value), editRef func(ref wire.Value)) {
	t.Helper()
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	headPath := filepath.Join(repo.StateDir, "head.json")
	hv, err := wire.Parse(readFile(t, headPath))
	if err != nil {
		t.Fatal(err)
	}
	seqv, _ := hv.Obj.Get("lastSeq")
	seq, err := wire.ParseCount("lastSeq", seqv.Str)
	if err != nil {
		t.Fatal(err)
	}
	name, err := snapshot.ReceiptName(uint64(seq.Int()))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo.StateDir, "receipts", name)
	rv, err := wire.Parse(readFile(t, path))
	if err != nil {
		t.Fatal(err)
	}
	posts, _ := rv.Obj.Get("post")
	var old, forged, request wire.Digest
	for _, p := range posts.Arr {
		at, _ := p.Obj.Get("path")
		if !strings.HasPrefix(at.Str, "evidence/") {
			continue
		}
		old = wire.Digest(strings.TrimPrefix(at.Str, "evidence/"))
		ev, err := wire.Parse(readFile(t, filepath.Join(repo.StateDir, "evidence", string(old))))
		if err != nil {
			t.Fatal(err)
		}
		req, _ := ev.Obj.Get("request")
		payload, _ := req.Obj.Get("payload")
		edit(payload)
		request = wire.Sum(wire.EncodeFile(req))
		ev.Obj.Set("requestSha256", wire.String(string(request)))
		raw := wire.EncodeFile(ev)
		forged = wire.Sum(raw)
		fixture.Write(t, filepath.Join(repo.StateDir, "evidence", string(forged)), raw)
		p.Obj.Set("path", wire.String("evidence/"+string(forged))).Set("sha256", wire.String(string(forged))).Set("blobSha256", wire.String(string(forged)))
	}
	if old == "" {
		t.Fatal("the head receipt posts no ledger event")
	}
	pres, _ := rv.Obj.Get("pre")
	for _, p := range pres.Arr {
		if at, _ := p.Obj.Get("path"); at.Str == "evidence/"+string(old) {
			p.Obj.Set("path", wire.String("evidence/"+string(forged)))
		}
	}
	for _, p := range posts.Arr {
		at, _ := p.Obj.Get("path")
		rec, _ := p.Obj.Get("record")
		switch {
		case strings.HasPrefix(at.Str, "intent/tickets/"):
			ref, _ := rec.Obj.Get("obligations")
			ref.Obj.Set("head", wire.String(string(forged)))
			if editRef != nil {
				editRef(ref)
			}
			fixture.Write(t, filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets", filepath.Base(at.Str)), wire.EncodeFile(rec))
		case strings.HasPrefix(at.Str, "requests/"):
			rec.Obj.Set("mutationSha256", wire.String(string(request)))
			if _, err := os.Stat(filepath.Join(repo.StateDir, at.Str)); err == nil {
				fixture.Write(t, filepath.Join(repo.StateDir, at.Str), wire.EncodeFile(rec))
			}
		default:
			continue
		}
		p.Obj.Set("sha256", wire.String(string(wire.Sum(wire.EncodeFile(rec)))))
	}
	out := wire.EncodeFile(rv)
	fixture.Write(t, path, out)
	hv.Obj.Set("lastReceiptSha256", wire.String(string(wire.Sum(out))))
	fixture.Write(t, headPath, wire.EncodeFile(hv))
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// TestTOLV0013_TamperedWorkerGenerationInconsistent: a WORKER witness whose
// retained request is rewritten, self-consistently, to an attempt
// generation the attempt ledger never recorded fails receipt audit as
// JOURNAL_FORKED; the same rewrite to the true generation (the control)
// stays consistent, so the refusal is the replayed provenance alone.
func TestTOLV0013_TamperedWorkerGenerationInconsistent(t *testing.T) {
	defer cli.SetObligationQualifiedVersions([]string{pwVersion})()
	for _, forge := range []bool{false, true} {
		r, id, head, attempt, gen := workerObligationRepo(t)
		x := atm(t, r.Root, nil, witnessArgs(id, "w-ok", obligationRevision(t, r.Root, id), head, "--role", "WORKER",
			"--from-playwright-report", obligationCaseReport(t, r.Root), "--attempt", attempt, "--generation", gen)...)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("WORKER witness: %s", x.stdout)
		}
		to := gen
		if forge {
			to = "2"
		}
		tolForgeWorkerGeneration(t, r.Root, to)
		a := atm(t, r.Root, nil, "receipt", "audit")
		if forge == auditConsistent(t, r.Root) || forge != strings.Contains(string(a.stdout), "obligation binding: the write does not replay") {
			t.Fatalf("forged=%v receipt audit: %s", forge, a.stdout)
		}
	}
}

// TestTOLV0013_DeclaredCommitAudited: a DECLARED witness rewritten, with
// every byte rehashed, to name a commit the repository lacks fails receipt
// audit; the shape and grant still replay, so only the commit check refuses.
func TestTOLV0013_DeclaredCommitAudited(t *testing.T) {
	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	if x := atm(t, r.Root, nil, declareArgs(id, "w-1", obligationRevision(t, r.Root, id), head, "AC-1")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("declared: %s", x.stdout)
	}
	if !auditConsistent(t, r.Root) {
		t.Fatal("the true declared witness does not audit")
	}
	tolForgeHeadWitness(t, r.Root, func(payload wire.Value) { payload.Obj.Set("commit", wire.String(strings.Repeat("c", 40))) }, nil)
	a := atm(t, r.Root, nil, "receipt", "audit")
	if auditConsistent(t, r.Root) || !strings.Contains(string(a.stdout), "holds no commit") {
		t.Fatalf("a declared witness at a missing commit audited: %s", a.stdout)
	}
}
