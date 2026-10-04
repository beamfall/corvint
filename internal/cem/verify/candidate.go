package verify

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// CandidateOptions contain independent verification inputs. No supplied patch
// or code-execution hook is admitted by this reference-integrity interface.
type CandidateOptions struct{ ExpectedBase, Target, ArtifactRoot string }

// CandidateResult deliberately has no test-success, criterion-satisfied or
// mutant-killed field. Limits apply equally to success and failure.
type CandidateResult struct {
	Profile            string            `json:"profile"`
	Spec               string            `json:"spec"`
	Integrity          string            `json:"integrity"`
	BaseRevision       string            `json:"baseRevision"`
	TargetRevision     string            `json:"targetRevision"`
	ReferenceIntegrity string            `json:"referenceIntegrity"`
	Limits             map[string]string `json:"limits"`
	Code               string            `json:"code"`
}

func candidateResult() CandidateResult {
	limits := map[string]string{}
	for _, key := range []string{"nativeAuthority", "historicalValidity", "currentApplicability", "sourceGitBinding", "authentication", "dependencyClosure", "criterionDiscrimination", "externalInteroperability"} {
		limits[key] = "NOT_OBSERVED"
	}
	return CandidateResult{Profile: "cem-candidate-verification/1", Spec: wire.CandidateSpec, Integrity: "NOT_VERIFIED", ReferenceIntegrity: "REFERENCE_INTEGRITY_ONLY", Limits: limits}
}

// ExperimentalCandidate verifies the explicit experimental profile without widening legacy
// Canonical or ParseMap admission. It checks immutable Git binding and opaque
// referenced bytes; it does not decode partial native Tasks or runner records.
func ExperimentalCandidate(ctx context.Context, repository *gitauth.Repository, raw []byte, options CandidateOptions) (result CandidateResult, retErr error) {
	result = candidateResult()
	defer func() {
		if retErr != nil {
			result.Code = cemcode.CodeOf(retErr)
		}
	}()
	document, err := wire.ParseCandidate(raw)
	if err != nil {
		return result, err
	}
	result.BaseRevision = document.Change.BaseRevision
	if !wire.IsGitOid(options.ExpectedBase) || !wire.IsGitOid(options.Target) || !filepath.IsAbs(options.ArtifactRoot) {
		return result, cemcode.New(cemcode.InvalidArguments, "candidate requires independent full base/target and absolute artifact root")
	}
	result.TargetRevision = options.Target
	limited, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	root, err := os.OpenRoot(options.ArtifactRoot)
	if err != nil {
		return result, cemcode.New("candidate-artifact-unavailable", "artifact root unavailable")
	}
	defer root.Close()
	if err = verifyCandidateArtifacts(limited, root, document.Artifacts); err != nil {
		return result, err
	}
	outcome, _, err := canonicalChange(limited, repository, &document.Change, CanonicalOptions{ExpectedBase: options.ExpectedBase, Target: options.Target, RawMapBytes: raw})
	if err != nil {
		return result, err
	}
	for _, drift := range outcome.Drift {
		if drift.Status != DriftStable && drift.Status != DriftRelocated {
			return result, cemcode.New(cemcode.EvidenceDrift, "candidate evidence drift is unresolved")
		}
	}
	if err = verifyCandidateArtifacts(limited, root, document.Artifacts); err != nil {
		return result, err
	}
	result.Integrity = "VERIFIED"
	result.ReferenceIntegrity = "REFERENCE_INTEGRITY_ONLY"
	return result, nil
}

func verifyCandidateArtifacts(ctx context.Context, root *os.Root, artifacts []wire.CandidateArtifact) error {
	total := int64(0)
	for _, a := range artifacts {
		if err := ctx.Err(); err != nil {
			return cemcode.New("candidate-artifact-unavailable", "artifact verification cancelled")
		}
		parts := strings.Split(a.Path, "/")
		for i := range parts {
			st, err := root.Lstat(strings.Join(parts[:i+1], "/"))
			if err != nil {
				return cemcode.New("candidate-artifact-unavailable", "declared artifact unavailable")
			}
			if st.Mode()&os.ModeSymlink != 0 || i < len(parts)-1 && !st.IsDir() || i == len(parts)-1 && !st.Mode().IsRegular() {
				return cemcode.New("candidate-artifact-unavailable", "artifact path must contain only directories and a regular file")
			}
		}
		f, err := root.Open(a.Path)
		if err != nil {
			return cemcode.New("candidate-artifact-unavailable", "declared artifact unavailable")
		}
		st, err := f.Stat()
		if err != nil || !st.Mode().IsRegular() || st.Size() > wire.CandidateMaxArtifactBytes {
			f.Close()
			return cemcode.New("candidate-artifact-bound", "artifact is not a bounded regular file")
		}
		data, err := io.ReadAll(io.LimitReader(f, wire.CandidateMaxArtifactBytes+1))
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			return cemcode.New("candidate-artifact-unavailable", "artifact read failed")
		}
		total += int64(len(data))
		if len(data) > wire.CandidateMaxArtifactBytes || total > wire.CandidateMaxTotalArtifactBytes {
			return cemcode.New("candidate-artifact-bound", "artifact byte inventory exceeds bound")
		}
		if err = ctx.Err(); err != nil {
			return cemcode.New("candidate-artifact-unavailable", "artifact verification cancelled")
		}
		digest := sha256.Sum256(data)
		if hex.EncodeToString(digest[:]) != a.Sha256 {
			return cemcode.New("candidate-artifact-digest", "artifact bytes differ from declared digest (%s)", a.Kind)
		}
	}
	return nil
}
