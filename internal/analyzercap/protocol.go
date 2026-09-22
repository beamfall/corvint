package analyzercap

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"unicode/utf8"
)

const (
	AnalyzerProtocolV0 = "corvint-analyzer/v0"
	MaxProtocolBytes   = 64 << 10
	MaxFacts           = 64
	MaxDiagnostics     = 32
)

type ProtocolLimits struct {
	WallMilliseconds uint64 `json:"wallMilliseconds"`
	MemoryBytes      uint64 `json:"memoryBytes"`
	MaxChildren      uint64 `json:"maxChildren"`
	MaxRequestBytes  uint64 `json:"maxRequestBytes"`
	MaxResponseBytes uint64 `json:"maxResponseBytes"`
	MaxStderrBytes   uint64 `json:"maxStderrBytes"`
}
type InputHandle struct {
	Handle Opaque `json:"handle"`
	Digest Opaque `json:"digest"`
}
type ProposedFact struct {
	Kind  Opaque `json:"kind"`
	Value Opaque `json:"value"`
}
type AnalyzerDiagnostic struct {
	Code Opaque `json:"code"`
}
type AnalyzerRequest struct {
	Protocol               Opaque         `json:"protocol"`
	RequestID              Opaque         `json:"requestId"`
	ProfileIdentity        Identity       `json:"profileIdentity"`
	LockDigest             Opaque         `json:"lockDigest"`
	RegistrySnapshotDigest Opaque         `json:"registrySnapshotDigest"`
	TrustEpoch             Opaque         `json:"trustEpoch"`
	ResolverDigest         Opaque         `json:"resolverDigest"`
	ClauseID               Opaque         `json:"clauseId"`
	CompilationUnitID      Opaque         `json:"compilationUnitId"`
	Target                 Platform       `json:"target"`
	ProjectionDigest       Opaque         `json:"projectionDigest"`
	ProjectionScheme       Opaque         `json:"projectionScheme"`
	Inputs                 []InputHandle  `json:"inputs"`
	InputBinding           Identity       `json:"inputBinding"`
	Features               []Feature      `json:"features"`
	Limits                 ProtocolLimits `json:"limits"`
}
type AnalyzerResponse struct {
	Protocol               Opaque               `json:"protocol"`
	RequestID              Opaque               `json:"requestId"`
	ProfileIdentity        Identity             `json:"profileIdentity"`
	LockDigest             Opaque               `json:"lockDigest"`
	RegistrySnapshotDigest Opaque               `json:"registrySnapshotDigest"`
	TrustEpoch             Opaque               `json:"trustEpoch"`
	ResolverDigest         Opaque               `json:"resolverDigest"`
	ClauseID               Opaque               `json:"clauseId"`
	CompilationUnitID      Opaque               `json:"compilationUnitId"`
	Target                 Platform             `json:"target"`
	ProjectionDigest       Opaque               `json:"projectionDigest"`
	ProjectionScheme       Opaque               `json:"projectionScheme"`
	Inputs                 []InputHandle        `json:"inputs"`
	InputBinding           Identity             `json:"inputBinding"`
	Facts                  []ProposedFact       `json:"facts"`
	ProfileEvidence        []InputHandle        `json:"profileEvidence"`
	Diagnostics            []AnalyzerDiagnostic `json:"diagnostics"`
}

func MarshalAnalyzerRequest(request AnalyzerRequest) ([]byte, error) {
	if err := request.validate(); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(request, json.Deterministic(true))
	if err != nil || len(encoded) > MaxProtocolBytes || uint64(len(encoded)) > request.Limits.MaxRequestBytes {
		return nil, fail(LimitExceeded)
	}
	return encoded, nil
}
func DecodeAnalyzerResponse(raw []byte, request AnalyzerRequest) (AnalyzerResponse, error) {
	if err := request.validate(); err != nil {
		return AnalyzerResponse{}, err
	}
	if len(raw) == 0 || len(raw) > MaxProtocolBytes || uint64(len(raw)) > request.Limits.MaxResponseBytes || !utf8.Valid(raw) {
		return AnalyzerResponse{}, fail(ProtocolEchoMismatch)
	}
	var response AnalyzerResponse
	if err := json.Unmarshal(raw, &response, json.RejectUnknownMembers(true)); err != nil {
		if errors.Is(err, json.ErrUnknownName) {
			return AnalyzerResponse{}, fail(ProtocolEchoMismatch)
		}
		return AnalyzerResponse{}, fail(ProtocolEchoMismatch)
	}
	canonical, err := json.Marshal(response, json.Deterministic(true))
	if err != nil || !bytes.Equal(canonical, raw) {
		return AnalyzerResponse{}, fail(ProtocolEchoMismatch)
	}
	if err := response.validate(request); err != nil {
		return AnalyzerResponse{}, err
	}
	return response, nil
}
func sameResponse(left, right AnalyzerResponse) bool {
	leftBytes, leftErr := json.Marshal(left, json.Deterministic(true))
	rightBytes, rightErr := json.Marshal(right, json.Deterministic(true))
	return leftErr == nil && rightErr == nil && bytes.Equal(leftBytes, rightBytes)
}
func (r AnalyzerRequest) validate() error {
	if r.Protocol != AnalyzerProtocolV0 || r.ProfileIdentity == "" || mustOpaque(Opaque(r.InputBinding)) != nil {
		return fail(ProfileUnavailable)
	}
	values := []Opaque{r.Protocol, r.RequestID, r.LockDigest, r.RegistrySnapshotDigest, r.TrustEpoch, r.ResolverDigest, r.ClauseID, r.CompilationUnitID, r.ProjectionDigest, r.ProjectionScheme}
	for _, value := range values {
		if err := mustOpaque(value); err != nil {
			return fail(ProfileUnavailable)
		}
	}
	if err := r.Target.validate(); err != nil {
		return fail(TargetPlatformUnsupported)
	}
	if err := r.Limits.validate(); err != nil {
		return err
	}
	if len(r.Inputs) > MaxInputDigests || len(r.Features) > MaxFeatures {
		return fail(LimitExceeded)
	}
	if err := validateInputs(r.Inputs); err != nil {
		return err
	}
	featureNames := map[Opaque]struct{}{}
	for _, feature := range r.Features {
		if err := mustOpaque(feature.Name); err != nil || !feature.State.valid() {
			return fail(ProfileUnavailable)
		}
		if _, duplicate := featureNames[feature.Name]; duplicate {
			return fail(ProfileUnavailable)
		}
		featureNames[feature.Name] = struct{}{}
	}
	return nil
}
func (r AnalyzerResponse) validate(request AnalyzerRequest) error {
	if r.Protocol != request.Protocol || r.RequestID != request.RequestID || r.ProfileIdentity != request.ProfileIdentity ||
		r.LockDigest != request.LockDigest || r.RegistrySnapshotDigest != request.RegistrySnapshotDigest || r.TrustEpoch != request.TrustEpoch ||
		r.ResolverDigest != request.ResolverDigest || r.ClauseID != request.ClauseID || r.CompilationUnitID != request.CompilationUnitID ||
		r.Target != request.Target || r.ProjectionDigest != request.ProjectionDigest || r.ProjectionScheme != request.ProjectionScheme ||
		r.InputBinding != request.InputBinding || !equalInputs(r.Inputs, request.Inputs) {
		return fail(ProtocolEchoMismatch)
	}
	if len(r.Facts) > MaxFacts || len(r.ProfileEvidence) > MaxInputDigests || len(r.Diagnostics) > MaxDiagnostics {
		return fail(LimitExceeded)
	}
	if err := validateInputs(r.ProfileEvidence); err != nil || !equalInputs(r.ProfileEvidence, request.Inputs) {
		return fail(ProtocolEchoMismatch)
	}
	for _, fact := range r.Facts {
		if err := mustOpaque(fact.Kind); err != nil {
			return fail(ProtocolEchoMismatch)
		}
		if err := mustOpaque(fact.Value); err != nil {
			return fail(ProtocolEchoMismatch)
		}
	}
	for _, diagnostic := range r.Diagnostics {
		if err := mustOpaque(diagnostic.Code); err != nil {
			return fail(ProtocolEchoMismatch)
		}
	}
	return nil
}
func (l ProtocolLimits) validate() error {
	if l.WallMilliseconds == 0 || l.WallMilliseconds > 100 || l.MaxChildren != 1 || l.MaxRequestBytes == 0 || l.MaxRequestBytes > MaxProtocolBytes || l.MaxResponseBytes == 0 || l.MaxResponseBytes > MaxProtocolBytes || l.MaxStderrBytes == 0 || l.MaxStderrBytes > MaxProtocolBytes {
		return fail(LimitExceeded)
	}
	return nil
}
func validateInputs(inputs []InputHandle) error {
	seen := map[Opaque]struct{}{}
	for _, input := range inputs {
		if err := mustOpaque(input.Handle); err != nil {
			return fail(InputEvidenceMismatch)
		}
		if err := mustOpaque(input.Digest); err != nil {
			return fail(InputEvidenceMismatch)
		}
		if _, duplicate := seen[input.Handle]; duplicate {
			return fail(InputEvidenceMismatch)
		}
		seen[input.Handle] = struct{}{}
	}
	return nil
}
func equalInputs(left, right []InputHandle) bool {
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
