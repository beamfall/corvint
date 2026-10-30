package transaction

import (
	"bytes"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/obligation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ProfilePoolRecoveryDetail is the bounded, secret-redacted detail document
// a `pool observe` failure records under evidence/ (PSR-V0-260).
const ProfilePoolRecoveryDetail = "taskman-pool-recovery-detail/0"

// MaxPoolReportInputBytes bounds the detail file `pool observe` reads; a
// larger file is refused LIMIT_EXCEEDED rather than silently cut
// (PSR-V0-260).
const MaxPoolReportInputBytes = 1 << 20

// MaxPoolRecoveryDetailBytes bounds the detail text kept: the tail of the
// redacted input, cut at a rune boundary (PSR-V0-260).
const MaxPoolRecoveryDetailBytes = 2048

// PoolRecoveryRedacted replaces each line the shared secret screen touches.
const PoolRecoveryRedacted = "[REDACTED]"

// PoolRecoveryDetail is one decoded detail document.
type PoolRecoveryDetail struct {
	Kind, Result, Detail string
	Bytes                wire.Size
	Truncated            bool
	RedactedLines        wire.Count
}

// NewPoolRecoveryDetail builds the canonical detail document for one failed
// readiness observation of kind ("health" or "cleanup") from raw, the
// operator's or probe's output (PSR-V0-260). Invalid UTF-8 and hostile code
// points become '?'; every line the shared secret screen touches, a match
// spanning lines included, becomes PoolRecoveryRedacted; then only the last
// MaxPoolRecoveryDetailBytes bytes are kept. The same raw input always
// yields the same bytes, so identical failures share one digest.
func NewPoolRecoveryDetail(kind, result string, raw []byte) ([]byte, error) {
	if kind != "health" && kind != "cleanup" {
		return nil, malformed("pool observe --kind is health or cleanup")
	}
	if result != "FAILED" {
		return nil, malformed("pool observe --result records FAILED only")
	}
	if len(raw) > MaxPoolReportInputBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "detail", "detail file longer than %d bytes (%d)", MaxPoolReportInputBytes, len(raw))
	}
	text := proseRunes(strings.ToValidUTF8(string(raw), "?"))
	text, n := redactLines(text)
	truncated := false
	if len(text) > MaxPoolRecoveryDetailBytes {
		cut := len(text) - MaxPoolRecoveryDetailBytes
		for cut < len(text) && !utf8.RuneStart(text[cut]) {
			cut++
		}
		text, truncated = text[cut:], true
		// A cut can only shorten a match, but screen the kept tail again.
		var m int
		text, m = redactLines(text)
		n += m
	}
	d := PoolRecoveryDetail{Kind: kind, Result: result, Detail: text, Bytes: wire.SizeOf(uint64(len(raw))), Truncated: truncated, RedactedLines: wire.CountOf(int64(n))}
	return d.encode(), nil
}

func (d PoolRecoveryDetail) encode() []byte {
	return wire.EncodeFile(object("profile", s(ProfilePoolRecoveryDetail), "kind", s(d.Kind), "result", s(d.Result), "detail", s(d.Detail), "bytes", s(string(d.Bytes)), "truncated", wire.Bool(d.Truncated), "redactedLines", s(string(d.RedactedLines))))
}

// DecodePoolRecoveryDetail reads a canonical detail document and refuses
// one whose detail still matches the secret screen (SECRET_DETECTED).
func DecodePoolRecoveryDetail(raw []byte) (*PoolRecoveryDetail, error) {
	if len(raw) > 2*MaxPoolRecoveryDetailBytes+1024 {
		return nil, limit("pool recovery detail document")
	}
	v, e := wire.Parse(raw)
	if e != nil {
		return nil, e
	}
	r := wire.NewReader(v, "detail")
	if e := r.Profile(ProfilePoolRecoveryDetail); e != nil {
		return nil, e
	}
	r.Closed("profile", "kind", "result", "detail", "bytes", "truncated", "redactedLines")
	d := &PoolRecoveryDetail{Kind: r.Field("kind").Enum("health", "cleanup"), Result: r.Field("result").Enum("FAILED"), Detail: r.Field("detail").Prose(0, MaxPoolRecoveryDetailBytes), Bytes: r.Field("bytes").Size(), Truncated: r.Field("truncated").Bool(), RedactedLines: r.Field("redactedLines").Count()}
	if e := r.Err(); e != nil {
		return nil, e
	}
	if !bytes.Equal(raw, d.encode()) {
		return nil, malformed("pool recovery detail is not canonical")
	}
	if obligation.MatchesSecretScreen(d.Detail) {
		return nil, wire.Errorf(wire.CodeSecretDetected, "detail", "pool recovery detail matches the secret screen")
	}
	return d, nil
}

// proseRunes replaces each code point prose refuses with '?'.
func proseRunes(text string) string {
	if _, e := wire.ParseProse("", text, 0, len(text)); e == nil {
		return text
	}
	var b strings.Builder
	for _, r := range text {
		if _, e := wire.ParseProse("", string(r), 0, 4); e != nil {
			r = '?'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// redactLines replaces every line a secret-screen match touches.
func redactLines(text string) (string, int) {
	spans := obligation.SecretScreenSpans(text)
	if len(spans) == 0 {
		return text, 0
	}
	lines := strings.SplitAfter(text, "\n")
	starts := make([]int, len(lines))
	for i, at := 1, len(lines[0]); i < len(lines); i, at = i+1, at+len(lines[i]) {
		starts[i] = at
	}
	// Each span marks the run of lines it overlaps in a difference array,
	// found by binary search, so the cost stays O((lines+spans) log lines).
	diff := make([]int, len(lines)+1)
	for _, sp := range spans {
		if sp[1] <= sp[0] {
			continue
		}
		first, _ := slices.BinarySearch(starts, sp[0]+1)
		last, _ := slices.BinarySearch(starts, sp[1])
		diff[first-1]++
		diff[last]--
	}
	hit := make([]bool, len(lines))
	for i, depth := 0, 0; i < len(lines); i++ {
		depth += diff[i]
		hit[i] = depth > 0
	}
	n := 0
	for i, l := range lines {
		if hit[i] {
			nl := ""
			if strings.HasSuffix(l, "\n") {
				nl = "\n"
			}
			lines[i], n = PoolRecoveryRedacted+nl, n+1
		}
	}
	return strings.Join(lines, ""), n
}

// planPoolReport records one failed readiness observation of a QUARANTINED
// member (PSR-V0-260, PSR-V0-262): the detail document joins evidence/ and
// the entry's recovery record counts consecutive identical failures, the
// same kind and detail digest; any other failure restarts the count.
func planPoolReport(c leaseContext) leaseOutcome {
	var en *snapshot.PoolEntry
	for _, x := range c.st.pools.Entries {
		if x.MemberID == c.l.Member {
			y := x
			en = &y
		}
	}
	if en == nil || (c.l.Allocation != "" && string(en.AllocationID) != c.l.Allocation) {
		return c.refuse(mutation.OutcomeRevisionConflict, wire.CodeFenced, "allocation differs")
	}
	if en.Sweep != nil {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeResourceCollision, "sweep owns allocation")
	}
	if en.State != "QUARANTINED" {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeQuiescenceUnproved, "a failed readiness observation records only a QUARANTINED member; "+en.MemberID+" is "+en.State)
	}
	raw := c.in.LeaseFacts.Pool.Detail
	if string(wire.Sum(raw)) != c.l.Evidence {
		return c.fail(malformed("pool observe detail differs from its evidence digest"))
	}
	d, e := DecodePoolRecoveryDetail(raw)
	if e != nil {
		return c.fail(e)
	}
	next := snapshot.PoolRecovery{Kind: d.Kind, Result: d.Result, DetailSha256: wire.Digest(c.l.Evidence), Consecutive: "1", FirstObservedAt: c.in.RecordedAt, ObservedAt: c.in.RecordedAt, ObservedSeq: c.seq}
	if p := en.Recovery; p != nil && p.Kind == next.Kind && p.DetailSha256 == next.DetailSha256 {
		next.Consecutive = wire.CountOf(min(p.Consecutive.Int()+1, math.MaxInt32))
		next.FirstObservedAt = p.FirstObservedAt
	}
	en.Recovery = &next
	out := c.putPool(en, false, raw)
	if out.result == nil {
		out.detail = fmt.Sprintf("QUARANTINED: %s: %s %s x%s", en.MemberID, d.Kind, d.Result, next.Consecutive)
	}
	return out
}

// stampQuarantine keeps the PSR-V0-261 quarantine timestamp of every entry
// a planned pools.json carries, whichever verb wrote it: an entry that
// becomes QUARANTINED records quarantinedAt = recordedAt, unless it
// continues a quarantine (the same allocation, previously QUARANTINED, or
// CLEANING with a recorded quarantinedAt), which keeps the recorded value,
// absent for a legacy entry. CLEANING carries a continuing quarantine's
// value; every other state, and a CLEANING entry that continues none,
// carries neither quarantinedAt nor a recovery record. A pools.json that
// needs no change is left byte-identical.
func (c leaseContext) stampQuarantine(out leaseOutcome) leaseOutcome {
	raw, ok := out.posts["pools.json"]
	if !ok {
		return out
	}
	state, e := snapshot.DecodePools(raw)
	if e != nil {
		return out
	}
	prior := map[string]snapshot.PoolEntry{}
	if c.st.pools != nil {
		for _, en := range c.st.pools.Entries {
			prior[en.MemberID] = en
		}
	}
	changed := false
	for i := range state.Entries {
		en := &state.Entries[i]
		p, had := prior[en.MemberID]
		same := had && p.AllocationID == en.AllocationID
		at, rec := en.QuarantinedAt, en.Recovery
		switch {
		case en.State == "QUARANTINED" && same && (p.State == "QUARANTINED" || (p.State == "CLEANING" && p.QuarantinedAt != "")):
			if at == "" {
				at = p.QuarantinedAt
			}
		case en.State == "QUARANTINED":
			at, rec = c.in.RecordedAt, nil
		case en.State == "CLEANING" && same && (p.State == "QUARANTINED" || p.State == "CLEANING"):
			if at == "" {
				at = p.QuarantinedAt
			}
		default:
			at, rec = "", nil
		}
		if at != en.QuarantinedAt || rec != en.Recovery {
			en.QuarantinedAt, en.Recovery, changed = at, rec, true
		}
	}
	if !changed {
		return out
	}
	if out.posts["pools.json"], e = state.Encode(); e != nil {
		return c.fail(e)
	}
	return out
}

// stalledRefusal is the PSR-V0-264 typed refusal of an acquire or claim
// whose every eligible member is QUARANTINED with a stalled recovery, or nil.
func (c leaseContext) stalledRefusal(poolID string, eligible []string) error {
	now, e := time.Parse(timestampLayout, string(c.in.RecordedAt))
	if e != nil || !c.st.pools.MembersStalled(poolID, eligible, now) {
		return nil
	}
	named := eligible
	more := ""
	if len(named) > 8 {
		named, more = named[:8], fmt.Sprintf(" and %d more", len(eligible)-8)
	}
	return wire.Errorf(wire.CodePoolMemberRecoveryStalled, "pool", "every eligible member of pool %s is QUARANTINED with a stalled recovery (%s%s); read `corvint-tasks pool status --pool %s`, repair the members, then confirm them safe", poolID, strings.Join(named, ", "), more, poolID)
}
