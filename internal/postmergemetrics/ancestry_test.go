//go:build darwin || linux

package postmergemetrics

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestMeasureRefusesRewrittenAncestry_PMMV0008 pins that a grafted or shallow
// checkout is unavailable Git evidence even though Git still answers that bot
// is an ancestor of approved, and that the plain checkout measures again.
func TestMeasureRefusesRewrittenAncestry_PMMV0008(t *testing.T) {
	for _, tc := range []struct{ name, file string }{
		{"grafts", filepath.Join(".git", "info", "grafts")},
		{"shallow", filepath.Join(".git", "shallow")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := gitFixture(t)
			put(t, root, "file", []byte("old\n"))
			bot := commit(t, root)
			put(t, root, "file", []byte("new\n"))
			approved := commit(t, root)
			g, err := NewGitMeasurer()
			if err != nil {
				t.Fatal(err)
			}
			pair := GitPair{root, bot, approved}
			path := filepath.Join(root, tc.file)
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(bot+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := g.Measure(context.Background(), pair); err != ErrGit {
				t.Fatalf("Measure = %v, want %v", err, ErrGit)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if m := measure(t, g, pair); m.Files != 1 {
				t.Fatalf("plain checkout: %+v", m)
			}
		})
	}
}
