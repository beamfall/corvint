package doccorpus

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/contextindex"
)

func TestLoadInputRefusesOverBoundBlobBeforeBody(t *testing.T) {
	t.Run("DCP-V1-018 V1-0747 input byte bound before allocation", func(t *testing.T) {
		root := t.TempDir()
		git(t, root, "init", "-q")
		git(t, root, "config", "user.name", "Corpus test")
		git(t, root, "config", "user.email", "corpus@example.invalid")
		const body = "123456789"
		if err := os.WriteFile(filepath.Join(root, "big.md"), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		git(t, root, "add", ".")
		git(t, root, "commit", "-qm", "source")
		auth, err := gitauth.Open(root, gitrun.NewDefaultBudget())
		if err != nil {
			t.Fatal(err)
		}
		index := &contextindex.Index{CommitRevision: git(t, root, "rev-parse", "HEAD"), Sources: map[string]contextindex.Source{}}
		var refused *Error
		if _, err := loadInput(context.Background(), auth, index, "big.md", len(body)-1); !errors.As(err, &refused) || refused.Code != "corpus-refused" || refused.Message != "input bound exceeded" {
			t.Fatalf("bound+1 input: %v", err)
		}
		source, err := loadInput(context.Background(), auth, index, "big.md", len(body))
		if err != nil || string(source.Data) != body {
			t.Fatalf("exact-bound input: %q %v", source.Data, err)
		}
	})
}
