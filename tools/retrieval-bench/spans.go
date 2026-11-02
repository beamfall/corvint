package main

// spanRow is one row of a context packet's `spans` member, keyed by `path`
// as the packet writes it.
type spanRow struct {
	Path  string `json:"path"`
	Start int    `json:"start_line"`
	End   int    `json:"end_line"`
}

// spanMetrics scores a span-mode context answer against the sample's gold
// spans (CEP-V0-755): span_line_coverage is the share of distinct gold lines
// that some span row covers, span_line_precision the share of distinct span
// lines that are gold, and span_lines the distinct span lines returned. It
// adds nothing for an arm without a `spans` member or a sample without gold
// spans, so file-mode reports are unchanged.
func spanMetrics(scores metrics, answer arm, item sample) {
	gold := goldSpanIntervals(item)
	if !answer.SpanMode || scores == nil || len(gold) == 0 {
		return
	}
	predicted := map[string][]lineInterval{}
	for _, row := range answer.Spans {
		if row.Path != "" && row.Start >= 1 && row.End >= row.Start && row.End <= maxContextBenchLine {
			predicted[row.Path] = append(predicted[row.Path], lineInterval{row.Start, row.End})
		}
	}
	goldLines, predictedLines, shared := 0, 0, 0
	for path, intervals := range gold {
		merged := mergeLines(intervals)
		goldLines += intervalLines(merged)
		shared += overlapLines(merged, mergeLines(predicted[path]))
	}
	for _, intervals := range predicted {
		predictedLines += intervalLines(mergeLines(intervals))
	}
	coverage, precision := coveragePrecision(predictedLines, goldLines, shared)
	if predictedLines == 0 {
		precision = 0
	}
	scores["span_line_coverage"] = coverage
	scores["span_line_precision"] = precision
	scores["span_lines"] = float64(predictedLines)
}

// goldSpanIntervals reads a sample's gold line spans: ContextBench's
// `gold.lines` (`file`) and a release's `gold.gold_spans` (`path` or `file`,
// with `start_line`/`end_line`, or `start`/`end`). JSON numbers arrive as
// float64 and converted rows as int; a row that is not a line range is
// ignored.
func goldSpanIntervals(item sample) map[string][]lineInterval {
	result := map[string][]lineInterval{}
	for _, key := range []string{"lines", "gold_spans"} {
		entries, _ := item.Gold[key].([]any)
		for _, entry := range entries {
			span, _ := entry.(map[string]any)
			path, _ := span["path"].(string)
			if path == "" {
				path, _ = span["file"].(string)
			}
			start, end := lineNumber(span, "start_line", "start"), lineNumber(span, "end_line", "end")
			if path == "" || start < 1 || end < start || end > maxContextBenchLine {
				continue
			}
			result[path] = append(result[path], lineInterval{start, end})
		}
	}
	return result
}

func lineNumber(span map[string]any, keys ...string) int {
	for _, key := range keys {
		switch value := span[key].(type) {
		case int:
			return value
		case float64:
			if value == float64(int(value)) {
				return int(value)
			}
			return 0
		}
	}
	return 0
}

func intervalLines(intervals []lineInterval) int {
	total := 0
	for _, interval := range intervals {
		total += interval.end - interval.start + 1
	}
	return total
}

// overlapLines counts the lines two merged, sorted interval lists share.
func overlapLines(left, right []lineInterval) int {
	shared, i, j := 0, 0, 0
	for i < len(left) && j < len(right) {
		start, end := max(left[i].start, right[j].start), min(left[i].end, right[j].end)
		if start <= end {
			shared += end - start + 1
		}
		if left[i].end < right[j].end {
			i++
		} else {
			j++
		}
	}
	return shared
}
