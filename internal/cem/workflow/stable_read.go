package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
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
	if options.Limits.MaxUnknown != nil || options.Limits.MaxMechanical != nil {
		return nil, stableRefusal{invalidArguments("%s %s does not accept policy ceilings", wire.StableSpec, action)}
	}
	if report && (options.OCMGiven || options.OCMPath != "") {
		return nil, stableRefusal{invalidArguments("%s %s does not accept --ocm", wire.StableSpec, action)}
	}
	repository := s.root
	if s.stableRepository != nil {
		repository = *s.stableRepository
	}
	stableOptions := verify.StableOptions{
		Repository: repository, ExpectedBase: options.ExpectedBase,
		Target: options.Target, ArtifactRoot: options.ArtifactRoot,
	}
	var result verify.StableResult
	var exit int
	if filepath.IsAbs(repository) {
		result, exit = verify.Stable(ctx, raw, stableOptions)
	} else {
		// `cem verify-stable` refuses a relative or empty --repository before it
		// reads the map. Named first, a value beginning with "--" also stops its
		// argument parse, so that refusal keeps no other supplied value.
		if strings.HasPrefix(repository, "--") {
			stableOptions = verify.StableOptions{Repository: repository}
		}
		result = verify.NewStableResult(stableOptions)
		exit = result.Refuse("arguments", "invalid-arguments", false)
	}
	envelope := map[string]any{
		"ok": exit == 0, "mutates": false, "tool": "cem-" + action,
		"stableVerification": StableVerification{Result: result, Exit: exit},
	}
	switch action {
	case "status":
		envelope["map"] = filepath.Join(s.workRoot.Path(), filepath.FromSlash(options.MapPath))
		envelope["state"] = stableStates[result.Outcome]
		// --citation names the map's citation class beside, never inside, the
		// verifier result, so the result stays what verify-stable reports and
		// never echoes the role (CEM-V1-095, CEM-V1-106).
		if options.Citation {
			if document, _ := wire.ParseStable(raw); document != nil {
				envelope["citationClass"] = StableCitationClass(document.EffectiveRole())
			}
		}
	case "report", ActionReportPreview:
		return s.stableReport(action, result, envelope, options)
	}
	return envelope, nil
}

// stableReport completes a stable report envelope. JSON adds the review
// projection; a preview adds the Markdown; a published report writes the same
// Markdown for every verifier outcome, locating the Git directories only after
// the verifier has run and without the 0.x object admission (CEM-V1-034,
// CEM-V1-035).
func (s *Session) stableReport(action string, result verify.StableResult, envelope map[string]any, options ReadOptions) (map[string]any, error) {
	envelope["tool"] = "cem-report"
	projection, err := stableReviewProjection(result)
	if err != nil {
		return nil, err
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
	if err := publish.Publish(publish.Output{Root: outputRoot, Relative: outputRelative, Data: []byte(markdown)}); err != nil {
		return nil, err
	}
	envelope["mutates"] = true
	envelope["report"] = filepath.Join(outputRoot.Path(), filepath.FromSlash(outputRelative))
	return envelope, nil
}

// Stable citation classes group `cem status --citation` maps by role: a
// bind-chore or seal-chore map records evidence of the bind loop itself, so
// its citation shape is never read as a change's (CEM-V1-106, DCW-V0-049).
const (
	StableCitationClassChange = "change"
	StableCitationClassChore  = "chore"
)

// StableCitationClass returns the status citationClass member for an
// effective role: class "change" for the change role and "chore" for any
// other, beside the role itself.
func StableCitationClass(role string) map[string]any {
	class := StableCitationClassChange
	if role != wire.StableRoleChange {
		class = StableCitationClassChore
	}
	return map[string]any{"class": class, "role": role}
}
