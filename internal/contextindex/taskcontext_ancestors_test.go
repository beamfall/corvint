package contextindex

import (
	"context"
	"testing"
)

func ancestorInstructionFixture(t *testing.T) *Index {
	t.Helper()
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, map[string]string{
		"go.mod":    "module example.test/ancestors\n\ngo 1.27.0\n",
		"AGENTS.md": "Root policy.\n", "CLAUDE.md": "Root addendum.\n",
		"app/AGENTS.md": "App policy.\n", "app/CLAUDE.md": "App addendum.\n",
		"app/deep/AGENTS.md": "Local policy.\n", "app/deep/CLAUDE.md": "Local addendum.\n",
		"app/other/AGENTS.md": "Sibling policy.\n", "application/AGENTS.md": "Prefix decoy.\n",
		"app/deep/source.go": "package deep\nfunc Work() {}\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	return index
}
