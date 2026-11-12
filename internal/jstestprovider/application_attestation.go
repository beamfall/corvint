package jstestprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/jstestreceipt"
	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

// AttestedExternalProfile is jstestreceipt.AttestedExternalProfile, kept under its old name.
const AttestedExternalProfile = jstestreceipt.AttestedExternalProfile

// ApplicationAttestationProfile is jstestreceipt.ApplicationAttestationProfile, kept under its old name.
const ApplicationAttestationProfile = jstestreceipt.ApplicationAttestationProfile

// ApplicationAttestationProviderProfile is jstestreceipt.ApplicationAttestationProviderProfile, kept under its old name.
const ApplicationAttestationProviderProfile = jstestreceipt.ApplicationAttestationProviderProfile

// ApplicationAttestationProvider is the typed, bounded local command used to
// observe an externally owned application. Corvint supplies the canonical
// configuration bytes on stdin and never supplies lifecycle verbs.
type ApplicationAttestationProvider struct {
	Argv       []string
	ConfigFile string
	Timeout    time.Duration
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

func prepareApplicationAttestationProvider(cfg ApplicationAttestationProvider, dir, scratch string, environment map[string]string) (*preparedApplicationAttestationProvider, error) {
	if len(cfg.Argv) == 0 || cfg.ConfigFile == "" || !filepath.IsAbs(cfg.ConfigFile) {
		return nil, errors.New("application-attestation-provider-required")
	}
	resolved := resolveArgv(cfg.Argv)
	if !filepath.IsAbs(resolved[0]) {
		return nil, errors.New("application-attestation-provider-unavailable")
	}
	executable, err := readBoundedRegularFile(resolved[0], jstestreceipt.ApplicationAttestationExecutableLimit)
	if err != nil {
		return nil, errors.New("application-attestation-provider-unavailable")
	}
	configRaw, err := readBoundedRegularFile(cfg.ConfigFile, jstestreceipt.ApplicationAttestationLimit)
	if err != nil {
		return nil, errors.New("application-attestation-config-unavailable")
	}
	var config jstestreceipt.ApplicationAttestationConfig
	if err := decodeCanonical(configRaw, &config); err != nil || config.Profile != jstestreceipt.ApplicationAttestationConfigProfile || jstestreceipt.ValidateExpectation(config.Expectation) != "" {
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
		ExecutableDigest: jstestreceipt.Sha256Hex(executable),
		ConfigPath:       cfg.ConfigFile,
		ConfigDigest:     jstestreceipt.Sha256Hex(configRaw),
		Environment:      providerEnvironment,
	}
	receipt := ApplicationAttestationReceipt{Provider: identity, Expectation: config.Expectation, Failures: []string{}}
	return &preparedApplicationAttestationProvider{receipt: receipt, configRaw: configRaw, configPath: cfg.ConfigFile, command: command, dir: dir, environment: environmentList(providerEnvironment), timeout: timeout}, nil
}

func (provider *preparedApplicationAttestationProvider) observe(ctx context.Context) (*ApplicationAttestationObservation, string) {
	obs := procgroup.Run(ctx, procgroup.Spec{Argv: provider.command, Dir: provider.dir, Env: provider.environment, Stdin: provider.configRaw, Timeout: provider.timeout, OutputLimit: jstestreceipt.ApplicationAttestationLimit})
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
	if reason := jstestreceipt.ValidateAttestationShape(attestation); reason != "" {
		return nil, reason
	}
	observation := &ApplicationAttestationObservation{OutputDigest: jstestreceipt.Sha256Hex(obs.Stdout), Attestation: attestation}
	if reason := jstestreceipt.CompareExpectation(provider.receipt.Expectation, attestation); reason != "" {
		return observation, reason
	}
	return observation, ""
}

func (provider *preparedApplicationAttestationProvider) unchanged() bool {
	executable, err := readBoundedRegularFile(provider.receipt.Provider.ExecutablePath, jstestreceipt.ApplicationAttestationExecutableLimit)
	if err != nil || jstestreceipt.Sha256Hex(executable) != provider.receipt.Provider.ExecutableDigest {
		return false
	}
	config, err := readBoundedRegularFile(provider.configPath, jstestreceipt.ApplicationAttestationLimit)
	return err == nil && jstestreceipt.Sha256Hex(config) == provider.receipt.Provider.ConfigDigest
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
	if len(data) == 0 || len(data) > jstestreceipt.ApplicationAttestationLimit || secretscreen.MatchString(string(data)) {
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

// ObserveTestRepositoryAtStart returns the test repository identity of dir
// before a basic (unprofiled) run, or nil when Git cannot name it. A dirty
// worktree still yields its identity with DirtyState "dirty"; the revision is
// what a consumer compares with HEAD to claim revision staleness (V1-1156).
func ObserveTestRepositoryAtStart(ctx context.Context, dir string) *ApplicationRepositoryIdentity {
	identity, _ := observeTestRepository(ctx, dir)
	return identity
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
	if len(lines) != 2 || !jstestreceipt.GitOIDPattern.MatchString(lines[0]) || !jstestreceipt.GitOIDPattern.MatchString(lines[1]) {
		return nil, "test-repository-identity-unavailable"
	}
	// Safe unguarded (EEP-V0-771): gitObservationEnvironment disables info/grafts, so no graft
	// can name a foreign root; a shallow boundary is listed as the root, but it is a real commit
	// of this history, so an identity comparison can only fail to match a full-history identity.
	roots, failure := gitObservation(ctx, canonicalRoot, "rev-list", "--max-parents=0", "HEAD")
	if failure != "" {
		return nil, "test-repository-identity-unavailable"
	}
	rootCommits := strings.Fields(string(roots))
	if len(rootCommits) != 1 || !jstestreceipt.GitOIDPattern.MatchString(rootCommits[0]) {
		return nil, "test-repository-identity-unavailable"
	}
	status, failure := gitObservation(ctx, canonicalRoot, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if failure != "" {
		return nil, "test-repository-identity-unavailable"
	}
	repository := &ApplicationRepositoryIdentity{RootCommit: rootCommits[0], Revision: lines[0], Tree: lines[1], DirtyState: "clean"}
	if len(status) != 0 {
		repository.DirtyState = "dirty"
		repository.DirtyDigest = "sha256:" + jstestreceipt.Sha256Hex(status)
		return repository, "test-repository-dirty"
	}
	return repository, ""
}

func gitObservation(ctx context.Context, dir string, args ...string) ([]byte, string) {
	obs := runGitObservation(ctx, dir, args...)
	if !obs.Started || !obs.WaitCompleted || !obs.ExitObserved || obs.ExitStatus != 0 || obs.OutputOverflow || !obs.OwnedProcessGroupCleanup {
		return nil, "git-unavailable"
	}
	return obs.Stdout, ""
}

// runGitObservation runs one observation Git command and returns the whole
// observation, stderr included, which gitObservation does not surface.
func runGitObservation(ctx context.Context, dir string, args ...string) procgroup.Observation {
	// GIT_GRAFT_FILE names the null device; keep Git's graft advice off stderr (V1-3588).
	argv := resolveArgv(append([]string{"git", "-c", "advice.graftFileDeprecated=false", "-C", dir}, args...))
	return procgroup.Run(ctx, procgroup.Spec{Argv: argv, Dir: dir, Env: gitObservationEnvironment(), Timeout: 5 * time.Second, OutputLimit: jstestreceipt.ApplicationAttestationLimit})
}

func gitObservationEnvironment() []string {
	environment := os.Environ()
	clean := make([]string, 0, len(environment))
	for _, entry := range environment {
		if !strings.HasPrefix(entry, "GIT_") {
			clean = append(clean, entry)
		}
	}
	// Git honours info/grafts even with replace objects disabled; /dev/null disables it.
	return append(clean, "GIT_GRAFT_FILE="+os.DevNull)
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
