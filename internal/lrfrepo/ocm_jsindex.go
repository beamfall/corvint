package lrfrepo

import "regexp"

// Per-blob JS derivation, computed ONCE.
//
// countJSSelector walked every `it(`/`test(` call in the blob and compared the
// selector each one derives against ONE selector, so verifying N claims in a
// blob walked it N times. The tally below is that identical walk, accumulating
// into a map instead of comparing, so the walk happens once and each claim's
// question is a lookup. Nothing about which calls count, or what selector each
// derives, changes.
// jsTestCallTail is the same call form anchored at end of line: what sits
// immediately before a claim's opening quote. Compiled once, not per claim.
var jsTestCallTail = regexp.MustCompile(`\b(?:it|test)(?:\.(?:concurrent|only|skip|todo))?\s*\(\s*$`)

var jsTestCallPrefix = regexp.MustCompile(`\b(?:it|test)(?:\.(?:concurrent|only|skip|todo))?\s*\(\s*`)

type jsBlobIndex struct {
	clean          []byte
	selectorCounts map[string]int
}

func newJSBlobIndex(blob []byte) *jsBlobIndex {
	index := &jsBlobIndex{clean: maskComments(blob, ".js"), selectorCounts: map[string]int{}}
	index.indexSelectors()
	return index
}

func (index *jsBlobIndex) indexSelectors() {
	clean := index.clean
	for _, match := range jsTestCallPrefix.FindAllIndex(clean, -1) {
		start := match[1]
		if start >= len(clean) || clean[start] != '"' && clean[start] != '\'' && clean[start] != '`' {
			continue
		}
		quote := clean[start]
		end := start + 1
		for end < len(clean) {
			if clean[end] == '\\' {
				end += 2
				continue
			}
			if clean[end] == quote {
				break
			}
			end++
		}
		if end >= len(clean) {
			continue
		}
		claimText, ok := decodeJSString(clean[start+1:end], quote)
		if !ok {
			continue
		}
		index.selectorCounts["test:"+selectorFragment(claimText)]++
	}
}

func (index *jsBlobIndex) selectorCount(selector string) int {
	return index.selectorCounts[selector]
}
