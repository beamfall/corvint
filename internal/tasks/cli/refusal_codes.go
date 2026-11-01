package cli

import (
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// derivedRefusalCodes maps a refusal outcome recorded without codes to the
// closed §11 code the envelope reports (TCL-V0-870). UNAUTHORIZED has no code
// of its own and reports MALFORMED, as wire.CodeOf does for an uncoded error.
var derivedRefusalCodes = map[string]string{
	mutation.OutcomeRevisionConflict:  wire.CodeStaleTicket,
	mutation.OutcomeValidationFailed:  wire.CodeMalformed,
	mutation.OutcomeUnauthorized:      wire.CodeMalformed,
	mutation.OutcomeBlocked:           wire.CodeTicketState,
	mutation.OutcomeUnsupported:       wire.CodeUnsupported,
	mutation.OutcomeCapacityExhausted: wire.CodeResourceExhausted,
	mutation.OutcomeUncertainEffect:   wire.CodeUncertainEffect,
	mutation.OutcomeRequestIDConflict: wire.CodeRequestIDConflict,
	mutation.OutcomeStorageFailed:     wire.CodeUnsupportedFilesystem,
}

// refusalCodes is the envelope codes of a refused outcome: its recorded
// codes, or the one code derived from the outcome when it recorded none.
// Receipts keep the recorded codes unchanged.
func refusalCodes(o mutation.Outcome) []string {
	if len(o.Codes) > 0 || o.Outcome == mutation.OutcomeCompleted {
		return o.Codes
	}
	if c, ok := derivedRefusalCodes[o.Outcome]; ok {
		return []string{c}
	}
	return []string{wire.CodeMalformed}
}

// nearVerbWords returns the distinct first words of ReadVerbs within edit
// distance 2 of word, nearest first, at most three (TCL-V0-859). A distance
// must also be at most half the word's length, so a short unrelated word
// suggests nothing.
func nearVerbWords(word string) []string {
	type candidate struct {
		word     string
		distance int
	}
	var found []candidate
	seen := map[string]bool{}
	for _, verb := range ReadVerbs {
		first, _, _ := strings.Cut(verb, " ")
		if seen[first] {
			continue
		}
		seen[first] = true
		if d := editDistance(word, first); d > 0 && d <= 2 && 2*d <= len(word) {
			found = append(found, candidate{first, d})
		}
	}
	slices.SortStableFunc(found, func(a, b candidate) int { return a.distance - b.distance })
	near := make([]string, 0, 3)
	for _, c := range found {
		if len(near) == 3 {
			break
		}
		near = append(near, c.word)
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
