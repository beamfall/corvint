//go:build darwin || linux

package authoritystore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepositoryConfigurationDetectsRestoredBytesAndMissingTransitions(t *testing.T) {
	t.Run("PLE-V0-003 configuration witness detects restored bytes", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "config")
		before, err := readRepositoryMetadata(p)
		if err != nil || before.present {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("[core]\n"), 0600); err != nil {
			t.Fatal(err)
		}
		first, err := readRepositoryMetadata(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("[core]\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, first.info.ModTime(), first.info.ModTime()); err != nil {
			t.Fatal(err)
		}
		b := repositoryBinding{configuration: []repositoryMetadata{first}}
		if b.configurationUnchanged() == nil {
			t.Fatal("restored content/mtime hid change time")
		}
		if err := os.Remove(p); err != nil {
			t.Fatal(err)
		}
		if b.configurationUnchanged() == nil {
			t.Fatal("removed config")
		}
		if err := os.Symlink("missing", p); err != nil {
			t.Fatal(err)
		}
		if _, err := readRepositoryMetadata(p); err == nil {
			t.Fatal("configuration symlink")
		}

	})
}
