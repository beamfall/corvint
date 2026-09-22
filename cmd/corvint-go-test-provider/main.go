package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	jsonv2 "encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/godiscovery"
	"github.com/Beamfall/corvint/internal/liveverify/gorunner"
	"github.com/Beamfall/corvint/internal/liveverify/parentverify"
	"github.com/Beamfall/corvint/internal/liveverify/provider"
)

const (
	maxBundleBytes     = 1 << 20
	maxVerifierBytes   = int64(256 << 20)
	attachmentProfile  = "corvint-go-live-authority-attachment/0"
	requestProfile     = "corvint-go-live-authority-request/0"
	responseProfile    = "corvint-go-live-authority-response/0"
	attachmentArgument = "--corvint-go-live-authority"
)

var lowerDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var leasePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,256}$`)
var challengePattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// authorityBundle pins a live parent-verifier attachment and the exact
// execution request. It deliberately contains no semantic identity. WSI,
// dependency, toolchain, capability, and plan identities come only from the
// live verifier lease for this execution.
type authorityBundle struct {
	Profile                  string   `json:"profile"`
	VerifierExecutable       string   `json:"verifierExecutable"`
	VerifierExecutableSHA256 string   `json:"verifierExecutableRawSha256"`
	GitExecutable            string   `json:"gitExecutable"`
	GitExecutableSHA256      string   `json:"gitExecutableRawSha256"`
	GoExecutable             string   `json:"goExecutable"`
	GoWorkPath               string   `json:"goWorkPath"`
	GOARCH                   string   `json:"goarch"`
	GOOS                     string   `json:"goos"`
	GOROOT                   string   `json:"goroot"`
	ModuleCacheDirectory     string   `json:"moduleCacheDirectory"`
	ModuleMode               string   `json:"moduleMode"`
	OutputLimitBytes         int64    `json:"outputLimitBytes"`
	Packages                 []string `json:"packages"`
	RepositoryRoot           string   `json:"repositoryRoot"`
	TemporaryParent          string   `json:"temporaryParent"`
	TimeoutMilliseconds      int64    `json:"timeoutMilliseconds"`
}

type attachmentRequest struct {
	Profile                  string                            `json:"profile"`
	VerifierExecutableSHA256 string                            `json:"verifierExecutableRawSha256"`
	Operation                string                            `json:"operation"`
	Phase                    string                            `json:"phase"`
	Ordinal                  string                            `json:"ordinal"`
	Challenge                string                            `json:"challenge"`
	ParentCapabilitySHA256   string                            `json:"parentCapabilitySha256"`
	LeaseID                  string                            `json:"leaseId"`
	Authority                *provider.AuthorityRequest        `json:"authorityRequest"`
	Discovery                *provider.DiscoveryRequest        `json:"discoveryRequest"`
	Canonical                *provider.CanonicalBindingRequest `json:"canonicalBindingRequest"`
	Bundle                   *authorityBundle                  `json:"authorityBundle"`
}

type attachmentResponse struct {
	Profile         string                               `json:"profile"`
	RequestSHA256   string                               `json:"requestSha256"`
	VerifierSHA256  string                               `json:"verifierExecutableRawSha256"`
	Operation       string                               `json:"operation"`
	Phase           string                               `json:"phase"`
	Ordinal         string                               `json:"ordinal"`
	Challenge       string                               `json:"challenge"`
	Status          string                               `json:"status"`
	ErrorCode       string                               `json:"errorCode"`
	LeaseID         string                               `json:"leaseId"`
	Binding         *provider.AuthorityBinding           `json:"binding"`
	DiscoveryBase64 string                               `json:"discoveryStdoutBase64"`
	DiscoveryID     string                               `json:"discoveryId"`
	Commitments     *godiscovery.Commitments             `json:"commitments"`
	Observation     *provider.ReceiptIdentityObservation `json:"observation"`
	Canonical       *provider.CanonicalBinding           `json:"canonicalBinding"`
}

type localAuthority struct {
	bundle             authorityBundle
	productionVerifier bool
	mu                 sync.Mutex
	ordinal            uint64
	request            *provider.AuthorityRequest
}
type localLease struct {
	authority      *localAuthority
	leaseID        string
	binding        provider.AuthorityBinding
	mu             sync.Mutex
	discoveryID    string
	environmentSHA string
	environment    []provider.CanonicalEnvironmentVariable
}

func main() {
	if len(os.Args) == 5 && os.Args[1] == "--experimental" && os.Args[2] == "--trusted-local" && os.Args[3] == attachmentArgument {
		capability, ok := readParentCapability()
		if !ok {
			os.Exit(2)
		}
		ctx, cancel := providerContext()
		defer cancel()
		os.Exit(runProductionAuthority(ctx, os.Args[4], capability, os.Stdout))
	}
	if len(os.Args) >= 2 && os.Args[1] == "session" {
		os.Exit(runSessionCommand(os.Args[2:], os.Stdin, os.Stdout, os.Stderr))
	}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(arguments []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("corvint-go-test-provider", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	experimental := flags.Bool("experimental", false, "enable the experimental provider")
	trusted := flags.Bool("trusted-local", false, "confirm an explicit trusted-local action")
	bundlePath := flags.String("authority-bundle", "", "pinned live parent-verifier attachment")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		writeDiagnostic(stdout, "IDENTITY_MISMATCH", "IDENTITY", "ARGUMENTS_REJECTED")
		return 2
	}
	_ = stderr
	if !*experimental {
		writeDiagnostic(stdout, "IDENTITY_MISMATCH", "IDENTITY", "EXPERIMENT_DISABLED")
		return 2
	}
	if !*trusted {
		writeDiagnostic(stdout, "IDENTITY_MISMATCH", "IDENTITY", "EXPLICIT_TRUSTED_LOCAL_ACTION_REQUIRED")
		return 2
	}
	if !supportedProviderHost() {
		writeDiagnostic(stdout, "UNSUPPORTED_PLATFORM", "EXECUTION", "UNSUPPORTED_PLATFORM")
		return 2
	}
	if *bundlePath == "" {
		writeDiagnostic(stdout, "IDENTITY_MISMATCH", "IDENTITY", "AUTHORITY_BUNDLE_REQUIRED")
		return 2
	}
	bundle, err := readBundle(*bundlePath)
	if err != nil {
		writeDiagnostic(stdout, "IDENTITY_MISMATCH", "IDENTITY", "AUTHORITY_BUNDLE_REJECTED")
		return 2
	}
	ctx, cancel := providerContext()
	defer cancel()
	transcript, err := executeProvider(ctx, bundle)
	if transcript.CanonicalTerminalSupported && len(transcript.Receipt.Transcript) != 0 {
		if _, writeErr := stdout.Write(transcript.Receipt.Transcript); writeErr != nil {
			return 1
		}
		if err != nil {
			return 1
		}
		return 0
	}
	if err != nil {
		return emitProviderFailure(stdout, transcript, err)
	}
	writeDiagnostic(stdout, "EXECUTION_EXIT", "EXECUTION", "TERMINAL_RECEIPT_MISSING")
	return 1
}

func executeProvider(ctx context.Context, bundle authorityBundle) (provider.Transcript, error) {
	return provider.Execute(ctx, provider.Config{
		ExperimentalEnabled: true, ExplicitTrustedLocalAction: true,
		RepositoryRoot: bundle.RepositoryRoot, WorkingDirectory: bundle.RepositoryRoot,
		TemporaryParent: bundle.TemporaryParent, GoExecutable: bundle.GoExecutable,
		GOROOT: bundle.GOROOT, GOOS: bundle.GOOS, GOARCH: bundle.GOARCH,
		ModuleMode: provider.ModuleMode(bundle.ModuleMode), Packages: append([]string(nil), bundle.Packages...),
		Timeout: time.Duration(bundle.TimeoutMilliseconds) * time.Millisecond, OutputLimitBytes: bundle.OutputLimitBytes,
		Authority: &localAuthority{bundle: bundle, productionVerifier: true},
	})
}

func emitProviderFailure(output io.Writer, transcript provider.Transcript, err error) int {
	if transcript.Runner.Started {
		// An absent post-launch terminal means the coordinator could not close the
		// attempt. GLTP-V0-022 permits no synthetic run or runId:null error here.
		return 1
	}
	code, phase, detail := prelaunchDiagnostic(err)
	writeDiagnostic(output, code, phase, detail)
	return 1
}

func prelaunchDiagnostic(err error) (string, string, string) {
	switch {
	case errors.Is(err, provider.ErrUnsupportedPlatform):
		return "UNSUPPORTED_PLATFORM", "EXECUTION", "UNSUPPORTED_PLATFORM"
	case errors.Is(err, provider.ErrDiscoveryLimit):
		return "LIMIT_EXCEEDED", "LIMIT", "DISCOVERY_OUTPUT_LIMIT"
	case errors.Is(err, provider.ErrAuthorityUnavailable):
		return "IDENTITY_MISMATCH", "IDENTITY", "PARENT_AUTHORITY_UNAVAILABLE"
	case errors.Is(err, provider.ErrAuthorityDrift):
		return "IDENTITY_MISMATCH", "IDENTITY", "PARENT_AUTHORITY_DRIFT"
	default:
		return "IDENTITY_MISMATCH", "IDENTITY", "PROVIDER_PRELAUNCH_REJECTED"
	}
}

func readBundle(path string) (authorityBundle, error) {
	if !cleanAbsolute(path) {
		return authorityBundle{}, errors.New("bundle path is not clean and absolute")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || resolved != path {
		return authorityBundle{}, errors.New("bundle path contains a symbolic link")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o600 || !safeAttachmentFile(info) {
		return authorityBundle{}, errors.New("bundle is not a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return authorityBundle{}, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) || opened.Mode() != info.Mode() {
		return authorityBundle{}, errors.New("bundle identity changed")
	}
	content, err := io.ReadAll(io.LimitReader(file, maxBundleBytes+1))
	if err != nil || len(content) == 0 || len(content) > maxBundleBytes {
		return authorityBundle{}, errors.New("bundle size is invalid")
	}
	var bundle authorityBundle
	if err := jsonv2.Unmarshal(content, &bundle, jsonv2.RejectUnknownMembers(true)); err != nil {
		return authorityBundle{}, err
	}
	canonical, err := canonicalJSON(bundle)
	if err != nil || !bytes.Equal(canonical, content) {
		return authorityBundle{}, errors.New("bundle is not canonical")
	}
	if bundle.Profile != attachmentProfile || bundle.TimeoutMilliseconds <= 0 || bundle.OutputLimitBytes <= 0 || len(bundle.Packages) == 0 ||
		!cleanAbsolute(bundle.VerifierExecutable) || !lowerDigestPattern.MatchString(bundle.VerifierExecutableSHA256) ||
		!cleanAbsolute(bundle.GitExecutable) || !lowerDigestPattern.MatchString(bundle.GitExecutableSHA256) ||
		!cleanAbsolute(bundle.RepositoryRoot) || !cleanAbsolute(bundle.TemporaryParent) || !cleanAbsolute(bundle.GoExecutable) ||
		!cleanAbsolute(bundle.GOROOT) || !cleanAbsolute(bundle.ModuleCacheDirectory) {
		return authorityBundle{}, errors.New("bundle is incomplete")
	}
	if digest, err := regularFileDigest(bundle.VerifierExecutable); err != nil || digest != bundle.VerifierExecutableSHA256 {
		return authorityBundle{}, errors.New("verifier executable identity mismatch")
	}
	if digest, err := regularFileDigest(bundle.GitExecutable); err != nil || digest != bundle.GitExecutableSHA256 {
		return authorityBundle{}, errors.New("Git executable identity mismatch")
	}
	return bundle, nil
}

func (authority *localAuthority) Acquire(ctx context.Context, request provider.AuthorityRequest) (provider.ExecutionLease, error) {
	b := authority.bundle
	if request.RepositoryRoot != b.RepositoryRoot || request.WorkingDirectory != b.RepositoryRoot ||
		request.GoExecutable != b.GoExecutable || request.GOROOT != b.GOROOT || request.GOOS != b.GOOS || request.GOARCH != b.GOARCH ||
		request.ExpectedGoVersion != provider.GoVersion || request.CGOEnabled != "0" || request.EnvironmentProfile != provider.EnvironmentProfile ||
		string(request.ModuleMode) != b.ModuleMode || !equalStrings(request.Packages, b.Packages) {
		return nil, errors.New("authority request differs from attachment")
	}
	authority.mu.Lock()
	copyRequest := request
	copyRequest.Packages = append([]string(nil), request.Packages...)
	authority.request = &copyRequest
	authority.mu.Unlock()
	response, err := authority.invoke(ctx, attachmentRequest{Profile: requestProfile, Operation: "ACQUIRE", Authority: &request})
	if err != nil || response.Binding == nil || !leasePattern.MatchString(response.LeaseID) {
		return nil, errors.Join(errors.New("parent authority acquire failed"), err)
	}
	return &localLease{authority: authority, leaseID: response.LeaseID, binding: *response.Binding}, nil
}

func (lease *localLease) Binding() provider.AuthorityBinding { return lease.binding }

func (lease *localLease) Discover(ctx context.Context, request provider.DiscoveryRequest, stdout, stderr io.Writer) (provider.DiscoveryResult, error) {
	response, err := lease.invoke(ctx, "DISCOVER", &request, nil)
	if err != nil || response.Commitments == nil || response.DiscoveryBase64 == "" {
		return provider.DiscoveryResult{}, errors.Join(errors.New("parent authority discovery failed"), err)
	}
	raw, err := base64.RawStdEncoding.DecodeString(response.DiscoveryBase64)
	if err != nil || len(raw) == 0 || len(raw) > godiscovery.MaxDiscoveryBytes {
		return provider.DiscoveryResult{}, errors.New("parent authority discovery bytes are invalid")
	}
	if _, err := stdout.Write(raw); err != nil {
		return provider.DiscoveryResult{}, err
	}
	_ = stderr
	lease.mu.Lock()
	lease.discoveryID = response.DiscoveryID
	lease.environment, lease.environmentSHA = canonicalAuthorityEnvironment(request.Environment)
	lease.mu.Unlock()
	return provider.DiscoveryResult{ExitCode: 0, Commitments: *response.Commitments}, nil
}

func (lease *localLease) Revalidate(ctx context.Context) error {
	_, err := lease.invoke(ctx, "REVALIDATE", nil, nil)
	return err
}

func (lease *localLease) ObserveReceiptIdentities(ctx context.Context) (provider.ReceiptIdentityObservation, error) {
	response, err := lease.invoke(ctx, "OBSERVE", nil, nil)
	if err != nil || response.Observation == nil || !lowerDigestPattern.MatchString(response.Observation.SourceSHA256) || !lowerDigestPattern.MatchString(response.Observation.ToolchainSHA256) {
		return provider.ReceiptIdentityObservation{}, errors.Join(errors.New("parent authority observation failed"), err)
	}
	return *response.Observation, nil
}

func (lease *localLease) BindCanonical(ctx context.Context, request provider.CanonicalBindingRequest) (provider.CanonicalBinding, error) {
	lease.mu.Lock()
	if lease.discoveryID == "" || lease.discoveryID != request.DiscoveryID ||
		lease.environmentSHA == "" || lease.environmentSHA != request.EnvironmentSHA256 ||
		!equalCanonicalEnvironment(lease.environment, request.Environment) {
		lease.mu.Unlock()
		return provider.CanonicalBinding{}, errors.New("parent canonical binding is detached from discovery")
	}
	lease.mu.Unlock()
	response, err := lease.invoke(ctx, "BIND_CANONICAL", nil, &request)
	if err != nil || response.Canonical == nil || !validPrefixed(response.Canonical.CapabilityID, "go-live-capability:sha256:") || !validPrefixed(response.Canonical.PlanID, "go-live-plan:sha256:") {
		return provider.CanonicalBinding{}, errors.Join(errors.New("parent canonical binding failed"), err)
	}
	return *response.Canonical, nil
}

func canonicalAuthorityEnvironment(environment []gorunner.EnvironmentVariable) ([]provider.CanonicalEnvironmentVariable, string) {
	values := make([]provider.CanonicalEnvironmentVariable, len(environment))
	rows := make([]any, len(environment))
	for index, variable := range environment {
		digest := sha256.Sum256([]byte(variable.Value))
		values[index] = provider.CanonicalEnvironmentVariable{Name: variable.Name, ValueSHA256: hex.EncodeToString(digest[:])}
		rows[index] = map[string]any{"name": variable.Name, "valueSha256": values[index].ValueSHA256}
	}
	body, err := canonicalJSON(rows)
	if err != nil {
		return nil, ""
	}
	return values, bareID("go-environment", "go-environment/0", body)
}

func equalCanonicalEnvironment(left, right []provider.CanonicalEnvironmentVariable) bool {
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

func (lease *localLease) Release(ctx context.Context) error {
	_, err := lease.invoke(ctx, "RELEASE", nil, nil)
	return err
}

func (lease *localLease) invoke(ctx context.Context, operation string, discovery *provider.DiscoveryRequest, canonical *provider.CanonicalBindingRequest) (attachmentResponse, error) {
	lease.mu.Lock()
	defer lease.mu.Unlock()
	return lease.authority.invoke(ctx, attachmentRequest{Profile: requestProfile, Operation: operation, LeaseID: lease.leaseID, Discovery: discovery, Canonical: canonical})
}

func (authority *localAuthority) invoke(ctx context.Context, request attachmentRequest) (attachmentResponse, error) {
	b := authority.bundle
	digest, err := regularFileDigest(b.VerifierExecutable)
	if err != nil || digest != b.VerifierExecutableSHA256 {
		return attachmentResponse{}, errors.New("verifier executable identity changed")
	}
	var challenge [16]byte
	if _, err := rand.Read(challenge[:]); err != nil {
		return attachmentResponse{}, errors.New("parent verifier challenge unavailable")
	}
	authority.mu.Lock()
	authority.ordinal++
	ordinal := authority.ordinal
	if authority.productionVerifier {
		bundleCopy := authority.bundle
		bundleCopy.Packages = append([]string(nil), authority.bundle.Packages...)
		request.Bundle = &bundleCopy
		if request.Authority == nil && authority.request != nil {
			requestCopy := *authority.request
			requestCopy.Packages = append([]string(nil), authority.request.Packages...)
			request.Authority = &requestCopy
		}
	}
	authority.mu.Unlock()
	request.VerifierExecutableSHA256 = b.VerifierExecutableSHA256
	request.Phase = request.Operation
	request.Ordinal = strconv.FormatUint(ordinal, 10)
	request.Challenge = hex.EncodeToString(challenge[:])
	parentCapability := make([]byte, 32)
	if _, err := rand.Read(parentCapability); err != nil {
		return attachmentResponse{}, errors.New("parent verifier capability unavailable")
	}
	capabilityDigest := sha256.Sum256(parentCapability)
	request.ParentCapabilitySHA256 = hex.EncodeToString(capabilityDigest[:])
	requestBytes, err := canonicalJSON(request)
	if err != nil {
		return attachmentResponse{}, err
	}
	if len(requestBytes) > maxBundleBytes {
		return attachmentResponse{}, errors.New("parent verifier request exceeds bound")
	}
	requestSHA256 := bareID("corvint-go-live-authority-request", requestProfile, requestBytes)
	attachmentEnvironment := []string{"CORVINT_GO_LIVE_AUTHORITY_PROTOCOL=" + responseProfile}
	result, err := runAuthorityCommand(ctx, b.VerifierExecutable, []string{"--experimental", "--trusted-local", attachmentArgument, base64.RawURLEncoding.EncodeToString(requestBytes)}, attachmentEnvironment, b.RepositoryRoot, time.Duration(b.TimeoutMilliseconds)*time.Millisecond, parentCapability)
	if err != nil || !result.Started || !result.Exited || !result.WaitCompleted || result.ExitCode != 0 || !result.ProcessCleanupDone || !result.PipesDrained || result.Cancelled || result.TimedOut || result.OutputLimitExceeded || len(result.Stderr) != 0 || len(result.Stdout) == 0 {
		return attachmentResponse{}, errors.Join(fmt.Errorf("%w: parent verifier transport failed", provider.ErrAuthorityUnavailable), err)
	}
	var response attachmentResponse
	if err := jsonv2.Unmarshal(result.Stdout, &response, jsonv2.RejectUnknownMembers(true)); err != nil {
		return attachmentResponse{}, errors.Join(fmt.Errorf("%w: parent verifier response is unreadable", provider.ErrAuthorityUnavailable), err)
	}
	canonical, err := canonicalJSON(response)
	if err != nil || !bytes.Equal(canonical, result.Stdout) || response.Profile != responseProfile || response.Operation != request.Operation ||
		response.RequestSHA256 != requestSHA256 || response.VerifierSHA256 != b.VerifierExecutableSHA256 ||
		response.Phase != request.Phase || response.Ordinal != request.Ordinal || response.Challenge != request.Challenge ||
		(request.Operation != "ACQUIRE" && response.LeaseID != request.LeaseID) {
		return attachmentResponse{}, fmt.Errorf("%w: parent verifier response is not canonical or attached", provider.ErrAuthorityUnavailable)
	}
	if response.Status != "OK" || response.ErrorCode != "" {
		return attachmentResponse{}, fmt.Errorf("%w: parent verifier rejected %s: %s", authoritySentinel(response.ErrorCode), request.Operation, response.ErrorCode)
	}
	if !validAttachmentResponse(request.Operation, response) {
		return attachmentResponse{}, fmt.Errorf("%w: parent verifier response has invalid operation shape", provider.ErrAuthorityUnavailable)
	}
	return response, nil
}

func runProductionAuthority(ctx context.Context, encoded string, parentCapability []byte, output io.Writer) int {
	if ctx == nil || output == nil || len(parentCapability) != 32 || len(os.Environ()) != 1 || os.Getenv("CORVINT_GO_LIVE_AUTHORITY_PROTOCOL") != responseProfile {
		return 2
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(raw) == 0 || len(raw) > maxBundleBytes {
		return 2
	}
	var request attachmentRequest
	if err := jsonv2.Unmarshal(raw, &request, jsonv2.RejectUnknownMembers(true)); err != nil ||
		request.Profile != requestProfile || request.Phase != request.Operation || request.Bundle == nil || request.Authority == nil ||
		!lowerDigestPattern.MatchString(request.VerifierExecutableSHA256) || request.VerifierExecutableSHA256 != request.Bundle.VerifierExecutableSHA256 ||
		!challengePattern.MatchString(request.Challenge) || !canonicalOrdinal(request.Ordinal) || !lowerDigestPattern.MatchString(request.ParentCapabilitySHA256) {
		return 2
	}
	capabilityDigest := sha256.Sum256(parentCapability)
	if request.ParentCapabilitySHA256 != hex.EncodeToString(capabilityDigest[:]) {
		return 2
	}
	canonical, err := canonicalJSON(request)
	if err != nil || !bytes.Equal(canonical, raw) {
		return 2
	}
	if err := validateProductionBundle(*request.Bundle); err != nil {
		return 2
	}
	config := parentConfig(*request.Bundle)
	runner := func(ctx context.Context, executable string, argv, environment []string, cwd string, timeout time.Duration) (parentverify.CommandResult, error) {
		result, runErr := runDirectCommand(ctx, executable, argv, environment, cwd, timeout)
		return parentverify.CommandResult{
			Stdout: result.Stdout, Stderr: result.Stderr, ExitCode: result.ExitCode,
			Started: result.Started, Exited: result.Exited, Cancelled: result.Cancelled, TimedOut: result.TimedOut,
			OutputLimitExceeded: result.OutputLimitExceeded, ProcessCleanupDone: result.ProcessCleanupDone,
			PipesDrained: result.PipesDrained, WaitCompleted: result.WaitCompleted,
		}, runErr
	}
	response := attachmentResponse{
		Profile: responseProfile, RequestSHA256: bareID("corvint-go-live-authority-request", requestProfile, raw),
		VerifierSHA256: request.VerifierExecutableSHA256, Operation: request.Operation, Phase: request.Phase,
		Ordinal: request.Ordinal, Challenge: request.Challenge, Status: "OK", LeaseID: request.LeaseID,
	}
	operationErr := executeProductionAuthority(ctx, request, config, runner, &response)
	if operationErr != nil {
		response.Status, response.ErrorCode = "REJECTED", productionErrorCode(operationErr)
		response.Binding, response.Commitments, response.Observation, response.Canonical = nil, nil, nil, nil
		response.DiscoveryBase64 = ""
		response.DiscoveryID = ""
	}
	encodedResponse, err := canonicalJSON(response)
	if err != nil {
		return 2
	}
	if _, err := output.Write(encodedResponse); err != nil {
		return 2
	}
	if operationErr != nil {
		// A semantic rejection is a successfully transported, canonical response.
		// The caller interprets its closed status and fails the provider phase.
		return 0
	}
	return 0
}

func executeProductionAuthority(ctx context.Context, request attachmentRequest, config parentverify.Config, runner parentverify.CommandRunner, response *attachmentResponse) error {
	switch request.Operation {
	case "ACQUIRE":
		if request.LeaseID != "" || request.Discovery != nil || request.Canonical != nil {
			return parentverify.ErrInvalid
		}
		snapshot, err := parentverify.Acquire(ctx, config, *request.Authority, runner)
		if err != nil {
			return err
		}
		response.LeaseID, response.Binding = snapshot.LeaseID, &snapshot.Binding
	case "DISCOVER":
		if request.Discovery == nil || request.Canonical != nil || !leasePattern.MatchString(request.LeaseID) {
			return parentverify.ErrInvalid
		}
		discovery, err := parentverify.Discover(ctx, config, *request.Authority, request.LeaseID, *request.Discovery, runner)
		if err != nil {
			return err
		}
		response.DiscoveryBase64 = base64.RawStdEncoding.EncodeToString(discovery.Raw)
		response.DiscoveryID = discovery.ID
		response.Commitments = &discovery.Commitments
	case "REVALIDATE", "RELEASE":
		if request.Discovery != nil || request.Canonical != nil || !leasePattern.MatchString(request.LeaseID) {
			return parentverify.ErrInvalid
		}
		_, err := parentverify.Revalidate(ctx, config, *request.Authority, request.LeaseID, runner)
		return err
	case "OBSERVE":
		if request.Discovery != nil || request.Canonical != nil || !leasePattern.MatchString(request.LeaseID) {
			return parentverify.ErrInvalid
		}
		snapshot, err := parentverify.Revalidate(ctx, config, *request.Authority, request.LeaseID, runner)
		if err != nil {
			return err
		}
		response.Observation = &provider.ReceiptIdentityObservation{SourceSHA256: snapshot.SourceObservation, ToolchainSHA256: snapshot.ToolchainObservation}
	case "BIND_CANONICAL":
		if request.Discovery != nil || request.Canonical == nil || !leasePattern.MatchString(request.LeaseID) {
			return parentverify.ErrInvalid
		}
		snapshot, err := parentverify.Revalidate(ctx, config, *request.Authority, request.LeaseID, runner)
		if err != nil {
			return err
		}
		binding, err := parentverify.BindCanonical(snapshot, config, *request.Canonical)
		if err != nil {
			return err
		}
		response.Canonical = &binding
	default:
		return parentverify.ErrInvalid
	}
	return nil
}

func parentConfig(bundle authorityBundle) parentverify.Config {
	return parentverify.Config{
		VerifierExecutable: bundle.VerifierExecutable, VerifierExecutableSHA256: bundle.VerifierExecutableSHA256,
		GitExecutable: bundle.GitExecutable, GitExecutableSHA256: bundle.GitExecutableSHA256,
		GoExecutable: bundle.GoExecutable, GoWorkPath: bundle.GoWorkPath, GOARCH: bundle.GOARCH, GOOS: bundle.GOOS,
		GOROOT: bundle.GOROOT, ModuleCacheDirectory: bundle.ModuleCacheDirectory, ModuleMode: provider.ModuleMode(bundle.ModuleMode),
		OutputLimitBytes: bundle.OutputLimitBytes, Packages: append([]string(nil), bundle.Packages...),
		RepositoryRoot: bundle.RepositoryRoot, TemporaryParent: bundle.TemporaryParent,
		Timeout: time.Duration(bundle.TimeoutMilliseconds) * time.Millisecond,
	}
}

func validateProductionBundle(bundle authorityBundle) error {
	if bundle.Profile != attachmentProfile || bundle.TimeoutMilliseconds != 1_800_000 || bundle.OutputLimitBytes != 8<<20 {
		return parentverify.ErrInvalid
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return err
	}
	actual, err := regularFileDigest(executable)
	if err != nil || actual != bundle.VerifierExecutableSHA256 {
		return parentverify.ErrDrift
	}
	return nil
}

func canonicalOrdinal(value string) bool {
	if value == "" || value == "0" || (len(value) > 1 && value[0] == '0') {
		return false
	}
	_, err := strconv.ParseUint(value, 10, 64)
	return err == nil
}

// authoritySentinel maps the parent's attachment sub-protocol errorCode onto the
// single named provider sentinel the requesting side propagates for it. Only
// AUTHORITY_DRIFT is an identity-drift refusal; the parent's own limit and
// invalid-request refusals are not this run's LIMIT phase.
func authoritySentinel(errorCode string) error {
	if errorCode == "AUTHORITY_DRIFT" {
		return provider.ErrAuthorityDrift
	}
	return provider.ErrAuthorityUnavailable
}

func productionErrorCode(err error) string {
	switch {
	case errors.Is(err, parentverify.ErrDrift):
		return "AUTHORITY_DRIFT"
	case errors.Is(err, parentverify.ErrLimit):
		return "LIMIT_EXCEEDED"
	case errors.Is(err, parentverify.ErrInvalid):
		return "INVALID_REQUEST"
	default:
		return "AUTHORITY_UNAVAILABLE"
	}
}

func validAttachmentResponse(operation string, response attachmentResponse) bool {
	baseEmpty := response.ErrorCode == ""
	switch operation {
	case "ACQUIRE":
		return baseEmpty && leasePattern.MatchString(response.LeaseID) && response.Binding != nil && response.DiscoveryBase64 == "" && response.DiscoveryID == "" && response.Commitments == nil && response.Observation == nil && response.Canonical == nil
	case "DISCOVER":
		return baseEmpty && leasePattern.MatchString(response.LeaseID) && response.Binding == nil && response.DiscoveryBase64 != "" && validPrefixed(response.DiscoveryID, "go-live-discovery:sha256:") && response.Commitments != nil && response.Observation == nil && response.Canonical == nil
	case "REVALIDATE", "RELEASE":
		return baseEmpty && leasePattern.MatchString(response.LeaseID) && response.Binding == nil && response.DiscoveryBase64 == "" && response.DiscoveryID == "" && response.Commitments == nil && response.Observation == nil && response.Canonical == nil
	case "OBSERVE":
		return baseEmpty && leasePattern.MatchString(response.LeaseID) && response.Binding == nil && response.DiscoveryBase64 == "" && response.DiscoveryID == "" && response.Commitments == nil && response.Observation != nil && response.Canonical == nil
	case "BIND_CANONICAL":
		return baseEmpty && leasePattern.MatchString(response.LeaseID) && response.Binding == nil && response.DiscoveryBase64 == "" && response.DiscoveryID == "" && response.Commitments == nil && response.Observation == nil && response.Canonical != nil
	default:
		return false
	}
}

func regularFileDigest(path string) (string, error) {
	if !cleanAbsolute(path) {
		return "", errors.New("path is not clean and absolute")
	}
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() < 0 || before.Size() > maxVerifierBytes {
		return "", errors.New("path is not a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) || opened.Mode() != before.Mode() || opened.Size() != before.Size() {
		return "", errors.New("file identity changed")
	}
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	after, err := file.Stat()
	if err != nil || !os.SameFile(opened, after) || after.Mode() != opened.Mode() || after.Size() != opened.Size() {
		return "", errors.New("file identity changed")
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func cleanAbsolute(path string) bool {
	return path != "" && filepath.IsAbs(path) && filepath.Clean(path) == path
}

func validPrefixed(value, prefix string) bool {
	return strings.HasPrefix(value, prefix) && lowerDigestPattern.MatchString(strings.TrimPrefix(value, prefix))
}

func writeDiagnostic(output io.Writer, code, phase, reason string) {
	body := []byte(fmt.Sprintf(`{"code":%q,"detail":%q,"phase":%q}`, code, reason, phase))
	detail := bareID("go-live-error-detail", "go-live-error-detail/0", body)
	_, _ = fmt.Fprintf(output, `{"code":%q,"detailSha256":%q,"phase":%q,"profile":"go-live-error/0","runId":null}`+"\n", code, detail, phase)
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

func canonicalJSON(value any) ([]byte, error) {
	return jsonv2.Marshal(value, jsonv2.Deterministic(true))
}

func prefixedID(kind, profile string, body []byte) string {
	return kind + ":sha256:" + bareID(kind, profile, body)
}

func bareID(kind, profile string, body []byte) string {
	digest := sha256.New()
	var length32 [4]byte
	var length64 [8]byte
	binary.BigEndian.PutUint32(length32[:], uint32(len(kind)))
	_, _ = digest.Write(length32[:])
	_, _ = digest.Write([]byte(kind))
	binary.BigEndian.PutUint32(length32[:], uint32(len(profile)))
	_, _ = digest.Write(length32[:])
	_, _ = digest.Write([]byte(profile))
	binary.BigEndian.PutUint64(length64[:], uint64(len(body)))
	_, _ = digest.Write(length64[:])
	_, _ = digest.Write(body)
	return hex.EncodeToString(digest.Sum(nil))
}
