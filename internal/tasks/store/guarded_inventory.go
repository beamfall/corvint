package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/safeopen"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Hooks are operation-local so fault tests never replace another lease's I/O.
type inventoryHooksKey struct{}
type inventoryHooks struct {
	audit            func(journal.Reader) (*journal.Result, journal.PhysicalObservation, error)
	open             func(string) (*os.Root, error)
	stat             func(*os.Root) (os.FileInfo, error)
	close            func(*os.Root) error
	read             func(*os.Root, string, string, int) ([]byte, error)
	closeGuard       func(*authority.ChangeGuard) error
	closePreparation func(*authority.PreparationLock) error
	// commitStage observes commitLease reaching "sweep" and then "lock".
	commitStage func(string)
}

func hooksForInventory(ctx context.Context) inventoryHooks {
	h, _ := ctx.Value(inventoryHooksKey{}).(inventoryHooks)
	if h.audit == nil {
		h.audit = (journal.Reader).AuditForWriteObserved
	}
	if h.open == nil {
		h.open = safeopen.Root
	}
	if h.stat == nil {
		h.stat = func(r *os.Root) (os.FileInfo, error) { return r.Stat(".") }
	}
	if h.close == nil {
		h.close = (*os.Root).Close
	}
	if h.read == nil {
		h.read = intent.ReadFileFromRoot
	}
	if h.closeGuard == nil {
		h.closeGuard = (*authority.ChangeGuard).Close
	}
	if h.closePreparation == nil {
		h.closePreparation = (*authority.PreparationLock).Close
	}
	if h.commitStage == nil {
		h.commitStage = func(string) {}
	}
	return h
}

type inventoryCleanup struct {
	stage string
	err   error
}

func fatalInventoryError(failures []inventoryCleanup, failure, observation error) error {
	var details []string
	if failure != nil {
		details = append(details, "scan: "+failure.Error())
	}
	if observation != nil {
		details = append(details, "observation: "+observation.Error())
	}
	for _, f := range failures {
		details = append(details, f.stage+": "+f.err.Error())
	}
	return wire.Errorf(wire.CodeUnsupportedFilesystem, "guarded inventory lifetime", "%s", strings.Join(details, "; "))
}

type parentInventoryReader struct {
	hooks    inventoryHooks
	root     *os.Root
	parent   string
	identity os.FileInfo
	boundary error
	fatal    []inventoryCleanup
}

func (r *parentInventoryReader) boundaryError(stage, path string, err error) error {
	failure := wire.Errorf(wire.CodeUnsupportedFilesystem, path, "%s: %v", stage, err)
	if r.boundary == nil {
		r.boundary = failure
	}
	return failure
}
func (r *parentInventoryReader) close(root *os.Root, stage string) {
	if err := r.hooks.close(root); err != nil {
		r.fatal = append(r.fatal, inventoryCleanup{stage, err})
	}
}

// finish rebinds once per parent run. Both roots are retired even when an
// observation or rebind fails; retirement failures have their own channel.
func (r *parentInventoryReader) finish() (err error) {
	if r.root == nil {
		return nil
	}
	root, parent := r.root, r.parent
	r.root = nil
	defer r.close(root, "parent close "+parent)
	if r.identity == nil {
		return r.boundary
	}
	rebound, err := r.hooks.open(parent)
	if err != nil {
		return r.boundaryError("parent rebind", parent, err)
	}
	defer r.close(rebound, "rebind close "+parent)
	info, err := r.hooks.stat(rebound)
	if err != nil {
		return r.boundaryError("rebind stat", parent, err)
	}
	if !os.SameFile(r.identity, info) {
		return wire.Errorf(wire.CodeSnapshotMoved, parent, "inventory parent identity changed")
	}
	return nil
}
func (r *parentInventoryReader) read(path string, max int) ([]byte, error) {
	parent := filepath.Dir(path)
	if r.root == nil || parent != r.parent {
		if err := r.finish(); err != nil {
			return nil, err
		}
		if len(r.fatal) > 0 {
			return nil, fatalInventoryError(r.fatal, nil, nil)
		}
		root, err := r.hooks.open(parent)
		if err != nil {
			return nil, r.boundaryError("parent open", parent, err)
		}
		r.root, r.parent, r.identity = root, parent, nil
		info, err := r.hooks.stat(root)
		if err != nil {
			return nil, r.boundaryError("parent stat", parent, err)
		}
		r.identity = info
	}
	return r.hooks.read(r.root, path, filepath.Base(path), max)
}

// Only prepareLease routes here, immediately after creating the matching live
// ChangeGuard. This retains one parent root plus a temporary rebind root, not
// a history-sized root cache. Other inventory callers keep the ordinary reader.
func guardedLeaseInventory(repo *intent.Repository, guard *authority.ChangeGuard, hooks inventoryHooks, observed ...map[string]journal.PhysicalFile) (*transaction.Inventory, error, []inventoryCleanup) {
	if guard == nil {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, "inventory", "missing change guard"), nil
	}
	if err := guard.Check(); err != nil {
		return nil, err, nil
	}
	r := &parentInventoryReader{hooks: hooks}
	files, dirs, err := scanWithReader(repo, r.read, observed...)
	finishErr := r.finish()
	if r.boundary != nil {
		err = r.boundary
	} else if err == nil {
		err = finishErr
	}
	if err == nil {
		err = guard.Check()
	}
	if err != nil || len(r.fatal) > 0 {
		return nil, err, r.fatal
	}
	inv, err := transaction.NewInventory(files, dirs)
	return inv, err, nil
}

func (p *preparedLease) fatalError() error {
	return fatalInventoryError(p.fatalCleanup, p.failure, p.observationFailure)
}
func (p *preparedLease) recordCleanup(stage string, err error) {
	if err != nil {
		p.fatalCleanup = append(p.fatalCleanup, inventoryCleanup{stage, err})
	}
}
