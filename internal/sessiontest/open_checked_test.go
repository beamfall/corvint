package sessiontest

import (
	"os"
	"path/filepath"
	"testing"
)

// TestOpenCheckedRefusesSymlinkAndNonRegular pins the non-unix fallback open:
// a symlink, a directory or a FIFO is refused by the Lstat before any open,
// and a file swapped in before the open is refused by the identity check.
func TestOpenCheckedRefusesSymlinkAndNonRegular(t *testing.T) {
	dir := t.TempDir()
	regular := filepath.Join(dir, "input.json")
	if err := os.WriteFile(regular, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(dir, "other.json")
	if err := os.WriteFile(other, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := openChecked(regular, os.Open)
	if err != nil {
		t.Fatalf("regular file refused: %v", err)
	}
	f.Close()
	refuseOpen := func(string) (*os.File, error) {
		t.Fatal("opened a path the Lstat should have refused")
		return nil, nil
	}
	refused := map[string]string{"directory": dir}
	if link := filepath.Join(dir, "link.json"); os.Symlink(regular, link) == nil {
		refused["symlink"] = link
	}
	if fifo := filepath.Join(dir, "fifo.json"); mkfifo(fifo) == nil {
		refused["fifo"] = fifo
	}
	for label, path := range refused {
		if f, err := openChecked(path, refuseOpen); err == nil {
			f.Close()
			t.Errorf("%s accepted", label)
		}
	}
	swapped := func(string) (*os.File, error) { return os.Open(other) }
	if f, err := openChecked(regular, swapped); err == nil {
		f.Close()
		t.Error("a file swapped in before the open was accepted")
	}
}
