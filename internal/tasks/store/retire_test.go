package store_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// retirable is a lease store whose only attempt was claimed, heartbeated and
// released, so every attempt is terminal and nothing is held: an eligible
// anchor (JRT-V0-007). It returns the store and a second OPEN ticket for
// later lease writes, whose lease clock runs from minute 100.
func retirable(t *testing.T, heartbeats int) (*leaseStore, string) {
	t.Helper()
	s := newLeaseStore(t)
	spare := s.ticket(t, "spare")
	id := s.ticket(t, "retire")
	c := s.claim(t, "claim-retire", id, 1)
	hb := transaction.LeaseRequest{Verb: transaction.LeaseHeartbeat, AttemptID: c.AttemptID, Generation: c.Generation}
	for i := 0; i < heartbeats; i++ {
		if r := s.lease(t, "hb-"+strconv.Itoa(i), hb, 2+i, store.NoScopeDeriver); r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("heartbeat %d: %+v", i, r)
		}
	}
	if r := s.lease(t, "release-retire", releaseOf(c), 3+heartbeats, store.NoScopeDeriver); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("release: %+v", r)
	}
	return s, spare
}

func retireReader(t *testing.T, s *leaseStore) journal.Reader {
	t.Helper()
	q, err := wire.ParseQueueID("queueId", fixture.QueueID)
	if err != nil {
		t.Fatal(err)
	}
	return journal.Reader{Source: journal.Native{StateDir: s.repo.StateDir, PrimaryWorktree: s.repo.PrimaryWorktree}, QueueID: q, PrimaryWorktree: s.repo.PrimaryWorktree}
}

// retiredAuditOK runs the complete receipt audit and the material binding
// folds and requires them to start at anchor (0: receipt 1).
func retiredAuditOK(t *testing.T, s *leaseStore, anchor uint64, label string) *journal.Result {
	t.Helper()
	proof, err := retireReader(t, s).Audit()
	if err != nil || proof.StructuralConsistency != "CONSISTENT" || proof.ProjectionAgreement != "AGREES" || proof.Mode != journal.ModeFull {
		t.Fatalf("%s: complete audit %+v %v", label, proof, err)
	}
	if got := proof.RetiredThrough(); got != anchor {
		t.Fatalf("%s: audit root %d, want %d", label, got, anchor)
	}
	if err := store.FoldReceiptBindings(s.repo, proof.LastSeq.Uint64(), nil); err != nil {
		t.Fatalf("%s: binding folds: %v", label, err)
	}
	return proof
}

func refusesCode(t *testing.T, err error, code string, label string) {
	t.Helper()
	if wire.CodeOf(err) != code {
		t.Fatalf("%s: got %v, want %s", label, err, code)
	}
}

func retireArchive(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "retired.tar")
}

// TestJRTV0002_RetireShrinksJournalAndRollsBack (JRT-V0-002..JRT-V0-005,
// JRT-V0-010, JRT-V0-011): a dry run writes nothing; retire keeps the anchor
// and drops receipts 1..S-1 only after the verified archive is bound into the
// seal; the complete audit and binding folds then start at the anchor; later
// lease writes and mutations commit and audit; export, compaction and a
// second retire refuse; unretire restores every retired receipt byte for
// byte, keeps the later receipts, and the audit starts at receipt 1 again.
func TestJRTV0002_RetireShrinksJournalAndRollsBack(t *testing.T) {
	t.Parallel()
	s, id2 := retirable(t, 6)
	before := receiptFiles(t, s)
	head := retiredAuditOK(t, s, 0, "before").LastSeq.Uint64()

	dry, err := store.RetirePrefix(context.Background(), s.repo, "", true)
	if err != nil || !dry.DryRun || dry.AnchorSeq != head || dry.Retired != head-1 || dry.ReceiptBytesAfter >= dry.ReceiptBytesBefore {
		t.Fatalf("dry run %+v %v", dry, err)
	}
	if !sameFiles(receiptFiles(t, s), before) {
		t.Fatal("dry run changed a receipt")
	}
	if _, err := os.Lstat(filepath.Join(s.repo.StateDir, journal.RetiredPrefixFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry run wrote a seal: %v", err)
	}

	path := retireArchive(t)
	rep, err := store.RetirePrefix(context.Background(), s.repo, path, false)
	if err != nil || rep.AnchorSeq != head || rep.Retired != head-1 || rep.ArchiveSha256 == "" {
		t.Fatalf("retire %+v %v", rep, err)
	}
	after := receiptFiles(t, s)
	anchorName, _ := snapshot.ReceiptName(head)
	if len(after) != 1 || !bytes.Equal(after[anchorName], before[anchorName]) {
		t.Fatalf("retire kept %d receipts, want only the anchor", len(after))
	}
	stream, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if v, err := archive.Verify(bytes.NewReader(stream)); err != nil || v.Manifest.ExportedAtSeq.Uint64() != head {
		t.Fatalf("retained archive: %v", err)
	}
	if wire.Sum(stream) != rep.ArchiveSha256 || uint64(len(stream)) != rep.ArchiveBytes {
		t.Fatal("report does not name the retained archive")
	}
	retiredAuditOK(t, s, head, "sealed")

	// Later writes on the sealed store commit and audit from the anchor.
	c2 := s.claim(t, "claim-after", id2, 100)
	if r := s.lease(t, "renew-after", renewOf(c2), 101, store.NoScopeDeriver); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("renew after seal: %+v", r)
	}
	if r := s.lease(t, "release-after", releaseOf(c2), 102, store.NoScopeDeriver); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("release after seal: %+v", r)
	}
	later := retiredAuditOK(t, s, head, "sealed+writes").LastSeq.Uint64()
	if later <= head {
		t.Fatal("no receipt after the anchor")
	}

	_, err = archive.Export(archive.ExportOptions{Repo: s.repo, Staging: t.TempDir(), Stdout: &bytes.Buffer{}})
	refusesCode(t, err, wire.CodeUnsupported, "export of a sealed store")
	_, err = store.CompactLiveness(context.Background(), s.repo, true)
	refusesCode(t, err, wire.CodeUnsupported, "compaction of a sealed store")
	_, err = store.RetirePrefix(context.Background(), s.repo, retireArchive(t), false)
	refusesCode(t, err, wire.CodeUnsupported, "second retire")

	un, err := store.UnretirePrefix(context.Background(), s.repo, path)
	if err != nil || un.Retired != head-1 {
		t.Fatalf("unretire %+v %v", un, err)
	}
	restored := receiptFiles(t, s)
	for name, raw := range before {
		if !bytes.Equal(restored[name], raw) {
			t.Fatalf("unretire did not restore %s", name)
		}
	}
	if uint64(len(restored)) != later {
		t.Fatalf("unretire left %d receipts, want %d", len(restored), later)
	}
	retiredAuditOK(t, s, 0, "unretired")
	compactAuditOK(t, s, "unretired export")
}

// TestJRTV0007_RetireRefusesIneligibleAnchor (JRT-V0-003, JRT-V0-006,
// JRT-V0-007): a live attempt and its held reservation, a paused store and a
// bad archive target each refuse before anything is written, and the store
// still audits from receipt 1.
func TestJRTV0007_RetireRefusesIneligibleAnchor(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	id := s.ticket(t, "live")
	s.claim(t, "claim-live", id, 1)
	before := receiptFiles(t, s)
	path := retireArchive(t)
	_, err := store.RetirePrefix(context.Background(), s.repo, path, false)
	refusesCode(t, err, wire.CodeAttemptLive, "live attempt in the prefix")
	_, err = store.RetirePrefix(context.Background(), s.repo, path, true)
	refusesCode(t, err, wire.CodeAttemptLive, "live attempt in the prefix (dry run)")
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("refused retire wrote the archive: %v", err)
	}
	if !sameFiles(receiptFiles(t, s), before) {
		t.Fatal("refused retire changed a receipt")
	}
	for _, name := range []string{journal.RetiredPrefixFile, journal.RetireIncompleteFile} {
		if _, err := os.Lstat(filepath.Join(s.repo.StateDir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("refused retire left %s: %v", name, err)
		}
	}
	retiredAuditOK(t, s, 0, "after refusal")

	r, _ := retirable(t, 1)
	inside := filepath.Join(r.repo.StateDir, "..", "inside.tar")
	_, err = store.RetirePrefix(context.Background(), r.repo, inside, false)
	refusesCode(t, err, wire.CodeMalformed, "archive inside the common directory")
	existing := retireArchive(t)
	fixture.Write(t, existing, []byte("keep"))
	_, err = store.RetirePrefix(context.Background(), r.repo, existing, false)
	refusesCode(t, err, wire.CodeMalformed, "existing archive path")
	if raw, _ := os.ReadFile(existing); string(raw) != "keep" {
		t.Fatal("retire replaced an existing file")
	}
	_, err = store.RetirePrefix(context.Background(), r.repo, "", false)
	refusesCode(t, err, wire.CodeMalformed, "no archive")
	if _, err := store.Barrier(context.Background(), r.repo, operator(), barrierRequest(transaction.Pause, "retire-pause"), r.at(t, 50)); err != nil {
		t.Fatalf("pause: %v", err)
	}
	_, err = store.RetirePrefix(context.Background(), r.repo, retireArchive(t), true)
	refusesCode(t, err, wire.CodePaused, "paused store")
}

// sealedStore is a retired store with one later lease write, its anchor and
// its retained archive.
func sealedStore(t *testing.T) (*leaseStore, uint64, string, map[string][]byte, *store.Report) {
	t.Helper()
	s, spare := retirable(t, 3)
	before := receiptFiles(t, s)
	path := retireArchive(t)
	rep, err := store.RetirePrefix(context.Background(), s.repo, path, false)
	if err != nil {
		t.Fatalf("retire: %v", err)
	}
	c := s.claim(t, "claim-sealed", spare, 100)
	retiredAuditOK(t, s, rep.AnchorSeq, "sealed")
	return s, rep.AnchorSeq, path, before, c
}

func flipByte(t *testing.T, path string, at int) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	orig := append([]byte(nil), raw...)
	if at < 0 {
		at += len(raw)
	}
	raw[at] ^= 0x01
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return orig
}

// TestJRTV0006_TamperRefuses (JRT-V0-004, JRT-V0-005, JRT-V0-006,
// JRT-V0-008, JRT-V0-011): an altered archived prefix, an altered or
// re-hashed retained seal, a receipt restored into or linking into the
// retired prefix, and an altered anchor each refuse as JOURNAL_FORKED; the
// complete audit and the lease writers refuse alike, and restoring the bytes
// restores the audit.
func TestJRTV0006_TamperRefuses(t *testing.T) {
	t.Parallel()
	s, anchor, path, before, c := sealedStore(t)
	sealPath := filepath.Join(s.repo.StateDir, journal.RetiredPrefixFile)
	audit := func(label string) error {
		t.Helper()
		_, err := retireReader(t, s).Audit()
		return err
	}

	// Altered archived prefix: unretire refuses and changes nothing.
	orig := flipByte(t, path, 2048)
	_, err := store.UnretirePrefix(context.Background(), s.repo, path)
	refusesCode(t, err, wire.CodeJournalForked, "altered archive")
	if err := os.WriteFile(path, orig, 0o600); err != nil {
		t.Fatal(err)
	}
	retiredAuditOK(t, s, anchor, "after refused unretire")

	// Altered retained seal: its trailer refuses.
	orig = flipByte(t, sealPath, 40)
	refusesCode(t, audit("altered seal"), wire.CodeJournalForked, "altered seal")
	_, err = s.try("note-tampered", renewOf(c), s.at(t, 101))
	refusesCode(t, err, wire.CodeJournalForked, "lease verb over an altered seal")
	if err := os.WriteFile(sealPath, orig, 0o600); err != nil {
		t.Fatal(err)
	}

	// A re-hashed seal whose prefix link or seeds differ refuses.
	seal, err := journal.DecodeRetiredPrefix(orig)
	if err != nil {
		t.Fatal(err)
	}
	forged := *seal
	forged.ElidedSha256 = wire.Sum([]byte("forged"))
	raw, err := forged.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sealPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	refusesCode(t, audit("forged link"), wire.CodeJournalForked, "re-hashed seal with another prefix link")
	forged = *seal
	forged.Seeds = map[string][]byte{}
	for p, b := range seal.Seeds {
		forged.Seeds[p] = b
	}
	forged.Seeds["reservations.json"] = []byte("{}\n")
	if raw, err = forged.Encode(); err == nil {
		if err := os.WriteFile(sealPath, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		refusesCode(t, audit("forged seed"), wire.CodeJournalForked, "re-hashed seal with an altered seed")
	}
	if err := os.WriteFile(sealPath, orig, 0o600); err != nil {
		t.Fatal(err)
	}
	retiredAuditOK(t, s, anchor, "seal restored")

	// A retired receipt put back into the live store refuses.
	first, _ := snapshot.ReceiptName(1)
	back := filepath.Join(s.repo.StateDir, "receipts", first)
	fixture.Write(t, back, before[first])
	refusesCode(t, audit("receipt back in the prefix"), wire.CodeJournalForked, "retired receipt in the live store")
	if err := os.Remove(back); err != nil {
		t.Fatal(err)
	}

	// The anchor receipt altered refuses.
	anchorName, _ := snapshot.ReceiptName(anchor)
	orig = flipByte(t, filepath.Join(s.repo.StateDir, "receipts", anchorName), -3)
	refusesCode(t, audit("altered anchor"), wire.CodeJournalForked, "altered anchor receipt")
	if err := os.WriteFile(filepath.Join(s.repo.StateDir, "receipts", anchorName), orig, 0o600); err != nil {
		t.Fatal(err)
	}

	// A later receipt that links into the retired prefix (its prev names
	// receipt S-1, not the anchor) refuses in the audit and the folds.
	nextName, _ := snapshot.ReceiptName(anchor + 1)
	nextPath := filepath.Join(s.repo.StateDir, "receipts", nextName)
	next, err := os.ReadFile(nextPath)
	if err != nil {
		t.Fatal(err)
	}
	anchorDigest := string(seal.Anchor.ReceiptSha256)
	linked := bytes.Replace(next, []byte(anchorDigest), []byte(string(seal.ElidedSha256)), 1)
	if bytes.Equal(linked, next) {
		t.Fatal("the receipt after the anchor does not name the anchor digest")
	}
	if err := os.WriteFile(nextPath, linked, 0o600); err != nil {
		t.Fatal(err)
	}
	refusesCode(t, audit("link into the prefix"), wire.CodeJournalForked, "later receipt linking into the retired prefix")
	refusesCode(t, store.FoldReceiptBindings(s.repo, anchor+1, nil), wire.CodeJournalForked, "folds over a later receipt linking into the retired prefix")
	_, err = s.try("renew-forked", renewOf(c), s.at(t, 102))
	refusesCode(t, err, wire.CodeJournalForked, "lease verb over a later receipt linking into the retired prefix (CAL-V0-116)")
	if err := os.WriteFile(nextPath, next, 0o600); err != nil {
		t.Fatal(err)
	}
	retiredAuditOK(t, s, anchor, "all restored")
}

// crashAt makes the retire or unretire run stop after stage as a crash would.
func crashAt(stage string) func() {
	return store.SetRetireStageForTest(func(s string) error {
		if s == stage {
			return errors.New("injected crash after " + s)
		}
		return nil
	})
}

// TestJRTV0009_InterruptedRetireIsOldOrNew (JRT-V0-009, CAL-V0-116): a run
// stopped after any durable step leaves the marker that refuses every read
// and write until the next retire or unretire run recovers the store under
// the lock: to the old store before the seal is published, to the new store
// after. The recovered store audits completely as one or the other and lease
// verbs commit on it; no step leaves a mixed store.
func TestJRTV0009_InterruptedRetireIsOldOrNew(t *testing.T) {
	cases := []struct {
		stage, recovered string
		sealedAfter      bool
	}{
		{"exported", "", true},
		{"marked", "ROLLED_BACK", false},
		{"sealed", "ROLLED_FORWARD", true},
		{"dropped", "ROLLED_FORWARD", true},
		{"unmarked", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.stage, func(t *testing.T) {
			s, spare := retirable(t, 2)
			before := receiptFiles(t, s)
			head := retiredAuditOK(t, s, 0, "before").LastSeq.Uint64()
			restore := crashAt(tc.stage)
			_, err := store.RetirePrefix(context.Background(), s.repo, retireArchive(t), false)
			restore()
			if err == nil {
				t.Fatal("injected crash did not stop the run")
			}
			marked := tc.recovered != ""
			if marked {
				_, err := retireReader(t, s).Audit()
				refusesCode(t, err, wire.CodeRestoreIncomplete, "audit of an interrupted retire")
				_, err = s.try("claim-during", claimOf(spare), s.at(t, 100))
				refusesCode(t, err, wire.CodeRestoreIncomplete, "claim on an interrupted retire")
			}
			rep, err := store.RetirePrefix(context.Background(), s.repo, retireArchive(t), false)
			switch {
			case tc.stage == "exported":
				if err != nil || rep.Recovered != "" || rep.AnchorSeq != head {
					t.Fatalf("retire after a crash before the marker: %+v %v", rep, err)
				}
			case tc.stage == "unmarked":
				refusesCode(t, err, wire.CodeUnsupported, "retire of a store sealed before the crash")
			default:
				if err != nil || rep.Recovered != tc.recovered {
					t.Fatalf("recovery %+v %v, want %s", rep, err, tc.recovered)
				}
			}
			if tc.sealedAfter {
				after := receiptFiles(t, s)
				anchorName, _ := snapshot.ReceiptName(head)
				if len(after) != 1 || !bytes.Equal(after[anchorName], before[anchorName]) {
					t.Fatalf("recovered sealed store kept %d receipts, want only the anchor", len(after))
				}
				retiredAuditOK(t, s, head, "recovered sealed")
			} else {
				if !sameFiles(receiptFiles(t, s), before) {
					t.Fatal("recovered old store differs from the store before the run")
				}
				if _, err := os.Lstat(filepath.Join(s.repo.StateDir, journal.RetiredPrefixFile)); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("rolled-back store kept a seal: %v", err)
				}
				retiredAuditOK(t, s, 0, "recovered old")
			}
			c := s.claim(t, "claim-after", spare, 100)
			if r := s.lease(t, "release-after", releaseOf(c), 101, store.NoScopeDeriver); r.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("release after recovery: %+v", r)
			}
		})
	}
}

// TestJRTV0011_InterruptedUnretireIsOldOrNew (JRT-V0-009, JRT-V0-011): an
// unretire stopped after any durable step recovers to the sealed store while
// the seal remains and to the unsealed store once it is gone.
func TestJRTV0011_InterruptedUnretireIsOldOrNew(t *testing.T) {
	cases := []struct {
		stage, recovered string
		sealedAfter      bool
	}{
		{"unretire.marked", "ROLLED_FORWARD", true},
		{"unretire.restored", "ROLLED_FORWARD", true},
		{"unretire.unsealed", "ROLLED_BACK", false},
	}
	for _, tc := range cases {
		t.Run(tc.stage, func(t *testing.T) {
			s, anchor, path, before, _ := sealedStore(t)
			restore := crashAt(tc.stage)
			_, err := store.UnretirePrefix(context.Background(), s.repo, path)
			restore()
			if err == nil {
				t.Fatal("injected crash did not stop the run")
			}
			_, err = retireReader(t, s).Audit()
			refusesCode(t, err, wire.CodeRestoreIncomplete, "audit of an interrupted unretire")
			rep, err := store.UnretirePrefix(context.Background(), s.repo, path)
			if err != nil || rep.Recovered != tc.recovered {
				t.Fatalf("recovery %+v %v, want %s", rep, err, tc.recovered)
			}
			if tc.sealedAfter {
				retiredAuditOK(t, s, anchor, "recovered sealed")
				if _, err := store.UnretirePrefix(context.Background(), s.repo, path); err != nil {
					t.Fatalf("unretire after recovery: %v", err)
				}
			}
			now := receiptFiles(t, s)
			for name, raw := range before {
				if !bytes.Equal(now[name], raw) {
					t.Fatalf("unretired store lacks %s", name)
				}
			}
			retiredAuditOK(t, s, 0, "unretired")
		})
	}
}

// TestJRTV0013_RetireBoundsAuditCost (JRT-V0-013) measures the complete
// audit and the live receipt bytes before and after retire on a store of
// lease renewals; JRT_MEASURE_CYCLES raises the size (default 40).
// The sealed audit reads the anchor and later receipts only, so its cost no
// longer grows with the retired history.
func TestJRTV0013_RetireBoundsAuditCost(t *testing.T) {
	t.Parallel()
	cycles := 40
	if v := os.Getenv("JRT_MEASURE_CYCLES"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			t.Fatalf("JRT_MEASURE_CYCLES=%q", v)
		}
		cycles = n
	}
	s := newLeaseStore(t)
	spare := s.ticket(t, "spare")
	id := s.ticket(t, "cycle")
	c := s.claim(t, "claim-cycle", id, 1)
	for i := 0; i < cycles; i++ {
		if r := s.lease(t, "renew-"+strconv.Itoa(i), renewOf(c), 2+2*i, store.NoScopeDeriver); r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("renew %d: %+v", i, r)
		}
	}
	if r := s.lease(t, "release-cycle", releaseOf(c), 3+2*cycles, store.NoScopeDeriver); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("release: %+v", r)
	}
	var reads map[string]int
	timed := func(label string) (time.Duration, uint64) {
		t.Helper()
		reader := retireReader(t, s)
		counted := &countingSource{inner: reader.Source, reads: map[string]int{}}
		reader.Source = counted
		start := time.Now()
		proof, err := reader.Audit()
		elapsed := time.Since(start)
		if err != nil || proof.Mode != journal.ModeFull {
			t.Fatalf("%s audit: %+v %v", label, proof, err)
		}
		reads = counted.reads
		return elapsed, proof.LastSeq.Uint64()
	}
	sizeOf := func() (n, total uint64) {
		for _, raw := range receiptFiles(t, s) {
			n++
			total += uint64(len(raw))
		}
		return n, total
	}
	beforeTime, head := timed("before")
	beforeN, beforeBytes := sizeOf()
	rep, err := store.RetirePrefix(context.Background(), s.repo, retireArchive(t), false)
	if err != nil {
		t.Fatalf("retire: %v", err)
	}
	if reads["requests"] < cycles || reads["receipts"] < cycles {
		t.Fatalf("unsealed audit read %v, want every receipt and request", reads)
	}
	afterTime, _ := timed("after")
	// The sealed audit reads the anchor receipt and no retired request,
	// however many cycles were retired.
	if reads["receipts"] != 1 || reads["requests"] != 0 {
		t.Fatalf("sealed audit read %v, want one receipt and no request", reads)
	}
	afterN, afterBytes := sizeOf()
	if afterN != 1 || afterBytes >= beforeBytes || rep.ReceiptBytesAfter != afterBytes || rep.ReceiptBytesBefore != beforeBytes {
		t.Fatalf("retire kept %d receipts in %d bytes (report %+v), before %d in %d", afterN, afterBytes, rep, beforeN, beforeBytes)
	}
	t.Logf("JRT-V0-013 receipts %d->%d bytes %d->%d seal %d archive %d audit %s->%s head %d",
		beforeN, afterN, beforeBytes, afterBytes, rep.SealBytes, rep.ArchiveBytes, beforeTime, afterTime, head)
	c = s.claim(t, "claim-after", spare, 4+2*cycles)
	if r := s.lease(t, "release-after", releaseOf(c), 5+2*cycles, store.NoScopeDeriver); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("release after retire: %+v", r)
	}
	retiredAuditOK(t, s, head, "after retire and a later cycle")
}

// TestJRTV0011_UnretireAfterCompaction (JRT-V0-011): a store compacted before
// it was retired has a tombstone at S-1, whose physical bytes differ from the
// digest receipt S's prev names. Unretire binds the digest the tombstone
// records, so claim, heartbeat, release, compact, retire and unretire round-trip
// and the store audits from receipt 1 again.
func TestJRTV0011_UnretireAfterCompaction(t *testing.T) {
	t.Parallel()
	s, _ := retirable(t, 6)
	if rep, err := store.CompactLiveness(context.Background(), s.repo, false); err != nil || rep.Elided == 0 {
		t.Fatalf("compaction %+v %v", rep, err)
	}
	compacted := receiptFiles(t, s)
	head := retiredAuditOK(t, s, 0, "compacted").LastSeq.Uint64()
	prior, _ := snapshot.ReceiptName(head - 1)
	if _, _, elided, err := snapshot.DecodeRetainedReceipt(compacted[prior]); err != nil || !elided {
		t.Fatalf("receipt S-1 is not a tombstone: %v", err)
	}
	path := retireArchive(t)
	if _, err := store.RetirePrefix(context.Background(), s.repo, path, false); err != nil {
		t.Fatalf("retire: %v", err)
	}
	retiredAuditOK(t, s, head, "sealed")
	if _, err := store.UnretirePrefix(context.Background(), s.repo, path); err != nil {
		t.Fatalf("unretire after compaction: %v", err)
	}
	if !sameFiles(receiptFiles(t, s), compacted) {
		t.Fatal("unretire did not restore the compacted receipts byte for byte")
	}
	retiredAuditOK(t, s, 0, "unretired")
}

// TestJRTV0006_ArchiveTargetResolvesLinks (JRT-V0-006): an archive path whose
// parent is a symbolic link into the state directory, the common directory or
// the primary worktree is refused as MALFORMED before anything is written.
func TestJRTV0006_ArchiveTargetResolvesLinks(t *testing.T) {
	r, _ := retirable(t, 1)
	before := receiptFiles(t, r)
	links := t.TempDir()
	for i, target := range []string{r.repo.StateDir, r.repo.CommonDir, r.repo.PrimaryWorktree} {
		link := filepath.Join(links, "alias-"+strconv.Itoa(i))
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(link, "retired.tar")
		_, err := store.RetirePrefix(context.Background(), r.repo, path, false)
		refusesCode(t, err, wire.CodeMalformed, "archive aliased into "+target)
		if _, err := os.Lstat(filepath.Join(target, "retired.tar")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("refused retire wrote through the alias into %s: %v", target, err)
		}
	}
	if !sameFiles(receiptFiles(t, r), before) {
		t.Fatal("refused retire changed a receipt")
	}
	retiredAuditOK(t, r, 0, "after alias refusals")
}

// TestJRTV0006_ArchiveDirectorySyncPrecedesSeal (JRT-V0-006): the archive's
// directory is synced after the archive is written and before the marker,
// seal or drop; a failed sync refuses with nothing dropped and no archive.
func TestJRTV0006_ArchiveDirectorySyncPrecedesSeal(t *testing.T) {
	r, _ := retirable(t, 1)
	before := receiptFiles(t, r)
	path := retireArchive(t)
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	var steps []string
	defer store.SetRetireStageForTest(func(s string) error { steps = append(steps, s); return nil })()
	restore := store.SetSyncArchiveDirForTest(func(dir string) error {
		steps = append(steps, "dirsync:"+dir)
		return errors.New("injected directory sync failure")
	})
	if _, err := store.RetirePrefix(context.Background(), r.repo, path, false); err == nil || len(steps) != 1 || steps[0] != "dirsync:"+parent {
		t.Fatalf("failed sync: %v after %v", err, steps)
	}
	restore()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed sync kept the archive: %v", err)
	}
	for _, name := range []string{journal.RetiredPrefixFile, journal.RetireIncompleteFile} {
		if _, err := os.Lstat(filepath.Join(r.repo.StateDir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("failed sync left %s: %v", name, err)
		}
	}
	if !sameFiles(receiptFiles(t, r), before) {
		t.Fatal("failed sync changed a receipt")
	}
	steps = nil
	defer store.SetSyncArchiveDirForTest(func(dir string) error { steps = append(steps, "dirsync:"+dir); return nil })()
	rep, err := store.RetirePrefix(context.Background(), r.repo, path, false)
	if err != nil || len(steps) < 2 || steps[0] != "dirsync:"+parent || steps[1] != "exported" {
		t.Fatalf("retire: %v after %v", err, steps)
	}
	retiredAuditOK(t, r, rep.AnchorSeq, "sealed")
}

// countingSource counts the audit's reads by top-level directory.
type countingSource struct {
	inner journal.Source
	reads map[string]int
}

func (c *countingSource) Read(p string, max int) ([]byte, error) {
	top, _, _ := strings.Cut(p, "/")
	c.reads[top]++
	return c.inner.Read(p, max)
}

func (c *countingSource) List(p string, max int) (journal.Listing, error) {
	return c.inner.List(p, max)
}

// TestJRTV0008_EscalationAndGateReuseAcrossRetirement (JRT-V0-007,
// JRT-V0-008, JRT-V0-010): a non-terminal ticket carrying escalation
// references refuses ESCALATION_PENDING; once it completes, the store
// retires, and after the anchor a new attempt at the same candidate tree
// reuses a gate PASS executed before it, and opens and answers an
// escalation from its own claim. The complete audit and every binding fold
// accept that history from the anchor, a retired request ID refuses
// UNSUPPORTED when replayed, and unretire restores an audit from receipt 1.
func TestJRTV0008_EscalationAndGateReuseAcrossRetirement(t *testing.T) {
	t.Parallel()
	open, id, src := escalationClaim(t)
	committed(t, escalate(t, open, holder, openRequest(t, "q-1", src, "", ""), 1), "OPEN")
	held := &store.Report{AttemptID: src.AttemptID, Generation: src.Generation}
	if r := open.lease(t, "release-1", releaseOf(held), 2, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("release: %+v", r)
	}
	if _, refs := escalationRefs(t, open, id); refs["q-1"].State != "OPEN" {
		t.Fatalf("refs after release %+v", refs)
	}
	_, err := store.RetirePrefix(context.Background(), open.repo, "", true)
	refusesCode(t, err, wire.CodeEscalationPending, "open ticket carrying escalation references")

	s := newLeaseStore(t, reuseGate("verify", "0", "30", true, true))
	idA, idB := s.ticket(t, "src"), s.ticket(t, "lib")
	a := s.claim(t, "claim-src", idA, 0, "src/")
	srcA := escalationSourceOf(t, s, idA, a)
	b := s.claim(t, "claim-lib", idB, 0, "lib/")
	commit, tree := s.commit(t, "src/a.go", "lib/a.go")
	for i, c := range []*store.Report{a, b} {
		if r := s.lease(t, "submit-"+strconv.Itoa(i), submitOf(c, tree), 1, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("submit %d: %+v", i, r)
		}
	}
	s.passes(t, "gate-src", gateOf(a, "verify"), 2)
	source := s.resultOf(t, a.AttemptID, "verify")
	committed(t, escalate(t, s, holder, openRequest(t, "esc-before", srcA, "", ""), 2), "OPEN")
	committed(t, answer(t, s, operator(), answerRequest(t, "ans-before", idA, operator(), "", ""), 3), "ANSWER")
	if r := s.lease(t, "release-lib", releaseOf(b), 3, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("release: %+v", r)
	}
	if r := s.lease(t, "complete-src", completeOf(a, commit), 4, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("complete: %+v", r)
	}
	path := retireArchive(t)
	rep, err := store.RetirePrefix(context.Background(), s.repo, path, false)
	if err != nil {
		t.Fatalf("retire: %v", err)
	}
	head := rep.AnchorSeq
	retiredAuditOK(t, s, head, "sealed")

	c := s.claim(t, "claim-after", idB, 100, "lib/")
	srcC := escalationSourceOf(t, s, idB, c)
	if r := s.lease(t, "submit-after", submitOf(c, tree), 101, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("submit after the anchor: %+v", r)
	}
	reused, err := s.gate(t, "gate-after", gateOf(c, "verify"), 102)
	if err != nil || reused.Outcome.Outcome != mutation.OutcomeCompleted || reused.GateReusedFrom == nil || string(reused.GateReusedFrom.Result) != source || s.runs(t, "verify") != 1 {
		t.Fatalf("reuse of a PASS from before the anchor: %+v %v", reused, err)
	}
	committed(t, escalate(t, s, holder, openRequest(t, "esc-after", srcC, "", ""), 103), "OPEN")
	committed(t, answer(t, s, operator(), answerRequest(t, "ans-after", idB, operator(), "", ""), 104), "ANSWER")
	if r := s.lease(t, "complete-after", completeOf(c, commit), 105, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("complete after the anchor: %+v", r)
	}
	retiredAuditOK(t, s, head, "sealed+transitions")

	before := receiptFiles(t, s)
	choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "release-lib", Root: s.root, Lease: releaseOf(b)}
	_, err = store.Lease(context.Background(), s.repo, operator(), choice, s.at(t, 106))
	refusesCode(t, err, wire.CodeUnsupported, "replay of a retired lease request")
	_, err = store.Escalate(context.Background(), s.repo, holder, fixture.QueueID, openRequest(t, "esc-before", srcA, "", ""), s.at(t, 106))
	refusesCode(t, err, wire.CodeUnsupported, "replay of a retired escalation request")
	if !sameFiles(receiptFiles(t, s), before) {
		t.Fatal("a refused replay wrote a receipt")
	}

	if _, err := store.UnretirePrefix(context.Background(), s.repo, path); err != nil {
		t.Fatalf("unretire: %v", err)
	}
	retiredAuditOK(t, s, 0, "unretired")
}

// TestJSGV0013_RetiredStoreRefusesJournalMigration checks that a store with
// a retired prefix stays unsegmented: `journal seal` and `journal unseal`
// refuse it as UNSUPPORTED with nothing written, because sealing from the
// anchor is deferred (JSG-V0-013).
func TestJSGV0013_RetiredStoreRefusesJournalMigration(t *testing.T) {
	s, _, _, _, _ := sealedStore(t)
	before := readTree(t, s.repo.StateDir)
	for name, verb := range map[string]func(context.Context, *intent.Repository) (*store.JournalSealReport, error){"seal": store.SealJournal, "unseal": store.UnsealJournal} {
		_, err := verb(context.Background(), s.repo)
		refusesCode(t, err, wire.CodeUnsupported, "journal "+name)
	}
	if after := readTree(t, s.repo.StateDir); after != before {
		t.Fatal("a refused journal migration changed the store")
	}
}
