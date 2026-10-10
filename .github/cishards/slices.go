package cishards

import (
	"bytes"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"sort"
	"strings"
)

// AFP-V0-041 (proposed, experimental): test-level slices of owner-allowed packages.
// Slice placement is advisory; every test still runs in exactly one slice.

//go:embed test-split-allow.json
var defaultAllow []byte

//go:embed test-slices.json
var defaultSlices []byte

// MaxSlices bounds the slices of one package, the catch-all included.
const MaxSlices = 16

// MaxPatternBytes keeps each -run or -skip argument far below Linux's 128 KiB
// per-argument limit (MAX_ARG_STRLEN). A longer pattern keeps its package whole.
const MaxPatternBytes = 64 << 10

const maxSliceFileBytes = 1 << 20

// Only plain ASCII top-level names enter a pattern: they need no regexp quoting,
// contain neither of the `/` and `|` separators that `go test` splits patterns on,
// and cannot be read as an option. Benchmarks never run without -bench.
var testName = regexp.MustCompile(`^(Test|Example|Fuzz)[A-Za-z0-9_]*$`)

// Slice is one `go test` invocation over part of a split package. Slices
// 0..Count-2 run their named top-level tests with an anchored -run pattern.
// Slice Count-1 is the catch-all: it runs the package with -skip over the union
// of those names, so every test the package has when it runs, including one
// added or renamed after the slice file was generated, runs in exactly one slice.
// A pattern names top-level tests only, so subtests always run with their parent.
type Slice struct {
	Package      string
	Index        int
	Count        int
	Flag         string
	Pattern      string
	Milliseconds int64
}

// Shard is one shard of a Plan: whole packages and test slices.
type Shard struct {
	Packages []string
	Slices   []Slice
}

type allowList struct {
	Profile  string            `json:"profile"`
	Packages map[string]string `json:"packages"`
}

type sliceFile struct {
	Profile string `json:"profile"`
	Source  struct {
		Revision  string `json:"revision"`
		RunURL    string `json:"runURL"`
		GoVersion string `json:"goVersion"`
	} `json:"source"`
	Packages map[string]struct {
		Named []struct {
			Milliseconds int64    `json:"milliseconds"`
			Tests        []string `json:"tests"`
		} `json:"named"`
		RestMilliseconds int64 `json:"restMilliseconds"`
	} `json:"packages"`
}

func strict(raw []byte, v any) bool {
	if len(raw) > maxSliceFileBytes {
		return false
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	return d.Decode(v) == nil && d.Decode(new(any)) == io.EOF
}

// Allowed returns the owner's split allow-list, or false when it is unusable.
// An unusable allow-list splits no package.
func Allowed(raw []byte) (map[string]string, bool) {
	var a allowList
	if !strict(raw, &a) || a.Profile != "corvint-ci-test-split-allow/0" || len(a.Packages) > MaxPackages {
		return nil, false
	}
	for p, reason := range a.Packages {
		if _, err := validate([]string{p}); err != nil || strings.TrimSpace(reason) == "" || len(reason) > 2000 {
			return nil, false
		}
	}
	return a.Packages, true
}

// SplitPackages returns the slices of each package in packages that the allow-list
// admits and the slice file splits validly into at most total slices. Any unusable
// file splits nothing; any unusable package entry leaves that package whole.
func SplitPackages(packages []string, total int, allowRaw, slicesRaw []byte) map[string][]Slice {
	allowed, ok := Allowed(allowRaw)
	var f sliceFile
	if !ok || total < 2 || !strict(slicesRaw, &f) || f.Profile != "corvint-ci-test-slices/0" || f.Source.GoVersion != "go1.27.1" || len(f.Source.Revision) != 40 || !strings.HasPrefix(f.Source.RunURL, "https://github.com/beamfall/corvint/actions/runs/") {
		return nil
	}
	if _, err := hex.DecodeString(f.Source.Revision); err != nil {
		return nil
	}
	universe := map[string]bool{}
	for _, p := range packages {
		universe[p] = true
	}
	out := map[string][]Slice{}
	for p, entry := range f.Packages {
		count := len(entry.Named) + 1
		if !universe[p] || allowed[p] == "" || count < 2 || count > MaxSlices || count > total || !cost(entry.RestMilliseconds) {
			continue
		}
		seen := map[string]bool{}
		slices := make([]Slice, 0, count)
		for i, named := range entry.Named {
			if !cost(named.Milliseconds) || !names(named.Tests, seen) {
				slices = nil
				break
			}
			slices = append(slices, Slice{Package: p, Index: i, Count: count, Flag: "-run", Pattern: anchored(named.Tests), Milliseconds: named.Milliseconds})
		}
		if slices == nil {
			continue
		}
		union := make([]string, 0, len(seen))
		for name := range seen {
			union = append(union, name)
		}
		sort.Strings(union)
		slices = append(slices, Slice{Package: p, Index: count - 1, Count: count, Flag: "-skip", Pattern: anchored(union), Milliseconds: entry.RestMilliseconds})
		if !compiled(slices) {
			continue
		}
		out[p] = slices
	}
	return out
}

// compiled bounds every pattern; the catch-all, the union of the named
// slices, is the longest.
func compiled(slices []Slice) bool {
	for _, s := range slices {
		if len(s.Pattern) > MaxPatternBytes {
			return false
		}
		if _, err := regexp.Compile(s.Pattern); err != nil {
			return false
		}
	}
	return true
}

func cost(ms int64) bool { return ms > 0 && ms <= 3000000 }

// names admits one named slice: non-empty, strictly ascending, valid, and
// disjoint from every earlier slice of the package.
func names(tests []string, seen map[string]bool) bool {
	if len(tests) == 0 {
		return false
	}
	for i, name := range tests {
		if !testName.MatchString(name) || seen[name] || (i > 0 && tests[i-1] >= name) {
			return false
		}
		seen[name] = true
	}
	return true
}

// anchored is one pattern element: `go test` splits patterns on `/` and on `|`
// outside parentheses, so the alternation is grouped and matches a whole
// top-level name.
func anchored(tests []string) string {
	return "^(" + strings.Join(tests, "|") + ")$"
}

// Splits reports whether Plan runs any package of the universe as test slices.
// The package-level driver refuses sharded execution while it does.
func Splits(packages []string, total int) bool {
	if _, _, ok := weights(defaultCosts); !ok {
		return false
	}
	return len(SplitPackages(packages, total, defaultAllow, defaultSlices)) != 0
}

// Plan places every package of the universe, whole or as test slices, in exactly
// one shard. Without a split package it equals Partition.
func Plan(packages []string, total int) ([]Shard, error) {
	return PlanFrom(packages, total, defaultCosts, defaultAllow, defaultSlices)
}

// PlanFrom is Plan over explicit cost, allow-list and slice files.
func PlanFrom(packages []string, total int, costs, allowRaw, slicesRaw []byte) ([]Shard, error) {
	if total < 1 || total > MaxShards {
		return nil, errors.New("invalid shard count")
	}
	universe, err := validate(packages)
	if err != nil {
		return nil, err
	}
	shards := make([]Shard, total)
	w, unknown, ok := weights(costs)
	if !ok {
		// Invalid costs keep the AFP-V0-022 lexical fallback and split nothing.
		for i, p := range universe {
			shards[i%total].Packages = append(shards[i%total].Packages, p)
		}
		return shards, nil
	}
	split := SplitPackages(universe, total, allowRaw, slicesRaw)
	type unit struct {
		pkg   string
		index int
		cost  int64
	}
	units := make([]unit, 0, len(universe))
	for _, p := range universe {
		if slices, ok := split[p]; ok {
			for _, s := range slices {
				units = append(units, unit{p, s.Index, s.Milliseconds})
			}
			continue
		}
		n, ok := w[p]
		if !ok {
			n = unknown
		}
		units = append(units, unit{p, -1, n})
	}
	sort.Slice(units, func(i, j int) bool {
		a, b := units[i], units[j]
		if a.cost != b.cost {
			return a.cost > b.cost
		}
		if a.pkg != b.pkg {
			return a.pkg < b.pkg
		}
		return a.index < b.index
	})
	loads := make([]int64, total)
	holds := make([]map[string]bool, total)
	for _, u := range units {
		// Longest unit first into the least-loaded shard (lowest index on ties). A
		// slice avoids shards that already hold a sibling: it would only queue there.
		bin := -1
		for i := 0; i < total; i++ {
			if u.index >= 0 && holds[i][u.pkg] {
				continue
			}
			if bin < 0 || loads[i] < loads[bin] {
				bin = i
			}
		}
		if u.index < 0 {
			shards[bin].Packages = append(shards[bin].Packages, u.pkg)
		} else {
			if holds[bin] == nil {
				holds[bin] = map[string]bool{}
			}
			holds[bin][u.pkg] = true
			shards[bin].Slices = append(shards[bin].Slices, split[u.pkg][u.index])
		}
		loads[bin] += u.cost
	}
	for _, s := range shards {
		sort.Strings(s.Packages)
		sort.Slice(s.Slices, func(i, j int) bool {
			if s.Slices[i].Package != s.Slices[j].Package {
				return s.Slices[i].Package < s.Slices[j].Package
			}
			return s.Slices[i].Index < s.Slices[j].Index
		})
	}
	return shards, nil
}
