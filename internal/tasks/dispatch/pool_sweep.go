package dispatch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// PoolSweepConfig opts into one owned native operation at a time.
type PoolSweepConfig struct {
	TimeoutSeconds  int `json:"timeoutSeconds"`
	IntervalSeconds int `json:"intervalSeconds"`
}

// PoolSweepRequest retains the immutable admission facts across restarts. The
// definition is a first-admission observation; allocation is a native selector.
type PoolSweepRequest struct {
	WorkRoot       string `json:"workRoot"`
	Program        string `json:"program"`
	Queue          string `json:"queue"`
	Pool           string `json:"pool"`
	Member         string `json:"member"`
	Allocation     string `json:"allocation"`
	Definition     string `json:"definition"`
	RequestID      string `json:"requestId"`
	Actor          string `json:"actor"`
	ActorRole      string `json:"actorRole"`
	ConfigDigest   string `json:"configDigest"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
}

type PoolSweepResult struct {
	Pending    bool   `json:"pending"`
	Evidence   string `json:"evidence,omitempty"`
	Receipt    string `json:"receipt,omitempty"`
	ReceiptSeq string `json:"receiptSeq,omitempty"`
	Outcome    string `json:"outcome,omitempty"`
}

type PoolSweepRecord struct {
	PoolSweepRequest
	Phase    string          `json:"phase"`
	Started  time.Time       `json:"started"`
	Observed time.Time       `json:"observed"`
	Result   PoolSweepResult `json:"result"`
	Reason   string          `json:"reason,omitempty"`
}

// PoolSweepQueue is an explicit opt-in boundary; legacy Queue implementations
// never gain command authority from a configuration field.
type PoolSweepQueue interface {
	PoolSweepActor() (id, role string, err error)
	PoolSweep(context.Context, PoolSweepRequest) (PoolSweepResult, error)
}

type poolSweepReturn struct {
	result PoolSweepResult
	err    error
}
type poolSweepJob struct {
	record   PoolSweepRecord
	cancel   context.CancelFunc
	done     <-chan poolSweepReturn
	returned *poolSweepReturn
}

func sweepID(program, queue, allocation string) string {
	h := sha256.Sum256([]byte(program + "\x00" + queue + "\x00" + allocation))
	return hex.EncodeToString(h[:])
}
func sweepKey(queue, pool, member string) string {
	h := sha256.Sum256([]byte(queue + "\x00" + pool + "\x00" + member))
	return hex.EncodeToString(h[:])
}
func sweepPending(phase string) bool {
	return phase == "STARTING" || phase == "RUNNING" || phase == "PENDING" || phase == "UNKNOWN"
}

// Strict parsing is activated only for the new opt-in field. Preserve the old
// profile's decoding behavior when it is absent.
func strictSweepConfig(raw []byte) bool {
	var members map[string]json.RawMessage
	if json.Unmarshal(raw, &members) != nil {
		return false
	}
	present := false
	for name := range members {
		if strings.EqualFold(name, "poolSweep") {
			present = true
			if name != "poolSweep" {
				return false
			}
		}
	}
	if !present {
		return true
	}
	if !validScalarJSON(raw) || bytes.Equal(bytes.TrimSpace(members["poolSweep"]), []byte("null")) {
		return false
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	if t, e := d.Token(); e != nil || t != json.Delim('{') {
		return false
	}
	seen := map[string]bool{}
	for d.More() {
		t, e := d.Token()
		if e != nil {
			return false
		}
		k, ok := t.(string)
		if !ok || seen[k] {
			return false
		}
		seen[k] = true
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return false
		}
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(members["poolSweep"], &fields) != nil || len(fields) != 2 {
		return false
	}
	for _, k := range []string{"timeoutSeconds", "intervalSeconds"} {
		if len(fields[k]) == 0 || bytes.Equal(bytes.TrimSpace(fields[k]), []byte("null")) {
			return false
		}
	}
	// Detect nested duplicates, which a map alone would erase.
	d = json.NewDecoder(bytes.NewReader(members["poolSweep"]))
	if t, e := d.Token(); e != nil || t != json.Delim('{') {
		return false
	}
	seen = map[string]bool{}
	for d.More() {
		t, e := d.Token()
		if e != nil {
			return false
		}
		k, ok := t.(string)
		if !ok || seen[k] {
			return false
		}
		seen[k] = true
		var v json.RawMessage
		if d.Decode(&v) != nil {
			return false
		}
	}
	return true
}

func (l *Ledger) validatePoolSweeps() error {
	if len(l.PoolSweeps) > 256 {
		return errors.New("pool sweep member history capacity exceeded")
	}
	outstanding := 0
	for key, r := range l.PoolSweeps {
		if r == nil || key != sweepKey(r.Queue, r.Pool, r.Member) || r.Program != l.Program || !clean(r.WorkRoot) || !ValidName(r.Program) || r.RequestID != sweepID(r.Program, r.Queue, r.Allocation) {
			return errors.New("pool sweep identity differs")
		}
		if _, e := wire.ParseQueueID("queue", r.Queue); e != nil {
			return e
		}
		for _, s := range []string{r.Allocation, r.Definition, r.ConfigDigest} {
			if !validProgressDigest(s) {
				return errors.New("invalid pool sweep digest")
			}
		}
		for _, s := range []string{r.Pool, r.Member} {
			if _, e := wire.ParseLabel("member", s); e != nil {
				return e
			}
		}
		if _, e := wire.ParseLabel("actor", r.Actor); e != nil {
			return e
		}
		if (r.ActorRole != "OWNER" && r.ActorRole != "OPERATOR") || r.TimeoutSeconds < 1 || r.TimeoutSeconds > 1800 || r.Started.IsZero() || r.Observed.IsZero() || r.Observed.Before(r.Started) || len(r.Reason) > 1024 {
			return errors.New("invalid pool sweep binding or time")
		}
		if r.Phase != "TERMINAL" && !sweepPending(r.Phase) {
			return errors.New("invalid pool sweep phase")
		}
		if sweepPending(r.Phase) {
			outstanding++
		}
		if r.Result.Evidence != "" && !validProgressDigest(r.Result.Evidence) {
			return errors.New("invalid sweep evidence")
		}
		if r.Result.ReceiptSeq != "" {
			if _, e := wire.ParseSize("receiptSeq", r.Result.ReceiptSeq); e != nil {
				return e
			}
		}
		if len(r.Result.Receipt) > 128 || len(r.Result.Outcome) > 128 {
			return errors.New("invalid sweep receipt/result")
		}
	}
	if outstanding > 1 {
		return errors.New("multiple outstanding pool sweeps")
	}
	return nil
}

// commitPoolSweep validates and bounds the entire candidate, including workers,
// backoff and progress, before a save hook or rename can publish it.
func (d *Dispatcher) commitPoolSweep(r PoolSweepRecord) error {
	raw, e := json.Marshal(d.ledger)
	if e != nil {
		return e
	}
	var next Ledger
	if e = json.Unmarshal(raw, &next); e != nil {
		return e
	}
	if next.PoolSweeps == nil {
		next.PoolSweeps = map[string]*PoolSweepRecord{}
	}
	next.PoolSweeps[sweepKey(r.Queue, r.Pool, r.Member)] = &r
	if e = next.validatePoolSweeps(); e != nil {
		return e
	}
	encoded, e := ledgerBytes(&next)
	if e != nil {
		return e
	}
	if d.poolSweepSave != nil {
		e = d.poolSweepSave(&next, d.dir)
	} else {
		e = writeAtomic(d.dir+"/state.json", encoded)
	}
	if e != nil {
		return e
	}
	d.ledger = &next
	return nil
}

func (d *Dispatcher) sweepLaneHeld(pool, member string) bool {
	if d.sweepJob != nil && d.sweepJob.record.Pool == pool && d.sweepJob.record.Member == member {
		return true
	}
	// Disabling the opt-in releases lanes held only by retained records. The
	// records stay for a later enable; native sweep ownership still refuses claims.
	if d.Config.PoolSweep == nil {
		return false
	}
	for _, r := range d.ledger.PoolSweeps {
		if r.Pool == pool && r.Member == member && sweepPending(r.Phase) {
			return true
		}
	}
	return false
}

// tickPoolSweep runs only on the dispatcher goroutine. The background call owns
// no ledger pointer, event/output writer or worker state.
func (d *Dispatcher) tickPoolSweep(ctx context.Context, obs *Observation) (*Observation, error) {
	if d.sweepJob != nil {
		job := d.sweepJob
		if d.Config.PoolSweep == nil {
			job.cancel()
		}
		if job.returned == nil {
			select {
			case got := <-job.done:
				job.returned = &got
			default:
				return obs, nil
			}
		}
		// Re-observe before releasing this lane; an old snapshot cannot free a successor.
		fresh, e := d.observe(ctx)
		if e != nil {
			return obs, e
		}
		if e = d.publishSweepReturn(job); e != nil {
			return obs, e
		}
		obs = fresh
	}
	if d.Config.PoolSweep == nil || ctx.Err() != nil {
		return obs, ctx.Err()
	}
	q, ok := d.Queue.(PoolSweepQueue)
	if !ok {
		return obs, errors.New("pool sweep enabled but native adapter unsupported")
	}
	actor, role, e := q.PoolSweepActor()
	if e != nil {
		return obs, e
	}
	if d.sweepTried == nil {
		d.sweepTried = map[string]bool{}
	}
	// Reconcile retained identity before checking today's eligibility/definition.
	keys := make([]string, 0, len(d.ledger.PoolSweeps))
	for k := range d.ledger.PoolSweeps {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		r := d.ledger.PoolSweeps[k]
		if !sweepPending(r.Phase) {
			continue
		}
		if r.Actor != actor || r.ActorRole != role {
			return obs, errors.New("pool sweep original actor differs; reconciliation held")
		}
		// Replay of the same identity never re-executes a phase; retry it once
		// per interval so explicit recovery can end it without a restart.
		if !d.sweepTried[r.RequestID] || !d.Now().Before(d.sweepNext) {
			d.startPoolSweep(ctx, q, *r)
		}
		return obs, nil
	}
	if !d.sweepNext.IsZero() && d.Now().Before(d.sweepNext) {
		return obs, nil
	}
	members := append([]Member{}, obs.Members...)
	sort.Slice(members, func(i, j int) bool {
		if members[i].Pool != members[j].Pool {
			return members[i].Pool < members[j].Pool
		}
		return members[i].Member < members[j].Member
	})
	configRaw, e := json.Marshal(d.Config)
	if e != nil {
		return obs, e
	}
	configDigest := sha256.Sum256(configRaw)
	for _, m := range members {
		if m.State != "QUARANTINED" || !m.SafeReuse || m.Owned || m.Allocation == "" || m.Definition == "" || m.Queue == "" {
			continue
		}
		busy := false
		for _, w := range d.ledger.Workers {
			if w.Key == laneKey(m.Pool, m.Member) {
				busy = true
			}
		}
		if busy {
			continue
		}
		key := sweepKey(m.Queue, m.Pool, m.Member)
		prior := d.ledger.PoolSweeps[key]
		if prior != nil && prior.Allocation == m.Allocation {
			continue
		}
		now := d.Now().UTC()
		r := PoolSweepRecord{PoolSweepRequest: PoolSweepRequest{WorkRoot: d.Config.WorkRoot, Program: d.Program, Queue: m.Queue, Pool: m.Pool, Member: m.Member, Allocation: m.Allocation, Definition: m.Definition, RequestID: sweepID(d.Program, m.Queue, m.Allocation), Actor: actor, ActorRole: role, ConfigDigest: hex.EncodeToString(configDigest[:]), TimeoutSeconds: d.Config.PoolSweep.TimeoutSeconds}, Phase: "STARTING", Started: now, Observed: now}
		if e = d.commitPoolSweep(r); e != nil {
			return obs, e
		}
		d.startPoolSweep(ctx, q, r)
		break
	}
	return obs, nil
}
func (d *Dispatcher) startPoolSweep(ctx context.Context, q PoolSweepQueue, r PoolSweepRecord) {
	op, cancel := context.WithCancel(ctx)
	done := make(chan poolSweepReturn, 1)
	d.sweepJob = &poolSweepJob{record: r, cancel: cancel, done: done}
	d.sweepTried[r.RequestID] = true
	// Retaining STARTING until the terminal publication makes a crash at any
	// point reconcile the same native identity; RUNNING is only an in-memory event.
	d.emit(Event{Kind: "pool-sweep-started", Message: fmt.Sprintf("pool sweep %s member %s allocation %s", r.RequestID, r.Member, r.Allocation)})
	go func() { result, err := q.PoolSweep(op, r.PoolSweepRequest); done <- poolSweepReturn{result, err} }()
}
func (d *Dispatcher) publishSweepReturn(job *poolSweepJob) error {
	r := job.record
	r.Result = job.returned.result
	r.Observed = d.Now().UTC()
	if r.Observed.Before(r.Started) {
		r.Observed = r.Started
	}
	r.Phase = "TERMINAL"
	if job.returned.err != nil {
		r.Phase = "UNKNOWN"
		r.Reason = "native outcome unavailable; reconcile original request"
	} else if r.Result.Pending {
		r.Phase = "PENDING"
	}
	if e := d.commitPoolSweep(r); e != nil {
		return fmt.Errorf("pool sweep joined; terminal publication UNKNOWN: %w", e)
	}
	job.cancel()
	d.sweepJob = nil
	interval := 1
	if d.Config.PoolSweep != nil {
		interval = d.Config.PoolSweep.IntervalSeconds
	}
	d.sweepNext = d.Now().Add(time.Duration(interval) * time.Second)
	d.emit(Event{Kind: "pool-sweep-returned", Message: fmt.Sprintf("pool sweep %s %s receipt %s", r.RequestID, r.Phase, r.Result.ReceiptSeq)})
	return nil
}

// stopPoolSweep NEVER races a timer against a still-live native writer. The
// bounded native engine must actually return; supervision continues until join.
func (d *Dispatcher) stopPoolSweep() error {
	if d.sweepJob == nil {
		return nil
	}
	job := d.sweepJob
	job.cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for job.returned == nil {
		select {
		case got := <-job.done:
			job.returned = &got
		case <-ticker.C:
			d.supervise()
		}
	}
	return d.publishSweepReturn(job)
}
