package behaviorfalsify

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gitprobe"
)

// V1-3588: gitBytes keeps Git's graft-file advice out of the stderr a failed
// read reports under GIT_GRAFT_FILE=/dev/null; resolving HEAD^ parses the commit,
// which is when Git prints the advice.
func TestGitBytesSuppressesGraftAdvice(t *testing.T) {
	if !gitprobe.GraftAdvice("git") {
		t.Skip("this git prints no graft-file advice, so there is nothing to suppress")
	}
	_, err := gitBytes(fixtureRepository(t), "rev-parse", "--verify", "HEAD^") // a root commit has no parent
	if err == nil || strings.Contains(err.Error(), "grafts is deprecated") {
		t.Fatalf("failed git read reported graft advice or succeeded: %v", err)
	}
}
