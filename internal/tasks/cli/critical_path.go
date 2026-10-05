package cli

import (
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Critical-path read (CAL-V0-079..081, issue 588, owner answer 2026-10-05 D8).
const (
	profileCriticalPath   = "taskman-critical-path/0"
	maxCriticalPathNodes  = 256
	maxCriticalPathChains = 32
	notObserved           = "NOT_OBSERVED"
)

// cpEdge is one unsatisfied (or unobservable) dependency obligation that the
// closure follows. Kind is DEPENDENCY in v0; later edge kinds extend it.
type cpEdge struct {
	to, kind, obligation, observation string
}

// cpNode is a compact closure record. Blockers are derived only for the
// bounded output, never for every closure node (CAL-V0-080).
type cpNode struct {
	id    string
	rec   *ticket.Record
	edges []cpEdge
	via   []string // cycle members first reached from this node
	dist  int      // nodes on the longest root-to-node path
	pred  string   // predecessor on that path
}

// criticalPath is the computed closure; render turns it into the profile.
type criticalPath struct {
	root     string
	nodes    map[string]*cpNode
	topo     []string // root first, every node before its dependencies
	frontier []string // nodes the closure follows no edge from
	cycles   [][]string
	blockers func(*ticket.Record) []transaction.ObservedBlocker
	memo     map[string][]transaction.ObservedBlocker
}

// criticalPathCommand is `corvint-tasks critical-path <ticket>`: a pure read
// over the same TM-V0-008 snapshot as `ticket show`. It takes no lock and
// writes nothing (product invariant 4).
func criticalPathCommand(env Env, args []string) *wire.Result {
	cmd := []string{"critical-path"}
	if len(args) != 1 || strings.HasPrefix(args[0], "--") {
		return failure(cmd, nil, wire.Errorf(wire.CodeMalformed, "argv", "critical-path takes exactly one ticket id or local token"))
	}
	var item *wire.Value
	notFound := ""
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		item, notFound = nil, ""
		id, err := resolveTicketArg(rc, args[0])
		if err != nil {
			return err
		}
		if _, ok := rc.store.Inventory.Get(id); !ok {
			notFound = id
			return nil
		}
		// Edge observation needs only the gate oracle; blockers come from
		// the planner input below.
		ctx := rc.store.Context()
		// The planner input: with no journal there is no barrier, attempt
		// or pool record, and a nil reservation set keeps attempt liveness
		// NOT_OBSERVED.
		in := transaction.PlanInput{Queue: rc.store.Queue, Policy: rc.store.Policy, Tickets: rc.store.Inventory}
		var attempts map[string]*snapshot.Attempt // by ticket; nil = NOT_OBSERVED
		if !rc.journalAbsent {
			var e error
			if in, _, e = planInput(rc); e != nil {
				return e
			}
			attempts = liveAttemptsByTicket(in)
		}
		cp := computeCriticalPath(rc.store.Inventory, ctx, id)
		cp.blockers = func(rec *ticket.Record) []transaction.ObservedBlocker {
			return transaction.ClaimBlockerObservations(in, rec)
		}
		v := cp.value(rc.store.Queue.QueueID.Raw, attempts)
		item = &v
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	res := success(cmd, rc)
	if notFound != "" {
		res.Outcome = wire.OutcomeRefused
		res.Warnings = append(res.Warnings, "ticket "+notFound+" does not exist in this queue")
		return res
	}
	res.Items = []wire.Value{*item}
	return res
}

// liveAttemptsByTicket maps each reserved ticket to its live attempt record.
func liveAttemptsByTicket(in transaction.PlanInput) map[string]*snapshot.Attempt {
	out := map[string]*snapshot.Attempt{}
	if in.Reservations == nil {
		return out
	}
	for _, en := range in.Reservations.Entries {
		if a, ok := in.Attempts[en.AttemptID]; ok && a.Live() {
			out[en.TicketID.Raw] = a
		}
	}
	return out
}

// edgeObservation reports whether one dependency obligation is satisfied.
// A GATE_PASSED obligation this reader cannot observe stays NOT_OBSERVED and
// is followed: an unknown is never treated as satisfied.
func edgeObservation(ctx ticket.Context, d ticket.Dependency, dep *ticket.Record) ticket.Observation {
	switch d.Obligation {
	case "COMPLETED":
		if dep.Status == ticket.StatusCompleted || (dep.Status == ticket.StatusArchived && dep.ArchivedFrom != nil && *dep.ArchivedFrom == ticket.StatusCompleted) {
			return ticket.Satisfied
		}
		return ticket.Unsatisfied
	case "GATE_PASSED":
		if ctx.Gates == nil {
			return ticket.NotObserved
		}
		gate := ""
		if d.GateID != nil {
			gate = *d.GateID
		}
		return ctx.Gates.GatePassed(dep.TicketID, gate, dep.AcceptanceRevision)
	}
	return ticket.NotObserved
}

// computeCriticalPath builds the transitive unsatisfied dependency closure of
// root. Edges between members of one dependency cycle are not followed, so
// the walk is acyclic and terminates; the cycle is reported instead.
func computeCriticalPath(inv *ticket.Inventory, ctx ticket.Context, root string) *criticalPath {
	cp := &criticalPath{root: root, nodes: map[string]*cpNode{}}
	rank := map[string]int{}
	for i, id := range inv.Sorted() {
		rank[id] = i
	}
	seenCycle := map[string]bool{}
	var post []string
	var visit func(id string)
	visit = func(id string) {
		rec, _ := inv.Get(id)
		n := &cpNode{id: id, rec: rec}
		cp.nodes[id] = n
		key, inCycle := inv.CycleKey(id)
		for _, d := range rec.Dependencies {
			dep, ok := inv.Get(d.TicketID.Raw)
			if !ok {
				continue // DEPENDENCY_MISSING stays a node blocker
			}
			if k, ok := inv.CycleKey(d.TicketID.Raw); inCycle && ok && k == key {
				continue // CYCLE stays a node blocker
			}
			obs := edgeObservation(ctx, d, dep)
			if obs == ticket.Satisfied {
				continue
			}
			n.edges = append(n.edges, cpEdge{to: d.TicketID.Raw, kind: "DEPENDENCY", obligation: d.Obligation, observation: string(obs)})
		}
		sort.SliceStable(n.edges, func(i, j int) bool { return rank[n.edges[i].to] < rank[n.edges[j].to] })
		for _, e := range n.edges {
			if _, seen := cp.nodes[e.to]; !seen {
				visit(e.to)
			}
		}
		// The first member reached of a cycle brings in the rest, once, so a
		// large cycle costs linear work and one copy of its member list.
		if inCycle && !seenCycle[key] {
			seenCycle[key] = true
			members := inv.CycleMembers(id)
			cp.cycles = append(cp.cycles, members)
			for _, m := range members {
				if _, seen := cp.nodes[m]; !seen {
					n.via = append(n.via, m)
					visit(m)
				}
			}
		}
		post = append(post, id)
	}
	visit(root)
	for i := len(post) - 1; i >= 0; i-- {
		cp.topo = append(cp.topo, post[i])
	}
	// Longest path from root over the acyclic edge set, ties broken by
	// planning order of the predecessor. A cycle member first reached through
	// its cycle (via) is one step beyond the member that reached it, so every
	// node lies on a root-anchored chain.
	cp.nodes[root].dist = 1
	for _, id := range cp.topo {
		n := cp.nodes[id]
		next := append([]string(nil), n.via...)
		for _, e := range n.edges {
			next = append(next, e.to)
		}
		for _, to := range next {
			m := cp.nodes[to]
			if n.dist+1 > m.dist || (n.dist+1 == m.dist && rank[id] < rank[m.pred]) {
				m.dist, m.pred = n.dist+1, id
			}
		}
	}
	for _, id := range cp.topo {
		if len(cp.nodes[id].edges) == 0 {
			cp.frontier = append(cp.frontier, id)
		}
	}
	sort.SliceStable(cp.frontier, func(i, j int) bool {
		a, b := cp.nodes[cp.frontier[i]], cp.nodes[cp.frontier[j]]
		if a.dist != b.dist {
			return a.dist > b.dist
		}
		return rank[a.id] < rank[b.id]
	})
	return cp
}

// chain is the longest root-to-frontier path ending at leaf, root first.
func (cp *criticalPath) chain(leaf string) []string {
	var rev []string
	for id := leaf; id != ""; id = cp.nodes[id].pred {
		rev = append(rev, id)
	}
	out := make([]string, len(rev))
	for i, id := range rev {
		out[len(rev)-1-i] = id
	}
	return out
}

func count(n int) wire.Value { return wire.String(string(wire.CountOf(int64(n)))) }

func (cp *criticalPath) value(queueID string, attempts map[string]*snapshot.Attempt) wire.Value {
	chainsTotal := len(cp.frontier)
	leaves := cp.frontier
	if len(leaves) > maxCriticalPathChains {
		leaves = leaves[:maxCriticalPathChains]
	}
	truncated := chainsTotal > maxCriticalPathChains || len(cp.nodes) > maxCriticalPathNodes
	// Node detail order: nodes on returned chains first, in chain order, then
	// the rest of the closure in topological order, up to the node bound.
	var order []string
	placed := map[string]bool{}
	place := func(id string) {
		if !placed[id] && len(order) < maxCriticalPathNodes {
			placed[id] = true
			order = append(order, id)
		}
	}
	chainIDs := make([][]string, len(leaves))
	for i, leaf := range leaves {
		chainIDs[i] = cp.chain(leaf)
		for _, id := range chainIDs[i] {
			place(id)
		}
	}
	for _, id := range cp.topo {
		place(id)
	}
	chains := make([]wire.Value, 0, len(leaves))
	header := "critical-path " + cp.root + ": " + string(wire.CountOf(int64(len(cp.nodes)))) + " node(s), " + string(wire.CountOf(int64(chainsTotal))) + " chain(s)"
	if truncated {
		header += " (truncated)"
	}
	human := []string{header}
	for i, ids := range chainIDs {
		chainTruncated := false
		if len(ids) > maxCriticalPathNodes {
			ids, chainTruncated = ids[:maxCriticalPathNodes], true
			truncated = true
		}
		o := wire.NewObject()
		o.Set("length", count(len(cp.chain(leaves[i]))))
		o.Set("ticketIds", wire.Strings(ids))
		o.Set("frontier", wire.String(leaves[i]))
		o.Set("truncated", wire.Bool(chainTruncated))
		chains = append(chains, wire.ObjectValue(o))
		parts := make([]string, len(ids))
		for j, id := range ids {
			parts[j] = cp.humanNode(id, attempts)
		}
		human = append(human, string(wire.CountOf(int64(i+1)))+". "+strings.Join(parts, " -> "))
	}
	cycles := make([]wire.Value, 0, len(cp.cycles))
	for _, c := range cp.cycles {
		cycles = append(cycles, wire.ObjectValue(wire.NewObject().Set("code", wire.String(wire.CodeCycle)).Set("ticketIds", wire.Strings(c))))
		human = append(human, "cycle "+wire.CodeCycle+": "+strings.Join(c, ", "))
	}
	nodes := make([]wire.Value, 0, len(order))
	for _, id := range order {
		nodes = append(nodes, cp.nodeValue(id, attempts))
	}
	o := wire.NewObject()
	o.Set("profile", wire.String(profileCriticalPath))
	o.Set("queueId", wire.String(queueID))
	o.Set("ticketId", wire.String(cp.root))
	o.Set("bounds", wire.ObjectValue(wire.NewObject().Set("maxNodes", count(maxCriticalPathNodes)).Set("maxChains", count(maxCriticalPathChains))))
	o.Set("nodesTotal", count(len(cp.nodes)))
	o.Set("nodesReturned", count(len(nodes)))
	o.Set("chainsTotal", count(chainsTotal))
	o.Set("chainsReturned", count(len(chains)))
	o.Set("truncated", wire.Bool(truncated))
	o.Set("chains", wire.Array(chains...))
	o.Set("nodes", wire.Array(nodes...))
	o.Set("cycles", wire.Array(cycles...))
	o.Set("blockerScope", wire.String("RECORDED_DEFAULT_EXTERNAL_AGENT_PLAN"))
	o.Set("estimate", wire.String(notObserved))
	o.Set("human", wire.Strings(human))
	o.Set("mutationAuthority", wire.Bool(false))
	return wire.ObjectValue(o)
}

// nodeBlockers is the planner's claim-blocker derivation for one node
// (transaction.ClaimBlockerObservations), memoized and derived only for
// rendered nodes. Codes form an open set: later derived blockers (execution
// prerequisites, LOOP_DETECTED, ESCALATION_PENDING holds) arrive as further
// entries of this same shape.
func (cp *criticalPath) nodeBlockers(id string) []transaction.ObservedBlocker {
	if bs, ok := cp.memo[id]; ok {
		return bs
	}
	if cp.memo == nil {
		cp.memo = map[string][]transaction.ObservedBlocker{}
	}
	var bs []transaction.ObservedBlocker
	if cp.blockers != nil {
		bs = cp.blockers(cp.nodes[id].rec)
	}
	cp.memo[id] = bs
	return bs
}

func blockerValues(bs []transaction.ObservedBlocker) []wire.Value {
	out := make([]wire.Value, 0, len(bs))
	for _, b := range bs {
		obs := "CERTAIN"
		if !b.Observed {
			obs = notObserved
		}
		o := wire.NewObject().Set("code", wire.String(b.Code)).Set("observation", wire.String(obs))
		if b.TicketID == "" {
			o.Set("ticketId", wire.Null())
		} else {
			o.Set("ticketId", wire.String(b.TicketID))
		}
		out = append(out, wire.ObjectValue(o))
	}
	return out
}

// firstBlocker is the index of the first certain blocker, else of the first
// NOT_OBSERVED one, else -1: the order RecordedClaimability reports.
func firstBlocker(bs []transaction.ObservedBlocker) int {
	for i, b := range bs {
		if b.Observed {
			return i
		}
	}
	if len(bs) > 0 {
		return 0
	}
	return -1
}

// eligibility follows the ticket view's rule: any certain blocker is
// BLOCKED; otherwise the honest answer is UNKNOWN, never ELIGIBLE.
func eligibility(bs []transaction.ObservedBlocker) string {
	for _, b := range bs {
		if b.Observed {
			return ticket.EligibilityBlocked
		}
	}
	return ticket.EligibilityUnknown
}

func (cp *criticalPath) nodeValue(id string, attempts map[string]*snapshot.Attempt) wire.Value {
	n := cp.nodes[id]
	rec := n.rec
	bs := cp.nodeBlockers(id)
	blockers := blockerValues(bs)
	first := wire.Null()
	if i := firstBlocker(bs); i >= 0 {
		first = blockers[i]
	}
	edges := make([]wire.Value, 0, len(n.edges))
	for _, e := range n.edges {
		edges = append(edges, wire.ObjectValue(wire.NewObject().Set("ticketId", wire.String(e.to)).Set("kind", wire.String(e.kind)).Set("obligation", wire.String(e.obligation)).Set("observation", wire.String(e.observation))))
	}
	holds := make([]string, len(rec.Holds))
	for i, h := range rec.Holds {
		holds[i] = h.HoldID
	}
	o := wire.NewObject()
	o.Set("ticketId", wire.String(id))
	o.Set("status", wire.String(rec.Status))
	o.Set("priority", wire.String(rec.Priority))
	o.Set("eligibility", wire.String(eligibility(bs)))
	o.Set("depth", count(n.dist))
	o.Set("firstBlocker", first)
	o.Set("blockers", wire.Array(blockers...))
	o.Set("holds", wire.Strings(holds))
	o.Set("waitingOn", wire.Array(edges...))
	o.Set("attempt", attemptFacts(id, attempts))
	return wire.ObjectValue(o)
}

// attemptFacts renders the live attempt of a node. Every fact the reader did
// not observe is the string NOT_OBSERVED; observation is LIVE, NONE or
// NOT_OBSERVED (journal absent).
func attemptFacts(id string, attempts map[string]*snapshot.Attempt) wire.Value {
	o := wire.NewObject()
	keys := []string{"attemptId", "phase", "holder", "stage", "member", "lastProgressSeq", "lastProgressAt"}
	vals := map[string]string{}
	for _, k := range keys {
		vals[k] = notObserved
	}
	observation := notObserved
	if attempts != nil {
		observation = "NONE"
		if a, ok := attempts[id]; ok {
			observation = "LIVE"
			vals["attemptId"] = a.AttemptID
			vals["phase"] = a.Phase
			vals["lastProgressSeq"] = string(a.PhaseSinceSeq)
			if a.Lease != nil {
				vals["holder"] = a.Lease.Holder
			}
			if a.Stage != "" {
				vals["stage"] = a.Stage
			}
			if a.PoolAllocation != nil {
				vals["member"] = a.PoolAllocation.MemberID
			}
			if a.LastHeartbeatAt != nil {
				vals["lastProgressAt"] = string(*a.LastHeartbeatAt)
			}
		}
	}
	o.Set("observation", wire.String(observation))
	for _, k := range keys {
		o.Set(k, wire.String(vals[k]))
	}
	return wire.ObjectValue(o)
}

// humanNode is the one-line human form of a node: local token, status, first
// blocker code and, when live, the attempt holder.
func (cp *criticalPath) humanNode(id string, attempts map[string]*snapshot.Attempt) string {
	n := cp.nodes[id]
	s := n.rec.TicketID.Local + "(" + n.rec.Status
	if bs := cp.nodeBlockers(id); firstBlocker(bs) >= 0 {
		b := bs[firstBlocker(bs)]
		s += " " + b.Code
		if !b.Observed {
			s += "?"
		}
	}
	if a, ok := attempts[id]; ok {
		s += " attempt " + a.AttemptID
		if a.Lease != nil {
			s += " holder " + a.Lease.Holder
		}
	}
	return s + ")"
}
