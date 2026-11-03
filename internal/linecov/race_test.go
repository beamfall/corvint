//go:build race

package linecov

// raceScale stretches the MCJ-V0-019 plan-time budget under -race, as CI's Go
// product shards run. The race runtime instruments every map and slice access
// of the plan: measured on the reference host, the plan's p95 was 25 ms
// uninstrumented and 281 ms under -race, about 11x. The absolute 100 ms budget
// holds the uninstrumented product; under -race the scaled bound only catches
// a gross regression, so the factor stays well clear of the measured 11x.
const raceScale = 20
