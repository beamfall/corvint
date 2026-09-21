package contextindex

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func identAttributeRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	writeTestFile(t, root, ".gitattributes", "*.go ident\n")
	writeTestFile(t, root, "go.mod", "module example.test/ident\n\ngo 1.27.0\n")
	writeTestFile(t, root, "pkg/demux.go", "package pkg\n\n// $Id$\nfunc Demux() {}\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "ident fixture")
	if err := os.Remove(filepath.Join(root, "pkg", "demux.go")); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "checkout", "--", "pkg/demux.go")
	if status := testGit(t, root, "status", "--porcelain=v1"); status != "" {
		t.Fatalf("ident fixture is not status-clean: %q", status)
	}
	worktree, err := os.ReadFile(filepath.Join(root, "pkg", "demux.go"))
	if err != nil {
		t.Fatal(err)
	}
	blob := []byte(testGit(t, root, "show", "HEAD:pkg/demux.go"))
	if bytes.Equal(bytes.TrimSpace(worktree), bytes.TrimSpace(blob)) {
		t.Fatal("ident fixture worktree bytes do not diverge from the committed blob")
	}
	return root
}

func TestSnapshotHitAndMissUseStatusDirtyPaths(t *testing.T) {
	t.Run("IDX-SNAP-V0-006", func(t *testing.T) {
		root := identAttributeRepository(t)
		built, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := WriteSnapshot(built); err != nil {
			t.Fatal(err)
		}
		loaded, hit, err := LoadSnapshot(context.Background(), root)
		if err != nil || !hit {
			t.Fatalf("snapshot load: hit=%v err=%v", hit, err)
		}
		if !reflect.DeepEqual(loaded.DirtyPaths, built.DirtyPaths) {
			t.Fatalf("status-clean hit dirty paths %v differ from miss %v", loaded.DirtyPaths, built.DirtyPaths)
		}
	})
}

func TestProbeSnapshotReadsOnlyTheMatchingHeader(t *testing.T) {
	index := taskContextFixture(t)
	engineID := engine()
	receipt, err := WriteSnapshot(index)
	if err != nil {
		t.Fatal(err)
	}
	path := snapshotPath(index.Root, index.ObjectFormat, index.Revision, engineID)
	probe, fresh, err := ProbeSnapshot(context.Background(), index.Root)
	if err != nil || !fresh {
		t.Fatalf("IDX-SNAP-V0-011: fresh=%v err=%v", fresh, err)
	}
	if probe.Path != path || probe.Tree != index.Revision || probe.Commit != index.CommitRevision || probe.Engine != engineID {
		t.Fatalf("IDX-SNAP-V0-011: probe = %+v", probe)
	}
	// A torn body behind an intact header is undecodable, so it is a miss
	// that `index --if-stale` must rewrite (IDX-SNAP-V0-003).
	if err := os.Truncate(path, receipt.Bytes-1); err != nil {
		t.Fatal(err)
	}
	if _, fresh, err := ProbeSnapshot(context.Background(), index.Root); err != nil || fresh {
		t.Fatalf("IDX-SNAP-V0-011: truncated body probed fresh=%v err=%v", fresh, err)
	}
	if _, hit, err := LoadSnapshot(context.Background(), index.Root); err != nil || hit {
		t.Fatalf("truncated body decoded as an index: hit=%v err=%v", hit, err)
	}
}

func TestConcurrentEngineDigestMatchesSerialComputation(t *testing.T) {
	serial := digestExecutable()
	if serial == "" {
		t.Fatal("serial executable digest is empty")
	}
	concurrent := make(chan string, 1)
	go func() {
		concurrent <- engine()
	}()
	if got := <-concurrent; got != serial {
		t.Fatalf("concurrent engine digest = %q, serial digest = %q", got, serial)
	}
}

// TestSnapshotLoadJoinsAnEngineDigestThatFinishesLast covers the dual-goroutine
// join in loadSnapshot rather than the digest function alone: the engine
// goroutine is held behind a release channel, so it is the last of the three
// reads to yield and the load cannot take snapshotPath before joining it. The
// load must block until then and still resolve the file the real digest names.
func TestSnapshotLoadJoinsAnEngineDigestThatFinishesLast(t *testing.T) {
	index := taskContextFixture(t)
	root := index.Root
	receipt, err := WriteSnapshot(index)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Path != snapshotPath(root, index.ObjectFormat, index.Revision, engine()) {
		t.Fatalf("the written snapshot is not the one the engine digest names: %q", receipt.Path)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	serial := snapshotEngineID
	snapshotEngineID = func() string {
		close(entered)
		<-release
		return serial()
	}
	t.Cleanup(func() { snapshotEngineID = serial })
	type load struct {
		index *Index
		hit   bool
		err   error
	}
	done := make(chan load, 1)
	go func() {
		loaded, hit, err := LoadSnapshot(context.Background(), root)
		done <- load{loaded, hit, err}
	}()
	<-entered
	// The identity and status reads run to completion while the digest is
	// held, so the engine goroutine finishes last. A load that returned here
	// would have named a snapshot file without the joined digest.
	select {
	case got := <-done:
		t.Fatalf("the load returned before the engine digest joined: hit=%v err=%v", got.hit, got.err)
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	got := <-done
	if got.err != nil || !got.hit {
		t.Fatalf("load behind a late engine digest: hit=%v err=%v", got.hit, got.err)
	}
	if !reflect.DeepEqual(got.index, index) {
		t.Fatal("the snapshot loaded behind a late engine digest differs from the written index")
	}
}

// TestSnapshotHitReportsTheLiveCommitForTheSameTree: the file is keyed by
// tree, so a later commit with that tree hits it and must report HEAD, not
// the commit the snapshot was written at (IDX-SNAP-V0-002).
func TestSnapshotHitReportsTheLiveCommitForTheSameTree(t *testing.T) {
	index := taskContextFixture(t)
	if _, err := WriteSnapshot(index); err != nil {
		t.Fatal(err)
	}
	testGit(t, index.Root, "commit", "-q", "--allow-empty", "-m", "same tree")
	head := strings.TrimSpace(testGit(t, index.Root, "rev-parse", "HEAD"))
	loaded, hit, err := LoadSnapshot(context.Background(), index.Root)
	if err != nil || !hit {
		t.Fatalf("hit=%v err=%v", hit, err)
	}
	if loaded.CommitRevision != head || head == index.CommitRevision {
		t.Fatalf("loaded commit %s, HEAD %s, written at %s", loaded.CommitRevision, head, index.CommitRevision)
	}
}

func TestSnapshotRoundTripAppliesDirtyPathsAndMissesOnANewTree(t *testing.T) {
	index := taskContextFixture(t)
	root := index.Root
	if _, hit, err := LoadSnapshot(context.Background(), root); err != nil || hit {
		t.Fatalf("IDX-SNAP-V0-003: load before any write: hit=%v err=%v", hit, err)
	}
	receipt, err := WriteSnapshot(index)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Tree != index.Revision || receipt.Sources != len(index.Sources) || filepath.Dir(receipt.Path) != SnapshotDirectory(root) {
		t.Fatalf("receipt = %+v", receipt)
	}
	loaded, hit, err := LoadSnapshot(context.Background(), root)
	if err != nil || !hit {
		t.Fatalf("IDX-SNAP-V0-002: hit=%v err=%v", hit, err)
	}
	if !reflect.DeepEqual(loaded, index) {
		t.Fatalf("IDX-SNAP-V0-001: loaded index differs from the built one")
	}
	event, hit, err := LoadEventSnapshot(context.Background(), root, false)
	if err != nil || !hit {
		t.Fatalf("event load: hit=%v err=%v", hit, err)
	}
	if event.Tracked != nil || event.Vocabulary != nil || !reflect.DeepEqual(event.Sources, index.Sources) {
		t.Fatalf("event snapshot did not retain only its required tables")
	}
	compact, hit, err := LoadEventSnapshot(context.Background(), root, true)
	if err != nil || !hit {
		t.Fatalf("compact load: hit=%v err=%v", hit, err)
	}
	if compact.ProfileID != index.ProfileID || compact.Sources != nil {
		t.Fatalf("clean compact snapshot = %+v", compact)
	}
	if err := os.WriteFile(filepath.Join(root, "cache", "cache.go"), []byte("package cache\n\nfunc Demux() string { return \"changed\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirty, hit, err := LoadSnapshot(context.Background(), root)
	if err != nil || !hit {
		t.Fatalf("dirty load: hit=%v err=%v", hit, err)
	}
	if !reflect.DeepEqual(dirty.DirtyPaths, []string{"cache/cache.go"}) || dirty.StatusSHA256 == index.StatusSHA256 || string(dirty.Sources["cache/cache.go"].Data) != string(index.Sources["cache/cache.go"].Data) {
		t.Fatalf("IDX-SNAP-V0-004: dirty view = %v %q", dirty.DirtyPaths, dirty.Sources["cache/cache.go"].Data)
	}
	compactDirty, hit, err := LoadEventSnapshot(context.Background(), root, true)
	if err != nil || !hit || !reflect.DeepEqual(compactDirty.DirtyPaths, []string{"cache/cache.go"}) || len(compactDirty.Sources) == 0 {
		t.Fatalf("dirty compact load: hit=%v err=%v paths=%v sources=%d", hit, err, compactDirty.DirtyPaths, len(compactDirty.Sources))
	}
	for _, arguments := range [][]string{{"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "next"}} {
		command := exec.Command("git", append([]string{"-c", "gc.auto=0", "-c", "maintenance.auto=false"}, arguments...)...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	if _, hit, err := LoadSnapshot(context.Background(), root); err != nil || hit {
		t.Fatalf("IDX-SNAP-V0-003: a new tree must miss: hit=%v err=%v", hit, err)
	}
}

func TestLoadSnapshotMissesWhenIdentityChangesDuringStatusRead(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test wrapper uses a POSIX shell")
	}
	index := taskContextFixture(t)
	if _, err := WriteSnapshot(index); err != nil {
		t.Fatal(err)
	}
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	alternateCommit := "0" + index.CommitRevision[1:]
	if index.CommitRevision[0] == '0' {
		alternateCommit = "1" + index.CommitRevision[1:]
	}
	bin := t.TempDir()
	counter := filepath.Join(t.TempDir(), "identity-count")
	statusDone := filepath.Join(t.TempDir(), "status-done")
	wrapper := fmt.Sprintf(`#!/bin/sh
for argument in "$@"; do
  if [ "$argument" = "rev-parse" ]; then
    count=0
    if [ -f %s ]; then count=$(sed -n '1p' %s); fi
    count=$((count + 1))
    printf '%%s\n' "$count" > %s
    commit=%s
    tree=%s
    if [ "$count" -gt 1 ]; then
      if [ ! -f %s ]; then exit 99; fi
      commit=%s
    fi
    printf '%%s\nfalse\n%%s\n%%s\n.git/info/grafts\n' %s "$commit" "$tree"
    exit 0
  fi
  if [ "$argument" = "status" ]; then
    %s "$@"
    result=$?
    : > %s
    exit "$result"
  fi
done
exec %s "$@"
`, strconv.Quote(counter), strconv.Quote(counter), strconv.Quote(counter),
		strconv.Quote(index.CommitRevision), strconv.Quote(index.Revision), strconv.Quote(statusDone),
		strconv.Quote(alternateCommit), strconv.Quote(index.ObjectFormat),
		strconv.Quote(realGit), strconv.Quote(statusDone), strconv.Quote(realGit))
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if _, hit, err := LoadSnapshot(context.Background(), index.Root); err != nil || hit {
		t.Fatalf("IDX-SNAP-V0-002: identity drift load: hit=%v err=%v", hit, err)
	}
	rawCount, err := os.ReadFile(counter)
	if err != nil || strings.TrimSpace(string(rawCount)) != "2" {
		t.Fatalf("identity probes = %q, err=%v; want 2", rawCount, err)
	}
}

func TestWriteSnapshotDoesNotRewriteMatchingGitIgnore(t *testing.T) {
	index := taskContextFixture(t)
	if _, err := WriteSnapshot(index); err != nil {
		t.Fatal(err)
	}
	ignore := filepath.Join(SnapshotDirectory(index.Root), ".gitignore")
	unchanged := time.Unix(946_684_800, 0)
	if err := os.Chtimes(ignore, unchanged, unchanged); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteSnapshot(index); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(ignore)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(unchanged) {
		t.Fatalf("IDX-SNAP-V0-005: matching .gitignore was rewritten at %s", info.ModTime())
	}
	if err := os.WriteFile(ignore, []byte("wrong\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(ignore)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WriteSnapshot(index); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(ignore)
	if err != nil || string(content) != "*\n" {
		t.Fatalf("IDX-SNAP-V0-005: repaired .gitignore = %q, err=%v", content, err)
	}
	after, err := os.Stat(ignore)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(before, after) {
		t.Fatal("IDX-SNAP-V0-005: different .gitignore content was rewritten in place")
	}
}

func TestBuildContextMatchesBuildIncludingImportEvidence(t *testing.T) {
	full := taskContextFixture(t)
	narrow, err := BuildContext(context.Background(), full.Root, "cache/cache.go")
	if err != nil {
		t.Fatal(err)
	}
	if _, kept := narrow.Imports["server/server.go"]; !kept {
		t.Fatalf("the importer of the subject's package lost its edge: %v", narrow.Imports)
	}
	if !reflect.DeepEqual(narrow.Imports, full.Imports) {
		t.Fatalf("cold imports differ from a full snapshot: cold %v full %v", narrow.Imports, full.Imports)
	}
	task := "Change Demux in cache/cache.go and rerun the suite"
	fromFull, err := TaskContext(context.Background(), full, task, "cache/cache.go", 20)
	if err != nil {
		t.Fatal(err)
	}
	fromNarrow, err := TaskContext(context.Background(), narrow, task, "cache/cache.go", 20)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fromFull, fromNarrow) {
		t.Fatalf("packets differ:\n%v\n%v", fromFull, fromNarrow)
	}
	none, err := BuildContext(context.Background(), full.Root, "legacy/Legacy.java")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(none.Imports, full.Imports) {
		t.Fatalf("a subject with no import rule lost test-anchor edges: %v", none.Imports)
	}
}

func TestSourceTextIsDecodedOnceWhenPinned(t *testing.T) {
	pinned := Source{Data: []byte("package x\n"), Checked: true, Valid: true}
	text, valid, loaded := pinned.Text()
	if !loaded || !valid || text != "package x\n" {
		t.Fatalf("checked source text = %q valid=%v loaded=%v", text, valid, loaded)
	}
	binary := Source{Data: []byte{0xff, 0xfe, 0}, Checked: true, Valid: textDecodable([]byte{0xff, 0xfe, 0})}
	if _, valid, loaded := binary.Text(); !loaded || valid {
		t.Fatalf("undecodable source valid=%v loaded=%v", valid, loaded)
	}
	unchecked := Source{Data: []byte("plain")}
	if text, valid, loaded := unchecked.Text(); !loaded || !valid || text != "plain" {
		t.Fatalf("unchecked source text = %q valid=%v loaded=%v", text, valid, loaded)
	}
	if text, valid, loaded := (Source{Checked: true, Valid: true}).Text(); !loaded || !valid || text != "" {
		t.Fatalf("empty checked source = %q valid=%v loaded=%v", text, valid, loaded)
	}
	if text, valid, loaded := (Source{Path: "not-loaded"}).Text(); loaded || valid || text != "" {
		t.Fatalf("unloaded source text = %q valid=%v loaded=%v", text, valid, loaded)
	}
}

func TestAdoptStatusTakesTheClosingStatus(t *testing.T) {
	index := &Index{DirtyPaths: []string{"a.go", "diverged.go"}, StatusSHA256: "opening"}
	adoptStatus(index, repositoryObservation{dirty: []string{"b.go"}, statusSHA256: "closing"})
	if !reflect.DeepEqual(index.DirtyPaths, []string{"b.go"}) || index.StatusSHA256 != "closing" {
		t.Fatalf("adopted = %v %s", index.DirtyPaths, index.StatusSHA256)
	}
}

func TestEvictSnapshotsKeepsNewestEightIncludingCurrent(t *testing.T) {
	directory := t.TempDir()
	start := time.Unix(1_700_000_000, 0)
	paths := make([]string, snapshotKeep+2)
	for index := range paths {
		paths[index] = filepath.Join(directory, fmt.Sprintf("snapshot-%02d.gob", index))
		if err := os.WriteFile(paths[index], nil, 0o644); err != nil {
			t.Fatal(err)
		}
		when := start.Add(time.Duration(index) * time.Second)
		if err := os.Chtimes(paths[index], when, when); err != nil {
			t.Fatal(err)
		}
	}
	current := paths[len(paths)-1]
	if evicted := evictSnapshots(directory, current); evicted != 2 {
		t.Fatalf("IDX-SNAP-V0-007: evicted %d snapshots, want 2", evicted)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != snapshotKeep {
		t.Fatalf("IDX-SNAP-V0-007: kept %d snapshots, want %d", len(entries), snapshotKeep)
	}
	for _, path := range paths[len(paths)-snapshotKeep:] {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("IDX-SNAP-V0-007: newest snapshot %s was not kept: %v", filepath.Base(path), err)
		}
	}
	if _, err := os.Stat(current); err != nil {
		t.Errorf("IDX-SNAP-V0-007: current snapshot was not kept: %v", err)
	}
}

func TestEvictSnapshotsRemovesStaleTemporaries(t *testing.T) {
	t.Run("IDX-SNAP-V0-007", func(t *testing.T) {
		directory := t.TempDir()
		now := time.Unix(1_700_000_000, 0)
		stale, err := os.CreateTemp(directory, "snapshot-*.tmp")
		if err != nil {
			t.Fatal(err)
		}
		if err := stale.Close(); err != nil {
			t.Fatal(err)
		}
		old := now.Add(-2 * snapshotTemporaryStaleAfter)
		if err := os.Chtimes(stale.Name(), old, old); err != nil {
			t.Fatal(err)
		}
		fresh, err := os.CreateTemp(directory, "snapshot-*.tmp")
		if err != nil {
			t.Fatal(err)
		}
		if err := fresh.Close(); err != nil {
			t.Fatal(err)
		}
		young := now.Add(-snapshotTemporaryStaleAfter / 2)
		if err := os.Chtimes(fresh.Name(), young, young); err != nil {
			t.Fatal(err)
		}
		unrelated := filepath.Join(directory, "other.tmp")
		if err := os.WriteFile(unrelated, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(unrelated, old, old); err != nil {
			t.Fatal(err)
		}

		if evicted := evictSnapshotsAt(directory, "", now); evicted != 0 {
			t.Fatalf("reported %d published snapshots evicted, want zero", evicted)
		}
		if _, err := os.Stat(stale.Name()); !os.IsNotExist(err) {
			t.Fatalf("stale temporary still exists: %v", err)
		}
		if _, err := os.Stat(fresh.Name()); err != nil {
			t.Fatalf("fresh temporary was reclaimed: %v", err)
		}
		if _, err := os.Stat(unrelated); err != nil {
			t.Fatalf("unrelated temporary was reclaimed: %v", err)
		}
	})
}

// A repository can commit .corvint or .corvint/index as a symlink. Following it
// would repair a .gitignore, publish snapshots and evict files outside the
// worktree, so index refuses before writing anything (IDX-SNAP-V0-005).
func TestWriteSnapshotRefusesCommittedSymlinkedSnapshotDirectory(t *testing.T) {
	for _, link := range []string{".corvint", ".corvint/index"} {
		t.Run(link, func(t *testing.T) {
			root := impactRepositoryWithFiles(t, map[string]string{
				"go.mod":     "module example.test/symlinked\n\ngo 1.27.0\n",
				"pkg/pkg.go": "package pkg\n\nfunc Run() {}\n",
			})
			outside := t.TempDir()
			for name, content := range map[string]string{".gitignore": "keep\n", "old.gob": "foreign\n"} {
				if err := os.WriteFile(filepath.Join(outside, name), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			linkPath := filepath.Join(root, filepath.FromSlash(link))
			if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, linkPath); err != nil {
				t.Fatal(err)
			}
			testGit(t, root, "add", ".")
			testGit(t, root, "commit", "-qm", "symlinked snapshot directory")
			index, err := BuildForSnapshot(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := WriteSnapshot(index); err == nil {
				t.Fatal("IDX-SNAP-V0-005: a symlinked snapshot directory was written through")
			}
			entries, err := os.ReadDir(outside)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 2 {
				t.Fatalf("IDX-SNAP-V0-005: outside directory gained entries: %v", entries)
			}
			if content, _ := os.ReadFile(filepath.Join(outside, ".gitignore")); string(content) != "keep\n" {
				t.Fatalf("IDX-SNAP-V0-005: outside .gitignore = %q", content)
			}
		})
	}
}

// The read side refuses what the writer refuses: a committed symlink can point
// .corvint or .corvint/index at a directory outside the worktree that holds a
// snapshot named for this tree, and every loader must miss on it rather than
// serve those outside bytes (IDX-SNAP-V0-005).
func TestSnapshotReadersMissThroughCommittedSymlinkedSnapshotDirectory(t *testing.T) {
	for _, link := range []string{".corvint", ".corvint/index"} {
		t.Run(link, func(t *testing.T) {
			root := impactRepositoryWithFiles(t, map[string]string{
				"go.mod":     "module example.test/symlinkedread\n\ngo 1.27.0\n",
				"pkg/pkg.go": "package pkg\n\nfunc Run() {}\n",
			})
			outside := filepath.Join(t.TempDir(), "outside")
			linkPath := filepath.Join(root, filepath.FromSlash(link))
			if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, linkPath); err != nil {
				t.Fatal(err)
			}
			testGit(t, root, "add", ".")
			testGit(t, root, "commit", "-qm", "symlinked snapshot directory")
			index, err := BuildForSnapshot(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			// Publish this tree's snapshot into a real directory, then move it
			// outside and restore the committed link over it.
			if err := os.Remove(linkPath); err != nil {
				t.Fatal(err)
			}
			if _, err := WriteSnapshot(index); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(linkPath, outside); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, linkPath); err != nil {
				t.Fatal(err)
			}
			if status := testGit(t, root, "status", "--porcelain"); status != "" {
				t.Fatalf("worktree must match the committed link: %q", status)
			}
			if _, hit, err := LoadSnapshot(context.Background(), root); err != nil || hit {
				t.Fatalf("IDX-SNAP-V0-005: LoadSnapshot through a symlink hit=%v err=%v", hit, err)
			}
			if _, hit, err := ProbeSnapshot(context.Background(), root); err != nil || hit {
				t.Fatalf("IDX-SNAP-V0-005: ProbeSnapshot through a symlink hit=%v err=%v", hit, err)
			}
			observation := Observation{ObjectFormat: index.ObjectFormat, CommitRevision: index.CommitRevision, Revision: index.Revision}
			if _, hit, err := LoadEventSnapshotObserved(root, false, observation); err != nil || hit {
				t.Fatalf("IDX-SNAP-V0-005: LoadEventSnapshotObserved through a symlink hit=%v err=%v", hit, err)
			}
		})
	}
}
