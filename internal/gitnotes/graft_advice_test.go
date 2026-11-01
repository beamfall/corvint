package gitnotes

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gitprobe"
)

// V1-3588: under GIT_GRAFT_FILE=/dev/null a Git that prints graft-file
// deprecation advice prints none into what this runner reads, including the
// stderr a failed command reports.
func TestGitRunnerSuppressesGraftAdvice(t *testing.T) {
	if !gitprobe.GraftAdvice("git") {
		t.Skip("this git prints no graft-file advice, so there is nothing to suppress")
	}
	r, err := open(fixtureRepo(t))
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.git(context.Background(), 1<<10, nil, "rev-parse", "--verify", "HEAD~1000") // past the root commit
	if err == nil || strings.Contains(err.Error(), "grafts is deprecated") {
		t.Fatalf("failed git reported graft advice or succeeded: %v", err)
	}
}
