package dogfoodoperation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSharedOperationCapabilityAndContention(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	held, unlock, err := Acquire(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if _, release, err := Acquire(context.Background(), dir); err == nil || release != nil {
		t.Fatal("second writer accepted")
	}
	nested, release, err := Acquire(held, dir)
	if err != nil {
		t.Fatal(err)
	}
	release()
	if _, err := os.Stat(filepath.Join(dir, "corvint", "local-completion", "operation.lock")); err != nil {
		t.Fatal("borrow released owner", err)
	}
	other, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, otherRelease, err := Acquire(nested, other)
	if err != nil {
		t.Fatal(err)
	}
	otherRelease()
	unlock()
	_, nextRelease, err := Acquire(nested, dir)
	if err != nil {
		t.Fatal(err)
	}
	nextRelease()
}

func TestCleanupHoldRetainsOperationAndRefusesEveryBorrow(t *testing.T) {
	for _, invalidDiagnostic := range []bool{false, true} {
		t.Run(map[bool]string{false: "diagnostic", true: "diagnostic-failure"}[invalidDiagnostic], func(t *testing.T) {
			dir, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			ctx, release, err := Acquire(context.Background(), dir)
			if err != nil {
				t.Fatal(err)
			}
			diagnostic := []byte(`{"state":"HOLD","leader":123}`)
			if invalidDiagnostic {
				diagnostic = nil
			}
			identity := new(int)
			if err := Hold(ctx, diagnostic, identity); !errors.Is(err, ErrCleanupHold) {
				t.Fatal(err)
			}
			release()
			if err := Check(ctx); !errors.Is(err, ErrCleanupHold) {
				t.Fatal(err)
			}
			if _, next, err := Acquire(ctx, dir); !errors.Is(err, ErrCleanupHold) || next != nil {
				t.Fatal("held capability borrowed", err)
			}
			if _, next, err := Acquire(context.Background(), dir); err == nil || next != nil {
				t.Fatal("fresh writer admitted")
			}
			lock := filepath.Join(dir, "corvint/local-completion/operation.lock")
			if _, err := os.Stat(lock); err != nil {
				t.Fatal("lock lost", err)
			}
			if !invalidDiagnostic {
				if raw, err := os.ReadFile(filepath.Join(lock, "hold.json")); err != nil || string(raw) != string(diagnostic) {
					t.Fatal("diagnostic lost", err)
				}
			}
			retained, ok := heldCapabilities.Load(dir)
			if !ok || retained.(*capability).retained != identity {
				t.Fatal("owner identity dropped")
			}
		})
	}
}

func TestReadOnlyHoldDoesNotCreateAdministrativeState(t *testing.T) {
	err := Hold(context.Background(), []byte(`{"state":"HOLD"}`), new(int))
	if !errors.Is(err, ErrCleanupHold) {
		t.Fatal(err)
	}
	if Check(context.Background()) != nil {
		t.Fatal("unrelated reader poisoned")
	}
}

func TestSharedOperationRefusesStaticSymlinkAndNeverSteals(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(dir, "corvint")); err != nil {
		t.Fatal(err)
	}
	if _, release, err := Acquire(context.Background(), dir); err == nil || release != nil {
		t.Fatal("symlink accepted")
	}
	dir, err = filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(dir, "corvint", "local-completion", "operation.lock")
	if err := os.MkdirAll(lock, 0700); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, release, err := Acquire(context.Background(), dir); err == nil || release != nil {
			t.Fatal("stale lock stolen")
		}
	}
	if _, err := os.Stat(lock); err != nil {
		t.Fatal("stale lock removed", err)
	}
}
