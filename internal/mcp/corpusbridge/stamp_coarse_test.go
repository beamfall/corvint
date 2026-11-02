// SPDX-License-Identifier: Apache-2.0

package corpusbridge

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// coarseModel stands in for a filesystem whose recorded times come from a
// server clock running skew behind the reader, truncated to granule. Every
// entry was last changed at base except the names in edits.
type coarseModel struct {
	granule, skew time.Duration
	base          time.Time
	edits         map[string]time.Time
}

func (m *coarseModel) times(info fs.FileInfo) (uint64, int64, int64, bool) {
	ino, _, _, ok := fileTimes(info)
	if !ok {
		return 0, 0, 0, false
	}
	at, edited := m.edits[info.Name()]
	if !edited {
		at = m.base
	}
	recorded := at.Add(-m.skew).Truncate(m.granule).UnixNano()
	return ino, recorded, recorded, true
}

// modelStamp installs m and a fixed clock for the rest of the test.
func modelStamp(t *testing.T, m *coarseModel, now *time.Time) {
	t.Helper()
	savedNow, savedTimes := stampNow, stampTimes
	t.Cleanup(func() { stampNow, stampTimes = savedNow, savedTimes })
	stampNow = func() time.Time { return *now }
	stampTimes = m.times
}

// TestDCRV1845CoarseTimestampsLeaveNoStamp: a checkout whose recorded times
// are all whole seconds, or one directory of it, leaves no stamp, and an
// artifact file with whole-second times leaves no file stamp, while the same
// fine-grained checkout is stamped.
func TestDCRV1845CoarseTimestampsLeaveNoStamp(t *testing.T) {
	supportedStampPlatform(t)
	shortenStampWindow(t)
	dir := stampRepository(t)
	if checkoutStamp(context.Background(), []string{dir}) == "" {
		t.Fatal("a fine-grained checkout outside the racy window has no stamp")
	}
	artifact := filepath.Join(dir, "corpus.json")
	if err := os.WriteFile(artifact, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if artifactFileStamp(dir, "corpus.json") == "" {
		t.Fatal("a fine-grained artifact file has no stamp")
	}

	now := time.Now()
	whole := &coarseModel{granule: time.Second, base: now.Add(-time.Hour)}
	modelStamp(t, whole, &now)
	if got := checkoutStamp(context.Background(), []string{dir}); got != "" {
		t.Fatalf("stamp %q over a checkout with whole-second times", got)
	}
	if got := artifactFileStamp(dir, "corpus.json"); got != "" {
		t.Fatalf("file stamp %q over whole-second times", got)
	}

	// Only docs/ is coarse: its entries all carry whole-second times while
	// every other entry keeps its real fine-grained times.
	stampTimes = func(info fs.FileInfo) (uint64, int64, int64, bool) {
		if name := info.Name(); name == "a.md" || name == "b.md" {
			return whole.times(info)
		}
		return fileTimes(info)
	}
	if got := checkoutStamp(context.Background(), []string{dir}); got != "" {
		t.Fatalf("stamp %q over a checkout with one coarse directory", got)
	}
}

// TestDCRV1845SkewedCoarseEditNeverServedStale sweeps a modelled coarse
// share: for each granule, server clock lag and edit offset it finds the
// first capture the stamp accepts after an edit, makes a same-size edit just
// after that capture, and requires a later capture to differ from it. An
// abstaining stamp is never stale.
func TestDCRV1845SkewedCoarseEditNeverServedStale(t *testing.T) {
	supportedStampPlatform(t)
	dir := stampRepository(t)
	page := filepath.Join(dir, "docs", "a.md")
	base := time.Unix(1_800_000_000, 0)
	var now time.Time
	model := &coarseModel{base: base}
	modelStamp(t, model, &now)
	stale, stamped := 0, 0
	for _, granule := range []time.Duration{time.Second, 2 * time.Second} {
		for _, skew := range []time.Duration{0, 500 * time.Millisecond, time.Second, 3 * time.Second} {
			model.granule, model.skew = granule, skew
			for offset := time.Duration(0); offset < granule; offset += 100 * time.Millisecond {
				first := base.Add(time.Hour + offset)
				model.edits = map[string]time.Time{"a.md": first}
				captured, before := "", time.Time{}
				for at := first; at.Before(first.Add(granule + skew + 5*time.Second)); at = at.Add(100 * time.Millisecond) {
					now = at
					if captured = checkoutStamp(context.Background(), []string{dir}); captured != "" {
						before = at
						break
					}
				}
				if captured == "" {
					continue
				}
				stamped++
				// The same-size in-place rewrite keeps the inode, as an editor
				// writing through the open file does.
				if err := os.WriteFile(page, []byte("ALPHA\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				model.edits["a.md"] = before.Add(time.Millisecond)
				now = before.Add(granule + skew + 3*time.Second)
				if later := checkoutStamp(context.Background(), []string{dir}); later == captured {
					stale++
					t.Errorf("granule %v, server %v behind, edit offset %v: an edit after the capture at %v kept stamp %s",
						granule, skew, offset, before.Sub(first), captured)
				}
				if err := os.WriteFile(page, []byte("alpha\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	t.Logf("DCR-V1-845 sweep: %d accepted captures, %d served stale", stamped, stale)
}
