package store

import (
	"path/filepath"
	"sort"
	"sync"
)

// retentionFailures names the derived checkpoint files whose best-effort
// retention failed since the last TakeRetentionFailures (LGL-V0-803,
// V1-1058). CAL-V0-060 keeps such a failure from failing or changing the
// transaction; this only lets the command that committed say so, where
// before a retention that kept failing showed only as slow complete reads.
var retentionFailures struct {
	sync.Mutex
	names map[string]bool
}

// noteRetentionFailure records that the derived file at path was not
// retained.
func noteRetentionFailure(path string) {
	retentionFailures.Lock()
	defer retentionFailures.Unlock()
	if retentionFailures.names == nil {
		retentionFailures.names = map[string]bool{}
	}
	retentionFailures.names[filepath.Base(path)] = true
}

// TakeRetentionFailures returns, sorted, the base names of the derived
// checkpoint files whose retention failed in this process since the last
// call, and forgets them. A command process makes one call when it reports
// its result.
func TakeRetentionFailures() []string {
	retentionFailures.Lock()
	defer retentionFailures.Unlock()
	var out []string
	for name := range retentionFailures.names {
		out = append(out, name)
	}
	retentionFailures.names = nil
	sort.Strings(out)
	return out
}
