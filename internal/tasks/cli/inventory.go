package cli

import (
	"errors"
	"os"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// withInventoryStore permits only an absent journal, never an incomplete or
// unreadable one. It captures current intent without conferring journal authority.
func withInventoryStore(env Env, body func(*readCtx) error) (*readCtx, error) {
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return nil, err
	}
	absent, err := journalAbsent(repo.StateDir)
	if err != nil || !absent {
		return withStore(env, body)
	}
	rc := &readCtx{repo: repo, journalAbsent: true}
	for attempt := 0; attempt < 4; attempt++ {
		if err := requireJournalAbsent(repo.StateDir); err != nil {
			return rc, err
		}
		st, err := intent.Load(repo.PrimaryWorktree)
		if err != nil {
			return rc, err
		}
		rc.store = st
		bodyErr := body(rc)
		if env.afterRead != nil {
			env.afterRead()
		}
		if err := requireJournalAbsent(repo.StateDir); err != nil {
			return rc, err
		}
		tree, err := intent.TreeDigest(repo.PrimaryWorktree)
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
