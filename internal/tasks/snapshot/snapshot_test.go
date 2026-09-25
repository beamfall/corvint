package snapshot_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func code(err error) string { return wire.CodeOf(err) }

// TestTMV0008_AS07_ProbeStates covers every state a read must report
// verbatim: UNINITIALIZED, UNSUPPORTED_VERSION, RESTORE_INCOMPLETE,
// REDO_PENDING, JOURNAL_FORKED (N4b, head ahead, digest mismatch) and the
// normal case; and that none of them writes anything.
func TestTMV0008_AS07_ProbeStates(t *testing.T) {
	r := fixture.TempRepo(t)
	if _, err := snapshot.Probe(r.StateDir); code(err) != wire.CodeUninitialized {
		t.Errorf("absent state dir: %v", err)
	}
	if snapshot.Exists(r.StateDir) {
		t.Errorf("Exists on absent dir")
	}
	fixture.WriteState(t, r)
	before := fixture.TreeSnapshot(t, r.StateDir)
	s, err := snapshot.Probe(r.StateDir)
	if err != nil || s.Head.LastSeq != "1" || s.Barrier != nil || s.LastReceiptRaw == nil {
		t.Fatalf("normal probe: %+v %v", s, err)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Errorf("probe modified the state dir")
	}
	// REDO_PENDING: <lastSeq+1> present; the partial snapshot carries the head.
	fixture.PlantReceipt(t, r, 2)
	s, err = snapshot.Probe(r.StateDir)
	if code(err) != wire.CodeRedoPending || s == nil || s.Head == nil || s.Head.LastSeq != "1" {
		t.Errorf("redo pending: %v (snapshot %+v)", err, s)
	}
	// N4b: <lastSeq+2> present with <lastSeq+1> present → JOURNAL_FORKED, not REDO_PENDING.
	fixture.PlantReceipt(t, r, 3)
	if _, err = snapshot.Probe(r.StateDir); code(err) != wire.CodeJournalForked {
		t.Errorf("fork with pending: %v", err)
	}
	// N4b: <lastSeq+2> present with <lastSeq+1> absent.
	os.Remove(filepath.Join(r.StateDir, "receipts", "000000000002.json"))
	if _, err = snapshot.Probe(r.StateDir); code(err) != wire.CodeJournalForked {
		t.Errorf("fork without pending: %v", err)
	}
	os.Remove(filepath.Join(r.StateDir, "receipts", "000000000003.json"))
	if _, err = snapshot.Probe(r.StateDir); err != nil {
		t.Fatalf("restored: %v", err)
	}
	// Head ahead of receipts.
	fixture.Commit(t, r, "MUTATION")
	os.Remove(filepath.Join(r.StateDir, "receipts", "000000000002.json"))
	if _, err = snapshot.Probe(r.StateDir); code(err) != wire.CodeJournalForked {
		t.Errorf("head ahead: %v", err)
	}
	// Receipt digest differs from head.
	fixture.Write(t, filepath.Join(r.StateDir, "receipts", "000000000002.json"), wire.EncodeFile(fixture.ReceiptValue(2, nil, "MUTATION", 0)))
	if _, err = snapshot.Probe(r.StateDir); code(err) != wire.CodeJournalForked {
		t.Errorf("digest mismatch: %v", err)
	}
	// RESTORE_INCOMPLETE marker.
	fixture.Write(t, filepath.Join(r.StateDir, "RESTORE_INCOMPLETE"), []byte("x"))
	if _, err = snapshot.Probe(r.StateDir); code(err) != wire.CodeRestoreIncomplete {
		t.Errorf("restore marker: %v", err)
	}
	os.Remove(filepath.Join(r.StateDir, "RESTORE_INCOMPLETE"))
	// Wrong VERSION is UNSUPPORTED_VERSION and never migrated.
	fixture.Write(t, filepath.Join(r.StateDir, "VERSION"), []byte("taskman-state/1\n"))
	if _, err = snapshot.Probe(r.StateDir); code(err) != wire.CodeUnsupportedVersion {
		t.Errorf("version: %v", err)
	}
	if raw, _ := os.ReadFile(filepath.Join(r.StateDir, "VERSION")); string(raw) != "taskman-state/1\n" {
		t.Errorf("VERSION was rewritten")
	}
	// Absent head.json is UNINITIALIZED.
	fixture.Write(t, filepath.Join(r.StateDir, "VERSION"), []byte(snapshot.VersionBytes))
	os.Remove(filepath.Join(r.StateDir, "head.json"))
	if _, err = snapshot.Probe(r.StateDir); code(err) != wire.CodeUninitialized {
		t.Errorf("no head: %v", err)
	}
}

// TestTMV0008_AS36_ReadRetriesThenSnapshotMoved: a store that commits on
// every probe exhausts the three retries and reports SNAPSHOT_MOVED with the
// body run exactly four times.
func TestTMV0008_AS36_ReadRetriesThenSnapshotMoved(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.WriteState(t, r)
	bodies := 0
	rd := snapshot.Reader{StateDir: r.StateDir}
	_, err := rd.Read(func(s *snapshot.Snapshot) error {
		bodies++
		fixture.Commit(t, r, "MUTATION")
		return nil
	})
	if code(err) != wire.CodeSnapshotMoved || bodies != 4 {
		t.Errorf("err %v bodies %d", err, bodies)
	}
	// A quiet store reads on the first attempt and returns the snapshot.
	bodies = 0
	s, err := rd.Read(func(s *snapshot.Snapshot) error { bodies++; return nil })
	if err != nil || bodies != 1 || s.Head.LastSeq != "5" {
		t.Errorf("quiet read: %v bodies %d seq %s", err, bodies, s.Head.LastSeq)
	}
	// A barrier appearing between probe and re-probe also moves the snapshot.
	rd.Retries = 1
	bodies = 0
	_, err = rd.Read(func(s *snapshot.Snapshot) error {
		bodies++
		if bodies == 1 {
			b := wire.NewObject()
			b.Set("profile", wire.String(snapshot.ProfileBarrier))
			b.Set("queueId", wire.String(fixture.QueueID))
			b.Set("scope", wire.String("ADMISSION"))
			b.Set("reason", wire.String("OPERATOR"))
			b.Set("actor", wire.String("op"))
			b.Set("sinceSeq", wire.String("5"))
			b.Set("since", wire.String(fixture.Timestamp))
			fixture.Write(t, filepath.Join(r.StateDir, "barrier.json"), wire.EncodeFile(wire.ObjectValue(b)))
		}
		return nil
	})
	if err != nil || bodies != 2 {
		t.Errorf("barrier appearance must retry once then succeed: %v bodies %d", err, bodies)
	}
	s, err = snapshot.Probe(r.StateDir)
	if err != nil || s.Barrier == nil || s.Barrier.Scope != "ADMISSION" {
		t.Errorf("barrier not read: %v", err)
	}
	env := s.EnvelopeSnapshot(wire.Sum([]byte("p")), false)
	if env.Barrier == nil || env.Barrier.Reason != "OPERATOR" || env.HeadSeq == nil || *env.HeadSeq != "5" {
		t.Errorf("envelope snapshot: %+v", env)
	}
}

// TestTMV0002_AS01_ReceiptAndHeadSchemas covers the read decoders for
// receipts and heads: closed keys, chain fields, inline post bounds.
func TestTMV0002_AS01_ReceiptAndHeadSchemas(t *testing.T) {
	d := wire.Sum([]byte("x"))
	rc := wire.EncodeFile(fixture.ReceiptValue(1, nil, "INIT", 0))
	if _, err := snapshot.DecodeReceipt(rc); err != nil {
		t.Fatalf("fixture receipt: %v", err)
	}
	if _, err := snapshot.DecodeReceipt(wire.EncodeFile(fixture.ReceiptValue(1, &d, "INIT", 0))); code(err) != wire.CodeMalformed {
		t.Errorf("seq 1 with prev: %v", err)
	}
	if _, err := snapshot.DecodeReceipt(wire.EncodeFile(fixture.ReceiptValue(2, nil, "MUTATION", 0))); code(err) != wire.CodeMalformed {
		t.Errorf("seq 2 without prev: %v", err)
	}
	if _, err := snapshot.DecodeReceipt(wire.EncodeFile(fixture.ReceiptValue(0, nil, "INIT", 0))); code(err) != wire.CodeMalformed {
		t.Errorf("seq 0: %v", err)
	}
	if _, err := snapshot.DecodeReceipt(wire.EncodeFile(fixture.ReceiptValue(1, nil, "BOGUS", 0))); code(err) != wire.CodeMalformed {
		t.Errorf("unknown kind: %v", err)
	}
	// Post entries: exactly one of record/blobSha256; ≤8 inline; ≤64 KiB each.
	post := func(n int, blob bool, big bool) wire.Value {
		v := fixture.ReceiptValue(2, &d, "MUTATION", 0)
		var entries, pres []wire.Value
		for i := 0; i < n; i++ {
			e := wire.NewObject()
			path := fmt.Sprintf("intent/tickets/A%02d.json", i)
			e.Set("path", wire.String(path))
			pre := wire.NewObject()
			pre.Set("path", wire.String(path))
			pre.Set("sha256", wire.Null())
			pres = append(pres, wire.ObjectValue(pre))
			e.Set("sha256", wire.String(string(d)))
			if blob {
				e.Set("record", wire.Null())
				e.Set("blobSha256", wire.String(string(d)))
			} else {
				rec := wire.NewObject()
				if big {
					rec.Set("body", wire.String(strings.Repeat("x", wire.MaxInlinePostEntryBytes)))
				}
				e.Set("record", wire.ObjectValue(rec))
				e.Set("sha256", wire.String(string(wire.Sum(wire.EncodeFile(wire.ObjectValue(rec))))))
				e.Set("blobSha256", wire.Null())
			}
			entries = append(entries, wire.ObjectValue(e))
		}
		v.Obj.Set("post", wire.Array(entries...))
		v.Obj.Set("pre", wire.Array(pres...))
		return v
	}
	if _, err := snapshot.DecodeReceipt(wire.EncodeFile(post(8, false, false))); err != nil {
		t.Errorf("8 inline entries: %v", err)
	}
	if _, err := snapshot.DecodeReceipt(wire.EncodeFile(post(9, false, false))); code(err) != wire.CodeLimitExceeded {
		t.Errorf("9 inline entries: %v", err)
	}
	if _, err := snapshot.DecodeReceipt(wire.EncodeFile(post(9, true, false))); err != nil {
		t.Errorf("9 blob entries are fine: %v", err)
	}
	if _, err := snapshot.DecodeReceipt(wire.EncodeFile(post(1, false, true))); code(err) != wire.CodeLimitExceeded {
		t.Errorf("oversized inline entry: %v", err)
	}
	both := post(1, false, false)
	pe, _ := both.Obj.Get("post")
	pe.Arr[0].Obj.Set("blobSha256", wire.String(string(d)))
	if _, err := snapshot.DecodeReceipt(wire.EncodeFile(both)); code(err) != wire.CodeMalformed {
		t.Errorf("record and blob both set: %v", err)
	}
	// Head: lastSeq ≥ 1 with a digest; closed keys; primaryWorktree is a
	// PathText (§2): absolute, ≤4096 bytes, longer than an Identifier may be.
	h := fixture.HeadValue("/tmp/x", 1, d, 0, d)
	if _, err := snapshot.DecodeHead(wire.EncodeFile(h)); err != nil {
		t.Errorf("fixture head: %v", err)
	}
	h.Obj.Set("lastSeq", wire.String("0"))
	if _, err := snapshot.DecodeHead(wire.EncodeFile(h)); code(err) != wire.CodeMalformed {
		t.Errorf("lastSeq 0: %v", err)
	}
	h = fixture.HeadValue("/"+strings.Repeat("p", 200), 1, d, 0, d)
	if _, err := snapshot.DecodeHead(wire.EncodeFile(h)); err != nil {
		t.Errorf("primaryWorktree over 128 bytes must be accepted as PathText: %v", err)
	}
	h = fixture.HeadValue("/"+strings.Repeat("p", wire.MaxPathTextBytes), 1, d, 0, d)
	if _, err := snapshot.DecodeHead(wire.EncodeFile(h)); code(err) != wire.CodeLimitExceeded {
		t.Errorf("primaryWorktree over %d bytes: %v", wire.MaxPathTextBytes, err)
	}
	h = fixture.HeadValue("relative/path", 1, d, 0, d)
	if _, err := snapshot.DecodeHead(wire.EncodeFile(h)); code(err) != wire.CodeMalformed {
		t.Errorf("relative primaryWorktree: %v", err)
	}
	h = fixture.HeadValue("/tmp/a\tb", 1, d, 0, d)
	if _, err := snapshot.DecodeHead(wire.EncodeFile(h)); code(err) != wire.CodeMalformed {
		t.Errorf("control byte in primaryWorktree: %v", err)
	}
	if _, err := snapshot.DecodeHead([]byte(strings.Repeat("x", wire.MaxJournalHeadBytes+1))); code(err) != wire.CodeLimitExceeded {
		t.Errorf("oversized head: %v", err)
	}
	if n, _ := snapshot.ReceiptName(1); n != "000000000001.json" {
		t.Errorf("receipt name %q", n)
	}
	if _, err := snapshot.ReceiptName(1000000000000); err == nil {
		t.Errorf("13-digit seq accepted")
	}
}
