package typescript

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

const PlaywrightDiscoveryMaxBytes = 4 << 20

type PlaywrightDiscoveryUnit struct {
	Project string `json:"project"`
	Test    string `json:"test"`
}

// PlaywrightDiscovery is caller-declared complete unfiltered listing evidence,
// not execution attestation. The source digest includes dirty source bytes.
type PlaywrightDiscovery struct {
	Config       PlaywrightConfigIdentity  `json:"config"`
	Profile      string                    `json:"profile"`
	Revision     string                    `json:"revision"`
	SourceDigest string                    `json:"sourceDigest"`
	Units        []PlaywrightDiscoveryUnit `json:"units"`
}

type PlaywrightDiscoverySummary struct {
	InputSHA256   string                    `json:"inputSha256"`
	OnlyInReceipt []PlaywrightDiscoveryUnit `json:"onlyInReceipt"`
	OnlyInStatic  []PlaywrightDiscoveryUnit `json:"onlyInStatic"`
	State         string                    `json:"state"`
	// Reason and Detail are present only when State is MALFORMED: Reason is one of
	// PlaywrightMalformedDecode, PlaywrightMalformedNonCanonical or PlaywrightMalformedField and
	// Detail is one bounded line naming the failure (TJAA-V0-020).
	Reason string `json:"reason,omitzero"`
	Detail string `json:"detail,omitzero"`
}

// MALFORMED discovery reasons (TJAA-V0-020).
const (
	PlaywrightMalformedDecode       = "DECODE_FAILED"
	PlaywrightMalformedNonCanonical = "NON_CANONICAL_BYTES"
	PlaywrightMalformedField        = "INVALID_FIELD"
)

// playwrightMalformed is why a discovery receipt is MALFORMED.
type playwrightMalformed struct{ reason, detail string }

const playwrightMalformedDetailMax = 240

// SelectPlaywright reconciles static project/file candidates with a bounded,
// canonical discovery receipt before exposing runnable file selections.
func SelectPlaywright(root, configPath, revision string, dirty []string, receipt []byte) (PlaywrightPlan, error) {
	plan, err := selectPlaywrightStatic(root, configPath, dirty)
	if err != nil {
		return PlaywrightPlan{}, err
	}
	plan.Discovery = reconcilePlaywrightDiscovery(plan, revision, receipt)
	if plan.Discovery.State != "MATCHED" {
		plan.Scope = affected.ScopeUnknown
		plan.Fallback = PlaywrightFallbackFullSuite
		plan.FallbackArgv = []string{"npx", "playwright", "test", "--config=" + configPath}
		plan.Selected = []PlaywrightSelection{}
		plan.Excluded = []PlaywrightExclusion{}
		plan.Unknown = canonicalPlaywrightUnknowns(append(plan.Unknown, selectionUnknown("playwright:discovery-unproven", plan.Discovery.State)))
	}
	return plan, nil
}

func reconcilePlaywrightDiscovery(plan PlaywrightPlan, revision string, raw []byte) PlaywrightDiscoverySummary {
	summary := PlaywrightDiscoverySummary{State: "MISSING", OnlyInReceipt: []PlaywrightDiscoveryUnit{}, OnlyInStatic: []PlaywrightDiscoveryUnit{}}
	if len(raw) == 0 {
		return summary
	}
	sum := sha256.Sum256(raw)
	summary.InputSHA256 = hex.EncodeToString(sum[:])
	summary.State = "MALFORMED"
	receipt, malformed := decodePlaywrightDiscovery(raw)
	if malformed != nil {
		summary.Reason, summary.Detail = malformed.reason, malformed.detail
		return summary
	}
	summary.State = "BINDING_MISMATCH"
	if receipt.Revision != revision || receipt.Config != plan.Config || receipt.SourceDigest != plan.SourceDigest {
		return summary
	}
	static := map[PlaywrightDiscoveryUnit]bool{}
	for _, unit := range plan.Selected {
		static[PlaywrightDiscoveryUnit{Project: unit.Project, Test: unit.Test}] = true
	}
	for _, unit := range plan.Excluded {
		static[PlaywrightDiscoveryUnit{Project: unit.Project, Test: unit.Test}] = true
	}
	for _, unit := range receipt.Units {
		if !static[unit] {
			summary.OnlyInReceipt = append(summary.OnlyInReceipt, unit)
		}
		delete(static, unit)
	}
	for unit := range static {
		summary.OnlyInStatic = append(summary.OnlyInStatic, unit)
	}
	sort.Slice(summary.OnlyInStatic, func(i, j int) bool { return discoveryUnitLess(summary.OnlyInStatic[i], summary.OnlyInStatic[j]) })
	summary.State = "STATIC_UNRESOLVED"
	for _, unknown := range plan.Unknown {
		if strings.HasPrefix(unknown.Reason, "playwright:") && unknown.Reason != PlaywrightUnknownDynamicSource && unknown.Axis == PlaywrightAxisSelection {
			return summary
		}
	}
	summary.State = "UNIVERSE_MISMATCH"
	if len(summary.OnlyInStatic) != 0 || len(summary.OnlyInReceipt) != 0 {
		return summary
	}
	summary.State = "MATCHED"
	return summary
}

// VerifyPlaywrightDiscovery decodes a canonical receipt and binds it to revision, the config bytes
// and the observed sources. The state is MISSING, MALFORMED, BINDING_MISMATCH or MATCHED; the
// receipt's units are returned only when it is MATCHED (AFU-V1-019).
func VerifyPlaywrightDiscovery(root, configPath, revision string, raw []byte) ([]PlaywrightDiscoveryUnit, string) {
	if len(raw) == 0 {
		return nil, "MISSING"
	}
	receipt, malformed := decodePlaywrightDiscovery(raw)
	if malformed != nil {
		return nil, "MALFORMED"
	}
	configBytes, err := affected.ReadSource(root, configPath)
	if err != nil {
		return nil, "BINDING_MISMATCH"
	}
	configSum := sha256.Sum256(configBytes)
	sourceDigest, err := ObservePlaywrightSources(root, configPath)
	config := PlaywrightConfigIdentity{Path: configPath, SHA256: hex.EncodeToString(configSum[:])}
	if err != nil || receipt.Revision != revision || receipt.Config != config || receipt.SourceDigest != sourceDigest {
		return nil, "BINDING_MISMATCH"
	}
	return receipt.Units, "MATCHED"
}

// decodePlaywrightDiscovery accepts only a bounded, closed, canonically encoded, valid receipt and
// otherwise names the first failure.
func decodePlaywrightDiscovery(raw []byte) (PlaywrightDiscovery, *playwrightMalformed) {
	var receipt PlaywrightDiscovery
	if len(raw) > PlaywrightDiscoveryMaxBytes {
		return receipt, malformedDiscovery(PlaywrightMalformedDecode, fmt.Sprintf("receipt is over the %d-byte bound", PlaywrightDiscoveryMaxBytes))
	}
	if err := json.Unmarshal(raw, &receipt, json.RejectUnknownMembers(true)); err != nil {
		detail := err.Error()
		if looksLikePlaywrightListing(raw) {
			detail = "input is a Playwright JSON report, not a playwright-discovery/0 receipt; convert it with `corvint affected discovery`: " + detail
		}
		return receipt, malformedDiscovery(PlaywrightMalformedDecode, detail)
	}
	canonical, err := json.Marshal(receipt, json.Deterministic(true))
	if err != nil {
		return receipt, malformedDiscovery(PlaywrightMalformedDecode, err.Error())
	}
	if body := bytes.TrimSuffix(raw, []byte{'\n'}); !bytes.Equal(body, canonical) {
		offset := 0
		for offset < len(body) && offset < len(canonical) && body[offset] == canonical[offset] {
			offset++
		}
		return receipt, malformedDiscovery(PlaywrightMalformedNonCanonical, fmt.Sprintf("bytes differ from the canonical encoding at byte %d (all members present, sorted keys, no whitespace, at most one trailing LF)", offset))
	}
	if field := invalidPlaywrightDiscoveryField(receipt); field != "" {
		return receipt, malformedDiscovery(PlaywrightMalformedField, field)
	}
	return receipt, nil
}

// looksLikePlaywrightListing reports a JSON object carrying Playwright reporter members.
func looksLikePlaywrightListing(raw []byte) bool {
	var members map[string]jsontext.Value
	if json.Unmarshal(raw, &members) != nil {
		return false
	}
	_, suites := members["suites"]
	return suites
}

func malformedDiscovery(reason, detail string) *playwrightMalformed {
	detail = strings.Join(strings.Fields(detail), " ")
	if len(detail) > playwrightMalformedDetailMax {
		cut := playwrightMalformedDetailMax
		for cut > 0 && !utf8.RuneStart(detail[cut]) {
			cut--
		}
		detail = detail[:cut] + "..."
	}
	return &playwrightMalformed{reason: reason, detail: detail}
}

// invalidPlaywrightDiscoveryField names the first field that breaks the receipt contract, or "".
func invalidPlaywrightDiscoveryField(receipt PlaywrightDiscovery) string {
	switch {
	case receipt.Profile != "playwright-discovery/0":
		return "profile: want playwright-discovery/0"
	case receipt.Units == nil:
		return "units: want an array"
	case !affected.ValidRelativePath(receipt.Config.Path):
		return "config.path: want a canonical repository-relative path"
	case !playwrightHex(receipt.Revision, 40) && !playwrightHex(receipt.Revision, 64):
		return "revision: want 40 or 64 lowercase hex digits"
	case !playwrightHex(receipt.Config.SHA256, 64):
		return "config.sha256: want 64 lowercase hex digits"
	case !strings.HasPrefix(receipt.SourceDigest, "playwright-sources:sha256:") || !playwrightHex(strings.TrimPrefix(receipt.SourceDigest, "playwright-sources:sha256:"), 64):
		return "sourceDigest: want playwright-sources:sha256: and 64 lowercase hex digits"
	}
	for index, unit := range receipt.Units {
		if strings.ContainsAny(unit.Project, "\x00\r\n") {
			return fmt.Sprintf("units[%d].project: contains NUL, CR or LF", index)
		}
		if !affected.ValidRelativePath(unit.Test) || !hasSourceExtension(unit.Test) {
			return fmt.Sprintf("units[%d].test: want a canonical repository-relative source path", index)
		}
		if index > 0 && !discoveryUnitLess(receipt.Units[index-1], unit) {
			return fmt.Sprintf("units[%d]: not strictly after units[%d] by (project, test)", index, index-1)
		}
	}
	return ""
}

func discoveryUnitLess(a, b PlaywrightDiscoveryUnit) bool {
	if a.Project != b.Project {
		return a.Project < b.Project
	}
	return a.Test < b.Test
}

func playwrightHex(value string, size int) bool {
	if len(value) != size || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
