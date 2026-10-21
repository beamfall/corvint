//go:build darwin || linux

package authority

import (
	"errors"
	"os"
	"testing"
)

// V1-2268: scan reads files through the retained parent roots and runs the
// full fixtureSession.check once at each end instead of twice per file. A
// directory swapped for a symlink after the session pinned it must still be
// refused by the next capture, whichever retained parent is swapped.
func TestFixtureCaptureRefusesParentSwappedToSymlink(t *testing.T) {
	for _, tc := range []struct {
		name string
		path func(h *ftHarness) string
		// refused: the retained-parent comparison names the swap; the
		// primary path instead fails safeopen's no-follow descent.
		refused bool
	}{
		{"primary", func(h *ftHarness) string { return h.r.Root }, false},
		{"state", func(h *ftHarness) string { return h.r.StateDir }, true},
		{"intent", func(h *ftHarness) string { return h.r.IntentDir }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := ftNew(t, true)
			if _, err := h.capture(); err != nil {
				t.Fatalf("capture before the swap: %v", err)
			}
			path := tc.path(h)
			moved := path + ".moved"
			fixtureMust(t, os.Rename(path, moved))
			fixtureMust(t, os.Symlink(moved, path))
			t.Cleanup(func() {
				_ = os.Remove(path)
				_ = os.Rename(moved, path)
			})
			want := func(op string, err error) {
				t.Helper()
				if err == nil || (tc.refused && !errors.Is(err, fixtureRefused)) {
					t.Fatalf("%s after %s became a symlink: got %v, want a refusal", op, tc.name, err)
				}
			}
			_, err := h.capture()
			want("capture", err)
			_, err = h.scan()
			want("scan", err)
		})
	}
}
