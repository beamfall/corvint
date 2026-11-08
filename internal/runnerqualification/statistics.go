// SPDX-License-Identifier: Apache-2.0

package runnerqualification

import "math"

// Upper95 returns the exact one-sided Clopper-Pearson upper confidence limit
// (numerically inverted for nonzero misses). No observations, malformed counts
// and all-miss observations conservatively return 1. It never reports a zero
// miss probability. The bound describes only the sampled mutant population.
func Upper95(misses, trials int) float64 {
	if trials <= 0 || misses < 0 || misses >= trials || trials > MaxSamples {
		return 1
	}
	if misses == 0 {
		return -math.Expm1(math.Log(0.05) / float64(trials))
	}
	lo, hi := float64(misses)/float64(trials), 1.0
	for i := 0; i < 64; i++ {
		p := (lo + hi) / 2
		if binomialCDF(misses, trials, p) > 0.05 {
			lo = p
		} else {
			hi = p
		}
	}
	return hi
}

func binomialCDF(k, n int, p float64) float64 {
	a, _ := math.Lgamma(float64(n + 1))
	b, _ := math.Lgamma(float64(k + 1))
	c, _ := math.Lgamma(float64(n - k + 1))
	term := a - b - c + float64(k)*math.Log(p) + float64(n-k)*math.Log1p(-p)
	sum := term
	for j := k; j > 0; j-- {
		term += math.Log(float64(j)/float64(n-j+1)) + math.Log1p(-p) - math.Log(p)
		if term > sum {
			sum = term + math.Log1p(math.Exp(sum-term))
		} else {
			sum += math.Log1p(math.Exp(term - sum))
		}
	}
	return math.Exp(sum)
}
