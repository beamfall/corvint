package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/mcp/protocol"
)

// DCP-V1-285..286: an opt-in aggregate outcome log. It holds only bounded
// counters per listed tool name; request arguments, query text, ids and paths
// never reach it, and recording never changes a result.
const (
	outcomeSchema   = "corvint-corpus-outcomes/1"
	outcomeMaxBytes = 1 << 20
	unlistedTool    = "unlisted"
)

// outcomeKinds is the closed outcome vocabulary: answered, the receipt miss
// kinds, a refused request and a failed call.
var outcomeKinds = map[string]bool{"answered": true, "no-match": true, "empty": true, "evidence-not-found": true, "page-exhausted": true, "capability-absent": true, "refused": true, "error": true}

// latencyBuckets are upper bounds in milliseconds; the last bucket is unbounded.
var latencyBuckets = []int64{10, 50, 100, 250, 500, 1000, 2500, 5000, 10000}

type outcomeFile struct {
	Schema         string                  `json:"schema"`
	ArtifactSHA256 string                  `json:"artifact_sha256"`
	WindowStart    string                  `json:"window_start"`
	WindowEnd      string                  `json:"window_end"`
	LatencyBounds  []int64                 `json:"latency_bucket_le_ms"`
	Tools          map[string]*toolOutcome `json:"tools"`
}

type toolOutcome struct {
	Calls    int            `json:"calls"`
	Outcomes map[string]int `json:"outcomes"`
	Latency  []int          `json:"latency_buckets"`
}

type outcomeLog struct {
	sync.Mutex
	path   string
	listed map[string]bool
	data   outcomeFile
	stderr io.Writer
	failed bool
	now    func() time.Time
}

// openOutcomeLog continues an existing log for the same artifact or starts an
// empty one; an unreadable, foreign or malformed log refuses startup.
func openOutcomeLog(path, artifactSHA256 string, listed []string, stderr io.Writer) (*outcomeLog, error) {
	l := &outcomeLog{path: path, listed: map[string]bool{}, stderr: stderr, now: time.Now}
	for _, name := range listed {
		l.listed[name] = true
	}
	raw, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		l.data = outcomeFile{Schema: outcomeSchema, ArtifactSHA256: artifactSHA256, LatencyBounds: latencyBuckets, Tools: map[string]*toolOutcome{}}
		return l, nil
	case err != nil:
		return nil, err
	}
	data, err := decodeOutcomes(raw)
	if err != nil {
		return nil, err
	}
	if data.ArtifactSHA256 != artifactSHA256 {
		return nil, errors.New("outcome log belongs to another artifact")
	}
	for name := range data.Tools {
		if name != unlistedTool && !l.listed[name] {
			return nil, errors.New("outcome log names a tool this server does not list")
		}
	}
	l.data = data
	return l, nil
}

func decodeOutcomes(raw []byte) (outcomeFile, error) {
	var data outcomeFile
	if len(raw) > outcomeMaxBytes {
		return data, errors.New("outcome log exceeds its bound")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&data) != nil || decoder.More() {
		return data, errors.New("outcome log is malformed")
	}
	if data.Schema != outcomeSchema || fmt.Sprint(data.LatencyBounds) != fmt.Sprint(latencyBuckets) || data.Tools == nil {
		return data, errors.New("outcome log is malformed")
	}
	for _, t := range data.Tools {
		sum := 0
		for kind, n := range t.Outcomes {
			if !outcomeKinds[kind] || n < 0 {
				return data, errors.New("outcome log is malformed")
			}
			sum += n
		}
		buckets := 0
		for _, n := range t.Latency {
			buckets += n
		}
		if t.Calls != sum || t.Calls != buckets || len(t.Latency) != len(latencyBuckets)+1 {
			return data, errors.New("outcome log is malformed")
		}
	}
	return data, nil
}

// classifyOutcome reads only the result's shape: the receipt miss kind, the
// error flag, or the refusal.
func classifyOutcome(result map[string]any, rpcErr *protocol.RPCError) string {
	if rpcErr != nil {
		if rpcErr.Code == protocol.CodeInvalidParams {
			return "refused"
		}
		return "error"
	}
	if result["isError"] != false {
		return "error"
	}
	structured, _ := result["structuredContent"].(map[string]any)
	miss, _ := structured["miss"].(string)
	if miss == "" {
		return "answered"
	}
	if outcomeKinds[miss] {
		return miss
	}
	return "error"
}

// record counts one call and rewrites the log atomically. A write failure is
// reported once on stderr and never alters the call's result.
func (l *outcomeLog) record(name, outcome string, elapsed time.Duration) {
	l.Lock()
	defer l.Unlock()
	if !l.listed[name] {
		name = unlistedTool
	}
	t := l.data.Tools[name]
	if t == nil {
		t = &toolOutcome{Outcomes: map[string]int{}, Latency: make([]int, len(latencyBuckets)+1)}
		l.data.Tools[name] = t
	}
	t.Calls++
	t.Outcomes[outcome]++
	bucket := sort.Search(len(latencyBuckets), func(i int) bool { return elapsed.Milliseconds() <= latencyBuckets[i] })
	t.Latency[bucket]++
	now := l.now().UTC().Format(time.RFC3339)
	if l.data.WindowStart == "" {
		l.data.WindowStart = now
	}
	l.data.WindowEnd = now
	if err := l.write(); err != nil && !l.failed {
		l.failed = true
		_, _ = fmt.Fprintln(l.stderr, "corvint-corpus-mcp: --outcome-log: write failed")
	}
}

func (l *outcomeLog) write() error {
	raw, err := json.MarshalIndent(l.data, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(l.path), ".outcomes-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if _, err = temp.Write(append(raw, '\n')); err == nil {
		err = temp.Close()
	} else {
		_ = temp.Close()
	}
	if err != nil {
		return err
	}
	return os.Rename(temp.Name(), l.path)
}

// outcomeFloor is a stored answered-share floor: overall and optionally per tool.
type outcomeFloor struct {
	AnsweredShare float64            `json:"answered_share"`
	Tolerance     float64            `json:"tolerance"`
	Tools         map[string]float64 `json:"tools,omitempty"`
}

type outcomeSummary struct {
	Schema         string                 `json:"schema"`
	ArtifactSHA256 string                 `json:"artifact_sha256"`
	WindowStart    string                 `json:"window_start"`
	WindowEnd      string                 `json:"window_end"`
	Calls          int                    `json:"calls"`
	AnsweredShare  float64                `json:"answered_share"`
	Tools          map[string]toolSummary `json:"tools"`
	Floor          *outcomeFloor          `json:"floor,omitempty"`
	Regressions    []string               `json:"regressions"`
}

type toolSummary struct {
	Calls         int     `json:"calls"`
	AnsweredShare float64 `json:"answered_share"`
}

// runOutcomeSummary implements --outcome-summary FILE [--floor FLOOR]: it
// prints the summary and exits 1 when the answered share, overall or for a
// floored tool, is below floor minus tolerance, or when no call is recorded
// under a floor; 2 for unusable input.
func runOutcomeSummary(arguments []string, stdout, stderr io.Writer) int {
	if len(arguments) != 2 && (len(arguments) != 4 || arguments[2] != "--floor" || arguments[3] == "") || arguments[1] == "" {
		_, _ = fmt.Fprintln(stderr, "corvint-corpus-mcp: invalid arguments")
		return 2
	}
	raw, err := os.ReadFile(arguments[1])
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "corvint-corpus-mcp: --outcome-summary: outcome log unavailable")
		return 2
	}
	data, err := decodeOutcomes(raw)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "corvint-corpus-mcp: --outcome-summary: "+err.Error())
		return 2
	}
	summary := outcomeSummary{Schema: "corvint-corpus-outcome-summary/1", ArtifactSHA256: data.ArtifactSHA256, WindowStart: data.WindowStart, WindowEnd: data.WindowEnd, Tools: map[string]toolSummary{}, Regressions: []string{}}
	answered := 0
	for name, t := range data.Tools {
		summary.Calls += t.Calls
		answered += t.Outcomes["answered"]
		summary.Tools[name] = toolSummary{Calls: t.Calls, AnsweredShare: share(t.Outcomes["answered"], t.Calls)}
	}
	summary.AnsweredShare = share(answered, summary.Calls)
	if len(arguments) == 4 {
		floor, ok := readFloor(arguments[3])
		if !ok {
			_, _ = fmt.Fprintln(stderr, "corvint-corpus-mcp: --floor: floor unavailable or malformed")
			return 2
		}
		summary.Floor = &floor
		check := func(label string, calls int, value, want float64) {
			if calls == 0 || value < want-floor.Tolerance {
				summary.Regressions = append(summary.Regressions, label)
			}
		}
		check("overall", summary.Calls, summary.AnsweredShare, floor.AnsweredShare)
		names := make([]string, 0, len(floor.Tools))
		for name := range floor.Tools {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			check(name, summary.Tools[name].Calls, summary.Tools[name].AnsweredShare, floor.Tools[name])
		}
	}
	out, _ := json.MarshalIndent(summary, "", "  ")
	_, _ = fmt.Fprintln(stdout, string(out))
	if len(summary.Regressions) > 0 {
		return 1
	}
	return 0
}

func share(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total)
}

func readFloor(path string) (outcomeFloor, bool) {
	var floor outcomeFloor
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) > outcomeMaxBytes {
		return floor, false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&floor) != nil || decoder.More() {
		return floor, false
	}
	valid := func(v float64) bool { return !math.IsNaN(v) && v >= 0 && v <= 1 }
	if !valid(floor.AnsweredShare) || !valid(floor.Tolerance) {
		return floor, false
	}
	for _, v := range floor.Tools {
		if !valid(v) {
			return floor, false
		}
	}
	return floor, true
}

// extractOutcomeLogFlag removes one --outcome-log FILE after the root and
// artifact arguments.
func extractOutcomeLogFlag(arguments []string) ([]string, string, bool) {
	remaining := make([]string, 0, len(arguments))
	path := ""
	for i := 0; i < len(arguments); i++ {
		if arguments[i] != "--outcome-log" {
			remaining = append(remaining, arguments[i])
			continue
		}
		if path != "" || i < 4 || i+1 >= len(arguments) || arguments[i+1] == "" {
			return nil, "", false
		}
		path = arguments[i+1]
		i++
	}
	return remaining, path, true
}
