// SPDX-License-Identifier: Apache-2.0

package docsenvelope

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CMS-V0-930: the not-installed refusal names the updater command that installs
// the core set into the running executable's directory, quoted when needed, and
// points at the manual install; a missing corvint names only the manual install.
func TestCMSV0930NotInstalledNamesTheInstallCommand(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		self = resolved
	}
	command := InstallCommand(filepath.Dir(self))
	message := NotInstalledMessage(Binary, "corvint flows")
	for _, want := range []string{"corvint flows is served by the corvint-docs executable, which is NOT_INSTALLED", command, "docs/INSTALL.md"} {
		if !strings.Contains(message, want) {
			t.Fatalf("message %q lacks %q", message, want)
		}
	}
	if core := NotInstalledMessage(CoreBinary, "--corpus"); strings.Contains(core, "corvint-update") || !strings.Contains(core, "docs/INSTALL.md") {
		t.Fatalf("missing corvint message %q", core)
	}
	for dir, want := range map[string]string{
		"":                 "corvint-update apply --allow-network --component core",
		"/usr/local/bin":   "corvint-update apply --allow-network --component core --bin-dir /usr/local/bin",
		"/opt/my bin":      "corvint-update apply --allow-network --component core --bin-dir '/opt/my bin'",
		"/home/o'neil/bin": `corvint-update apply --allow-network --component core --bin-dir '/home/o'\''neil/bin'`,
	} {
		if got := InstallCommand(dir); got != want {
			t.Fatalf("InstallCommand(%q) = %q, want %q", dir, got, want)
		}
	}
}
