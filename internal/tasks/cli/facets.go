package cli

import (
	"sort"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// facetMode is the parsed `--facets` / `--count` request of a filtering or
// paging ticket read (CAL-V0-206, CAL-V0-207). facets adds the facet-count
// summary next to the page; count returns the summary alone, with no items
// and no page.
type facetMode struct {
	facets bool
	count  bool
}

// maxOpenFacetKeys bounds each open-ended facet (milestones, labels) so the
// summary stays well inside MaxCommandResultBytes for any store
// (CAL-V0-206): the most frequent keys are kept, ties in byte order, and the
// number of omitted keys is reported instead of guessed.
const maxOpenFacetKeys = 64

// facetFlags removes `--facets` and `--count` from args. Tokens in the value
// position of one of values are left alone, so a filter value spelled
// `--count` is still a value. A repeated flag refuses MALFORMED; `--count`
// implies the summary, so `--count --facets` is the same request as
// `--count`.
func facetFlags(args []string, values []string) (facetMode, []string, error) {
	var m facetMode
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case containsArg(values, a) && i+1 < len(args):
			rest = append(rest, a, args[i+1])
			i++
		case a == "--facets":
			if m.facets {
				return m, nil, wire.Errorf(wire.CodeMalformed, "--facets", "duplicate --facets")
			}
			m.facets = true
		case a == "--count":
			if m.count {
				return m, nil, wire.Errorf(wire.CodeMalformed, "--count", "duplicate --count")
			}
			m.count = true
		default:
			rest = append(rest, a)
		}
	}
	return m, rest, nil
}

// checkCount refuses `--count` with a paging flag: the summary is always over
// the whole match set, so a window would be ignored silently otherwise.
func (m facetMode) checkCount(fl map[string]string) error {
	if !m.count {
		return nil
	}
	for _, name := range []string{"offset", "limit"} {
		if _, ok := fl[name]; ok {
			return wire.Errorf(wire.CodeMalformed, "--"+name, "--count returns no page; drop --%s", name)
		}
	}
	return nil
}

// apply sets the summary on a successful read. Under `--count` the items and
// the page are dropped; the untrusted label is kept when the summary counts
// any ticket, because milestone and label keys are queue data.
func (m facetMode) apply(res *wire.Result, summary *wire.Value, matched int) {
	if res == nil || res.Outcome != wire.OutcomeOK || summary == nil || (!m.facets && !m.count) {
		return
	}
	res.Facets = summary
	if m.count {
		res.Items = nil
		res.Page = nil
	}
	res.Untrusted = matched > 0 || len(res.Items) > 0
}

// facetSummary counts the matched tickets ids by status, priority, kind,
// execution class, eligibility, next action, blocker code, milestone and
// label (CAL-V0-206). It is computed over every matched id, independent of
// any page window, from the same views list items render. Closed enums list
// every value, zeros included; open sets list only non-zero keys. A blocker
// code counts each ticket once and only on a non-terminal ticket, matching
// list items, which drop blockers from COMPLETED and ARCHIVED tickets.
func facetSummary(rc *readCtx, ids []string) wire.Value {
	inv := rc.store.Inventory
	ctx := rc.store.Context()
	byStatus := zeroCounts(ticket.Statuses)
	byPriority := zeroCounts(ticket.Priorities)
	byKind := zeroCounts(ticket.Kinds)
	byClass := zeroCounts(ticket.ExecutionClasses)
	byEligibility := zeroCounts([]string{ticket.EligibilityBlocked, ticket.EligibilityUnknown})
	byNext := map[string]int64{}
	byBlocker := map[string]int64{}
	byMilestone := map[string]int64{}
	byLabel := map[string]int64{}
	without := int64(0)
	for _, id := range ids {
		v, ok := inv.View(id, ctx)
		if !ok || v.Record == nil {
			continue
		}
		rec := v.Record
		byStatus[rec.Status]++
		byPriority[rec.Priority]++
		byKind[rec.Kind]++
		byClass[rec.ExecutionClass]++
		byEligibility[v.Eligibility]++
		byNext[v.NextAction]++
		if rec.Status != ticket.StatusCompleted && rec.Status != ticket.StatusArchived {
			seen := map[string]bool{}
			for _, b := range v.Blockers {
				if !seen[b.Code] {
					seen[b.Code] = true
					byBlocker[b.Code]++
				}
			}
		}
		if rec.Milestone == nil {
			without++
		} else {
			byMilestone[*rec.Milestone]++
		}
		for _, l := range rec.Labels {
			byLabel[l]++
		}
	}
	o := wire.NewObject()
	o.Set("total", countValue(int64(len(ids))))
	o.Set("byStatus", countsValue(byStatus))
	o.Set("byPriority", countsValue(byPriority))
	o.Set("byKind", countsValue(byKind))
	o.Set("byExecutionClass", countsValue(byClass))
	o.Set("byEligibility", countsValue(byEligibility))
	o.Set("byNextAction", countsValue(byNext))
	o.Set("byBlockerCode", countsValue(byBlocker))
	milestones, milestonesOmitted := topCounts(byMilestone)
	o.Set("byMilestone", countsValue(milestones))
	o.Set("milestonesOmitted", countValue(milestonesOmitted))
	o.Set("withoutMilestone", countValue(without))
	labels, labelsOmitted := topCounts(byLabel)
	o.Set("byLabel", countsValue(labels))
	o.Set("labelsOmitted", countValue(labelsOmitted))
	return wire.ObjectValue(o)
}

func zeroCounts(keys []string) map[string]int64 {
	m := make(map[string]int64, len(keys))
	for _, k := range keys {
		m[k] = 0
	}
	return m
}

func countValue(n int64) wire.Value { return wire.String(string(wire.CountOf(n))) }

// countsValue renders a key→count map as an object of wire counts with its
// keys inserted in byte order, so even the in-memory value is deterministic.
func countsValue(m map[string]int64) wire.Value {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	o := wire.NewObject()
	for _, k := range keys {
		o.Set(k, countValue(m[k]))
	}
	return wire.ObjectValue(o)
}

// topCounts keeps the maxOpenFacetKeys most frequent keys (ties in byte
// order) and returns how many keys it dropped.
func topCounts(m map[string]int64) (map[string]int64, int64) {
	if len(m) <= maxOpenFacetKeys {
		return m, 0
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if m[keys[i]] != m[keys[j]] {
			return m[keys[i]] > m[keys[j]]
		}
		return keys[i] < keys[j]
	})
	kept := make(map[string]int64, maxOpenFacetKeys)
	for _, k := range keys[:maxOpenFacetKeys] {
		kept[k] = m[k]
	}
	return kept, int64(len(keys) - maxOpenFacetKeys)
}
