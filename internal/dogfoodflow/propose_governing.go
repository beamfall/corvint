package dogfoodflow

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/patch"
)

// requirementDefinition is a BASE spec line that defines a requirement, in
// either list form the specs use: - `ID`: or - **ID:**.
var requirementDefinition = regexp.MustCompile("^- (?:`([A-Z][A-Z0-9-]{2,31}-[0-9]{3})`:|\\*\\*([A-Z][A-Z0-9-]{2,31}-[0-9]{3}):\\*\\*)")

// citedDecision is a decision number a requirement's definition line cites,
// such as "intent decision 0431" or "decision 0537".
var citedDecision = regexp.MustCompile(`\bdecisions? ([0-9]{4})\b`)

// proseToken is one word of at least four letters or one number; the
// governing-decision ranking counts the distinct tokens a line shares with
// the amended text.
var proseToken = regexp.MustCompile(`[A-Za-z]{4,}|[0-9]+`)

// minGoverningOverlap is the fewest distinct shared tokens that make a
// decision line the hunk's governing line; below it the first line naming the
// requirement is drafted instead.
const minGoverningOverlap = 3

// governingProposal drafts a hunk that amends a requirement of a spec the
// change modifies (DCW-V0-805): the BASE requirement whose definition opens
// the list item the hunk edits. The requirement's own row lies in the file
// the hunk modifies, which a draft never cites (DCW-V0-393), so the row is
// named in the note and the draft cites the decision that definition cites:
// the stable line of an untouched cited decision that shares the most words
// with the hunk's BASE text, else the first one naming the requirement. It
// returns "" for any other hunk or when no cited decision has such a line.
func (p *proposal) governingProposal(hunk proposedHunk, ordinal string, edited map[string][][2]int, used map[string]bool) (string, string, string) {
	derived := p.derivedHunk(hunk)
	if derived == nil || !strings.HasPrefix(hunk.path, "docs/specs/") || !strings.HasSuffix(hunk.path, ".md") {
		return "", "", ""
	}
	id, line := p.amendedRequirement(hunk.path, derived)
	if id == "" {
		return "", "", ""
	}
	definition := p.baseLines(hunk.path)[line-1]
	amended := map[string]bool{}
	for _, body := range derived.Body {
		if body.Prefix != '+' {
			for _, token := range proseToken.FindAll(body.Payload, -1) {
				amended[strings.ToLower(string(token))] = true
			}
		}
	}
	type candidate struct {
		path        string
		line, score int
	}
	var best, named *candidate
	for _, number := range citedDecision.FindAllSubmatch(definition, -1) {
		name := p.decisionPath(string(number[1]))
		if name == "" || p.changed[name] {
			continue
		}
		for index, text := range p.baseLines(name) {
			cite := fmt.Sprintf("%s:%d:%d", name, index+1, index+1)
			if used[cite] || !p.stable(name, index+1, edited) {
				continue
			}
			if named == nil && bytes.Contains(text, []byte(id)) {
				named = &candidate{name, index + 1, 0}
			}
			shared := map[string]bool{}
			for _, token := range proseToken.FindAll(text, -1) {
				if word := strings.ToLower(string(token)); amended[word] {
					shared[word] = true
				}
			}
			if len(shared) >= minGoverningOverlap && (best == nil || len(shared) > best.score) {
				best = &candidate{name, index + 1, len(shared)}
			}
		}
	}
	if best == nil {
		best = named
	}
	if best == nil {
		return "", "", ""
	}
	return fmt.Sprintf("%s\t%s\t%d:%d\t%s", ordinal, best.path, best.line, best.line, proposalRelation(best.path)),
		fmt.Sprintf("%s:%d:%d", best.path, best.line, best.line),
		fmt.Sprintf("the hunk amends requirement %s, defined at BASE %s:%d, which this change modifies (a review hint only); cited to the decision its definition cites (DCW-V0-805)", id, hunk.path, line)
}

// amendedRequirement returns the requirement ID and BASE line of the
// definition that opens the list item holding the hunk's first edited BASE
// line, or "" and 0 when a heading, a blank line, a table row or another
// list item comes first. A hunk that adds a requirement definition adds a
// requirement rather than amending one, and a pure insertion amends the
// item it extends only when its first added line continues that item.
func (p *proposal) amendedRequirement(name string, hunk *patch.Hunk) (string, int) {
	lines := p.baseLines(name)
	first, removed, added := int(hunk.OldRange.Start), false, []byte(nil)
	leading := true
	definitions := map[string]int{}
	for _, body := range hunk.Body {
		match := requirementDefinition.FindSubmatch(body.Payload)
		switch {
		case body.Prefix == ' ' && leading:
			first++
		case body.Prefix == '-':
			removed, leading = true, false
			if match != nil {
				definitions[string(match[1])+string(match[2])]--
			}
		case body.Prefix == '+':
			if match != nil {
				definitions[string(match[1])+string(match[2])]++
			}
			if added == nil {
				added = body.Payload
			}
			leading = false
		}
	}
	// A definition the hunk adds without removing it, rather than one it
	// rewrites in place, is a new requirement.
	for _, count := range definitions {
		if count > 0 {
			return "", 0
		}
	}
	if !removed {
		// The insertion follows line first-1, the last leading context line.
		if trimmed := bytes.TrimSpace(added); len(trimmed) == 0 || !bytes.HasPrefix(added, []byte(" ")) {
			return "", 0
		}
		first--
	}
	for line := min(first, len(lines)); line >= 1; line-- {
		text := lines[line-1]
		if match := requirementDefinition.FindSubmatch(text); match != nil {
			return string(match[1]) + string(match[2]), line
		}
		if len(bytes.TrimSpace(text)) == 0 || bytes.HasPrefix(text, []byte("#")) || bytes.HasPrefix(text, []byte("- ")) || bytes.HasPrefix(text, []byte("|")) {
			return "", 0
		}
	}
	return "", 0
}

// decisionPath returns the BASE path of decision number, such as
// docs/decisions/0431-<slug>.md, or "" when BASE has none or more than one.
func (p *proposal) decisionPath(number string) string {
	if p.decisions == nil {
		p.decisions = map[string]string{}
		out, status, bounded := p.boundedGit("ls-tree", "--name-only", p.base, "docs/decisions/")
		if status != 0 || !bounded {
			return ""
		}
		for _, name := range strings.Split(out, "\n") {
			base := strings.TrimPrefix(name, "docs/decisions/")
			if len(base) < 5 || base[4] != '-' || !strings.HasSuffix(base, ".md") {
				continue
			}
			if _, seen := p.decisions[base[:4]]; seen {
				p.decisions[base[:4]] = ""
			} else {
				p.decisions[base[:4]] = name
			}
		}
	}
	return p.decisions[number]
}
