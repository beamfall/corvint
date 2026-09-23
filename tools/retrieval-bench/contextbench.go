package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// contextBenchTask is the task type a ContextBench row is read as
// (CEP-V0-001). ContextBench (arXiv 2602.05892) ships its rows as Parquet on
// Hugging Face (`Contextbench/ContextBench`); this tool reads the same rows
// exported one JSON object per line and never reads Parquet or the network.
const contextBenchTask = "contextbench"

// contextBenchRow is one ContextBench row, keeping the dataset's column names.
// gold_context is itself a JSON-encoded list of spans.
type contextBenchRow struct {
	InstanceID       string `json:"instance_id"`
	OriginalID       string `json:"original_inst_id"`
	Repo             string `json:"repo"`
	BaseCommit       string `json:"base_commit"`
	ProblemStatement string `json:"problem_statement"`
	GoldContext      string `json:"gold_context"`
	Language         string `json:"language"`
	Source           string `json:"source"`
}

type contextBenchSpan struct {
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

// decodeSample reads one line as an Agent Retrieval Bench sample, or as a
// ContextBench row when it carries `instance_id`.
func decodeSample(line []byte) (sample, error) {
	var probe struct {
		InstanceID *string `json:"instance_id"`
	}
	if err := json.Unmarshal(line, &probe); err != nil {
		return sample{}, err
	}
	if probe.InstanceID == nil {
		var item sample
		err := json.Unmarshal(line, &item)
		return item, err
	}
	return contextBenchSample(line)
}

// contextBenchSample maps one row to one sample: the problem statement is the
// query, every gold span's file is gold, and the spans are kept for the line
// metrics. A row it cannot map field for field is refused.
func contextBenchSample(line []byte) (sample, error) {
	var row contextBenchRow
	if err := json.Unmarshal(line, &row); err != nil {
		return sample{}, err
	}
	if row.InstanceID == "" || row.Repo == "" || row.BaseCommit == "" || row.ProblemStatement == "" {
		return sample{}, fmt.Errorf("contextbench row lacks instance_id, repo, base_commit, or problem_statement")
	}
	var spans []contextBenchSpan
	if err := json.Unmarshal([]byte(row.GoldContext), &spans); err != nil {
		return sample{}, fmt.Errorf("contextbench row %s: gold_context: %w", row.InstanceID, err)
	}
	files := map[string]bool{}
	lines := make([]any, 0, len(spans))
	for _, span := range spans {
		path := contextBenchPath(span.File)
		if path == "" {
			continue
		}
		if span.StartLine < 1 || span.EndLine < span.StartLine {
			return sample{}, fmt.Errorf("contextbench row %s: span %s:%d-%d is not a line range", row.InstanceID, path, span.StartLine, span.EndLine)
		}
		files[path] = true
		lines = append(lines, map[string]any{"file": path, "start_line": float64(span.StartLine), "end_line": float64(span.EndLine)})
	}
	goldPaths := make([]any, 0, len(files))
	for _, path := range sortedKeys(files) {
		goldPaths = append(goldPaths, path)
	}
	gold := map[string]any{"files": goldPaths, "lines": lines}
	return sample{
		ID: row.InstanceID, TaskType: contextBenchTask, Repo: row.Repo, BaseCommit: row.BaseCommit,
		Query:    map[string]any{"problem_statement": row.ProblemStatement},
		Gold:     gold,
		Metadata: map[string]any{"original_inst_id": row.OriginalID, "language": row.Language, "source": row.Source},
	}, nil
}

// contextBenchPath is ContextBench's `_normalize_rel_path`, byte for byte,
// including Python's `lstrip("./")`, which strips every leading `.` and `/`.
func contextBenchPath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	if rest, found := strings.CutPrefix(path, "/testbed/"); found {
		return rest
	}
	if rest, found := strings.CutPrefix(path, "/workspace/"); found {
		if _, after, split := strings.Cut(rest, "/"); split {
			return after
		}
		return rest
	}
	if strings.HasPrefix(path, "/") {
		return strings.TrimLeft(path, "/")
	}
	return strings.TrimLeft(path, "./")
}

// lineInterval is an inclusive line range.
type lineInterval struct{ start, end int }

// contextBenchMetrics adds ContextBench's file and line granularities
// (CEP-V0-002) for a positive ContextBench sample: coverage is the gold share
// the answer covers and precision the answer share that is gold, with the
// bench's edge rules (empty gold: coverage 1; empty answer: precision 1). The
// answer is the ranking cut at k, and a ranked file covers all of its lines in
// the snapshot, because a packet names files, not line spans.
func contextBenchMetrics(result metrics, answer arm, item sample, root string, limit int) {
	ranked := answer.Ranked[:min(limit, len(answer.Ranked))]
	gold := goldFiles(item)
	hits := len(intersection(ranked, gold))
	result["cb_file_coverage"], result["cb_file_precision"] = coveragePrecision(len(ranked), len(gold), hits)
	goldLines := goldLineIntervals(item)
	predicted := map[string][]lineInterval{}
	for _, path := range ranked {
		if count := fileLineCount(filepath.Join(root, filepath.FromSlash(path))); count > 0 {
			predicted[path] = []lineInterval{{1, count}}
		}
	}
	result["cb_line_coverage"], result["cb_line_precision"] = coveragePrecision(totalLines(predicted), totalLines(goldLines), sharedLines(predicted, goldLines))
}

func coveragePrecision(predicted, gold, shared int) (float64, float64) {
	coverage, precision := 1.0, 1.0
	if gold > 0 {
		coverage = float64(shared) / float64(gold)
	}
	if predicted > 0 {
		precision = float64(shared) / float64(predicted)
	}
	return coverage, precision
}

func goldLineIntervals(item sample) map[string][]lineInterval {
	spans, _ := item.Gold["lines"].([]any)
	result := map[string][]lineInterval{}
	for _, entry := range spans {
		span, _ := entry.(map[string]any)
		path, _ := span["file"].(string)
		start, _ := span["start_line"].(float64)
		end, _ := span["end_line"].(float64)
		result[path] = append(result[path], lineInterval{int(start), int(end)})
	}
	return result
}

// mergeLines merges overlapping or adjacent intervals, as the bench does.
func mergeLines(intervals []lineInterval) []lineInterval {
	sorted := append([]lineInterval(nil), intervals...)
	sort.Slice(sorted, func(left, right int) bool {
		if sorted[left].start != sorted[right].start {
			return sorted[left].start < sorted[right].start
		}
		return sorted[left].end < sorted[right].end
	})
	merged := make([]lineInterval, 0, len(sorted))
	for _, current := range sorted {
		last := len(merged) - 1
		if last >= 0 && current.start <= merged[last].end+1 {
			merged[last].end = max(merged[last].end, current.end)
			continue
		}
		merged = append(merged, current)
	}
	return merged
}

func totalLines(byFile map[string][]lineInterval) int {
	total := 0
	for _, intervals := range byFile {
		for _, interval := range mergeLines(intervals) {
			total += interval.end - interval.start + 1
		}
	}
	return total
}

func sharedLines(left, right map[string][]lineInterval) int {
	total := 0
	for path, intervals := range left {
		for _, a := range mergeLines(intervals) {
			for _, b := range mergeLines(right[path]) {
				total += max(0, min(a.end, b.end)-max(a.start, b.start)+1)
			}
		}
	}
	return total
}

// fileLineCount counts a regular file's lines as an editor numbers them; a
// path that is not a regular file in the snapshot covers no line.
func fileLineCount(path string) int {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return 0
	}
	file, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	buffer := make([]byte, 64<<10)
	count, last := 0, byte('\n')
	for {
		read, err := reader.Read(buffer)
		if read > 0 {
			count += bytes.Count(buffer[:read], []byte{'\n'})
			last = buffer[read-1]
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0
		}
	}
	if last != '\n' {
		count++
	}
	return count
}
