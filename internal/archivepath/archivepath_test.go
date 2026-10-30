package archivepath

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestV13179JoinKeepsLocalMembers(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a", "a/b.txt", "dir/", "a..b/c", "./a"} {
		got, err := Join(root, name)
		if err != nil {
			t.Fatalf("Join(%q): %v", name, err)
		}
		if want := filepath.Join(root, filepath.FromSlash(name)); got != want {
			t.Fatalf("Join(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestV13179JoinRefusesEscapingMembers(t *testing.T) {
	root := t.TempDir()
	names := []string{"", "..", "../x", "a/../../x", "/etc/passwd", "a/../.."}
	if runtime.GOOS == "windows" {
		names = append(names, `C:\x`, `C:x`, `\x`, "NUL", "a/CON")
	}
	for _, name := range names {
		if got, err := Join(root, name); err == nil {
			t.Fatalf("Join(%q) = %q, want refusal", name, got)
		}
	}
}
