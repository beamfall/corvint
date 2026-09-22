package frontier

import (
	"bytes"
	"math"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/wp3codec"
)

// Closed key sets for the CF-V0-017 and CF-V0-018 shapes. "Closed" is the
// point: an extra field — a stop-decision field above all, which CF-V0-004
// forbids V0 from having — is `noncanonical-frontier`, not an ignored unknown.
var (
	documentKeys   = []string{"frontierState", "id", "inputs", "items", "profile", "scope", "universeId"}
	inputsKeys     = []string{"cemSha256", "lrfSha256", "ocmSha256", "policy", "tcqId", "testMode"}
	scopeKeys      = []string{"baseRevision", "excludedPath", "intentBlobOid", "intentPath", "intentSpan", "intentSpanSha256", "objectFormat", "patchSha256", "targetRevision"}
	intentSpanKeys = []string{"end", "start"}
	itemKeys       = []string{"authorityClass", "id", "kind", "nextAction", "reasons", "relatedIds", "resolutionClass", "subjectId"}
)

var (
	admittedStates      = closedSet(StateEmpty, StateOpen)
	admittedKinds       = closedSet(KindHunkBasis, KindIntentChange, KindIntentTest)
	admittedAuthorities = closedSet(AuthorityNone, AuthorityProducerDeclared, AuthorityCallerReported)
	admittedResolutions = closedSet(ResolutionActionable, ResolutionAuthorityRequired, ResolutionProfileRequired)
	admittedTestModes   = closedSet(string(TestModeStatic), string(TestModeDynamicCallerReported))
)

// VerifyDocument checks candidate Frontier bytes against the whole wire
// contract: the CF-V0-019 codec round trip, the closed CF-V0-017/018 shapes,
// the CF-V0-004 state law, the CF-V0-020 orders, the CF-V0-023 bounds, and the
// recomputed CF-V0-019 identity. Every violation is `noncanonical-frontier`,
// which is what makes an independent consumer able to refuse an almost-right
// document instead of half-reading it (CF-V0-028).
func VerifyDocument(data []byte) *Error {
	body, err := splitTerminalLF(data)
	if err != nil {
		return err
	}
	if codecErr := wp3codec.Verify(body); codecErr != nil {
		return fail(CodeNoncanonical, "document bytes are not canonical codec bytes")
	}
	root, parseErr := wp3codec.Parse(body)
	if parseErr != nil {
		return fail(CodeNoncanonical, "document bytes are not parseable")
	}
	document, err := decodeDocument(root)
	if err != nil {
		return err
	}
	if err := checkIdentityBindings(document); err != nil {
		return err
	}
	if err := checkLimits(document.Items); err != nil {
		return err
	}
	if err := checkStateLaw(document.FrontierState, len(document.Items)); err != nil {
		return err
	}
	identity, identityErr := documentID(document)
	if identityErr != nil || identity != document.ID {
		return fail(CodeNoncanonical, "document id does not bind its own content")
	}
	return nil
}

// checkIdentityBindings re-derives the two nested identity layers before the
// outer document ID. Syntax alone is insufficient: otherwise a producer can
// wrap invented universe or item IDs in an internally consistent outer hash.
func checkIdentityBindings(document Document) *Error {
	if err := checkScopeContract(document.Scope); err != nil {
		return err
	}
	universeID, err := UniverseID(document.Scope)
	if err != nil || universeID != document.UniverseID {
		return fail(CodeNoncanonical, "universe id does not bind scope")
	}
	for _, item := range document.Items {
		itemID, err := ItemID(item.Kind, item.SubjectID, document.UniverseID)
		if err != nil || itemID != item.ID {
			return fail(CodeNoncanonical, "item id does not bind kind, subject, and universe")
		}
	}
	return nil
}

func checkScopeContract(scope Scope) *Error {
	oidBytes := 40
	if scope.ObjectFormat == "sha256" {
		oidBytes = 64
	}
	for _, oid := range []string{scope.BaseRevision, scope.TargetRevision, scope.IntentBlobOID} {
		if len(oid) != oidBytes || !isLowerHex(oid) {
			return fail(CodeNoncanonical, "scope object id does not match object format")
		}
	}
	if err := wire.ValidatePath(scope.IntentPath); err != nil {
		return fail(CodeNoncanonical, "intent path is not canonical")
	}
	if scope.IntentSpan.Start < 0 || scope.IntentSpan.End <= scope.IntentSpan.Start {
		return fail(CodeNoncanonical, "intent span is not nonempty")
	}
	if !lowerHex64.MatchString(scope.IntentSpanSHA256) || !lowerHex64.MatchString(scope.PatchSHA256) {
		return fail(CodeNoncanonical, "scope digest is not lowercase 64-hex")
	}
	return nil
}

func isLowerHex(value string) bool {
	for _, digit := range []byte(value) {
		if (digit < '0' || digit > '9') && (digit < 'a' || digit > 'f') {
			return false
		}
	}
	return true
}

// splitTerminalLF enforces "complete Frontier documents are codec(document)
// plus exactly one LF" (CF-V0-019).
func splitTerminalLF(data []byte) ([]byte, *Error) {
	if len(data) == 0 || data[len(data)-1] != '\n' {
		return nil, fail(CodeNoncanonical, "document does not end with exactly one LF")
	}
	body := data[:len(data)-1]
	if bytes.Contains(body, []byte{'\n'}) {
		return nil, fail(CodeNoncanonical, "document contains an interior LF")
	}
	return body, nil
}

func decodeDocument(root wp3codec.Value) (Document, *Error) {
	if err := requireExactKeys(root, documentKeys); err != nil {
		return Document{}, err
	}
	if err := requireLiteral(root, "profile", Profile); err != nil {
		return Document{}, err
	}
	state, err := requireEnum(root, "frontierState", admittedStates)
	if err != nil {
		return Document{}, err
	}
	identity, err := requirePrefixedID(root, "id", documentIDPrefix)
	if err != nil {
		return Document{}, err
	}
	universeID, err := requirePrefixedID(root, "universeId", universeIDPrefix)
	if err != nil {
		return Document{}, err
	}
	inputs, err := decodeInputs(root)
	if err != nil {
		return Document{}, err
	}
	scope, err := decodeScope(root)
	if err != nil {
		return Document{}, err
	}
	items, err := decodeItems(root)
	if err != nil {
		return Document{}, err
	}
	return Document{
		FrontierState: state,
		ID:            identity,
		Inputs:        inputs,
		Items:         items,
		Scope:         scope,
		UniverseID:    universeID,
	}, nil
}

func decodeInputs(root wp3codec.Value) (Inputs, *Error) {
	object, err := requireObject(root, "inputs", inputsKeys)
	if err != nil {
		return Inputs{}, err
	}
	if err := requireLiteral(object, "policy", Policy); err != nil {
		return Inputs{}, err
	}
	mode, err := requireEnum(object, "testMode", admittedTestModes)
	if err != nil {
		return Inputs{}, err
	}
	cemDigest, err := requireDigest(object, "cemSha256")
	if err != nil {
		return Inputs{}, err
	}
	lrfDigest, err := requireDigest(object, "lrfSha256")
	if err != nil {
		return Inputs{}, err
	}
	ocmDigest, err := requireDigest(object, "ocmSha256")
	if err != nil {
		return Inputs{}, err
	}
	tcqID, err := requirePrefixedID(object, "tcqId", "tcq:sha256:")
	if err != nil {
		return Inputs{}, err
	}
	return Inputs{
		CEMSHA256: cemDigest,
		LRFSHA256: lrfDigest,
		OCMSHA256: ocmDigest,
		TCQID:     tcqID,
		TestMode:  TestMode(mode),
	}, nil
}

func decodeScope(root wp3codec.Value) (Scope, *Error) {
	object, err := requireObject(root, "scope", scopeKeys)
	if err != nil {
		return Scope{}, err
	}
	if err := requireLiteral(object, "excludedPath", ExcludedPath); err != nil {
		return Scope{}, err
	}
	span, err := decodeSpan(object)
	if err != nil {
		return Scope{}, err
	}
	format, err := requireEnum(object, "objectFormat", closedSet("sha1", "sha256"))
	if err != nil {
		return Scope{}, err
	}
	patchDigest, err := requireDigest(object, "patchSha256")
	if err != nil {
		return Scope{}, err
	}
	spanDigest, err := requireDigest(object, "intentSpanSha256")
	if err != nil {
		return Scope{}, err
	}
	base, err := requireString(object, "baseRevision")
	if err != nil {
		return Scope{}, err
	}
	target, err := requireString(object, "targetRevision")
	if err != nil {
		return Scope{}, err
	}
	blob, err := requireString(object, "intentBlobOid")
	if err != nil {
		return Scope{}, err
	}
	path, err := requireString(object, "intentPath")
	if err != nil {
		return Scope{}, err
	}
	return Scope{
		BaseRevision:     base,
		IntentBlobOID:    blob,
		IntentPath:       path,
		IntentSpan:       span,
		IntentSpanSHA256: spanDigest,
		ObjectFormat:     format,
		PatchSHA256:      patchDigest,
		TargetRevision:   target,
	}, nil
}

// decodeSpan enforces the codec's number ban at the one place a producer is
// most tempted to break it: both offsets are canonical decimal strings.
func decodeSpan(scope wp3codec.Value) (Span, *Error) {
	object, err := requireObject(scope, "intentSpan", intentSpanKeys)
	if err != nil {
		return Span{}, err
	}
	start, err := requireDecimal(object, "start")
	if err != nil {
		return Span{}, err
	}
	end, err := requireDecimal(object, "end")
	if err != nil {
		return Span{}, err
	}
	return Span{Start: start, End: end}, nil
}

func decodeItems(root wp3codec.Value) ([]Item, *Error) {
	value, found := root.Lookup("items")
	if !found || value.Kind() != wp3codec.KindArray {
		return nil, fail(CodeNoncanonical, "items must be an array")
	}
	items := make([]Item, 0, len(value.Items()))
	for _, entry := range value.Items() {
		item, err := decodeItem(entry)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := checkItemOrder(items); err != nil {
		return nil, err
	}
	return items, nil
}

func decodeItem(entry wp3codec.Value) (Item, *Error) {
	if entry.Kind() != wp3codec.KindObject {
		return Item{}, fail(CodeNoncanonical, "item must be an object")
	}
	if err := requireExactKeys(entry, itemKeys); err != nil {
		return Item{}, err
	}
	kind, err := requireEnum(entry, "kind", admittedKinds)
	if err != nil {
		return Item{}, err
	}
	authority, err := requireEnum(entry, "authorityClass", admittedAuthorities)
	if err != nil {
		return Item{}, err
	}
	resolution, err := requireEnum(entry, "resolutionClass", admittedResolutions)
	if err != nil {
		return Item{}, err
	}
	identity, err := requirePrefixedID(entry, "id", itemIDPrefix)
	if err != nil {
		return Item{}, err
	}
	subject, err := requireString(entry, "subjectId")
	if err != nil {
		return Item{}, err
	}
	action, err := requireString(entry, "nextAction")
	if err != nil {
		return Item{}, err
	}
	reasons, err := requireStringArray(entry, "reasons")
	if err != nil {
		return Item{}, err
	}
	related, err := requireStringArray(entry, "relatedIds")
	if err != nil {
		return Item{}, err
	}
	item := Item{
		AuthorityClass:  authority,
		ID:              identity,
		Kind:            kind,
		NextAction:      action,
		Reasons:         reasons,
		RelatedIDs:      related,
		ResolutionClass: resolution,
		SubjectID:       subject,
	}
	return item, checkItemContract(item)
}

// checkItemContract enforces the parts of CF-V0-016, CF-V0-017 and CF-V0-020
// that are visible from the document alone: the frozen reason order, the
// first-reason disposition rule, and lexicographic related IDs.
func checkItemContract(item Item) *Error {
	if len(item.Reasons) == 0 {
		return fail(CodeNoncanonical, "item has no retained reason")
	}
	order := reasonOrders[item.Kind]
	previous := -1
	seen := map[string]bool{}
	for _, reason := range item.Reasons {
		position := indexOf(order, reason)
		switch {
		case position < 0:
			return fail(CodeNoncanonical, "reason is not admitted for its kind")
		case seen[reason]:
			return fail(CodeNoncanonical, "reasons must be unique")
		case position <= previous:
			return fail(CodeNoncanonical, "reasons are not in the frozen order")
		}
		seen[reason] = true
		previous = position
	}
	chosen := reasonDispositions[item.Kind][item.Reasons[0]]
	if chosen.resolution != item.ResolutionClass || chosen.action != item.NextAction {
		return fail(CodeNoncanonical, "resolution or action does not follow the first retained reason")
	}
	if item.AuthorityClass != expectedItemAuthority(item.Kind, item.Reasons[0]) {
		return fail(CodeNoncanonical, "authority does not follow the item kind and reason")
	}
	if !sort.StringsAreSorted(item.RelatedIDs) {
		return fail(CodeNoncanonical, "related ids are not lexicographically sorted")
	}
	for index := 1; index < len(item.RelatedIDs); index++ {
		if item.RelatedIDs[index] == item.RelatedIDs[index-1] {
			return fail(CodeNoncanonical, "related ids must be unique")
		}
	}
	return nil
}

func expectedItemAuthority(kind, firstReason string) string {
	if kind == KindIntentTest {
		return AuthorityCallerReported
	}
	switch firstReason {
	case ReasonHunkNoEvidence, ReasonHunkInsufficientEvidence, ReasonHunkConflictingEvidence,
		ReasonObligationUnassessed, ReasonObligationNoTestClaim,
		ReasonObligationInsufficientEvidence, ReasonObligationConflictingEvidence:
		return AuthorityNone
	default:
		return AuthorityProducerDeclared
	}
}

// checkItemOrder enforces the CF-V0-020 kind order. Patch and requirement
// order within a kind cannot be re-derived from the document alone, so they
// are asserted by the projection tests rather than here.
func checkItemOrder(items []Item) *Error {
	for index := 1; index < len(items); index++ {
		if kindOrder[items[index].Kind] < kindOrder[items[index-1].Kind] {
			return fail(CodeNoncanonical, "items are not in the frozen kind order")
		}
	}
	return nil
}

func requireExactKeys(object wp3codec.Value, expected []string) *Error {
	if object.Kind() != wp3codec.KindObject {
		return fail(CodeNoncanonical, "value must be an object")
	}
	keys := object.Keys()
	if len(keys) != len(expected) {
		return fail(CodeNoncanonical, "object does not have exactly the closed key set")
	}
	sorted := append([]string(nil), expected...)
	sort.Strings(sorted)
	for index, key := range keys {
		if key != sorted[index] {
			return fail(CodeNoncanonical, "object does not have exactly the closed key set")
		}
	}
	return nil
}

func requireObject(parent wp3codec.Value, key string, expected []string) (wp3codec.Value, *Error) {
	value, found := parent.Lookup(key)
	if !found {
		return wp3codec.Value{}, fail(CodeNoncanonical, "missing required object")
	}
	return value, requireExactKeys(value, expected)
}

func requireString(object wp3codec.Value, key string) (string, *Error) {
	value, found := object.Lookup(key)
	if !found || value.Kind() != wp3codec.KindString {
		return "", fail(CodeNoncanonical, "field must be a string")
	}
	return value.Text(), nil
}

func requireLiteral(object wp3codec.Value, key, literal string) *Error {
	text, err := requireString(object, key)
	if err != nil {
		return err
	}
	if text != literal {
		return fail(CodeNoncanonical, "field does not carry its frozen literal")
	}
	return nil
}

func requireEnum(object wp3codec.Value, key string, admitted map[string]bool) (string, *Error) {
	text, err := requireString(object, key)
	if err != nil {
		return "", err
	}
	if !admitted[text] {
		return "", fail(CodeNoncanonical, "field value is not admitted")
	}
	return text, nil
}

func requireDigest(object wp3codec.Value, key string) (string, *Error) {
	text, err := requireString(object, key)
	if err != nil {
		return "", err
	}
	if !lowerHex64.MatchString(text) {
		return "", fail(CodeNoncanonical, "digest is not lowercase 64-hex")
	}
	return text, nil
}

func requirePrefixedID(object wp3codec.Value, key, prefix string) (string, *Error) {
	text, err := requireString(object, key)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(text, prefix) || !lowerHex64.MatchString(strings.TrimPrefix(text, prefix)) {
		return "", fail(CodeNoncanonical, "id does not use its owning profile grammar")
	}
	return text, nil
}

// requireDecimal reads a count, ordinal, or offset in its only admitted form:
// a base-10 string with no sign and no leading zero except "0".
func requireDecimal(object wp3codec.Value, key string) (int64, *Error) {
	text, err := requireString(object, key)
	if err != nil {
		return 0, err
	}
	if text == "" || (len(text) > 1 && text[0] == '0') {
		return 0, fail(CodeNoncanonical, "decimal string is not canonical")
	}
	var parsed int64
	for _, digit := range []byte(text) {
		if digit < '0' || digit > '9' {
			return 0, fail(CodeNoncanonical, "decimal string is not canonical")
		}
		if parsed > (math.MaxInt64-int64(digit-'0'))/10 {
			return 0, fail(CodeNoncanonical, "decimal string exceeds the int64 range")
		}
		parsed = parsed*10 + int64(digit-'0')
	}
	return parsed, nil
}

func requireStringArray(object wp3codec.Value, key string) ([]string, *Error) {
	value, found := object.Lookup(key)
	if !found || value.Kind() != wp3codec.KindArray {
		return nil, fail(CodeNoncanonical, "field must be an array")
	}
	items := make([]string, 0, len(value.Items()))
	for _, entry := range value.Items() {
		if entry.Kind() != wp3codec.KindString {
			return nil, fail(CodeNoncanonical, "array must contain only strings")
		}
		items = append(items, entry.Text())
	}
	return items, nil
}

func indexOf(values []string, target string) int {
	for index, value := range values {
		if value == target {
			return index
		}
	}
	return -1
}
