// Package witnesscollapse compiles explicitly declared common causes over one
// exact CEM into a conservative upper bound on independent witnesses.
//
// It is an experimental, structural-only library. It neither discovers common
// causes nor proves that undeclared witnesses are independent.
package witnesscollapse

const (
	Profile       = "corvint-correlated-witness-collapse/experimental-v0"
	DeliveryStage = "experimental"
	Claim         = "UNPROVEN"
	Assurance     = "declared-common-causes-only"

	MaxCauses       = 8192
	MaxAttributions = 16384

	CausePrefix = "cause:sha256:"
	GroupPrefix = "correlation-group:sha256:"
)

// CauseKind is one closed common-cause dimension.
type CauseKind string

const (
	CauseSource    CauseKind = "source"
	CauseGenerator CauseKind = "generator"
	CauseParser    CauseKind = "parser"
	CauseOracle    CauseKind = "oracle"
	CausePremise   CauseKind = "premise"
)

// Attribution declares that one witness depends on one common cause. The
// attestation is a distinct immutable CEM evidence ID that explains the
// declared membership; its presence does not prove the declaration's semantics.
type Attribution struct {
	WitnessEvidenceID     string    `json:"witness_evidence_id"`
	Kind                  CauseKind `json:"kind"`
	CauseSubjectSHA256    string    `json:"cause_subject_sha256"`
	AttestationEvidenceID string    `json:"attestation_evidence_id"`
}

// CauseMember is one normalized witness membership and its pinned attestation.
type CauseMember struct {
	WitnessEvidenceID     string `json:"witness_evidence_id"`
	AttestationEvidenceID string `json:"attestation_evidence_id"`
}

// Cause is one normalized common cause with at least two distinct witnesses.
type Cause struct {
	ID            string        `json:"id"`
	Kind          CauseKind     `json:"kind"`
	SubjectSHA256 string        `json:"subject_sha256"`
	Members       []CauseMember `json:"members"`
}

// CorrelationGroup is one non-singleton transitive common-cause component.
type CorrelationGroup struct {
	ID                 string   `json:"id"`
	WitnessEvidenceIDs []string `json:"witness_evidence_ids"`
	CauseIDs           []string `json:"cause_ids"`
}

// HunkAssessment preserves the CEM disposition and reports only an upper
// bound: undeclared dependencies can make the true independent count lower.
type HunkAssessment struct {
	HunkID                     string   `json:"hunk_id"`
	Disposition                string   `json:"disposition"`
	WitnessEvidenceIDs         []string `json:"witness_evidence_ids"`
	RawWitnessCount            int      `json:"raw_witness_count"`
	IndependenceUpperBound     int      `json:"independence_upper_bound"`
	AppliedCorrelationGroupIDs []string `json:"applied_correlation_group_ids"`
}

// CEMBinding identifies the exact structural input. Repository verification is
// deliberately outside this package.
type CEMBinding struct {
	Spec         string `json:"spec"`
	MapSHA256    string `json:"map_sha256"`
	BaseRevision string `json:"base_revision"`
	PatchSHA256  string `json:"patch_sha256"`
}

// Report is the closed experimental output shape.
type Report struct {
	Profile       string             `json:"profile"`
	DeliveryStage string             `json:"delivery_stage"`
	Claim         string             `json:"claim"`
	Assurance     string             `json:"assurance"`
	CEM           CEMBinding         `json:"cem"`
	Causes        []Cause            `json:"causes"`
	Groups        []CorrelationGroup `json:"groups"`
	Hunks         []HunkAssessment   `json:"hunks"`
}
