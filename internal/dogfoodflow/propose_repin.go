package dogfoodflow

import (
	"bytes"
	"fmt"
	"regexp"

	"github.com/Beamfall/corvint/internal/cem/patch"
)

// repinContract is the BASE spec that defines the `--repin` rewrite of a
// pinned line citation, and repinRequirement the requirement whose definition
// line a repin-only hunk is drafted to (DCW-V0-548, V1-3545).
const (
	repinContract    = "docs/specs/documentation-citation-gate-v0.md"
	repinRequirement = "DCG-V0-045"
)

// citationPin matches the line part of a `path:line[-line][@hex]` citation
// token: what `script/check-line-citations.sh --repin` rewrites.
var citationPin = regexp.MustCompile(`:[0-9]+(?:[-:][0-9]+)?(?:@[0-9a-f]{4,64})?`)

// repinOnly reports whether every edited line of hunk pairs, in order, with a
// line of the other side that differs only in citation pins, and at least one
// pin changed. Any other edit, an insertion or a deletion makes it false.
func repinOnly(hunk *patch.Hunk) bool {
	var removed, added [][]byte
	for _, body := range hunk.Body {
		switch body.Prefix {
		case '-':
			removed = append(removed, body.Payload)
		case '+':
			added = append(added, body.Payload)
		}
	}
	if len(removed) == 0 || len(removed) != len(added) {
		return false
	}
	pinned := false
	for index := range removed {
		if bytes.Equal(removed[index], added[index]) {
			continue
		}
		before := citationPin.ReplaceAll(removed[index], []byte(":N"))
		after := citationPin.ReplaceAll(added[index], []byte(":N"))
		if !bytes.Equal(before, after) {
			return false
		}
		pinned = true
	}
	return pinned
}

// repinProposal drafts a hunk that only repins line citations to the BASE
// definition line of repinRequirement, the rule that rewrote it, instead of a
// same-file anchor (DCW-V0-548, V1-3545). It returns "" for any other hunk,
// or when that line is absent, edited by the change or already cited.
func (p *proposal) repinProposal(hunk proposedHunk, ordinal string, edited map[string][][2]int, used map[string]bool) (string, string, string) {
	derived := p.derivedHunk(hunk)
	if derived == nil || !repinOnly(derived) {
		return "", "", ""
	}
	prefix := []byte(definitionPattern(repinRequirement))
	for index, text := range p.baseLines(repinContract) {
		if !bytes.HasPrefix(text, prefix) {
			continue
		}
		cite := fmt.Sprintf("%s:%d:%d", repinContract, index+1, index+1)
		if used[cite] || !p.stable(repinContract, index+1, edited) {
			return "", "", ""
		}
		p.proposals[ordinal] = true
		return fmt.Sprintf("%s\t%s\t%d:%d\t%s", ordinal, repinContract, index+1, index+1, proposalRelation(repinContract)), cite,
			"the hunk changes only line-citation pins, which `script/check-line-citations.sh --repin` rewrites; cited to " + repinRequirement + " (DCW-V0-548)"
	}
	return "", "", ""
}
