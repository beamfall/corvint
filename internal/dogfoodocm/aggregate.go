// Package dogfoodocm verifies the private ordered set of unchanged OCM maps
// used by Corvint's repository dogfood loop.
package dogfoodocm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/publish"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/lrfrepo"
)

const (
	manifestPath    = ".corvint/change.ocm-intents"
	mapPathFormat   = ".corvint/change.ocm.%03d.json"
	maxManifestSize = 16 * 513
	maxScopes       = 16
	maxMapBytes     = 1 << 20
	// maxMapObligations and maxMapClaims are the ocm/0.1-experimental per-map
	// bounds, which each scope's own verification enforces at its map.
	maxMapObligations = lrfrepo.MaxOCMObligations
	maxMapClaims      = lrfrepo.MaxOCMClaims
	// The aggregate bounds are derived from the scope count: each scope is a
	// separate map already bounded per map, so a sum capped at one map's bound
	// refused ordinary multi-spec changes (V1-2971). Claims are summed per map
	// and may recur across maps, so they reuse the same derivation. The cost at
	// these bounds is measured in
	// https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-28-ocm-aggregate-obligation-bound.md.
	maxObligations = maxScopes * maxMapObligations
	maxClaims      = maxScopes * maxMapClaims
	scopeSetDomain = "corvint-dogfood-ocm-scope-set/0"
)

// MaxObligations is the aggregate bound on obligations across every scope
// (OCM-V0-021), 16 scopes of one 256-obligation map each. It is fixed: a caller may only report it earlier
// (OCM-V0-028), never widen it (OCM-V0-023).
const MaxObligations = maxObligations

// Options binds every scope to the same independently supplied CEM authority.
type Options struct {
	Root         string
	CEMPath      string
	ExpectedBase string
	Target       string
}

type Coverage struct {
	Linked  int `json:"linked"`
	Total   int `json:"total"`
	Unknown int `json:"unknown"`
}

type ScopeVerdict struct {
	Coverage  Coverage `json:"coverage"`
	Map       string   `json:"map"`
	MapSHA256 string   `json:"mapSha256"`
	Path      string   `json:"path"`
	State     string   `json:"state"`
}

type WorkItem struct {
	ClaimIDs    []string `json:"claimIds"`
	Disposition string   `json:"disposition"`
	HunkIDs     []string `json:"hunkIds"`
	ID          string   `json:"id"`
	Reason      string   `json:"reason"`
	Scope       int      `json:"scope"`
	Selector    int      `json:"selector"`
}

// Finding is a reviewer-visible gap that leaves the verdict unchanged.
type Finding struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type AggregateVerdict struct {
	Coverage Coverage  `json:"coverage"`
	Findings []Finding `json:"findings,omitempty"`
	State    string    `json:"state"`
}

// Result is private coordination state, not an OCM map or bundle.
type Result struct {
	Aggregate      AggregateVerdict `json:"aggregate"`
	Mutates        bool             `json:"mutates"`
	OK             bool             `json:"ok"`
	ScopeSetSHA256 string           `json:"scopeSetSha256"`
	Scopes         []ScopeVerdict   `json:"scopes"`
	TestExecution  map[string]any   `json:"testExecution"`
	Tool           string           `json:"tool"`
	Worklist       []WorkItem       `json:"worklist"`
}

type mapDocument struct {
	CEM struct {
		MapSHA256   string `json:"mapSha256"`
		PatchSHA256 string `json:"patchSha256"`
	} `json:"cem"`
	Claims      []json.RawMessage `json:"claims"`
	IntentScope struct {
		Path string `json:"path"`
	} `json:"intentScope"`
	Obligations []struct {
		ClaimIDs    []string `json:"claimIds"`
		Disposition string   `json:"disposition"`
		HunkIDs     []string `json:"hunkIds"`
		ID          string   `json:"id"`
		Reason      string   `json:"reason"`
	} `json:"obligations"`
	TargetRevision string `json:"targetRevision"`
}

type verifiedScope struct {
	declaredPath string
	mapPath      string
	before       []byte
	after        []byte
	document     mapDocument
	coverage     Coverage
}

type scopeSetEntry struct {
	MapSHA256 string `json:"mapSha256"`
	Path      string `json:"path"`
}

// Status freshly verifies every final map and returns an aggregate only after
// the manifest, bindings, map bytes, limits, and requirement identities close.
func Status(ctx context.Context, options Options) (Result, error) {
	root, err := publish.OpenRoot(options.Root)
	if err != nil {
		return Result{}, err
	}
	manifestBefore, err := root.ReadBounded(manifestPath, maxManifestSize, "missing-intent-scope")
	if err != nil {
		return Result{}, fail("missing-intent-scope", "intent scope manifest is unavailable")
	}
	paths, err := parseManifest(manifestBefore)
	if err != nil {
		return Result{}, err
	}
	scopes := make([]verifiedScope, 0, len(paths))
	// Every scope binds the same CEM, so one accepted stable verdict serves
	// them all; each reuse still re-reads the artifacts and re-derives the
	// patch (OCM-V0-027).
	reuse := lrfrepo.NewStableReuse()
	for index, path := range paths {
		mapPath := fmt.Sprintf(mapPathFormat, index+1)
		scope, err := verifyScope(ctx, root, options, reuse, path, mapPath)
		if err != nil {
			return Result{}, err
		}
		scopes = append(scopes, scope)
	}
	manifestAfter, err := root.ReadBounded(manifestPath, maxManifestSize, "intent-scope-drift")
	if err != nil {
		return Result{}, fail("intent-scope-drift", "intent scope manifest "+manifestPath+" drifted")
	}
	return aggregateVerified(manifestBefore, manifestAfter, scopes)
}

func verifyScope(ctx context.Context, root *publish.Root, options Options, reuse *lrfrepo.StableReuse, path, mapPath string) (verifiedScope, error) {
	before, err := root.ReadBounded(mapPath, maxMapBytes, "missing-intent-scope")
	if err != nil {
		return verifiedScope{}, fail("missing-intent-scope", "one required OCM map is unavailable")
	}
	checked, err := lrfrepo.ReadOCM(ctx, options.Root, lrfrepo.OCMReadOptions{
		OCMPath: mapPath, CEMPath: options.CEMPath,
		ExpectedBase: options.ExpectedBase, Target: options.Target,
		ExpectedBaseGiven: true, TargetGiven: true, StableReuse: reuse,
	})
	if err != nil {
		return verifiedScope{}, err
	}
	if checked.State != "ready-for-review" {
		return verifiedScope{}, verificationError(mapPath, checked)
	}
	// after is bound to checked.OCMRaw, the exact bytes ReadOCM verified,
	// rather than a third independent read of mapPath. A third read could
	// observe content that was swapped back after ReadOCM verified a
	// different version and before that read (an A-B-A race), letting a
	// drifted map pass the before/after equality check below while the
	// digest and document it renders never matched what was verified.
	after := checked.OCMRaw
	var document mapDocument
	if err := json.Unmarshal(after, &document); err != nil {
		return verifiedScope{}, fail("invalid-json", "verified OCM map could not be decoded")
	}
	return verifiedScope{
		declaredPath: path, mapPath: mapPath, before: before, after: after,
		document: document, coverage: coverageOf(checked.Counts),
	}, nil
}

func parseManifest(raw []byte) ([]string, error) {
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return nil, fail("missing-intent-scope", "intent scope manifest must be non-empty and LF-terminated")
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(lines) == 0 || len(lines) > maxScopes {
		return nil, fail("missing-intent-scope", "intent scope manifest has an invalid scope count")
	}
	paths := make([]string, 0, len(lines))
	previous := ""
	for _, line := range lines {
		path := string(line)
		if path == "" || path[0] == '#' || wire.ValidatePath(path) != nil {
			return nil, fail("missing-intent-scope", "intent scope manifest contains an invalid path")
		}
		if previous != "" && previous >= path {
			return nil, fail("missing-intent-scope", "intent scope manifest is not strictly bytewise ordered")
		}
		paths = append(paths, path)
		previous = path
	}
	return paths, nil
}

func aggregateVerified(manifestBefore, manifestAfter []byte, scopes []verifiedScope) (Result, error) {
	if len(scopes) == 0 {
		return Result{}, fail("missing-intent-scope", "no OCM scopes were supplied")
	}
	if !bytes.Equal(manifestBefore, manifestAfter) {
		return Result{}, fail("intent-scope-drift", "intent scope manifest "+manifestPath+" drifted")
	}
	paths, err := parseManifest(manifestAfter)
	if err != nil || len(paths) != len(scopes) {
		return Result{}, fail("intent-scope-drift", "intent scope set drifted")
	}
	if err := validateScopeBindings(paths, scopes); err != nil {
		return Result{}, err
	}
	return renderAggregate(scopes)
}

func validateScopeBindings(paths []string, scopes []verifiedScope) error {
	totalBytes := 0
	totalClaims := 0
	totalObligations := 0
	scopeBytes := make([]int, len(scopes))
	scopeClaims := make([]int, len(scopes))
	scopeObligations := make([]int, len(scopes))
	firstTarget := scopes[0].document.TargetRevision
	firstCEMMap := scopes[0].document.CEM.MapSHA256
	firstCEMPatch := scopes[0].document.CEM.PatchSHA256
	for index, scope := range scopes {
		if paths[index] != scope.declaredPath || scope.declaredPath != scope.document.IntentScope.Path {
			return fail("intent-scope-drift", "an OCM intent binding drifted")
		}
		if !bytes.Equal(scope.before, scope.after) {
			return fail("intent-scope-drift", "OCM map "+scope.mapPath+" drifted")
		}
		if scope.document.TargetRevision != firstTarget || scope.document.CEM.MapSHA256 != firstCEMMap || scope.document.CEM.PatchSHA256 != firstCEMPatch {
			return fail("intent-scope-drift", "OCM revision or CEM bindings differ across scopes")
		}
		scopeBytes[index] = len(scope.after)
		scopeClaims[index] = len(scope.document.Claims)
		scopeObligations[index] = len(scope.document.Obligations)
		totalBytes += scopeBytes[index]
		totalClaims += scopeClaims[index]
		totalObligations += scopeObligations[index]
	}
	if err := rejectDuplicateRequirements(scopes); err != nil {
		return err
	}
	if totalBytes > maxMapBytes {
		return fail("map-too-large", boundMessage("aggregate OCM map bytes exceed the limit", totalBytes, maxMapBytes, scopeBytes))
	}
	if totalClaims > maxClaims {
		return fail("too-many-claims", boundMessage("aggregate OCM claims exceed the limit", totalClaims, maxClaims, scopeClaims))
	}
	if totalObligations > maxObligations {
		return fail("too-many-obligations", boundMessage("aggregate OCM obligations exceed the limit", totalObligations, maxObligations, scopeObligations))
	}
	return nil
}

// boundMessage names the observed total, the fixed bound and each scope's
// share by its 1-based manifest line (OCM-V0-021). At most 2+2*maxScopes
// integers follow the fixed text, so the message stays bounded and carries no path,
// requirement body or map content; the manifest line names the intent.
func boundMessage(text string, total, bound int, perScope []int) string {
	var output strings.Builder
	fmt.Fprintf(&output, "%s: observed %d, bound %d; per scope", text, total, bound)
	for index, count := range perScope {
		separator := ","
		if index == 0 {
			separator = ""
		}
		fmt.Fprintf(&output, "%s %d=%d", separator, index+1, count)
	}
	// OCM-V0-024 (V1-1068): name the supported narrowing route; it carries no path.
	output.WriteString(boundNarrowing)
	return output.String()
}

// boundNarrowing is the fixed route a bound refusal names: split the scopes
// across staged changes rather than raising the bound or dropping a scope.
const boundNarrowing = "; narrow: list fewer intent scopes in the manifest and bind the rest as a separate staged change"

func rejectDuplicateRequirements(scopes []verifiedScope) error {
	seen := make(map[string]bool)
	for _, scope := range scopes {
		for _, obligation := range scope.document.Obligations {
			if seen[obligation.ID] {
				return fail("duplicate-requirement", "requirement ID repeats across intent scopes")
			}
			seen[obligation.ID] = true
		}
	}
	return nil
}

func renderAggregate(scopes []verifiedScope) (Result, error) {
	coverage := Coverage{}
	verdicts := make([]ScopeVerdict, 0, len(scopes))
	worklist := make([]WorkItem, 0)
	entries := make([]scopeSetEntry, 0, len(scopes))
	for scopeIndex, scope := range scopes {
		digest := sha256Hex(scope.after)
		coverage.Linked += scope.coverage.Linked
		coverage.Total += scope.coverage.Total
		coverage.Unknown += scope.coverage.Unknown
		verdicts = append(verdicts, ScopeVerdict{
			Coverage: scope.coverage, Map: scope.mapPath, MapSHA256: digest,
			Path: scope.declaredPath, State: "ready-for-review",
		})
		entries = append(entries, scopeSetEntry{MapSHA256: digest, Path: scope.declaredPath})
		for obligationIndex, obligation := range scope.document.Obligations {
			worklist = append(worklist, WorkItem{
				ClaimIDs: obligation.ClaimIDs, Disposition: obligation.Disposition,
				HunkIDs: obligation.HunkIDs, ID: obligation.ID, Reason: obligation.Reason,
				Scope: scopeIndex + 1, Selector: obligationIndex + 1,
			})
		}
	}
	canonical := canonicalScopeSet(entries)
	preimage := append(append([]byte(scopeSetDomain), 0), canonical...)
	return Result{
		Aggregate: AggregateVerdict{Coverage: coverage, Findings: linkageFindings(coverage), State: "ready-for-review"},
		Mutates:   false, OK: true, ScopeSetSHA256: sha256Hex(preimage), Scopes: verdicts,
		TestExecution: map[string]any{"state": "NOT_RUN"}, Tool: "dogfood-ocm-status",
		Worklist: worklist,
	}, nil
}

// linkageFindings names an aggregate that links none of its declared
// requirements (OCM-V0-016). Total is never zero: OCM-V0-001 refuses an empty
// scope.
func linkageFindings(coverage Coverage) []Finding {
	if coverage.Linked > 0 {
		return nil
	}
	message := fmt.Sprintf("0 of %d declared requirements are linked to the change", coverage.Total)
	return []Finding{{Code: "no-requirements-linked", Message: message}}
}

func canonicalScopeSet(entries []scopeSetEntry) []byte {
	var output strings.Builder
	output.WriteByte('[')
	for index, entry := range entries {
		if index > 0 {
			output.WriteByte(',')
		}
		output.WriteString(`{"mapSha256":`)
		output.WriteString(wire.CanonicalString(entry.MapSHA256))
		output.WriteString(`,"path":`)
		output.WriteString(wire.CanonicalString(entry.Path))
		output.WriteByte('}')
	}
	output.WriteByte(']')
	return []byte(output.String())
}

func coverageOf(counts map[string]any) Coverage {
	return Coverage{Linked: intValue(counts["linked"]), Total: intValue(counts["total"]), Unknown: intValue(counts["unknown"])}
}

func intValue(value any) int {
	result, _ := value.(int)
	return result
}

func verificationError(mapPath string, checked *lrfrepo.OCMReadResult) error {
	message := mapPath + ": one OCM map failed verification"
	if checked.VerificationCause != nil {
		message = mapPath + ": " + checked.VerificationCause.Error()
	}
	issues, _ := checked.Verification["issues"].([]any)
	if len(issues) > 0 {
		issue, _ := issues[0].(map[string]any)
		code, _ := issue["code"].(string)
		if code != "" {
			return fail(code, message)
		}
	}
	return fail("invalid-map", message)
}

func fail(code, message string) error {
	return &lrfrepo.Error{Code: code, Message: message}
}

func sha256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
