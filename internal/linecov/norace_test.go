//go:build !race

package linecov

// raceScale is 1 without -race: the MCJ-V0-019 budget applies unscaled.
const raceScale = 1
