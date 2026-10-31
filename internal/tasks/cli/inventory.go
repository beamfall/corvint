package cli

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// withInventoryStore permits only an absent journal, never an incomplete or
// unreadable one. It captures current intent without conferring journal authority.
func withInventoryStore(env Env, body func(*readCtx) error) (*readCtx, error) {
	if path := snapshotBundlePath(env.Cwd); path != "" {
		return withSnapshotStore(path, body)
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return nil, err
	}
	absent, err := journalAbsent(repo.StateDir)
	if err != nil || !absent {
		return withStore(env, body)
	}
	rc := &readCtx{repo: repo, journalAbsent: true}
	idx := closedIndexFor(env, repo)
	for attempt := 0; attempt < 4; attempt++ {
		if err := requireJournalAbsent(repo.StateDir); err != nil {
			return rc, err
		}
		tree, err := intent.TreeDigestIndexed(repo.IntentRoot(), idx)
		if err != nil {
			return rc, err
		}
		st, err := intent.LoadTreeIndexed(repo.IntentRoot(), tree, "", idx)
		if err != nil {
			return rc, err
		}
		rc.store = st
		bodyErr := runBody(rc, body)
		if env.afterRead != nil {
			env.afterRead()
		}
		if err := requireJournalAbsent(repo.StateDir); err != nil {
			return rc, err
		}
		tree, err = intent.TreeDigestIndexed(repo.IntentRoot(), idx)
		if err != nil {
			return rc, err
		}
		if err := requireJournalAbsent(repo.StateDir); err != nil {
			return rc, err
		}
		if tree.Sha256 == st.Tree.Sha256 {
			return rc, bodyErr
		}
	}
	return rc, wire.Errorf(wire.CodeSnapshotMoved, repo.StateDir, "intent projection changed during four read attempts")
}

func journalAbsent(path string) (bool, error) {
	if err := intent.CheckNoSymlink(path); err != nil {
		return false, err
	}
	_, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, wire.Errorf(wire.CodeUnsupportedFilesystem, path, "cannot inspect journal directory: %v", err)
	}
	return false, nil
}

func requireJournalAbsent(path string) error {
	absent, err := journalAbsent(path)
	if err != nil {
		return err
	}
	if !absent {
		return wire.Errorf(wire.CodeSnapshotMoved, path, "journal appeared during an unaudited inventory read")
	}
	return nil
}

// closedIndexFor is the closed-ticket index a lazy read uses, read once per
// command; nil keeps the full scan (TCX-V0-982).
func closedIndexFor(env Env, repo *intent.Repository) *intent.ClosedIndex {
	if !env.lazyClosed || !useClosedIndex {
		return nil
	}
	return intent.ReadClosedIndex(repo.StateDir)
}

// useClosedIndex exists so the parity tests can compare indexed and full
// reads.
var useClosedIndex = true

// runBody runs a read body and reports a closed record that could not be
// loaded on demand as the read's error (TCX-V0-982).
func runBody(rc *readCtx, body func(*readCtx) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			me, ok := r.(ticket.MaterializeError)
			if !ok {
				panic(r)
			}
			err = me.Err
		}
	}()
	return body(rc)
}

// closedTicket reports a COMPLETED or ARCHIVED ticket from its summary, so
// a scan over OPEN and HELD tickets skips it undecoded (TCX-V0-982).
func closedTicket(inv *ticket.Inventory, id string) bool {
	s, ok := inv.Summary(id)
	return ok && ticket.Closed(s.Status)
}

// closedIndexRefresher returns the step that rewrites the derived
// closed-ticket index after a command that is not a pure read, run only when
// the command committed a mutation, seen as a changed journal head
// (TCX-V0-983). A refused or failed command leaves the filesystem as it was.
// The rewrite is best effort: any failure leaves the old index, whose stale
// entries readers reject, or none.
func closedIndexRefresher(env Env) func() {
	if snapshotBundlePath(env.Cwd) != "" {
		return func() {}
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return func() {}
	}
	if fi, err := os.Stat(repo.StateDir); err != nil || !fi.IsDir() {
		return func() {}
	}
	head := filepath.Join(repo.StateDir, "head.json")
	before := headStamp(head)
	return func() {
		if headStamp(head) != before {
			_ = intent.WriteClosedIndex(repo.IntentRoot(), repo.StateDir)
		}
	}
}

type fileStamp struct {
	size, mtime int64
	ok          bool
}

func headStamp(path string) fileStamp {
	fi, err := os.Stat(path)
	if err != nil {
		return fileStamp{}
	}
	return fileStamp{size: fi.Size(), mtime: fi.ModTime().UnixNano(), ok: true}
}
