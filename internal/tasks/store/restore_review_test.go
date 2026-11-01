package store

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// restoreFixture is a store with one ticket, its archive on disk, and its
// state dir and ticket files removed so a restore must recreate both.
func restoreFixture(t *testing.T) (*intent.Repository, string, string) {
	t.Helper()
	base := t.TempDir()
	real, err := filepath.EvalSymlinks(base)
	if err != nil {
		t.Fatal(err)
	}
	repo := historyStoreAt(t, filepath.Join(real, "repo"), 1, 2)
	var buf bytes.Buffer
	if _, err := archive.Export(archive.ExportOptions{Repo: repo, Staging: t.TempDir(), Stdout: &buf}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(real, "queue.tar")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	tickets := filepath.Join(repo.IntentRoot(), intent.Dir, "tickets")
	if err := os.RemoveAll(repo.StateDir); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(tickets); err != nil {
		t.Fatal(err)
	}
	return repo, path, tickets
}

// TestTRLV0871_FailedIntentWriteLeavesARetryableRoot (TRL-V0-871): a
// restore whose intent write fails part-way leaves no partial intent file
// and no state dir, and an interrupted restore's temp beside an intent file
// does not block the rerun, which removes it and completes.
func TestTRLV0871_FailedIntentWriteLeavesARetryableRoot(t *testing.T) {
	repo, path, tickets := restoreFixture(t)
	prev := restoreWriteFile
	t.Cleanup(func() { restoreWriteFile = prev })
	restoreWriteFile = func(f *os.File, raw []byte) error {
		if _, err := f.Write(raw[:len(raw)/2]); err != nil {
			return err
		}
		return errors.New("injected short write")
	}
	if _, err := RestoreArchive(context.Background(), repo, path); err == nil {
		t.Fatal("restore with a failing intent write succeeded")
	}
	if _, err := os.Lstat(repo.StateDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("state dir after a failed restore: %v", err)
	}
	entries, _ := os.ReadDir(tickets)
	for _, e := range entries {
		t.Errorf("failed restore left %s in the tickets dir", e.Name())
	}
	restoreWriteFile = prev
	// A kill mid-write leaves only the temp: simulate it and rerun.
	if err := os.MkdirAll(tickets, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(tickets, "V1-0001.json"+restoreTempInfix+"123")
	if err := os.WriteFile(stale, []byte("{\"trunc"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := RestoreArchive(context.Background(), repo, path)
	if err != nil {
		t.Fatalf("rerun after an interrupted restore: %v", err)
	}
	if rep.IntentWritten == 0 {
		t.Fatal("rerun wrote no intent file")
	}
	if _, err := os.Lstat(stale); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rerun kept the interrupted restore's temp: %v", err)
	}
	entries, _ = os.ReadDir(tickets)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Errorf("restore left temp %s", e.Name())
		}
	}
}

// TestTRLV0871_RestoreSyncsIntentDirectories (TRL-V0-871): every directory
// holding a written intent file is synced before the state dir appears.
func TestTRLV0871_RestoreSyncsIntentDirectories(t *testing.T) {
	repo, path, tickets := restoreFixture(t)
	prev := restoreSyncDir
	t.Cleanup(func() { restoreSyncDir = prev })
	synced := map[string]bool{}
	restoreSyncDir = func(d string) error {
		if _, err := os.Lstat(repo.StateDir); err == nil {
			t.Errorf("sync of %s after the state dir was published", d)
		}
		synced[d] = true
		return prev(d)
	}
	if _, err := RestoreArchive(context.Background(), repo, path); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{tickets, filepath.Join(repo.IntentRoot(), intent.Dir)} {
		if !synced[d] {
			t.Errorf("intent directory %s was not synced before publication", d)
		}
	}
}

// TestTRLV0871_DuplicateStagedMemberRefuses (TRL-V0-871): a staging read
// that meets one verified member twice and another not at all refuses,
// instead of materializing the absent member as an empty file.
func TestTRLV0871_DuplicateStagedMemberRefuses(t *testing.T) {
	a, b := []byte(`{"a":1}`), []byte(`{"b":2}`)
	want := map[string]archive.FileEntry{
		"intent/tickets/A.json": {Path: "intent/tickets/A.json", Sha256: wire.Sum(a), Bytes: wire.SizeOf(uint64(len(a)))},
		"intent/tickets/B.json": {Path: "intent/tickets/B.json", Sha256: wire.Sum(b), Bytes: wire.SizeOf(uint64(len(b)))},
	}
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for i := 0; i < 2; i++ {
		if err := tw.WriteHeader(&tar.Header{Name: "intent/tickets/A.json", Typeflag: tar.TypeReg, Size: int64(len(a)), Mode: 0o644}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(a); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := restoreStage(&buf, t.TempDir(), want, &RestoreReport{})
	if wire.CodeOf(err) != wire.CodeMalformed {
		t.Fatalf("duplicate member staged: err=%v, members=%d", err, len(raw))
	}
}

// TestTRLV0870_OversizedIntentFileRefusesUnread (TRL-V0-870): an existing
// intent file larger than the archive's member refuses INTENT_DIVERGED
// without being loaded.
func TestTRLV0870_OversizedIntentFileRefusesUnread(t *testing.T) {
	root := t.TempDir()
	repo := &intent.Repository{PrimaryWorktree: root}
	q := filepath.Join(root, intent.Dir, intent.QueueFile)
	if err := os.MkdirAll(filepath.Dir(q), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(q)
	if err != nil {
		t.Fatal(err)
	}
	const huge = 256 << 20
	if err := f.Truncate(huge); err != nil {
		t.Fatal(err)
	}
	f.Close()
	small := []byte(`{"queue":1}`)
	want := map[string]archive.FileEntry{"intent/" + intent.QueueFile: {Path: "intent/" + intent.QueueFile, Sha256: wire.Sum(small), Bytes: wire.SizeOf(uint64(len(small)))}}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	_, err = restoreIntent(repo, want, false)
	runtime.ReadMemStats(&after)
	if wire.CodeOf(err) != wire.CodeIntentDiverged {
		t.Fatalf("oversized queue.json: %v", err)
	}
	if grew := after.TotalAlloc - before.TotalAlloc; grew > 16<<20 {
		t.Fatalf("restore preflight allocated %d bytes for an oversized intent file", grew)
	}
}
