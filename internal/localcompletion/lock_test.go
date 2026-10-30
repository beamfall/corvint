package localcompletion

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestOperationLockFilesystemErrors(t *testing.T) {
	t.Run("LCP-V0-002 permission is not contention", func(t *testing.T) {
		dir, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		repo := &repository{directory: dir}
		if err := os.Chmod(dir, 0500); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(dir, 0700)
		probe := filepath.Join(dir, "probe")
		err = os.Mkdir(probe, 0700)
		if err == nil {
			os.Remove(probe)
			t.Skip("filesystem credentials do not enforce directory permissions")
		}
		if !errors.Is(err, os.ErrPermission) {
			t.Fatal(err)
		}
		unlock, err := repo.lock()
		if unlock != nil || err == nil || err.Error() != "operation-lock-permission-denied" || !errors.Is(err, os.ErrPermission) {
			t.Fatalf("unlock=%v error=%v", unlock != nil, err)
		}
		var pe *os.PathError
		if !errors.As(err, &pe) || pe.Path != filepath.Join(dir, "operation.lock") {
			t.Fatalf("lost path cause: %v", err)
		}
	})
	t.Run("LCP-V0-002 existing lock survives and owner releases", func(t *testing.T) {
		dir, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		repo := &repository{directory: dir}
		unlock, err := repo.lock()
		if err != nil {
			t.Fatal(err)
		}
		second, err := repo.lock()
		if second != nil || err == nil || err.Error() != "operation-in-progress" {
			t.Fatalf("%v", err)
		}
		if _, err := os.Stat(filepath.Join(repo.directory, "operation.lock")); err != nil {
			t.Fatal(err)
		}
		unlock()
		last, err := repo.lock()
		if err != nil {
			t.Fatal(err)
		}
		last()
	})
}

func TestOperationLockFailureCauses(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cause error
		code  string
	}{
		{"permission", os.ErrPermission, "operation-lock-permission-denied"},
		{"parent exists", os.ErrExist, "operation-lock-create-failed"},
		{"storage failure", syscall.ENOSPC, "operation-lock-create-failed"},
	} {
		t.Run("LCP-V0-002 "+tc.name, func(t *testing.T) {
			cause := &os.PathError{Op: "mkdir", Path: "/private/fixture/operation.lock", Err: tc.cause}
			err := operationLockFailure(cause)
			var got *os.PathError
			if err.Error() != tc.code || !errors.Is(err, tc.cause) || !errors.As(err, &got) || got != cause {
				t.Fatalf("code=%q cause=%v", err.Error(), err)
			}
		})
	}
}
