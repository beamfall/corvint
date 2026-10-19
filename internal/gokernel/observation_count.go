package gokernel

import "math"

// countValue reads a non-negative wire count whether the provider wrote a Go int or the block
// was decoded from JSON, as sanitizeHarnessQueryContext decodes every user-prompt packet into
// float64 numbers. Reading only int recorded every user-prompt count as zero (SOL-V0-011, V1-1977).
func countValue(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		if typed >= 0 {
			return typed, true
		}
	case float64:
		if typed >= 0 && typed <= math.MaxInt32 && typed == math.Trunc(typed) {
			return int(typed), true
		}
	}
	return 0, false
}
