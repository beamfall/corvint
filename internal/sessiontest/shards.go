package sessiontest

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/.github/cishards"
	"github.com/Beamfall/corvint/internal/gokernel"
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
	// 64 shard headers, 64 revisions and the bounded fallback reason;
	// MaxPlannedSliceBytes and 64 KiB more cover STR-V0-057 slices and
	// their per-shard members.
	MaxShardPlanBytes = 6*MaxDocumentBytes + 3*16*MaxUnits + 64<<10 + MaxPlannedSliceBytes + 64<<10
	// MaxPlannedSlices and MaxPlannedSliceBytes bound the test slices one
	// plan carries; over either the plan splits nothing (STR-V0-057).
	MaxPlannedSlices     = 256
	MaxPlannedSliceBytes = 4 << 20
)

// History is one supplied retained full-run table: its name for reports and
// its raw bytes, nil when it could not be read.
type History struct {
	Name string
	Raw  []byte
}

// PlannedShard is one shard of a ShardPlan: its index, its files in lexical
// order, its test slices by package and index (STR-V0-057; omitted when it
// has none) and their summed weight (0 under the round-robin fallback).
type PlannedShard struct {
	Files              []string       `json:"files"`
	Index              int            `json:"index"`
	Slices             []PlannedSlice `json:"slices,omitempty"`
	WeightMilliseconds int64          `json:"weightMilliseconds"`
}

// PlannedSlice is one AFP-V0-041 test slice of a listed entry: slices
// 0..Count-2 run their named tests with -run, slice Count-1 runs the rest
// with -skip (STR-V0-057).
type PlannedSlice struct {
	Count              int    `json:"count"`
	Flag               string `json:"flag"`
	Index              int    `json:"index"`
	Package            string `json:"package"`
	Pattern            string `json:"pattern"`
	WeightMilliseconds int64  `json:"weightMilliseconds"`
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
// count and each shard's files in order (STR-V0-052), and, when any shard
// has test slices, each shard's slices without their weights (STR-V0-057).
func ShardPartitionDigest(count int, shards []PlannedShard) (string, error) {
	files := make([][]string, len(shards))
	slices := make([][]map[string]any, len(shards))
	sliced := false
	for i, shard := range shards {
		files[i] = shard.Files
		slices[i] = []map[string]any{}
		for _, slice := range shard.Slices {
			sliced = true
			slices[i] = append(slices[i], map[string]any{
				"count": slice.Count, "flag": slice.Flag, "index": slice.Index, "package": slice.Package, "pattern": slice.Pattern,
			})
		}
	}
	bound := map[string]any{"count": count, "profile": ShardPlanProfile, "shards": files}
	if sliced {
		bound["slices"] = slices
	}
	return Digest(bound)
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
	return PlanShardsSliced(files, count, runs, history, nil, nil)
}

// PlanShardsSliced is PlanShards that, when history decides, places each
// listed entry the AFP-V0-041 allow-list and slice file split for count
// shards as its test slices, each on a distinct shard (STR-V0-057). Unusable
// slice inputs, or slices over the plan's slice bounds, split nothing.
func PlanShardsSliced(files []string, count, runs int, history []History, allowRaw, slicesRaw []byte) (ShardPlan, error) {
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
		placeUnits(shards, planUnits(files, weights, sliceUnits(files, count, weights, allowRaw, slicesRaw)))
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

// planUnit is one placed unit: a whole listed entry (slice nil) or one of
// its test slices.
type planUnit struct {
	name   string
	slice  *PlannedSlice
	weight int64
}

// sliceUnits returns the weighted test slices of every listed entry the
// allow-list and slice file split for count shards, or nil when there are
// none or they exceed MaxPlannedSlices or MaxPlannedSliceBytes (STR-V0-057).
// Slice i weighs max(1, w*m_i/sum(m)), rounded down.
func sliceUnits(files []string, count int, weights map[string]int64, allowRaw, slicesRaw []byte) map[string][]PlannedSlice {
	if allowRaw == nil || slicesRaw == nil {
		return nil
	}
	split := cishards.SplitPackages(files, count, allowRaw, slicesRaw)
	out := map[string][]PlannedSlice{}
	all := []PlannedSlice{}
	for _, file := range files {
		parts := split[file]
		if len(parts) == 0 {
			continue
		}
		var sum int64
		for _, part := range parts {
			sum += part.Milliseconds
		}
		for _, part := range parts {
			slice := PlannedSlice{
				Count: part.Count, Flag: part.Flag, Index: part.Index, Package: file, Pattern: part.Pattern,
				WeightMilliseconds: max(1, weights[file]*part.Milliseconds/sum),
			}
			out[file] = append(out[file], slice)
			all = append(all, slice)
		}
	}
	if len(all) == 0 || len(all) > MaxPlannedSlices {
		return nil
	}
	if encoded, err := gokernel.CanonicalJSON(all); err != nil || len(encoded) > MaxPlannedSliceBytes {
		return nil
	}
	return out
}

// planUnits lists every listed entry once: whole, or as all its slices.
func planUnits(files []string, weights map[string]int64, sliced map[string][]PlannedSlice) []planUnit {
	units := []planUnit{}
	for _, file := range files {
		if parts, ok := sliced[file]; ok {
			for i := range parts {
				units = append(units, planUnit{name: file, slice: &parts[i], weight: parts[i].WeightMilliseconds})
			}
			continue
		}
		units = append(units, planUnit{name: file, weight: weights[file]})
	}
	return units
}

// placeUnits puts the units heaviest first (ties by name, then slice index)
// on the lightest shard (ties by lowest index) that holds no sibling slice
// (STR-V0-052, STR-V0-057); a package has at most as many slices as shards.
func placeUnits(shards []PlannedShard, units []planUnit) {
	sort.SliceStable(units, func(i, j int) bool {
		a, b := units[i], units[j]
		if a.weight != b.weight {
			return a.weight > b.weight
		}
		if a.name != b.name {
			return a.name < b.name
		}
		return a.slice != nil && b.slice != nil && a.slice.Index < b.slice.Index
	})
	holds := map[string]map[int]bool{}
	for _, unit := range units {
		lightest := -1
		for i := range shards {
			if unit.slice != nil && holds[unit.name][i] {
				continue
			}
			if lightest < 0 || shards[i].WeightMilliseconds < shards[lightest].WeightMilliseconds {
				lightest = i
			}
		}
		if unit.slice == nil {
			shards[lightest].Files = append(shards[lightest].Files, unit.name)
		} else {
			if holds[unit.name] == nil {
				holds[unit.name] = map[int]bool{}
			}
			holds[unit.name][lightest] = true
			shards[lightest].Slices = append(shards[lightest].Slices, *unit.slice)
		}
		shards[lightest].WeightMilliseconds += unit.weight
	}
	for i := range shards {
		sort.Strings(shards[i].Files)
		sort.Slice(shards[i].Slices, func(a, b int) bool { return sliceBefore(shards[i].Slices[a], shards[i].Slices[b]) })
	}
}

func sliceBefore(a, b PlannedSlice) bool {
	if a.Package != b.Package {
		return a.Package < b.Package
	}
	return a.Index < b.Index
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
	sliced, err := checkPlannedSlices(plan, seen)
	if err != nil {
		return ShardPlan{}, err
	}
	if units := len(seen) + sliced; units == 0 || units > MaxUnits {
		return ShardPlan{}, fmt.Errorf("the shard plan places %d files, want 1 to %d", units, MaxUnits)
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

// checkPlannedSlices checks a plan's STR-V0-057 slices and returns how many
// packages they split: each slice well formed (-run on all but the last,
// -skip on the last, a compiling pattern within the AFP-V0-041 bound), every
// package complete with each index once, no sibling on one shard, no package
// also placed whole, and each shard's slices in package and index order.
func checkPlannedSlices(plan ShardPlan, whole map[string]bool) (int, error) {
	counts := map[string]int{}
	indexes := map[string]map[int]bool{}
	total := 0
	for _, shard := range plan.Shards {
		if shard.Slices != nil && len(shard.Slices) == 0 {
			return 0, fmt.Errorf("the shard plan's shard %d has an empty slices member", shard.Index)
		}
		onShard := map[string]bool{}
		for i, slice := range shard.Slices {
			total++
			name := slice.Package
			switch {
			case i > 0 && !sliceBefore(shard.Slices[i-1], slice):
				return 0, fmt.Errorf("the shard plan's shard %d slices are out of order", shard.Index)
			case name == "" || whole[name]:
				return 0, fmt.Errorf("the shard plan places %q both whole and as slices, or names an empty package", name)
			case slice.Count < 2 || slice.Count > cishards.MaxSlices || slice.Count > plan.Count || slice.Index < 0 || slice.Index >= slice.Count:
				return 0, fmt.Errorf("the shard plan's slice %d/%d of %s is out of range", slice.Index, slice.Count, name)
			case (slice.Index < slice.Count-1 && slice.Flag != "-run") || (slice.Index == slice.Count-1 && slice.Flag != "-skip"):
				return 0, fmt.Errorf("the shard plan's slice %d/%d of %s has flag %q", slice.Index, slice.Count, name, slice.Flag)
			case slice.Pattern == "" || len(slice.Pattern) > cishards.MaxPatternBytes || slice.WeightMilliseconds < 1:
				return 0, fmt.Errorf("the shard plan's slice %d/%d of %s has no usable pattern or weight", slice.Index, slice.Count, name)
			case onShard[name]:
				return 0, fmt.Errorf("the shard plan puts two slices of %s on shard %d", name, shard.Index)
			case counts[name] != 0 && counts[name] != slice.Count:
				return 0, fmt.Errorf("the shard plan's slices of %s disagree on their count", name)
			case indexes[name][slice.Index]:
				return 0, fmt.Errorf("the shard plan places slice %d of %s twice", slice.Index, name)
			}
			if _, err := regexp.Compile(slice.Pattern); err != nil {
				return 0, fmt.Errorf("the shard plan's slice %d/%d of %s has an invalid pattern", slice.Index, slice.Count, name)
			}
			onShard[name] = true
			counts[name] = slice.Count
			if indexes[name] == nil {
				indexes[name] = map[int]bool{}
			}
			indexes[name][slice.Index] = true
		}
	}
	if total > MaxPlannedSlices {
		return 0, fmt.Errorf("the shard plan carries %d slices, over the %d bound", total, MaxPlannedSlices)
	}
	for name, count := range counts {
		if len(indexes[name]) != count {
			return 0, fmt.Errorf("the shard plan places %d of the %d slices of %s", len(indexes[name]), count, name)
		}
	}
	return len(counts), nil
}
