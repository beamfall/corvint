package cli_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// poolStatusRepo returns a pool db whose member a is ALLOCATED, b is
// QUARANTINED after its attempt ended and review is FREE. Member a runs a
// passing health command at claim, so its retained outcome is observable.
func poolStatusRepo(t *testing.T) (*fixture.Repo, string, string) {
	t.Helper()
	r := exclusionCLIRepo(t)
	raw, err := os.ReadFile(filepath.Join(r.IntentDir, "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	policy, err := wire.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	pools, _ := policy.Obj.Get("pools")
	health := wire.ObjectValue(wire.NewObject().Set("argv", wire.Strings([]string{"/usr/bin/true"})).Set("cwd", wire.String("REPOSITORY")).Set("env", wire.Array()).Set("timeoutSeconds", wire.String("10")))
	pools.Arr[0].Obj.Set("memberConfig", wire.ObjectValue(wire.NewObject().Set("a", wire.ObjectValue(wire.NewObject().Set("health", health)))))
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(policy))
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatal(x.res)
	}
	held := planTicket(t, r.Root, "held", "P1", `["held"]`)
	ended := planTicket(t, r.Root, "ended", "P2", `["ended"]`)
	a := atm(t, r.Root, nil, "claim", held, "--holder", "builder", "--request-id", "claim-a", "--pool", "db", "--stage", "implement", "--exclude-member", "b", "--exclude-member", "review")
	if a.res.Outcome != wire.OutcomeOK || field(field(a.res.Items[0], "poolAllocation"), "memberId").Str != "a" {
		t.Fatalf("claim a %s", a.stdout)
	}
	b := atm(t, r.Root, nil, "claim", ended, "--holder", "other", "--request-id", "claim-b", "--pool", "db", "--stage", "implement", "--exclude-member", "a", "--exclude-member", "review")
	item := b.res.Items[0]
	if b.res.Outcome != wire.OutcomeOK || field(field(item, "poolAllocation"), "memberId").Str != "b" {
		t.Fatalf("claim b %s", b.stdout)
	}
	if x := atm(t, r.Root, nil, "release", "--attempt", field(item, "attemptId").Str, "--generation", field(item, "generation").Str, "--request-id", "release-b"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("release b %s", x.stdout)
	}
	return r, field(a.res.Items[0], "attemptId").Str, field(item, "attemptId").Str
}

func poolStatusMembers(t *testing.T, root string, args ...string) map[string]wire.Value {
	t.Helper()
	x := atm(t, root, nil, append([]string{"pool", "status"}, args...)...)
	if x.res.Outcome != wire.OutcomeOK || x.res.Snapshot == nil {
		t.Fatalf("pool status %v: %s", args, x.stdout)
	}
	it := x.res.Items[0]
	if field(it, "profile").Str != "taskman-pool-status/0" || field(it, "mutationAuthority").Bool {
		t.Fatalf("envelope item %s", x.stdout)
	}
	out := map[string]wire.Value{}
	for _, p := range field(it, "pools").Arr {
		for _, m := range field(p, "members").Arr {
			out[field(p, "poolId").Str+"/"+field(m, "memberId").Str] = m
		}
	}
	return out
}

// PSR-V0-013: allocated, quarantined and idle members report state,
// allocation, holder/attempt/generation, quarantine reason and the retained
// health/cleanup outcome; unrecorded facts are NOT_OBSERVED, never guessed.
func TestPSRV0013_PoolStatusMembers(t *testing.T) {
	r, heldAttempt, endedAttempt := poolStatusRepo(t)
	members := poolStatusMembers(t, r.Root)
	if len(members) != 3 {
		t.Fatalf("members %v", members)
	}
	a := members["db/a"]
	if field(a, "state").Str != "ALLOCATED" || field(a, "attemptId").Str != heldAttempt || field(a, "generation").Str != "1" || field(a, "holder").Str != "builder" || field(a, "allocationId").Str == "" || field(a, "quarantine").Kind != wire.KindNull {
		t.Fatalf("allocated %s", wire.Encode(a))
	}
	health := field(a, "lastHealth")
	if health.Kind != wire.KindObject || field(health, "class").Str != "EXIT_ZERO" || !field(health, "passed").Bool || field(health, "observedAt").Str != "NOT_OBSERVED" || field(health, "observationSha256").Str != field(a, "observationSha256").Str {
		t.Fatalf("health outcome %s", wire.Encode(a))
	}
	if field(a, "lastCleanup").Str != "NOT_OBSERVED" {
		t.Fatalf("cleanup guessed %s", wire.Encode(a))
	}
	// The claim's health command finished; its retained kind is not pending.
	if field(a, "commandKind").Kind != wire.KindNull || field(members["db/b"], "commandKind").Kind != wire.KindNull {
		t.Fatalf("finished command reported pending %s %s", wire.Encode(a), wire.Encode(members["db/b"]))
	}
	b := members["db/b"]
	q := field(b, "quarantine")
	if field(b, "state").Str != "QUARANTINED" || field(b, "attemptId").Str != endedAttempt || field(b, "holder").Str != "other" || q.Kind != wire.KindObject || !strings.Contains(field(q, "reason").Str, "physical safe reuse unproved") || field(q, "changedSeq").Str != field(b, "changedSeq").Str || field(q, "since").Str != "NOT_OBSERVED" {
		t.Fatalf("quarantined %s", wire.Encode(b))
	}
	if field(b, "lastHealth").Str != "NOT_OBSERVED" || field(b, "lastCleanup").Str != "NOT_OBSERVED" || field(b, "observationSha256").Kind != wire.KindNull {
		t.Fatalf("quarantined outcome guessed %s", wire.Encode(b))
	}
	idle := members["db/review"]
	if field(idle, "state").Str != "FREE" || field(idle, "allocationId").Kind != wire.KindNull || field(idle, "attemptId").Kind != wire.KindNull || field(idle, "reservedFor").Str != "review" || field(idle, "lastHealth").Str != "NOT_OBSERVED" || field(idle, "lastCleanup").Str != "NOT_OBSERVED" {
		t.Fatalf("idle %s", wire.Encode(idle))
	}
}

// PSR-V0-014: --pool and --member filter the view; an unknown pool or member,
// a repeated or empty filter and an unknown flag refuse MALFORMED.
func TestPSRV0014_PoolStatusFilters(t *testing.T) {
	r, _, _ := poolStatusRepo(t)
	if got := poolStatusMembers(t, r.Root, "--pool", "db"); len(got) != 3 {
		t.Fatalf("pool filter %v", got)
	}
	got := poolStatusMembers(t, r.Root, "--member", "b")
	if len(got) != 1 || field(got["db/b"], "state").Str != "QUARANTINED" {
		t.Fatalf("member filter %v", got)
	}
	if got := poolStatusMembers(t, r.Root, "--pool", "db", "--member", "review"); len(got) != 1 || field(got["db/review"], "state").Str != "FREE" {
		t.Fatalf("both filters %v", got)
	}
	for _, args := range [][]string{
		{"--pool", "nope"}, {"--member", "nope"}, {"--pool", "db", "--member", "nope"},
		{"--pool"}, {"--pool", ""}, {"--pool", "db", "--pool", "db"}, {"--member", "a", "--member", "b"},
		{"--stage", "review"}, {"db"}, {"--pool", "bad\npool"},
	} {
		x := atm(t, r.Root, nil, append([]string{"pool", "status"}, args...)...)
		if x.res.Outcome != wire.OutcomeError || !hasCode(x.res, wire.CodeMalformed) {
			t.Fatalf("%v accepted: %s", args, x.stdout)
		}
	}
	x := atm(t, r.Root, nil, "pool", "status", "--member", "ghost")
	if !strings.Contains(strings.Join(x.res.Warnings, " "), "unknown pool member") {
		t.Fatalf("unclear refusal %s", x.stdout)
	}
	// The verb is discoverable from help, pool --help and its own help.
	if x := atm(t, r.Root, nil, "help"); !strings.Contains(string(wire.Encode(field(x.res.Items[0], "implemented"))), `"pool status"`) {
		t.Fatalf("help %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, "pool", "--help"); !strings.Contains(field(x.res.Items[0], "usage").Str, "status") {
		t.Fatalf("pool --help %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, "pool", "status", "--help"); !strings.Contains(field(x.res.Items[0], "usage").Str, "[--member ID]") || !strings.Contains(field(x.res.Items[0], "note").Str, "no lock") {
		t.Fatalf("pool status --help %s", x.stdout)
	}
}

// PSR-V0-015: pool status takes no lock and writes nothing. It succeeds
// while another process holds both the writer and the preparation lock,
// and leaves the state and intent trees byte-identical.
func TestPSRV0015_PoolStatusLockFreeAndWritesNothing(t *testing.T) {
	r, _, _ := poolStatusRepo(t)
	repo, err := intent.Resolve(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	writer, err := authority.AcquireLock(context.Background(), repo, authority.LockOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	prep := holdPreparation(t, repo)
	defer prep.Close()
	state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	for _, args := range [][]string{{}, {"--pool", "db"}, {"--member", "a"}, {"--member", "nope"}} {
		atm(t, r.Root, nil, append([]string{"pool", "status"}, args...)...)
	}
	poolStatusMembers(t, r.Root)
	if !fixture.SameTree(state, fixture.TreeSnapshot(t, r.StateDir)) || !fixture.SameTree(intents, fixture.TreeSnapshot(t, r.IntentDir)) {
		t.Fatal("pool status wrote")
	}
}
