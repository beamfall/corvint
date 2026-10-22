package dogfoodflow

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
)

// maxTierWarnings bounds the per-hunk tier warnings one check prints; the rest
// are counted on one closing line.
const maxTierWarnings = 50

// tierBelowAnchoredFix names the CEM-V1-081 anchor rules a hunk below
// `anchored` misses and how an author meets one of them.
const tierBelowAnchoredFix = "rule=CEM-V1-081 fix=\"cite a span that defines a requirement ID the hunk adds (- `ID`:) or that contains the hunk path or its basename of at least 6 bytes\""

// tierBelowAnchoredUnavailable names why the hunks a NOTE line counts carry no
// tier warning.
const tierBelowAnchoredUnavailable = "rule=CEM-V1-081 reason=\"no stable BASE line names the hunk path, or its basename of at least 6 bytes, or defines a requirement ID the hunk adds, so no citation can anchor it; a touched-anchored or all-anchored cem-verdict gate still refuses it\""

// tierRows reads the stable `cem status` line and returns, in citation order,
// the hunks whose recomputed tier is below `anchored` (`cited` or `unknown`)
// with their paths and tiers. A line without a citation block yields none.
func tierRows(cemLine []byte) (below []string, paths, tiers map[string]string) {
	var status struct {
		StableVerification *struct {
			Hunks []struct {
				ID   string `json:"id"`
				Path string `json:"path"`
			} `json:"hunks"`
			Citation *struct {
				Hunks []struct {
					HunkID string `json:"hunkId"`
					Tier   string `json:"tier"`
				} `json:"hunks"`
			} `json:"citation"`
		} `json:"stableVerification"`
	}
	if json.Unmarshal(cemLine, &status) != nil || status.StableVerification == nil || status.StableVerification.Citation == nil {
		return nil, nil, nil
	}
	paths, tiers = map[string]string{}, map[string]string{}
	for _, hunk := range status.StableVerification.Hunks {
		paths[hunk.ID] = hunk.Path
	}
	for _, row := range status.StableVerification.Citation.Hunks {
		if row.Tier == "cited" || row.Tier == "unknown" {
			below = append(below, row.HunkID)
			tiers[row.HunkID] = row.Tier
		}
	}
	return below, paths, tiers
}

// belowAnchoredWarnings returns one warning per hunk whose recomputed tier is
// below `anchored`, naming its path and the anchor rule it misses. A policy
// cem-verdict gate with the touched-anchored or all-anchored predicate refuses
// such a hunk at closeout, after the map is sealed, so the check names it
// first (V1-1760, DCW-V0-112). A hunk in unanchorable, for which no citation
// could earn an anchor, gets no warning: one NOTE line counts them instead
// (DCW-V0-165). The lines never change the check's outcome.
func belowAnchoredWarnings(cemLine []byte, unanchorable map[string]bool) []string {
	below, paths, tiers := tierRows(cemLine)
	lines := []string{}
	warned, skipped := 0, 0
	for _, id := range below {
		if unanchorable[id] {
			skipped++
			continue
		}
		warned++
		if warned > maxTierWarnings {
			continue
		}
		tier := tiers[id]
		lines = append(lines, fmt.Sprintf("dogfood-check: WARN tier-below-anchored path=%s hunk=%s tier=%s %s",
			strconv.Quote(paths[id]), strconv.Quote(id), summaryString(&tier), tierBelowAnchoredFix))
	}
	if warned > maxTierWarnings {
		lines = append(lines, fmt.Sprintf("dogfood-check: WARN tier-below-anchored more=%d", warned-maxTierWarnings))
	}
	if skipped > 0 {
		lines = append(lines, fmt.Sprintf("dogfood-check: NOTE tier-below-anchored-unanchorable hunks=%d %s", skipped, tierBelowAnchoredUnavailable))
	}
	return lines
}

// tierWarnings returns the check's CEM-V1-081 tier lines for cemLine.
func (c *check) tierWarnings(cemLine []byte) []string {
	return belowAnchoredWarnings(cemLine, c.unanchorableHunks(cemLine))
}

// unanchorableHunks returns the below-anchored hunks of cemLine for which the
// DCW-V0-161 search, widened to path basenames of 6 bytes or more, finds no
// stable BASE line that earns a CEM-V1-081 anchor: none defines a requirement
// ID the hunk adds and none names its path or that basename. It
// reads the bound map in the clean worktree, so its hunks and old ranges are
// the ones the status verified. Any read it cannot complete (an unreadable
// map, an oversized or failed patch or grep, a Git refusal) returns none, so
// every such hunk keeps its warning (DCW-V0-165).
func (c *check) unanchorableHunks(cemLine []byte) (unanchorable map[string]bool) {
	below, _, _ := tierRows(cemLine)
	if len(below) == 0 {
		return nil
	}
	data, err := readPrefix(filepath.Join(c.root, ".corvint", "change.cem.json"), maxPlanBytes)
	if err != nil {
		return nil
	}
	baseRevision, hunks, ok := preparedHunks(data)
	if !ok || baseRevision != c.base {
		return nil
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			if unwound, isStop := recovered.(stop); !isStop || unwound.code == -1 {
				panic(recovered)
			}
			unanchorable = nil
		}
	}()
	searched := c.flow
	searched.stderr = io.Discard
	search := &proposal{flow: searched, blobs: map[string][]byte{}, lines: map[string][][]byte{}, basenames: true}
	wanted := map[string]bool{}
	for _, id := range below {
		wanted[id] = true
	}
	owed := []proposedHunk{}
	for _, hunk := range hunks {
		if wanted[hunk.id] {
			owed = append(owed, hunk)
		}
	}
	if !search.prepareAnchors(owed) {
		return nil
	}
	edited := editedRanges(hunks)
	unanchorable = map[string]bool{}
	for _, hunk := range owed {
		if row, _ := search.anchoredProposal(hunk, "", edited, map[string]bool{}); row == "" {
			unanchorable[hunk.id] = true
		}
	}
	return unanchorable
}

// writeLines writes each line followed by a newline.
func writeLines(out io.Writer, lines []string) {
	if out == nil {
		return
	}
	for _, line := range lines {
		_, _ = io.WriteString(out, line+"\n")
	}
}
