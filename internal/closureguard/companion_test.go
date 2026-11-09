package closureguard

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// CTF-V0-007, CTF-V0-010: the test-runner companion entry is not linked into core `corvint`.
// The other folded groups left with the E2E proof lab (decision 0593).
func TestCTFV0007CoreLinksNoFoldedGroup(t *testing.T) {
	_, foldedFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	foldedRoot := filepath.Dir(filepath.Dir(filepath.Dir(foldedFile)))
	command := exec.Command("go", "list", "-deps", "./cmd/corvint")
	command.Dir = foldedRoot
	out, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, dep := range strings.Fields(string(out)) {
		if strings.HasSuffix(dep, "/internal/testrunner/cli") {
			t.Fatalf("core corvint imports %s", dep)
		}
	}
}
