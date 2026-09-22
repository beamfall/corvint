package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAccuracyDashboardCorpusIgnoresRepositoryGrafts(t *testing.T) {
	t.Run("LOD-V0-005 closed Git authority ignores repository grafts", func(t *testing.T) {
		testAccuracyDashboardCorpusIgnoresRepositoryGrafts(t)
	})
}

func testAccuracyDashboardCorpusIgnoresRepositoryGrafts(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			t.Setenv("GIT_DEFAULT_HASH", format)
			for _, kind := range []string{"real-parent", "invented-parent"} {
				t.Run(kind, func(t *testing.T) {
					root, parent := plantedRepository(t)
					parentTree := gitTest(t, root, "rev-parse", "HEAD^{tree}")
					invented := gitTest(t, root, "commit-tree", parentTree, "-m", "unreachable fixture")
					if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("changed\n"), 0600); err != nil {
						t.Fatal(err)
					}
					gitTest(t, root, "add", "tracked.txt")
					gitTest(t, root, "commit", "-qm", "real child")
					head := gitTest(t, root, "rev-parse", "HEAD")
					candidate := parent
					if kind == "invented-parent" {
						candidate = invented
					}
					row := plantedTraceRow(t, candidate)
					if err := os.WriteFile(filepath.Join(root, ".context-corvint", "traces", candidate+".jsonl"), row, 0600); err != nil {
						t.Fatal(err)
					}
					manifest := plantedManifest(t)
					read := func() traceCorpusResult {
						authority, err := newClosedGitAuthority(root)
						if err != nil {
							t.Fatal(err)
						}
						defer authority.cancel()
						defer authority.executableFD.Close()
						corpus, err := validateTraceCorpusWithAuthority(root, manifest, authority)
						if err != nil {
							t.Fatal(err)
						}
						return corpus
					}
					before := read()
					want := 1
					if kind == "invented-parent" {
						want = 0
					}
					if len(before.members) != want || (want == 0 && before.sources[0].terminal["SOURCE_INVALID_IDENTITY"] != 1) {
						t.Fatalf("ancestry control: members=%d terminal=%v", len(before.members), before.sources[0].terminal)
					}
					if err := os.WriteFile(filepath.Join(root, ".git", "info", "grafts"), []byte(head+" "+invented+"\n"), 0600); err != nil {
						t.Fatal(err)
					}
					after := read()
					t.Logf("candidate=%s members before=%d after=%d terminal before=%v after=%v", kind, len(before.members), len(after.members), before.sources[0].terminal, after.sources[0].terminal)
					if len(after.members) != want {
						t.Fatalf("graft changed corpus authority: members=%d want=%d", len(after.members), want)
					}
				})
			}
		})
	}
}
