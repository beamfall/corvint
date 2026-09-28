//go:build darwin || linux

package authority

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func guardRepo(t *testing.T) (*intent.Repository, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r := &intent.Repository{PrimaryWorktree: root, StateDir: filepath.Join(root, ".git", "taskman")}
	for _, dir := range []string{r.StateDir, filepath.Join(root, intent.Dir, "tickets")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	file := filepath.Join(root, intent.Dir, "tickets", "x.json")
	if err := os.WriteFile(file, []byte("aaaa"), 0600); err != nil {
		t.Fatal(err)
	}
	return r, file
}

// CAL-V0-026: all mutation classes invalidate the off-lock observation.
func TestCALV0026_ChangeGuard(t *testing.T) {
	for _, name := range []string{"in-place-restored-time", "replace", "add", "remove", "directory", "ancestor"} {
		t.Run(name, func(t *testing.T) {
			repo, file := guardRepo(t)
			g, err := WatchChanges(repo)
			if err != nil {
				t.Fatal(err)
			}
			defer g.Close()
			if err := g.Check(); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "in-place-restored-time":
				st, _ := os.Stat(file)
				err = os.WriteFile(file, []byte("bbbb"), 0600)
				if err == nil {
					err = os.Chtimes(file, st.ModTime(), st.ModTime())
				}
			case "replace":
				err = os.Rename(file, file+".old")
				if err == nil {
					err = os.WriteFile(file, []byte("aaaa"), 0600)
				}
			case "add":
				err = os.WriteFile(file+".new", []byte("new"), 0600)
			case "remove":
				err = os.Remove(file)
			case "directory":
				err = os.Rename(filepath.Dir(file), filepath.Dir(file)+".old")
			case "ancestor":
				err = os.Rename(filepath.Dir(repo.StateDir), filepath.Dir(repo.StateDir)+".old")
			}
			if err != nil {
				t.Fatal(err)
			}
			if wire.CodeOf(g.Check()) != wire.CodeSnapshotMoved {
				t.Fatal("mutation admitted")
			}
			if wire.CodeOf(g.Check()) != wire.CodeSnapshotMoved {
				t.Fatal("invalidation was not sticky")
			}
		})
	}
}

func TestCALV0026_ChangeGuardClosesAndRefusesSymlinks(t *testing.T) {
	repo, file := guardRepo(t)
	for i := 0; i < 20; i++ {
		g, err := WatchChanges(repo)
		if err != nil {
			t.Fatal(err)
		}
		if err = g.Close(); err != nil {
			t.Fatal(err)
		}
		if err = g.Close(); err != nil {
			t.Fatal(err)
		}
		if g.Check() == nil {
			t.Fatal("closed guard usable")
		}
	}
	if err := os.Symlink(file, file+".link"); err != nil {
		t.Fatal(err)
	}
	if g, err := WatchChanges(repo); err == nil {
		g.Close()
		t.Fatal("symlink admitted")
	}
}

type brokenWatch struct{}

func (brokenWatch) add(string, bool) (os.FileInfo, error) { return nil, errors.New("unavailable") }
func (brokenWatch) changed() (bool, error)                { return false, errors.New("invalid descriptor") }
func (brokenWatch) close() error                          { return nil }

func TestCALV0026_ChangeGuardFailureIsNotValidity(t *testing.T) {
	g := &ChangeGuard{watch: brokenWatch{}}
	if wire.CodeOf(g.Check()) != wire.CodeUnsupportedFilesystem {
		t.Fatal("watch failure admitted")
	}
}
