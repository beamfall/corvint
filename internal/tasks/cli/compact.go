package cli

import (
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Compact agent output (CAL-V0-165..CAL-V0-168): `--fields` projects each
// item of an OK read to named keys and `--summary` returns a fixed small
// shape. Both run after the underlying read has produced its result, so they
// change nothing the read observes or writes; the envelope (command, outcome,
// codes, snapshot, untrusted, warnings, maxNodes) is kept as the read built it.

const maxCompactFields = 32

// compactSpec describes one read's summary. summary maps a full item to its
// documented shape; extra is appended to the read's args under --summary for
// a field that is opt-in on the full output (unless already given); a flag in
// exclusive cannot be combined with --summary. values names the read's own
// flags that take the next argument as their value; that value is passed
// through unread, so `--text --summary` still searches for "--summary".
type compactSpec struct {
	summary   func(item wire.Value) wire.Value
	extra     string
	exclusive string
	values    []string
}

// compactRead strips `--fields` and `--summary` from args, runs read on the
// remaining args and applies the requested projection to an OK result.
func compactRead(cmd []string, args []string, spec compactSpec, read func([]string) *wire.Result) *wire.Result {
	var fields []string
	haveFields, wantSummary := false, false
	rest := make([]string, 0, len(args))
	flagged := map[string]bool{} // arguments in a flag position, not values
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case containsArg(spec.values, a) && i+1 < len(args):
			rest = append(rest, a, args[i+1])
			i++
		case a == "--summary":
			if wantSummary {
				return compactRefusal(cmd, "duplicate --summary")
			}
			wantSummary = true
		case a == "--fields" || strings.HasPrefix(a, "--fields="):
			if haveFields {
				return compactRefusal(cmd, "duplicate --fields")
			}
			haveFields = true
			value, inline := strings.CutPrefix(a, "--fields=")
			if !inline {
				if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
					return compactRefusal(cmd, "--fields needs a comma-separated list of item keys")
				}
				i++
				value = args[i]
			}
			parsed, msg := parseCompactFields(value)
			if msg != "" {
				return compactRefusal(cmd, msg)
			}
			fields = parsed
		default:
			flagged[a] = true
			rest = append(rest, a)
		}
	}
	if haveFields && wantSummary {
		return compactRefusal(cmd, "--fields and --summary are exclusive")
	}
	if wantSummary && spec.exclusive != "" && flagged[spec.exclusive] {
		return compactRefusal(cmd, "--summary cannot be combined with "+spec.exclusive)
	}
	if wantSummary && spec.extra != "" && !flagged[spec.extra] {
		rest = append(rest, spec.extra)
	}
	res := read(rest)
	if res == nil || res.Outcome != wire.OutcomeOK || (!haveFields && !wantSummary) {
		return res
	}
	if wantSummary {
		for i, item := range res.Items {
			res.Items[i] = spec.summary(item)
		}
		return res
	}
	if msg := checkCompactFields(res.Items, fields); msg != "" {
		refused := compactRefusal(cmd, msg)
		refused.Snapshot = res.Snapshot
		return refused
	}
	for i, item := range res.Items {
		res.Items[i] = projectFields(item, fields)
	}
	return res
}

func containsArg(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func compactRefusal(cmd []string, msg string) *wire.Result {
	return failure(cmd, nil, wire.Errorf(wire.CodeMalformed, "fields", "%s", msg))
}

// parseCompactFields accepts 1..32 distinct names, each `key` or `key.sub`
// with ASCII alphanumeric segments starting with a letter.
func parseCompactFields(value string) ([]string, string) {
	names := strings.Split(value, ",")
	if value == "" || len(names) > maxCompactFields {
		return nil, "--fields needs 1 to 32 comma-separated item keys"
	}
	seen := map[string]bool{}
	for _, n := range names {
		segments := strings.Split(n, ".")
		if len(segments) > 2 {
			return nil, "field " + quoteField(n) + " nests deeper than one level"
		}
		for _, s := range segments {
			if !fieldSegment(s) {
				return nil, "field " + quoteField(n) + " is not a key or key.sub name"
			}
		}
		if seen[n] {
			return nil, "duplicate field " + quoteField(n)
		}
		seen[n] = true
	}
	return names, ""
}

func fieldSegment(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for i, r := range s {
		letter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		if !letter && (i == 0 || r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func quoteField(n string) string {
	if len(n) > 80 {
		n = n[:80]
	}
	return "\"" + n + "\""
}

// checkCompactFields refuses a name no item carries; an item without it
// omits it. A `key.sub` over null or an empty array is kept as observed; over
// a scalar, or when no element of any non-empty container carries sub, it is
// refused. A page with no items has nothing to check against.
func checkCompactFields(items []wire.Value, fields []string) string {
	for _, item := range items {
		if item.Kind != wire.KindObject {
			return "this read's items are not objects"
		}
	}
	if len(items) == 0 {
		return ""
	}
	for _, f := range fields {
		key, sub, nested := strings.Cut(f, ".")
		present, found, containers := false, false, 0
		for _, item := range items {
			v, ok := item.Obj.Get(key)
			if !ok {
				continue
			}
			present = true
			if !nested {
				continue
			}
			switch v.Kind {
			case wire.KindNull:
			case wire.KindObject:
				containers++
				if _, ok := v.Obj.Get(sub); ok {
					found = true
				}
			case wire.KindArray:
				for _, el := range v.Arr {
					containers++
					if el.Kind == wire.KindObject {
						if _, ok := el.Obj.Get(sub); ok {
							found = true
						}
					}
				}
			default:
				return "field " + quoteField(key) + " is a scalar and has no sub-key"
			}
		}
		if !present {
			if key == "retries" {
				return "field \"retries\" is not in this item; queue status includes it only with --retries"
			}
			return "field " + quoteField(key) + " is not in this read's items"
		}
		if nested && containers > 0 && !found {
			return "field " + quoteField(f) + " is not in any element of " + quoteField(key)
		}
	}
	return ""
}

// projectFields keeps the named keys in the requested order. Sub-keys of one
// container are merged; an element without the sub-key keeps no value for it
// rather than an invented null.
func projectFields(item wire.Value, fields []string) wire.Value {
	whole := map[string]bool{}
	subs := map[string][]string{}
	var order []string
	for _, f := range fields {
		key, sub, nested := strings.Cut(f, ".")
		if !whole[key] && subs[key] == nil {
			order = append(order, key)
		}
		if nested {
			subs[key] = append(subs[key], sub)
		} else {
			whole[key] = true
		}
	}
	out := wire.NewObject()
	for _, key := range order {
		v, ok := item.Obj.Get(key)
		if !ok {
			continue
		}
		if whole[key] {
			out.Set(key, v)
			continue
		}
		out.Set(key, pickNested(v, subs[key]))
	}
	return wire.ObjectValue(out)
}

func pickNested(v wire.Value, keys []string) wire.Value {
	switch v.Kind {
	case wire.KindObject:
		return pick(v, keys...)
	case wire.KindArray:
		out := make([]wire.Value, len(v.Arr))
		for i, el := range v.Arr {
			out[i] = el
			if el.Kind == wire.KindObject {
				out[i] = pick(el, keys...)
			}
		}
		return wire.Array(out...)
	}
	return v
}

// pick copies the present keys of an object; absent keys stay absent.
func pick(v wire.Value, keys ...string) wire.Value {
	out := wire.NewObject()
	if v.Kind != wire.KindObject {
		return wire.ObjectValue(out)
	}
	for _, k := range keys {
		if x, ok := v.Obj.Get(k); ok {
			out.Set(k, x)
		}
	}
	return wire.ObjectValue(out)
}

func pickArray(v wire.Value, keys ...string) wire.Value {
	if v.Kind != wire.KindArray {
		return v
	}
	return pickNested(v, keys)
}

// queueStatusSummary is the CAL-V0-167 queue status summary.
func queueStatusSummary(item wire.Value) wire.Value {
	out := pick(item, "queueId", "tickets", "byStatus", "blocked", "headSeq", "writeBarrier", "barrier", "attempts")
	if live, ok := item.Obj.Get("liveAttempts"); ok {
		out.Obj.Set("liveAttempts", pickArray(live, "attemptId", "ticketId", "phase", "holder", "expiresAt", "holderStatus"))
	}
	if pools, ok := item.Obj.Get("pools"); ok && pools.Kind == wire.KindArray {
		lanes := []wire.Value{}
		for _, p := range pools.Arr {
			id, _ := p.Obj.Get("poolId")
			members, _ := p.Obj.Get("members")
			for _, m := range members.Arr {
				lane := pick(m, "memberId", "state", "attemptId")
				lane.Obj.Set("poolId", id)
				lanes = append(lanes, lane)
			}
		}
		out.Obj.Set("lanes", wire.Array(lanes...))
	}
	out.Obj.Set("retriesExhausted", exhaustedCount(item))
	return out
}

// exhaustedCount counts listed tickets whose retry admission is exhausted;
// any unobserved exhaustion makes the count NOT_OBSERVED.
func exhaustedCount(item wire.Value) wire.Value {
	retries, ok := item.Obj.Get("retries")
	if !ok || retries.Kind != wire.KindArray {
		return wire.String("NOT_OBSERVED")
	}
	n := int64(0)
	for _, r := range retries.Arr {
		obs, _ := r.Obj.Get("retries")
		if obs.Kind != wire.KindObject {
			return wire.String("NOT_OBSERVED")
		}
		ex, _ := obs.Obj.Get("exhausted")
		if ex.Kind != wire.KindBool {
			return wire.String("NOT_OBSERVED")
		}
		if ex.Bool {
			n++
		}
	}
	return wire.String(string(wire.CountOf(n)))
}

// attemptSummary is the CAL-V0-167 attempt show summary.
func attemptSummary(item wire.Value) wire.Value {
	out := pick(item, "attemptId", "ticketId", "generation", "phase")
	lease, _ := item.Obj.Get("lease")
	holder, expires := wire.Null(), wire.Null()
	if lease.Kind == wire.KindObject {
		if v, ok := lease.Obj.Get("holder"); ok {
			holder = v
		}
		if v, ok := lease.Obj.Get("expiresAt"); ok {
			expires = v
		}
	}
	out.Obj.Set("holder", holder)
	out.Obj.Set("expiresAt", expires)
	for _, k := range []string{"holderStatus", "lastHeartbeatAt", "retryCount"} {
		if v, ok := item.Obj.Get(k); ok {
			out.Obj.Set(k, v)
		}
	}
	return out
}

// ticketSummary is the CAL-V0-167 ticket show summary.
func ticketSummary(item wire.Value) wire.Value {
	out := pick(item, "ticketId", "revision", "status", "title", "priority", "eligibility", "claimable", "claimabilityReason", "nextAction", "nextStage")
	if r, ok := item.Obj.Get("retries"); ok {
		if r.Kind == wire.KindObject {
			r = pick(r, "remaining", "exhausted")
		}
		out.Obj.Set("retries", r)
	}
	return out
}

// planSummary is the CAL-V0-167 plan preview summary.
func planSummary(item wire.Value) wire.Value {
	out := pick(item, "planningProfile", "queueId", "headSeq", "mutationAuthority")
	out.Obj.Set("profile", wire.String("taskman-plan-summary/0"))
	if entries, ok := item.Obj.Get("entries"); ok {
		out.Obj.Set("entries", pickArray(entries, "ticketId", "state", "reason", "nextStage", "nextAction"))
	}
	return out
}

// listSummary is the CAL-V0-174 ticket list and ticket search item summary.
func listSummary(item wire.Value) wire.Value {
	return pick(item, "ticketId", "status", "priority", "title", "eligibility", "nextAction")
}

// roadmapSummary is the CAL-V0-174 roadmap item summary.
func roadmapSummary(item wire.Value) wire.Value {
	return pick(item, "ticketId", "milestone", "status", "priority", "title", "nextAction")
}
