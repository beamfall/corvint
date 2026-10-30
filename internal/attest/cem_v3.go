package attest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"strconv"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// CEMPredicateTypeV3 is the additive predicate revision of CAX-V0-001: the
// cem/v2 claim plus the map's per-disposition hunk counts and the policy
// ceilings the producer applied. It is a separate predicateType, so a cem/v2
// statement stays byte for byte as before and a cem/v2-only verifier refuses
// a v3 envelope by type instead of misreading it.
const CEMPredicateTypeV3 = "https://corvint-context.dev/attestation/cem/v3"

// ErrCEMCountsMismatch is wrapped when the supplied map's hunk counts differ
// from the signed counts.
var ErrCEMCountsMismatch = errors.New("attest: map hunk counts do not match the attested counts")

// CEMCounts are the per-disposition hunk counts of a cem/1.0 map.
type CEMCounts struct {
	Total, Supported, Unknown, Mechanical int64
}

// CEMCeilings are the policy ceilings a producer applied; nil is not applied.
type CEMCeilings struct {
	MaxUnknown, MaxMechanical *int64
}

// StableMapCounts counts the hunks of a parsed cem/1.0 map by disposition,
// the same counts cem status reports (CCP-V0-010).
func StableMapCounts(document *wire.StableMap) CEMCounts {
	counts := CEMCounts{}
	for _, hunk := range document.Change.Hunks {
		counts.Total++
		switch hunk.Disposition {
		case "supported":
			counts.Supported++
		case "unknown":
			counts.Unknown++
		case "mechanical":
			counts.Mechanical++
		}
	}
	return counts
}

// exceeded names each ceiling counts exceed, in a fixed order.
func (ceilings CEMCeilings) exceeded(counts CEMCounts) []string {
	var issues []string
	if ceilings.MaxUnknown != nil && counts.Unknown > *ceilings.MaxUnknown {
		issues = append(issues, fmt.Sprintf("unknown %d > %d", counts.Unknown, *ceilings.MaxUnknown))
	}
	if ceilings.MaxMechanical != nil && counts.Mechanical > *ceilings.MaxMechanical {
		issues = append(issues, fmt.Sprintf("mechanical %d > %d", counts.Mechanical, *ceilings.MaxMechanical))
	}
	return issues
}

func number(value int64) json.Number { return json.Number(strconv.FormatInt(value, 10)) }

// CEMStatementV3 builds the CAX-V0-001 statement: the subjects and the
// baseRevision, patchSha256 and target members of CEMStatementV2, plus
// counts {total, supported, unknown, mechanical} read from the map and
// ceilings carrying exactly the applied maxUnknown and maxMechanical (an empty
// object when none applied). It refuses what CEMStatementV2 refuses, a
// negative ceiling, and a map whose counts exceed a ceiling, so no v3
// statement ever records a ceiling its counts break.
func CEMStatementV3(name string, cem []byte, target CEMTarget, ceilings CEMCeilings) ([]byte, error) {
	return cemStatementV3(name, cem, target, ceilings, nil)
}

// CEMPairedRange is the optional pairedRange member of a cem/v3 predicate
// (CAX-V0-034): the cem-paired-evidence/1 document's range pins and the
// SHA-256 of its exact bytes.
type CEMPairedRange struct {
	Name, BaseRevision, HeadRevision, EvidenceSha256 string
}

// CEMStatementV3Paired is CEMStatementV3 plus the pairedRange member read from
// a cem-paired-evidence/1 document, which must parse and bind the map's
// change and hunks (CAX-V0-034). The document's citations were re-verified
// against the second repository by cem status --paired, not here.
func CEMStatementV3Paired(name string, cem []byte, target CEMTarget, ceilings CEMCeilings, paired []byte) ([]byte, error) {
	document, err := wire.ParsePairedEvidence(paired)
	if err != nil {
		return nil, fmt.Errorf("attest: %w", err)
	}
	if _, err := wire.BindPairedEvidence(document, cem); err != nil {
		return nil, fmt.Errorf("attest: %w", err)
	}
	digest := sha256.Sum256(paired)
	return cemStatementV3(name, cem, target, ceilings, &CEMPairedRange{
		Name: document.Range.Name, BaseRevision: document.Range.BaseRevision,
		HeadRevision: document.Range.HeadRevision, EvidenceSha256: hex.EncodeToString(digest[:]),
	})
}

func cemStatementV3(name string, cem []byte, target CEMTarget, ceilings CEMCeilings, paired *CEMPairedRange) ([]byte, error) {
	v2, err := CEMStatementV2(name, cem, target)
	if err != nil {
		return nil, err
	}
	document, err := wire.ParseStable(cem)
	if err != nil {
		return nil, fmt.Errorf("attest: parse stable CEM: %w", err)
	}
	for _, ceiling := range []*int64{ceilings.MaxUnknown, ceilings.MaxMechanical} {
		if ceiling != nil && *ceiling < 0 {
			return nil, errors.New("attest: a CEM ceiling is negative")
		}
	}
	counts := StableMapCounts(document)
	if issues := ceilings.exceeded(counts); len(issues) != 0 {
		return nil, fmt.Errorf("attest: the map exceeds its ceilings: %v", issues)
	}
	var statement map[string]any
	decoder := json.NewDecoder(bytes.NewReader(v2))
	decoder.UseNumber()
	if err := decoder.Decode(&statement); err != nil {
		return nil, fmt.Errorf("attest: re-read cem/v2 statement: %w", err)
	}
	predicate := statement["predicate"].(map[string]any)
	predicate["counts"] = map[string]any{
		"total": number(counts.Total), "supported": number(counts.Supported),
		"unknown": number(counts.Unknown), "mechanical": number(counts.Mechanical),
	}
	applied := map[string]any{}
	if ceilings.MaxUnknown != nil {
		applied["maxUnknown"] = number(*ceilings.MaxUnknown)
	}
	if ceilings.MaxMechanical != nil {
		applied["maxMechanical"] = number(*ceilings.MaxMechanical)
	}
	predicate["ceilings"] = applied
	if paired != nil {
		predicate["pairedRange"] = map[string]any{
			"name": paired.Name, "baseRevision": paired.BaseRevision,
			"headRevision": paired.HeadRevision, "evidenceSha256": paired.EvidenceSha256,
		}
	}
	statement["predicateType"] = CEMPredicateTypeV3
	return canonicalMarshal(statement)
}

type cemV3Counts struct {
	Total      int64 `json:"total"`
	Supported  int64 `json:"supported"`
	Unknown    int64 `json:"unknown"`
	Mechanical int64 `json:"mechanical"`
}

type cemV3Ceilings struct {
	MaxUnknown    *int64 `json:"maxUnknown,omitempty"`
	MaxMechanical *int64 `json:"maxMechanical,omitempty"`
}

type cemV3Predicate struct {
	BaseRevision string        `json:"baseRevision"`
	PatchSha256  string        `json:"patchSha256"`
	Target       cemV2Target   `json:"target"`
	Counts       cemV3Counts   `json:"counts"`
	Ceilings     cemV3Ceilings `json:"ceilings"`
	PairedRange  *struct {
		Name           string `json:"name"`
		BaseRevision   string `json:"baseRevision"`
		HeadRevision   string `json:"headRevision"`
		EvidenceSha256 string `json:"evidenceSha256"`
	} `json:"pairedRange,omitempty"`
}

// parseCEMStatementV3 reads a verified CEMPredicateTypeV3 payload: the closed
// predicate of CEMStatementV3 (unknown members refused at every level, counts
// and ceilings required), then the cem/v2 member rules for the rest. It
// refuses counts that do not sum to total, a negative count or ceiling, and
// counts that exceed a signed ceiling.
func parseCEMStatementV3(payload []byte) (CEMVerification, error) {
	var raw struct {
		Predicate jsontext.Value `json:"predicate"`
	}
	if err := jsonv2.Unmarshal(payload, &raw); err != nil {
		return CEMVerification{}, fmt.Errorf("attest: parse CEM statement: %w", err)
	}
	var closed cemV3Predicate
	if err := jsonv2.Unmarshal(raw.Predicate, &closed, jsonv2.RejectUnknownMembers(true)); err != nil {
		return CEMVerification{}, fmt.Errorf("attest: cem/v3 predicate is not closed: %w", err)
	}
	var members struct {
		Counts   jsontext.Value `json:"counts"`
		Ceilings jsontext.Value `json:"ceilings"`
	}
	if err := jsonv2.Unmarshal(raw.Predicate, &members); err != nil || members.Counts == nil || members.Ceilings == nil {
		return CEMVerification{}, errors.New("attest: cem/v3 predicate needs counts and ceilings")
	}
	if err := requireCountMembers(members.Counts); err != nil {
		return CEMVerification{}, err
	}
	// The v2 rules apply to the predicate without the two v3 members.
	var v2 map[string]any
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&v2); err != nil {
		return CEMVerification{}, fmt.Errorf("attest: parse CEM statement: %w", err)
	}
	predicate, ok := v2["predicate"].(map[string]any)
	if !ok {
		return CEMVerification{}, errors.New("attest: cem/v3 predicate is not an object")
	}
	delete(predicate, "counts")
	delete(predicate, "ceilings")
	delete(predicate, "pairedRange")
	stripped, err := json.Marshal(v2)
	if err != nil {
		return CEMVerification{}, fmt.Errorf("attest: parse CEM statement: %w", err)
	}
	claim, err := parseCEMStatementV2(stripped)
	if err != nil {
		return CEMVerification{}, err
	}
	counts := CEMCounts{closed.Counts.Total, closed.Counts.Supported, closed.Counts.Unknown, closed.Counts.Mechanical}
	if counts.Total < 0 || counts.Supported < 0 || counts.Unknown < 0 || counts.Mechanical < 0 ||
		counts.Supported+counts.Unknown+counts.Mechanical != counts.Total {
		return CEMVerification{}, errors.New("attest: cem/v3 counts are negative or do not sum to total")
	}
	ceilings := CEMCeilings{MaxUnknown: closed.Ceilings.MaxUnknown, MaxMechanical: closed.Ceilings.MaxMechanical}
	for _, ceiling := range []*int64{ceilings.MaxUnknown, ceilings.MaxMechanical} {
		if ceiling != nil && *ceiling < 0 {
			return CEMVerification{}, errors.New("attest: cem/v3 ceiling is negative")
		}
	}
	if issues := ceilings.exceeded(counts); len(issues) != 0 {
		return CEMVerification{}, fmt.Errorf("attest: cem/v3 counts exceed the signed ceilings: %v", issues)
	}
	claim.V2.Counts, claim.V2.Ceilings = &counts, &ceilings
	if pin := closed.PairedRange; pin != nil {
		if !wire.ValidPairedRangeName(pin.Name) || !wire.IsGitOid(pin.BaseRevision) || !wire.IsGitOid(pin.HeadRevision) ||
			pin.BaseRevision == pin.HeadRevision || !wire.IsSha256(pin.EvidenceSha256) {
			return CEMVerification{}, errors.New("attest: cem/v3 pairedRange needs a valid name, two distinct full commit IDs and a SHA-256 evidence digest")
		}
		claim.V2.Paired = &CEMPairedRange{pin.Name, pin.BaseRevision, pin.HeadRevision, pin.EvidenceSha256}
	}
	return claim, nil
}

// requireCountMembers requires all four count members, so an absent count is
// never read as zero.
func requireCountMembers(value jsontext.Value) error {
	var present map[string]jsontext.Value
	if err := jsonv2.Unmarshal(value, &present); err != nil {
		return errors.New("attest: cem/v3 counts is not an object")
	}
	for _, name := range []string{"total", "supported", "unknown", "mechanical"} {
		if _, ok := present[name]; !ok {
			return fmt.Errorf("attest: cem/v3 counts lacks %s", name)
		}
	}
	return nil
}

// checkCEMV3Counts compares the supplied map's counts with the signed counts,
// after checkCEMV2 has matched the map bytes to the subject.
func checkCEMV3Counts(claim CEMVerification, check CEMCheck) (CEMVerification, error) {
	if check.Map == nil || claim.V2 == nil || claim.V2.Counts == nil {
		return claim, nil
	}
	document, err := wire.ParseStable(check.Map)
	if err != nil {
		return CEMVerification{}, fmt.Errorf("attest: attested CEM bytes are not a cem/1.0 stable map: %v", err)
	}
	if got := StableMapCounts(document); got != *claim.V2.Counts {
		return CEMVerification{}, fmt.Errorf("%w: the map has %+v, the statement signs %+v", ErrCEMCountsMismatch, got, *claim.V2.Counts)
	}
	return claim, nil
}
