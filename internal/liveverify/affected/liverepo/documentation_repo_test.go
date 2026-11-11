// SPDX-License-Identifier: Apache-2.0

package liverepo_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AFP-V0-160 (6) as AFP-V0-465 and AFP-V0-508 amend it: this repository's
// declaration names `docs/build-log/`, `docs/specs/` and
// `docs/decisions/`, and every top-level `docs/*.md` file; every declared
// file exists, so no stale entry hides a path that has become something else.
func TestRepositoryDeclaresBuildLogDocumentation_AFPV0160(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	_, declared, err := affected.ReadDocumentationDeclaration(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{"docs/build-log/", "docs/specs/", "docs/decisions/"} {
		if !slices.Contains(declared, directory) {
			t.Errorf("%s does not declare %s", affected.DocumentationPathsPath, directory)
		}
	}
	for _, entry := range declared {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(entry))); err != nil {
			t.Errorf("declared %s: %v", entry, err)
		}
	}
	docs, err := filepath.Glob(filepath.Join(root, "docs", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, doc := range docs {
		if name := "docs/" + filepath.Base(doc); !slices.Contains(declared, name) {
			t.Errorf("%s is not declared in %s", name, affected.DocumentationPathsPath)
		}
	}
}

// AFP-V0-763: this repository declares its own instruction files.
func TestRepositoryDeclaresItsInstructionFiles_AFPV0763(t *testing.T) {
	t.Parallel()
	_, declared, err := affected.ReadDocumentationDeclaration(repositoryRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		if !slices.Contains(declared, name) {
			t.Errorf("%s does not declare %s", affected.DocumentationPathsPath, name)
		}
	}
}
