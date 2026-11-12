// SPDX-License-Identifier: Apache-2.0
package forwardrow

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// requireGitHubChange fails T unless the checked-out .github holds the change T tests. These tests
// assert a .github change their requirement depends on, so a tree without it has not delivered the
// requirement: the test fails rather than skips, and a missing half can never count as a pass on
// main (AFP-V0-9984, V1-5337). PATH is relative to the repository root; MARKER is text the file
// must contain, or, when empty, the file must only exist.
func requireGitHubChange(t *testing.T, path, marker string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(path)))
	switch {
	case err == nil && strings.Contains(string(body), marker):
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		t.Fatal(err)
	case err != nil:
		t.Fatalf("%s is absent: this test asserts a .github change its requirement depends on; land that change with the test (AFP-V0-9984)", path)
	default:
		t.Fatalf("%s lacks %q: this test asserts a .github change its requirement depends on; land that change with the test (AFP-V0-9984)", path, marker)
	}
}

// githubChangeHelperEnv selects the helper process's case: "absent" names a missing file, "marker"
// an existing file without the marker.
const githubChangeHelperEnv = "CORVINT_GITHUB_CHANGE_HELPER"

// TestGitHubChangeHelperProcess runs requireGitHubChange on a missing .github half when the parent
// test starts it; run directly it does nothing.
func TestGitHubChangeHelperProcess(t *testing.T) {
	switch os.Getenv(githubChangeHelperEnv) {
	case "absent":
		requireGitHubChange(t, ".github/forward-rows/absent-for-afp-v0-9984.sh", "")
	case "marker":
		requireGitHubChange(t, ".github/workflows/ci.yml", "AFP-V0-9984 marker that no workflow holds")
	}
}

// AFP-V0-9984 (V1-5337): a test whose .github half is missing fails; before the change
// requireGitHubChange skipped it, so the package passed with the requirement undelivered.
func TestRequireGitHubChangeFailsNotSkips_AFPV09984(t *testing.T) {
	for _, mode := range []string{"absent", "marker"} {
		t.Run(mode, func(t *testing.T) {
			command := exec.Command(os.Args[0], "-test.run=^TestGitHubChangeHelperProcess$", "-test.v", "-test.count=1")
			command.Env = append(os.Environ(), githubChangeHelperEnv+"="+mode)
			output, err := command.CombinedOutput()
			text := string(output)
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 {
				t.Fatalf("helper exit %v, want a test failure (exit 1)\n%s", err, text)
			}
			if !strings.Contains(text, "--- FAIL: TestGitHubChangeHelperProcess") || strings.Contains(text, "--- SKIP") || !strings.Contains(text, "AFP-V0-9984") {
				t.Fatalf("a missing .github half must be reported as a failure naming AFP-V0-9984, not a skip:\n%s", text)
			}
		})
	}
}
