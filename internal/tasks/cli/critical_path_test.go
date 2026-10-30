package cli_test

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func cpIDs(v wire.Value) []string {
	var out []string
	for _, x := range v.Arr {
		out = append(out, x.Str)
	}
	return out
}

func cpNode(t *testing.T, item wire.Value, local string) wire.Value {
	t.Helper()
	for _, n := range field(item, "nodes").Arr {
		if field(n, "ticketId").Str == fixture.TicketID(local) {
			return n
		}
	}
	t.Fatalf("node %s missing: %s", local, wire.Encode(item))
	return wire.Value{}
}

func cpWrite(t *testing.T, r *fixture.Repo, journal bool, recs ...*ticket.Record) {
	t.Helper()
	// No enforced budget fields, so queue-wide BUDGET_UNKNOWN does not mask
	// each node's own first blocker.
	policy := fixture.PolicyValue()
	budgets, _ := policy.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	cpWritePolicy(t, r, journal, policy, recs...)
}

func cpWritePolicy(t *testing.T, r *fixture.Repo, journal bool, policy wire.Value, recs ...*ticket.Record) {
	t.Helper()
	if journal {
		fixture.WriteState(t, r)
	}
	fixture.WriteIntent(t, r, recs...)
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(policy))
	if journal {
		posts := map[string][]byte{"intent/policy.json": wire.EncodeFile(policy)}
		for _, rec := range recs {
			posts["intent/tickets/"+rec.TicketID.Local+".json"] = rec.Encode()
		}
		fixture.CommitPosts(t, r, "MUTATION", "", posts)
	}
}

// CAL-V0-079 and CAL-V0-081: the unsatisfied closure as chains, longest
// first, with per-node facts; satisfied edges are not followed; the read is
// pure and journal-absent liveness is NOT_OBSERVED.
func TestCALV0079_CriticalPathChainsAndNodeFacts(t *testing.T) {
	for _, journal := range []bool{true, false} {
		t.Run(fmt.Sprintf("journal=%v", journal), func(t *testing.T) {
			r := fixture.TempRepo(t)
			gate, a, b, c, d, e := fixture.Ticket("GATE"), fixture.Ticket("A"), fixture.Ticket("B"), fixture.Ticket("C"), fixture.Ticket("D"), fixture.Ticket("E")
			gate.Dependencies = []ticket.Dependency{fixture.Dep("A"), fixture.Dep("B"), fixture.Dep("C")}
			a.Dependencies = []ticket.Dependency{fixture.Dep("D")}
			d.Dependencies = []ticket.Dependency{fixture.Dep("E")}
			d.Status = ticket.StatusHeld
			d.Holds = []ticket.Hold{{HoldID: "review", Actor: "op", Reason: "x", PlacedAt: fixture.Timestamp}}
			c.Status = ticket.StatusCompleted
			reason := "done"
			c.Completion = &ticket.Completion{Kind: "MANUAL", Actor: fixture.Actor, Reason: &reason, Evidence: []wire.Digest{}, RecordedAt: fixture.Timestamp}
			cpWrite(t, r, journal, gate, a, b, c, d, e)
			state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
			x := atm(t, r.Root, nil, "critical-path", "GATE")
			if x.res.Outcome != wire.OutcomeOK || x.res.Mutation != nil {
				t.Fatalf("critical-path: %s", x.stdout)
			}
			if !fixture.SameTree(state, fixture.TreeSnapshot(t, r.StateDir)) || !fixture.SameTree(intents, fixture.TreeSnapshot(t, r.IntentDir)) {
				t.Fatal("critical-path wrote")
			}
			it := x.res.Items[0]
			if field(it, "profile").Str != "taskman-critical-path/0" || field(it, "estimate").Str != "NOT_OBSERVED" || field(it, "truncated").Bool || field(it, "nodesTotal").Str != "5" || field(it, "chainsTotal").Str != "2" || len(field(it, "cycles").Arr) != 0 {
				t.Fatalf("header: %s", wire.Encode(it))
			}
			chains := field(it, "chains").Arr
			want0 := []string{fixture.TicketID("GATE"), fixture.TicketID("A"), fixture.TicketID("D"), fixture.TicketID("E")}
			if strings.Join(cpIDs(field(chains[0], "ticketIds")), ",") != strings.Join(want0, ",") || field(chains[0], "length").Str != "4" || strings.Join(cpIDs(field(chains[1], "ticketIds")), ",") != fixture.TicketID("GATE")+","+fixture.TicketID("B") {
				t.Fatalf("chains: %s", wire.Encode(field(it, "chains")))
			}
			for _, n := range field(it, "nodes").Arr {
				if field(n, "ticketId").Str == fixture.TicketID("C") {
					t.Fatal("satisfied dependency followed")
				}
			}
			dn := cpNode(t, it, "D")
			if field(dn, "status").Str != "HELD" || field(field(dn, "firstBlocker"), "code").Str != wire.CodeTicketHeld || field(field(dn, "firstBlocker"), "observation").Str != "CERTAIN" || field(dn, "depth").Str != "3" {
				t.Fatalf("held node: %s", wire.Encode(dn))
			}
			gn := cpNode(t, it, "GATE")
			if field(field(gn, "firstBlocker"), "code").Str != wire.CodeDependencyUnsatisfied || len(field(gn, "waitingOn").Arr) != 2 || field(field(gn, "waitingOn").Arr[0], "kind").Str != "DEPENDENCY" {
				t.Fatalf("gate node: %s", wire.Encode(gn))
			}
			att := field(cpNode(t, it, "E"), "attempt")
			wantObs := "NONE"
			if !journal {
				wantObs = "NOT_OBSERVED"
			}
			if field(att, "observation").Str != wantObs || field(att, "attemptId").Str != "NOT_OBSERVED" || field(att, "holder").Str != "NOT_OBSERVED" || field(att, "lastProgressAt").Str != "NOT_OBSERVED" {
				t.Fatalf("attempt facts: %s", wire.Encode(att))
			}
			human := cpIDs(field(it, "human"))
			if len(human) != 3 || !strings.Contains(human[1], "GATE(OPEN DEPENDENCY_UNSATISFIED) -> A(") || !strings.Contains(human[1], "D(HELD TICKET_HELD)") {
				t.Fatalf("human: %q", human)
			}
			// A satisfied root still answers, with an empty closure beyond it.
			if x := atm(t, r.Root, nil, "critical-path", "C"); x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "nodesTotal").Str != "1" {
				t.Fatalf("completed root: %s", x.stdout)
			}
		})
	}
}

// CAL-V0-080: node and chain bounds report truncated with totals, and a
// dependency cycle is reported with CYCLE instead of being looped.
func TestCALV0080_CriticalPathBoundsAndCycles(t *testing.T) {
	r := fixture.TempRepo(t)
	var recs []*ticket.Record
	// A 300-ticket line exceeds the node bound.
	for i := 0; i < 300; i++ {
		rec := fixture.Ticket(fmt.Sprintf("L%03d", i))
		if i < 299 {
			rec.Dependencies = []ticket.Dependency{fixture.Dep(fmt.Sprintf("L%03d", i+1))}
		}
		recs = append(recs, rec)
	}
	// A fan of 40 frontier tickets exceeds the chain bound.
	fan := fixture.Ticket("FAN")
	for i := 0; i < 40; i++ {
		leaf := fixture.Ticket(fmt.Sprintf("F%02d", i))
		recs = append(recs, leaf)
		fan.Dependencies = append(fan.Dependencies, fixture.Dep(leaf.TicketID.Local))
	}
	// Q -> X <-> Y is a cycle; Y also waits on Z.
	q, x, y, z := fixture.Ticket("Q"), fixture.Ticket("X"), fixture.Ticket("Y"), fixture.Ticket("Z")
	q.Dependencies = []ticket.Dependency{fixture.Dep("X")}
	x.Dependencies = []ticket.Dependency{fixture.Dep("Y")}
	y.Dependencies = []ticket.Dependency{fixture.Dep("X"), fixture.Dep("Z")}
	recs = append(recs, fan, q, x, y, z)
	cpWrite(t, r, false, recs...)

	line := atm(t, r.Root, nil, "critical-path", "L000")
	it := line.res.Items[0]
	chain := field(it, "chains").Arr[0]
	if line.res.Outcome != wire.OutcomeOK || !field(it, "truncated").Bool || field(it, "nodesTotal").Str != "300" || field(it, "nodesReturned").Str != "256" || len(field(it, "nodes").Arr) != 256 || field(chain, "length").Str != "300" || len(field(chain, "ticketIds").Arr) != 256 || !field(chain, "truncated").Bool {
		t.Fatalf("node bound: %s", line.stdout[:400])
	}
	f := atm(t, r.Root, nil, "critical-path", "FAN").res.Items[0]
	if !field(f, "truncated").Bool || field(f, "chainsTotal").Str != "40" || field(f, "chainsReturned").Str != "32" || len(field(f, "chains").Arr) != 32 || field(f, "nodesTotal").Str != "41" {
		t.Fatalf("chain bound: %s", wire.Encode(f))
	}
	c := atm(t, r.Root, nil, "critical-path", "Q")
	ci := c.res.Items[0]
	cycles := field(ci, "cycles").Arr
	if c.res.Outcome != wire.OutcomeOK || len(cycles) != 1 || field(cycles[0], "code").Str != wire.CodeCycle || strings.Join(cpIDs(field(cycles[0], "ticketIds")), ",") != fixture.TicketID("X")+","+fixture.TicketID("Y") || field(ci, "nodesTotal").Str != "4" {
		t.Fatalf("cycle: %s", c.stdout)
	}
	if field(field(cpNode(t, ci, "X"), "firstBlocker"), "code").Str != wire.CodeCycle {
		t.Fatalf("cycle blocker: %s", c.stdout)
	}
	if got := strings.Join(cpIDs(field(field(ci, "chains").Arr[0], "ticketIds")), ","); got != strings.Join([]string{fixture.TicketID("Q"), fixture.TicketID("X"), fixture.TicketID("Y"), fixture.TicketID("Z")}, ",") {
		t.Fatalf("cycle chain: %s", got)
	}
	for _, args := range [][]string{{"critical-path"}, {"critical-path", "A", "B"}, {"critical-path", "--limit", "1"}} {
		if x := atm(t, r.Root, nil, args...); x.res.Outcome != wire.OutcomeError || !hasCode(x.res, wire.CodeMalformed) {
			t.Fatalf("%v: %s", args, x.stdout)
		}
	}
	if x := atm(t, r.Root, nil, "critical-path", "NOPE"); x.res.Outcome != wire.OutcomeRefused {
		t.Fatalf("unknown: %s", x.stdout)
	}
}

// CAL-V0-079: a live claimed dependency reports its attempt, holder, stage,
// pool member and last recorded progress seq from the journal.
func TestCALV0079_CriticalPathLiveAttemptFacts(t *testing.T) {
	r := exclusionCLIRepo(t)
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatal(x.res)
	}
	dep := planTicket(t, r.Root, "dep", "P2", `["dep"]`)
	payload := strings.NewReplacer(`"dependencies":[]`, `"dependencies":[{"gateId":null,"obligation":"COMPLETED","ticketId":"`+dep+`"}]`, `"title":"Console ticket"`, `"title":"root"`).Replace(createPayloadJSON)
	root := atm(t, r.Root, nil, "ticket", "create", "--request-id", "create-root", "--issued-at", "2026-09-27T12:00:00Z", "--payload", payload)
	if root.res.Outcome != wire.OutcomeOK {
		t.Fatalf("create root: %s", root.stdout)
	}
	claim := atm(t, r.Root, nil, "claim", dep, "--holder", "builder", "--request-id", "claim-dep", "--pool", "db", "--stage", "review")
	if claim.res.Outcome != wire.OutcomeOK {
		t.Fatalf("claim: %s", claim.stdout)
	}
	state := fixture.TreeSnapshot(t, r.StateDir)
	x := atm(t, r.Root, nil, "critical-path", field(root.res.Items[0], "ticketId").Str)
	if x.res.Outcome != wire.OutcomeOK || !fixture.SameTree(state, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Fatalf("critical-path: %s", x.stdout)
	}
	var att wire.Value
	for _, n := range field(x.res.Items[0], "nodes").Arr {
		if field(n, "ticketId").Str == dep {
			att = field(n, "attempt")
			if field(field(n, "firstBlocker"), "code").Str != wire.CodeAttemptLive {
				t.Fatalf("dep blocker: %s", wire.Encode(n))
			}
		}
	}
	ci := claim.res.Items[0]
	if field(att, "observation").Str != "LIVE" || field(att, "attemptId").Str != field(ci, "attemptId").Str || field(att, "holder").Str != "builder" || field(att, "stage").Str != "review" || field(att, "member").Str != field(field(ci, "poolAllocation"), "memberId").Str || field(att, "lastProgressSeq").Str == "NOT_OBSERVED" || field(att, "lastProgressAt").Str == "NOT_OBSERVED" || field(att, "phase").Str != "RUNNING" {
		t.Fatalf("attempt facts: %s", wire.Encode(att))
	}
	if !strings.Contains(strings.Join(cpIDs(field(x.res.Items[0], "human")), "\n"), "holder builder") {
		t.Fatalf("human: %s", x.stdout)
	}
}

// cpBlockerRefs is the plan-preview blocker reference set of a node: its
// blocker codes plus the tickets they name.
func cpBlockerRefs(n wire.Value) []string {
	set := map[string]bool{}
	for _, b := range field(n, "blockers").Arr {
		set[field(b, "code").Str] = true
		if id := field(b, "ticketId"); id.Kind == wire.KindString {
			set[id.Str] = true
		}
	}
	out := []string{}
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// cpPlanParity checks every OPEN or HELD node against `plan preview`: a
// BLOCKED entry has the node's first blocker as its reason and the node's
// blockers as its references, and any other entry has no node blocker.
func cpPlanParity(t *testing.T, r *fixture.Repo, item wire.Value) {
	t.Helper()
	p := atm(t, r.Root, nil, "plan", "preview")
	if p.res.Outcome != wire.OutcomeOK {
		t.Fatalf("plan preview: %s", p.stdout)
	}
	entries := map[string]wire.Value{}
	for _, e := range field(p.res.Items[0], "entries").Arr {
		entries[field(e, "ticketId").Str] = e
	}
	for _, n := range field(item, "nodes").Arr {
		id := field(n, "ticketId").Str
		if s := field(n, "status").Str; s != ticket.StatusOpen && s != ticket.StatusHeld {
			continue
		}
		e, ok := entries[id]
		if !ok {
			t.Fatalf("%s missing from plan", id)
		}
		refs := cpBlockerRefs(n)
		if field(e, "state").Str != "BLOCKED" {
			if len(refs) != 0 {
				t.Errorf("%s: plan %s but critical-path blockers %v", id, field(e, "state").Str, refs)
			}
			continue
		}
		if field(e, "reason").Str != field(field(n, "firstBlocker"), "code").Str || strings.Join(cpIDs(field(e, "blockers")), ",") != strings.Join(refs, ",") {
			t.Errorf("%s: plan %s %v, critical-path %s", id, wire.Encode(e), cpIDs(field(e, "blockers")), wire.Encode(n))
		}
	}
}

func cpHasBlocker(n wire.Value, code, observation, ticketID string) bool {
	for _, b := range field(n, "blockers").Arr {
		tid := ""
		if v := field(b, "ticketId"); v.Kind == wire.KindString {
			tid = v.Str
		}
		if field(b, "code").Str == code && field(b, "observation").Str == observation && tid == ticketID {
			return true
		}
	}
	return false
}

// CAL-V0-081: node blockers are the planner's claim blockers, so they agree
// with `plan preview` for a required pool, an excluded COVERAGE_UNKNOWN and
// enforced budgets; a GATE_PASSED edge this reader cannot observe is followed
// as NOT_OBSERVED; a dependency archived from COMPLETED is satisfied and one
// archived otherwise is not.
func TestCALV0081_CriticalPathBlockersMatchPlanner(t *testing.T) {
	r := fixture.TempRepo(t)
	root := fixture.Ticket("ROOT")
	pool, cov, gate, done, dropped := fixture.Ticket("POOLED"), fixture.Ticket("COV"), fixture.Ticket("GATED"), fixture.Ticket("DONE"), fixture.Ticket("DROPPED")
	root.Dependencies = []ticket.Dependency{fixture.Dep("POOLED"), fixture.Dep("COV"), fixture.GateDep("GATED", "verify"), fixture.Dep("DONE"), fixture.Dep("DROPPED")}
	pool.RequiresPool = "db"
	cov.Effects.Coverage = "UNKNOWN"
	completed, open := ticket.StatusCompleted, ticket.StatusOpen
	reason := "done"
	done.Status, done.ArchivedFrom = ticket.StatusArchived, &completed
	done.Completion = &ticket.Completion{Kind: "MANUAL", Actor: fixture.Actor, Reason: &reason, Evidence: []wire.Digest{}, RecordedAt: fixture.Timestamp}
	dropped.Status, dropped.ArchivedFrom = ticket.StatusArchived, &open
	recs := []*ticket.Record{root, pool, cov, gate, done, dropped}
	cpWrite(t, r, true, recs...)
	x := atm(t, r.Root, nil, "critical-path", "ROOT")
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("critical-path: %s", x.stdout)
	}
	it := x.res.Items[0]
	if field(it, "blockerScope").Str != "RECORDED_DEFAULT_EXTERNAL_AGENT_PLAN" || field(it, "nodesTotal").Str != "5" {
		t.Fatalf("closure: %s", wire.Encode(it))
	}
	for _, n := range field(it, "nodes").Arr {
		if field(n, "ticketId").Str == fixture.TicketID("DONE") {
			t.Fatal("dependency archived from COMPLETED was followed")
		}
		if cpHasBlocker(n, wire.CodeCoverageUnknown, "CERTAIN", "") || cpHasBlocker(n, wire.CodeCoverageUnknown, "NOT_OBSERVED", "") {
			t.Fatalf("planner-excluded COVERAGE_UNKNOWN reported: %s", wire.Encode(n))
		}
	}
	if s := atm(t, r.Root, nil, "ticket", "show", "COV"); !strings.Contains(string(s.stdout), wire.CodeCoverageUnknown) {
		t.Fatalf("fixture does not exercise COVERAGE_UNKNOWN: %s", s.stdout)
	}
	if n := cpNode(t, it, "POOLED"); field(field(n, "firstBlocker"), "code").Str != wire.CodeResourceCollision {
		t.Fatalf("pool: %s", wire.Encode(n))
	}
	if n := cpNode(t, it, "COV"); len(field(n, "blockers").Arr) != 0 || field(n, "firstBlocker").Kind != wire.KindNull || field(n, "eligibility").Str != ticket.EligibilityUnknown {
		t.Fatalf("coverage: %s", wire.Encode(n))
	}
	if n := cpNode(t, it, "DROPPED"); field(field(n, "firstBlocker"), "code").Str != wire.CodeTicketState {
		t.Fatalf("archived from OPEN: %s", wire.Encode(n))
	}
	rn := cpNode(t, it, "ROOT")
	var gateEdge wire.Value
	for _, e := range field(rn, "waitingOn").Arr {
		if field(e, "ticketId").Str == fixture.TicketID("GATED") {
			gateEdge = e
		}
	}
	if field(gateEdge, "obligation").Str != "GATE_PASSED" || field(gateEdge, "observation").Str != "NOT_OBSERVED" || !cpHasBlocker(rn, wire.CodeDependencyUnsatisfied, "NOT_OBSERVED", fixture.TicketID("GATED")) {
		t.Fatalf("gate observation: %s", wire.Encode(rn))
	}
	cpPlanParity(t, r, it)

	// Enforced budget fields refuse every claim with BUDGET_UNKNOWN.
	b := fixture.TempRepo(t)
	cpWritePolicy(t, b, true, fixture.PolicyValue(), recs...)
	budget := atm(t, b.Root, nil, "critical-path", "ROOT").res.Items[0]
	if field(field(cpNode(t, budget, "COV"), "firstBlocker"), "code").Str != wire.CodeBudgetUnknown {
		t.Fatalf("budget: %s", wire.Encode(budget))
	}
	cpPlanParity(t, b, budget)
}

// CAL-V0-081: retry exhaustion and an admission barrier appear as they do in
// `plan preview`.
func TestCALV0081_CriticalPathRetryAndPauseMatchPlanner(t *testing.T) {
	r := exclusionCLIRepo(t)
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatal(x.res)
	}
	dep := planTicket(t, r.Root, "dep", "P2", `["dep"]`)
	payload := strings.NewReplacer(`"dependencies":[]`, `"dependencies":[{"gateId":null,"obligation":"COMPLETED","ticketId":"`+dep+`"}]`, `"title":"Console ticket"`, `"title":"root"`).Replace(createPayloadJSON)
	created := atm(t, r.Root, nil, "ticket", "create", "--request-id", "create-root", "--issued-at", "2026-09-27T12:00:00Z", "--payload", payload)
	if created.res.Outcome != wire.OutcomeOK {
		t.Fatalf("create root: %s", created.stdout)
	}
	root := field(created.res.Items[0], "ticketId").Str
	// The initial admission is free; three charged retries exhaust the limit.
	for i := 1; i <= 4; i++ {
		a := atm(t, r.Root, nil, "claim", dep, "--holder", "agent", "--request-id", fmt.Sprintf("retry-%d", i))
		if a.res.Outcome != wire.OutcomeOK {
			t.Fatalf("claim %d: %s", i, a.stdout)
		}
		it := a.res.Items[0]
		if x := atm(t, r.Root, nil, "release", "--attempt", field(it, "attemptId").Str, "--generation", field(it, "generation").Str, "--request-id", fmt.Sprintf("cancel-%d", i)); x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("release %d: %s", i, x.stdout)
		}
	}
	it := atm(t, r.Root, nil, "critical-path", root).res.Items[0]
	var dn wire.Value
	for _, n := range field(it, "nodes").Arr {
		if field(n, "ticketId").Str == dep {
			dn = n
		}
	}
	if dn.Kind != wire.KindObject || field(dn, "firstBlocker").Kind != wire.KindObject || field(field(dn, "firstBlocker"), "code").Str != wire.CodeRetryExhausted || field(dn, "eligibility").Str != ticket.EligibilityBlocked {
		t.Fatalf("retry: %s\nplan: %s", wire.Encode(it), atm(t, r.Root, nil, "plan", "preview").stdout)
	}
	cpPlanParity(t, r, it)
	if p := atm(t, r.Root, nil, "pause", "--request-id", "pause"); p.res.Outcome != wire.OutcomeOK {
		t.Fatalf("pause: %s", p.stdout)
	}
	paused := atm(t, r.Root, nil, "critical-path", root).res.Items[0]
	for _, n := range field(paused, "nodes").Arr {
		if field(field(n, "firstBlocker"), "code").Str != wire.CodePaused {
			t.Fatalf("paused: %s", wire.Encode(n))
		}
	}
	cpPlanParity(t, r, paused)
}
