package tcq

import (
	"sort"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// nullableString renders the document's `null` for an absent optional field, so
// an abstention can never carry an empty-string identity in its place.
func nullableString(value string) wire.Value {
	if value == "" {
		return jsonNull()
	}
	return jsonString(value)
}

// claimValue is the exact TCQ-V0-037 claim shape.
func claimValue(claim ClaimResult) wire.Value {
	return jsonObject(
		member{"anchorProfile", nullableString(claim.AnchorProfile)},
		member{"associationKind", nullableString(claim.AssociationKind)},
		member{"associationState", jsonString(claim.AssociationState)},
		member{"authorityClass", jsonString(claim.AuthorityClass)},
		member{"claimId", jsonString(claim.ClaimID)},
		member{"executionKeySha256", nullableString(claim.ExecutionKeySha256)},
		member{"hygieneState", jsonString(claim.HygieneState)},
		member{"obligationId", jsonString(claim.ObligationID)},
		member{"reasons", jsonStrings(claim.Reasons)},
		member{"relation", nullableString(claim.Relation)},
		member{"reportState", jsonString(claim.ReportState)},
		member{"rowIds", jsonStrings(claim.RowIDs)},
		member{"testUnitId", nullableString(claim.TestUnitID)},
	)
}

// issueValues is the TCQ-V0-038 issue list: exactly one object for every claim
// reason and no others, sorted by OCM obligation order, claim ID, then reason
// order.
func issueValues(claims []ClaimResult) []wire.Value {
	type issue struct {
		ordinal int
		claimID string
		reason  string
		obligID string
	}
	var issues []issue
	for ordinal, claim := range claims {
		for _, reason := range claim.Reasons {
			issues = append(issues, issue{ordinal, claim.ClaimID, reason, claim.ObligationID})
		}
	}
	sort.SliceStable(issues, func(left, right int) bool {
		if issues[left].ordinal != issues[right].ordinal {
			return issues[left].ordinal < issues[right].ordinal
		}
		if issues[left].claimID != issues[right].claimID {
			return issues[left].claimID < issues[right].claimID
		}
		return reasonIndex[issues[left].reason] < reasonIndex[issues[right].reason]
	})
	values := make([]wire.Value, 0, len(issues))
	for _, item := range issues {
		values = append(values, jsonObject(
			member{"claimId", jsonString(item.claimID)},
			member{"obligationId", jsonString(item.obligID)},
			member{"reason", jsonString(item.reason)},
		))
	}
	return values
}

// unitValues is the unique unit set referenced by associated claims, sorted by
// unit ID (TCQ-V0-038).
func unitValues(units map[string]testUnit) []wire.Value {
	identifiers := make([]string, 0, len(units))
	for identifier := range units {
		identifiers = append(identifiers, identifier)
	}
	sort.Strings(identifiers)
	values := make([]wire.Value, 0, len(identifiers))
	for _, identifier := range identifiers {
		values = append(values, units[identifier].wireValue())
	}
	return values
}

// observationSummary is the exact TCQ-V0-036 dynamic summary. Values are copied
// from the verified observation; no other conditional shape is valid.
func observationSummary(document parsedDocuments, dynamic dynamicContext) wire.Value {
	if document.observation == nil {
		return jsonNull()
	}
	return jsonObject(
		member{"commandId", jsonString(document.observation.commandID)},
		member{"exitCode", jsonInt(document.observation.exitCode)},
		member{"id", jsonString(document.observation.id)},
		member{"reportSha256", jsonString(dynamic.report.sha256)},
	)
}

// encodeResult builds the TCQ-V0-036 document and its TCQ-V0-040 identity. The
// identity hashes the document without its own `id`, so no result can commit to
// an identity it did not derive.
func encodeResult(document parsedDocuments, dynamic dynamicContext, resolved Resolved, claims []ClaimResult, units map[string]testUnit, expected []byte) (Result, error) {
	claimValues := make([]wire.Value, 0, len(claims))
	for _, claim := range claims {
		claimValues = append(claimValues, claimValue(claim))
	}
	issues := issueValues(claims)
	if len(issues) > maxIssues {
		return Result{}, fail(CodeResourceExhausted)
	}
	commandID := jsonNull()
	if document.command != nil {
		commandID = jsonString(document.command.id)
	}
	body := jsonObject(
		member{"baseRevision", jsonString(resolved.BaseRevision)},
		member{"claims", jsonArray(claimValues)},
		member{"commandId", commandID},
		member{"issues", jsonArray(issues)},
		member{"observation", observationSummary(document, dynamic)},
		member{"ocmSha256", jsonString(document.ocm.sha256)},
		member{"profile", jsonString(Profile)},
		member{"targetRevision", jsonString(resolved.TargetRevision)},
		member{"units", jsonArray(unitValues(units))},
	)
	identity := tcqPrefix + domainHash(domainTCQ, canonicalValue(body))
	body.Obj.Keys = append(body.Obj.Keys, "id")
	body.Obj.Values["id"] = jsonString(identity)
	raw := canonicalJSON(body)
	if len(raw) > maxResultBytes {
		return Result{}, fail(CodeResourceExhausted)
	}
	if expected != nil && string(expected) != string(raw) {
		return Result{}, fail(CodeInvalidTCQ)
	}
	return Result{raw: raw, id: identity, claims: claims}, nil
}
