// SPDX-License-Identifier: Apache-2.0

package dogfoodflow

import (
	"bytes"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/verify"
)

// bareHeading excludes ATX and setext heading lines from automatic drafts.
func bareHeading(name string, text []byte) bool {
	if !strings.EqualFold(path.Ext(name), ".md") {
		return false
	}
	s := strings.TrimSpace(string(text))
	if s == "" {
		return false
	}
	if strings.Trim(s, "=") == "" || strings.Trim(s, "-") == "" {
		return true
	}
	n := len(s) - len(strings.TrimLeft(s, "#"))
	return n >= 1 && n <= 6 && (n == len(s) || s[n] == ' ' || s[n] == '\t')
}

func (p *proposal) headingLine(name string, line int) bool {
	lines := p.baseLines(name)
	if line < 1 || line > len(lines) {
		return false
	}
	if bareHeading(name, lines[line-1]) {
		return true
	}
	if strings.EqualFold(path.Ext(name), ".md") && line < len(lines) {
		next := strings.TrimSpace(string(lines[line]))
		return next != "" && (strings.Trim(next, "=") == "" || strings.Trim(next, "-") == "")
	}
	return false
}

// draftSpanAllowed applies the same anchor rule to every automatic route,
// including fallback proposals. Reviewed refresh rows are not redrafted.
func (p *proposal) draftSpanAllowed(hunk proposedHunk, name, span string) bool {
	blob, ok := p.blob(p.base, name)
	if !ok {
		return false
	}
	byteSpan, ok := lineByteSpan(blob, span)
	if !ok {
		return false
	}
	text := spanBytes(blob, byteSpan)
	if len(text) == 0 || bareHeading(name, text) {
		return false
	}
	from, _, _ := strings.Cut(byteSpan, ":")
	start, _ := strconv.Atoi(from)
	if !bytes.Contains(bytes.TrimSpace(text), []byte("\n")) && p.headingLine(name, bytes.Count(blob[:start], []byte("\n"))+1) {
		return false
	}
	kinds, _ := verify.SpanAnchors(p.derivedHunk(hunk), text)
	return len(kinds) > 0
}

func (p *proposal) propose(hunk proposedHunk, edited map[string][][2]int, used map[string]bool) (string, string, string) {
	key := fmt.Sprint(hunk.ordinal)
	if p.keyByID {
		key = hunk.id
	}
	// Prefer the governing BASE definition even when it is unchanged hunk
	// context. The precise verifier below rejects removed/repeated text.
	if p.derivedHunk(hunk) != nil && strings.HasPrefix(hunk.path, "docs/specs/") && strings.HasSuffix(hunk.path, ".md") {
		if id, line := p.amendedRequirement(hunk.path, p.derivedHunk(hunk)); id != "" &&
			bytes.HasPrefix(p.baseLines(hunk.path)[line-1], []byte(definitionPattern(id))) {
			span := fmt.Sprintf("%d:%d", line, line)
			if p.draftSpanAllowed(hunk, hunk.path, span) && p.definitionSurvives(hunk.path, span) {
				p.proposals[key] = true
				p.reasons[key] = "the stable BASE definition of governing requirement " + id + " (DWC-V0-928)"
				return fmt.Sprintf("%s\t%s\t%s\tspecification", key, hunk.path, span), hunk.path + ":" + span, ""
			}
		}
	}
	row, cite, note := p.proposeUnchecked(hunk, edited, used)
	f := strings.Split(row, "\t")
	if cite == "" || len(f) == 4 && p.draftSpanAllowed(hunk, f[1], f[2]) {
		return row, cite, note
	}
	delete(p.proposals, key)
	delete(p.reasons, key)
	// A fallback guess must not hide a valid path/basename/ID anchor.
	row, cite, filtered := p.anchoredProposal(hunk, key, edited, used)
	if row != "" {
		return row, cite, ""
	}
	return p.unknownProposal(hunk, key, "no substantive BASE span names the hunk path or basename (at least 6 bytes), or defines an ID the hunk adds or names (DWC-V0-926, DWC-V0-927)"+filtered+p.editedDefinitions(hunk, edited)), "", ""
}

func (p *proposal) definitionSurvives(name, span string) bool {
	parsed, ok := p.canonicalPatch()
	if !ok {
		return false
	}
	for _, group := range parsed.Groups {
		if group.OldPath != nil && *group.OldPath == name {
			removed := []removedLines{}
			for _, h := range group.Hunks {
				removed = append(removed, hunkRemovals(h)...)
			}
			return p.citeRefusal(name, span, group, removed) == ""
		}
	}
	return false
}
