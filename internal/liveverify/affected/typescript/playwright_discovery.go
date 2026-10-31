package typescript

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"slices"
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
	// UnverifiedUnits, present only when nonempty, lists the units the listing omitted that the
	// producer kept because the project's grep or grepInvert may exclude every case of the file but
	// the listing cannot prove it: the pattern is not statically evaluable, or the exclusion rests on
	// tags the listing reports only in part (TJAA-V0-021). Each is also in Units.
	UnverifiedUnits []PlaywrightDiscoveryUnit `json:"unverifiedUnits,omitzero"`
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
	return SelectPlaywrightWithBase(root, configPath, revision, dirty, receipt, nil)
}

// SelectPlaywrightWithBase is SelectPlaywright whose advisory narrowing also reports changed
// symbols against the content that before reads at the comparison base; a nil reader reports every
// owned dirty path's symbols as UNRESOLVED (TJAA-V0-033).
func SelectPlaywrightWithBase(root, configPath, revision string, dirty []string, receipt []byte, before PlaywrightBaseReader) (PlaywrightPlan, error) {
	return SelectPlaywrightWithEnvironment(root, configPath, revision, dirty, receipt, before, nil)
}

// SelectPlaywrightWithEnvironment is SelectPlaywrightWithBase that also records the caller-named
// environment variables and resolves a computed executablePath that reads one (AFP-V0-382).
func SelectPlaywrightWithEnvironment(root, configPath, revision string, dirty []string, receipt []byte, before PlaywrightBaseReader, environment []PlaywrightEnvironmentInput) (PlaywrightPlan, error) {
	plan, draft, err := selectPlaywrightStaticDraft(root, configPath, dirty, environment)
	if err != nil {
		return PlaywrightPlan{}, err
	}
	var unverified, listed []PlaywrightDiscoveryUnit
	var mismatch string
	plan.Discovery, unverified, listed, mismatch = reconcilePlaywrightDiscovery(plan, revision, receipt)
	if plan.Discovery.State == "MATCHED" && len(unverified) != 0 {
		// The units are the config's file membership either way; what is missing is the listing's
		// cross-check, so a run of such a unit may find no case. It stays visible and names the
		// units (TJAA-V0-021, AFP-V0-411). The receipt is bound to the current sources, so the first
		// unit's tag closure is walked again to name where it stops (AFP-V0-419).
		detail := fmt.Sprintf("%d discovery unit(s) omitted by the listing under a project grep/grepInvert exclusion the listing cannot verify: %s", len(unverified), playwrightUnitList(unverified))
		if _, stop := newPlaywrightTagSources(root, configPath).exactOrStop(configPath, unverified[0].Test); stop != "" {
			detail += "; the tag closure of " + unverified[0].Test + " stops because " + stop
		}
		plan.Unknown = canonicalPlaywrightUnknowns(append(plan.Unknown, PlaywrightUnknown{
			Axis: PlaywrightAxisExecution, Reason: PlaywrightUnknownDiscoveryOmission, Detail: detail,
		}))
	}
	if plan.Discovery.State != "MATCHED" {
		plan.Scope = affected.ScopeUnknown
		plan.Fallback = PlaywrightFallbackFullSuite
		plan.FallbackArgv = []string{"npx", "playwright", "test", "--config=" + configPath}
		plan.Selected = []PlaywrightSelection{}
		plan.Excluded = []PlaywrightExclusion{}
		detail := plan.Discovery.State
		if detail == "MISSING" {
			// AFP-V0-388: name the absent input, because it alone empties the selection.
			detail = "MISSING: no --playwright-discovery receipt was given, so no file selection is proven; write one with `corvint affected discovery --playwright-config PATH --playwright-list FILE`"
		}
		if mismatch != "" {
			detail = mismatch
		}
		plan.Unknown = canonicalPlaywrightUnknowns(append(plan.Unknown, selectionUnknown("playwright:discovery-unproven", detail)))
	}
	if plan.Fallback == PlaywrightFallbackFullSuite {
		narrowing := finishPlaywrightNarrowing(draft, plan.Discovery, listed)
		narrowing.Symbols = playwrightChangedSymbols(root, plan.Dirty, before)
		plan.Narrowed = &narrowing
	}
	applyPlaywrightPackageManager(&plan, root)
	return plan, nil
}

// reconcilePlaywrightDiscovery returns the discovery summary and, for a decoded receipt, its
// unverified units; for a receipt bound to this revision, config and sources it also returns the
// listed units, which advisory narrowing compares per project (TJAA-V0-032).
func reconcilePlaywrightDiscovery(plan PlaywrightPlan, revision string, raw []byte) (PlaywrightDiscoverySummary, []PlaywrightDiscoveryUnit, []PlaywrightDiscoveryUnit, string) {
	summary := PlaywrightDiscoverySummary{State: "MISSING", OnlyInReceipt: []PlaywrightDiscoveryUnit{}, OnlyInStatic: []PlaywrightDiscoveryUnit{}}
	if len(raw) == 0 {
		return summary, nil, nil, ""
	}
	sum := sha256.Sum256(raw)
	summary.InputSHA256 = hex.EncodeToString(sum[:])
	summary.State = "MALFORMED"
	receipt, malformed := decodePlaywrightDiscovery(raw)
	if malformed != nil {
		summary.Reason, summary.Detail = malformed.reason, malformed.detail
		return summary, nil, nil, ""
	}
	summary.State = "BINDING_MISMATCH"
	if mismatch := playwrightBindingMismatch(receipt, plan, revision); mismatch != "" {
		return summary, nil, nil, mismatch
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
		// A graph reachability unknown leaves membership static, so the receipt can still match it
		// (TJAA-V0-106).
		if strings.HasPrefix(unknown.Reason, "playwright:") && unknown.Reason != PlaywrightUnknownDynamicSource && unknown.Reason != PlaywrightUnknownOutOfSuiteSource && unknown.Axis == PlaywrightAxisSelection {
			return summary, receipt.UnverifiedUnits, receipt.Units, ""
		}
	}
	summary.State = "UNIVERSE_MISMATCH"
	if len(summary.OnlyInStatic) != 0 || len(summary.OnlyInReceipt) != 0 {
		return summary, receipt.UnverifiedUnits, receipt.Units, ""
	}
	summary.State = "MATCHED"
	return summary, receipt.UnverifiedUnits, receipt.Units, ""
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
	if receipt.UnverifiedUnits != nil && len(receipt.UnverifiedUnits) == 0 {
		return "unverifiedUnits: omit the member when empty"
	}
	for index, unit := range receipt.UnverifiedUnits {
		if _, found := slices.BinarySearchFunc(receipt.Units, unit, compareDiscoveryUnits); !found {
			return fmt.Sprintf("unverifiedUnits[%d]: not one of units", index)
		}
		if index > 0 && !discoveryUnitLess(receipt.UnverifiedUnits[index-1], unit) {
			return fmt.Sprintf("unverifiedUnits[%d]: not strictly after unverifiedUnits[%d] by (project, test)", index, index-1)
		}
	}
	return ""
}

func compareDiscoveryUnits(a, b PlaywrightDiscoveryUnit) int {
	if a.Project != b.Project {
		return strings.Compare(a.Project, b.Project)
	}
	return strings.Compare(a.Test, b.Test)
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

// playwrightBindingMismatch names each binding of receipt that differs from this plan (the
// revision, the config path, the config digest and the source digest) and says how to
// regenerate the receipt, or returns "" when every binding holds (AFP-V0-412).
func playwrightBindingMismatch(receipt PlaywrightDiscovery, plan PlaywrightPlan, revision string) string {
	differs := []string{}
	if receipt.Revision != revision {
		differs = append(differs, fmt.Sprintf("revision (receipt %s, plan %s)", playwrightShortDigest(receipt.Revision), playwrightShortDigest(revision)))
	}
	if receipt.Config.Path != plan.Config.Path {
		differs = append(differs, fmt.Sprintf("config path (receipt %q, plan %q)", receipt.Config.Path, plan.Config.Path))
	}
	if receipt.Config.SHA256 != plan.Config.SHA256 {
		differs = append(differs, fmt.Sprintf("config digest (receipt %s, plan %s)", playwrightShortDigest(receipt.Config.SHA256), playwrightShortDigest(plan.Config.SHA256)))
	}
	if receipt.SourceDigest != plan.SourceDigest {
		differs = append(differs, fmt.Sprintf("source digest (receipt %s, plan %s)", playwrightShortDigest(strings.TrimPrefix(receipt.SourceDigest, "playwright-sources:sha256:")), playwrightShortDigest(strings.TrimPrefix(plan.SourceDigest, "playwright-sources:sha256:"))))
	}
	if len(differs) == 0 {
		return ""
	}
	return "BINDING_MISMATCH: the discovery receipt does not match this plan in its " + strings.Join(differs, ", ") + ", so it no longer proves the file universe; regenerate it at this revision with `corvint affected discovery --playwright-config " + plan.Config.Path + " --playwright-list FILE` from a fresh `playwright test --list --reporter=json`"
}

// playwrightShortDigest returns the first 12 characters of an object id or digest.
func playwrightShortDigest(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	return value
}

// playwrightUnitListMax bounds how many units a detail names before counting the rest.
const playwrightUnitListMax = 10

// playwrightUnitList names the first playwrightUnitListMax units as project "P" test PATH and
// counts the rest (AFP-V0-411).
func playwrightUnitList(units []PlaywrightDiscoveryUnit) string {
	named := make([]string, 0, min(len(units), playwrightUnitListMax))
	for _, unit := range units[:min(len(units), playwrightUnitListMax)] {
		named = append(named, fmt.Sprintf("project %q test %s", unit.Project, unit.Test))
	}
	if rest := len(units) - len(named); rest > 0 {
		named = append(named, fmt.Sprintf("and %d more", rest))
	}
	return strings.Join(named, ", ")
}
