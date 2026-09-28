//go:build darwin

package authority

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// CAL-V0-026: partial kqueue registration releases descriptors on exhaustion.
func TestCALV0026_ChangeGuardDescriptorExhaustion(t *testing.T) {
	repo, file := guardRepo(t)
	for i := 0; i < 100; i++ {
		if err := os.WriteFile(filepath.Join(filepath.Dir(file), fmt.Sprintf("%03d", i)), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var original syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &original); err != nil {
		t.Fatal(err)
	}
	defer syscall.Setrlimit(syscall.RLIMIT_NOFILE, &original)
	before, err := os.ReadDir("/dev/fd")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		limited := original
		limited.Cur = min(original.Cur, 64)
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &limited); err != nil {
			t.Fatal(err)
		}
		g, watchErr := WatchChanges(repo)
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &original); err != nil {
			t.Fatal(err)
		}
		if watchErr == nil {
			g.Close()
			t.Fatal("descriptor exhaustion did not refuse")
		}
	}
	after, err := os.ReadDir("/dev/fd")
	if err != nil {
		t.Fatal(err)
	}
	if len(after) > len(before) {
		t.Fatalf("descriptors leaked: before %d after %d", len(before), len(after))
	}
}

func TestCALV0026_RegistrationSeesMembershipChange(t *testing.T) {
	repo, file := guardRepo(t)
	w, err := newChangeWatch()
	if err != nil {
		t.Fatal(err)
	}
	defer w.close()
	if _, err := w.add(filepath.Dir(file), true); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file+".new", []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := w.add(repo.StateDir, true); err != nil {
		t.Fatal(err)
	}
	changed, err := w.changed()
	if err != nil || !changed {
		t.Fatalf("registration gap accepted: %v %v", changed, err)
	}
}
