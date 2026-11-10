package cishards

import (
	"fmt"
	"testing"
)

// sizeFixture is a universe whose costs make the right count obvious: one package no slice
// file splits at 450 s (above the 400 s budget), one split package of three 300 s slices, and
// twenty 100 s packages.
func sizeFixture() (universe []string, costs, allow, slices []byte) {
	ms := map[string]int64{"example.org/m/big": 450000, "example.org/m/split": 900000}
	for i := 0; i < 20; i++ {
		ms[fmt.Sprintf("example.org/m/p%02d", i)] = 100000
	}
	for p := range ms {
		universe = append(universe, p)
	}
	return universe, costsFile(ms), allowFile("example.org/m/split"),
		slicesFile(map[string]fixtureSplit{"example.org/m/split": {named: [][]string{{"TestA"}, {"TestB"}}, ms: []int64{300000, 300000, 300000}}})
}

// TestAFPV02723SizeIsTheFewestShardsWithinTheBudget: the sized count is the fewest whose
// balanced placement keeps every shard's predicted wall time within the budget, at the
// parallelism the shards run with.
func TestAFPV02723SizeIsTheFewestShardsWithinTheBudget_AFPV02723(t *testing.T) {
	universe, costs, allow, slices := sizeFixture()
	// Twenty 100 s packages, 2,000 s of work: two shards at -p 4 predict 1,000 / 4 = 250 s each,
	// within a 400 s budget; within 200 s only three shards fit (700 / 4 = 175 s).
	small := []string{}
	for i := 0; i < 20; i++ {
		small = append(small, fmt.Sprintf("example.org/m/p%02d", i))
	}
	n, err := SizeFrom(universe, small, 2, 16, 4, 400000, costs, allow, slices)
	if err != nil || n != 2 {
		t.Fatalf("twenty 100 s packages at -p 4 within 400 s size to %d (%v), want 2", n, err)
	}
	if n, err = SizeFrom(universe, small, 2, 16, 4, 200000, costs, allow, slices); err != nil || n != 3 {
		t.Fatalf("twenty 100 s packages at -p 4 within 200 s size to %d (%v), want 3", n, err)
	}
	// Serial shards (-p 1) need 2,000 s over 400 s shards: five.
	if n, err = SizeFrom(universe, small, 2, 16, 1, 400000, costs, allow, slices); err != nil || n != 5 {
		t.Fatalf("twenty 100 s packages at -p 1 size to %d (%v), want 5", n, err)
	}
	// A wide budget sizes to the floor.
	if n, err = SizeFrom(universe, small, 2, 16, 4, 3600000, costs, allow, slices); err != nil || n != 2 {
		t.Fatalf("a wide budget sizes to %d (%v), want the floor 2", n, err)
	}
	// Every count checked is a real placement: each sized plan places every selected package once.
	plan, err := SelectFromParallel(universe, small, 3, 4, costs, allow, slices)
	if err != nil {
		t.Fatal(err)
	}
	requireComplete(t, small, plan)
	w, unknown, _ := weights(costs)
	if wall := PredictedWallMs(plan, 4, w, unknown); wall > 200000 {
		t.Fatalf("the sized plan predicts %d ms of wall time, above the 200000 ms budget", wall)
	}
	if two, _ := SelectFromParallel(universe, small, 2, 4, costs, allow, slices); PredictedWallMs(two, 4, w, unknown) != 250000 {
		t.Fatalf("two shards predict %d ms, want 250000", PredictedWallMs(two, 4, w, unknown))
	}
}

// TestAFPV02723UnitsAboveTheBudgetWidenTheBound: a whole package no slice file splits that alone
// exceeds the budget cannot be brought under it by any count, so the bound becomes that unit;
// a split package's slices are the units, never the whole package, so a count too small to
// split it (fewer shards than slices) is refused rather than accepted at the whole cost.
func TestAFPV02723UnitsAboveTheBudgetWidenTheBound_AFPV02723(t *testing.T) {
	universe, costs, allow, slices := sizeFixture()
	// big (450 s) alone: one shard holds it; min 2 keeps two.
	n, err := SizeFrom(universe, []string{"example.org/m/big"}, 2, 16, 4, 400000, costs, allow, slices)
	if err != nil || n != 2 {
		t.Fatalf("one 450 s package sizes to %d (%v), want the floor 2", n, err)
	}
	// big plus twenty 100 s packages: the bound is 450 s; two shards hold 450 and 2,000/4 = 500,
	// three hold 450, 350 and 350 (1,400/4 and 1,400/4 of wall... the placement decides); the
	// answer is the fewest whose worst shard is within 450 s.
	sel := []string{"example.org/m/big"}
	for i := 0; i < 20; i++ {
		sel = append(sel, fmt.Sprintf("example.org/m/p%02d", i))
	}
	n, err = SizeFrom(universe, sel, 2, 16, 4, 400000, costs, allow, slices)
	if err != nil {
		t.Fatal(err)
	}
	w, unknown, _ := weights(costs)
	for total := 2; total < n; total++ {
		plan, _ := SelectFromParallel(universe, sel, total, 4, costs, allow, slices)
		if PredictedWallMs(plan, 4, w, unknown) <= 450000 {
			t.Fatalf("%d shards already fit the 450 s bound, yet Size chose %d", total, n)
		}
	}
	plan, _ := SelectFromParallel(universe, sel, n, 4, costs, allow, slices)
	if wall := PredictedWallMs(plan, 4, w, unknown); wall > 450000 {
		t.Fatalf("the sized plan of %d predicts %d ms, above the 450 s unit bound", n, wall)
	}
	// split (three 300 s slices, 900 s whole): with two shards the package stays whole at 900 s,
	// which is not within the 400 s budget nor the 300 s longest slice, so the count must reach
	// three, where each slice runs alone within 400 s.
	n, err = SizeFrom(universe, []string{"example.org/m/split"}, 2, 16, 4, 400000, costs, allow, slices)
	if err != nil || n != 3 {
		t.Fatalf("a package of three 300 s slices sizes to %d (%v), want 3", n, err)
	}
}

// TestAFPV02723SizeRefusesOrKeepsTheMaximumWhenItCannotPrice: bounds and parallelism are
// validated like the placement's; an empty selection takes the floor; a cost table the
// placement would discard sizes to the maximum, so a job is never traded on an unpriced guess.
func TestAFPV02723SizeRefusesOrKeepsTheMaximumWhenItCannotPrice_AFPV02723(t *testing.T) {
	universe, costs, allow, slices := sizeFixture()
	for _, tc := range []struct {
		name          string
		min, max, par int
		budget        int64
	}{{"min below 1", 0, 16, 4, 400000}, {"max below min", 4, 2, 4, 400000}, {"max above MaxShards", 2, 65, 4, 400000},
		{"parallel 0", 2, 16, 0, 400000}, {"budget 0", 2, 16, 4, 0}} {
		if _, err := SizeFrom(universe, universe, tc.min, tc.max, tc.par, tc.budget, costs, allow, slices); err == nil {
			t.Errorf("%s: no error", tc.name)
		}
	}
	if n, err := SizeFrom(universe, []string{}, 2, 16, 4, 400000, costs, allow, slices); err != nil || n != 2 {
		t.Fatalf("an empty selection sizes to %d (%v), want the floor", n, err)
	}
	if n, err := SizeFrom(universe, []string{"example.org/other"}, 2, 16, 4, 400000, costs, allow, slices); err != nil || n != 2 {
		t.Fatalf("a selection outside the universe sizes to %d (%v), want the floor", n, err)
	}
	if n, err := SizeFrom(universe, universe, 2, 16, 4, 400000, []byte("{}"), allow, slices); err != nil || n != 16 {
		t.Fatalf("an unusable cost table sizes to %d (%v), want the maximum 16", n, err)
	}
	if _, err := SizeFrom([]string{"bad..name"}, universe, 2, 16, 4, 400000, costs, allow, slices); err == nil {
		t.Fatal("an invalid universe sized without error")
	}
	// The committed tables price the repository: the whole universe sizes to the maximum at the
	// default budget, since a full run needs every shard, and a one-package selection to the floor.
	if n, err := Size([]string{"github.com/Beamfall/corvint/internal/projectpath"}, []string{"github.com/Beamfall/corvint/internal/projectpath"}, MinNarrowedShards, 16, 4, DefaultWallBudgetMs); err != nil || n != MinNarrowedShards {
		t.Fatalf("one package sizes to %d (%v), want %d", n, err, MinNarrowedShards)
	}
}
