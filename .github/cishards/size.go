package cishards

import "errors"

// AFP-V0-2723 (V1-4934): the fewest shards a narrowed pull request needs. A narrowed run used to
// start the full sixteen-shard matrix whatever it selected, and ten to thirteen of those shards
// ran one package behind about two minutes of fixed setup each. The matrix is now sized from the
// predicted wall time of the selection's own balanced placement (AFP-V0-590, AFP-V0-2181).

// MinNarrowedShards is the fewest shards a sized matrix keeps. Two shards bound the fallback: when
// a shard's own gate records FULL after the plan narrowed, each shard runs its complete-universe
// partition over the sized matrix, and half the suite at the PR shards' parallelism stays far
// inside the job timeout.
const MinNarrowedShards = 2

// DefaultWallBudgetMs is the predicted wall time a sized shard may reach: 400 s, the AFP-V0-060
// slice bound, so a narrowed shard is never planned longer than the longest slice a full shard
// runs. The shard-plan job passes it as --size-budget-ms; a unit above it (a whole package no
// slice file splits) widens the bound to that unit alone, since no count brings it under.
const DefaultWallBudgetMs = 400000

// Size returns the fewest shards from min to max whose balanced placement of selected over
// universe (SelectParallel) keeps every shard's predicted wall time, Wall over parallel, within
// budgetMs, or within the longest unit of the max-shard placement when that unit alone exceeds
// budgetMs; max when no count does. A selection that names no package of the universe sizes to
// min. Costs that Partition would discard (AFP-V0-022 lexical fallback) predict nothing, so the
// answer is max: the sizing never trades a job it cannot price.
func Size(universe, selected []string, min, max, parallel int, budgetMs int64) (int, error) {
	return SizeFrom(universe, selected, min, max, parallel, budgetMs, defaultCosts, defaultAllow, defaultSlices)
}

// SizeFrom is Size over explicit cost, allow-list and slice files.
func SizeFrom(universe, selected []string, min, max, parallel int, budgetMs int64, costs, allowRaw, slicesRaw []byte) (int, error) {
	if min < 1 || max < min || max > MaxShards {
		return 0, errors.New("invalid shard bounds")
	}
	if parallel < 1 || parallel > MaxParallel {
		return 0, errors.New("invalid shard parallelism")
	}
	if budgetMs <= 0 {
		return 0, errors.New("invalid wall budget")
	}
	w, unknown, ok := weights(costs)
	if !ok {
		if _, err := SelectFromParallel(universe, selected, max, parallel, costs, allowRaw, slicesRaw); err != nil {
			return 0, err
		}
		return max, nil
	}
	finest, err := SelectFromParallel(universe, selected, max, parallel, costs, allowRaw, slicesRaw)
	if err != nil {
		return 0, err
	}
	bound := budgetMs
	placed := false
	for _, s := range finest {
		for _, p := range s.Packages {
			placed = true
			bound = max64(bound, costOf(p, w, unknown))
		}
		for _, sl := range s.Slices {
			placed = true
			bound = max64(bound, sl.Milliseconds)
		}
	}
	if !placed {
		return min, nil
	}
	for total := min; total <= max; total++ {
		plan := finest
		if total != max {
			if plan, err = SelectFromParallel(universe, selected, total, parallel, costs, allowRaw, slicesRaw); err != nil {
				return 0, err
			}
		}
		if PredictedWallMs(plan, parallel, w, unknown) <= bound {
			return total, nil
		}
	}
	return max, nil
}

// PredictedWallMs is the longest predicted wall time of the plan's shards at parallel: each
// shard's Wall of its summed and longest unit costs, over parallel (AFP-V0-2181).
func PredictedWallMs(plan []Shard, parallel int, w map[string]int64, unknown int64) int64 {
	var worst int64
	for _, s := range plan {
		var sum, longest int64
		for _, p := range s.Packages {
			c := costOf(p, w, unknown)
			sum += c
			longest = max64(longest, c)
		}
		for _, sl := range s.Slices {
			sum += sl.Milliseconds
			longest = max64(longest, sl.Milliseconds)
		}
		worst = max64(worst, Wall(sum, longest, parallel)/int64(parallel))
	}
	return worst
}

// Weights returns the admitted cost table and the median an unpriced package takes, as the
// placement reads them; false when the table is unusable.
func Weights(costs []byte) (map[string]int64, int64, bool) { return weights(costs) }

func costOf(p string, w map[string]int64, unknown int64) int64 {
	if n, ok := w[p]; ok {
		return n
	}
	return unknown
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
