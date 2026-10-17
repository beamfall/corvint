package store

import (
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCDV0002_ScanIsBoundedAndPlain: the scan argv runs no hook,
// fsmonitor, signature program or first-parent walk and passes revisions
// after --end-of-options; an output over the bound is LIMIT_EXCEEDED; a
// second-parent constituent is scanned (TCD-V0-002, TCD-V0-003).
func TestTCDV0002_ScanIsBoundedAndPlain(t *testing.T) {
	args := CloseoutLogArgs("REF", "SINCE", 5)
	for _, want := range []string{"core.hooksPath=", "core.fsmonitor=false", "credential.helper=", "--no-show-signature", "--max-count=6"} {
		if !slices.ContainsFunc(args, func(a string) bool { return strings.HasPrefix(a, want) }) {
			t.Errorf("argv %v lacks %s", args, want)
		}
	}
	if slices.Contains(args, "--first-parent") || slices.Index(args, "--end-of-options") > slices.Index(args, "REF") || args[len(args)-1] != "^SINCE" {
		t.Errorf("argv %v", args)
	}

	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid"}, args...)...)
		c.Dir = root
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("commit", "-q", "--allow-empty", "-m", "base")
	run("checkout", "-q", "-b", "side")
	run("commit", "-q", "--allow-empty", "-m", "constituent\n\nRefs V1-1")
	run("checkout", "-q", "main")
	run("merge", "-q", "--no-ff", "-m", "Merge side", "side")
	head, err := CloseoutRevision(root, "main")
	if err != nil {
		t.Fatal(err)
	}
	scan, err := ScanCommits(root, head, "", 10)
	if err != nil || len(scan.Commits) != 3 || scan.Truncated || scan.Commits[0].Commit != head || scan.Commits[0].Subject != "Merge side" {
		t.Fatalf("scan %+v %v", scan, err)
	}
	if !slices.ContainsFunc(scan.Commits, func(c ScannedCommit) bool {
		return c.Subject == "constituent" && strings.Contains(c.Message, "Refs V1-1")
	}) {
		t.Fatalf("second-parent constituent not scanned: %+v", scan.Commits)
	}
	prev := MaxCloseoutScanBytes
	MaxCloseoutScanBytes = 64
	t.Cleanup(func() { MaxCloseoutScanBytes = prev })
	if _, err := ScanCommits(root, head, "", 10); wire.CodeOf(err) != wire.CodeLimitExceeded {
		t.Fatalf("over-bound scan: %v", err)
	}
	if _, err := CloseoutRevision(root, "no-such"); wire.CodeOf(err) != wire.CodeMalformed {
		t.Fatalf("unresolvable revision: %v", err)
	}
}
