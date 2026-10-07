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

// gitShim puts a `git` on PATH that logs every call to the returned log file,
// runs the shell `cases` (inside a `case "$argument" in ... esac` over the
// arguments, with %[1]s the log, %[2]s a sentinel path, %[3]s the real git
// and %[4]s oid) for the first matching argument, and otherwise execs the
// real git. It returns the log and sentinel paths.
func gitShim(t *testing.T, cases, oid string) (log, sentinel string) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	log = filepath.Join(bin, "calls")
	sentinel = filepath.Join(bin, "sentinel")
	shim := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$*" >> %[1]s
for argument do
  case "$argument" in
`+cases+`
  esac
done
exec %[3]s "$@"
`, strconv.Quote(log), strconv.Quote(sentinel), strconv.Quote(realGit), strconv.Quote(oid))
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(shim), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log, sentinel
}

// oneSourceRepository commits one Go module file and returns the root and
// the blob oid of go.mod, which the shims below reuse for fabricated entries.
func oneSourceRepository(t *testing.T) (root, oid string) {
	t.Helper()
	root = t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	benchmarkWriteFile(t, root, "go.mod", "module example.test/count\n\ngo 1.27.0\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "one source")
	return root, testGit(t, root, "rev-parse", "HEAD:go.mod")
}

// overLimitListing fabricates maxIndexedSources+1 blob entries in ls-tree's
// -l -z shape, every one pointing at the committed go.mod blob.
const overLimitListing = `      awk -v oid=%[4]s 'BEGIN { for (i = 0; i < 200001; i++) printf "100644 blob %%s %%7d\tf%%d.go\n", oid, 1, i }' | tr '\n' '\000'
      exit 0 ;;`

// TestBuildRefusesSourceCountBeforeStatusFinishesOrBlobsRead pins the order
// of a source-count refusal on a fresh observation (V1-0416): the tree listing
// runs beside the status scan, so a tree over maxIndexedSources is refused
// while the scan is still running, and no cat-file is ever spawned. The shim's
// ls-tree fabricates 200,001 blob entries; its status waits for a sentinel the
// test writes only after Build has returned, and logs when it finishes. A build
// that still waited for the scan would see that line before the refusal.
func TestBuildRefusesSourceCountBeforeStatusFinishesOrBlobsRead(t *testing.T) {
	root, oid := oneSourceRepository(t)
	log, sentinel := gitShim(t, `    status)
      waited=0
      while [ ! -f %[2]s ] && [ "$waited" -lt 100 ]; do /bin/sleep 0.1; waited=$((waited + 1)); done
      printf 'status-finished\n' >> %[1]s
      exec %[3]s "$@" ;;
    ls-tree)
`+overLimitListing+`
    cat-file)
      printf 'cat-file-spawned\n' >> %[1]s
      exit 95 ;;`, oid)

	_, err := Build(context.Background(), root)
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

// TestBuildReportsAnIndependentStatusFailureOverAnOverLimitListing pins the
// other half of the ordering: a status scan that fails on its own, before the
// listing is refused, keeps the refusal the sequential build gave. Only a scan
// that fails because the build cancelled it yields to the listing's error.
// The shim's status fails at once and releases the listing through the
// sentinel, so the status failure is complete before the listing is read.
func TestBuildReportsAnIndependentStatusFailureOverAnOverLimitListing(t *testing.T) {
	root, oid := oneSourceRepository(t)
	log, _ := gitShim(t, `    status)
      printf 'fatal: status refused by the shim\n' >&2
      : > %[2]s
      exit 128 ;;
    ls-tree)
      waited=0
      while [ ! -f %[2]s ] && [ "$waited" -lt 100 ]; do /bin/sleep 0.1; waited=$((waited + 1)); done
`+overLimitListing+`
    cat-file)
      printf 'cat-file-spawned\n' >> %[1]s
      exit 95 ;;`, oid)

	_, err := Build(context.Background(), root)
	if err == nil || !strings.Contains(err.Error(), "status refused by the shim") {
		t.Fatalf("Build error = %v, want the independent status failure", err)
	}
	calls, readErr := os.ReadFile(log)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if strings.Contains(string(calls), "cat-file") {
		t.Fatalf("a blob was read before the refusal:\n%s", calls)
	}
}
