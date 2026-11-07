package observations

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSOLV0001TrackedGitignoreCoversBothLedgers pins that this repository's
// tracked ignore files satisfy the ledger writers' ignore check, so a clean
// checkout can record rows: `corvint index` writes .corvint/.gitignore only
// when it writes a snapshot, and a worktree reusing the shared snapshot never
// does, while the root .gitignore's negations fail the conservative check.
func TestSOLV0001TrackedGitignoreCoversBothLedgers(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".corvint"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".gitignore", filepath.Join(".corvint", ".gitignore")} {
		data, err := os.ReadFile(filepath.Join("..", "..", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, names := range [][]string{
		{"self-observations.jsonl", ".self-observations.*"},
		{"unplanned-reads.jsonl", ".unplanned-reads.*"},
	} {
		if !CorvintEntriesIgnored(root, names...) {
			t.Errorf("tracked .gitignore does not cover %v", names)
		}
	}
}
