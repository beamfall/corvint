package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// gitOutput runs git in root and returns its stdout.
func gitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

// TestCTWV0002_TicketCreateFromAFeatureBranch is the V1-0325 acceptance at the
// command surface, on a real Git repository: from a primary on a feature
// branch, `ticket create` first refuses with the exact fix, then, once a
// linked worktree holds the intent branch, succeeds from either checkout
// without switching the primary, and reads list the ticket.
func TestCTWV0002_TicketCreateFromAFeatureBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	git(t, r.Root, "init", "-q", "-b", "main")
	git(t, r.Root, "config", "user.email", "fixture@example.invalid")
	git(t, r.Root, "config", "user.name", "Fixture")
	git(t, r.Root, "add", ".taskman")
	git(t, r.Root, "commit", "-q", "-m", "queue")
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	git(t, r.Root, "switch", "-q", "-c", "feature")

	refused := atm(t, r.Root, nil, "ticket", "create", "--request-id", "from-feature", "--issued-at", "2026-10-04T12:00:00Z", "--payload", createPayloadJSON)
	if refused.res.Outcome == wire.OutcomeOK || !hasCode(refused.res, wire.CodeIntentBranchMismatch) {
		t.Fatalf("feature-branch create without an intent worktree: %+v", refused.res)
	}
	linked := r.Root + "-main"
	if want := `worktree add \"` + linked + `\" main`; !strings.Contains(string(refused.stdout), want) {
		t.Fatalf("refusal lacks the fix %s:\n%s", want, refused.stdout)
	}

	git(t, r.Root, "worktree", "add", "-q", linked, "main")
	head := gitOutput(t, r.Root, "rev-parse", "--abbrev-ref", "HEAD")
	status := gitOutput(t, r.Root, "status", "--porcelain=v1", "--untracked-files=all")
	index, err := os.ReadFile(filepath.Join(r.CommonDir, "index"))
	if err != nil {
		t.Fatal(err)
	}
	first := createTicket(t, r.Root, "from-feature")
	second := createTicket(t, linked, "from-linked")
	if head != gitOutput(t, r.Root, "rev-parse", "--abbrev-ref", "HEAD") || head != "feature\n" {
		t.Fatalf("primary branch moved from %q", head)
	}
	if status != gitOutput(t, r.Root, "status", "--porcelain=v1", "--untracked-files=all") {
		t.Fatal("primary working tree changed")
	}
	if after, _ := os.ReadFile(filepath.Join(r.CommonDir, "index")); string(after) != string(index) {
		t.Fatal("primary index changed")
	}
	pending := gitOutput(t, linked, "status", "--porcelain=v1", "--untracked-files=all")
	for _, id := range []string{first, second} {
		local := id[strings.LastIndex(id, ":")+1:]
		if !strings.Contains(pending, "?? .taskman/tickets/"+local+".json") {
			t.Errorf("%s is not pending in the intent worktree:\n%s", id, pending)
		}
	}
	list := atm(t, r.Root, nil, "ticket", "list")
	if list.res.Outcome != wire.OutcomeOK || len(list.res.Items) != 2 {
		t.Fatalf("ticket list from the feature branch: %+v", list.res)
	}
}
