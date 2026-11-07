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

// bareHeading recognizes ATX headings; setext needs its preceding paragraph.
func bareHeading(name string, text []byte) bool {
	if !strings.EqualFold(path.Ext(name), ".md") {
		return false
	}
	s := strings.TrimSpace(string(text))
	n := len(s) - len(strings.TrimLeft(s, "#"))
	return n >= 1 && n <= 6 && (n == len(s) || s[n] == ' ' || s[n] == '\t')
}

func paragraphLine(text []byte) bool {
	s := strings.TrimSpace(string(text))
	if s == "" || strings.Contains(s, ": ") || strings.HasPrefix(s, "- ") || strings.HasPrefix(s, "* ") || strings.HasPrefix(s, "+ ") || strings.HasPrefix(s, "#") || strings.HasPrefix(s, ">") || strings.HasPrefix(s, "|") || strings.HasSuffix(s, ":") {
		return false
	}
	if i := strings.IndexAny(s, ".)"); i > 0 && i < len(s)-1 && s[i+1] == ' ' {
		if _, err := strconv.Atoi(s[:i]); err == nil {
			return false
		}
	}
	return strings.Trim(s, "=-") != ""
}

func (p *proposal) headingLine(name string, line int) bool {
	lines := p.baseLines(name)
	if !strings.EqualFold(path.Ext(name), ".md") || line < 1 || line > len(lines) {
		return false
	}
	var fence byte
	width := 0
	for i, text := range lines[:line] {
		s := strings.TrimSpace(string(text))
		if len(s) > 0 && (s[0] == '`' || s[0] == '~') {
			n := len(s) - len(strings.TrimLeft(s, string(s[0])))
			if n >= 3 {
				if fence == 0 {
					fence, width = s[0], n
				} else if s[0] == fence && n >= width && strings.TrimSpace(s[n:]) == "" {
					fence = 0
				}
				if i == line-1 {
					return false
				}
				continue
			}
		}
		if i == line-1 && fence != 0 {
			return false
		}
	}
	if bareHeading(name, lines[line-1]) {
		return true
	}
	underline := func(text []byte) bool {
		s := strings.TrimSpace(string(text))
		return s != "" && (strings.Trim(s, "=") == "" || strings.Trim(s, "-") == "")
	}
	if line > 1 && underline(lines[line-1]) && paragraphLine(lines[line-2]) {
		return true
	}
	// DWC-V0-927: a setext heading's text may span several paragraph lines, so
	// every line of the paragraph an underline closes is a heading line.
	if !paragraphLine(lines[line-1]) {
		return false
	}
	for next := line; next < len(lines); next++ {
		if underline(lines[next]) {
			return true
		}
		if !paragraphLine(lines[next]) {
			return false
		}
	}
	return false
}

// draftSpanAllowed requires evidence for cross-file routes and preserves the
// deterministic operational and marked same-file proposals. Reviewed refresh rows are not redrafted.
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
	if len(bytes.TrimSpace(text)) == 0 {
		return false
	}
	from, _, _ := strings.Cut(byteSpan, ":")
	start, _ := strconv.Atoi(from)
	if !bytes.Contains(bytes.TrimSpace(text), []byte("\n")) && p.headingLine(name, bytes.Count(blob[:start], []byte("\n"))+1) {
		return false
	}
	kinds, _ := verify.SpanAnchors(p.derivedHunk(hunk), text)
	if len(kinds) > 0 || name == hunk.path {
		return true
	}
	// These deterministic contracts explain inclusion without pretending to
	// earn the CEM-V1-081 anchored tier.
	if sealedCEMPath.MatchString(hunk.path) {
		return name == sealContract && bytes.Contains(text, []byte(".corvint/changes/<bind")) || name == "docs/specs/daily-change-evidence-workflow-v0.md" && bytes.HasPrefix(text, []byte(definitionPattern("DCW-V0-385")))
	}
	return name == repinRuleSpec && p.derivedHunk(hunk) != nil && repinOnly(p.derivedHunk(hunk)) && bytes.HasPrefix(text, []byte(definitionPattern(repinRequirement)))
}

func (p *proposal) propose(hunk proposedHunk, edited map[string][][2]int, used map[string]bool) (string, string, string) {
	key := fmt.Sprint(hunk.ordinal)
	if p.keyByID {
		key = hunk.id
	}
	hint := ""
	if derived := p.derivedHunk(hunk); derived != nil && strings.HasPrefix(hunk.path, "docs/specs/") && strings.HasSuffix(hunk.path, ".md") {
		if id, line := p.amendedRequirement(hunk.path, derived); id != "" {
			span := fmt.Sprintf("%d:%d", line, line)
			hint = fmt.Sprintf("governing requirement %s at BASE %s:%d (DWC-V0-928)", id, hunk.path, line)
			if !used[hunk.path+":"+span] && p.definitionSurvives(hunk.path, span) {
				kinds, _ := verify.SpanAnchors(derived, p.baseLines(hunk.path)[line-1])
				if len(kinds) == 0 {
					p.proposals[key] = true
					p.reasons[key] = hint + "; anchored-by-id review note only: CEM-V1-081 cannot anchor this continuation edit because its added bytes do not name the ID; this is a same-file proposal"
				}
				return fmt.Sprintf("%s\t%s\t%s\tspecification", key, hunk.path, span), hunk.path + ":" + span, ""
			}
			hint += "; its definition is removed, repeated or already used: anchored-by-id review note only, never a cite of that definition"
		}
	}
	row, cite, note := p.proposeUnchecked(hunk, edited, used)
	f := strings.Split(row, "\t")
	if cite == "" || len(f) == 4 && p.draftSpanAllowed(hunk, f[1], f[2]) {
		if hint != "" {
			note += "; " + hint
		}
		return row, cite, note
	}
	delete(p.proposals, key)
	delete(p.reasons, key)
	// A fallback guess must not hide a valid path/basename/ID anchor.
	row, cite, filtered := p.anchoredProposal(hunk, key, edited, used)
	if row != "" {
		return row, cite, ""
	}
	return p.unknownProposal(hunk, key, "no substantive BASE span names the hunk path or basename (at least 6 bytes), or defines an ID the hunk adds or names (DWC-V0-926, DWC-V0-927)"+filtered+p.editedDefinitions(hunk, edited)+"; "+hint), "", ""
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

// Reject an unsupported route before it can displace a later same-file proposal.
func (p *proposal) allowedDraftRow(hunk proposedHunk, row string) bool {
	fields := strings.Split(row, "\t")
	if len(fields) != 4 || !p.draftSpanAllowed(hunk, fields[1], fields[2]) {
		key, _, _ := strings.Cut(row, "\t")
		delete(p.proposals, key)
		delete(p.reasons, key)
		return false
	}
	return true
}
