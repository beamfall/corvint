//go:build darwin || linux

package authority

import (
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestLGLV0013_PrepareAllStagesSyncsAndPublishesLikePrepare: one PrepareAll
// of two receipts and a head crosses every item's create,
// write and file-sync boundaries in order, makes one file sync per item
// before any stage-close, registers every stage, and each stage then links
// or replaces with the bytes it was given. A single item crosses exactly
// the boundaries Prepare does.
func TestLGLV0013_PrepareAllStagesSyncsAndPublishesLikePrepare(t *testing.T) {
	restoreHooks(t)
	var mu sync.Mutex
	files := 0
	var boundaries []string
	syncFile = func(f *os.File) error {
		mu.Lock()
		files++
		mu.Unlock()
		return fullSync(f)
	}
	s, r := fixtureHarness(t)
	head := filepath.Join(r.StateDir, "head.json")
	fixtureMust(t, os.WriteFile(head, []byte("pre"), 0o600))
	s.before = func(at string) error {
		mu.Lock()
		defer mu.Unlock()
		if at == "stage-close" && files != 3 {
			t.Errorf("stage-close after %d file syncs, want all 3 first", files)
		}
		boundaries = append(boundaries, at)
		return nil
	}
	before := s.syncCalls
	stages, err := (&Session{inner: s}).PrepareAll([]Preparation{
		{Slot: "a00", Role: RoleReceipt, Data: []byte("receipt")},
		{Slot: "a01", Role: RoleReceipt, Data: []byte("receipt2")},
		{Slot: "a02", Role: RoleHead, Data: []byte("post")},
	})
	fixtureMust(t, err)
	s.before = nil
	want := []string{"create", "write", "file-sync", "create", "write", "file-sync", "create", "write", "file-sync", "sync:staging", "stage-close", "sync:staging", "stage-close", "sync:staging", "stage-close"}
	if !reflect.DeepEqual(boundaries, want) {
		t.Fatalf("boundaries\n got %v\nwant %v", boundaries, want)
	}
	if files != 3 {
		t.Fatalf("file syncs %d, want 3", files)
	}
	if got := s.syncCalls - before; got != 3+3 {
		t.Fatalf("SyncCalls grew by %d, want 3 file and 3 staging syncs", got)
	}
	if len(stages) != 3 || len(s.stages) != 3 {
		t.Fatalf("stages %d registered %d, want 3", len(stages), len(s.stages))
	}
	for i, data := range []string{"receipt", "receipt2", "post"} {
		if stages[i].Sha256() != wire.Sum([]byte(data)) {
			t.Fatalf("stage %d digest differs from %q", i, data)
		}
	}
	sess := &Session{inner: s}
	fixtureMust(t, sess.Link(stages[0], Target{RoleReceipt, "000000000001.json"}))
	fixtureMust(t, sess.RemoveStage("a00"))
	fixtureMust(t, sess.Link(stages[1], Target{RoleReceipt, "000000000002.json"}))
	fixtureMust(t, sess.RemoveStage("a01"))
	pre := wire.Sum([]byte("pre"))
	fixtureMust(t, sess.Replace(stages[2], Target{RoleHead, "head.json"}, &pre))
	fixtureBytes(t, filepath.Join(r.StateDir, "receipts", "000000000001.json"), "receipt")
	fixtureBytes(t, filepath.Join(r.StateDir, "receipts", "000000000002.json"), "receipt2")
	fixtureBytes(t, head, "post")
	stagingEmpty(t, r.StateDir)

	// One item is Prepare's own path.
	boundaries = nil
	s.before = func(at string) error { boundaries = append(boundaries, at); return nil }
	_, err = sess.PrepareAll([]Preparation{{Slot: "a03", Role: RoleReceipt, Data: []byte("r2")}})
	fixtureMust(t, err)
	single := boundaries
	boundaries = nil
	_, err = sess.Prepare("a04", RoleReceipt, []byte("r3"))
	fixtureMust(t, err)
	s.before = nil
	if !reflect.DeepEqual(single, boundaries) {
		t.Fatalf("single-item PrepareAll %v, Prepare %v", single, boundaries)
	}
	if _, err := sess.PrepareAll(nil); err == nil {
		t.Fatal("an empty PrepareAll was accepted")
	}
}

// TestLGLV0013_PrepareAllSyncFailureCleansEveryStage: a failed file sync of
// any item fails the whole call as a created-stage effect, leaves no file in
// staging and registers no stage, so nothing is left for a publish.
func TestLGLV0013_PrepareAllSyncFailureCleansEveryStage(t *testing.T) {
	for _, failing := range []string{"receipt", "post"} {
		t.Run(failing, func(t *testing.T) {
			restoreHooks(t)
			s, r := fixtureHarness(t)
			syncFile = func(f *os.File) error {
				b := make([]byte, 16)
				n, _ := f.ReadAt(b, 0)
				if string(b[:n]) == failing {
					return syscall.EIO
				}
				return fullSync(f)
			}
			_, err := (&Session{inner: s}).PrepareAll([]Preparation{
				{Slot: "a00", Role: RoleReceipt, Data: []byte("receipt")},
				{Slot: "a01", Role: RoleHead, Data: []byte("post")},
			})
			fixtureEffect(t, err, true)
			if len(s.stages) != 0 {
				t.Fatalf("registered %d stages after a failure", len(s.stages))
			}
			stagingEmpty(t, r.StateDir)
		})
	}
}

func stagingEmpty(t *testing.T, stateDir string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(stateDir, "staging"))
	fixtureMust(t, err)
	if len(entries) != 0 {
		t.Fatalf("staging residue: %d entries, first %s", len(entries), entries[0].Name())
	}
}
