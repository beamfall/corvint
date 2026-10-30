package extmutate

import "sort"

func sortStrings(values []string) { sort.Strings(values) }

func sortMutants(mutants []Mutant) {
	sort.SliceStable(mutants, func(a, b int) bool {
		x, y := mutants[a], mutants[b]
		if x.Hunk != y.Hunk {
			return x.Hunk < y.Hunk
		}
		if x.Line != y.Line {
			return x.Line < y.Line
		}
		return x.Column < y.Column
	})
}
