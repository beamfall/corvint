// SPDX-License-Identifier: Apache-2.0
package watch

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
)

// relevantPaths reports whether any changed path is Go source or a path Git
// does not ignore. Ignored Go source still compiles into go test (and
// WorkingDiff selects it), so only ignored non-Go paths such as test output
// are suppressed. Paths under .git are never saves. When Git cannot answer,
// it reports true rather than miss a save.
func relevantPaths(ctx context.Context, root string, paths []string) bool {
	gitDir := filepath.Join(root, ".git")
	var ask []string
	for _, path := range paths {
		if path == gitDir || strings.HasPrefix(path, gitDir+string(filepath.Separator)) {
			continue
		}
		if strings.HasSuffix(path, ".go") {
			return true
		}
		ask = append(ask, path)
	}
	if len(ask) == 0 {
		return false
	}
	cmd := exec.CommandContext(ctx, "git", append([]string{"check-ignore", "--"}, ask...)...)
	cmd.Dir = root
	out, err := cmd.Output()
	var exit *exec.ExitError
	if err != nil && !(errors.As(err, &exit) && exit.ExitCode() == 1) {
		return true
	}
	ignored := map[string]bool{}
	for _, l := range strings.Split(string(out), "\n") {
		if l != "" {
			if !filepath.IsAbs(l) {
				l = filepath.Join(root, l)
			}
			ignored[l] = true
		}
	}
	for _, p := range ask {
		if !ignored[p] {
			return true
		}
	}
	return false
}
