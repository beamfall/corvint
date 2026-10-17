package sessiontest

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// ShardPlanProfile names the STR-V0-050 duration-balanced shard plan.
	ShardPlanProfile = "corvint-test-shards/0"
	// HistoryProfile is the retained full-run duration table that
	// `ci-shard-costs refresh` writes (STR-V0-051).
	HistoryProfile = "corvint-ci-package-costs/0"
	// DefaultHistoryRuns is K, the retained full runs a weight is the median
	// of, when --runs is not given (STR-V0-051).
	DefaultHistoryRuns = 5
	// HistoryInvalidCode begins the fallback reason when a supplied table
	// is not admitted (STR-V0-056).
	HistoryInvalidCode = "test-shards-history-invalid"
	// MaxHistoryRuns bounds K and the number of supplied tables.
	MaxHistoryRuns = 64
	// MaxShardPlanShards bounds the shard count, as the cishards partition.
	MaxShardPlanShards = 64
	// maxHistoryMilliseconds bounds one recorded duration: one day.
	maxHistoryMilliseconds = 24 * 60 * 60 * 1000
	// MaxFallbackBytes bounds the recorded fallback reason (STR-V0-053).
	MaxFallbackBytes = 1024
	// MaxShardPlanBytes bounds an encoded plan, derived from the admitted
	// inputs (STR-V0-050): the listed names total at most MaxDocumentBytes
	// of valid UTF-8 without control characters, so each escapes to at most
	// twice its bytes, and a name appears at most three times (a shard, the
	// weights, unknown); each appearance adds at most 16 bytes of quotes,
	// separators and an 8-digit weight. 64 KiB covers the fixed members,
	// 64 shard headers, 64 revisions and the bounded fallback reason.
	MaxShardPlanBytes = 6*MaxDocumentBytes + 3*16*MaxUnits + 64<<10
)

// History is one supplied retained full-run table: its name for reports and
// its raw bytes, nil when it could not be read.
type History struct {
	Name string
	Raw  []byte
}

// PlannedShard is one shard of a ShardPlan: its index, its files in lexical
// order and their summed weight (0 under the round-robin fallback).
type PlannedShard struct {
	Files              []string `json:"files"`
	Index              int      `json:"index"`
	WeightMilliseconds int64    `json:"weightMilliseconds"`
}

// ShardPlan is the STR-V0-050 corvint-test-shards/0 document.
type ShardPlan struct {
	Count int `json:"count"`
	// Fallback is empty when the weights came from history, else the
	// reason the plan is the complete round-robin (STR-V0-053).
	Fallback string `json:"fallback"`
	// History names the revisions of the runs the weights are the median
	// of, oldest first.
	History         []string       `json:"history"`
	PartitionDigest string         `json:"partitionDigest"`
	Profile         string         `json:"profile"`
	Runs            int            `json:"runs"`
	Shards          []PlannedShard `json:"shards"`
	// Unknown lists the listed files no used run recorded; each weighs the
	// median of the known listed files' weights (STR-V0-051).
	Unknown            []string         `json:"unknown"`
	WeightMilliseconds map[string]int64 `json:"weightMilliseconds"`
}

// ShardPartitionDigest binds the assignment alone: the profile, the shard
// count and each shard's files in order (STR-V0-052).
func ShardPartitionDigest(count int, shards []PlannedShard) (string, error) {
	files := make([][]string, len(shards))
	for i, shard := range shards {
		files[i] = shard.Files
	}
	return Digest(map[string]any{"count": count, "profile": ShardPlanProfile, "shards": files})
}

// ListedFiles parses the listed universe: whitespace-separated entries, at
// least one and at most MaxUnits, none repeated (STR-V0-050).
func ListedFiles(raw []byte) ([]string, error) {
	if len(raw) > MaxDocumentBytes {
		return nil, fmt.Errorf("the file list exceeds %d bytes", MaxDocumentBytes)
	}
	files := strings.Fields(string(raw))
	switch {
	case len(files) == 0:
		return nil, errors.New("the file list is empty")
	case len(files) > MaxUnits:
		return nil, fmt.Errorf("the file list names %d files, over the %d bound", len(files), MaxUnits)
	}
	seen := map[string]bool{}
	for _, file := range files {
		if seen[file] {
			return nil, fmt.Errorf("the file list names %s twice", file)
		}
		if !utf8.ValidString(file) || strings.IndexFunc(file, unicode.IsControl) >= 0 {
			return nil, fmt.Errorf("the file list entry %q is not valid UTF-8 or has a control character", file)
		}
		seen[file] = true
	}
	sort.Strings(files)
	return files, nil
}

type historyTable struct {
	Milliseconds map[string]int64 `json:"milliseconds"`
	Profile      string           `json:"profile"`
	Source       struct {
		GoVersion string `json:"goVersion"`
		Revision  string `json:"revision"`
		RunURL    string `json:"runURL"`
	} `json:"source"`
}

// readHistory admits one retained full-run table or says why not.
func readHistory(entry History) (historyTable, error) {
	var table historyTable
	if entry.Raw == nil {
		return table, fmt.Errorf("%s is unreadable", entry.Name)
	}
	if len(entry.Raw) > MaxDocumentBytes {
		return table, fmt.Errorf("%s exceeds %d bytes", entry.Name, MaxDocumentBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(entry.Raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&table); err != nil {
		return table, fmt.Errorf("%s is not a %s table: %v", entry.Name, HistoryProfile, err)
	}
	if decoder.Decode(new(any)) != io.EOF {
		return table, fmt.Errorf("%s has trailing data", entry.Name)
	}
	if table.Profile != HistoryProfile {
		return table, fmt.Errorf("%s profile is %q, want %q", entry.Name, table.Profile, HistoryProfile)
	}
	if _, err := hex.DecodeString(table.Source.Revision); err != nil || (len(table.Source.Revision) != 40 && len(table.Source.Revision) != 64) {
		return table, fmt.Errorf("%s names no full source revision", entry.Name)
	}
	if len(table.Milliseconds) == 0 || len(table.Milliseconds) > MaxUnits {
		return table, fmt.Errorf("%s records %d durations, want 1 to %d", entry.Name, len(table.Milliseconds), MaxUnits)
	}
	keys := make([]string, 0, len(table.Milliseconds))
	for file := range table.Milliseconds {
		keys = append(keys, file)
	}
	sort.Strings(keys)
	for _, file := range keys {
		milliseconds := table.Milliseconds[file]
		if file == "" || strings.IndexFunc(file, unicode.IsSpace) >= 0 || milliseconds <= 0 || milliseconds > maxHistoryMilliseconds {
			return table, fmt.Errorf("%s records an invalid duration for %q", entry.Name, file)
		}
	}
	return table, nil
}

// median is the middle of values, the mean of the two middle ones rounded
// down for an even count; values must be non-empty.
func median(values []int64) int64 {
	sorted := append([]int64{}, values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle]
	}
	return (sorted[middle-1] + sorted[middle]) / 2
}

// PlanShards derives the duration-balanced partition of files (STR-V0-050
// to STR-V0-053). It is pure: the same files, count, K and history give
// the same document and digest. history is ordered oldest first; the last
// runs of it weigh each file. Any unusable table, no table, or no listed
// file with a recorded duration falls back to the complete round-robin and
// says why. files must come from ListedFiles.
func PlanShards(files []string, count, runs int, history []History) (ShardPlan, error) {
	if count < 1 || count > MaxShardPlanShards {
		return ShardPlan{}, fmt.Errorf("shard count must be 1 to %d", MaxShardPlanShards)
	}
	if runs < 1 || runs > MaxHistoryRuns {
		return ShardPlan{}, fmt.Errorf("runs must be 1 to %d", MaxHistoryRuns)
	}
	plan := ShardPlan{
		Count: count, History: []string{}, Profile: ShardPlanProfile, Runs: runs,
		Unknown: []string{}, WeightMilliseconds: map[string]int64{},
	}
	weights, fallback := historyWeights(files, runs, history, &plan)
	shards := make([]PlannedShard, count)
	for i := range shards {
		shards[i] = PlannedShard{Files: []string{}, Index: i}
	}
	if fallback != "" {
		fallback = boundFallback(fallback)
		plan.Fallback, plan.History, plan.Unknown, plan.WeightMilliseconds = fallback, []string{}, []string{}, map[string]int64{}
		for i, file := range files {
			shards[i%count].Files = append(shards[i%count].Files, file)
		}
	} else {
		order := append([]string{}, files...)
		sort.SliceStable(order, func(i, j int) bool {
			if weights[order[i]] != weights[order[j]] {
				return weights[order[i]] > weights[order[j]]
			}
			return order[i] < order[j]
		})
		for _, file := range order {
			lightest := 0
			for i := 1; i < count; i++ {
				if shards[i].WeightMilliseconds < shards[lightest].WeightMilliseconds {
					lightest = i
				}
			}
			shards[lightest].Files = append(shards[lightest].Files, file)
			shards[lightest].WeightMilliseconds += weights[file]
		}
		for i := range shards {
			sort.Strings(shards[i].Files)
		}
		plan.WeightMilliseconds = weights
	}
	plan.Shards = shards
	digest, err := ShardPartitionDigest(count, shards)
	if err != nil {
		return ShardPlan{}, err
	}
	plan.PartitionDigest = digest
	return plan, nil
}

// EncodeShardPlan is the canonical plan bytes, refused over
// MaxShardPlanBytes (STR-V0-050).
func EncodeShardPlan(plan ShardPlan) ([]byte, error) {
	return encodeBounded(plan, MaxShardPlanBytes)
}

// boundFallback cuts a reason to MaxFallbackBytes of valid UTF-8, so a
// long table name or key cannot grow the plan (STR-V0-053).
func boundFallback(reason string) string {
	reason = strings.ToValidUTF8(reason, "?")
	if len(reason) <= MaxFallbackBytes {
		return reason
	}
	cut := MaxFallbackBytes - len("...")
	for cut > 0 && !utf8.RuneStart(reason[cut]) {
		cut--
	}
	return reason[:cut] + "..."
}

// historyWeights is the STR-V0-051 weight of every listed file, or the
// STR-V0-053 reason there is none.
func historyWeights(files []string, runs int, history []History, plan *ShardPlan) (map[string]int64, string) {
	if len(history) == 0 {
		return nil, "no retained run durations were supplied"
	}
	if len(history) > MaxHistoryRuns {
		return nil, fmt.Sprintf("%d duration tables exceed the %d bound", len(history), MaxHistoryRuns)
	}
	tables := make([]historyTable, 0, len(history))
	for _, entry := range history {
		table, err := readHistory(entry)
		if err != nil {
			return nil, HistoryInvalidCode + ": " + err.Error()
		}
		tables = append(tables, table)
	}
	if len(tables) > runs {
		tables = tables[len(tables)-runs:]
	}
	for _, table := range tables {
		plan.History = append(plan.History, table.Source.Revision)
	}
	weights := map[string]int64{}
	known := []int64{}
	for _, file := range files {
		samples := []int64{}
		for _, table := range tables {
			if milliseconds, ok := table.Milliseconds[file]; ok {
				samples = append(samples, milliseconds)
			}
		}
		if len(samples) == 0 {
			plan.Unknown = append(plan.Unknown, file)
			continue
		}
		weights[file] = median(samples)
		known = append(known, weights[file])
	}
	if len(known) == 0 {
		return nil, "no listed file has a retained run duration"
	}
	unknownWeight := median(known)
	for _, file := range plan.Unknown {
		weights[file] = unknownWeight
	}
	return weights, ""
}

// DecodeShardPlan decodes one canonical shard plan the caller read through a
// bounded, non-blocking regular-file open (STR-V0-055) and checks that it is
// one complete partition: count shards indexed in order, every file in
// exactly one shard, within the unit bound, and the recorded digest
// recomputed (STR-V0-054). name only labels errors.
func DecodeShardPlan(name string, raw []byte) (ShardPlan, error) {
	var plan ShardPlan
	if err := decodeStrictBytes(name, ShardPlanProfile, MaxShardPlanBytes, raw, &plan, func() string { return plan.Profile }); err != nil {
		return ShardPlan{}, err
	}
	if plan.Count < 1 || plan.Count > MaxShardPlanShards || len(plan.Shards) != plan.Count {
		return ShardPlan{}, fmt.Errorf("the shard plan has %d shards for count %d", len(plan.Shards), plan.Count)
	}
	seen := map[string]bool{}
	for i, shard := range plan.Shards {
		if shard.Index != i {
			return ShardPlan{}, fmt.Errorf("the shard plan's shard %d is indexed %d", i, shard.Index)
		}
		for _, file := range shard.Files {
			if file == "" || seen[file] {
				return ShardPlan{}, fmt.Errorf("the shard plan places %q more than once or names an empty file", file)
			}
			seen[file] = true
		}
	}
	if len(seen) == 0 || len(seen) > MaxUnits {
		return ShardPlan{}, fmt.Errorf("the shard plan places %d files, want 1 to %d", len(seen), MaxUnits)
	}
	digest, err := ShardPartitionDigest(plan.Count, plan.Shards)
	if err != nil {
		return ShardPlan{}, err
	}
	if digest != plan.PartitionDigest {
		return ShardPlan{}, errors.New("the shard plan's partitionDigest does not match its shards")
	}
	return plan, nil
}
