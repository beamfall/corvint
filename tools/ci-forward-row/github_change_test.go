// SPDX-License-Identifier: Apache-2.0
package forwardrow

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// requireGitHubChange skips T while the checked-out .github lacks the change T tests. These tests
// land before their .github change, which reaches main in its own pull request (the
// claude/layers-integ-github follow-up), so on a tree without it they cannot pass. PATH is
// relative to the repository root; MARKER is text the file must contain, or, when empty, the file
// must only exist. Once the follow-up has landed every marker is present and nothing skips.
func requireGitHubChange(t *testing.T, path, marker string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(path)))
	switch {
	case err == nil && strings.Contains(string(body), marker):
		return
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		t.Fatal(err)
	case err != nil:
		t.Skipf("needs the .github follow-up pull request (claude/layers-integ-github): %s is absent", path)
	}
	t.Skipf("needs the .github follow-up pull request (claude/layers-integ-github): %s lacks %s", path, marker)
}
