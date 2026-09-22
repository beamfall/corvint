package frontier

import (
	"regexp"

	"github.com/Beamfall/corvint/internal/wp3codec"
)

// lowerHex64 is the CF-V0-018 digest grammar. IDs reuse it after their own
// profile prefix.
var lowerHex64 = regexp.MustCompile(`^[0-9a-f]{64}$`)

// documentValue projects one Document into codec values. `withID` is false
// when building the CF-V0-019 identity preimage, which is the document without
// its `id` field.
func documentValue(document Document, withID bool) wp3codec.Value {
	members := []wp3codec.Member{
		{Key: "frontierState", Value: wp3codec.String(document.FrontierState)},
		{Key: "inputs", Value: inputsValue(document.Inputs)},
		{Key: "items", Value: itemsValue(document.Items)},
		{Key: "profile", Value: wp3codec.String(Profile)},
		{Key: "scope", Value: scopeValue(document.Scope)},
		{Key: "universeId", Value: wp3codec.String(document.UniverseID)},
	}
	if withID {
		members = append(members, wp3codec.Member{Key: "id", Value: wp3codec.String(document.ID)})
	}
	return wp3codec.Object(members...)
}

func inputsValue(inputs Inputs) wp3codec.Value {
	return wp3codec.Object(
		wp3codec.Member{Key: "cemSha256", Value: wp3codec.String(inputs.CEMSHA256)},
		wp3codec.Member{Key: "lrfSha256", Value: wp3codec.String(inputs.LRFSHA256)},
		wp3codec.Member{Key: "ocmSha256", Value: wp3codec.String(inputs.OCMSHA256)},
		wp3codec.Member{Key: "policy", Value: wp3codec.String(Policy)},
		wp3codec.Member{Key: "tcqId", Value: wp3codec.String(inputs.TCQID)},
		wp3codec.Member{Key: "testMode", Value: wp3codec.String(string(inputs.TestMode))},
	)
}

// scopeValue emits both intent offsets as canonical decimal strings. A
// negative offset has no canonical form; it cannot reach here because the
// verifier resolves the span from Git, so it degrades to the empty string
// rather than silently emitting a signed value.
func scopeValue(scope Scope) wp3codec.Value {
	return wp3codec.Object(
		wp3codec.Member{Key: "baseRevision", Value: wp3codec.String(scope.BaseRevision)},
		wp3codec.Member{Key: "excludedPath", Value: wp3codec.String(ExcludedPath)},
		wp3codec.Member{Key: "intentBlobOid", Value: wp3codec.String(scope.IntentBlobOID)},
		wp3codec.Member{Key: "intentPath", Value: wp3codec.String(scope.IntentPath)},
		wp3codec.Member{Key: "intentSpan", Value: wp3codec.Object(
			wp3codec.Member{Key: "end", Value: decimalOrEmpty(scope.IntentSpan.End)},
			wp3codec.Member{Key: "start", Value: decimalOrEmpty(scope.IntentSpan.Start)},
		)},
		wp3codec.Member{Key: "intentSpanSha256", Value: wp3codec.String(scope.IntentSpanSHA256)},
		wp3codec.Member{Key: "objectFormat", Value: wp3codec.String(scope.ObjectFormat)},
		wp3codec.Member{Key: "patchSha256", Value: wp3codec.String(scope.PatchSHA256)},
		wp3codec.Member{Key: "targetRevision", Value: wp3codec.String(scope.TargetRevision)},
	)
}

func decimalOrEmpty(value int64) wp3codec.Value {
	decimal, err := wp3codec.Decimal(value)
	if err != nil {
		return wp3codec.String("")
	}
	return decimal
}

func itemsValue(items []Item) wp3codec.Value {
	values := make([]wp3codec.Value, 0, len(items))
	for _, item := range items {
		values = append(values, itemValue(item))
	}
	return wp3codec.Array(values...)
}

func itemValue(item Item) wp3codec.Value {
	return wp3codec.Object(
		wp3codec.Member{Key: "authorityClass", Value: wp3codec.String(item.AuthorityClass)},
		wp3codec.Member{Key: "id", Value: wp3codec.String(item.ID)},
		wp3codec.Member{Key: "kind", Value: wp3codec.String(item.Kind)},
		wp3codec.Member{Key: "nextAction", Value: wp3codec.String(item.NextAction)},
		wp3codec.Member{Key: "reasons", Value: wp3codec.Strings(item.Reasons)},
		wp3codec.Member{Key: "relatedIds", Value: wp3codec.Strings(item.RelatedIDs)},
		wp3codec.Member{Key: "resolutionClass", Value: wp3codec.String(item.ResolutionClass)},
		wp3codec.Member{Key: "subjectId", Value: wp3codec.String(item.SubjectID)},
	)
}

// CanonicalBytes returns the complete Frontier document: codec(document) plus
// exactly one LF (CF-V0-019).
func CanonicalBytes(document Document) ([]byte, error) {
	encoded, err := wp3codec.Encode(documentValue(document, true))
	if err != nil {
		return nil, fail(CodeNoncanonical, "document is not codec-encodable")
	}
	return append(encoded, '\n'), nil
}

// seal completes cascade stage 9: Frontier bounds, canonical encoding, the
// state invariant, and the ID. It is the last step before output, so a
// violation here fails closed with no partial result.
func seal(document Document) (Document, []byte, *Error) {
	if err := checkLimits(document.Items); err != nil {
		return Document{}, nil, err
	}
	if err := checkStateLaw(document.FrontierState, len(document.Items)); err != nil {
		return Document{}, nil, err
	}
	for _, item := range document.Items {
		if err := checkItemContract(item); err != nil {
			return Document{}, nil, err
		}
	}
	if err := checkItemOrder(document.Items); err != nil {
		return Document{}, nil, err
	}
	if err := checkIdentityBindings(document); err != nil {
		return Document{}, nil, err
	}
	identity, err := documentID(document)
	if err != nil {
		return Document{}, nil, fail(CodeNoncanonical, "document identity is not derivable")
	}
	document.ID = identity
	encoded, encodeErr := CanonicalBytes(document)
	if encodeErr != nil {
		return Document{}, nil, encodeErr.(*Error)
	}
	// Byte arithmetic is checked before any caller allocates the output.
	if len(encoded) > MaxOutputBytes {
		return Document{}, nil, fail(CodeResourceExhausted, "output exceeds the frontier byte ceiling")
	}
	return document, encoded, nil
}

// checkStateLaw enforces CF-V0-004 exactly: EMPTY requires `items: []`, OPEN
// requires at least one item, and no other combination is valid.
func checkStateLaw(state string, count int) *Error {
	switch {
	case state == StateEmpty && count == 0:
		return nil
	case state == StateOpen && count > 0:
		return nil
	}
	return fail(CodeNoncanonical, "frontier state does not match the item count")
}

// checkLimits applies the CF-V0-023 counts. Counts are checked here as a
// whole-result assertion in addition to the per-append checks in the
// projection, so neither path can be the only guard.
func checkLimits(items []Item) *Error {
	counts := map[string]int{}
	for _, item := range items {
		counts[item.Kind]++
		if len(item.RelatedIDs) > MaxRelatedIDsPerItem {
			return fail(CodeResourceExhausted, "related id ceiling exceeded")
		}
		if len(item.Reasons) > MaxReasonsPerItem {
			return fail(CodeResourceExhausted, "reason ceiling exceeded")
		}
	}
	switch {
	case counts[KindHunkBasis] > MaxHunkItems:
		return fail(CodeResourceExhausted, "hunk item ceiling exceeded")
	case counts[KindIntentChange] > MaxIntentChangeItems:
		return fail(CodeResourceExhausted, "intent change item ceiling exceeded")
	case counts[KindIntentTest] > MaxIntentTestItems:
		return fail(CodeResourceExhausted, "intent test item ceiling exceeded")
	case len(items) > MaxTotalItems:
		return fail(CodeResourceExhausted, "total item ceiling exceeded")
	}
	return nil
}
