package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Witness is the dispatcher's cheap change witness (CAL-V0-139). It hashes
// the journal head, barrier and VERSION bytes, the identity of the state
// and receipt directories, and the top-level identity of the intent tree:
// every entry's name, mode, size and modification time, so a created,
// removed or renamed ticket, release or policy file changes it. A ticket
// file rewritten in place without a journal write is seen by the
// dispatcher's bounded full read instead. Any error is doubt: the caller
// reads the store in full.
func (q dispatchQueue) Witness() (string, error) {
	repo, err := intent.Resolve(q.env.Cwd)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	fmt.Fprintf(h, "taskman-dispatch-witness/0\x00%s\x00%s\x00", repo.StateDir, repo.IntentRoot())
	for _, f := range []struct {
		name string
		max  int
	}{{"VERSION", 64}, {"head.json", wire.MaxJournalHeadBytes}, {"barrier.json", wire.MaxBarrierBytes}} {
		raw, err := intent.ReadFile(filepath.Join(repo.StateDir, f.name), f.max)
		switch {
		case os.IsNotExist(err):
			fmt.Fprintf(h, "%s absent\x00", f.name)
		case err != nil:
			return "", err
		default:
			fmt.Fprintf(h, "%s %d %x\x00", f.name, len(raw), sha256.Sum256(raw))
		}
	}
	for _, p := range []string{repo.StateDir, filepath.Join(repo.StateDir, "receipts"), filepath.Join(repo.StateDir, "RESTORE_INCOMPLETE")} {
		if err := witnessStat(h, p); err != nil {
			return "", err
		}
	}
	root := filepath.Join(repo.IntentRoot(), intent.Dir)
	if err := witnessStat(h, root); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if err := witnessStat(h, filepath.Join(root, e.Name())); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func witnessStat(h hash.Hash, path string) error {
	fi, err := os.Lstat(path)
	if os.IsNotExist(err) {
		fmt.Fprintf(h, "%s absent\x00", path)
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(h, "%s %v %d %d\x00", path, fi.Mode(), fi.Size(), fi.ModTime().UnixNano())
	return nil
}
