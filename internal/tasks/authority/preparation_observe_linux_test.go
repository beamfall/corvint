//go:build linux

package authority

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// assumeCompleteLockTable lets the counting tests run in a container whose
// PID namespace is not the initial one, where every writer is a test
// process in the same namespace. The real check is used when it passes;
// otherwise the unassumed product read must first report the typed
// NOT_OBSERVED degradation for a held slot.
func assumeCompleteLockTable(t *testing.T) {
	t.Helper()
	if procLocksComplete() == nil {
		return
	}
	requireNotObserved(t, "lock table omits owners outside this PID namespace")
	link := filepath.Join(t.TempDir(), "pid")
	if err := os.Symlink(initialPIDNamespace, link); err != nil {
		t.Fatal(err)
	}
	old := procSelfPIDNamespace
	t.Cleanup(func() { procSelfPIDNamespace = old })
	procSelfPIDNamespace = link
	t.Log("reader is outside the initial PID namespace; table completeness assumed for same-namespace test writers")
}

// requireNotObserved holds one slot and requires the product read, under the
// current lock view, to abstain with reason instead of counting.
func requireNotObserved(t *testing.T, reason string) {
	t.Helper()
	_, repo := preparationOpenFixture(t)
	heldSlot(t, repo.CommonDir, 0, admissionRecord(1))
	q := ObservePreparationQueue(repo)
	if _, ok := q.WouldBeRank(); q.NotObserved != reason || q.Method != "" || q.Registered != 0 || ok {
		t.Fatalf("product read did not report NOT_OBSERVED %q: %+v", reason, q)
	}
}

// btrfsMagic is BTRFS_SUPER_MAGIC from statfs(2).
const btrfsMagic = 0x9123683e

// requireObservableFlocks skips a counting test on a btrfs subvolume, where
// /proc/locks names a test flock under the superblock's s_dev while stat
// reports the subvolume's anonymous device. The product read must then
// report the typed ambiguity, never a count; on any other filesystem a
// refused or missing match fails.
func requireObservableFlocks(t *testing.T) {
	t.Helper()
	f, err := os.Create(filepath.Join(t.TempDir(), "flock-probe"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = withFD(f, flockExclusiveNB); err != nil {
		t.Fatal(err)
	}
	st, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	v, err := loadPreparationLockView()
	if err != nil {
		t.Fatal(err)
	}
	held, err := v.held(f, st)
	if err == nil && held {
		return
	}
	if err == nil {
		t.Fatalf("the reader's own flock is absent from %s", procLocksPath)
	}
	var fs syscall.Statfs_t
	if e := syscall.Fstatfs(int(f.Fd()), &fs); e != nil || uint32(fs.Type) != btrfsMagic {
		t.Fatalf("flock not matched off btrfs (statfs type %#x, %v): %v", fs.Type, e, err)
	}
	sys := st.Sys().(*syscall.Stat_t)
	dev := uint64(sys.Dev)
	requireNotObserved(t, err.Error())
	t.Skipf("flock owners are not observable on this btrfs subvolume: stat reports device %d:%d but %s lists %v for inode %d; product read abstains with %q",
		(dev>>8)&0xfff|(dev>>32)&^uint64(0xfff), dev&0xff|(dev>>12)&^uint64(0xff), procLocksPath, v.(procLocksView).locked[uint64(sys.Ino)], sys.Ino, err)
}

// fOFDSetlk is F_OFD_SETLK, identical on every Linux architecture.
const fOFDSetlk = 37

var recordLockKinds = []string{"posix", "ofd"}

func holdRecordLock(f *os.File, kind string) error {
	cmd := syscall.F_SETLK
	if kind == "ofd" {
		cmd = fOFDSetlk
	}
	return syscall.FcntlFlock(f.Fd(), cmd, &syscall.Flock_t{Type: syscall.F_RDLCK, Start: 100, Len: 1})
}

// Linux record locks do not interact with flock and mark no registration.
func checkRecordLockObservation(t *testing.T, kind string, q PreparationQueue) {
	t.Helper()
	if r, ok := q.WouldBeRank(); q.NotObserved != "" || q.Registered != 0 || !ok || r != 1 {
		t.Fatalf("%s: %+v", kind, q)
	}
}

func TestCALV0095_ProcLocksParse(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "slot")
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	sys := st.Sys().(*syscall.Stat_t)
	dev := uint64(sys.Dev)
	major := (dev>>8)&0xfff | (dev>>32)&^uint64(0xfff)
	minor := dev&0xff | (dev>>12)&^uint64(0xff)
	id := func(maj, min uint64) string {
		return fmt.Sprintf("%02x:%02x:%d", maj, min, sys.Ino)
	}
	for _, c := range []struct {
		name, table string
		held, err   bool
	}{
		{"empty", "", false, false},
		{"granted-flock", "1: FLOCK  ADVISORY  WRITE 42 " + id(major, minor) + " 0 EOF\n", true, false},
		{"blocked-waiter-only", "1: -> FLOCK  ADVISORY  WRITE 42 " + id(major, minor) + " 0 EOF\n", false, false},
		{"lease-ignored", "1: LEASE  ACTIVE    READ  42 " + id(major, minor) + " 0 EOF\n", false, false},
		{"posix-record-lock-ignored", "1: POSIX  ADVISORY  WRITE 42 " + id(major, minor) + " 0 EOF\n", false, false},
		{"ofd-record-lock-ignored", "1: OFDLCK ADVISORY  READ  -1 " + id(major, minor) + " 100 100\n", false, false},
		{"other-device-same-inode", "1: FLOCK  ADVISORY  WRITE 42 " + id(major+1, minor) + " 0 EOF\n", false, true},
	} {
		v, err := parseProcLocks(c.table)
		if err != nil {
			t.Fatal(c.name, err)
		}
		held, err := v.held(nil, st)
		if held != c.held || (err != nil) != c.err {
			t.Errorf("%s: held=%t err=%v", c.name, held, err)
		}
	}
	if _, err := parseProcLocks("1: FLOCK  ADVISORY  WRITE 42 zz:01:5 0 EOF\n"); err == nil {
		t.Error("malformed identity accepted")
	}
}

// TestCALV0095_ProcLocksCompleteness: a table read from a reader outside the
// initial PID namespace, or not visible in the procfs mount, abstains even
// when it lists the slot's flock.
func TestCALV0095_ProcLocksCompleteness(t *testing.T) {
	dir := t.TempDir()
	table := filepath.Join(dir, "locks")
	if err := os.WriteFile(table, []byte("1: FLOCK  ADVISORY  WRITE 42 08:01:5 0 EOF\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTable, oldNS := procLocksPath, procSelfPIDNamespace
	t.Cleanup(func() { procLocksPath, procSelfPIDNamespace = oldTable, oldNS })
	procLocksPath = table
	for _, c := range []struct {
		name, target string
		complete     bool
	}{
		{"initial", initialPIDNamespace, true},
		{"child-namespace", "pid:[4026532999]", false},
		{"not-visible", "", false},
	} {
		procSelfPIDNamespace = filepath.Join(dir, c.name)
		if c.target != "" {
			if err := os.Symlink(c.target, procSelfPIDNamespace); err != nil {
				t.Fatal(err)
			}
		}
		v, err := loadPreparationLockView()
		if c.complete != (err == nil) || (c.complete && len(v.(procLocksView).locked) != 1) {
			t.Errorf("%s: view=%v err=%v", c.name, v, err)
		}
	}
	// Through the real observer: a held slot under an incomplete table is
	// NOT_OBSERVED, never zero.
	procLocksPath = oldTable
	procSelfPIDNamespace = filepath.Join(dir, "child-namespace")
	_, repo := preparationOpenFixture(t)
	heldSlot(t, repo.CommonDir, 0, admissionRecord(3))
	q := ObservePreparationQueue(repo)
	if q.NotObserved != "lock table omits owners outside this PID namespace" || q.Registered != 0 {
		t.Fatalf("%+v", q)
	}
	if _, ok := q.WouldBeRank(); ok {
		t.Fatal("rank reported from an incomplete lock table")
	}
}
