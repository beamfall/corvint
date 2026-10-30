package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/publish"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// StableVerification carries one complete stable verifier result and its exit
// code through a Core read envelope. It encodes as the unmodified result, so a
// Core caller sees exactly what `cem verify-stable` reports (CEM-V1-017).
type StableVerification struct {
	Result verify.StableResult
	Exit   int
}

// MarshalJSON encodes only the verifier's result; the exit code is the
// command's process status, never a wire field.
func (v StableVerification) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.Result)
}

// stableStates names a stable outcome in the status envelope. It never reuses
// the 0.x ready-for-ci, invalid or incomplete states (CEM-V1-017).
var stableStates = map[string]string{"ACCEPT": "accepted", "REJECT": "rejected", "UNSUPPORTED": "unsupported"}

// SetStableRepository records the repository root byte for byte as the caller
// named it, before any entry-point tilde expansion, absolutization, cleaning or
// symlink resolution of the session root. Only a stable read uses it: it must
// judge the caller's path exactly as `cem verify-stable --repository` judges the
// same string (CEM-V1-017), so an empty path is kept and refused. A session
// whose entry point never sets it admits the session root.
func (s *Session) SetStableRepository(path string) { s.stableRepository = &path }

// StableOCMReader verifies supplied OCM bytes against an exact cem/1.0 map
// through the owning adapter's stable admission and returns the same review
// OCM projection a 0.x report carries (valid, state, verification, obligations,
// claims). A closed refusal is a projection with valid false; only an
// unexpected failure is an error (V1-1305).
type StableOCMReader func(ctx context.Context, repository string, cemRaw, ocmRaw []byte, expectedBase, target, artifactRoot string) (map[string]any, error)

// SetStableOCMReader installs the stable OCM verifier a report --ocm uses.
func (s *Session) SetStableOCMReader(reader StableOCMReader) { s.stableOCM = reader }

// stableRefusal marks a refusal Core issues for an exact cem/1.0 map before the
// stable verifier runs. It keeps the wrapped code and message, and lets a
// projection such as the MCP bridge name it apart from a malformed 0.x map
// (CEM-V1-036, MCPV0-037).
type stableRefusal struct{ error }

func (r stableRefusal) Unwrap() error { return r.error }

// IsStableRefusal reports whether a read refused an exact cem/1.0 map before
// its stable verifier ran.
func IsStableRefusal(err error) bool {
	var refusal stableRefusal
	return errors.As(err, &refusal)
}

// readStable runs a Core status, verify or report over an exact cem/1.0 map
// through the stable verifier. Core adds no 0.x verification, counts or
// worklist and opens no repository first: the verifier performs its own
// metadata admission before any Git process runs (CEM-V1-017, CEM-V1-018). A
// report embeds the complete result beside a review projection derived only
// from it (CEM-V1-034, CEM-V1-035).
func (s *Session) readStable(ctx context.Context, action string, raw []byte, options ReadOptions) (map[string]any, error) {
	report := action == "report" || action == ActionReportPreview
	if action != "status" && action != "verify" && !report {
		return nil, stableRefusal{cemcode.New(cemcode.UnsupportedSpec, "%s %s is not implemented; read it with cem status, cem verify or cem report and --artifacts", wire.StableSpec, action)}
	}
	if options.PatchGiven {
		return nil, stableRefusal{invalidArguments("%s %s does not accept --patch", wire.StableSpec, action)}
	}
	// Status and report admit policy ceilings beside the verifier result;
	// verify stays the bare verifier read and the MCP report preview keeps
	// its refusal (CEM-V1-145, CEM-V1-637).
	limited := options.Limits.MaxUnknown != nil || options.Limits.MaxMechanical != nil
	if limited && action != "status" && action != "report" {
		return nil, stableRefusal{invalidArguments("%s %s does not accept policy ceilings", wire.StableSpec, action)}
	}
	// A deferral bound is a status or report policy; verify and the MCP
	// report preview accept no bound (CEM-V1-645).
	if options.PathPolicy && action != "status" {
		return nil, stableRefusal{invalidArguments("%s %s does not accept --path-policy", wire.StableSpec, action)}
	}
	if options.MaxDeferredIntents != nil && action != "status" && action != "report" {
		return nil, stableRefusal{invalidArguments("%s %s does not accept --max-deferred-intents", wire.StableSpec, action)}
	}
	if report && options.OCMGiven && options.OCMPath == "" {
		return nil, stableRefusal{invalidArguments("%s %s does not accept --ocm with an empty path", wire.StableSpec, action)}
	}
	repository := s.root
	if s.stableRepository != nil {
		repository = *s.stableRepository
	}
	stableOptions := verify.StableOptions{
		Repository: repository, ExpectedBase: options.ExpectedBase,
		Target: options.Target, ArtifactRoot: options.ArtifactRoot,
		ScaledOperations: options.ScaledOperations, NonRegular: options.NonRegular,
	}
	var result verify.StableResult
	var exit int
	var revert *verify.HunkRevert
	var guards *verify.GuardingTests
	if options.HunkRevert && action != "verify" {
		return nil, stableRefusal{invalidArguments("%s %s does not accept --hunk-revert", wire.StableSpec, action)}
	}
	if options.GuardingTests && action != "verify" {
		return nil, stableRefusal{invalidArguments("%s %s does not accept --guarding-tests", wire.StableSpec, action)}
	}
	// The producer search and the consumer falsifier are separate steps: one
	// run never both suggests a test and credits it (TCQ-V0-062).
	if options.GuardingTests && options.HunkRevert {
		return nil, stableRefusal{invalidArguments("%s %s does not accept --guarding-tests with --hunk-revert", wire.StableSpec, action)}
	}
	switch {
	case filepath.IsAbs(repository) && options.HunkRevert:
		result, exit, revert = verify.StableWithHunkRevert(ctx, raw, stableOptions, verify.DefaultHunkRevertLimits)
	case filepath.IsAbs(repository) && options.GuardingTests:
		result, exit, guards = verify.StableWithGuardingTests(ctx, raw, stableOptions, verify.DefaultHunkRevertLimits)
	case filepath.IsAbs(repository):
		result, exit = verify.Stable(ctx, raw, stableOptions)
	default:
		// `cem verify-stable` refuses a relative or empty --repository before it
		// reads the map. Named first, a value beginning with "--" also stops its
		// argument parse, so that refusal keeps no other supplied value.
		if strings.HasPrefix(repository, "--") {
			stableOptions = verify.StableOptions{Repository: repository}
		}
		result = verify.NewStableResult(stableOptions)
		exit = result.Refuse("arguments", "invalid-arguments", true)
	}
	envelope := map[string]any{
		"ok": exit == 0, "mutates": false, "tool": "cem-" + action,
		"stableVerification": StableVerification{Result: result, Exit: exit},
	}
	// The falsifier observation is beside, never inside, the verifier result:
	// it depends on the host, so it never enters verdictSha256, and it is null
	// unless the result accepted (CEM-V1-088).
	if options.HunkRevert {
		envelope["hunkRevert"] = revert
	}
	// The guarding-test search is producer guidance beside the result: it
	// sets no support tier and is null unless the result accepted (TCQ-V0-062).
	if options.GuardingTests {
		envelope["guardingTests"] = guards
	}
	// A relative root or artifact root is refused, never resolved; the fix is
	// named beside, never inside, the closed result (CEM-V1-117).
	if result.Code != nil && *result.Code == "invalid-arguments" {
		if fix := verify.RelativePathFix([2]string{"--root", repository}, [2]string{"--artifacts", options.ArtifactRoot}); fix != "" {
			envelope["fix"] = fix
		}
	}
	// The non-regular rows are beside, never inside, the closed result, so
	// verdictSha256 is unchanged (CAX-V0-021).
	excluded := -1
	if options.NonRegular {
		envelope["nonRegularProfile"] = gitauth.NonRegularProfile
		envelope["nonRegular"] = NonRegularRows(result.NonRegular)
		excluded = len(result.NonRegular)
	}
	// The path-policy rows are beside, never inside, the closed result
	// (CAX-V0-060).
	inert := -1
	if options.PathPolicy {
		inert = s.stablePathPolicy(ctx, envelope, raw, result.Outcome == "ACCEPT", options.Target)
	}
	if limited {
		stablePolicy(envelope, raw, options.Limits, exit, excluded, inert)
	} else if action == "status" || action == "report" {
		stableCounts(envelope, raw, excluded, inert)
	}
	// A recorded intent-scope deferral is checked beside, never inside, the
	// verifier result; any issue clears ok while state stays the verifier
	// verdict (CEM-V1-645).
	var deferred map[string]any
	if action != "verify" {
		deferred = s.deferredIntents(ctx, repository, raw, result, options.MaxDeferredIntents)
		if deferred != nil && len(deferred["issues"].([]any)) != 0 {
			envelope["ok"] = false
		}
	}
	switch action {
	case "status":
		envelope["map"] = options.MapPath
		if !filepath.IsAbs(options.MapPath) {
			envelope["map"] = filepath.Join(s.workRoot.Path(), filepath.FromSlash(options.MapPath))
		}
		envelope["state"] = stableStates[result.Outcome]
		if deferred != nil {
			envelope["deferredIntents"] = deferred
		}
		// --citation names the map's citation class beside, never inside, the
		// verifier result, so the result stays what verify-stable reports and
		// never echoes the role (CEM-V1-095, CEM-V1-106).
		// A governing instruction file the change rewrites is reported beside
		// the result and never changes the state (CEM-V1-270).
		if document, _ := wire.ParseStable(raw); document != nil {
			if rows := selfModifiedAuthority(stableMechanics(document)); len(rows) != 0 {
				envelope["selfModifiedAuthority"] = rows
			}
		}
		if options.Citation {
			if document, _ := wire.ParseStable(raw); document != nil {
				paths := make([]string, 0, len(document.Change.Hunks))
				for _, hunk := range document.Change.Hunks {
					paths = append(paths, hunk.Path)
				}
				envelope["citationClass"] = StableCitationClass(document.EffectiveRole(), paths)
			}
		}
	case "report", ActionReportPreview:
		return s.stableReport(ctx, action, repository, raw, result, envelope, options, deferred)
	}
	return envelope, nil
}

// stablePolicy applies status disposition ceilings to a stable map beside,
// never inside, the verifier result: counts come from the map's hunks and
// policyIssues name each exceeded ceiling. The envelope is ok only when the
// verifier accepts and no ceiling is exceeded; a map the stable parser refuses
// has no counts and was already refused by the verifier (CEM-V1-145).
// policyState names the ceiling verdict separately from state, which stays the
// structural verifier verdict (CEM-V1-017, CEM-V1-636); a report applies the
// same ceilings before its OCM combination (CEM-V1-637).
func stablePolicy(envelope map[string]any, raw []byte, limits PolicyLimits, exit, nonRegular, declaredInert int) {
	document, err := wire.ParseStable(raw)
	if err != nil {
		return
	}
	change := wire.Map(document.Change)
	counts, _ := countsAndWorklist(&change)
	addNonRegularCounts(counts, nonRegular)
	addDeclaredInertCounts(counts, declaredInert)
	issues := policyIssues(limits, counts)
	envelope["counts"] = counts
	envelope["policyIssues"] = issues
	envelope["policyState"] = "within-ceilings"
	if len(issues) != 0 {
		envelope["policyState"] = "ceiling-exceeded"
	}
	envelope["ok"] = exit == 0 && len(issues) == 0
}

// stableCounts adds the per-disposition hunk counts to a status or report read
// without ceilings, beside the unmodified verifier result. It adds no
// policyIssues and no policyState, leaves ok and the exit unchanged, and adds
// nothing for a map the stable parser refuses (CCP-V0-010, amending
// CEM-V1-017).
func stableCounts(envelope map[string]any, raw []byte, nonRegular, declaredInert int) {
	document, err := wire.ParseStable(raw)
	if err != nil {
		return
	}
	change := wire.Map(document.Change)
	counts, _ := countsAndWorklist(&change)
	addNonRegularCounts(counts, nonRegular)
	addDeclaredInertCounts(counts, declaredInert)
	envelope["counts"] = counts
}

// addNonRegularCounts counts each cem-non-regular/1 path as one unknown, so
// --max-unknown bounds it like any unknown hunk, and names the share in
// counts.nonRegular (CAX-V0-022). A negative value means the profile is off
// and leaves the cem/1.0 counts unchanged.
func addNonRegularCounts(counts map[string]any, nonRegular int) {
	if nonRegular < 0 {
		return
	}
	counts["total"] = counts["total"].(int) + nonRegular
	counts["unknown"] = counts["unknown"].(int) + nonRegular
	counts["nonRegular"] = nonRegular
}

// StablePolicyIssues returns the CEM-V1-145 counts and policy issues of a
// stable map's hunks under limits, for a consumer outside the Core read such
// as prove --attest-cem-v2 (CEM-V1-638). A map the stable parser refuses is an
// error, never an empty issue list.
func StablePolicyIssues(raw []byte, limits PolicyLimits) (map[string]any, []any, error) {
	document, err := wire.ParseStable(raw)
	if err != nil {
		return nil, nil, err
	}
	change := wire.Map(document.Change)
	counts, _ := countsAndWorklist(&change)
	return counts, policyIssues(limits, counts), nil
}

// stableReport completes a stable report envelope. JSON adds the review
// projection; a preview adds the Markdown; a published report writes the same
// Markdown for every verifier outcome, locating the Git directories only after
// the verifier has run and without the 0.x object admission (CEM-V1-034,
// CEM-V1-035).
func (s *Session) stableReport(ctx context.Context, action, repository string, raw []byte, result verify.StableResult, envelope map[string]any, options ReadOptions, deferred map[string]any) (map[string]any, error) {
	envelope["tool"] = "cem-report"
	projection, err := stableReviewProjection(result)
	if err != nil {
		return nil, err
	}
	if options.OCMPath != "" {
		ocm, err := s.stableReportOCM(ctx, repository, raw, options)
		if err != nil {
			return nil, err
		}
		applyStableOCM(projection, ocm, s.stableOCM != nil)
		envelope["ok"] = envelope["ok"] == true && projection["ocmValid"] == true
	}
	// The deferral member sits beside the record set: it joins the projection
	// after every recordSetSha256 computation, so that digest never covers it
	// (CEM-V1-646).
	if deferred != nil {
		projection["deferredIntents"] = deferred
	}
	if action == "report" && options.Format == "json" {
		envelope["review"] = projection
		return envelope, nil
	}
	markdown := renderStableReport(result, projection)
	if len(markdown) > maxReportBytes {
		return nil, invalidArguments("report exceeds %d bytes", maxReportBytes)
	}
	envelope["recordSetSha256"] = projection["recordSetSha256"]
	if action == ActionReportPreview {
		envelope["markdown"] = markdown
		return envelope, nil
	}
	if err := s.openPublicationDirectories(); err != nil {
		return nil, err
	}
	outputRoot, outputRelative, err := s.reportOutput(options)
	if err != nil {
		return nil, err
	}
	publishReport := publish.Publish
	if filepath.IsAbs(options.Output) {
		publishReport = publish.PublishMember
	}
	if err := publishReport(publish.Output{Root: outputRoot, Relative: outputRelative, Data: []byte(markdown)}); err != nil {
		return nil, err
	}
	envelope["mutates"] = true
	envelope["report"] = filepath.Join(outputRoot.Path(), filepath.FromSlash(outputRelative))
	return envelope, nil
}

// Stable citation classes group `cem status --citation` maps: a bind-chore or
// seal-chore map records evidence of the bind loop itself, and a record map
// changes only evidence records, so neither citation shape is read as a
// change's (CEM-V1-106, CEM-V1-170, DCW-V0-049, DCW-V0-057).
const (
	StableCitationClassChange = "change"
	StableCitationClassChore  = "chore"
	StableCitationClassRecord = "record"
)

// StableCitationClass returns the status citationClass member for an
// effective role and the map's hunk paths: class "chore" for any role other
// than change; for the change role, class "record" when the hunks name at
// least one evidence-record path and every other hunk path is a CEM sidecar
// path, and "change" otherwise; beside the role itself. The record rule is
// internal/dogfoodflow.RecordChange applied to hunk paths; keep the two in
// step (CEM-V1-170, DCW-V0-057).
func StableCitationClass(role string, paths []string) map[string]any {
	class := StableCitationClassChange
	switch {
	case role != wire.StableRoleChange:
		class = StableCitationClassChore
	case stableRecordPaths(paths):
		class = StableCitationClassRecord
	}
	return map[string]any{"class": class, "role": role}
}

// stableRecordPaths reports hunk paths that name at least one path under
// docs/build-log/ or .agent-evidence/ and otherwise only
// .corvint/change.cem.json or .corvint/changes/<NAME>.cem.json (DCW-V0-057).
func stableRecordPaths(paths []string) bool {
	record := false
	for _, path := range paths {
		build, isBuild := strings.CutPrefix(path, "docs/build-log/")
		evidence, isEvidence := strings.CutPrefix(path, ".agent-evidence/")
		name, isChanges := strings.CutPrefix(path, ".corvint/changes/")
		stem, sealed := strings.CutSuffix(name, ".cem.json")
		switch {
		case isBuild && build != "", isEvidence && evidence != "":
			record = true
		case path == ".corvint/change.cem.json", isChanges && sealed && stem != "" && !strings.Contains(stem, "/"):
		default:
			return false
		}
	}
	return record
}

// maxStableOCMBytes bounds a supplied OCM exactly as the 0.x review reader does.
const maxStableOCMBytes = 1 << 20

// stableReportOCM reads a supplied OCM inside the session root and verifies it
// through the installed stable reader against the exact map bytes, the caller's
// binding and artifact root. Without a reader the obligations stay
// NOT_PRODUCED with reason ocm-verifier-unavailable (V1-1305).
func (s *Session) stableReportOCM(ctx context.Context, repository string, raw []byte, options ReadOptions) (map[string]any, error) {
	relative := options.OCMPath
	if filepath.IsAbs(relative) {
		inside, err := filepath.Rel(s.workRoot.Path(), filepath.Clean(relative))
		if err != nil || inside == "." || inside == ".." || filepath.IsAbs(inside) || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
			return nil, invalidArguments("--ocm must remain inside repository root")
		}
		relative = filepath.ToSlash(inside)
	}
	if filepath.Clean(filepath.FromSlash(relative)) != filepath.FromSlash(relative) {
		return nil, invalidArguments("--ocm must be normalized")
	}
	ocmRaw, err := s.workRoot.ReadBounded(relative, maxStableOCMBytes, cemcode.MapUnavailable)
	if err != nil {
		return nil, err
	}
	if s.stableOCM == nil {
		return map[string]any{"state": "NOT_PRODUCED", "reason": "ocm-verifier-unavailable", "obligations": []any{}, "claims": []any{}}, nil
	}
	ocm, err := s.stableOCM(ctx, repository, append([]byte(nil), raw...), ocmRaw, options.ExpectedBase, options.Target, options.ArtifactRoot)
	if err != nil {
		return nil, err
	}
	ocmSum, cemSum := sha256.Sum256(ocmRaw), sha256.Sum256(raw)
	ocm["mapSha256"], ocm["cemMapSha256"] = hex.EncodeToString(ocmSum[:]), hex.EncodeToString(cemSum[:])
	return ocm, nil
}
