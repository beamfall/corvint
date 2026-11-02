package sharedpath

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gitprobe"
)

// V1-3588: under GIT_GRAFT_FILE=/dev/null a Git that prints graft-file
// deprecation advice prints none on the stderr of the command run builds, and
// leaves the stdout run parses exact.
func TestGitRunnerSuppressesGraftAdvice(t *testing.T) {
	if !gitprobe.GraftAdvice("git") {
		t.Skip("this git prints no graft-file advice, so there is nothing to suppress")
	}
	f := newFixture(t)
	head := f.commit("one", map[string]string{"a.txt": "a\n"})
	g := git{ctx: context.Background(), root: f.root}
	out, code, err := g.run(nil, nil, "rev-list", "HEAD")
	if err != nil || code != 0 || string(out) != head+"\n" {
		t.Fatalf("rev-list HEAD = %q, %d, %v; want %q", out, code, err, head+"\n")
	}
	cmd := g.command(context.Background(), nil, "rev-list", "HEAD")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("rev-list HEAD: %v\n%s", err, stderr.String())
	}
	if strings.Contains(stderr.String(), "grafts is deprecated") || strings.Contains(stderr.String(), "hint:") {
		t.Fatalf("graft advice reached stderr: %q", stderr.String())
	}
}
