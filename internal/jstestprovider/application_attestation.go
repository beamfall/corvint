package jstestprovider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

const (
	AttestedExternalProfile               = "corvint-playwright-external/1"
	ApplicationAttestationProfile         = "corvint-application-attestation/0"
	ApplicationAttestationProviderProfile = "corvint-application-attestation-command/0"
	applicationAttestationConfigProfile   = "corvint-application-attestation-config/0"
	applicationAttestationLimit           = 64 << 10
	applicationAttestationExecutableLimit = 64 << 20
)

var (
	gitOIDPattern    = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)
	digestPattern    = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	rawDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	tokenPattern     = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
)

// ApplicationAttestationProvider is the typed, bounded local command used to
// observe an externally owned application. Corvint supplies the canonical
// configuration bytes on stdin and never supplies lifecycle verbs.
type ApplicationAttestationProvider struct {
	Argv       []string
	ConfigFile string
	Timeout    time.Duration
}

type applicationAttestationConfig struct {
	Profile     string                            `json:"profile"`
	Expectation ApplicationAttestationExpectation `json:"expectation"`
}

type preparedApplicationAttestationProvider struct {
	receipt     ApplicationAttestationReceipt
	configRaw   []byte
	configPath  string
	command     []string
	dir         string
	environment []string
	timeout     time.Duration
}

func isExternalProfile(profile string) bool {
	return profile == ExternalProfile || profile == AttestedExternalProfile
}

func prepareApplicationAttestationProvider(cfg ApplicationAttestationProvider, dir, scratch string, environment map[string]string) (*preparedApplicationAttestationProvider, error) {
	if len(cfg.Argv) == 0 || cfg.ConfigFile == "" || !filepath.IsAbs(cfg.ConfigFile) {
		return nil, errors.New("application-attestation-provider-required")
	}
	resolved := resolveArgv(cfg.Argv)
	if !filepath.IsAbs(resolved[0]) {
		return nil, errors.New("application-attestation-provider-unavailable")
	}
	executable, err := readBoundedRegularFile(resolved[0], applicationAttestationExecutableLimit)
	if err != nil {
		return nil, errors.New("application-attestation-provider-unavailable")
	}
	configRaw, err := readBoundedRegularFile(cfg.ConfigFile, applicationAttestationLimit)
	if err != nil {
		return nil, errors.New("application-attestation-config-unavailable")
	}
	var config applicationAttestationConfig
	if err := decodeCanonical(configRaw, &config); err != nil || config.Profile != applicationAttestationConfigProfile || validateExpectation(config.Expectation) != "" {
		return nil, errors.New("application-attestation-config-invalid")
	}
	staged := filepath.Join(scratch, "application-attestation-provider")
	if err := os.WriteFile(staged, executable, 0o700); err != nil {
		return nil, errors.New("application-attestation-provider-unavailable")
	}
	command := append([]string{staged}, resolved[1:]...)
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	providerEnvironment := cloneStrings(environment)
	for _, key := range []string{"HOME", "DOCKER_CONFIG", "DOCKER_CONTEXT", "DOCKER_HOST"} {
		if value, present := os.LookupEnv(key); present {
			providerEnvironment[key] = value
		}
	}
	identity := ApplicationAttestationProviderIdentity{
		Profile:          ApplicationAttestationProviderProfile,
		Argv:             append([]string(nil), resolved...),
		ExecutablePath:   resolved[0],
		ExecutableDigest: sha256Hex(executable),
		ConfigPath:       cfg.ConfigFile,
		ConfigDigest:     sha256Hex(configRaw),
		Environment:      providerEnvironment,
	}
	receipt := ApplicationAttestationReceipt{Provider: identity, Expectation: config.Expectation, Failures: []string{}}
	return &preparedApplicationAttestationProvider{receipt: receipt, configRaw: configRaw, configPath: cfg.ConfigFile, command: command, dir: dir, environment: environmentList(providerEnvironment), timeout: timeout}, nil
}

func (provider *preparedApplicationAttestationProvider) observe(ctx context.Context) (*ApplicationAttestationObservation, string) {
	obs := procgroup.Run(ctx, procgroup.Spec{Argv: provider.command, Dir: provider.dir, Env: provider.environment, Stdin: provider.configRaw, Timeout: provider.timeout, OutputLimit: applicationAttestationLimit})
	if obs.Cancelled {
		return nil, "application-attestation-provider-cancelled"
	}
	if obs.TimedOut {
		return nil, "application-attestation-provider-timeout"
	}
	if !obs.Started || !obs.WaitCompleted || !obs.ExitObserved || obs.ExitStatus != 0 || obs.OutputOverflow || obs.StdoutOverflow || obs.StderrOverflow || !obs.OwnedProcessGroupCleanup {
		return nil, "application-attestation-provider-unavailable"
	}
	var attestation ApplicationAttestation
	if err := decodeCanonical(obs.Stdout, &attestation); err != nil {
		return nil, "application-attestation-output-invalid"
	}
	if reason := validateAttestationShape(attestation); reason != "" {
		return nil, reason
	}
	observation := &ApplicationAttestationObservation{OutputDigest: sha256Hex(obs.Stdout), Attestation: attestation}
	if reason := compareExpectation(provider.receipt.Expectation, attestation); reason != "" {
		return observation, reason
	}
	return observation, ""
}

func (provider *preparedApplicationAttestationProvider) unchanged() bool {
	executable, err := readBoundedRegularFile(provider.receipt.Provider.ExecutablePath, applicationAttestationExecutableLimit)
	if err != nil || sha256Hex(executable) != provider.receipt.Provider.ExecutableDigest {
		return false
	}
	config, err := readBoundedRegularFile(provider.configPath, applicationAttestationLimit)
	return err == nil && sha256Hex(config) == provider.receipt.Provider.ConfigDigest
}

func readBoundedRegularFile(path string, limit int64) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() {
		return nil, errors.New("not-regular")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) {
		return nil, errors.New("file-replaced")
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, errors.New("file-bound-exceeded")
	}
	return data, nil
}

func decodeCanonical(data []byte, output any) error {
	if len(data) == 0 || len(data) > applicationAttestationLimit || secretscreen.MatchString(string(data)) {
		return errors.New("invalid-canonical-input")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil {
		return err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return errors.New("trailing-data")
	}
	canonical, err := json.Marshal(output)
	if err != nil {
		return err
	}
	canonical = append(canonical, '\n')
	if !bytes.Equal(data, canonical) {
		return errors.New("noncanonical-input")
	}
	return nil
}

func validateExpectation(expectation ApplicationAttestationExpectation) string {
	if !validRepositoryExpectation(expectation.Repository) {
		return "application-attestation-expectation-invalid"
	}
	if !validArtifact(expectation.Build) || !validArtifact(expectation.Configuration) || !tokenPattern.MatchString(expectation.InstanceKind) {
		return "application-attestation-expectation-invalid"
	}
	return ""
}

func validRepositoryExpectation(repository ApplicationRepositoryExpectation) bool {
	if !gitOIDPattern.MatchString(repository.RootCommit) || !gitOIDPattern.MatchString(repository.Revision) || !gitOIDPattern.MatchString(repository.Tree) {
		return false
	}
	switch repository.DirtyPolicy {
	case "require-clean":
		return repository.DirtyDigest == ""
	case "allow-dirty-with-digest":
		return digestPattern.MatchString(repository.DirtyDigest)
	default:
		return false
	}
}

func validArtifact(artifact ApplicationArtifactIdentity) bool {
	return tokenPattern.MatchString(artifact.Kind) && digestPattern.MatchString(artifact.Digest)
}

func validateAttestationShape(attestation ApplicationAttestation) string {
	if attestation.Profile != ApplicationAttestationProfile {
		return "application-attestation-profile-unknown"
	}
	repository := attestation.Repository
	if !gitOIDPattern.MatchString(repository.RootCommit) || !gitOIDPattern.MatchString(repository.Revision) || !gitOIDPattern.MatchString(repository.Tree) {
		return "application-attestation-identity-unavailable"
	}
	if repository.DirtyState != "clean" && repository.DirtyState != "dirty" {
		return "application-attestation-identity-unavailable"
	}
	if repository.DirtyState == "clean" && repository.DirtyDigest != "" {
		return "application-attestation-contradictory"
	}
	if repository.DirtyState == "dirty" && !digestPattern.MatchString(repository.DirtyDigest) {
		return "application-attestation-identity-unavailable"
	}
	if !validArtifact(attestation.Build) || !validArtifact(attestation.Configuration) {
		return "application-attestation-identity-unavailable"
	}
	if !tokenPattern.MatchString(attestation.Instance.Kind) || strings.TrimSpace(attestation.Instance.ID) == "" || strings.TrimSpace(attestation.Instance.StartGeneration) == "" || len(attestation.Instance.ID) > 4096 || len(attestation.Instance.StartGeneration) > 4096 {
		return "application-attestation-identity-unavailable"
	}
	if attestation.Health.State != "healthy" && attestation.Health.State != "unhealthy" && attestation.Health.State != "unknown" {
		return "application-attestation-contradictory"
	}
	if len(attestation.Health.Detail) > 4096 {
		return "application-attestation-output-invalid"
	}
	if attestation.Health.State != "healthy" {
		return "application-attestation-unhealthy"
	}
	return ""
}

func compareExpectation(expectation ApplicationAttestationExpectation, actual ApplicationAttestation) string {
	switch {
	case actual.Repository.RootCommit != expectation.Repository.RootCommit:
		return "application-root-commit-mismatch"
	case actual.Repository.Revision != expectation.Repository.Revision:
		return "application-revision-mismatch"
	case actual.Repository.Tree != expectation.Repository.Tree:
		return "application-tree-mismatch"
	case expectation.Repository.DirtyPolicy == "require-clean" && actual.Repository.DirtyState != "clean":
		return "application-dirty-policy-mismatch"
	case expectation.Repository.DirtyPolicy == "allow-dirty-with-digest" && actual.Repository.DirtyDigest != expectation.Repository.DirtyDigest:
		return "application-dirty-digest-mismatch"
	case actual.Build != expectation.Build:
		return "application-build-mismatch"
	case actual.Configuration != expectation.Configuration:
		return "application-configuration-mismatch"
	case actual.Instance.Kind != expectation.InstanceKind:
		return "application-instance-kind-mismatch"
	}
	return ""
}

func compareApplicationAttestations(before, after ApplicationAttestation) []string {
	var failures []string
	if before.Repository != after.Repository {
		failures = append(failures, "application-repository-drift")
	}
	if before.Build != after.Build {
		failures = append(failures, "application-build-drift")
	}
	if before.Configuration != after.Configuration {
		failures = append(failures, "application-configuration-drift")
	}
	if before.Instance != after.Instance {
		failures = append(failures, "application-restarted")
	}
	return failures
}

func applicationAttestationUnknown(receipt *ApplicationAttestationReceipt) bool {
	if receipt == nil || len(receipt.Failures) != 0 || receipt.Before == nil || receipt.After == nil {
		return true
	}
	if receipt.Provider.Profile != ApplicationAttestationProviderProfile || len(receipt.Provider.Argv) == 0 || receipt.Provider.Argv[0] != receipt.Provider.ExecutablePath || !filepath.IsAbs(receipt.Provider.ExecutablePath) || !rawDigestPattern.MatchString(receipt.Provider.ExecutableDigest) || !filepath.IsAbs(receipt.Provider.ConfigPath) || !rawDigestPattern.MatchString(receipt.Provider.ConfigDigest) || receipt.Provider.Environment == nil {
		return true
	}
	if validateExpectation(receipt.Expectation) != "" {
		return true
	}
	config, err := json.Marshal(applicationAttestationConfig{Profile: applicationAttestationConfigProfile, Expectation: receipt.Expectation})
	if err != nil || sha256Hex(append(config, '\n')) != receipt.Provider.ConfigDigest {
		return true
	}
	for _, observation := range []*ApplicationAttestationObservation{receipt.Before, receipt.After} {
		canonical, err := json.Marshal(observation.Attestation)
		canonical = append(canonical, '\n')
		if err != nil || sha256Hex(canonical) != observation.OutputDigest || validateAttestationShape(observation.Attestation) != "" || compareExpectation(receipt.Expectation, observation.Attestation) != "" {
			return true
		}
	}
	return len(compareApplicationAttestations(receipt.Before.Attestation, receipt.After.Attestation)) != 0
}

func observeTestRepository(ctx context.Context, dir string) (*ApplicationRepositoryIdentity, string) {
	top, failure := gitObservation(ctx, dir, "rev-parse", "--show-toplevel")
	if failure != "" {
		return nil, "test-repository-identity-unavailable"
	}
	root := strings.TrimSpace(string(top))
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, "test-repository-identity-unavailable"
	}
	identity, failure := gitObservation(ctx, canonicalRoot, "log", "-1", "--format=%H%n%T", "HEAD")
	if failure != "" {
		return nil, "test-repository-identity-unavailable"
	}
	lines := strings.Fields(string(identity))
	if len(lines) != 2 || !gitOIDPattern.MatchString(lines[0]) || !gitOIDPattern.MatchString(lines[1]) {
		return nil, "test-repository-identity-unavailable"
	}
	roots, failure := gitObservation(ctx, canonicalRoot, "rev-list", "--max-parents=0", "HEAD")
	if failure != "" {
		return nil, "test-repository-identity-unavailable"
	}
	rootCommits := strings.Fields(string(roots))
	if len(rootCommits) != 1 || !gitOIDPattern.MatchString(rootCommits[0]) {
		return nil, "test-repository-identity-unavailable"
	}
	status, failure := gitObservation(ctx, canonicalRoot, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if failure != "" {
		return nil, "test-repository-identity-unavailable"
	}
	repository := &ApplicationRepositoryIdentity{RootCommit: rootCommits[0], Revision: lines[0], Tree: lines[1], DirtyState: "clean"}
	if len(status) != 0 {
		repository.DirtyState = "dirty"
		repository.DirtyDigest = "sha256:" + sha256Hex(status)
		return repository, "test-repository-dirty"
	}
	return repository, ""
}

func gitObservation(ctx context.Context, dir string, args ...string) ([]byte, string) {
	argv := resolveArgv(append([]string{"git", "-C", dir}, args...))
	obs := procgroup.Run(ctx, procgroup.Spec{Argv: argv, Dir: dir, Env: gitObservationEnvironment(), Timeout: 5 * time.Second, OutputLimit: applicationAttestationLimit})
	if !obs.Started || !obs.WaitCompleted || !obs.ExitObserved || obs.ExitStatus != 0 || obs.OutputOverflow || !obs.OwnedProcessGroupCleanup {
		return nil, "git-unavailable"
	}
	return obs.Stdout, ""
}

func gitObservationEnvironment() []string {
	environment := os.Environ()
	clean := make([]string, 0, len(environment))
	for _, entry := range environment {
		if !strings.HasPrefix(entry, "GIT_") {
			clean = append(clean, entry)
		}
	}
	return clean
}

func testRepositoryUnknown(start, publish *ApplicationRepositoryIdentity) bool {
	if start == nil || publish == nil || *start != *publish {
		return true
	}
	if start.DirtyState != "clean" || start.DirtyDigest != "" {
		return true
	}
	return !gitOIDPattern.MatchString(start.RootCommit) || !gitOIDPattern.MatchString(start.Revision) || !gitOIDPattern.MatchString(start.Tree)
}

func sha256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func cloneStrings(input map[string]string) map[string]string {
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func environmentList(environment map[string]string) []string {
	keys := make([]string, 0, len(environment))
	for key := range environment {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+environment[key])
	}
	return result
}
