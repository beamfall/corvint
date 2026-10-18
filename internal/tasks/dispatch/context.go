package dispatch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ContextProfile is the profile of a worker's ATR-V0-033 context record.
const ContextProfile = "taskman-dispatch-context/0"

// Context levels (ATR-V0-032, ATR-V0-033), in ascending order after NONE.
const (
	ContextNone    = "NONE"
	ContextWarn    = "WARN"
	ContextHandoff = "HANDOFF"
	ContextHard    = "HARD"
)

const (
	// maxContextTokens bounds each ATR-V0-030 threshold.
	maxContextTokens = 1_000_000_000
	// maxContextRecord bounds a context record's bytes.
	maxContextRecord = 64 << 10
	// contextFile is the record's name in the worker directory.
	contextFile = "context.json"
)

// ContextLimits is a role's ATR-V0-030 context thresholds. Zero is unset.
type ContextLimits struct {
	WarnTokens    uint64 `json:"warnTokens,omitempty"`
	HandoffTokens uint64 `json:"handoffTokens,omitempty"`
	HardTokens    uint64 `json:"hardTokens,omitempty"`
	// ExhaustedAfter is the ATR-V0-106 count of consecutive hard stops of
	// one role on one ticket that marks it CONTEXT_EXHAUSTED; 0 is 3.
	ExhaustedAfter uint64 `json:"exhaustedAfter,omitempty"`
}

var contextLimitMembers = []string{"warnTokens", "handoffTokens", "hardTokens", "exhaustedAfter"}

// UnmarshalJSON decodes the thresholds exactly: an unknown, repeated or
// case-folded member, null, 0 or a non-integer is refused.
func (c *ContextLimits) UnmarshalJSON(raw []byte) error {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil || m == nil {
		return errors.New("context is a JSON object")
	}
	if err := supervisor.ExactMembers(raw, contextLimitMembers); err != nil {
		return fmt.Errorf("context: %v", err)
	}
	var out ContextLimits
	dst := map[string]*uint64{"warnTokens": &out.WarnTokens, "handoffTokens": &out.HandoffTokens, "hardTokens": &out.HardTokens, "exhaustedAfter": &out.ExhaustedAfter}
	for k, v := range m {
		p, ok := dst[k]
		if !ok {
			return fmt.Errorf("context has unknown member %q", k)
		}
		if bytes.Equal(bytes.TrimSpace(v), []byte("null")) || json.Unmarshal(v, p) != nil || *p == 0 {
			return fmt.Errorf("context %s is a positive integer when present", k)
		}
	}
	*c = out
	return nil
}

// contextLevel is one set threshold.
type contextLevel struct {
	level  string
	tokens uint64
}

// levels lists the set thresholds in ascending level order.
func (c *ContextLimits) levels() []contextLevel {
	if c == nil {
		return nil
	}
	var out []contextLevel
	for _, l := range []contextLevel{{ContextWarn, c.WarnTokens}, {ContextHandoff, c.HandoffTokens}, {ContextHard, c.HardTokens}} {
		if l.tokens > 0 {
			out = append(out, l)
		}
	}
	return out
}

// validateContext checks a role's ATR-V0-030 thresholds against its usage
// format and the heal settings.
func (r *Role) validateContext(h Heal) error {
	c := r.Context
	if c == nil {
		return nil
	}
	set := c.levels()
	if len(set) == 0 {
		return errors.New("context needs at least one of warnTokens, handoffTokens, hardTokens")
	}
	for i, l := range set {
		if l.tokens > maxContextTokens {
			return fmt.Errorf("context thresholds must be 1..%d", maxContextTokens)
		}
		if i > 0 && l.tokens <= set[i-1].tokens {
			return errors.New("context thresholds must strictly ascend: warnTokens < handoffTokens < hardTokens")
		}
	}
	if !contextFormat(r.UsageFormat) {
		return errors.New("context needs usageFormat claude-code or opencode: no other stream reports a per-call context figure")
	}
	if c.HardTokens > 0 && !h.Handoff {
		return errors.New("context hardTokens needs heal.handoff, so the stopped worker's attempt is handed off")
	}
	if c.ExhaustedAfter > 0 && c.HardTokens == 0 {
		return errors.New("context exhaustedAfter needs hardTokens: it counts consecutive hard stops")
	}
	if c.ExhaustedAfter > maxExhaustedAfter {
		return fmt.Errorf("context exhaustedAfter must be 1..%d", maxExhaustedAfter)
	}
	return nil
}

// contextFormat reports a usage format whose stream carries a ATR-V0-031
// context figure.
func contextFormat(format string) bool { return format == "claude-code" || format == "opencode" }

// contextMarker is the event type of a context line, so an unreadable one
// is recognised (ATR-V0-031).
var contextMarker = map[string][]byte{"claude-code": []byte(`"assistant"`), "opencode": []byte(`"step_finish"`)}

// observeContext records one context observation of this read for
// noteContext; known false is an UNKNOWN estimate (ATR-V0-031).
func (u *WorkerUsage) observeContext(tokens uint64, known bool) {
	if contextFormat(u.Format) {
		u.ctxSeen, u.ctxKnown, u.ctxTokens = true, known, tokens
	}
}

// claudeContext reads a claude-code line as a ATR-V0-031 context line: a
// main-session assistant event's message usage. Other events are ignored.
func (u *WorkerUsage) claudeContext(b []byte, typ string) {
	if supervisor.ExactMembers(b, []string{"type", "message", "parent_tool_use_id"}) != nil {
		u.observeContext(0, false) // a repeated or aliased type may hide an assistant event
		return
	}
	if typ != "assistant" {
		return
	}
	var ev map[string]json.RawMessage
	if json.Unmarshal(b, &ev) != nil {
		u.observeContext(0, false)
		return
	}
	parent, ok := ev["parent_tool_use_id"]
	if !ok {
		u.observeContext(0, false)
		return
	}
	if !bytes.Equal(bytes.TrimSpace(parent), []byte("null")) {
		return // a subagent's message is not the session's context
	}
	n, ok := claudeMessageContext(ev["message"])
	u.observeContext(n, ok)
}

// claudeMessageContext sums an assistant message's usage counters.
func claudeMessageContext(raw json.RawMessage) (uint64, bool) {
	msg, ok := exactObject(raw, "usage")
	if !ok {
		return 0, false
	}
	usage, ok := exactObject(msg["usage"], "input_tokens", "output_tokens", "cache_creation_input_tokens", "cache_read_input_tokens")
	if !ok {
		return 0, false
	}
	var total uint64
	for i, k := range []string{"input_tokens", "output_tokens", "cache_creation_input_tokens", "cache_read_input_tokens"} {
		v, present := usage[k]
		if !present && i >= 2 {
			continue
		}
		var n uint64
		if !present || bytes.Equal(bytes.TrimSpace(v), []byte("null")) || json.Unmarshal(v, &n) != nil || total+n < total {
			return 0, false
		}
		total += n
	}
	return total, true
}

// exactObject decodes a JSON object that carries no member differing from
// one of names only by case; repeats were refused by the line's check.
func exactObject(raw json.RawMessage, names ...string) (map[string]json.RawMessage, bool) {
	var m map[string]json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &m) != nil || m == nil {
		return nil, false
	}
	for k := range m {
		for _, n := range names {
			if k != n && strings.EqualFold(k, n) {
				return nil, false
			}
		}
	}
	return m, true
}

// ContextCrossing is one recorded threshold crossing (ATR-V0-033).
type ContextCrossing struct {
	Level     string `json:"level"`
	Threshold string `json:"threshold"`
	Tokens    string `json:"tokens"`
	At        string `json:"at"`
}

// ContextRecord is a worker's ATR-V0-033 record and ATR-V0-034 notice.
// Tokens is decimal or UNKNOWN, ObservedAt RFC 3339 or UNKNOWN, and each
// threshold decimal or NONE.
type ContextRecord struct {
	Profile       string            `json:"profile"`
	Worker        string            `json:"worker"`
	Tokens        string            `json:"tokens"`
	ObservedAt    string            `json:"observedAt"`
	WarnTokens    string            `json:"warnTokens"`
	HandoffTokens string            `json:"handoffTokens"`
	HardTokens    string            `json:"hardTokens"`
	Level         string            `json:"level"`
	Crossings     []ContextCrossing `json:"crossings"`
}

func freshContextRecord(worker string) ContextRecord {
	return ContextRecord{Profile: ContextProfile, Worker: worker, Tokens: UsageUnknown, ObservedAt: UsageUnknown, WarnTokens: ContextNone, HandoffTokens: ContextNone, HardTokens: ContextNone, Level: ContextNone, Crossings: []ContextCrossing{}}
}

// Estimate is the record's known estimate.
func (r *ContextRecord) Estimate() (uint64, bool) { return decimal(r.Tokens) }

// crossed reports a recorded crossing of level.
func (r *ContextRecord) crossed(level string) *ContextCrossing {
	for i := range r.Crossings {
		if r.Crossings[i].Level == level {
			return &r.Crossings[i]
		}
	}
	return nil
}

// decimal reads a canonical unsigned decimal.
func decimal(s string) (uint64, bool) {
	n, err := strconv.ParseUint(s, 10, 64)
	return n, err == nil && strconv.FormatUint(n, 10) == s
}

func (r *ContextRecord) encode() []byte {
	raw, _ := json.Marshal(r)
	return append(raw, '\n')
}

var contextLevels = []string{ContextNone, ContextWarn, ContextHandoff, ContextHard}

// validate checks every member's form and that the level is the highest
// recorded crossing, each level crossed at most once.
func (r *ContextRecord) validate(worker string) error {
	stamp := func(s string) bool { _, err := time.Parse(time.RFC3339, s); return err == nil }
	limit := func(s string) bool { _, ok := decimal(s); return ok || s == ContextNone }
	_, known := r.Estimate()
	switch {
	case r.Profile != ContextProfile:
		return fmt.Errorf("context record profile %q is not %s", r.Profile, ContextProfile)
	case r.Worker != worker:
		return fmt.Errorf("context record names worker %q", r.Worker)
	case !known && r.Tokens != UsageUnknown, !stamp(r.ObservedAt) && r.ObservedAt != UsageUnknown:
		return errors.New("malformed context estimate")
	case !limit(r.WarnTokens) || !limit(r.HandoffTokens) || !limit(r.HardTokens):
		return errors.New("malformed context threshold")
	case len(r.Crossings) > 3:
		return errors.New("too many context crossings")
	}
	high := 0
	for i, c := range r.Crossings {
		lv := slices.Index(contextLevels, c.Level)
		_, t1 := decimal(c.Threshold)
		_, t2 := decimal(c.Tokens)
		if lv < 1 || !t1 || !t2 || !stamp(c.At) || r.crossed(c.Level) != &r.Crossings[i] {
			return errors.New("malformed context crossing")
		}
		high = max(high, lv)
	}
	if r.Level != contextLevels[high] {
		return errors.New("context level is not the highest crossing")
	}
	return nil
}

// ContextRecordPath is worker's ATR-V0-033 record under the state directory.
func ContextRecordPath(dir, worker string) string {
	return filepath.Join(dir, "workers", worker, contextFile)
}

// ReadContextRecord reads worker's record, refusing one that is not exactly
// its canonical encoding. A missing record is fs.ErrNotExist.
func ReadContextRecord(dir, worker string) (*ContextRecord, error) {
	raw, err := readBounded(ContextRecordPath(dir, worker), maxContextRecord)
	if err != nil {
		return nil, err
	}
	// A record another build wrote is UNSUPPORTED_VERSION (CAL-V0-131).
	if err := wire.RawProfileVersion("/profile", raw, ContextProfile); err != nil {
		return nil, err
	}
	var r ContextRecord
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	if err := r.validate(worker); err != nil {
		return nil, err
	}
	if !bytes.Equal(raw, r.encode()) {
		return nil, errors.New("context record is not in canonical form")
	}
	return &r, nil
}

// contextState is the dispatcher's view of one worker's record.
type contextState struct {
	rec ContextRecord
	// dirty forces a write the last tick could not make; failing is set
	// from a failed write to the next successful one, so a run of failed
	// writes alerts once.
	dirty   bool
	failing bool
	// reloaded marks a record read back after a restart: its estimate may
	// predate lines the ledger's usage offset already passed (a failed
	// write), so it crosses nothing and stops nothing until a line read by
	// this run replaces it (ATR-V0-038).
	reloaded bool
	// foreign marks a record another build wrote (UNSUPPORTED_VERSION):
	// this run never writes over it (ATR-V0-054).
	foreign bool
	// noticeFailing marks a failing ATR-V0-125 notice file append, so its
	// alert is raised once.
	noticeFailing bool
}

// contextFor returns worker w's state, loading its record after a restart.
// A damaged record alerts once and is replaced (ATR-V0-033, ATR-V0-055); a
// record of another profile version alerts once and is preserved
// (ATR-V0-054).
func (d *Dispatcher) contextFor(w *Worker) *contextState {
	if st := d.contexts[w.ID]; st != nil {
		return st
	}
	if d.contexts == nil {
		d.contexts = map[string]*contextState{}
	}
	st := &contextState{rec: freshContextRecord(w.ID)}
	switch rec, err := ReadContextRecord(d.dir, w.ID); {
	case err == nil:
		st.rec, st.reloaded = *rec, true
	case errors.Is(err, fs.ErrNotExist):
		st.dirty = true // first written on the first supervising tick
	case wire.CodeOf(err) == wire.CodeUnsupportedVersion:
		st.foreign = true
		d.emit(Event{Kind: "alert", Ticket: w.Ticket, Worker: w.ID, Message: fmt.Sprintf("the context record of %s was written by another build; it is kept unchanged and this dispatcher records no crossing for the worker: %v", w.ID, err)})
	default:
		st.dirty = true
		d.emit(Event{Kind: "alert", Ticket: w.Ticket, Worker: w.ID, Message: fmt.Sprintf("the context record of %s is unreadable; starting a fresh one: %v", w.ID, err)})
	}
	d.contexts[w.ID] = st
	return st
}

// noteContext applies worker w's latest context observation and the applied
// role's thresholds (ATR-V0-031, ATR-V0-033, ATR-V0-036), and reports
// whether its known estimate reached hardTokens (ATR-V0-035).
func (d *Dispatcher) noteContext(w *Worker, now time.Time) bool {
	u := w.Usage
	if u == nil || !contextFormat(u.Format) {
		return false
	}
	st := d.contextFor(w)
	next := st.rec
	next.Crossings = slices.Clone(st.rec.Crossings)
	if u.ctxSeen {
		next.Tokens, next.ObservedAt = UsageUnknown, now.UTC().Format(time.RFC3339)
		if u.ctxKnown {
			next.Tokens = strconv.FormatUint(u.ctxTokens, 10)
		}
		u.ctxSeen, st.reloaded = false, false
	}
	var lim *ContextLimits
	if r := d.Config.roleNamed(w.Role); r != nil {
		lim = r.Context
	}
	text := func(n uint64) string {
		if n == 0 {
			return ContextNone
		}
		return strconv.FormatUint(n, 10)
	}
	next.WarnTokens, next.HandoffTokens, next.HardTokens = ContextNone, ContextNone, ContextNone
	if lim != nil {
		next.WarnTokens, next.HandoffTokens, next.HardTokens = text(lim.WarnTokens), text(lim.HandoffTokens), text(lim.HardTokens)
	}
	tokens, known := next.Estimate()
	known = known && !st.reloaded
	var crossed []ContextCrossing
	for _, l := range lim.levels() {
		if known && tokens >= l.tokens && next.crossed(l.level) == nil {
			c := ContextCrossing{Level: l.level, Threshold: text(l.tokens), Tokens: next.Tokens, At: now.UTC().Format(time.RFC3339)}
			next.Crossings, crossed = append(next.Crossings, c), append(crossed, c)
		}
	}
	for _, c := range next.Crossings {
		if slices.Index(contextLevels, c.Level) > slices.Index(contextLevels, next.Level) {
			next.Level = c.Level
		}
	}
	if st.foreign {
		// Nothing is written or reported; the hard stop still follows the
		// estimate (ATR-V0-054).
		next.Crossings, next.Level = st.rec.Crossings, st.rec.Level
		st.rec = next
	} else if st.dirty || !bytes.Equal(next.encode(), st.rec.encode()) {
		if err := writeAtomic(ContextRecordPath(d.dir, w.ID), next.encode()); err != nil {
			// The estimate is kept; the crossings wait for a written record.
			next.Crossings, next.Level = st.rec.Crossings, st.rec.Level
			st.rec, st.dirty = next, true
			if !st.failing {
				st.failing = true
				d.emit(Event{Kind: "alert", Ticket: w.Ticket, Worker: w.ID, Message: fmt.Sprintf("could not write the context record of %s; retrying next tick: %v", w.ID, err)})
			}
		} else {
			st.rec, st.dirty, st.failing = next, false, false
			for _, c := range crossed {
				d.emit(Event{Kind: "context", Ticket: w.Ticket, Role: w.Role, Worker: w.ID, Message: fmt.Sprintf("%s worker %s context estimate %s tokens reached its %s threshold of %s tokens", w.Role, w.ID, c.Tokens, c.Level, c.Threshold), Detail: map[string]string{"level": c.Level, "threshold": c.Threshold, "tokens": c.Tokens}})
			}
		}
	}
	if !st.foreign && !st.dirty {
		d.deliverNotices(w, st)
	}
	return known && lim != nil && lim.HardTokens > 0 && tokens >= lim.HardTokens
}

// deliverNotices delivers worker w's crossings through the ATR-V0-125
// notice file when its host declares one.
func (d *Dispatcher) deliverNotices(w *Worker, st *contextState) {
	if h, ok := d.Config.Hosts[w.Host]; !ok || !hostUsesNoticeFile(h) {
		return
	}
	levels, err := deliverNoticeFile(d.dir, &st.rec)
	if err != nil {
		if !st.noticeFailing {
			st.noticeFailing = true
			d.emit(Event{Kind: "alert", Ticket: w.Ticket, Worker: w.ID, Message: fmt.Sprintf("could not append the context notice of %s to its notice file; retrying next tick: %v", w.ID, err)})
		}
		return
	}
	st.noticeFailing = false
	for _, l := range levels {
		d.emit(Event{Kind: "context", Ticket: w.Ticket, Role: w.Role, Worker: w.ID, Message: fmt.Sprintf("delivered the %s context notice of %s worker %s to its notice file", l, w.Role, w.ID), Detail: map[string]string{"level": l, "delivery": DeliveryFile, "file": ContextNoticePath(d.dir, w.ID)}})
	}
}

// contextDetailKeys are the ATR-V0-035 members a context hand-off carries.
var contextDetailKeys = []string{"reason", "contextTokens", "hardTokens"}

// contextOnly is the ATR-V0-035 part of a hand-off detail, nil without one.
func contextOnly(detail map[string]string) map[string]string {
	if detail["reason"] != "CONTEXT" {
		return nil
	}
	out := map[string]string{}
	for _, k := range contextDetailKeys {
		out[k] = detail[k]
	}
	return out
}

// contextHandoffDetail is the ATR-V0-035 hand-off detail of a worker
// stopped for context, from its record; nil for any other worker.
func (d *Dispatcher) contextHandoffDetail(w *Worker) map[string]string {
	if w.KillReason != "CONTEXT" {
		return nil
	}
	detail := map[string]string{"reason": "CONTEXT", "contextTokens": UsageUnknown, "hardTokens": UsageUnknown}
	var rec *ContextRecord
	if st := d.contexts[w.ID]; st != nil {
		rec = &st.rec
	} else if r, err := ReadContextRecord(d.dir, w.ID); err == nil {
		rec = r
	}
	if rec != nil {
		detail["contextTokens"] = rec.Tokens
		if c := rec.crossed(ContextHard); c != nil {
			detail["hardTokens"] = c.Threshold
		} else if rec.HardTokens != ContextNone {
			detail["hardTokens"] = rec.HardTokens
		}
	}
	return detail
}
