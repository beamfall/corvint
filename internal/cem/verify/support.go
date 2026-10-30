package verify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// CitationProfile names the consumer-computed support report (decision 0523
// P0, CEM-V1-080..082).
const CitationProfile = "cem-citation-support/0"

// Support tiers, by precedence. TierHeld is reserved and never produced.
const (
	TierHeld             = "held"
	TierMechanicalProved = "mechanical-proved"
	TierAnchored         = "anchored"
	TierCited            = "cited"
	TierUnknown          = "unknown"
)

// Anchor kinds. A Go-only symbol kind is a recorded limit and never produced.
const (
	AnchorID   = "id"
	AnchorPath = "path"
)

// Citation is the consumer-computed support block. Field order is the JSON
// order and is part of the cross-consumer contract (CEM-V1-082). Nothing in it
// is a verdict: it never changes validity, acceptance, codes or drift.
type Citation struct {
	Profile           string          `json:"profile"`
	Hunks             []CitationHunk  `json:"hunks"`
	HunksByClass      CitationClasses `json:"hunksByClass"`
	AnnotationHunks   int             `json:"annotationHunks"`
	DistinctEvidence  int             `json:"distinctEvidence"`
	EvidenceFanOut    []FanOut        `json:"evidenceFanOut"`
	MaxFanOut         int             `json:"maxFanOut"`
	SamePathCitations int             `json:"samePathCitations"`
	ConcentratedHunks int             `json:"concentratedHunks"`
	Invariant         bool            `json:"invariant"`
	Specificity       int             `json:"specificity"`
}

// CitationHunk is one hunk's derived tier.
type CitationHunk struct {
	HunkID     string   `json:"hunkId"`
	Tier       string   `json:"tier"`
	Anchors    []string `json:"anchors"`
	Annotation bool     `json:"annotation"`
}

// CitationClasses counts hunks per tier.
type CitationClasses struct {
	Held             int `json:"held"`
	MechanicalProved int `json:"mechanical-proved"`
	Anchored         int `json:"anchored"`
	Cited            int `json:"cited"`
	Unknown          int `json:"unknown"`
}

// FanOut is one cited evidence ID and the number of distinct hunks citing it,
// encoded as the two-element array [evidenceId, count].
type FanOut struct {
	EvidenceID string
	Count      int
}

func (f FanOut) MarshalJSON() ([]byte, error) {
	return json.Marshal([2]any{f.EvidenceID, f.Count})
}

func (f *FanOut) UnmarshalJSON(data []byte) error {
	var pair []json.RawMessage
	if err := json.Unmarshal(data, &pair); err != nil {
		return err
	}
	if len(pair) != 2 {
		return fmt.Errorf("evidenceFanOut entry has %d elements, want 2", len(pair))
	}
	if err := json.Unmarshal(pair[0], &f.EvidenceID); err != nil {
		return err
	}
	return json.Unmarshal(pair[1], &f.Count)
}

// supportRequirementID is the lexical-relevance-floor-v0 requirement ID
// grammar (internal/lrf requirementIDAtStart); boundaries are checked apart.
var supportRequirementID = regexp.MustCompile(`^[A-Z][A-Z0-9-]{2,31}-[0-9]{3}`)

// supportStopTerms are the annotation stop terms (CEM-V1-081): the
// lexical-relevance-floor-v0 stop list without the 21 terms that are a
// keyword, reserved word, predeclared identifier or builtin in a common
// language (V1-1335), so a short code line such as "return value - 1" is not
// read as annotation. The portable consumer mirrors the list byte for byte.
var supportStopTerms = func() map[string]bool {
	words := []string{
		"also", "been", "build", "change", "changed", "changes", "claim", "dist", "docs", "document",
		"documentation", "evidence", "generated", "have", "hunk", "must", "only", "packages", "report",
		"reports", "requirement", "requirements", "shall", "should", "sidecar", "sources", "supported",
		"testing", "tests", "that", "their", "there", "these", "they", "those", "vendor", "vendored",
		"which", "will",
	}
	result := make(map[string]bool, len(words))
	for _, word := range words {
		result[word] = true
	}
	return result
}()

// ComputeCitation derives the support block from the verified map, its parsed
// canonical patch and the bound span bytes keyed by evidence ID. It assumes
// the inherited verification already passed: every map hunk is derived from
// the patch, every span was read and bound, and every mechanical hunk was
// re-proved by checkMechanical (the wire forbids bases on mechanical hunks).
func ComputeCitation(document *wire.Map, parsed *patch.Patch, spans map[string][]byte) *Citation {
	defined := map[string]map[string]bool{}
	for evidenceID, span := range spans {
		defined[evidenceID] = definedIDs(span)
	}
	derived := map[string]*patch.Hunk{}
	for _, hunk := range parsed.Hunks {
		derived[hunk.ID] = hunk
	}
	paths := map[string]string{}
	for _, record := range document.Evidence {
		paths[record.ID] = record.Path
	}
	hunks := append([]wire.Hunk(nil), document.Hunks...)
	sort.Slice(hunks, func(i, j int) bool { return hunks[i].ID < hunks[j].ID })

	citation := &Citation{Profile: CitationProfile, Hunks: []CitationHunk{}, EvidenceFanOut: []FanOut{}, Invariant: true}
	facts := map[string]hunkFacts{}
	var supported []wire.Hunk
	citing := map[string]map[string]bool{}
	for _, mapped := range hunks {
		facts[mapped.ID] = newHunkFacts(derived[mapped.ID])
		row := CitationHunk{HunkID: mapped.ID, Anchors: []string{}, Annotation: facts[mapped.ID].annotation}
		switch {
		case mapped.Disposition == "mechanical":
			row.Tier = TierMechanicalProved
			citation.HunksByClass.MechanicalProved++
		case len(mapped.Basis) == 0:
			row.Tier = TierUnknown
			citation.HunksByClass.Unknown++
		default:
			supported = append(supported, mapped)
			ids := basisSet(mapped.Basis)
			row.Anchors = facts[mapped.ID].anchors(ids, spans, defined)
			row.Tier = TierCited
			if len(row.Anchors) > 0 {
				row.Tier = TierAnchored
				citation.HunksByClass.Anchored++
			} else {
				citation.HunksByClass.Cited++
			}
			for _, id := range ids {
				if citing[id] == nil {
					citing[id] = map[string]bool{}
				}
				citing[id][mapped.ID] = true
			}
			for _, basis := range mapped.Basis {
				if paths[basis.EvidenceID] == facts[mapped.ID].path {
					citation.SamePathCitations++
				}
			}
		}
		if row.Annotation {
			citation.AnnotationHunks++
		}
		citation.Hunks = append(citation.Hunks, row)
	}

	for id, hunkIDs := range citing {
		citation.EvidenceFanOut = append(citation.EvidenceFanOut, FanOut{EvidenceID: id, Count: len(hunkIDs)})
		if len(hunkIDs) > citation.MaxFanOut {
			citation.MaxFanOut = len(hunkIDs)
		}
	}
	sort.Slice(citation.EvidenceFanOut, func(i, j int) bool {
		left, right := citation.EvidenceFanOut[i], citation.EvidenceFanOut[j]
		if left.Count != right.Count {
			return left.Count > right.Count
		}
		return left.EvidenceID < right.EvidenceID
	})
	citation.DistinctEvidence = len(citation.EvidenceFanOut)

	// Concentration needs a shared item: with maxFanOut below 2 no evidence
	// is cited by two hunks and no hunk is concentrated (V1-1331).
	for _, mapped := range supported {
		if citation.MaxFanOut < 2 {
			break
		}
		concentrated := true
		for _, id := range basisSet(mapped.Basis) {
			if len(citing[id]) != citation.MaxFanOut {
				concentrated = false
				break
			}
		}
		if concentrated {
			citation.ConcentratedHunks++
		}
	}

	// Shift null: supported hunks are already in hunkId order; hunk i takes
	// hunk i-1's basis set and hunk 0 takes the last one's.
	if len(supported) >= 2 {
		first := strings.Join(basisSet(supported[0].Basis), "\x00")
		shifted := 0
		for index, mapped := range supported {
			previous := supported[(index+len(supported)-1)%len(supported)]
			ids := basisSet(previous.Basis)
			if strings.Join(ids, "\x00") != first {
				citation.Invariant = false
			}
			if len(facts[mapped.ID].anchors(ids, spans, defined)) > 0 {
				shifted++
			}
		}
		citation.Specificity = citation.HunksByClass.Anchored - shifted
	}
	return citation
}

// basisSet returns a hunk's distinct basis evidence IDs in ascending order.
func basisSet(bases []wire.Basis) []string {
	seen := map[string]bool{}
	ids := []string{}
	for _, basis := range bases {
		if !seen[basis.EvidenceID] {
			seen[basis.EvidenceID] = true
			ids = append(ids, basis.EvidenceID)
		}
	}
	sort.Strings(ids)
	return ids
}

// hunkFacts are the patch-derived inputs of one hunk's anchors.
type hunkFacts struct {
	path       string   // post-image path, or the old path for a pure deletion
	ids        []string // distinct requirement IDs in the added bytes
	annotation bool
}

func newHunkFacts(hunk *patch.Hunk) hunkFacts {
	var facts hunkFacts
	if hunk == nil {
		return facts
	}
	if hunk.NewPath != nil {
		facts.path = *hunk.NewPath
	} else if hunk.OldPath != nil {
		facts.path = *hunk.OldPath
	}
	var added []byte
	addedLines := 0
	for _, line := range hunk.Body {
		if line.Prefix == '+' {
			added = append(added, line.Payload...)
			addedLines++
		}
	}
	occurrences := requirementIDs(added)
	seen := map[string]bool{}
	for _, occurrence := range occurrences {
		id := string(added[occurrence[0]:occurrence[1]])
		if !seen[id] {
			seen[id] = true
			facts.ids = append(facts.ids, id)
		}
	}
	sort.Strings(facts.ids)
	facts.annotation = addedLines > 0 && neutralOnly(added, occurrences)
	return facts
}

// anchors returns the sorted anchor kinds found against any of the spans.
func (f hunkFacts) anchors(evidenceIDs []string, spans map[string][]byte, defined map[string]map[string]bool) []string {
	idAnchor, pathAnchor := false, false
	base := f.path[strings.LastIndexByte(f.path, '/')+1:]
	for _, evidenceID := range evidenceIDs {
		span := spans[evidenceID]
		if !idAnchor {
			for _, id := range f.ids {
				if defined[evidenceID][id] {
					idAnchor = true
					break
				}
			}
		}
		if !pathAnchor && f.path != "" {
			pathAnchor = bytes.Contains(span, []byte(f.path)) || len(base) >= 6 && bytes.Contains(span, []byte(base))
		}
	}
	kinds := []string{}
	if idAnchor {
		kinds = append(kinds, AnchorID)
	}
	if pathAnchor {
		kinds = append(kinds, AnchorPath)
	}
	return kinds
}

// SpanAnchors returns the sorted CEM-V1-081 anchor kinds span earns for
// hunk, by the rule ComputeCitation tiers a cited hunk with, and the distinct
// requirement IDs the hunk's added bytes carry. A citation drafter asks it of
// each candidate span so that the rows it prefers are the ones the check
// counts as anchored (DCW-V0-161).
func SpanAnchors(hunk *patch.Hunk, span []byte) (kinds, addedIDs []string) {
	facts := newHunkFacts(hunk)
	kinds = facts.anchors([]string{""}, map[string][]byte{"": span}, map[string]map[string]bool{"": definedIDs(span)})
	return kinds, facts.ids
}

// definedIDs returns every ID a span defines with a line that, after optional
// leading spaces and tabs, reads "- `ID`:". It scans the span once without
// materialising its lines, so a large span costs no more than its bytes.
func definedIDs(span []byte) map[string]bool {
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

// idBoundary reports the bytes that may not touch a requirement ID token.
func idBoundary(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-'
}

// requirementIDs returns the [start, end) offsets of every bounded
// requirement ID token in data, left to right and non-overlapping.
func requirementIDs(data []byte) [][2]int {
	var found [][2]int
	for index := 0; index < len(data); index++ {
		if data[index] < 'A' || data[index] > 'Z' || index > 0 && idBoundary(data[index-1]) {
			continue
		}
		matched := supportRequirementID.FindIndex(data[index:])
		if matched == nil {
			continue
		}
		end := index + matched[1]
		if end < len(data) && idBoundary(data[end]) {
			continue
		}
		found = append(found, [2]int{index, end})
		index = end - 1
	}
	return found
}

// neutralOnly reports whether every maximal [A-Za-z0-9_] token of data,
// outside the requirement ID occurrences, is shorter than 3 bytes or a stop
// term. ID occurrences are neutral as whole tokens.
func neutralOnly(data []byte, occurrences [][2]int) bool {
	masked := append([]byte(nil), data...)
	for _, occurrence := range occurrences {
		for index := occurrence[0]; index < occurrence[1]; index++ {
			masked[index] = ' '
		}
	}
	for index := 0; index < len(masked); {
		if !tokenByte(masked[index]) {
			index++
			continue
		}
		end := index + 1
		for end < len(masked) && tokenByte(masked[end]) {
			end++
		}
		token := masked[index:end]
		if len(token) >= 3 && !supportStopTerms[strings.ToLower(string(token))] {
			return false
		}
		index = end
	}
	return true
}

func tokenByte(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_'
}
