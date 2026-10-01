package cli_test

import (
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// With CORVINT_HANDOFF_TEST_BINARY set this runs the actual candidate executable
// over disposable initialized fixture queues. All subprocesses are waited for.
func TestCALV0045_CLIConfiguredRetriesAndNoTreeHandoff(t *testing.T) {
	for _, limit := range []int{0, 4} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
			id := planTicket(t, root, "external", "P1", `["src/"]`)
			repo, e := intent.Resolve(root)
			if e != nil {
				t.Fatal(e)
			}
			runOK := func(args ...string) run {
				t.Helper()
				r := handoffCLI(t, root, args...)
				if r.code != 0 {
					t.Fatalf("%v: %s", args, r.stdout)
				}
				return r
			}
			loaded, e := intent.Load(repo.PrimaryWorktree)
			if e != nil {
				t.Fatal(e)
			}
			policy, e := wire.Parse(loaded.Policy.Raw)
			if e != nil {
				t.Fatal(e)
			}
			policy.Obj.Set("policyVersion", wire.String("2"))
			rt, _ := policy.Obj.Get("retries")
			rt.Obj.Set("admissionsPerRevision", wire.String(fmt.Sprint(limit)))
			policyDir, e := filepath.EvalSymlinks(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			policyFile := filepath.Join(policyDir, "policy.json")
			fixture.Write(t, policyFile, wire.EncodeFile(policy))
			runOK("policy", "update", "--request-id", "configure", "--expected-policy-version", "1", "--file", policyFile)
			preview := func(want string) {
				t.Helper()
				before := fixture.TreeSnapshot(t, repo.StateDir)
				r := runOK("plan", "preview", "--stage", "integrate")
				entries := field(r.res.Items[0], "entries").Arr
				if len(entries) != 1 || field(entries[0], "state").Str != want {
					t.Fatalf("preview: %s", r.stdout)
				}
				if !fixture.SameTree(before, fixture.TreeSnapshot(t, repo.StateDir)) {
					t.Fatal("preview wrote")
				}
			}
			var a, g string
			for i := 0; i <= limit; i++ {
				preview("SELECTED")
				args := []string{"claim", id, "--holder", "qualification", "--stage", "integrate", "--request-id", fmt.Sprintf("claim-%d", i)}
				if i%2 == 1 {
					args[1] = "--next"
				}
				r := runOK(args...)
				a = field(r.res.Items[0], "attemptId").Str
				g = field(r.res.Items[0], "generation").Str
				show := runOK("attempt", "show", a)
				if field(show.res.Items[0], "retryCount").Str != fmt.Sprint(i) {
					t.Fatalf("debt: %s", show.stdout)
				}
				if i == limit {
					// Actual external artifact: the queue tree is never submitted merely to
					// unlock accounting. The inert reference does not prove its contents.
					external := []byte("external review complete\n")
					fixture.Write(t, filepath.Join(t.TempDir(), "review.txt"), external)
					ref := "sha256:" + string(wire.Sum(external))
					args := []string{"release", "--attempt", a, "--generation", g, "--request-id", "handoff", "--reason", "HANDOFF", "--evidence", ref}
					runOK(args...)
					replay := runOK(args...)
					if !field(replay.res.Items[0], "replayed").Bool {
						t.Fatal("missing replay")
					}
					args[len(args)-1] = "local:changed"
					conflict := handoffCLI(t, root, args...)
					if !hasCode(conflict.res, wire.CodeRequestIDConflict) {
						t.Fatalf("changed reference: %s", conflict.stdout)
					}
					show = runOK("attempt", "show", a)
					if field(show.res.Items[0], "handoffEvidence").Str != ref || field(show.res.Items[0], "candidateTreeOid").Kind != wire.KindNull {
						t.Fatalf("no-tree record: %s", show.stdout)
					}
					status := runOK("queue", "status")
					if field(status.res.Items[0], "attempts").Str != "0" {
						t.Fatalf("reservation/live count: %s", status.stdout)
					}
					preview("SELECTED")
					r = runOK("claim", id, "--holder", "next", "--stage", "review", "--request-id", "after-handoff")
					a = field(r.res.Items[0], "attemptId").Str
					g = field(r.res.Items[0], "generation").Str
					show = runOK("attempt", "show", a)
					if field(show.res.Items[0], "retryCount").Str != fmt.Sprint(limit) {
						t.Fatal("handoff spent retry")
					}
				}
				runOK("release", "--attempt", a, "--generation", g, "--request-id", fmt.Sprintf("cancel-%d", i))
			}
			preview("BLOCKED")
			refused := handoffCLI(t, root, "claim", id, "--holder", "blocked", "--request-id", "exhausted")
			if !hasCode(refused.res, wire.CodeRetryExhausted) {
				t.Fatalf("exhaustion: %s", refused.stdout)
			}
			runOK("ticket", "reopen", "--request-id", "recover", "--target", id, "--expected-revision", "1", "--payload", `{"reason":"safe owner readmission"}`)
			preview("SELECTED")
			audit := runOK("receipt", "audit")
			if field(audit.res.Items[0], "structuralConsistency").Str != "CONSISTENT" {
				t.Fatalf("audit: %s", audit.stdout)
			}
			if output := os.Getenv("CORVINT_TASKS_QUALIFICATION_OUTPUT"); output != "" {
				if e := os.WriteFile(filepath.Join(output, fmt.Sprintf("configured-%d-audit.json", limit)), audit.stdout, 0600); e != nil {
					t.Fatal(e)
				}
			}
		})
	}
}

func TestCALV0046_CLICompatibility(t *testing.T) {
	legacy := os.Getenv("CORVINT_TASKS_LEGACY_BINARY")
	if legacy == "" {
		t.Skip("exact previous binary must be supplied for cross-version qualification")
	}
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	id := planTicket(t, root, "legacy", "P1", `["src/"]`)
	repo, e := intent.Resolve(root)
	if e != nil {
		t.Fatal(e)
	}
	ok := func(r run) run {
		t.Helper()
		if r.code != 0 {
			t.Fatalf("CLI refused: %s", r.stdout)
		}
		return r
	}
	a := ok(handoffCLIWithBinary(t, root, legacy, "claim", id, "--holder", "legacy", "--stage", "review", "--request-id", "legacy-claim"))
	attempt, generation := field(a.res.Items[0], "attemptId").Str, field(a.res.Items[0], "generation").Str
	release := []string{"release", "--attempt", attempt, "--generation", generation, "--request-id", "legacy-release", "--reason", "GATE_FAILED"}
	ok(handoffCLIWithBinary(t, root, legacy, release...))
	replay := ok(handoffCLI(t, root, release...))
	if !field(replay.res.Items[0], "replayed").Bool {
		t.Fatal("old RELEASE did not replay")
	}
	a = ok(handoffCLI(t, root, "claim", id, "--holder", "new", "--stage", "review", "--request-id", "new-claim"))
	attempt, generation = field(a.res.Items[0], "attemptId").Str, field(a.res.Items[0], "generation").Str
	ok(handoffCLI(t, root, "release", "--attempt", attempt, "--generation", generation, "--request-id", "new-handoff", "--reason", "REVIEW_RETURNED", "--evidence", "local:outside-review"))
	before := fixture.TreeSnapshot(t, root)
	old := handoffCLIWithBinary(t, root, legacy, "plan", "preview")
	if old.code == 0 {
		t.Fatalf("old planner silently accepted no-tree metadata: %s", old.stdout)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, root)) {
		t.Fatal("old reader changed store")
	}
	ok(handoffCLI(t, root, "receipt", "audit"))
	loaded, e := intent.Load(repo.PrimaryWorktree)
	if e != nil {
		t.Fatal(e)
	}
	policy, e := wire.Parse(loaded.Policy.Raw)
	if e != nil {
		t.Fatal(e)
	}
	policy.Obj.Set("policyVersion", wire.String("2"))
	rt, _ := policy.Obj.Get("retries")
	rt.Obj.Set("admissionsPerRevision", wire.String("4"))
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(dir, "policy.json")
	fixture.Write(t, file, wire.EncodeFile(policy))
	ok(handoffCLI(t, root, "policy", "update", "--request-id", "raised-policy", "--expected-policy-version", "1", "--file", file))
	before = fixture.TreeSnapshot(t, root)
	old = handoffCLIWithBinary(t, root, legacy, "plan", "preview")
	if old.code == 0 {
		t.Fatalf("old reader accepted unsupported retry bound: %s", old.stdout)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, root)) {
		t.Fatal("old planner changed store")
	}
	if output := os.Getenv("CORVINT_TASKS_QUALIFICATION_OUTPUT"); output != "" {
		if e := os.WriteFile(filepath.Join(output, "legacy-release-replay.json"), replay.stdout, 0600); e != nil {
			t.Fatal(e)
		}
	}
}

func TestCALV0046_CLIPoolHandoffQuarantines(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	planTicket(t, root, "pool", "P1", `["src/"]`)
	repo, e := intent.Resolve(root)
	if e != nil {
		t.Fatal(e)
	}
	ok := func(args ...string) run {
		t.Helper()
		r := handoffCLI(t, root, args...)
		if r.code != 0 {
			t.Fatalf("%v: %s", args, r.stdout)
		}
		return r
	}
	loaded, e := intent.Load(repo.PrimaryWorktree)
	if e != nil {
		t.Fatal(e)
	}
	v, e := wire.Parse(loaded.Policy.Raw)
	if e != nil {
		t.Fatal(e)
	}
	v.Obj.Set("policyVersion", wire.String("2"))
	rt, _ := v.Obj.Get("retries")
	rt.Obj.Set("admissionsPerRevision", wire.String("0"))
	v.Obj.Set("pools", wire.Array(wire.ObjectValue(wire.NewObject().Set("id", wire.String("lane")).Set("members", wire.Strings([]string{"integrator"})).Set("reservedFor", wire.ObjectValue(wire.NewObject().Set("integrator", wire.String("integrate")))))))
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(dir, "policy.json")
	fixture.Write(t, file, wire.EncodeFile(v))
	ok("policy", "update", "--request-id", "pool-policy", "--expected-policy-version", "1", "--file", file)
	preview := func(want string) {
		t.Helper()
		r := ok("plan", "preview", "--pool", "lane", "--stage", "integrate")
		entries := field(r.res.Items[0], "entries").Arr
		if len(entries) != 1 || field(entries[0], "state").Str != want {
			t.Fatalf("pool plan: %s", r.stdout)
		}
	}
	preview("SELECTED")
	a := ok("claim", "--next", "--pool", "lane", "--stage", "integrate", "--holder", "integrator", "--request-id", "claim")
	idA, g := field(a.res.Items[0], "attemptId").Str, field(a.res.Items[0], "generation").Str
	allocation := field(field(a.res.Items[0], "poolAllocation"), "allocationId").Str
	ok("release", "--attempt", idA, "--generation", g, "--request-id", "handoff", "--reason", "HANDOFF", "--evidence", "local:external-baseline")
	raw, e := os.ReadFile(filepath.Join(repo.StateDir, "pools.json"))
	if e != nil {
		t.Fatal(e)
	}
	pools, e := wire.Parse(raw)
	if e != nil {
		t.Fatal(e)
	}
	entries := field(pools, "entries").Arr
	if len(entries) != 1 || field(entries[0], "state").Str != "QUARANTINED" {
		t.Fatalf("quarantine: %s", raw)
	}
	preview("BLOCKED")
	// This fixture declared no external runner/resource. Operator confirmation
	// records only that fixture fact, not a general proof of physical cleanup.
	ok("pool", "confirm-safe", "--member", "integrator", "--allocation", allocation, "--evidence", "local:fixture-no-external-runner", "--reason", "fixture has no external runner or resource", "--request-id", "safe")
	preview("SELECTED")
	next := ok("claim", "--next", "--pool", "lane", "--stage", "integrate", "--holder", "successor", "--request-id", "next")
	idA, g = field(next.res.Items[0], "attemptId").Str, field(next.res.Items[0], "generation").Str
	ok("release", "--attempt", idA, "--generation", g, "--request-id", "cancel")
	preview("BLOCKED")
	ok("receipt", "audit")
	if output := os.Getenv("CORVINT_TASKS_QUALIFICATION_OUTPUT"); output != "" {
		if e := os.WriteFile(filepath.Join(output, "no-tree-pool-quarantine.json"), raw, 0600); e != nil {
			t.Fatal(e)
		}
	}
}
