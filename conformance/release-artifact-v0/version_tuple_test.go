package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPublicReleaseVersionTupleMovesTogether binds PUB-V0-001's current version tuple to VERSION.
// The native binary and archive smoke are already bound transitively (TestGoOnlySourceAndVersion,
// go-archive-gate smoke); the VS Code exact admission and its conformance case are not, so a
// version bump that forgot them would otherwise pass every gate.
func TestPublicReleaseVersionTupleMovesTogether(t *testing.T) {
	t.Run("PUB-V0-001-current-version-tuple", func(t *testing.T) {
		root := filepath.Join("..", "..")
		read := func(relative string) string {
			data, err := os.ReadFile(filepath.Join(root, relative))
			if err != nil {
				t.Fatal(err)
			}
			return string(data)
		}
		version := strings.TrimSpace(read("VERSION"))
		if version == "" {
			t.Fatal("VERSION is empty")
		}
		manifest, err := loadManifest("manifest.json")
		if err != nil {
			t.Fatal(err)
		}
		if manifest.Smoke.ExpectedVersion != "Corvint "+version {
			t.Fatalf("manifest smoke expects %q, VERSION is %q", manifest.Smoke.ExpectedVersion, version)
		}
		for relative, want := range map[string]string{
			"cmd/corvint/main.go":                        `const version = "` + version + `"`,
			"extensions/vscode/src/executable.ts":        `!== "` + version + `"`,
			"conformance/vscode-extension-v0/cases.json": `"versionExact": "` + version + `"`,
		} {
			if !strings.Contains(read(relative), want) {
				t.Fatalf("%s does not carry %s", relative, want)
			}
		}
	})
}
