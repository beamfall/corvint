package workflow

import (
	"strconv"
	"testing"
)

// TCQ-V0-055: the runner's int bound must be exact before any work starts.
func TestV13179_DiscriminationNativeBounds(t *testing.T) {
	max := uint64(^uint(0) >> 1)
	for _, c := range []struct {
		text  string
		want  int64
		valid bool
	}{
		{"", 8, true}, {"1", 1, true},
		{strconv.FormatUint(max, 10), int64(max), true},
		{strconv.FormatUint(max+1, 10), 0, false},
		{"2147483648", 2147483648, strconv.IntSize == 64},
		{"0", 0, false}, {"-1", 0, false}, {"bad", 0, false},
	} {
		for _, name := range []string{"max-hunks", "max-mutants"} {
			got, err := parseBound(c.text, name, 8)
			if (err == nil) != c.valid || (err == nil && got != c.want) {
				t.Errorf("%s=%q: got %d, %v; want %d valid=%v", name, c.text, got, err, c.want, c.valid)
			}
		}
	}
}
