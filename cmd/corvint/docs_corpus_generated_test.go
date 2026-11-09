package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDCPV1608DocsCorpusManifestNamesGeneratedInputs: `docs corpus manifest` refuses a scope
// holding generated output, naming every such input with its repository, path, revision and check.
func TestDCPV1608DocsCorpusManifestNamesGeneratedInputs(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"docs/guide.md":         "# Guide\n",
		"docs/gen/reference.md": "# Reference\n",
		"docs/corpus.json":      `{"schema":"corvint-corpus-input/1"}`,
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cemGit(t, root, "init", "-q", "-b", "main")
	cemGit(t, root, "add", ".")
	cemGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-qm", "fixture")
	rev := cemGit(t, root, "rev-parse", "HEAD")
	code, out, stderr := corpusCLI(t, root, "docs", "corpus", "manifest", "--revision", rev, "--scope", "docs", "--timestamp", "2026-10-28T00:00:00Z")
	if code == 0 || out != "" {
		t.Fatalf("manifest pinned generated output: exit %d\n%s", code, out)
	}
	for _, want := range []string{
		"generated output cannot be source input (2 inputs;",
		"repository=primary path=docs/corpus.json revision=" + rev + " check=corpus-schema",
		"repository=primary path=docs/gen/reference.md revision=" + rev + " check=generated-path",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("refusal lacks %q:\n%s", want, stderr)
		}
	}
}
