package doccompiler

import (
	"crypto/sha256"
	"encoding/hex"
)

type ReceiptEvidence struct {
	ClaimID    string `json:"claim_id"`
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
	Revision   string `json:"revision"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	Authority  string `json:"authority"`
	Confidence string `json:"confidence"`
	Reason     string `json:"reason"`
}

type Receipt struct {
	Profile             string             `json:"profile"`
	DeliveryStage       string             `json:"delivery_stage"`
	Claim               string             `json:"claim"`
	MaterialProfile     string             `json:"material_profile"`
	Toolchain           Toolchain          `json:"toolchain"`
	Config              FilePin            `json:"config"`
	ConfigObservations  ConfigObservations `json:"config_observations"`
	ConfigQualification string             `json:"config_qualification"`
	PlanSHA256          string             `json:"plan_sha256"`
	Build               BuildResult        `json:"build"`
	Evidence            []ReceiptEvidence  `json:"evidence"`
	Uncertainty         []string           `json:"uncertainty"`
}

func NewReceipt(environment Environment, plan PatchPlan, build BuildResult) (Receipt, error) {
	if environment.Qualification == "" || plan.PlanSHA256 == "" {
		return Receipt{}, failure("receipt-input-invalid", "environment and plan must be closed before receipt creation")
	}
	if len(plan.Evidence) != 0 {
		return Receipt{}, failure("immutable-evidence-unavailable", "experimental receipts cannot authenticate plan evidence")
	}
	uncertainty := append([]string{}, environment.Observations.Uncertainty...)
	seenClaims := map[string]struct{}{}
	for _, document := range plan.Documents {
		for _, claim := range document.Claims {
			if err := validateClaim(claim, seenClaims); err != nil {
				return Receipt{}, err
			}
			uncertainty = append(uncertainty, "UNKNOWN "+claim.ID+": "+claim.Uncertainty)
		}
	}
	uncertainty = append(uncertainty, build.Uncertainty...)
	if build.OfflineQualification != "PASS" {
		uncertainty = append(uncertainty, "offline behavior is not qualified without build-bound external network-denial evidence")
	}
	receipt := Receipt{
		Profile: ReceiptProfile, DeliveryStage: "experimental", Claim: "UNPROVEN", MaterialProfile: MaterialProfile,
		Toolchain: environment.Toolchain, Config: environment.Config,
		ConfigObservations: environment.Observations, ConfigQualification: environment.Qualification,
		PlanSHA256: plan.PlanSHA256, Build: build, Evidence: []ReceiptEvidence{},
		Uncertainty: uniqueSorted(uncertainty),
	}
	if err := VerifyReceipt(receipt); err != nil {
		return Receipt{}, err
	}
	return receipt, nil
}

func CanonicalReceipt(receipt Receipt) ([]byte, string, error) {
	if err := VerifyReceipt(receipt); err != nil {
		return nil, "", err
	}
	encoded, err := CanonicalJSON(receipt)
	if err != nil {
		return nil, "", failure("receipt-encoding-failed", "cannot encode canonical receipt")
	}
	digest := sha256.Sum256(encoded)
	return encoded, hex.EncodeToString(digest[:]), nil
}

func VerifyReceipt(receipt Receipt) error {
	if receipt.Profile != ReceiptProfile || receipt.MaterialProfile != MaterialProfile || receipt.DeliveryStage != "experimental" || receipt.Claim != "UNPROVEN" {
		return failure("invalid-receipt-envelope", "receipt profile, stage, or claim is invalid")
	}
	if receipt.ConfigQualification != "QUALIFIED" && receipt.ConfigQualification != "UNQUALIFIED" {
		return failure("invalid-receipt-qualification", "config qualification is invalid")
	}
	if receipt.Build.BuildStrictStatus != "PASS" && receipt.Build.BuildStrictStatus != "FAIL" && receipt.Build.BuildStrictStatus != "NOT_RUN" {
		return failure("invalid-build-status", "BUILD_STRICT status is invalid")
	}
	if receipt.Build.OfflineQualification != "PASS" && receipt.Build.OfflineQualification != "FAIL" && receipt.Build.OfflineQualification != "NOT_OBSERVED" && receipt.Build.OfflineQualification != "UNKNOWN" {
		return failure("invalid-offline-status", "offline qualification is invalid")
	}
	if receipt.Build.BuildStrictStatus == "PASS" {
		if !digestPattern.MatchString(receipt.Build.OutputSHA256) || receipt.Build.OutputFiles < 1 || receipt.Build.OutputBytes < 1 {
			return failure("invalid-build-evidence", "passing strict build lacks bounded output evidence")
		}
	}
	if receipt.Build.OfflineQualification == "PASS" {
		return failure("invalid-offline-evidence", "offline PASS is unavailable without a concrete build-bound external observer")
	}
	if !digestPattern.MatchString(receipt.PlanSHA256) || !digestPattern.MatchString(receipt.Config.SHA256) {
		return failure("invalid-receipt-digest", "receipt plan or config digest is invalid")
	}
	if !digestPattern.MatchString(receipt.Toolchain.ProjectLock.SHA256) || !digestPattern.MatchString(receipt.Toolchain.EnvironmentSHA256) || receipt.Toolchain.ProjectLock.Path == "" {
		return failure("invalid-toolchain-pin", "receipt lacks an exact project lock and environment digest")
	}
	if receipt.Toolchain.MkDocsVersion == "" || receipt.Toolchain.MaterialVersion == "" || receipt.Toolchain.MarkdownVersion == "" || receipt.Toolchain.PyMdownVersion == "" {
		return failure("invalid-toolchain-pin", "receipt lacks an exact MkDocs/Material/Markdown/PyMdown version")
	}
	if receipt.Toolchain.DistributionCount != len(receipt.Toolchain.Distributions) {
		return failure("invalid-toolchain-inventory", "distribution count does not match inventory")
	}
	if len(receipt.Evidence) != 0 {
		return failure("immutable-evidence-unavailable", "experimental receipts cannot authenticate immutable evidence or authority")
	}
	return nil
}
