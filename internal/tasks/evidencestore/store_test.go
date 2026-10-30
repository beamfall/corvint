package evidencestore_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/evidencestore"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func stateDir(t *testing.T) string {
	t.Helper()
	d := filepath.Join(t.TempDir(), "taskman")
	if err := os.Mkdir(d, 0o700); err != nil {
		t.Fatal(err)
	}
	return d
}

func code(err error) string {
	var we *wire.Error
	if errors.As(err, &we) {
		return wire.CodeOf(err)
	}
	return ""
}

// TestTOLV0134_PutWriteOnce: a put stores the bytes under their SHA-256, a
// repeat is EXISTING and leaves the file, and a stored file whose bytes no
// longer match its name is refused rather than overwritten.
func TestTOLV0134_PutWriteOnce(t *testing.T) {
	dir := stateDir(t)
	raw := []byte(`{"report":1}`)
	p, err := evidencestore.Put(dir, raw)
	if err != nil || p.Stored != evidencestore.StoredNew || p.Sha256 != wire.Sum(raw) || p.Bytes != int64(len(raw)) {
		t.Fatalf("put: %+v %v", p, err)
	}
	if p.Path != filepath.Join(dir, evidencestore.Dir, string(p.Sha256)) {
		t.Fatalf("path %s", p.Path)
	}
	if again, err := evidencestore.Put(dir, raw); err != nil || again.Stored != evidencestore.StoredExisting {
		t.Fatalf("repeat: %+v %v", again, err)
	}
	if err := os.WriteFile(p.Path, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := evidencestore.Put(dir, raw); code(err) != wire.CodeMalformed {
		t.Fatalf("a mismatched stored file was not refused: %v", err)
	}
	if got, _ := os.ReadFile(p.Path); string(got) != "tampered" {
		t.Fatal("a put replaced a stored file")
	}
	entries, err := os.ReadDir(filepath.Join(dir, evidencestore.Dir))
	if err != nil || len(entries) != 1 {
		t.Fatalf("temp files left behind: %v %v", entries, err)
	}
	if _, err := evidencestore.Put(filepath.Join(dir, "absent"), raw); code(err) != wire.CodeUninitialized {
		t.Fatalf("a put created a state directory: %v", err)
	}
}

// TestTOLV0136_GetVerifiesAndCreatesNothing: get re-hashes the stored
// bytes; an absent digest is MISSING_EVIDENCE, a tampered file MALFORMED,
// and a read on a store that does not exist creates nothing.
func TestTOLV0136_GetVerifiesAndCreatesNothing(t *testing.T) {
	dir := stateDir(t)
	d := wire.Sum([]byte("never stored"))
	if _, err := evidencestore.Get(dir, d); code(err) != wire.CodeMissingEvidence {
		t.Fatalf("absent store: %v", err)
	}
	if st, _ := evidencestore.Check(dir, d); st != evidencestore.StateMissing {
		t.Fatalf("check absent store: %s", st)
	}
	if entries, _ := evidencestore.List(dir); entries != nil {
		t.Fatalf("list of an absent store: %v", entries)
	}
	if _, err := os.Lstat(filepath.Join(dir, evidencestore.Dir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a read created the store")
	}
	raw := []byte("report bytes")
	p, err := evidencestore.Put(dir, raw)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := evidencestore.Get(dir, p.Sha256); err != nil || string(got) != string(raw) {
		t.Fatalf("get: %q %v", got, err)
	}
	if st, err := evidencestore.Check(dir, p.Sha256); st != evidencestore.StatePresent || err != nil {
		t.Fatalf("check: %s %v", st, err)
	}
	if err := os.WriteFile(p.Path, []byte("other"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := evidencestore.Get(dir, p.Sha256); code(err) != wire.CodeMalformed {
		t.Fatalf("tampered get: %v", err)
	}
	if st, _ := evidencestore.Check(dir, p.Sha256); st != evidencestore.StateMismatch {
		t.Fatalf("tampered check: %s", st)
	}
	if err := os.Remove(p.Path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "elsewhere"), p.Path); err != nil {
		t.Fatal(err)
	}
	if st, _ := evidencestore.Check(dir, p.Sha256); st != evidencestore.StateUnreadable {
		t.Fatalf("symlinked check: %s", st)
	}
}

// TestTOLV0133_ListSkipsTempRefusesUnknown: a put in progress is skipped;
// any other non-digest entry is refused, never ignored.
func TestTOLV0133_ListSkipsTempRefusesUnknown(t *testing.T) {
	dir := stateDir(t)
	p, err := evidencestore.Put(dir, []byte("a"))
	if err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(dir, evidencestore.Dir)
	if err := os.WriteFile(filepath.Join(store, evidencestore.TempPrefix+"0011"), []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err := evidencestore.List(dir)
	if err != nil || len(entries) != 1 || entries[0].Sha256 != p.Sha256 || entries[0].Bytes != 1 {
		t.Fatalf("list: %+v %v", entries, err)
	}
	if err := os.WriteFile(filepath.Join(store, "notes.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := evidencestore.List(dir); code(err) != wire.CodeMalformed {
		t.Fatalf("an unknown entry was not refused: %v", err)
	}
}

// TestTOLV0140_RemoveKeepsRefreshed: remove deletes a file last put before
// the cutoff and keeps one put again after it.
func TestTOLV0140_RemoveKeepsRefreshed(t *testing.T) {
	dir := stateDir(t)
	p, err := evidencestore.Put(dir, []byte("old"))
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := evidencestore.Remove(dir, p.Sha256, time.Now().Add(-time.Hour)); ok || err != nil {
		t.Fatalf("a fresh file was removed: %v %v", ok, err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(p.Path, old, old); err != nil {
		t.Fatal(err)
	}
	if ok, err := evidencestore.Remove(dir, p.Sha256, time.Now().Add(-time.Hour)); !ok || err != nil {
		t.Fatalf("an old file was kept: %v %v", ok, err)
	}
	if ok, err := evidencestore.Remove(dir, p.Sha256, time.Now()); ok || err != nil {
		t.Fatalf("absent remove: %v %v", ok, err)
	}
}
