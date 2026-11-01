package dogfoodflow

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// repinLines re-pins a carried cite row's BASE lines of name from the prior
// BASE its plan was drafted at to the current BASE (DCW-V0-842): the lines
// stay when both BASEs hold the same text there, move to the single
// occurrence of that text at BASE otherwise, and are kept as written, for the
// DCW-V0-840 vet to judge, when the text is gone or occurs more than once. It
// returns the lines and a note naming any move.
func (p *proposal) repinLines(name, lines string) (string, string) {
	if p.priorBase == "" || p.priorBase == p.base {
		return lines, ""
	}
	prior, ok := p.blob(p.priorBase, name)
	if !ok {
		return lines, ""
	}
	span, ok := lineByteSpan(prior, lines)
	if !ok {
		return lines, ""
	}
	text := spanBytes(prior, span)
	current, _ := p.blob(p.base, name)
	if now, ok := lineByteSpan(current, lines); ok && bytes.Equal(spanBytes(current, now), text) {
		return lines, ""
	}
	found := -1
	for offset := 0; offset <= len(current); {
		index := bytes.Index(current[offset:], text)
		if index < 0 || len(text) == 0 {
			break
		}
		start := offset + index
		end := start + len(text)
		if (start == 0 || current[start-1] == '\n') && (end == len(current) || text[len(text)-1] == '\n') {
			if found >= 0 {
				return lines, ""
			}
			found = start
		}
		offset = start + 1
	}
	if found < 0 {
		return lines, ""
	}
	first := bytes.Count(current[:found], []byte("\n")) + 1
	last := first + bytes.Count(bytes.TrimSuffix(text, []byte("\n")), []byte("\n"))
	moved := fmt.Sprintf("%d:%d", first, last)
	return moved, fmt.Sprintf("%s %s re-pinned to %s: the plan was drafted at BASE %s and BASE %s moved that text (DCW-V0-842)", name, lines, moved, p.priorBase, p.base)
}

// spanBytes returns blob's bytes of a lineByteSpan START:END result.
func spanBytes(blob []byte, span string) []byte {
	from, to, _ := strings.Cut(span, ":")
	start, _ := strconv.Atoi(from)
	end, _ := strconv.Atoi(to)
	return blob[start:end]
}

// carriedForward proposes, for an owed hunk --refresh carried no row to, the
// first cite row carried to another hunk of the same file, labelled carried
// forward (DCW-V0-842); "" when that file carries no cite row.
func (p *proposal) carriedForward(hunk proposedHunk, ordinal string) (string, string) {
	carried, ok := p.carriedFiles[hunk.path]
	if !ok {
		return "", ""
	}
	fields := strings.Split(carried, "\t")
	p.proposals[ordinal] = true
	p.reasons[ordinal] = "carried forward: --refresh carried no row to this hunk, and this is the first citation it carried to another hunk of " + hunk.path + "; confirm the new hunk depends on or implements it (DCW-V0-842)"
	return ordinal + "\t" + carried, fields[0] + ":" + fields[1]
}
