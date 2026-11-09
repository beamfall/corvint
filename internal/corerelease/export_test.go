//go:build darwin || linux

package corerelease

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/execfile"
)

func TestRefuseUnsafePath(t *testing.T) {
	t.Parallel()
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

func TestVerifyBlobDigestRefusesMismatch(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

//
// exportSource takes gitPath as a plain executable path that procgroup execs
// directly (no PATH lookup, no shell), so a tiny script stands in for git: it
// answers rev-parse/ls-tree/cat-file by catting canned fixture files placed
// under exportSource's own HOME (closedGitEnv sets HOME to the scratchParent
// argument). That reaches exportSource's real bound checks (export.go:71 and
// export.go:102) directly, without committing an over-bound real Git
// repository. The byte-bound tests lower exportLimits.totalBytes through
// withExportByteBound so they never allocate the 512 MiB production bound.

const fakeGitScript = `#!/bin/sh
case "$1" in
  rev-parse) echo "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef" ;;
  ls-tree) cat "$HOME/lstree.out" ;;
  cat-file) cat "$HOME/catfile.out" ;;
  *) exit 1 ;;
esac
`

// withExportByteBound lowers exportSource's byte bound to n for the test.
// No test in this package runs in parallel, so the override is not raced.
func withExportByteBound(t *testing.T, n int64) {
	t.Helper()
	saved := exportLimits
	exportLimits.totalBytes = n
	t.Cleanup(func() { exportLimits = saved })
}

func writeFakeGit(t *testing.T, home string) string {
	t.Helper()
	path := filepath.Join(home, "fake-git.sh")
	if err := execfile.WriteFile(path, []byte(fakeGitScript), 0o755); err != nil {
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
// captured cat-file output at the byte bound plus 1 MiB (proc.go:65), so
// overshooting the bound by too much trips that raw output-size guard
// instead of exportSource's own byte-count refusal. The bound is lowered to
// 4 MiB (withExportByteBound); the refusal logic is the production one.
func TestExportSourceRefusesTotalBytesBound(t *testing.T) {
	withExportByteBound(t, 4<<20)
	home := t.TempDir()
	root := t.TempDir()
	gitPath := writeFakeGit(t, home)

	const overBoundBy = 500_000
	half := int(exportLimits.totalBytes / 2)
	sizes := []int{half, half + overBoundBy} // each under the bound alone; sum is not
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

// TestBatchBlobsRefusesTrailingData guards both cat-file batch parsers
// against silently ignoring bytes after the last requested object: each
// only reads exactly len(oids) records and returned successfully even when
// the fake git process appended data past the last one, so a `cat-file
// --batch` stream carrying more than what listTreeEntries actually asked
// for went unnoticed instead of refused.
func TestBatchBlobsRefusesTrailingData(t *testing.T) {
	t.Parallel()
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

func TestVerifyTarGzRefusesTamperedArchive(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

// TestVerifyTarGzAdmitsFullSizeExportArchive checks that the source archive
// of an export at maxExportEntries verifies: verifyTarGz's entry bound must
// not refuse what exportSource admits (V1-1528).
func TestVerifyTarGzAdmitsFullSizeExportArchive(t *testing.T) {
	t.Parallel()
	entries := make([]ArchiveEntry, maxExportEntries)
	for i := range entries {
		entries[i] = ArchiveEntry{Path: fmt.Sprintf("d%d/f%d.go", i%97, i), Mode: 0o644}
	}
	archive, err := buildTarGz(entries)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := verifyTarGz(archive, entries); err != nil {
		t.Fatalf("full-size export archive: want verified, got %v", err)
	}
	if _, err := decodeTarGz(archive, maxExportEntries, maxExportTotalBytes); err != nil {
		t.Fatalf("full-size export archive at the export bounds: %v", err)
	}
}

// TestRefuseCaseFoldCollisionsMatchesPairwiseEqualFold checks the linear
// fold-key check against the pairwise strings.EqualFold definition it
// replaced, over every pair and triple of a corpus that includes multi-byte
// folds (Kelvin sign, long s), byte-length-changing folds and invalid UTF-8.
func TestRefuseCaseFoldCollisionsMatchesPairwiseEqualFold(t *testing.T) {
	t.Parallel()
	pairwise := func(paths []string) bool {
		dirPrefix := func(file, other string) bool {
			return len(other) > len(file) && other[len(file)] == '/' && strings.EqualFold(file, other[:len(file)])
		}
		for i := range paths {
			for j := i + 1; j < len(paths); j++ {
				a, b := paths[i], paths[j]
				if strings.EqualFold(a, b) || dirPrefix(a, b) || dirPrefix(b, a) {
					return true
				}
			}
		}
		return false
	}
	corpus := []string{
		"a", "A", "a/b", "A/b", "a/B", "b", "ab", "a.go", "A.go", "k", "K", "\u212a", "\u212a/x", "k/x",
		"s", "\u017f", "S/t", "\u017f/t", "\xff", "\xfe", "\ufffd", "\xff/a", "\xc3/a", "x/\u212a/y", "x/k",
		"\u0130", "i", "\u0131", "\u03a3", "\u03c3", "\u03c2", "\u03c2/z", "\u03a3",
	}
	for i, a := range corpus {
		for j, b := range corpus {
			if i == j {
				continue
			}
			for _, paths := range [][]string{{a, b}, {a, b, corpus[(i+j)%len(corpus)]}} {
				want := pairwise(paths)
				if got := refuseCaseFoldCollisions(paths) != nil; got != want {
					t.Fatalf("refuseCaseFoldCollisions(%q) refused=%v, pairwise EqualFold refused=%v", paths, got, want)
				}
			}
		}
	}
}

// TestVerifyTarGzRefusesCaseFoldedMembers drives PUB-V0-012's case-fold
// refusal at the archive-member verification boundary: buildTarGz's own
// duplicate check is exact-path only, so it happily assembles an archive
// holding both "A.go" and "a.go"; verifyTarGz must still refuse it before
// the archive is retained, since extracting it onto a case-insensitive
// filesystem collapses the two members into one.
func TestVerifyTarGzRefusesCaseFoldedMembers(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

func companionOwnedFixture(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "child.pid")
	script := filepath.Join(dir, "tool")
	body := fmt.Sprintf("#!/bin/sh\nsleep 60 &\nchild=$!\ntrap 'kill \"$child\" 2>/dev/null; wait \"$child\" 2>/dev/null' EXIT\ntrap 'exit 0' INT TERM\necho $child > '%s'\nwait\n", pidfile)
	if err := execfile.WriteFile(script, []byte(body), 0o755); err != nil {
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
		if err := execfile.WriteFile(gitPath, []byte(script), 0o755); err != nil {
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

// TestVerifyTarGzRefusesMissingChangedAndNonRegularMembers covers the closed
// inventory decode check from the other side of the extra-member case: an
// expected member absent from the archive, a member whose same-size content
// differs, and a non-regular member that otherwise matches its expectation.
func TestVerifyTarGzRefusesMissingChangedAndNonRegularMembers(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
		withExportByteBound(t, 4<<20)
		half := int(exportLimits.totalBytes / 2)
		gitPath, root, home := writeFixture(t, []int{half, half})
		export, err := exportSource(context.Background(), gitPath, root, home)
		if err != nil || export.TotalBytes != exportLimits.totalBytes {
			t.Fatalf("exactly %d bytes: want admitted, got total=%d, err=%v", exportLimits.totalBytes, export.TotalBytes, err)
		}
	})
	t.Run("production-bounds", func(t *testing.T) {
		if maxExportEntries != 50_000 || maxExportTotalBytes != 512<<20 {
			t.Fatalf("export bounds = %d entries, %d bytes; PUB-V0-012 names 50,000 and 512 MiB", maxExportEntries, maxExportTotalBytes)
		}
		if exportLimits.entries != maxExportEntries || exportLimits.totalBytes != maxExportTotalBytes {
			t.Fatalf("exportLimits = %+v, want the production bounds", exportLimits)
		}
		if maxRetainedEntries < maxExportEntries {
			t.Fatalf("maxRetainedEntries %d is below the export entry bound %d", maxRetainedEntries, maxExportEntries)
		}
	})
}

// TestBatchBlobsRefusesWrongRecordTerminator checks that each cat-file record
// must end in a newline byte, not merely in some byte.
func TestBatchBlobsRefusesWrongRecordTerminator(t *testing.T) {
	t.Parallel()
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

// TestResolveToolchainRefusesMismatchedGoVersion checks the go on PATH must
// report exactly the pinned GOVERSION.
func TestResolveToolchainRefusesMismatchedGoVersion(t *testing.T) {
	stubDir := t.TempDir()
	if err := execfile.WriteFile(filepath.Join(stubDir, "go"), []byte("#!/bin/sh\necho go1.0.0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, err := resolveToolchain(context.Background(), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "go toolchain mismatch") {
		t.Fatalf("go reporting another GOVERSION: want toolchain mismatch refusal, got %v", err)
	}
}
