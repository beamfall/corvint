//go:build darwin || linux

package authority

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// stagingSyncRun is what one publish cycle observed: every boundary in
// order, the directory sync calls by directory name and the file sync calls.
type stagingSyncRun struct {
	boundaries []string
	dirs       map[string]int
	files      int
	calls      int
}

// stagingSyncCycle stages and links a receipt, clears its slot, then stages
// a head and replaces head.json, the slot-consuming rename. skip sets
// SkipStagingSync through the exported Session before the first stage.
func stagingSyncCycle(t *testing.T, skip bool) stagingSyncRun {
	t.Helper()
	restoreHooks(t)
	run := stagingSyncRun{dirs: map[string]int{}}
	var s *fixtureSession
	syncDirectory = func(f *os.File) error {
		// Retained parents open as "."; name each by its retained key.
		st, err := f.Stat()
		fixtureMust(t, err)
		keys := make([]string, 0, len(s.parents))
		for key := range s.parents {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		name := "unretained"
		for _, key := range keys {
			if p, err := s.parents[key].file.Stat(); err == nil && os.SameFile(st, p) {
				name = key
				break
			}
		}
		run.dirs[name]++
		return directorySync(f)
	}
	syncFile = func(f *os.File) error {
		run.files++
		return fullSync(f)
	}
	s, r := fixtureHarness(t)
	head := filepath.Join(r.StateDir, "head.json")
	fixtureMust(t, os.WriteFile(head, []byte("pre"), 0o600))
	if skip {
		(&Session{inner: s}).SkipStagingSync()
	}
	s.before = func(at string) error {
		run.boundaries = append(run.boundaries, at)
		return nil
	}
	receipt, err := s.prepare("a00", fixtureReceipt, []byte("receipt"))
	fixtureMust(t, err)
	fixtureMust(t, s.link(receipt, fixtureTarget{fixtureReceipt, "000000000001.json"}))
	fixtureMust(t, s.removeStage("a00"))
	post, err := s.prepare("a01", fixtureHead, []byte("post"))
	fixtureMust(t, err)
	pre := wire.Sum([]byte("pre"))
	fixtureMust(t, s.replace(post, fixtureTarget{fixtureHead, "head.json"}, &pre))
	s.before = nil
	fixtureBytes(t, filepath.Join(r.StateDir, "receipts", "000000000001.json"), "receipt")
	fixtureBytes(t, head, "post")
	fixtureAbsent(t, filepath.Join(r.StateDir, "staging", "a00"))
	fixtureAbsent(t, filepath.Join(r.StateDir, "staging", "a01"))
	run.calls = (&Session{inner: s}).SyncCalls()
	return run
}

// TestLGLV0005_SkippedStagingSyncKeepsEveryOtherSync: a session that skips
// staging syncs makes no staging-directory sync call, makes exactly the file
// and destination-directory sync calls a full session makes, crosses the
// same boundaries in the same order, and publishes the same bytes. SyncCalls
// counts every call either session made (LGL-V0-006).
func TestLGLV0005_SkippedStagingSyncKeepsEveryOtherSync(t *testing.T) {
	full := stagingSyncCycle(t, false)
	skipped := stagingSyncCycle(t, true)
	if full.dirs["staging"] != 4 {
		t.Fatalf("a full session synced staging %d times, want 4 (two prepares, a slot removal, a rename): %v", full.dirs["staging"], full.dirs)
	}
	if skipped.dirs["staging"] != 0 {
		t.Fatalf("a skipping session synced staging %d times: %v", skipped.dirs["staging"], skipped.dirs)
	}
	delete(full.dirs, "staging")
	if !reflect.DeepEqual(full.dirs, skipped.dirs) || full.files != skipped.files || full.files != 2 {
		t.Fatalf("destination or file syncs differ: full %v/%d, skipped %v/%d", full.dirs, full.files, skipped.dirs, skipped.files)
	}
	if !reflect.DeepEqual(full.boundaries, skipped.boundaries) {
		t.Fatalf("boundaries differ:\nfull    %v\nskipped %v", full.boundaries, skipped.boundaries)
	}
	if full.calls != full.files+4+len(full.dirs)*0+sum(full.dirs) || skipped.calls != skipped.files+sum(skipped.dirs) {
		t.Fatalf("SyncCalls: full %d (files %d dirs %v), skipped %d (files %d dirs %v)", full.calls, full.files, full.dirs, skipped.calls, skipped.files, skipped.dirs)
	}
}

func sum(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}
