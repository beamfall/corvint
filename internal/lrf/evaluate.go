package lrf

import (
	"regexp"
	"sort"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

var obligationIDPattern = regexp.MustCompile(`^[A-Z][A-Z0-9-]{2,31}-[0-9]{3}$`)

type boundExceeded struct{}

func (boundExceeded) Error() string { return "relevance bound exceeded" }

// Evaluate applies the frozen default profile bounds.
func Evaluate(request Request) (Result, error) {
	return evaluate(request, DefaultLimits(), "")
}

// EvaluateWithLimits is the internal conformance seam. Limits may only be
// tightened relative to the frozen defaults.
func EvaluateWithLimits(request Request, limits Limits) (Result, error) {
	return evaluate(request, limits, "")
}

func evaluate(request Request, limits Limits, fault string) (result Result, err error) {
	if err := validateLimits(limits); err != nil {
		return Result{}, err
	}
	if err := validateRequest(request); err != nil {
		return Result{}, err
	}
	switch fault {
	case "":
	case "timeout", "allocator", "io", "interruption":
		return Result{}, &Error{Code: "lrf-operational-failure", Message: "LRF evaluation aborted operationally"}
	default:
		return Result{}, invalid("fault is outside the closed test enum")
	}
	inputs := request.Context.inputs()
	bound := sealResult(boundResult(inputs))
	if len(bound.canonical) > limits.OutputBytes {
		return Result{}, invalid("output bound is too small for the canonical bound document")
	}
	defer func() {
		if _, bounded := err.(boundExceeded); bounded {
			result = bound
			err = nil
		}
	}()
	if exceedsLexicalBytes(request, limits.LexicalBytes) {
		return Result{}, boundExceeded{}
	}
	return evaluateEdges(request, limits, inputs)
}

func evaluateEdges(request Request, limits Limits, inputs [14]any) (Result, error) {
	hunks := append([]Hunk(nil), request.Hunks...)
	obligations := append([]Obligation(nil), request.Obligations...)
	sort.Slice(hunks, func(left, right int) bool { return hunks[left].Ordinal < hunks[right].Ordinal })
	sort.Slice(obligations, func(left, right int) bool { return obligations[left].Ordinal < obligations[right].Ordinal })
	evidenceByID := make(map[string]Evidence, len(request.Evidence))
	for _, item := range request.Evidence {
		evidenceByID[item.ID] = item
	}
	hunkByID := make(map[string]Hunk, len(hunks))
	for _, item := range hunks {
		hunkByID[item.ID] = item
	}

	hunkTermCache := map[string]termCache{}
	evidenceTermCache := map[string]termCache{}
	obligationTermCache := map[string]termCache{}
	results := make([]ResultTuple, 0)
	issues := make([]IssueTuple, 0)
	resultSeen := map[ResultTuple]struct{}{}
	issueSeen := map[IssueTuple]struct{}{}
	witnessed := map[string]struct{}{}
	edges := 0

	appendResult := func(row ResultTuple) error {
		if _, found := resultSeen[row]; found {
			return nil
		}
		if len(results) >= limits.Results {
			return boundExceeded{}
		}
		resultSeen[row] = struct{}{}
		results = append(results, row)
		return nil
	}
	appendIssue := func(row IssueTuple) error {
		if _, found := issueSeen[row]; found {
			return nil
		}
		if len(issues) >= limits.Issues {
			return boundExceeded{}
		}
		issueSeen[row] = struct{}{}
		issues = append(issues, row)
		return nil
	}
	consumeEdge := func() error {
		if edges >= limits.Edges {
			return boundExceeded{}
		}
		edges++
		return nil
	}

	for _, hunk := range hunks {
		if hunk.Disposition != "supported" {
			continue
		}
		basis := uniqueSortedBasis(hunk.Basis)
		for _, relation := range basis {
			if err := consumeEdge(); err != nil {
				return Result{}, err
			}
			item := evidenceByID[relation.EvidenceID]
			outcome, code := evaluateBasis(hunk, item, limits, hunkTermCache, evidenceTermCache)
			if outcome == "cem-lexical-v0" {
				witnessed[hunk.ID] = struct{}{}
			}
			row := ResultTuple{"cem-basis", hunk.ID, item.ID, relation.Relation, AuthorityClass, outcome}
			if err := appendResult(row); err != nil {
				return Result{}, err
			}
			if code != "" {
				issue := IssueTuple{"cem-basis", hunk.ID, item.ID, relation.Relation, code}
				if err := appendIssue(issue); err != nil {
					return Result{}, err
				}
			}
		}
	}

	claimPaths := map[string]struct{}{}
	for _, obligation := range obligations {
		for _, path := range obligation.ClaimPaths {
			claimPaths[path] = struct{}{}
		}
	}
	for _, obligation := range obligations {
		candidate := false
		hunkIDs := append([]string(nil), obligation.HunkIDs...)
		sort.Strings(hunkIDs)
		for _, hunkID := range hunkIDs {
			if err := consumeEdge(); err != nil {
				return Result{}, err
			}
			hunk := hunkByID[hunkID]
			outcome, code := evaluateObligationEdge(
				request.Context, obligation, hunk, witnessed, claimPaths, limits,
				hunkTermCache, obligationTermCache,
			)
			row := ResultTuple{"ocm-hunk", obligation.ID, hunkID, "requirement-hunk", AuthorityClass, outcome}
			if err := appendResult(row); err != nil {
				return Result{}, err
			}
			if outcome == "lexically-proximate-candidate" {
				candidate = true
			} else {
				issue := IssueTuple{"ocm-hunk", obligation.ID, hunkID, "requirement-hunk", code}
				if err := appendIssue(issue); err != nil {
					return Result{}, err
				}
			}
		}
		if !candidate {
			issue := IssueTuple{"ocm-obligation", obligation.ID, "", "requirement-hunk", "no-material-hunk"}
			if err := appendIssue(issue); err != nil {
				return Result{}, err
			}
		}
	}

	result := Result{inputs: inputs, issues: issues, results: results}
	encoded := rawCanonicalBytes(result)
	if len(encoded) > limits.OutputBytes {
		return Result{}, boundExceeded{}
	}
	result.canonical = encoded
	return result, nil
}

type termCache struct {
	terms    map[string]struct{}
	overflow bool
}

func evaluateBasis(hunk Hunk, evidence Evidence, limits Limits, hunkCache, evidenceCache map[string]termCache) (string, string) {
	if hunk.NewPath == nil {
		return "abstained", "deletion-relation-required"
	}
	if len(evidence.Span) > limits.EvidenceBytes || logicalLines(evidence.Span) > limits.EvidenceLines {
		return "rejected", "evidence-span-too-broad"
	}
	if pathEquals(evidence.Path, hunk.OldPath) || pathEquals(evidence.Path, hunk.NewPath) {
		return "rejected", "self-referential-basis"
	}
	hunkTerms := cachedSubjectTerms(hunkCache, hunk.ID, hunk.Added, hunk.lexicalPath(), limits.Terms)
	evidenceTerms := cachedSubjectTerms(evidenceCache, evidence.ID, evidence.Span, evidence.Path, limits.Terms)
	if hunkTerms.overflow || evidenceTerms.overflow {
		return "abstained", "subject-term-bound-exceeded"
	}
	if !intersects(hunkTerms.terms, evidenceTerms.terms) {
		return "rejected", "insufficient-lexical-support"
	}
	return "cem-lexical-v0", ""
}

func evaluateObligationEdge(
	context Context,
	obligation Obligation,
	hunk Hunk,
	witnessed map[string]struct{},
	claimPaths map[string]struct{},
	limits Limits,
	hunkCache, obligationCache map[string]termCache,
) (string, string) {
	hunkTerms := cachedSubjectTerms(hunkCache, hunk.ID, hunk.Added, hunk.lexicalPath(), limits.Terms)
	obligationTerms := cachedObligationTerms(obligationCache, obligation, limits.Terms)
	if hunkTerms.overflow || obligationTerms.overflow {
		return "abstained", "subject-term-bound-exceeded"
	}
	if _, found := witnessed[hunk.ID]; !found {
		return "rejected", "obligation-hunk-mismatch"
	}
	if hunk.NewPath == nil {
		return "rejected", "obligation-hunk-mismatch"
	}
	if context.IntentPath != nil && hunk.path() == *context.IntentPath {
		return "rejected", "obligation-hunk-mismatch"
	}
	if _, excluded := claimPaths[hunk.path()]; excluded {
		return "rejected", "obligation-hunk-mismatch"
	}
	if !intersects(hunkTerms.terms, obligationTerms.terms) {
		return "rejected", "obligation-hunk-mismatch"
	}
	return "lexically-proximate-candidate", ""
}

func cachedSubjectTerms(cache map[string]termCache, id string, body []byte, path string, limit int) termCache {
	if value, found := cache[id]; found {
		return value
	}
	terms, overflow := subjectTerms(body, path, limit)
	value := termCache{terms: terms, overflow: overflow}
	cache[id] = value
	return value
}

func cachedObligationTerms(cache map[string]termCache, obligation Obligation, limit int) termCache {
	if value, found := cache[obligation.ID]; found {
		return value
	}
	body, err := requirementBody(obligation.Statement, obligation.ID)
	if err != nil {
		return termCache{overflow: true}
	}
	terms, overflow := identifierTerms(body, limit)
	value := termCache{terms: terms, overflow: overflow}
	cache[obligation.ID] = value
	return value
}

func uniqueSortedBasis(items []wire.Basis) []wire.Basis {
	seen := map[wire.Basis]struct{}{}
	for _, item := range items {
		seen[item] = struct{}{}
	}
	result := make([]wire.Basis, 0, len(seen))
	for item := range seen {
		result = append(result, item)
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].EvidenceID != result[right].EvidenceID {
			return result[left].EvidenceID < result[right].EvidenceID
		}
		return result[left].Relation < result[right].Relation
	})
	return result
}

func pathEquals(path string, endpoint *string) bool { return endpoint != nil && path == *endpoint }

func logicalLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	lines := 0
	for _, value := range data {
		if value == '\n' {
			lines++
		}
	}
	if data[len(data)-1] != '\n' {
		lines++
	}
	return lines
}

func boundResult(inputs [14]any) Result {
	return Result{
		inputs:        inputs,
		issues:        []IssueTuple{{"profile", "", "", "", "relevance-bound-exceeded"}},
		results:       []ResultTuple{},
		boundExceeded: true,
	}
}

func sealResult(result Result) Result {
	result.canonical = rawCanonicalBytes(result)
	return result
}
