package doccorpus

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

// MaintenanceFile binds an output to the bytes inspected by the caller. Nil Before
// means exclusive creation. Existing originals remain named, even after success.
type MaintenanceFile struct {
	Path         string
	Before, Next []byte
}

// MaintenancePublication exposes partial progress; it is never a success receipt.
type MaintenancePublication struct {
	Path      string `json:"path"`
	Recovery  string `json:"recovery,omitempty"`
	Published bool   `json:"published"`
}

type stagedMaintenance struct {
	file                 MaintenanceFile
	parent               *os.Root
	parentInfo, original os.FileInfo
	stage                string
	state                MaintenancePublication
}

// PublishMaintenancePair stages both files before any capture, then publishes each
// through no-clobber links. It deliberately offers no pair/crash atomicity. verify
// runs before staging and again before capture, with exact prospective publication
// metadata for receipt preflight; checkpoint is a package-private test seam.
func PublishMaintenancePair(ctx context.Context, root string, files [2]MaintenanceFile, verify func([]MaintenancePublication) error) ([]MaintenancePublication, error) {
	return publishMaintenancePair(ctx, root, files, verify, nil)
}

func publishMaintenancePair(ctx context.Context, root string, files [2]MaintenanceFile, verify func([]MaintenancePublication) error, checkpoint func(string, int)) (states []MaintenancePublication, err error) {
	repository, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer repository.Close()
	staged := []*stagedMaintenance{}
	defer func() {
		for _, s := range staged {
			states = append(states, s.state)
			_ = s.parent.Remove(s.stage)
			_ = s.parent.Close()
		}
		if err != nil {
			err = fmt.Errorf("maintenance publication incomplete: %w; retained state: %+v", err, states)
		}
	}()
	// Reserve names in memory so the caller can preflight the exact success
	// receipt, including recovery metadata, before even staging a filesystem write.
	planned := make([]MaintenancePublication, len(files))
	for i, f := range files {
		planned[i] = MaintenancePublication{Path: f.Path, Published: true}
		if f.Before != nil {
			planned[i].Recovery = path.Join(path.Dir(f.Path), ".corvint-flow-recovery-"+rand.Text())
		}
	}
	if verify != nil {
		if err = verify(planned); err != nil {
			return nil, err
		}
	}
	for i, f := range files {
		if !validPath(f.Path) || f.Path == "." || strings.EqualFold(files[0].Path, files[1].Path) {
			return nil, fail("invalid or duplicate maintenance destination")
		}
		for _, part := range strings.Split(f.Path, "/") {
			if strings.EqualFold(part, ".git") {
				return nil, fail("unsafe maintenance destination")
			}
		}
		parent, e := corpusParent(repository, path.Dir(f.Path))
		if e != nil {
			return nil, e
		}
		s := &stagedMaintenance{file: f, parent: parent, state: MaintenancePublication{Path: f.Path}}
		staged = append(staged, s)
		s.parentInfo, e = parent.Stat(".")
		if e != nil {
			return nil, e
		}
		s.original, e = parent.Lstat(path.Base(f.Path))
		if f.Before == nil {
			if !os.IsNotExist(e) {
				return nil, fail("maintenance destination exists")
			}
		} else {
			if e != nil || !s.original.Mode().IsRegular() {
				return nil, fail("maintenance destination is not regular")
			}
			raw, e := testvaliditydoc.ReadFile(parent, path.Base(f.Path))
			if e != nil || !bytes.Equal(raw, f.Before) {
				return nil, fail("maintenance destination changed")
			}
		}
		if i == 1 && s.original != nil && staged[0].original != nil && os.SameFile(s.original, staged[0].original) {
			return nil, fail("aliased maintenance destinations")
		}
		s.stage = ".corvint-flow-stage-" + rand.Text()
		output, e := parent.OpenFile(s.stage, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return nil, e
		}
		mode := os.FileMode(0644)
		if s.original != nil {
			mode = s.original.Mode().Perm()
		}
		e = output.Chmod(mode)
		if e == nil {
			_, e = output.Write(f.Next)
		}
		if e == nil {
			e = output.Sync()
		}
		closeErr := output.Close()
		if e == nil {
			e = closeErr
		}
		if e != nil {
			return nil, e
		}
	}
	if verify != nil {
		if err = verify(planned); err != nil {
			return nil, err
		}
	}
	for i, s := range staged {
		if checkpoint != nil {
			checkpoint("before-capture", i)
		}
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		live, e := corpusParent(repository, path.Dir(s.file.Path))
		if e != nil {
			return nil, fail("maintenance parent changed")
		}
		liveInfo, e := live.Stat(".")
		_ = live.Close()
		if e != nil || !os.SameFile(s.parentInfo, liveInfo) {
			return nil, fail("maintenance parent changed")
		}
		name := path.Base(s.file.Path)
		if s.original != nil {
			recovery := path.Base(planned[i].Recovery)
			reserved, e := s.parent.OpenFile(recovery, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if e != nil {
				return nil, e
			}
			_ = reserved.Close()
			if e = s.parent.Rename(name, recovery); e != nil {
				_ = s.parent.Remove(recovery)
				return nil, e
			}
			s.state.Recovery = path.Join(path.Dir(s.file.Path), recovery)
			restore := func() { _ = s.parent.Link(recovery, name) }
			captured, e := s.parent.Lstat(recovery)
			if e != nil || !captured.Mode().IsRegular() || !os.SameFile(s.original, captured) {
				restore()
				return nil, fail("maintenance destination replaced")
			}
			raw, e := testvaliditydoc.ReadFile(s.parent, recovery)
			if e != nil || !bytes.Equal(raw, s.file.Before) {
				restore()
				return nil, fail("maintenance destination changed")
			}
			if checkpoint != nil {
				checkpoint("before-publish", i)
			}
			if e = ctx.Err(); e != nil {
				restore()
				return nil, e
			}
			if e = s.parent.Link(s.stage, name); e != nil {
				restore()
				return nil, fail("maintenance publication refused")
			}
		} else {
			if checkpoint != nil {
				checkpoint("before-publish", i)
			}
			if e = ctx.Err(); e != nil {
				return nil, e
			}
			if e = s.parent.Link(s.stage, name); e != nil {
				return nil, fail("maintenance creation refused")
			}
		}
		s.state.Published = true
	}
	// Recheck both visible names and pinned parents before returning success. A
	// later external edit remains possible; this is conditional publication, not a lock.
	for _, s := range staged {
		live, e := corpusParent(repository, path.Dir(s.file.Path))
		if e != nil {
			return nil, fail("maintenance parent changed after publication")
		}
		info, e := live.Stat(".")
		_ = live.Close()
		if e != nil || !os.SameFile(info, s.parentInfo) {
			return nil, fail("maintenance parent changed after publication")
		}
		raw, e := testvaliditydoc.ReadFile(s.parent, path.Base(s.file.Path))
		if e != nil || !bytes.Equal(raw, s.file.Next) {
			return nil, fail("maintenance output changed after publication")
		}
	}
	return nil, nil
}
