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
// process in the same namespace. The real check is used when it passes.
func assumeCompleteLockTable(t *testing.T) {
	t.Helper()
	if procLocksComplete() == nil {
		return
	}
	link := filepath.Join(t.TempDir(), "pid")
	if err := os.Symlink(initialPIDNamespace, link); err != nil {
		t.Fatal(err)
	}
	old := procSelfPIDNamespace
	t.Cleanup(func() { procSelfPIDNamespace = old })
	procSelfPIDNamespace = link
	t.Log("reader is outside the initial PID namespace; table completeness assumed for same-namespace test writers")
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
