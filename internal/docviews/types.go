// Package docviews compiles proof-bound audience projections from an existing
// admitted Human Documentation Compiler plan. It does not generate prose, grant
// access, inspect a repository, or mutate its inputs.
package docviews

import (
	"fmt"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/doccompiler"
)

const (
	TruthProfile        = "corvint-cross-audience-truth/0"
	ViewProfile         = "corvint-cross-audience-view/0"
	BundleProfile       = "corvint-cross-audience-bundle/0"
	VerificationProfile = "corvint-cross-audience-verification/0"

	maxClaims              = 10_000
	maxEvidencePerClaim    = 256
	maxLimitationsPerClaim = 64
	maxClaimIDBytes        = 256
	maxTextBytes           = 64 << 10
	maxLimitationBytes     = 4 << 10
	maxBundleBytes         = 64 << 20
)

type Audience string

const (
	AudienceNovice    Audience = "NOVICE"
	AudienceOperator  Audience = "OPERATOR"
	AudienceAPI       Audience = "API"
	AudienceSecurity  Audience = "SECURITY"
	AudienceExecutive Audience = "EXECUTIVE"
)

var audienceOrder = [...]Audience{
	AudienceNovice,
	AudienceOperator,
	AudienceAPI,
	AudienceSecurity,
	AudienceExecutive,
}

type Detail string

const (
	DetailBrief    Detail = "BRIEF"
	DetailStandard Detail = "STANDARD"
	DetailFull     Detail = "FULL"
)

type ReviewState string

const (
	ReviewGenerated ReviewState = "GENERATED"
	ReviewVerified  ReviewState = "VERIFIED"
	ReviewReviewed  ReviewState = "REVIEWED"
)

type Currency string

const (
	CurrencyCurrent Currency = "CURRENT"
	CurrencyStale   Currency = "STALE"
	CurrencyUnknown Currency = "UNKNOWN"
)

type ClaimOverlay struct {
	ClaimID        string      `json:"claim_id"`
	Currency       Currency    `json:"currency"`
	CurrencyReason string      `json:"currency_reason,omitempty"`
	Limitations    []string    `json:"limitations"`
	ReviewState    ReviewState `json:"review_state"`
}

type ProjectionRecipe struct {
	Audience Audience `json:"audience"`
	ClaimIDs []string `json:"claim_ids"`
	Detail   Detail   `json:"detail"`
}

type CompileOptions struct {
	DisclosurePolicySHA256 string
	DisclosureScope        string
	Overlays               []ClaimOverlay
	Recipes                []ProjectionRecipe
}

// SourcePlan is the exact corvint-human-documentation-plan/0 bytes, the proposal
// patch they bind, and the in-memory index doccompiler.VerifyAdmittedPlan
// reproduces them against (CATN-V0-001). The index is only read.
type SourcePlan struct {
	Index *contextindex.Index
	Plan  []byte
	Patch []byte
}

// TruthEvidence is one admitted HDC evidence anchor, copied with every member
// into the truth corpus. Its revision is the plan's source revision.
type TruthEvidence = doccompiler.Anchor

// TruthClaim contains every factual or epistemic field exactly once. Audience
// views may reference its ID but cannot carry replacements for these fields.
type TruthClaim struct {
	Currency       Currency        `json:"currency"`
	CurrencyReason string          `json:"currency_reason,omitempty"`
	Evidence       []TruthEvidence `json:"evidence"`
	Frontier       string          `json:"frontier,omitempty"`
	ID             string          `json:"id"`
	Kind           string          `json:"kind"`
	Limitations    []string        `json:"limitations"`
	ReviewState    ReviewState     `json:"review_state"`
	Scope          string          `json:"scope"`
	State          string          `json:"state"`
	Text           string          `json:"text"`
}

type TruthCorpus struct {
	Claims                 []TruthClaim `json:"claims"`
	DisclosurePolicySHA256 string       `json:"disclosure_policy_sha256"`
	DisclosureScope        string       `json:"disclosure_scope"`
	PlanSHA256             string       `json:"plan_sha256"`
	Profile                string       `json:"profile"`
	Revision               string       `json:"revision"`
}

// AudienceView contains presentation choices only. A consumer must resolve
// ClaimIDs against the bound TruthCorpus.
type AudienceView struct {
	Audience               Audience `json:"audience"`
	ClaimIDs               []string `json:"claim_ids"`
	Detail                 Detail   `json:"detail"`
	DisclosurePolicySHA256 string   `json:"disclosure_policy_sha256"`
	DisclosureScope        string   `json:"disclosure_scope"`
	Profile                string   `json:"profile"`
	TruthSHA256            string   `json:"truth_sha256"`
}

type Bundle struct {
	Profile     string         `json:"profile"`
	Truth       TruthCorpus    `json:"truth"`
	TruthSHA256 string         `json:"truth_sha256"`
	Views       []AudienceView `json:"views"`
}

type Divergence struct {
	Audience Audience `json:"audience,omitempty"`
	ClaimID  string   `json:"claim_id,omitempty"`
	Code     string   `json:"code"`
}

type VerificationReport struct {
	Divergences []Divergence `json:"divergences"`
	PlanSHA256  string       `json:"plan_sha256"`
	Profile     string       `json:"profile"`
	Status      string       `json:"status"`
	TruthSHA256 string       `json:"truth_sha256"`
}

type Error struct {
	Code    string
	Message string
}

func (problem *Error) Error() string { return problem.Code + ": " + problem.Message }

func failure(code, format string, values ...any) error {
	return &Error{Code: code, Message: fmt.Sprintf(format, values...)}
}
