package doccorpus

import (
	"fmt"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// moduleObjectClaims attaches exported object keys to their owning module's
// searchable claim text, retaining exact source spans (DCP-V1-760).
func (c *compiler) moduleObjectClaims(provider Provider, in Input, id, text string) error {
	keys, ok := contextindex.WebExportedObjectKeys(in.Path, text)
	if !ok || len(keys) == 0 {
		return nil
	}
	lines := strings.Split(text, "\n")
	seen := map[string]bool{}
	for _, key := range keys {
		claimID := fmt.Sprintf("%s:object-key:%d:%s", id, key.Line, key.Name)
		if seen[claimID] {
			continue
		}
		seen[claimID] = true
		if len(c.artifact.Claims) >= c.manifest.recordLimit() {
			return fail(c.boundMessage("native record bound exceeded", in.Path, key.Line))
		}
		start := objectCommentStart(lines, key.Line)
		a, err := c.anchor(in.Revision, in.Path, start, key.Line, "syntax", "source", "exported object key and adjacent leading comments")
		if err != nil {
			return err
		}
		ev := Evidence{Derivation: "source-derived", Trust: "generated", State: "supported", Freshness: "fresh", Anchors: []Anchor{a}, Limitations: []string{"exact source excerpt; configuration semantics and runtime behavior unknown"}}
		c.artifact.Claims = append(c.artifact.Claims, Claim{ID: claimID, Subject: id, Text: strings.Join(lines[start-1:key.Line], "\n"), Provider: provider.ID, Evidence: ev})
	}
	return nil
}

// objectCommentStart includes only adjacent full-line // or complete /* */
// comments, bounded to 32 preceding lines and the existing excerpt byte cap.
// A blank line, code, incomplete block or over-bound block ends the walk.
func objectCommentStart(lines []string, line int) int {
	start, size := line, len(lines[line-1])+1
	for start > 1 && line-start < 32 {
		i := start - 2
		text := strings.TrimSpace(lines[i])
		first := i
		if !strings.HasPrefix(text, "//") {
			if !strings.HasSuffix(text, "*/") {
				break
			}
			for first >= 0 && line-first-1 <= 32 && !strings.Contains(lines[first], "/*") {
				first--
			}
			if first < 0 || line-first-1 > 32 || !strings.HasPrefix(strings.TrimSpace(lines[first]), "/*") {
				break
			}
			block := strings.TrimSpace(strings.Join(lines[first:i+1], "\n"))
			if strings.Index(block, "*/") != len(block)-2 {
				// Code after a closed block is not a leading comment.
				break
			}
		}
		added := 0
		for _, s := range lines[first : start-1] {
			added += len(s) + 1
		}
		if size+added > maxSpanBytes {
			break
		}
		size += added
		start = first + 1
	}
	return start
}
