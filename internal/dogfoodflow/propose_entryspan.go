package dogfoodflow

import (
	"bytes"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// entryEnd returns the last BASE line of the requirement entry whose stable
// definition line (- `ID`:) is line, so a drafted cite covers the requirement
// rather than its first line (DCW-V0-748). The entry runs over the indented
// continuation lines that follow, up to a blank, unindented or edited line.
// The span must still be cited as cem cite would admit it: under the citation
// byte bound and occurring exactly once at BASE and after the change; it is
// shortened until it is, down to the definition line itself.
func (p *proposal) entryEnd(name string, line int, edited map[string][][2]int) int {
	lines := p.baseLines(name)
	if line < 1 || line > len(lines) || !bytes.HasPrefix(lines[line-1], []byte("- `")) {
		return line
	}
	end := line
	for next := line + 1; next <= len(lines); next++ {
		text := lines[next-1]
		if len(bytes.TrimSpace(text)) == 0 || (text[0] != ' ' && text[0] != '\t') || p.editedLine(name, next, edited) {
			break
		}
		end = next
	}
	baseBlob, _ := p.blob(p.base, name)
	targetBlob, _ := p.blob(p.target, name)
	for ; end > line; end-- {
		span := bytes.Join(lines[line-1:end], nil)
		if len(span) <= wire.MaxCitationSpanBytes && spanOccurrences(baseBlob, span) == 1 && spanOccurrences(targetBlob, span) == 1 {
			break
		}
	}
	return end
}

// cutLast splits text around the last sep.
func cutLast(text, sep string) (before, after string, found bool) {
	if index := strings.LastIndex(text, sep); index >= 0 {
		return text[:index], text[index+len(sep):], true
	}
	return text, "", false
}

// editedLine reports whether a hunk of the change covers BASE line of name.
func (p *proposal) editedLine(name string, line int, edited map[string][][2]int) bool {
	for _, span := range edited[name] {
		if line >= span[0] && line <= span[1] {
			return true
		}
	}
	return false
}
