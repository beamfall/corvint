package lrf

import (
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

var allowedRelations = map[string]struct{}{
	"specification": {}, "decision": {}, "test-claim": {}, "implementation": {},
	"call-site": {}, "dependency": {}, "incident": {},
}

func validateLimits(limits Limits) error {
	values := []int{
		limits.LexicalBytes, limits.Terms, limits.Edges, limits.Results,
		limits.Issues, limits.OutputBytes, limits.EvidenceBytes, limits.EvidenceLines,
	}
	ceilings := []int{
		maxLexicalBytes, maxTerms, maxEdges, maxResults, maxIssues, maxOutputBytes,
		maxEvidenceBytes, maxEvidenceLines,
	}
	for index, value := range values {
		if value < 1 || value > ceilings[index] {
			return invalid("LRF limits must be positive and cannot exceed profile defaults")
		}
	}
	return nil
}

func validateRequest(request Request) error {
	if err := validateContext(request.Context); err != nil {
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

func validateContext(context Context) error {
	if !wire.IsSha256(context.CEMMapSHA256) || !wire.IsSha256(context.PatchSHA256) || !wire.IsGitOid(context.BaseRevision) {
		return invalid("CEM context identity is invalid")
	}
	if context.CEMSpec == wire.Spec01 {
		if context.PatchSource != "explicit-out-of-band" && context.PatchSource != "default-out-of-band" {
			return unsupported("CEM 0.1 patch source is unsupported")
		}
		if context.ExcludedPath != nil || context.OCMSpec != nil || context.OCMMapSHA256 != nil ||
			context.IntentPath != nil || context.IntentBlobOID != nil || context.IntentStart != nil ||
			context.IntentEnd != nil || context.IntentSpanSHA256 != nil {
			return unsupported("OCM plus CEM 0.1 is unsupported")
		}
		if context.TargetRevision != nil && !wire.IsGitOid(*context.TargetRevision) {
			return invalid("target revision is invalid")
		}
		return nil
	}
	if context.CEMSpec != wire.Spec02 {
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

func exceedsLexicalBytes(request Request, limit int) bool {
	total := 0
	stems := map[string][]byte{}
	add := func(length int) bool {
		if length > math.MaxInt-total || length > limit-total {
			return true
		}
		total += length
		return false
	}
	accountPath := func(path string) {
		if path == "" {
			return
		}
		basename := path
		if slash := strings.LastIndexByte(path, '/'); slash >= 0 {
			basename = path[slash+1:]
		}
		stem := basenameStem(path)
		stems[basename+"\x00"+string(stem)] = stem
	}
	for _, hunk := range request.Hunks {
		if add(len(hunk.Added)) {
			return true
		}
		accountPath(hunk.lexicalPath())
	}
	for _, evidence := range request.Evidence {
		if add(len(evidence.Span)) {
			return true
		}
		accountPath(evidence.Path)
	}
	for _, obligation := range request.Obligations {
		if add(len(obligation.Statement)) {
			return true
		}
	}
	for _, stem := range stems {
		if add(len(stem)) {
			return true
		}
	}
	return false
}

func validUTF8(value string) bool { return utf8.ValidString(value) }
