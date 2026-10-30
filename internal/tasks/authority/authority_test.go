//go:build darwin || linux

package authority_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func code(err error) string { return wire.CodeOf(err) }

// resolved builds a fixture repository and resolves it exactly as a
// mutating command would (intent.Resolve, never a hand-built Repository).
func resolved(t *testing.T) (*fixture.Repo, *intent.Repository) {
	t.Helper()
	r := fixture.TempRepo(t)
	repo, err := intent.Resolve(r.Root)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if repo.LockPath != filepath.Join(r.CommonDir, authority.LockFileName) {
		t.Fatalf("lock path %s", repo.LockPath)
	}
	return r, repo
}

// leftovers lists probe/temp files under dir; the primitives must leave none.
func leftovers(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir %s: %v", dir, err)
	}
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".taskman-qualify-") || strings.Contains(e.Name(), ".tmp-") {
			out = append(out, e.Name())
		}
	}
	return out
}

// regularEntries keeps the regular-file entries of a tree snapshot.
func regularEntries(in []fixture.Entry) []fixture.Entry {
	var out []fixture.Entry
	for _, e := range in {
		if e.Mode.IsRegular() {
			out = append(out, e)
		}
	}
	return out
}

// TestTMV0009_AS29_LockContentionAcrossDescriptors: two independent open
// file descriptions on taskman.lock exclude each other; the second waits
// its bounded time and fails LOCK_TIMEOUT holding nothing; after the first
// releases, the same caller reacquires, and the first can reacquire again.
func TestTMV0009_AS29_LockContentionAcrossDescriptors(t *testing.T) {
	_, repo := resolved(t)
	ctx := context.Background()
	a, err := authority.AcquireLock(ctx, repo, authority.LockOptions{})
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if a.Path() != repo.LockPath {
		t.Errorf("path %s", a.Path())
	}
	start := time.Now()
	b, err := authority.AcquireLock(ctx, repo, authority.LockOptions{Wait: 150 * time.Millisecond, Poll: 5 * time.Millisecond})
	elapsed := time.Since(start)
	if b != nil || code(err) != wire.CodeLockTimeout {
		t.Fatalf("contended acquire: lock=%v err=%v", b, err)
	}
	if elapsed < 150*time.Millisecond {
		t.Errorf("returned after %v, before the bounded wait elapsed", elapsed)
	}
	if elapsed > 5*time.Second {
		t.Errorf("returned after %v, far beyond the bounded wait", elapsed)
	}
	if err := a.Close(); err != nil {
		t.Fatalf("release: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Errorf("second Close must be a no-op: %v", err)
	}
	b, err = authority.AcquireLock(ctx, repo, authority.LockOptions{Wait: 2 * time.Second})
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	// While b holds, a fresh descriptor is still excluded.
	if c, err := authority.AcquireLock(ctx, repo, authority.LockOptions{Wait: 50 * time.Millisecond, Poll: 5 * time.Millisecond}); err == nil {
		c.Close()
		t.Fatalf("third descriptor acquired while the second holds")
	} else if code(err) != wire.CodeLockTimeout {
		t.Fatalf("third descriptor: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("release b: %v", err)
	}
	a, err = authority.AcquireLock(ctx, repo, authority.LockOptions{Wait: 2 * time.Second})
	if err != nil {
		t.Fatalf("reacquire by the first caller: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Fatalf("release: %v", err)
	}
	if left := leftovers(t, repo.CommonDir); len(left) != 0 {
		t.Errorf("leftover files: %v", left)
	}
}

// TestTMV0009_AS29_LockCancellationHoldsNothing: a cancelled or expired
// context returns ctx.Err() and the failed caller holds no lock (a later
// acquisition succeeds as soon as the real holder releases); the context
// wins over a longer configured wait.
func TestTMV0009_AS29_LockCancellationHoldsNothing(t *testing.T) {
	_, repo := resolved(t)
	holder, err := authority.AcquireLock(context.Background(), repo, authority.LockOptions{})
	if err != nil {
		t.Fatalf("holder: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	l, err := authority.AcquireLock(ctx, repo, authority.LockOptions{Wait: 10 * time.Second, Poll: 5 * time.Millisecond})
	if l != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled acquire: lock=%v err=%v", l, err)
	}
	if time.Since(start) > 3*time.Second {
		t.Errorf("cancellation took %v", time.Since(start))
	}
	dctx, dcancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer dcancel()
	l, err = authority.AcquireLock(dctx, repo, authority.LockOptions{Wait: 10 * time.Second, Poll: 5 * time.Millisecond})
	if l != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline acquire: lock=%v err=%v", l, err)
	}
	// An already-done context never opens the file.
	done, dc := context.WithCancel(context.Background())
	dc()
	if l, err := authority.AcquireLock(done, repo, authority.LockOptions{}); l != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-cancelled acquire: lock=%v err=%v", l, err)
	}
	if err := holder.Close(); err != nil {
		t.Fatalf("release holder: %v", err)
	}
	// Nothing lingers from the failed callers: the lock is free now.
	next, err := authority.AcquireLock(context.Background(), repo, authority.LockOptions{Wait: 300 * time.Millisecond, Poll: 5 * time.Millisecond})
	if err != nil {
		t.Fatalf("acquire after failed callers: %v", err)
	}
	if err := next.Close(); err != nil {
		t.Fatalf("release: %v", err)
	}
}

// TestTMV0009_LockWaitClampedToFrozenLimit: §1 lock wait is 30 s by default
// and at most.
func TestTMV0009_LockWaitClampedToFrozenLimit(t *testing.T) {
	if authority.DefaultLockWait != 30*time.Second || authority.MaxLockWait != 30*time.Second {
		t.Fatalf("frozen lock wait changed: %v / %v", authority.DefaultLockWait, authority.MaxLockWait)
	}
	cases := map[time.Duration]time.Duration{
		0:                 30 * time.Second,
		-time.Second:      30 * time.Second,
		time.Second:       time.Second,
		30 * time.Second:  30 * time.Second,
		31 * time.Second:  30 * time.Second,
		10 * time.Minute:  30 * time.Second,
		time.Millisecond:  time.Millisecond,
		29 * time.Second:  29 * time.Second,
		300 * time.Second: 30 * time.Second,
	}
	for in, want := range cases {
		if got := authority.EffectiveWait(in); got != want {
			t.Errorf("EffectiveWait(%v) = %v, want %v", in, got, want)
		}
	}
}

// TestTMV0001_AS29_LockRefusesSymlinkNonRegularAndDrift: a symlinked lock
// file, a FIFO or directory at the lock name, a hand-altered lock path and
// a common dir that re-resolves elsewhere are all refused before any flock.
func TestTMV0001_AS29_LockRefusesSymlinkNonRegularAndDrift(t *testing.T) {
	ctx := context.Background()
	t.Run("symlink", func(t *testing.T) {
		r, repo := resolved(t)
		target := filepath.Join(r.Root, "elsewhere.lock")
		if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, repo.LockPath); err != nil {
			t.Fatal(err)
		}
		l, err := authority.AcquireLock(ctx, repo, authority.LockOptions{})
		if l != nil || code(err) != wire.CodeUnsupportedFilesystem || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("symlinked lock: lock=%v err=%v", l, err)
		}
	})
	t.Run("fifo", func(t *testing.T) {
		_, repo := resolved(t)
		if err := syscall.Mkfifo(repo.LockPath, 0o600); err != nil {
			t.Skipf("mkfifo: %v", err)
		}
		l, err := authority.AcquireLock(ctx, repo, authority.LockOptions{})
		if l != nil || code(err) != wire.CodeUnsupportedFilesystem || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("fifo lock: lock=%v err=%v", l, err)
		}
	})
	t.Run("directory", func(t *testing.T) {
		_, repo := resolved(t)
		if err := os.Mkdir(repo.LockPath, 0o755); err != nil {
			t.Fatal(err)
		}
		l, err := authority.AcquireLock(ctx, repo, authority.LockOptions{})
		if l != nil || code(err) != wire.CodeUnsupportedFilesystem {
			t.Fatalf("directory lock: lock=%v err=%v", l, err)
		}
	})
	t.Run("lock path drift", func(t *testing.T) {
		r, repo := resolved(t)
		drift := *repo
		drift.LockPath = filepath.Join(r.Root, "taskman.lock")
		l, err := authority.AcquireLock(ctx, &drift, authority.LockOptions{})
		if l != nil || code(err) != wire.CodeUnsupportedFilesystem || !strings.Contains(err.Error(), "drift") {
			t.Fatalf("lock path drift: lock=%v err=%v", l, err)
		}
		if _, err := os.Lstat(drift.LockPath); !os.IsNotExist(err) {
			t.Errorf("a refused acquisition created %s", drift.LockPath)
		}
	})
	t.Run("common dir drift", func(t *testing.T) {
		r, repo := resolved(t)
		other := fixture.TempRepo(t)
		drift := *repo
		drift.CommonDir = other.CommonDir
		drift.LockPath = filepath.Join(other.CommonDir, authority.LockFileName)
		l, err := authority.AcquireLock(ctx, &drift, authority.LockOptions{})
		if l != nil || code(err) != wire.CodeUnsupportedFilesystem || !strings.Contains(err.Error(), "drift") {
			t.Fatalf("common dir drift: lock=%v err=%v", l, err)
		}
		if _, err := os.Lstat(drift.LockPath); !os.IsNotExist(err) {
			t.Errorf("a refused acquisition created %s", drift.LockPath)
		}
		if _, err := os.Lstat(filepath.Join(r.CommonDir, authority.LockFileName)); !os.IsNotExist(err) {
			t.Errorf("a refused acquisition created the real lock file")
		}
	})
	t.Run("symlinked common dir", func(t *testing.T) {
		r := fixture.TempRepo(t)
		link := filepath.Join(filepath.Dir(r.Root), "linked")
		if err := os.Symlink(r.Root, link); err != nil {
			t.Fatal(err)
		}
		if _, err := intent.Resolve(link); code(err) != wire.CodeUnsupportedFilesystem {
			t.Fatalf("Resolve through a symlink: %v", err)
		}
	})
}

// TestTMV0009_LockFileNeverTruncatedOrReadAsProof: whatever bytes the lock
// file holds survive acquire/release untouched; an absent file is created
// (a mutating command may) and left empty.
func TestTMV0009_LockFileNeverTruncatedOrReadAsProof(t *testing.T) {
	_, repo := resolved(t)
	if _, err := os.Lstat(repo.LockPath); !os.IsNotExist(err) {
		t.Fatalf("fixture must start without a lock file: %v", err)
	}
	l, err := authority.AcquireLock(context.Background(), repo, authority.LockOptions{})
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(repo.LockPath); err != nil || len(raw) != 0 {
		t.Fatalf("created lock file: %q %v", raw, err)
	}
	want := []byte("owner: nobody; this is not proof\n")
	if err := os.WriteFile(repo.LockPath, want, 0o644); err != nil {
		t.Fatal(err)
	}
	l, err = authority.AcquireLock(context.Background(), repo, authority.LockOptions{})
	if err != nil {
		t.Fatalf("acquire over content: %v", err)
	}
	got, err := os.ReadFile(repo.LockPath)
	if err != nil || string(got) != string(want) {
		t.Fatalf("lock file content while held: %q %v", got, err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	got, err = os.ReadFile(repo.LockPath)
	if err != nil || string(got) != string(want) {
		t.Fatalf("lock file content after release: %q %v", got, err)
	}
}

// TestTMV0010_AS29_HostFilesystemQualification: the actual host either
// qualifies (allowed type, full sync, flock and directory sync all observed)
// or is refused with an explicit UNSUPPORTED_FILESYSTEM naming the observed
// type. Either way the probe leaves nothing behind and touches no other
// file.
func TestTMV0010_AS29_HostFilesystemQualification(t *testing.T) {
	r, repo := resolved(t)
	marker := filepath.Join(r.CommonDir, "HEAD")
	if err := os.WriteFile(marker, []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := regularEntries(fixture.TreeSnapshot(t, r.CommonDir))
	q, err := authority.Qualify(repo.CommonDir)
	if q == nil {
		t.Fatalf("Qualify returned no result (err=%v)", err)
	}
	if left := leftovers(t, r.CommonDir); len(left) != 0 {
		t.Errorf("probe left files: %v", left)
	}
	// The directory's own mtime legitimately moves (a probe file was created
	// and removed); every pre-existing file must be byte-, mode- and
	// mtime-identical and no file may have appeared.
	after := regularEntries(fixture.TreeSnapshot(t, r.CommonDir))
	if !fixture.SameTree(before, after) {
		t.Errorf("qualification changed user content:\nbefore %+v\nafter  %+v", before, after)
	}
	if err == nil {
		t.Logf("host qualified: %s %q local=%v", q.Filesystem.Platform, q.Filesystem.Type, q.Filesystem.Local)
		if q.Filesystem.Platform != runtime.GOOS {
			t.Errorf("platform %q", q.Filesystem.Platform)
		}
		if !authority.Classify(q.Filesystem.Platform, q.Filesystem.Type) || !q.Allowed {
			t.Errorf("qualified on a type outside the allowlist: %+v", q)
		}
		if !q.FullSync || !q.Flock || !q.DirSync || !q.Filesystem.Local {
			t.Errorf("qualified without observing every step: %+v", q)
		}
		if q.ProbeName == "" || !strings.HasPrefix(q.ProbeName, ".taskman-qualify-") {
			t.Errorf("probe name %q", q.ProbeName)
		}
		return
	}
	if code(err) != wire.CodeUnsupportedFilesystem {
		t.Fatalf("host refused with code %s: %v", code(err), err)
	}
	if !strings.Contains(err.Error(), q.Filesystem.Type) && q.Filesystem.Type != "" {
		t.Errorf("refusal does not name the observed type %q: %v", q.Filesystem.Type, err)
	}
	t.Logf("host NOT qualified (explicit): %v; observation %+v", err, q)
}

// TestTMV0010_AS29_FilesystemClassificationTable: the §5.1 allowlist and
// representative refused types on each platform, plus unsupported platforms.
func TestTMV0010_AS29_FilesystemClassificationTable(t *testing.T) {
	cases := []struct {
		platform, fstype string
		want             bool
	}{
		{"darwin", "apfs", true}, {"darwin", "hfs", true},
		{"darwin", "nfs", false}, {"darwin", "smbfs", false}, {"darwin", "exfat", false},
		{"darwin", "msdos", false}, {"darwin", "osxfuse", false}, {"darwin", "macfuse", false},
		{"darwin", "APFS", false}, {"darwin", "", false}, {"darwin", "ext4", false},
		{"linux", "ext4", true}, {"linux", "xfs", true}, {"linux", "btrfs", true}, {"linux", "tmpfs", true},
		{"linux", "nfs", false}, {"linux", "fuse", false}, {"linux", "overlay", false}, {"linux", "9p", false},
		{"linux", "vfat", false}, {"linux", "cifs", false}, {"linux", "magic:0x6969", false},
		{"linux", "apfs", false}, {"linux", "", false},
		{"windows", "ntfs", false}, {"freebsd", "ufs", false}, {"", "apfs", false}, {"", "", false},
	}
	for _, c := range cases {
		if got := authority.Classify(c.platform, c.fstype); got != c.want {
			t.Errorf("Classify(%q,%q) = %v, want %v", c.platform, c.fstype, got, c.want)
		}
	}
}

// TestTMV0010_QualifyRefusesSymlinkRelativeOrFileRoot: a symlinked, relative
// or non-directory root is refused before anything is written.
func TestTMV0010_QualifyRefusesSymlinkRelativeOrFileRoot(t *testing.T) {
	r := fixture.TempRepo(t)
	link := filepath.Join(filepath.Dir(r.Root), "root-link")
	if err := os.Symlink(r.CommonDir, link); err != nil {
		t.Fatal(err)
	}
	if q, err := authority.Qualify(link); code(err) != wire.CodeUnsupportedFilesystem || (q != nil && q.ProbeName != "") {
		t.Fatalf("symlinked root: %v %+v", err, q)
	}
	if left := leftovers(t, r.CommonDir); len(left) != 0 {
		t.Errorf("probe wrote through the symlink: %v", left)
	}
	if _, err := authority.Qualify("relative/dir"); code(err) != wire.CodeUnsupportedFilesystem {
		t.Fatalf("relative root: %v", err)
	}
	file := filepath.Join(r.Root, "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.Qualify(file); code(err) != wire.CodeUnsupportedFilesystem {
		t.Fatalf("file root: %v", err)
	}
	if _, err := authority.Qualify(filepath.Join(r.Root, "absent")); code(err) != wire.CodeUnsupportedFilesystem {
		t.Fatalf("absent root: %v", err)
	}
}

func openReceipts(t *testing.T, r *fixture.Repo, repo *intent.Repository) *authority.Dir {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(r.StateDir, "receipts"), 0o755); err != nil {
		t.Fatal(err)
	}
	d, err := authority.OpenDir(repo, "receipts")
	if err != nil {
		t.Fatalf("OpenDir: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// TestTMV0009_AS36_N4c_LinkInNeverReplacesDestination: the commit primitive
// publishes exactly once; a second link-in of the same name reports
// ErrExists, leaves the original bytes untouched and removes its temp.
func TestTMV0009_AS36_N4c_LinkInNeverReplacesDestination(t *testing.T) {
	r, repo := resolved(t)
	d := openReceipts(t, r, repo)
	first := []byte("{\"seq\":\"1\"}\n")
	if err := d.LinkIn("1.json", first); err != nil {
		t.Fatalf("first link-in: %v", err)
	}
	dest := filepath.Join(d.Path(), "1.json")
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != string(first) {
		t.Fatalf("published bytes %q %v", got, err)
	}
	fi, err := os.Lstat(dest)
	if err != nil || !fi.Mode().IsRegular() {
		t.Fatalf("destination %v %v", fi, err)
	}
	err = d.LinkIn("1.json", []byte("{\"seq\":\"1\",\"forged\":true}\n"))
	if !errors.Is(err, authority.ErrExists) {
		t.Fatalf("second link-in: %v", err)
	}
	if code(err) == wire.CodeJournalForked {
		t.Errorf("ErrExists must not be pre-mapped to a §11 code; the caller maps it")
	}
	got, err = os.ReadFile(dest)
	if err != nil || string(got) != string(first) {
		t.Fatalf("destination changed: %q %v", got, err)
	}
	if left := leftovers(t, d.Path()); len(left) != 0 {
		t.Errorf("temp left after ErrExists: %v", left)
	}
	entries, _ := os.ReadDir(d.Path())
	if len(entries) != 1 {
		t.Errorf("receipts dir has %d entries, want 1", len(entries))
	}
}

// TestTMV0009_LinkInPublishesCompleteFilesAndValidatesInput: a successful
// link-in leaves only the destination; names and sizes are bounded.
func TestTMV0009_LinkInPublishesCompleteFilesAndValidatesInput(t *testing.T) {
	r, repo := resolved(t)
	d := openReceipts(t, r, repo)
	for _, name := range []string{"2.json", "key.boot", "key.ack"} {
		if err := d.LinkIn(name, []byte("{}\n")); err != nil {
			t.Fatalf("link-in %s: %v", name, err)
		}
	}
	entries, err := os.ReadDir(d.Path())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Errorf("entries %d", len(entries))
	}
	if left := leftovers(t, d.Path()); len(left) != 0 {
		t.Errorf("temp left: %v", left)
	}
	bad := []string{"", ".", "..", "a/b", "a\\b", "x\x00y", "3.json.tmp-1", strings.Repeat("n", 256)}
	for _, name := range bad {
		if err := d.LinkIn(name, []byte("x")); err == nil || errors.Is(err, authority.ErrExists) {
			t.Errorf("name %q accepted: %v", name, err)
		}
	}
	if err := d.LinkIn("empty.json", nil); code(err) != wire.CodeMalformed {
		t.Errorf("empty publication: %v", err)
	}
	if err := d.LinkIn("huge.json", make([]byte, authority.MaxLinkInBytes+1)); code(err) != wire.CodeLimitExceeded {
		t.Errorf("oversize publication: %v", err)
	}
	entries, _ = os.ReadDir(d.Path())
	if len(entries) != 3 {
		t.Errorf("refused publications wrote something: %d entries", len(entries))
	}
	if err := d.Sync(); err != nil {
		t.Errorf("Dir.Sync: %v", err)
	}
	f, err := os.Open(filepath.Join(d.Path(), "2.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := authority.Fsync(f); err != nil {
		t.Errorf("Fsync on a published file: %v", err)
	}
	if err := authority.Fsync(nil); err == nil {
		t.Errorf("Fsync(nil) accepted")
	}
}

// TestTMV0001_OpenDirRefusesSymlinkEscapeAndUninitialized: publication is
// confined to the state dir; symlinked components, escapes, dot components
// and an absent state dir are refused.
func TestTMV0001_OpenDirRefusesSymlinkEscapeAndUninitialized(t *testing.T) {
	r, repo := resolved(t)
	if _, err := authority.OpenDir(repo, "receipts"); code(err) != wire.CodeUninitialized {
		t.Fatalf("absent state dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(r.StateDir, "effects"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := fixture.TempDirOutside(t)
	if err := os.Symlink(outside, filepath.Join(r.StateDir, "receipts")); err != nil {
		t.Fatal(err)
	}
	if d, err := authority.OpenDir(repo, "receipts"); err == nil || code(err) != wire.CodeUnsupportedFilesystem {
		if d != nil {
			d.Close()
		}
		t.Fatalf("symlinked receipts: %v", err)
	}
	for _, rel := range []string{"..", "../taskman", "/tmp", "effects/../effects", ".hidden", "effects/", "absent"} {
		if d, err := authority.OpenDir(repo, rel); err == nil {
			d.Close()
			t.Errorf("rel %q accepted", rel)
		}
	}
	d, err := authority.OpenDir(repo, "effects")
	if err != nil {
		t.Fatalf("effects: %v", err)
	}
	defer d.Close()
	if err := d.LinkIn("k.boot", []byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(outside, "k.boot")); !os.IsNotExist(err) {
		t.Errorf("published outside the state dir")
	}
	root, err := authority.OpenDir(repo, "")
	if err != nil {
		t.Fatalf("state dir itself: %v", err)
	}
	defer root.Close()
	if root.Path() != r.StateDir {
		t.Errorf("root path %s", root.Path())
	}
	drift := *repo
	drift.StateDir = filepath.Join(r.Root, "taskman")
	if d, err := authority.OpenDir(&drift, ""); err == nil || code(err) != wire.CodeUnsupportedFilesystem {
		if d != nil {
			d.Close()
		}
		t.Fatalf("state dir drift: %v", err)
	}
}

// TestPrerequisitesAreExplicit: the integration holds are named, unique and
// never phrased as done.
func TestPrerequisitesAreExplicit(t *testing.T) {
	seen := map[string]bool{}
	want := map[string]bool{"AUTH-ACTOR-BINDING": false, "AUTH-CAPACITY-ESCROW": false, "AUTH-TRANSACTION-WRITER": false}
	for _, p := range authority.Prerequisites {
		if p.ID == "" || p.Owner == "" || p.Statement == "" {
			t.Errorf("incomplete prerequisite %+v", p)
		}
		if seen[p.ID] {
			t.Errorf("duplicate prerequisite %s", p.ID)
		}
		seen[p.ID] = true
		if _, ok := want[p.ID]; ok {
			want[p.ID] = true
		}
		if strings.Contains(strings.ToLower(p.Statement), "is implemented") {
			t.Errorf("prerequisite %s claims implementation", p.ID)
		}
	}
	for id, ok := range want {
		if !ok {
			t.Errorf("prerequisite %s missing", id)
		}
	}
}
