package cli

import (
	"bytes"
	"io"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const oneReadPayload = `{"acceptanceCriteria":["it exists"],"body":null,"capabilities":[],` +
	`"dependencies":[],"dueDate":null,"effects":{"coverage":"QUALIFIED","externalUnbounded":false,` +
	`"resources":[],"touchPaths":[]},"estimateMinutes":null,"executionClass":"AUTONOMOUS",` +
	`"kind":"FEATURE","labels":[],"milestone":"r1","order":"0","owner":null,"priority":"P2",` +
	`"requiredGates":[],"requirementRefs":[],"source":{"kind":"NATIVE","sourceItemId":null,` +
	`"sourceQueueId":"queue:acme:main","sourceRevisionSha256":null},"supersededBy":null,` +
	`"supersedes":null,"title":"Release member"}`

// TestTCLV0736_ReadinessReadsTheStoreOnce pins V1-4148: release readiness
// takes its member counts and its lifecycle counts from one store read. A
// write lands after the read's first body, so the read retries; both counts
// then come from the retried read. Before TCL-V0-736 the member counts came
// from a separate earlier intent load and still showed the pre-write
// priority while the lifecycle read saw the new store.
func TestTCLV0736_ReadinessReadsTheStoreOnce(t *testing.T) {
	fixture.SkipUnsupportedPlatform(t)
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	run := func(stdin string, args ...string) *wire.Result {
		t.Helper()
		var out bytes.Buffer
		Run(Env{Cwd: r.Root, Args: args, Stdin: bytes.NewReader([]byte(stdin)), Stdout: &out, Stderr: io.Discard})
		res, err := wire.DecodeResult(out.Bytes())
		if err != nil || res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %v\n%s", args, err, out.Bytes())
		}
		return res
	}
	run("", "init")
	created := run("", "ticket", "create", "--request-id", "member", "--issued-at", "2026-11-01T12:00:00Z", "--payload", oneReadPayload)
	id, _ := created.Items[0].Obj.Get("ticketId")
	run("", "release", "create", "--request-id", "r1", "--target", "r1", "--issued-at", "2026-11-01T12:01:00Z", "--payload",
		`{"acceptanceCriteria":["release criterion"],"predecessorReleaseIds":[],"requiredGates":[],"ticketIds":["`+id.Str+`"],"title":"R1","version":"1"}`)

	calls := 0
	res := releaseRead(Env{Cwd: r.Root, afterRead: func() {
		calls++
		if calls == 1 {
			run("", "ticket", "prioritize", "--request-id", "p0", "--target", id.Str, "--expected-revision", "1", "--issued-at", "2026-11-01T12:02:00Z", "--payload", `{"order":"0","priority":"P0"}`)
		}
	}}, "readiness", []string{"r1"})
	if res.Outcome != wire.OutcomeOK || calls < 2 {
		t.Fatalf("calls=%d readiness=%+v", calls, res)
	}
	counts, _ := res.Items[0].Obj.Get("memberCounts")
	byPriority, _ := counts.Obj.Get("byPriority")
	p0, _ := byPriority.Obj.Get("P0")
	if p0.Str != "1" {
		t.Fatalf("member counts were not taken from the retried read: %s", wire.EncodeFile(res.Items[0]))
	}
}
