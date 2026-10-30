package proofabstraction

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"unicode/utf8"
)

const (
	evidencePrefix    = "proof-evidence:sha256:"
	claimPrefix       = "proof-claim:sha256:"
	frontierPrefix    = "proof-frontier:sha256:"
	envelopePrefix    = "proof-state:sha256:"
	certificatePrefix = "ppa-certificate:sha256:"

	evidenceDomain    = "corvint-proof-evidence/0-experimental"
	claimDomain       = "corvint-proof-claim/0-experimental"
	frontierDomain    = "corvint-proof-frontier/0-experimental"
	envelopeDomain    = "corvint-proof-state/0-experimental"
	certificateDomain = "corvint-proof-preserving-abstraction-certificate/0-experimental"
)

type claimIdentity struct {
	ScopeSHA256     string `json:"scopeSha256"`
	StatementSHA256 string `json:"statementSha256"`
}

// BindEvidence validates and content-addresses one evidence wrapper.
func BindEvidence(value Evidence) (Evidence, error) {
	value.ID = ""
	if err := validateEvidenceBody(value); err != nil {
		return Evidence{}, err
	}
	id, err := domainID(evidencePrefix, evidenceDomain, value)
	if err != nil {
		return Evidence{}, err
	}
	value.ID = id
	return value, nil
}

// BindClaim validates and content-addresses one claim atom. Envelope binding
// later verifies that every evidence reference resolves.
func BindClaim(value Claim) (Claim, error) {
	value.ID = ""
	if err := validateClaimBody(value, nil); err != nil {
		return Claim{}, err
	}
	id, err := claimID(value)
	if err != nil {
		return Claim{}, err
	}
	value.ID = id
	return value, nil
}

// BindFrontier validates and content-addresses one frontier atom. Envelope
// binding later verifies whether its claim is present or intentionally absent.
func BindFrontier(value Frontier) (Frontier, error) {
	value.ID = ""
	if err := validateFrontierBody(value); err != nil {
		return Frontier{}, err
	}
	id, err := domainID(frontierPrefix, frontierDomain, value)
	if err != nil {
		return Frontier{}, err
	}
	value.ID = id
	return value, nil
}

// BindEnvelope validates and content-addresses a complete canonical envelope.
func BindEnvelope(value Envelope) (Envelope, error) {
	value.EnvelopeID = ""
	if err := validateEnvelopeBody(value); err != nil {
		return Envelope{}, err
	}
	id, err := domainID(envelopePrefix, envelopeDomain, value)
	if err != nil {
		return Envelope{}, err
	}
	value.EnvelopeID = id
	return value, nil
}

// MarshalEnvelope emits one deterministic, newline-free wire object.
func MarshalEnvelope(value Envelope) ([]byte, error) {
	if err := validateEnvelope(value); err != nil {
		return nil, err
	}
	return marshal(value)
}

// DecodeEnvelope accepts only the exact canonical encoding.
func DecodeEnvelope(raw []byte) (Envelope, error) {
	var value Envelope
	if err := decode(raw, &value); err != nil {
		return Envelope{}, err
	}
	if err := validateEnvelope(value); err != nil {
		return Envelope{}, err
	}
	return value, nil
}

// MarshalRequest emits one deterministic, newline-free request.
func MarshalRequest(value Request) ([]byte, error) {
	if err := validateRequest(value); err != nil {
		return nil, err
	}
	return marshal(value)
}

// DecodeRequest accepts only the exact canonical encoding.
func DecodeRequest(raw []byte) (Request, error) {
	var value Request
	if err := decode(raw, &value); err != nil {
		return Request{}, err
	}
	if err := validateRequest(value); err != nil {
		return Request{}, err
	}
	return value, nil
}

// MarshalCertificate emits one deterministic, newline-free certificate.
func MarshalCertificate(value Certificate) ([]byte, error) {
	if err := validateCertificate(value); err != nil {
		return nil, err
	}
	return marshal(value)
}

// DecodeCertificate accepts only the exact canonical encoding.
func DecodeCertificate(raw []byte) (Certificate, error) {
	var value Certificate
	if err := decode(raw, &value); err != nil {
		return Certificate{}, err
	}
	if err := validateCertificate(value); err != nil {
		return Certificate{}, err
	}
	return value, nil
}

func decode(raw []byte, target any) error {
	if len(raw) > MaxArtifactBytes {
		return fail(LimitExceeded)
	}
	if len(raw) == 0 || !utf8.Valid(raw) {
		return fail(Noncanonical)
	}
	if err := json.Unmarshal(raw, target, json.RejectUnknownMembers(true), json.MatchCaseInsensitiveNames(false)); err != nil {
		return fail(Noncanonical)
	}
	canonical, err := marshal(target)
	if err != nil || !bytes.Equal(canonical, raw) {
		return fail(Noncanonical)
	}
	return nil
}

func marshal(value any) ([]byte, error) {
	encoded, err := json.Marshal(value, json.Deterministic(true))
	if err != nil {
		return nil, fail(Noncanonical)
	}
	if len(encoded) > MaxArtifactBytes {
		return nil, fail(LimitExceeded)
	}
	return encoded, nil
}

func domainID(prefix, domain string, value any) (string, error) {
	encoded, err := marshal(value)
	if err != nil {
		return "", err
	}
	hasher := sha256.New()
	_, _ = hasher.Write([]byte(domain))
	_, _ = hasher.Write([]byte{0})
	_, _ = hasher.Write(encoded)
	return prefix + hex.EncodeToString(hasher.Sum(nil)), nil
}

func claimID(value Claim) (string, error) {
	identity := claimIdentity{ScopeSHA256: value.ScopeSHA256, StatementSHA256: value.StatementSHA256}
	return domainID(claimPrefix, claimDomain, identity)
}

func certificateID(value Certificate) (string, error) {
	value.CertificateID = ""
	return domainID(certificatePrefix, certificateDomain, value)
}

func requestSHA256(value Request) (string, error) {
	encoded, err := marshal(value)
	if err != nil {
		return "", err
	}
	hasher := sha256.New()
	_, _ = hasher.Write([]byte(RequestProfile))
	_, _ = hasher.Write([]byte{0})
	_, _ = hasher.Write(encoded)
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func validateEnvelope(value Envelope) error {
	if err := validateEnvelopeBody(value); err != nil {
		return err
	}
	expected, err := domainID(envelopePrefix, envelopeDomain, envelopeWithoutID(value))
	if err != nil || value.EnvelopeID != expected {
		return fail(SourceIDMismatch)
	}
	return nil
}

func envelopeWithoutID(value Envelope) Envelope {
	value.EnvelopeID = ""
	return value
}

func validateEnvelopeBody(value Envelope) error {
	if value.Profile != EnvelopeProfile {
		return fail(Noncanonical)
	}
	if err := validateRepository(value.Repository); err != nil {
		return err
	}
	if !isSHA256(value.PolicySHA256) || !isSHA256(value.PurposeSHA256) || !validCompleteness(value.Completeness) {
		return fail(Noncanonical)
	}
	if value.Claims == nil || value.Evidence == nil || value.Frontier == nil {
		return fail(Noncanonical)
	}
	if len(value.Claims) > MaxClaims || len(value.Evidence) > MaxEvidence || len(value.Frontier) > MaxFrontier {
		return fail(LimitExceeded)
	}
	evidenceByID, err := validateEvidenceList(value.Evidence)
	if err != nil {
		return err
	}
	claimIDs, referenced, err := validateClaimList(value.Claims, evidenceByID)
	if err != nil {
		return err
	}
	if len(referenced) != len(evidenceByID) {
		return fail(EvidenceMutated)
	}
	if err := validateFrontierList(value.Frontier, claimIDs); err != nil {
		return err
	}
	return nil
}

func validateEvidenceList(values []Evidence) (map[string]Evidence, error) {
	result := make(map[string]Evidence, len(values))
	previous := ""
	for _, value := range values {
		if value.ID <= previous {
			return nil, fail(Noncanonical)
		}
		if err := validateEvidenceBody(value); err != nil {
			return nil, err
		}
		expected, err := domainID(evidencePrefix, evidenceDomain, evidenceWithoutID(value))
		if err != nil || value.ID != expected {
			return nil, fail(EvidenceMutated)
		}
		result[value.ID] = value
		previous = value.ID
	}
	return result, nil
}

func evidenceWithoutID(value Evidence) Evidence {
	value.ID = ""
	return value
}

func validateEvidenceBody(value Evidence) error {
	if !validToken(value.SourceProfile) || !validToken(value.ExternalID) || !isSHA256(value.ContentSHA256) {
		return fail(Noncanonical)
	}
	return nil
}

func validateClaimList(values []Claim, evidence map[string]Evidence) (map[string]Claim, map[string]struct{}, error) {
	claims := make(map[string]Claim, len(values))
	referenced := make(map[string]struct{})
	previous := ""
	for _, value := range values {
		if value.ID <= previous {
			return nil, nil, fail(Noncanonical)
		}
		if err := validateClaimBody(value, evidence); err != nil {
			return nil, nil, err
		}
		expected, err := claimID(value)
		if err != nil || value.ID != expected {
			return nil, nil, fail(ClaimIdentityChanged)
		}
		claims[value.ID] = value
		for _, id := range value.Evidence {
			referenced[id] = struct{}{}
		}
		for _, id := range value.CounterEvidence {
			referenced[id] = struct{}{}
		}
		previous = value.ID
	}
	return claims, referenced, nil
}

func validateClaimBody(value Claim, evidence map[string]Evidence) error {
	if !isSHA256(value.StatementSHA256) || !isSHA256(value.ScopeSHA256) {
		return fail(Noncanonical)
	}
	if !validVerdict(value.Verdict) || !validAuthority(value.AuthorityClass) {
		return fail(Noncanonical)
	}
	if value.EvidenceClasses == nil || value.Evidence == nil || value.CounterEvidence == nil || value.UnknownReasons == nil {
		return fail(Noncanonical)
	}
	if len(value.Evidence)+len(value.CounterEvidence) > MaxClaimEvidence {
		return fail(LimitExceeded)
	}
	if !sortedEvidenceClasses(value.EvidenceClasses) || !sortedStrings(value.Evidence) || !sortedStrings(value.CounterEvidence) || !sortedTokens(value.UnknownReasons) {
		return fail(Noncanonical)
	}
	if intersects(value.Evidence, value.CounterEvidence) {
		return fail(Noncanonical)
	}
	if evidence != nil {
		if !referencesExist(value.Evidence, evidence) || !referencesExist(value.CounterEvidence, evidence) {
			return fail(EvidenceAdded)
		}
	}
	return validateVerdictShape(value)
}

func validateVerdictShape(value Claim) error {
	switch value.Verdict {
	case VerdictProved:
		if len(value.Evidence) == 0 || len(value.CounterEvidence) != 0 || len(value.UnknownReasons) != 0 || value.AuthorityClass == AuthorityNone {
			return fail(Noncanonical)
		}
	case VerdictRefuted:
		if len(value.Evidence) != 0 || len(value.CounterEvidence) == 0 || len(value.UnknownReasons) != 0 || value.AuthorityClass == AuthorityNone {
			return fail(Noncanonical)
		}
	case VerdictConflicted:
		if len(value.Evidence) == 0 || len(value.CounterEvidence) == 0 || len(value.UnknownReasons) != 0 || value.AuthorityClass == AuthorityNone {
			return fail(Noncanonical)
		}
	case VerdictUnknown:
		if len(value.UnknownReasons) == 0 {
			return fail(Noncanonical)
		}
	}
	return nil
}

func validateFrontierList(values []Frontier, claims map[string]Claim) error {
	previous := ""
	for _, value := range values {
		if value.ID <= previous {
			return fail(Noncanonical)
		}
		if err := validateFrontierBody(value); err != nil {
			return err
		}
		expected, err := domainID(frontierPrefix, frontierDomain, frontierWithoutID(value))
		if err != nil || value.ID != expected {
			return fail(FrontierRemoved)
		}
		_, claimPresent := claims[value.ClaimID]
		if value.Kind == FrontierAbstractedClaim && claimPresent {
			return fail(Noncanonical)
		}
		if value.Kind != FrontierAbstractedClaim && !claimPresent {
			return fail(Noncanonical)
		}
		if value.Kind == FrontierSourceUnknown && claims[value.ClaimID].Verdict != VerdictUnknown {
			return fail(Noncanonical)
		}
		if value.Kind == FrontierSourceConflict && claims[value.ClaimID].Verdict != VerdictConflicted {
			return fail(Noncanonical)
		}
		previous = value.ID
	}
	return nil
}

func validateFrontierBody(value Frontier) error {
	if !hasDigestPrefix(value.ClaimID, claimPrefix) || !validToken(value.Reason) || !validFrontierKind(value.Kind) {
		return fail(Noncanonical)
	}
	if value.Kind == FrontierAbstractedClaim {
		if value.SourceEnvelopeID == nil || !hasDigestPrefix(*value.SourceEnvelopeID, envelopePrefix) || value.Recovery != RecoveryLoadSourceEnvelope {
			return fail(Noncanonical)
		}
		return nil
	}
	if value.SourceEnvelopeID != nil || value.Recovery != RecoverySourceVerifier {
		return fail(Noncanonical)
	}
	return nil
}

func frontierWithoutID(value Frontier) Frontier {
	value.ID = ""
	return value
}

func validateRequest(value Request) error {
	if value.Profile != RequestProfile || !hasDigestPrefix(value.SourceEnvelopeID, envelopePrefix) || !isSHA256(value.PurposeSHA256) {
		return fail(Noncanonical)
	}
	if value.Actions == nil {
		return fail(Noncanonical)
	}
	if len(value.Actions) > MaxClaims {
		return fail(LimitExceeded)
	}
	previous := ""
	for _, action := range value.Actions {
		if action.ClaimID <= previous || !hasDigestPrefix(action.ClaimID, claimPrefix) {
			return fail(Noncanonical)
		}
		if err := validateAction(action); err != nil {
			return err
		}
		previous = action.ClaimID
	}
	return nil
}

func validateAction(value Action) error {
	switch value.Action {
	case ActionRetain:
		if value.Reason != nil {
			return fail(Noncanonical)
		}
	case ActionDemoteToUnknown, ActionOmit:
		if value.Reason == nil || !validReason(*value.Reason) {
			return fail(Noncanonical)
		}
	default:
		return fail(Noncanonical)
	}
	return nil
}

func validateCertificate(value Certificate) error {
	if value.Profile != CertificateProfile || value.Algorithm != AlgorithmProfile || value.Claim != CertificateClaim {
		return fail(CertificateMismatch)
	}
	if !hasDigestPrefix(value.SourceEnvelopeID, envelopePrefix) || !hasDigestPrefix(value.DestinationEnvelopeID, envelopePrefix) {
		return fail(CertificateMismatch)
	}
	if err := validateRepository(value.Repository); err != nil {
		return err
	}
	if !isSHA256(value.RequestSHA256) || !isSHA256(value.PolicySHA256) || !isSHA256(value.PurposeSHA256) || !validCompletenessRelation(value.CompletenessRelation) {
		return fail(CertificateMismatch)
	}
	if value.ClaimRelations == nil || value.OmittedClaims == nil || value.EvidenceAccounting == nil || value.FrontierAccounting.Preserved == nil || value.FrontierAccounting.Added == nil {
		return fail(CertificateMismatch)
	}
	if len(value.ClaimRelations)+len(value.OmittedClaims) > MaxClaims || len(value.EvidenceAccounting) > MaxEvidence || len(value.FrontierAccounting.Preserved)+len(value.FrontierAccounting.Added) > MaxFrontier {
		return fail(LimitExceeded)
	}
	if err := validateClaimRelations(value.ClaimRelations); err != nil {
		return err
	}
	if err := validateOmittedClaims(value.OmittedClaims); err != nil {
		return err
	}
	if err := validateEvidenceAccounting(value.EvidenceAccounting); err != nil {
		return err
	}
	if !sortedPrefixed(value.FrontierAccounting.Preserved, frontierPrefix) || !sortedPrefixed(value.FrontierAccounting.Added, frontierPrefix) {
		return fail(CertificateMismatch)
	}
	expected, err := certificateID(value)
	if err != nil || value.CertificateID != expected {
		return fail(CertificateMismatch)
	}
	return nil
}

func validateClaimRelations(values []ClaimRelation) error {
	previous := ""
	for _, value := range values {
		if value.SourceClaimID <= previous || !hasDigestPrefix(value.SourceClaimID, claimPrefix) || value.DestinationClaimID != value.SourceClaimID {
			return fail(CertificateMismatch)
		}
		switch value.Relation {
		case RelationIdentical:
			if value.Reason != nil {
				return fail(CertificateMismatch)
			}
		case RelationDemotedToUnknown:
			if value.Reason == nil || !validReason(*value.Reason) {
				return fail(CertificateMismatch)
			}
		default:
			return fail(CertificateMismatch)
		}
		previous = value.SourceClaimID
	}
	return nil
}

func validateOmittedClaims(values []OmittedClaim) error {
	previous := ""
	for _, value := range values {
		if value.SourceClaimID <= previous || !hasDigestPrefix(value.SourceClaimID, claimPrefix) || !hasDigestPrefix(value.FrontierID, frontierPrefix) || !validReason(value.Reason) {
			return fail(CertificateMismatch)
		}
		if value.SourceVerdict != VerdictProved && value.SourceVerdict != VerdictRefuted {
			return fail(CertificateMismatch)
		}
		previous = value.SourceClaimID
	}
	return nil
}

func validateEvidenceAccounting(values []EvidenceAccounting) error {
	previous := ""
	for _, value := range values {
		if value.SourceClaimIDs == nil || value.EvidenceID <= previous || !hasDigestPrefix(value.EvidenceID, evidencePrefix) || !sortedPrefixed(value.SourceClaimIDs, claimPrefix) {
			return fail(CertificateMismatch)
		}
		if value.State != EvidencePreserved && value.State != EvidenceOmitted {
			return fail(CertificateMismatch)
		}
		previous = value.EvidenceID
	}
	return nil
}

func validateRepository(value Repository) error {
	length := 0
	switch value.ObjectFormat {
	case "sha1":
		length = 40
	case "sha256":
		length = 64
	default:
		return fail(Noncanonical)
	}
	if !isLowerHex(value.Revision, length) || !isLowerHex(value.Tree, length) {
		return fail(Noncanonical)
	}
	return nil
}

func referencesExist(values []string, evidence map[string]Evidence) bool {
	for _, value := range values {
		if _, ok := evidence[value]; !ok {
			return false
		}
	}
	return true
}

func intersects(left, right []string) bool {
	seen := make(map[string]struct{}, len(left))
	for _, value := range left {
		seen[value] = struct{}{}
	}
	for _, value := range right {
		if _, ok := seen[value]; ok {
			return true
		}
	}
	return false
}

func sortedEvidenceClasses(values []EvidenceClass) bool {
	previous := EvidenceClass("")
	for _, value := range values {
		if value <= previous || !validEvidenceClass(value) {
			return false
		}
		previous = value
	}
	return true
}

func sortedTokens(values []string) bool {
	if !sortedStrings(values) {
		return false
	}
	for _, value := range values {
		if !validToken(value) {
			return false
		}
	}
	return true
}

func sortedPrefixed(values []string, prefix string) bool {
	if !sortedStrings(values) {
		return false
	}
	for _, value := range values {
		if !hasDigestPrefix(value, prefix) {
			return false
		}
	}
	return true
}

func sortedStrings(values []string) bool {
	previous := ""
	for _, value := range values {
		if value <= previous {
			return false
		}
		previous = value
	}
	return true
}

func validToken(value string) bool {
	if len(value) == 0 || len(value) > MaxTokenBytes {
		return false
	}
	for index := 0; index < len(value); index++ {
		if value[index] < 0x21 || value[index] > 0x7e {
			return false
		}
	}
	return true
}

func isSHA256(value string) bool { return isLowerHex(value, 64) }

func isLowerHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func hasDigestPrefix(value, prefix string) bool {
	return len(value) == len(prefix)+64 && value[:len(prefix)] == prefix && isSHA256(value[len(prefix):])
}

func validVerdict(value Verdict) bool {
	return value == VerdictProved || value == VerdictRefuted || value == VerdictConflicted || value == VerdictUnknown
}

func validAuthority(value AuthorityClass) bool {
	switch value {
	case AuthorityRepositoryAccepted, AuthorityOwningVerifier, AuthorityProviderQualified,
		AuthorityAdapterQualified, AuthorityCallerReported, AuthorityAdvisory, AuthorityNone:
		return true
	default:
		return false
	}
}

func validEvidenceClass(value EvidenceClass) bool {
	switch value {
	case EvidenceDeclared, EvidenceImplemented, EvidenceVerified, EvidenceObserved, EvidenceInferred:
		return true
	default:
		return false
	}
}

func validCompleteness(value Completeness) bool {
	return value == CompletenessComplete || value == CompletenessPartial || value == CompletenessUnknown
}

func validFrontierKind(value FrontierKind) bool {
	return value == FrontierSourceUnknown || value == FrontierSourceConflict || value == FrontierSourceExclusion || value == FrontierAbstractedClaim
}

func validReason(value Reason) bool { return value == ReasonAudience || value == ReasonBudget }

func validCompletenessRelation(value CompletenessRelation) bool {
	return value == CompletenessEqual || value == CompletenessWeakenedToPartial
}
