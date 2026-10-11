package workflow

import (
	"context"
	"encoding/json"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
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

// readStable runs a Core status or verify over an exact cem/1.0 map through the
// stable verifier. Core adds no 0.x verification, counts or worklist and opens
// no repository first: the verifier performs its own metadata admission before
// any Git process runs (CEM-V1-017, CEM-V1-018).
func (s *Session) readStable(ctx context.Context, action string, raw []byte, options ReadOptions) (map[string]any, error) {
	if action != "status" && action != "verify" {
		return nil, cemcode.New(cemcode.UnsupportedSpec, "%s %s is not implemented; read it with cem status or cem verify and --artifacts", wire.StableSpec, action)
	}
	if options.PatchGiven {
		return nil, invalidArguments("%s %s does not accept --patch", wire.StableSpec, action)
	}
	if options.Limits.MaxUnknown != nil || options.Limits.MaxMechanical != nil {
		return nil, invalidArguments("%s %s does not accept policy ceilings", wire.StableSpec, action)
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
		// reads the map.
		result = verify.NewStableResult(stableOptions)
		exit = result.Refuse("arguments", "invalid-arguments", false)
	}
	envelope := map[string]any{
		"ok": exit == 0, "mutates": false, "tool": "cem-" + action,
		"stableVerification": StableVerification{Result: result, Exit: exit},
	}
	if action == "status" {
		envelope["map"] = filepath.Join(s.workRoot.Path(), filepath.FromSlash(options.MapPath))
		envelope["state"] = stableStates[result.Outcome]
	}
	return envelope, nil
}
