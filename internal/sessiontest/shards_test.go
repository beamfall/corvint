package sessiontest

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// readShardPlanFile reads a plan the test wrote as a regular file; the
// product reads it through a bounded, non-blocking open (STR-V0-055).
func readShardPlanFile(name string) (ShardPlan, error) {
	raw, err := os.ReadFile(name)
	if err != nil {
		return ShardPlan{}, err
	}
	return DecodeShardPlan(name, raw)
}

// historyTableJSON is one retained full-run table in the
// corvint-ci-package-costs/0 shape `ci-shard-costs refresh` writes.
func historyTableJSON(revision byte, milliseconds map[string]int64) []byte {
	entries := []string{}
	for _, file := range sortedKeys(func() map[string]bool {
		set := map[string]bool{}
		for file := range milliseconds {
			set[file] = true
		}
		return set
	}()) {
		entries = append(entries, fmt.Sprintf("%q: %d", file, milliseconds[file]))
	}
	return []byte(`{"milliseconds": {` + strings.Join(entries, ", ") + `}, "profile": "corvint-ci-package-costs/0", "source": {"goVersion": "go1.27.2", "revision": "` +
		strings.Repeat(string(revision), 40) + `", "runURL": "https://example.test/runs/1"}}`)
}

func planFiles(plan ShardPlan) [][]string {
	files := [][]string{}
	for _, shard := range plan.Shards {
		files = append(files, shard.Files)
	}
	return files
}

// STR-V0-050: the listed universe is whitespace-separated, sorted, bounded
// and names every file once.
func TestSTRV0050ListedFiles(t *testing.T) {
	files, err := ListedFiles([]byte("b\n a\r\n\nc\n"))
	if err != nil || !reflect.DeepEqual(files, []string{"a", "b", "c"}) {
		t.Fatalf("listed %v %v", files, err)
	}
	for name, raw := range map[string]string{
		"empty": " \n", "duplicate": "a\nb\na\n", "control character": "a\x01b\n",
		"over the unit bound": func() string {
			var b strings.Builder
			for i := 0; i <= MaxUnits; i++ {
				fmt.Fprintf(&b, "f%d\n", i)
			}
			return b.String()
		}(),
	} {
		if _, err := ListedFiles([]byte(raw)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := ListedFiles(make([]byte, MaxDocumentBytes+1)); err == nil {
		t.Error("an oversized list was accepted")
	}
}

// STR-V0-051: each file weighs the median of the last K supplied runs that
// record it; K is configurable; a file no used run records is flagged and
// weighs the median of the known listed files' weights.
func TestSTRV0051MedianOfLastKRuns(t *testing.T) {
	history := []History{
		{Name: "old", Raw: historyTableJSON('1', map[string]int64{"a": 1000, "b": 100, "c": 50})},
		{Name: "mid", Raw: historyTableJSON('2', map[string]int64{"a": 10, "b": 200, "c": 60})},
		{Name: "new", Raw: historyTableJSON('3', map[string]int64{"a": 20, "b": 300})},
	}
	files := []string{"a", "b", "c", "d"}
	plan, err := PlanShards(files, 2, 2, history)
	if err != nil || plan.Fallback != "" {
		t.Fatalf("plan %v %q", err, plan.Fallback)
	}
	// K=2: a median(10,20)=15, b median(200,300)=250, c only in mid: 60;
	// d unknown: median(15,60,250)=60.
	want := map[string]int64{"a": 15, "b": 250, "c": 60, "d": 60}
	if !reflect.DeepEqual(plan.WeightMilliseconds, want) || !reflect.DeepEqual(plan.Unknown, []string{"d"}) {
		t.Fatalf("weights %v unknown %v", plan.WeightMilliseconds, plan.Unknown)
	}
	if !reflect.DeepEqual(plan.History, []string{strings.Repeat("2", 40), strings.Repeat("3", 40)}) || plan.Runs != 2 {
		t.Fatalf("history %v runs %d", plan.History, plan.Runs)
	}
	// K=3 brings back the oldest run: a median(1000,10,20)=20, c (50,60)=55.
	plan, err = PlanShards(files, 2, 3, history)
	if err != nil || plan.WeightMilliseconds["a"] != 20 || plan.WeightMilliseconds["c"] != 55 || plan.WeightMilliseconds["b"] != 200 || len(plan.History) != 3 {
		t.Fatalf("K=3 weights %v history %v %v", plan.WeightMilliseconds, plan.History, err)
	}
	if _, err := PlanShards(files, 2, 0, history); err == nil {
		t.Error("K=0 accepted")
	}
	if _, err := PlanShards(files, 2, MaxHistoryRuns+1, history); err == nil {
		t.Error("K over the bound accepted")
	}
}

// STR-V0-052: longest first onto the lightest shard; every listed file in
// exactly one shard for 1 to 16 shards; the same inputs give the same
// bytes and digest, and the digest moves with the partition.
func TestSTRV0052LongestFirstExactlyOnceDeterministic(t *testing.T) {
	history := []History{{Name: "run", Raw: historyTableJSON('a', map[string]int64{"a": 10, "b": 9, "c": 8, "d": 1, "e": 1, "f": 1})}}
	plan, err := PlanShards([]string{"a", "b", "c", "d", "e", "f"}, 2, 5, history)
	if err != nil {
		t.Fatal(err)
	}
	// a(10)->0, b(9)->1, c(8)->1 (9<10), d->0 (10<17), e->0 (11<17), f->0 (12<17).
	if got := planFiles(plan); !reflect.DeepEqual(got, [][]string{{"a", "d", "e", "f"}, {"b", "c"}}) {
		t.Fatalf("partition %v", got)
	}
	if plan.Shards[0].WeightMilliseconds != 13 || plan.Shards[1].WeightMilliseconds != 17 {
		t.Fatalf("shard weights %+v", plan.Shards)
	}
	files := []string{}
	weights := map[string]int64{}
	for i := 0; i < 97; i++ {
		file := fmt.Sprintf("pkg/%02d", i)
		files = append(files, file)
		if i%7 != 0 {
			weights[file] = int64(1 + (i*37)%101)
		}
	}
	runs := []History{{Name: "r1", Raw: historyTableJSON('1', weights)}, {Name: "r2", Raw: historyTableJSON('2', weights)}}
	for total := 1; total <= 16; total++ {
		first, err := PlanShards(files, total, 5, runs)
		if err != nil {
			t.Fatal(err)
		}
		again, _ := PlanShards(append([]string{}, files...), total, 5, runs)
		a, _ := Encode(first)
		b, _ := Encode(again)
		if string(a) != string(b) || first.PartitionDigest != again.PartitionDigest || !strings.HasPrefix(first.PartitionDigest, "sha256:") {
			t.Fatalf("%d shards: not deterministic", total)
		}
		seen := map[string]int{}
		for _, shard := range first.Shards {
			for _, file := range shard.Files {
				seen[file]++
			}
		}
		if len(seen) != len(files) {
			t.Fatalf("%d shards: %d of %d files placed", total, len(seen), len(files))
		}
		for file, n := range seen {
			if n != 1 {
				t.Fatalf("%d shards: %s placed %d times", total, file, n)
			}
		}
		if len(first.Unknown) != 14 {
			t.Fatalf("%d shards: %d unknown files flagged, want 14", total, len(first.Unknown))
		}
		if total > 1 {
			other, _ := PlanShards(files, total-1, 5, runs)
			if other.PartitionDigest == first.PartitionDigest {
				t.Fatalf("%d and %d shards share a digest", total, total-1)
			}
		}
	}
}

// STR-V0-053: no history, any unusable table, or no listed file with a
// duration falls back to the complete round-robin and says why.
func TestSTRV0053FallbackIsCompleteRoundRobin(t *testing.T) {
	files := []string{"a", "b", "c", "d", "e"}
	valid := History{Name: "ok", Raw: historyTableJSON('1', map[string]int64{"a": 5})}
	cases := map[string][]History{
		"no history":       nil,
		"unreadable":       {valid, {Name: "gone"}},
		"not json":         {valid, {Name: "bad", Raw: []byte("{")}},
		"trailing data":    {{Name: "bad", Raw: append(historyTableJSON('1', map[string]int64{"a": 5}), []byte("{}")...)}},
		"other profile":    {{Name: "bad", Raw: []byte(strings.Replace(string(valid.Raw), "package-costs/0", "package-costs/9", 1))}},
		"unknown member":   {{Name: "bad", Raw: []byte(strings.Replace(string(valid.Raw), `"profile"`, `"extra": 1, "profile"`, 1))}},
		"short revision":   {{Name: "bad", Raw: []byte(strings.Replace(string(valid.Raw), strings.Repeat("1", 40), "1234", 1))}},
		"zero duration":    {{Name: "bad", Raw: historyTableJSON('1', map[string]int64{"a": 0})}},
		"over one day":     {{Name: "bad", Raw: historyTableJSON('1', map[string]int64{"a": maxHistoryMilliseconds + 1})}},
		"empty table":      {{Name: "bad", Raw: historyTableJSON('1', map[string]int64{})}},
		"no listed file":   {{Name: "other", Raw: historyTableJSON('1', map[string]int64{"z": 5})}},
		"too many tables":  make([]History, MaxHistoryRuns+1),
		"invalid is first": {{Name: "bad", Raw: []byte("[]")}, valid},
	}
	for name, history := range cases {
		plan, err := PlanShards(files, 2, 5, history)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if plan.Fallback == "" || len(plan.History) != 0 || len(plan.Unknown) != 0 || len(plan.WeightMilliseconds) != 0 {
			t.Fatalf("%s: fallback %q history %v unknown %v", name, plan.Fallback, plan.History, plan.Unknown)
		}
		if got := planFiles(plan); !reflect.DeepEqual(got, [][]string{{"a", "c", "e"}, {"b", "d"}}) {
			t.Fatalf("%s: round-robin %v", name, got)
		}
		// STR-V0-056: a table that is not admitted names the code.
		admitted := name == "no history" || name == "no listed file" || name == "too many tables"
		if named := strings.HasPrefix(plan.Fallback, HistoryInvalidCode+": "); named == admitted {
			t.Fatalf("%s: fallback %q", name, plan.Fallback)
		}
	}
}

// STR-V0-054: a written plan reads back only when it is canonical, one
// complete partition and its digest recomputes.
func TestSTRV0054ReadShardPlan(t *testing.T) {
	plan, err := PlanShards([]string{"a", "b", "c"}, 2, 5, nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := Encode(plan)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	read, err := readShardPlanFile(write("ok.json", string(encoded)))
	if err != nil || read.PartitionDigest != plan.PartitionDigest {
		t.Fatalf("round trip %v", err)
	}
	text := string(encoded)
	for name, body := range map[string]string{
		"tampered digest": strings.Replace(text, plan.PartitionDigest, "sha256:"+strings.Repeat("0", 64), 1),
		"moved file":      strings.Replace(text, `"files":["a","c"]`, `"files":["a"]`, 1),
		"duplicate file":  strings.Replace(text, `"files":["b"]`, `"files":["a","b"]`, 1),
		"wrong count":     strings.Replace(text, `"count":2`, `"count":3`, 1),
		"not canonical":   strings.Replace(text, `{"count"`, `{ "count"`, 1),
		"other profile":   strings.Replace(text, ShardPlanProfile, "corvint-test-shards/9", 1),
		"reindexed":       strings.Replace(text, `"index":1`, `"index":0`, 1),
	} {
		if body == text {
			t.Fatalf("%s: the mutation did not apply", name)
		}
		if _, err := readShardPlanFile(write(strings.ReplaceAll(name, " ", "-")+".json", body)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// STR-V0-054: measured placement puts every run-set package whole on its
// planned shard, NARROWED and FULL alike; the manifest binds the plan's
// digest; a plan for another count or one that omits a package places no
// Go unit and is INCOMPLETE.
func TestSTRV0054MeasuredPlacement(t *testing.T) {
	plan, err := PlanShards([]string{"example.test/m/a", "example.test/m/b", "example.test/m/c"}, 2, 5,
		[]History{{Name: "run", Raw: historyTableJSON('1', map[string]int64{"example.test/m/a": 9, "example.test/m/b": 5, "example.test/m/c": 5})}})
	if err != nil {
		t.Fatal(err)
	}
	build := func(mutate func(*Input)) Manifest {
		input := fixtureInput()
		input.Placement, input.ShardCount, input.ShardPlan = PlacementMeasured, 2, &plan
		mutate(&input)
		m, err := Build(input)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	narrowed := build(func(input *Input) { input.GoSelection = "run example.test/m/b example.test/m/a" })
	if got := unitIDs(narrowed); !reflect.DeepEqual(got, []string{"0 go:example.test/m/a whole", "1 go:example.test/m/b whole"}) || narrowed.PartitionDigest != plan.PartitionDigest || narrowed.Placement != PlacementMeasured {
		t.Fatalf("narrowed %v %s %s", got, narrowed.PartitionDigest, narrowed.Placement)
	}
	full := build(func(input *Input) { input.GoSelection = "FALLBACK go.mod changed" })
	goUnits := []string{}
	for _, id := range unitIDs(full) {
		if strings.Contains(id, " go:") {
			goUnits = append(goUnits, id)
		}
	}
	if got := goUnits; full.Scope != ScopeFull || !reflect.DeepEqual(got, []string{"0 go:example.test/m/a whole", "1 go:example.test/m/b whole", "1 go:example.test/m/c whole"}) {
		t.Fatalf("full %s %v", full.Scope, got)
	}
	for name, mutate := range map[string]func(*Input){
		"other count": func(input *Input) { input.ShardCount = 3 },
		"omitted package": func(input *Input) {
			input.GoSelection = "run example.test/m/d"
			input.Universe = append(input.Universe, "example.test/m/d")
		},
	} {
		m := build(mutate)
		if len(m.Incomplete) != 1 || !strings.HasPrefix(m.Incomplete[0], "go: the shard plan") {
			t.Fatalf("%s: incomplete %v", name, m.Incomplete)
		}
		for _, unit := range m.Units {
			if unit.Adapter == AdapterGo {
				t.Fatalf("%s: placed %s", name, unit.Key())
			}
		}
	}
	input := fixtureInput()
	input.Placement = PlacementMeasured
	if _, err := Build(input); err == nil {
		t.Fatal("measured placement without a plan was accepted")
	}
}

// STR-V0-050: a plan built from inputs at the admitted bounds, with names
// that escape to twice their bytes and nearly all of them unknown, encodes
// within MaxShardPlanBytes and reads back byte-identically; a non-UTF-8
// entry is refused at admission.
func TestSTRV0050NearBoundaryPlanRoundTrip(t *testing.T) {
	names := make([]string, MaxUnits)
	for i := range names {
		names[i] = fmt.Sprintf("%04x", i) + strings.Repeat(`\`, 251)
	}
	raw := []byte(strings.Join(names, "\n"))
	if len(raw) > MaxDocumentBytes || len(raw) < MaxDocumentBytes-8 {
		t.Fatalf("the list is %d bytes, not at the %d bound", len(raw), MaxDocumentBytes)
	}
	files, err := ListedFiles(raw)
	if err != nil {
		t.Fatal(err)
	}
	history := []History{{Name: "run", Raw: historyTableJSON('7', map[string]int64{files[0]: maxHistoryMilliseconds})}}
	plan, err := PlanShards(files, MaxShardPlanShards, MaxHistoryRuns, history)
	if err != nil || plan.Fallback != "" || len(plan.Unknown) != MaxUnits-1 {
		t.Fatalf("plan: %v fallback %q unknown %d", err, plan.Fallback, len(plan.Unknown))
	}
	encoded, err := EncodeShardPlan(plan)
	if err != nil {
		t.Fatalf("a plan from admitted inputs is refused: %v", err)
	}
	if len(encoded) <= MaxDocumentBytes {
		t.Fatalf("the near-boundary plan is only %d bytes", len(encoded))
	}
	name := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(name, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	read, err := readShardPlanFile(name)
	if err != nil {
		t.Fatalf("the near-boundary plan does not read back: %v", err)
	}
	again, err := EncodeShardPlan(read)
	if err != nil || string(again) != string(encoded) {
		t.Fatalf("round trip differs: %v", err)
	}
	if _, err := ListedFiles([]byte("a\n\xff\n")); err == nil {
		t.Fatal("a non-UTF-8 entry was admitted")
	}
}

// STR-V0-052, STR-V0-053: a table with several invalid durations falls back
// with the same reason, plan bytes and digest on every run, and a reason
// naming a long key stays within MaxFallbackBytes.
func TestSTRV0053FallbackReasonIsDeterministic(t *testing.T) {
	invalid := map[string]int64{}
	for i := range 16 {
		invalid[fmt.Sprintf("p%02d", i)] = -1
	}
	history := []History{{Name: "run", Raw: historyTableJSON('8', invalid)}}
	var first []byte
	for range 50 {
		plan, err := PlanShards([]string{"p00", "p01"}, 2, DefaultHistoryRuns, history)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := EncodeShardPlan(plan)
		if err != nil {
			t.Fatal(err)
		}
		if first == nil {
			first = encoded
			if !strings.Contains(plan.Fallback, `"p00"`) {
				t.Fatalf("fallback %q does not name the first invalid key", plan.Fallback)
			}
		} else if string(encoded) != string(first) {
			t.Fatalf("fallback output differs between runs:\n%s\n%s", first, encoded)
		}
	}
	long := []History{{Name: strings.Repeat("n", 4*MaxFallbackBytes), Raw: nil}}
	plan, err := PlanShards([]string{"a"}, 1, 1, long)
	if err != nil || len(plan.Fallback) > MaxFallbackBytes || !strings.HasSuffix(plan.Fallback, "...") {
		t.Fatalf("long fallback %d bytes: %v", len(plan.Fallback), err)
	}
}

// sliceFixtures is an AFP-V0-041 allow-list admitting example.test/m/a and a
// slice file splitting it into two named slices and the rest, 2 s each.
func sliceFixtures() (allow, slices []byte) {
	allow = []byte(`{"packages":{"example.test/m/a":"fixture: tests are order-independent"},"profile":"corvint-ci-test-split-allow/0"}`)
	slices = []byte(`{"packages":{"example.test/m/a":{"named":[{"milliseconds":2000,"tests":["TestA1"]},{"milliseconds":2000,"tests":["TestA2"]}],"restMilliseconds":2000}},` +
		`"profile":"corvint-ci-test-slices/0","source":{"goVersion":"go1.27.2","revision":"` + strings.Repeat("d", 40) + `","runURL":"https://github.com/beamfall/corvint/actions/runs/1"}}`)
	return allow, slices
}

func slicedHistory() []History {
	return []History{{Name: "run", Raw: historyTableJSON('1', map[string]int64{"example.test/m/a": 9000, "example.test/m/b": 1000, "example.test/m/c": 1000})}}
}

// STR-V0-057: when history decides, an allow-listed package the slice file
// splits is placed as its slices, each weighing its share of the package's
// median, heaviest first onto the lightest shard holding no sibling, so the
// package no longer sets the shard floor; every listed entry appears exactly
// once, the digest binds the slices, and a plan with no slice is
// byte-identical to an unsliced plan.
func TestSTRV0057SlicedPlan(t *testing.T) {
	files := []string{"example.test/m/a", "example.test/m/b", "example.test/m/c"}
	allow, slices := sliceFixtures()
	plan, err := PlanShardsSliced(files, 3, 5, slicedHistory(), allow, slices)
	if err != nil {
		t.Fatal(err)
	}
	whole, err := PlanShards(files, 3, 5, slicedHistory())
	if err != nil {
		t.Fatal(err)
	}
	if got := planFiles(whole); !reflect.DeepEqual(got, [][]string{{"example.test/m/a"}, {"example.test/m/b"}, {"example.test/m/c"}}) || whole.Shards[0].WeightMilliseconds != 9000 {
		t.Fatalf("unsliced floor %v", got)
	}
	if got := planFiles(plan); !reflect.DeepEqual(got, [][]string{{"example.test/m/b"}, {"example.test/m/c"}, {}}) {
		t.Fatalf("sliced files %v", got)
	}
	weights := []int64{}
	for i, shard := range plan.Shards {
		weights = append(weights, shard.WeightMilliseconds)
		if len(shard.Slices) != 1 || shard.Slices[0].Package != "example.test/m/a" || shard.Slices[0].Index != i || shard.Slices[0].Count != 3 || shard.Slices[0].WeightMilliseconds != 3000 {
			t.Fatalf("shard %d slices %+v", i, shard.Slices)
		}
	}
	if !reflect.DeepEqual(weights, []int64{4000, 4000, 3000}) {
		t.Fatalf("weights %v", weights)
	}
	if got := plan.Shards[0].Slices[0]; got.Flag != "-run" || got.Pattern != "^(TestA1)$" || plan.Shards[2].Slices[0].Flag != "-skip" || plan.Shards[2].Slices[0].Pattern != "^(TestA1|TestA2)$" {
		t.Fatalf("slice flags %+v %+v", got, plan.Shards[2].Slices[0])
	}
	if plan.PartitionDigest == whole.PartitionDigest {
		t.Fatal("the digest does not bind the slices")
	}
	encoded, err := Encode(plan)
	if err != nil {
		t.Fatal(err)
	}
	again, err := PlanShardsSliced(files, 3, 5, slicedHistory(), allow, slices)
	if err != nil {
		t.Fatal(err)
	}
	if encodedAgain, _ := Encode(again); string(encodedAgain) != string(encoded) {
		t.Fatal("the sliced plan is not deterministic")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.json")
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	if read, err := readShardPlanFile(path); err != nil || !reflect.DeepEqual(read, plan) {
		t.Fatalf("round trip %v", err)
	}
	wholeEncoded, _ := Encode(whole)
	if strings.Contains(string(wholeEncoded), `"slices"`) {
		t.Fatal("an unsliced plan encodes a slices member")
	}
	for name, args := range map[string]struct {
		count         int
		history       []History
		allow, slices []byte
	}{
		"no allow-list":           {3, slicedHistory(), nil, slices},
		"unusable slice file":     {3, slicedHistory(), allow, []byte(`{}`)},
		"more slices than shards": {2, slicedHistory(), allow, slices},
		"fallback":                {3, nil, allow, slices},
	} {
		got, err := PlanShardsSliced(files, args.count, 5, args.history, args.allow, args.slices)
		if err != nil {
			t.Fatal(err)
		}
		want, _ := PlanShards(files, args.count, 5, args.history)
		gotEncoded, _ := Encode(got)
		wantEncoded, _ := Encode(want)
		if string(gotEncoded) != string(wantEncoded) {
			t.Errorf("%s: the plan split a package", name)
		}
	}
}

// STR-V0-057: the plan reader refuses malformed slices even under a
// recomputed digest: a wrong flag, an invalid or empty pattern, two siblings
// on one shard, a missing or duplicated slice, a package both whole and
// sliced, slices out of order, a count over the plan's, an empty member.
func TestSTRV0057ReadShardPlanSliceRefusals(t *testing.T) {
	files := []string{"example.test/m/a", "example.test/m/b", "example.test/m/c"}
	allow, slices := sliceFixtures()
	dir := t.TempDir()
	for name, mutate := range map[string]func(*ShardPlan){
		"flag":          func(p *ShardPlan) { p.Shards[0].Slices[0].Flag = "-skip" },
		"last flag":     func(p *ShardPlan) { p.Shards[2].Slices[0].Flag = "-run" },
		"pattern":       func(p *ShardPlan) { p.Shards[1].Slices[0].Pattern = "^(" },
		"empty pattern": func(p *ShardPlan) { p.Shards[1].Slices[0].Pattern = "" },
		"zero weight":   func(p *ShardPlan) { p.Shards[1].Slices[0].WeightMilliseconds = 0 },
		"sibling": func(p *ShardPlan) {
			p.Shards[0].Slices = append(p.Shards[0].Slices, p.Shards[1].Slices[0])
			p.Shards[1].Slices = nil
		},
		"missing":   func(p *ShardPlan) { p.Shards[2].Slices = nil },
		"duplicate": func(p *ShardPlan) { p.Shards[2].Slices[0] = p.Shards[1].Slices[0] },
		"whole and sliced": func(p *ShardPlan) {
			p.Shards[2].Files = []string{"example.test/m/a"}
		},
		"out of order": func(p *ShardPlan) {
			extra := p.Shards[0].Slices[0]
			extra.Package = "example.test/m/0"
			p.Shards[0].Slices = append(p.Shards[0].Slices, extra)
		},
		"count over plan": func(p *ShardPlan) {
			for i := range p.Shards {
				p.Shards[i].Slices[0].Count = 4
			}
		},
		"empty member": func(p *ShardPlan) { p.Shards[2].Slices = []PlannedSlice{} },
	} {
		plan, err := PlanShardsSliced(files, 3, 5, slicedHistory(), allow, slices)
		if err != nil {
			t.Fatal(err)
		}
		mutate(&plan)
		if plan.PartitionDigest, err = ShardPartitionDigest(plan.Count, plan.Shards); err != nil {
			t.Fatal(err)
		}
		encoded, err := Encode(plan)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, strings.ReplaceAll(name, " ", "-")+".json")
		if err := os.WriteFile(path, encoded, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := readShardPlanFile(path); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// STR-V0-057: measured placement runs each slice of a sliced package on its
// planned shard with its flag and pattern, NARROWED and FULL alike, and a
// sliced package counts as planned.
func TestSTRV0057MeasuredPlacementRunsSlices(t *testing.T) {
	allow, slices := sliceFixtures()
	plan, err := PlanShardsSliced([]string{"example.test/m/a", "example.test/m/b", "example.test/m/c"}, 3, 5, slicedHistory(), allow, slices)
	if err != nil {
		t.Fatal(err)
	}
	input := fixtureInput()
	input.Placement, input.ShardCount, input.ShardPlan = PlacementMeasured, 3, &plan
	input.GoSelection = "run example.test/m/b example.test/m/a"
	m, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"0 go:example.test/m/a 0/3", "0 go:example.test/m/b whole", "1 go:example.test/m/a 1/3", "2 go:example.test/m/a 2/3"}
	if got := unitIDs(m); !reflect.DeepEqual(got, want) || len(m.Incomplete) != 0 || m.PartitionDigest != plan.PartitionDigest {
		t.Fatalf("units %v incomplete %v", got, m.Incomplete)
	}
	if m.Units[0].Flag != "-run" || m.Units[0].Pattern != "^(TestA1)$" || m.Units[3].Flag != "-skip" || m.Units[3].Pattern != "^(TestA1|TestA2)$" {
		t.Fatalf("slice flags %+v %+v", m.Units[0], m.Units[3])
	}
}
