package contextindex

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func requireIndexStat(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("Git index stat routing is darwin/linux only")
	}
}

// statRoutedRepository commits files, then rewrites the index so every
// entry's modification time is in a whole second before the index file's.
func statRoutedRepository(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	for path, content := range files {
		writeTestFile(t, root, path, content)
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "stat routed pins")
	settleIndex(t, root)
	return root
}

// settleIndex backdates every tracked file past the one-second racy window
// and rewrites the index, so no entry is racy.
func settleIndex(t *testing.T, root string) {
	t.Helper()
	earlier := time.Now().Add(-3 * time.Second)
	for _, path := range strings.Split(testGit(t, root, "ls-files", "-z"), "\x00") {
		if path == "" {
			continue
		}
		if err := os.Chtimes(filepath.Join(root, filepath.FromSlash(path)), earlier, earlier); err != nil {
			t.Fatal(err)
		}
	}
	testGit(t, root, "update-index", "--really-refresh")
	testGit(t, root, "update-index", "--force-write-index")
}

func gitIndexPath(t *testing.T, root string) string {
	t.Helper()
	path := testGit(t, root, "rev-parse", "--git-path", "index")
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	return path
}

// captureWorktreeReads records every path the worktree pin pass opens.
func captureWorktreeReads(t *testing.T) func() []string {
	t.Helper()
	var mu sync.Mutex
	read := []string{}
	original := worktreePinRead
	worktreePinRead = func(path string) {
		mu.Lock()
		read = append(read, path)
		mu.Unlock()
	}
	t.Cleanup(func() { worktreePinRead = original })
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		out := append([]string(nil), read...)
		read = read[:0]
		sort.Strings(out)
		return out
	}
}

func revisionEvidence(t *testing.T, root string, routed bool) *Index {
	t.Helper()
	ctx := context.Background()
	identity, err := readIdentity(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if routed {
		ctx = withStatRoutedPins(ctx)
	}
	index, err := buildEvidence(ctx, root, identity, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	return index
}

var statRoutedFiles = map[string]string{
	"go.mod":                    "module example.test/routed\n\ngo 1.27.0\n",
	"internal/auth/session.go":  "package auth\n\nfunc Session() bool { return true }\n",
	"internal/auth/touched.go":  "package auth\n\nfunc Touched() int { return 1 }\n",
	"internal/auth/modified.go": "package auth\n\nfunc Modified() int { return 2 }\n",
	"internal/copy/one.go":      "package copyone\n\nfunc Shared() {}\n",
	"internal/copy/two.go":      "package copyone\n\nfunc Shared() {}\n",
	"internal/empty/empty.go":   "",
	"docs/guide.md":             "# Guide\n\nSee internal/auth/session.go.\n",
}

// TestDCRV1452GitIndexVouchesUnchangedEntries: on an unchanged checkout every
// eligible entry is vouched by Git's index stat data and pinned from its
// committed blob, so the routed pass opens no worktree file; the unrouted pass
// still opens every one. A file whose stat changed (same bytes, new times) or
// whose content changed is opened and hashed.
func TestDCRV1452GitIndexVouchesUnchangedEntries(t *testing.T) {
	requireIndexStat(t)
	root := statRoutedRepository(t, statRoutedFiles)
	reads := captureWorktreeReads(t)
	routed := revisionEvidence(t, root, true)
	if got := reads(); len(got) != 0 {
		t.Fatalf("routed pass opened %v on an unchanged checkout", got)
	}
	plain := revisionEvidence(t, root, false)
	if got := reads(); len(got) != len(statRoutedFiles) {
		t.Fatalf("unrouted pass opened %d files, want %d: %v", len(got), len(statRoutedFiles), got)
	}
	assertIndexEvidenceEqual(t, root, plain, routed)

	later := time.Now().Add(time.Second)
	if err := os.Chtimes(filepath.Join(root, "internal/auth/touched.go"), later, later); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, root, "internal/auth/modified.go", "package auth\n\nfunc Modified() int { return 3 }\n")
	routed = revisionEvidence(t, root, true)
	if got := reads(); len(got) != 2 || got[0] != "internal/auth/modified.go" || got[1] != "internal/auth/touched.go" {
		t.Fatalf("routed pass opened %v, want the modified and touched files", got)
	}
	plain = revisionEvidence(t, root, false)
	reads()
	assertIndexEvidenceEqual(t, root, plain, routed)
	if got := string(routed.Sources["internal/auth/modified.go"].Data); got != statRoutedFiles["internal/auth/modified.go"] {
		t.Fatalf("modified file pinned %q, want the committed blob", got)
	}
}

// TestDCRV1452RacyEntryIsNotVouched: an entry whose recorded modification time
// falls in the index file's second or later is not vouched even when every
// stat field matches (Git's racy-clean rule with a whole-second window), and
// one in an earlier second is.
func TestDCRV1452RacyEntryIsNotVouched(t *testing.T) {
	requireIndexStat(t)
	path := filepath.Join(t.TempDir(), "file.go")
	if err := os.WriteFile(path, []byte("package file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	recorded, ok := fileIndexStat(info)
	if !ok {
		t.Fatal("no stat fields")
	}
	recorded.mode = 0o100644
	at := func(sec, nsec uint32) *gitIndex { return &gitIndex{mtimeSec: sec, mtimeNsec: nsec} }
	if at(recorded.mtimeSec, recorded.mtimeNsec).vouches(recorded, info) {
		t.Fatal("an entry as new as the index was vouched")
	}
	if at(recorded.mtimeSec-1, 999_999_999).vouches(recorded, info) {
		t.Fatal("an entry newer than the index was vouched")
	}
	if at(recorded.mtimeSec, 999_999_999).vouches(recorded, info) {
		t.Fatal("an entry in the index file's second was vouched")
	}
	later := at(recorded.mtimeSec+1, 0)
	if !later.vouches(recorded, info) {
		t.Fatal("an entry from an earlier second than the index was not vouched")
	}
	for name, edit := range map[string]func(*gitIndexStat){
		"size":  func(s *gitIndexStat) { s.size++ },
		"mtime": func(s *gitIndexStat) { s.mtimeNsec ^= 1 },
		"ctime": func(s *gitIndexStat) { s.ctimeNsec ^= 1 },
		"inode": func(s *gitIndexStat) { s.ino++ },
		"uid":   func(s *gitIndexStat) { s.uid++ },
		"gid":   func(s *gitIndexStat) { s.gid++ },
		"mode":  func(s *gitIndexStat) { s.mode = 0o100755 },
		"link":  func(s *gitIndexStat) { s.mode = 0o120000 },
	} {
		changed := recorded
		edit(&changed)
		if later.vouches(changed, info) {
			t.Errorf("%s mismatch was vouched", name)
		}
	}
}

// TestDCRV1452IndexVersionsAndFlags: version 2, 3 and 4 indexes parse to the
// same stage-0 stat data; a skip-worktree, assume-unchanged or intent-to-add
// entry, and a conflicted path, are left out so they are never vouched.
func TestDCRV1452IndexVersionsAndFlags(t *testing.T) {
	requireIndexStat(t)
	root := statRoutedRepository(t, statRoutedFiles)
	index, ok := readGitIndex(gitIndexPath(t, root), "sha1")
	if !ok || len(index.entries) != len(statRoutedFiles) {
		t.Fatalf("v2 index: ok=%v entries=%d", ok, len(index.entries))
	}
	testGit(t, root, "update-index", "--index-version", "4")
	v4, ok := readGitIndex(gitIndexPath(t, root), "sha1")
	if !ok || len(v4.entries) != len(index.entries) {
		t.Fatalf("v4 index: ok=%v entries=%d", ok, len(v4.entries))
	}
	for path, stat := range index.entries {
		if v4.entries[path] != stat {
			t.Fatalf("v4 stat for %s = %+v, want %+v", path, v4.entries[path], stat)
		}
	}
	testGit(t, root, "update-index", "--index-version", "2")
	testGit(t, root, "update-index", "--skip-worktree", "internal/auth/session.go")
	testGit(t, root, "update-index", "--assume-unchanged", "docs/guide.md")
	writeTestFile(t, root, "internal/new.go", "package internal\n")
	testGit(t, root, "add", "-N", "internal/new.go")
	data, err := os.ReadFile(gitIndexPath(t, root))
	if err != nil {
		t.Fatal(err)
	}
	if version := binary.BigEndian.Uint32(data[4:8]); version != 3 {
		t.Fatalf("extended flags wrote index version %d, want 3", version)
	}
	v3, ok := readGitIndex(gitIndexPath(t, root), "sha1")
	if !ok {
		t.Fatal("v3 index refused")
	}
	for _, path := range []string{"internal/auth/session.go", "docs/guide.md", "internal/new.go"} {
		if _, present := v3.entries[path]; present {
			t.Errorf("%s is flagged and must not be vouchable", path)
		}
	}
	if _, present := v3.entries["go.mod"]; !present {
		t.Error("an unflagged entry is missing from the v3 index")
	}
	testGit(t, root, "-c", "index.skipHash=true", "update-index", "--force-write-index")
	data, err = os.ReadFile(gitIndexPath(t, root))
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range data[len(data)-20:] {
		if b != 0 {
			t.Skip("this Git does not write an index.skipHash trailer")
		}
	}
	if skipped, ok := readGitIndex(gitIndexPath(t, root), "sha1"); !ok || len(skipped.entries) != len(v3.entries) {
		t.Fatalf("skipHash index: ok=%v", ok)
	}
}

// FuzzParseGitIndex: no input panics the index parser; a hostile index can
// only vouch for nothing.
func FuzzParseGitIndex(f *testing.F) {
	f.Add([]byte("DIRC\x00\x00\x00\x02\x00\x00\x00\x00"))
	f.Add(append([]byte("DIRC\x00\x00\x00\x04\x00\x00\x00\x01"), make([]byte, 120)...))
	f.Fuzz(func(t *testing.T, data []byte) {
		for _, format := range []string{"sha1", "sha256"} {
			parseGitIndex(data, format)
			hashSize := map[string]int{"sha1": 20, "sha256": 32}[format]
			if len(data) >= hashSize {
				zeroed := append(append([]byte{}, data[:len(data)-hashSize]...), make([]byte, hashSize)...)
				parseGitIndex(zeroed, format)
			}
		}
	})
}

// TestDCRV1453UnusableIndexVouchesNothing: a corrupt checksum, an unsupported
// version, a truncated file, a split index and a missing index each vouch for
// nothing, and the routed build then opens and hashes every eligible file and
// pins exactly what the unrouted build pins.
func TestDCRV1453UnusableIndexVouchesNothing(t *testing.T) {
	requireIndexStat(t)
	resum := func(data []byte) []byte {
		sum := sha1.Sum(data[:len(data)-20])
		copy(data[len(data)-20:], sum[:])
		return data
	}
	cases := map[string]func(t *testing.T, root, index string){
		"checksum": func(t *testing.T, root, index string) {
			data, _ := os.ReadFile(index)
			data[len(data)-1] ^= 0xff
			writeIndex(t, index, data)
		},
		"version": func(t *testing.T, root, index string) {
			data, _ := os.ReadFile(index)
			binary.BigEndian.PutUint32(data[4:8], 5)
			writeIndex(t, index, resum(data))
		},
		"truncated": func(t *testing.T, root, index string) {
			data, _ := os.ReadFile(index)
			writeIndex(t, index, data[:len(data)/2])
		},
		"entry count": func(t *testing.T, root, index string) {
			data, _ := os.ReadFile(index)
			binary.BigEndian.PutUint32(data[8:12], binary.BigEndian.Uint32(data[8:12])+1)
			writeIndex(t, index, resum(data))
		},
		"split": func(t *testing.T, root, index string) {
			testGit(t, root, "update-index", "--split-index")
		},
		"missing": func(t *testing.T, root, index string) {
			if err := os.Remove(index); err != nil {
				t.Fatal(err)
			}
		},
		"unknown extended flag": func(t *testing.T, root, index string) {
			testGit(t, root, "update-index", "--skip-worktree", "go.mod")
			data, _ := os.ReadFile(index)
			at := indexEntryExtendedFlags(t, data, "go.mod")
			binary.BigEndian.PutUint16(data[at:], binary.BigEndian.Uint16(data[at:])|0x0001)
			writeIndex(t, index, resum(data))
		},
		"long pathname": func(t *testing.T, root, index string) {
			long := strings.Repeat("a", maxGitIndexPathBytes)
			writeIndex(t, index, syntheticIndexV4([]string{long, long + "b"}))
		},
		"expanded pathnames": func(t *testing.T, root, index string) {
			names := []string{strings.Repeat("a", 2000)}
			for range 400 {
				names = append(names, names[len(names)-1]+"b")
			}
			writeIndex(t, index, syntheticIndexV4(names))
		},
		"symlink": func(t *testing.T, root, index string) {
			moved := index + ".real"
			if err := os.Rename(index, moved); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(moved, index); err != nil {
				t.Skip(err)
			}
		},
	}
	for name, damage := range cases {
		t.Run(name, func(t *testing.T) {
			root := statRoutedRepository(t, statRoutedFiles)
			path := gitIndexPath(t, root)
			damage(t, root, path)
			if _, ok := readGitIndex(path, "sha1"); ok {
				t.Fatal("damaged index was read")
			}
			reads := captureWorktreeReads(t)
			routed := revisionEvidence(t, root, true)
			if got := reads(); len(got) != len(statRoutedFiles) {
				t.Fatalf("routed pass opened %d files, want every one of %d", len(got), len(statRoutedFiles))
			}
			assertIndexEvidenceEqual(t, root, revisionEvidence(t, root, false), routed)
		})
	}
}

// TestDCRV1453IndexPathnameBounds: a version 4 index whose prefix compression
// stays within the pathname bounds parses; one pathname past PATH_MAX, or
// decoded pathnames past the expansion bound, refuse the whole index before
// the names are allocated.
func TestDCRV1453IndexPathnameBounds(t *testing.T) {
	chain := func(first, count int) []string {
		names := []string{strings.Repeat("a", first)}
		for range count {
			names = append(names, names[len(names)-1]+"b")
		}
		return names
	}
	if index, ok := parseGitIndex(syntheticIndexV4(chain(40, 40)), "sha1"); !ok || len(index.entries) != 41 {
		t.Fatalf("bounded v4 index: ok=%v", ok)
	}
	if index, ok := parseGitIndex(syntheticIndexV4(chain(maxGitIndexPathBytes, 0)), "sha1"); !ok || len(index.entries) != 1 {
		t.Fatalf("PATH_MAX pathname: ok=%v", ok)
	}
	if _, ok := parseGitIndex(syntheticIndexV4(chain(maxGitIndexPathBytes, 1)), "sha1"); ok {
		t.Fatal("a pathname past PATH_MAX was parsed")
	}
	if _, ok := parseGitIndex(syntheticIndexV4(chain(2000, 400)), "sha1"); ok {
		t.Fatal("pathnames past the expansion bound were parsed")
	}
}

// indexEntryExtendedFlags returns the offset of path's extended flag word in a
// version 3 SHA-1 index.
func indexEntryExtendedFlags(t *testing.T, data []byte, path string) int {
	t.Helper()
	offset := 12
	for range binary.BigEndian.Uint32(data[8:12]) {
		flags := binary.BigEndian.Uint16(data[offset+60:])
		cursor := 62
		if flags&0x4000 != 0 {
			cursor += 2
		}
		end := bytes.IndexByte(data[offset+cursor:], 0)
		if string(data[offset+cursor:offset+cursor+end]) == path {
			if flags&0x4000 == 0 {
				t.Fatalf("%s has no extended flags", path)
			}
			return offset + 62
		}
		offset += (cursor + end + 8) &^ 7
	}
	t.Fatalf("%s is not in the index", path)
	return 0
}

// syntheticIndexV4 encodes a checksummed version 4 index whose entries each
// keep the previous pathname whole (strip 0) and append their own suffix.
func syntheticIndexV4(names []string) []byte {
	data := []byte("DIRC\x00\x00\x00\x04")
	data = binary.BigEndian.AppendUint32(data, uint32(len(names)))
	previous := ""
	for _, name := range names {
		entry := make([]byte, 62)
		binary.BigEndian.PutUint32(entry[24:], 0o100644)
		binary.BigEndian.PutUint16(entry[60:], uint16(min(len(name), 0x0fff)))
		entry = append(entry, 0)
		entry = append(entry, strings.TrimPrefix(name, previous)...)
		entry = append(entry, 0)
		data = append(data, entry...)
		previous = name
	}
	sum := sha1.Sum(data)
	return append(data, sum[:]...)
}

func writeIndex(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestDCRV1453FailedBlobBatchFallsBackToWorktree: when the committed blobs of
// vouched entries cannot be read, the entries take the hashed worktree pass
// instead, so the routed build still pins every source the unrouted one does.
func TestDCRV1453FailedBlobBatchFallsBackToWorktree(t *testing.T) {
	requireIndexStat(t)
	root := statRoutedRepository(t, statRoutedFiles)
	original := fetchVouchedBlobs
	fetchVouchedBlobs = func(context.Context, string, []treeEntry, []treeEntry) residualBlobs {
		return residualBlobs{err: &Error{Message: "object store unavailable"}}
	}
	t.Cleanup(func() { fetchVouchedBlobs = original })
	reads := captureWorktreeReads(t)
	routed := revisionEvidence(t, root, true)
	if got := reads(); len(got) != len(statRoutedFiles) {
		t.Fatalf("after a failed batch the routed pass opened %d files, want %d", len(got), len(statRoutedFiles))
	}
	assertIndexEvidenceEqual(t, root, revisionEvidence(t, root, false), routed)
	if got := string(routed.Sources["internal/auth/session.go"].Data); got != statRoutedFiles["internal/auth/session.go"] {
		t.Fatalf("session.go pinned %q", got)
	}
}

// TestDCRV1453RoutedPinMatchesParityRepository: over the pin parity fixture
// (dirty, deleted, unreadable, symlinked, executable, shared-blob, empty and
// generated entries) the routed pass pins byte-identical evidence.
func TestDCRV1453RoutedPinMatchesParityRepository(t *testing.T) {
	requireIndexStat(t)
	root, _ := pinParityRepository(t)
	assertIndexEvidenceEqual(t, root, revisionEvidence(t, root, false), revisionEvidence(t, root, true))
	if _, err := BuildRevisionContext(context.Background(), root, testGit(t, root, "rev-parse", "HEAD")); err != nil {
		t.Fatal(err)
	}
}
