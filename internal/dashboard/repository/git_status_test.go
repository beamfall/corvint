//go:build darwin || linux

package repository

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestAuthorityStatusCannotExecuteConfiguredFilter(t *testing.T) {
	t.Run("EAF-V0-007", func(t *testing.T) {
		git, err := exec.LookPath("git")
		if err != nil {
			t.Fatal(err)
		}
		root := stableTestDirectory(t, "status-filter-")
		runTestGit(t, git, root, "init", "-q")
		source := filepath.Join(root, "value.go")
		if err := os.WriteFile(source, []byte("package value\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("*.go filter=hostile\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		runTestGit(t, git, root, "add", ".")
		runTestGit(t, git, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-qm", "fixture")
		marker := filepath.Join(t.TempDir(), "filter-executed")
		// No shell metacharacters: this value passes the authority's existing
		// config grammar, which is not an executable-filter screen.
		runTestGit(t, git, root, "config", "filter.hostile.clean", "touch "+marker)
		if err := os.WriteFile(source, []byte("package other\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		old := time.Unix(1700000000, 0)
		if err := os.Chtimes(source, old, old); err != nil {
			t.Fatal(err)
		}
		authority, _, failure := New(context.Background(), root)
		if authority != nil {
			authority.Close()
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatalf("dashboard status executed configured filter: %v", err)
		}
		if failure == nil {
			t.Fatal("unsafe dashboard status admitted")
		}
	})
}
