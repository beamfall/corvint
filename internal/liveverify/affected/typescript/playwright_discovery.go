package typescript

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"sort"
	"strings"

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
}

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
	receipt, ok := decodePlaywrightDiscovery(raw)
	if !ok {
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
	receipt, ok := decodePlaywrightDiscovery(raw)
	if !ok {
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

// decodePlaywrightDiscovery accepts only a bounded, closed, canonically encoded, valid receipt.
func decodePlaywrightDiscovery(raw []byte) (PlaywrightDiscovery, bool) {
	var receipt PlaywrightDiscovery
	if len(raw) > PlaywrightDiscoveryMaxBytes || json.Unmarshal(raw, &receipt, json.RejectUnknownMembers(true)) != nil {
		return receipt, false
	}
	canonical, err := json.Marshal(receipt, json.Deterministic(true))
	return receipt, err == nil && bytes.Equal(bytes.TrimSuffix(raw, []byte{'\n'}), canonical) && validPlaywrightDiscovery(receipt)
}

func validPlaywrightDiscovery(receipt PlaywrightDiscovery) bool {
	if receipt.Profile != "playwright-discovery/0" || receipt.Units == nil || !affected.ValidRelativePath(receipt.Config.Path) {
		return false
	}
	if !playwrightHex(receipt.Revision, 40) && !playwrightHex(receipt.Revision, 64) {
		return false
	}
	if !playwrightHex(receipt.Config.SHA256, 64) || !strings.HasPrefix(receipt.SourceDigest, "playwright-sources:sha256:") || !playwrightHex(strings.TrimPrefix(receipt.SourceDigest, "playwright-sources:sha256:"), 64) {
		return false
	}
	for index, unit := range receipt.Units {
		if unit.Project == "" || strings.ContainsAny(unit.Project, "\x00\r\n") || !affected.ValidRelativePath(unit.Test) || !hasSourceExtension(unit.Test) {
			return false
		}
		if index > 0 && !discoveryUnitLess(receipt.Units[index-1], unit) {
			return false
		}
	}
	return true
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
