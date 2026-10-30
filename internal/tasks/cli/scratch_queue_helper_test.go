package cli_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const scratchQueueHelper = "../../../script/tasks-scratch-queue.sh"

// TestTCDV0183_ScratchQueueHelperIsExecutionReady (V1-2834): the documented
// helper makes a fixture scratch queue on which a ticket is created and
// claimed with --scope-from-diff at once, with no cutover step; a non-empty
// target refuses with exit 2 and writes nothing; and a queue initialized from
// the same templates without the fixture edit still refuses the claim with
// CUTOVER_MISSING, so the live execution cutover gate is unchanged.
func TestTCDV0183_ScratchQueueHelperIsExecutionReady(t *testing.T) {
	binary := cli.CompiledTasksBinary(t)
	t.Setenv("CORVINT_TASKS", binary)
	t.Setenv("CORVINT_TASKS_ACTOR", "scratch")
	t.Setenv("ATM_ACTOR", "scratch")
	dir := filepath.Join(t.TempDir(), "q")

	out, err := exec.Command(scratchQueueHelper, dir).CombinedOutput()
	if err != nil {
		t.Fatalf("helper: %v %s", err, out)
	}
	if got, want := strings.TrimSpace(string(out)), "READY "+dir+" queue:scratch:main fixture:true"; got != want {
		t.Fatalf("helper output = %q, want %q", got, want)
	}
	call := func(root string, args ...string) *wire.Result {
		t.Helper()
		c := exec.Command(binary, args...)
		c.Dir = root
		raw, _ := c.Output()
		result, decodeErr := wire.DecodeResult(raw)
		if decodeErr != nil {
			t.Fatalf("%v: %v %s", args, decodeErr, raw)
		}
		return result
	}
	payload, err := os.ReadFile("testdata/external-agents/ticket-create.json")
	if err != nil {
		t.Fatal(err)
	}
	scratchPayload := strings.Replace(string(payload), `"sourceQueueId":"queue:example:main"`, `"sourceQueueId":"queue:scratch:main"`, 1)
	created := call(dir, "ticket", "create", "--request-id", "t1", "--payload", scratchPayload)
	if created.Outcome != wire.OutcomeOK {
		t.Fatalf("ticket create: %+v", created)
	}
	id := field(created.Items[0], "ticketId").Str
	if !strings.HasPrefix(id, "ticket:scratch:main:SCR-") {
		t.Fatalf("ticket id %q lacks the scratch prefix", id)
	}
	base := revParse(t, dir, "HEAD")
	fixture.Write(t, filepath.Join(dir, "src", "a.txt"), []byte("a\n"))
	git(t, dir, "add", "src")
	git(t, dir, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "change")
	claim := call(dir, "claim", id, "--holder", "scratch", "--stage", "implement", "--request-id", "c1", "--base", base, "--scope-from-diff", "HEAD")
	if claim.Outcome != wire.OutcomeOK {
		t.Fatalf("claim on the scratch queue: %+v", claim)
	}
	scope := field(claim.Items[0], "scope")
	if src := field(scope, "source").Str; src != "DERIVED" {
		t.Fatalf("scope source = %q, want DERIVED", src)
	}

	// A non-empty target refuses before writing anything.
	busy := t.TempDir()
	fixture.Write(t, filepath.Join(busy, "keep"), []byte("x"))
	err = exec.Command(scratchQueueHelper, busy).Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 {
		t.Fatalf("non-empty target: err = %v, want exit 2", err)
	}
	if _, statErr := os.Stat(filepath.Join(busy, ".taskman")); !os.IsNotExist(statErr) {
		t.Fatalf("refused helper wrote .taskman: %v", statErr)
	}

	// The unedited templates (fixture:false) still need the execution cutover.
	live := t.TempDir()
	git(t, live, "init", "-q", "-b", "main")
	git(t, live, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "--allow-empty", "-m", "base")
	for _, name := range []string{"queue.json", "policy.json"} {
		body, readErr := os.ReadFile(filepath.Join("testdata/external-agents", name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		fixture.Write(t, filepath.Join(live, ".taskman", name), body)
	}
	if x := call(live, "init", "--request-id", "i1"); x.Outcome != wire.OutcomeOK {
		t.Fatalf("live init: %+v", x)
	}
	created = call(live, "ticket", "create", "--request-id", "t1", "--payload", string(payload))
	if created.Outcome != wire.OutcomeOK {
		t.Fatalf("live ticket create: %+v", created)
	}
	refused := call(live, "claim", field(created.Items[0], "ticketId").Str, "--holder", "scratch", "--stage", "implement", "--request-id", "c1")
	if !hasCode(refused, wire.CodeCutoverMissing) {
		t.Fatalf("live claim without cutover: %+v, want CUTOVER_MISSING", refused)
	}
}
