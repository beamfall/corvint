package jstestreceipt

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	AttestedExternalProfile               = "corvint-playwright-external/1"
	ApplicationAttestationProfile         = "corvint-application-attestation/0"
	ApplicationAttestationProviderProfile = "corvint-application-attestation-command/0"
	ApplicationAttestationConfigProfile   = "corvint-application-attestation-config/0"
	ApplicationAttestationLimit           = 64 << 10
	ApplicationAttestationExecutableLimit = 64 << 20
)

var (
	GitOIDPattern    = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)
	digestPattern    = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	RawDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	TokenPattern     = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
)

type ApplicationAttestationConfig struct {
	Profile     string                            `json:"profile"`
	Expectation ApplicationAttestationExpectation `json:"expectation"`
}

func IsExternalProfile(profile string) bool {
	return profile == ExternalProfile || profile == AttestedExternalProfile || profile == SensitiveExternalProfile || profile == AttemptExternalProfile
}

func ValidateExpectation(expectation ApplicationAttestationExpectation) string {
	if !validRepositoryExpectation(expectation.Repository) {
		return "application-attestation-expectation-invalid"
	}
	if !validArtifact(expectation.Build) || !validArtifact(expectation.Configuration) || !TokenPattern.MatchString(expectation.InstanceKind) {
		return "application-attestation-expectation-invalid"
	}
	return ""
}

func validRepositoryExpectation(repository ApplicationRepositoryExpectation) bool {
	if !GitOIDPattern.MatchString(repository.RootCommit) || !GitOIDPattern.MatchString(repository.Revision) || !GitOIDPattern.MatchString(repository.Tree) {
		return false
	}
	switch repository.DirtyPolicy {
	case "require-clean":
		return repository.DirtyDigest == ""
	case "allow-dirty-with-digest":
		return digestPattern.MatchString(repository.DirtyDigest)
	default:
		return false
	}
}

func validArtifact(artifact ApplicationArtifactIdentity) bool {
	return TokenPattern.MatchString(artifact.Kind) && digestPattern.MatchString(artifact.Digest)
}

func ValidateAttestationShape(attestation ApplicationAttestation) string {
	if attestation.Profile != ApplicationAttestationProfile {
		return "application-attestation-profile-unknown"
	}
	repository := attestation.Repository
	if !GitOIDPattern.MatchString(repository.RootCommit) || !GitOIDPattern.MatchString(repository.Revision) || !GitOIDPattern.MatchString(repository.Tree) {
		return "application-attestation-identity-unavailable"
	}
	if repository.DirtyState != "clean" && repository.DirtyState != "dirty" {
		return "application-attestation-identity-unavailable"
	}
	if repository.DirtyState == "clean" && repository.DirtyDigest != "" {
		return "application-attestation-contradictory"
	}
	if repository.DirtyState == "dirty" && !digestPattern.MatchString(repository.DirtyDigest) {
		return "application-attestation-identity-unavailable"
	}
	if !validArtifact(attestation.Build) || !validArtifact(attestation.Configuration) {
		return "application-attestation-identity-unavailable"
	}
	if !TokenPattern.MatchString(attestation.Instance.Kind) || strings.TrimSpace(attestation.Instance.ID) == "" || strings.TrimSpace(attestation.Instance.StartGeneration) == "" || len(attestation.Instance.ID) > 4096 || len(attestation.Instance.StartGeneration) > 4096 {
		return "application-attestation-identity-unavailable"
	}
	if attestation.Health.State != "healthy" && attestation.Health.State != "unhealthy" && attestation.Health.State != "unknown" {
		return "application-attestation-contradictory"
	}
	if len(attestation.Health.Detail) > 4096 {
		return "application-attestation-output-invalid"
	}
	if attestation.Health.State != "healthy" {
		return "application-attestation-unhealthy"
	}
	return ""
}

func CompareExpectation(expectation ApplicationAttestationExpectation, actual ApplicationAttestation) string {
	switch {
	case actual.Repository.RootCommit != expectation.Repository.RootCommit:
		return "application-root-commit-mismatch"
	case actual.Repository.Revision != expectation.Repository.Revision:
		return "application-revision-mismatch"
	case actual.Repository.Tree != expectation.Repository.Tree:
		return "application-tree-mismatch"
	case expectation.Repository.DirtyPolicy == "require-clean" && actual.Repository.DirtyState != "clean":
		return "application-dirty-policy-mismatch"
	case expectation.Repository.DirtyPolicy == "allow-dirty-with-digest" && actual.Repository.DirtyDigest != expectation.Repository.DirtyDigest:
		return "application-dirty-digest-mismatch"
	case actual.Build != expectation.Build:
		return "application-build-mismatch"
	case actual.Configuration != expectation.Configuration:
		return "application-configuration-mismatch"
	case actual.Instance.Kind != expectation.InstanceKind:
		return "application-instance-kind-mismatch"
	}
	return ""
}

func CompareApplicationAttestations(before, after ApplicationAttestation) []string {
	var failures []string
	if before.Repository != after.Repository {
		failures = append(failures, "application-repository-drift")
	}
	if before.Build != after.Build {
		failures = append(failures, "application-build-drift")
	}
	if before.Configuration != after.Configuration {
		failures = append(failures, "application-configuration-drift")
	}
	if before.Instance != after.Instance {
		failures = append(failures, "application-restarted")
	}
	return failures
}

func ApplicationAttestationUnknown(receipt *ApplicationAttestationReceipt) bool {
	if receipt == nil || len(receipt.Failures) != 0 || receipt.Before == nil || receipt.After == nil {
		return true
	}
	if receipt.Provider.Profile != ApplicationAttestationProviderProfile || len(receipt.Provider.Argv) == 0 || receipt.Provider.Argv[0] != receipt.Provider.ExecutablePath || !filepath.IsAbs(receipt.Provider.ExecutablePath) || !RawDigestPattern.MatchString(receipt.Provider.ExecutableDigest) || !filepath.IsAbs(receipt.Provider.ConfigPath) || !RawDigestPattern.MatchString(receipt.Provider.ConfigDigest) || receipt.Provider.Environment == nil {
		return true
	}
	if ValidateExpectation(receipt.Expectation) != "" {
		return true
	}
	config, err := json.Marshal(ApplicationAttestationConfig{Profile: ApplicationAttestationConfigProfile, Expectation: receipt.Expectation})
	if err != nil || Sha256Hex(append(config, '\n')) != receipt.Provider.ConfigDigest {
		return true
	}
	for _, observation := range []*ApplicationAttestationObservation{receipt.Before, receipt.After} {
		canonical, err := json.Marshal(observation.Attestation)
		canonical = append(canonical, '\n')
		if err != nil || Sha256Hex(canonical) != observation.OutputDigest || ValidateAttestationShape(observation.Attestation) != "" || CompareExpectation(receipt.Expectation, observation.Attestation) != "" {
			return true
		}
	}
	return len(CompareApplicationAttestations(receipt.Before.Attestation, receipt.After.Attestation)) != 0
}

func TestRepositoryUnknown(start, publish *ApplicationRepositoryIdentity) bool {
	if start == nil || publish == nil || *start != *publish {
		return true
	}
	if start.DirtyState != "clean" || start.DirtyDigest != "" {
		return true
	}
	return !GitOIDPattern.MatchString(start.RootCommit) || !GitOIDPattern.MatchString(start.Revision) || !GitOIDPattern.MatchString(start.Tree)
}

func Sha256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
