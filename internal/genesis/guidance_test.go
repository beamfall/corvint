package genesis

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestRepositoryGuidanceNestedGitOutputBudget(t *testing.T) {
	t.Run("RGV-V0-011 nested Git output stays within both budgets", func(t *testing.T) {
		git, err := exec.LookPath("git")
		if err != nil {
			t.Fatal(err)
		}
		root := genesisRepository(t, git, "sha1")
		genesisGit(t, git, root, "update-index", "--index-version=4")
		snapshot, err := OpenGuidance(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		before := snapshot.output
		if err = snapshot.Check(context.Background()); err != nil {
			t.Fatal(err)
		}
		if snapshot.output <= before {
			t.Fatal("nested private status output was not charged")
		}
		if _, err = snapshot.run(context.Background(), 32<<20, "rev-parse", "HEAD"); err != nil {
			t.Fatalf("request was not narrowed to parent ceiling: %v", err)
		}
		snapshot.output = (64 << 20) - 8
		if _, err = snapshot.run(context.Background(), 32<<20, "rev-parse", "HEAD"); err == nil || !strings.Contains(err.Error(), "output-budget-exceeded") {
			t.Fatalf("remaining global output cap was bypassed: %v", err)
		}
	})
}
