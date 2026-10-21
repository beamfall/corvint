package dogfoodflow

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

// maxTierWarnings bounds the per-hunk tier warnings one check prints; the rest
// are counted on one closing line.
const maxTierWarnings = 50

// tierBelowAnchoredFix names the CEM-V1-081 anchor rules a hunk below
// `anchored` misses and how an author meets one of them.
const tierBelowAnchoredFix = "rule=CEM-V1-081 fix=\"cite a span that defines a requirement ID the hunk adds (- `ID`:) or that contains the hunk path or its basename of at least 6 bytes\""

// belowAnchoredWarnings reads the stable `cem status` line and returns one
// warning per hunk whose recomputed tier is below `anchored` (`cited` or
// `unknown`), naming its path and the anchor rule it misses. A policy cem-verdict
// gate with the touched-anchored or all-anchored predicate refuses such a hunk
// at closeout, after the map is sealed, so the check names it first (V1-1760,
// DCW-V0-112). The warning never changes the check's outcome. A line without a
// citation block yields none.
func belowAnchoredWarnings(cemLine []byte) []string {
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
		return nil
	}
	paths := map[string]string{}
	for _, hunk := range status.StableVerification.Hunks {
		paths[hunk.ID] = hunk.Path
	}
	lines := []string{}
	below := 0
	for _, row := range status.StableVerification.Citation.Hunks {
		if row.Tier != "cited" && row.Tier != "unknown" {
			continue
		}
		below++
		if below > maxTierWarnings {
			continue
		}
		lines = append(lines, fmt.Sprintf("dogfood-check: WARN tier-below-anchored path=%s hunk=%s tier=%s %s",
			strconv.Quote(paths[row.HunkID]), strconv.Quote(row.HunkID), summaryString(&row.Tier), tierBelowAnchoredFix))
	}
	if below > maxTierWarnings {
		lines = append(lines, fmt.Sprintf("dogfood-check: WARN tier-below-anchored more=%d", below-maxTierWarnings))
	}
	return lines
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
