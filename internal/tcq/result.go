package tcq

// Axis values (TCQ-V0-006). No axis rewrites another and none is a confidence,
// coverage, proof, or generic success score.
const (
	AssociationAssociated = "ASSOCIATED"
	AssociationAbstained  = "ABSTAINED"

	HygieneEligible   = "ELIGIBLE"
	HygieneIneligible = "INELIGIBLE"
	HygieneAbstained  = "ABSTAINED"

	ReportNotMatched = "NOT_MATCHED"
	ReportPassed     = "PASSED"
	ReportFailed     = "FAILED"
	ReportError      = "ERROR"
	ReportSkipped    = "SKIPPED"
	ReportAmbiguous  = "AMBIGUOUS"
)

// ClaimResult is one selected claim edge, in the shape of the reference vector
// at docs/specs/test-claim-qualification-v0.md:391. Empty string stands for the
// document's `null` in the nullable fields.
type ClaimResult struct {
	ObligationID       string
	ClaimID            string
	AnchorProfile      string
	AssociationKind    string
	AssociationState   string
	HygieneState       string
	ReportState        string
	TestUnitID         string
	ExecutionKeySha256 string
	RowIDs             []string
	Reasons            []string
	Relation           string
	// AuthorityClass is CALLER_REPORTED for every edge, including abstentions
	// (TCQ-V0-006). It is carried rather than assumed so no future producer can
	// silently upgrade it.
	AuthorityClass string
}

// Result and ClaimResult are the producer side of the frozen seam declared in
// internal/frontier/upstream.go. A shim converts Result.ID/Result.Claims into
// frontier.TCQResult/TCQClaimResult field for field; this package deliberately
// does not import frontier, which consumes it. Frontier's TCQRequest carries no
// CEM bytes, so a shim must supply the canonical `cem/0.2` artifact the OCM
// binds — TCQ-V0-001 requires it and TCQ-V0-002 refuses `cem/0.1`.
//
// Result is one `tcq/0` document. It is a recomputable local cache, never an
// authority token or execution record (TCQ-V0-043): Raw is the canonical bytes
// the identity commits to, and every consumer must recompute rather than trust.
type Result struct {
	raw    []byte
	id     string
	claims []ClaimResult
}

// Raw returns the canonical document bytes, terminal LF included.
func (result Result) Raw() []byte { return append([]byte(nil), result.raw...) }

// ID is the `tcq:sha256:` identity of TCQ-V0-040.
func (result Result) ID() string { return result.id }

// Claims returns one result per selected edge, in TCQ-V0-003 order.
func (result Result) Claims() []ClaimResult { return append([]ClaimResult(nil), result.claims...) }
