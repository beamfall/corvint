package releasecandidate

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strings"
)

// The stable-readiness record (SRR-V1, proposed under decision 0420) binds one
// verified Core candidate to operator-supplied gate, platform, compliance,
// external and policy evidence digests before any tag or publication. It
// never runs a gate: missing evidence is recorded as NOT_RUN, never PASS.
const (
	readinessProfile     = "corvint-stable-readiness-record/1"
	readinessDecision    = "0420"
	pinnedToolchain      = "go1.27.1"
	firstStableCandidate = "1.0.0-rc.1"
)

var (
	decisionPattern     = regexp.MustCompile(`^[0-9]{4}$`)
	storeReleasePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	goToolchainProbe    = probeLocalToolchain
)

type ReadinessRecord struct {
	Profile       string                 `json:"profile"`
	Identity      ReadinessIdentity      `json:"identity"`
	Core          ReadinessCore          `json:"core"`
	StoreRelease  *ReadinessStoreRelease `json:"storeRelease"`
	Vulnerability ReadinessVulnerability `json:"vulnerability"`
	Rows          []ReadinessRow         `json:"rows"`
}

type ReadinessIdentity struct {
	Version     string `json:"version"`
	Tag         string `json:"tag"`
	BuildNumber string `json:"buildNumber"`
	GoVersion   string `json:"goVersion"`
	Commit      string `json:"commit"`
	Tree        string `json:"tree"`
}

type ReadinessCore struct {
	CandidateProfile string  `json:"candidateProfile"`
	ChecksumsSHA256  string  `json:"checksumsSha256"`
	Archives         []Asset `json:"archives"`
	Reproducibility  string  `json:"reproducibility"`
}

type ReadinessStoreRelease struct {
	ReleaseID       string `json:"releaseId"`
	CandidateSHA256 string `json:"candidateSha256"`
}

type ReadinessVulnerability struct {
	Decision          string `json:"decision"`
	RequireDirectives int    `json:"requireDirectives"`
	Toolchain         string `json:"toolchain"`
	Status            string `json:"status"`
}

type ReadinessRow struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	SHA256   string `json:"sha256"`
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

// ReadinessEvidence is one operator-supplied row: a status, the evidence file
// whose digest is recorded, and the accepting decision or reason.
type ReadinessEvidence struct {
	Status   string
	Path     string
	Decision string
	Reason   string
}

type ReadinessOptions struct {
	CandidateDirectory   string
	SourceRoot           string
	StoreReleaseID       string
	StoreCandidateSHA256 string
	Evidence             map[string]ReadinessEvidence
}

// readinessRule is one catalogue row. A fixed row is written by the builder
// and must match exactly; any other row takes operator evidence or records
// missing. Only platform rows admit FALLBACK.
type readinessRule struct {
	id       string
	fixed    bool
	fallback bool
	missing  ReadinessRow
}

var readinessCatalogue = []readinessRule{
	operatorRule("gate/full-gate"),
	operatorRule("gate/interop-gate"),
	operatorRule("gate/focused-docs"),
	operatorRule("gate/companion-release"),
	operatorRule("gate/release-checklist-pre-promotion"),
	platformRule("platform/darwin-arm64/lifecycle", ""),
	platformRule("platform/darwin-arm64/host-lifecycle", ""),
	platformRule("platform/linux-amd64/lifecycle", readinessDecision),
	platformRule("platform/linux-amd64/host-lifecycle", readinessDecision),
	operatorRule("compliance/legal-files"),
	operatorRule("external/untouched-repository"),
	operatorRule("policy/rollback-exercise"),
	{id: "policy/signing", missing: ReadinessRow{ID: "policy/signing", Status: "NOT_RUN", Reason: "signing selection not recorded"}},
	fixedRule("policy/native-performance", readinessDecision, "GOC-V0-005: native performance is unmeasured"),
	fixedRule("owner/toolchain-security-review", readinessDecision, "subsequent owner action: compare the recorded toolchain with Go security releases before tagging"),
	fixedRule("owner/tag", "", "subsequent owner action"),
	fixedRule("owner/publication", "", "subsequent owner action"),
	fixedRule("owner/promotion", "", "subsequent owner action"),
}

func operatorRule(id string) readinessRule {
	return readinessRule{id: id, missing: ReadinessRow{ID: id, Status: "NOT_RUN", Reason: "evidence not supplied"}}
}

func platformRule(id, decision string) readinessRule {
	return readinessRule{id: id, fallback: true, missing: ReadinessRow{ID: id, Status: "FALLBACK", Decision: decision, Reason: "native lifecycle evidence not supplied (PRS-V1-004)"}}
}

func fixedRule(id, decision, reason string) readinessRule {
	return readinessRule{id: id, fixed: true, missing: ReadinessRow{ID: id, Status: "NOT_RUN", Decision: decision, Reason: reason}}
}

// readinessRules returns the catalogue for one version. Decision 0420 fixes
// 1.0.0-rc.1 signing as "No signing"; every other version records its own
// selection.
func readinessRules(version string) []readinessRule {
	rules := append([]readinessRule(nil), readinessCatalogue...)
	if version != firstStableCandidate {
		return rules
	}
	for index := range rules {
		if rules[index].id == "policy/signing" {
			rules[index] = fixedRule("policy/signing", readinessDecision, "No signing: SHA256SUMS only; publisher identity NOT_VERIFIED")
		}
	}
	return rules
}

// BuildReadinessRecord assembles the canonical record from one verified
// candidate, the source root and operator evidence. It runs no gate and
// writes nothing; the caller owns the output path.
func BuildReadinessRecord(ctx context.Context, options ReadinessOptions) (*ReadinessRecord, []byte, error) {
	candidate, err := VerifyContext(ctx, options.CandidateDirectory)
	if err != nil {
		return nil, nil, err
	}
	identity, core := readinessBinding(candidate)
	source := Options{SourceRoot: options.SourceRoot, Scratch: os.DevNull, Version: identity.Version}
	build, err := sourceBuildNumber(ctx, source, identity.Commit, identity.Tree)
	if err != nil {
		return nil, nil, err
	}
	if build != identity.BuildNumber {
		return nil, nil, fmt.Errorf("source build number %s does not equal candidate build %s", build, identity.BuildNumber)
	}
	vulnerability, err := readinessVulnerability(ctx, source, identity)
	if err != nil {
		return nil, nil, err
	}
	store, err := readinessStore(options.StoreReleaseID, options.StoreCandidateSHA256)
	if err != nil {
		return nil, nil, err
	}
	rows, err := readinessRows(readinessRules(identity.Version), options.Evidence)
	if err != nil {
		return nil, nil, err
	}
	record := &ReadinessRecord{Profile: readinessProfile, Identity: identity, Core: core, StoreRelease: store, Vulnerability: vulnerability, Rows: rows}
	raw, err := canonicalJSON(record)
	if err != nil {
		return nil, nil, err
	}
	return record, raw, nil
}

func readinessBinding(candidate *VerifiedCandidate) (ReadinessIdentity, ReadinessCore) {
	manifest := candidate.Manifest
	source := manifest.Sources[0]
	identity := ReadinessIdentity{Version: manifest.Version, Tag: "v" + manifest.Version, BuildNumber: manifest.BuildNumber, GoVersion: manifest.GoVersion, Commit: source.Commit, Tree: source.Tree}
	archives := []Asset{}
	for _, asset := range manifest.Assets {
		if asset.Role == "core-archive" {
			archives = append(archives, asset)
		}
	}
	core := ReadinessCore{CandidateProfile: manifest.Profile, ChecksumsSHA256: digest(candidate.files["SHA256SUMS"]), Archives: archives, Reproducibility: "PASS"}
	return identity, core
}

// readinessVulnerability applies decision 0420 rule (c): the Core go.mod at
// the bound commit has no require directive and the local toolchain is the
// pinned one. No scanner and no network are consulted.
func readinessVulnerability(ctx context.Context, source Options, identity ReadinessIdentity) (ReadinessVulnerability, error) {
	goMod, err := runSourceGit(ctx, source, "show", identity.Commit+":go.mod")
	if err != nil {
		return ReadinessVulnerability{}, fmt.Errorf("read Core go.mod at %s", identity.Commit)
	}
	toolchain, err := goToolchainProbe(ctx, source.SourceRoot)
	if err != nil {
		return ReadinessVulnerability{}, fmt.Errorf("probe local Go toolchain: %w", err)
	}
	requires := requireDirectives(goMod)
	return ReadinessVulnerability{Decision: readinessDecision, RequireDirectives: requires, Toolchain: toolchain, Status: vulnerabilityStatus(requires, toolchain, identity.GoVersion)}, nil
}

func vulnerabilityStatus(requires int, toolchain, candidateToolchain string) string {
	if requires == 0 && toolchain == pinnedToolchain && candidateToolchain == pinnedToolchain {
		return "PASS"
	}
	return "FAIL"
}

// requireDirectives counts single-line and block require directives. Space,
// tab and carriage return separate words, as in the go.mod lexer.
func requireDirectives(goMod string) int {
	count := 0
	for _, line := range strings.Split(goMod, "\n") {
		code, _, _ := strings.Cut(line, "//")
		words := strings.FieldsFunc(code, func(r rune) bool { return r == ' ' || r == '\t' || r == '\r' || r == '(' })
		if len(words) > 0 && words[0] == "require" {
			count++
		}
	}
	return count
}

func probeLocalToolchain(ctx context.Context, directory string) (string, error) {
	command := exec.CommandContext(ctx, "go", "env", "GOVERSION")
	command.Dir = directory
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	out, err := command.Output()
	return strings.TrimSpace(string(out)), err
}

func readinessStore(releaseID, candidateSHA256 string) (*ReadinessStoreRelease, error) {
	if releaseID == "" && candidateSHA256 == "" {
		return nil, nil
	}
	if !storeReleasePattern.MatchString(releaseID) || !digestPattern.MatchString(candidateSHA256) {
		return nil, fmt.Errorf("store release binding needs both a release id and a candidateSha256")
	}
	return &ReadinessStoreRelease{ReleaseID: releaseID, CandidateSHA256: candidateSHA256}, nil
}

func readinessRows(rules []readinessRule, evidence map[string]ReadinessEvidence) ([]ReadinessRow, error) {
	byID := map[string]readinessRule{}
	for _, rule := range rules {
		byID[rule.id] = rule
	}
	for id := range evidence {
		if rule, known := byID[id]; !known || rule.fixed {
			return nil, fmt.Errorf("readiness row %s does not take operator evidence", id)
		}
	}
	rows := make([]ReadinessRow, 0, len(rules))
	for _, rule := range rules {
		row, err := readinessRow(rule, evidence)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func readinessRow(rule readinessRule, evidence map[string]ReadinessEvidence) (ReadinessRow, error) {
	supplied, present := evidence[rule.id]
	if !present {
		return rule.missing, nil
	}
	row := ReadinessRow{ID: rule.id, Status: supplied.Status, Decision: supplied.Decision, Reason: supplied.Reason}
	if supplied.Path != "" {
		raw, err := readRegular(supplied.Path, maxInputBytes)
		if err != nil {
			return ReadinessRow{}, err
		}
		row.SHA256 = digest(raw)
	}
	return row, validateReadinessRow(rule, row)
}

// validateReadinessRow is the status grammar: PASS and FAIL carry an evidence
// digest; NOT_RUN carries none and names its decision or reason; FALLBACK is
// admitted only on platform rows and also names its decision or reason. A
// platform row is never NOT_RUN, and a platform row whose default names a
// decision keeps that decision while it falls back (SRR-V1-007).
func validateReadinessRow(rule readinessRule, row ReadinessRow) error {
	explained := row.Decision != "" || row.Reason != ""
	valid := map[string]bool{
		"PASS":     digestPattern.MatchString(row.SHA256),
		"FAIL":     digestPattern.MatchString(row.SHA256),
		"NOT_RUN":  row.SHA256 == "" && explained && !rule.fallback,
		"FALLBACK": rule.fallback && explained && (row.SHA256 == "" || digestPattern.MatchString(row.SHA256)) && (rule.missing.Decision == "" || row.Decision == rule.missing.Decision),
	}[row.Status]
	if !valid || row.ID != rule.id || (row.Decision != "" && !decisionPattern.MatchString(row.Decision)) {
		return fmt.Errorf("readiness row %s has an invalid %s shape", rule.id, row.Status)
	}
	if rule.fixed && row != rule.missing {
		return fmt.Errorf("readiness row %s differs from its fixed value", rule.id)
	}
	return nil
}

// VerifyReadinessRecord refuses a malformed or non-canonical record, re-binds
// it to the verified candidate and recomputes every evidence digest from the
// operator-named files (row id to path).
func VerifyReadinessRecord(ctx context.Context, raw []byte, candidateDirectory string, evidence map[string]string) (*ReadinessRecord, error) {
	var record ReadinessRecord
	if err := decodeClosed(raw, &record); err != nil {
		return nil, fmt.Errorf("readiness record: %w", err)
	}
	canonical, err := canonicalJSON(record)
	if err != nil || !bytes.Equal(canonical, raw) || record.Profile != readinessProfile {
		return nil, fmt.Errorf("readiness record is not canonical %s", readinessProfile)
	}
	candidate, err := VerifyContext(ctx, candidateDirectory)
	if err != nil {
		return nil, err
	}
	identity, core := readinessBinding(candidate)
	if record.Identity != identity || !reflect.DeepEqual(record.Core, core) {
		return nil, fmt.Errorf("readiness record does not bind the verified candidate")
	}
	if err := verifyReadinessClaims(record); err != nil {
		return nil, err
	}
	if err := verifyReadinessEvidence(record.Rows, evidence); err != nil {
		return nil, err
	}
	return &record, nil
}

func verifyReadinessClaims(record ReadinessRecord) error {
	if record.StoreRelease != nil {
		store, err := readinessStore(record.StoreRelease.ReleaseID, record.StoreRelease.CandidateSHA256)
		if err != nil {
			return err
		}
		if store == nil {
			return fmt.Errorf("store release binding is empty; record null instead")
		}
	}
	vulnerability := record.Vulnerability
	if vulnerability.Decision != readinessDecision || vulnerability.RequireDirectives < 0 || vulnerability.Status != vulnerabilityStatus(vulnerability.RequireDirectives, vulnerability.Toolchain, record.Identity.GoVersion) {
		return fmt.Errorf("readiness vulnerability rule is inconsistent")
	}
	rules := readinessRules(record.Identity.Version)
	if len(record.Rows) != len(rules) {
		return fmt.Errorf("readiness row catalogue is not closed")
	}
	for index, rule := range rules {
		if err := validateReadinessRow(rule, record.Rows[index]); err != nil {
			return err
		}
	}
	return nil
}

func verifyReadinessEvidence(rows []ReadinessRow, evidence map[string]string) error {
	digested := map[string]string{}
	for _, row := range rows {
		if row.SHA256 != "" {
			digested[row.ID] = row.SHA256
		}
	}
	if len(evidence) != len(digested) {
		return fmt.Errorf("readiness evidence files do not match the digest-bearing rows")
	}
	for id, want := range digested {
		raw, err := readRegular(evidence[id], maxInputBytes)
		if err != nil || digest(raw) != want {
			return fmt.Errorf("readiness row %s evidence digest does not reproduce", id)
		}
	}
	return nil
}
