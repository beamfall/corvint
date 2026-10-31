package dogfoodflow

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// refinesClause is the part of a requirement entry's opening parenthesis that
// names the BASE requirements it refines or amends, up to the closing ')'.
var refinesClause = regexp.MustCompile(`\b(?:refines|refining|amends|amending)\b[^)]*`)

// maxRefinesLines bounds how many added lines of an entry are read for its
// refines clause: the definition line and the wrapped lines after it.
const maxRefinesLines = 4

// refinedRequirements maps each requirement ID the hunk defines to the IDs its
// entry says it refines or amends, then the other IDs its entry names, in
// order and without the ID itself (DCW-V0-967).
func refinedRequirements(hunk *patch.Hunk) map[string][]string {
	refined := map[string][]string{}
	for index, line := range hunk.Body {
		if line.Prefix != '+' {
			continue
		}
		ids := addedDefinitions(&patch.Hunk{Body: []patch.BodyLine{line}})
		if len(ids) != 1 {
			continue
		}
		var entry bytes.Buffer
		for next := index; next < len(hunk.Body) && next < index+maxRefinesLines; next++ {
			if next > index && (hunk.Body[next].Prefix != '+' || len(addedDefinitions(&patch.Hunk{Body: hunk.Body[next : next+1]})) > 0) {
				break
			}
			entry.Write(hunk.Body[next].Payload)
			entry.WriteByte(' ')
		}
		clause := refinesClause.Find(entry.Bytes())
		seen := map[string]bool{ids[0]: true}
		for _, text := range [][]byte{clause, entry.Bytes()} {
			for _, id := range requirementToken.FindAllString(string(text), -1) {
				if !seen[id] {
					seen[id] = true
					refined[ids[0]] = append(refined[ids[0]], id)
				}
			}
		}
		// The defining hunk's place in its spec, for insertionProposal.
		refined[ids[0]] = append(refined[ids[0]], insertionMarker)
	}
	return refined
}

// insertionMarker ends each refines list: past the named BASE IDs, the draft
// falls back to the nearest stable BASE entry or heading to the defining hunk.
const insertionMarker = ""

// insertionProposal cites, in the spec that defines id, the nearest stable,
// unused BASE requirement entry or heading above the hunk that adds it, or
// else below it.
func (p *proposal) insertionProposal(ordinal, id string, edited map[string][][2]int, used map[string]bool) (string, string) {
	// p.derived holds each hunk under two keys; sorted keys keep the pick
	// deterministic.
	for _, key := range sortedKeys(p.derived) {
		defining := p.derived[key]
		if !strings.HasPrefix(key, wire.HunkPrefix) || defining.DisplayPath != p.changeIDs[id] || !slices.Contains(addedDefinitions(defining), id) {
			continue
		}
		lines := p.baseLines(defining.DisplayPath)
		// Above the insertion first, nearest first, then below it.
		start := min(int(defining.OldRange.Start)+int(defining.OldRange.Count), len(lines))
		order := []int{}
		for line := start; line >= 1; line-- {
			order = append(order, line)
		}
		for line := start + 1; line <= len(lines); line++ {
			order = append(order, line)
		}
		for _, line := range order {
			text := string(lines[line-1])
			cite := fmt.Sprintf("%s:%d:%d", defining.DisplayPath, line, line)
			if !strings.HasPrefix(text, "- `") && !strings.HasPrefix(text, "#") || used[cite] || !p.stable(defining.DisplayPath, line, edited) {
				continue
			}
			p.reasons[ordinal] = fmt.Sprintf("the nearest stable BASE entry or heading to where %s adds the same-change requirement %s; no BASE requirement the entry names has a stable definition, so confirm it, or cite the BASE contract %s refines (DCW-V0-440, DCW-V0-967)", defining.DisplayPath, id, id)
			return fmt.Sprintf("%s\t%s\t%d:%d\t%s", ordinal, defining.DisplayPath, line, line, proposalRelation(defining.DisplayPath)), cite
		}
	}
	return "", ""
}

// refinedProposal drafts, for a hunk that defines or names the same-change
// requirement id, a cite of the first stable BASE definition line of
// a requirement id's entry refines, amends or names, then of the stable entry
// or heading above where id is added, marked as a proposal with that reason
// (DCW-V0-967). It returns "" when there is none.
func (p *proposal) refinedProposal(hunk proposedHunk, ordinal, id string, edited map[string][][2]int, used map[string]bool) (string, string) {
	for _, refined := range p.refines[id] {
		if refined == insertionMarker {
			if row, cite := p.insertionProposal(ordinal, id, edited, used); row != "" {
				p.proposals[ordinal] = true
				return row, cite
			}
			continue
		}
		// Every hunk implementing the same refined requirement may cite its
		// one definition line, an exception to the span dedupe of DCW-V0-382;
		// the uniform-plan rule (DCW-V0-048) still holds.
		for _, candidate := range p.matches[definitionPattern(refined)] {
			cite := fmt.Sprintf("%s:%d:%d", candidate.path, candidate.line, candidate.line)
			if !p.stable(candidate.path, candidate.line, edited) {
				continue
			}
			p.proposals[ordinal] = true
			p.reasons[ordinal] = fmt.Sprintf("the BASE definition of %s, which the entry of the same-change requirement %s refines, amends or names; same-change intent is not BASE evidence (DCW-V0-440), so confirm %s is the contract the hunk depends on (DCW-V0-967)", refined, id, refined)
			return fmt.Sprintf("%s\t%s\t%d:%d\t%s", ordinal, candidate.path, candidate.line, candidate.line, proposalRelation(candidate.path)), cite
		}
	}
	return "", ""
}

// definedRequirements returns the requirement IDs the hunk's added lines
// define (- `ID`:).
func (p *proposal) definedRequirements(hunk proposedHunk) []string {
	derived := p.derivedHunk(hunk)
	if derived == nil {
		return nil
	}
	return addedDefinitions(derived)
}

// ownDefinitionLines returns, in the hunk's own file, the BASE definition lines
// of the requirement IDs the hunk names, so a hunk editing a requirement's
// entry proposes that requirement's own stable heading line first (DCW-V0-967).
func (p *proposal) ownDefinitionLines(hunk proposedHunk) []int {
	_, ids := verify.SpanAnchors(p.derivedHunk(hunk), nil)
	lines := []int{}
	for _, id := range ids {
		for _, candidate := range p.matches[definitionPattern(id)] {
			if candidate.path == hunk.path {
				lines = append(lines, candidate.line)
			}
		}
	}
	return lines
}

// editedDefinitions names each BASE definition of a requirement the hunk names
// that a hunk of this change edits, so the unknown row says why that span
// cannot be cited: cem cite refuses it as cite-span-not-stable (DCW-V0-967).
func (p *proposal) editedDefinitions(hunk proposedHunk, edited map[string][][2]int) string {
	_, ids := verify.SpanAnchors(p.derivedHunk(hunk), nil)
	named := []string{}
	for _, id := range ids {
		for _, candidate := range p.matches[definitionPattern(id)] {
			if !p.stable(candidate.path, candidate.line, edited) {
				named = append(named, fmt.Sprintf("%s at %s:%d", id, candidate.path, candidate.line))
			}
		}
	}
	if len(named) == 0 {
		return ""
	}
	return "; the BASE definition of " + strings.Join(named, ", ") + " is edited by this change or repeated at HEAD, so cem cite would refuse it as cite-span-not-stable"
}

// sealedFallback is the DCW-V0-385 definition line of the daily workflow spec,
// cited for a prior seal's CEM when no line of sealContract can be (DCW-V0-966).
func (p *proposal) sealedFallback(ordinal string, edited map[string][][2]int, used map[string]bool) (string, string) {
	const spec = "docs/specs/daily-change-evidence-workflow-v0.md"
	for index, text := range p.baseLines(spec) {
		line := index + 1
		cite := fmt.Sprintf("%s:%d:%d", spec, line, line)
		if !bytes.HasPrefix(text, []byte(definitionPattern("DCW-V0-385"))) || used[cite] || !p.stable(spec, line, edited) {
			continue
		}
		return fmt.Sprintf("%s\t%s\t%d:%d\t%s", ordinal, spec, line, line, proposalRelation(spec)), cite
	}
	return "", ""
}

// allUnknown reports carried rows that keep their hunk unknown.
func allUnknown(rows []string) bool {
	for _, row := range rows {
		if !unknownRow(strings.Split(row, "\t")) {
			return false
		}
	}
	return len(rows) > 0
}
