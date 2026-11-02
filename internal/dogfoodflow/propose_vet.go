package dogfoodflow

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/sim"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
)

// removedLines is one hunk's run of BASE lines it removes, not its context.
type removedLines struct {
	hunk       string
	start, end int
}

// vetRows judges every cite row of the draft, drafted or carried, by the
// span rule cem cite applies, and prints a row it would refuse as unknown
// with the refusal named, so no printed span is later refused
// cite-span-not-stable (DCW-V0-840). A row whose lines overlap lines a hunk
// removes is refused as well, naming that hunk (V1-3595).
func (p *proposal) vetRows(owed []proposedHunk, rows []string) ([]string, []string) {
	keyed := map[string]proposedHunk{}
	for _, hunk := range owed {
		keyed[strconv.Itoa(hunk.ordinal)], keyed[hunk.id] = hunk, hunk
	}
	groups := map[string]*patch.Group{}
	removed := map[string][]removedLines{}
	if parsed, ok := p.canonicalPatch(); ok {
		for _, group := range parsed.Groups {
			if group.OldPath == nil {
				continue
			}
			groups[*group.OldPath] = group
			for _, hunk := range group.Hunks {
				removed[*group.OldPath] = append(removed[*group.OldPath], hunkRemovals(hunk)...)
			}
		}
	}
	notes := []string{}
	taken := map[string]bool{}
	for _, row := range rows {
		if fields := strings.Split(row, "\t"); len(fields) == 4 && fields[1] != "unknown" {
			taken[fields[1]+":"+fields[2]] = true
		}
	}
	for index, row := range rows {
		fields := strings.Split(row, "\t")
		if len(fields) != 4 || fields[1] == "unknown" {
			continue
		}
		hunk, ok := keyed[fields[0]]
		if !ok {
			continue
		}
		refusal := p.citeRefusal(fields[1], fields[2], groups[fields[1]], removed[fields[1]])
		if refusal == "" {
			continue
		}
		if span := p.nearestSurvivingSpan(fields[1], fields[2], refusal, groups[fields[1]], removed[fields[1]], taken); span != "" {
			delete(taken, fields[1]+":"+fields[2])
			taken[fields[1]+":"+span] = true
			rows[index] = fields[0] + "\t" + fields[1] + "\t" + span + "\t" + fields[3]
			if _, ok := p.proposals[fields[0]]; ok {
				p.reasons[fields[0]] = fmt.Sprintf("nearest surviving span: %s %s was moved to %s, the nearest BASE lines the change leaves intact; confirm it still supports the hunk (DCW-V0-980)", fields[1], fields[2], span)
			}
			notes = append(notes, fmt.Sprintf("row %s: the cite row %s:%s was re-pinned to the nearest surviving span %s:%s: %s (DCW-V0-980)", fields[0], fields[1], fields[2], fields[1], span, refusal))
			continue
		}
		delete(p.proposals, fields[0])
		delete(p.reasons, fields[0])
		rows[index] = p.unknownProposal(hunk, fields[0], fmt.Sprintf("the cite row %s %s %s was not kept: %s, so cem cite would refuse it; cite a BASE span the change leaves intact (DCW-V0-840)", fields[1], fields[2], fields[3], refusal))
		notes = append(notes, fmt.Sprintf("row %s: the cite row %s:%s was replaced by an unknown row: %s (DCW-V0-840)", fields[0], fields[1], fields[2], refusal))
	}
	return rows, notes
}

// hunkRemovals lists the runs of BASE lines hunk removes.
func hunkRemovals(hunk *patch.Hunk) []removedLines {
	runs := []removedLines{}
	line := int(hunk.OldRange.Start)
	for _, body := range hunk.Body {
		switch body.Prefix {
		case '-':
			if count := len(runs); count > 0 && runs[count-1].end == line-1 {
				runs[count-1].end = line
			} else {
				runs = append(runs, removedLines{hunk.ID, line, line})
			}
			line++
		case ' ':
			line++
		}
	}
	return runs
}

// citeRefusal names why cem cite would refuse BASE lines of name, or "" when
// it admits them; group is the patch group of name, nil when the patch has
// none, and removed the runs of BASE lines its hunks remove.
func (p *proposal) citeRefusal(name, lines string, group *patch.Group, removed []removedLines) string {
	// A bytes:START:END row names the byte span the binder cites directly
	// (DCW-V0-990); its BASE lines are derived for the removed-lines check.
	byteRow := strings.HasPrefix(lines, bytesSpanPrefix)
	startText, endText, found := strings.Cut(strings.TrimPrefix(lines, bytesSpanPrefix), ":")
	start, startErr := strconv.Atoi(startText)
	end, endErr := strconv.Atoi(endText)
	if !found || startErr != nil || endErr != nil || start < 0 || !byteRow && start < 1 || end < start {
		return "its span " + lines + " is not START:END or bytes:START:END"
	}
	blob, ok := p.blob(p.base, name)
	if !ok {
		return name + " is absent at BASE or not a regular file"
	}
	byteSpan, ok := lineByteSpan(blob, lines)
	if !ok {
		if byteRow {
			return "bytes " + lines + " are outside " + name + " at BASE"
		}
		return "lines " + lines + " are outside " + name + " at BASE"
	}
	from, to, _ := strings.Cut(byteSpan, ":")
	spanStart, _ := strconv.ParseInt(from, 10, 64)
	spanEnd, _ := strconv.ParseInt(to, 10, 64)
	if byteRow {
		start = bytes.Count(blob[:spanStart], []byte("\n")) + 1
		end = start + bytes.Count(bytes.TrimSuffix(blob[spanStart:spanEnd], []byte("\n")), []byte("\n"))
	}
	if spanEnd-spanStart > wire.MaxCitationSpanBytes {
		return fmt.Sprintf("its span is %d bytes, over the %d-byte bound", spanEnd-spanStart, wire.MaxCitationSpanBytes)
	}
	if workflow.GoverningInstructionPath(name) && p.changed[name] {
		return name + " is a governing instruction file this change modifies"
	}
	if group == nil {
		return ""
	}
	if group.Binary {
		return name + " changes as a binary file"
	}
	for _, run := range removed {
		if start <= run.end && run.start <= end {
			return fmt.Sprintf("lines %s overlap BASE lines %d:%d that hunk %s removes", lines, run.start, run.end, run.hunk)
		}
	}
	patched, err := sim.ApplyHunks(group, blob)
	if err != nil {
		return "the patch of " + name + " does not apply to its BASE blob"
	}
	status, _ := verify.ClassifySpan(group.NewPath != nil && *group.NewPath == name, bytes.Equal(blob, patched), patched, blob[spanStart:spanEnd], wire.Span{Start: spanStart, End: spanEnd})
	if status == verify.DriftStable || status == verify.DriftRelocated {
		return ""
	}
	return fmt.Sprintf("its text is %s after the change (it must occur exactly once in the changed file)", status)
}

// nearestSurvivingSpanReach bounds how many lines past the refused span
// nearestSurvivingSpan looks for a line the change leaves intact.
const nearestSurvivingSpanReach = 8

// nearestSurvivingSpan returns a START:END span of name near the refused
// line span lines that cem cite admits, or "" when there is none (DCW-V0-980).
// Only a span refused for overlapping removed lines or for unstable text is
// moved: the longest part of it no hunk removes is tried first, then single
// lines outward from it, nearest first and the earlier line on a tie. A span
// another draft row already cites is skipped, so no two rows are made one.
func (p *proposal) nearestSurvivingSpan(name, lines, refusal string, group *patch.Group, removed []removedLines, taken map[string]bool) string {
	if strings.HasPrefix(lines, bytesSpanPrefix) || !strings.Contains(refusal, " overlap BASE lines ") && !strings.HasPrefix(refusal, "its text is ") {
		return ""
	}
	startText, endText, _ := strings.Cut(lines, ":")
	start, startErr := strconv.Atoi(startText)
	end, endErr := strconv.Atoi(endText)
	if startErr != nil || endErr != nil || start < 1 || end < start {
		return ""
	}
	candidates := []string{}
	for _, part := range survivingParts(start, end, removed) {
		if part[0] != start || part[1] != end {
			candidates = append(candidates, fmt.Sprintf("%d:%d", part[0], part[1]))
		}
	}
	for distance := 1; distance <= nearestSurvivingSpanReach; distance++ {
		for _, line := range []int{start - distance, end + distance} {
			if line >= 1 {
				candidates = append(candidates, fmt.Sprintf("%d:%d", line, line))
			}
		}
	}
	for _, candidate := range candidates {
		if !taken[name+":"+candidate] && p.citeRefusal(name, candidate, group, removed) == "" {
			return candidate
		}
	}
	return ""
}

// survivingParts splits start..end around the removed runs, longest first and
// the earlier part on a tie.
func survivingParts(start, end int, removed []removedLines) [][2]int {
	parts := [][2]int{{start, end}}
	for _, run := range removed {
		next := [][2]int{}
		for _, part := range parts {
			if run.end < part[0] || part[1] < run.start {
				next = append(next, part)
				continue
			}
			if part[0] < run.start {
				next = append(next, [2]int{part[0], run.start - 1})
			}
			if run.end < part[1] {
				next = append(next, [2]int{run.end + 1, part[1]})
			}
		}
		parts = next
	}
	sort.SliceStable(parts, func(i, j int) bool { return parts[i][1]-parts[i][0] > parts[j][1]-parts[j][0] })
	return parts
}
