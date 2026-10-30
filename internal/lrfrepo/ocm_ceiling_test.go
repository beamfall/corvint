package lrfrepo

import (
	"math"
	"time"
)

// ceilingRounds is how many back-to-back samples each legal-ceiling test takes
// of one blob derivation and of the whole ceiling. The ratio those tests pin is
// a property of the code, so the FASTEST sample of each is the one that carries
// it: a single sample flaked under a parallel `go test ./...`, where one
// preempted enumeration measured 2.6s against a 0.19s derivation (0.3s against
// 0.08s in isolation).
const ceilingRounds = 5

// fastestCeilingSamples interleaves derive and work for ceilingRounds rounds and
// returns the fastest observation of each.
func fastestCeilingSamples(derive, work func()) (derivation, elapsed time.Duration) {
	derivation, elapsed = time.Duration(math.MaxInt64), time.Duration(math.MaxInt64)
	for round := 0; round < ceilingRounds; round++ {
		derivation = min(derivation, timed(derive))
		elapsed = min(elapsed, timed(work))
	}
	return derivation, elapsed
}

func timed(step func()) time.Duration {
	started := time.Now()
	step()
	return time.Since(started)
}
