// SPDX-License-Identifier: Apache-2.0

package docsenvelope

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/versionline"
)

// CMS-V0-930: the not-installed refusal names the updater command that installs
// the core set into the running executable's directory, quoted when needed, and
// points at the manual install; where that command would refuse (no corvint
// beside, or an unnumbered one) and for a missing corvint it names only the
// manual install.
func TestCMSV0930NotInstalledNamesTheInstallCommand(t *testing.T) {
	// The test executable has no corvint beside it, so the message names the
	// manual install, not an updater command that would refuse.
	message := NotInstalledMessage(Binary, "corvint flows")
	for _, want := range []string{"corvint flows is served by the corvint-docs executable, which is NOT_INSTALLED", "docs/INSTALL.md", "none is beside this executable"} {
		if !strings.Contains(message, want) {
			t.Fatalf("message %q lacks %q", message, want)
		}
	}
	if strings.Contains(message, "corvint-update") {
		t.Fatalf("message %q names the updater with no corvint to replace", message)
	}

	// A numbered corvint beside gets the command with its directory; an
	// unnumbered or silent one, or none, gets the manual install.
	dir := filepath.Join(t.TempDir(), "my bin")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	probe := func(build string, err error) func(context.Context, string) (versionline.Line, error) {
		return func(_ context.Context, p string) (versionline.Line, error) {
			if p != filepath.Join(dir, CoreBinary) {
				t.Fatalf("probed %s", p)
			}
			return versionline.Line{Name: "corvint", Version: "1.0.0", Build: build}, err
		}
	}
	if got := installHint(dir, probe("7", nil)); !strings.Contains(got, "none is beside") {
		t.Fatalf("no corvint: %q", got)
	}
	if err := os.WriteFile(filepath.Join(dir, CoreBinary), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, want := installHint(dir, probe("7", nil)), "install the core set with: "+InstallCommand(dir)+" (or the manual core-set archive install in docs/INSTALL.md)"; got != want {
		t.Fatalf("numbered corvint: %q, want %q", got, want)
	}
	for _, p := range []func(context.Context, string) (versionline.Line, error){probe("", nil), probe("0", nil), probe("7", errors.New("no answer"))} {
		if got := installHint(dir, p); strings.Contains(got, "corvint-update") || !strings.Contains(got, "unnumbered corvint") {
			t.Fatalf("unnumbered corvint: %q", got)
		}
	}
	if got := installHint("", probe("7", nil)); !strings.Contains(got, InstallCommand("")) {
		t.Fatalf("unknown dir: %q", got)
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
