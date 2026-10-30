package journal

import (
	"crypto/sha256"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestJSGV0003_CompleteAuditVerifiesCarriedValues covers JSG-V0-003: a
// complete audit verifies every value a retained writer checkpoint carries,
// in both its base-free and its base-backed form, after more receipts have
// been appended. Each carried size, digest, sequence, request afterimage,
// aggregate and entry it forges is reported as a difference, and a result
// that cannot verify says so instead of passing.
func TestJSGV0003_CompleteAuditVerifiesCarriedValues(t *testing.T) {
	repo, r := refreshStore(t)
	full, obs, err := r.AuditForWriteObserved()
	if err != nil || full.Mode != ModeFull {
		t.Fatalf("complete audit: %v", err)
	}
	derived, err := full.WriterCheckpoint(obs.Files)
	if err != nil {
		t.Fatal(err)
	}
	base, delta, err := derived.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decode := func() *WriterCheckpoint {
		w, err := DecodeWriterCheckpoint(delta, base)
		if err != nil {
			t.Fatal(err)
		}
		w, err = w.full()
		if err != nil {
			t.Fatal(err)
		}
		return w
	}
	appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("B"): fixture.Ticket("B").Encode()}, "", true, true, false)
	appendRequest(t, repo, "after-checkpoint")
	later, laterObs, err := r.AuditForWriteObserved()
	if err != nil || later.Mode != ModeFull || later.LastSeq.Uint64() != derived.Seq.Uint64()+2 {
		t.Fatalf("later complete audit: %+v %v", later, err)
	}

	for name, old := range map[string]*WriterCheckpoint{"base-free": derived, "base-backed": func() *WriterCheckpoint {
		w, err := DecodeWriterCheckpoint(delta, base)
		if err != nil {
			t.Fatal(err)
		}
		return w
	}()} {
		if ok, err := later.VerifyWriterCheckpoint(old, laterObs.Files); !ok || err != nil {
			t.Fatalf("%s: honest checkpoint ok=%v err=%v", name, ok, err)
		}
	}

	ticket := -1
	for i, e := range derived.Entries {
		if strings.HasPrefix(e.Path, "intent/tickets/") {
			ticket = i
		}
	}
	if ticket < 0 || len(derived.requests) == 0 {
		t.Fatal("fixture carries no ticket entry or request")
	}
	forgeries := map[string]func(w *WriterCheckpoint){
		"entry size":         func(w *WriterCheckpoint) { w.sizes[ticket]++ },
		"entry sequence":     func(w *WriterCheckpoint) { w.Entries[ticket].Seq = wire.SizeOf(w.Entries[ticket].Seq.Uint64() + 1) },
		"entry digest":       func(w *WriterCheckpoint) { w.Entries[ticket].Sha256 = nil },
		"dropped entry":      func(w *WriterCheckpoint) { w.Entries, w.sizes = w.Entries[1:], w.sizes[1:] },
		"request afterimage": func(w *WriterCheckpoint) { w.requests[sha256.Size] ^= 1 },
		"dropped request":    func(w *WriterCheckpoint) { w.requests = w.requests[writerRequestBytes:] },
		"receipt bytes":      func(w *WriterCheckpoint) { w.ReceiptBytes++ },
		"payload bytes":      func(w *WriterCheckpoint) { w.Cost.PayloadBytes++ },
		"tar bytes":          func(w *WriterCheckpoint) { w.Cost.TarBytes++ },
		"file count":         func(w *WriterCheckpoint) { w.Cost.Files++ },
		"init digest":        func(w *WriterCheckpoint) { w.InitSha256 = wire.Sum([]byte("other")) },
	}
	for name, forge := range forgeries {
		w := decode()
		forge(w)
		if ok, err := later.VerifyWriterCheckpoint(w, laterObs.Files); !ok || err == nil {
			t.Errorf("forged %s: ok=%v err=%v, want a difference", name, ok, err)
		}
	}

	// Results that cannot verify say so rather than passing or failing.
	if ok, err := later.VerifyWriterCheckpoint(decode(), nil); ok || err != nil {
		t.Fatalf("no physical observation: ok=%v err=%v", ok, err)
	}
	tail, tailObs, err := r.AuditForWriter(derived, "", false)
	if err != nil || tail.Mode != ModeWriter {
		t.Fatalf("writer audit: %+v %v", tail, err)
	}
	if ok, err := tail.VerifyWriterCheckpoint(decode(), tailObs.Files); ok || err != nil {
		t.Fatalf("writer audit verified a checkpoint: ok=%v err=%v", ok, err)
	}
}
