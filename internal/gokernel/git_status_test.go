//go:build darwin || linux

package gokernel

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStandaloneProbeRejectsExecutableGitFilters(t *testing.T) {
	t.Run("EAF-V0-007", func(t *testing.T) {
		for _, driver := range []string{"clean", "process"} {
			t.Run(driver, func(t *testing.T) {
				root := testRepository(t)
				marker := filepath.Join(root, "filter-executed")
				command := ": > '" + strings.ReplaceAll(marker, "'", "'\"'\"'") + "'; cat"
				runGit(t, root, "config", "filter.hostile."+driver, command)
				if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("*.go filter=hostile\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				source := filepath.Join(root, "src", "main.go")
				if err := os.WriteFile(source, []byte("package nope\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				old := time.Unix(1700000000, 0)
				if err := os.Chtimes(source, old, old); err != nil {
					t.Fatal(err)
				}
				if _, err := ProbeRepositoryContext(context.Background(), root); kernelCode(err) != "repository-probe-failed" {
					t.Fatalf("unsafe standalone probe error=%v", err)
				}
				if _, err := os.Stat(marker); !os.IsNotExist(err) {
					t.Fatalf("standalone probe executed filter: %v", err)
				}
			})
		}
	})
}

func TestStandaloneStatusPreservesCancellationCode(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := git(ctx, "/nonexistent", maxGitStatusBytes, "status", "--porcelain=v1", "-z"); kernelCode(err) != "repository-probe-cancelled" {
		t.Fatalf("cancelled standalone status error=%v", err)
	}
}
