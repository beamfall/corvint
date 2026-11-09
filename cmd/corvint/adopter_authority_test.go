package main

import (
	"os"
	"path/filepath"
	"testing"
)

// adopterAuthorityRepository is a synthetic repository shaped like an external
// Go modular-monolith adopter (V1-1977): a Go module, a root AGENTS.md, and
// numbered ADRs under docs/adr. Its content is invented for the test.
func adopterAuthorityRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		".gitignore": ".corvint/\n",
		"go.mod":     "module example.test/mediahost\n\ngo 1.27.0\n",
		"AGENTS.md": "# Agent contract\n\n" +
			"Read the accepted ADRs under docs/adr before changing the plugin host.\n" +
			"Every plugin registers through the plugin registry; never import a plugin from core.\n",
		"docs/adr/0001-plugin-host-architecture.md": "# 0001. Plugin host architecture\n\n" +
			"Status: Accepted\n\n## Decision\n\nThe media server core loads every catalog plugin through one plugin registry.\n",
		"docs/adr/0002-plugin-registry-manifest.md": "# 0002. Plugin registry manifest\n\n" +
			"Status: Accepted\n\n## Decision\n\nA plugin declares its manifest; the plugin registry validates the manifest before load.\n",
		"internal/plugin/registry.go": "package plugin\n\n// Registry holds loaded plugins.\ntype Registry struct{ names []string }\n\n" +
			"// Register adds a plugin manifest to the registry.\nfunc (r *Registry) Register(name string) { r.names = append(r.names, name) }\n",
		"cmd/mediahost/main.go": "package main\n\nfunc main() {}\n",
	}
	for path, content := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"},
		{"config", "user.name", "Corvint Test"}, {"add", "."}, {"commit", "-qm", "initial"},
	} {
		command := fixtureGitCommand(arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	return root
}
