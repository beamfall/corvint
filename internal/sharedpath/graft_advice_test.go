package sharedpath

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gitprobe"
)

// V1-3588: under GIT_GRAFT_FILE=/dev/null a Git that prints graft-file
// deprecation advice leaves the stdout this runner parses exact.
func TestGitRunnerReadsStdoutOnlyUnderGraftAdvice(t *testing.T) {
	if !gitprobe.GraftAdvice("git") {
		t.Skip("this git prints no graft-file advice, so there is nothing to suppress")
	}
	f := newFixture(t)
	head := f.commit("one", map[string]string{"a.txt": "a\n"})
	out, code, err := git{ctx: context.Background(), root: f.root}.run(nil, nil, "rev-list", "HEAD")
	if err != nil || code != 0 || string(out) != head+"\n" {
		t.Fatalf("rev-list HEAD = %q, %d, %v; want %q", out, code, err, head+"\n")
	}
	if strings.Contains(string(out), "hint:") {
		t.Fatalf("graft advice reached stdout: %q", out)
	}
}
