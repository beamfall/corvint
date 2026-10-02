package affected

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSourceReadBoundsAndStickyFailure(t *testing.T) {
	t.Run("DLT-V0-002 sticky read failure", testSourceReadBoundsAndStickyFailure)
}
func testSourceReadBoundsAndStickyFailure(t *testing.T) {
	source := FSSource(fstest.MapFS{"ok": {Data: []byte(strings.Repeat("x", MaxSourceBytes))}, "big": {Data: []byte(strings.Repeat("x", MaxSourceBytes+1))}})
	if body, err := source.Read("ok"); err != nil || len(body) != MaxSourceBytes {
		t.Fatalf("boundary: %d %v", len(body), err)
	}
	if _, err := source.Read("missing"); !errors.Is(err, fs.ErrNotExist) || source.Err() != nil {
		t.Fatalf("optional missing: %v %v", err, source.Err())
	}
	if _, err := source.Read("big"); !errors.Is(err, ErrWalkLimit) || !errors.Is(source.Err(), ErrWalkLimit) {
		t.Fatalf("bound: %v %v", err, source.Err())
	}
	source.Read("missing")
	if !errors.Is(source.Err(), ErrWalkLimit) {
		t.Fatal("lost first failure")
	}
}

func TestSourceHiddenWalkRetainsIgnoredFailure(t *testing.T) {
	t.Run("DLT-V0-002 hidden walk failure retained", testSourceHiddenWalkRetainsIgnoredFailure)
}
func testSourceHiddenWalkRetainsIgnoredFailure(t *testing.T) {
	source := FSSource(fstest.MapFS{".maestro/escape": {Mode: fs.ModeSymlink}})
	_ = source.Walk(".maestro", func(string, fs.DirEntry, error) error { return nil })
	if !errors.Is(source.Err(), ErrWalkUnrepresentable) {
		t.Fatalf("hidden mode: %v", source.Err())
	}
}
