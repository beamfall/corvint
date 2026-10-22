package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CTS-V0-013 (V1-1660): a `git clone --shared` checkout, whose objects live
// behind objects/info/alternates, is a supported Tasks repository. init,
// a ticket create and the inventory reads succeed there with no
// UNSUPPORTED or UNSUPPORTED_FILESYSTEM refusal.
func TestCTSV0013_SharedCloneIsSupported(t *testing.T) {
	r := exclusionCLIRepo(t)
	git(t, r.Root, "add", "-A")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "intent store")
	clone := filepath.Join(filepath.Dir(r.Root), "shared-clone")
	if out, err := exec.Command("git", "clone", "-q", "--shared", r.Root, clone).CombinedOutput(); err != nil {
		t.Fatalf("git clone --shared: %s %v", out, err)
	}
	if _, err := os.Stat(filepath.Join(clone, ".git", "objects", "info", "alternates")); err != nil {
		t.Fatalf("the fixture clone has no alternates file: %v", err)
	}
	if x := atm(t, clone, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init in a --shared clone: %s", x.stdout)
	}
	planTicket(t, clone, "shared", "P2", `["shared"]`)
	for _, args := range [][]string{{"queue", "status"}, {"ticket", "list"}, {"receipt", "audit"}, {"doctor"}} {
		if x := atm(t, clone, nil, args...); x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v in a --shared clone: %s", args, x.stdout)
		}
	}
}
