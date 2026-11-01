// SPDX-License-Identifier: Apache-2.0

package ticket

import (
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Progress phases (TLF-V0-880).
const (
	PhaseInProgress             = "IN_PROGRESS"
	PhaseDeliveredOnBranch      = "DELIVERED_ON_BRANCH"
	PhaseInReview               = "IN_REVIEW"
	PhaseInMergeQueue           = "IN_MERGE_QUEUE"
	PhaseMergedAwaitingCloseout = "MERGED_AWAITING_CLOSEOUT"
	PhaseBlocked                = "BLOCKED"
	PhaseCleared                = "CLEARED"
)

// ProgressMark is the writer-supplied part of one progress entry: the
// PROGRESS_SET payload (TLF-V0-880).
type ProgressMark struct {
	Phase  string
	Lane   *string
	Host   *string
	Branch *string
	Head   *string
	PR     *wire.Count
	Reason *string
}

// Progress is one append-only TLF-V0-880 entry. Actor and RecordedAt come
// from the writer's trusted binding and clock. Progress is advisory: no
// admission, eligibility, planning, gate or completion read consults it
// (TLF-V0-881).
type Progress struct {
	ProgressMark
	Actor      string
	RecordedAt wire.Timestamp
}

// ReadProgressMark decodes the closed mark members of r, which the caller
// has already closed over its key set.
func ReadProgressMark(r *wire.Reader) ProgressMark {
	m := ProgressMark{
		Phase:  r.Field("phase").Enum(wire.ProgressPhases...),
		Lane:   r.Field("lane").LabelOrNull(),
		Host:   r.Field("host").LabelOrNull(),
		Branch: r.Field("branch").StringOrNull((*wire.Reader).Identifier),
		Head:   r.Field("head").StringOrNull((*wire.Reader).OID),
		PR:     r.Field("pr").CountOrNull(),
		Reason: r.Field("reason").StringOrNull(readProgressReason),
	}
	if r.Err() == nil {
		if err := m.Validate(); err != nil {
			r.Fail(wire.CodeMalformed, "%s", err.Error())
		}
	}
	return m
}

func readProgressReason(r *wire.Reader) string {
	s := r.Prose(1, wire.ProgressMaxReasonBytes)
	if r.Err() == nil && strings.TrimSpace(s) == "" {
		r.Fail(wire.CodeMalformed, "reason must contain non-whitespace prose")
		return ""
	}
	return s
}

// ProgressMarkKeys is the closed PROGRESS_SET payload key set.
var ProgressMarkKeys = []string{"branch", "head", "host", "lane", "phase", "pr", "reason"}

// Validate enforces the per-phase required members of TLF-V0-880.
func (m ProgressMark) Validate() error {
	need := func(ok bool, what string) error {
		if ok {
			return nil
		}
		return wire.Errorf(wire.CodeMalformed, "/phase", "phase %s requires %s", m.Phase, what)
	}
	switch m.Phase {
	case PhaseDeliveredOnBranch:
		return need(m.Branch != nil && m.Head != nil, "branch and head")
	case PhaseInReview, PhaseInMergeQueue:
		return need(m.PR != nil, "pr")
	case PhaseMergedAwaitingCloseout:
		return need(m.Head != nil, "head (the merge commit)")
	case PhaseBlocked:
		return need(m.Reason != nil, "reason")
	case PhaseCleared:
		return need(m.Lane == nil && m.Host == nil && m.Branch == nil && m.Head == nil && m.PR == nil, "lane, host, branch, head and pr to be null")
	}
	return nil
}

// Equal reports whether two marks carry the same members.
func (m ProgressMark) Equal(o ProgressMark) bool {
	eq := func(a, b *string) bool { return (a == nil) == (b == nil) && (a == nil || *a == *b) }
	return m.Phase == o.Phase && eq(m.Lane, o.Lane) && eq(m.Host, o.Host) && eq(m.Branch, o.Branch) &&
		eq(m.Head, o.Head) && eq(m.Reason, o.Reason) && (m.PR == nil) == (o.PR == nil) && (m.PR == nil || *m.PR == *o.PR)
}

// ReadProgress decodes the optional progress member: a non-empty array in
// append order (absence is the empty list) of closed entries.
func ReadProgress(r *wire.Reader) []Progress {
	entries := r.Array(wire.ProgressMaxEntries, true)
	if r.Err() == nil && len(entries) == 0 {
		r.Fail(wire.CodeMalformed, "progress is omitted when empty")
		return nil
	}
	out := make([]Progress, 0, len(entries))
	for _, e := range entries {
		e.Closed(append([]string{"actor", "recordedAt"}, ProgressMarkKeys...)...)
		p := Progress{ProgressMark: ReadProgressMark(e), Actor: e.Field("actor").Label(), RecordedAt: e.Field("recordedAt").Timestamp()}
		out = append(out, p)
	}
	return out
}

// ProgressMarkValue sets m's members on o.
func ProgressMarkValue(o *wire.Object, m ProgressMark) {
	o.Set("phase", wire.String(m.Phase))
	o.Set("lane", wire.StringOrNull(m.Lane))
	o.Set("host", wire.StringOrNull(m.Host))
	o.Set("branch", wire.StringOrNull(m.Branch))
	o.Set("head", wire.StringOrNull(m.Head))
	if m.PR == nil {
		o.Set("pr", wire.Null())
	} else {
		o.Set("pr", wire.String(string(*m.PR)))
	}
	o.Set("reason", wire.StringOrNull(m.Reason))
}

// ProgressEntryValue encodes one entry.
func ProgressEntryValue(p Progress) wire.Value {
	o := wire.NewObject()
	ProgressMarkValue(o, p.ProgressMark)
	o.Set("actor", wire.String(p.Actor))
	o.Set("recordedAt", wire.String(string(p.RecordedAt)))
	return wire.ObjectValue(o)
}

// ProgressValue encodes the entries in append order.
func ProgressValue(entries []Progress) wire.Value {
	vs := make([]wire.Value, 0, len(entries))
	for _, p := range entries {
		vs = append(vs, ProgressEntryValue(p))
	}
	return wire.Array(vs...)
}

// Live reports a DRAFT, OPEN or HELD status, the ones that admit progress.
func Live(status string) bool {
	return status == StatusDraft || status == StatusOpen || status == StatusHeld
}

// EffectiveProgress is rec's latest entry while rec is live and that entry
// is not CLEARED, else nil (TLF-V0-881): completion and archive end it
// without a write.
func (rec *Record) EffectiveProgress() *Progress {
	if !Live(rec.Status) || len(rec.Progress) == 0 {
		return nil
	}
	p := rec.Progress[len(rec.Progress)-1]
	if p.Phase == PhaseCleared {
		return nil
	}
	return &p
}

// EffectivePhase is the phase of rec's effective progress, or "NONE".
func (rec *Record) EffectivePhase() string {
	if p := rec.EffectiveProgress(); p != nil {
		return p.Phase
	}
	return "NONE"
}
