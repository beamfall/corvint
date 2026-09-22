// Package provider coordinates one explicitly requested, local Go test run.
//
// This package is an experimental canonical transcript producer. Its receipts
// remain non-persistent, UNKNOWN-scope observations and never claim LPCV qualification.
package provider

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/godiscovery"
	"github.com/Beamfall/corvint/internal/liveverify/gorunner"
	"github.com/Beamfall/corvint/internal/liveverify/gotest"
)

const (
	QualificationExperimentalTranscript = "EXPERIMENTAL_TRANSCRIPT"
	ScopeUnknown                        = "UNKNOWN"
	CoverageNone                        = "NONE"
	PersistenceNone                     = "NONE"
	GoVersion                           = "go1.27.1"
	EnvironmentProfile                  = "GO127_CGO0_OFFLINE_POSIX_0"

	// Exact source/dependency/toolchain re-observation includes a bounded full
	// Go-root walk. Keep the post-run deadline independent from the run budget,
	// but large enough for the production parent verifier on a cold filesystem.
	postAuthorityTimeout  = 15 * time.Second
	runDirectoryNameBytes = 16
	runDirectoryRetries   = 8
)

var (
	ErrDisabled             = errors.New("go-live provider: experimental provider is disabled")
	ErrExplicitAction       = errors.New("go-live provider: trusted local explicit action is required")
	ErrInvalidConfig        = errors.New("go-live provider: invalid configuration")
	ErrAuthorityUnavailable = errors.New("go-live provider: execution authority is unavailable")
	ErrAuthorityDrift       = errors.New("go-live provider: execution authority changed")
	ErrCleanupIncomplete    = errors.New("go-live provider: ephemeral cleanup is incomplete")
	ErrUnsupportedPlatform  = errors.New("go-live provider: unsupported platform")
	ErrDiscoveryLimit       = errors.New("go-live provider: discovery output limit exceeded")
)

// ExecutionAuthority is supplied by the parent source, discovery, toolchain,
// and dependency-verification layer. Acquire must fail unless it can bind all
// four identities. The lease keeps them stable until Release. In particular,
// ModuleCacheDirectory is a separately verified, complete, read-only offline
// dependency materialization; it is not created or deleted by this provider.
type ExecutionAuthority interface {
	Acquire(ctx context.Context, request AuthorityRequest) (ExecutionLease, error)
}

type AuthorityRequest struct {
	RepositoryRoot     string
	WorkingDirectory   string
	GoExecutable       string
	GOROOT             string
	ExpectedGoVersion  string
	GOOS               string
	GOARCH             string
	CGOEnabled         string
	EnvironmentProfile string
	ModuleMode         ModuleMode
	Packages           []string
}

// ExecutionLease represents caller-verified, pinned execution inputs.
// Revalidate is the mandatory complete verifier boundary: it must revalidate
// every admitted source, workspace/vendor/local-replacement, dependency, and
// toolchain entry, honor ctx, and return only after its bounded verification
// subprocesses and I/O have quiesced. It runs immediately before launch and
// after descendant cleanup. Provider-local pathname checks are defense in
// depth and do not replace or claim this semantic completeness.
type ExecutionLease interface {
	Binding() AuthorityBinding
	Discover(ctx context.Context, request DiscoveryRequest, stdout, stderr io.Writer) (DiscoveryResult, error)
	Revalidate(ctx context.Context) error
	Release(ctx context.Context) error
}

// ReceiptIdentityLease is the optional canonical-receipt extension. The
// observations are independently recomputable bare digests, not asserted IDs.
// A canonical request fails before launch when its authority lacks this seam.
type ReceiptIdentityLease interface {
	ObserveReceiptIdentities(ctx context.Context) (ReceiptIdentityObservation, error)
	BindCanonical(ctx context.Context, request CanonicalBindingRequest) (CanonicalBinding, error)
}

type ReceiptIdentityObservation struct {
	SourceSHA256    string
	ToolchainSHA256 string
}

type CanonicalBindingRequest struct {
	DiscoveryID        string
	EnvironmentSHA256  string
	PackagePatterns    []string
	CWDPathSHA256      string
	DependencyIdentity string
	ModuleMode         ModuleMode
	SourceIdentity     string
	ToolchainIdentity  string
	Environment        []CanonicalEnvironmentVariable
}

type CanonicalEnvironmentVariable struct {
	Name        string
	ValueSHA256 string
}

type CanonicalBinding struct {
	CapabilityID             string
	CapabilityPreimage       []byte
	PlanID                   string
	PlanPreimage             []byte
	VerifierExecutableSHA256 string
}

type AuthorityBinding struct {
	SourceIdentity       string
	ToolchainIdentity    string
	DependencyIdentity   string
	ModuleCacheDirectory string
	GoVersion            string
	GOOS                 string
	GOARCH               string
	CGOEnabled           string
	EnvironmentProfile   string
	GoWorkPath           string
}

// DiscoveryRequest freezes the exact environment and writable run root shared
// by listing and execution. Discover must directly run the pinned Go executable
// with godiscovery.ClosedFields, no -e and no shell, then return a verified
// closed document. Non-zero exit, any stderr, decode error, or drift is an error.
type DiscoveryRequest struct {
	GoExecutable     string
	WorkingDirectory string
	Environment      []gorunner.EnvironmentVariable
	PackagePatterns  []string
	RunRoot          string
}

type DiscoveryResult struct {
	ExitCode    int
	Commitments godiscovery.Commitments
}

type boundedBuffer struct {
	buffer   bytes.Buffer
	limit    int
	exceeded bool
}

func newBoundedBuffer(limit int) *boundedBuffer { return &boundedBuffer{limit: limit} }

func (buffer *boundedBuffer) Write(value []byte) (int, error) {
	if len(value) > buffer.limit-buffer.buffer.Len() {
		buffer.exceeded = true
		return 0, ErrDiscoveryLimit
	}
	return buffer.buffer.Write(value)
}

func (buffer *boundedBuffer) bytes() []byte { return buffer.buffer.Bytes() }

type ModuleMode string

const (
	ModuleReadonly  ModuleMode = "MODULE_READONLY"
	ModuleWorkspace ModuleMode = "WORKSPACE"
	ModuleVendor    ModuleMode = "VENDOR"
)

// Config contains no ambient defaults. Every filesystem and toolchain value
// that can affect the child is explicit and validated before authority is
// acquired. No provider run is possible without a complete authority lease.
type Config struct {
	ExperimentalEnabled        bool
	ExplicitTrustedLocalAction bool
	RepositoryRoot             string
	WorkingDirectory           string
	TemporaryParent            string
	GoExecutable               string
	GOROOT                     string
	GOOS                       string
	GOARCH                     string
	ModuleMode                 ModuleMode
	Packages                   []string
	Timeout                    time.Duration
	OutputLimitBytes           int64
	Authority                  ExecutionAuthority
}

type Execution string

const (
	ExecutionPassed     Execution = "PASSED"
	ExecutionFailed     Execution = "FAILED"
	ExecutionIncomplete Execution = "INCOMPLETE"
)

type Transcript struct {
	Qualification              string
	Scope                      string
	Coverage                   string
	Persistence                string
	LPCVQualified              bool
	CanonicalTerminalSupported bool
	Execution                  Execution
	Authority                  AuthorityBinding
	Discovery                  godiscovery.Document
	DiscoveryCanonical         []byte
	ConformanceContext         ConformanceContext
	Runner                     gorunner.Result
	Observation                gotest.Observation
	RunnerError                error
	DecodeError                error
	AuthorityError             error
	CleanupError               error
	EphemeralDeletionComplete  bool
	Receipt                    Receipt
}

type ConformanceContext struct {
	ActualEnvironmentSHA256  string   `json:"actualEnvironmentSha256"`
	CapabilityID             string   `json:"capabilityId"`
	CapabilityPreimageBase64 string   `json:"capabilityPreimageBase64"`
	DiscoveredPackagePaths   []string `json:"discoveredPackagePaths"`
	DiscoveryID              string   `json:"discoveryId"`
	GOARCH                   string   `json:"goarch"`
	GOOS                     string   `json:"goos"`
	ListedPackages           []string `json:"listedPackages"`
	Nonce                    string   `json:"nonce"`
	PlanID                   string   `json:"planId"`
	PlanPreimageBase64       string   `json:"planPreimageBase64"`
	RequestedPackagePatterns []string `json:"requestedPackagePatterns"`
	RequestedRunnerPackages  []string `json:"requestedRunnerPackages"`
	ToolchainID              string   `json:"toolchainId"`
	VerifierExecutableSHA256 string   `json:"verifierExecutableSha256"`
}

// leaseSentinel names the single sentinel an authority-layer error propagates.
// A recognised ErrAuthorityDrift survives; every other error, including a nil
// one, maps to ErrAuthorityUnavailable so no untyped error reaches the caller
// and no chain carries two named sentinels.
func leaseSentinel(err error) error {
	if errors.Is(err, ErrAuthorityDrift) {
		return ErrAuthorityDrift
	}
	return ErrAuthorityUnavailable
}

// Execute runs a trusted repository's explicit package selection with a
// minimal child environment. Returned runner bytes, normalized observations,
// and canonical receipt are caller-owned transient values.
func Execute(ctx context.Context, config Config) (transcript Transcript, returnErr error) {
	config.Packages = append([]string(nil), config.Packages...)
	transcript = Transcript{
		Qualification: QualificationExperimentalTranscript,
		Scope:         ScopeUnknown,
		Coverage:      CoverageNone,
		Persistence:   PersistenceNone,
		Execution:     ExecutionIncomplete,
	}
	resolved, err := validateConfig(config)
	if err != nil {
		return transcript, err
	}

	runRoot, err := newRunRoot(resolved.temporaryParent, resolved.temporaryParentIdentity, resolved.repositoryRoot)
	if err != nil {
		return transcript, fmt.Errorf("%w: create run root", ErrInvalidConfig)
	}
	defer func() {
		cleanupErr := runRoot.remove()
		transcript.CleanupError = cleanupErr
		transcript.EphemeralDeletionComplete = cleanupErr == nil
		if cleanupErr != nil {
			transcript.Execution = ExecutionIncomplete
			returnErr = errors.Join(returnErr, cleanupErr)
		}
	}()

	authorityContext, cancelAuthority := context.WithTimeout(ctx, config.Timeout)
	lease, err := config.Authority.Acquire(authorityContext, AuthorityRequest{
		RepositoryRoot:     resolved.repositoryRoot,
		WorkingDirectory:   resolved.workingDirectory,
		GoExecutable:       resolved.goExecutable,
		GOROOT:             resolved.goRoot,
		ExpectedGoVersion:  GoVersion,
		GOOS:               config.GOOS,
		GOARCH:             config.GOARCH,
		CGOEnabled:         "0",
		EnvironmentProfile: EnvironmentProfile,
		ModuleMode:         config.ModuleMode,
		Packages:           append([]string(nil), config.Packages...),
	})
	cancelAuthority()
	if err != nil || lease == nil {
		return transcript, fmt.Errorf("%w: acquire complete execution binding", ErrAuthorityUnavailable)
	}
	released := false
	defer func() {
		if !released {
			if releaseErr := releaseWithTimeout(lease); releaseErr != nil {
				transcript.AuthorityError = fmt.Errorf("%w: release execution authority", ErrAuthorityDrift)
				transcript.Execution = ExecutionIncomplete
				returnErr = errors.Join(returnErr, transcript.AuthorityError)
			}
		}
	}()
	transcript.Authority = lease.Binding()
	moduleCache, goWork, err := validateBinding(transcript.Authority, config.ModuleMode, resolved.repositoryRoot, runRoot.path)
	if err != nil {
		return transcript, err
	}
	environment, artifactRoot, err := createEnvironment(runRoot, moduleCache, goWork, resolved, config.ModuleMode)
	if err != nil {
		return transcript, err
	}
	if err := runRoot.validateLayout(); err != nil {
		return transcript, err
	}
	stdout := newBoundedBuffer(godiscovery.MaxDiscoveryBytes)
	stderr := newBoundedBuffer(0)
	authorityContext, cancelAuthority = context.WithTimeout(ctx, config.Timeout)
	discoveryResult, err := lease.Discover(authorityContext, DiscoveryRequest{
		GoExecutable:     resolved.goExecutable,
		WorkingDirectory: resolved.workingDirectory,
		Environment:      append([]gorunner.EnvironmentVariable(nil), environment...),
		PackagePatterns:  append([]string(nil), config.Packages...),
		RunRoot:          runRoot.path,
	}, stdout, stderr)
	cancelAuthority()
	if err != nil || stdout.exceeded || stderr.exceeded {
		return transcript, fmt.Errorf("%w: discovery acquisition failed", leaseSentinel(err))
	}
	document, discoveryCanonical, discoveredPackages, requestedPackages, err := composeDiscovery(
		discoveryResult, stdout.bytes(), stderr.bytes(), transcript.Authority, environment, config,
		filepath.Join(runRoot.path, "go-build-cache"),
	)
	if err != nil {
		return transcript, err
	}
	transcript.Discovery = document
	transcript.DiscoveryCanonical = discoveryCanonical
	if err := runRoot.validateLayout(); err != nil {
		return transcript, err
	}
	authorityContext, cancelAuthority = context.WithTimeout(ctx, config.Timeout)
	err = lease.Revalidate(authorityContext)
	cancelAuthority()
	if err != nil {
		return transcript, fmt.Errorf("%w: pre-launch revalidation", leaseSentinel(err))
	}
	if err := runRoot.validateLayout(); err != nil {
		return transcript, err
	}
	var receiptLease ReceiptIdentityLease
	var receiptPre ReceiptIdentityObservation
	var ok bool
	receiptLease, ok = lease.(ReceiptIdentityLease)
	if !ok {
		return transcript, fmt.Errorf("%w: canonical identity observations are unavailable", ErrAuthorityUnavailable)
	}
	authorityContext, cancelAuthority = context.WithTimeout(ctx, config.Timeout)
	receiptPre, err = receiptLease.ObserveReceiptIdentities(authorityContext)
	cancelAuthority()
	if err != nil || !validLowerDigest(receiptPre.SourceSHA256) || !validLowerDigest(receiptPre.ToolchainSHA256) ||
		transcript.Authority.SourceIdentity != "workspace-source:sha256:"+receiptPre.SourceSHA256 {
		return transcript, fmt.Errorf("%w: canonical pre-identities are unavailable", ErrAuthorityUnavailable)
	}
	authorityContext, cancelAuthority = context.WithTimeout(ctx, config.Timeout)
	canonicalBinding, err := receiptLease.BindCanonical(authorityContext, CanonicalBindingRequest{
		DiscoveryID: transcript.Discovery.ID, EnvironmentSHA256: transcript.Discovery.EnvironmentSHA256,
		PackagePatterns:    append([]string(nil), config.Packages...),
		CWDPathSHA256:      nativePathDigest(runtime.GOOS, resolved.workingDirectory),
		DependencyIdentity: transcript.Authority.DependencyIdentity, ModuleMode: config.ModuleMode,
		SourceIdentity: transcript.Authority.SourceIdentity, ToolchainIdentity: transcript.Authority.ToolchainIdentity,
		Environment: canonicalEnvironment(environment),
	})
	cancelAuthority()
	if err != nil || !validPrefixedDigest(canonicalBinding.CapabilityID, "go-live-capability:sha256:") ||
		!validPrefixedDigest(canonicalBinding.PlanID, "go-live-plan:sha256:") {
		return transcript, fmt.Errorf("%w: canonical plan binding failed", ErrAuthorityUnavailable)
	}
	var nonceBytes [16]byte
	if _, err := rand.Read(nonceBytes[:]); err != nil {
		return transcript, fmt.Errorf("%w: attempt nonce unavailable", ErrAuthorityUnavailable)
	}
	nonce := hex.EncodeToString(nonceBytes[:])
	listedPackages := make([]string, len(transcript.Discovery.Packages))
	for index, pack := range transcript.Discovery.Packages {
		listedPackages[index] = pack.ID
	}
	sort.Strings(listedPackages)
	actualEnvironmentSHA256, err := environmentDigest(environment)
	if err != nil || actualEnvironmentSHA256 != transcript.Discovery.EnvironmentSHA256 {
		return transcript, fmt.Errorf("%w: pre-exec environment changed", ErrAuthorityDrift)
	}
	transcript.ConformanceContext = ConformanceContext{
		ActualEnvironmentSHA256: actualEnvironmentSHA256, CapabilityID: canonicalBinding.CapabilityID,
		CapabilityPreimageBase64: base64.RawStdEncoding.EncodeToString(canonicalBinding.CapabilityPreimage),
		DiscoveredPackagePaths:   append([]string(nil), discoveredPackages...), DiscoveryID: transcript.Discovery.ID,
		GOARCH: runtime.GOARCH, GOOS: runtime.GOOS,
		ListedPackages: listedPackages, Nonce: nonce, PlanID: canonicalBinding.PlanID,
		PlanPreimageBase64:       base64.RawStdEncoding.EncodeToString(canonicalBinding.PlanPreimage),
		RequestedPackagePatterns: append([]string(nil), config.Packages...),
		RequestedRunnerPackages:  append([]string(nil), requestedPackages...), ToolchainID: transcript.Authority.ToolchainIdentity,
		VerifierExecutableSHA256: canonicalBinding.VerifierExecutableSHA256,
	}
	plan := gorunner.Plan{
		GoExecutable:     resolved.goExecutable,
		WorkingDirectory: resolved.workingDirectory,
		Environment:      environment,
		Packages:         append([]string(nil), config.Packages...),
		OutputLimitBytes: config.OutputLimitBytes,
		RetainOutput:     true,
		Timeout:          config.Timeout,
	}
	transcript.Runner, transcript.RunnerError = gorunner.Run(ctx, plan)
	if transcript.Runner.Started && transcript.Runner.Exited && transcript.Runner.ExitCode >= 0 &&
		transcript.Runner.Stdout.Drained && !transcript.Runner.OutputLimitExceeded {
		transcript.Observation, transcript.DecodeError = gotest.Observe(bytes.NewReader(transcript.Runner.Stdout.Data), gotest.Config{
			DiscoveredPackages: discoveredPackages,
			RequestedPackages:  requestedPackages,
			ArtifactRoot:       artifactRoot,
			Process: gotest.ProcessOutcome{
				Exited:   transcript.Runner.Exited,
				ExitCode: uint32(transcript.Runner.ExitCode),
			},
		})
	}
	if layoutErr := runRoot.validateLayout(); layoutErr != nil {
		transcript.AuthorityError = layoutErr
	}

	postContext, cancelPost := context.WithTimeout(context.Background(), postAuthorityTimeout)
	transcript.AuthorityError = errors.Join(transcript.AuthorityError, lease.Revalidate(postContext))
	cancelPost()
	var receiptPost ReceiptIdentityObservation
	postContext, cancelPost = context.WithTimeout(context.Background(), postAuthorityTimeout)
	receiptPost, err = receiptLease.ObserveReceiptIdentities(postContext)
	cancelPost()
	if err != nil || !validLowerDigest(receiptPost.SourceSHA256) || !validLowerDigest(receiptPost.ToolchainSHA256) {
		receiptPost = ReceiptIdentityObservation{}
		transcript.AuthorityError = errors.Join(transcript.AuthorityError, errors.New("canonical post-identities unavailable"))
	}
	if releaseErr := releaseWithTimeout(lease); releaseErr != nil {
		transcript.AuthorityError = errors.Join(transcript.AuthorityError, releaseErr)
	}
	released = true
	if transcript.AuthorityError != nil {
		transcript.AuthorityError = fmt.Errorf("%w: post-run revalidation", ErrAuthorityDrift)
	}

	transcript.Execution = classify(transcript)
	cleanupErr := runRoot.remove()
	transcript.CleanupError = cleanupErr
	transcript.EphemeralDeletionComplete = cleanupErr == nil
	if cleanupErr != nil {
		transcript.Execution = ExecutionIncomplete
		returnErr = errors.Join(returnErr, cleanupErr)
	}
	{
		decoder := DecoderComplete
		if !transcript.Runner.Stdout.Drained || transcript.Runner.OutputLimitExceeded {
			decoder = DecoderTruncated
		} else if transcript.DecodeError != nil || !transcript.Runner.Started || !transcript.Runner.Exited ||
			transcript.Runner.Cancelled || transcript.Runner.TimedOut || transcript.RunnerError != nil {
			decoder = DecoderRejected
		}
		transcript.Receipt, err = ComposeReceipt(ReceiptInput{
			ActualEnvironmentSHA256: actualEnvironmentSHA256,
			CapabilityID:            canonicalBinding.CapabilityID, PlanID: canonicalBinding.PlanID, Nonce: nonce,
			Discovery: transcript.Discovery, DiscoveryCanonical: transcript.DiscoveryCanonical,
			Events: transcript.Observation.Events, Observation: transcript.Observation,
			Runner: transcript.Runner, Decoder: decoder, SourcePreSHA256: receiptPre.SourceSHA256,
			SourcePostSHA256: receiptPost.SourceSHA256, ToolchainPreSHA256: receiptPre.ToolchainSHA256,
			ToolchainPostSHA256: receiptPost.ToolchainSHA256, EphemeralDeletionComplete: transcript.EphemeralDeletionComplete,
			ProviderFailure: transcript.RunnerError != nil || transcript.AuthorityError != nil || transcript.CleanupError != nil,
		})
		if err != nil {
			transcript.Execution = ExecutionIncomplete
			return transcript, errors.Join(returnErr, err)
		}
		transcript.CanonicalTerminalSupported = true
	}
	if transcript.AuthorityError != nil {
		return transcript, errors.Join(returnErr, transcript.AuthorityError)
	}
	return transcript, returnErr
}

func releaseWithTimeout(lease ExecutionLease) error {
	ctx, cancel := context.WithTimeout(context.Background(), postAuthorityTimeout)
	defer cancel()
	return lease.Release(ctx)
}

type resolvedConfig struct {
	repositoryRoot          string
	workingDirectory        string
	temporaryParent         string
	goExecutable            string
	goRoot                  string
	temporaryParentIdentity os.FileInfo
}

func validateConfig(config Config) (resolvedConfig, error) {
	if !config.ExperimentalEnabled {
		return resolvedConfig{}, ErrDisabled
	}
	if !config.ExplicitTrustedLocalAction {
		return resolvedConfig{}, ErrExplicitAction
	}
	if !supportedPlatform() {
		return resolvedConfig{}, ErrUnsupportedPlatform
	}
	if config.Authority == nil {
		return resolvedConfig{}, fmt.Errorf("%w: authority is required", ErrAuthorityUnavailable)
	}
	paths := []struct {
		label string
		value string
		dir   bool
	}{
		{"RepositoryRoot", config.RepositoryRoot, true},
		{"WorkingDirectory", config.WorkingDirectory, true},
		{"TemporaryParent", config.TemporaryParent, true},
		{"GOROOT", config.GOROOT, true},
		{"GoExecutable", config.GoExecutable, false},
	}
	resolvedPaths := make(map[string]string, len(paths))
	for _, candidate := range paths {
		resolved, err := resolvePath(candidate.value, candidate.dir)
		if err != nil {
			return resolvedConfig{}, invalid(candidate.label + " is not a resolved regular path")
		}
		resolvedPaths[candidate.label] = resolved
	}
	resolved := resolvedConfig{
		repositoryRoot:   resolvedPaths["RepositoryRoot"],
		workingDirectory: resolvedPaths["WorkingDirectory"],
		temporaryParent:  resolvedPaths["TemporaryParent"],
		goExecutable:     resolvedPaths["GoExecutable"],
		goRoot:           resolvedPaths["GOROOT"],
	}
	parentIdentity, err := os.Stat(resolved.temporaryParent)
	if err != nil {
		return resolvedConfig{}, invalid("TemporaryParent identity is unavailable")
	}
	resolved.temporaryParentIdentity = parentIdentity
	if !ownedByCurrentUser(parentIdentity) {
		return resolvedConfig{}, invalid("TemporaryParent must be owned by the current user")
	}
	if resolved.repositoryRoot != resolved.workingDirectory {
		return resolvedConfig{}, invalid("WorkingDirectory must equal the verified source materialization root")
	}
	if within(resolved.repositoryRoot, resolved.temporaryParent) {
		return resolvedConfig{}, invalid("TemporaryParent must be outside RepositoryRoot")
	}
	if resolved.goExecutable != filepath.Join(resolved.goRoot, "bin", executableName(config.GOOS)) {
		return resolvedConfig{}, invalid("GoExecutable must be the GOROOT Go executable")
	}
	if config.GOOS == "" || config.GOARCH == "" || !safeToken(config.GOOS) || !safeToken(config.GOARCH) {
		return resolvedConfig{}, invalid("GOOS and GOARCH must be non-empty lowercase ASCII tokens")
	}
	if config.GOOS != runtime.GOOS || config.GOARCH != runtime.GOARCH {
		return resolvedConfig{}, invalid("cross-platform execution is not admitted")
	}
	if config.ModuleMode != ModuleReadonly && config.ModuleMode != ModuleWorkspace && config.ModuleMode != ModuleVendor {
		return resolvedConfig{}, invalid("ModuleMode is not admitted")
	}
	if !validPackagePatterns(config.Packages) {
		return resolvedConfig{}, invalid("Packages are not admitted")
	}
	if !validListArgv(resolved.goExecutable, config.Packages) {
		return resolvedConfig{}, invalid("discovery argv exceeds the admitted bound")
	}
	if config.Timeout != gorunner.MaxRunTime {
		return resolvedConfig{}, invalid("Timeout must equal the frozen run limit")
	}
	if config.OutputLimitBytes != gorunner.MaxOutputBytes {
		return resolvedConfig{}, invalid("OutputLimitBytes must equal the frozen output limit")
	}
	return resolved, nil
}

func validateBinding(binding AuthorityBinding, moduleMode ModuleMode, repositoryRoot, runRoot string) (string, string, error) {
	if !validPrefixedDigest(binding.SourceIdentity, "workspace-source:sha256:") {
		return "", "", fmt.Errorf("%w: invalid source identity", ErrAuthorityUnavailable)
	}
	if !validPrefixedDigest(binding.ToolchainIdentity, "go-toolchain:sha256:") {
		return "", "", fmt.Errorf("%w: invalid toolchain identity", ErrAuthorityUnavailable)
	}
	if !validLowerDigest(binding.DependencyIdentity) {
		return "", "", fmt.Errorf("%w: invalid dependency identity", ErrAuthorityUnavailable)
	}
	if binding.GoVersion != GoVersion || binding.GOOS != runtime.GOOS || binding.GOARCH != runtime.GOARCH ||
		binding.CGOEnabled != "0" || binding.EnvironmentProfile != EnvironmentProfile {
		return "", "", fmt.Errorf("%w: exact Go 1.27.1 tuple binding is absent", ErrAuthorityUnavailable)
	}
	moduleCache, err := resolvePath(binding.ModuleCacheDirectory, true)
	if err != nil || within(repositoryRoot, moduleCache) || within(runRoot, moduleCache) {
		return "", "", fmt.Errorf("%w: dependency cache is unavailable or not independently materialized", ErrAuthorityUnavailable)
	}
	info, err := os.Stat(moduleCache)
	if err != nil || info.Mode().Perm()&0o222 != 0 {
		return "", "", fmt.Errorf("%w: dependency cache is not read-only", ErrAuthorityUnavailable)
	}
	goWork := "off"
	if moduleMode == ModuleWorkspace {
		goWork, err = resolvePath(binding.GoWorkPath, false)
		if err != nil || !within(repositoryRoot, goWork) || filepath.Base(goWork) != "go.work" {
			return "", "", fmt.Errorf("%w: verified workspace file is absent", ErrAuthorityUnavailable)
		}
	} else if binding.GoWorkPath != "" {
		return "", "", fmt.Errorf("%w: unexpected workspace file binding", ErrAuthorityUnavailable)
	}
	return moduleCache, goWork, nil
}

func composeDiscovery(result DiscoveryResult, rawStdout, rawStderr []byte, binding AuthorityBinding, environment []gorunner.EnvironmentVariable, config Config, goCache string) (godiscovery.Document, []byte, []string, []string, error) {
	if result.ExitCode != 0 || len(rawStderr) != 0 || len(rawStdout) == 0 || len(rawStdout) > godiscovery.MaxDiscoveryBytes {
		return godiscovery.Document{}, nil, nil, nil, fmt.Errorf("%w: discovery process was not clean and bounded", ErrAuthorityUnavailable)
	}
	if result.Commitments.SourceWSI != binding.SourceIdentity ||
		result.Commitments.DependencyMaterializationSHA256 != binding.DependencyIdentity {
		return godiscovery.Document{}, nil, nil, nil, fmt.Errorf("%w: discovery commitments are not authority-bound", ErrAuthorityUnavailable)
	}
	if err := validateCommitmentBounds(result.Commitments); err != nil {
		return godiscovery.Document{}, nil, nil, nil, err
	}
	environmentSHA256, err := environmentDigest(environment)
	if err != nil {
		return godiscovery.Document{}, nil, nil, nil, fmt.Errorf("%w: environment commitment failed", ErrAuthorityUnavailable)
	}
	expectedMode := godiscovery.ModuleModeModule
	if config.ModuleMode == ModuleWorkspace {
		expectedMode = godiscovery.ModuleModeWorkspace
	} else if config.ModuleMode == ModuleVendor {
		expectedMode = godiscovery.ModuleModeVendor
	}
	if result.Commitments.ModuleMode != expectedMode {
		return godiscovery.Document{}, nil, nil, nil, fmt.Errorf("%w: discovery module mode mismatch", ErrAuthorityUnavailable)
	}
	if err := validateGeneratedTestMainCommitments(result.Commitments, goCache); err != nil {
		return godiscovery.Document{}, nil, nil, nil, err
	}
	manifest, err := godiscovery.Decode(bytes.NewReader(rawStdout), result.Commitments)
	if err != nil {
		return godiscovery.Document{}, nil, nil, nil, fmt.Errorf("%w: discovery decode failed", ErrAuthorityUnavailable)
	}
	emptyStderr := sha256.Sum256(nil)
	document, canonical, err := godiscovery.Compose(manifest, godiscovery.DocumentInput{
		EnvironmentSHA256: environmentSHA256,
		PackagePatterns:   append([]string(nil), config.Packages...),
		RawStderrSHA256:   hex.EncodeToString(emptyStderr[:]),
		ToolchainID:       binding.ToolchainIdentity,
	})
	if err != nil || godiscovery.VerifyDocument(document, canonical) != nil {
		return godiscovery.Document{}, nil, nil, nil, fmt.Errorf("%w: discovery document composition failed", ErrAuthorityUnavailable)
	}
	discoveredSet := make(map[string]struct{}, len(document.Packages))
	for _, pack := range document.Packages {
		if _, duplicate := discoveredSet[pack.ImportPath]; duplicate {
			return godiscovery.Document{}, nil, nil, nil, fmt.Errorf("%w: duplicate discovered import path", ErrAuthorityUnavailable)
		}
		discoveredSet[pack.ImportPath] = struct{}{}
	}
	discovered := make([]string, 0, len(discoveredSet))
	for importPath := range discoveredSet {
		discovered = append(discovered, importPath)
	}
	sort.Strings(discovered)
	requested := append([]string(nil), document.RequestedRunnerPackages...)
	if !validDiscoveredPackages(discovered) || !validImportPaths(requested) || !isSubset(requested, discovered) {
		return godiscovery.Document{}, nil, nil, nil, fmt.Errorf("%w: discovery package relation is invalid", ErrAuthorityUnavailable)
	}
	return document, canonical, discovered, requested, nil
}

func validateCommitmentBounds(commitments godiscovery.Commitments) error {
	if len(commitments.Packages) == 0 || len(commitments.Packages) > godiscovery.MaxPackages ||
		len(commitments.ModulePaths) > 4*len(commitments.Packages) {
		return fmt.Errorf("%w: discovery commitment collection is unbounded", ErrAuthorityUnavailable)
	}
	return nil
}

func validateGeneratedTestMainCommitments(commitments godiscovery.Commitments, goCache string) error {
	cacheRoot, err := os.OpenRoot(goCache)
	if err != nil {
		return fmt.Errorf("%w: open run-owned Go cache", ErrAuthorityUnavailable)
	}
	defer cacheRoot.Close()
	for _, material := range commitments.Packages {
		for _, file := range material.Files {
			if file.Role != godiscovery.RoleGeneratedTestMain {
				continue
			}
			if file.Field != godiscovery.FieldGoFiles || !filepath.IsAbs(file.ListedName) || filepath.Clean(file.ListedName) != file.ListedName {
				return fmt.Errorf("%w: invalid generated testmain path", ErrAuthorityUnavailable)
			}
			relative, err := filepath.Rel(goCache, file.ListedName)
			if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				return fmt.Errorf("%w: generated testmain escaped Go cache", ErrAuthorityUnavailable)
			}
			info, err := cacheRoot.Lstat(relative)
			if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || fmt.Sprintf("%04o", info.Mode().Perm()) != file.Mode {
				return fmt.Errorf("%w: generated testmain identity mismatch", ErrAuthorityUnavailable)
			}
			handle, err := cacheRoot.Open(relative)
			if err != nil {
				return fmt.Errorf("%w: generated testmain unreadable", ErrAuthorityUnavailable)
			}
			openedInfo, statErr := handle.Stat()
			if statErr != nil || !os.SameFile(info, openedInfo) {
				_ = handle.Close()
				return fmt.Errorf("%w: generated testmain identity changed", ErrAuthorityUnavailable)
			}
			digest := sha256.New()
			_, copyErr := io.CopyBuffer(digest, handle, make([]byte, 32<<10))
			closeErr := handle.Close()
			if copyErr != nil || closeErr != nil || hex.EncodeToString(digest.Sum(nil)) != file.RawSHA256 {
				return fmt.Errorf("%w: generated testmain content mismatch", ErrAuthorityUnavailable)
			}
		}
	}
	return nil
}

func environmentDigest(environment []gorunner.EnvironmentVariable) (string, error) {
	if len(environment) == 0 || !sort.SliceIsSorted(environment, func(left, right int) bool {
		return environment[left].Name < environment[right].Name
	}) {
		return "", errors.New("environment is absent or unsorted")
	}
	var body bytes.Buffer
	body.WriteByte('[')
	previous := ""
	for index, variable := range environment {
		if variable.Name == "" || variable.Name <= previous {
			return "", errors.New("environment names are not unique")
		}
		previous = variable.Name
		if index != 0 {
			body.WriteByte(',')
		}
		valueDigest := sha256.Sum256([]byte(variable.Value))
		fmt.Fprintf(&body, `{"name":"%s","valueSha256":"%s"}`, variable.Name, hex.EncodeToString(valueDigest[:]))
	}
	body.WriteByte(']')
	digest := sha256.New()
	writeLength32(digest, len("go-environment"))
	_, _ = digest.Write([]byte("go-environment"))
	writeLength32(digest, len("go-environment/0"))
	_, _ = digest.Write([]byte("go-environment/0"))
	writeLength64(digest, body.Len())
	_, _ = digest.Write(body.Bytes())
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func canonicalEnvironment(environment []gorunner.EnvironmentVariable) []CanonicalEnvironmentVariable {
	result := make([]CanonicalEnvironmentVariable, len(environment))
	for index, value := range environment {
		digest := sha256.Sum256([]byte(value.Value))
		result[index] = CanonicalEnvironmentVariable{Name: value.Name, ValueSHA256: hex.EncodeToString(digest[:])}
	}
	return result
}

type hashWriter interface {
	Write([]byte) (int, error)
}

func writeLength32(writer hashWriter, length int) {
	var value [4]byte
	binary.BigEndian.PutUint32(value[:], uint32(length))
	_, _ = writer.Write(value[:])
}

func writeLength64(writer hashWriter, length int) {
	var value [8]byte
	binary.BigEndian.PutUint64(value[:], uint64(length))
	_, _ = writer.Write(value[:])
}

func validPrefixedDigest(value, prefix string) bool {
	return strings.HasPrefix(value, prefix) && validLowerDigest(strings.TrimPrefix(value, prefix))
}

func nativePathDigest(goos, path string) string {
	if !utf8.ValidString(path) {
		return ""
	}
	body := append(appendQuote([]byte(`{"goos":`), goos), `,"path":`...)
	digest, _ := godiscovery.Hash("go-native-path", "go-native-path/0", append(appendQuote(body, path), '}'))
	return digest
}

func validLowerDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func validImportPaths(values []string) bool {
	if len(values) == 0 || len(values) > gotest.MaxPackages || !sort.StringsAreSorted(values) {
		return false
	}
	previous := ""
	for _, value := range values {
		if value == previous || value == "" || len(value) > 4_096 || !utf8.ValidString(value) ||
			strings.HasPrefix(value, "-") || strings.ContainsAny(value, "@,\\*?[") {
			return false
		}
		for _, character := range value {
			if unicode.IsSpace(character) || unicode.IsControl(character) {
				return false
			}
		}
		for _, component := range strings.Split(value, "/") {
			if component == "" || component == "." || component == ".." || component == "..." {
				return false
			}
		}
		previous = value
	}
	return true
}

func validDiscoveredPackages(values []string) bool {
	if len(values) == 0 || len(values) > gotest.MaxPackages || !sort.StringsAreSorted(values) {
		return false
	}
	previous := ""
	for _, value := range values {
		if value == previous || value == "" || len(value) > gotest.MaxLineBytes || !utf8.ValidString(value) {
			return false
		}
		for _, character := range value {
			if unicode.IsControl(character) {
				return false
			}
		}
		previous = value
	}
	return true
}

func validPackagePatterns(values []string) bool {
	if len(values) == 1 && values[0] == "./..." {
		return true
	}
	if len(values) == 0 || len(values) > gorunner.MaxPackages {
		return false
	}
	return validImportPaths(values)
}

func validListArgv(goExecutable string, packages []string) bool {
	arguments := []string{"list", "-deps", "-test", "-json=" + godiscovery.ClosedFields}
	arguments = append(arguments, packages...)
	if len(arguments)+1 > 4_096 {
		return false
	}
	total := len(goExecutable) + 1
	for _, argument := range arguments {
		total += len(argument) + 1
	}
	return total <= gorunner.MaxArgvBytes
}

func isSubset(subset, superset []string) bool {
	index := 0
	for _, value := range subset {
		for index < len(superset) && superset[index] < value {
			index++
		}
		if index == len(superset) || superset[index] != value {
			return false
		}
	}
	return true
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func createEnvironment(runRoot *ownedRunRoot, moduleCache, goWork string, config resolvedConfig, moduleMode ModuleMode) ([]gorunner.EnvironmentVariable, string, error) {
	paths := map[string]string{
		"GOCACHE":  filepath.Join(runRoot.path, "go-build-cache"),
		"GOTMPDIR": filepath.Join(runRoot.path, "go-tmp"),
		"HOME":     filepath.Join(runRoot.path, "home"),
		"TEMP":     filepath.Join(runRoot.path, "tmp"),
		"TMP":      filepath.Join(runRoot.path, "tmp"),
		"TMPDIR":   filepath.Join(runRoot.path, "tmp"),
	}
	for _, name := range []string{"go-build-cache", "go-tmp", "home", "tmp"} {
		if err := runRoot.root.Mkdir(name, 0o700); err != nil {
			return nil, "", fmt.Errorf("%w: create ephemeral directory", ErrInvalidConfig)
		}
		if err := runRoot.bindChild(name); err != nil {
			return nil, "", err
		}
	}
	goFlags := "-mod=readonly"
	if moduleMode == ModuleVendor {
		goFlags = "-mod=vendor"
	}
	values := map[string]string{
		"CGO_ENABLED": "0",
		"GOCACHE":     paths["GOCACHE"],
		"GOENV":       "off",
		"GOFLAGS":     goFlags,
		"GOMODCACHE":  moduleCache,
		"GONOPROXY":   "",
		"GONOSUMDB":   "",
		"GOPRIVATE":   "",
		"GOPROXY":     "off",
		"GOROOT":      config.goRoot,
		"GOSUMDB":     "off",
		"GOTOOLCHAIN": "local",
		"GOTMPDIR":    paths["GOTMPDIR"],
		"GOVCS":       "*:off",
		"GOWORK":      goWork,
		"GOARCH":      runtime.GOARCH,
		"GOOS":        runtime.GOOS,
		"HOME":        paths["HOME"],
		"TEMP":        paths["TEMP"],
		"TMP":         paths["TMP"],
		"TMPDIR":      paths["TMPDIR"],
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	environment := make([]gorunner.EnvironmentVariable, 0, len(names))
	for _, name := range names {
		environment = append(environment, gorunner.EnvironmentVariable{Name: name, Value: values[name]})
	}
	return environment, filepath.Join(runRoot.path, "tmp"), nil
}

func classify(transcript Transcript) Execution {
	if !transcript.Runner.Started || transcript.RunnerError != nil || transcript.DecodeError != nil ||
		transcript.Runner.Cancelled || transcript.Runner.TimedOut || transcript.Runner.OutputLimitExceeded ||
		transcript.Runner.PipeWaitExpired || !transcript.Runner.ProcessCleanupDone ||
		!transcript.Runner.Stdout.Drained || !transcript.Runner.Stderr.Drained ||
		transcript.AuthorityError != nil || transcript.CleanupError != nil ||
		len(transcript.Observation.IncompleteReasons) != 0 {
		return ExecutionIncomplete
	}
	if transcript.Runner.ExitCode == 0 && !hasFailedTerminal(transcript.Observation) &&
		allRequestedPassed(transcript.Discovery.RequestedRunnerPackages, transcript.Observation) {
		return ExecutionPassed
	}
	if transcript.Runner.ExitCode != 0 && hasFailedTerminal(transcript.Observation) {
		return ExecutionFailed
	}
	return ExecutionIncomplete
}

// allRequestedPassed mirrors the receipt's PASSED rule: a requested runner
// package that ends in skip (for example, one with no test files) is not a pass.
func allRequestedPassed(requested []string, observation gotest.Observation) bool {
	status := make(map[string]string, len(observation.Packages))
	for _, pkg := range observation.Packages {
		status[pkg.Name] = pkg.Status
	}
	for _, name := range requested {
		if status[name] != "pass" {
			return false
		}
	}
	return len(requested) != 0
}

func hasFailedTerminal(observation gotest.Observation) bool {
	for _, pkg := range observation.Packages {
		if pkg.Status == "fail" {
			return true
		}
		for _, test := range pkg.Tests {
			if test.Status == "fail" {
				return true
			}
		}
	}
	for _, build := range observation.Builds {
		if build.Status == "fail" {
			return true
		}
	}
	return false
}

type ownedRunRoot struct {
	parent       *os.Root
	root         *os.Root
	name         string
	path         string
	rootIdentity os.FileInfo
	children     map[string]os.FileInfo
	closed       bool
	done         bool
}

func newRunRoot(parentPath string, expectedParent os.FileInfo, repositoryRoot string) (*ownedRunRoot, error) {
	parent, err := os.OpenRoot(parentPath)
	if err != nil {
		return nil, err
	}
	actualParent, err := parent.Stat(".")
	if err != nil || expectedParent == nil || !os.SameFile(expectedParent, actualParent) {
		_ = parent.Close()
		return nil, errors.New("temporary parent identity changed")
	}
	for attempt := 0; attempt < runDirectoryRetries; attempt++ {
		var randomBytes [runDirectoryNameBytes]byte
		if _, err := rand.Read(randomBytes[:]); err != nil {
			_ = parent.Close()
			return nil, err
		}
		name := "corvint-go-live-" + hex.EncodeToString(randomBytes[:])
		if err := parent.Mkdir(name, 0o700); errors.Is(err, os.ErrExist) {
			continue
		} else if err != nil {
			_ = parent.Close()
			return nil, err
		}
		path := filepath.Join(parentPath, name)
		if within(repositoryRoot, path) {
			_ = parent.Remove(name)
			_ = parent.Close()
			return nil, errors.New("run root is inside repository")
		}
		root, err := parent.OpenRoot(name)
		if err != nil {
			_ = parent.Remove(name)
			_ = parent.Close()
			return nil, err
		}
		nameInfo, nameErr := parent.Lstat(name)
		rootInfo, rootErr := root.Stat(".")
		if nameErr != nil || rootErr != nil || nameInfo.Mode()&os.ModeSymlink != 0 || !nameInfo.IsDir() ||
			nameInfo.Mode().Perm() != 0o700 || !ownedByCurrentUser(nameInfo) || !os.SameFile(nameInfo, rootInfo) {
			_ = root.Close()
			_ = parent.RemoveAll(name)
			_ = parent.Close()
			return nil, errors.New("run root identity could not be bound")
		}
		return &ownedRunRoot{
			parent: parent, root: root, name: name, path: path,
			rootIdentity: nameInfo, children: make(map[string]os.FileInfo),
		}, nil
	}
	_ = parent.Close()
	return nil, errors.New("random run directory collision")
}

func (runRoot *ownedRunRoot) remove() error {
	if runRoot == nil || runRoot.done {
		return nil
	}
	if runRoot.closed {
		return ErrCleanupIncomplete
	}
	if err := runRoot.validateRootIdentity(); err != nil {
		runRoot.closed = true
		return errors.Join(ErrCleanupIncomplete, err, runRoot.root.Close(), runRoot.parent.Close())
	}
	rootErr := runRoot.root.Close()
	var removeErr error
	for attempt := 0; attempt < runDirectoryRetries; attempt++ {
		removeErr = runRoot.parent.RemoveAll(runRoot.name)
		if removeErr == nil {
			if _, statErr := runRoot.parent.Lstat(runRoot.name); errors.Is(statErr, os.ErrNotExist) {
				break
			} else if statErr != nil {
				removeErr = statErr
			} else {
				removeErr = errors.New("run root remained after deletion")
			}
		}
		if info, statErr := runRoot.parent.Lstat(runRoot.name); statErr == nil {
			if !os.SameFile(runRoot.rootIdentity, info) {
				removeErr = errors.Join(removeErr, errors.New("run root identity changed during deletion"))
				break
			}
		} else if !errors.Is(statErr, os.ErrNotExist) {
			removeErr = errors.Join(removeErr, statErr)
			break
		}
	}
	closeErr := runRoot.parent.Close()
	runRoot.closed = true
	if rootErr != nil || removeErr != nil || closeErr != nil {
		return errors.Join(ErrCleanupIncomplete, rootErr, removeErr, closeErr)
	}
	runRoot.done = true
	return nil
}

func (runRoot *ownedRunRoot) validateLayout() error {
	if err := runRoot.validateRootIdentity(); err != nil {
		return err
	}
	directory, err := runRoot.root.Open(".")
	if err != nil {
		return fmt.Errorf("%w: open ephemeral root", ErrAuthorityDrift)
	}
	entries, readErr := directory.ReadDir(len(runRoot.children) + 1)
	closeErr := directory.Close()
	if (readErr != nil && !errors.Is(readErr, io.EOF)) || closeErr != nil || len(entries) != len(runRoot.children) {
		return fmt.Errorf("%w: ephemeral directory set changed", ErrAuthorityDrift)
	}
	for _, entry := range entries {
		if _, admitted := runRoot.children[entry.Name()]; !admitted {
			return fmt.Errorf("%w: unexpected ephemeral entry", ErrAuthorityDrift)
		}
	}
	for name, identity := range runRoot.children {
		info, err := runRoot.root.Lstat(name)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm() != 0o700 ||
			!ownedByCurrentUser(info) || !os.SameFile(identity, info) {
			return fmt.Errorf("%w: ephemeral directory identity changed", ErrAuthorityDrift)
		}
	}
	return nil
}

func (runRoot *ownedRunRoot) validateRootIdentity() error {
	rootInfo, err := runRoot.parent.Lstat(runRoot.name)
	openedInfo, openedErr := runRoot.root.Stat(".")
	if err != nil || openedErr != nil || rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() || rootInfo.Mode().Perm() != 0o700 ||
		!ownedByCurrentUser(rootInfo) || runRoot.rootIdentity == nil || !os.SameFile(runRoot.rootIdentity, rootInfo) ||
		!os.SameFile(runRoot.rootIdentity, openedInfo) || !os.SameFile(rootInfo, openedInfo) {
		return fmt.Errorf("%w: run root identity changed", ErrAuthorityDrift)
	}
	return nil
}

func (runRoot *ownedRunRoot) bindChild(name string) error {
	info, err := runRoot.root.Lstat(name)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm() != 0o700 || !ownedByCurrentUser(info) {
		return fmt.Errorf("%w: ephemeral directory could not be bound", ErrAuthorityDrift)
	}
	if _, exists := runRoot.children[name]; exists {
		return fmt.Errorf("%w: duplicate ephemeral directory", ErrAuthorityDrift)
	}
	runRoot.children[name] = info
	return nil
}

func resolvePath(path string, directory bool) (string, error) {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return "", errors.New("path is not clean and absolute")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	resolved = filepath.Clean(resolved)
	info, err := os.Lstat(resolved)
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("path is unavailable or linked")
	}
	if directory && !info.IsDir() {
		return "", errors.New("path is not a directory")
	}
	if !directory && !info.Mode().IsRegular() {
		return "", errors.New("path is not a regular file")
	}
	return resolved, nil
}

func within(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func safeToken(value string) bool {
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '_' {
			return false
		}
	}
	return value != ""
}

func executableName(goos string) string {
	if goos == "windows" {
		return "go.exe"
	}
	return "go"
}

func invalid(detail string) error {
	return fmt.Errorf("%w: %s", ErrInvalidConfig, detail)
}
