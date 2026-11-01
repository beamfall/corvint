package main

import (
	"slices"
	"strings"
)

// didYouMean is the suffix an invalid-choice message gains when a choice is
// near the typed value (ACX-V0-859): "; did you mean 'context'?", or "" when
// none is.
func didYouMean(value string, choices []string) string {
	near := suggestChoices(value, choices)
	if len(near) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(near))
	for _, choice := range near {
		quoted = append(quoted, pythonRepr(choice))
	}
	return "; did you mean " + strings.Join(quoted, " or ") + "?"
}

// suggestChoices returns up to three choices within edit distance 2 of
// value, nearest first, ties in choice order. A distance must also be at
// most half the value's length, so a short unrelated word suggests nothing.
func suggestChoices(value string, choices []string) []string {
	type candidate struct {
		choice   string
		distance int
	}
	var found []candidate
	for _, choice := range choices {
		if d := editDistance(value, choice); d > 0 && d <= 2 && 2*d <= len(value) {
			found = append(found, candidate{choice, d})
		}
	}
	slices.SortStableFunc(found, func(a, b candidate) int { return a.distance - b.distance })
	near := make([]string, 0, 3)
	for _, c := range found {
		if len(near) == 3 {
			break
		}
		near = append(near, c.choice)
	}
	return near
}

// editDistance is the Levenshtein distance between a and b over bytes.
func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}
