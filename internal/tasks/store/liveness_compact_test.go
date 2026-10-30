package store_test

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// compactAuditOK runs the complete receipt audit from receipt 1 and the
// archive export plus verify over the live store, and returns the export.
func compactAuditOK(t *testing.T, s *leaseStore, label string) []byte {
	t.Helper()
	q, err := wire.ParseQueueID("queueId", fixture.QueueID)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := (journal.Reader{Source: journal.Native{StateDir: s.repo.StateDir, PrimaryWorktree: s.repo.PrimaryWorktree}, QueueID: q, PrimaryWorktree: s.repo.PrimaryWorktree}).Audit()
	if err != nil || proof.StructuralConsistency != "CONSISTENT" || proof.ProjectionAgreement != "AGREES" {
		t.Fatalf("%s: complete audit %+v %v", label, proof, err)
	}
	var stream bytes.Buffer
	if _, err := archive.Export(archive.ExportOptions{Repo: s.repo, Staging: t.TempDir(), Stdout: &stream}); err != nil {
		t.Fatalf("%s: archive export %v", label, err)
	}
	if _, err := archive.Verify(bytes.NewReader(stream.Bytes())); err != nil {
		t.Fatalf("%s: archive verify %v", label, err)
	}
	return stream.Bytes()
}

// receiptFiles reads every receipt by name.
func receiptFiles(t *testing.T, s *leaseStore) map[string][]byte {
	t.Helper()
	dir := filepath.Join(s.repo.StateDir, "receipts")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	for _, e := range entries {
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		out[e.Name()] = raw
	}
	return out
}

func compact(t *testing.T, s *leaseStore, dryRun bool) *store.CompactReport {
	t.Helper()
	r, err := store.CompactLiveness(context.Background(), s.repo, dryRun)
	if err != nil {
		t.Fatalf("compact dryRun=%v: %v", dryRun, err)
	}
	return r
}

func isElided(raw []byte) bool {
	return bytes.Contains(raw, []byte(`"`+snapshot.ProfileElidedReceipt+`"`))
}

// TestCALV0302_CompactLivenessKeepsAuditReplayAndFencing (CAL-V0-300..CAL-V0-304):
// an opt-in compaction rewrites only superseded heartbeat and renew receipts;
// the complete audit and archive verify pass before and after; the strict
// (old-binary) decoder refuses a tombstone; a request replay whose receipt is
// a tombstone returns the original result; fencing and expiry are unchanged;
// a rerun is idempotent; restoring the original bytes over the tombstones
// (the documented rollback) leaves a valid journal even after later appends.
func TestCALV0302_CompactLivenessKeepsAuditReplayAndFencing(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	id := s.ticket(t, "compact")
	c := s.lease(t, "claim-compact", claimOf(id, "src/"), 1, store.NoScopeDeriver)
	hb := transaction.LeaseRequest{Verb: transaction.LeaseHeartbeat, AttemptID: c.AttemptID, Generation: c.Generation}
	reports := map[string]*store.Report{}
	for i := 0; i < 12; i++ {
		rid, req := "hb-"+strconv.Itoa(i), hb
		if i%4 == 3 {
			rid, req = "renew-"+strconv.Itoa(i), renewOf(c)
		}
		r := s.lease(t, rid, req, 2+i, store.NoScopeDeriver)
		if r.Outcome.Outcome != mutation.OutcomeCompleted || r.Receipt == "" {
			t.Fatalf("%s: %+v", rid, r)
		}
		reports[rid] = r
	}
	rollback := compactAuditOK(t, s, "before")
	before := receiptFiles(t, s)

	dry := compact(t, s, true)
	if !sameFiles(receiptFiles(t, s), before) {
		t.Fatal("dry run wrote a receipt")
	}
	first := compact(t, s, false)
	if first.Elided == 0 || first.Elided != dry.Elided || first.BytesAfter != dry.BytesAfter || first.BytesAfter >= first.BytesBefore || first.AlreadyElided != 0 {
		t.Fatalf("compaction report %+v dry %+v", first, dry)
	}
	// Every liveness receipt but the last is superseded by the next one.
	if first.Elided != 11 {
		t.Fatalf("elided %d, want 11", first.Elided)
	}
	after := receiptFiles(t, s)
	elided := []string{}
	for name, raw := range after {
		if !isElided(raw) {
			if !bytes.Equal(raw, before[name]) {
				t.Fatalf("%s changed without elision", name)
			}
			continue
		}
		elided = append(elided, name)
		rc, digest, ok, err := snapshot.DecodeRetainedReceipt(raw)
		if err != nil || !ok || digest != wire.Sum(before[name]) || rc.Kind != "TRANSITION" {
			t.Fatalf("%s: tombstone %v %v %v", name, ok, digest, err)
		}
		// An old binary's strict decode refuses rather than misreads.
		if _, err := snapshot.DecodeReceipt(raw); err == nil || !strings.Contains(err.Error(), wire.CodeUnsupportedVersion) {
			t.Fatalf("%s: strict decode of a tombstone = %v", name, err)
		}
	}
	if uint64(len(elided)) != first.Elided {
		t.Fatalf("%d tombstones on disk, report %d", len(elided), first.Elided)
	}
	head, err := snapshot.ReceiptName(first.HeadSeq)
	if err != nil || isElided(after[head]) {
		t.Fatalf("head %s elided (%v)", head, err)
	}
	if last := filepath.Base(reports["renew-11"].Receipt); isElided(after[last]) {
		t.Fatalf("latest liveness receipt %s elided", last)
	}
	compactAuditOK(t, s, "after")

	// A replayed request whose receipt is now a tombstone returns its
	// original result and writes nothing.
	old := reports["hb-0"]
	if !isElided(after[filepath.Base(old.Receipt)]) {
		t.Fatalf("hb-0 receipt %s not elided", old.Receipt)
	}
	replay := s.lease(t, "hb-0", hb, 30, store.NoScopeDeriver)
	if !replay.Outcome.Replayed || replay.Outcome.Outcome != old.Outcome.Outcome || replay.AttemptID != old.AttemptID || replay.Generation != old.Generation {
		t.Fatalf("replay from tombstone %+v, original %+v", replay, old)
	}
	renewReplay := s.lease(t, "renew-3", renewOf(c), 30, store.NoScopeDeriver)
	if !renewReplay.Outcome.Replayed || renewReplay.AttemptID != c.AttemptID || renewReplay.Generation != c.Generation {
		t.Fatalf("renew replay from tombstone %+v", renewReplay)
	}
	if !sameFiles(receiptFiles(t, s), after) {
		t.Fatal("a replay wrote a receipt")
	}

	// Fencing and expiry are unchanged after compaction.
	live := s.lease(t, "hb-live", hb, 31, store.NoScopeDeriver)
	if live.Outcome.Outcome != mutation.OutcomeCompleted || live.Outcome.Replayed {
		t.Fatalf("heartbeat after compaction %+v", live)
	}
	wrong := hb
	wrong.Generation = "9"
	refusedWith(t, s.lease(t, "hb-wrong-generation", wrong, 32, store.NoScopeDeriver), mutation.OutcomeRevisionConflict, wire.CodeFenced)
	refusedWith(t, s.lease(t, "hb-expired", hb, 200, store.NoScopeDeriver), mutation.OutcomeRevisionConflict, wire.CodeFenced)
	s.lease(t, "reap-compact", transaction.LeaseRequest{Verb: transaction.LeaseReap, AttemptID: c.AttemptID, Generation: c.Generation}, 201, store.NoScopeDeriver)
	next := s.lease(t, "claim-next", claimOf(id, "src/"), 202, store.NoScopeDeriver)
	if next.Outcome.Outcome != mutation.OutcomeCompleted || next.Generation == c.Generation {
		t.Fatalf("readmission after compaction %+v", next)
	}
	refusedWith(t, s.lease(t, "hb-old-generation", hb, 203, store.NoScopeDeriver), mutation.OutcomeRevisionConflict, wire.CodeFenced)
	rerunRollback := compactAuditOK(t, s, "after appends")

	// A rerun keeps the earlier tombstones and elides only what is newly
	// superseded: renew-11 by hb-live, and hb-live by the reap.
	second := compact(t, s, false)
	if second.AlreadyElided != first.Elided || second.Elided != 2 {
		t.Fatalf("rerun %+v", second)
	}
	compactAuditOK(t, s, "rerun")

	// Rollback: each run's pre-compaction export restored over the tombstones
	// that run wrote leaves the journal valid for the old decoder.
	exported := receiptFiles(t, s)
	for name, raw := range exported {
		if isElided(raw) {
			original := archiveMember(t, rollback, "receipts/"+name)
			if original == nil {
				original = archiveMember(t, rerunRollback, "receipts/"+name)
			}
			if original == nil || isElided(original) {
				t.Fatalf("no pre-compaction export holds %s", name)
			}
			fixture.Write(t, filepath.Join(s.repo.StateDir, "receipts", name), original)
		}
	}
	for name, raw := range receiptFiles(t, s) {
		if _, err := snapshot.DecodeReceipt(raw); err != nil {
			t.Fatalf("rolled back %s: %v", name, err)
		}
	}
	compactAuditOK(t, s, "rolled back")
}

// TestCALV0304_CompactLivenessIssueShape (CAL-V0-304) measures BASE against
// fixed receipt bytes on a journal of the #982 shape: several concurrent
// attempts whose receipts are mostly heartbeats and renews among ownership
// transitions. BASE has no compaction, so its bytes are BytesBefore.
func TestCALV0304_CompactLivenessIssueShape(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	var ids []string
	for i := 0; i < 4; i++ {
		ids = append(ids, s.ticket(t, "shape-"+strconv.Itoa(i)))
	}
	claims := make([]*store.Report, len(ids))
	for i, id := range ids {
		claims[i] = s.lease(t, "claim-"+id, claimOf(id, "src/"+strconv.Itoa(i)+"/"), 1, store.NoScopeDeriver)
	}
	var liveness int
	for round := 0; round < 6; round++ {
		for i, c := range claims {
			rid := "hb-" + strconv.Itoa(round) + "-" + strconv.Itoa(i)
			req := transaction.LeaseRequest{Verb: transaction.LeaseHeartbeat, AttemptID: c.AttemptID, Generation: c.Generation}
			if round%3 == 2 {
				req = renewOf(c)
			}
			if r := s.lease(t, rid, req, 2+round, store.NoScopeDeriver); r.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("%s: %+v", rid, r)
			}
			liveness++
		}
	}
	for i, c := range claims[:2] {
		rel := transaction.LeaseRequest{Verb: transaction.LeaseRelease, AttemptID: c.AttemptID, Generation: c.Generation}
		if r := s.lease(t, "release-"+strconv.Itoa(i), rel, 10, store.NoScopeDeriver); r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("release: %+v", r)
		}
	}
	compactAuditOK(t, s, "base")
	r := compact(t, s, false)
	compactAuditOK(t, s, "fixed")
	share := float64(liveness) / float64(r.HeadSeq)
	saved := 1 - float64(r.BytesAfter)/float64(r.BytesBefore)
	t.Logf("receipts=%d liveness=%d (%.0f%%) elided=%d BASE bytes=%d fixed bytes=%d saved=%.0f%%", r.HeadSeq, liveness, 100*share, r.Elided, r.BytesBefore, r.BytesAfter, 100*saved)
	// Every liveness receipt is superseded except the latest of each attempt
	// still held; a release supersedes the latest of a released attempt.
	if r.Elided != uint64(liveness-len(claims)+2) {
		t.Fatalf("elided %d of %d liveness receipts", r.Elided, liveness)
	}
	if saved < 0.25 {
		t.Fatalf("compaction saved %.0f%% of receipt bytes", 100*saved)
	}
}

func sameFiles(a, b map[string][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || !bytes.Equal(v, w) {
			return false
		}
	}
	return true
}

// archiveMember reads one member of an archive export stream, nil when the
// stream has no such member.
func archiveMember(t *testing.T, stream []byte, path string) []byte {
	t.Helper()
	tr := tar.NewReader(bytes.NewReader(stream))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			t.Fatalf("archive member %s: %v", path, err)
		}
		if hdr.Name == path {
			raw, err := io.ReadAll(tr)
			if err != nil {
				t.Fatal(err)
			}
			return raw
		}
	}
}

// TestCALV0300_ElisionScopeAndTamper (CAL-V0-300, CAL-V0-301): only heartbeat
// and renew receipts are elidable; claims, releases, ticket and policy
// receipts never are. A tombstone restores to its original bytes and digest,
// and a changed recorded digest, changed liveness member or wrong
// predecessor record refuses with JOURNAL_FORKED instead of restoring.
func TestCALV0300_ElisionScopeAndTamper(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	id := s.ticket(t, "scope")
	c := s.lease(t, "claim-scope", claimOf(id, "src/"), 1, store.NoScopeDeriver)
	hb := transaction.LeaseRequest{Verb: transaction.LeaseHeartbeat, AttemptID: c.AttemptID, Generation: c.Generation}
	liveness := map[string]bool{}
	for i, req := range []transaction.LeaseRequest{hb, renewOf(c), hb} {
		r := s.lease(t, "live-"+strconv.Itoa(i), req, 2+i, store.NoScopeDeriver)
		liveness[filepath.Base(r.Receipt)] = true
	}
	s.lease(t, "release-scope", releaseOf(c), 6, store.NoScopeDeriver)
	files := receiptFiles(t, s)
	restore := &snapshot.ReceiptRestorer{}
	type elidable struct {
		name, path                string
		original, base, tombstone []byte
	}
	var got []elidable
	for seq := uint64(1); ; seq++ {
		name, err := snapshot.ReceiptName(seq)
		if err != nil {
			t.Fatal(err)
		}
		raw, ok := files[name]
		if !ok {
			break
		}
		header, err := snapshot.DecodeReceipt(raw)
		if err != nil {
			t.Fatal(err)
		}
		var base []byte
		path := ""
		if header.AttemptID != nil {
			path = "attempts/" + *header.AttemptID + ".json"
			base = restore.Base(path)
		}
		if _, _, err := restore.Next(raw); err != nil {
			t.Fatal(err)
		}
		tomb, ok, err := snapshot.ElideLiveness(raw, base)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if ok != liveness[name] {
			t.Fatalf("%s: elidable %v, liveness %v", name, ok, liveness[name])
		}
		if ok {
			got = append(got, elidable{name, path, raw, base, tomb})
		}
	}
	if len(got) != len(liveness) {
		t.Fatalf("elidable %d of %d liveness receipts", len(got), len(liveness))
	}
	e := got[0]
	header, digest, elided, err := snapshot.DecodeRetainedReceipt(e.tombstone)
	if err != nil || !elided || digest != wire.Sum(e.original) || header.Seq.Uint64() == 0 {
		t.Fatalf("retained decode %v %v %v", elided, digest, err)
	}
	record, err := snapshot.ElidedRecord(e.tombstone, e.base)
	if err != nil {
		t.Fatal(err)
	}
	full, err := snapshot.DecodeReceipt(e.original)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range full.Post {
		if p.Path == e.path && wire.Sum(record) != *p.Sha256 {
			t.Fatal("restored record differs from the post digest")
		}
	}
	// Restoring from the walk's own bases round-trips; tampered forms do not.
	replayTo := func(tomb []byte) error {
		r := &snapshot.ReceiptRestorer{}
		for seq := uint64(1); ; seq++ {
			name, err := snapshot.ReceiptName(seq)
			if err != nil {
				return err
			}
			if name == e.name {
				_, back, err := r.Next(tomb)
				if err == nil && !bytes.Equal(back, e.original) {
					t.Fatal("restored bytes differ")
				}
				return err
			}
			if _, _, err := r.Next(files[name]); err != nil {
				return err
			}
		}
	}
	if err := replayTo(e.tombstone); err != nil {
		t.Fatalf("untampered tombstone: %v", err)
	}
	// A same-shaped receipt whose post also changes the lease holder is an
	// ownership change, never liveness: it is not elidable.
	forged, err := wire.Parse(e.original)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range forged.Obj.Vals["post"].Arr {
		if path, _ := p.Obj.Get("path"); path.Kind == wire.KindString && path.Str == e.path {
			rec := p.Obj.Vals["record"].Obj
			rec.Vals["lease"].Obj.Set("holder", wire.String("someone-else"))
			p.Obj.Set("sha256", wire.String(string(wire.Sum(wire.EncodeFile(wire.ObjectValue(rec))))))
		}
	}
	if _, ok, err := snapshot.ElideLiveness(wire.EncodeFile(forged), e.base); ok || err != nil {
		t.Fatalf("a holder change was elidable: ok %v err %v", ok, err)
	}
	// flip changes the character at offset in the string value after from.
	flip := func(raw []byte, from string, offset int) []byte {
		i := bytes.Index(raw, []byte(from))
		if i < 0 {
			t.Fatalf("no %q in tombstone", from)
		}
		out := bytes.Clone(raw)
		j := i + len(from) + offset
		if out[j] == '0' {
			out[j] = '1'
		} else {
			out[j] = '0'
		}
		return out
	}
	for label, tampered := range map[string][]byte{
		"receiptSha256":   flip(e.tombstone, `"receiptSha256":"`, 0),
		"lastHeartbeatAt": flip(e.tombstone, `"elided":{"lastHeartbeatAt":"`, 18),
	} {
		if err := replayTo(tampered); err == nil || !strings.Contains(err.Error(), wire.CodeJournalForked) {
			t.Fatalf("tampered %s restored: %v", label, err)
		}
	}
	// Without its predecessor record a tombstone does not restore.
	if _, _, err := (&snapshot.ReceiptRestorer{}).Next(e.tombstone); err == nil || !strings.Contains(err.Error(), wire.CodeJournalForked) {
		t.Fatalf("tombstone restored without its predecessor: %v", err)
	}
	if _, err := snapshot.ElidedRecord(e.tombstone, record[:len(record)-2]); err == nil {
		t.Fatal("a malformed ancestor restored the record")
	}
}

// TestCALV0300_UnsupersededTombstoneRefused (CAL-V0-300, CAL-V0-301): a
// tombstone that restores exactly but is still the latest post of its
// attempt is refused by the complete audit and by archive verify, so a
// hand-elided latest record never passes as a valid compacted store.
func TestCALV0300_UnsupersededTombstoneRefused(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	id := s.ticket(t, "latest")
	other := s.ticket(t, "other")
	c := s.lease(t, "claim-latest", claimOf(id, "src/"), 1, store.NoScopeDeriver)
	hb := transaction.LeaseRequest{Verb: transaction.LeaseHeartbeat, AttemptID: c.AttemptID, Generation: c.Generation}
	s.lease(t, "hb-a", hb, 2, store.NoScopeDeriver)
	last := s.lease(t, "hb-b", hb, 3, store.NoScopeDeriver)
	// A later receipt of another attempt keeps hb-b off the head.
	s.lease(t, "claim-other", claimOf(other, "lib/"), 4, store.NoScopeDeriver)
	export := compactAuditOK(t, s, "before")
	files := receiptFiles(t, s)
	latestName := filepath.Base(last.Receipt)
	restore := &snapshot.ReceiptRestorer{}
	var tomb []byte
	for seq := uint64(1); ; seq++ {
		name, err := snapshot.ReceiptName(seq)
		if err != nil {
			t.Fatal(err)
		}
		if name == latestName {
			var ok bool
			tomb, ok, err = snapshot.ElideLiveness(files[name], restore.Base("attempts/"+c.AttemptID+".json"))
			if err != nil || !ok {
				t.Fatalf("latest heartbeat not elidable: %v %v", ok, err)
			}
			break
		}
		if _, _, err := restore.Next(files[name]); err != nil {
			t.Fatal(err)
		}
	}
	// The tombstone itself restores; only its position is invalid.
	if _, _, err := restore.Next(tomb); err != nil {
		t.Fatalf("latest tombstone does not restore: %v", err)
	}
	if err := restore.Superseded(); err == nil || !strings.Contains(err.Error(), wire.CodeJournalForked) {
		t.Fatalf("restorer accepted an unsuperseded tombstone: %v", err)
	}
	if err := os.WriteFile(filepath.Join(s.repo.StateDir, "receipts", latestName), tomb, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{journal.CheckpointPath(s.repo.StateDir), journal.WriterCheckpointPath(s.repo.StateDir), journal.WriterBasePath(s.repo.StateDir)} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	q, err := wire.ParseQueueID("queueId", fixture.QueueID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (journal.Reader{Source: journal.Native{StateDir: s.repo.StateDir, PrimaryWorktree: s.repo.PrimaryWorktree}, QueueID: q, PrimaryWorktree: s.repo.PrimaryWorktree}).Audit(); err == nil || !strings.Contains(err.Error(), wire.CodeJournalForked) || !strings.Contains(err.Error(), "latest post") {
		t.Fatalf("complete audit accepted an elided latest record: %v", err)
	}
	// Export runs the same audit and refuses; archive verify refuses an
	// archive of the same store, built from the earlier export with the
	// tombstone in place of its original and the manifest entry to match.
	if _, err := archive.Export(archive.ExportOptions{Repo: s.repo, Staging: t.TempDir(), Stdout: io.Discard}); err == nil || !strings.Contains(err.Error(), "latest post") {
		t.Fatalf("archive export accepted an elided latest record: %v", err)
	}
	tampered := replaceArchiveMember(t, export, "receipts/"+latestName, tomb)
	if _, err := archive.Verify(bytes.NewReader(tampered)); err == nil || !strings.Contains(err.Error(), wire.CodeJournalForked) || !strings.Contains(err.Error(), "latest post") {
		t.Fatalf("archive verify accepted an elided latest record: %v", err)
	}
}

// replaceArchiveMember rewrites an archive export stream with one member's
// bytes replaced, its manifest entry updated to match and the members in
// the manifest's canonical order.
func replaceArchiveMember(t *testing.T, stream []byte, path string, data []byte) []byte {
	t.Helper()
	tr := tar.NewReader(bytes.NewReader(stream))
	headers, members := map[string]*tar.Header{}, map[string][]byte{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		headers[hdr.Name], members[hdr.Name] = hdr, raw
	}
	m, err := archive.DecodeManifest(members[archive.ManifestName])
	if err != nil {
		t.Fatal(err)
	}
	for i := range m.Files {
		if m.Files[i].Path == path {
			m.Files[i].Sha256 = wire.Sum(data)
			m.Files[i].Bytes = wire.SizeOf(uint64(len(data)))
		}
	}
	archive.SortFiles(m.Files)
	members[archive.ManifestName], members[path] = m.Encode(), data
	order := []string{archive.ManifestName}
	for _, f := range m.Files {
		order = append(order, f.Path)
	}
	var out bytes.Buffer
	tw := tar.NewWriter(&out)
	for _, name := range order {
		hdr := headers[name]
		hdr.Size = int64(len(members[name]))
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(members[name]); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// TestCALV0302_CompactLivenessBoundsElidedRuns (CAL-V0-302): compaction keeps
// a superseded liveness receipt whole when the latest whole record of its
// attempt is the run bound or more receipts back, so every tombstone can be
// restored by a backward reader within that bound.
func TestCALV0302_CompactLivenessBoundsElidedRuns(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	id := s.ticket(t, "runs")
	c := s.lease(t, "claim-runs", claimOf(id, "src/"), 1, store.NoScopeDeriver)
	hb := transaction.LeaseRequest{Verb: transaction.LeaseHeartbeat, AttemptID: c.AttemptID, Generation: c.Generation}
	for i := 0; i < 12; i++ {
		s.lease(t, "hb-"+strconv.Itoa(i), hb, 2+i, store.NoScopeDeriver)
	}
	const maxRun = 4
	r, err := store.CompactLivenessRunForTest(context.Background(), s.repo, false, maxRun)
	if err != nil {
		t.Fatal(err)
	}
	// Heartbeats 4 and 8 after the claim stay whole; the last is never superseded.
	if r.Elided != 9 {
		t.Fatalf("elided %d, want 9", r.Elided)
	}
	compactAuditOK(t, s, "bounded")
	files := receiptFiles(t, s)
	path := "attempts/" + c.AttemptID + ".json"
	last := uint64(0)
	for seq := uint64(1); ; seq++ {
		name, err := snapshot.ReceiptName(seq)
		if err != nil {
			t.Fatal(err)
		}
		raw, ok := files[name]
		if !ok {
			break
		}
		rc, _, elided, err := snapshot.DecodeRetainedReceipt(raw)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range rc.Post {
			if p.Path != path {
				continue
			}
			if !elided {
				last = seq
			} else if last == 0 || seq-last >= maxRun {
				t.Fatalf("%s: tombstone %d receipts after the latest whole record (bound %d)", name, seq-last, maxRun)
			}
		}
	}
}
