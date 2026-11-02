package main

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gitprobe"
)

// V1-3588: canonicalEnvironment pins GIT_GRAFT_FILE to the null device, and a
// Git that prints graft-file deprecation advice prints none into the stderr a
// failed canonicalGit reports, nor into the stdout a successful one returns.
func TestCanonicalGitSuppressesGraftAdvice(t *testing.T) {
	if !gitprobe.GraftAdvice("git") {
		t.Skip("this git prints no graft-file advice, so there is nothing to suppress")
	}
	ctx := context.Background()
	repo := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-q", "--allow-empty", "-m", "one"}} {
		if _, err := canonicalGit(ctx, repo, args...); err != nil {
			t.Fatal(err)
		}
	}
	out, err := canonicalGit(ctx, repo, "rev-list", "HEAD")
	if err != nil || len(strings.TrimSpace(out)) != 40 {
		t.Fatalf("rev-list HEAD = %q, %v; want one object name", out, err)
	}
	_, err = canonicalGit(ctx, repo, "rev-parse", "--verify", "HEAD~1000") // past the root commit
	if err == nil || strings.Contains(err.Error(), "grafts is deprecated") {
		t.Fatalf("failed git reported graft advice or succeeded: %v", err)
	}
}
