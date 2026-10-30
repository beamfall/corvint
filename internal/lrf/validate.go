package lrf

import (
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

var allowedRelations = map[string]struct{}{
	"specification": {}, "decision": {}, "test-claim": {}, "implementation": {},
	"call-site": {}, "dependency": {}, "incident": {},
}

func validateRequest(request Request) error {
	if err := validateStableContext(request.Context); err != nil {
		return err
	}
	evidenceIDs := make(map[string]struct{}, len(request.Evidence))
	for _, item := range request.Evidence {
		if !prefixedSHA256(item.ID, wire.EvidencePrefix) || !validPath(item.Path) || len(item.Span) == 0 {
			return invalid("evidence projection is invalid")
		}
		if _, duplicate := evidenceIDs[item.ID]; duplicate {
			return invalid("evidence identities must be unique")
		}
		evidenceIDs[item.ID] = struct{}{}
	}
	hunkIDs := make(map[string]struct{}, len(request.Hunks))
	hunkOrdinals := make([]int, 0, len(request.Hunks))
	for _, item := range request.Hunks {
		if !prefixedSHA256(item.ID, wire.HunkPrefix) || item.path() == "" {
			return invalid("hunk projection is invalid")
		}
		if _, duplicate := hunkIDs[item.ID]; duplicate {
			return invalid("hunk identities must be unique")
		}
		if item.OldPath != nil && !validPath(*item.OldPath) || item.NewPath != nil && !validPath(*item.NewPath) {
			return invalid("hunk path is invalid")
		}
		if item.Disposition != "supported" && item.Disposition != "unknown" && item.Disposition != "mechanical" {
			return invalid("hunk disposition is invalid")
		}
		if (item.Disposition == "supported") != (len(item.Basis) > 0) {
			return invalid("hunk disposition/basis projection is invalid")
		}
		for _, basis := range item.Basis {
			if _, found := evidenceIDs[basis.EvidenceID]; !found {
				return invalid("basis references unknown evidence")
			}
			if _, allowed := allowedRelations[basis.Relation]; !allowed {
				return invalid("basis relation is invalid")
			}
		}
		hunkIDs[item.ID] = struct{}{}
		hunkOrdinals = append(hunkOrdinals, item.Ordinal)
	}
	if !consecutiveOrdinals(hunkOrdinals) {
		return invalid("hunk ordinals are invalid")
	}
	obligationIDs := map[string]struct{}{}
	obligationOrdinals := make([]int, 0, len(request.Obligations))
	for _, item := range request.Obligations {
		if !obligationIDPattern.MatchString(item.ID) || len(item.HunkIDs) == 0 {
			return invalid("obligation projection is invalid")
		}
		if _, duplicate := obligationIDs[item.ID]; duplicate {
			return invalid("obligation identities must be unique")
		}
		if _, err := requirementBody(item.Statement, item.ID); err != nil {
			return err
		}
		linked := map[string]struct{}{}
		for _, hunkID := range item.HunkIDs {
			if _, found := hunkIDs[hunkID]; !found {
				return invalid("obligation references unknown hunk")
			}
			if _, duplicate := linked[hunkID]; duplicate {
				return invalid("obligation hunk links must be unique")
			}
			linked[hunkID] = struct{}{}
		}
		claims := map[string]struct{}{}
		for _, path := range item.ClaimPaths {
			if !validPath(path) {
				return invalid("claim path is invalid")
			}
			if _, duplicate := claims[path]; duplicate {
				return invalid("claim paths must be unique")
			}
			claims[path] = struct{}{}
		}
		obligationIDs[item.ID] = struct{}{}
		obligationOrdinals = append(obligationOrdinals, item.Ordinal)
	}
	if !consecutiveOrdinals(obligationOrdinals) {
		return invalid("obligation ordinals are invalid")
	}
	if len(request.Obligations) > 0 && request.Context.OCMSpec == nil {
		return invalid("OCM obligations require OCM context")
	}
	return nil
}

func validateContextSpec(context Context) error {
	if !wire.IsSha256(context.CEMMapSHA256) || !wire.IsSha256(context.PatchSHA256) || !wire.IsGitOid(context.BaseRevision) {
		return invalid("CEM context identity is invalid")
	}
	if context.CEMSpec != wire.StableSpec {
		return unsupported("CEM profile is unsupported")
	}
	if context.PatchSource != "canonical-derived" || context.TargetRevision == nil ||
		!wire.IsGitOid(*context.TargetRevision) || context.ExcludedPath == nil ||
		*context.ExcludedPath != wire.ExcludedCEMPath {
		return unsupported("canonical CEM context is invalid")
	}
	ocmFields := []bool{
		context.OCMSpec != nil, context.OCMMapSHA256 != nil, context.IntentPath != nil,
		context.IntentBlobOID != nil, context.IntentStart != nil, context.IntentEnd != nil,
		context.IntentSpanSHA256 != nil,
	}
	present := 0
	for _, field := range ocmFields {
		if field {
			present++
		}
	}
	if present == 0 {
		return nil
	}
	if present != len(ocmFields) || *context.OCMSpec != OCMSpec ||
		!wire.IsSha256(*context.OCMMapSHA256) || !validPath(*context.IntentPath) ||
		!wire.IsGitOid(*context.IntentBlobOID) || *context.IntentStart < 0 ||
		*context.IntentEnd <= *context.IntentStart || !wire.IsSha256(*context.IntentSpanSHA256) {
		return invalid("OCM context identity is invalid")
	}
	return nil
}

func consecutiveOrdinals(values []int) bool {
	sorted := append([]int(nil), values...)
	sort.Ints(sorted)
	for index, value := range sorted {
		if value != index {
			return false
		}
	}
	return true
}

func prefixedSHA256(value, prefix string) bool {
	return strings.HasPrefix(value, prefix) && wire.IsSha256(strings.TrimPrefix(value, prefix))
}

// ValidateStableRequest is the owning LRF admission of one verified cem/1.0
// change projection (CEM-V1-055). It applies every frozen request check, but
// admits only the stable context row: cem/1.0, canonical-derived patch, full
// target, the excluded sidecar path and a complete OCM binding. It evaluates nothing. lrf/0 is retired
// (CEM-V1-215, V1-2665): Evaluate refuses every context as unsupported-lrf-context, so no lrf/0
// result can carry a map.
func ValidateStableRequest(request Request) error {
	return validateRequest(request)
}

// validateStableContext admits exactly the stable row: cem/1.0 with canonical-derived
// checks. Every other profile and a context without a complete OCM binding are unsupported.
func validateStableContext(context Context) error {
	err := validateContextSpec(context)
	if err == nil && context.OCMSpec == nil {
		// The shared checks keep the OCM all-or-none; the stable row needs it.
		return unsupported("stable LRF context requires a complete OCM binding")
	}
	return err
}
