package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/mdreport"
	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/publish"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// ReadOptions configure status, verify, and report.
type ReadOptions struct {
	MapPath      string
	PatchPath    string // 0.1 only: explicit out-of-band patch
	PatchGiven   bool   // whether --patch appeared at all
	ExpectedBase string
	Target       string
	Limits       PolicyLimits
	Output       string // report only
}

// Read runs one of the three verification-consuming commands. The action is
// "status", "verify", or "report"; rendering is not a second trust path.
func (s *Session) Read(ctx context.Context, action string, options ReadOptions) (map[string]any, error) {
	raw, document, err := s.readMapInput(options.MapPath)
	if err != nil {
		return nil, err
	}
	// Stage 3: profile-forbidden arguments.
	if document.Spec == wire.Spec02 && options.PatchGiven {
		return nil, invalidArguments("cem/0.2 %s does not accept --patch", action)
	}
	// Stage 4: profile-required independent inputs.
	if document.Spec == wire.Spec02 {
		if options.ExpectedBase == "" {
			return nil, cemcode.New(cemcode.ExpectedBaseRequired, "cem/0.2 %s requires --expected-base", action)
		}
		if options.Target == "" {
			return nil, cemcode.New(cemcode.TargetRequired, "cem/0.2 %s requires --target", action)
		}
	}
	// Stages 5–6: repository validation, after every stage-2/3/4 judgment.
	if err := s.openRepository(); err != nil {
		return nil, err
	}
	verification, counts, work, envelope, err := s.runVerification(ctx, document, raw, options)
	if err != nil {
		return nil, err
	}
	policy := policyIssues(options.Limits, counts)
	valid := verification["valid"] == true
	switch action {
	case "verify":
		result := map[string]any{
			"ok": valid && len(policy) == 0, "mutates": false, "tool": "cem-verify",
			"verification": verification,
		}
		// The oracle reports counts and policy issues from verify only when a policy
		// ceiling was actually supplied; status reports them unconditionally.
		if options.Limits.MaxUnknown != nil || options.Limits.MaxMechanical != nil {
			result["counts"] = counts
			result["policyIssues"] = policy
		}
		envelope.apply(result, false)
		return result, nil
	case "status":
		state := "ready-for-ci"
		if !valid {
			state = "invalid"
		} else if len(policy) != 0 {
			state = "incomplete"
		}
		result := map[string]any{
			"ok": state == "ready-for-ci", "mutates": false, "tool": "cem-status",
			"map":   filepath.Join(s.workRoot.Path(), filepath.FromSlash(options.MapPath)),
			"state": state, "counts": counts, "worklist": work, "policyIssues": policy,
			"verification": verification,
		}
		envelope.apply(result, true)
		return result, nil
	case "report":
		return s.renderReport(document, verification, counts, work, policy, envelope, options)
	default:
		return nil, invalidArguments("unknown CEM command")
	}
}

// patchEnvelope carries the frozen WP2 top-level patch-binding fields.
type patchEnvelope struct {
	legacyPatch any // status-only legacy field
	patchSource string
	excluded    any
	warnings    []any
}

// apply writes the frozen envelope table fields. The legacy patch field
// appears only on status.
func (e patchEnvelope) apply(result map[string]any, isStatus bool) {
	if isStatus {
		result["patch"] = e.legacyPatch
	}
	result["patchSource"] = e.patchSource
	result["excludedPath"] = e.excluded
	result["warnings"] = e.warnings
}

func (s *Session) runVerification(ctx context.Context, document *wire.Map, raw []byte, options ReadOptions) (map[string]any, map[string]any, []any, patchEnvelope, error) {
	if document.Spec == wire.Spec02 {
		envelope := patchEnvelope{
			legacyPatch: nil, patchSource: "canonical-derived",
			excluded: wire.ExcludedCEMPath, warnings: []any{},
		}
		outcome, canonicallyBound, err := verify.Canonical(ctx, s.repository, document, verify.CanonicalOptions{
			ExpectedBase: options.ExpectedBase, Target: options.Target, RawMapBytes: raw,
		})
		if fatalVerificationError(err) {
			return nil, nil, nil, patchEnvelope{}, err
		}
		verification := verificationFor(document, outcome, err, canonicallyBound)
		counts, work := countsAndWorklist(document)
		return verification, counts, work, envelope, nil
	}
	patchPath, patchBytes, envelope, readErr := s.readLegacyPatch(options)
	if readErr != nil {
		return nil, nil, nil, patchEnvelope{}, readErr
	}
	envelope.legacyPatch = patchPath
	outcome, err := verify.Exact(ctx, s.repository, document, patchBytes, verify.ExactOptions{
		ExpectedBase: options.ExpectedBase, Target: options.Target,
	})
	if fatalVerificationError(err) {
		return nil, nil, nil, patchEnvelope{}, err
	}
	verification := verificationFor(document, outcome, err, false)
	counts, work := countsAndWorklist(document)
	return verification, counts, work, envelope, nil
}

// verificationFor renders success, drift-rejection (context retained), or
// failure verification objects.
func verificationFor(document *wire.Map, outcome *verify.Outcome, err error, canonicallyBound bool) map[string]any {
	switch {
	case err == nil:
		return successVerification(document, outcome, false)
	case cemcode.CodeOf(err) == cemcode.EvidenceDrift && outcome != nil:
		return successVerification(document, outcome, true)
	default:
		return failureVerification(document.Spec, err, canonicallyBound)
	}
}

// fatalVerificationError separates a failed verification from a failed READ.
// A revision the caller named that will not resolve is not a verdict about the
// map: the oracle aborts the command, and folding it into a verification object
// let an unresolvable --expected-base come back as an ordinary status.
func fatalVerificationError(err error) bool {
	switch cemcode.CodeOf(err) {
	case cemcode.GitReadFailed, cemcode.GitDiffFailed, cemcode.GitDiffTimeout,
		cemcode.RepositoryObjectUnavailable, cemcode.GitStartFailed, cemcode.GitExitFailure:
		return true
	}
	return false
}

// readLegacyPatch applies the frozen 0.1 patch-source rules: an explicit value
// is read out of band; omission reads the historical per-worktree default and
// a missing file keeps the bounded patch-read failure.
func (s *Session) readLegacyPatch(options ReadOptions) (string, []byte, patchEnvelope, error) {
	envelope := patchEnvelope{excluded: nil}
	if options.PatchGiven {
		if options.PatchPath == "" {
			return "", nil, envelope, invalidArguments("--patch requires a value")
		}
		envelope.patchSource = "explicit-out-of-band"
		envelope.warnings = []any{"patch-supplied-out-of-band"}
		data, err := s.readPatchInput(options.PatchPath)
		if err != nil {
			return "", nil, envelope, err
		}
		resolved := options.PatchPath
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(s.workRoot.Path(), filepath.FromSlash(options.PatchPath))
		}
		return resolved, data, envelope, nil
	}
	envelope.patchSource = "default-out-of-band"
	envelope.warnings = []any{"patch-defaulted-out-of-band"}
	data, err := s.gitRoot.ReadBounded(defaultPatchRelative, patch.MaxPatchBytes, cemcode.PatchUnavailable)
	if err != nil {
		return "", nil, envelope, err
	}
	return filepath.Join(s.gitRoot.Path(), filepath.FromSlash(defaultPatchRelative)), data, envelope, nil
}

const reportWarning = "Paths and content-derived identifiers or digests can disclose repository information. Keep this report local unless the repository owner approves sharing it."

func (s *Session) renderReport(document *wire.Map, verification, counts map[string]any, work, policy []any, envelope patchEnvelope, options ReadOptions) (map[string]any, error) {
	outputRoot, outputRelative := s.gitRoot, defaultReportRelative
	if options.Output != "" {
		if filepath.IsAbs(options.Output) {
			return nil, invalidArguments("--output must be a repository-relative path")
		}
		outputRoot, outputRelative = s.workRoot, options.Output
	}
	// Fold case: on a case-insensitive volume a case variant names the map.
	if outputRoot == s.workRoot && strings.EqualFold(outputRelative, options.MapPath) {
		return nil, invalidArguments("report and map paths must differ")
	}
	if outputRoot == s.workRoot {
		if err := checkMapSidecarAliasing(options.MapPath, outputRelative); err != nil {
			return nil, err
		}
	}
	// Folding misses the volume's own aliases (NFD/NFC, locale folds): compare
	// the existing output's file identity with the map's.
	if sameFile(filepath.Join(outputRoot.Path(), filepath.FromSlash(outputRelative)),
		filepath.Join(s.workRoot.Path(), filepath.FromSlash(options.MapPath))) {
		return nil, invalidArguments("report and map paths must differ")
	}
	report := renderReportText(document, verification, counts, work, policy)
	if len(report) > maxReportBytes {
		return nil, invalidArguments("report exceeds %d bytes", maxReportBytes)
	}
	if err := publish.Publish(publish.Output{Root: outputRoot, Relative: outputRelative, Data: []byte(report)}); err != nil {
		return nil, err
	}
	valid := verification["valid"] == true
	result := map[string]any{
		"ok": valid && len(policy) == 0, "mutates": true, "tool": "cem-report",
		"report": filepath.Join(outputRoot.Path(), filepath.FromSlash(outputRelative)),
		"counts": counts, "policyIssues": policy, "verification": verification,
	}
	envelope.apply(result, false)
	return result, nil
}

// sameFile reports whether both paths name one existing file.
func sameFile(first, second string) bool {
	firstInfo, firstErr := os.Lstat(first)
	secondInfo, secondErr := os.Lstat(second)
	return firstErr == nil && secondErr == nil && os.SameFile(firstInfo, secondInfo)
}

// renderReportText renders the deterministic human report.
func renderReportText(document *wire.Map, verification, counts map[string]any, work, policy []any) string {
	var out strings.Builder
	out.WriteString("# Change Evidence Map review\n\n")
	out.WriteString("> " + reportWarning + "\n\n")
	out.WriteString(fmt.Sprintf("- Spec: `%s`\n", document.Spec))
	out.WriteString(fmt.Sprintf("- Valid: `%v`\n", verification["valid"]))
	out.WriteString(fmt.Sprintf("- Base revision: `%s`\n", document.BaseRevision))
	if target, ok := verification["targetRevision"].(string); ok && target != "" {
		out.WriteString(fmt.Sprintf("- Target revision: `%s`\n", target))
	}
	out.WriteString(fmt.Sprintf("- Patch SHA-256: `%s`\n", document.PatchSha256))
	if assurance, ok := verification["assurance"].(string); ok {
		out.WriteString(fmt.Sprintf("- Assurance: `%s`\n", assurance))
	}
	out.WriteString(fmt.Sprintf("\n## Dispositions\n\n- total: %d\n- supported: %d\n- unknown: %d\n- mechanical: %d\n",
		counts["total"], counts["supported"], counts["unknown"], counts["mechanical"]))
	if issues, ok := verification["issues"].([]any); ok && len(issues) > 0 {
		out.WriteString("\n## Issues\n\n")
		for _, issue := range issues {
			row := issue.(map[string]any)
			out.WriteString(fmt.Sprintf("- `%v`: %v\n", row["code"], row["message"]))
		}
	}
	if len(policy) > 0 {
		out.WriteString("\n## Policy issues\n\n")
		for _, issue := range policy {
			row := issue.(map[string]any)
			out.WriteString(fmt.Sprintf("- `%v`: %v (actual %v, maximum %v)\n",
				row["code"], row["message"], row["actual"], row["maximum"]))
		}
	}
	if drift, ok := verification["drift"].([]any); ok && len(drift) > 0 {
		out.WriteString("\n## Evidence drift\n\n")
		for _, item := range drift {
			row := item.(map[string]any)
			out.WriteString(fmt.Sprintf("- %s at %s: `%v`\n",
				mdreport.CodeSpan(fmt.Sprint(row["evidenceId"])), mdreport.CodeSpan(fmt.Sprint(row["path"])), row["status"]))
		}
	}
	out.WriteString("\n## Worklist\n\n")
	for _, entry := range work {
		row := entry.(map[string]any)
		out.WriteString(fmt.Sprintf("%v. %s `%v` — %v (%v); next: %v\n",
			row["selector"], mdreport.CodeSpan(fmt.Sprint(row["path"])), row["id"], row["disposition"], row["reason"], row["next"]))
	}
	return out.String()
}
