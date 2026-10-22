package main

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// runs reports whether a shard's selection runs a top-level test name, as
// `go test` applies -run and -skip to it.
func runs(t *testing.T, flag, pattern, name string) bool {
	t.Helper()
	re, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatalf("pattern %q: %v", pattern, err)
	}
	if flag == "-skip" {
		return !re.MatchString(name)
	}
	return re.MatchString(name)
}

func TestASDV0026_EveryTestRunsInExactlyOneShard(t *testing.T) {
	var names []string
	for i := 0; i < 600; i++ {
		names = append(names, fmt.Sprintf("TestCase%03d_Name", i))
	}
	names = append(names, "ExampleThing", "FuzzParse")
	unlisted := []string{"TestAddedAfterEnumeration", "TestCase", "TestCase000", "TestCase000_NameX"}
	for _, n := range []int{2, 3, 5, 8, 16, MaxShards} {
		count := map[string]int{}
		last := map[string]int{}
		for i := 0; i < n; i++ {
			flag, pattern, err := Selection(names, n, i)
			if err != nil {
				t.Fatalf("n=%d i=%d: %v", n, i, err)
			}
			if want := "-run"; i == n-1 {
				want = "-skip"
				if flag != want {
					t.Fatalf("n=%d catch-all flag %q", n, flag)
				}
			} else if flag != want {
				t.Fatalf("n=%d i=%d flag %q", n, i, flag)
			}
			for _, name := range append(append([]string{}, names...), unlisted...) {
				if runs(t, flag, pattern, name) {
					count[name]++
					last[name] = i
				}
			}
		}
		for _, name := range names {
			if count[name] != 1 {
				t.Fatalf("n=%d: %s runs in %d shards", n, name, count[name])
			}
		}
		// A test the enumeration did not see, including a prefix or extension of
		// a listed name, runs exactly once: in the catch-all.
		for _, name := range unlisted {
			if count[name] != 1 || last[name] != n-1 {
				t.Fatalf("n=%d: unlisted %s runs %d times, last in shard %d", n, name, count[name], last[name])
			}
		}
	}
}

func TestASDV0026_PlacementIsStableWhenTestsAreAdded(t *testing.T) {
	before := []string{"TestA", "TestB", "TestC", "TestD"}
	after := append([]string{"TestAA", "TestZ"}, before...)
	for i := 0; i < 3; i++ {
		_, p1, _ := Selection(before, 4, i)
		_, p2, _ := Selection(after, 4, i)
		for _, name := range before {
			if regexp.MustCompile(p1).MatchString(name) != regexp.MustCompile(p2).MatchString(name) {
				t.Fatalf("shard %d: adding tests moved %s", i, name)
			}
		}
	}
}

func TestASDV0026_NamesParsesGoTestList(t *testing.T) {
	listing := []byte("TestB\nTestA\nExampleX\nFuzzY\nBenchmarkZ\nTestA\nok  \tgithub.com/x/y\t0.123s\n")
	got, err := Names(listing)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "ExampleX,FuzzY,TestA,TestB" {
		t.Fatalf("names %v", got)
	}
	if _, err := Names([]byte("TestBad-Name\n")); err == nil {
		t.Fatal("an uncarriable name was accepted")
	}
}

func TestASDV0026_RefusesBadArgumentsAndEmptyPackages(t *testing.T) {
	for _, c := range [][2]int{{1, 0}, {MaxShards + 1, 0}, {4, 4}, {4, -1}} {
		if _, _, err := Selection([]string{"TestA"}, c[0], c[1]); err == nil {
			t.Fatalf("shards=%d shard=%d accepted", c[0], c[1])
		}
	}
	var out bytes.Buffer
	if code := emit(&out, []byte("ok  \tpkg\t0.1s\n"), 2, 0); code == 0 {
		t.Fatal("a package with no tests was accepted")
	}
	// An empty named shard runs nothing; an empty catch-all skips nothing.
	flag, pattern, err := Selection(nil, 2, 0)
	if err != nil || flag != "-run" || pattern != "^$" || runs(t, flag, pattern, "TestA") {
		t.Fatalf("empty named shard: %q %q %v", flag, pattern, err)
	}
	flag, pattern, _ = Selection(nil, 2, 1)
	if !runs(t, flag, pattern, "TestA") {
		t.Fatal("empty catch-all skipped a test")
	}
	long := make([]string, 0, 5000)
	for i := 0; i < 5000; i++ {
		long = append(long, fmt.Sprintf("TestAVeryLongTestNameNumber%05d", i))
	}
	if _, _, err := Selection(long, 2, 1); err == nil {
		t.Fatal("an over-long pattern was accepted")
	}
	out.Reset()
	if code := emit(&out, []byte("TestA\nTestB\n"), 2, 1); code != 0 || !strings.HasPrefix(out.String(), "-skip ") {
		t.Fatalf("emit: %d %q", code, out.String())
	}
}
