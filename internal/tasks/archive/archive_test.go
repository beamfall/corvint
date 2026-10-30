package archive

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func code(err error) string { return wire.CodeOf(err) }

func repoWithStore(t *testing.T) (*fixture.Repo, *intent.Repository) {
	t.Helper()
	r := fixture.TempRepo(t)
	fixture.WriteState(t, r)
	fixture.WriteIntent(t, r, fixture.Ticket("A"), fixture.Ticket("B"))
	fixture.CommitPosts(t, r, "MUTATION", "", map[string][]byte{"intent/tickets/A.json": fixture.Ticket("A").Encode(), "intent/tickets/B.json": fixture.Ticket("B").Encode()})
	reqPath, _ := snapshot.RequestPath("r1")
	seq := wire.Size("3")
	out := mutation.Outcome{RequestID: "r1", Outcome: mutation.OutcomeCompleted, ReceiptSeq: &seq}
	req := wire.NewObject()
	req.Set("requestId", wire.String("r1"))
	req.Set("seq", wire.String("3"))
	req.Set("mutationSha256", wire.String(string(wire.Sum([]byte("fixture mutation\n")))))
	req.Set("outcome", out.Value())
	fixture.CommitPosts(t, r, "MUTATION", "r1", map[string][]byte{reqPath: wire.EncodeFile(wire.ObjectValue(req))})
	// Extra state-dir content every archive must carry.
	fixture.Write(t, filepath.Join(r.StateDir, "pinned", string(wire.Sum([]byte("{}\n")))+".json"), []byte("{}\n"))
	fixture.Write(t, filepath.Join(r.StateDir, "evidence", string(wire.Sum([]byte("blob")))), []byte("blob"))
	fixture.Write(t, filepath.Join(r.StateDir, "effects", strings.Repeat("d", 64)+".boot"), []byte("{}\n"))
	if err := os.MkdirAll(filepath.Join(r.StateDir, "worktrees", "x-1"), 0o755); err != nil {
		t.Fatal(err)
	}
	repo, err := intent.Resolve(r.Root)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	return r, repo
}

func export(t *testing.T, repo *intent.Repository, staging string) ([]byte, *ExportResult, error) {
	t.Helper()
	var out bytes.Buffer
	res, err := Export(ExportOptions{Repo: repo, Staging: staging, Stdout: &out})
	return out.Bytes(), res, err
}

// TestTMV0022_AS09_ExportThenVerify: export → verify round trip; the stream
// carries every state-dir and intent file byte-identically; the store,
// the intent tree and the lock path are untouched and the staging dir is
// left empty (N4f).
func TestTMV0022_AS09_ExportThenVerify(t *testing.T) {
	r, repo := repoWithStore(t)
	staging := fixture.TempDirOutside(t)
	stateBefore := fixture.TreeSnapshot(t, r.StateDir)
	intentBefore := fixture.TreeSnapshot(t, r.IntentDir)
	stream, res, err := export(t, repo, staging)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	fixture.AssertUntouched(t, r, stateBefore, intentBefore, "export")
	if entries, _ := os.ReadDir(staging); len(entries) != 0 {
		t.Errorf("staging files left behind: %d", len(entries))
	}
	vr, err := Verify(bytes.NewReader(stream))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if vr.Manifest.ExportedAtSeq != "3" || vr.Manifest.ReceiptCount != "3" || vr.Head.LastSeq != "3" || vr.Manifest.HeadGeneration != "0" {
		t.Errorf("manifest: %+v", vr.Manifest)
	}
	if string(vr.Manifest.Encode()) != string(res.Manifest.Encode()) {
		t.Errorf("verified manifest differs from the export's")
	}
	q, _ := wire.ParseQueueID("", fixture.QueueID)
	descriptor := snapshot.Init{QueueID: q, PrimaryWorktree: r.Root, VersionSha256: wire.Sum([]byte(snapshot.VersionBytes))}
	wantPaths := []string{
		"reservations.json", "pinned/" + string(wire.Sum(wire.EncodeFile(descriptor.Value()))) + ".json", "evidence/" + string(wire.Sum([]byte(snapshot.VersionBytes))),
		"VERSION", "head.json", "receipts/000000000001.json", "receipts/000000000002.json", "receipts/000000000003.json",
		func() string { p, _ := snapshot.RequestPath("r1"); return p }(), "pinned/" + string(wire.Sum([]byte("{}\n"))) + ".json",
		"evidence/" + string(wire.Sum([]byte("blob"))), "effects/" + strings.Repeat("d", 64) + ".boot",
		"intent/queue.json", "intent/policy.json", "intent/tickets/A.json", "intent/tickets/B.json",
	}
	got := map[string]FileEntry{}
	for _, f := range vr.Manifest.Files {
		got[f.Path] = f
	}
	if len(got) != len(wantPaths) {
		t.Errorf("archive has %d files, want %d: %v", len(got), len(wantPaths), got)
	}
	for _, p := range wantPaths {
		fe, ok := got[p]
		if !ok {
			t.Errorf("missing %s", p)
			continue
		}
		src := filepath.Join(r.StateDir, filepath.FromSlash(p))
		if strings.HasPrefix(p, "intent/") {
			src = filepath.Join(r.IntentDir, filepath.FromSlash(strings.TrimPrefix(p, "intent/")))
		}
		raw, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if wire.Sum(raw) != fe.Sha256 || fe.Bytes.Uint64() != uint64(len(raw)) {
			t.Errorf("%s: archive digest/size differ from disk", p)
		}
	}
	// The tar entries themselves are byte-identical to disk (read them back).
	tr := tar.NewReader(bytes.NewReader(stream))
	hdr, err := tr.Next()
	if err != nil || hdr.Name != ManifestName {
		t.Fatalf("first entry: %v %v", hdr, err)
	}
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		var buf bytes.Buffer
		buf.ReadFrom(tr)
		if wire.Sum(buf.Bytes()) != got[hdr.Name].Sha256 {
			t.Errorf("%s: tar body differs from manifest digest", hdr.Name)
		}
	}
	// Determinism: a second export of the same snapshot is byte-identical.
	stream2, _, err := export(t, repo, staging)
	if err != nil || !bytes.Equal(stream, stream2) {
		t.Errorf("export is not deterministic: %v", err)
	}
	// intentTreeSha256 equals the live tree digest.
	tree, _ := intent.TreeDigest(r.Root)
	if vr.Manifest.IntentTreeSha256 != tree.Sha256 {
		t.Errorf("intent tree digest differs")
	}
}

// TestTMV0022_AS36_N4dSnapshotMovedDuringExport: commits injected on every
// probe make the export report SNAPSHOT_MOVED after three retries with zero
// bytes on stdout and no staging file left behind. The injected commits are
// valid (receipt N+1, N+2 and an advanced head), so the layout scan's
// "receipt beyond head.lastSeq" failure is a read across a commit: the
// re-probe shows a moved store and the attempt is retried, never reported
// as JOURNAL_FORKED (TM-V0-008). A stable fork stays JOURNAL_FORKED: see
// TestTMV0022_AS36_ExportRefusesInconsistentStores.
func TestTMV0022_AS36_N4dSnapshotMovedDuringExport(t *testing.T) {
	r, repo := repoWithStore(t)
	staging := fixture.TempDirOutside(t)
	stateBefore := fixture.TreeSnapshot(t, r.IntentDir)
	var out bytes.Buffer
	probes := 0
	_, err := Export(ExportOptions{Repo: repo, Staging: staging, Stdout: &out, afterProbe: func() {
		probes++
		fixture.Commit(t, r, "MUTATION")
		fixture.Commit(t, r, "MUTATION")
	}})
	if code(err) != wire.CodeSnapshotMoved {
		t.Errorf("want SNAPSHOT_MOVED, got %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("stdout received %d bytes", out.Len())
	}
	if probes != 4 {
		t.Errorf("expected 4 attempts, got %d", probes)
	}
	if entries, _ := os.ReadDir(staging); len(entries) != 0 {
		t.Errorf("torn staging file left behind")
	}
	if !fixture.SameTree(stateBefore, fixture.TreeSnapshot(t, r.IntentDir)) {
		t.Errorf("intent tree changed")
	}
	// Intent tree change between probe and copy is also SNAPSHOT_MOVED.
	out.Reset()
	probes = 0
	_, err = Export(ExportOptions{Repo: repo, Staging: staging, Stdout: &out, afterProbe: func() {
		probes++
		x := fixture.Ticket("A")
		x.Title = "moved " + string(wire.CountOf(int64(probes)))
		fixture.Write(t, filepath.Join(r.IntentDir, "tickets", "A.json"), x.Encode())
	}})
	if code(err) != wire.CodeSnapshotMoved || out.Len() != 0 {
		t.Errorf("intent move: %v, %d bytes", err, out.Len())
	}
}

// TestTMV0022_AS36_N4fStagingRefused: staging inside the state dir, the
// repository (including a linked worktree), on a symlinked path, or under
// malformed Git authority is refused before anything is read.
func TestTMV0022_AS36_N4fStagingRefused(t *testing.T) {
	r, repo := repoWithStore(t)
	stateBefore := fixture.TreeSnapshot(t, r.StateDir)
	inState := filepath.Join(r.StateDir, "pinned")
	inRepo := filepath.Join(r.Root, "build")
	os.MkdirAll(inRepo, 0o755)
	for _, s := range []string{inState, inRepo, r.Root, r.StateDir} {
		out, _, err := export(t, repo, s)
		if code(err) != wire.CodeUnsupportedFilesystem || len(out) != 0 {
			t.Errorf("staging %s: %v (%d bytes)", s, err, len(out))
		}
	}
	outside := fixture.TempDirOutside(t)
	link := filepath.Join(outside, "link")
	if err := os.Symlink(outside, link); err == nil {
		if out, _, err := export(t, repo, link); code(err) != wire.CodeUnsupportedFilesystem || len(out) != 0 {
			t.Errorf("symlinked staging: %v", err)
		}
	}
	if _, _, err := export(t, repo, filepath.Join(outside, "missing")); code(err) != wire.CodeUnsupportedFilesystem {
		t.Errorf("missing staging dir: %v", err)
	}
	malformed := fixture.TempDirOutside(t)
	fixture.Write(t, filepath.Join(malformed, ".git"), []byte("not a gitdir\n"))
	malformedBefore := fixture.TreeSnapshot(t, malformed)
	if out, _, err := export(t, repo, malformed); code(err) != wire.CodeUnsupportedFilesystem || len(out) != 0 {
		t.Errorf("malformed staging authority: %v (%d bytes)", err, len(out))
	}
	if !fixture.SameTree(malformedBefore, fixture.TreeSnapshot(t, malformed)) {
		t.Errorf("malformed staging authority changed")
	}
	wtGit := filepath.Join(r.CommonDir, "worktrees", "staging")
	linked := filepath.Join(filepath.Dir(r.Root), "linked-staging")
	fixture.Write(t, filepath.Join(wtGit, "commondir"), []byte("../..\n"))
	fixture.Write(t, filepath.Join(wtGit, "gitdir"), []byte(filepath.Join(linked, ".git")+"\n"))
	fixture.Write(t, filepath.Join(linked, ".git"), []byte("gitdir: "+wtGit+"\n"))
	repoBefore := fixture.TreeSnapshot(t, r.Root)
	linkedBefore := fixture.TreeSnapshot(t, linked)
	if out, _, err := export(t, repo, linked); code(err) != wire.CodeUnsupportedFilesystem || len(out) != 0 {
		t.Errorf("linked-worktree staging: %v (%d bytes)", err, len(out))
	}
	if !fixture.SameTree(repoBefore, fixture.TreeSnapshot(t, r.Root)) {
		t.Errorf("linked-worktree refusal changed repository")
	}
	if !fixture.SameTree(linkedBefore, fixture.TreeSnapshot(t, linked)) {
		t.Errorf("linked-worktree refusal created staging files")
	}
	fixture.AssertUntouched(t, r, stateBefore, fixture.TreeSnapshot(t, r.IntentDir), "refused export")
	ordinary := fixture.TempDirOutside(t)
	if out, _, err := export(t, repo, ordinary); err != nil || len(out) == 0 {
		t.Errorf("ordinary outside staging: %v (%d bytes)", err, len(out))
	}
	if entries, err := os.ReadDir(ordinary); err != nil || len(entries) != 0 {
		t.Errorf("ordinary outside staging cleanup: %v (%d entries)", err, len(entries))
	}
	// The default staging (OS temp dir) works and is outside both trees.
	if out, _, err := export(t, repo, ""); err != nil || len(out) == 0 {
		t.Errorf("default staging: %v", err)
	}
}

// TestTMV0022_AS36_ExportRefusesInconsistentStores: a pending receipt, a
// fork, an unknown state-dir entry, a moved primary worktree and a state
// dir symlink all refuse with zero bytes.
func TestTMV0022_AS36_ExportRefusesInconsistentStores(t *testing.T) {
	r, repo := repoWithStore(t)
	staging := fixture.TempDirOutside(t)
	check := func(name, want string) {
		t.Helper()
		out, _, err := export(t, repo, staging)
		if code(err) != want || len(out) != 0 {
			t.Errorf("%s: %v (%d bytes), want %s", name, err, len(out), want)
		}
	}
	fixture.PlantReceipt(t, r, 4)
	check("pending", wire.CodeRedoPending)
	fixture.PlantReceipt(t, r, 5)
	check("fork", wire.CodeJournalForked)
	os.Remove(filepath.Join(r.StateDir, "receipts", "000000000004.json"))
	os.Remove(filepath.Join(r.StateDir, "receipts", "000000000005.json"))
	fixture.Write(t, filepath.Join(r.StateDir, "stray.txt"), []byte("x"))
	check("unknown entry", wire.CodeMalformed)
	os.Remove(filepath.Join(r.StateDir, "stray.txt"))
	fixture.Write(t, filepath.Join(r.StateDir, "receipts", "000000000002.tmp-123"), []byte("x"))
	if out, _, err := export(t, repo, staging); err != nil || len(out) == 0 {
		t.Errorf("a receipts temp file must be skipped, not fatal: %v", err)
	}
	os.Remove(filepath.Join(r.StateDir, "receipts", "000000000002.tmp-123"))
	hraw, _ := os.ReadFile(filepath.Join(r.StateDir, "head.json"))
	moved := strings.Replace(string(hraw), `"primaryWorktree":"`+r.Root+`"`, `"primaryWorktree":"/elsewhere"`, 1)
	fixture.Write(t, filepath.Join(r.StateDir, "head.json"), []byte(moved))
	check("moved primary", wire.CodeUnsupportedFilesystem)
	fixture.Write(t, filepath.Join(r.StateDir, "head.json"), hraw)
	// A receipt numbered beyond the head that sits at lastSeq+3 (not caught
	// by the slot check) is still JOURNAL_FORKED via the layout scan.
	fixture.PlantReceipt(t, r, 6)
	check("far receipt", wire.CodeJournalForked)
}

// TestTMV0022_AS10_ExportEntryBoundDuringListing: the archive scan bound
// (§3.5) is applied to every directory entry the export scans, directories
// and skipped temp entries included, during the listing and before any name
// is validated or any file opened. The exported-file bound is a separate
// budget checked by Export over state-dir files plus the intent tree.
func TestTMV0022_AS10_ExportEntryBoundDuringListing(t *testing.T) {
	r, repo := repoWithStore(t)
	// Fixture state dir: 9 root entries (VERSION, head.json, receipts,
	// requests, pinned, evidence, effects, worktrees, reservations.json), 3 receipts, 1 shard
	// holding 1 file, 2 pinned, 2 evidence, 1 effect: 19 entries scanned,
	// 12 archived files.
	files, scanned, err := stateLayout(r.StateDir, wire.MaxArchiveScanEntries)
	if err != nil || len(files) != 12 || scanned != 19 {
		t.Fatalf("baseline layout: %d files, %d scanned, %v", len(files), scanned, err)
	}
	if wire.MaxArchiveScanEntries != wire.MaxArchiveFiles+4096 {
		t.Errorf("scan bound must be the files bound plus 4,096 (§1)")
	}
	if _, _, err := stateLayout(r.StateDir, 19); err != nil {
		t.Errorf("bound equal to the entry count must pass: %v", err)
	}
	if _, _, err := stateLayout(r.StateDir, 18); code(err) != wire.CodeLimitExceeded {
		t.Errorf("one entry over the bound must be LIMIT_EXCEEDED: %v", err)
	}
	// A skipped temp entry still counts.
	tmp := filepath.Join(r.StateDir, "receipts", "000000000002.tmp-123")
	fixture.Write(t, tmp, []byte("x"))
	files, scanned, err = stateLayout(r.StateDir, 20)
	if err != nil || len(files) != 12 || scanned != 20 {
		t.Errorf("temp entry must be scanned but not archived: %d files, %d scanned, %v", len(files), scanned, err)
	}
	if _, _, err := stateLayout(r.StateDir, 19); code(err) != wire.CodeLimitExceeded {
		t.Errorf("temp entry must count against the bound: %v", err)
	}
	os.Remove(tmp)
	// The cumulative bound fires while a directory is being listed: with a
	// stray root entry that sorts after the sub-directories, the bound is
	// passed inside `requests/82` and reported LIMIT_EXCEEDED before the
	// stray name is ever reached and validated (MALFORMED).
	stray := filepath.Join(r.StateDir, "stray.txt")
	fixture.Write(t, stray, []byte("x"))
	if _, _, err := stateLayout(r.StateDir, 19); code(err) != wire.CodeLimitExceeded {
		t.Errorf("count bound must fire before the stray entry is validated: %v", err)
	}
	if _, _, err := stateLayout(r.StateDir, 20); code(err) != wire.CodeMalformed {
		t.Errorf("within the bound the stray entry is MALFORMED: %v", err)
	}
	os.Remove(stray)
	// The state dir and intent tree are untouched by every listing.
	stateBefore := fixture.TreeSnapshot(t, r.StateDir)
	intentBefore := fixture.TreeSnapshot(t, r.IntentDir)
	if _, _, err := stateLayout(r.StateDir, wire.MaxArchiveScanEntries); err != nil {
		t.Fatal(err)
	}
	fixture.AssertUntouched(t, r, stateBefore, intentBefore, "bounded listing")
	// A flood of empty files under evidence/ is refused during the listing
	// with LIMIT_EXCEEDED once the scan bound is passed; the flood is never
	// opened (its names would otherwise be MALFORMED). The bound is
	// injected small here; the frozen 2,104,096 is exercised only by the
	// constant checks above (no multi-million-file fixture is built).
	dir := filepath.Join(r.StateDir, "evidence")
	for i := 0; i < 300; i++ {
		f, err := os.OpenFile(filepath.Join(dir, "f"+string(wire.CountOf(int64(i)))), os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	if _, _, err := stateLayout(r.StateDir, 200); code(err) != wire.CodeLimitExceeded || !strings.Contains(err.Error(), "entries under evidence/") {
		t.Errorf("flooded evidence/ must stop during the listing: %v", err)
	}
	if _, _, err := stateLayout(r.StateDir, 400); code(err) != wire.CodeMalformed {
		t.Errorf("within the scan bound the flood names are MALFORMED: %v", err)
	}
	// The whole export refuses the same store with zero bytes on stdout.
	out, _, err := export(t, repo, fixture.TempDirOutside(t))
	if code(err) != wire.CodeMalformed || len(out) != 0 {
		t.Errorf("export over a malformed evidence/ name: %v (%d bytes)", err, len(out))
	}
}

// plantChain extends the fixture chain to lastSeq receipts in one pass
// (receipt 1 is the fixture INIT) and writes the matching head, so a store
// with many receipts is built without one head rewrite per receipt.
func plantChain(t *testing.T, r *fixture.Repo, lastSeq uint64) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(r.StateDir, "receipts", "000000000001.json"))
	if err != nil {
		t.Fatal(err)
	}
	prev := wire.Sum(raw)
	initSha := prev
	for seq := uint64(2); seq <= lastSeq; seq++ {
		rc := wire.EncodeFile(fixture.ReceiptValue(seq, &prev, "MUTATION", 0))
		fixture.Write(t, filepath.Join(r.StateDir, "receipts", receiptFileName(seq)), rc)
		prev = wire.Sum(rc)
	}
	fixture.Write(t, filepath.Join(r.StateDir, "head.json"), wire.EncodeFile(fixture.HeadValue(r.Root, lastSeq, prev, 0, initSha)))
}

func receiptFileName(seq uint64) string {
	s := string(wire.SizeOf(seq))
	return strings.Repeat("0", 12-len(s)) + s + ".json"
}

// TestTMV0022_AS10_ExportBeyondTenThousandMembers (B4 regression): a valid
// store whose archive holds more than 10,000 members (10,001 receipts plus
// 10,000 tickets and their mandatory metadata) exports and verifies whole;
// nothing is truncated. The stream is verified through the same
// `archive verify` path. Skipped under -short (writes ~20,000 small files).
func TestTMV0022_AS10_ExportBeyondTenThousandMembers(t *testing.T) {
	if testing.Short() {
		t.Skip("writes about 20,000 fixture files")
	}
	r := fixture.TempRepo(t)
	fixture.WriteState(t, r)
	recs := make([]*ticket.Record, 0, wire.MaxTicketsPerQueue)
	for i := 0; i < wire.MaxTicketsPerQueue; i++ {
		recs = append(recs, fixture.Ticket("T"+string(wire.CountOf(int64(i)))))
	}
	fixture.WriteIntent(t, r, recs...)
	const lastSeq = uint64(10001)
	plantChain(t, r, lastSeq)
	repo, err := intent.Resolve(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	stateBefore := fixture.TreeSnapshot(t, r.StateDir)
	intentBefore := fixture.TreeSnapshot(t, r.IntentDir)
	stream, res, err := export(t, repo, fixture.TempDirOutside(t))
	if err != nil {
		t.Fatalf("export beyond 10,000 members: %v", err)
	}
	fixture.AssertUntouched(t, r, stateBefore, intentBefore, "large export")
	// VERSION, head.json, genesis descriptor/blob/reservations, 10,001 receipts,
	// queue, policy, 10,000 tickets; historical ticket semantics remain unobserved.
	want := 5 + int(lastSeq) + 2 + wire.MaxTicketsPerQueue
	if len(res.Manifest.Files) != want || len(res.Manifest.Files) <= wire.MaxJSONArrayElements {
		t.Fatalf("manifest carries %d files, want %d (> %d)", len(res.Manifest.Files), want, wire.MaxJSONArrayElements)
	}
	vr, err := Verify(bytes.NewReader(stream))
	if err != nil {
		t.Fatalf("verify beyond 10,000 members: %v", err)
	}
	if vr.Files != want || vr.Manifest.ReceiptCount != wire.SizeOf(lastSeq) || vr.Head.LastSeq != wire.SizeOf(lastSeq) {
		t.Errorf("verified %d files, receiptCount %s, head %s", vr.Files, vr.Manifest.ReceiptCount, vr.Head.LastSeq)
	}
	// The same manifest bytes are refused by the default parser: the
	// widening is the archive's opt-in entry point only.
	if _, err := wire.Parse(res.Manifest.Encode()); code(err) != wire.CodeLimitExceeded {
		t.Errorf("default parser must refuse a files array over 10,000: %v", err)
	}
	if _, err := ParseManifestDocument(res.Manifest.Encode()); err != nil {
		t.Errorf("archive parser must accept it: %v", err)
	}
}

// TestTMV0022_AS10_VerifyBeyondTenThousandReceipts (B4 regression, in
// memory): a hand-built consistent store with 10,001 receipts verifies;
// the manifest byte cap is checked on the tar header before any manifest
// byte is read; the default parser and the ordinary bounds are unchanged.
func TestTMV0022_AS10_VerifyBeyondTenThousandReceipts(t *testing.T) {
	const n = 10001
	m, entries := handStore(t, n, n)
	if len(m.Files) != n+3 {
		t.Fatalf("hand store has %d files", len(m.Files))
	}
	vr, err := Verify(bytes.NewReader(buildStream(t, m, entries, true)))
	if err != nil {
		t.Fatalf("verify 10,001 receipts: %v", err)
	}
	if vr.Files != n+3 || vr.Manifest.ExportedAtSeq != wire.SizeOf(n) {
		t.Errorf("verified %d files at seq %s", vr.Files, vr.Manifest.ExportedAtSeq)
	}
	raw := m.Encode()
	if _, err := wire.Parse(raw); code(err) != wire.CodeLimitExceeded {
		t.Errorf("default parser must refuse the manifest: %v", err)
	}
	if _, err := DecodeManifest(raw); err != nil {
		t.Errorf("archive decoder must accept it: %v", err)
	}
	if MaxManifestBytes != wire.MaxArchiveManifestBytes || ManifestNodeCap != 4*wire.MaxArchiveFiles+14 {
		t.Errorf("manifest caps differ from the §3.5 freeze")
	}
	// Byte cap: one byte over is refused before parsing, whatever the bytes.
	if _, err := ParseManifestDocument(make([]byte, MaxManifestBytes+1)); code(err) != wire.CodeLimitExceeded {
		t.Errorf("manifest over the byte cap must be LIMIT_EXCEEDED before parsing: %v", err)
	}
	// A tar header announcing an oversize manifest is refused on the header
	// alone; no body is read (the stream carries none).
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: ManifestName, Mode: 0o644, Size: int64(MaxManifestBytes) + 1, ModTime: time.Unix(0, 0).UTC(), Format: tar.FormatPAX}); err != nil {
		t.Fatal(err)
	}
	tw.Flush()
	if _, err := Verify(bytes.NewReader(buf.Bytes())); code(err) != wire.CodeLimitExceeded {
		t.Errorf("oversize manifest header must be LIMIT_EXCEEDED: %v", err)
	}
	// A nested array inside a files entry keeps the ordinary bound.
	big := strings.TrimSuffix(strings.Repeat(`"",`, wire.MaxJSONArrayElements+1), ",")
	bad := strings.Replace(string(raw), `"complete":true`, `"complete":true,"extra":[`+big+`]`, 1)
	if _, err := ParseManifestDocument([]byte(bad)); code(err) != wire.CodeLimitExceeded {
		t.Errorf("a sibling array over 10,000 must stay LIMIT_EXCEEDED under the archive parser: %v", err)
	}
}

func buildStream(t *testing.T, m *Manifest, entries []struct {
	name string
	data []byte
}, marker bool) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	write := func(name string, data []byte) {
		if err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: name, Mode: 0o644, Size: int64(len(data)), ModTime: time.Unix(0, 0).UTC(), Format: tar.FormatPAX}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	write(ManifestName, m.Encode())
	for _, e := range entries {
		write(e.name, e.data)
	}
	if marker {
		tw.Close()
	} else {
		tw.Flush()
	}
	return buf.Bytes()
}

type entry = struct {
	name string
	data []byte
}

// handStore builds a minimal consistent store in memory (VERSION, head,
// receipts 1..n, queue.json) and its manifest.
func handStore(t *testing.T, receipts int, receiptCount uint64) (*Manifest, []entry) {
	t.Helper()
	version := []byte("taskman-state/0\n")
	var prev *wire.Digest
	var recs []entry
	for seq := 1; seq <= receipts; seq++ {
		rc := wire.EncodeFile(fixture.ReceiptValue(uint64(seq), prev, map[bool]string{true: "INIT", false: "MUTATION"}[seq == 1], 0))
		d := wire.Sum(rc)
		prev = &d
		recs = append(recs, entry{"receipts/" + strings.Repeat("0", 12-len(wire.SizeOf(uint64(seq)))) + string(wire.SizeOf(uint64(seq))) + ".json", rc})
	}
	head := wire.EncodeFile(fixture.HeadValue("/repo", uint64(receipts), *prev, 0, *prev))
	queue := fixture.QueueBytes()
	all := append([]entry{{"VERSION", version}, {"head.json", head}}, recs...)
	all = append(all, entry{"intent/queue.json", queue})
	m := &Manifest{
		ExportedAtSeq:     wire.SizeOf(receiptCount),
		HeadSha256:        wire.Sum(head),
		VersionSha256:     wire.Sum(version),
		PrimaryWorktree:   "/repo",
		ReceiptCount:      wire.SizeOf(receiptCount),
		LastReceiptSha256: *prev,
		HeadGeneration:    "0",
		Complete:          true,
	}
	m.QueueID, _ = wire.ParseQueueID("", fixture.QueueID)
	var tree []intent.File
	for _, e := range all {
		m.Files = append(m.Files, FileEntry{Path: e.name, Sha256: wire.Sum(e.data), Bytes: wire.SizeOf(uint64(len(e.data)))})
		if strings.HasPrefix(e.name, "intent/") {
			tree = append(tree, intent.File{Path: strings.TrimPrefix(e.name, "intent/"), Sha256: wire.Sum(e.data)})
		}
	}
	m.IntentTreeSha256 = intent.DigestOfFiles(tree)
	SortFiles(m.Files)
	ordered := make([]entry, 0, len(all))
	byName := map[string]entry{}
	for _, e := range all {
		byName[e.name] = e
	}
	for _, f := range m.Files {
		ordered = append(ordered, byName[f.Path])
	}
	return m, ordered
}

// TestTMV0022_AS36_N4eReceiptCountBeyondHead: receiptCount = lastSeq + 2
// never verifies (JOURNAL_FORKED), with or without the extra receipts.
func TestTMV0022_AS36_N4eReceiptCountBeyondHead(t *testing.T) {
	m, entries := handStore(t, 1, 3)
	if _, err := Verify(bytes.NewReader(buildStream(t, m, entries, true))); code(err) != wire.CodeJournalForked {
		t.Errorf("receiptCount beyond head: %v", err)
	}
	// Three receipts present but head at 1: the chain has receipts beyond
	// lastSeq → JOURNAL_FORKED.
	m3, e3 := handStore(t, 3, 3)
	m3.ReceiptCount = "3"
	m3.ExportedAtSeq = "3"
	// Replace head.json with one that says lastSeq 1 (digest of receipt 1).
	var r1 []byte
	for _, e := range e3 {
		if e.name == "receipts/000000000001.json" {
			r1 = e.data
		}
	}
	newHead := wire.EncodeFile(fixture.HeadValue("/repo", 1, wire.Sum(r1), 0, wire.Sum(r1)))
	for i := range e3 {
		if e3[i].name == "head.json" {
			e3[i].data = newHead
		}
	}
	for i := range m3.Files {
		if m3.Files[i].Path == "head.json" {
			m3.Files[i].Sha256 = wire.Sum(newHead)
			m3.Files[i].Bytes = wire.SizeOf(uint64(len(newHead)))
		}
	}
	m3.HeadSha256 = wire.Sum(newHead)
	SortFiles(m3.Files)
	byName := map[string]entry{}
	for _, e := range e3 {
		byName[e.name] = e
	}
	var ordered []entry
	for _, f := range m3.Files {
		ordered = append(ordered, byName[f.Path])
	}
	if _, err := Verify(bytes.NewReader(buildStream(t, m3, ordered, true))); code(err) != wire.CodeJournalForked {
		t.Errorf("receipts beyond head.lastSeq: %v", err)
	}
	// The consistent hand-built store verifies.
	m, entries = handStore(t, 2, 2)
	if _, err := Verify(bytes.NewReader(buildStream(t, m, entries, true))); err != nil {
		t.Errorf("consistent hand-built store: %v", err)
	}
}

// TestTMV0022_AS36_VerifyRejectsTornStreams covers truncation, a missing
// end-of-archive marker, trailing bytes, reordered entries, a mutated body,
// a broken chain link, a wrong head generation, and complete:false.
func TestTMV0022_AS36_VerifyRejectsTornStreams(t *testing.T) {
	m, entries := handStore(t, 2, 2)
	good := buildStream(t, m, entries, true)
	if _, err := Verify(bytes.NewReader(good)); err != nil {
		t.Fatalf("good stream: %v", err)
	}
	if _, err := Verify(bytes.NewReader(good[:len(good)-1024])); code(err) != wire.CodeMalformed {
		t.Errorf("missing marker: %v", err)
	}
	if _, err := Verify(bytes.NewReader(good[:len(good)/2])); code(err) != wire.CodeMalformed {
		t.Errorf("truncated: %v", err)
	}
	if _, err := Verify(bytes.NewReader(append(append([]byte{}, good...), 0))); code(err) != wire.CodeMalformed {
		t.Errorf("trailing byte: %v", err)
	}
	if _, err := Verify(bytes.NewReader(buildStream(t, m, entries, false))); code(err) != wire.CodeMalformed {
		t.Errorf("no marker written: %v", err)
	}
	if _, err := Verify(bytes.NewReader(nil)); code(err) != wire.CodeMalformed {
		t.Errorf("empty stream: %v", err)
	}
	swapped := append([]entry{}, entries...)
	swapped[0], swapped[1] = swapped[1], swapped[0]
	if _, err := Verify(bytes.NewReader(buildStream(t, m, swapped, true))); code(err) != wire.CodeMalformed {
		t.Errorf("reordered: %v", err)
	}
	mutated := append([]entry{}, entries...)
	for i := range mutated {
		if mutated[i].name == "VERSION" {
			mutated[i] = entry{"VERSION", []byte("taskman-state/0\r")}
		}
	}
	if _, err := Verify(bytes.NewReader(buildStream(t, m, mutated, true))); code(err) != wire.CodeMalformed {
		t.Errorf("mutated body: %v", err)
	}
	// Extra entry after the last files entry.
	extra := append(append([]entry{}, entries...), entry{"evidence/" + strings.Repeat("e", 64), []byte("x")})
	if _, err := Verify(bytes.NewReader(buildStream(t, m, extra, true))); code(err) != wire.CodeMalformed {
		t.Errorf("extra entry: %v", err)
	}
	// Stream that does not start with manifest.json.
	noManifest := buildStream(t, m, entries, true)
	tr := tar.NewReader(bytes.NewReader(noManifest))
	tr.Next()
	var rest bytes.Buffer
	rest.ReadFrom(tr)
	if _, err := Verify(bytes.NewReader(rest.Bytes())); code(err) != wire.CodeMalformed {
		t.Errorf("manifest-less stream: %v", err)
	}
	// complete:false never verifies.
	mf := *m
	mf.Complete = false
	if _, err := Verify(bytes.NewReader(buildStream(t, &mf, entries, true))); code(err) != wire.CodeMalformed {
		t.Errorf("complete:false: %v", err)
	}
	// Broken chain: receipt 2 whose prev names a foreign digest.
	broken := append([]entry{}, entries...)
	mb := *m
	mb.Files = append([]FileEntry{}, m.Files...)
	foreign := wire.Sum([]byte("foreign"))
	rc2 := wire.EncodeFile(fixture.ReceiptValue(2, &foreign, "MUTATION", 0))
	for i := range broken {
		if broken[i].name == "receipts/000000000002.json" {
			broken[i] = entry{broken[i].name, rc2}
		}
	}
	for i := range mb.Files {
		if mb.Files[i].Path == "receipts/000000000002.json" {
			mb.Files[i].Sha256 = wire.Sum(rc2)
			mb.Files[i].Bytes = wire.SizeOf(uint64(len(rc2)))
		}
	}
	// head must name the new receipt 2 digest for the chain check to be the
	// thing that fails.
	newHead := wire.EncodeFile(fixture.HeadValue("/repo", 2, wire.Sum(rc2), 0, wire.Sum(entries[0].data)))
	for i := range broken {
		if broken[i].name == "head.json" {
			broken[i] = entry{"head.json", newHead}
		}
	}
	for i := range mb.Files {
		if mb.Files[i].Path == "head.json" {
			mb.Files[i].Sha256 = wire.Sum(newHead)
			mb.Files[i].Bytes = wire.SizeOf(uint64(len(newHead)))
		}
	}
	mb.HeadSha256 = wire.Sum(newHead)
	mb.LastReceiptSha256 = wire.Sum(rc2)
	SortFiles(mb.Files)
	byName := map[string]entry{}
	for _, e := range broken {
		byName[e.name] = e
	}
	var ordered []entry
	for _, f := range mb.Files {
		ordered = append(ordered, byName[f.Path])
	}
	if _, err := Verify(bytes.NewReader(buildStream(t, &mb, ordered, true))); code(err) != wire.CodeJournalForked {
		t.Errorf("broken chain link: %v", err)
	}
	// Wrong headGeneration in the manifest.
	mg := *m
	mg.HeadGeneration = "1"
	if _, err := Verify(bytes.NewReader(buildStream(t, &mg, entries, true))); code(err) != wire.CodeJournalForked {
		t.Errorf("wrong generation: %v", err)
	}
	// Manifest schema: unknown key and unsupported version.
	raw := string(m.Encode())
	bad := strings.Replace(raw, `"profile":"taskman-archive/0"`, `"profile":"taskman-archive/1"`, 1)
	if _, err := DecodeManifest([]byte(bad)); code(err) != wire.CodeUnsupportedVersion {
		t.Errorf("manifest version: %v", err)
	}
	if _, err := DecodeManifest([]byte(strings.Replace(raw, `"complete":true`, `"complete":true,"extra":"x"`, 1))); code(err) != wire.CodeMalformed {
		t.Errorf("manifest unknown key: %v", err)
	}
}

type terminalFailure struct{ io.Reader }

func (r terminalFailure) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err == io.EOF {
		return n, syscall.EIO
	}
	return n, err
}
func TestTMV0022_AS09_TerminalReadError(t *testing.T) {
	_, repo := repoWithStore(t)
	stream, _, err := export(t, repo, fixture.TempDirOutside(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Verify(terminalFailure{bytes.NewReader(stream)}); err == nil || !strings.Contains(err.Error(), syscall.EIO.Error()) {
		t.Fatal("accepted terminal I/O failure")
	}
}

type prefixFailure struct{ buf bytes.Buffer }

func (w *prefixFailure) Write(p []byte) (int, error) {
	n := len(p)
	if n > 17 {
		n = 17
	}
	w.buf.Write(p[:n])
	return n, errors.New("destination failed")
}
func TestTMV0022_AS09_PartialDelivery(t *testing.T) {
	r, repo := repoWithStore(t)
	staging := fixture.TempDirOutside(t)
	before := fixture.TreeSnapshot(t, r.StateDir)
	intentBefore := fixture.TreeSnapshot(t, r.IntentDir)
	w := &prefixFailure{}
	res, err := Export(ExportOptions{Repo: repo, Staging: staging, Stdout: w})
	if res != nil || err == nil || w.buf.Len() != 17 || !strings.Contains(err.Error(), "17 bytes") {
		t.Fatalf("partial delivery: result=%v bytes=%d err=%v", res, w.buf.Len(), err)
	}
	fixture.AssertUntouched(t, r, before, intentBefore, "partial delivery")
	if entries, _ := os.ReadDir(staging); len(entries) != 0 {
		t.Fatal("staging leaked")
	}
}
