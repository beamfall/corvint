package lrfrepo

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/publish"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// The OCM write actions mutate the parsed JSON TREE rather than a typed struct.
// The oracle deep-copies the loaded document, edits it, and re-serializes it
// canonically, so any member the Go types do not model still has to survive the
// round trip byte-for-byte.

// unknownReasons is the oracle's bounded allowlist for `ocm mark`.
var unknownReasons = []string{"conflicting-evidence", "insufficient-evidence", "no-test-claim", "unassessed"}

var decimalSelector = regexp.MustCompile(`^[1-9][0-9]*$`)

// MarkOptions configure one `ocm mark` invocation.
type MarkOptions struct {
	MapPath    string
	Output     string
	Obligation string
	Reason     string
}

// MarkResult reports what one mark changed.
type MarkResult struct {
	MapPath      string
	ObligationID string
}

// resolveObligationSelector accepts a requirement ID or its canonical one-based
// map ordinal, exactly as the oracle does. A selector that merely STARTS with a
// digit is rejected rather than treated as an ID, so "01" cannot silently mean
// something other than the first obligation.
func resolveObligationSelector(obligations []wire.Value, selector string) (string, error) {
	if len(obligations) > maxObligations {
		return "", &Error{Code: "invalid-obligations", Message: "obligations must be a bounded array"}
	}
	if !decimalSelector.MatchString(selector) {
		if selector != "" && selector[0] >= '0' && selector[0] <= '9' {
			return "", &Error{Code: "invalid-obligation-selector",
				Message: "decimal obligation selectors must be canonical and one-based"}
		}
		return selector, nil
	}
	if len(selector) > len(strconv.Itoa(maxObligations)) {
		return "", &Error{Code: "obligation-selector-out-of-range", Message: "obligation selector is outside the worklist"}
	}
	ordinal, err := strconv.Atoi(selector)
	if err != nil || ordinal > len(obligations) {
		return "", &Error{Code: "obligation-selector-out-of-range", Message: "obligation selector is outside the worklist"}
	}
	selected := obligations[ordinal-1]
	if selected.Kind != wire.KindObject {
		return "", &Error{Code: "invalid-obligation", Message: "selected obligation has no valid id"}
	}
	id, ok := selected.Obj.Get("id")
	if !ok || id.Kind != wire.KindString {
		return "", &Error{Code: "invalid-obligation", Message: "selected obligation has no valid id"}
	}
	return id.Str, nil
}

func objectMember(object *wire.Object, key string, value wire.Value) {
	if _, exists := object.Values[key]; !exists {
		object.Keys = append(object.Keys, key)
	}
	object.Values[key] = value
}

func stringValue(text string) wire.Value { return wire.Value{Kind: wire.KindString, Str: text} }

func emptyArray() wire.Value { return wire.Value{Kind: wire.KindArray, Arr: []wire.Value{}} }

// markObligation applies the oracle's mark: the obligation becomes unknown with
// the given reason and loses its links, and every claim no longer referenced by
// ANY obligation is pruned. Pruning matters because a claim pins a blob span;
// leaving orphans behind would keep verifying evidence nothing still cites.
func markObligation(document wire.Value, obligationID, reason string) (wire.Value, error) {
	if !containsString(unknownReasons, reason) {
		return wire.Value{}, &Error{Code: "invalid-unknown-reason", Message: "unknown reason is outside the allowlist"}
	}
	root, err := requireObject(document, "OCM map")
	if err != nil {
		return wire.Value{}, err
	}
	obligations, err := requireArray(root, "obligations")
	if err != nil {
		return wire.Value{}, err
	}
	found := false
	for _, item := range obligations {
		object, objectErr := requireObject(item, "obligation")
		if objectErr != nil {
			return wire.Value{}, objectErr
		}
		id, ok := object.Get("id")
		if !ok || id.Kind != wire.KindString || id.Str != obligationID {
			continue
		}
		found = true
		objectMember(object, "disposition", stringValue("unknown"))
		objectMember(object, "reason", stringValue(reason))
		objectMember(object, "hunkIds", emptyArray())
		objectMember(object, "claimIds", emptyArray())
	}
	if !found {
		return wire.Value{}, &Error{Code: "unknown-obligation-id", Message: "obligation ID must select exactly one row"}
	}
	return pruneUnreferencedClaims(document, root, obligations)
}

func pruneUnreferencedClaims(document wire.Value, root *wire.Object, obligations []wire.Value) (wire.Value, error) {
	used := map[string]struct{}{}
	for _, item := range obligations {
		if item.Kind != wire.KindObject {
			continue
		}
		ids, ok := item.Obj.Get("claimIds")
		if !ok || ids.Kind != wire.KindArray {
			continue
		}
		for _, id := range ids.Arr {
			if id.Kind == wire.KindString {
				used[id.Str] = struct{}{}
			}
		}
	}
	claims, ok := root.Get("claims")
	if !ok || claims.Kind != wire.KindArray {
		return wire.Value{}, &Error{Code: "invalid-claims", Message: "claims must be an array"}
	}
	kept := make([]wire.Value, 0, len(claims.Arr))
	for _, claim := range claims.Arr {
		if claim.Kind != wire.KindObject {
			continue
		}
		if id, present := claim.Obj.Get("id"); present && id.Kind == wire.KindString {
			if _, referenced := used[id.Str]; referenced {
				kept = append(kept, claim)
			}
		}
	}
	objectMember(root, "claims", wire.Value{Kind: wire.KindArray, Arr: kept})
	return document, nil
}

func requireObject(value wire.Value, subject string) (*wire.Object, error) {
	if value.Kind != wire.KindObject || value.Obj == nil {
		return nil, &Error{Code: "invalid-ocm", Message: fmt.Sprintf("%s must be an object", subject)}
	}
	return value.Obj, nil
}

func requireArray(root *wire.Object, key string) ([]wire.Value, error) {
	value, ok := root.Get(key)
	if !ok || value.Kind != wire.KindArray {
		return nil, &Error{Code: "invalid-" + key, Message: key + " must be a bounded array"}
	}
	return value.Arr, nil
}

func containsString(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

// UnknownReasons returns the mark reason allowlist in the oracle's sorted order.
func UnknownReasons() []string { return append([]string(nil), unknownReasons...) }

func canonicalOCMBytes(document wire.Value) []byte {
	return append(canonicalValue(document), '\n')
}

// MarkOCM applies one `ocm mark` and republishes the map. The map is re-read,
// mutated, and written whole; nothing is patched in place.
func MarkOCM(ctx context.Context, root string, options MarkOptions) (*MarkResult, error) {
	inputRoot, err := publish.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	raw, err := readRootInput(inputRoot, options.MapPath, maxOCMBytes, cemcode.MapUnavailable)
	if err != nil {
		return nil, ocmInputFailure(inputRoot.Path(), options.MapPath, err, "OCM map", maxOCMBytes)
	}
	document, err := wire.Parse(raw)
	if err != nil || document.Kind != wire.KindObject {
		return nil, fail("unsupported-ocm-json-profile", "native Go OCM does not approximate this JSON failure profile")
	}
	if _, err := parseOCM(raw); err != nil {
		return nil, err
	}
	// mark consumes no CEM; the CEM contract fixes its path (decision 0273).
	if options.Output != "" && sameInputFile(inputRoot.Path(), options.Output, wire.ExcludedCEMPath) {
		return nil, &Error{Code: "output-path-conflict", Message: "OCM and CEM map paths must differ"}
	}
	obligations, err := requireArray(document.Obj, "obligations")
	if err != nil {
		return nil, err
	}
	obligationID, err := resolveObligationSelector(obligations, options.Obligation)
	if err != nil {
		return nil, err
	}
	updated, err := markObligation(document, obligationID, options.Reason)
	if err != nil {
		return nil, err
	}
	payload := canonicalOCMBytes(updated)
	if len(payload) > maxOCMBytes {
		return nil, &Error{Code: "map-too-large", Message: "OCM map exceeds the size limit"}
	}
	destination := options.Output
	if destination == "" {
		destination = options.MapPath
	}
	written, err := publishOCMMap(ctx, inputRoot, destination, payload)
	if err != nil {
		return nil, err
	}
	return &MarkResult{MapPath: written, ObligationID: obligationID}, nil
}

// publishOCMMap writes the map through the same confined, owner-private path the
// report publisher uses, so a mutating action cannot escape the repository root
// or follow a symlink.
func publishOCMMap(ctx context.Context, root *publish.Root, relative string, data []byte) (string, error) {
	if relative == "" || relative == "." {
		return "", fail("ocm-cli-error", "output path must be normalized")
	}
	if filepath.IsAbs(relative) {
		return "", fail("unsupported-ocm-output-path", "native Go OCM map output must remain inside the repository root")
	}
	if err := wire.ValidatePath(relative); err != nil {
		return "", fail("unsupported-ocm-output-path", "native Go OCM map output must be normalized and repository-relative")
	}
	return publishOCMPath(ctx, root, relative, data, true, false)
}

// PrepareOptions configure one `ocm prepare` invocation.
type PrepareOptions struct {
	MapPath      string
	CEMPath      string
	IntentPath   string
	Target       string
	ExpectedBase string
	Replace      bool
	MaxUnknown   *int
}

// DefaultOCMMapPath is the oracle's default OCM map location.
const DefaultOCMMapPath = ".corvint/change.ocm.json"

// OCMCountsAndWorklist exposes the shared counts/worklist derivation so prepare
// reports exactly what status reports.
func OCMCountsAndWorklist(document wire.Value) (map[string]any, []any) {
	return ocmCountsAndWorklist(document)
}

// PrepareResult reports what one prepare created or resumed.
type PrepareResult struct {
	Document     wire.Value
	MapAbsolute  string
	CEMAbsolute  string
	Target       string
	IntentScope  map[string]any
	Resumed      bool
	Requirements []string
	StatusAction []any
}

// PrepareOCM creates or safely resumes the local OCM reviewer artifact. It is
// the only write action that can invent a document, so it verifies the CEM
// first: every obligation it writes is derived from an intent blob pinned at a
// target the CEM already binds.
func PrepareOCM(ctx context.Context, root string, options PrepareOptions) (*PrepareResult, error) {
	inputRoot, err := publish.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	cemRaw, err := readRootInput(inputRoot, options.CEMPath, wire.MaxMapBytes, cemcode.MapUnavailable)
	if err != nil {
		return nil, ocmInputFailure(inputRoot.Path(), options.CEMPath, err, "CEM map", wire.MaxMapBytes)
	}
	if sameInputFile(inputRoot.Path(), options.MapPath, options.CEMPath) {
		return nil, &Error{Code: "output-path-conflict", Message: "OCM and CEM map paths must differ"}
	}
	repository, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		return nil, err
	}
	cem, err := wire.ParseMap(cemRaw)
	if err != nil {
		return nil, err
	}
	candidate, requirements, target, err := beginOCM(ctx, repository, cem, cemRaw, options)
	if err != nil {
		return nil, err
	}
	document, resumed, rewrite, err := resumeOrReplace(ctx, repository, inputRoot, cemRaw, candidate, options)
	if err != nil {
		return nil, err
	}
	payload := canonicalOCMBytes(document)
	if len(payload) > maxOCMBytes {
		return nil, &Error{Code: "map-too-large", Message: "OCM map exceeds the size limit"}
	}
	written := absoluteInput(inputRoot.Path(), options.MapPath)
	if !resumed || rewrite {
		if written, err = publishOCMMap(ctx, inputRoot, options.MapPath, payload); err != nil {
			return nil, err
		}
	}
	scope, err := decodeIntentScope(document)
	if err != nil {
		return nil, err
	}
	return &PrepareResult{
		StatusAction: prepareStatusAction(ctx, repository, cem, options),
		Document:     document, MapAbsolute: written,
		CEMAbsolute: absoluteInput(inputRoot.Path(), options.CEMPath),
		Target:      target, IntentScope: scope, Resumed: resumed, Requirements: requirements,
	}, nil
}

// beginOCM builds the deterministic candidate: every exact requirement in the
// intent scope becomes one visibly unknown obligation.
func beginOCM(ctx context.Context, repository *gitauth.Repository, cem *wire.Map, cemRaw []byte, options PrepareOptions) (wire.Value, []string, string, error) {
	// A canonical CEM binds both endpoints, so the caller must name the target by
	// full object id; a ref could move between this call and the next.
	if cem.Spec == wire.Spec02 && !fullObjectID(options.Target) {
		return wire.Value{}, nil, "", &Error{Code: "invalid-target-revision", Message: "target must be a full commit OID"}
	}
	verified, err := verifyPrepareCEM(ctx, repository, cem, cemRaw, options)
	if err != nil {
		return wire.Value{}, nil, "", err
	}
	target, err := repository.Resolve(ctx, options.Target)
	if err != nil {
		return wire.Value{}, nil, "", err
	}
	reader := &ocmBlobReader{ctx: ctx, repository: repository, cache: map[string][]byte{}}
	entry, exists, err := repository.LookupTreeEntry(ctx, target, options.IntentPath)
	if err != nil {
		return wire.Value{}, nil, "", err
	}
	if !exists || entry.Type != "blob" {
		return wire.Value{}, nil, "", fail("repository-object-unavailable", "repository object unavailable")
	}
	blob, err := reader.blob(target, options.IntentPath, entry.OID)
	if err != nil {
		return wire.Value{}, nil, "", err
	}
	intent, requirements, _, err := requirementsFromBlob(options.IntentPath, entry.OID, blob)
	if err != nil {
		return wire.Value{}, nil, "", err
	}
	if err := enforceBootstrap(ctx, repository, cem, options.IntentPath); err != nil {
		return wire.Value{}, nil, "", err
	}
	return buildOCMCandidate(target, intent, requirements, sha256Hex(cemRaw), sha256Hex(verified.patch)), requirements, target, nil
}

func verifyPrepareCEM(ctx context.Context, repository *gitauth.Repository, cem *wire.Map, cemRaw []byte, options PrepareOptions) (*verifiedCEM, error) {
	document := &ocmDocument{target: options.Target}
	resolved, err := repository.Resolve(ctx, options.Target)
	if err != nil {
		return nil, err
	}
	document.target = resolved
	return verifyReadCEM(ctx, repository, cem, cemRaw, document, OCMReadOptions{
		ExpectedBase: options.ExpectedBase, Target: options.Target,
	})
}

func objectValue(pairs ...[2]any) wire.Value {
	object := &wire.Object{Values: map[string]wire.Value{}}
	for _, pair := range pairs {
		key := pair[0].(string)
		object.Keys = append(object.Keys, key)
		object.Values[key] = pair[1].(wire.Value)
	}
	return wire.Value{Kind: wire.KindObject, Obj: object}
}

func intValue(value int64) wire.Value { return wire.Value{Kind: wire.KindInt, Int: value} }

// buildOCMCandidate renders the same document the oracle's begin_ocm returns.
// It is built as a tree so that prepare, mark, and link all serialize through
// one canonical encoder.
func buildOCMCandidate(target string, intent ocmIntent, requirements []string, mapSHA256, patchSHA256 string) wire.Value {
	obligations := make([]wire.Value, 0, len(requirements))
	for _, identity := range requirements {
		obligations = append(obligations, objectValue(
			[2]any{"id", stringValue(identity)},
			[2]any{"disposition", stringValue("unknown")},
			[2]any{"reason", stringValue("unassessed")},
			[2]any{"hunkIds", emptyArray()},
			[2]any{"claimIds", emptyArray()},
		))
	}
	return objectValue(
		[2]any{"spec", stringValue(ocmSpec)},
		[2]any{"targetRevision", stringValue(target)},
		[2]any{"intentScope", objectValue(
			[2]any{"path", stringValue(intent.path)},
			[2]any{"blobOid", stringValue(intent.blobOID)},
			[2]any{"span", objectValue(
				[2]any{"start", intValue(intent.start)},
				[2]any{"end", intValue(intent.end)},
			)},
			[2]any{"spanSha256", stringValue(intent.spanSHA256)},
		)},
		[2]any{"cem", objectValue(
			[2]any{"mapSha256", stringValue(mapSHA256)},
			[2]any{"patchSha256", stringValue(patchSHA256)},
		)},
		[2]any{"claims", emptyArray()},
		[2]any{"obligations", wire.Value{Kind: wire.KindArray, Arr: obligations}},
	)
}

// resumeOrReplace applies the oracle's resume rules. An existing map that binds
// the same intent and CEM is kept when either its target is unchanged or the
// target moved only by the excluded sidecar, so a reviewer's recorded links
// survive a re-run. Other binding changes are refused unless --replace was
// passed, because silently discarding review is not detectable afterward.
func resumeOrReplace(ctx context.Context, repository *gitauth.Repository, root *publish.Root, cemRaw []byte, candidate wire.Value, options PrepareOptions) (wire.Value, bool, bool, error) {
	raw, err := readRootInput(root, options.MapPath, maxOCMBytes, cemcode.MapUnavailable)
	if err != nil {
		return candidate, false, false, unreadableExistingMap(root.Path(), options, err)
	}
	existing, parseErr := wire.Parse(raw)
	if parseErr != nil || existing.Kind != wire.KindObject {
		if options.Replace {
			return candidate, false, false, nil
		}
		return wire.Value{}, false, false, ocmJSONFailure(existing, parseErr)
	}
	if sameOCMBinding(existing, candidate) {
		checked, err := readOCMBytes(ctx, root, raw, cemRaw, OCMReadOptions{
			OCMPath: options.MapPath, CEMPath: options.CEMPath,
			ExpectedBase: options.ExpectedBase, Target: options.Target,
			ExpectedBaseGiven: options.ExpectedBase != "", TargetGiven: options.Target != "",
		})
		if err != nil {
			return wire.Value{}, false, false, err
		}
		if err := ocmLinkVerificationError(checked); err != nil {
			if options.Replace {
				return candidate, false, false, nil
			}
			return wire.Value{}, false, false, err
		}
		return existing, true, false, nil
	}
	if !options.Replace && movedTargetBinding(ctx, repository, existing, candidate) {
		claims, claimsOK := existing.Obj.Get("claims")
		obligations, obligationsOK := existing.Obj.Get("obligations")
		if claimsOK && obligationsOK {
			objectMember(candidate.Obj, "claims", claims)
			objectMember(candidate.Obj, "obligations", obligations)
			return candidate, true, true, nil
		}
	}
	if !options.Replace {
		return wire.Value{}, false, false, &Error{Code: "map-outdated",
			Message: "existing OCM map binds a different target, intent, or CEM"}
	}
	return candidate, false, false, nil
}

// movedTargetBinding recognizes a target advanced only by the excluded CEM
// sidecar. The exact CEM and intent bindings prove the base, patch, hunks, and
// evidence are unchanged, so reviewer-owned claims and assessments remain
// valid after re-pinning the target.
func movedTargetBinding(ctx context.Context, repository *gitauth.Repository, existing, candidate wire.Value) bool {
	existingTarget, existingOK := existing.Obj.Get("targetRevision")
	candidateTarget, candidateOK := candidate.Obj.Get("targetRevision")
	if !existingOK || !candidateOK || existingTarget.Kind != wire.KindString || candidateTarget.Kind != wire.KindString {
		return false
	}
	if existingTarget.Str == candidateTarget.Str {
		return false
	}
	for _, member := range []string{"intentScope", "cem"} {
		existingValue, existingOK := existing.Obj.Get(member)
		candidateValue, candidateOK := candidate.Obj.Get(member)
		if existingOK != candidateOK || !bytesEqual(canonicalValue(existingValue), canonicalValue(candidateValue)) {
			return false
		}
	}
	drift, err := repository.CanonicalDiff(ctx, existingTarget.Str, candidateTarget.Str)
	return err == nil && len(drift) == 0
}

// sameOCMBinding compares only the binding members. Claims and obligations are
// the reviewer's work and are deliberately NOT part of the comparison.
func sameOCMBinding(left, right wire.Value) bool {
	for _, member := range []string{"targetRevision", "intentScope", "cem"} {
		leftValue, leftOK := left.Obj.Get(member)
		rightValue, rightOK := right.Obj.Get(member)
		if leftOK != rightOK || !bytesEqual(canonicalValue(leftValue), canonicalValue(rightValue)) {
			return false
		}
	}
	return true
}

func bytesEqual(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func decodeIntentScope(document wire.Value) (map[string]any, error) {
	scope, ok := document.Obj.Get("intentScope")
	if !ok || scope.Kind != wire.KindObject {
		return nil, fail("invalid-intent-scope", "intentScope must be an object")
	}
	span, ok := scope.Obj.Get("span")
	if !ok || span.Kind != wire.KindObject {
		return nil, fail("invalid-intent-scope", "intentScope span must be an object")
	}
	start, _ := span.Obj.Get("start")
	end, _ := span.Obj.Get("end")
	path, _ := scope.Obj.Get("path")
	blob, _ := scope.Obj.Get("blobOid")
	digest, _ := scope.Obj.Get("spanSha256")
	return map[string]any{
		"path": path.Str, "blobOid": blob.Str, "spanSha256": digest.Str,
		"span": map[string]any{"start": start.Int, "end": end.Int},
	}, nil
}

// prepareStatusAction renders the exact follow-up command the oracle prints. A
// canonical CEM binds both endpoints, so the action names the RESOLVED base and
// target rather than whatever the caller typed. argv[0] is the contract command
// name `corvint`, not the `corvint` build name (decision 0122).
func prepareStatusAction(ctx context.Context, repository *gitauth.Repository, cem *wire.Map, options PrepareOptions) []any {
	action := []any{"corvint", "ocm", "status", "--map", options.MapPath, "--cem", options.CEMPath}
	if cem.Spec == wire.Spec02 {
		base, baseErr := repository.Resolve(ctx, options.ExpectedBase)
		target, targetErr := repository.Resolve(ctx, options.Target)
		if baseErr == nil && targetErr == nil {
			action = append(action, "--expected-base", base, "--target", target)
		}
	} else if options.ExpectedBase != "" {
		if base, err := repository.Resolve(ctx, options.ExpectedBase); err == nil {
			action = append(action, "--expected-base", base)
		}
	}
	if options.MaxUnknown != nil {
		action = append(action, "--max-unknown", strconv.Itoa(*options.MaxUnknown))
	}
	return action
}

var fullOIDPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

func fullObjectID(value string) bool { return fullOIDPattern.MatchString(value) }

// ocmJSONFailure preserves the oracle's small JSON taxonomy rather than
// collapsing it into the GPK-V0-037 refusal. That refusal is for families the
// shared parser genuinely cannot tell apart; these three it can.
func ocmJSONFailure(value wire.Value, parseErr error) error {
	if parseErr == nil {
		return &Error{Code: "map-type", Message: "OCM root must be an object"}
	}
	if strings.Contains(parseErr.Error(), "duplicate") {
		return &Error{Code: "duplicate-key", Message: "JSON contains a duplicate key"}
	}
	return &Error{Code: "invalid-json", Message: "OCM is not valid UTF-8 JSON"}
}

// LinkOptions configure one `ocm link` invocation.
type LinkOptions struct {
	MapPath      string
	CEMPath      string
	Output       string
	Obligation   string
	Hunks        []string
	TestPath     string
	Claims       []string
	ExpectedBase string
	Target       string
	// beforeVerification is a test seam for replacing the map between reads.
	beforeVerification func()
}

// LinkResult reports what one link recorded.
type LinkResult struct {
	MapPath      string
	ObligationID string
	HunkIDs      []string
	ClaimIDs     []string
}

// LinkOCM links one obligation to CEM hunks and pinned test claims. It verifies
// the existing map FIRST: a link records a reviewer's judgement, so it may only
// be added to a map that still holds together.
func LinkOCM(ctx context.Context, root string, options LinkOptions) (*LinkResult, error) {
	inputRoot, err := publish.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	if options.beforeVerification != nil {
		options.beforeVerification()
	}
	checked, err := ReadOCM(ctx, root, OCMReadOptions{
		OCMPath: options.MapPath, CEMPath: options.CEMPath,
		ExpectedBase: options.ExpectedBase, Target: options.Target,
		ExpectedBaseGiven: options.ExpectedBase != "", TargetGiven: options.Target != "",
	})
	if err != nil {
		return nil, err
	}
	if err := ocmLinkVerificationError(checked); err != nil {
		return nil, err
	}
	raw, cemRaw := checked.OCMRaw, checked.CEMRaw
	if options.Output != "" && sameInputFile(inputRoot.Path(), options.Output, options.CEMPath) {
		return nil, &Error{Code: "output-path-conflict", Message: "OCM and CEM map paths must differ"}
	}
	document, parseErr := wire.Parse(raw)
	if parseErr != nil || document.Kind != wire.KindObject {
		return nil, ocmJSONFailure(document, parseErr)
	}
	parsed, err := parseOCM(raw)
	if err != nil {
		return nil, err
	}
	obligations, err := requireArray(document.Obj, "obligations")
	if err != nil {
		return nil, err
	}
	obligationID, err := resolveObligationSelector(obligations, options.Obligation)
	if err != nil {
		return nil, err
	}
	cem, err := wire.ParseMap(cemRaw)
	if err != nil {
		return nil, err
	}
	hunkIDs, err := resolveHunkSelectors(cem, options.Hunks)
	if err != nil {
		return nil, err
	}
	repository, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		return nil, err
	}
	selected, err := selectClaims(ctx, repository, parsed.target, options)
	if err != nil {
		return nil, err
	}
	updated, err := linkObligation(document, obligationID, hunkIDs, selected)
	if err != nil {
		return nil, err
	}
	payload := canonicalOCMBytes(updated)
	if len(payload) > maxOCMBytes {
		return nil, &Error{Code: "map-too-large", Message: "OCM map exceeds the size limit"}
	}
	candidate, err := parseOCM(payload)
	if err != nil {
		return nil, err
	}
	intentContext, err := verifyOCMIntent(ctx, repository, cem, candidate)
	if err != nil {
		return nil, err
	}
	if _, err := verifyOCMClosure(cem, candidate, payload, intentContext, true); err != nil {
		return nil, err
	}
	destination := options.Output
	if destination == "" {
		destination = options.MapPath
	}
	written, err := publishOCMMap(ctx, inputRoot, destination, payload)
	if err != nil {
		return nil, err
	}
	claimIDs := make([]string, 0, len(selected))
	for _, claim := range selected {
		claimIDs = append(claimIDs, claim.id)
	}
	return &LinkResult{MapPath: written, ObligationID: obligationID, HunkIDs: hunkIDs, ClaimIDs: claimIDs}, nil
}

// A semantic reader failure has a nil error; mutators must refuse its verdict.
func ocmLinkVerificationError(checked *OCMReadResult) error {
	if checked == nil {
		return fail("invalid-map", "OCM verification failed")
	}
	if checked.Verification["valid"] == true {
		return nil
	}
	issues, _ := checked.Verification["issues"].([]any)
	if len(issues) == 0 {
		return fail("invalid-map", "OCM verification failed")
	}
	issue, _ := issues[0].(map[string]any)
	code, _ := issue["code"].(string)
	if code == "" {
		return fail("invalid-map", "OCM verification failed")
	}
	message, _ := issue["message"].(string)
	return &Error{Code: code, Message: message}
}

func selectClaims(ctx context.Context, repository *gitauth.Repository, target string, options LinkOptions) ([]ocmClaim, error) {
	entry, exists, err := repository.LookupTreeEntry(ctx, target, options.TestPath)
	if err != nil {
		return nil, err
	}
	if !exists || entry.Type != "blob" {
		return nil, fail("repository-object-unavailable", "repository object unavailable")
	}
	reader := &ocmBlobReader{ctx: ctx, repository: repository, cache: map[string][]byte{}}
	blob, err := reader.blob(target, options.TestPath, entry.OID)
	if err != nil {
		return nil, err
	}
	claims, err := enumerateClaims(options.TestPath, entry.OID, blob)
	if err != nil {
		return nil, err
	}
	return resolveClaimSelectors(claims, options.Claims)
}

// resolveHunkSelectors accepts CEM hunk ids or canonical one-based CEM ordinals.
// Every resolved hunk must be `supported`: linking an obligation to an unknown
// or mechanical hunk would claim evidence the CEM itself does not assert.
func resolveHunkSelectors(cem *wire.Map, selectors []string) ([]string, error) {
	if len(selectors) == 0 {
		return nil, &Error{Code: "missing-hunk", Message: "at least one hunk is required"}
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(selectors))
	for _, selector := range selectors {
		id, err := resolveOneHunk(cem, selector)
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, &Error{Code: "duplicate-hunk-reference", Message: "hunk selectors must be unique"}
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	for _, id := range result {
		matches := 0
		supported := false
		for _, hunk := range cem.Hunks {
			if hunk.ID == id {
				matches++
				supported = hunk.Disposition == "supported"
			}
		}
		if matches != 1 || !supported {
			return nil, &Error{Code: "unsupported-hunk-selector", Message: "linked CEM hunk must be supported"}
		}
	}
	return result, nil
}

func resolveOneHunk(cem *wire.Map, selector string) (string, error) {
	if decimalSelector.MatchString(selector) {
		ordinal, err := strconv.Atoi(selector)
		if err != nil || ordinal > len(cem.Hunks) {
			return "", &Error{Code: "invalid-arguments", Message: "hunk selector is outside the worklist"}
		}
		return cem.Hunks[ordinal-1].ID, nil
	}
	// A non-decimal selector is returned as given; whether it names a real,
	// supported hunk is decided by the caller's supported check, which is where
	// the oracle reports it.
	return selector, nil
}

// resolveClaimSelectors accepts claim ids, extractor selectors, or canonical
// one-based ordinals into the ENUMERATED worklist.
func resolveClaimSelectors(claims []ocmClaim, selectors []string) ([]ocmClaim, error) {
	if len(selectors) == 0 {
		return nil, &Error{Code: "missing-claim", Message: "at least one claim is required"}
	}
	if len(claims) > maxClaims {
		return nil, &Error{Code: "too-many-claims", Message: "claim worklist exceeds the limit"}
	}
	seen := map[string]struct{}{}
	resolved := make([]ocmClaim, 0, len(selectors))
	for _, selector := range selectors {
		matches := matchClaims(claims, selector)
		if len(matches) == 0 {
			return nil, &Error{Code: "claim-selector-out-of-range", Message: claimSelectorMissMessage(selector)}
		}
		if len(matches) != 1 {
			return nil, &Error{Code: "ambiguous-claim-selector", Message: "claim selector does not identify exactly one claim"}
		}
		if _, duplicate := seen[matches[0].id]; duplicate {
			return nil, &Error{Code: "duplicate-claim-reference", Message: "claim selectors must be unique"}
		}
		seen[matches[0].id] = struct{}{}
		resolved = append(resolved, matches[0])
	}
	return resolved, nil
}

// A miss hint describes normalization only; selection remains exact and the
// normalized fragment need not identify an existing or unique claim.
func claimSelectorMissMessage(selector string) string {
	message := "claim selector is outside the extracted worklist"
	_, suffix, found := strings.Cut(selector, "/case:")
	if !found {
		return message
	}
	normalized := selectorFragment(suffix)
	if normalized == suffix {
		return message
	}
	return message + "; normalized case fragment: " + normalized
}

func matchClaims(claims []ocmClaim, selector string) []ocmClaim {
	if decimalSelector.MatchString(selector) {
		if len(selector) > len(strconv.Itoa(maxClaims)) {
			return nil
		}
		ordinal, err := strconv.Atoi(selector)
		if err != nil || ordinal > len(claims) {
			return nil
		}
		return []ocmClaim{claims[ordinal-1]}
	}
	var matches []ocmClaim
	for _, claim := range claims {
		if claim.id == selector || claim.selector == selector {
			matches = append(matches, claim)
		}
	}
	return matches
}

// linkObligation records the link and merges the selected claims into the map's
// claim set. A claim id that already exists with different fields is a conflict,
// not an overwrite: the id is content-derived, so disagreement means one of the
// two was not derived from what it claims to describe.
func linkObligation(document wire.Value, obligationID string, hunkIDs []string, claims []ocmClaim) (wire.Value, error) {
	unique := map[string]struct{}{}
	for _, id := range hunkIDs {
		unique[id] = struct{}{}
	}
	if len(unique) == 0 || len(unique) != len(hunkIDs) || len(unique) > maxReferences {
		return wire.Value{}, &Error{Code: "invalid-hunk-references", Message: "hunk references must be non-empty and unique"}
	}
	claimIDs := make([]string, 0, len(claims))
	for _, claim := range claims {
		claimIDs = append(claimIDs, claim.id)
	}
	if len(claimIDs) == 0 || len(claimIDs) > maxReferences {
		return wire.Value{}, &Error{Code: "invalid-claim-references", Message: "claim references must be non-empty and unique"}
	}
	merged, err := mergeClaims(document, claims)
	if err != nil {
		return wire.Value{}, err
	}
	objectMember(document.Obj, "claims", merged)
	obligations, err := requireArray(document.Obj, "obligations")
	if err != nil {
		return wire.Value{}, err
	}
	matched := 0
	for _, item := range obligations {
		if item.Kind != wire.KindObject {
			continue
		}
		if id, ok := item.Obj.Get("id"); !ok || id.Kind != wire.KindString || id.Str != obligationID {
			continue
		}
		matched++
		objectMember(item.Obj, "disposition", stringValue("linked"))
		objectMember(item.Obj, "reason", stringValue("change-and-test-linked"))
		objectMember(item.Obj, "hunkIds", stringArray(sortedCopy(hunkIDs)))
		objectMember(item.Obj, "claimIds", stringArray(sortedCopy(claimIDs)))
	}
	if matched != 1 {
		return wire.Value{}, &Error{Code: "unknown-obligation-id", Message: "obligation ID must select exactly one row"}
	}
	return document, nil
}

func mergeClaims(document wire.Value, incoming []ocmClaim) (wire.Value, error) {
	existing, ok := document.Obj.Get("claims")
	if !ok || existing.Kind != wire.KindArray {
		return wire.Value{}, &Error{Code: "invalid-claims", Message: "claims must be an array"}
	}
	byID := map[string]wire.Value{}
	order := []string{}
	for _, claim := range existing.Arr {
		if claim.Kind != wire.KindObject {
			continue
		}
		id, present := claim.Obj.Get("id")
		if !present || id.Kind != wire.KindString {
			continue
		}
		byID[id.Str] = claim
		order = append(order, id.Str)
	}
	for _, claim := range incoming {
		encoded := claimValue(claim)
		if previous, present := byID[claim.id]; present {
			if !bytesEqual(canonicalValue(previous), canonicalValue(encoded)) {
				return wire.Value{}, &Error{Code: "claim-conflict", Message: "claim ID has conflicting fields"}
			}
			continue
		}
		byID[claim.id] = encoded
		order = append(order, claim.id)
	}
	if len(byID) > maxClaims {
		return wire.Value{}, &Error{Code: "too-many-claims", Message: "map exceeds the claim limit"}
	}
	sort.Strings(order)
	values := make([]wire.Value, 0, len(order))
	for index, id := range order {
		if index > 0 && order[index-1] == id {
			continue
		}
		values = append(values, byID[id])
	}
	return wire.Value{Kind: wire.KindArray, Arr: values}, nil
}

func claimValue(claim ocmClaim) wire.Value {
	return objectValue(
		[2]any{"id", stringValue(claim.id)},
		[2]any{"extractor", stringValue(claim.extractor)},
		[2]any{"path", stringValue(claim.path)},
		[2]any{"blobOid", stringValue(claim.blobOID)},
		[2]any{"selector", stringValue(claim.selector)},
		[2]any{"span", objectValue(
			[2]any{"start", intValue(claim.span.start)},
			[2]any{"end", intValue(claim.span.end)},
		)},
		[2]any{"spanSha256", stringValue(claim.spanSHA256)},
	)
}

func stringArray(values []string) wire.Value {
	items := make([]wire.Value, 0, len(values))
	for _, value := range values {
		items = append(items, stringValue(value))
	}
	return wire.Value{Kind: wire.KindArray, Arr: items}
}

func sortedCopy(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}
