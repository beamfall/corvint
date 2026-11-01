package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestATRV0801_CompiledBinaryAdmitsDistinctBuildLogFiles is the compiled
// native qualification of V1-3185: on a scratch fixture queue, the built
// corvint-tasks binary admits two live attempts whose tickets declare
// distinct exact build-log files, while a third ticket declaring one of those
// files and a fourth declaring the parent directory both refuse
// RESOURCE_COLLISION, the directory refusal naming the set-effects next
// action (ATR-V0-800, ATR-V0-801).
func TestATRV0801_CompiledBinaryAdmitsDistinctBuildLogFiles(t *testing.T) {
	binary := cli.CompiledTasksBinary(t)
	t.Setenv("CORVINT_TASKS", binary)
	t.Setenv("CORVINT_TASKS_ACTOR", "scratch")
	t.Setenv("ATM_ACTOR", "scratch")
	dir := filepath.Join(t.TempDir(), "q")
	if out, err := exec.Command(scratchQueueHelper, dir).CombinedOutput(); err != nil {
		t.Fatalf("helper: %v %s", err, out)
	}
	var raw []byte
	call := func(args ...string) *wire.Result {
		t.Helper()
		c := exec.Command(binary, args...)
		c.Dir = dir
		raw, _ = c.Output()
		result, err := wire.DecodeResult(raw)
		if err != nil {
			t.Fatalf("%v: %v %s", args, err, raw)
		}
		return result
	}
	payload, err := os.ReadFile("testdata/external-agents/ticket-create.json")
	if err != nil {
		t.Fatal(err)
	}
	ticket := func(name, path string) string {
		t.Helper()
		p := strings.NewReplacer(
			`"sourceQueueId":"queue:example:main"`, `"sourceQueueId":"queue:scratch:main"`,
			`"title":"Verify repository setup"`, `"title":"`+name+`"`,
			`"coverage":"INCOMPLETE"`, `"coverage":"QUALIFIED"`,
			`"touchPaths":[]`, `"touchPaths":["`+path+`"]`,
		).Replace(string(payload))
		x := call("ticket", "create", "--request-id", "create-"+name, "--payload", p)
		if x.Outcome != wire.OutcomeOK {
			t.Fatalf("ticket create %s: %+v", name, x)
		}
		return field(x.Items[0], "ticketId").Str
	}
	claim := func(name, id string) *wire.Result {
		t.Helper()
		return call("claim", id, "--holder", "scratch-"+name, "--stage", "implement", "--request-id", "claim-"+name)
	}
	a := ticket("a", "docs/build-log/2026-11-01-a.md")
	b := ticket("b", "docs/build-log/2026-11-01-b.md")
	same := ticket("same", "docs/build-log/2026-11-01-a.md")
	parent := ticket("parent", "docs/build-log/")
	for name, id := range map[string]string{"a": a, "b": b} {
		x := claim(name, id)
		if x.Outcome != wire.OutcomeOK {
			t.Fatalf("claim %s on a distinct build-log file: %+v", name, x)
		}
		if src := field(field(x.Items[0], "scope"), "source").Str; src != "DECLARED" {
			t.Fatalf("claim %s scope source = %q, want DECLARED", name, src)
		}
	}
	const hint = "declare exact file touchPaths with ticket set-effects before claiming (ATR-V0-800)"
	for name, id := range map[string]string{"same": same, "parent": parent} {
		x := claim(name, id)
		if !hasCode(x, wire.CodeResourceCollision) {
			t.Fatalf("claim %s: %+v, want RESOURCE_COLLISION", name, x)
		}
		if (name == "parent") != strings.Contains(string(raw), hint) {
			t.Fatalf("claim %s result %s: the set-effects hint must be present only on the directory claim", name, raw)
		}
	}
}
