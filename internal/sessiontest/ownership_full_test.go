// SPDX-License-Identifier: Apache-2.0
package sessiontest

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// Ordinary content selections keep the pre-obligation partitioned manifest:
// execution slices omit package slicing reasons, in both JSON and its digest.
func TestOwnershipPartitionedContentOnlyBytes_AZU_V0_9900(t *testing.T) {
	files, universe := repositoryPartition(t)
	const pkg = "github.com/Beamfall/corvint/internal/liveverify/affected"
	plan, err := PlanShardsSliced(universe, 16, 1, []History{{Name: "committed", Raw: files.Costs}}, files.Allow, files.Slices)
	if err != nil || plan.Fallback != "" {
		t.Fatalf("measured partition: %v, fallback %q", err, plan.Fallback)
	}
	for _, placement := range []string{PlacementUniverse, PlacementBalanced, PlacementMeasured} {
		for _, full := range []bool{false, true} {
			for _, slice := range []GoSlice{
				{Reason: "whole: DIRECT_SOURCE_CHANGE via pkg/pkg.go"},
				{Pattern: "^(TestContent)$", Reason: "selected: TestContent via pkg/pkg_test.go"},
			} {
				t.Run(fmt.Sprintf("%s/full=%t/%s", placement, full, slice.Reason), func(t *testing.T) {
					input := fixtureInput()
					input.Universe, input.Partition, input.ShardCount = universe, files, 16
					input.Placement, input.ShardPlan = placement, &plan
					input.GoSelection = "run " + pkg
					if full {
						input.GoSelection = "FALLBACK go.mod changed"
					}
					want, err := Build(input)
					if err != nil || len(want.Incomplete) != 0 {
						t.Fatalf("compatibility manifest: %v, %+v", err, want)
					}
					input.GoSlices = map[string]GoSlice{pkg: slice}
					got, err := Build(input)
					if err != nil || len(got.Incomplete) != 0 {
						t.Fatalf("content manifest: %v, %+v", err, got)
					}
					partitioned := 0
					for _, unit := range got.Units {
						if unit.Unit == "go:"+pkg && unit.Slice != SliceWhole && unit.Pattern != "" {
							partitioned++
							if unit.Reason != "" || unit.ReasonCode != "" {
								t.Errorf("content-only partition %s acquired reason %q and code %q", unit.Key(), unit.Reason, unit.ReasonCode)
							}
						}
					}
					if partitioned < 2 {
						t.Fatalf("expected partitioned execution units, got %d", partitioned)
					}
					wantJSON, err := Encode(want)
					if err != nil {
						t.Fatal(err)
					}
					gotJSON, err := Encode(got)
					if err != nil {
						t.Fatal(err)
					}
					wantDigest, err := Digest(want)
					if err != nil {
						t.Fatal(err)
					}
					gotDigest, err := Digest(got)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(gotJSON, wantJSON) || gotDigest != wantDigest {
						t.Errorf("content-only partition changed manifest bytes or digest: got %s, want %s", gotDigest, wantDigest)
					}
				})
			}
		}
	}
}

// AZU-V0-9900: FULL preserves explanations from both gate-only slices and
// independently reached whole packages without mutating the caller's slices.
func TestOwnershipFullReasons_AZU_V0_9900(t *testing.T) {
	input := fixtureInput()
	input.GoSelection = "FALLBACK UNINDEXED_SOURCE_PATH gone/only.go"
	const selected = "selected: TestOwnershipAuditRatchet_AZU_V0_002 via gone/only.go (AZU-V0-9900)"
	const mustRun = "whole: MUST_RUN (TestOwnershipAuditRatchet_AZU_V0_002 via gone/only.go (AZU-V0-9900))"
	input.GoSlices = map[string]GoSlice{
		"example.test/m/a": {Pattern: "^(TestOwnershipAuditRatchet_AZU_V0_002)$", Reason: selected, FullReason: "whole: FULL; required " + selected},
		"example.test/m/b": {Reason: mustRun, FullReason: mustRun},
		"example.test/m/c": {Reason: "whole: DIRECT_SOURCE_CHANGE via c/c.go"},
	}
	manifest, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Scope != ScopeFull || len(manifest.Units) != len(input.Universe) {
		t.Fatalf("not FULL: %+v", manifest)
	}
	for _, unit := range manifest.Units {
		if unit.Slice != SliceWhole || unit.Pattern != "" || unit.Flag != "" {
			t.Fatalf("FULL narrowed: %+v", unit)
		}
		switch unit.Unit {
		case "go:example.test/m/a":
			if !strings.Contains(unit.Reason, selected) {
				t.Fatalf("lost selected explanation: %+v", unit)
			}
		case "go:example.test/m/b":
			if unit.Reason != mustRun {
				t.Fatalf("lost whole explanation: %+v", unit)
			}
		case "go:example.test/m/c":
			if unit.Reason != "" {
				t.Fatalf("unrelated fallback receipt changed: %+v", unit)
			}
		}
	}
	if input.GoSlices["example.test/m/a"].Pattern == "" {
		t.Fatal("Build mutated the input slice")
	}
}

// AZU-V0-9900: partitioning must retain a named obligation on every CI
// slice, including the catch-all that executes the ownership ratchet.
func TestOwnershipPartitionedReasons_AZU_V0_9900(t *testing.T) {
	files, universe := repositoryPartition(t)
	const pkg = "github.com/Beamfall/corvint/internal/liveverify/affected"
	const selected = "selected: TestOwnershipAuditRatchet_AZU_V0_002 via added.txt (AZU-V0-9900)"
	const mustRun = "whole: MUST_RUN (TestOwnershipAuditRatchet_AZU_V0_002 via added.txt (AZU-V0-9900))"
	plan, err := PlanShardsSliced(universe, 16, 1, []History{{Name: "committed", Raw: files.Costs}}, files.Allow, files.Slices)
	if err != nil || plan.Fallback != "" {
		t.Fatalf("measured partition: %v, fallback %q", err, plan.Fallback)
	}
	for _, placement := range []string{PlacementUniverse, PlacementBalanced, PlacementMeasured} {
		for _, full := range []bool{false, true} {
			for _, whole := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/full=%t/whole=%t", placement, full, whole), func(t *testing.T) {
					input := fixtureInput()
					input.Universe, input.Partition, input.ShardCount = universe, files, 16
					input.Placement, input.ShardPlan = placement, &plan
					input.GoSelection = "run " + pkg
					slice := GoSlice{Pattern: "^(TestOwnershipAuditRatchet_AZU_V0_002)$", Reason: selected, FullReason: "whole: FULL; required " + selected}
					if whole {
						slice = GoSlice{Reason: mustRun, FullReason: mustRun}
					}
					wantReason, wantCode, wantScope := slice.Reason, ReasonOther, ScopeNarrowed
					if whole {
						wantCode = ReasonReachedPrefix + "must-run"
					}
					if full {
						input.GoSelection = "FALLBACK go.mod changed"
						wantReason, wantScope = slice.FullReason, ScopeFull
					}
					input.GoSlices = map[string]GoSlice{pkg: slice}
					manifest, err := Build(input)
					if err != nil || len(manifest.Incomplete) != 0 || manifest.Scope != wantScope {
						t.Fatalf("Build: %v, manifest %+v", err, manifest)
					}
					shards, catchAll := map[int]bool{}, false
					for _, unit := range manifest.Units {
						if unit.Unit != "go:"+pkg {
							if unit.Reason != "" || unit.ReasonCode != "" {
								t.Fatalf("unrelated package acquired an obligation: %+v", unit)
							}
							continue
						}
						if unit.Reason != wantReason || unit.ReasonCode != wantCode {
							t.Errorf("lost obligation: %+v; want reason %q, code %q", unit, wantReason, wantCode)
						}
						if unit.Slice == SliceWhole || unit.Pattern == "" || shards[unit.Shard] {
							t.Fatalf("not partitioned across distinct shards: %+v", unit)
						}
						shards[unit.Shard] = true
						catchAll = catchAll || unit.Flag == "-skip"
					}
					if len(shards) < 2 || !catchAll {
						t.Fatalf("missing sibling or catch-all slices: shards %v, catch-all %t", shards, catchAll)
					}
					if input.GoSlices[pkg] != slice {
						t.Fatal("Build mutated the input slice")
					}
				})
			}
		}
	}
}
