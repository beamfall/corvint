package genesis

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gitancestry/ancestrytest"
)

// EEP-V0-771: the guidance snapshot's merge-base read stays unguarded because opening (and every
// Check) already refuses a grafted or shallow checkout.
func TestOpenGuidanceRefusesRewrittenAncestry_EEPV0771(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"-c", "user.name=F", "-c", "user.email=f@example.invalid", "commit", "-q", "--allow-empty", "-m", "one"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	if _, err := OpenGuidance(context.Background(), root); err != nil {
		t.Fatalf("plain checkout refused: %v", err)
	}
	for name, apply := range map[string]func() func(){
		"info/grafts": func() func() { return ancestrytest.Graft(t, root, "HEAD") },
		"shallow":     func() func() { return ancestrytest.Shallow(t, root, "HEAD") },
	} {
		undo := apply()
		if _, err := OpenGuidance(context.Background(), root); err == nil || !strings.Contains(err.Error(), "unsupported ancestry metadata: "+name) {
			t.Errorf("%s checkout opened: %v", name, err)
		}
		undo()
	}
}
