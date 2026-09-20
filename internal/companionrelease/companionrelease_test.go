//go:build darwin || linux

package companionrelease

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// --- target validation ---

func TestValidateTargetRefusesUnsupported(t *testing.T) {
	for _, target := range []string{"darwin/amd64", "linux/arm64", "windows/amd64", "", "bogus"} {
		if err := validateTarget(target); err == nil {
			t.Fatalf("target %q: want refusal, got nil", target)
		}
	}
	if err := validateTarget(supportedTarget); err != nil {
		t.Fatalf("supported target refused: %v", err)
	}
}

// --- unsafe path refusal ---

func TestRefuseUnsafePath(t *testing.T) {
	bad := []string{"", "/abs", "a/../b", "..", ".", "a\\b", ".git/config", "a/.git/x", ".GIT/config", "a/.Git/x"}
	for _, p := range bad {
		if err := refuseUnsafePath(p); err == nil {
			t.Fatalf("path %q: want refusal, got nil", p)
		}
	}
	good := []string{"a/b.go", "README.md", "cmd/x/main.go"}
	for _, p := range good {
		if err := refuseUnsafePath(p); err != nil {
			t.Fatalf("path %q: unexpected refusal: %v", p, err)
		}
	}
}

// --- blob digest refusal ---

func TestVerifyBlobDigestRefusesMismatch(t *testing.T) {
	data := []byte("hello world")
	// sha1("blob 11\x00hello world")
	if err := verifyBlobDigest("95d09f2b10159347eece71399a7e2e907ea3df4f", data); err != nil {
		t.Fatalf("correct sha1 oid refused: %v", err)
	}
	if err := verifyBlobDigest("0000000000000000000000000000000000000a", data); err == nil {
		t.Fatal("wrong sha1 oid: want refusal, got nil")
	}
	if err := verifyBlobDigest("nothex", data); err == nil {
		t.Fatal("malformed oid: want refusal, got nil")
	}
}

// --- listTreeEntries bounds and hygiene ---
//
// listTreeEntries takes a `git` runner closure rather than a repo, so these
// tests fake that closure to return canned `ls-tree -rz` output — no real
// Git process or checkout is needed to exercise the parsing refusals.

func fakeLsTreeGit(output string) func(time.Duration, []byte, ...string) ([]byte, error) {
	return func(time.Duration, []byte, ...string) ([]byte, error) {
		return []byte(output), nil
	}
}

// TestListTreeEntriesParsesLargeEntryCount exercises listTreeEntries at a
// scale past maxExportEntries; exportSource enforces the actual bound
// refusal (len(entries) > maxExportEntries) immediately after calling this
// parser, using the real HEAD tree and cat-file passes.
// TestExportSourceRefusesOversizedEntryCount below drives that refusal
// end-to-end.
func TestListTreeEntriesParsesLargeEntryCount(t *testing.T) {
	var b strings.Builder
	for i := 0; i < maxExportEntries+1; i++ {
		fmt.Fprintf(&b, "100644 blob 95d09f2b10159347eece71399a7e2e907ea3df4f\tf%d\x00", i)
	}
	entries, err := listTreeEntries(fakeLsTreeGit(b.String()), "HEAD")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(entries) <= maxExportEntries {
		t.Fatalf("test fixture produced %d entries, want > %d", len(entries), maxExportEntries)
	}
}

// TestExportSourceRefusesOversizedEntryCount drives exportSource's real
// maxExportEntries refusal against an actual Git repository, closing the gap
// TestListTreeEntriesParsesLargeEntryCount's parser-only fixture leaves: a
// repo with maxExportEntries+1 regular-file tree entries, built with one
// shared blob via `git mktree`/`git commit-tree` rather than that many
// working-tree files, so the fixture stays proportionate to what it checks.
func TestExportSourceRefusesOversizedEntryCount(t *testing.T) {
	t.Run("PUB-V0-012 source-export-entry-bound", func(t *testing.T) {
		gitPath, err := exec.LookPath("git")
		if err != nil {
			t.Skip("git not on PATH")
		}
		root, scratch := t.TempDir(), t.TempDir()
		env := append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@x", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@x")
		git := func(stdin string, args ...string) string {
			t.Helper()
			cmd := exec.Command(gitPath, args...)
			cmd.Dir, cmd.Env, cmd.Stdin = root, env, strings.NewReader(stdin)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
			return strings.TrimSpace(string(out))
		}
		git("", "init", "-q")
		blob := git("", "hash-object", "-w", "--stdin")
		var tree strings.Builder
		for i := 0; i < maxExportEntries+1; i++ {
			fmt.Fprintf(&tree, "100644 blob %s\tf%d\n", blob, i)
		}
		treeOID := git(tree.String(), "mktree")
		commit := git("", "commit-tree", treeOID, "-m", "oversized export fixture")
		git("", "update-ref", "HEAD", commit)

		got, err := exportSource(context.Background(), gitPath, root, scratch)
		wantErr := fmt.Sprintf("export entries %d exceed bound %d", maxExportEntries+1, maxExportEntries)
		if err == nil {
			t.Fatal("export entries over bound: want refusal, got nil")
		}
		if err.Error() != wantErr {
			t.Fatalf("error = %q, want %q", err, wantErr)
		}
		if got.Root != "" || got.HeadCommit != "" || got.HeadTree != "" || got.Files != nil || got.TotalBytes != 0 {
			t.Fatalf("refused export returned output: %+v", got)
		}
		entries, err := os.ReadDir(scratch)
		if err != nil {
			t.Fatalf("read scratch directory: %v", err)
		}
		if len(entries) != 0 {
			t.Fatalf("refused export wrote %d scratch entries", len(entries))
		}
	})
}

func TestListTreeEntriesRefusesSymlinkAndSubmodule(t *testing.T) {
	cases := []string{
		"120000 blob 95d09f2b10159347eece71399a7e2e907ea3df4f\tlink\x00",
		"160000 commit 95d09f2b10159347eece71399a7e2e907ea3df4f\tsubmod\x00",
	}
	for _, raw := range cases {
		if _, err := listTreeEntries(fakeLsTreeGit(raw), "HEAD"); err == nil {
			t.Fatalf("entry %q: want refusal, got nil", raw)
		}
	}
}

func TestListTreeEntriesRefusesDuplicatePaths(t *testing.T) {
	raw := "100644 blob 95d09f2b10159347eece71399a7e2e907ea3df4f\ta\x00" +
		"100644 blob 95d09f2b10159347eece71399a7e2e907ea3df4f\ta\x00"
	if _, err := listTreeEntries(fakeLsTreeGit(raw), "HEAD"); err == nil {
		t.Fatal("duplicate path: want refusal, got nil")
	}
}

// TestListTreeEntriesRefusesCaseFoldCollisions drives PUB-V0-012's
// case-fold refusal at the export's read boundary, before exportSource
// returns and before fixtures.go's materializeExport ever writes a byte: a
// tree holding both "A.go" and "a.go" would otherwise reach a
// case-insensitive filesystem as one file holding whichever entry's write
// landed last (docs/agent-memory/bugs.md, 2026-09-12). The second case is
// the directory-prefix variant: a file "A" case-folds onto the implied
// directory segment of "a/b.go".
func TestListTreeEntriesRefusesCaseFoldCollisions(t *testing.T) {
	cases := []string{
		"100644 blob 95d09f2b10159347eece71399a7e2e907ea3df4f\tA.go\x00" +
			"100644 blob 95d09f2b10159347eece71399a7e2e907ea3df4f\ta.go\x00",
		"100644 blob 95d09f2b10159347eece71399a7e2e907ea3df4f\tA\x00" +
			"100644 blob 95d09f2b10159347eece71399a7e2e907ea3df4f\ta/b.go\x00",
		"100644 blob 95d09f2b10159347eece71399a7e2e907ea3df4f\tA/b.go\x00" +
			"100644 blob 95d09f2b10159347eece71399a7e2e907ea3df4f\ta\x00",
	}
	for _, raw := range cases {
		if _, err := listTreeEntries(fakeLsTreeGit(raw), "HEAD"); err == nil {
			t.Fatalf("case-fold collision %q: want refusal, got nil", raw)
		}
	}
}

// --- exportSource entry-count and total-bytes bound refusals ---
//
// exportSource takes gitPath as a plain executable path that procgroup execs
// directly (no PATH lookup, no shell), so a tiny script stands in for git: it
// answers rev-parse/ls-tree/cat-file by catting canned fixture files placed
// under exportSource's own HOME (closedGitEnv sets HOME to the scratchParent
// argument). That reaches exportSource's real bound checks (export.go:71 and
// export.go:102) directly, without committing a 10,001-file or 128 MiB+ real
// Git repository.

const fakeGitScript = `#!/bin/sh
case "$1" in
  rev-parse) echo "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef" ;;
  ls-tree) cat "$HOME/lstree.out" ;;
  cat-file) cat "$HOME/catfile.out" ;;
  *) exit 1 ;;
esac
`

func writeFakeGit(t *testing.T, home string) string {
	t.Helper()
	path := filepath.Join(home, "fake-git.sh")
	if err := os.WriteFile(path, []byte(fakeGitScript), 0o755); err != nil {
		t.Fatalf("write fake git script: %v", err)
	}
	return path
}

// TestExportSourceRefusesTotalBytesBound drives export.go:102's cumulative
// total > maxExportTotalBytes refusal inside exportSource itself: two blobs,
// each individually under the per-blob size parseBatchHeader itself enforces
// (export.go:278), whose sizes sum just past the bound. Both carry real sha1
// digests so verifyBlobDigest accepts them before the byte-count check runs.
// The margin over the bound is kept well under 1 MiB: runCapturedStdin caps
// captured cat-file output at maxExportTotalBytes+1MiB (proc.go:53), so
// overshooting the bound by too much trips that raw output-size guard
// instead of exportSource's own byte-count refusal.
func TestExportSourceRefusesTotalBytesBound(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitPath := writeFakeGit(t, home)

	const overBoundBy = 500_000
	sizes := []int{maxExportTotalBytes / 2, maxExportTotalBytes/2 + overBoundBy} // each under maxExportTotalBytes alone; sum is not
	var lstree strings.Builder
	var catfile bytes.Buffer
	for i, size := range sizes {
		data := make([]byte, size)
		frame := fmt.Sprintf("blob %d\x00", size)
		h := sha1.New()
		h.Write([]byte(frame))
		h.Write(data)
		oid := fmt.Sprintf("%x", h.Sum(nil))

		fmt.Fprintf(&lstree, "100644 blob %s\tf%d\x00", oid, i)
		fmt.Fprintf(&catfile, "%s blob %d\n", oid, size)
		catfile.Write(data)
		catfile.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(home, "lstree.out"), []byte(lstree.String()), 0o644); err != nil {
		t.Fatalf("write lstree fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, "catfile.out"), catfile.Bytes(), 0o644); err != nil {
		t.Fatalf("write catfile fixture: %v", err)
	}

	if _, err := exportSource(context.Background(), gitPath, root, home); err == nil {
		t.Fatal("total bytes over bound: want refusal, got nil")
	} else if !strings.Contains(err.Error(), "export total bytes exceed bound") {
		t.Fatalf("want total-bytes bound refusal, got: %v", err)
	}
}

// --- batchBlobs/batchBlobsAlt: no data after the last requested object ---

// TestBatchBlobsRefusesTrailingData guards both cat-file batch parsers
// against silently ignoring bytes after the last requested object: each
// only reads exactly len(oids) records and returned successfully even when
// the fake git process appended data past the last one, so a `cat-file
// --batch` stream carrying more than what listTreeEntries actually asked
// for went unnoticed instead of refused.
func TestBatchBlobsRefusesTrailingData(t *testing.T) {
	entries := []treeEntry{{oid: "95d09f2b10159347eece71399a7e2e907ea3df4f", path: "f"}}
	valid := fmt.Sprintf("%s blob %d\n%s\n", entries[0].oid, len("hello"), "hello")
	fakeGit := func(time.Duration, []byte, ...string) ([]byte, error) {
		return []byte(valid + "unexpected-trailing-object\n"), nil
	}
	if _, err := batchBlobs(fakeGit, entries, forwardOrder); err == nil {
		t.Fatal("batchBlobs: trailing data after the last object: want refusal, got nil")
	}
	if _, err := batchBlobsAlt(fakeGit, entries, forwardOrder); err == nil {
		t.Fatal("batchBlobsAlt: trailing data after the last object: want refusal, got nil")
	}
}

// --- canonical tar.gz archive corruption refusal ---

func TestVerifyTarGzRefusesTamperedArchive(t *testing.T) {
	entries := []ArchiveEntry{
		{Path: "a.txt", Mode: 0o644, Data: []byte("alpha")},
		{Path: "dir/b.txt", Mode: 0o755, Data: []byte("beta")},
	}
	archive, err := buildTarGz(entries)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := verifyTarGz(archive, entries); err != nil {
		t.Fatalf("untampered archive refused: %v", err)
	}

	if err := verifyTarGz(archive[:len(archive)-1], entries); err == nil {
		t.Fatal("truncated archive: want refusal, got nil")
	}
	if err := verifyTarGz(append(append([]byte(nil), archive...), 0xAB), entries); err == nil {
		t.Fatal("trailing-byte archive: want refusal, got nil")
	}

	wrongMode := []ArchiveEntry{
		{Path: "a.txt", Mode: 0o755, Data: []byte("alpha")},
		{Path: "dir/b.txt", Mode: 0o755, Data: []byte("beta")},
	}
	if err := verifyTarGz(archive, wrongMode); err == nil {
		t.Fatal("mode mismatch against expectation: want refusal, got nil")
	}

	missingMember := entries[:1]
	if err := verifyTarGz(archive, missingMember); err == nil {
		t.Fatal("archive with an unexpected extra member: want refusal, got nil")
	}
}

// TestVerifyTarGzRefusesSecondGzipMember drives PUB-V0-014's single-member
// rule: a second complete gzip member appended to a valid archive, and bytes
// hidden after the tar terminator inside the one member, must both be refused
// rather than silently decoded and discarded.
func TestVerifyTarGzRefusesSecondGzipMember(t *testing.T) {
	entries := []ArchiveEntry{{Path: "a.txt", Mode: 0o644, Data: []byte("alpha")}}
	archive, err := buildTarGz(entries)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var extra bytes.Buffer
	gw := gzip.NewWriter(&extra)
	if _, err := gw.Write([]byte("smuggled")); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := verifyTarGz(append(append([]byte(nil), archive...), extra.Bytes()...), entries); err == nil {
		t.Fatal("archive with a second gzip member: want refusal, got nil")
	}

	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	if err := tw.WriteHeader(&tar.Header{Name: "a.txt", Typeflag: tar.TypeReg, Mode: 0o644, Size: 5}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("alpha")); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	tarBuf.WriteString("hidden after the terminator")
	var single bytes.Buffer
	gw = gzip.NewWriter(&single)
	if _, err := gw.Write(tarBuf.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := verifyTarGz(single.Bytes(), entries); err == nil {
		t.Fatal("bytes after the tar terminator: want refusal, got nil")
	}
}

// TestVerifyTarGzRefusesCaseFoldedMembers drives PUB-V0-012's case-fold
// refusal at the archive-member verification boundary: buildTarGz's own
// duplicate check is exact-path only, so it happily assembles an archive
// holding both "A.go" and "a.go"; verifyTarGz must still refuse it before
// the archive is retained, since extracting it onto a case-insensitive
// filesystem collapses the two members into one.
func TestVerifyTarGzRefusesCaseFoldedMembers(t *testing.T) {
	entries := []ArchiveEntry{
		{Path: "A.go", Mode: 0o644, Data: []byte("upper")},
		{Path: "a.go", Mode: 0o644, Data: []byte("lower")},
	}
	archive, err := buildTarGz(entries)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := verifyTarGz(archive, entries); err == nil {
		t.Fatal("case-folded archive members: want refusal, got nil")
	}
}

func TestBuildTarGzRefusesDuplicatePathsAndBadModes(t *testing.T) {
	if _, err := buildTarGz([]ArchiveEntry{
		{Path: "a", Mode: 0o644, Data: []byte("x")},
		{Path: "a", Mode: 0o644, Data: []byte("y")},
	}); err == nil {
		t.Fatal("duplicate archive path: want refusal, got nil")
	}
	if _, err := buildTarGz([]ArchiveEntry{{Path: "a", Mode: 0o600, Data: []byte("x")}}); err == nil {
		t.Fatal("non-canonical mode: want refusal, got nil")
	}
}

func TestBuildTarGzTwiceIsDeterministic(t *testing.T) {
	entries := []ArchiveEntry{
		{Path: "z.txt", Mode: 0o644, Data: []byte("last")},
		{Path: "a.txt", Mode: 0o644, Data: []byte("first")},
	}
	a, err := buildTarGz(entries)
	if err != nil {
		t.Fatal(err)
	}
	b, err := buildTarGz(entries)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Fatal("two assemblies of identical entries produced different bytes")
	}
}

func TestBuildTarGzTwiceRefusesDivergingBuilds(t *testing.T) {
	calls := 0
	build := func() ([]ArchiveEntry, error) {
		calls++
		data := "first-build"
		if calls == 2 {
			data = "second-build" // genuinely different from the first call's result
		}
		return []ArchiveEntry{{Path: "a.txt", Mode: 0o644, Data: []byte(data)}}, nil
	}
	if _, err := buildTarGzTwice(build); err == nil {
		t.Fatal("build() returning different entries on its two calls: want refusal, got nil")
	}
	if calls != 2 {
		t.Fatalf("want build called exactly twice, got %d", calls)
	}
}

func TestBuildTarGzTwiceAcceptsAgreeingIndependentBuilds(t *testing.T) {
	calls := 0
	build := func() ([]ArchiveEntry, error) {
		calls++
		// A fresh slice each call, not a shared reference, but with equal content.
		return []ArchiveEntry{{Path: "a.txt", Mode: 0o644, Data: []byte("stable")}}, nil
	}
	if _, err := buildTarGzTwice(build); err != nil {
		t.Fatalf("two independently built but agreeing entry sets: want success, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("want build called exactly twice, got %d", calls)
	}
}

// --- notices: sourced from the Git-verified export, not the live checkout ---

func TestNoticeEntriesReadsFromExportNotWorktree(t *testing.T) {
	moduleRoot := t.TempDir()
	// A worktree file that disagrees with what the (pinned) export carries.
	// If noticeEntries ever falls back to disk, this tampered content would
	// leak into the bundle instead of the Git-pinned text.
	if err := os.WriteFile(filepath.Join(moduleRoot, "LICENSE"), []byte("tampered worktree content"), 0o644); err != nil {
		t.Fatalf("write worktree LICENSE: %v", err)
	}
	export := Export{
		Root: moduleRoot,
		Files: []SourceFile{
			{Path: "LICENSE", Mode: "100644", OID: "deadbeef", Data: []byte("pinned git content")},
		},
	}

	entries, err := noticeEntries(export, "notices/corvint", []string{"LICENSE"})
	if err != nil {
		t.Fatalf("noticeEntries: %v", err)
	}
	if len(entries) != 1 || string(entries[0].Data) != "pinned git content" {
		t.Fatalf("want the export's pinned LICENSE content, got %+v", entries)
	}

	if _, err := noticeEntries(export, "notices/corvint", []string{"PROVENANCE.md"}); err == nil {
		t.Fatal("notice absent from the export: want refusal, got nil")
	}
}

// --- retainBundle: no output on failure, never overwrite ---

func TestRetainBundleNeverOverwrites(t *testing.T) {
	scratch := t.TempDir()
	outputParent := t.TempDir()
	files := []ArchiveEntry{{Path: "a.txt", Mode: 0o644, Data: []byte("x")}}

	path, err := retainBundle(scratch, outputParent, "bundle", files)
	if err != nil {
		t.Fatalf("first retain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(path, "a.txt")); err != nil {
		t.Fatalf("retained file missing: %v", err)
	}

	if _, err := retainBundle(scratch, outputParent, "bundle", []ArchiveEntry{{Path: "b.txt", Mode: 0o644, Data: []byte("y")}}); err == nil {
		t.Fatal("retaining over an existing bundle dir: want refusal, got nil")
	}
	if _, err := os.Stat(filepath.Join(outputParent, "bundle", "b.txt")); !os.IsNotExist(err) {
		t.Fatal("failed second retain leaked partial output into the existing bundle")
	}
}

// TestWriteTreeClearsStaleEntries guards PUB-V0-015's "exact extracted
// bundle bytes": a reused smoke-extract scratch directory must not still
// carry a file from an earlier attempt's entry set once the current bundle
// no longer includes it, or the installed smoke test would run against
// extra files that were never part of what got assembled.
func TestWriteTreeClearsStaleEntries(t *testing.T) {
	dir := t.TempDir()
	if err := writeTree(dir, []ArchiveEntry{
		{Path: "a.txt", Mode: 0o644, Data: []byte("first")},
		{Path: "stale.txt", Mode: 0o644, Data: []byte("old")},
	}); err != nil {
		t.Fatalf("first writeTree: %v", err)
	}
	if err := writeTree(dir, []ArchiveEntry{{Path: "a.txt", Mode: 0o644, Data: []byte("second")}}); err != nil {
		t.Fatalf("second writeTree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "stale.txt")); !os.IsNotExist(err) {
		t.Fatal("stale.txt from the first writeTree survived the second")
	}
	got, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	if err != nil || string(got) != "second" {
		t.Fatalf("a.txt = %q, %v; want \"second\", nil", got, err)
	}
}

// TestRetainBundlePreservesExecutableMode guards against retainBundle
// flattening every retained file to 0o600: a bin/* entry built with the
// executable bit set (as companionrelease.Run's sumEntries carries it) must
// still be runnable after retention, not just after the separate
// smoke-extract staging chmod.
func TestRetainBundlePreservesExecutableMode(t *testing.T) {
	scratch := t.TempDir()
	outputParent := t.TempDir()
	files := []ArchiveEntry{{Path: "bin/corvint", Mode: 0o755, Data: []byte("binary")}}

	path, err := retainBundle(scratch, outputParent, "bundle", files)
	if err != nil {
		t.Fatalf("retain: %v", err)
	}
	info, err := os.Stat(filepath.Join(path, "bin/corvint"))
	if err != nil {
		t.Fatalf("retained binary missing: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("retained bin/corvint is not executable: mode=%v", info.Mode())
	}
}

// --- validateOutputParent ---

func TestValidateOutputParentRefusesNestedLocation(t *testing.T) {
	corvintRoot := t.TempDir()
	taskmanRoot := t.TempDir()
	nested := filepath.Join(corvintRoot, "out")
	if err := validateOutputParent(context.Background(), nested, corvintRoot, taskmanRoot); err == nil {
		t.Fatal("output parent nested in corvint root: want refusal, got nil")
	}
	if err := validateOutputParent(context.Background(), corvintRoot, corvintRoot, taskmanRoot); err == nil {
		t.Fatal("output parent equal to corvint root: want refusal, got nil")
	}
	if err := validateOutputParent(context.Background(), filepath.Join(taskmanRoot, "out"), corvintRoot, taskmanRoot); err == nil {
		t.Fatal("output parent nested in taskman root: want refusal, got nil")
	}
	outside := t.TempDir()
	if err := validateOutputParent(context.Background(), outside, corvintRoot, taskmanRoot); err != nil {
		t.Fatalf("output parent outside both roots refused: %v", err)
	}
}

// PUB-V0-011: nesting is judged on resolved paths, so neither a symlinked
// output parent (existing or not yet created) nor a symlinked root spelling
// lets a bundle be retained inside a checkout root.
func TestValidateOutputParentRefusesSymlinkedNestedLocation(t *testing.T) {
	base := t.TempDir()
	corvintRoot := filepath.Join(base, "corvint")
	taskmanRoot := filepath.Join(base, "taskman")
	for _, dir := range []string{corvintRoot, filepath.Join(taskmanRoot, "out")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for link, dest := range map[string]string{"corvint-alias": corvintRoot, "taskman-alias": taskmanRoot} {
		if err := os.Symlink(dest, filepath.Join(base, link)); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct{ name, out, corvint, taskman string }{
		{"not-yet-created parent through a symlink into the corvint root", filepath.Join(base, "corvint-alias", "out", "deeper"), corvintRoot, taskmanRoot},
		{"existing parent through a symlink into the taskman root", filepath.Join(base, "taskman-alias", "out"), corvintRoot, taskmanRoot},
		{"real parent under a root spelled through a symlink", filepath.Join(corvintRoot, "out"), filepath.Join(base, "corvint-alias"), taskmanRoot},
	}
	for _, tc := range cases {
		if err := validateOutputParent(context.Background(), tc.out, tc.corvint, tc.taskman); err == nil {
			t.Errorf("%s: want refusal, got nil", tc.name)
		}
	}
}

// PUB-V0-011: the bundle name is one path element, so it cannot carry the
// retained directory out of the validated output parent into a root.
func TestValidateBundleNameRefusesPathTraversal(t *testing.T) {
	for _, name := range []string{"../corvint/out", "nested/out", "..", ".", ""} {
		if err := validateBundleName(name); err == nil {
			t.Errorf("bundle name %q: want refusal, got nil", name)
		}
	}
	if err := validateBundleName("corvint-companion-darwin-arm64"); err != nil {
		t.Errorf("single-element bundle name refused: %v", err)
	}
}

// PUB-V0-011: Run refuses a scratch directory inside either checkout root,
// by real spelling or through a symlink, before it writes anything there.
func TestRunRefusesScratchInsideCheckoutRoot(t *testing.T) {
	base := t.TempDir()
	corvintRoot := filepath.Join(base, "corvint")
	taskmanRoot := filepath.Join(base, "taskman")
	if err := os.MkdirAll(filepath.Join(taskmanRoot, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(corvintRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(taskmanRoot, "sub"), filepath.Join(base, "taskman-alias")); err != nil {
		t.Fatal(err)
	}
	for name, scratch := range map[string]string{
		"not-yet-created scratch in the corvint root":              filepath.Join(corvintRoot, "scratch", "deeper"),
		"existing scratch through a symlink into the taskman root": filepath.Join(base, "taskman-alias"),
	} {
		report, err := Run(context.Background(), Options{CorvintRoot: corvintRoot, TaskmanRoot: taskmanRoot, Target: supportedTarget, Scratch: scratch, OutputParent: t.TempDir(), BundleName: "b"})
		if err == nil || report != nil || !strings.Contains(err.Error(), "scratch") {
			t.Errorf("%s: want scratch refusal, got report=%v err=%v", name, report, err)
		}
		for _, dir := range []string{corvintRoot, filepath.Join(taskmanRoot, "sub")} {
			if names, _ := os.ReadDir(dir); len(names) != 0 {
				t.Errorf("%s: refused run wrote under %s: %v", name, dir, names)
			}
		}
	}
	if err := validateScratch(context.Background(), filepath.Join(base, "scratch", "new"), corvintRoot, taskmanRoot); err != nil {
		t.Errorf("not-yet-created scratch outside both roots refused: %v", err)
	}
}

// PUB-V0-011: Run refuses a checkout root inside the scratch directory before
// any write, so the staged build tree it later clears cannot be that root.
func TestRunRefusesCheckoutRootInsideScratch(t *testing.T) {
	scratch := t.TempDir()
	corvintRoot := filepath.Join(scratch, "corvint-build-src")
	if err := os.Mkdir(corvintRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(corvintRoot, "go.mod"), []byte("module corvint\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := Run(context.Background(), Options{CorvintRoot: corvintRoot, TaskmanRoot: t.TempDir(), Target: supportedTarget, Scratch: scratch, OutputParent: t.TempDir(), BundleName: "b"})
	if err == nil || report != nil || !strings.Contains(err.Error(), "scratch") {
		t.Fatalf("want scratch refusal, got report=%v err=%v", report, err)
	}
	if names, _ := os.ReadDir(scratch); len(names) != 1 {
		t.Errorf("refused run wrote under scratch: %v", names)
	}
	if data, err := os.ReadFile(filepath.Join(corvintRoot, "go.mod")); err != nil || string(data) != "module corvint\n" {
		t.Errorf("checkout root contents did not survive: data=%q err=%v", data, err)
	}
}

// PUB-V0-011: an output parent spelled through a case alias of a checkout
// root is refused by directory identity on a case-insensitive volume.
func TestRunRefusesCaseAliasOutputParent(t *testing.T) {
	base := t.TempDir()
	corvintRoot := filepath.Join(base, "corvint")
	if err := os.Mkdir(corvintRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(base, "CORVINT")); err != nil {
		t.Skipf("temp volume is case-sensitive: %v", err)
	}
	report, err := Run(context.Background(), Options{CorvintRoot: corvintRoot, TaskmanRoot: t.TempDir(), Target: supportedTarget, Scratch: t.TempDir(), OutputParent: filepath.Join(base, "CORVINT", "out"), BundleName: "b"})
	if err == nil || report != nil || !strings.Contains(err.Error(), "output parent") {
		t.Fatalf("want output parent refusal, got report=%v err=%v", report, err)
	}
	if names, _ := os.ReadDir(corvintRoot); len(names) != 0 {
		t.Errorf("refused run wrote under the corvint root: %v", names)
	}
}

// --- subprocess cleanup on context cancellation ---
// Adapts internal/console/lifecycle_test.go's pidfile-child fixture: a
// script backgrounds a child, records its pid, then waits. Canceling the
// context must leave no live descendant, proving runCaptured's sole
// procgroup.Run call site owns and reaps the process group it starts.

func companionOwnedFixture(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "child.pid")
	script := filepath.Join(dir, "tool")
	body := fmt.Sprintf("#!/bin/sh\nsleep 60 &\nchild=$!\ntrap 'kill \"$child\" 2>/dev/null; wait \"$child\" 2>/dev/null' EXIT\ntrap 'exit 0' INT TERM\necho $child > '%s'\nwait\n", pidfile)
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if raw, err := os.ReadFile(pidfile); err == nil {
			if pid, _ := strconv.Atoi(strings.TrimSpace(string(raw))); pid > 0 {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	})
	return script, pidfile
}

func companionWaitChild(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if raw, err := os.ReadFile(path); err == nil {
			if pid, _ := strconv.Atoi(strings.TrimSpace(string(raw))); pid > 0 {
				return pid
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("child not started")
	return 0
}

func companionAssertChildGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		cmd := exec.CommandContext(ctx, "/bin/ps", "-p", strconv.Itoa(pid), "-o", "stat=")
		cmd.WaitDelay = time.Second
		out, err := cmd.Output()
		cancel()
		if err != nil && errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return
		}
		if strings.HasPrefix(strings.TrimSpace(string(out)), "Z") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("owned descendant %d survived cancellation", pid)
}

func TestRunCapturedCleansUpDescendantsOnCancel(t *testing.T) {
	binary, pidPath := companionOwnedFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, _, err := runCaptured(ctx, filepath.Dir(binary), nil, 5*time.Second, binary)
		done <- err
	}()

	pid := companionWaitChild(t, pidPath)
	if err := syscall.Kill(pid, 0); err != nil {
		t.Fatal("descendant was not live before cancellation")
	}
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("runCaptured did not return after cancellation")
	}
	companionAssertChildGone(t, pid)
}

// --- installed-path mutation refusal (PUB-V0-005) ---

func TestCheckMutationRefusalsRequiresRefusalWithoutStoreEffect(t *testing.T) {
	board := `<input type="hidden" name="token" value="t0k">` + "\n" + `<input type="hidden" name="verb" value="create">`
	for name, tc := range map[string]struct {
		accept bool
		wantOK bool
	}{"refusing console": {false, true}, "accepting console": {true, false}} {
		t.Run(name, func(t *testing.T) {
			store := t.TempDir()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.ParseForm()
				switch {
				case tc.accept:
					_ = os.WriteFile(filepath.Join(store, "ticket.json"), []byte(r.PostForm.Get("title")), 0o600)
				case r.Header.Get("Origin") != "http://"+r.Host:
					http.Error(w, "unexpected Origin", http.StatusForbidden)
				default:
					http.Error(w, "invalid form token; reload this page", http.StatusForbidden)
				}
			}))
			defer server.Close()
			for _, s := range checkMutationRefusals(server.URL, board, store) {
				if s.Name != "console-create-form" && s.OK != tc.wantOK {
					t.Errorf("step %s OK=%v, want %v (%s)", s.Name, s.OK, tc.wantOK, s.Detail)
				}
			}
		})
	}
}

// --- installed-path console pages (PUB-V0-004) ---

func TestCheckConsolePagesRequiresReasonedControlsAppliedEditAndEvidence(t *testing.T) {
	controls := ""
	for _, verb := range consoleControlVerbs {
		controls += `<input type="hidden" name="verb" value="` + verb + `">`
	}
	editForm := `<form class="ticket-form" method="post" action="/mutate"><input type="hidden" name="verb" value="refine"></form>`
	for name, working := range map[string]bool{"working console": true, "broken console": false} {
		t.Run(name, func(t *testing.T) {
			title := "before"
			disabled := `<button type="submit" disabled>Archive</button>`
			if working {
				disabled += `<div class="disabled-reason">disabled: not implemented</div>`
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/mutate":
					_ = r.ParseForm()
					if working && r.PostForm.Get("verb") == "refine" {
						title = r.PostForm.Get("title")
						_, _ = io.WriteString(w, "The owning tool applied this change.")
					}
				case "/ticket":
					_, _ = io.WriteString(w, title)
				case "/evidence":
					if working {
						_, _ = io.WriteString(w, "<h2>Observation boundary</h2>")
					}
				}
			}))
			defer server.Close()
			steps := checkConsolePages(server.URL, "T-1", controls+disabled+editForm)
			if len(steps) != 3 {
				t.Fatalf("got %d steps, want 3", len(steps))
			}
			for _, s := range steps {
				if s.OK != working {
					t.Errorf("step %s OK=%v, want %v (%s)", s.Name, s.OK, working, s.Detail)
				}
			}
		})
	}
}

func TestCheckConsoleLinksRequiresPinnedResolvingLinksAndExplicitGaps(t *testing.T) {
	const commit = "c0ffee"
	for name, working := range map[string]bool{"working console": true, "broken console": false} {
		t.Run(name, func(t *testing.T) {
			pin := "&amp;at=" + commit
			if !working {
				pin = ""
			}
			gap := `<div class="unknown">gap: no Traceability row</div>`
			if !working {
				gap = `<a href="/code?path=src%2finvented.go&amp;at=c0ffee">invented</a>`
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				id, path := r.URL.Query().Get("id"), r.URL.Query().Get("path")
				switch {
				case r.URL.Path == "/requirement" && id == "SMK-V0-001":
					_, _ = io.WriteString(w, `<th>clause</th><a href="/code?path=src%2flinked.go`+pin+`">blob</a>`)
				case r.URL.Path == "/requirement":
					_, _ = io.WriteString(w, gap)
				case r.URL.Path == "/code" && path == "src/linked.go" && working:
					_, _ = io.WriteString(w, `<h2>Requirements citing this path</h2><a href="/requirement?id=SMK-V0-001`+pin+`">SMK-V0-001</a>`)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			steps := checkConsoleLinks(server.URL, commit)
			if len(steps) != 3 {
				t.Fatalf("got %d steps, want 3", len(steps))
			}
			for _, s := range steps {
				if s.OK != working {
					t.Errorf("step %s OK=%v, want %v (%s)", s.Name, s.OK, working, s.Detail)
				}
			}
		})
	}
}

// --- export entry-count bound ---

// --- spec-coverage audit 2026-09-12: clauses whose prior evidence could not fail ---

// TestRequireCleanTreeRefusesDirtyCheckout drives PUB-V0-011's clean-tree
// refusal against a real Git checkout: a modified tracked file and an
// untracked file each make requireCleanTree refuse, and the committed state
// is accepted.
func TestRequireCleanTreeRefusesDirtyCheckout(t *testing.T) {
	t.Run("PUB-V0-011 clean-tree-refusal", func(t *testing.T) {
		gitPath, err := exec.LookPath("git")
		if err != nil {
			t.Skip("git not on PATH")
		}
		root, scratch := t.TempDir(), t.TempDir()
		git := func(args ...string) {
			t.Helper()
			cmd := exec.Command(gitPath, args...)
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@x", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@x")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
		}
		tracked := filepath.Join(root, "a.txt")
		if err := os.WriteFile(tracked, []byte("committed\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		git("init", "-q")
		git("add", "a.txt")
		git("commit", "-qm", "fixture")
		if err := requireCleanTree(context.Background(), gitPath, root, scratch); err != nil {
			t.Fatalf("clean checkout refused: %v", err)
		}

		if err := os.WriteFile(tracked, []byte("edited\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := requireCleanTree(context.Background(), gitPath, root, scratch); err == nil {
			t.Fatal("modified tracked file: want refusal, got nil")
		}
		git("checkout", "--", "a.txt")

		if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := requireCleanTree(context.Background(), gitPath, root, scratch); err == nil {
			t.Fatal("untracked file: want refusal, got nil")
		}
	})
}

// TestBundleReportsNameNotRunTargetsAndBrowserScope pins PUB-V0-011's four
// NOT_RUN platforms and PUB-V0-015's browser/UI out-of-scope statement in the
// bundle's own emitted records (README.md and MANIFEST.json).
func TestBundleReportsNameNotRunTargetsAndBrowserScope(t *testing.T) {
	want := []string{"darwin/amd64", "linux/amd64", "linux/arm64", "windows/amd64"}
	manifest := BundleManifest{Target: supportedTarget, NotRun: NotRunTargets}
	readme := string(renderBundleReadme(manifest))
	t.Run("PUB-V0-011 target-not-run-record", func(t *testing.T) {
		if strings.Join(NotRunTargets, ",") != strings.Join(want, ",") {
			t.Fatalf("NotRunTargets = %v, want %v", NotRunTargets, want)
		}
		for _, target := range want {
			if !strings.Contains(readme, target) {
				t.Fatalf("README does not name NOT_RUN target %s:\n%s", target, readme)
			}
		}
		manifestJSON, err := renderManifestJSON(manifest)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(manifestJSON), `"notRunTargets": [
    "darwin/amd64",
    "linux/amd64",
    "linux/arm64",
    "windows/amd64"
  ]`) {
			t.Fatalf("MANIFEST.json does not carry the four NOT_RUN targets:\n%s", manifestJSON)
		}
	})
	t.Run("WQO-V0-047 adoption-record", func(t *testing.T) {
		for _, want := range []string{"corvint-tasks --version", "github.com/Beamfall/corvint-tasks", "corvint work init --repository NAME --corvint-executable /absolute/path/to/corvint", "work rebind", ".corvint/work-queue-policy.json", "adapter\nreceipt", "ERROR/SOURCE_UNQUALIFIED", "ERROR/ADAPTER_FAILED", "STALE"} {
			if !strings.Contains(readme, want) {
				t.Fatalf("README does not record %q:\n%s", want, readme)
			}
		}
	})
	t.Run("PUB-V0-015 browser-scope-record", func(t *testing.T) {
		if !strings.Contains(readme, "Browser/UI qualification is a separate, explicitly out-of-scope step") {
			t.Fatalf("README does not name browser/UI qualification as out of scope:\n%s", readme)
		}
	})
}

// TestExportSourceRefusesDisagreeingPasses drives PUB-V0-012's two-pass
// agreement: the primary cat-file pass returns the blob Git names, the
// independent pass returns different bytes of the same length, and the
// export must refuse instead of trusting either pass.
func TestExportSourceRefusesDisagreeingPasses(t *testing.T) {
	t.Run("PUB-V0-012 independent-source-export-agreement", func(t *testing.T) {
		home, root := t.TempDir(), t.TempDir()
		script := `#!/bin/sh
case "$1" in
  rev-parse) echo "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef" ;;
  ls-tree) cat "$HOME/lstree.out" ;;
  cat-file)
    if [ -e "$HOME/second" ]; then cat "$HOME/catfile2.out"; else : > "$HOME/second"; cat "$HOME/catfile.out"; fi ;;
  *) exit 1 ;;
esac
`
		gitPath := filepath.Join(home, "fake-git.sh")
		if err := os.WriteFile(gitPath, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		const oid = "95d09f2b10159347eece71399a7e2e907ea3df4f" // blob "hello world"
		fixtures := map[string]string{
			"lstree.out":   "100644 blob " + oid + "\tf0\x00",
			"catfile.out":  oid + " blob 11\nhello world\n",
			"catfile2.out": oid + " blob 11\nhello_world\n",
		}
		for name, body := range fixtures {
			if err := os.WriteFile(filepath.Join(home, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		_, err := exportSource(context.Background(), gitPath, root, home)
		if err == nil || !strings.Contains(err.Error(), "export passes disagree on f0") {
			t.Fatalf("disagreeing export passes: want refusal, got %v", err)
		}
	})
}

// TestVerifyBuildInfoRequiresPinnedClosedBuild drives PUB-V0-013's embedded
// buildinfo check with real binaries: a closed build under the pinned
// toolchain is accepted, while a different requested target, a build without
// -trimpath, and a cgo-enabled build record are each refused.
func TestVerifyBuildInfoRequiresPinnedClosedBuild(t *testing.T) {
	t.Run("PUB-V0-013 pinned-buildinfo", func(t *testing.T) {
		if runtime.Version() != requiredGoVersion {
			t.Skipf("pinned toolchain %s required, running %s", requiredGoVersion, runtime.Version())
		}
		goPath, err := exec.LookPath("go")
		if err != nil {
			t.Skip("go not on PATH")
		}
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module buildinfofixture\n\ngo 1.27\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		build := func(name string, cgo string, args ...string) string {
			t.Helper()
			out := filepath.Join(dir, name)
			cmd := exec.Command(goPath, append(append([]string{"build"}, args...), "-o", out, ".")...)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=", "CGO_ENABLED="+cgo)
			if raw, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("go build %s: %v\n%s", name, err, raw)
			}
			return out
		}
		closed := build("closed", "0", "-trimpath", "-buildvcs=false")
		if err := verifyBuildInfo(closed, runtime.GOOS, runtime.GOARCH); err != nil {
			t.Fatalf("closed pinned build refused: %v", err)
		}
		otherArch := "arm64"
		if runtime.GOARCH == "arm64" {
			otherArch = "amd64"
		}
		if err := verifyBuildInfo(closed, runtime.GOOS, otherArch); err == nil {
			t.Fatal("buildinfo target differing from the requested target: want refusal, got nil")
		}
		if err := verifyBuildInfo(build("untrimmed", "0", "-buildvcs=false"), runtime.GOOS, runtime.GOARCH); err == nil {
			t.Fatal("build without -trimpath: want refusal, got nil")
		}
		if err := verifyBuildInfo(build("cgo", "1", "-trimpath", "-buildvcs=false"), runtime.GOOS, runtime.GOARCH); err == nil {
			t.Fatal("cgo-enabled build: want refusal, got nil")
		}
		raw, err := os.ReadFile(closed)
		if err != nil {
			t.Fatal(err)
		}
		otherVersion := filepath.Join(dir, "other-version")
		if err := os.WriteFile(otherVersion, bytes.ReplaceAll(raw, []byte(requiredGoVersion), []byte("go1.27.9")), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := verifyBuildInfo(otherVersion, runtime.GOOS, runtime.GOARCH); err == nil || !strings.Contains(err.Error(), "Go version") {
			t.Fatalf("buildinfo naming another Go version: want Go version refusal, got %v", err)
		}
	})
}

// --- build source, bundle archive and failure ordering ---

// TestBuildUsesStagedExportNotCheckout drives PUB-V0-013's build input: a
// gitignored .go file in the checkout that breaks the package (a second main)
// fails a build rooted at the checkout, while the staged, digest-verified
// export excludes it and builds twice identically. A file added to the staged
// tree after staging is refused by the re-verification.
func TestBuildUsesStagedExportNotCheckout(t *testing.T) {
	t.Run("PUB-V0-013 staged-export-input", func(t *testing.T) {
		if runtime.Version() != requiredGoVersion {
			t.Skipf("pinned toolchain %s required, running %s", requiredGoVersion, runtime.Version())
		}
		gitPath, err := exec.LookPath("git")
		if err != nil {
			t.Skip("git not on PATH")
		}
		goPath, err := exec.LookPath("go")
		if err != nil {
			t.Skip("go not on PATH")
		}
		root, scratch := t.TempDir(), t.TempDir()
		files := map[string]string{
			"go.mod":           "module stagedfixture\n\ngo 1.27\n",
			".gitignore":       "cmd/x/ignored.go\n",
			"cmd/x/main.go":    "package main\n\nfunc main() {}\n",
			"cmd/x/ignored.go": "package main\n\nfunc main() {}\n",
		}
		for rel, body := range files {
			full := filepath.Join(root, rel)
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"commit", "-qm", "fixture"}} {
			cmd := exec.Command(gitPath, args...)
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@x", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@x")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
		}
		ctx := context.Background()
		if err := runGoBuild(ctx, goPath, root, "./cmd/x", filepath.Join(scratch, "checkout-x"), closedGoEnv(filepath.Join(scratch, "home-checkout"), supportedTarget)); err == nil {
			t.Fatal("fixture invalid: the gitignored file did not break a checkout-rooted build")
		}

		export, err := exportSource(ctx, gitPath, root, scratch)
		if err != nil {
			t.Fatalf("exportSource: %v", err)
		}
		staged, err := stageBuildSource(export, filepath.Join(scratch, "build-src"))
		if err != nil {
			t.Fatalf("stageBuildSource: %v", err)
		}
		if _, err := os.Stat(filepath.Join(staged, "cmd/x/ignored.go")); !os.IsNotExist(err) {
			t.Fatalf("gitignored checkout file reached the staged build tree: %v", err)
		}
		if _, err := buildComponentTwice(ctx, staged, "./cmd/x", "x", supportedTarget, scratch); err != nil {
			t.Fatalf("build from staged export: %v", err)
		}

		if err := os.WriteFile(filepath.Join(staged, "cmd/x/late.go"), []byte("package main\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := verifyStagedTree(export, staged); err == nil {
			t.Fatal("file added to the staged build tree: want refusal, got nil")
		}
	})
}

// TestBuildComponentTwiceRefusesDivergentOrUnverifiedBuilds drives PUB-V0-013's
// acceptance rule with a stub `go` on PATH: two builds whose bytes differ are
// refused even though each succeeded, the two builds run with separate
// GOCACHE directories, and byte-identical output is still refused when the
// retained binary carries no valid buildinfo.
func TestBuildComponentTwiceRefusesDivergentOrUnverifiedBuilds(t *testing.T) {
	t.Run("PUB-V0-013 independent-build-agreement", func(t *testing.T) {
		stubDir, log := t.TempDir(), filepath.Join(t.TempDir(), "gocache.log")
		t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))
		writeStub := func(payload string) {
			script := "#!/bin/sh\nwhile [ $# -gt 0 ]; do [ \"$1\" = -o ] && out=$2; shift; done\n" +
				"printf '%s\\n' \"$GOCACHE\" >> '" + log + "'\nprintf '%s' " + payload + " > \"$out\"\n"
			if err := os.WriteFile(filepath.Join(stubDir, "go"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		ctx := context.Background()

		writeStub(`"$out"`)
		_, err := buildComponentTwice(ctx, t.TempDir(), "./cmd/x", "x", supportedTarget, t.TempDir())
		if err == nil || !strings.Contains(err.Error(), "different bytes") {
			t.Fatalf("divergent builds: want byte-identity refusal, got %v", err)
		}
		caches, readErr := os.ReadFile(log)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if lines := strings.Fields(string(caches)); len(lines) != 2 || lines[0] == lines[1] {
			t.Fatalf("the two builds must use separate GOCACHE directories, got %q", lines)
		}

		writeStub("identical")
		if _, err := buildComponentTwice(ctx, t.TempDir(), "./cmd/x", "x", supportedTarget, t.TempDir()); err == nil || !strings.Contains(err.Error(), "buildinfo") {
			t.Fatalf("identical bytes without buildinfo: want buildinfo refusal, got %v", err)
		}
	})
}

// PUB-V0-011: a directory reused directly under a validated scratch that is a
// planted symlink is refused before any build writes through it.
func TestBuildComponentTwiceRefusesPlantedScratchLink(t *testing.T) {
	stubDir, outside, scratch := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	script := "#!/bin/sh\nwhile [ $# -gt 0 ]; do [ \"$1\" = -o ] && out=$2; shift; done\nprintf same > \"$out\"\n"
	if err := os.WriteFile(filepath.Join(stubDir, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(scratch, "build-a-x")); err != nil {
		t.Fatal(err)
	}
	_, err := buildComponentTwice(context.Background(), t.TempDir(), "./cmd/x", "x", supportedTarget, scratch)
	if err == nil || !strings.Contains(err.Error(), "not a real directory") {
		t.Fatalf("planted scratch link: want refusal, got %v", err)
	}
	if names, _ := os.ReadDir(outside); len(names) != 0 {
		t.Fatalf("build wrote through the planted link into %s: %v", outside, names)
	}
}

// TestCorvintBundledBinariesBuildTwiceIdentically drives PUB-V0-013's
// byte-identity with the real toolchain: the three bundled binaries from this
// module each build twice with cold, separate caches and match byte-for-byte.
// `atm` is built from the separate corvint-taskman repository, which this
// module's tests cannot reach, so its double build stays gate-measured.
func TestCorvintBundledBinariesBuildTwiceIdentically(t *testing.T) {
	t.Run("PUB-V0-013 corvint-binary-double-build", func(t *testing.T) {
		if runtime.Version() != requiredGoVersion {
			t.Skipf("pinned toolchain %s required, running %s", requiredGoVersion, runtime.Version())
		}
		if _, err := exec.LookPath("go"); err != nil {
			t.Skip("go not on PATH")
		}
		root, err := filepath.Abs(filepath.Join("..", ".."))
		if err != nil {
			t.Fatal(err)
		}
		scratch := t.TempDir()
		for _, name := range []string{"corvint", "corvint-console", "corvint-dashboard-snapshot"} {
			built, err := buildComponentTwice(context.Background(), root, "./cmd/"+name, name, supportedTarget, scratch)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if built.Size == 0 {
				t.Fatalf("%s: empty binary accepted", name)
			}
		}
	})
}

// TestBundleArchiveCarriesManifestAndSumsForEveryMember drives PUB-V0-014's
// bundle contents: the component manifest records the export's commit and tree
// and the digests of the shipped binary and source archive, SHA256SUMS lists
// every other member with its digest, and the assembled bundle archive builds
// twice identically (no wall-clock README line) and decode-verifies.
func TestBundleArchiveCarriesManifestAndSumsForEveryMember(t *testing.T) {
	t.Run("PUB-V0-014 deterministic-archive-manifest-sums", func(t *testing.T) {
		export := Export{
			HeadCommit: strings.Repeat("a", 40),
			HeadTree:   strings.Repeat("b", 40),
			Files: []SourceFile{
				{Path: "LICENSE", Mode: "100644", Data: []byte("license")},
				{Path: "PROVENANCE.md", Mode: "100644", Data: []byte("provenance")},
				{Path: "main.go", Mode: "100644", Data: []byte("package main\n")},
			},
		}
		bin := []byte("binary bytes")
		built := BuiltBinary{Name: "atm", SHA256: sha256Hex(bin), Size: int64(len(bin))}
		files, component, err := assembleComponent("atm", "corvint-taskman", export, built, bin)
		if err != nil {
			t.Fatalf("assembleComponent: %v", err)
		}
		byPath := map[string][]byte{}
		for _, f := range files {
			byPath[f.Path] = f.Data
		}
		if component.Commit != export.HeadCommit || component.Tree != export.HeadTree || component.Module != "corvint-taskman" {
			t.Fatalf("component identity = %+v, want the export's commit and tree", component)
		}
		if component.BinarySHA256 != sha256Hex(byPath[component.BinaryPath]) || component.BinarySizeBytes != int64(len(bin)) {
			t.Fatalf("component binary digest does not match shipped %s", component.BinaryPath)
		}
		if component.SourceArchiveSHA256 != sha256Hex(byPath[component.SourceArchivePath]) {
			t.Fatalf("component source archive digest does not match shipped %s", component.SourceArchivePath)
		}
		for _, notice := range []string{"notices/atm/LICENSE", "notices/atm/PROVENANCE.md", "notices/atm/THIRD-PARTY-NOTICES.txt"} {
			if _, ok := byPath[notice]; !ok {
				t.Fatalf("component files lack %s", notice)
			}
		}

		manifest := BundleManifest{Target: supportedTarget, NotRun: NotRunTargets, Components: []ComponentManifest{component}}
		assemble := func() ([]ArchiveEntry, error) { return assembleBundleEntries(files, manifest) }
		archive, err := buildTarGzTwice(assemble)
		if err != nil {
			t.Fatalf("bundle archive not byte-stable: %v", err)
		}
		entries, err := assemble()
		if err != nil {
			t.Fatal(err)
		}
		if err := verifyTarGz(archive, entries); err != nil {
			t.Fatalf("bundle archive decode verification: %v", err)
		}

		shipped := byPathOf(entries)
		sums := map[string]string{}
		for _, line := range strings.Split(strings.TrimSuffix(string(shipped["SHA256SUMS"]), "\n"), "\n") {
			digest, path, ok := strings.Cut(line, "  ")
			if !ok {
				t.Fatalf("malformed SHA256SUMS line %q", line)
			}
			sums[path] = digest
		}
		for _, e := range entries {
			if e.Path == "SHA256SUMS" {
				continue
			}
			if sums[e.Path] != sha256Hex(e.Data) {
				t.Fatalf("SHA256SUMS digest for %s = %q, want %s", e.Path, sums[e.Path], sha256Hex(e.Data))
			}
			delete(sums, e.Path)
		}
		if len(sums) != 0 {
			t.Fatalf("SHA256SUMS missing, or lists paths the bundle does not ship: %v", sums)
		}
		if !strings.Contains(string(shipped["MANIFEST.json"]), `"sourceArchiveSha256": "`+component.SourceArchiveSHA256+`"`) {
			t.Fatal("MANIFEST.json does not carry the component source archive digest")
		}
	})
}

func byPathOf(entries []ArchiveEntry) map[string][]byte {
	out := make(map[string][]byte, len(entries))
	for _, e := range entries {
		out[e.Path] = e.Data
	}
	return out
}

// TestRunFailuresRetainNoOutputAndNoQualifiedReport drives PUB-V0-003's and
// PUB-V0-015's failure ordering: a refused gate in Run, a smoke error and a
// failed smoke step each return no report and leave the output parent without
// the bundle, while a passing smoke retains the archive and its checksum.
func TestRunFailuresRetainNoOutputAndNoQualifiedReport(t *testing.T) {
	requireNoOutput := func(t *testing.T, report *Report, err error, outputParent string) {
		t.Helper()
		if err == nil || report != nil {
			t.Fatalf("want refusal with no report, got report=%+v err=%v", report, err)
		}
		if names, _ := os.ReadDir(outputParent); len(names) != 0 {
			t.Fatalf("failed run left output under %s: %v", outputParent, names)
		}
	}
	t.Run("PUB-V0-003 fail-before-retain", func(t *testing.T) {
		gateOutput := t.TempDir()
		report, err := Run(context.Background(), Options{Target: "linux/amd64", Scratch: t.TempDir(), OutputParent: gateOutput, BundleName: "b"})
		requireNoOutput(t, report, err, gateOutput)
	})
	t.Run("PUB-V0-015 smoke-before-retain", func(t *testing.T) {
		entries := []ArchiveEntry{{Path: "bin/atm", Mode: 0o755, Data: []byte("bin")}, {Path: "SHA256SUMS", Mode: 0o644, Data: []byte("sums\n")}}
		archive, err := buildTarGz(entries)
		if err != nil {
			t.Fatal(err)
		}
		smokes := map[string]func(context.Context, string) ([]SmokeStep, error){
			"smoke error": func(context.Context, string) ([]SmokeStep, error) { return nil, errors.New("boom") },
			"failed step": func(context.Context, string) ([]SmokeStep, error) {
				return []SmokeStep{{Name: "atm version", OK: true}, {Name: "console board", OK: false, Detail: "500"}}, nil
			},
		}
		for name, smoke := range smokes {
			opts := Options{Scratch: t.TempDir(), OutputParent: t.TempDir(), BundleName: "b"}
			report, err := qualifyAndRetain(context.Background(), opts, Report{}, entries, archive, smoke)
			if err == nil {
				t.Fatalf("%s: want refusal", name)
			}
			requireNoOutput(t, report, err, opts.OutputParent)
		}

		opts := Options{Scratch: t.TempDir(), OutputParent: t.TempDir(), BundleName: "b"}
		pass := func(_ context.Context, extractDir string) ([]SmokeStep, error) {
			info, err := os.Stat(filepath.Join(extractDir, "bin/atm"))
			return []SmokeStep{{Name: "staged", OK: err == nil && info.Mode()&0o111 != 0}}, err
		}
		report, err := qualifyAndRetain(context.Background(), opts, Report{}, entries, archive, pass)
		if err != nil {
			t.Fatalf("passing smoke refused: %v", err)
		}
		retained, err := os.ReadFile(report.ArchivePath)
		if err != nil || !bytes.Equal(retained, archive) || report.ArchiveSHA256 != sha256Hex(archive) {
			t.Fatalf("retained archive does not match the verified archive: %v", err)
		}
		checksum, err := os.ReadFile(report.ArchivePath + ".sha256")
		if err != nil || string(checksum) != sha256Hex(archive)+"  b.tar.gz\n" {
			t.Fatalf("retained archive checksum = %q, %v", checksum, err)
		}
	})
}

// TestRetainBundleClearsStaleStaging ensures a staging directory left behind
// by an earlier failed retain cannot add its files to a new retained bundle.
func TestRetainBundleClearsStaleStaging(t *testing.T) {
	scratch, outputParent := t.TempDir(), t.TempDir()
	stale := filepath.Join(scratch, "retain-staging-bundle", "stale.txt")
	if err := os.MkdirAll(filepath.Dir(stale), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	path, err := retainBundle(scratch, outputParent, "bundle", []ArchiveEntry{{Path: "a.txt", Mode: 0o644, Data: []byte("x")}})
	if err != nil {
		t.Fatalf("retain: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(path, "stale.txt")); !os.IsNotExist(err) {
		t.Fatalf("stale staging file reached the retained bundle: %v", err)
	}
}

// TestVerifyTarGzRefusesMissingChangedAndNonRegularMembers covers the closed
// inventory decode check from the other side of the extra-member case: an
// expected member absent from the archive, a member whose same-size content
// differs, and a non-regular member that otherwise matches its expectation.
func TestVerifyTarGzRefusesMissingChangedAndNonRegularMembers(t *testing.T) {
	entries := []ArchiveEntry{{Path: "a.txt", Mode: 0o644, Data: []byte("alpha")}}
	archive, err := buildTarGz(entries)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	withMissing := append(append([]ArchiveEntry(nil), entries...), ArchiveEntry{Path: "b.txt", Mode: 0o644, Data: []byte("beta")})
	if err := verifyTarGz(archive, withMissing); err == nil {
		t.Fatal("archive missing an expected member: want refusal, got nil")
	}
	if err := verifyTarGz(archive, []ArchiveEntry{{Path: "a.txt", Mode: 0o644, Data: []byte("alphA")}}); err == nil {
		t.Fatal("same-size content mismatch: want refusal, got nil")
	}

	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	if err := tw.WriteHeader(&tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd", Mode: 0o644}); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	var gzBuf bytes.Buffer
	gw := gzip.NewWriter(&gzBuf)
	if _, err := gw.Write(tarBuf.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := verifyTarGz(gzBuf.Bytes(), []ArchiveEntry{{Path: "link", Mode: 0o644}}); err == nil {
		t.Fatal("symlink archive member: want refusal, got nil")
	}
}

// TestExportSourceRefusesBlobDigestMismatch drives PUB-V0-012's blob digest
// check inside exportSource: both passes agree on bytes that do not hash to
// the object id ls-tree declared, so pass agreement alone must not admit them.
func TestExportSourceRefusesBlobDigestMismatch(t *testing.T) {
	home, root := t.TempDir(), t.TempDir()
	gitPath := writeFakeGit(t, home)
	const oid = "95d09f2b10159347eece71399a7e2e907ea3df4f" // blob "hello world"
	fixtures := map[string]string{
		"lstree.out":  "100644 blob " + oid + "\tf0\x00",
		"catfile.out": oid + " blob 11\nhello_world\n",
	}
	for name, body := range fixtures {
		if err := os.WriteFile(filepath.Join(home, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, err := exportSource(context.Background(), gitPath, root, home)
	if err == nil || !strings.Contains(err.Error(), "blob digest mismatch") {
		t.Fatalf("agreeing passes with a wrong digest: want refusal, got %v", err)
	}
}

// TestSourceArchiveEntriesMapsGitModes checks that the Git file mode decides
// the archive mode: 100755 ships executable and 100644 does not.
func TestSourceArchiveEntriesMapsGitModes(t *testing.T) {
	got := sourceArchiveEntries(Export{Files: []SourceFile{
		{Path: "run.sh", Mode: "100755", Data: []byte("#!/bin/sh\n")},
		{Path: "a.go", Mode: "100644", Data: []byte("package a\n")},
	}})
	if len(got) != 2 || got[0].Mode != 0o755 || got[1].Mode != 0o644 {
		t.Fatalf("source archive modes = %+v", got)
	}
}

// TestExportSourceAdmitsCountsAtTheirBounds checks that both export bounds are
// inclusive: exactly maxExportEntries entries and exactly maxExportTotalBytes
// bytes are admitted; only exceeding either is refused.
func TestExportSourceAdmitsCountsAtTheirBounds(t *testing.T) {
	writeFixture := func(t *testing.T, sizes []int) (string, string, string) {
		t.Helper()
		home, root := t.TempDir(), t.TempDir()
		gitPath := writeFakeGit(t, home)
		var lstree, catfile bytes.Buffer
		for i, size := range sizes {
			data := make([]byte, size)
			h := sha1.New()
			fmt.Fprintf(h, "blob %d\x00", size)
			h.Write(data)
			oid := fmt.Sprintf("%x", h.Sum(nil))
			fmt.Fprintf(&lstree, "100644 blob %s\tf%d\x00", oid, i)
			fmt.Fprintf(&catfile, "%s blob %d\n", oid, size)
			catfile.Write(data)
			catfile.WriteByte('\n')
		}
		for name, body := range map[string][]byte{"lstree.out": lstree.Bytes(), "catfile.out": catfile.Bytes()} {
			if err := os.WriteFile(filepath.Join(home, name), body, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return gitPath, root, home
	}
	t.Run("entries", func(t *testing.T) {
		gitPath, root, home := writeFixture(t, make([]int, maxExportEntries))
		export, err := exportSource(context.Background(), gitPath, root, home)
		if err != nil || len(export.Files) != maxExportEntries {
			t.Fatalf("exactly %d entries: want admitted, got %d files, err=%v", maxExportEntries, len(export.Files), err)
		}
	})
	t.Run("total-bytes", func(t *testing.T) {
		gitPath, root, home := writeFixture(t, []int{maxExportTotalBytes / 2, maxExportTotalBytes / 2})
		export, err := exportSource(context.Background(), gitPath, root, home)
		if err != nil || export.TotalBytes != maxExportTotalBytes {
			t.Fatalf("exactly %d bytes: want admitted, got total=%d, err=%v", maxExportTotalBytes, export.TotalBytes, err)
		}
	})
}

// TestBatchBlobsRefusesWrongRecordTerminator checks that each cat-file record
// must end in a newline byte, not merely in some byte.
func TestBatchBlobsRefusesWrongRecordTerminator(t *testing.T) {
	entries := []treeEntry{{oid: "95d09f2b10159347eece71399a7e2e907ea3df4f", path: "f"}}
	fakeGit := func(time.Duration, []byte, ...string) ([]byte, error) {
		return []byte(entries[0].oid + " blob 11\nhello worldX"), nil
	}
	if _, err := batchBlobs(fakeGit, entries, forwardOrder); err == nil {
		t.Fatal("batchBlobs: record ending in a non-newline byte: want refusal, got nil")
	}
	if _, err := batchBlobsAlt(fakeGit, entries, forwardOrder); err == nil {
		t.Fatal("batchBlobsAlt: record ending in a non-newline byte: want refusal, got nil")
	}
}

// TestVerifyStagedTreeRefusesMissingFile checks the staged build tree must
// hold every exported file, not merely no file the export lacks.
func TestVerifyStagedTreeRefusesMissingFile(t *testing.T) {
	var files []SourceFile
	for _, name := range []string{"a.go", "b.go"} {
		data := []byte("package " + strings.TrimSuffix(name, ".go") + "\n")
		h := sha1.New()
		fmt.Fprintf(h, "blob %d\x00", len(data))
		h.Write(data)
		files = append(files, SourceFile{Path: name, Mode: "100644", OID: fmt.Sprintf("%x", h.Sum(nil)), Data: data})
	}
	export := Export{Files: files}
	dir, err := stageBuildSource(export, filepath.Join(t.TempDir(), "build-src"))
	if err != nil {
		t.Fatalf("stage complete export: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, "b.go")); err != nil {
		t.Fatal(err)
	}
	if err := verifyStagedTree(export, dir); err == nil {
		t.Fatal("staged tree missing an exported file: want refusal, got nil")
	}
}

// TestResolveToolchainRefusesMismatchedGoVersion checks the go on PATH must
// report exactly the pinned GOVERSION.
func TestResolveToolchainRefusesMismatchedGoVersion(t *testing.T) {
	stubDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(stubDir, "go"), []byte("#!/bin/sh\necho go1.0.0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, err := resolveToolchain(context.Background(), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "go toolchain mismatch") {
		t.Fatalf("go reporting another GOVERSION: want toolchain mismatch refusal, got %v", err)
	}
}

// TestCheckRefusedRequiresTheNamedReason checks a 403 passes the refusal step
// only when its body names the expected reason.
func TestCheckRefusedRequiresTheNamedReason(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unexpected Origin", http.StatusForbidden)
	}))
	defer server.Close()
	if s := checkRefused("named", server.URL, server.URL, url.Values{}, "unexpected Origin"); !s.OK {
		t.Fatalf("403 naming the expected reason: want OK, got %+v", s)
	}
	if s := checkRefused("other", server.URL, server.URL, url.Values{}, "invalid form token"); s.OK {
		t.Fatalf("403 naming another reason: want failed step, got %+v", s)
	}
}
