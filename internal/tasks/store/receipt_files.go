package store

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/safeopen"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// receiptFiles reads retained receipts beneath one pinned receipts/
// descriptor, so a whole-history fold costs one no-follow openat per receipt
// instead of a root-to-leaf traversal per receipt (CAL-V0-135). Stat,
// validation and byte bounds are intent.ReadFile's. close re-binds the named
// path to the pinned directory, so a receipts/ directory replaced during the
// fold refuses as SNAPSHOT_MOVED rather than answering from a detached tree.
type receiptFiles struct {
	dir    string
	root   *os.Root
	pinned pinnedDirReader
}

func newReceiptFiles(repo *intent.Repository) *receiptFiles {
	return &receiptFiles{dir: filepath.Join(repo.StateDir, "receipts")}
}

func (r *receiptFiles) read(seq uint64) ([]byte, error) {
	name, err := snapshot.ReceiptName(seq)
	if err != nil {
		return nil, err
	}
	if r.root == nil {
		root, err := safeopen.Root(r.dir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, err
			}
			return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, r.dir, "cannot open: %v", err)
		}
		r.root = root
	}
	return r.pinned.read(r.root, filepath.Join(r.dir, name), name, wire.MaxReceiptFileBytes)
}

// close releases the pinned descriptors. bind additionally requires the
// named receipts/ path to still be the pinned directory.
func (r *receiptFiles) close(bind bool) error {
	if r.root == nil {
		return nil
	}
	var err error
	if bind {
		err = r.binding()
	}
	// A binding refusal keeps its own code; a release failure is reported
	// only when the fold is otherwise good.
	if e := errors.Join(r.pinned.close(), r.root.Close()); err == nil {
		err = e
	}
	r.root = nil
	return err
}

func (r *receiptFiles) binding() error {
	named, err := safeopen.Root(r.dir)
	if err != nil {
		return wire.Errorf(wire.CodeSnapshotMoved, r.dir, "previously observed identity disappeared or changed")
	}
	defer named.Close()
	a, err := named.Stat(".")
	if err != nil {
		return wire.Errorf(wire.CodeUnsupportedFilesystem, r.dir, "cannot stat: %v", err)
	}
	b, err := r.root.Stat(".")
	if err != nil {
		return wire.Errorf(wire.CodeUnsupportedFilesystem, r.dir, "cannot stat: %v", err)
	}
	if !os.SameFile(a, b) {
		return wire.Errorf(wire.CodeSnapshotMoved, r.dir, "previously observed identity disappeared or changed")
	}
	return nil
}
