//go:build linux

package authority

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// refuseDirectoryWatches makes the kernel refuse, with ENOSPC, every
// directory watch, as it does when the user's fs.inotify.max_user_watches
// allowance is spent.
func refuseDirectoryWatches(t *testing.T) {
	t.Helper()
	original := inotifyAddWatch
	inotifyAddWatch = func(fd int, path string, mask uint32) (int, error) {
		if info, err := os.Lstat(path); err == nil && info.IsDir() {
			return -1, syscall.ENOSPC
		}
		return original(fd, path, mask)
	}
	t.Cleanup(func() { inotifyAddWatch = original })
}

// drain discards the inotify events queued so far and clears the sticky
// result, so that what follows is seen by the stat identity alone.
func drain(w *notifyWatch) {
	w.dirty = false
	var buf [4096]byte
	for {
		if n, err := syscall.Read(w.fd, buf[:]); err != nil || n == 0 {
			return
		}
	}
}

// PSR-V0-222: no regular file takes a watch, and every mutation class of a
// file singly linked in a watched directory is reported by the directory's
// events, whatever the number of files; quiescence is not reported.
func TestPSRV0222_ChangeGuardSeesFileMutationsThroughItsDirectory(t *testing.T) {
	for _, name := range []string{"none", "write-same-size", "write-resize", "in-place-restored-time", "open-before-guard", "truncate", "utimes", "chmod", "rename-replace", "remove", "sibling"} {
		t.Run(name, func(t *testing.T) {
			repo, file := guardRepo(t)
			dir := filepath.Dir(file)
			staging := filepath.Join(filepath.Dir(repo.StateDir), "staging-outside")
			if err := os.Mkdir(staging, 0700); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 4; i++ {
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%03d", i)), []byte("x"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			var early *os.File
			if name == "open-before-guard" {
				var err error
				if early, err = os.OpenFile(file, os.O_WRONLY, 0); err != nil {
					t.Fatal(err)
				}
				defer early.Close()
			}
			original := inotifyAddWatch
			t.Cleanup(func() { inotifyAddWatch = original })
			files := 0
			inotifyAddWatch = func(fd int, path string, mask uint32) (int, error) {
				if info, err := os.Lstat(path); err == nil && !info.IsDir() {
					files++
				}
				return original(fd, path, mask)
			}
			g, err := WatchChanges(repo)
			if err != nil {
				t.Fatal(err)
			}
			defer g.Close()
			if w := g.watch.(*notifyWatch); files != 0 || len(w.uncovered) != 0 {
				t.Fatalf("%d file watches and %d uncovered files, want none", files, len(w.uncovered))
			}
			switch name {
			case "write-same-size":
				err = os.WriteFile(file, []byte("bbbb"), 0600)
			case "write-resize":
				err = os.WriteFile(file, []byte("bbbbbb"), 0600)
			case "in-place-restored-time":
				st, _ := os.Stat(file)
				err = os.WriteFile(file, []byte("cccc"), 0600)
				if err == nil {
					err = os.Chtimes(file, st.ModTime(), st.ModTime())
				}
			case "open-before-guard":
				// A descriptor opened through this entry before the guard
				// writes through it; one opened through another link would
				// not be reported (PSR-V0-222).
				_, err = early.WriteAt([]byte("dddd"), 0)
			case "truncate":
				err = os.Truncate(file, 1)
			case "utimes":
				err = os.Chtimes(file, time.Unix(1, 0), time.Unix(1, 0))
			case "chmod":
				err = os.Chmod(file, 0640)
			case "rename-replace":
				// Staged outside the observation, so only the rename over
				// the entry can report it.
				staged := filepath.Join(staging, "x.json")
				err = os.WriteFile(staged, []byte("aaaa"), 0600)
				if err == nil {
					err = os.Rename(staged, file)
				}
			case "remove":
				err = os.Remove(file)
			case "sibling":
				err = os.WriteFile(file+".new", []byte("new"), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			want := wire.CodeSnapshotMoved
			if name == "none" {
				want = ""
			}
			if got := wire.CodeOf(g.CheckEvents()); got != want {
				t.Fatalf("check = %q, want %q", got, want)
			}
		})
	}
}

// PSR-V0-222: a regular file with a second link, through which a write
// bypasses the watched directory, is tracked by its stat identity, and a
// write or mode change through that link is seen by the sweep alone.
func TestPSRV0222_ChangeGuardTracksAMultiplyLinkedFileByStat(t *testing.T) {
	for _, name := range []string{"none", "write-same-size", "in-place-restored-time", "chmod"} {
		t.Run(name, func(t *testing.T) {
			repo, file := guardRepo(t)
			outside := filepath.Join(t.TempDir(), "link")
			if err := os.Link(file, outside); err != nil {
				t.Skipf("hard links unsupported: %v", err)
			}
			g, err := WatchChanges(repo)
			if err != nil {
				t.Fatal(err)
			}
			defer g.Close()
			w := g.watch.(*notifyWatch)
			if len(w.uncovered) != 1 || w.files[w.uncovered[0]].path != file {
				t.Fatalf("uncovered %v of %v, want only %s", w.uncovered, w.files, file)
			}
			switch name {
			case "write-same-size":
				err = os.WriteFile(outside, []byte("bbbb"), 0600)
			case "in-place-restored-time":
				st, _ := os.Stat(outside)
				err = os.WriteFile(outside, []byte("cccc"), 0600)
				if err == nil {
					err = os.Chtimes(outside, st.ModTime(), st.ModTime())
				}
			case "chmod":
				err = os.Chmod(outside, 0640)
			}
			if err != nil {
				t.Fatal(err)
			}
			drain(w)
			if got := w.sweep(); got != (name != "none") {
				t.Fatalf("sweep = %v after %s", got, name)
			}
			want := wire.CodeSnapshotMoved
			if name == "none" {
				want = ""
			}
			if got := wire.CodeOf(g.Check()); got != want {
				t.Fatalf("check = %q, want %q", got, want)
			}
		})
	}
}

// PSR-V0-223: a link made to a singly linked file after registration, outside
// every watched directory, and a write or mode change through it, or through a
// descriptor opened through a link removed before registration, reaches no
// watched directory; the stat identity re-read by Sweep (link count, size,
// mode, change time) reports it to CheckEvents under the writer lock, and
// Check reports it too.
func TestPSRV0223_ChangeGuardSeesAWriteThroughAnAliasLink(t *testing.T) {
	for _, name := range []string{"none", "alias", "alias-write-same-size", "alias-write-then-unlink", "alias-chmod", "removed-link-descriptor"} {
		t.Run(name, func(t *testing.T) {
			repo, file := guardRepo(t)
			outside := filepath.Join(t.TempDir(), "alias")
			var early *os.File
			if name == "removed-link-descriptor" {
				if err := os.Link(file, outside); err != nil {
					t.Skipf("hard links unsupported: %v", err)
				}
				var err error
				if early, err = os.OpenFile(outside, os.O_WRONLY, 0); err != nil {
					t.Fatal(err)
				}
				defer early.Close()
				if err := os.Remove(outside); err != nil {
					t.Fatal(err)
				}
			}
			g, err := WatchChanges(repo)
			if err != nil {
				t.Fatal(err)
			}
			defer g.Close()
			if w := g.watch.(*notifyWatch); len(w.uncovered) != 0 {
				t.Fatalf("uncovered %v, want none: the file is singly linked", w.uncovered)
			}
			alias := func() error {
				if err := os.Link(file, outside); err != nil {
					t.Skipf("hard links unsupported: %v", err)
				}
				return nil
			}
			switch name {
			case "alias":
				err = alias()
			case "alias-write-same-size":
				if err = alias(); err == nil {
					err = os.WriteFile(outside, []byte("bbbb"), 0600)
				}
			case "alias-write-then-unlink":
				// The link count is restored; the size is not.
				if err = alias(); err == nil {
					err = os.WriteFile(outside, []byte("bbbbbb"), 0600)
				}
				if err == nil {
					err = os.Remove(outside)
				}
			case "alias-chmod":
				if err = alias(); err == nil {
					err = os.Chmod(outside, 0640)
				}
			case "removed-link-descriptor":
				_, err = early.WriteAt([]byte("dddddd"), 0)
			}
			if err != nil {
				t.Fatal(err)
			}
			want := wire.CodeSnapshotMoved
			if name == "none" {
				want = ""
			}
			g.Sweep()
			if got := wire.CodeOf(g.CheckEvents()); got != want {
				t.Fatalf("check events after sweep = %q, want %q", got, want)
			}
			if got := wire.CodeOf(g.Check()); got != want {
				t.Fatalf("check = %q, want %q", got, want)
			}
		})
	}
}

// PSR-V0-223: a file with a second link at registration is re-read by every
// poll, so a write or mode change through that link after Sweep and before the
// check under the writer lock is still reported by CheckEvents.
func TestPSRV0223_ChangeGuardSeesASecondLinkWriteAfterTheSweep(t *testing.T) {
	for _, name := range []string{"none", "write-resize", "truncate", "chmod"} {
		t.Run(name, func(t *testing.T) {
			repo, file := guardRepo(t)
			outside := filepath.Join(t.TempDir(), "link")
			if err := os.Link(file, outside); err != nil {
				t.Skipf("hard links unsupported: %v", err)
			}
			g, err := WatchChanges(repo)
			if err != nil {
				t.Fatal(err)
			}
			defer g.Close()
			if err := g.Check(); err != nil {
				t.Fatal(err)
			}
			g.Sweep()
			switch name {
			case "write-resize":
				err = os.WriteFile(outside, []byte("bbbbbb"), 0600)
			case "truncate":
				err = os.Truncate(outside, 1)
			case "chmod":
				err = os.Chmod(outside, 0640)
			}
			if err != nil {
				t.Fatal(err)
			}
			want := wire.CodeSnapshotMoved
			if name == "none" {
				want = ""
			}
			if got := wire.CodeOf(g.CheckEvents()); got != want {
				t.Fatalf("check events after sweep = %q, want %q", got, want)
			}
		})
	}
}

// PSR-V0-222: a directory already watched with the content mask keeps it when
// it is added again as an ancestor, so a write to an entry is still reported.
func TestPSRV0222_AncestorAddKeepsTheContentMask(t *testing.T) {
	_, file := guardRepo(t)
	dir := filepath.Dir(file)
	cw, err := newChangeWatch()
	if err != nil {
		t.Fatal(err)
	}
	w := cw.(*notifyWatch)
	defer w.close()
	for _, contents := range []bool{true, false} {
		if _, err := w.add(dir, contents); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.add(file, true); err != nil || len(w.uncovered) != 0 {
		t.Fatalf("file add: %v, %d uncovered", err, len(w.uncovered))
	}
	if err := os.WriteFile(file, []byte("bbbb"), 0600); err != nil {
		t.Fatal(err)
	}
	if changed, err := w.poll(); err != nil || !changed {
		t.Fatalf("write after an ancestor re-add: changed %v, %v", changed, err)
	}
}

// CAL-V0-364, PSR-V0-222: a directory the kernel refuses to watch still fails
// the registration, as UNSUPPORTED_FILESYSTEM naming the allowance, because
// only directory events report entries created or removed at any time.
func TestCALV0364_ChangeGuardRefusesADirectoryBeyondTheInotifyLimit(t *testing.T) {
	refuseDirectoryWatches(t)
	repo, _ := guardRepo(t)
	g, err := WatchChanges(repo)
	if err == nil {
		g.Close()
		t.Fatal("a directory without a watch was accepted")
	}
	if wire.CodeOf(err) != wire.CodeUnsupportedFilesystem || !strings.Contains(err.Error(), "fs.inotify.max_user_watches") {
		t.Fatalf("refusal %v (%s)", err, wire.CodeOf(err))
	}
}

// watchHeadroom is how many more inotify watches the kernel grants this user
// now: it adds watches on distinct files of a scratch directory until the
// kernel refuses with ENOSPC, then closes the instance, which releases them.
// It measures the per-user charge the kernel applies, whatever other
// processes or users hold, and gives up beyond limit.
func watchHeadroom(t *testing.T, scratch string, limit int) int {
	t.Helper()
	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(fd)
	for n := 0; ; n++ {
		if n > limit {
			t.Fatalf("the kernel granted more than %d watches at a limit of %d", n, limit)
		}
		path := filepath.Join(scratch, strconv.Itoa(n))
		if _, err := os.Lstat(path); err != nil {
			if err := os.WriteFile(path, nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := syscall.InotifyAddWatch(fd, path, syscall.IN_ATTRIB); err != nil {
			if errors.Is(err, syscall.ENOSPC) {
				return n
			}
			t.Fatal(err)
		}
	}
}

// guardWatches counts the watches a registration of repo asks for.
func guardWatches(t *testing.T, repo *intent.Repository) int {
	t.Helper()
	original := inotifyAddWatch
	n := 0
	inotifyAddWatch = func(fd int, path string, mask uint32) (int, error) {
		n++
		return original(fd, path, mask)
	}
	defer func() { inotifyAddWatch = original }()
	g, err := WatchChanges(repo)
	if err != nil {
		t.Fatal(err)
	}
	g.Close()
	return n
}

// CAL-V0-364, PSR-V0-222: through the real kernel, an allowance with room for
// exactly the guard's directory watches registers a store of many more files,
// and a write to one of them is reported. The test lowers
// fs.inotify.max_user_watches for the host while it runs and restores it, so
// it is opt-in (CORVINT_TASKS_INOTIFY_LIMIT=1) and needs root. The kernel
// charges watches per user, so the limit is derived from this user's
// measured headroom rather than from a count over every process.
func TestCALV0364_ChangeGuardUnderTheRealInotifyLimit(t *testing.T) {
	if os.Getenv("CORVINT_TASKS_INOTIFY_LIMIT") != "1" {
		t.Skip("set CORVINT_TASKS_INOTIFY_LIMIT=1 to lower fs.inotify.max_user_watches while this test runs")
	}
	const sysctl = "/proc/sys/fs/inotify/max_user_watches"
	raw, err := os.ReadFile(sysctl)
	if err != nil {
		t.Skipf("inotify limit is not readable: %v", err)
	}
	original, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("inotify limit %q", raw)
	}
	repo, file := guardRepo(t)
	dir := filepath.Dir(file)
	for i := 0; i < 60; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%03d", i)), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	watches := guardWatches(t, repo)
	scratch := t.TempDir()
	t.Cleanup(func() { os.WriteFile(sysctl, raw, 0o644) })
	setLimit := func(n int) {
		if err := os.WriteFile(sysctl, []byte(strconv.Itoa(n)), 0o644); err != nil {
			t.Skipf("cannot set the inotify limit: %v", err)
		}
	}
	// The watches this user holds now, from its headroom under a limit small
	// enough to probe. Another process of the user may add or drop watches
	// meanwhile; the limit is re-derived until two probes agree.
	probe := min(original, 1<<16)
	for attempt := 0; ; attempt++ {
		if attempt == 5 {
			t.Skip("this user's inotify watch count kept changing while the test measured it")
		}
		setLimit(probe)
		free := watchHeadroom(t, scratch, probe)
		if free == 0 {
			t.Skipf("this user already holds %d or more watches, beyond what the test probes", probe)
		}
		used := probe - free
		limit := used + watches
		setLimit(limit)
		if h := watchHeadroom(t, scratch, limit); h != watches {
			t.Logf("headroom %d at limit %d, want %d; measuring again", h, limit, watches)
			continue
		}
		t.Logf("this user holds %d watches; limit %d leaves the guard's %d", used, limit, watches)
		break
	}
	g, err := WatchChanges(repo)
	if err != nil {
		t.Fatalf("registration with room for its %d directory watches: %v", watches, err)
	}
	defer g.Close()
	if err := g.Check(); err != nil {
		t.Fatalf("a quiet store: %v", err)
	}
	if err := os.WriteFile(file, []byte("bbbb"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := wire.CodeOf(g.Check()); got != wire.CodeSnapshotMoved {
		t.Fatalf("check after a write = %q", got)
	}
}

// PSR-V0-274: an ancestor directory watch reports the attribute changes of
// every entry in it, so a mode or time change on an entry unrelated to the
// store, including any direct entry of the system temporary directory that
// another process makes, must not invalidate the observation; the ancestor's
// own events and those of its entry on the path to the store still do.
func TestPSRV0274_ChangeGuardIgnoresUnrelatedEntriesOfAnAncestor(t *testing.T) {
	for _, name := range []string{"none", "sibling-chmod", "sibling-dir-chmod", "temp-child-chmod", "temp-child-chtimes", "on-path-chmod", "ancestor-chmod", "ancestor-rename"} {
		t.Run(name, func(t *testing.T) {
			repo, _ := guardRepo(t)
			root := repo.PrimaryWorktree
			g, err := WatchChanges(repo)
			if err != nil {
				t.Fatal(err)
			}
			defer g.Close()
			if err := g.Check(); err != nil {
				t.Fatal(err)
			}
			g.Sweep()
			want := wire.CodeSnapshotMoved
			switch name {
			case "none":
				want = ""
			case "sibling-chmod":
				want = ""
				sibling := filepath.Join(root, "README")
				if err = os.WriteFile(sibling, []byte("x"), 0600); err == nil {
					err = os.Chmod(sibling, 0640)
				}
			case "sibling-dir-chmod":
				want = ""
				sibling := filepath.Join(filepath.Dir(root), "sibling")
				if err = os.Mkdir(sibling, 0700); err == nil {
					err = os.Chmod(sibling, 0750)
				}
			case "temp-child-chmod":
				// As internal/analyzerexec and internal/testsupport do in
				// another test binary of the same CI shard.
				want = ""
				var dir string
				if dir, err = os.MkdirTemp("", "psr274-"); err == nil {
					defer os.RemoveAll(dir)
					err = os.Chmod(dir, 0o700)
				}
			case "temp-child-chtimes":
				want = ""
				var f *os.File
				if f, err = os.CreateTemp("", "psr274-"); err == nil {
					defer os.Remove(f.Name())
					f.Close()
					err = os.Chtimes(f.Name(), time.Unix(0, 0), time.Unix(0, 0))
				}
			case "on-path-chmod":
				err = os.Chmod(filepath.Dir(repo.StateDir), 0750)
			case "ancestor-chmod":
				err = os.Chmod(root, 0750)
			case "ancestor-rename":
				err = os.Rename(root, root+".old")
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := wire.CodeOf(g.CheckEvents()); got != want {
				t.Fatalf("check events after %s = %q, want %q", name, got, want)
			}
			if got := wire.CodeOf(g.Check()); got != want {
				t.Fatalf("check after %s = %q, want %q", name, got, want)
			}
		})
	}
}

// PSR-V0-274: a relevant event queued behind more ignored sibling events than
// one poll reads is still reported; a poll that stops before draining the
// queue reports a change rather than returning clean.
func TestPSRV0274_ChangeGuardReportsAnEventBehindAnIgnoredBacklog(t *testing.T) {
	repo, _ := guardRepo(t)
	root := repo.PrimaryWorktree
	g, err := WatchChanges(repo)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	if err := g.Check(); err != nil {
		t.Fatal(err)
	}
	g.Sweep()
	// Alternate two siblings so the kernel cannot coalesce consecutive
	// identical events; each event is 32 bytes, so 9000 exceed 64 buffers.
	siblings := []string{filepath.Join(root, "SIBA"), filepath.Join(root, "SIBB")}
	for _, s := range siblings {
		if err := os.WriteFile(s, []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := g.CheckEvents(); err != nil {
		t.Fatalf("check events after creating siblings: %v", err)
	}
	for i := 0; i < 9000; i++ {
		if err := os.Chmod(siblings[i%2], os.FileMode(0600|(i%2)<<5)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(filepath.Dir(repo.StateDir), 0750); err != nil {
		t.Fatal(err)
	}
	if got := wire.CodeOf(g.CheckEvents()); got != wire.CodeSnapshotMoved {
		t.Fatalf("check events after a backlog and an on-path chmod = %q", got)
	}
}
