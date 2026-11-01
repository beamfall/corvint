package authority

import (
	"os"
	"sync"
)

// qualifyFresh is the qualification QualifyReused runs; a test replaces it
// to count probes.
var qualifyFresh = Qualify

// qualified holds, per root, the directory a successful qualification
// probed and its result (LGL-V0-890).
var qualified struct {
	sync.Mutex
	byRoot map[string]qualifiedRoot
}

type qualifiedRoot struct {
	dir os.FileInfo
	q   *Qualification
}

// QualifyReused is Qualify, reused within one process (LGL-V0-890): a root
// that qualified before and is still the same file, by os.SameFile on a
// fresh Lstat, returns that qualification without probing again. A failed
// qualification is never kept, and a root replaced or remounted at the same
// path is probed again. Every write still syncs what it publishes; only the
// probe is skipped.
func QualifyReused(root string) (*Qualification, error) {
	now, err := os.Lstat(root)
	if err == nil {
		qualified.Lock()
		kept, ok := qualified.byRoot[root]
		qualified.Unlock()
		if ok && os.SameFile(kept.dir, now) {
			return kept.q, nil
		}
	}
	q, err := qualifyFresh(root)
	if err != nil {
		return q, err
	}
	// The identity taken before the probe: a root replaced during it is
	// probed again by the next call.
	if now != nil {
		qualified.Lock()
		if qualified.byRoot == nil {
			qualified.byRoot = map[string]qualifiedRoot{}
		}
		qualified.byRoot[root] = qualifiedRoot{dir: now, q: q}
		qualified.Unlock()
	}
	return q, nil
}
