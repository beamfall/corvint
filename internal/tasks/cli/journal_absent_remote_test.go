package cli

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func realGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// TestCTSV0821_JournalAbsentMissNamesOriginSyncStep: a lane clone whose
// .taskman predates a ticket its origin holds gets a refusal naming the
// origin commit, the read-only git show and the ff-only sync step; a ticket
// origin lacks too names git fetch. The refusal shape of CTS-V0-010 holds.
func TestCTSV0821_JournalAbsentMissNamesOriginSyncStep(t *testing.T) {
	fixture.SkipUnsupportedPlatform(t)
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(base, "primary")
	realGit(t, base, "init", "-q", "-b", "main", primary)
	fixture.Write(t, filepath.Join(primary, ".taskman", "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(primary, ".taskman", "policy.json"), fixture.PolicyBytes())
	realGit(t, primary, "add", ".taskman")
	realGit(t, primary, "commit", "-qm", "queue and policy")
	before := realGit(t, primary, "rev-parse", "HEAD")
	if got := projectionRead(t, Env{Cwd: primary, Args: []string{"init"}}); got.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", got)
	}
	payload := `{"acceptanceCriteria":["it exists"],"body":null,"capabilities":[],"dependencies":[],"dueDate":null,` +
		`"effects":{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":[]},"estimateMinutes":null,` +
		`"executionClass":"AUTONOMOUS","kind":"FEATURE","labels":[],"milestone":null,"order":"0","owner":null,"priority":"P2",` +
		`"requiredGates":[],"requirementRefs":[],"source":{"kind":"NATIVE","sourceItemId":null,"sourceQueueId":"queue:acme:main",` +
		`"sourceRevisionSha256":null},"supersededBy":null,"supersedes":null,"title":"Created after the lane cloned"}`
	created := projectionRead(t, Env{Cwd: primary, Args: []string{"ticket", "create", "--request-id", "after-clone", "--issued-at", "2026-11-01T12:00:00Z", "--payload", payload}})
	if created.Outcome != wire.OutcomeOK {
		t.Fatalf("create: %+v", created)
	}
	id, _ := created.Items[0].Obj.Get("ticketId")
	local := id.Str[strings.LastIndexByte(id.Str, ':')+1:]
	realGit(t, primary, "add", ".taskman")
	realGit(t, primary, "commit", "-qm", "ticket")
	after := realGit(t, primary, "rev-parse", "HEAD")

	// The lane clone's checkout predates the ticket; its origin/main holds it.
	clone := filepath.Join(base, "clone")
	realGit(t, base, "clone", "-q", primary, clone)
	realGit(t, clone, "reset", "-q", "--hard", before)

	got := projectionRead(t, Env{Cwd: clone, Args: []string{"ticket", "show", id.Str}})
	text := strings.Join(got.Warnings, " ")
	if got.Outcome != wire.OutcomeRefused || len(got.Items) != 0 || got.Snapshot != nil || len(got.Codes) != 0 {
		t.Fatalf("clone read: %+v", got)
	}
	rel := ".taskman/tickets/" + local + ".json"
	for _, want := range []string{"is not in this checkout's intent projection", "origin/", after,
		"`git show " + after + ":" + rel + "`", "`git merge --ff-only origin/"} {
		if !strings.Contains(text, want) {
			t.Errorf("clone refusal lacks %q: %s", want, text)
		}
	}
	// The named read-only step works as written.
	if out := realGit(t, clone, "show", after+":"+rel); !strings.Contains(out, "Created after the lane cloned") {
		t.Fatalf("named git show: %s", out)
	}

	miss := projectionRead(t, Env{Cwd: clone, Args: []string{"ticket", "show", "ZZ-999"}})
	text = strings.Join(miss.Warnings, " ")
	if miss.Outcome != wire.OutcomeRefused || !strings.Contains(text, "does not hold .taskman/tickets/ZZ-999.json either") || !strings.Contains(text, "`git fetch origin`") {
		t.Fatalf("absent on origin too: %+v", miss)
	}
}
