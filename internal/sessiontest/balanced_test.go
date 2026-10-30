package sessiontest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// repositoryPartition reads this repository's .github/cishards placement
// files and returns them with the cost table's package universe.
func repositoryPartition(t *testing.T) (PartitionFiles, []string) {
	t.Helper()
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join("..", "..", ".github", "cishards", name))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	files := PartitionFiles{Allow: read("test-split-allow.json"), Costs: read("package-costs.json"), Slices: read("test-slices.json")}
	var costs struct {
		Milliseconds map[string]json.RawMessage `json:"milliseconds"`
	}
	if err := json.Unmarshal(files.Costs, &costs); err != nil {
		t.Fatal(err)
	}
	universe := []string{}
	for pkg := range costs.Milliseconds {
		if strings.HasPrefix(pkg, "github.com/") {
			universe = append(universe, pkg)
		}
	}
	sort.Strings(universe)
	if len(universe) < 20 {
		t.Fatalf("cost table names %d packages", len(universe))
	}
	return files, universe
}

// unitKeys returns each unit's (unit, slice) key, sorted, and fails on a
// unit placed twice or on a shard outside 0..total-1.
func unitKeys(t *testing.T, m Manifest, total int) []string {
	t.Helper()
	keys := []string{}
	seen := map[string]bool{}
	for _, unit := range m.Units {
		if seen[unit.Key()] {
			t.Fatalf("unit %s placed twice", unit.Key())
		}
		if unit.Shard < 0 || unit.Shard >= total {
			t.Fatalf("unit %s on shard %d of %d", unit.Key(), unit.Shard, total)
		}
		seen[unit.Key()] = true
		keys = append(keys, unit.Key())
	}
	sort.Strings(keys)
	return keys
}

// STR-V0-008: balanced placement places every execution unit of the run set
// exactly once for 1 to 16 shards, a split package's catch-all slice
// included; its units equal universe placement's; a run set of at least N
// units uses every shard; and sibling slices land on different shards.
func TestSTRV0008BalancedPlacesEveryUnitOnce(t *testing.T) {
	files, universe := repositoryPartition(t)
	const split = "github.com/Beamfall/corvint/internal/tasks/store"
	// count is the committed slice count of the split package; with fewer
	// shards than slices it runs whole (AFP-V0-041).
	var sliceFile struct {
		Packages map[string]struct {
			Named []json.RawMessage `json:"named"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(files.Slices, &sliceFile); err != nil {
		t.Fatal(err)
	}
	count := len(sliceFile.Packages[split].Named) + 1
	if count < 2 {
		t.Fatalf("the committed slice file does not split %s", split)
	}
	runSets := map[string][]string{"one package": {universe[0]}, "split package only": {split}}
	thirds, all := []string{}, append([]string{}, universe...)
	for index, pkg := range universe {
		if index%3 == 0 || pkg == split {
			thirds = append(thirds, pkg)
		}
	}
	runSets["every third and the split package"], runSets["whole universe narrowed"] = thirds, all
	for name, run := range runSets {
		for total := 1; total <= 16; total++ {
			build := func(placement string) Manifest {
				input := fixtureInput()
				input.Universe, input.Partition, input.ShardCount, input.Placement = universe, files, total, placement
				input.GoSelection = "run " + strings.Join(run, " ")
				m, err := Build(input)
				if err != nil || len(m.Incomplete) != 0 || m.Scope != ScopeNarrowed {
					t.Fatalf("%s, %d shards, %s: %v %v %s", name, total, placement, err, m.Incomplete, m.Scope)
				}
				return m
			}
			balanced, universal := build(PlacementBalanced), build(PlacementUniverse)
			if balanced.Placement != PlacementBalanced || universal.Placement != PlacementUniverse {
				t.Fatalf("placement recorded as %q and %q", balanced.Placement, universal.Placement)
			}
			got, want := unitKeys(t, balanced, total), unitKeys(t, universal, total)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%s, %d shards: balanced units differ from universe units\ngot  %v\nwant %v", name, total, got, want)
			}
			slices := 0
			for _, key := range got {
				if strings.HasPrefix(key, "go:"+split+" ") && !strings.HasSuffix(key, " "+SliceWhole) {
					slices++
				}
			}
			catchAll := fmt.Sprintf("go:%s %d/%d", split, count-1, count)
			if containsString(run, split) && total >= count && (slices != count || !containsString(got, catchAll)) {
				t.Fatalf("%s, %d shards: the split package's slices, catch-all included, are missing: %v", name, total, got)
			}
			used := map[int]bool{}
			siblings := map[string]int{}
			for _, unit := range balanced.Units {
				used[unit.Shard] = true
				if unit.Slice == SliceWhole {
					continue
				}
				if shard, ok := siblings[unit.Unit]; ok && shard == unit.Shard {
					t.Fatalf("%s, %d shards: sibling slices of %s share shard %d", name, total, unit.Unit, shard)
				}
				siblings[unit.Unit] = unit.Shard
			}
			if len(got) >= total && len(used) != total {
				t.Fatalf("%s, %d shards: %d units use only %d shards", name, total, len(got), len(used))
			}
		}
	}
}

// STR-V0-008: four packages the universe partition puts on one shard use
// all four shards under balanced placement, with or without a usable cost
// table (an unusable one is the lexical round-robin over the run set).
func TestSTRV0008BalancedUsesEveryShard(t *testing.T) {
	files, universe := repositoryPartition(t)
	input := fixtureInput()
	input.Universe, input.Partition, input.ShardCount = universe, files, 4
	input.GoSelection = "run " + strings.Join(universe, " ")
	m, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	// The heaviest package can fill a shard alone, so take the first shard
	// holding at least four whole packages.
	byShard := map[int][]string{}
	for _, unit := range m.Units {
		if unit.Slice == SliceWhole {
			byShard[unit.Shard] = append(byShard[unit.Shard], strings.TrimPrefix(unit.Unit, "go:"))
		}
	}
	shard, run := -1, []string{}
	for s := 0; s < 4 && shard < 0; s++ {
		if len(byShard[s]) >= 4 {
			shard, run = s, byShard[s][:4]
		}
	}
	input.GoSelection = "run " + strings.Join(run, " ")
	if m, err = Build(input); err != nil || len(m.Units) != 4 || m.Units[3].Shard != shard {
		t.Fatalf("universe placement of %v: %v %v", run, err, unitIDs(m))
	}
	for _, partition := range []PartitionFiles{files, {}} {
		input.Partition, input.Placement = partition, PlacementBalanced
		m, err := Build(input)
		if err != nil || len(m.Incomplete) != 0 {
			t.Fatal(err, m.Incomplete)
		}
		used := map[int]bool{}
		for _, unit := range m.Units {
			used[unit.Shard] = true
		}
		if len(m.Units) != 4 || len(used) != 4 {
			t.Fatalf("costs usable=%v: units %v", partition.Costs != nil, unitIDs(m))
		}
	}
}

// STR-V0-008, STR-V0-018: FULL keeps the complete-universe partition in
// either mode; the manifest still binds the requested mode, every shard
// recomputes the same digest, and an unknown mode is refused.
func TestSTRV0008BalancedFullAndManifest(t *testing.T) {
	files, universe := repositoryPartition(t)
	build := func(placement, selection string) Manifest {
		input := fixtureInput()
		input.Universe, input.Partition, input.ShardCount, input.Placement = universe, files, 6, placement
		input.GoSelection = selection
		m, err := Build(input)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	full := "FALLBACK a deleted source file"
	balancedFull, universeFull := build(PlacementBalanced, full), build(PlacementUniverse, full)
	if balancedFull.Scope != ScopeFull || !reflect.DeepEqual(unitIDs(balancedFull), unitIDs(universeFull)) {
		t.Fatalf("FULL balanced placement differs from the universe partition")
	}
	if balancedFull.Placement != PlacementBalanced {
		t.Fatalf("FULL manifest records placement %q", balancedFull.Placement)
	}
	narrowed := "run " + strings.Join(universe[:9], " ")
	digests := map[string]bool{}
	for shard := 0; shard < 6; shard++ { // each shard recomputes the manifest from the same input
		digest, err := Digest(build(PlacementBalanced, narrowed))
		if err != nil {
			t.Fatal(err)
		}
		digests[digest] = true
	}
	universeDigest, err := Digest(build(PlacementUniverse, narrowed))
	if err != nil {
		t.Fatal(err)
	}
	if len(digests) != 1 || digests[universeDigest] {
		t.Fatalf("balanced digests %v; universe digest %s", digests, universeDigest)
	}
	input := fixtureInput()
	input.Placement = "packed"
	if _, err := Build(input); err == nil || !strings.Contains(err.Error(), "placement must be") {
		t.Fatalf("unknown placement accepted: %v", err)
	}
	input.Placement = ""
	if m, err := Build(input); err != nil || m.Placement != PlacementUniverse {
		t.Fatalf("empty placement: %v %q", err, m.Placement)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
