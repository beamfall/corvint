package companionrelease

import (
	"context"
	"fmt"
	"strings"

	"github.com/Beamfall/corvint/internal/gitstatus"
)

// maxStatusBytes bounds one git status capture used for the clean-tree gate.
const maxStatusBytes = 4 << 20

// requireCleanTree fails closed unless git status reports zero changes
// against root's checkout. It reuses internal/gitstatus, the package the
// rest of Corvint already trusts for a hardened, sandboxed status read.
func requireCleanTree(ctx context.Context, gitPath, root, scratchParent string) error {
	env := closedGitEnv(scratchParent)
	runner := func(ctx context.Context, dir string, limit int, args ...string) ([]byte, error) {
		stdout, _, err := runCaptured(ctx, dir, env, subprocessTimeout, append([]string{gitPath}, args...)...)
		if len(stdout) > limit {
			stdout = stdout[:limit]
		}
		return stdout, err
	}
	raw, err := gitstatus.StatusIn(ctx, root, scratchParent, maxStatusBytes, runner,
		"status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignored=no")
	if err != nil {
		return fmt.Errorf("git status unavailable for %s: %w", root, err)
	}
	if len(strings.TrimSpace(string(raw))) != 0 {
		return fmt.Errorf("checkout %s is not clean", root)
	}
	return nil
}
