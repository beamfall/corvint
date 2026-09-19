package taskman

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// Native task-store SPEC section 11, source-bound by decision 0321.
const nativeCodes = "ADJUDICATION ADOPT_UNSUPPORTED_FIELD APPROVAL_MISSING APPROVAL_REVOKED ATTEMPT_LIVE BOOT_FENCED BOOT_TIMEOUT BUDGET_EXCEEDED BUDGET_UNKNOWN CAPABILITY_UNAVAILABLE CEM_MISSING CONTAMINATED COVERAGE_UNKNOWN CUTOVER_IN_PROGRESS CUTOVER_MISSING CYCLE DEPENDENCY_MISSING DEPENDENCY_UNSATISFIED DEVELOPMENT_MODE DIRTY_WORKTREE DOCS_MISSING DUPLICATE_ID EFFECT_OWNED EXTERNAL_UNBOUNDED FENCED GATE_FAILED GATE_STALE GATE_UNKNOWN INDEPENDENCE_UNVERIFIED INTENT_BRANCH_MISMATCH INTENT_DIVERGED INVALID_PRIORITY JOURNAL_FORKED JOURNAL_SATURATED LIMIT_EXCEEDED LOCK_TIMEOUT MALFORMED MISSING_EVIDENCE MISSING_GATE NOEXEC OCM_MISSING OUT_OF_SCOPE PAUSED PLAN_STALE QUIESCENCE_UNPROVED REDO_PENDING REQUEST_ID_CONFLICT RESOURCE_COLLISION RESTORED RESTORE_INCOMPLETE RETRY_EXHAUSTED REVIEW_INCOMPLETE REVIEW_REJECTED SIGNAL_REFUSED_IDENTITY SNAPSHOT_MOVED STALE_POLICY STALE_TICKET STALE_TREE SUPERVISOR_LOST SURVIVORS TICKET_HELD TICKET_STATE UNCERTAIN_EFFECT UNINITIALIZED UNPUBLISHED UNRESOLVED_FINDING UNSUPPORTED UNSUPPORTED_FILESYSTEM UNSUPPORTED_VERSION"

const ticketKeys = "profile ticketId revision acceptanceRevision previousRecordSha256 status archivedFrom title body kind owner milestone priority order labels dependencies acceptanceCriteria requirementRefs source effects capabilities requiredGates holds executionClass approvals completion dueDate estimateMinutes supersedes supersededBy shadowOverlay createdAt updatedAt updatedBy"

func oneOf(s, allowed string) bool {
	for _, x := range strings.Fields(allowed) {
		if s == x {
			return true
		}
	}
	return false
}
func boolField(v wire.Value, k string) error {
	if value(v, k).Kind != wire.KindBool {
		return errors.New("boolean required")
	}
	return nil
}
func decodeTicket(v wire.Value) (ticket, error) {
	t := ticket{raw: v, id: stringAt(v, "ticketId"), revision: stringAt(v, "acceptanceRevision"), status: stringAt(v, "status"), priority: stringAt(v, "priority")}
	if e := object(v, ticketKeys); e != nil {
		return t, e
	}
	if stringAt(v, "profile") != "taskman-ticket/0" || !strings.HasPrefix(t.id, "ticket:") {
		return t, errors.New("ticket identity")
	}
	rev, e := number(value(v, "acceptanceRevision"), 2147483647)
	if e != nil || rev == 0 {
		return t, errors.New("acceptance revision")
	}
	chain, e := number(value(v, "revision"), 2147483647)
	if e != nil || chain < rev {
		return t, errors.New("ticket revision")
	}
	t.order, e = number(value(v, "order"), 2147483647)
	if e != nil {
		return t, e
	}
	if !oneOf(t.status, "DRAFT OPEN HELD COMPLETED ARCHIVED") || !oneOf(t.priority, "P0 P1 P2 P3") || !oneOf(stringAt(v, "executionClass"), "AUTONOMOUS APPROVAL_REQUIRED MANUAL EXTERNAL NEVER") {
		return t, errors.New("ticket enum")
	}
	if _, e = array(value(v, "holds"), 1024); e != nil {
		return t, e
	}
	if _, e = stringsAt(v, "capabilities", 1024); e != nil {
		return t, e
	}
	effects := value(v, "effects")
	if e = object(effects, "coverage touchPaths resources externalUnbounded"); e != nil {
		return t, e
	}
	if e = boolField(effects, "externalUnbounded"); e != nil {
		return t, e
	}
	if !oneOf(stringAt(effects, "coverage"), "QUALIFIED INCOMPLETE UNKNOWN") {
		return t, errors.New("coverage")
	}
	t.paths, e = stringsAt(effects, "touchPaths", 4096)
	if e != nil {
		return t, e
	}
	for _, p := range t.paths {
		if !validPath(p) {
			return t, errors.New("touch path")
		}
	}
	t.resources, e = resources(value(effects, "resources"))
	if e != nil {
		return t, e
	}
	deps, e := array(value(v, "dependencies"), 1024)
	if e != nil {
		return t, e
	}
	for _, d := range deps {
		if e = object(d, "ticketId obligation gateId"); e != nil {
			return t, e
		}
		if !strings.HasPrefix(stringAt(d, "ticketId"), "ticket:") || !oneOf(stringAt(d, "obligation"), "COMPLETED GATE_PASSED") {
			return t, errors.New("dependency")
		}
	}
	approvals, e := array(value(v, "approvals"), 1024)
	if e != nil {
		return t, e
	}
	for _, a := range approvals {
		if e = object(a, "grantId actor operation targetRevision scope revoked grantedAt"); e != nil {
			return t, e
		}
		if e = boolField(a, "revoked"); e != nil {
			return t, e
		}
		if _, e = number(value(a, "targetRevision"), 2147483647); e != nil {
			return t, e
		}
		if !oneOf(stringAt(a, "operation"), "RUN COMPLETE INTEGRATE ADJUDICATE") {
			return t, errors.New("approval operation")
		}
	}
	source := value(v, "source")
	if e = object(source, "kind sourceQueueId sourceItemId sourceRevisionSha256"); e != nil {
		return t, e
	}
	if !oneOf(stringAt(source, "kind"), "NATIVE IMPORT") {
		return t, errors.New("source kind")
	}
	return t, nil
}
func decodeObservations(raw []byte, c captured, commit, tree string) (observation, error) {
	o := observation{}
	v, e := document(raw, 16<<20)
	if e != nil {
		return o, e
	}
	o.raw = v
	if e = object(v, "profile sourceCommit sourceTree queueId policySha256 headSeq intentTreeSha256 reservationsComplete attemptsComplete historyComplete reservationSet history"); e != nil {
		return o, e
	}
	if stringAt(v, "profile") != "corvint-taskman-fixture-observations/0" || stringAt(v, "sourceCommit") != commit || stringAt(v, "sourceTree") != tree {
		return o, errors.New("observation source drift")
	}
	for _, k := range []string{"queueId", "policySha256"} {
		if stringAt(v, k) != stringAt(c.queue, k) {
			return o, errors.New("observation queue drift")
		}
	}
	for _, k := range []string{"headSeq", "intentTreeSha256"} {
		if stringAt(v, k) != stringAt(c.snapshot, k) {
			return o, errors.New("observation snapshot drift")
		}
	}
	for _, k := range []string{"reservationsComplete", "attemptsComplete", "historyComplete"} {
		if e = boolField(v, k); e != nil {
			return o, e
		}
	}
	if !boolAt(v, "historyComplete") {
		return o, errors.New("history not observed")
	}
	o.complete = boolAt(v, "reservationsComplete") && boolAt(v, "attemptsComplete")
	set := value(v, "reservationSet")
	if e = object(set, "profile queueId entries"); e != nil {
		return o, e
	}
	if stringAt(set, "profile") != "taskman-reservation-set/0" || stringAt(set, "queueId") != stringAt(c.queue, "queueId") {
		return o, errors.New("reservation identity")
	}
	o.reservationDigest = sum(append(canonical(set), '\n'))
	rows, e := array(value(set, "entries"), 128)
	if e != nil {
		return o, e
	}
	known := map[string]ticket{}
	for _, t := range c.tickets {
		known[t.id] = t
	}
	attempts := map[string]bool{}
	ticketIDs := map[string]bool{}
	head, _ := number(value(c.snapshot, "headSeq"), ^uint64(0))
	for _, r := range rows {
		if e = object(r, "attemptId generation ticketId ticketRevision resources capacityUses workers state createdSeq coverage"); e != nil {
			return o, e
		}
		id := stringAt(r, "attemptId")
		tid := stringAt(r, "ticketId")
		if !nativeID(id, "attempt", stringAt(c.queue, "queueId")) || !nativeID(tid, "ticket", stringAt(c.queue, "queueId")) || attempts[id] || ticketIDs[tid] {
			return o, errors.New("duplicate or invalid reservation identity")
		}
		attempts[id] = true
		ticketIDs[tid] = true
		for _, k := range []string{"generation", "createdSeq"} {
			n, err := number(value(r, k), ^uint64(0))
			if err != nil || n == 0 || k == "createdSeq" && n > head {
				return o, errors.New("reservation sequence")
			}
		}
		n, err := number(value(r, "ticketRevision"), 2147483647)
		if current, ok := known[tid]; err != nil || n == 0 || !ok || count(n) != current.revision {
			return o, errors.New("reservation revision or ticket mismatch")
		}
		workers, err := number(value(r, "workers"), 2147483647)
		if err != nil || workers == 0 {
			return o, errors.New("reservation workers")
		}
		if !oneOf(stringAt(r, "state"), "ACTIVE QUIESCING BLOCKED_RECOVERY") || !oneOf(stringAt(r, "coverage"), "QUALIFIED WHOLE_REPOSITORY") {
			return o, errors.New("reservation state")
		}
		uses, err := array(value(r, "capacityUses"), 128)
		if err != nil || len(uses) > 0 {
			return o, errors.New("fixture reservation capacity classes unsupported")
		}
		resources, err := resources(value(r, "resources"))
		if err != nil {
			return o, err
		}
		if stringAt(r, "coverage") == "WHOLE_REPOSITORY" {
			resources = normalized(append(resources, Resource{"WHOLE_REPOSITORY", "repository"}))
		}
		declared := append([]Resource{}, known[tid].resources...)
		for _, path := range known[tid].paths {
			declared = append(declared, Resource{"PATH", path})
		}
		for _, required := range declared {
			if !covers(resources, required) {
				return o, errors.New("reservation omits declared resource")
			}
		}
		o.reservations = append(o.reservations, reservation{tid, workers, resources})
	}
	history, e := array(value(v, "history"), 256)
	if e != nil {
		return o, e
	}
	prior := uint64(0)
	for _, v := range history {
		p, err := decodePlan(v)
		if err != nil {
			return o, err
		}
		seq, err := number(value(v, "headSeq"), ^uint64(0))
		if err != nil || seq <= prior || seq > head || p.QueueID != stringAt(c.queue, "queueId") || p.PolicySHA256 != stringAt(c.queue, "policySha256") {
			return o, errors.New("history binding or order")
		}
		for _, entry := range p.Entries {
			current, ok := known[entry.TicketID]
			n, _ := number(wire.Value{Kind: wire.KindString, Str: entry.TicketRevision}, 2147483647)
			latest, _ := number(wire.Value{Kind: wire.KindString, Str: current.revision}, 2147483647)
			if !ok || n > latest {
				return o, errors.New("history ticket or revision mismatch")
			}
		}
		prior = seq
		o.history = append(o.history, p)
	}
	return o, nil
}
func covers(resources []Resource, required Resource) bool {
	for _, held := range resources {
		if held.Class == "WHOLE_REPOSITORY" || held == required {
			return true
		}
		if held.Class == "PATH" && required.Class == "PATH" && strings.HasSuffix(held.Key, "/") && strings.HasPrefix(required.Key, held.Key) {
			return true
		}
	}
	return false
}
func decodePlan(v wire.Value) (Plan, error) {
	p := Plan{}
	if e := object(v, "profile planningProfile queueId policySha256 headSeq intentTreeSha256 reservationSetSha256 capacity entries mutationAuthority"); e != nil {
		return p, e
	}
	if stringAt(v, "profile") != "taskman-plan/0" || stringAt(v, "planningProfile") != "taskman-priority-first/0" || value(v, "mutationAuthority").Kind != wire.KindBool || boolAt(v, "mutationAuthority") {
		return p, errors.New("history profile")
	}
	for _, k := range []string{"policySha256", "intentTreeSha256", "reservationSetSha256"} {
		if !digest(stringAt(v, k)) {
			return p, errors.New("history digest")
		}
	}
	queue := stringAt(v, "queueId")
	head, e := number(value(v, "headSeq"), ^uint64(0))
	if !queueID(queue) || e != nil || head == 0 {
		return p, errors.New("history identity or head")
	}
	capacity := value(v, "capacity")
	if e := object(capacity, "maxActiveAttempts availableWorkers"); e != nil {
		return p, e
	}
	for _, k := range []string{"maxActiveAttempts", "availableWorkers"} {
		if _, e := number(value(capacity, k), 2147483647); e != nil {
			return p, e
		}
	}
	entries, e := array(value(v, "entries"), 1000)
	if e != nil {
		return p, e
	}
	ids := map[string]bool{}
	for _, entry := range entries {
		if e = object(entry, "ticketId ticketRevision resources closureComplete state reason deferredSinceSeq blockers"); e != nil {
			return p, e
		}
		id := stringAt(entry, "ticketId")
		if !nativeID(id, "ticket", queue) || ids[strings.ToLower(id)] {
			return p, errors.New("history ticket identity")
		}
		ids[strings.ToLower(id)] = true
		if n, err := number(value(entry, "ticketRevision"), 2147483647); err != nil || n == 0 {
			return p, errors.New("history ticket revision")
		}
		if _, e = resources(value(entry, "resources")); e != nil {
			return p, e
		}
		if e = boolField(entry, "closureComplete"); e != nil {
			return p, e
		}
		if !oneOf(stringAt(entry, "state"), "SELECTED DEFERRED BLOCKED") || !oneOf(stringAt(entry, "reason"), nativeCodes) {
			return p, errors.New("history state")
		}
		if value(entry, "deferredSinceSeq").Kind != wire.KindNull {
			if n, err := number(value(entry, "deferredSinceSeq"), ^uint64(0)); err != nil || n == 0 || n > head {
				return p, errors.New("history deferral sequence")
			}
		}
		blockers, err := stringsAt(entry, "blockers", 1000)
		if err != nil {
			return p, err
		}
		for _, blocker := range blockers {
			if !oneOf(blocker, nativeCodes) && !nativeID(blocker, "ticket", queue) {
				return p, errors.New("history blocker")
			}
		}
	}
	e = json.NewDecoder(bytes.NewReader(canonical(v))).Decode(&p)
	return p, e
}
