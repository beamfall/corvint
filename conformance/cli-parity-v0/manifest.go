package main

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	manifestProfile         = "corvint-cli-parity/0"
	maximumStdin            = 64 * 1024
	repositoryQueryPrompt   = "session expiry device revocation enforcement"
	queryFloorPrompt        = "how do I bake sourdough bread at high altitude"
	queryVersionTokenPrompt = "enforce session revocation v18"
)

type expectationProduction struct {
	Producer          string   `json:"producer"`
	Method            string   `json:"method"`
	SourceRevision    string   `json:"sourceRevision"`
	SourceTree        string   `json:"sourceTree"`
	SourceSHA256      string   `json:"sourceSha256"`
	RuntimeVersion    string   `json:"runtimeVersion"`
	CandidateUsed     bool     `json:"candidateUsed"`
	ExpectationFields []string `json:"expectationFields"`
}

type commandInventoryEntry struct {
	Command string `json:"command"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
}

type qualificationMatrix struct {
	SHA1Fixtures              string `json:"sha1Fixtures"`
	SHA256Fixtures            string `json:"sha256Fixtures"`
	OwnedProcessGroupCleanup  string `json:"ownedProcessGroupCleanup"`
	DetachedDescendantCleanup string `json:"detachedDescendantCleanup"`
	FullGPKV0005Matrix        string `json:"fullGpkV0005Matrix"`
	NonPOSIX                  string `json:"nonPosix"`
}

type fixtureSetup struct {
	Kind       string `json:"kind"`
	Path       string `json:"path"`
	Mode       uint32 `json:"mode,omitempty"`
	DataBase64 string `json:"dataBase64,omitempty"`
}

type acceptedDivergence struct {
	OracleOnlyCreatedPath string `json:"oracleOnlyCreatedPath"`
	CandidateMutation     string `json:"candidateMutation"`
	Reason                string `json:"reason"`
}

// knownDivergence declares one adjudicated `python-defect` (`GPK-V0-033`) whose
// observable is stdout or stderr rather than a repository mutation: the candidate satisfies
// ratified spec text that the oracle contradicts, so the two runtimes are
// *expected* to disagree and that disagreement is recorded rather than repaired.
//
// It is the narrowest possible admission. Every rewrite names the exact candidate
// bytes and the exact oracle bytes they stand in for; the candidate bytes are
// authored from the cited clause, never transcribed from the candidate; each must
// occur exactly once in its declared stream; and once every rewrite is applied
// both streams must equal the oracle's byte-for-byte. A candidate that stops
// emitting the spec-required form fails on the uniqueness check, and any other
// divergence fails on the byte comparison. `validKnownDivergence` pins the
// admitted case, register entry, clause, and the shape of every rewrite.
type knownDivergence struct {
	Register       string              `json:"register"`
	Clause         string              `json:"clause"`
	Rewrites       []divergenceRewrite `json:"rewrites"`
	StderrRewrites []divergenceRewrite `json:"stderrRewrites,omitempty"`
	Reason         string              `json:"reason"`
}

// divergenceRewrite is one exact byte substitution: Candidate is what the clause
// requires the candidate to emit, Oracle is what the oracle emits in its place.
type divergenceRewrite struct {
	Candidate string `json:"candidate"`
	Oracle    string `json:"oracle"`
}

// retiredCase withdraws one frozen oracle case from execution under decision
// 0308: the candidate carries the single name `corvint` in its state paths and
// tooling terms, while the case's frozen argv, fixture and expectation address
// the oracle's `.atlas/` and `.context-atlas/` paths or the retired product-name
// token. The frozen expectation stays in the manifest unchanged; the runner
// neither executes nor scores the case and reports it as `RETIRED`, never as a
// PASS. `retiredCases` pins the closed set so the declaration cannot become a
// blanket exclusion.
type retiredCase struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

type locationNormalization struct {
	Fields []string `json:"fields"`
	Kind   string   `json:"kind"`
	Reason string   `json:"reason"`
}

type structuralComparison struct {
	Fields []string `json:"fields"`
	Kind   string   `json:"kind"`
	Reason string   `json:"reason"`
}

type parityCase struct {
	ID                       string                 `json:"id"`
	Support                  string                 `json:"support"`
	ExpectationSource        string                 `json:"expectationSource"`
	Argv                     []string               `json:"argv"`
	Repository               string                 `json:"repository"`
	Setup                    []fixtureSetup         `json:"setup,omitempty"`
	CommitRevision           string                 `json:"commitRevision"`
	TreeRevision             string                 `json:"treeRevision"`
	FixtureSHA256            string                 `json:"fixtureSha256"`
	ExitStatus               int                    `json:"exitStatus"`
	Mutation                 string                 `json:"mutation"`
	AcceptedDivergence       *acceptedDivergence    `json:"acceptedDivergence,omitempty"`
	KnownDivergence          *knownDivergence       `json:"knownDivergence,omitempty"`
	ExclusionCountDivergence *knownDivergence       `json:"exclusionCountDivergence,omitempty"`
	IdentityRenameDivergence *knownDivergence       `json:"identityRenameDivergence,omitempty"`
	Retired                  *retiredCase           `json:"retired,omitempty"`
	LocationNormalization    *locationNormalization `json:"locationNormalization,omitempty"`
	StructuralComparison     *structuralComparison  `json:"structuralComparison,omitempty"`
	StatusBeforeSHA256       string                 `json:"statusBeforeSha256"`
	StatusAfterSHA256        string                 `json:"statusAfterSha256"`
	RepositoryBeforeSHA256   string                 `json:"repositoryBeforeSha256"`
	RepositoryAfterSHA256    string                 `json:"repositoryAfterSha256"`
	FileModesBeforeSHA256    string                 `json:"fileModesBeforeSha256"`
	FileModesAfterSHA256     string                 `json:"fileModesAfterSha256"`
	StdoutSHA256             string                 `json:"stdoutSha256"`
	StdoutBytes              int                    `json:"stdoutBytes"`
	StderrSHA256             string                 `json:"stderrSha256"`
	StderrBytes              int                    `json:"stderrBytes"`
	StdinSHA256              string                 `json:"stdinSha256"`
	StdinBase64              string                 `json:"stdinBase64"`
	TimeoutMilliseconds      int                    `json:"timeoutMilliseconds"`
	TimedOut                 bool                   `json:"timedOut"`
	ProcessStarted           bool                   `json:"processStarted"`
	WaitCompleted            bool                   `json:"waitCompleted"`
	PipesDrained             bool                   `json:"pipesDrained"`
	OwnedProcessGroupCleanup bool                   `json:"ownedProcessGroupCleanup"`
	CleanupScope             string                 `json:"cleanupScope"`
}

type refusalCase struct {
	ID                  string         `json:"id"`
	Support             string         `json:"support"`
	Authority           string         `json:"authority"`
	ExpectedType        string         `json:"expectedType"`
	Argv                []string       `json:"argv"`
	Repository          string         `json:"repository"`
	Setup               []fixtureSetup `json:"setup,omitempty"`
	TimeoutMilliseconds int            `json:"timeoutMilliseconds"`
	Reason              string         `json:"reason"`
	Retired             *retiredCase   `json:"retired,omitempty"`
}

type parityManifest struct {
	Profile               string                  `json:"profile"`
	OracleCommand         string                  `json:"oracleCommand"`
	CandidateCommand      string                  `json:"candidateCommand"`
	Workers               int                     `json:"workers"`
	SelectedEnvironment   map[string]string       `json:"selectedEnvironment"`
	RuntimeAuthority      bool                    `json:"runtimeAuthority"`
	ExpectationProduction expectationProduction   `json:"expectationProduction"`
	Qualification         qualificationMatrix     `json:"qualification"`
	Commands              []commandInventoryEntry `json:"commands"`
	Cases                 []parityCase            `json:"cases"`
	Refusals              []refusalCase           `json:"refusals"`
}

func loadManifest(raw []byte) (parityManifest, error) {
	var manifest parityManifest
	if err := decodeStrictJSON(raw, &manifest); err != nil {
		return manifest, err
	}
	if err := validateManifest(manifest); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func validateManifest(manifest parityManifest) error {
	if manifest.Profile != manifestProfile {
		return fmt.Errorf("profile: got %q", manifest.Profile)
	}
	if manifest.OracleCommand == "" || manifest.CandidateCommand == "" || manifest.OracleCommand == manifest.CandidateCommand {
		return fmt.Errorf("oracleCommand and candidateCommand must be distinct")
	}
	if manifest.Workers < 1 {
		return fmt.Errorf("workers must be positive")
	}
	if manifest.RuntimeAuthority {
		return fmt.Errorf("runtimeAuthority must be false")
	}
	wantEnvironment := map[string]string{
		"CORVINT_CACHE_DIR": "external-private-temp", "GIT_ATTR_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": "disabled", "GIT_CONFIG_NOSYSTEM": "1",
		"GIT_NO_LAZY_FETCH": "1", "GIT_NO_REPLACE_OBJECTS": "1", "GIT_OPTIONAL_LOCKS": "0",
		"GIT_TERMINAL_PROMPT": "0", "HOME": "external-private-temp", "LANG": "C", "LC_ALL": "C",
		"PYTHONDONTWRITEBYTECODE": "1", "PYTHONHASHSEED": "0", "PYTHONPATH": "immutable-oracle-src",
		"TEMP": "external-private-temp", "TMP": "external-private-temp", "TMPDIR": "external-private-temp", "TZ": "UTC",
	}
	if len(manifest.SelectedEnvironment) != len(wantEnvironment) {
		return fmt.Errorf("selectedEnvironment is not closed")
	}
	for key, value := range wantEnvironment {
		if manifest.SelectedEnvironment[key] != value {
			return fmt.Errorf("selectedEnvironment %s=%q", key, manifest.SelectedEnvironment[key])
		}
	}
	production := manifest.ExpectationProduction
	if production.Producer != "python-oracle" || production.Method != "cli-parity-v0 capture" || production.CandidateUsed {
		return fmt.Errorf("expectation production is not oracle-only")
	}
	if !lowerHex(production.SourceRevision, 40) || !lowerHex(production.SourceTree, 40) || !lowerHex(production.SourceSHA256, 64) || production.RuntimeVersion == "" {
		return fmt.Errorf("expectation production identity is incomplete")
	}
	wantFields := []string{"exitStatus", "fileModesAfterSha256", "fileModesBeforeSha256", "repositoryAfterSha256", "repositoryBeforeSha256", "statusAfterSha256", "statusBeforeSha256", "stderrSha256", "stdoutSha256"}
	if strings.Join(production.ExpectationFields, "\x00") != strings.Join(wantFields, "\x00") {
		return fmt.Errorf("expectationFields is not closed")
	}
	if err := validateInventory(manifest.Commands); err != nil {
		return err
	}
	if manifest.Qualification.SHA1Fixtures != "PASS" || manifest.Qualification.SHA256Fixtures != "NOT_RUN" || manifest.Qualification.OwnedProcessGroupCleanup != "PASS" || manifest.Qualification.DetachedDescendantCleanup != "NOT_RUN" || manifest.Qualification.FullGPKV0005Matrix != "PARTIAL" || manifest.Qualification.NonPOSIX != "NOT_RUN" {
		return fmt.Errorf("qualification matrix is not explicit")
	}
	seen := make(map[string]struct{})
	previous := ""
	retired, renamed, retiredRefusals := 0, 0, 0
	for _, item := range manifest.Cases {
		if item.ID <= previous {
			return fmt.Errorf("cases must be sorted by id")
		}
		previous = item.ID
		if err := validateParityCase(item); err != nil {
			return fmt.Errorf("case %s: %w", item.ID, err)
		}
		seen[item.ID] = struct{}{}
		if item.Retired != nil {
			retired++
		}
		if item.IdentityRenameDivergence != nil {
			renamed++
		}
	}
	if retired != len(retiredCases) || renamed != len(identityRenameDivergenceCases) {
		return fmt.Errorf("decision 0308 declarations are not closed: retired=%d identity-renames=%d", retired, renamed)
	}
	previous = ""
	for _, item := range manifest.Refusals {
		if item.ID <= previous {
			return fmt.Errorf("refusals must be sorted by id")
		}
		previous = item.ID
		if _, exists := seen[item.ID]; exists {
			return fmt.Errorf("duplicate case id %s", item.ID)
		}
		if err := validateRefusalCase(item); err != nil {
			return fmt.Errorf("refusal %s: %w", item.ID, err)
		}
		seen[item.ID] = struct{}{}
		if item.Retired != nil {
			retiredRefusals++
		}
	}
	if retiredRefusals != len(retiredRefusalCases) {
		return fmt.Errorf("decision 0308 declarations are not closed: retired-refusals=%d", retiredRefusals)
	}
	if len(manifest.Cases) < 2 || len(manifest.Refusals) < 3 {
		return fmt.Errorf("seed matrix is incomplete")
	}
	return nil
}

func validateInventory(entries []commandInventoryEntry) error {
	// PARTIAL names a command with at least one case retired under decision
	// 0308; its executed cases still replay byte-exactly.
	want := map[string]string{
		"adopt": "PASS", "cem": "PARTIAL", "eval": "PARTIAL", "feature": "PASS",
		"harness": "PASS", "impact": "PASS", "init": "PASS", "lrf": "PARTIAL",
		"migrate-traces": "PARTIAL", "ocm": "PARTIAL", "query": "PARTIAL", "record": "PARTIAL",
	}
	if len(entries) != len(want) {
		return fmt.Errorf("command inventory entries=%d want=%d", len(entries), len(want))
	}
	for index, entry := range entries {
		if index > 0 && entries[index-1].Command >= entry.Command {
			return fmt.Errorf("command inventory must be sorted")
		}
		if want[entry.Command] != entry.Status || entry.Reason == "" {
			return fmt.Errorf("command inventory %s=%s", entry.Command, entry.Status)
		}
		delete(want, entry.Command)
	}
	if len(want) != 0 {
		return fmt.Errorf("command inventory is incomplete")
	}
	return nil
}

func validateParityCase(item parityCase) error {
	if item.ID == "" || item.Support != "parity" || item.ExpectationSource != "python-oracle" || item.Repository == "" || len(item.Argv) == 0 {
		return fmt.Errorf("identity/support is invalid")
	}
	if item.Argv[0] != "query" && item.Argv[0] != "impact" && item.Argv[0] != "migrate-traces" && item.Argv[0] != "record" && item.Argv[0] != "lrf" && item.Argv[0] != "eval" && item.Argv[0] != "ocm" && item.Argv[0] != "harness" && item.Argv[0] != "feature" && item.Argv[0] != "init" && item.Argv[0] != "adopt" && item.Argv[0] != "cem" {
		return fmt.Errorf("command %q is outside the seed", item.Argv[0])
	}
	if item.Argv[0] == "query" && !validQueryArgv(item) {
		return fmt.Errorf("query argv is outside the ported profiles")
	}
	if item.Argv[0] == "impact" && !validImpactArgv(item) {
		return fmt.Errorf("impact argv is outside the ported seed")
	}
	if item.Argv[0] == "migrate-traces" && !validMigrateTracesArgv(item.Argv) {
		return fmt.Errorf("migrate-traces argv is outside the ported seed")
	}
	if item.Argv[0] == "record" && !validRecordArgv(item.Argv) {
		return fmt.Errorf("record argv is outside the ported seed")
	}
	if item.Argv[0] == "lrf" && !validLRFArgv(item.Argv) {
		return fmt.Errorf("lrf argv is outside the ported seed")
	}
	if item.Argv[0] == "eval" && !validEvalArgv(item.Argv) {
		return fmt.Errorf("eval argv is outside the ported seed")
	}
	if item.Argv[0] == "ocm" && !validOCMArgv(item.Argv) {
		return fmt.Errorf("ocm argv is outside the ported read-only seed")
	}
	if item.Argv[0] == "harness" && !validHarnessArgv(item.Argv) {
		return fmt.Errorf("harness argv is outside the ported seed")
	}
	if item.Argv[0] == "cem" && !validCEMArgv(item.Argv) {
		return fmt.Errorf("cem argv is outside the ported seed")
	}
	if item.Argv[0] == "feature" && !validFeatureArgv(item.Argv) {
		return fmt.Errorf("feature argv is outside the ported seed")
	}
	if (item.Argv[0] == "init" || item.Argv[0] == "adopt") && !validActivationArgv(item.Argv) {
		return fmt.Errorf("activation argv is outside the ported seed")
	}
	if !lowerHex(item.CommitRevision, 40) || !lowerHex(item.TreeRevision, 40) || !lowerHex(item.FixtureSHA256, 64) {
		return fmt.Errorf("fixture identity is invalid")
	}
	for name, value := range map[string]string{
		"statusBeforeSha256": item.StatusBeforeSHA256, "statusAfterSha256": item.StatusAfterSHA256,
		"repositoryBeforeSha256": item.RepositoryBeforeSHA256, "repositoryAfterSha256": item.RepositoryAfterSHA256,
		"fileModesBeforeSha256": item.FileModesBeforeSHA256, "fileModesAfterSha256": item.FileModesAfterSHA256,
		"stdoutSha256": item.StdoutSHA256, "stderrSha256": item.StderrSHA256, "stdinSha256": item.StdinSHA256,
	} {
		if !lowerHex(value, 64) {
			return fmt.Errorf("%s is invalid", name)
		}
	}
	stdin, err := base64.StdEncoding.DecodeString(item.StdinBase64)
	if err != nil || len(stdin) > maximumStdin || sha256Hex(stdin) != item.StdinSHA256 {
		return fmt.Errorf("stdin is invalid")
	}
	if item.StdoutBytes < 0 || item.StderrBytes < 0 || item.TimeoutMilliseconds < 1 {
		return fmt.Errorf("bounds are invalid")
	}
	ocmMapMutation := validOCMMapMutation(item)
	// File MODES may move here, unlike the read-only classes: the publisher
	// rewrites the map owner-private, so a fixture-materialized 0644 map becomes
	// 0600. Git status must still not move, because the map is gitignored.
	if ocmMapMutation && (item.StatusBeforeSHA256 != item.StatusAfterSHA256 ||
		item.RepositoryBeforeSHA256 == item.RepositoryAfterSHA256) {
		return fmt.Errorf("ocm map mutation evidence is invalid")
	}
	cemArtifactMutation := validCEMArtifactMutation(item)
	if cemArtifactMutation && (item.StatusBeforeSHA256 != item.StatusAfterSHA256 ||
		item.RepositoryBeforeSHA256 == item.RepositoryAfterSHA256) {
		return fmt.Errorf("cem artifact mutation evidence is invalid")
	}
	traceStoreMutation := validTraceStoreMutation(item)
	// Two oracle-only classes share one evidence shape: the oracle scaffolds
	// exactly one thing before it rejects, and the candidate rejects first.
	// GPK-V0-008 names both the lock and the owner-private parent directory.
	acceptedOracleOnly := validAcceptedLockDivergence(item) || validAcceptedPrivateParentDivergence(item)
	if traceStoreMutation && (item.StatusBeforeSHA256 != item.StatusAfterSHA256 || item.RepositoryBeforeSHA256 == item.RepositoryAfterSHA256 || item.FileModesBeforeSHA256 == item.FileModesAfterSHA256) {
		return fmt.Errorf("trace-store mutation evidence is invalid")
	}
	if item.AcceptedDivergence != nil && !acceptedOracleOnly {
		return fmt.Errorf("accepted divergence is invalid")
	}
	if item.LocationNormalization != nil && !validLocationNormalization(item) {
		return fmt.Errorf("location normalization is invalid")
	}
	if item.StructuralComparison != nil && !validStructuralComparison(item) {
		return fmt.Errorf("structural comparison is invalid")
	}
	if item.KnownDivergence != nil && !validKnownDivergence(item) {
		return fmt.Errorf("known divergence is invalid")
	}
	if item.ExclusionCountDivergence != nil && !validExclusionCountDivergence(item) {
		return fmt.Errorf("exclusion count divergence is invalid")
	}
	if item.IdentityRenameDivergence != nil && !validIdentityRenameDivergence(item) {
		return fmt.Errorf("identity rename divergence is invalid")
	}
	if item.Retired != nil && !validRetired(item) {
		return fmt.Errorf("retirement is invalid")
	}
	if acceptedOracleOnly && (item.ExitStatus == 0 || item.StatusBeforeSHA256 != item.StatusAfterSHA256 || item.RepositoryBeforeSHA256 == item.RepositoryAfterSHA256 || item.FileModesBeforeSHA256 == item.FileModesAfterSHA256) {
		return fmt.Errorf("accepted oracle-only mutation evidence is invalid")
	}
	if !traceStoreMutation && !acceptedOracleOnly && !ocmMapMutation && !cemArtifactMutation && (item.Mutation != "none" || item.StatusBeforeSHA256 != item.StatusAfterSHA256 || item.RepositoryBeforeSHA256 != item.RepositoryAfterSHA256 || item.FileModesBeforeSHA256 != item.FileModesAfterSHA256) {
		return fmt.Errorf("read-only mutation evidence is invalid")
	}
	if item.TimedOut || !item.ProcessStarted || !item.WaitCompleted || !item.PipesDrained || !item.OwnedProcessGroupCleanup || item.CleanupScope != "owned-process-group" {
		return fmt.Errorf("process evidence is incomplete")
	}
	return validateSetups(item.Setup)
}

func validQueryArgv(item parityCase) bool {
	joined := strings.Join(item.Argv, "\x00")
	switch item.ID {
	case "query-clean-authority-start":
		return joined == strings.Join([]string{"query", "--task", parityPrompt, "--limit", "1"}, "\x00")
	case "query-authority-start-default":
		return joined == "query\x00--task\x00"+parityPrompt
	case "query-authority-start-learned":
		return joined == "query\x00--task\x00"+authorityLearnedPrompt+"\x00--limit\x003"
	case "query-authority-start-limit-50":
		return joined == "query\x00--task\x00"+parityPrompt+"\x00--limit\x0050"
	case "query-authority-start-limit-51":
		return joined == "query\x00--task\x00"+parityPrompt+"\x00--limit\x0051"
	case "query-authority-start-non-ascii":
		return joined == "query\x00--task\x00"+parityPrompt+" — café\x00--limit\x001"
	case "query-repository-agent-tooling":
		return joined == "query\x00--task\x00atlas agent context roadmap"
	case "query-repository-non-ascii":
		return joined == "query\x00--task\x00fix café parser"
	case "query-repository-default":
		return joined == "query\x00--task\x00"+repositoryQueryPrompt
	case "query-repository-limit-1":
		return joined == "query\x00--task\x00"+repositoryQueryPrompt+"\x00--limit\x001"
	case "query-repository-limit-10":
		return joined == "query\x00--task\x00"+repositoryQueryPrompt+"\x00--limit\x0010"
	case "query-repository-limit-50":
		return joined == "query\x00--task\x00"+repositoryQueryPrompt+"\x00--limit\x0050"
	case "query-repository-budget-selection":
		return joined == "query\x00--task\x00"+repositoryQueryPrompt+"\x00--limit\x0010\x00--budget-bytes\x002200"
	case "query-repository-out-of-scope":
		return joined == "query\x00--task\x00"+queryFloorPrompt
	case "query-version-token":
		return joined == "query\x00--task\x00"+queryVersionTokenPrompt
	case "query-repository-empty-task":
		return joined == "query\x00--task\x00"
	case "query-repository-limit-malformed":
		return joined == "query\x00--task\x00"+repositoryQueryPrompt+"\x00--limit\x00ten"
	case "query-repository-limit-zero":
		return joined == "query\x00--task\x00"+repositoryQueryPrompt+"\x00--limit\x000"
	case "query-repository-limit-51":
		return joined == "query\x00--task\x00"+repositoryQueryPrompt+"\x00--limit\x0051"
	case "query-repository-budget-malformed":
		return joined == "query\x00--task\x00"+repositoryQueryPrompt+"\x00--budget-bytes\x00small"
	case "query-repository-budget-1023":
		return joined == "query\x00--task\x00"+repositoryQueryPrompt+"\x00--budget-bytes\x001023"
	case "query-repository-budget-1000001":
		return joined == "query\x00--task\x00"+repositoryQueryPrompt+"\x00--budget-bytes\x001000001"
	case "query-repository-task-oversized":
		return len(item.Argv) == 3 && item.Argv[0] == "query" && item.Argv[1] == "--task" && item.Argv[2] == strings.Repeat("x", 8001)
	case "query-repository-trace-budget-selection":
		return joined == "query\x00--task\x00"+repositoryQueryPrompt+"\x00--limit\x0010\x00--budget-bytes\x002200"
	case "query-repository-trace-matching", "query-repository-trace-mixed-worktree", "query-repository-trace-nonmatching-nonpassed":
		return joined == "query\x00--task\x00"+repositoryQueryPrompt
	}
	return false
}

// validImpactArgv pins the ported `impact` seed. Its explicit `--limit 10`
// equals the command's own inferred default (`impact-ranked-past-limit`'s
// `--limit 3` already discriminates the option itself), so a case dropping
// that unit reproduces the frozen bytes and neither the self-check nor the
// comparison could see it before this allowlist closed the argv.
func validImpactArgv(item parityCase) bool {
	joined := strings.Join(item.Argv, "\x00")
	switch item.ID {
	case "impact-clean-tracked", "impact-self-authored-adr":
		return joined == "impact\x00pkg/sample.go\x00--limit\x0010"
	case "impact-go-root":
		return joined == "impact\x00logger.go"
	case "impact-python-module", "impact-python-nomodule":
		return joined == "impact\x00src/pkg/engine.py\x00--limit\x0010"
	case "impact-python-package":
		return joined == "impact\x00src/pkg/__init__.py\x00--limit\x0010"
	case "impact-ranked-past-limit":
		return joined == "impact\x00pkg/sample.go\x00--limit\x003"
	case "impact-web-component":
		return joined == "impact\x00internal/web/app/src/widget/button.ts\x00--limit\x0010"
	case "impact-web-directory":
		return joined == "impact\x00internal/web/app/src/widget/index.ts\x00--limit\x0010"
	}
	return false
}

func validMigrateTracesArgv(argv []string) bool {
	joined := strings.Join(argv, "\x00")
	if joined == "migrate-traces" || joined == "migrate-traces\x00--dry-run" || joined == "migrate-traces\x00--dry-run\x00--apply" {
		return true
	}
	return len(argv) == 4 && argv[1] == "--apply" && argv[2] == "--plan-digest" && lowerHex(argv[3], 64)
}

func validRecordArgv(argv []string) bool {
	joined := strings.Join(argv, "\x00")
	return joined == "record\x00--task\x00task\x00--changed\x00internal/example/value.go\x00--verify\x00go test ./...\x00--outcome\x00passed" ||
		joined == "record\x00--task\x00task\x00--verify\x00go test ./...\x00--outcome\x00passed" ||
		joined == "record\x00--task\x00task\x00--changed\x00missing.go\x00--verify\x00go test ./...\x00--outcome\x00passed"
}

// validOCMMapMutation admits the mutation class the three OCM write actions
// produce: the map file changes, the map is gitignored so Git status does not,
// and no file mode moves. It is deliberately separate from the trace-store
// class, which has different evidence.
func validOCMMapMutation(item parityCase) bool {
	if item.ExitStatus != 0 || item.Mutation != "ocm-map" || item.Argv[0] != "ocm" {
		return false
	}
	switch item.Argv[1] {
	case "prepare", "link", "mark":
		return item.Repository == "ocm-linked" || item.Repository == "ocm-prepared"
	}
	return false
}

// validCEMArtifactMutation admits the mutation class the four CEM write actions
// produce. Every artifact they touch sits outside the index — the map and the
// begin output under the gitignored .atlas/, the derived patch cache inside the
// Git directory — so the repository content moves while Git status must not.
// File MODES may move for the same two reasons the OCM class allows: a created
// file adds an entry, and the publisher rewrites an existing map owner-private.
func validCEMArtifactMutation(item parityCase) bool {
	if item.ExitStatus != 0 || item.Mutation != "cem-artifact" || item.Argv[0] != "cem" {
		return false
	}
	switch item.Argv[1] {
	case "begin", "prepare", "cite", "mark":
		return item.Repository == "ocm-linked" || item.Repository == "cem-unprepared"
	}
	return false
}

func validTraceStoreMutation(item parityCase) bool {
	if item.ExitStatus != 0 || item.Mutation != "trace-store" {
		return false
	}
	if item.Argv[0] == "migrate-traces" {
		return len(item.Argv) == 4 && item.Argv[1] == "--apply"
	}
	return item.ID == "record-create-trace" && strings.Join(item.Argv, "\x00") == "record\x00--task\x00task\x00--changed\x00internal/example/value.go\x00--verify\x00go test ./...\x00--outcome\x00passed" && validLocationNormalization(item)
}

func validLocationNormalization(item parityCase) bool {
	if item.LocationNormalization == nil {
		return false
	}
	normalization := item.LocationNormalization
	if normalization.Kind != "repository-root-prefix" || normalization.Reason == "" || item.AcceptedDivergence != nil {
		return false
	}
	joined := strings.Join(item.Argv, "\x00")
	fields := strings.Join(normalization.Fields, "\x00")
	switch item.ID {
	case "ocm-link-obligation", "ocm-mark-obligation":
		return item.Mutation == "ocm-map" && fields == "stdout.map"
	case "cem-begin-map", "cem-cite-evidence", "cem-cite-lines-span", "cem-mark-hunk-unknown", "cem-mark-hunk-mechanical":
		return item.Mutation == "cem-artifact" && fields == "stdout.map"
	case "cem-prepare-create", "cem-prepare-resume":
		// prepare is the one CEM write action that reports two resolved paths:
		// the map it created or resumed, and the derived patch cache.
		return item.Mutation == "cem-artifact" && fields == "stdout.map\x00stdout.patch"
	case "ocm-prepare-resume":
		return item.Mutation == "none" && fields == "stdout.cem\x00stdout.map"
	case "cem-status-bound":
		return joined == "cem\x00status\x00--map\x00.atlas/change.cem.json\x00--expected-base\x00"+ocmLinkedBase+"\x00--target\x00"+ocmLinkedTarget &&
			item.Mutation == "none" && fields == "stdout.map"
	case "record-create-trace":
		return joined == "record\x00--task\x00task\x00--changed\x00internal/example/value.go\x00--verify\x00go test ./...\x00--outcome\x00passed" &&
			item.Mutation == "trace-store" && fields == "stdout.store"
	case "eval-frozen-corpus":
		return joined == "eval" && item.Mutation == "none" &&
			fields == "stdout.evaluation.promotion.frozen_corpus.path"
	}
	return false
}

// withheldAuthorityUncertaintyMark is the reason text `CF-V0-032` requires the
// derived uncertainty entry to carry. Pinning it here means a rewrite cannot
// declare away an arbitrary uncertainty array — only the one the withheld
// authority produces.
const withheldAuthorityUncertaintyMark = "no caller-independent revision is available"

// packetBytesPrefix names the one member the runner reconciles arithmetically
// instead of by declaration. `packet_bytes` is the receipt's own canonical byte
// count, so no clause can author its value and transcribing it would take an
// expected byte from the candidate. The runner instead requires the two runtimes'
// counts to differ by exactly the length the declared rewrites account for, plus
// the width of the count's own member, which is part of the packet it counts.
const packetBytesPrefix = `"packet_bytes":`

// validKnownDivergence closes the stdout-divergence admission to the eight
// adjudicated `python-defect` entries that have one (DR-0007, DR-0008, DR-0009,
// DR-0015, DR-0016, DR-0017, DR-0025, DR-0035, over nine cases) plus the two
// intentional Go extensions DR-0039 and DR-0040 (one case each), and to exactly
// the byte regions each one's clause produces. Nothing else may be declared away, and
// every rewrite's candidate side is the form the clause names rather than bytes
// read off the candidate.
func validKnownDivergence(item parityCase) bool {
	divergence := item.KnownDivergence
	if divergence == nil || divergence.Reason == "" || item.Mutation != "none" {
		return false
	}
	if item.AcceptedDivergence != nil || item.LocationNormalization != nil || item.StructuralComparison != nil {
		return false
	}
	switch item.ID {
	case "impact-self-authored-adr":
		return validWithheldAuthorityDivergence(item)
	case "harness-user-prompt-out-of-scope":
		return validRelevanceFloorDivergence(item)
	case "query-repository-out-of-scope":
		return validRelevanceFloorDivergence(item)
	case "impact-ranked-past-limit":
		return validAdmittedUniverseDivergence(item)
	case "query-repository-limit-1":
		return validQueryAdmittedUniverseDivergence(item)
	case "query-repository-trace-matching":
		return validTraceRevisionDisclosureDivergence(item)
	case "impact-go-root":
		return validRootPackageDivergence(item)
	case "query-repository-task-oversized":
		return validTaskBoundDivergence(item)
	case "query-version-token":
		return validVersionTokenDivergence(item)
	case "cem-mark-invalid-reason":
		return validMarkReasonDivergence(item)
	case "cem-invalid-subcommand":
		return validCEMActionDivergence(item)
	}
	return false
}

// cemActionChoices is the `cem` action list the candidate's invalid-subcommand
// refusal enumerates: the oracle's seven actions plus `cover` (`TCQ-V0-051`,
// decision 0347), `discriminate` (`TCQ-V0-055`, decision 0353), `anchor` and
// `provenance` (`FPK-V0-037`, decision 0355), and `export` (`RCB-V0-001`),
// which the candidate adds last in that order.
const cemActionChoices = "'begin', 'prepare', 'cite', 'mark', 'verify', 'status', 'report', 'cover', 'discriminate', 'anchor', 'provenance', 'export'"

// oracleCEMActionChoices is the retired oracle's seven-action list.
const oracleCEMActionChoices = "'begin', 'prepare', 'cite', 'mark', 'verify', 'status', 'report'"

// validCEMActionDivergence pins `DR-0040` to the `cem` invalid-subcommand
// refusal: both runtimes refuse with exit status 2 and an empty stdout, and the
// single stderr rewrite replaces the candidate's eleven-action list with the
// oracle's seven.
func validCEMActionDivergence(item parityCase) bool {
	divergence := item.KnownDivergence
	if item.Argv[0] != "cem" || divergence.Register != "DR-0040" || divergence.Clause != "TCQ-V0-051" {
		return false
	}
	if len(divergence.Rewrites) != 0 || len(divergence.StderrRewrites) != 1 {
		return false
	}
	rewrite := divergence.StderrRewrites[0]
	return rewrite.Candidate == cemActionChoices && rewrite.Oracle == oracleCEMActionChoices
}

// markReasonChoices is the `cem mark --reason` choice list `CEM-SM-001`
// (decision 0338) requires the candidate to enumerate: the five `cem/0.2`
// reasons plus the four structural reasons, in the order the refusal names them.
const markReasonChoices = "'conflicting-evidence', 'formatter-only', 'import-reorder', 'insufficient-evidence', 'line-ending-only', 'move', 'no-evidence', 'rename', 'whitespace-only'"

// oracleMarkReasonChoices is the retired oracle's five-reason choice list.
const oracleMarkReasonChoices = "'conflicting-evidence', 'insufficient-evidence', 'line-ending-only', 'no-evidence', 'whitespace-only'"

// validMarkReasonDivergence pins `DR-0039` to the `cem mark` invalid-reason
// refusal: both runtimes refuse with exit status 2 and an empty stdout, and the
// single stderr rewrite replaces the candidate's nine-reason choice list with
// the oracle's five. The argv must name an unknown reason so that neither side
// reaches the map.
func validMarkReasonDivergence(item parityCase) bool {
	divergence := item.KnownDivergence
	if item.Argv[0] != "cem" || divergence.Register != "DR-0039" || divergence.Clause != "CEM-SM-001" {
		return false
	}
	if len(divergence.Rewrites) != 0 || len(divergence.StderrRewrites) != 1 {
		return false
	}
	rewrite := divergence.StderrRewrites[0]
	return rewrite.Candidate == markReasonChoices && rewrite.Oracle == oracleMarkReasonChoices
}

// exclusionCountPrefix and exclusionCountSuffix bound the one receipt region
// `DR-0023` may move: the decimal value of `exclusions.count`, closed by the
// comma before `samples` or `samples_omitted`, so a rewrite cannot match a
// longer number that merely begins with the declared digits.
const (
	exclusionCountPrefix = `"exclusions":{"count":`
	exclusionCountSuffix = `,`
)

// exclusionCountDivergenceCases is the closed set of receipts `DR-0023`
// (decision 0166) moves: every receipt-bearing case over a fixture whose tree
// tracks a path no index suffix admits. The set is measured, not open, so a
// receipt that starts or stops counting such a path must be declared here.
var exclusionCountDivergenceCases = map[string]bool{
	"harness-file-change": true, "harness-session-start-compact-rehydrated": true, "harness-user-prompt": true,
	"query-authority-start-default": true, "query-authority-start-learned": true, "query-authority-start-limit-50": true,
	"query-authority-start-non-ascii": true, "query-clean-authority-start": true,
	"query-repository-agent-tooling": true, "query-repository-budget-selection": true, "query-repository-default": true,
	"query-repository-limit-1": true, "query-repository-limit-10": true, "query-repository-limit-50": true,
	"query-repository-non-ascii": true, "query-repository-trace-budget-selection": true,
	"query-repository-trace-matching": true, "query-repository-trace-mixed-worktree": true,
	"query-repository-trace-nonmatching-nonpassed": true, "query-version-token": true,
}

// validExclusionCountDivergence pins `DR-0023` to exactly one stdout region:
// the value of `exclusions.count`, which `GPK-V0-063` requires to include the
// tracked paths the suffix allow-list never reads and the oracle leaves out.
// It is a separate declaration from `knownDivergence` because three of its
// cases already carry one, and it moves no byte those declarations pin. The
// candidate value is enumerated from the fixture tree (decision 0166), so the
// validator admits only a larger integer than the oracle's, never a new member.
func validExclusionCountDivergence(item parityCase) bool {
	divergence := item.ExclusionCountDivergence
	if divergence == nil || divergence.Reason == "" || item.Mutation != "none" || !exclusionCountDivergenceCases[item.ID] {
		return false
	}
	if item.AcceptedDivergence != nil || item.LocationNormalization != nil || item.StructuralComparison != nil {
		return false
	}
	if divergence.Register != "DR-0023" || divergence.Clause != "GPK-V0-063" {
		return false
	}
	if len(divergence.Rewrites) != 1 || len(divergence.StderrRewrites) != 0 {
		return false
	}
	candidate, candidateOK := exclusionCountValue(divergence.Rewrites[0].Candidate)
	oracle, oracleOK := exclusionCountValue(divergence.Rewrites[0].Oracle)
	return candidateOK && oracleOK && candidate > oracle
}

// retiredCases is the closed set decision 0308 withdraws from execution: the
// cases whose frozen argv, fixture or expectation address the oracle's
// `.atlas/` or `.context-atlas/` state paths, plus the one whose frozen task
// carries the retired product name as a tooling term.
var retiredCases = map[string]bool{
	"cem-cite-empty-span": true, "cem-cite-evidence": true, "cem-cite-invalid-line-range": true,
	"cem-cite-line-range-out-of-range": true, "cem-cite-lines-span": true, "cem-cite-missing-evidence": true,
	"cem-cite-span-out-of-range": true, "cem-mark-hunk-mechanical": true, "cem-mark-hunk-unknown": true,
	"cem-mark-unknown-hunk": true, "cem-prepare-create": true, "cem-prepare-resume": true,
	"cem-status-bound": true, "cem-status-unresolvable-expected-base": true, "cem-verify-bound": true,
	"cem-verify-policy-limit": true, "eval-frozen-corpus": true, "migrate-traces-apply-legacy": true,
	"migrate-traces-dry-run-legacy": true, "ocm-link-obligation": true, "ocm-mark-obligation": true,
	"ocm-prepare-resume": true, "ocm-verify-cem-binding": true, "ocm-verify-linked": true,
	"query-repository-agent-tooling": true, "query-repository-trace-budget-selection": true,
	"query-repository-trace-matching": true, "query-repository-trace-nonmatching-nonpassed": true,
	"record-create-trace": true,
}

// validRetired admits a retirement only for a pinned case, only under decision
// 0308, and only with a stated reason.
func validRetired(item parityCase) bool {
	return item.Retired != nil && item.Retired.Decision == "0308" && item.Retired.Reason != "" && retiredCases[item.ID]
}

// retiredRefusalCases is the closed set of refusal items decision 0308 withdraws:
// the frozen argv or setup addresses the oracle's `.atlas/` maps or its
// `.context-atlas/traces` store, so the candidate never reaches the typed
// refusal the item certifies.
var retiredRefusalCases = map[string]bool{
	"lrf-ocm-python-claim-refusal": true, "query-present-trace-store-refusal": true,
}

func validRetiredRefusal(item refusalCase) bool {
	return item.Retired != nil && item.Retired.Decision == "0308" && item.Retired.Reason != "" && retiredRefusalCases[item.ID]
}

// identityRenameDivergenceCases pins `DR-0038` to the ten harness receipts whose
// only divergence from the frozen oracle bytes is the event profile name.
var identityRenameDivergenceCases = map[string]bool{
	"harness-file-change": true, "harness-post-tool": true, "harness-session-end-outcome": true,
	"harness-session-start": true, "harness-session-start-compact-clean": true,
	"harness-session-start-compact-rehydrated": true, "harness-stop": true, "harness-user-prompt": true,
	"harness-user-prompt-non-ascii": true, "harness-user-prompt-out-of-scope": true,
}

const (
	identityRenameCandidateProfile = `"profile":"corvint-harness-event/0"`
	identityRenameOracleProfile    = `"profile":"atlas-harness-event/0"`
)

// validIdentityRenameDivergence pins `DR-0038` (decision 0308, `CRB-DEC-002`) to
// exactly one stdout region: the harness event profile member, whose value
// carries the product name. The rewrite lies outside the counted packet, so it
// moves no `packet_bytes` and coexists with the DR-0008 and DR-0023
// declarations three of these cases already carry.
func validIdentityRenameDivergence(item parityCase) bool {
	divergence := item.IdentityRenameDivergence
	if divergence == nil || divergence.Reason == "" || item.Mutation != "none" || !identityRenameDivergenceCases[item.ID] {
		return false
	}
	if item.Retired != nil || item.AcceptedDivergence != nil || item.LocationNormalization != nil || item.StructuralComparison != nil {
		return false
	}
	if divergence.Register != "DR-0038" || divergence.Clause != "CRB-DEC-002" {
		return false
	}
	if len(divergence.Rewrites) != 1 || len(divergence.StderrRewrites) != 0 {
		return false
	}
	return divergence.Rewrites[0].Candidate == identityRenameCandidateProfile && divergence.Rewrites[0].Oracle == identityRenameOracleProfile
}

// exclusionCountValue parses one `"exclusions":{"count":N,` region.
func exclusionCountValue(region string) (int, bool) {
	if !strings.HasPrefix(region, exclusionCountPrefix) || !strings.HasSuffix(region, exclusionCountSuffix) {
		return 0, false
	}
	digits := strings.TrimSuffix(strings.TrimPrefix(region, exclusionCountPrefix), exclusionCountSuffix)
	value, err := strconv.Atoi(digits)
	return value, err == nil && value >= 0 && strconv.Itoa(value) == digits
}

// validVersionTokenDivergence pins `DR-0015` to the standalone `query`
// receipt of a task carrying a `vN` token and to exactly the seven regions the
// version-token filter moves. Each candidate side is the published form
// `GPK-V0-043` requires after decision 0017 -- an inactive abstention, a
// `READY` state, a non-empty `results` array, positive included and requested
// counts, and the GPK-V0-046 syntax-only uncertainty line --
// and each oracle side is the withdrawn form the oracle's filter produces.
// No clause can author a fixture's own results, counts, or verification plan,
// so those are pinned by shape rather than transcribed: the counts must be a
// positive integer against the oracle's zero, the results must open an object
// against the oracle's `[]`, and the verification plan must name the same
// member and actually differ.
func validVersionTokenDivergence(item parityCase) bool {
	divergence := item.KnownDivergence
	if item.Argv[0] != "query" {
		return false
	}
	if divergence.Register != "DR-0015" || divergence.Clause != "GPK-V0-043" {
		return false
	}
	if len(divergence.Rewrites) != 7 {
		return false
	}
	abstention, results, state, verification := divergence.Rewrites[0], divergence.Rewrites[4], divergence.Rewrites[5], divergence.Rewrites[6]
	if abstention.Candidate != `"abstention":{"active":false,"reason":"none"}` ||
		abstention.Oracle != `"abstention":{"active":true,"reason":"no-relevant-candidates"}` {
		return false
	}
	for index, member := range []string{`"included_results":`, `"requested_results":`} {
		if !validAnsweredCountRewrite(divergence.Rewrites[index+1], member) {
			return false
		}
	}
	// GPK-V0-046: every result the candidate answers with here rests on syntax
	// alone, so it counts zero authoritative results, the same zero the oracle
	// reaches by abstaining, and names that condition where the oracle names
	// nothing. The count is no longer a divergence; the uncertainty line is.
	uncertainty := divergence.Rewrites[3]
	if uncertainty.Candidate != `"uncertainty":["all results are syntax matches; no project-owned authority corroborates the task"]` ||
		uncertainty.Oracle != `"uncertainty":[]` {
		return false
	}
	if !strings.HasPrefix(results.Candidate, `"results":[{`) || results.Oracle != `"results":[]` {
		return false
	}
	if state.Candidate != `"state":"READY"` || state.Oracle != `"state":"OUT_OF_SCOPE"` {
		return false
	}
	return strings.HasPrefix(verification.Candidate, `"verification":[`) &&
		strings.HasPrefix(verification.Oracle, `"verification":[`) &&
		verification.Candidate != verification.Oracle
}

// validAnsweredCountRewrite admits one coverage count the candidate publishes
// as a positive integer where the oracle publishes zero.
func validAnsweredCountRewrite(rewrite divergenceRewrite, member string) bool {
	if rewrite.Oracle != member+"0" || !strings.HasPrefix(rewrite.Candidate, member) {
		return false
	}
	value, err := strconv.Atoi(strings.TrimPrefix(rewrite.Candidate, member))
	return err == nil && value > 0
}

// validWithheldAuthorityDivergence pins `DR-0007` to exactly the three byte
// regions `CF-V0-031` / `CF-V0-032` produce on `impact`: the withheld authority
// label, its withheld confidence, and the derived uncertainty entry that
// explains them.
func validWithheldAuthorityDivergence(item parityCase) bool {
	divergence := item.KnownDivergence
	if item.Argv[0] != "impact" {
		return false
	}
	if divergence.Register != "DR-0007" || divergence.Clause != "CF-V0-031" {
		return false
	}
	if len(divergence.Rewrites) != 3 {
		return false
	}
	authority, confidence, uncertainty := divergence.Rewrites[0], divergence.Rewrites[1], divergence.Rewrites[2]
	if authority.Candidate != `"authority":"unverified-contract"` || authority.Oracle != `"authority":"accepted-contract"` {
		return false
	}
	if confidence.Candidate != `"confidence":"low"` || confidence.Oracle != `"confidence":"authoritative"` {
		return false
	}
	return strings.HasPrefix(uncertainty.Candidate, `"uncertainty":["`) &&
		strings.Contains(uncertainty.Candidate, withheldAuthorityUncertaintyMark) &&
		uncertainty.Oracle == `"uncertainty":[]`
}

// relevanceFloorRewrites is the closed set `DR-0008` admits, in the order the
// members appear in the receipt. Each candidate side is the withdrawn form
// `GPK-V0-039` requires -- an emptied packet, its zeroed coverage counts, the
// withdrawn state, and the verification plan of a packet that cites no result,
// which for a profiled repository is the profile gate alone. Each is paired with
// the member key its oracle side must also name, so a rewrite can only change
// one named member's VALUE and can never introduce or drop a member.
var relevanceFloorRewrites = []struct{ member, candidate string }{
	{`"authoritative_results":`, `"authoritative_results":0`},
	{`"critical":`, `"critical":[]`},
	{`"included_results":`, `"included_results":0`},
	{`"requested_results":`, `"requested_results":0`},
	{`"results":`, `"results":[]`},
	{`"state":`, `"state":"OUT_OF_SCOPE"`},
	{`"verification":`, `"verification":["make gate"]`},
}

var standaloneRelevanceFloorRewrite = struct{ member, candidate string }{
	`"abstention":`, `"abstention":{"active":true,"reason":"below-relevance-floor"}`,
}

// validRelevanceFloorDivergence pins `DR-0008` to the `harness user-prompt`
// receipt and to exactly the eight regions the relevance floor moves. The oracle
// side of every rewrite is left to the oracle -- it is its bytes, not the
// clause's -- but it must name the same member and must actually differ, so the
// declaration cannot quietly become a no-op.
func validRelevanceFloorDivergence(item parityCase) bool {
	divergence := item.KnownDivergence
	if item.Argv[0] != "harness" && item.Argv[0] != "query" {
		return false
	}
	if divergence.Register != "DR-0008" || divergence.Clause != "GPK-V0-039" {
		return false
	}
	// GPK-V0-045 keeps `abstention.reason` on every branch of both wires, so
	// the withdrawn packet's abstention member leads the rewrite list for the
	// harness receipt exactly as it does for the standalone query.
	if len(divergence.Rewrites) != len(relevanceFloorRewrites)+1 ||
		!validPinnedDivergenceRewrite(divergence.Rewrites[0], standaloneRelevanceFloorRewrite) {
		return false
	}
	for index, pinned := range relevanceFloorRewrites {
		if !validPinnedDivergenceRewrite(divergence.Rewrites[index+1], pinned) {
			return false
		}
	}
	return true
}

func validPinnedDivergenceRewrite(rewrite divergenceRewrite, pinned struct{ member, candidate string }) bool {
	return rewrite.Candidate == pinned.candidate && strings.HasPrefix(rewrite.Oracle, pinned.member) && rewrite.Oracle != rewrite.Candidate
}

// admittedUniverseUncertaintyMark is the reason text an omission carries in
// `uncertainty`. `GPK-V0-040` requires a nonzero omission to be named there, so
// pinning the text means the third rewrite cannot declare away an arbitrary
// uncertainty array -- only the entry the dropped results produce.
const admittedUniverseUncertaintyMark = " ranked results omitted by result limit"

// validAdmittedUniverseDivergence pins `DR-0009` to the `impact` receipt and to
// exactly the three regions a ranking ceiling moves. No clause can author the
// fixture's own result count, so the numbers are not transcribed: the omission
// count is read out of the first rewrite and every other value is required to
// satisfy the identity `GPK-V0-040` states. The oracle side is the defect the
// register records -- a structurally zero omission, a `requested_results` that
// names the narrowed output, and the uncertainty entry that therefore never
// appears.
func validAdmittedUniverseDivergence(item parityCase) bool {
	divergence := item.KnownDivergence
	if item.Argv[0] != "impact" {
		return false
	}
	if divergence.Register != "DR-0009" || divergence.Clause != "GPK-V0-040" {
		return false
	}
	if len(divergence.Rewrites) != 3 {
		return false
	}
	omitted, requested, uncertainty := divergence.Rewrites[0], divergence.Rewrites[1], divergence.Rewrites[2]
	dropped, ok := coverageCount(omitted.Candidate, `"omitted_results":`)
	if !ok || dropped < 1 || omitted.Oracle != `"omitted_results":0` {
		return false
	}
	admitted, ok := coverageCount(requested.Candidate, `"requested_results":`)
	if !ok {
		return false
	}
	// The oracle denominates `requested_results` in the results it emitted, so
	// its value IS the included count, and the candidate's must exceed it by
	// exactly the results the ceiling dropped. That identity is the clause, and
	// it leaves no number in this declaration free to be read off the candidate.
	emitted, ok := coverageCount(requested.Oracle, `"requested_results":`)
	if !ok || admitted != emitted+dropped {
		return false
	}
	return uncertainty.Candidate == `"uncertainty":["`+strconv.Itoa(dropped)+admittedUniverseUncertaintyMark+`"]` &&
		uncertainty.Oracle == `"uncertainty":[]`
}

// validQueryAdmittedUniverseDivergence pins `DR-0025` to the `query` receipt and
// to the same three regions a ranking ceiling moves in `validAdmittedUniverseDivergence`.
// The defect is one defect on two command paths: the oracle truncates before it
// measures, so its `requested_results` names the narrowed output and its
// `omitted_results` is structurally zero. The declaration transcribes no fixture
// number -- the omission count is read out of the first rewrite and every other
// value must satisfy the identity `GPK-V0-040` states.
func validQueryAdmittedUniverseDivergence(item parityCase) bool {
	divergence := item.KnownDivergence
	if item.Argv[0] != "query" {
		return false
	}
	if divergence.Register != "DR-0025" || divergence.Clause != "GPK-V0-040" {
		return false
	}
	if len(divergence.Rewrites) != 3 {
		return false
	}
	omitted, requested, uncertainty := divergence.Rewrites[0], divergence.Rewrites[1], divergence.Rewrites[2]
	dropped, ok := coverageCount(omitted.Candidate, `"omitted_results":`)
	if !ok || dropped < 1 || omitted.Oracle != `"omitted_results":0` {
		return false
	}
	admitted, ok := coverageCount(requested.Candidate, `"requested_results":`)
	if !ok {
		return false
	}
	emitted, ok := coverageCount(requested.Oracle, `"requested_results":`)
	if !ok || admitted != emitted+dropped {
		return false
	}
	return uncertainty.Candidate == `"uncertainty":["`+strconv.Itoa(dropped)+admittedUniverseUncertaintyMark+`"]` &&
		uncertainty.Oracle == `"uncertainty":[]`
}

// traceRevisionDisclosureMark is the clause text `GPK-V0-044` appends to a
// learned-path evidence reason, ahead of the 12-character trace revision.
const traceRevisionDisclosureMark = "; trace recorded at commit "

// validTraceRevisionDisclosureDivergence pins `DR-0035` to the `query` receipt
// of a fixture whose trace store holds one trace recorded at the fixture commit,
// and to exactly one rewrite per learned-path role (changed, then opened). Each
// oracle side is a learned-path reason closed by its JSON quote; each candidate
// side is that reason with only the disclosure `GPK-V0-044` requires inserted
// before the quote, naming the manifest's own commit revision rather than bytes
// read off the candidate.
func validTraceRevisionDisclosureDivergence(item parityCase) bool {
	divergence := item.KnownDivergence
	if item.Argv[0] != "query" {
		return false
	}
	if divergence.Register != "DR-0035" || divergence.Clause != "GPK-V0-044" {
		return false
	}
	if len(divergence.Rewrites) != 2 || !lowerHex(item.CommitRevision, 40) {
		return false
	}
	disclosure := traceRevisionDisclosureMark + item.CommitRevision[:12] + `"`
	for index, role := range []string{" changed this path; ", " opened this path; "} {
		rewrite := divergence.Rewrites[index]
		reason, closed := strings.CutSuffix(rewrite.Oracle, `"`)
		if !closed || !strings.HasPrefix(reason, "successful local trace ") || !strings.Contains(reason, role) {
			return false
		}
		if rewrite.Candidate != reason+disclosure {
			return false
		}
	}
	return true
}

// rootPackageImporterRewrite is the exact reverse-import row `impact-go-root`'s
// frozen fixture produces: `cmd/tool/main.go`'s committed blob hash, the score
// and summary `GPK-V0-027` computes for a direct importer of the root package.
// Pinning it exactly, the way the other known-divergence rewrites are pinned,
// closes a self-check gap a `strings.Contains` admission left open: a byte
// appended after the matched substrings still satisfies `Contains`, so only
// the comparison -- not the self-check -- caught a changed declaration.
const rootPackageImporterRewrite = `,{"evidence":[{"authority":"syntax","blob_hash":"bf5ab3508009e5bc2cb8da4a1872c7d7fbaef5c6","confidence":"high","line":3,"path":"cmd/tool/main.go","reason":"imports example.test/rootpkg"}],"id":"cmd/tool/main.go","kind":"reverse-import","score":700,"summary":"directly imports package/module containing logger.go"}`

// coverageCount returns the value of an encoded coverage member, and whether the
// declaration is exactly that member and its integer -- nothing before it and
// nothing after.
// validRootPackageDivergence pins `DR-0017` to `impact` on a root-package path:
// the candidate resolves the root package's import path to the module path
// (GPK-V0-027) and admits the importer the oracle's `module/.` target can
// never find. The declaration removes exactly one reverse-import row and the
// verification command it contributed, and every coverage count the oracle
// emits is the candidate's less that one row; the packet byte count is
// reconciled by the replay itself (reconcilePacketBytes), so no number is free
// to be read off either side.
func validRootPackageDivergence(item parityCase) bool {
	divergence := item.KnownDivergence
	if item.Argv[0] != "impact" {
		return false
	}
	if divergence.Register != "DR-0017" || divergence.Clause != "GPK-V0-027" {
		return false
	}
	if len(divergence.Rewrites) != 5 {
		return false
	}
	importer := divergence.Rewrites[0]
	if importer.Oracle != "" || importer.Candidate != rootPackageImporterRewrite {
		return false
	}
	for index, member := range []string{`"authoritative_results":`, `"included_results":`, `"requested_results":`} {
		rewrite := divergence.Rewrites[1+index]
		candidate, ok := coverageCount(rewrite.Candidate, member)
		if !ok {
			return false
		}
		oracle, ok := coverageCount(rewrite.Oracle, member)
		if !ok || candidate != oracle+1 {
			return false
		}
	}
	verification := divergence.Rewrites[4]
	return verification.Oracle == "" && strings.HasPrefix(verification.Candidate, `"go test ./`) && strings.HasSuffix(verification.Candidate, `/...",`)
}

// validTaskBoundDivergence pins `DR-0016` to the oversized `query` task: both
// runtimes refuse, and only the bound each names in its refusal differs
// (GPK-V0-028 raised the candidate's to 8,000). The task must exceed the
// candidate's bound so that stdout is empty on both sides and the single
// stderr rewrite is the whole divergence.
func validTaskBoundDivergence(item parityCase) bool {
	divergence := item.KnownDivergence
	if item.Argv[0] != "query" || divergence.Register != "DR-0016" || divergence.Clause != "GPK-V0-028" {
		return false
	}
	if len(divergence.Rewrites) != 0 || len(divergence.StderrRewrites) != 1 {
		return false
	}
	task := ""
	for position, argument := range item.Argv {
		if argument == "--task" && position+1 < len(item.Argv) {
			task = item.Argv[position+1]
		}
	}
	if len(task) <= 8000 {
		return false
	}
	rewrite := divergence.StderrRewrites[0]
	return rewrite.Candidate == "exceeds 8000 characters" && rewrite.Oracle == "exceeds 2000 characters"
}

func coverageCount(encoded, member string) (int, bool) {
	if !strings.HasPrefix(encoded, member) {
		return 0, false
	}
	value, err := strconv.Atoi(strings.TrimPrefix(encoded, member))
	if err != nil {
		return 0, false
	}
	return value, true
}

// validStructuralComparison closes decision 0005 to the one field that earns it:
// a measurement of the run rather than a result of the computation.
func validStructuralComparison(item parityCase) bool {
	if item.StructuralComparison == nil {
		return false
	}
	structural := item.StructuralComparison
	return item.ID == "eval-frozen-corpus" && strings.Join(item.Argv, "\x00") == "eval" &&
		strings.Join(structural.Fields, "\x00") == "stdout.evaluation.metrics.latency_ms" &&
		structural.Kind == "measured-run-latency" && structural.Reason != ""
}

func validLRFArgv(argv []string) bool {
	joined := strings.Join(argv, "\x00")
	return joined == "lrf\x00--cem\x00.atlas/change.cem.json\x00--patch\x00change.patch" ||
		joined == "lrf"
}

func validEvalArgv(argv []string) bool {
	joined := strings.Join(argv, "\x00")
	return joined == "eval" || joined == "eval\x00--bogus" || joined == "eval\x00--goldens\x00nope.json"
}

// validOCMArgv admits only the read-only actions the Go port implements.
// prepare, link, and mark are deliberately unimplemented and are carried as
// refusals, never as parity cases.
// validHarnessArgv admits the one harness invocation shape plus the two argument
// surfaces a negative case needs: an unknown subcommand, and an omitted --host.
// Every other flag stays pinned so a case cannot quietly widen the seed.
func validHarnessArgv(argv []string) bool {
	if strings.Join(argv, "\x00") == "harness\x00bogus" {
		return true
	}
	if len(argv) < 2 || argv[1] != "event" {
		return len(argv) > 0 && argv[0] == "harness" && validHarnessEventFlags(argv[1:])
	}
	return validHarnessEventFlags(argv[2:])
}

// validHarnessEventFlags pins each flag to its seed value. --event is the one
// free field, because a negative case has to reach the invalid-choice path.
func validHarnessEventFlags(flags []string) bool {
	pinned := map[string]string{
		"--host": "claude-code", "--host-version": "1.0.0", "--surface": "plugin",
		"--adapter-version": "0.1.0", "--input": "-",
	}
	seen := make(map[string]struct{}, len(pinned)+1)
	for index := 0; index+1 < len(flags); index += 2 {
		name, value := flags[index], flags[index+1]
		if _, duplicate := seen[name]; duplicate {
			return false
		}
		seen[name] = struct{}{}
		if name == "--event" {
			continue
		}
		if expected, ok := pinned[name]; !ok || value != expected {
			return false
		}
	}
	if len(flags)%2 != 0 {
		return false
	}
	_, hasEvent := seen["--event"]
	return hasEvent
}

// validCEMArgv pins the cem seed. cem has seven subcommands over an argparse
// surface whose message ORDER is itself observable, so the seed is an explicit
// set of invocations rather than a shape rule.
func validCEMArgv(argv []string) bool {
	switch strings.Join(argv, "\x00") {
	case
		"cem",
		"cem\x00begin\x00--bogus",
		"cem\x00begin\x00--patch\x00.atlas/change.patch\x00--base\x00c192e99eb59d229b502dbc01701da847de437cff\x00--output\x00.atlas/begun.cem.json",
		"cem\x00begin\x00--patch\x00.atlas/nope.patch\x00--base\x00c192e99eb59d229b502dbc01701da847de437cff\x00--output\x00.atlas/begun.cem.json",
		"cem\x00bogus",
		"cem\x00cite\x00--map\x00.atlas/change.cem.json\x00--hunk\x00hunk:sha256:0000000000000000000000000000000000000000000000000000000000000000\x00--evidence-path\x00docs/rule.md\x00--bytes\x000:1\x00--lines\x001:1\x00--relation\x00decision",
		"cem\x00cite\x00--map\x00.atlas/change.cem.json\x00--hunk\x00hunk:sha256:0000000000000000000000000000000000000000000000000000000000000000\x00--evidence-path\x00docs/rule.md\x00--bytes\x000:1\x00--relation\x00nope",
		"cem\x00cite\x00--map\x00.atlas/change.cem.json\x00--hunk\x00hunk:sha256:0000000000000000000000000000000000000000000000000000000000000000\x00--evidence-path\x00docs/rule.md\x00--relation\x00decision",
		"cem\x00cite\x00--map\x00.atlas/change.cem.json\x00--hunk\x00hunk:sha256:0000000000000000000000000000000000000000000000000000000000000000\x00--evidence-path\x00nope.md\x00--bytes\x000:1\x00--relation\x00decision",
		"cem\x00cite\x00--map\x00.atlas/change.cem.json\x00--hunk\x00hunk:sha256:64c44e7d40bd23c3b89fc78ff6425741d9d90cb3dfbfa11c7b836887847f65c5\x00--evidence-path\x00../escape.md\x00--bytes\x000:1\x00--relation\x00decision",
		"cem\x00cite\x00--map\x00.atlas/change.cem.json\x00--hunk\x00hunk:sha256:64c44e7d40bd23c3b89fc78ff6425741d9d90cb3dfbfa11c7b836887847f65c5\x00--evidence-path\x00docs/rule.md\x00--bytes\x000:24\x00--relation\x00decision",
		"cem\x00cite\x00--map\x00.atlas/change.cem.json\x00--hunk\x00hunk:sha256:64c44e7d40bd23c3b89fc78ff6425741d9d90cb3dfbfa11c7b836887847f65c5\x00--evidence-path\x00docs/rule.md\x00--bytes\x000:999999\x00--relation\x00decision",
		"cem\x00cite\x00--map\x00.atlas/change.cem.json\x00--hunk\x00hunk:sha256:64c44e7d40bd23c3b89fc78ff6425741d9d90cb3dfbfa11c7b836887847f65c5\x00--evidence-path\x00docs/rule.md\x00--bytes\x000\x00--relation\x00decision",
		"cem\x00cite\x00--map\x00.atlas/change.cem.json\x00--hunk\x00hunk:sha256:64c44e7d40bd23c3b89fc78ff6425741d9d90cb3dfbfa11c7b836887847f65c5\x00--evidence-path\x00docs/rule.md\x00--bytes\x0010:5\x00--relation\x00decision",
		"cem\x00cite\x00--map\x00.atlas/change.cem.json\x00--hunk\x00hunk:sha256:64c44e7d40bd23c3b89fc78ff6425741d9d90cb3dfbfa11c7b836887847f65c5\x00--evidence-path\x00docs/rule.md\x00--bytes\x005:-1\x00--relation\x00decision",
		"cem\x00cite\x00--map\x00.atlas/change.cem.json\x00--hunk\x00hunk:sha256:64c44e7d40bd23c3b89fc78ff6425741d9d90cb3dfbfa11c7b836887847f65c5\x00--evidence-path\x00docs/rule.md\x00--bytes\x005:5\x00--relation\x00decision",
		"cem\x00cite\x00--map\x00.atlas/change.cem.json\x00--hunk\x00hunk:sha256:64c44e7d40bd23c3b89fc78ff6425741d9d90cb3dfbfa11c7b836887847f65c5\x00--evidence-path\x00docs/rule.md\x00--bytes\x00a:b\x00--relation\x00decision",
		"cem\x00cite\x00--map\x00.atlas/change.cem.json\x00--hunk\x00hunk:sha256:64c44e7d40bd23c3b89fc78ff6425741d9d90cb3dfbfa11c7b836887847f65c5\x00--evidence-path\x00docs/rule.md\x00--lines\x000:2\x00--relation\x00decision",
		"cem\x00cite\x00--map\x00.atlas/change.cem.json\x00--hunk\x00hunk:sha256:64c44e7d40bd23c3b89fc78ff6425741d9d90cb3dfbfa11c7b836887847f65c5\x00--evidence-path\x00docs/rule.md\x00--lines\x001:2\x00--relation\x00decision",
		"cem\x00cite\x00--map\x00.atlas/change.cem.json\x00--hunk\x00hunk:sha256:64c44e7d40bd23c3b89fc78ff6425741d9d90cb3dfbfa11c7b836887847f65c5\x00--evidence-path\x00docs/rule.md\x00--lines\x001:999\x00--relation\x00decision",
		"cem\x00mark\x00--map\x00.atlas/change.cem.json\x00--hunk\x00hunk:sha256:0000000000000000000000000000000000000000000000000000000000000000\x00--disposition\x00nope\x00--reason\x00no-evidence",
		"cem\x00mark\x00--map\x00.atlas/change.cem.json\x00--hunk\x00hunk:sha256:0000000000000000000000000000000000000000000000000000000000000000\x00--disposition\x00unknown\x00--reason\x00no-evidence",
		"cem\x00mark\x00--map\x00.atlas/change.cem.json\x00--hunk\x00hunk:sha256:0000000000000000000000000000000000000000000000000000000000000000\x00--disposition\x00unknown\x00--reason\x00nope",
		"cem\x00mark\x00--map\x00.atlas/change.cem.json\x00--hunk\x00hunk:sha256:64c44e7d40bd23c3b89fc78ff6425741d9d90cb3dfbfa11c7b836887847f65c5\x00--disposition\x00mechanical\x00--reason\x00whitespace-only",
		"cem\x00mark\x00--map\x00.atlas/change.cem.json\x00--hunk\x00hunk:sha256:64c44e7d40bd23c3b89fc78ff6425741d9d90cb3dfbfa11c7b836887847f65c5\x00--disposition\x00unknown\x00--reason\x00no-evidence",
		"cem\x00prepare\x00--base\x00c192e99eb59d229b502dbc01701da847de437cff\x00--target\x003bd8f2c220fea2801e791715fb3b39b805b6e144",
		"cem\x00prepare\x00--base\x00c192e99eb59d229b502dbc01701da847de437cff\x00--target\x003bd8f2c220fea2801e791715fb3b39b805b6e144\x00--map\x00.atlas/other.json",
		"cem\x00prepare\x00--base\x00nope\x00--target\x003bd8f2c220fea2801e791715fb3b39b805b6e144",
		"cem\x00status\x00--map\x00.atlas/change.cem.json\x00--expected-base\x00c192e99eb59d229b502dbc01701da847de437cff\x00--target\x003bd8f2c220fea2801e791715fb3b39b805b6e144",
		"cem\x00status\x00--map\x00.atlas/change.cem.json\x00--expected-base\x00nope\x00--target\x003bd8f2c220fea2801e791715fb3b39b805b6e144",
		"cem\x00verify",
		"cem\x00verify\x00--map",
		"cem\x00verify\x00--map\x00.atlas/change.cem.json\x00--bogus",
		"cem\x00verify\x00--map\x00.atlas/change.cem.json\x00--expected-base\x00c192e99eb59d229b502dbc01701da847de437cff\x00--target\x003bd8f2c220fea2801e791715fb3b39b805b6e144",
		"cem\x00verify\x00--map\x00.atlas/change.cem.json\x00--expected-base\x00c192e99eb59d229b502dbc01701da847de437cff\x00--target\x003bd8f2c220fea2801e791715fb3b39b805b6e144\x00--max-unknown\x005",
		"cem\x00verify\x00--map\x00.atlas/nope.json":
		return true
	}
	return false
}

func validFeatureArgv(argv []string) bool {
	joined := strings.Join(argv, "\x00")
	return joined == "feature" || joined == "feature\x00a-first" || joined == "feature\x00nope" ||
		joined == "feature\x00a-first\x00--bogus" || joined == "feature\x00a-first\x00--limit\x000" ||
		joined == "feature\x00a-first\x00--budget-bytes\x002000"
}

// validActivationArgv pins the init and adopt seed. Both take the same bootstrap
// parser, so one allowlist covers them: the bare command plus one option each.
func validActivationArgv(argv []string) bool {
	options := strings.Join(argv[1:], "\x00")
	switch options {
	case "", "--bogus", "--full-receipt", "--authority-id\x00example",
		"--exclude-prefix\x00pkg", "--exclude-prefix\x00pkg/", "--revision\x00nope":
		return true
	}
	return false
}

// ocmLinkedBase and ocmLinkedTarget are the two commits of the ocm-linked
// fixture. A cem/0.2 map binds a base and a target, so the only invocation that
// can reach a SUCCESSFUL OCM verification names both.
const (
	ocmLinkedBase   = "c192e99eb59d229b502dbc01701da847de437cff"
	ocmLinkedTarget = "3bd8f2c220fea2801e791715fb3b39b805b6e144"
)

// lrfPythonBase and lrfPythonTarget are the two commits of the lrf-ocm-python
// fixture, whose OCM map carries the one claim class Go refuses to verify.
const (
	lrfPythonBase   = "f5389c641f092d653de73bca8e1c7122f74d3d2d"
	lrfPythonTarget = "7203184eba7a325ca94c7396d7d54863ff98a7f4"
)

// validLRFPythonClaimRefusalArgv pins the single invocation the GPK-V0-041
// abstention covers: an `lrf` evaluation whose --ocm map carries a .py claim.
// Both revisions are named because a cem/0.2 map binds a base and a target.
func validLRFPythonClaimRefusalArgv(argv []string) bool {
	return strings.Join(argv, "\x00") ==
		"lrf\x00--cem\x00.atlas/change.cem.json\x00--ocm\x00.atlas/change.ocm.json\x00--expected-base\x00"+
			lrfPythonBase+"\x00--target\x00"+lrfPythonTarget
}

func validOCMArgv(argv []string) bool {
	switch strings.Join(argv, "\x00") {
	case
		"ocm\x00link\x00--map\x00.atlas/change.ocm.json\x00--cem\x00.atlas/change.cem.json\x00--obligation\x00HTTP-001\x00--hunk\x00hunk:sha256:64c44e7d40bd23c3b89fc78ff6425741d9d90cb3dfbfa11c7b836887847f65c5\x00--test-path\x00pkg/app/app_test.go\x00--claim\x00test:TestConstructorSelection/case:http-the-second-generation-server-constructor-must-be-used\x00--expected-base\x00c192e99eb59d229b502dbc01701da847de437cff\x00--target\x003bd8f2c220fea2801e791715fb3b39b805b6e144",
		"ocm\x00mark\x00--map\x00.atlas/change.ocm.json\x00--obligation\x00HTTP-001\x00--reason\x00unassessed",
		"ocm\x00prepare\x00--target\x003bd8f2c220fea2801e791715fb3b39b805b6e144\x00--expected-base\x00c192e99eb59d229b502dbc01701da847de437cff\x00--intent\x00docs/rule.md\x00--cem\x00.atlas/change.cem.json\x00--map\x00.atlas/change.ocm.json":
		return true
	}
	joined := strings.Join(argv, "\x00")
	return joined == "ocm" || joined == "ocm\x00status" || joined == "ocm\x00verify" ||
		joined == "ocm\x00report" || joined == "ocm\x00verify\x00--map\x00.atlas/change.ocm.json" ||
		joined == "ocm\x00verify\x00--map\x00.atlas/change.ocm.json\x00--expected-base\x00"+ocmLinkedBase+"\x00--target\x00"+ocmLinkedTarget
}

func validAcceptedLockDivergence(item parityCase) bool {
	if item.AcceptedDivergence == nil {
		return false
	}
	divergence := item.AcceptedDivergence
	if divergence.OracleOnlyCreatedPath != ".context-atlas/traces/.trace-operation.lock" || divergence.CandidateMutation != "none" || divergence.Reason == "" || item.Mutation != "oracle-only-trace-operation-lock" {
		return false
	}
	argv := strings.Join(item.Argv, "\x00")
	staleMigration := item.ID == "migrate-traces-plan-digest-mismatch" && argv == "migrate-traces\x00--apply\x00--plan-digest\x00"+strings.Repeat("0", 64)
	staleRecord := item.ID == "record-untracked-changed" && argv == "record\x00--task\x00task\x00--changed\x00missing.go\x00--verify\x00go test ./...\x00--outcome\x00passed"
	return staleMigration || staleRecord
}

// validAcceptedPrivateParentDivergence admits the second oracle-only class
// GPK-V0-008 names: `cem prepare` resolves its revisions AFTER creating the patch
// cache's owner-private parent, so a rejected invocation leaves `.git/atlas`
// behind. Go resolves first and creates nothing. Adjudicated in DR-0003; the
// oracle is not repaired, so the divergence is carried, not hidden.
func validAcceptedPrivateParentDivergence(item parityCase) bool {
	if item.AcceptedDivergence == nil {
		return false
	}
	divergence := item.AcceptedDivergence
	if divergence.OracleOnlyCreatedPath != ".git/atlas" || divergence.CandidateMutation != "none" || divergence.Reason == "" || item.Mutation != "oracle-only-private-parent" {
		return false
	}
	argv := strings.Join(item.Argv, "\x00")
	return item.ID == "cem-prepare-unresolvable-base" && argv == "cem\x00prepare\x00--base\x00nope\x00--target\x00"+ocmLinkedTarget
}

func validateRefusalCase(item refusalCase) error {
	if item.ID == "" || item.Support != "unsupported" || item.Repository == "" || len(item.Argv) == 0 || item.TimeoutMilliseconds < 1 || item.Reason == "" {
		return fmt.Errorf("generic refusal contract is invalid")
	}
	if item.Retired != nil && !validRetiredRefusal(item) {
		return fmt.Errorf("retirement is invalid")
	}
	// Each command's refusals are pinned to the clause that authorizes them and to
	// that clause's own refusal vocabulary; the set stays closed per command.
	switch item.Argv[0] {
	case "query":
		if !validQueryRefusal(item) {
			return fmt.Errorf("query refusal contract is invalid")
		}
	case "lrf":
		if item.Authority != "GPK-V0-041" || item.ExpectedType != "unsupported-ocm-python-claims" {
			return fmt.Errorf("lrf refusal contract is invalid")
		}
		if !validLRFPythonClaimRefusalArgv(item.Argv) {
			return fmt.Errorf("lrf refusal argv is outside the clause")
		}
	default:
		return fmt.Errorf("refusal command %q is outside the seed", item.Argv[0])
	}
	return validateSetups(item.Setup)
}

func validQueryRefusal(item refusalCase) bool {
	joined := strings.Join(item.Argv, "\x00")
	switch item.ID {
	case "query-missing-authority-refusal":
		return item.Authority == "GPK-V0-028" && item.ExpectedType == "unsupported-query-authority" &&
			joined == "query\x00--task\x00"+parityPrompt+"\x00--limit\x001"
	case "query-present-trace-store-refusal":
		return item.Authority == "GPK-V0-028" && item.ExpectedType == "unsupported-query-trace-state" &&
			joined == "query\x00--task\x00"+parityPrompt+"\x00--limit\x001"
	}
	return false
}

// validOCMRefusalArgv admits exactly the three write actions the Go port
// deliberately does not implement.

func validateSetups(setups []fixtureSetup) error {
	for _, setup := range setups {
		if setup.Path == "" || strings.HasPrefix(setup.Path, "/") || strings.Contains(setup.Path, "..") {
			return fmt.Errorf("unsafe setup path %q", setup.Path)
		}
		if setup.Kind != "mkdir" && setup.Kind != "write" && setup.Kind != "remove" {
			return fmt.Errorf("unknown setup kind %q", setup.Kind)
		}
		if setup.Kind == "write" {
			data, err := base64.StdEncoding.DecodeString(setup.DataBase64)
			if err != nil || len(data) > 1_000_000 || setup.Mode == 0 {
				return fmt.Errorf("invalid write setup")
			}
		}
	}
	return nil
}

func lowerHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func sha256Hex(value []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(value))
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
