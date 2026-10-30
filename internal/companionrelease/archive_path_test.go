package companionrelease

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func v13179EscapingPaths() []string {
	paths := []string{"../outside", "bin/../../outside", "/outside"}
	if runtime.GOOS == "windows" {
		paths = append(paths, "NUL", "bin/CON")
	}
	return paths
}

// V1-3179: verified extraction joins every entry through archivepath.Join, so
// it never relies on the decoder alone to keep a member beneath the target.
func TestV13179ExtractConfinesEntriesToTarget(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "target")
	ok := &VerifiedRetainedBundle{entries: []ArchiveEntry{{Path: "bin/corvint", Mode: 0o600, Data: []byte("x")}}}
	if err := ok.Extract(target); err != nil {
		t.Fatalf("local entry refused: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(target, "bin", "corvint")); err != nil || string(got) != "x" {
		t.Fatalf("extracted entry = %q, %v", got, err)
	}
	for _, p := range v13179EscapingPaths() {
		t.Run(p, func(t *testing.T) {
			base := t.TempDir()
			target := filepath.Join(base, "target")
			bad := &VerifiedRetainedBundle{entries: []ArchiveEntry{{Path: p, Mode: 0o600, Data: []byte("x")}}}
			if err := bad.Extract(target); err == nil {
				t.Fatalf("entry %q extracted", p)
			}
			if _, err := os.Stat(filepath.Join(base, "outside")); !os.IsNotExist(err) {
				t.Fatalf("entry %q wrote outside the target: %v", p, err)
			}
			if _, err := os.Lstat(target); !os.IsNotExist(err) {
				t.Fatalf("refused extraction left its target: %v", err)
			}
		})
	}
}

// V1-3179: the smoke tree writer applies the same containment.
func TestV13179WriteTreeConfinesEntriesToDir(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "tree")
	if err := writeTree(dir, []ArchiveEntry{{Path: "bin/corvint", Mode: 0o600, Data: []byte("x")}}); err != nil {
		t.Fatalf("local entry refused: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "bin", "corvint")); err != nil || string(got) != "x" {
		t.Fatalf("written entry = %q, %v", got, err)
	}
	for _, p := range v13179EscapingPaths() {
		t.Run(p, func(t *testing.T) {
			base := t.TempDir()
			if err := writeTree(filepath.Join(base, "tree"), []ArchiveEntry{{Path: p, Mode: 0o600, Data: []byte("x")}}); err == nil {
				t.Fatalf("entry %q written", p)
			}
			if _, err := os.Stat(filepath.Join(base, "outside")); !os.IsNotExist(err) {
				t.Fatalf("entry %q wrote outside the tree: %v", p, err)
			}
		})
	}
}
