// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
)

func write(t *testing.T, dir, name, text string) string {
	t.Helper()
	file := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return file
}

// WSD-V0-001 WSD-V0-002: a declaration decodes with relative roots resolved against its
// directory; every malformed or out-of-bound declaration refuses as workspace-invalid.
func TestWSDV0001Load(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(t.TempDir(), "tests")
	file := write(t, dir, ".corvint/workspace.json", `{"profile":"corvint-workspace/0","repos":{"e2e":{"root":"../../e2e","revision":"main"},"docs":{"root":"`+filepath.ToSlash(abs)+`"}},"manifest_repo":"e2e"}`)
	w, err := Load(file)
	if err != nil {
		t.Fatal(err)
	}
	if got := w.Repos["e2e"]; got.Root != filepath.Join(filepath.Dir(dir), "e2e") || got.Revision != "main" {
		t.Fatalf("relative root %+v", got)
	}
	if w.Repos["docs"].Root != abs || w.ManifestRepo != "e2e" || w.File != file || strings.Join(w.Aliases(), ",") != "docs,e2e" {
		t.Fatalf("workspace %+v", w)
	}
	big := `{"profile":"corvint-workspace/0","repos":{"e2e":{"root":"x"}},"pad":"` + strings.Repeat("a", MaxBytes) + `"}`
	for name, text := range map[string]string{
		"not json":         "{",
		"unknown member":   `{"profile":"corvint-workspace/0","repos":{"e2e":{"root":"x"}},"tests":"e2e"}`,
		"unknown repo key": `{"profile":"corvint-workspace/0","repos":{"e2e":{"root":"x","rev":"main"}}}`,
		"trailing":         `{"profile":"corvint-workspace/0","repos":{"e2e":{"root":"x"}}} {}`,
		"profile":          `{"profile":"corvint-workspace/1","repos":{"e2e":{"root":"x"}}}`,
		"no repos":         `{"profile":"corvint-workspace/0","repos":{}}`,
		"alias":            `{"profile":"corvint-workspace/0","repos":{"E2E":{"root":"x"}}}`,
		"empty root":       `{"profile":"corvint-workspace/0","repos":{"e2e":{"root":""}}}`,
		"option revision":  `{"profile":"corvint-workspace/0","repos":{"e2e":{"root":"x","revision":"--all"}}}`,
		"spaced revision":  `{"profile":"corvint-workspace/0","repos":{"e2e":{"root":"x","revision":"a b"}}}`,
		"manifest alias":   `{"profile":"corvint-workspace/0","repos":{"e2e":{"root":"x"}},"manifest_repo":"tests"}`,
		"oversized":        big,
	} {
		_, err := Load(write(t, t.TempDir(), "w.json", text))
		var coded *gokernel.Error
		if !errors.As(err, &coded) || coded.Code != "workspace-invalid" || !strings.HasPrefix(err.Error(), "workspace-invalid: ") {
			t.Errorf("%s: %v", name, err)
		}
	}
	link := filepath.Join(t.TempDir(), "link.json")
	if err := os.Symlink(file, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(link); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("symlink: %v", err)
	}
}

// WSD-V0-003: discovery reads root/.corvint/workspace.json; an absent file is no workspace and a
// present invalid one refuses.
func TestWSDV0003Discover(t *testing.T) {
	root := t.TempDir()
	if w, err := Discover(root); w != nil || err != nil {
		t.Fatalf("absent: %v %v", w, err)
	}
	write(t, root, DiscoveryPath, `{"profile":"corvint-workspace/0","repos":{"e2e":{"root":"../e2e"}}}`)
	w, err := Discover(root)
	if err != nil || w.Repos["e2e"].Root != filepath.Join(root, "e2e") {
		t.Fatalf("discovered %+v %v", w, err)
	}
	write(t, root, DiscoveryPath, `{"profile":"corvint-workspace/0"}`)
	if _, err := Discover(root); err == nil || !strings.Contains(err.Error(), "workspace-invalid") {
		t.Fatalf("invalid discovered: %v", err)
	}
}
