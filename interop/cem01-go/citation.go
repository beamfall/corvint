package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

// Consumer-computed citation support (decision 0523 P0; CEM-V1-080..082).
// Independently authored from the pinned rules; it shares no code with the
// native verifier. Nothing here is a verdict: the block never changes the
// outcome, codes or drift of the verification that carries it.

const citationProfile = "cem-citation-support/0"

// citationStopTerms are the CEM-V1-081 annotation stop terms: the
// lexical-relevance-floor-v0 stop list less the 21 terms that are code
// keywords or builtins (V1-1335), kept sorted.
var citationStopTerms = map[string]bool{}

func init() {
	for _, w := range []string{
		"also", "been", "build", "change", "changed", "changes", "claim", "dist", "docs", "document",
		"documentation", "evidence", "generated", "have", "hunk", "must", "only", "packages", "report",
		"reports", "requirement", "requirements", "shall", "should", "sidecar", "sources", "supported",
		"testing", "tests", "that", "their", "there", "these", "they", "those", "vendor", "vendored",
		"which", "will",
	} {
		citationStopTerms[w] = true
	}
}

type citationHunk struct {
	HunkID     string   `json:"hunkId"`
	Tier       string   `json:"tier"`
	Anchors    []string `json:"anchors"`
	Annotation bool     `json:"annotation"`
}

type citationClasses struct {
	Held             int `json:"held"`
	MechanicalProved int `json:"mechanical-proved"`
	Anchored         int `json:"anchored"`
	Cited            int `json:"cited"`
	Unknown          int `json:"unknown"`
}

// citationFanOut renders as the two-element array [evidenceId, hunks].
type citationFanOut struct {
	EvidenceID string
	Hunks      int
}

func (f citationFanOut) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{f.EvidenceID, f.Hunks})
}

func (f *citationFanOut) UnmarshalJSON(b []byte) error {
	var pair []json.RawMessage
	if err := json.Unmarshal(b, &pair); err != nil {
		return err
	}
	if len(pair) != 2 {
		return errors.New("evidenceFanOut entry is not a pair")
	}
	if err := json.Unmarshal(pair[0], &f.EvidenceID); err != nil {
		return err
	}
	return json.Unmarshal(pair[1], &f.Hunks)
}

type citationBlock struct {
	Profile           string           `json:"profile"`
	Hunks             []citationHunk   `json:"hunks"`
	HunksByClass      citationClasses  `json:"hunksByClass"`
	AnnotationHunks   int              `json:"annotationHunks"`
	DistinctEvidence  int              `json:"distinctEvidence"`
	EvidenceFanOut    []citationFanOut `json:"evidenceFanOut"`
	MaxFanOut         int              `json:"maxFanOut"`
	SamePathCitations int              `json:"samePathCitations"`
	ConcentratedHunks int              `json:"concentratedHunks"`
	Invariant         bool             `json:"invariant"`
	Specificity       int              `json:"specificity"`
}

// citationIDByte reports membership in the requirement-ID boundary set
// [A-Za-z0-9_-].
func citationIDByte(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-'
}

// citationIDAt returns the end of a requirement ID
// [A-Z][A-Z0-9-]{2,31}-[0-9]{3} starting at i with token boundaries on both
// sides, or -1. Only the longest candidate can satisfy the trailing boundary,
// so trying lengths from longest down is equivalent to any-match semantics.
func citationIDAt(b []byte, i int) int {
	if i >= len(b) || b[i] < 'A' || b[i] > 'Z' || i > 0 && citationIDByte(b[i-1]) {
		return -1
	}
	run := i + 1
	for run < len(b) && run-i-1 < 35 && (b[run] >= 'A' && b[run] <= 'Z' || b[run] >= '0' && b[run] <= '9' || b[run] == '-') {
		run++
	}
	for mid := 31; mid >= 2; mid-- {
		end := i + 1 + mid + 4
		if end > run {
			continue
		}
		d := b[end-4 : end]
		if d[0] != '-' || d[1] < '0' || d[1] > '9' || d[2] < '0' || d[2] > '9' || d[3] < '0' || d[3] > '9' {
			continue
		}
		if end < len(b) && citationIDByte(b[end]) {
			continue
		}
		return end
	}
	return -1
}

// citationIDs returns the requirement IDs in b and a copy of b with every
// matched ID replaced by a space.
func citationIDs(b []byte) (map[string]bool, []byte) {
	ids := map[string]bool{}
	rest := make([]byte, 0, len(b))
	for i := 0; i < len(b); {
		if end := citationIDAt(b, i); end > 0 {
			ids[string(b[i:end])] = true
			rest = append(rest, ' ')
			i = end
			continue
		}
		rest = append(rest, b[i])
		i++
	}
	return ids, rest
}

// citationDefined returns the IDs a span defines with a line of the form
// optional spaces/tabs, then "- `ID`:".
func citationDefined(span []byte) map[string]bool {
	defined := map[string]bool{}
	for len(span) > 0 {
		line := span
		if k := bytes.IndexByte(span, '\n'); k >= 0 {
			line, span = span[:k], span[k+1:]
		} else {
			span = nil
		}
		line = bytes.TrimLeft(line, " \t")
		if !bytes.HasPrefix(line, []byte("- `")) {
			continue
		}
		line = line[3:]
		if k := bytes.IndexByte(line, '`'); k > 0 && k+1 < len(line) && line[k+1] == ':' {
			defined[string(line[:k])] = true
		}
	}
	return defined
}

func citationTokenByte(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_'
}

// citationNeutral reports whether every [A-Za-z0-9_] token of b is neutral.
// Requirement IDs were already blanked by citationIDs.
func citationNeutral(b []byte) bool {
	for i := 0; i < len(b); {
		if !citationTokenByte(b[i]) {
			i++
			continue
		}
		j := i
		for j < len(b) && citationTokenByte(b[j]) {
			j++
		}
		tok := b[i:j]
		i = j
		if len(tok) < 3 || citationIDAt(tok, 0) == len(tok) || citationStopTerms[strings.ToLower(string(tok))] {
			continue
		}
		return false
	}
	return true
}

type citationInput struct {
	id, path, disposition string
	evidence              []string // distinct cited evidence IDs, sorted
	samePath              int      // (hunk, basis) pairs citing evidence at the hunk path
	addedIDs              map[string]bool
	annotation            bool
}

// citationAnchors returns the sorted anchor kinds the hunk finds against the
// given cited spans.
func citationAnchors(h *citationInput, cited []string, spans map[string][]byte, defs map[string]map[string]bool) []string {
	id, path := false, false
	base := h.path[strings.LastIndexByte(h.path, '/')+1:]
	for _, eid := range cited {
		span := spans[eid]
		for x := range h.addedIDs {
			if defs[eid][x] {
				id = true
			}
		}
		if h.path != "" && bytes.Contains(span, []byte(h.path)) || len(base) >= 6 && bytes.Contains(span, []byte(base)) {
			path = true
		}
	}
	out := []string{}
	if id {
		out = append(out, "id")
	}
	if path {
		out = append(out, "path")
	}
	return out
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// computeCitation derives the block for a verified canonical map. It must be
// called only after the mechanical re-proof and evidence span checks passed,
// so every mechanical hunk is proven and every cited span is loaded.
func computeCitation(hunks []mappedHunk, patch map[string]*patchHunk, evidence []evidence) *citationBlock {
	spans := map[string][]byte{}
	paths := map[string]string{}
	defs := map[string]map[string]bool{}
	for _, e := range evidence {
		spans[e.ID] = e.data
		paths[e.ID] = e.Path
		defs[e.ID] = citationDefined(e.data)
	}
	in := make([]*citationInput, 0, len(hunks))
	for _, h := range hunks {
		x := &citationInput{id: h.ID, path: h.Path, disposition: h.Disposition, evidence: []string{}}
		seen := map[string]bool{}
		for _, b := range h.Basis {
			if paths[b.EvidenceID] == h.Path {
				x.samePath++
			}
			if !seen[b.EvidenceID] {
				seen[b.EvidenceID] = true
				x.evidence = append(x.evidence, b.EvidenceID)
			}
		}
		sort.Strings(x.evidence)
		var added []byte
		lines := 0
		if p := patch[h.ID]; p != nil {
			for _, bl := range p.Body {
				if bl.prefix == '+' {
					lines++
					added = append(added, bl.payload...)
				}
			}
		}
		var rest []byte
		x.addedIDs, rest = citationIDs(added)
		x.annotation = lines > 0 && citationNeutral(rest)
		in = append(in, x)
	}
	sort.Slice(in, func(i, j int) bool { return in[i].id < in[j].id })

	c := &citationBlock{Profile: citationProfile, Hunks: []citationHunk{}, EvidenceFanOut: []citationFanOut{}, Invariant: true}
	fan := map[string]int{}
	var supported []*citationInput
	anchored := 0
	for _, x := range in {
		row := citationHunk{HunkID: x.id, Anchors: []string{}, Annotation: x.annotation}
		switch {
		case x.disposition == "mechanical":
			row.Tier = "mechanical-proved"
			c.HunksByClass.MechanicalProved++
		case len(x.evidence) > 0:
			supported = append(supported, x)
			row.Anchors = citationAnchors(x, x.evidence, spans, defs)
			row.Tier = "cited"
			if len(row.Anchors) > 0 {
				row.Tier = "anchored"
				anchored++
				c.HunksByClass.Anchored++
			} else {
				c.HunksByClass.Cited++
			}
			for _, eid := range x.evidence {
				fan[eid]++
			}
			c.SamePathCitations += x.samePath
		default:
			row.Tier = "unknown"
			c.HunksByClass.Unknown++
		}
		if x.annotation {
			c.AnnotationHunks++
		}
		c.Hunks = append(c.Hunks, row)
	}
	for eid, n := range fan {
		c.EvidenceFanOut = append(c.EvidenceFanOut, citationFanOut{eid, n})
		if n > c.MaxFanOut {
			c.MaxFanOut = n
		}
	}
	c.DistinctEvidence = len(fan)
	sort.Slice(c.EvidenceFanOut, func(i, j int) bool {
		a, b := c.EvidenceFanOut[i], c.EvidenceFanOut[j]
		if a.Hunks != b.Hunks {
			return a.Hunks > b.Hunks
		}
		return a.EvidenceID < b.EvidenceID
	})
	// No shared evidence (maxFanOut below 2) means no concentration (V1-1331).
	for _, x := range supported {
		if c.MaxFanOut < 2 {
			break
		}
		all := true
		for _, eid := range x.evidence {
			if fan[eid] != c.MaxFanOut {
				all = false
			}
		}
		if all {
			c.ConcentratedHunks++
		}
	}
	if len(supported) >= 2 {
		shifted := 0
		for i, x := range supported {
			prev := supported[(i+len(supported)-1)%len(supported)]
			if !sameStrings(x.evidence, supported[0].evidence) {
				c.Invariant = false
			}
			if len(citationAnchors(x, prev.evidence, spans, defs)) > 0 {
				shifted++
			}
		}
		c.Specificity = anchored - shifted
	}
	return c
}

// citationHunks flattens decoded 0.3/1.0 hunks for computeCitation.
func citationHunks(hs []hunk03) []mappedHunk {
	out := make([]mappedHunk, len(hs))
	for i, h := range hs {
		out[i] = h.mappedHunk
	}
	return out
}
