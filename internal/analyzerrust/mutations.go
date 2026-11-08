// SPDX-License-Identifier: Apache-2.0

package analyzerrust

import (
	"fmt"
	"sort"
)

// BooleanMutation describes a single Rust boolean-token replacement for the
// core qualification harness. It neither edits files nor runs/qualifies a
// mutant. The harness owns sampling, applicability and FULL-killed trials.
type BooleanMutation struct {
	Offset              int
	Before, After, Body string
}

// BooleanMutations is the Rust plugin's bounded boolean-flip operator. It uses
// the same exact-blob dialect profile as executable regions; a malformed,
// capped or unsupported source produces no partial candidate set. Strings,
// comments, raw identifiers and booleans outside function interiors are inert.
func BooleanMutations(source []byte) ([]BooleanMutation, error) {
	bodies, err := ExecutableBodies(source)
	if err != nil {
		return nil, err
	}
	tokens, reason := tokenize(source)
	if reason != "" {
		return nil, fmt.Errorf("rust-mutations:%s", reason)
	}
	starts := []int{0}
	for i, b := range source {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}
	var out []BooleanMutation
	for _, t := range tokens {
		if t.kind != tokenIdent || t.text != "true" && t.text != "false" {
			continue
		}
		line := sort.Search(len(starts), func(i int) bool { return starts[i] > t.start }) - 1
		row, col := int32(line+1), int32(t.start-starts[line]+1)
		body := ""
		for _, b := range bodies {
			r := b.Range
			if (r.Start < row || r.Start == row && r.StartColumn <= col) && (row < r.End || row == r.End && col < r.EndColumn) {
				body = b.ID
			}
		}
		if body == "" {
			continue
		}
		after := "true"
		if t.text == "true" {
			after = "false"
		}
		out = append(out, BooleanMutation{Offset: t.start, Before: t.text, After: after, Body: body})
		if len(out) > MaxFacts {
			return nil, fmt.Errorf("rust-mutations:cap-exceeded")
		}
	}
	return out, nil
}
