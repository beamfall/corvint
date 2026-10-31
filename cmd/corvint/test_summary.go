package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/sessiontest"
)

// testSummaryLimit bounds --dry-run --summary output (STR-V0-800): a FULL
// plan's JSON runs to tens of kilobytes, which an agent reads only to learn
// the scope, the reason and roughly what runs.
const testSummaryLimit = 4096

// testSummaryItem bounds one quoted reason, unit or incomplete entry.
const testSummaryItem = 200

// testDryRunSummary renders the shard's plan as at most testSummaryLimit
// bytes of text: the scope, placement, shard and manifest digest; the
// fallback reason; the unit and invocation counts by adapter and slice; the
// first selected units; and the counts of still-required checks, no-suite
// languages and incomplete entries, naming the first incomplete entries.
// Lists stop with "(+N more)" once the bound would be passed.
func testDryRunSummary(m sessiontest.Manifest, digest string, invocations []sessiontest.Planned, shard int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "corvint-test: DRY-RUN scope=%s placement=%s shard=%d/%d manifest=%s\n", m.Scope, m.Placement, shard, m.ShardCount, digest)
	reason := "none"
	if m.Reason != "" {
		reason = summaryClip(m.Reason, testSummaryItem)
	}
	fmt.Fprintf(&b, "  reason: %s\n", reason)
	counts := map[string]map[string]int{}
	var units []string
	for _, unit := range m.Units {
		if unit.Shard != shard {
			continue
		}
		if counts[unit.Adapter] == nil {
			counts[unit.Adapter] = map[string]int{}
		}
		counts[unit.Adapter][unit.Slice]++
		name := unit.Unit
		if unit.Slice != "" && unit.Slice != "whole" {
			name += " [" + unit.Slice + "]"
		}
		units = append(units, summaryClip(name, testSummaryItem))
	}
	adapters := make([]string, 0, len(counts))
	for adapter := range counts {
		adapters = append(adapters, adapter)
	}
	sort.Strings(adapters)
	parts := []string{}
	for _, adapter := range adapters {
		slices := make([]string, 0, len(counts[adapter]))
		total := 0
		for slice, count := range counts[adapter] {
			slices = append(slices, fmt.Sprintf("%s=%d", slice, count))
			total += count
		}
		sort.Strings(slices)
		parts = append(parts, fmt.Sprintf("%s %d (%s)", adapter, total, strings.Join(slices, " ")))
	}
	if len(parts) == 0 {
		parts = append(parts, "none")
	}
	fmt.Fprintf(&b, "  units: %d; %s; invocations=%d\n", len(units), strings.Join(parts, ", "), len(invocations))
	fmt.Fprintf(&b, "  checks still required: %d; no-suite languages: %d; incomplete: %d\n", len(m.Checks), len(m.NoSuite), len(m.Incomplete))
	tail := "  full plan: rerun without --summary for the JSON argv\n"
	incomplete := make([]string, 0, len(m.Incomplete))
	for _, entry := range m.Incomplete {
		incomplete = append(incomplete, summaryClip(entry, testSummaryItem))
	}
	// Incomplete entries explain a nonzero exit, so they come before units.
	summaryList(&b, "incomplete", incomplete, testSummaryLimit/2)
	summaryList(&b, "selected", units, testSummaryLimit-len(tail))
	b.WriteString(tail)
	return b.String()
}

// summaryList appends "  label: " and as many of items as keep b within
// limit bytes, then "(+N more)" for the rest; it appends nothing for none.
func summaryList(b *strings.Builder, label string, items []string, limit int) {
	if len(items) == 0 {
		return
	}
	const more = " (+9999999 more)\n"
	b.WriteString("  " + label + ":")
	written := 0
	for _, item := range items {
		if b.Len()+1+len(item)+len(more) > limit {
			break
		}
		b.WriteString(" " + item)
		written++
	}
	if rest := len(items) - written; rest > 0 {
		fmt.Fprintf(b, " (+%d more)", rest)
	}
	b.WriteString("\n")
}

// summaryClip cuts text to at most limit bytes on a rune boundary, marking the cut.
func summaryClip(text string, limit int) string {
	text = strings.Join(strings.Fields(text), " ")
	if len(text) <= limit {
		return text
	}
	cut := limit - len("...")
	for cut > 0 && !utf8RuneStart(text[cut]) {
		cut--
	}
	return text[:cut] + "..."
}
