package dogfoodflow

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// repinLines re-pins a carried cite row's BASE span of name from the prior
// BASE its plan was drafted at to the current BASE (DCW-V0-842): the span
// stays when both BASEs hold the same bytes there and moves to the single
// occurrence of those bytes at BASE otherwise, a line span to whole lines and
// a bytes:START:END span anywhere. It returns the span and a note naming any
// move, or, when the cited bytes no longer occur exactly once at BASE (or do
// not resolve at the prior BASE), a non-empty gone reason: such a row is never
// re-pinned or kept, since the same coordinates would now cite other text.
func (p *proposal) repinLines(name, lines string) (string, string, string) {
	if p.priorBase == "" || p.priorBase == p.base {
		return lines, "", ""
	}
	prior, ok := p.blob(p.priorBase, name)
	if !ok {
		return lines, "", fmt.Sprintf("%s is absent at the prior BASE %s", name, p.priorBase)
	}
	span, ok := lineByteSpan(prior, lines)
	if !ok {
		return lines, "", fmt.Sprintf("%s %s does not resolve at the prior BASE %s", name, lines, p.priorBase)
	}
	text := spanBytes(prior, span)
	current, _ := p.blob(p.base, name)
	if now, ok := lineByteSpan(current, lines); ok && bytes.Equal(spanBytes(current, now), text) {
		return lines, "", ""
	}
	_, byteRow := strings.CutPrefix(lines, bytesSpanPrefix)
	found := -1
	for offset := 0; offset <= len(current) && len(text) > 0; {
		index := bytes.Index(current[offset:], text)
		if index < 0 {
			break
		}
		start := offset + index
		end := start + len(text)
		if byteRow || (start == 0 || current[start-1] == '\n') && (end == len(current) || text[len(text)-1] == '\n') {
			if found >= 0 {
				found = -2
				break
			}
			found = start
		}
		offset = start + 1
	}
	if found < 0 {
		occurs := "no longer occur"
		if found == -2 {
			occurs = "occur more than once"
		}
		return lines, "", fmt.Sprintf("the bytes %s %s cited at the prior BASE %s %s at BASE %s", name, lines, p.priorBase, occurs, p.base)
	}
	moved := fmt.Sprintf("%s%d:%d", bytesSpanPrefix, found, found+len(text))
	if !byteRow {
		first := bytes.Count(current[:found], []byte("\n")) + 1
		last := first + bytes.Count(bytes.TrimSuffix(text, []byte("\n")), []byte("\n"))
		moved = fmt.Sprintf("%d:%d", first, last)
	}
	return moved, fmt.Sprintf("%s %s re-pinned to %s: the plan was drafted at BASE %s and BASE %s moved that text (DCW-V0-842)", name, lines, moved, p.priorBase, p.base), ""
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
