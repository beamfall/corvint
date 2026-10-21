package main

import (
	"bytes"
	"strings"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// screenSourceLines returns lines[first-1:last] of source (1-based,
// inclusive) as text with every line that a secret-screen match overlaps
// replaced by secretscreen.Placeholder, keeping its line ending, and the
// 1-based numbers of the replaced lines (ESV-V0-012). The screen runs over the
// whole source, so a match that spans lines, such as a private-key block,
// redacts every line it touches, including lines whose own bytes look clean.
func screenSourceLines(source []byte, lines [][]byte, first, last int) (string, []int) {
	selected := bytes.Join(lines[first-1:last], nil)
	spans := secretscreen.MatchIndexes(string(source))
	if len(spans) == 0 {
		return string(selected), nil
	}
	var text strings.Builder
	redacted := []int{}
	offset, next := 0, 0
	for index, line := range lines[:last] {
		start, end := offset, offset+len(line)
		offset = end
		for next < len(spans) && spans[next][1] <= start {
			next++
		}
		if index < first-1 {
			continue
		}
		hit := false
		for _, span := range spans[next:] {
			if span[0] >= end {
				break
			}
			if span[1] > start {
				hit = true
				break
			}
		}
		if !hit {
			text.Write(line)
			continue
		}
		redacted = append(redacted, index+1)
		text.WriteString(secretscreen.Placeholder)
		text.Write(line[len(bytes.TrimRight(line, "\r\n")):])
	}
	if len(redacted) == 0 {
		return string(selected), nil
	}
	return text.String(), redacted
}
