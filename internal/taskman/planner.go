package taskman

import (
	"errors"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/workqueue"
)

type Resource struct {
	Class string `json:"class"`
	Key   string `json:"key"`
}
type Entry struct {
	TicketID         string     `json:"ticketId"`
	TicketRevision   string     `json:"ticketRevision"`
	Resources        []Resource `json:"resources"`
	ClosureComplete  bool       `json:"closureComplete"`
	State            string     `json:"state"`
	Reason           string     `json:"reason"`
	DeferredSinceSeq *string    `json:"deferredSinceSeq"`
	Blockers         []string   `json:"blockers"`
}
type Capacity struct {
	MaxActiveAttempts string `json:"maxActiveAttempts"`
	AvailableWorkers  string `json:"availableWorkers"`
}
type Plan struct {
	Profile              string   `json:"profile"`
	PlanningProfile      string   `json:"planningProfile"`
	QueueID              string   `json:"queueId"`
	PolicySHA256         string   `json:"policySha256"`
	HeadSeq              string   `json:"headSeq"`
	IntentTreeSHA256     string   `json:"intentTreeSha256"`
	ReservationSetSHA256 string   `json:"reservationSetSha256"`
	Capacity             Capacity `json:"capacity"`
	Entries              []Entry  `json:"entries"`
	MutationAuthority    bool     `json:"mutationAuthority"`
}
type ticket struct {
	raw                            wire.Value
	id, revision, status, priority string
	order                          uint64
	paths                          []string
	resources                      []Resource
}
type reservation struct {
	ticket    string
	workers   uint64
	resources []Resource
}
type observation struct {
	raw               wire.Value
	reservations      []reservation
	reservationDigest string
	complete          bool
	history           []Plan
}
type captured struct {
	queue, policy, snapshot wire.Value
	tickets                 []ticket
	digests                 map[string]string
	observed                observation
	index                   *contextindex.Index
}

func resources(v wire.Value) ([]Resource, error) {
	a, e := array(v, 4096)
	if e != nil {
		return nil, e
	}
	r := make([]Resource, 0, len(a))
	for _, x := range a {
		if e = object(x, "class key"); e != nil {
			return nil, e
		}
		c, k := stringAt(x, "class"), stringAt(x, "key")
		if !identifier(k) {
			return nil, errors.New("resource key")
		}
		switch c {
		case "PATH":
			if !validPath(k) {
				return nil, errors.New("resource path")
			}
		case "SHARED_GATE", "SCHEMA", "GENERATED_OUTPUT", "PORT", "DATABASE", "WHOLE_REPOSITORY", "OTHER":
		default:
			return nil, errors.New("resource class")
		}
		r = append(r, Resource{c, k})
	}
	return normalized(r), nil
}
func normalized(r []Resource) []Resource {
	m := map[Resource]bool{}
	for _, x := range r {
		m[x] = true
	}
	out := make([]Resource, 0, len(m))
	for x := range m {
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Class != out[j].Class {
			return out[i].Class < out[j].Class
		}
		return out[i].Key < out[j].Key
	})
	return out
}
func overlap(a, b Resource) bool {
	if a.Class == "WHOLE_REPOSITORY" || b.Class == "WHOLE_REPOSITORY" {
		return true
	}
	if a.Class != b.Class {
		return false
	}
	if a.Key == b.Key {
		return true
	}
	if a.Class != "PATH" {
		return false
	}
	return strings.HasPrefix(a.Key, strings.TrimSuffix(b.Key, "/")+"/") && strings.HasSuffix(b.Key, "/") || strings.HasPrefix(b.Key, strings.TrimSuffix(a.Key, "/")+"/") && strings.HasSuffix(a.Key, "/") || strings.TrimSuffix(a.Key, "/") == strings.TrimSuffix(b.Key, "/")
}
func collide(a, b []Resource) bool {
	for _, group := range [][]Resource{a, b} {
		for _, r := range group {
			if r.Class == "WHOLE_REPOSITORY" {
				return true
			}
		}
	}
	for _, x := range a {
		for _, y := range b {
			if overlap(x, y) {
				return true
			}
		}
	}
	return false
}
func nativeClosure(t ticket, index *contextindex.Index, source workqueue.CollisionSource) ([]Resource, bool) {
	pathsDeclared := append([]string{}, t.paths...)
	for _, resource := range t.resources {
		if resource.Class == "PATH" {
			pathsDeclared = append(pathsDeclared, resource.Key)
		}
	}
	r := append([]Resource{}, t.resources...)
	for _, p := range pathsDeclared {
		r = append(r, Resource{"PATH", p})
	}
	complete := index != nil
	if index == nil {
		return normalized(r), false
	}
	paths, ok := source.Closure(pathsDeclared)
	complete = complete && ok
	for _, p := range paths {
		r = append(r, Resource{"PATH", p})
	}
	for _, p := range pathsDeclared {
		if strings.HasSuffix(p, "/") {
			complete = false
			continue
		}
		if _, ok := index.Sources[p]; !ok {
			complete = false
		}
		if !strings.HasSuffix(p, ".go") {
			complete = false
		}
		for _, u := range index.Unparsed {
			if strings.HasSuffix(u.Path, ".go") {
				complete = false
			}
		}
	}
	for _, excluded := range index.Exclusions {
		if strings.HasSuffix(excluded.Path, ".go") {
			complete = false
		}
	}
	return normalized(r), complete
}
func blocker(t ticket, all map[string]ticket, c captured) string {
	if t.status != "OPEN" || len(value(t.raw, "holds").Arr) > 0 {
		return "TICKET_STATE"
	}
	if !c.observed.complete {
		return "MISSING_EVIDENCE"
	}
	for _, r := range c.observed.reservations {
		if r.ticket == t.id {
			return "ATTEMPT_LIVE"
		}
	}
	if boolAt(value(t.raw, "effects"), "externalUnbounded") {
		return "EXTERNAL_UNBOUNDED"
	}
	execution := stringAt(t.raw, "executionClass")
	if execution != "AUTONOMOUS" && execution != "APPROVAL_REQUIRED" {
		return "TICKET_STATE"
	}
	if execution == "APPROVAL_REQUIRED" {
		found := false
		for _, a := range value(t.raw, "approvals").Arr {
			if stringAt(a, "operation") == "RUN" && stringAt(a, "targetRevision") == t.revision && !boolAt(a, "revoked") {
				found = true
			}
		}
		if !found {
			return "APPROVAL_MISSING"
		}
	}
	for _, d := range value(t.raw, "dependencies").Arr {
		if stringAt(d, "obligation") == "GATE_PASSED" {
			return "GATE_UNKNOWN"
		}
		dep, ok := all[stringAt(d, "ticketId")]
		if !ok {
			return "DEPENDENCY_MISSING"
		}
		if dep.status != "COMPLETED" && !(dep.status == "ARCHIVED" && stringAt(dep.raw, "archivedFrom") == "COMPLETED") {
			return "DEPENDENCY_UNSATISFIED"
		}
	}
	if len(value(t.raw, "capabilities").Arr) > 0 {
		return "CAPABILITY_UNAVAILABLE"
	}
	if len(value(value(c.policy, "capacity"), "classes").Arr) > 0 {
		return "UNSUPPORTED"
	}
	if value(c.snapshot, "barrier").Kind != wire.KindNull || stringAt(c.queue, "writeBarrier") != "NONE" {
		return "PAUSED"
	}
	return ""
}
func age(t ticket, history []Plan) *string {
	var result *string
	for _, p := range history {
		for _, e := range p.Entries {
			if e.TicketID != t.id || e.TicketRevision != t.revision {
				continue
			}
			if e.State == "SELECTED" {
				result = nil
			}
			if e.State == "DEFERRED" && result == nil {
				s := p.HeadSeq
				result = &s
			}
		}
	}
	return result
}
func plan(c captured) (Plan, error) {
	capacity := value(c.policy, "capacity")
	active, e := number(value(capacity, "maxActiveAttempts"), 2147483647)
	if e != nil {
		return Plan{}, e
	}
	workers, e := number(value(capacity, "maxWorkersTotal"), 2147483647)
	if e != nil {
		return Plan{}, e
	}
	used := uint64(0)
	for _, r := range c.observed.reservations {
		used += r.workers
	}
	available := uint64(0)
	if c.observed.complete && used < workers {
		available = workers - used
	}
	p := Plan{Profile: "taskman-plan/0", PlanningProfile: "taskman-priority-first/0", QueueID: stringAt(c.queue, "queueId"), PolicySHA256: stringAt(c.queue, "policySha256"), HeadSeq: stringAt(c.snapshot, "headSeq"), IntentTreeSHA256: stringAt(c.snapshot, "intentTreeSha256"), ReservationSetSHA256: c.observed.reservationDigest, Capacity: Capacity{count(active), count(available)}, Entries: []Entry{}}
	tickets := append([]ticket{}, c.tickets...)
	sort.Slice(tickets, func(i, j int) bool { return ticketLess(tickets[i], tickets[j]) })
	all := map[string]ticket{}
	for _, t := range tickets {
		all[t.id] = t
	}
	selected := []Entry{}
	closure := workqueue.IndexCollisionSource(c.index)
	for _, t := range tickets {
		r, complete := nativeClosure(t, c.index, closure)
		if len(r) > 4096 {
			return Plan{}, errors.New("collision resource bound")
		}
		entry := Entry{TicketID: t.id, TicketRevision: t.revision, Resources: r, ClosureComplete: complete, State: "BLOCKED", Reason: blocker(t, all, c), DeferredSinceSeq: age(t, c.observed.history), Blockers: []string{}}
		if entry.Reason == "" {
			entry = choose(entry, t, c, selected, active, available)
		}
		if entry.State == "SELECTED" {
			selected = append(selected, entry)
			available--
		}
		if entry.State != "SELECTED" && len(entry.Blockers) == 0 {
			entry.Blockers = []string{entry.Reason}
		}
		if len(entry.Resources) > 4096 {
			return Plan{}, errors.New("collision resource bound")
		}
		for _, r := range entry.Resources {
			if !identifier(r.Key) {
				return Plan{}, errors.New("native resource key bound")
			}
		}
		p.Entries = append(p.Entries, entry)
	}
	return p, nil
}
func choose(e Entry, t ticket, c captured, selected []Entry, active, workers uint64) Entry {
	qualified := stringAt(value(t.raw, "effects"), "coverage") == "QUALIFIED" && e.ClosureComplete
	if !qualified {
		if stringAt(c.policy, "serialFallback") != "WHOLE_REPOSITORY" {
			e.Reason = "COVERAGE_UNKNOWN"
			return e
		}
		e.Resources = normalized(append(e.Resources, Resource{"WHOLE_REPOSITORY", "repository"}))
	}
	e.State = "DEFERRED"
	for _, r := range c.observed.reservations {
		if collide(e.Resources, r.resources) || !qualified {
			e.Reason = "RESOURCE_COLLISION"
			e.Blockers = []string{r.ticket}
			return e
		}
	}
	for _, s := range selected {
		if collide(e.Resources, s.Resources) || !qualified {
			e.Reason = "RESOURCE_COLLISION"
			e.Blockers = []string{s.TicketID}
			return e
		}
	}
	if uint64(len(c.observed.reservations)+len(selected)) >= active || workers == 0 {
		e.Reason = "LIMIT_EXCEEDED"
		return e
	}
	e.State = "SELECTED"
	e.Reason = "DEVELOPMENT_MODE"
	return e
}

func ticketLess(a, b ticket) bool {
	if a.priority != b.priority {
		return a.priority < b.priority
	}
	if a.order != b.order {
		return a.order < b.order
	}
	return a.id < b.id
}
