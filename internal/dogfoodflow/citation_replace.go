package dogfoodflow

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// maxCitationReplacements bounds the DOGFOOD_CITATION_REPLACE entries one run
// admits (DCW-V0-391).
const maxCitationReplacements = 16

// citationMap is the part of the prepared map the plan checks read.
type citationMap struct {
	BaseRevision string `json:"baseRevision"`
	PatchSha256  string `json:"patchSha256"`
	Evidence     []struct {
		ID      string `json:"id"`
		BlobOid string `json:"blobOid"`
		Path    string `json:"path"`
		Span    struct {
			Start int64 `json:"start"`
			End   int64 `json:"end"`
		} `json:"span"`
	} `json:"evidence"`
	Hunks []struct {
		ID          string `json:"id"`
		Disposition string `json:"disposition"`
		Reason      string `json:"reason"`
		Basis       []struct {
			EvidenceID string `json:"evidenceId"`
			Relation   string `json:"relation"`
		} `json:"basis"`
	} `json:"hunks"`
}

func (c *change) readCitationMap() (*citationMap, bool) {
	data, err := readPrefix(c.path(".corvint/change.cem.json"), maxPlanBytes)
	if err != nil {
		return nil, false
	}
	var cem citationMap
	if json.Unmarshal(data, &cem) != nil {
		return nil, false
	}
	return &cem, true
}

// hunkDigest is the wire.HunkDigest of the hunk at index, the expected-old
// digest cem cite --replace binds to (CEM-V1-694).
func (m *citationMap) hunkDigest(index int) string {
	hunk := m.Hunks[index]
	basis := make([]wire.Basis, 0, len(hunk.Basis))
	for _, entry := range hunk.Basis {
		basis = append(basis, wire.Basis{EvidenceID: entry.EvidenceID, Relation: entry.Relation})
	}
	return wire.HunkDigest(m.BaseRevision, m.PatchSha256, hunk.ID, hunk.Disposition, hunk.Reason, basis)
}

// hunkIndex resolves a plan row's selector, an ordinal or a full hunk ID, or
// returns -1.
func (m *citationMap) hunkIndex(selector string) int {
	for position, candidate := range m.Hunks {
		if candidate.ID == selector || strconv.Itoa(position+1) == selector {
			return position
		}
	}
	return -1
}

func (m *citationMap) evidenceKeys() map[string]string {
	keys := map[string]string{}
	for _, record := range m.Evidence {
		keys[record.ID] = fmt.Sprintf("%s\x00%d:%d", record.Path, record.Span.Start, record.Span.End)
	}
	return keys
}

// rowHeld reports whether the hunk at index already holds what the plan row
// would write: the same citation in its basis, or for an unknown row the same
// unknown reason with no basis. resolved is false when the row's span does
// not resolve, which cem cite judges instead.
func (c *change) rowHeld(m *citationMap, evidence map[string]string, blobs map[string][]byte, index int, fields []string) (held, resolved bool) {
	hunk := m.Hunks[index]
	if unknownRow(fields) {
		return hunk.Disposition == "unknown" && hunk.Reason == fields[2] && len(hunk.Basis) == 0, true
	}
	span, ok := c.baseLineSpan(blobs, fields[1], fields[2])
	if !ok {
		return false, false
	}
	for _, basis := range hunk.Basis {
		if basis.Relation == fields[3] && evidence[basis.EvidenceID] == fields[1]+"\x00"+span {
			return true, true
		}
	}
	return false, true
}

// resolveCitationReplace checks DOGFOOD_CITATION_REPLACE against the plan and
// the prepared map before any cite or mark runs. Each HUNK_ID=DIGEST entry
// names, by full ID, a hunk the map already cites and the plan names, with
// the hunk's current expected-old digest; that hunk's earlier citations are
// then replaced by the plan's rows for it, and every other hunk is cited as
// before (DCW-V0-390). An entry whose hunk already holds exactly the plan's
// rows was applied by an earlier run, so a rerun of the same inputs replays
// (DCW-V0-392). It returns "" or the cem-cite refusal (DCW-V0-391).
func (c *change) resolveCitationReplace(plan []byte) string {
	value := c.options.CitationReplace
	if strings.TrimSpace(value) == "" {
		return ""
	}
	entries := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' })
	if len(entries) > maxCitationReplacements {
		return "citation-replace-invalid"
	}
	m, ok := c.readCitationMap()
	if !ok {
		return "citation-replace-invalid"
	}
	lines := readLines(plan)
	evidence, blobs := m.evidenceKeys(), map[string][]byte{}
	digests, selectors := map[string]string{}, map[string]string{}
	for _, entry := range entries {
		id, digest, found := strings.Cut(entry, "=")
		index := -1
		for position, hunk := range m.Hunks {
			if hunk.ID == id {
				index = position
			}
		}
		if !found || index < 0 || len(digest) != 64 || strings.Trim(digest, "0123456789abcdef") != "" {
			return "citation-replace-invalid"
		}
		if _, duplicate := digests[id]; duplicate {
			return "citation-replace-invalid"
		}
		rows := [][]string{}
		for _, line := range lines {
			fields := strings.SplitN(line, "\t", 4)
			if len(fields) == 4 && m.hunkIndex(fields[0]) == index {
				rows = append(rows, fields)
				selectors[fields[0]] = id
			}
		}
		if len(rows) == 0 {
			return "citation-replace-invalid"
		}
		current := m.hunkDigest(index)
		if digest == current {
			digests[id] = digest
			continue
		}
		if c.rowsAlreadyApplied(m, evidence, blobs, index, rows) {
			c.replaceReplayed = append(c.replaceReplayed, id)
			for selector, owner := range selectors {
				if owner == id {
					delete(selectors, selector)
				}
			}
			continue
		}
		c.recitedHunk, c.recitedDigest = id, current
		return "citation-replace-digest-mismatch"
	}
	c.replaceDigests, c.replaceSelectors = digests, selectors
	return ""
}

// rowsAlreadyApplied reports a hunk whose citation state is exactly what its
// plan rows write: each row is held and the basis holds nothing else.
func (c *change) rowsAlreadyApplied(m *citationMap, evidence map[string]string, blobs map[string][]byte, index int, rows [][]string) bool {
	cites := 0
	keys := []string{}
	for _, fields := range rows {
		held, resolved := c.rowHeld(m, evidence, blobs, index, fields)
		if !held || !resolved {
			return false
		}
		if unknownRow(fields) {
			continue
		}
		key := strings.Join(fields[1:], "\t")
		if !slices.Contains(keys, key) {
			keys = append(keys, key)
			cites++
		}
	}
	return cites == len(m.Hunks[index].Basis)
}

// replaceArgs are the guard arguments for the first row the plan writes to a
// replaced hunk; later rows for that hunk add to the replacement.
func (c *change) replaceArgs(fields []string, replaced map[string]bool) []string {
	id, ok := c.replaceSelectors[fields[0]]
	if !ok || replaced[id] {
		return nil
	}
	replaced[id] = true
	c.replaceApplied = append(c.replaceApplied, id)
	if unknownRow(fields) {
		return []string{"--expected-hunk-digest", c.replaceDigests[id]}
	}
	return []string{"--replace", "--expected-hunk-digest", c.replaceDigests[id]}
}
