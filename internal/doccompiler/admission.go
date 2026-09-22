package doccompiler

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// Closed clause vocabularies of HDCV0-023..025.
const (
	ClauseSupported  = "SUPPORTED"
	ClauseConflicted = "CONFLICTED"
	ClauseUnknown    = "UNKNOWN"

	KindPrescriptive = "PRESCRIPTIVE"
	KindDescriptive  = "DESCRIPTIVE"

	ScopeObserved = "OBSERVED"
	ScopeGeneral  = "GENERAL"

	FrontierNoQualifyingSource = "NO_QUALIFYING_SOURCE"
	FrontierStaleAnchor        = "STALE_ANCHOR"
	FrontierResolverUndecided  = "RESOLVER_UNDECIDED"

	AuthorityAcceptedIntent   = "ACCEPTED_INTENT"
	AuthorityPinnedSource     = "PINNED_SOURCE"
	AuthorityPinnedTest       = "PINNED_TEST"
	AuthorityExecutionReceipt = "EXECUTION_RECEIPT"

	// ProseTemplateSet names the closed, versioned template set of HDCV0-026.
	ProseTemplateSet = "corvint-hdc-prose-templates/0"

	maxClauses        = 2_000
	maxAnchors        = 10_000
	maxAnchorSpan     = 8 << 10
	maxClauseIDBytes  = 256
	maxClauseTextSize = 1 << 20
)

// Anchor is the evidence-anchor definition: every member is required.
type Anchor struct {
	Path       string `json:"path"`
	Blob       string `json:"blob"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	SpanSHA256 string `json:"span_sha256"`
	Authority  string `json:"authority"`
	Reason     string `json:"reason"`
}

// Clause is one HDCV0-023 output clause. Frontier is empty unless State is UNKNOWN.
type Clause struct {
	ID       string   `json:"id"`
	State    string   `json:"state"`
	Kind     string   `json:"kind"`
	Scope    string   `json:"scope"`
	Text     string   `json:"text"`
	Frontier string   `json:"frontier,omitempty"`
	Anchors  []Anchor `json:"anchors"`
}

var clauseStates = map[string]bool{ClauseSupported: true, ClauseConflicted: true, ClauseUnknown: true}
var clauseKinds = map[string]bool{KindPrescriptive: true, KindDescriptive: true}
var clauseScopes = map[string]bool{"": true, ScopeObserved: true, ScopeGeneral: true}
var clauseFrontiers = map[string]bool{FrontierNoQualifyingSource: true, FrontierStaleAnchor: true, FrontierResolverUndecided: true}

// kindAuthorities is HDCV0-024: prescriptive clauses qualify only on accepted
// intent, descriptive clauses only on pinned source, test, or execution receipt.
var kindAuthorities = map[string]map[string]bool{
	KindPrescriptive: {AuthorityAcceptedIntent: true},
	KindDescriptive:  {AuthorityPinnedSource: true, AuthorityPinnedTest: true, AuthorityExecutionReceipt: true},
}

// anchorAuthorities is the closed authority set; any other class never anchors.
var anchorAuthorities = map[string]bool{AuthorityAcceptedIntent: true, AuthorityPinnedSource: true, AuthorityPinnedTest: true, AuthorityExecutionReceipt: true}

// observedOnlyAuthorities is HDCV0-025: these anchors cannot support a GENERAL clause.
var observedOnlyAuthorities = map[string]bool{AuthorityPinnedTest: true, AuthorityExecutionReceipt: true}

type anchorVerdict struct {
	fresh, stale bool
}

// AdmitClauses applies HDCV0-023..025 against the Git-pinned sources of index.
// A malformed clause is refused. A SUPPORTED or CONFLICTED clause whose
// qualifying anchors do not establish that state is admitted as UNKNOWN with a
// frontier and keeps its anchors as review context; no state is ever upgraded.
func AdmitClauses(index *contextindex.Index, clauses []Clause) ([]Clause, error) {
	if len(clauses) > maxClauses {
		return nil, failure("clause-limit-exceeded", "more than %d clauses", maxClauses)
	}
	anchors := 0
	for _, clause := range clauses {
		anchors += len(clause.Anchors)
	}
	if anchors > maxAnchors {
		return nil, failure("clause-limit-exceeded", "more than %d evidence anchors", maxAnchors)
	}
	seen := map[string]bool{}
	admitted := make([]Clause, 0, len(clauses))
	for _, clause := range clauses {
		if err := validateClauseShape(clause); err != nil {
			return nil, err
		}
		if seen[clause.ID] {
			return nil, failure("duplicate-clause", "clause ID is duplicated: %s", clause.ID)
		}
		seen[clause.ID] = true
		admitted = append(admitted, admitClause(index, clause))
	}
	sort.Slice(admitted, func(left, right int) bool { return admitted[left].ID < admitted[right].ID })
	return admitted, nil
}

func validateClauseShape(clause Clause) error {
	if !validClauseID(clause.ID) {
		return failure("invalid-clause", "clause ID must be 1..%d bytes of UTF-8 without whitespace, controls, or format characters", maxClauseIDBytes)
	}
	if strings.TrimSpace(clause.Text) == "" || len(clause.Text) > maxClauseTextSize || !utf8.ValidString(clause.Text) {
		return failure("invalid-clause", "clause %s review text must be non-empty bounded UTF-8", clause.ID)
	}
	if !clauseStates[clause.State] || !clauseKinds[clause.Kind] || !clauseScopes[clause.Scope] {
		return failure("invalid-clause", "clause %s has a state, kind, or scope outside the closed sets", clause.ID)
	}
	if clause.State == ClauseUnknown && !clauseFrontiers[clause.Frontier] {
		return failure("invalid-clause", "UNKNOWN clause %s requires exactly one closed frontier", clause.ID)
	}
	if clause.State != ClauseUnknown && clause.Frontier != "" {
		return failure("invalid-clause", "%s clause %s cannot carry a frontier", clause.State, clause.ID)
	}
	return nil
}

func validClauseID(id string) bool {
	if id == "" || len(id) > maxClauseIDBytes || !utf8.ValidString(id) {
		return false
	}
	return strings.IndexFunc(id, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) }) < 0
}

func admitClause(index *contextindex.Index, clause Clause) Clause {
	clause.Anchors = append([]Anchor(nil), clause.Anchors...)
	if clause.Scope == "" {
		clause.Scope = ScopeGeneral
	}
	if clause.State == ClauseUnknown {
		return clause
	}
	verdicts := make([]anchorVerdict, len(clause.Anchors))
	for position, anchor := range clause.Anchors {
		verdicts[position] = verifyAnchor(index, anchor)
	}
	established := supportEstablished
	if clause.State == ClauseConflicted {
		established = conflictEstablished
	}
	if established(clause, verdicts) {
		return clause
	}
	clause.State = ClauseUnknown
	clause.Frontier = FrontierNoQualifyingSource
	for _, verdict := range verdicts {
		if verdict.stale {
			clause.Frontier = FrontierStaleAnchor
		}
	}
	return clause
}

// supportEstablished needs one fresh anchor whose authority qualifies for the
// clause kind and, for a GENERAL clause, is not test or receipt evidence.
func supportEstablished(clause Clause, verdicts []anchorVerdict) bool {
	for position, anchor := range clause.Anchors {
		qualifies := verdicts[position].fresh && kindAuthorities[clause.Kind][anchor.Authority]
		if qualifies && !(clause.Scope == ScopeGeneral && observedOnlyAuthorities[anchor.Authority]) {
			return true
		}
	}
	return false
}

// conflictEstablished needs two fresh anchors at distinct locations, at least
// one qualifying for the clause kind; the other may be the opposing authority
// class, which is how intent/implementation drift stays visible (decision 0229).
func conflictEstablished(clause Clause, verdicts []anchorVerdict) bool {
	locations := map[string]bool{}
	kindQualified := false
	for position, anchor := range clause.Anchors {
		if !verdicts[position].fresh {
			continue
		}
		locations[anchorLocation(anchor)] = true
		kindQualified = kindQualified || kindAuthorities[clause.Kind][anchor.Authority]
	}
	return kindQualified && len(locations) >= 2
}

func anchorLocation(anchor Anchor) string {
	return strings.Join([]string{anchor.Path, anchor.Blob, strconv.Itoa(anchor.StartLine), strconv.Itoa(anchor.EndLine)}, "\x00")
}

// verifyAnchor reports whether an anchor qualifies at the pinned source identity.
// Stale means the path is pinned at a different blob than the anchor names in
// the same object format; a blob ID in the other format is disqualified.
func verifyAnchor(index *contextindex.Index, anchor Anchor) anchorVerdict {
	if !anchorMembersPresent(anchor) {
		return anchorVerdict{}
	}
	if !anchorAuthorities[anchor.Authority] {
		return anchorVerdict{}
	}
	if !revisionPattern.MatchString(anchor.Blob) {
		return anchorVerdict{}
	}
	if !digestPattern.MatchString(anchor.SpanSHA256) {
		return anchorVerdict{}
	}
	source, pinned := index.Sources[anchor.Path]
	if !pinned {
		return anchorVerdict{}
	}
	// IDs in two object formats never compare, so the difference says nothing about staleness.
	if len(source.BlobHash) != len(anchor.Blob) {
		return anchorVerdict{}
	}
	if source.BlobHash != anchor.Blob {
		return anchorVerdict{stale: true}
	}
	text, valid, loaded := source.Text()
	if !valid {
		return anchorVerdict{}
	}
	if !loaded {
		return anchorVerdict{}
	}
	if gitBlobID([]byte(text), len(anchor.Blob)) != anchor.Blob {
		return anchorVerdict{}
	}
	span, inRange := anchorSpan([]byte(text), anchor.StartLine, anchor.EndLine)
	if !inRange {
		return anchorVerdict{}
	}
	if len(span) > maxAnchorSpan {
		return anchorVerdict{}
	}
	digest := sha256.Sum256(span)
	return anchorVerdict{fresh: hex.EncodeToString(digest[:]) == anchor.SpanSHA256}
}

func anchorMembersPresent(anchor Anchor) bool {
	members := []string{anchor.Path, anchor.Blob, anchor.SpanSHA256, anchor.Authority, anchor.Reason}
	for _, member := range members {
		if strings.TrimSpace(member) == "" {
			return false
		}
	}
	return anchor.StartLine >= 1 && anchor.EndLine >= anchor.StartLine
}

// gitBlobID is the Git object ID of a blob: SHA-1 for 40 hex digits, else SHA-256.
func gitBlobID(data []byte, hexLength int) string {
	header := fmt.Sprintf("blob %d\x00", len(data))
	if hexLength == 40 {
		digest := sha1.Sum(append([]byte(header), data...))
		return hex.EncodeToString(digest[:])
	}
	digest := sha256.Sum256(append([]byte(header), data...))
	return hex.EncodeToString(digest[:])
}

// anchorSpan returns the bytes of one-based inclusive lines start..end, each
// with its terminating LF when present. A final LF does not open another line.
func anchorSpan(data []byte, start, end int) ([]byte, bool) {
	begin := 0
	line := 1
	for offset := 0; offset < len(data); line++ {
		stop := len(data)
		if next := bytes.IndexByte(data[offset:], '\n'); next >= 0 {
			stop = offset + next + 1
		}
		if line == start {
			begin = offset
		}
		if line == end {
			return data[begin:stop], true
		}
		offset = stop
	}
	return nil, false
}

// RenderAdmittedProse admits clauses and renders them with only the closed
// ProseTemplateSet strings. Caller-supplied values are rendered as literals.
func RenderAdmittedProse(index *contextindex.Index, clauses []Clause) ([]byte, error) {
	admitted, err := AdmitClauses(index, clauses)
	if err != nil {
		return nil, err
	}
	var output strings.Builder
	fmt.Fprintf(&output, "<!-- %s -->\n## Evidence and uncertainty\n", ProseTemplateSet)
	for _, clause := range admitted {
		fmt.Fprintf(&output, "\n- Clause %s is **%s** (%s, %s", claimLiteral(clause.ID), clause.State, clause.Kind, clause.Scope)
		if clause.Frontier != "" {
			fmt.Fprintf(&output, ", frontier %s", clause.Frontier)
		}
		fmt.Fprintf(&output, "): %s\n", claimLiteral(clause.Text))
		for _, anchor := range clause.Anchors {
			fmt.Fprintf(&output, "  - Anchor %s lines %d-%d, blob %s, authority %s\n", claimLiteral(anchor.Path), anchor.StartLine, anchor.EndLine, claimLiteral(anchor.Blob), claimLiteral(anchor.Authority))
		}
	}
	return []byte(output.String()), nil
}

// VerifyAdmittedProse refuses a candidate transformation unless it is exactly
// the rendering of its admitted clauses: any other sentence is refused.
func VerifyAdmittedProse(index *contextindex.Index, clauses []Clause, candidate []byte) error {
	rendered, err := RenderAdmittedProse(index, clauses)
	if err != nil {
		return err
	}
	if !bytes.Equal(rendered, candidate) {
		return failure("unadmitted-prose", "candidate prose is not admitted clause text within %s", ProseTemplateSet)
	}
	return nil
}
