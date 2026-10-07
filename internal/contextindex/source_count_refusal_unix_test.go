//go:build darwin || linux

package contextindex

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestBuildRefusesSourceCountBeforeStatusFinishesOrBlobsRead pins the order
// of a source-count refusal on a fresh observation (V1-0416): the tree listing
// runs beside the status scan, so a tree over maxIndexedSources is refused
// while the scan is still running, and no cat-file is ever spawned. The shim's
// ls-tree fabricates 200,001 blob entries; its status waits for a sentinel the
// test writes only after Build has returned, and logs when it finishes. A build
// that still waited for the scan would see that line before the refusal.
func TestBuildRefusesSourceCountBeforeStatusFinishesOrBlobsRead(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	benchmarkWriteFile(t, root, "go.mod", "module example.test/count\n\ngo 1.27.0\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "one source")
	oid := testGit(t, root, "rev-parse", "HEAD:go.mod")

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	log := filepath.Join(bin, "calls")
	sentinel := filepath.Join(bin, "release-status")
	shim := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$*" >> %[1]s
for argument do
  case "$argument" in
    status)
      waited=0
      while [ ! -f %[2]s ] && [ "$waited" -lt 100 ]; do /bin/sleep 0.1; waited=$((waited + 1)); done
      printf 'status-finished\n' >> %[1]s
      exec %[3]s "$@" ;;
    ls-tree)
      awk -v oid=%[4]s 'BEGIN { for (i = 0; i < 200001; i++) printf "100644 blob %%s %%7d\tf%%d.go\n", oid, 1, i }' | tr '\n' '\000'
      exit 0 ;;
    cat-file)
      printf 'cat-file-spawned\n' >> %[1]s
      exit 95 ;;
  esac
done
exec %[3]s "$@"
`, strconv.Quote(log), strconv.Quote(sentinel), strconv.Quote(realGit), strconv.Quote(oid))
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(shim), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err = Build(context.Background(), root)
	calls, readErr := os.ReadFile(log)
	if writeErr := os.WriteFile(sentinel, nil, 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}
	if err == nil || err.Error() != "Git tree exceeds the source-count limit" {
		t.Fatalf("Build error = %v, want the source-count refusal", err)
	}
	if readErr != nil {
		t.Fatal(readErr)
	}
	if strings.Contains(string(calls), "status-finished") {
		t.Fatalf("the refusal waited for the status scan:\n%s", calls)
	}
	if strings.Contains(string(calls), "cat-file") {
		t.Fatalf("a blob was read before the refusal:\n%s", calls)
	}
	if !strings.Contains(string(calls), "ls-tree -r -l -z --full-tree") {
		t.Fatalf("no tree listing was spawned:\n%s", calls)
	}
}
