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

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// setFileCeiling fixes the number of regular files a guard watches with
// inotify.
func setFileCeiling(t *testing.T, n int) {
	t.Helper()
	original := inotifyFileCeiling
	inotifyFileCeiling = func() int { return n }
	t.Cleanup(func() { inotifyFileCeiling = original })
}

// failFileWatchesAfter makes the kernel refuse, with ENOSPC, the watch of
// every regular file after the first budget, as it does when the user's
// fs.inotify.max_user_watches allowance is spent. Directories are always
// accepted unless dirs is true.
func failFileWatchesAfter(t *testing.T, budget int, dirs bool) *int {
	t.Helper()
	original := inotifyAddWatch
	files := 0
	inotifyAddWatch = func(fd int, path string, mask uint32) (int, error) {
		info, err := os.Lstat(path)
		if err == nil && (dirs && info.IsDir() || info.Mode().IsRegular() && files >= budget) {
			return -1, syscall.ENOSPC
		}
		if err == nil && info.Mode().IsRegular() {
			files++
		}
		return original(fd, path, mask)
	}
	t.Cleanup(func() { inotifyAddWatch = original })
	return &files
}

// CAL-V0-364: a regular file beyond the per-guard file-watch ceiling, or
// whose inotify watch the kernel refuses, is tracked by its stat identity;
// every mutation class is reported, quiescence is not, and the registration
// does not fail.
func TestCALV0364_ChangeGuardTracksFilesBeyondTheInotifyLimit(t *testing.T) {
	for _, source := range []string{"kernel-refusal", "ceiling"} {
		for _, budget := range []int{0, 2} {
			for _, name := range []string{"none", "write-same-size", "write-resize", "in-place-restored-time", "chmod", "rename-replace", "remove", "sibling"} {
				t.Run(fmt.Sprintf("%s/%d/%s", source, budget, name), func(t *testing.T) {
					if source == "kernel-refusal" {
						failFileWatchesAfter(t, budget, false)
					} else {
						setFileCeiling(t, budget)
					}
					repo, file := guardRepo(t)
					dir := filepath.Dir(file)
					for i := 0; i < 4; i++ {
						if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%03d", i)), []byte("x"), 0600); err != nil {
							t.Fatal(err)
						}
					}
					g, err := WatchChanges(repo)
					if err != nil {
						t.Fatalf("registration refused when the watch allowance ran out: %v", err)
					}
					w := g.watch.(*notifyWatch)
					// x.json sorts after 000..003, so it is over the limit in both cases.
					if len(w.stats) != 5-budget || w.stats[len(w.stats)-1].path != file {
						t.Fatalf("%d files tracked by stat, last %v", len(w.stats), w.stats)
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
					case "chmod":
						err = os.Chmod(file, 0640)
					case "rename-replace":
						err = os.WriteFile(file+".tmp", []byte("aaaa"), 0600)
						if err == nil {
							err = os.Rename(file+".tmp", file)
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
					if got := wire.CodeOf(g.Check()); got != want {
						t.Fatalf("check = %q, want %q", got, want)
					}
					if name != "sibling" && name != "none" {
						// The stat identity alone must see it, without the inotify event.
						w.dirty = false
						var buf [4096]byte
						for {
							if n, err := syscall.Read(w.fd, buf[:]); err != nil || n == 0 {
								break
							}
						}
						if !w.sweep() {
							t.Fatalf("stat identity missed %s", name)
						}
					}
					if err := g.Close(); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

// CAL-V0-364: the ceiling is a 32nd of the user's watch allowance, at most
// 4,096, and 256 when the allowance cannot be read.
func TestCALV0364_FileCeilingFollowsTheAllowance(t *testing.T) {
	original := inotifyWatchesLimit
	t.Cleanup(func() { inotifyWatchesLimit = original })
	for _, tc := range []struct {
		content string
		want    int
	}{{"8192\n", 256}, {"130016\n", 4063}, {"1048576\n", 4096}, {"64\n", 2}, {"garbage", 256}, {"0", 256}} {
		path := filepath.Join(t.TempDir(), "max_user_watches")
		if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
			t.Fatal(err)
		}
		inotifyWatchesLimit = path
		if got := defaultInotifyFileCeiling(); got != tc.want {
			t.Errorf("allowance %q: ceiling %d, want %d", strings.TrimSpace(tc.content), got, tc.want)
		}
	}
	inotifyWatchesLimit = filepath.Join(t.TempDir(), "absent")
	if got := defaultInotifyFileCeiling(); got != 256 {
		t.Errorf("unreadable allowance: ceiling %d, want 256", got)
	}
}

// CAL-V0-364: a directory the kernel refuses to watch still fails the
// registration, as UNSUPPORTED_FILESYSTEM naming the allowance, because
// only directory events report entries created or removed at any time.
func TestCALV0364_ChangeGuardRefusesADirectoryBeyondTheInotifyLimit(t *testing.T) {
	failFileWatchesAfter(t, 1<<30, true)
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

// CAL-V0-364: regular files register after every directory, so an allowance
// that holds exactly the directories is spent on their events and each file,
// including one that sorts before a directory still to be visited, is tracked
// by stat.
func TestCALV0364_DirectoriesRegisterBeforeFiles(t *testing.T) {
	repo, file := guardRepo(t)
	// A file that sorts before the directory z below it in the state directory.
	for _, path := range []string{filepath.Join(repo.StateDir, "a.json"), filepath.Join(repo.StateDir, "z", "b.json"), file} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	original := inotifyAddWatch
	t.Cleanup(func() { inotifyAddWatch = original })
	dirs, allowed := 0, -1 // -1: no limit
	inotifyAddWatch = func(fd int, path string, mask uint32) (int, error) {
		if info, err := os.Lstat(path); err == nil && info.IsDir() {
			dirs++
		}
		if allowed == 0 {
			return -1, syscall.ENOSPC
		}
		if allowed > 0 {
			allowed--
		}
		return original(fd, path, mask)
	}
	// Count the directories with no limit, then allow exactly them.
	g, err := WatchChanges(repo)
	if err != nil {
		t.Fatal(err)
	}
	g.Close()
	dirs, allowed = 0, dirs
	g, err = WatchChanges(repo)
	if err != nil {
		t.Fatalf("an allowance that holds every directory refused the registration: %v", err)
	}
	defer g.Close()
	if n := len(g.watch.(*notifyWatch).stats); n != 3 {
		t.Fatalf("%d files tracked by stat, want the 3 files", n)
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

// guardWatches counts the directory and regular-file watches a registration
// of repo asks for, with no allowance or ceiling in its way.
func guardWatches(t *testing.T, repo *intent.Repository) (dirs, files int) {
	t.Helper()
	original := inotifyAddWatch
	inotifyAddWatch = func(fd int, path string, mask uint32) (int, error) {
		if info, err := os.Lstat(path); err == nil && info.IsDir() {
			dirs++
		} else if err == nil && info.Mode().IsRegular() {
			files++
		}
		return original(fd, path, mask)
	}
	defer func() { inotifyAddWatch = original }()
	setFileCeiling(t, 1<<30)
	g, err := WatchChanges(repo)
	if err != nil {
		t.Fatal(err)
	}
	g.Close()
	return dirs, files
}

// recordFileRefusals counts the regular-file watches the real kernel refuses
// with ENOSPC.
func recordFileRefusals(t *testing.T) *int {
	t.Helper()
	original := inotifyAddWatch
	refused := 0
	inotifyAddWatch = func(fd int, path string, mask uint32) (int, error) {
		wd, err := original(fd, path, mask)
		if errors.Is(err, syscall.ENOSPC) {
			if info, lerr := os.Lstat(path); lerr == nil && info.Mode().IsRegular() {
				refused++
			}
		}
		return wd, err
	}
	t.Cleanup(func() { inotifyAddWatch = original })
	return &refused
}

// checkGuardSeesAWrite requires a quiet registration to pass and a write to
// the stat-tracked file to be reported.
func checkGuardSeesAWrite(t *testing.T, g *ChangeGuard, file string) {
	t.Helper()
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

// CAL-V0-364: the same through the real kernel. The kernel-refusal case
// lowers fs.inotify.max_user_watches for the host while it runs and restores
// it, so the test is opt-in (CORVINT_TASKS_INOTIFY_LIMIT=1) and needs root.
// The kernel charges watches per user, so the limit is derived from this
// user's measured headroom rather than from a count over every process, and
// each case asserts that its own refusal, and only it, happened.
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
	dirs, files := guardWatches(t, repo)
	const granted = 8 // the regular files the kernel-refusal case lets the guard watch
	if files <= granted {
		t.Fatalf("%d regular files, want more than %d", files, granted)
	}

	t.Run("ceiling", func(t *testing.T) {
		// The host's own allowance, which the ceiling keeps the guard far below.
		setFileCeiling(t, granted)
		refused := recordFileRefusals(t)
		g, err := WatchChanges(repo)
		if err != nil {
			t.Fatalf("registration under a ceiling of %d files: %v", granted, err)
		}
		defer g.Close()
		if *refused != 0 {
			t.Fatalf("the kernel refused %d file watches, so the ceiling was not what bound the guard", *refused)
		}
		if n := len(g.watch.(*notifyWatch).stats); n != files-granted {
			t.Fatalf("%d files tracked by stat, want the %d beyond the ceiling", n, files-granted)
		}
		checkGuardSeesAWrite(t, g, file)
	})

	t.Run("kernel-refusal", func(t *testing.T) {
		setFileCeiling(t, 1<<30) // the ceiling never binds: only the kernel refuses
		scratch := t.TempDir()
		t.Cleanup(func() { os.WriteFile(sysctl, raw, 0o644) })
		setLimit := func(n int) {
			if err := os.WriteFile(sysctl, []byte(strconv.Itoa(n)), 0o644); err != nil {
				t.Skipf("cannot set the inotify limit: %v", err)
			}
		}
		// The watches this user holds now, from its headroom under a limit
		// small enough to probe. Another process of the user may add or drop
		// watches meanwhile; the limit is re-derived until two probes agree.
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
			limit := used + dirs + granted
			setLimit(limit)
			if h := watchHeadroom(t, scratch, limit); h != dirs+granted {
				t.Logf("headroom %d at limit %d, want %d; measuring again", h, limit, dirs+granted)
				continue
			}
			t.Logf("this user holds %d watches; limit %d leaves %d directories and %d files", used, limit, dirs, granted)
			break
		}
		refused := recordFileRefusals(t)
		g, err := WatchChanges(repo)
		if err != nil {
			t.Fatalf("registration with room for every directory and %d files: %v", granted, err)
		}
		defer g.Close()
		if *refused == 0 {
			t.Fatal("the kernel refused no file watch, so its exhaustion was not exercised")
		}
		if n := len(g.watch.(*notifyWatch).stats); n != *refused {
			t.Fatalf("%d files tracked by stat, want the %d the kernel refused", n, *refused)
		}
		t.Logf("the kernel refused %d of %d file watches with ENOSPC", *refused, files)
		checkGuardSeesAWrite(t, g, file)
	})
}
