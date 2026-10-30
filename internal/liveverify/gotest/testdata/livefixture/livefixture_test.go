package livefixture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPass(t *testing.T) {
	t.Attr("corvint", "fixture")
	artifactDir := t.ArtifactDir()
	if err := os.WriteFile(filepath.Join(artifactDir, "proof.txt"), []byte("proof"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Log("output is digested")
	t.Run("subtest", func(t *testing.T) {})
}

func TestParallel(t *testing.T) {
	t.Parallel()
}

func TestSkip(t *testing.T) {
	t.Skip("fixture skip")
}
