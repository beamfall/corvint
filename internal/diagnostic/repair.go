package diagnostic

import "slices"

// RepairStop is DRC-V0-009's stop rule, published for an adapter that drives a repair loop;
// Corvint itself drives none (decision 0201 (h)). outstanding[0] is the count of outstanding
// diagnostics before any round and outstanding[i] the count after round i. The loop stops when
// nothing is outstanding, or when each of the last two rounds failed to reach a new minimum
// below every earlier count; whatever is still outstanding then is unresolved, not a partial
// success.
func RepairStop(outstanding []int) bool {
	rounds := len(outstanding) - 1
	if rounds < 0 {
		return false
	}
	if outstanding[rounds] == 0 {
		return true
	}
	if rounds < 2 {
		return false
	}
	return !newMinimum(outstanding, rounds) && !newMinimum(outstanding, rounds-1)
}

func newMinimum(outstanding []int, round int) bool {
	return outstanding[round] < slices.Min(outstanding[:round])
}
