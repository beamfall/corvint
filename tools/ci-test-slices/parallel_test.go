// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/.github/cishards"
)

// measuredSample uses only the 17 longest passing, unsplit package outcomes in
// run 42106672783. This is a placement regression sample, not a refresh-qualified
// full run: that run also had a failed tasks/store slice. The two synthetic logs
// below exercise replay admission without presenting them as original CI logs.
func measuredSample(t *testing.T) ([]string, []byte, map[string]int64, []string) {
	t.Helper()
	raw, err := os.ReadFile("testdata/run-42106672783-costs.json")
	if err != nil {
		t.Fatal(err)
	}
	costs, ok := cishards.Costs(raw)
	if !ok {
		t.Fatal("invalid measured cost fixture")
	}
	u := make([]string, 0, len(costs))
	for p := range costs {
		u = append(u, p)
	}
	sort.Strings(u)
	var streams [2]strings.Builder
	for i, p := range u {
		fmt.Fprintf(&streams[i%2], "{\"Action\":\"start\",\"Package\":%q}\n{\"Action\":\"pass\",\"Package\":%q,\"Elapsed\":%.3f}\n", p, p, float64(costs[p])/1000)
	}
	logs := []string{
		write(t, filepath.Join(t.TempDir(), "sample-0.log"), streams[0].String()),
		write(t, filepath.Join(t.TempDir(), "sample-1.log"), streams[1].String()),
	}
	return u, raw, costs, logs
}

func TestCINV07620ReplayParallelPlacement(t *testing.T) {
	u, raw, costs, logs := measuredSample(t)
	root := t.TempDir()
	costsFile := write(t, filepath.Join(root, "costs.json"), string(raw))
	write(t, filepath.Join(root, allowPath), "{}\n")
	write(t, filepath.Join(root, slicesPath), "{}\n")
	obs := &observation{packages: costs}
	serial, err := cishards.PlanFrom(u, 2, raw, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	parallel, err := cishards.PlanFromParallel(u, 2, 4, raw, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Compare both placements under the SAME p=4 model and common overhead.
	// Serial LPT: 729.270s; parallel-aware placement: 723.828s.
	before := slices.Max(predict(obs, serial, 4, 360000))
	after := slices.Max(predict(obs, parallel, 4, 360000))
	if before != 729270 || after != 723828 || after >= before {
		t.Fatalf("measured sample before=%d after=%d", before, after)
	}
	var out, notes bytes.Buffer
	args := append([]string{"--root", root, "--costs", costsFile, "--shards", "2", "--parallel", "4", "--compile-link", "6m"}, logs...)
	if code, err := run(context.Background(), "replay", args, &out, &notes); code != 0 || err != nil {
		t.Fatalf("parallel replay: code=%d err=%v notes=%s", code, err, &notes)
	}
	for _, want := range []string{"model    parallel=4 compile-link=360.000s", "whole    shard0=713.9s shard1=723.8s max=723.8s", "sliced   shard0=713.9s shard1=723.8s max=723.8s"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("replay lacks %q:\n%s", want, &out)
		}
	}
	// A real replay slice file exercises placement and prediction together.
	write(t, filepath.Join(root, allowPath), allowBig)
	candidate := write(t, filepath.Join(root, "candidate.json"), `{"profile":"corvint-ci-test-slices/0","source":{"revision":"0123456789abcdef0123456789abcdef01234567","runURL":"`+runURL+`","goVersion":"go1.27.1"},"packages":{"example.org/big":{"named":[{"milliseconds":17500,"tests":["TestA","TestB","TestC"]}],"restMilliseconds":17500}}}`)
	out.Reset()
	args = append([]string{"--root", root, "--shards", "2", "--slices", candidate, "--parallel", "4", "--compile-link", "6m"}, stream(t)...)
	if code, err := run(context.Background(), "replay", args, &out, &notes); code != 0 || err != nil || !strings.Contains(out.String(), "max=380.0s") {
		t.Fatalf("sliced parallel replay: code=%d err=%v\n%s", code, err, &out)
	}
	// Exercise slices as well as whole packages, both long-pole and sum/p
	// branches, upward rounding, and the explicit compile/link allowance.
	slicedObs := &observation{packages: map[string]int64{"example.org/a": 1201, "example.org/b": 1000}}
	plan := []cishards.Shard{{Packages: []string{"example.org/b"}, Slices: []cishards.Slice{
		{Package: "example.org/a", Index: 0, Count: 2, Flag: "-run", Pattern: "^(TestA)$"},
	}}}
	if got := predict(slicedObs, plan, 4, 7); !reflect.DeepEqual(got, []int64{1007}) {
		t.Fatalf("slice long pole: %v", got)
	}
	plan = []cishards.Shard{{Packages: []string{"a", "b", "c", "d", "e"}}}
	if got := predict(&observation{packages: map[string]int64{"a": 1, "b": 1, "c": 1, "d": 1, "e": 1}}, plan, 4, 7); !reflect.DeepEqual(got, []int64{9}) {
		t.Fatalf("sum/p rounding: %v", got)
	}
}

func TestCINV07620SerialGoldenAndDeterminism(t *testing.T) {
	u, raw, _, logs := measuredSample(t)
	want, err := os.ReadFile("testdata/run-42106672783-p1.golden")
	if err != nil {
		t.Fatal(err)
	}
	serial, err := cishards.PlanFrom(u, 2, raw, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var golden strings.Builder
	for i, s := range serial {
		fmt.Fprintf(&golden, "%d\t%s\n", i, strings.Join(s.Packages, " "))
	}
	if golden.String() != string(want) {
		t.Fatalf("p=1 legacy placement differs:\n%s", &golden)
	}
	for _, p := range []int{1, 4} {
		first, err := cishards.PlanFromParallel(u, 2, p, raw, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if p == 1 && !reflect.DeepEqual(first, serial) {
			t.Fatal("parallel=1 differs from legacy placement")
		}
		for range 3 {
			slices.Reverse(u)
			got, err := cishards.PlanFromParallel(u, 2, p, raw, nil, nil)
			selected, selErr := cishards.SelectFromParallel(append(slices.Clone(u), "example.org/unselected"), u, 2, p, raw, nil, nil)
			if err != nil || selErr != nil || !reflect.DeepEqual(got, first) || !reflect.DeepEqual(selected, first) {
				t.Fatalf("Plan/Select determinism p=%d: %v %v", p, err, selErr)
			}
		}
	}
	root := t.TempDir()
	costsFile := write(t, filepath.Join(root, "costs.json"), string(raw))
	write(t, filepath.Join(root, allowPath), "{}\n")
	write(t, filepath.Join(root, slicesPath), "{}\n")
	var defaultOut, explicitOut, notes bytes.Buffer
	args := append([]string{"--root", root, "--costs", costsFile, "--shards", "2"}, logs...)
	if code, err := run(context.Background(), "replay", args, &defaultOut, &notes); code != 0 || err != nil {
		t.Fatalf("default replay: %d %v", code, err)
	}
	args = append([]string{"--root", root, "--costs", costsFile, "--shards", "2", "--parallel", "1", "--compile-link", "0s"}, logs...)
	if code, err := run(context.Background(), "replay", args, &explicitOut, &notes); code != 0 || err != nil || !bytes.Equal(defaultOut.Bytes(), explicitOut.Bytes()) {
		t.Fatalf("explicit p=1 differs: %d %v\n%s\n%s", code, err, &defaultOut, &explicitOut)
	}
	wantReplay := "universe 17 packages 2870.9s ideal 1435.5s over 2 shards\nunits    17 placed exactly once: 17 whole packages, 0 test slices\nobserved shard0=1398.0s shard1=1472.9s max=1472.9s\nwhole    shard0=1477.1s shard1=1393.8s max=1477.1s\nsliced   shard0=1477.1s shard1=1393.8s max=1477.1s\n"
	if defaultOut.String() != wantReplay {
		t.Fatalf("default output changed:\n%s", &defaultOut)
	}
}

func TestCINV07620ReplayOptions(t *testing.T) {
	for _, mode := range []string{"replay", "generate"} {
		for _, flags := range [][]string{{"--parallel", "0"}, {"--parallel", "65"}, {"--compile-link", "-1ns"}} {
			code, err := run(context.Background(), mode, append(flags, "missing.log"), &bytes.Buffer{}, &bytes.Buffer{})
			if code != 2 || err == nil || err.Error() != "usage: ci-test-slices generate|replay [flags] LOG..." {
				t.Fatalf("%s %v: code=%d err=%v", mode, flags, code, err)
			}
		}
	}
	for _, flags := range [][]string{{"--parallel", "4"}, {"--compile-link", "1ns"}} {
		if code, err := run(context.Background(), "generate", append(flags, "missing.log"), &bytes.Buffer{}, &bytes.Buffer{}); code != 2 || err == nil || err.Error() != "usage: ci-test-slices generate|replay [flags] LOG..." {
			t.Fatalf("generate accepted replay options: %d %v", code, err)
		}
	}
}
