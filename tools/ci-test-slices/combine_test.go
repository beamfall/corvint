package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/.github/cishards"
)

// AFP-V0-049: generate and replay combine the per-slice outcomes of a package
// that the tested tree's allow-list and slice file split, and refuse every
// other repeat.

// lifecycle is one log's run of pkg: each top-level test, with a subtest, takes
// 10 ms, and the package ends after seconds.
func lifecycle(pkg string, seconds float64, tests ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "{\"Action\":\"start\",\"Package\":%q}\n", pkg)
	for _, name := range tests {
		for _, test := range []string{name, name + "/sub"} {
			fmt.Fprintf(&b, "{\"Action\":\"run\",\"Package\":%q,\"Test\":%q}\n", pkg, test)
		}
		for _, test := range []string{name + "/sub", name} {
			fmt.Fprintf(&b, "{\"Action\":\"pass\",\"Package\":%q,\"Test\":%q,\"Elapsed\":0.01}\n", pkg, test)
		}
	}
	fmt.Fprintf(&b, "{\"Action\":\"pass\",\"Package\":%q,\"Elapsed\":%g}\n", pkg, seconds)
	return b.String()
}

func sliceFixture(named ...[]string) string {
	f := map[string]any{
		"profile": "corvint-ci-test-slices/0",
		"source":  map[string]string{"revision": "0123456789abcdef0123456789abcdef01234567", "runURL": runURL, "goVersion": partitionGoVersion},
	}
	packages := map[string]any{}
	if len(named) != 0 {
		var slices []map[string]any
		for _, tests := range named {
			slices = append(slices, map[string]any{"milliseconds": 10000, "tests": tests})
		}
		packages["example.org/big"] = map[string]any{"named": slices, "restMilliseconds": 10000}
	}
	f["packages"] = packages
	raw, _ := json.Marshal(f)
	return string(raw)
}

var (
	// Two slices: A..C, and the catch-all, which runs D..F, TestG (added after
	// generation) and TestÜ (never nameable).
	twoSlices = sliceFixture([]string{"TestA", "TestB", "TestC"})
	// Three slices; the second names only tests the run no longer has.
	threeSlices = sliceFixture([]string{"TestA", "TestB"}, []string{"TestX", "TestY"})
	noSplit     = sliceFixture()
	allowSmall  = `{"profile":"corvint-ci-test-split-allow/0","packages":{"example.org/small":"fixture"}}`
	catchAll    = []string{"TestD", "TestE", "TestF", "TestG", "TestÜ"}
	everyTest   = []string{"TestA", "TestB", "TestC", "TestD", "TestE", "TestF", "TestG", "TestÜ"}
)

func logs(t *testing.T, bodies ...string) []string {
	dir := t.TempDir()
	var paths []string
	for i, body := range bodies {
		paths = append(paths, write(t, filepath.Join(dir, fmt.Sprintf("shard-%d.json", i)), body))
	}
	return paths
}

func replayIn(t *testing.T, allow, slices string, shards int, logs []string) (string, int, error) {
	root := t.TempDir()
	write(t, filepath.Join(root, allowPath), allow)
	if slices != "" {
		write(t, filepath.Join(root, slicesPath), slices)
	}
	var out bytes.Buffer
	code, err := run(context.Background(), "replay", append([]string{"--root", root, "--shards", fmt.Sprint(shards)}, logs...), &out, &bytes.Buffer{})
	return out.String(), code, err
}

// TestAFPV0049ReplayCombinesSlicedRuns replays runs in which the split package
// ran as two slices over two shards, and as three slices over three shards with
// one slice whose tests the run lacks.
func TestAFPV0049ReplayCombinesSlicedRuns(t *testing.T) {
	two := logs(t,
		lifecycle("example.org/big", 15, "TestA", "TestB", "TestC")+lifecycle("example.org/small", 5, "TestS"),
		lifecycle("example.org/big", 20, catchAll...))
	out, code, err := replayIn(t, allowBig, twoSlices, 2, two)
	if code != 0 || err != nil {
		t.Fatalf("two slices: code=%d err=%v", code, err)
	}
	// big: 35 s over 80 ms of top-level tests; A..C are 30 ms, the catch-all 50 ms.
	for _, want := range []string{
		"universe 2 packages 40.0s ideal 20.0s over 2 shards",
		"units    3 placed exactly once: 1 whole packages, 2 test slices",
		"observed shard0=20.0s shard1=20.0s max=20.0s",
		"whole    shard0=35.0s shard1=5.0s max=35.0s",
		"slice    shard=0 example.org/big#0/2 -run 3 tests predicted=13.1s",
		"slice    shard=1 example.org/big#1/2 -skip 5 tests predicted=21.9s",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("two slices: replay lacks %q:\n%s", want, out)
		}
	}

	three := logs(t,
		lifecycle("example.org/big", 10, "TestA", "TestB"),
		lifecycle("example.org/big", 1)+lifecycle("example.org/small", 5, "TestS"),
		lifecycle("example.org/big", 24, "TestC", "TestD", "TestE", "TestF", "TestG", "TestÜ"))
	out, code, err = replayIn(t, allowBig, threeSlices, 3, three)
	if code != 0 || err != nil {
		t.Fatalf("three slices: code=%d err=%v", code, err)
	}
	for _, want := range []string{
		"universe 2 packages 40.0s ideal 13.3s over 3 shards",
		"observed shard0=10.0s shard1=6.0s shard2=24.0s max=24.0s",
		"example.org/big#1/3 -run 0 tests predicted=0.0s",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("three slices: replay lacks %q:\n%s", want, out)
		}
	}
}

// TestAFPV0049GenerateRegeneratesFromASlicedRun regenerates the slice file from
// a run made with it: the package's time is the sum of its slices and its test
// weights the union of theirs.
func TestAFPV0049GenerateRegeneratesFromASlicedRun(t *testing.T) {
	root, head := repoWith(t, allowBig, twoSlices)
	fakeList(t, map[string][]string{"example.org/big": everyTest, "example.org/small": {"TestS"}})
	run2 := logs(t,
		lifecycle("example.org/big", 15, "TestA", "TestB", "TestC")+lifecycle("example.org/small", 5, "TestS"),
		lifecycle("example.org/big", 20, catchAll...))
	var notes bytes.Buffer
	code, err := run(context.Background(), "generate", append([]string{"--root", root, "--revision", head, "--run-url", runURL, "--shards", "2"}, run2...), nil, &notes)
	if code != 0 || err != nil {
		t.Fatalf("generate code=%d err=%v notes=%s", code, err, notes.String())
	}
	if !strings.Contains(notes.String(), "split example.org/big: 35000ms into 2 slices") || !strings.Contains(notes.String(), "whole example.org/small: 5000ms within the 20000ms target") {
		t.Fatalf("notes:\n%s", notes.String())
	}
	raw, _ := os.ReadFile(filepath.Join(root, slicesPath))
	var f sliceFile
	if err := json.Unmarshal(raw, &f); err != nil || len(f.Packages) != 1 {
		t.Fatalf("regenerated file: %s %v", raw, err)
	}
	// The 35 s package over the union of both slices' eight equal tests: 4|4.
	if big := f.Packages["example.org/big"]; strings.Join(big.Named[0].Tests, ",") != "TestA,TestB,TestC,TestD" || big.Named[0].Milliseconds != 17500 || big.RestMilliseconds != 17500 {
		t.Fatalf("regenerated split: %+v", big)
	}
}

// TestAFPV0049RefusesEveryOtherRepeat keeps the AFP-V0-041 (5) refusals for
// every repeat that is not one outcome per declared test slice, in both modes,
// and leaves the slice file unwritten.
func TestAFPV0049RefusesEveryOtherRepeat(t *testing.T) {
	s0 := lifecycle("example.org/big", 15, "TestA", "TestB", "TestC")
	rest := lifecycle("example.org/big", 20, catchAll...)
	small := lifecycle("example.org/small", 5, "TestS")
	other := lifecycle("example.org/other", 1)
	for name, c := range map[string]struct {
		allow, slices string
		bodies        []string
		want          string
	}{
		"unsplit package twice":             {allowBig, twoSlices, []string{s0 + small, rest + small}, "package example.org/small has two terminal outcomes"},
		"package not in the slice file":     {allowBig, noSplit, []string{s0, rest}, "package example.org/big has two terminal outcomes"},
		"package not in the allow-list":     {allowSmall, twoSlices, []string{s0, rest}, "package example.org/big has two terminal outcomes"},
		"no slice file":                     {allowBig, "", []string{s0, rest}, "package example.org/big has two terminal outcomes"},
		"same test in two slices":           {allowBig, twoSlices, []string{s0, lifecycle("example.org/big", 1, "TestA")}, "test example.org/big TestA has two terminal outcomes"},
		"whole run twice":                   {allowBig, twoSlices, []string{lifecycle("example.org/big", 35, everyTest...), lifecycle("example.org/big", 35, everyTest...)}, "test example.org/big TestD is outside test slice 0/2, which its shard log ran"},
		"test outside its log's slice":      {allowBig, twoSlices, []string{lifecycle("example.org/big", 15, "TestA", "TestB", "TestD"), lifecycle("example.org/big", 20, "TestC", "TestE")}, "test example.org/big TestD is outside test slice 0/2, which its shard log ran"},
		"named test after a catch-all test": {allowBig, twoSlices, []string{lifecycle("example.org/big", 15, "TestB"), lifecycle("example.org/big", 20, "TestA0", "TestC")}, "test example.org/big TestC is outside test slice 1/2, which its shard log ran"},
		"slice ended twice":                 {allowBig, twoSlices, []string{lifecycle("example.org/big", 5, "TestA"), lifecycle("example.org/big", 5, "TestB", "TestC")}, "test slice 0/2 of package example.org/big ended twice"},
		"missing slice":                     {allowBig, twoSlices, []string{s0 + small, other}, "package example.org/big has 1 terminal outcomes for its 2 test slices"},
		"pre-split whole run":               {allowBig, twoSlices, []string{lifecycle("example.org/big", 35, everyTest...), other}, "package example.org/big has 1 terminal outcomes for its 2 test slices"},
		"extra slice":                       {allowBig, twoSlices, []string{s0, rest, lifecycle("example.org/big", 1)}, "package example.org/big has 3 terminal outcomes for its 2 test slices"},
		"more slices than shards":           {allowBig, threeSlices, []string{lifecycle("example.org/big", 35, everyTest...), small}, "package example.org/big is split into 3 test slices, more than --shards 2"},
	} {
		shards := len(c.bodies)
		run := logs(t, c.bodies...)
		if out, code, err := replayIn(t, c.allow, c.slices, shards, run); code != 2 || err == nil || !strings.Contains(err.Error(), c.want) || out != "" {
			t.Fatalf("%s: replay code=%d err=%v, want %q", name, code, err, c.want)
		}
		if c.slices == "" {
			continue // generate needs a committed slice file to compare
		}
		root, head := repoWith(t, c.allow, c.slices)
		fakeList(t, map[string][]string{"example.org/big": everyTest, "example.org/small": {"TestS"}})
		code, err := runGenerate(root, head, shards, run)
		if code != 2 || err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s: generate code=%d err=%v, want %q", name, code, err, c.want)
		}
		if after, _ := os.ReadFile(filepath.Join(root, slicesPath)); string(after) != c.slices {
			t.Fatalf("%s: refused generate wrote the slice file", name)
		}
	}
}

func runGenerate(root, head string, shards int, logs []string) (int, error) {
	return run(context.Background(), "generate", append([]string{"--root", root, "--revision", head, "--run-url", runURL, "--shards", fmt.Sprint(shards)}, logs...), nil, &bytes.Buffer{})
}

// TestAFPV0049ReplayRefusesAPlanThatMisplacesAUnit checks the replay report's
// exactly-once line against plans that drop, repeat or add a unit.
func TestAFPV0049ReplayRefusesAPlanThatMisplacesAUnit(t *testing.T) {
	universe := []string{"a", "b"}
	sl := func(i int) cishards.Slice { return cishards.Slice{Package: "b", Index: i, Count: 2} }
	if got, err := placedOnce(universe, []cishards.Shard{{Packages: []string{"a"}, Slices: []cishards.Slice{sl(1)}}, {Slices: []cishards.Slice{sl(0)}}}); err != nil || got != "3 placed exactly once: 1 whole packages, 2 test slices" {
		t.Fatalf("good plan: %q %v", got, err)
	}
	for name, plan := range map[string][]cishards.Shard{
		"missing package":           {{Slices: []cishards.Slice{sl(0), sl(1)}}},
		"package twice":             {{Packages: []string{"a", "b"}}, {Packages: []string{"a"}}},
		"unknown package":           {{Packages: []string{"a", "b", "c"}}},
		"missing slice":             {{Packages: []string{"a"}, Slices: []cishards.Slice{sl(0)}}},
		"slice twice":               {{Packages: []string{"a"}, Slices: []cishards.Slice{sl(0), sl(1)}}, {Slices: []cishards.Slice{sl(1)}}},
		"slice and whole":           {{Packages: []string{"a", "b"}, Slices: []cishards.Slice{sl(0), sl(1)}}},
		"inconsistent slice counts": {{Packages: []string{"a"}, Slices: []cishards.Slice{sl(0)}}, {Slices: []cishards.Slice{{Package: "b", Index: 1, Count: 3}}}},
		"slice out of range":        {{Packages: []string{"a"}, Slices: []cishards.Slice{sl(0), sl(2)}}},
	} {
		if got, err := placedOnce(universe, plan); err == nil {
			t.Fatalf("%s: accepted as %q", name, got)
		}
	}
}
