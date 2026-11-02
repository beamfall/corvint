package cli

import (
	"os"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const profileClosedIndexRebuild = "taskman-closed-index-rebuild/0"

// doctorRebuildClosedIndex is `corvint-tasks doctor --rebuild-closed-index`
// (TCL-V0-738, TCX-V0-988): it writes the closed-ticket index of a journaled store, or of
// a journal-free copy whose tracked intent tree loads, with the writer the CLI
// runs after a committed mutation (TCX-V0-984), so both give the same bytes
// but for the capture time. The index file is its only write. A store that is
// neither refuses with the read's own code: UNINITIALIZED, MALFORMED or
// another load refusal, and a snapshot bundle refuses UNSUPPORTED.
func doctorRebuildClosedIndex(env Env) *wire.Result {
	cmd := []string{"doctor"}
	if snapshotBundlePath(env.Cwd) != "" {
		return failure(cmd, nil, wire.Errorf(wire.CodeUnsupported, "snapshot", "a snapshot bundle has no closed-ticket index to rebuild"))
	}
	rc, err := withInventoryStore(env, func(*readCtx) error { return nil })
	if err != nil {
		return failure(cmd, rc, err)
	}
	if err := intent.WriteClosedIndex(rc.repo.IntentRoot(), rc.repo.StateDir); err != nil {
		return failure(cmd, rc, err)
	}
	path := intent.ClosedIndexPath(rc.repo.StateDir)
	raw, err := os.ReadFile(path)
	if err != nil {
		return failure(cmd, rc, wire.Errorf(wire.CodeUnsupported, "closed-index", "the rebuilt index does not read back: %v", err))
	}
	journal := "PRESENT"
	if rc.journalAbsent {
		journal = "ABSENT"
	}
	o := wire.NewObject()
	o.Set("profile", wire.String(profileClosedIndexRebuild))
	o.Set("path", wire.String(path))
	o.Set("journal", wire.String(journal))
	o.Set("indexSha256", wire.String(string(wire.Sum(raw))))
	o.Set("human", wire.Strings([]string{"closed-ticket index written to " + path + " (journal " + journal + ")"}))
	res := success(cmd, rc)
	res.Items = []wire.Value{wire.ObjectValue(o)}
	return res
}
