package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCTV0001_SkeletonIsTheCompletePayload is V1-2703 acceptance criterion
// 1: --template --skeleton prints exactly the complete CREATE payload, every
// closed key present with nulls, empty arrays and the native source, and
// filling only the title makes ticket create accept it.
func TestTCTV0001_SkeletonIsTheCompletePayload(t *testing.T) {
	r := receiptFixture(t)
	state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	res := templateRun(t, r.Root, "--skeleton")
	fixture.AssertUntouched(t, r, state, intents, "ticket create --template --skeleton")
	if res.Outcome != wire.OutcomeOK || len(res.Items) != 1 {
		t.Fatalf("skeleton: %+v", res)
	}
	sk := res.Items[0]
	keys := sk.Obj.SortedKeys()
	if len(keys) != len(mutation.PayloadKeys[mutation.OpCreate]) {
		t.Fatalf("skeleton keys = %v", keys)
	}
	for _, k := range mutation.PayloadKeys[mutation.OpCreate] {
		if _, ok := sk.Obj.Get(k); !ok {
			t.Errorf("skeleton omits %s", k)
		}
	}
	for _, k := range []string{"owner", "milestone", "dueDate", "estimateMinutes", "supersedes", "supersededBy"} {
		if v, _ := sk.Obj.Get(k); v.Kind != wire.KindNull {
			t.Errorf("%s = %s, want null", k, wire.Encode(v))
		}
	}
	src := field(sk, "source")
	if field(src, "kind").Str != "NATIVE" || field(src, "sourceQueueId").Str == "" || field(src, "sourceItemId").Kind != wire.KindNull || field(src, "sourceRevisionSha256").Kind != wire.KindNull {
		t.Fatalf("source = %s", wire.Encode(src))
	}
	full := templateRun(t, r.Root)
	if got, want := string(wire.Encode(sk)), field(full.Items[0], "payloadCanonical").Str; got != want {
		t.Fatalf("skeleton differs from the template payload:\n%s\n%s", got, want)
	}
	sk.Obj.Set("title", wire.String("Skeleton ticket"))
	created := atm(t, r.Root, wire.Encode(sk), "ticket", "create", "--request-id", "sk-1", "--issued-at", "2026-10-04T12:00:00Z", "--payload-stdin")
	if created.res.Outcome != wire.OutcomeOK {
		t.Fatalf("skeleton with only a title refused: %+v", created.res)
	}
	bad := atm(t, r.Root, nil, "ticket", "create", "--skeleton", "--request-id", "x", "--payload", "{}")
	if bad.res.Outcome == wire.OutcomeOK || !strings.Contains(strings.Join(bad.res.Warnings, " "), "--skeleton is accepted only with --template") {
		t.Fatalf("--skeleton without --template: %+v", bad.res)
	}
}

// TestTCTV0002_MalformedCreateNamesEveryKey is V1-2703 acceptance criterion
// 3: one MALFORMED names every missing, unknown and invalid key with its
// expected shape, while the closed contract still refuses the payload.
func TestTCTV0002_MalformedCreateNamesEveryKey(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	// The reporter's case: nullable keys and source omitted, plus a bad
	// priority, a non-decimal order and an unknown key.
	payload := `{"title":"t","body":null,"kind":"FEATURE","milestone":null,"priority":"P9","order":"zero",` +
		`"labels":[],"dependencies":[],"acceptanceCriteria":[],"requirementRefs":[],"effects":{"coverage":"INCOMPLETE","externalUnbounded":false,"resources":[],"touchPaths":[]},` +
		`"capabilities":[],"requiredGates":[],"executionClass":"AUTONOMOUS","dueDate":null,"colour":"red"}`
	got := atm(t, r.Root, nil, "ticket", "create", "--request-id", "diag-1", "--issued-at", "2026-10-04T12:00:00Z", "--payload", payload)
	if got.res.Outcome == wire.OutcomeOK || len(got.res.Codes) != 1 || got.res.Codes[0] != wire.CodeMalformed {
		t.Fatalf("diagnosis outcome: %+v", got.res)
	}
	msg := strings.Join(got.res.Warnings, " ")
	for _, want := range []string{
		"missing estimateMinutes: expected Count decimal string or null; skeleton default null",
		"missing owner:", "missing supersededBy:", "missing supersedes:",
		`missing source: expected object {kind, sourceItemId, sourceQueueId, sourceRevisionSha256}; skeleton default {"kind":"NATIVE","sourceItemId":null,"sourceQueueId":"queue:acme:main","sourceRevisionSha256":null}`,
		"unknown key colour", "invalid priority (", "expected enum [", "invalid order (",
		"CREATE payload has 8 problem(s)", "--template --skeleton",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("diagnosis lacks %q:\n%s", want, msg)
		}
	}
	for _, valid := range []string{"invalid title", "invalid kind", "invalid labels", "invalid effects"} {
		if strings.Contains(msg, valid) {
			t.Errorf("diagnosis flags a valid key (%s):\n%s", valid, msg)
		}
	}
	// A payload that is not an object keeps the codec's own error.
	arr := atm(t, r.Root, nil, "ticket", "create", "--request-id", "diag-2", "--issued-at", "2026-10-04T12:00:00Z", "--payload", "[]")
	if arr.res.Outcome == wire.OutcomeOK || strings.Contains(strings.Join(arr.res.Warnings, " "), "problem(s)") {
		t.Fatalf("non-object payload: %+v", arr.res)
	}
}
