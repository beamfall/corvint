package contextindex

import (
	"fmt"

	"github.com/Beamfall/corvint/internal/runtimeenv"
)

// Two opt-in span modes above TCP-V0-025's `on` (docs/specs/context-ranking-v0.md,
// CRK-V0-009..012). `CORVINT_CONTEXT_SPANS=hits` is `on` plus each span's hit
// line range and the hit-window fallback for a declaration past the per-span
// clip or the remaining line budget; `merged` is `hits` plus the folding of
// overlapping or adjacent spans of one file into one row that lists every
// contributor. `on` keeps its bytes, and no mode touches `results`.

// contextSpanMode is the span flag's mode, "" when spans are off.
func contextSpanMode() string {
	switch value := runtimeenv.Value("CONTEXT_SPANS"); value {
	case "on", "hits", "merged":
		return value
	}
	return ""
}

// hitWindow is the window of contextSpanLineRadius lines either side of the
// span's hit range, clamped to the file: the range CRK-V0-010's fallback
// returns in place of an expansion that does not fit.
func (span contextSpan) hitWindow(count int) contextSpan {
	window := span
	window.start, window.end = max(span.hitStart-contextSpanLineRadius, 1), min(span.hitEnd+contextSpanLineRadius, count)
	return window
}

// clipNote is the reason, under `hits`, for a declaration of `lines` lines
// that the span keeps only the first contextSpanMaxLines of (CRK-V0-010).
func (ranker *spanRanker) clipNote(symbol string, lines int) string {
	if !ranker.hits || lines <= contextSpanMaxLines {
		return ""
	}
	return fmt.Sprintf("; `%s` runs %d lines, beyond the %d-line cap, so the hit range is returned", symbol, lines, contextSpanMaxLines)
}

// pastClipNote is the reason, under `hits`, for the hit window returned when
// the hit lies inside a declaration but past its span cap (CRK-V0-010).
func (ranker *spanRanker) pastClipNote(symbol string, lines int) string {
	if !ranker.hits {
		return ""
	}
	return fmt.Sprintf("; the hit lies past the first %d of the %d lines of `%s`, so the hit window is returned", contextSpanMaxLines, lines, symbol)
}

// budgetFallback is the hit window and its reason when the span would overrun
// the remaining line budget but its hit window fits (CRK-V0-010).
func (ranker *spanRanker) budgetFallback(span contextSpan, room int) (contextSpan, bool) {
	if !ranker.hits || span.hitStart < 1 {
		return span, false
	}
	lines, ok := ranker.lines(span.path)
	if !ok {
		return span, false
	}
	window, found := span.hitRange()
	if !found || window.end > len(lines) {
		return span, false
	}
	if window.lines() >= span.lines() || window.lines() > room {
		return span, false
	}
	window.note = span.note + "; the expansion would overrun the remaining line budget, so the hit range is returned"
	return window, true
}

// hitRange is the span with its range narrowed to the union of its
// contributors' hit ranges; false when any contributor lacks a hit.
func (span contextSpan) hitRange() (contextSpan, bool) {
	window := span
	window.start, window.end = 0, 0
	for _, part := range span.contributors() {
		if part.hitStart < 1 || part.hitEnd < part.hitStart {
			return span, false
		}
		if window.start == 0 {
			window.start, window.end = part.hitStart, part.hitEnd
			continue
		}
		window.start, window.end = min(window.start, part.hitStart), max(window.end, part.hitEnd)
	}
	return window, window.start > 0
}

// capMerged bounds a coalesced row by the per-span cap (TCP-V0-2221): a union
// past contextSpanMaxLines returns its contributors' hit range with a reason
// when that fits the cap, else it reports false and the row is omitted.
func capMerged(span contextSpan) (contextSpan, bool) {
	if span.lines() <= contextSpanMaxLines {
		return span, true
	}
	window, found := span.hitRange()
	if !found || window.lines() > contextSpanMaxLines {
		return span, false
	}
	window.note = span.note + fmt.Sprintf("; the merged union runs %d lines, beyond the %d-line cap, so the hit range is returned", span.lines(), contextSpanMaxLines)
	return window, true
}

// touches is overlap or adjacency within one file.
func (span contextSpan) touches(other contextSpan) bool {
	return span.path == other.path && span.start <= other.end+1 && other.start <= span.end+1
}

// contributors is a span's merged parts, or the span itself when unmerged.
func (span contextSpan) contributors() []contextSpan {
	if len(span.parts) > 0 {
		return span.parts
	}
	return []contextSpan{span}
}

// mergeSpans folds span and every selected span of its file that it touches,
// transitively, into one row at the earliest of those positions; the row
// keeps that earlier span's role, symbol, reason, hit range and authority and
// lists every contributor in rank order, the candidate last (CRK-V0-011). It is a
// pure function of its arguments and leaves the other rows in order. It
// reports false, and changes nothing, when span touches no selected span or
// the union would exceed contextSpanMaxLines.
func mergeSpans(selected []contextSpan, span contextSpan) ([]contextSpan, bool) {
	return mergeSpansBounded(selected, span, contextSpanMaxLines)
}

// coalesceSpans is a pure rank-stable normalization, independent of budgets.
// Its unions are unbounded; rank applies capMerged to each before selection.
// Only a fresh parts slice is extended; contributor slices in input are immutable.
func coalesceSpans(candidates []contextSpan) []contextSpan {
	selected := make([]contextSpan, 0, len(candidates))
	for _, span := range candidates {
		if merged, ok := mergeSpansBounded(selected, span, int(^uint(0)>>1)); ok {
			selected = merged
		} else {
			selected = append(selected, span)
		}
	}
	return selected
}

func mergeSpansBounded(selected []contextSpan, span contextSpan, maxLines int) ([]contextSpan, bool) {
	start, end := span.start, span.end
	joined := map[int]bool{}
	for grew := true; grew; {
		grew = false
		for index, other := range selected {
			if !joined[index] && other.touches(contextSpan{path: span.path, start: start, end: end}) {
				joined[index], grew = true, true
				start, end = min(start, other.start), max(end, other.end)
			}
		}
	}
	if len(joined) == 0 || end-start+1 > maxLines {
		return selected, false
	}
	merged := make([]contextSpan, 0, len(selected))
	head := -1
	for index, other := range selected {
		if !joined[index] {
			merged = append(merged, other)
			continue
		}
		if head < 0 {
			head = len(merged)
			row := other
			row.parts = nil
			merged = append(merged, row)
		}
		merged[head].parts = append(merged[head].parts, other.contributors()...)
	}
	row := &merged[head]
	row.parts = append(row.parts, span.contributors()...)
	row.start, row.end = start, end
	return merged, true
}

func spanLines(spans []contextSpan) int {
	total := 0
	for _, span := range spans {
		total += span.lines()
	}
	return total
}

// spanHitFields adds the hit range under `hits` and, under `merged`, the
// contributors a merged row folded in after its own (CRK-V0-009, -011): the
// row is its first contributor, so it is not repeated.
func (ranker *spanRanker) spanHitFields(row map[string]any, span contextSpan) {
	if !ranker.hits {
		return
	}
	if span.hitStart > 0 {
		row["hit_start_line"], row["hit_end_line"] = span.hitStart, span.hitEnd
	}
	row["slots"] = span.slots
	if len(span.parts) < 2 {
		return
	}
	parts := make([]any, 0, len(span.parts))
	for _, part := range span.parts {
		entry := map[string]any{
			"role": part.role, "slots": part.slots,
		}
		if part.hitStart > 0 {
			entry["hit_start_line"], entry["hit_end_line"] = part.hitStart, part.hitEnd
		}
		parts = append(parts, entry)
	}
	row["contributors"] = parts
}
