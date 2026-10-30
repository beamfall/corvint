package tcq

import (
	"errors"
	"strings"
)

// blobAnalysis is the single bounded scan TCQ-V0-007 permits per distinct
// selected target blob. It supplies execution keys and collision counts and
// nothing else: no sibling anchor, claim, association, or hygiene evidence
// crosses from one selected edge to another.
type blobAnalysis struct {
	path       string
	oid        string
	candidates []anchorCandidate
	python     pythonAnalysis
	goScan     goAnalysis
	allUnits   []testUnit
	// failure is the per-claim abstention reason every edge in this blob
	// inherits when the blob itself could not be scanned (TCQ-V0-011).
	failure string
}

func analyzeBlob(path, oid string, data []byte) (blobAnalysis, error) {
	switch {
	case strings.HasSuffix(path, ".py"):
		return analyzePythonBlob(path, oid, data)
	case strings.HasSuffix(path, ".go"):
		return analyzeGoBlob(path, oid, data)
	}
	return blobAnalysis{path: path, oid: oid}, nil
}

func analyzePythonBlob(path, oid string, data []byte) (blobAnalysis, error) {
	analysis, err := analyzePython(path, oid, data)
	blob := blobAnalysis{path: path, oid: oid, python: analysis, candidates: pythonCandidates(analysis)}
	switch {
	case errors.Is(err, errPythonGrammar):
		blob.failure = reasonUnsupportedPythonGrammar
		return blob, nil
	case errors.Is(err, errPythonOffset):
		blob.failure = reasonPythonOffsetMismatch
		return blob, nil
	case err != nil:
		return blobAnalysis{}, err
	}
	for _, function := range pythonUnitFunctions(analysis) {
		blob.allUnits = append(blob.allUnits, function.unit)
	}
	if len(blob.allUnits) > maxCollisionUnitsPerBlob {
		return blobAnalysis{}, fail(CodeResourceExhausted)
	}
	return blob, nil
}

func analyzeGoBlob(path, oid string, data []byte) (blobAnalysis, error) {
	analysis, err := analyzeGo(path, oid, data)
	blob := blobAnalysis{path: path, oid: oid, goScan: analysis, candidates: goCandidates(data)}
	switch {
	case errors.Is(err, errGoParse):
		blob.failure = reasonUnparseableTestUnit
		return blob, nil
	case err != nil:
		return blobAnalysis{}, err
	}
	candidateCount := 0
	for _, function := range analysis.functions {
		blob.allUnits = append(blob.allUnits, function.unit)
		candidateCount += 1 + len(function.cases)
		for _, item := range function.cases {
			blob.allUnits = append(blob.allUnits, caseUnit(function.unit, item.value))
		}
	}
	if candidateCount > maxCollisionUnitsPerBlob {
		return blobAnalysis{}, fail(CodeResourceExhausted)
	}
	return blob, nil
}

// pythonUnitFunctions narrows the extractor's candidate set to the functions
// TCQ-V0-013 admits as units: a complete `test_[A-Za-z0-9_]+` name whose direct
// lexical parent is the module or a class.
func pythonUnitFunctions(analysis pythonAnalysis) []pythonFunction {
	var admitted []pythonFunction
	for _, function := range analysis.functions {
		if function.topLevel && pythonTestName.MatchString(function.name) {
			admitted = append(admitted, function)
		}
	}
	return admitted
}

// association is the outcome of TCQ-V0-007..011 for one selected edge: either a
// unit, or exactly one abstention reason with the anchor profile retained.
type association struct {
	profile string
	unit    *testUnit
	reason  string
}

var supportedAnchorProfiles = map[string]bool{
	anchorPythonTestName:  true,
	anchorPythonDocstring: true,
	anchorGoTestName:      true,
	anchorGoTableCase:     true,
}

// associate resolves one selected claim edge against its blob. The second
// result is the blob's unit set for collision counting; TCQ-V0-007 releases it
// only when the edge itself reached the scan.
func associate(blob blobAnalysis, selector string, start, end int64) (association, []testUnit) {
	profile, ok := anchorProfile(blob.candidates, selector, start, end)
	if !ok || !supportedAnchorProfiles[profile] {
		return association{reason: reasonUnsupportedAnchorProfile}, nil
	}
	if blob.failure != "" {
		return association{profile: profile, reason: blob.failure}, nil
	}
	matches := candidateUnits(blob, profile, selector, start, end)
	switch len(matches) {
	case 0:
		return association{profile: profile, reason: reasonClaimAssociationMissing}, blob.allUnits
	case 1:
		return association{profile: profile, unit: &matches[0]}, blob.allUnits
	}
	return association{profile: profile, reason: reasonClaimAssociationAmbiguous}, blob.allUnits
}

// candidateUnits applies the TCQ-V0-005 dispatch table. Each branch inspects
// only the selected anchor and the smallest enclosing construct.
func candidateUnits(blob blobAnalysis, profile, selector string, start, end int64) []testUnit {
	switch profile {
	case anchorPythonTestName:
		return pythonNameMatches(blob, start, end)
	case anchorPythonDocstring:
		return pythonDocstringMatches(blob, start, end)
	case anchorGoTestName:
		return goNameMatches(blob, start, end)
	case anchorGoTableCase:
		return goCaseMatches(blob, selector, start, end)
	}
	return nil
}

// pythonNameMatches implements TCQ-V0-008: the span must be the identifier span
// of one `test_[A-Za-z0-9_]+` function.
func pythonNameMatches(blob blobAnalysis, start, end int64) []testUnit {
	var matches []testUnit
	for _, function := range pythonUnitFunctions(blob.python) {
		if int64(function.nameStart) == start && int64(function.nameEnd) == end {
			matches = append(matches, withKind(function.unit, kindPythonTestFunction))
		}
	}
	return matches
}

// pythonDocstringMatches implements TCQ-V0-008: the span must lie within the
// leading docstring statement of exactly one such enclosing function. Several
// requirement IDs in one docstring stay separate edges over the same unit.
func pythonDocstringMatches(blob blobAnalysis, start, end int64) []testUnit {
	var matches []testUnit
	for _, function := range pythonUnitFunctions(blob.python) {
		if !function.hasDoc {
			continue
		}
		if int64(function.docStart) <= start && start < end && end <= int64(function.docEnd) {
			matches = append(matches, withKind(function.unit, kindPythonDocstringFunction))
		}
	}
	return matches
}

// goNameMatches implements TCQ-V0-009 for `go-test-name/1`.
func goNameMatches(blob blobAnalysis, start, end int64) []testUnit {
	var matches []testUnit
	for _, function := range blob.goScan.functions {
		if int64(function.nameStart) == start && int64(function.nameEnd) == end {
			matches = append(matches, withKind(function.unit, kindGoTestFunction))
		}
	}
	return matches
}

// goCaseMatches implements TCQ-V0-009 for `go-table-case/1`: a leaf component
// inside exactly one validated parent test. The selector must name that parent:
// the claim extractor reads comment-masked bytes only, so a `func TestX(` inside
// a string can make it name a parent the unit scanner never admitted.
func goCaseMatches(blob blobAnalysis, selector string, start, end int64) []testUnit {
	var matches []testUnit
	for _, function := range blob.goScan.functions {
		for _, item := range function.cases {
			if int64(item.start) != start || int64(item.end) != end {
				continue
			}
			if selector == "test:"+function.unit.runtimeName+"/case:"+selectorFragment(item.value) {
				matches = append(matches, caseUnit(function.unit, item.value))
			}
		}
	}
	return matches
}

// withKind stamps the association kind the matching branch established. Unit
// identity does not include the kind (TCQ-V0-022), so a name edge and a
// docstring edge over one function still share one unit row.
func withKind(unit testUnit, kind string) testUnit {
	unit.associationKind = kind
	return unit
}
