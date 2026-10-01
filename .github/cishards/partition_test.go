package cishards

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestAFPCompleteBalancedPartition(t *testing.T) {
	universe := []string{"example.org/a", "example.org/b", "example.org/c", "example.org/new"}
	for _, raw := range [][]byte{defaultCosts, nil, []byte(`{"profile":"bad"}`)} {
		parts, err := partition(universe, 4, raw)
		if err != nil {
			t.Fatal(err)
		}
		all := []string{}
		seen := map[string]bool{}
		for _, s := range parts {
			for _, p := range s {
				if seen[p] {
					t.Fatalf("duplicate %s", p)
				}
				seen[p] = true
				all = append(all, p)
			}
		}
		sort.Strings(all)
		if !reflect.DeepEqual(all, universe) {
			t.Fatalf("lost package: %v", all)
		}
		reversed := append([]string(nil), universe...)
		sort.Sort(sort.Reverse(sort.StringSlice(reversed)))
		again, err := partition(reversed, 4, raw)
		if err != nil || !reflect.DeepEqual(parts, again) {
			t.Fatal("input order changes partition")
		}
	}
}
func TestAFPMixedAdmissionPreservesSelectedUnion(t *testing.T) {
	universe := []string{"example.org/a", "example.org/b", "example.org/c", "example.org/d", "example.org/e", "example.org/f"}
	selected := []string{"example.org/a", "example.org/c", "example.org/f"}
	for mask := 0; mask < 16; mask++ {
		seen := map[string]bool{}
		for shard := 0; shard < 4; shard++ {
			s := selected
			if mask&(1<<shard) != 0 {
				s = []string{"./..."}
			}
			actual, err := Intersect(universe, s, shard, 4)
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range actual {
				if seen[p] {
					t.Fatalf("duplicate %s in mixed execution", p)
				}
				seen[p] = true
			}
		}
		for _, p := range selected {
			if !seen[p] {
				t.Fatalf("mask %d omitted selected package %s", mask, p)
			}
		}
	}
}
func TestAFPPartitionRefusesInvalidUniverse(t *testing.T) {
	for _, input := range []string{"", "-flag", "example.org/a example.org/a", "example.org/../a", "/absolute", "example.org/a\x00x", strings.Repeat("x", MaxInputBytes+1)} {
		if _, err := Packages([]byte(input)); err == nil {
			t.Fatalf("accepted invalid universe %.40q", input)
		}
	}
	for _, n := range []int{0, 65} {
		if _, err := Partition([]string{"example.org/a"}, n); err == nil {
			t.Fatal("bad count accepted")
		}
	}
	if _, err := Intersect([]string{"example.org/a"}, []string{"example.org/missing"}, 0, 4); err == nil {
		t.Fatal("missing selected package accepted")
	}
	empty, err := Intersect([]string{"example.org/a"}, []string{"./..."}, 3, 4)
	if err != nil || len(empty) != 0 {
		t.Fatal("empty shard wrong")
	}
}
func TestAFPInvalidCostFallback(t *testing.T) {
	var e estimates
	if json.Unmarshal(defaultCosts, &e) != nil {
		t.Fatal("bad committed estimates")
	}
	e.Source.GoVersion = "go1.0"
	raw, _ := json.Marshal(e)
	p, err := partition([]string{"example.org/b", "example.org/a"}, 2, raw)
	if err != nil || !reflect.DeepEqual(p, [][]string{{"example.org/a"}, {"example.org/b"}}) {
		t.Fatal("stale costs did not use complete fallback")
	}
	if len(ProfileDigest()) != 64 {
		t.Fatal("profile digest invalid")
	}
}

func TestAFPMeasuredCostsBalanceAndUnknownPackages(t *testing.T) {
	raw := []byte(`{"profile":"corvint-ci-package-costs/0","source":{"revision":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","runURL":"https://github.com/beamfall/corvint/actions/runs/36878582999","goVersion":"go1.27.1"},"milliseconds":{"example.org/a":9000,"example.org/b":8000,"example.org/c":2000,"example.org/d":1000}}`)
	parts, err := partition([]string{"example.org/d", "example.org/b", "example.org/a", "example.org/c"}, 2, raw)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(parts, [][]string{{"example.org/a", "example.org/d"}, {"example.org/b", "example.org/c"}}) {
		t.Fatalf("unbalanced partition: %v", parts)
	}
	parts, err = partition([]string{"example.org/a", "example.org/new"}, 2, raw)
	if err != nil || len(parts[0]) != 1 || len(parts[1]) != 1 {
		t.Fatalf("new package was lost: %v %v", parts, err)
	}
}

func TestAFPIsolatedBuildIgnoresModuleRedirection(t *testing.T) {
	t.Run("AFP-V0-022", func(t *testing.T) {
		source := t.TempDir()
		isolated := filepath.Join(source, "protected-helper")
		if err := os.MkdirAll(filepath.Join(isolated, "cmd"), 0700); err != nil {
			t.Fatal(err)
		}
		// A tested module may redirect this import, but it never enters the isolated build.
		poisoned := "module example.org/poison\n\ngo 1.27.1\n\nrequire github.com/Beamfall/corvint/.github/cishards v0.0.0\nreplace github.com/Beamfall/corvint/.github/cishards => ./malicious\n"
		if err := os.WriteFile(filepath.Join(source, "go.mod"), []byte(poisoned), 0600); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"partition.go", "cmd/main.go", "package-costs.json"} {
			raw, err := profileFiles.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(isolated, name), raw, 0600); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(isolated, "go.mod"), []byte("module github.com/Beamfall/corvint/.github/cishards\n\ngo 1.27.1\n"), 0600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-o", filepath.Join(isolated, "helper"), "./cmd")
		cmd.Dir = isolated
		replace := map[string]string{"GOENV": "off", "GOWORK": "off", "GOTOOLCHAIN": "local", "GOPROXY": "off", "GOSUMDB": "off", "GOFLAGS": ""}
		for _, value := range os.Environ() {
			if _, ok := replace[strings.SplitN(value, "=", 2)[0]]; !ok {
				cmd.Env = append(cmd.Env, value)
			}
		}
		for key, value := range replace {
			cmd.Env = append(cmd.Env, key+"="+value)
		}
		if raw, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("isolated build: %v %s", err, raw)
		}
		raw, err := exec.Command(filepath.Join(isolated, "helper"), "--profile").CombinedOutput()
		if err != nil || strings.TrimSpace(string(raw)) != ProfileDigest() {
			t.Fatalf("protected profile redirected: %s %v", raw, err)
		}
	})
}
