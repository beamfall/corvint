package adapters

import (
	"context"
	"os"
	"sort"
	"strconv"
	"time"

	dashboardauthority "github.com/Beamfall/corvint/internal/dashboard/authority"
	"github.com/Beamfall/corvint/internal/dashboard/model"
	"github.com/Beamfall/corvint/internal/dashboard/source"
)

type memberValidation struct {
	summary  TraceSummary
	code     AdapterIssueCode
	limit    uint64
	evidence source.Evidence
	digest   string
	bytes    uint64
	rowCount uint64
	traceIDs []string
	paths    []string
}

func sameEvidence(left, right source.Evidence) bool {
	return left.AdapterID() == right.AdapterID() &&
		left.ByteCount() == right.ByteCount() &&
		left.ConfiguredOrdinal() == right.ConfiguredOrdinal() &&
		left.ContentSHA256() == right.ContentSHA256() &&
		left.Start().Equal(right.Start()) && left.End().Equal(right.End()) &&
		left.FileIdentityBefore() == right.FileIdentityBefore() &&
		left.FileIdentityAfter() == right.FileIdentityAfter() &&
		left.Validity() == right.Validity()
}

// Scan performs exactly one dashboard acquisition/compile attempt. The caller
// owns Authority construction, terminal Finish/cleanup, and whole-attempt
// retry. SourceBudget is caller-owned and is never refunded.
func Scan(ctx context.Context, request ScanRequest) ([]byte, error) {
	planned, failure := planRequest(request)
	if failure != nil {
		return nil, failure
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, &ScanError{Code: "DASHBOARD_INTERRUPTED"}
	}
	if failure := preflightSources(planned, request.SourceBudget); failure != nil {
		return nil, failure
	}
	repository, ok := repositoryModel(request.Authority.Snapshot())
	if !ok {
		return nil, &ScanError{Code: "DASHBOARD_REPOSITORY_UNAVAILABLE"}
	}
	root, err := os.OpenRoot(request.Root)
	if err != nil {
		return nil, &ScanError{Code: "DASHBOARD_REPOSITORY_UNAVAILABLE"}
	}
	defer root.Close()

	clock, start, err := scanClock(request)
	if err != nil {
		return nil, invalidArgumentError()
	}
	sources := make([]model.SourceInput, 0, len(planned))
	metrics := make([]model.MetricInput, 0)
	issues := make([]model.IssueInput, 0)
	scanState := model.ScanComplete
	for _, configured := range planned {
		if err := ctx.Err(); err != nil {
			return nil, &ScanError{Code: "DASHBOARD_INTERRUPTED"}
		}
		var sourceInput model.SourceInput
		var sourceMetrics []model.MetricInput
		var sourceIssues []model.IssueInput
		var partial bool
		if configured.adapterID == AdapterLocalTrace {
			sourceInput, sourceMetrics, sourceIssues, partial, failure = scanTraceSource(ctx, root, configured, request, repository, clock)
			if failure != nil {
				return nil, failure
			}
		} else {
			sourceInput, sourceIssues, failure = unsupportedSourceInput(configured)
			if failure != nil {
				return nil, failure
			}
		}
		if partial {
			scanState = model.ScanPartial
		}
		sources = append(sources, sourceInput)
		metrics = append(metrics, sourceMetrics...)
		issues = append(issues, sourceIssues...)
	}
	metrics, err = MergeTraceUsageMetrics(metrics)
	if err != nil {
		return nil, &ScanError{Code: "DASHBOARD_INTERNAL_ERROR"}
	}
	end := start
	if request.GeneratedAt == nil {
		end = canonicalTime(time.Now().UTC())
		if end < start {
			end = start
		}
	}
	_, encoded, compileErr := model.Compile(model.Input{
		GeneratedAt: end,
		Observation: model.ObservationInput{
			ClockSource: model.ClockSource(request.ClockSource), Start: start, End: end, ScanState: scanState,
		},
		Repository: repository, Registry: Registrations(), Sources: sources, Metrics: metrics, Issues: issues,
	})
	if compileErr != nil {
		if modelFailure, matched := compileErr.(*model.Error); matched && modelFailure.Code == "DASHBOARD_RESOURCE_EXHAUSTED" {
			return nil, &ScanError{Code: "DASHBOARD_RESOURCE_EXHAUSTED"}
		}
		return nil, &ScanError{Code: "DASHBOARD_INTERNAL_ERROR"}
	}
	return encoded, nil
}

func preflightSources(planned []plannedSource, budget *source.Budget) *ScanError {
	if budget == nil {
		return invalidArgumentError()
	}
	for _, configured := range planned {
		descriptor, registered := Lookup(configured.adapterID)
		if !registered {
			return invalidArgumentError()
		}
		declaredBound, err := strconv.ParseUint(descriptor.MaxBytes, 10, 64)
		if err != nil {
			return &ScanError{Code: "DASHBOARD_INTERNAL_ERROR"}
		}
		decision := budget.Preflight(string(configured.adapterID), configured.ordinal, declaredBound)
		if decision.Allowed() {
			continue
		}
		switch decision.Code() {
		case source.BudgetLogicalExhausted, source.BudgetPhysicalExhausted:
			return &ScanError{Code: "DASHBOARD_RESOURCE_EXHAUSTED"}
		default:
			return &ScanError{Code: "DASHBOARD_INTERNAL_ERROR"}
		}
	}
	return nil
}

func scanClock(request ScanRequest) (source.Clock, string, error) {
	if request.GeneratedAt != nil {
		fixed, err := time.Parse("2006-01-02T15:04:05.000000000Z", *request.GeneratedAt)
		if err != nil {
			return source.Clock{}, "", err
		}
		clock, err := source.FixedClock(fixed)
		return clock, *request.GeneratedAt, err
	}
	return source.ProcessClock(), canonicalTime(time.Now().UTC()), nil
}

func canonicalTime(value time.Time) string {
	return value.UTC().Format("2006-01-02T15:04:05.000000000Z")
}

func repositoryModel(snapshot dashboardauthority.Snapshot) (model.Repository, bool) {
	if snapshot.WorktreeState != dashboardauthority.WorktreeClean && snapshot.WorktreeState != dashboardauthority.WorktreeMixed ||
		!validObjectID(snapshot.HeadRevision, snapshot.ObjectFormat) || !validObjectID(snapshot.TreeRevision, snapshot.ObjectFormat) ||
		!validSHA256Digest(snapshot.DirtyPathsSHA256) {
		return model.Repository{}, false
	}
	dirtyCount := strconv.FormatUint(snapshot.DirtyPathCount, 10)
	return model.Repository{
		DirtyPathCount: &dirtyCount, DirtyPathsSHA256: stringPointer(snapshot.DirtyPathsSHA256),
		HeadRevision: stringPointer(snapshot.HeadRevision), ObjectFormat: stringPointer(snapshot.ObjectFormat),
		TreeRevision: stringPointer(snapshot.TreeRevision), WorktreeState: model.WorktreeState(snapshot.WorktreeState),
	}, true
}

func scanTraceSource(ctx context.Context, root *os.Root, configured plannedSource, request ScanRequest, repository model.Repository, clock source.Clock) (model.SourceInput, []model.MetricInput, []model.IssueInput, bool, *ScanError) {
	format := source.ObjectFormat(*repository.ObjectFormat)
	validations := make(map[string]memberValidation)
	store := source.ScanTraceStoreWithClock(root, configured.relativePath, configured.ordinal, format, request.SourceBudget, clock,
		func(revision string, content source.StableContent, evidence source.Evidence) {
			summary, paths, code, limit := preflightStableTraceContent(content, revision)
			validations[revision] = memberValidation{
				summary: summary, code: code, limit: limit, evidence: evidence,
				digest: content.SHA256(), bytes: content.Len(), rowCount: summary.RetainedRows,
				traceIDs: append([]string(nil), summary.traceIDs...), paths: paths,
			}
		})
	if store.Issue != source.IssueNone {
		return traceStoreRootFailure(configured, store)
	}

	terminal := make(map[AdapterIssueCode]uint64)
	type provisionalMember struct {
		validation memberValidation
	}
	provisional := make([]provisionalMember, 0, len(store.Members))
	traceIDs := make(map[string]struct{})
	var retainedRows uint64
	var boundLimit uint64
	for _, member := range store.Members {
		if member.Result.Validity != source.ValidityStable {
			code, override, limit := memberIssue(member.Result.Issue)
			if override {
				if code == IssueTraceStoreBound && boundLimit == 0 {
					boundLimit = limit
				}
				continue
			}
			terminal[code]++
			continue
		}
		validation, exists := validations[member.Revision]
		if !exists || validation.evidence.ConfiguredOrdinal() != configured.ordinal || validation.digest == "" ||
			member.Result.SHA256 == nil || *member.Result.SHA256 != validation.digest ||
			member.Result.Bytes != validation.bytes || member.Result.Evidence == nil ||
			!sameEvidence(*member.Result.Evidence, validation.evidence) {
			return model.SourceInput{}, nil, nil, false, &ScanError{Code: "DASHBOARD_INTERNAL_ERROR"}
		}
		if validation.rowCount > maxTraceRows-retainedRows {
			if boundLimit == 0 {
				boundLimit = maxTraceRows
			}
			continue
		}
		retainedRows += validation.rowCount
		if validation.code != "" {
			switch validation.code {
			case IssueTraceStoreBound:
				if boundLimit == 0 {
					boundLimit = validation.limit
				}
			default:
				terminal[validation.code]++
			}
			continue
		}
		provisional = append(provisional, provisionalMember{validation: validation})
	}
	if boundLimit != 0 {
		return traceRootFailure(configured, source.IssueTraceStoreBound, boundLimit)
	}
	eligible := make([]provisionalMember, 0, len(provisional))
	for _, candidate := range provisional {
		duplicate := false
		for _, traceID := range candidate.validation.traceIDs {
			if _, exists := traceIDs[traceID]; exists {
				duplicate = true
				break
			}
		}
		if duplicate {
			terminal[IssueVerifierRejected]++
			continue
		}
		for _, traceID := range candidate.validation.traceIDs {
			traceIDs[traceID] = struct{}{}
		}
		eligible = append(eligible, candidate)
	}
	admitted := make([]VerifiedTraceMember, 0, len(eligible))
	for _, candidate := range eligible {
		if ctx.Err() != nil {
			return model.SourceInput{}, nil, nil, false, &ScanError{Code: "DASHBOARD_INTERRUPTED"}
		}
		validation := candidate.validation
		summary, code, _, qualificationFailure := qualifyTraceSummary(ctx, validation.summary, validation.paths, request.Authority)
		if qualificationFailure != nil {
			return model.SourceInput{}, nil, nil, false, qualificationFailure
		}
		if code != "" {
			terminal[code]++
			continue
		}
		admitted = append(admitted, VerifiedTraceMember{
			Summary: summary, ContentSHA256: validation.digest, ByteCount: validation.bytes,
			Start: canonicalTime(validation.evidence.Start()), End: canonicalTime(validation.evidence.End()),
		})
	}
	counts := terminalCounts(terminal)
	if len(admitted) == 0 && len(counts) != 0 {
		return rejectedTraceSource(configured, counts)
	}
	head := model.RepositoryWitness{Kind: "SNAPSHOT_HEAD", ObjectFormat: *repository.ObjectFormat, ObjectID: *repository.HeadRevision, ObjectType: "commit"}
	aggregate, code := AggregateTraceMembers(head, admitted)
	if code != "" {
		return model.SourceInput{}, nil, nil, false, &ScanError{Code: "DASHBOARD_INTERNAL_ERROR"}
	}
	completeness := model.CompletenessComplete
	partial := false
	if len(counts) != 0 {
		completeness = model.CompletenessPartial
		partial = true
	}
	sourceInput, metrics, issues, err := TraceModelInputs(configured.ordinal, aggregate, admitted, repository.DirtyPathsSHA256, completeness, counts)
	if err != nil {
		return model.SourceInput{}, nil, nil, false, &ScanError{Code: "DASHBOARD_INTERNAL_ERROR"}
	}
	return sourceInput, metrics, issues, partial, nil
}

func traceStoreRootFailure(configured plannedSource, store source.TraceStoreResult) (model.SourceInput, []model.MetricInput, []model.IssueInput, bool, *ScanError) {
	if (store.Issue == source.IssueTraceStoreBound && store.Limit != maxTraceRows && store.Limit != maxTraceStoreBytes) ||
		(store.Issue != source.IssueTraceStoreBound && store.Limit != 0) {
		return model.SourceInput{}, nil, nil, false, &ScanError{Code: "DASHBOARD_INTERNAL_ERROR"}
	}
	return traceRootFailure(configured, store.Issue, store.Limit)
}

func terminalCounts(input map[AdapterIssueCode]uint64) []TraceTerminalCount {
	result := make([]TraceTerminalCount, 0, len(input))
	for code, count := range input {
		if count != 0 {
			result = append(result, TraceTerminalCount{Code: code, Count: count})
		}
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Code < result[right].Code })
	return result
}

func memberIssue(issue source.IssueCode) (AdapterIssueCode, bool, uint64) {
	switch issue {
	case source.IssueSourceUnreadable, source.IssueSourceUnavailable:
		return "SOURCE_INACCESSIBLE", false, 0
	case source.IssueSourceSymlink:
		return "SOURCE_SYMLINK", false, 0
	case source.IssueSourceHardLinked:
		return "SOURCE_MULTILINK_UNQUALIFIED", false, 0
	case source.IssueSourceNotRegular:
		return "SOURCE_SPECIAL_FILE", false, 0
	case source.IssueSourceTooLarge:
		return "SOURCE_OVERSIZED", false, 0
	case source.IssueSourceUnstable:
		return "SOURCE_CHANGED_DURING_READ", false, 0
	case source.IssueTraceStoreBound, source.IssueAggregateBudgetExceeded:
		return IssueTraceStoreBound, true, maxTraceStoreBytes
	default:
		return IssueSourceInvalidIdentity, false, 0
	}
}

func traceRootFailure(configured plannedSource, issue source.IssueCode, limit uint64) (model.SourceInput, []model.MetricInput, []model.IssueInput, bool, *ScanError) {
	switch issue {
	case source.IssueAggregateBudgetExceeded:
		return model.SourceInput{}, nil, nil, false, &ScanError{Code: "DASHBOARD_RESOURCE_EXHAUSTED"}
	case source.IssueSourceUnavailable:
		return traceRootFailureCode(configured, "SOURCE_NOT_PRESENT", model.SeverityWarning, nil, model.ValidityNotPresent, model.CompletenessUnknown)
	case source.IssueSourceUnreadable:
		return traceRootFailureCode(configured, "SOURCE_INACCESSIBLE", model.SeverityError, nil, model.ValidityInaccessible, model.CompletenessUnknown)
	case source.IssueSourceSymlink:
		return traceRootFailureCode(configured, "SOURCE_SYMLINK", model.SeverityError, nil, model.ValidityInvalid, model.CompletenessUnknown)
	case source.IssueSourceHardLinked:
		return traceRootFailureCode(configured, "SOURCE_MULTILINK_UNQUALIFIED", model.SeverityError, nil, model.ValidityInvalid, model.CompletenessUnknown)
	case source.IssueSourceNotRegular:
		return traceRootFailureCode(configured, "SOURCE_SPECIAL_FILE", model.SeverityError, nil, model.ValidityInvalid, model.CompletenessUnknown)
	case source.IssueStoreChanged:
		return traceRootFailureCode(configured, "STORE_CHANGED", model.SeverityError, nil, model.ValidityInvalid, model.CompletenessUnknown)
	case source.IssueTraceStoreBound:
		if limit != maxTraceRows && limit != maxTraceRowBytes && limit != maxTraceStoreBytes {
			return model.SourceInput{}, nil, nil, false, &ScanError{Code: "DASHBOARD_INTERNAL_ERROR"}
		}
		text := strconv.FormatUint(limit, 10)
		return traceRootFailureCode(configured, "TRACE_STORE_BOUND", model.SeverityError, &text, model.ValidityInvalid, model.CompletenessUnknown)
	default:
		return model.SourceInput{}, nil, nil, false, &ScanError{Code: "DASHBOARD_INTERNAL_ERROR"}
	}
}

func traceRootFailureCode(configured plannedSource, code string, severity model.Severity, limit *string, validity model.Validity, completeness model.Completeness) (model.SourceInput, []model.MetricInput, []model.IssueInput, bool, *ScanError) {
	ordinal := strconv.FormatUint(configured.ordinal, 10)
	sourceID, err := model.ComputeSourceID(model.SourceIdentity{AdapterID: string(AdapterLocalTrace), ConfiguredOrdinal: ordinal, Profile: configured.profile})
	if err != nil {
		return model.SourceInput{}, nil, nil, false, &ScanError{Code: "DASHBOARD_INTERNAL_ERROR"}
	}
	issueInput := model.IssueInput{Code: code, Severity: severity, SourceID: &sourceID, Limit: cloneOptional(limit)}
	issue, err := model.NewIssue(issueInput)
	if err != nil {
		return model.SourceInput{}, nil, nil, false, &ScanError{Code: "DASHBOARD_INTERNAL_ERROR"}
	}
	return model.SourceInput{
		AdapterID: string(AdapterLocalTrace), ConfiguredOrdinal: ordinal, Profile: configured.profile,
		AuthorityClass: model.AuthorityNone, Completeness: completeness, Currency: model.CurrencyUnknown,
		DeliveryStage: model.DeliveryNotStarted, EpistemicClass: model.EpistemicNotObserved,
		Exclusions: []string{issue.ID}, Validity: validity, VerifierID: "go-local-trace-v1",
	}, nil, []model.IssueInput{issueInput}, validity == model.ValidityInvalid || validity == model.ValidityInaccessible, nil
}

func rejectedTraceSource(configured plannedSource, counts []TraceTerminalCount) (model.SourceInput, []model.MetricInput, []model.IssueInput, bool, *ScanError) {
	ordinal := strconv.FormatUint(configured.ordinal, 10)
	sourceID, err := model.ComputeSourceID(model.SourceIdentity{AdapterID: string(AdapterLocalTrace), ConfiguredOrdinal: ordinal, Profile: configured.profile})
	if err != nil {
		return model.SourceInput{}, nil, nil, false, &ScanError{Code: "DASHBOARD_INTERNAL_ERROR"}
	}
	terminalIssues, exclusions, err := TraceTerminalIssueInputs(sourceID, counts)
	if err != nil {
		return model.SourceInput{}, nil, nil, false, &ScanError{Code: "DASHBOARD_INTERNAL_ERROR"}
	}
	observation := model.IssueInput{Code: "OBSERVATION_TIME_UNKNOWN", Severity: model.SeverityInfo, SourceID: &sourceID}
	empty := []model.TraceMember{}
	issues := append(terminalIssues, observation)
	return model.SourceInput{
		AdapterID: string(AdapterLocalTrace), ConfiguredOrdinal: ordinal, Profile: configured.profile,
		AuthorityClass: model.AuthorityAdapterQualified, Completeness: model.CompletenessPartial,
		Currency: model.CurrencyValidatedAt, DeliveryStage: model.DeliveryNotStarted,
		EpistemicClass: model.EpistemicObserved, Exclusions: exclusions, Members: &empty,
		Validity: model.ValidityInvalid, VerifierID: "go-local-trace-v1",
	}, nil, issues, true, nil
}

func unsupportedSourceInput(configured plannedSource) (model.SourceInput, []model.IssueInput, *ScanError) {
	descriptor, ok := Lookup(configured.adapterID)
	if !ok {
		return model.SourceInput{}, nil, invalidArgumentError()
	}
	ordinal := strconv.FormatUint(configured.ordinal, 10)
	sourceInput := model.SourceInput{
		AdapterID: string(configured.adapterID), ConfiguredOrdinal: ordinal, Profile: configured.profile,
		AuthorityClass: model.AuthorityNone, Completeness: model.CompletenessUnknown,
		Currency: model.CurrencyUnknown, DeliveryStage: model.DeliveryStage(descriptor.DeliveryStage),
		EpistemicClass: model.EpistemicNotObserved, Validity: model.ValidityUnsupported,
		VerifierID: descriptor.VerifierID,
	}
	sourceID, err := model.ComputeSourceID(model.SourceIdentity{AdapterID: sourceInput.AdapterID, ConfiguredOrdinal: ordinal, Profile: sourceInput.Profile})
	if err != nil {
		return model.SourceInput{}, nil, &ScanError{Code: "DASHBOARD_INTERNAL_ERROR"}
	}
	issueInput := model.IssueInput{Code: "SOURCE_UNSUPPORTED", Severity: model.SeverityInfo, SourceID: &sourceID}
	issue, err := model.NewIssue(issueInput)
	if err != nil {
		return model.SourceInput{}, nil, &ScanError{Code: "DASHBOARD_INTERNAL_ERROR"}
	}
	sourceInput.Exclusions = []string{issue.ID}
	return sourceInput, []model.IssueInput{issueInput}, nil
}
