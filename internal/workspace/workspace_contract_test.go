// SPDX-License-Identifier: Apache-2.0
package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCMPV1003WorkspaceFileContract (CMP-V1-003 item 3, CMP-V1-004; V1-5434): the documented file
// .corvint/workspace.json (corvint-workspace/0) keeps every member the 1.x compatibility promise
// freezes. A declaration naming each member is discovered and read unchanged, so a renamed or
// removed member is refused as unknown; an unknown member or another profile is refused.
func TestCMPV1003WorkspaceFileContract(t *testing.T) {
	const declaration = `{"profile":"corvint-workspace/0",` +
		`"repos":{"app":{"root":"../app","revision":"main"},"lib":{"root":"../lib"}},` +
		`"manifest_repo":"app"}`
	root := t.TempDir()
	write := func(body string) {
		t.Helper()
		file := filepath.Join(root, filepath.FromSlash(DiscoveryPath))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(declaration)
	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, ".corvint")
	if got.Profile != Profile || got.ManifestRepo != "app" || len(got.Repos) != 2 ||
		got.Repos["app"] != (Repo{Root: filepath.Join(dir, "..", "app"), Revision: "main"}) ||
		got.Repos["lib"] != (Repo{Root: filepath.Join(dir, "..", "lib")}) {
		t.Fatalf("read %+v", got)
	}
	for label, body := range map[string]string{
		"unknown member":      strings.Replace(declaration, `"repos":`, `"added":1,"repos":`, 1),
		"unknown repo member": strings.Replace(declaration, `"root":"../lib"`, `"root":"../lib","added":1`, 1),
		"other profile":       strings.Replace(declaration, Profile, "corvint-workspace/1", 1),
	} {
		write(body)
		if _, err := Discover(root); err == nil {
			t.Errorf("%s: accepted", label)
		}
	}
}
