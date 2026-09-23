package tcq

import "sort"

// blobCache bounds the source TCQ-V0-041 allows one invocation to read and
// carries the single per-blob scan TCQ-V0-007 permits.
type blobCache struct {
	repository  Repository
	analyses    map[string]blobAnalysis
	totalSource int
	collisions  map[string]int
	scanned     map[string]bool
	totalUnits  int
}

func newBlobCache(repository Repository) *blobCache {
	return &blobCache{
		repository: repository,
		analyses:   map[string]blobAnalysis{},
		collisions: map[string]int{},
		scanned:    map[string]bool{},
	}
}

func (cache *blobCache) analyze(claim ocmClaim) (blobAnalysis, error) {
	key := claim.path + "\x00" + claim.blobOID
	if analysis, ok := cache.analyses[key]; ok {
		return analysis, nil
	}
	data, err := cache.repository.Blob(claim.blobOID)
	if err != nil {
		return blobAnalysis{}, fail(CodeObjectUnavailable)
	}
	if len(data) > maxSourceBlobBytes {
		return blobAnalysis{}, fail(CodeResourceExhausted)
	}
	cache.totalSource += len(data)
	if cache.totalSource > maxSourceBytes {
		return blobAnalysis{}, fail(CodeResourceExhausted)
	}
	analysis, err := analyzeBlob(claim.path, claim.blobOID, data)
	if err != nil {
		return blobAnalysis{}, err
	}
	cache.analyses[key] = analysis
	return analysis, nil
}

// countCollisions folds one blob's unit set into the execution-key histogram.
// TCQ-V0-007 permits nothing else to cross the blob boundary: the histogram
// supplies only the collision cardinality for an edge's own key.
func (cache *blobCache) countCollisions(claim ocmClaim, units []testUnit) error {
	key := claim.path + "\x00" + claim.blobOID
	if len(units) == 0 || cache.scanned[key] {
		return nil
	}
	if len(units) > maxCollisionUnitsPerBlob {
		return fail(CodeResourceExhausted)
	}
	cache.totalUnits += len(units)
	if cache.totalUnits > maxCollisionUnits {
		return fail(CodeResourceExhausted)
	}
	for _, unit := range units {
		cache.collisions[unit.executionKey]++
	}
	cache.scanned[key] = true
	return nil
}

// assemble runs TCQ-V0-042 stages 9 and 10: association, hygiene, execution
// matching, reference closure, then output bounds, canonical encoding, and the
// TCQ identity.
func assemble(repository Repository, document parsedDocuments, dynamic dynamicContext, resolved Resolved, expected []byte) (Result, error) {
	cache := newBlobCache(repository)
	associations := make([]association, len(document.ocm.edges))
	for index, edge := range document.ocm.edges {
		analysis, err := cache.analyze(edge.claim)
		if err != nil {
			return Result{}, err
		}
		resolvedEdge, units := associate(analysis, edge.claim.selector, edge.claim.start, edge.claim.end)
		if err := cache.countCollisions(edge.claim, units); err != nil {
			return Result{}, err
		}
		associations[index] = resolvedEdge
	}
	claims, units := projectClaims(document, dynamic, cache, associations)
	if len(units) > maxUnits {
		return Result{}, fail(CodeResourceExhausted)
	}
	return encodeResult(document, dynamic, resolved, claims, units, expected)
}

// projectClaims evaluates every selected edge into its claim result and collects
// the unique unit set the results reference (TCQ-V0-038 forbids an orphan unit).
func projectClaims(document parsedDocuments, dynamic dynamicContext, cache *blobCache, associations []association) ([]ClaimResult, map[string]testUnit) {
	claims := make([]ClaimResult, 0, len(associations))
	units := map[string]testUnit{}
	for index, edge := range document.ocm.edges {
		resolvedEdge := associations[index]
		if resolvedEdge.unit == nil {
			claims = append(claims, abstainedClaim(edge, resolvedEdge))
			continue
		}
		claim, unit := associatedClaim(edge, resolvedEdge, document, dynamic, cache)
		units[unit.identity()] = unit
		claims = append(claims, claim)
	}
	return claims, units
}

// abstainedClaim is the TCQ-V0-011 shape: no executable identity is permitted on
// an abstention, and `anchorProfile` survives unless the profile itself was
// unsupported.
func abstainedClaim(edge selectedEdge, resolved association) ClaimResult {
	return ClaimResult{
		ObligationID:     edge.obligationID,
		ClaimID:          edge.claim.id,
		AnchorProfile:    resolved.profile,
		AssociationState: AssociationAbstained,
		HygieneState:     HygieneAbstained,
		ReportState:      ReportNotMatched,
		Reasons:          []string{resolved.reason},
		AuthorityClass:   AuthorityClass,
	}
}

func associatedClaim(edge selectedEdge, resolved association, document parsedDocuments, dynamic dynamicContext, cache *blobCache) (ClaimResult, testUnit) {
	unit := *resolved.unit
	reasons, hygiene := hygieneReasons(unit)
	projection := projectReport(unit, document, dynamic, cache)
	reasons = append(reasons, projection.reasons...)
	relation := ""
	if hygiene == HygieneEligible && len(reasons) == 0 && projection.state == ReportPassed {
		relation = MatchedRelation
	}
	return ClaimResult{
		ObligationID:       edge.obligationID,
		ClaimID:            edge.claim.id,
		AnchorProfile:      resolved.profile,
		AssociationKind:    unit.associationKind,
		AssociationState:   AssociationAssociated,
		HygieneState:       hygiene,
		ReportState:        projection.state,
		TestUnitID:         unit.identity(),
		ExecutionKeySha256: unit.executionKey,
		RowIDs:             projection.rowIDs,
		Reasons:            sortReasons(reasons),
		Relation:           relation,
		AuthorityClass:     AuthorityClass,
	}, unit
}

// hygieneReasons implements TCQ-V0-019: both reasons are retained when both
// apply, and a safely delimited unit is otherwise ELIGIBLE — which means
// runnable-shaped, never adequate or correct.
func hygieneReasons(unit testUnit) ([]string, string) {
	var reasons []string
	if unit.empty {
		reasons = append(reasons, reasonEmptyBody)
	}
	if unit.skipped {
		reasons = append(reasons, reasonUnconditionalSkip)
	}
	if len(reasons) > 0 {
		return reasons, HygieneIneligible
	}
	return nil, HygieneEligible
}

type reportProjection struct {
	state   string
	rowIDs  []string
	reasons []string
}

// projectReport matches the unit's rows, then applies TCQ-V0-050: a claim whose
// execution key is flaky across observations and whose own projection matched
// at least one row adds `test-flaky`, which TCQ-V0-035 turns into no relation.
func projectReport(unit testUnit, document parsedDocuments, dynamic dynamicContext, cache *blobCache) reportProjection {
	projection := matchReport(unit, document, dynamic, cache)
	if len(projection.rowIDs) > 0 && dynamic.flaky[unit.executionKey] {
		projection.reasons = append(projection.reasons, reasonTestFlaky)
	}
	return projection
}

// matchReport implements TCQ-V0-021's collision rule and TCQ-V0-034's matching
// rule. Execution ambiguity is decided before any row is consulted: a duplicated
// target key is AMBIGUOUS even when exactly one row matches, because a key match
// never identifies which colliding body ran.
func matchReport(unit testUnit, document parsedDocuments, dynamic dynamicContext, cache *blobCache) reportProjection {
	if cache.collisions[unit.executionKey] > 1 {
		return reportProjection{state: ReportAmbiguous, reasons: []string{reasonExecutionIdentityAmbiguous}}
	}
	if dynamic.report == nil {
		return reportProjection{state: ReportNotMatched, reasons: []string{reasonTestNotMatched}}
	}
	matches := matchingRows(*dynamic.report, unit.executionKey)
	switch {
	case len(matches) == 0:
		return reportProjection{state: ReportNotMatched, reasons: []string{noMatchReason(*dynamic.report)}}
	case len(matches) > 1:
		return reportProjection{state: ReportAmbiguous, rowIDs: sortedRowIDs(matches), reasons: []string{reasonRepeatedTestRows}}
	}
	return uniqueRowProjection(matches[0], document)
}

// noMatchReason distinguishes "the report contains no such row" from "the report
// contains rows this verifier could not key" (TCQ-V0-034).
func noMatchReason(report junitReport) string {
	if report.unkeyedCount > 0 {
		return reasonRowIdentityUnavailable
	}
	return reasonTestNotMatched
}

var nonPassingReason = map[string]string{
	ReportSkipped: reasonTestSkipped,
	ReportFailed:  reasonTestFailed,
	ReportError:   reasonTestError,
}

// uniqueRowProjection carries a unique row's own status through. A skipped,
// failed, or errored row never becomes passing; a passing row still collects the
// TCQ-V0-035 preconditions it fails.
func uniqueRowProjection(row junitRow, document parsedDocuments) reportProjection {
	projection := reportProjection{state: row.status, rowIDs: []string{row.id}}
	if reason, nonPassing := nonPassingReason[row.status]; nonPassing {
		projection.reasons = append(projection.reasons, reason)
		return projection
	}
	if document.command.cleanTarget != CleanTargetAttested {
		projection.reasons = append(projection.reasons, reasonTargetCleanlinessNotAttested)
	}
	if document.observation.exitCode != 0 {
		projection.reasons = append(projection.reasons, reasonCommandFailed)
	}
	return projection
}

func sortedRowIDs(rows []junitRow) []string {
	identifiers := make([]string, 0, len(rows))
	for _, row := range rows {
		identifiers = append(identifiers, row.id)
	}
	sort.Strings(identifiers)
	return identifiers
}
