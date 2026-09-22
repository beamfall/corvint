package tracerecordrepo

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/tracemigraterepo"
)

func TestAccuracyTraceReadIgnoresRepositoryGrafts(t *testing.T) {
	t.Run("LTPM-V0-002 reachable trace authority ignores repository grafts", func(t *testing.T) {
		testAccuracyGrafts(t, func(ctx context.Context, root string) (string, error) {
			index, err := contextindex.Build(ctx, root)
			if err != nil {
				return "", err
			}
			_, state, err := Read(ctx, root, index)
			return state, err
		})
	})
}

func TestAccuracyMigrationDryRunIgnoresRepositoryGrafts(t *testing.T) {
	t.Run("LTPM-V0-003 LTPM-V0-004 migration authority ignores repository grafts", func(t *testing.T) {
		testAccuracyGrafts(t, func(ctx context.Context, root string) (string, error) {
			_, err := tracemigraterepo.Evaluate(ctx, root, tracemigraterepo.Options{})
			if err != nil {
				return "", err
			}
			return "ready", nil
		})
	})
}

func testAccuracyGrafts(t *testing.T, read func(context.Context, string) (string, error)) {
	t.Helper()
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			t.Setenv("GIT_DEFAULT_HASH", format)
			for _, candidateKind := range []string{"real-parent", "invented-parent"} {
				t.Run(candidateKind, func(t *testing.T) {
					root := newAdapterFixture(t)
					parent := gitAdapterFixture(t, root, "rev-parse", "HEAD")
					parentTree := gitAdapterFixture(t, root, "rev-parse", "HEAD^{tree}")
					invented := gitAdapterFixture(t, root, "commit-tree", parentTree, "-m", "unreachable fixture")
					writeAdapterFixture(t, root, "internal/value.go", "package internal\n\nconst Value = 2\n")
					gitAdapterFixture(t, root, "add", "internal/value.go")
					gitAdapterFixture(t, root, "commit", "-qm", "real child")
					head := gitAdapterFixture(t, root, "rev-parse", "HEAD")
					candidate := parent
					if candidateKind == "invented-parent" {
						candidate = invented
					}
					writeAdapterFixture(t, root, filepath.ToSlash(filepath.Join(".context-corvint", "traces", candidate+".jsonl")), "")
					beforeState, beforeErr := read(context.Background(), root)
					if candidateKind == "real-parent" && (beforeErr != nil || beforeState != "ready") {
						t.Fatalf("valid ancestry control failed: %q %v", beforeState, beforeErr)
					}
					if candidateKind == "invented-parent" && (beforeErr == nil || !strings.Contains(beforeErr.Error(), "unreachable")) {
						t.Fatalf("unreachable ancestry control failed: %q %v", beforeState, beforeErr)
					}
					writeAdapterFixture(t, root, ".git/info/grafts", head+" "+invented+"\n")
					afterState, afterErr := read(context.Background(), root)
					t.Logf("candidate=%s before=%q/%v after=%q/%v", candidateKind, beforeState, beforeErr, afterState, afterErr)
					if candidateKind == "real-parent" && (afterErr != nil || afterState != "ready") {
						t.Fatalf("graft must not remove real ancestry: %q %v", afterState, afterErr)
					}
					if candidateKind == "invented-parent" && (afterErr == nil || !strings.Contains(afterErr.Error(), "unreachable")) {
						t.Fatalf("graft must not invent ancestry: %q %v", afterState, afterErr)
					}
				})
			}
		})
	}
}
