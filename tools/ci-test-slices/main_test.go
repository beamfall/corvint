package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/.github/cishards"
)

const runURL = "https://github.com/beamfall/corvint/actions/runs/1"

func write(t *testing.T, path, body string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// stream is one shard's hosted log: package big has tests A..F, with F twice as
// slow; packages small and other are whole.
func stream(t *testing.T) string {
	var b strings.Builder
	b.WriteString("shard (0)\tGo tests\t2026-10-10T13:20:59Z not json {\n")
	for i, name := range []string{"TestA", "TestB", "TestC", "TestD", "TestE", "TestF"} {
		ms := 10.0
		if i == 5 {
			ms = 20
		}
		fmt.Fprintf(&b, "2026-10-10T13:20:59Z {\"Action\":\"pass\",\"Package\":\"example.org/big\",\"Test\":%q,\"Elapsed\":%g}\n", name+"/sub", ms)
		fmt.Fprintf(&b, "2026-10-10T13:20:59Z {\"Action\":\"pass\",\"Package\":\"example.org/big\",\"Test\":%q,\"Elapsed\":%g}\n", name, ms)
	}
	b.WriteString(`{"Action":"pass","Package":"example.org/big","Elapsed":35}
{"Action":"pass","Package":"example.org/small","Test":"TestS","Elapsed":5}
{"Action":"pass","Package":"example.org/small","Elapsed":5}
{"Action":"skip","Package":"example.org/other","Elapsed":0}
`)
	return write(t, filepath.Join(t.TempDir(), "shard0.log"), b.String())
}

func repo(t *testing.T, allow string) (string, string) {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, allowPath), allow)
	write(t, filepath.Join(root, slicesPath), "{}\n")
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@example.org", "commit", "-q", "-m", "fixture"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	head, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return root, strings.TrimSpace(string(head))
}

func fakeList(t *testing.T, lists map[string][]string) {
	t.Helper()
	old := enumerate
	enumerate = func(_ context.Context, _ string, pkg string) ([]string, error) {
		names, ok := lists[pkg]
		if !ok {
			return nil, errors.New("build failed")
		}
		return names, nil
	}
	t.Cleanup(func() { enumerate = old })
}

const allowBig = `{"profile":"corvint-ci-test-split-allow/0","packages":{"example.org/big":"fixture","example.org/small":"fixture","example.org/broken":"fixture"}}`

func TestAFPV0041GenerateSplitsOnlySlowListablePackages(t *testing.T) {
	root, head := repo(t, allowBig)
	fakeList(t, map[string][]string{
		// TestG is new since the run: it takes the median. TestÜ cannot be named.
		"example.org/big":   {"TestA", "TestB", "TestC", "TestD", "TestE", "TestF", "TestG", "TestÜ", "ExampleBig"},
		"example.org/small": {"TestS"},
	})
	var notes bytes.Buffer
	log := stream(t)
	// Ideal share 40/2 = 20 s: big (35 s) splits in two; small stays whole.
	code, err := run(context.Background(), "generate", []string{"--root", root, "--revision", head, "--run-url", runURL, "--shards", "2", log}, nil, &notes)
	if code != 0 || err != nil {
		t.Fatalf("generate code=%d err=%v notes=%s", code, err, notes.String())
	}
	raw, err := os.ReadFile(filepath.Join(root, slicesPath))
	if err != nil {
		t.Fatal(err)
	}
	var f sliceFile
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Packages) != 1 || f.Source.Revision != head || f.Source.RunURL != runURL {
		t.Fatalf("unexpected file: %s", raw)
	}
	big := f.Packages["example.org/big"]
	// Sorted costs 10 (ExampleBig, median) 10 10 10 10 10 20 10 (TestG, median):
	// the earliest cut that minimises the larger slice is 40|50.
	if !reflect.DeepEqual(big.Named, []namedSlice{{Milliseconds: 15556, Tests: []string{"ExampleBig", "TestA", "TestB", "TestC"}}}) || big.RestMilliseconds != 19444 {
		t.Fatalf("unexpected split: %+v", big)
	}
	for _, want := range []string{"split example.org/big: 35000ms into 2 slices", "whole example.org/small: 5000ms within the 20000ms target", "whole example.org/broken: no observed outcome"} {
		if !strings.Contains(notes.String(), want) {
			t.Fatalf("notes lack %q:\n%s", want, notes.String())
		}
	}
	if got := cishards.SplitPackages([]string{"example.org/big"}, 2, []byte(allowBig), raw); len(got["example.org/big"]) != 2 {
		t.Fatal("generated file not admitted")
	}

	// An enumeration failure keeps the package whole.
	root, head = repo(t, allowBig)
	fakeList(t, map[string][]string{})
	notes.Reset()
	if code, err = run(context.Background(), "generate", []string{"--root", root, "--revision", head, "--run-url", runURL, "--shards", "2", log}, nil, &notes); code != 0 || err != nil {
		t.Fatalf("generate code=%d err=%v", code, err)
	}
	if raw, _ = os.ReadFile(filepath.Join(root, slicesPath)); !bytes.Contains(raw, []byte(`"packages": {}`)) || !strings.Contains(notes.String(), "whole example.org/big: build failed") {
		t.Fatalf("enumeration failure split the package: %s %s", raw, notes.String())
	}
}

func TestAFPV0041GenerateRefusesUnboundInputs(t *testing.T) {
	log := stream(t)
	fakeList(t, map[string][]string{"example.org/big": {"TestA", "TestB"}})
	root, head := repo(t, allowBig)
	for name, args := range map[string][]string{
		"other revision": {"--revision", strings.Repeat("0", 40)},
		"bad run URL":    {"--revision", head, "--run-url", "https://example.org/run"},
	} {
		a := append([]string{"--root", root, "--shards", "2", "--run-url", runURL}, args...)
		if code, err := run(context.Background(), "generate", append(a, log), nil, &bytes.Buffer{}); code != 2 || err == nil {
			t.Fatalf("%s: code=%d err=%v", name, code, err)
		}
	}
	write(t, filepath.Join(root, allowPath), allowBig+"\n")
	if code, err := run(context.Background(), "generate", []string{"--root", root, "--revision", head, "--run-url", runURL, log}, nil, &bytes.Buffer{}); code != 2 || err == nil || !strings.Contains(err.Error(), "tracked files differ") {
		t.Fatalf("dirty tree: code=%d err=%v", code, err)
	}
	failed := write(t, filepath.Join(t.TempDir(), "failed.log"), `{"Action":"fail","Package":"example.org/big","Test":"TestA","Elapsed":1}`+"\n")
	twice := write(t, filepath.Join(t.TempDir(), "twice.log"), `{"Action":"pass","Package":"example.org/big","Elapsed":1}`+"\n")
	for _, logs := range [][]string{{failed}, {twice, twice}} {
		if code, err := run(context.Background(), "replay", logs, nil, &bytes.Buffer{}); code != 2 || err == nil {
			t.Fatalf("unusable run accepted: %v", logs)
		}
	}
}

func TestAFPV0041ListParsing(t *testing.T) {
	ok := "TestA\nBenchmarkB\nExampleC\nFuzzD\nTestÜ\nok  \texample.org/p\t0.1s\n"
	names, err := parseList([]byte(ok), "example.org/p")
	if err != nil || !reflect.DeepEqual(names, []string{"ExampleC", "FuzzD", "TestA", "TestÜ"}) {
		t.Fatalf("names=%v err=%v", names, err)
	}
	for _, bad := range []string{"TestA\n", "TestA\nsetup noise\nok  \texample.org/p\t0.1s\n", "ok  \texample.org/p\t0.1s\nTestA\n"} {
		if _, err := parseList([]byte(bad), "example.org/p"); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func TestAFPV0041SliceWithoutTimesSplitsByCount(t *testing.T) {
	s, ok := slice([]string{"TestA", "TestB", "TestC", "TestD", "TestE", "TestF"}, nil, 60000, 3)
	if !ok || len(s.Named) != 2 || len(s.Named[0].Tests) != 2 || len(s.Named[1].Tests) != 2 || s.Named[0].Milliseconds != 20000 || s.RestMilliseconds != 20000 {
		t.Fatalf("uneven count split: %+v", s)
	}
	if _, ok = slice([]string{"TestA", "TestÜ"}, nil, 1, 3); ok {
		t.Fatal("split with fewer nameable tests than named slices")
	}
}

func TestAFPV0041MinMaxCutsIsOptimal(t *testing.T) {
	for _, c := range []struct {
		costs []int64
		k     int
	}{{[]int64{5, 1, 1, 1, 5, 0}, 2}, {[]int64{9, 1, 1, 1, 1, 1, 1, 3}, 3}, {[]int64{1, 1, 1, 1}, 4}, {[]int64{3, 0}, 2}} {
		cuts := minMaxCuts(c.costs, c.k)
		if len(cuts) != c.k || cuts[0] != 0 {
			t.Fatalf("cuts %v", cuts)
		}
		if got, want := largest(c.costs, cuts), brute(c.costs, c.k); got != want {
			t.Fatalf("%v k=%d: largest %d, optimum %d", c.costs, c.k, got, want)
		}
	}
}

func largest(costs []int64, cuts []int) int64 {
	var top int64
	for i := range cuts {
		end := len(costs)
		if i+1 < len(cuts) {
			end = cuts[i+1]
		}
		var sum int64
		for _, c := range costs[cuts[i]:end] {
			sum += c
		}
		top = max(top, sum)
	}
	return top
}

func brute(costs []int64, k int) int64 {
	if k == 1 {
		var sum int64
		for _, c := range costs {
			sum += c
		}
		return sum
	}
	best := int64(1 << 62)
	for i := 1; i <= len(costs)-(k-1); i++ {
		var head int64
		for _, c := range costs[:i] {
			head += c
		}
		best = min(best, max(head, brute(costs[i:], k-1)))
	}
	return best
}

func TestAFPV0041ReplayPredictsShardSums(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, allowPath), allowBig)
	slices := `{"profile":"corvint-ci-test-slices/0","source":{"revision":"0123456789abcdef0123456789abcdef01234567","runURL":"` + runURL + `","goVersion":"go1.27.1"},
"packages":{"example.org/big":{"named":[{"milliseconds":17500,"tests":["TestA","TestB","TestC"]}],"restMilliseconds":17500}}}`
	write(t, filepath.Join(root, slicesPath), slices)
	var out bytes.Buffer
	if code, err := run(context.Background(), "replay", []string{"--root", root, "--shards", "2", stream(t)}, &out, &bytes.Buffer{}); code != 0 || err != nil {
		t.Fatalf("replay code=%d err=%v", code, err)
	}
	// big: 35 s over 70 ms of top-level tests; A..C are 30 ms (15 s), D..F 40 ms (20 s).
	for _, want := range []string{
		"universe 3 packages 40.0s ideal 20.0s over 2 shards",
		"whole    shard0=35.0s shard1=5.0s max=35.0s",
		"sliced   shard0=20.0s shard1=20.0s max=20.0s",
		"slice    shard=0 example.org/big#0/2 -run 3 tests predicted=15.0s planned=17.5s",
		"slice    shard=1 example.org/big#1/2 -skip 3 tests predicted=20.0s planned=17.5s",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("replay lacks %q:\n%s", want, out.String())
		}
	}
}
