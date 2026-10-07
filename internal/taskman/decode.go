package taskman

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
	taskswire "github.com/Beamfall/corvint/internal/tasks/wire"
)

// Native task-store SPEC section 11 codes and the closed ticket record keys,
// imported from the in-tree Tasks wire package (decision 0397).
var nativeCodes = strings.Join(taskswire.Codes, " ")

var ticketKeys = strings.Join(taskswire.TicketRecordKeys, " ")

func oneOf(s, allowed string) bool {
	for _, x := range strings.Fields(allowed) {
		if s == x {
			return true
		}
	}
	return false
}

// optionalTicketMembers validates each record key a ticket may omit, given the
// record's revision and acceptance revision. ticketObject refuses a shared
// optional key with no entry here, so a key the Tasks codec gains fails this
// reader closed until it is validated.
var optionalTicketMembers = map[string]func(v wire.Value, revision, acceptance uint64) error{
	"operatorNote": operatorNote,
	"requiresPool": func(v wire.Value, _, _ uint64) error {
		if v.Kind != wire.KindString {
			return errors.New("label required")
		}
		_, e := taskswire.ParseLabel("/requiresPool", v.Str)
		return e
	},
	"requiredRoles": func(v wire.Value, _, _ uint64) error {
		if e := object(v, "implement review integrate"); e != nil {
			return e
		}
		for _, stage := range []string{"implement", "review", "integrate"} {
			roles, e := array(value(v, stage), 1024)
			if e != nil || len(roles) == 0 {
				return errors.New("stage roles")
			}
			for i, r := range roles {
				if r.Kind != wire.KindString || !oneOf(r.Str, "BUILDER REVIEWER VERIFIER REPAIR DOCS") {
					return errors.New("stage role")
				}
				if i > 0 && roles[i-1].Str >= r.Str {
					return errors.New("unsorted/duplicate stage roles")
				}
			}
		}
		return nil
	},
	"escalations":            escalationRefs,
	"externalReviews":        externalReviews,
	"executionPrerequisites": executionPrerequisites,
	"attachedEvidence":       attachedEvidence,
	"knowHow":                knowHow,
}

// ticketObject checks the closed ticket record object and returns the
// optional members it carries.
func ticketObject(v wire.Value) ([]string, error) {
	if v.Kind != wire.KindObject {
		return nil, errors.New("object required")
	}
	keys := ticketKeys
	var present []string
	for _, k := range taskswire.TicketRecordOptionalKeys {
		if _, ok := v.Obj.Values[k]; !ok {
			continue
		}
		if optionalTicketMembers[k] == nil {
			return nil, fmt.Errorf("unsupported optional member %s", k)
		}
		present = append(present, k)
		keys += " " + k
	}
	return present, object(v, keys)
}

// escalationRefs checks the issue 502 reference (ESC-V0-002) as far as the
// record alone allows: its shape, the event/transaction capacity, the
// current-acceptance OPEN bound, and that it names no control write or
// acceptance revision after the record's own. Event chains stay with the Tasks
// readers, which hold the evidence blobs.
func escalationRefs(v wire.Value, revision, acceptance uint64) error {
	if e := object(v, "revision lastControlTicketRevision workRevision entries"); e != nil {
		return e
	}
	var counts [3]uint64
	for i, k := range []string{"revision", "lastControlTicketRevision", "workRevision"} {
		n, e := number(value(v, k), 2147483647)
		if e != nil || n == 0 {
			return errors.New("positive revision required")
		}
		counts[i] = n
	}
	if counts[2] >= counts[1] || counts[1] > revision {
		return errors.New("work/control revision")
	}
	entries, e := array(value(v, "entries"), 64)
	if e != nil || len(entries) == 0 {
		return errors.New("request count")
	}
	var total uint64
	open := 0
	for i, x := range entries {
		if e = object(x, "requestId originSha256 headSha256 revision acceptanceRevision kind state"); e != nil {
			return e
		}
		id := stringAt(x, "requestId")
		if value(x, "requestId").Kind != wire.KindString {
			return errors.New("request identifier")
		}
		if _, e = taskswire.ParseIdentifier("/escalations/entries/requestId", id); e != nil {
			return e
		}
		if i > 0 && stringAt(entries[i-1], "requestId") >= id {
			return errors.New("unsorted/duplicate requests")
		}
		if !digest(stringAt(x, "originSha256")) || !digest(stringAt(x, "headSha256")) {
			return errors.New("request digest")
		}
		n, e := number(value(x, "revision"), 64)
		if e != nil || n == 0 {
			return errors.New("request history cap")
		}
		total += n
		a, e := number(value(x, "acceptanceRevision"), 2147483647)
		if e != nil || a == 0 || a > acceptance {
			return errors.New("request acceptance revision")
		}
		if !oneOf(stringAt(x, "kind"), "decision infrastructure scope blocked") || !oneOf(stringAt(x, "state"), "OPEN ANSWERED SUPERSEDED") {
			return errors.New("request enum")
		}
		if stringAt(x, "state") == "OPEN" && a == acceptance {
			open++
		}
	}
	if total > 4096 || counts[0] > total || total > 2*counts[0] {
		return errors.New("event/transaction capacity")
	}
	if open > taskswire.EscalationMaxCurrentOpen {
		return errors.New("current OPEN request bound")
	}
	return nil
}

// escalationPending returns the sorted request IDs of the ESCALATION_PENDING
// derived hold (ESC-V0-006): current OPEN decision, scope or blocked
// questions, through the predicate the native Tasks reader shares (decision
// 0397). Infrastructure and stale questions do not hold admission.
func escalationPending(t ticket) []string {
	var entries []taskswire.EscalationHoldEntry
	for _, x := range value(value(t.raw, "escalations"), "entries").Arr {
		entries = append(entries, taskswire.EscalationHoldEntry{RequestID: stringAt(x, "requestId"), AcceptanceRevision: stringAt(x, "acceptanceRevision"), Kind: stringAt(x, "kind"), State: stringAt(x, "state")})
	}
	ids, _ := taskswire.EscalationPending(t.revision, entries)
	return ids
}
func boolField(v wire.Value, k string) error {
	if value(v, k).Kind != wire.KindBool {
		return errors.New("boolean required")
	}
	return nil
}
func decodeTicket(v wire.Value) (ticket, error) {
	t := ticket{raw: v, id: stringAt(v, "ticketId"), revision: stringAt(v, "acceptanceRevision"), status: stringAt(v, "status"), priority: stringAt(v, "priority")}
	present, e := ticketObject(v)
	if e != nil {
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
	for _, k := range present {
		if e = optionalTicketMembers[k](value(v, k), chain, rev); e != nil {
			return t, fmt.Errorf("%s: %w", k, e)
		}
	}
	if e = prerequisiteOwner(t.id, value(v, "executionPrerequisites")); e != nil {
		return t, fmt.Errorf("executionPrerequisites: %w", e)
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
	keys := "profile planningProfile queueId policySha256 headSeq intentTreeSha256 reservationSetSha256 capacity entries mutationAuthority"
	if hasMember(v, "resourceDeferred") {
		keys += " resourceDeferred"
	}
	if e := object(v, keys); e != nil {
		return p, e
	}
	pools, e := planPools(v)
	if e != nil {
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
			poolWait := pools[blocker] && stringAt(entry, "state") == "DEFERRED" && stringAt(entry, "reason") == "RESOURCE_COLLISION"
			if !oneOf(blocker, nativeCodes) && !nativeID(blocker, "ticket", queue) && !poolWait {
				return p, errors.New("history blocker")
			}
		}
	}
	e = json.NewDecoder(bytes.NewReader(canonical(v))).Decode(&p)
	return p, e
}

// operatorNote validates the optional closed {revision,current,head} reference
// with Core's own primitives: revision 1..4096, head a digest, current null or
// equal to head. Absence is valid; a whole-null reference is not
// (ON-V0-001).
func hasMember(v wire.Value, k string) bool {
	if v.Kind != wire.KindObject {
		return false
	}
	_, ok := v.Obj.Values[k]
	return ok
}

// planPools validates a default plan's optional resourceDeferred rows
// (CAL-V0-097) and returns their pool IDs: the only labels a DEFERRED
// RESOURCE_COLLISION entry may name as its blocker.
func planPools(v wire.Value) (map[string]bool, error) {
	pools := map[string]bool{}
	if !hasMember(v, "resourceDeferred") {
		return pools, nil
	}
	rows, e := array(value(v, "resourceDeferred"), 1000)
	if e != nil {
		return nil, e
	}
	for _, row := range rows {
		if e := object(row, "poolId availability freeEligibleMembers selected deferred"); e != nil {
			return nil, e
		}
		id := stringAt(row, "poolId")
		if _, e := taskswire.ParseLabel("/resourceDeferred/poolId", id); e != nil || value(row, "poolId").Kind != wire.KindString || pools[id] {
			return nil, errors.New("history resource pool")
		}
		pools[id] = true
		free := value(row, "freeEligibleMembers")
		switch stringAt(row, "availability") {
		case "OBSERVED":
			if _, e := number(free, 2147483647); e != nil {
				return nil, errors.New("history resource availability")
			}
		case "NOT_OBSERVED":
			if free.Kind != wire.KindNull {
				return nil, errors.New("history resource availability")
			}
		default:
			return nil, errors.New("history resource availability")
		}
		for _, k := range []string{"selected", "deferred"} {
			if _, e := number(value(row, k), 2147483647); e != nil {
				return nil, e
			}
		}
	}
	return pools, nil
}
func operatorNote(n wire.Value, _, _ uint64) error {
	if e := object(n, "revision current head"); e != nil {
		return errors.New("operator note reference")
	}
	if r, e := number(value(n, "revision"), 4096); e != nil || r == 0 {
		return errors.New("operator note revision")
	}
	head := value(n, "head")
	if head.Kind != wire.KindString || !digest(head.Str) {
		return errors.New("operator note head")
	}
	current := value(n, "current")
	if current.Kind != wire.KindNull && (current.Kind != wire.KindString || current.Str != head.Str) {
		return errors.New("operator note current")
	}
	return nil
}

// externalReviews validates the optional ERG-V0-009 gate reference map: 1..16
// label keys, each a closed {generation,revision,head} with
// 1 <= generation <= revision <= 4096 and head a digest. Absence is valid.
func externalReviews(m wire.Value, _, _ uint64) error {
	if m.Kind != wire.KindObject || len(m.Obj.Keys) == 0 || len(m.Obj.Keys) > 16 {
		return errors.New("external review references")
	}
	for _, gate := range m.Obj.Keys {
		if _, e := taskswire.ParseLabel("externalReviews", gate); e != nil {
			return errors.New("external review gate")
		}
		ref := m.Obj.Values[gate]
		if e := object(ref, "generation revision head"); e != nil {
			return errors.New("external review reference")
		}
		g, e1 := number(value(ref, "generation"), 4096)
		r, e2 := number(value(ref, "revision"), 4096)
		if e1 != nil || e2 != nil || g == 0 || g > r {
			return errors.New("external review counters")
		}
		if head := value(ref, "head"); head.Kind != wire.KindString || !digest(head.Str) {
			return errors.New("external review head")
		}
	}
	return nil
}

// attachedEvidence validates the optional TEA-V0-001 list: 1..32 closed
// entries in append order, each naming an acceptance revision in
// 1..acceptance that never decreases, 1..16 sorted unique digests that do not
// repeat within one acceptance revision, a label actor, a nonblank prose
// reason of at most 512 bytes and a timestamp. Absence is valid.
func attachedEvidence(v wire.Value, _, acceptance uint64) error {
	entries, e := array(v, taskswire.AttachedEvidenceMaxEntries)
	if e != nil || len(entries) == 0 {
		return errors.New("attached evidence entries")
	}
	seen := map[string]bool{}
	var prev uint64
	for _, x := range entries {
		if e = object(x, "acceptanceRevision actor evidence reason recordedAt"); e != nil {
			return errors.New("attached evidence entry")
		}
		n, e := number(value(x, "acceptanceRevision"), 2147483647)
		if e != nil || n == 0 || n > acceptance || n < prev {
			return errors.New("attached evidence acceptance revision")
		}
		prev = n
		if a := value(x, "actor"); a.Kind != wire.KindString {
			return errors.New("attached evidence actor")
		} else if _, e = taskswire.ParseLabel("actor", a.Str); e != nil {
			return errors.New("attached evidence actor")
		}
		r := value(x, "reason")
		if r.Kind != wire.KindString || strings.TrimSpace(r.Str) == "" {
			return errors.New("attached evidence reason")
		}
		if _, e = taskswire.ParseProse("reason", r.Str, 1, taskswire.AttachedEvidenceMaxReasonBytes); e != nil {
			return errors.New("attached evidence reason")
		}
		if t := value(x, "recordedAt"); t.Kind != wire.KindString {
			return errors.New("attached evidence time")
		} else if _, e = taskswire.ParseTimestamp("recordedAt", t.Str); e != nil {
			return errors.New("attached evidence time")
		}
		ds, e := array(value(x, "evidence"), taskswire.AttachedEvidenceMaxDigests)
		if e != nil || len(ds) == 0 {
			return errors.New("attached evidence digests")
		}
		for i, d := range ds {
			if d.Kind != wire.KindString || !digest(d.Str) || (i > 0 && ds[i-1].Str >= d.Str) {
				return errors.New("attached evidence digest")
			}
			key := count(n) + ":" + d.Str
			if seen[key] {
				return errors.New("attached evidence digest repeated within an acceptance revision")
			}
			seen[key] = true
		}
	}
	return nil
}
