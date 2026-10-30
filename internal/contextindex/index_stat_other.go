//go:build !darwin && !linux

package contextindex

import (
	"errors"
	"os"
)

// Without lstat inode and change-time fields no index entry can be vouched,
// so every eligible entry takes the hashed worktree pass (DCR-V1-453).
func fileIndexStat(os.FileInfo) (gitIndexStat, bool) { return gitIndexStat{}, false }

func openGitIndex(string) (*os.File, error) {
	return nil, errors.New("git index stat routing is unsupported on this platform")
}
