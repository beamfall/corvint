package dogfoodflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// refreshing reports whether the draft carries rows from --refresh or
// --from-cem (DCW-V0-842, DCW-V0-843).
func (p *proposal) refreshing() bool {
	return p.options.Refresh != "" || p.options.FromCEM != ""
}

// readRegularInput reads an operator-named file of at most maxPlanBytes. A
// non-regular file such as a FIFO is refused before it is opened, which
// could otherwise block with no writer.
func readRegularInput(name string) ([]byte, error) {
	if !filepath.IsAbs(name) {
		if wd, err := os.Getwd(); err == nil {
			name = filepath.Join(wd, name)
		}
	}
	if !isRegular(name) {
		return nil, errors.New("not a regular file")
	}
	data, err := readPrefix(name, maxPlanBytes+1)
	if err == nil && len(data) > maxPlanBytes {
		err = errors.New("over the plan bound")
	}
	return data, err
}

// sealedCEMPlan renders the --from-cem CEM as refresh plan rows keyed by
// hunk ID (DCW-V0-843): a cite row per basis evidence, its byte span read as
// the lines of the evidence blob at the CEM's base, and an unknown row with
// the CEM's reason for a hunk it marks unknown. The refresh then carries the
// rows to the current hunks as DCW-V0-842 does, with the CEM's base as the
// prior BASE.
func (p *proposal) sealedCEMPlan() []byte {
	data, err := readRegularInput(p.options.FromCEM)
	var cem citationMap
	if err == nil {
		err = json.Unmarshal(data, &cem)
	}
	if err == nil && (cem.BaseRevision == "" || len(cem.Hunks) == 0) {
		err = errors.New("no baseRevision or hunks")
	}
	if err != nil {
		p.say("%s: REFUSE from-cem-unreadable\n  --from-cem %s is not a readable change evidence map of at most %d bytes with a baseRevision and hunks\n  fix: name a sealed CEM such as .corvint/changes/COMMIT.cem.json, or draft without --from-cem (DCW-V0-843)\n",
			p.prefix, strconv.Quote(p.options.FromCEM), maxPlanBytes)
		p.exit(2)
	}
	p.cemBase = cem.BaseRevision
	evidence := map[string]int{}
	for index, entry := range cem.Evidence {
		evidence[entry.ID] = index
	}
	rows := []string{}
	for _, hunk := range cem.Hunks {
		if len(hunk.Basis) == 0 {
			if hunk.Disposition == "unknown" {
				// A plan row's reason is insufficient-evidence or
				// conflicting-evidence; any other CEM reason, such as an
				// allowed bootstrap hunk's no-evidence, is kept in the NOTE.
				reason, note := hunk.Reason, ""
				if reason != "insufficient-evidence" && reason != "conflicting-evidence" {
					if reason != "" && !strings.ContainsAny(reason, "\t\n") {
						note = " with reason " + reason
					}
					reason = "insufficient-evidence"
				}
				rows = append(rows, hunk.ID+"\tunknown\t"+reason+"\tcarried from the sealed CEM "+filepath.Base(p.options.FromCEM)+note+" (DCW-V0-843)")
			}
			continue
		}
		for _, basis := range hunk.Basis {
			index, ok := evidence[basis.EvidenceID]
			if !ok {
				continue
			}
			entry := cem.Evidence[index]
			blob, status := p.git(true, "cat-file", "blob", entry.BlobOid)
			lines, ok := byteSpanLines([]byte(blob), entry.Span.Start, entry.Span.End)
			if status != 0 || !ok || strings.ContainsAny(entry.Path+basis.Relation, "\t\n") {
				continue
			}
			rows = append(rows, hunk.ID+"\t"+entry.Path+"\t"+lines+"\t"+basis.Relation)
		}
	}
	return []byte(strings.Join(rows, "\n") + "\n")
}

// byteSpanLines names the 1-based START:END lines a byte span of blob covers.
func byteSpanLines(blob []byte, start, end int64) (string, bool) {
	if start < 0 || end <= start || end > int64(len(blob)) {
		return "", false
	}
	first := bytes.Count(blob[:start], []byte("\n")) + 1
	last := bytes.Count(blob[:end-1], []byte("\n")) + 1
	return strconv.Itoa(first) + ":" + strconv.Itoa(last), true
}
