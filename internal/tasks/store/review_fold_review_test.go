package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-138: the carried fold is bound to the receipt-name listing, so an
// earlier receipt removed after the fold was carried falls back to the
// whole-history fold and refuses exactly as FoldExternalReviews does.
func TestCALV0138_CarriedFoldFallsBackWhenAnEarlierReceiptIsRemoved(t *testing.T) {
	t.Parallel()
	repo := historyStore(t, 120)
	var f ReviewFold
	if _, err := f.Fold(repo, 120); err != nil {
		t.Fatal(err)
	}
	name, err := snapshot.ReceiptName(1)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(repo.StateDir, "receipts", name)); err != nil {
		t.Fatal(err)
	}
	_, want := FoldExternalReviews(repo, 120, nil)
	if want == nil {
		t.Fatal("the whole-history fold accepted a missing receipt")
	}
	got, err := f.Fold(repo, 120)
	if err == nil || wire.CodeOf(err) != wire.CodeOf(want) || err.Error() != want.Error() {
		t.Fatalf("carried fold after a removed receipt: %v (answered %t), want %v", err, got != nil, want)
	}
	if f.audit != nil || f.seq != 0 {
		t.Fatal("a refused fold kept carried state")
	}
}

// CAL-V0-138: the listing binding requires every carried receipt name to be
// a regular file, so an earlier receipt replaced by a symlink or a directory
// falls back to the whole-history fold and refuses exactly as it does.
func TestCALV0138_CarriedFoldFallsBackWhenAnEarlierReceiptIsNotRegular(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		replace func(path string) error
	}{
		{"symlink", func(path string) error {
			moved := path + ".moved"
			if err := os.Rename(path, moved); err != nil {
				return err
			}
			return os.Symlink(filepath.Base(moved), path)
		}},
		{"directory", func(path string) error {
			if err := os.Remove(path); err != nil {
				return err
			}
			return os.Mkdir(path, 0o755)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := historyStore(t, 120)
			var f ReviewFold
			if _, err := f.Fold(repo, 120); err != nil {
				t.Fatal(err)
			}
			name, err := snapshot.ReceiptName(1)
			if err != nil {
				t.Fatal(err)
			}
			if err := tc.replace(filepath.Join(repo.StateDir, "receipts", name)); err != nil {
				t.Fatal(err)
			}
			_, want := FoldExternalReviews(repo, 120, nil)
			if want == nil {
				t.Fatal("the whole-history fold accepted a non-regular receipt")
			}
			got, err := f.Fold(repo, 120)
			if err == nil || wire.CodeOf(err) != wire.CodeOf(want) || err.Error() != want.Error() {
				t.Fatalf("carried fold after a %s receipt: %v (answered %t), want %v", tc.name, err, got != nil, want)
			}
			if f.audit != nil || f.seq != 0 {
				t.Fatal("a refused fold kept carried state")
			}
		})
	}
}
