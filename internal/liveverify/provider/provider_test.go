package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/godiscovery"
	"github.com/Beamfall/corvint/internal/liveverify/gorunner"
)

type realListModule struct {
	Path, Version, Time, Dir, GoMod, GoVersion, Sum, GoModSum string
	Main, Indirect                                            bool
	Replace                                                   *realListModule
}

type realListPackage struct {
	Dir, ImportPath, Name, ForTest string
	Match                          []string
	DepOnly                        bool
	Module                         *realListModule
	GoFiles, CgoFiles, CFiles, CXXFiles, MFiles, HFiles,
	FFiles, SFiles, SwigFiles, SwigCXXFiles, SysoFiles,
	EmbedFiles, TestGoFiles, XTestGoFiles, TestEmbedFiles,
	XTestEmbedFiles []string
}

type realAuthority struct {
	binding        AuthorityBinding
	repositoryRoot string
	goRoot         string
	sourceIdentity string
}

func (authority *realAuthority) Acquire(_ context.Context, _ AuthorityRequest) (ExecutionLease, error) {
	return &realLease{authority: authority}, nil
}

type realLease struct{ authority *realAuthority }

func (lease *realLease) Binding() AuthorityBinding { return lease.authority.binding }

func (lease *realLease) Discover(ctx context.Context, request DiscoveryRequest, stdout, stderr io.Writer) (DiscoveryResult, error) {
	arguments := []string{"list", "-deps", "-test", "-json=" + godiscovery.ClosedFields}
	arguments = append(arguments, request.PackagePatterns...)
	command := exec.CommandContext(ctx, request.GoExecutable, arguments...)
	command.Dir = request.WorkingDirectory
	for _, value := range request.Environment {
		command.Env = append(command.Env, value.Name+"="+value.Value)
	}
	var rawStdout, rawStderr bytes.Buffer
	command.Stdout = &rawStdout
	command.Stderr = &rawStderr
	err := command.Run()
	if rawStderr.Len() != 0 {
		_, _ = stderr.Write(rawStderr.Bytes())
	}
	if err != nil {
		return DiscoveryResult{}, err
	}
	commitments, err := realDiscoveryCommitments(
		rawStdout.Bytes(), lease.authority.binding, lease.authority.repositoryRoot,
		lease.authority.goRoot, request.RunRoot,
	)
	if err != nil {
		return DiscoveryResult{}, err
	}
	if _, err := stdout.Write(rawStdout.Bytes()); err != nil {
		return DiscoveryResult{}, err
	}
	return DiscoveryResult{ExitCode: 0, Commitments: commitments}, nil
}

func (lease *realLease) Revalidate(_ context.Context) error {
	identity, err := repositoryDigest(lease.authority.repositoryRoot)
	if err != nil {
		return err
	}
	if identity != lease.authority.sourceIdentity {
		return fmt.Errorf("%w: source changed", ErrAuthorityDrift)
	}
	return nil
}

func (lease *realLease) ObserveReceiptIdentities(_ context.Context) (ReceiptIdentityObservation, error) {
	source, err := repositoryDigest(lease.authority.repositoryRoot)
	if err != nil {
		return ReceiptIdentityObservation{}, err
	}
	contents, err := os.ReadFile(filepath.Join(lease.authority.goRoot, "bin", executableName(runtime.GOOS)))
	if err != nil {
		return ReceiptIdentityObservation{}, err
	}
	toolchain := sha256.Sum256(contents)
	return ReceiptIdentityObservation{SourceSHA256: source, ToolchainSHA256: hex.EncodeToString(toolchain[:])}, nil
}

func (lease *realLease) BindCanonical(_ context.Context, request CanonicalBindingRequest) (CanonicalBinding, error) {
	if request.DiscoveryID == "" || request.EnvironmentSHA256 == "" || len(request.PackagePatterns) == 0 {
		return CanonicalBinding{}, errors.New("canonical binding mismatch")
	}
	return CanonicalBinding{CapabilityID: "go-live-capability:sha256:" + strings.Repeat("c", 64), PlanID: "go-live-plan:sha256:" + strings.Repeat("d", 64)}, nil
}

func (lease *realLease) Release(_ context.Context) error { return nil }

type testAuthority struct {
	mu               sync.Mutex
	binding          AuthorityBinding
	repositoryRoot   string
	sourceIdentity   string
	revalidationErr  error
	postErr          error
	releaseErr       error
	discoveryMutator func(*DiscoveryResult, *[]byte)
	acquisitions     int
	revalidations    int
}

func (authority *testAuthority) Acquire(_ context.Context, request AuthorityRequest) (ExecutionLease, error) {
	authority.mu.Lock()
	defer authority.mu.Unlock()
	authority.acquisitions++
	if request.RepositoryRoot != authority.repositoryRoot {
		return nil, errors.New("unexpected repository")
	}
	if request.ExpectedGoVersion != GoVersion {
		return nil, errors.New("unexpected Go version")
	}
	return &testLease{authority: authority}, nil
}

type testLease struct {
	authority *testAuthority
	released  bool
}

func (lease *testLease) Binding() AuthorityBinding {
	return lease.authority.binding
}

func (lease *testLease) Discover(_ context.Context, request DiscoveryRequest, stdout, _ io.Writer) (DiscoveryResult, error) {
	result, raw, err := fixtureDiscovery(lease.authority.repositoryRoot, lease.authority.binding, request)
	if err == nil && lease.authority.discoveryMutator != nil {
		lease.authority.discoveryMutator(&result, &raw)
	}
	if err == nil {
		_, err = stdout.Write(raw)
	}
	return result, err
}

func (lease *testLease) Revalidate(_ context.Context) error {
	lease.authority.mu.Lock()
	defer lease.authority.mu.Unlock()
	lease.authority.revalidations++
	if lease.authority.revalidationErr != nil {
		return lease.authority.revalidationErr
	}
	if lease.authority.revalidations > 1 && lease.authority.postErr != nil {
		return lease.authority.postErr
	}
	identity, err := repositoryDigest(lease.authority.repositoryRoot)
	if err != nil {
		return err
	}
	if identity != lease.authority.sourceIdentity {
		return fmt.Errorf("%w: source changed", ErrAuthorityDrift)
	}
	return nil
}

func (lease *testLease) ObserveReceiptIdentities(_ context.Context) (ReceiptIdentityObservation, error) {
	source, err := repositoryDigest(lease.authority.repositoryRoot)
	if err != nil {
		return ReceiptIdentityObservation{}, err
	}
	return ReceiptIdentityObservation{SourceSHA256: source, ToolchainSHA256: strings.Repeat("e", 64)}, nil
}

func (lease *testLease) BindCanonical(_ context.Context, request CanonicalBindingRequest) (CanonicalBinding, error) {
	if request.DiscoveryID == "" || request.EnvironmentSHA256 == "" || len(request.PackagePatterns) == 0 {
		return CanonicalBinding{}, errors.New("canonical binding mismatch")
	}
	return CanonicalBinding{CapabilityID: "go-live-capability:sha256:" + strings.Repeat("c", 64), PlanID: "go-live-plan:sha256:" + strings.Repeat("d", 64)}, nil
}

func (lease *testLease) Release(_ context.Context) error {
	lease.released = true
	return lease.authority.releaseErr
}

func TestExecuteIsOffByDefaultAndRequiresAuthority(t *testing.T) {
	transcript, err := Execute(context.Background(), Config{})
	if !errors.Is(err, ErrDisabled) {
		t.Fatalf("Execute error = %v, want ErrDisabled", err)
	}
	if transcript.Qualification != QualificationExperimentalTranscript || transcript.Execution != ExecutionIncomplete {
		t.Fatalf("unexpected transcript boundary: %+v", transcript)
	}
	config := Config{ExperimentalEnabled: true}
	_, err = Execute(context.Background(), config)
	if !errors.Is(err, ErrExplicitAction) {
		t.Fatalf("Execute explicit-action error = %v", err)
	}
	config.ExplicitTrustedLocalAction = true
	_, err = Execute(context.Background(), config)
	if !errors.Is(err, ErrAuthorityUnavailable) {
		t.Fatalf("Execute authority error = %v", err)
	}
}

func TestExecuteUsesIsolatedEnvironmentLeavesRepositoryUnchangedAndCleans(t *testing.T) {
	fixture := newFixture(t, false)
	t.Setenv("CORVINT_AMBIENT_SECRET", "must-not-reach-child")
	before, err := repositoryDigest(fixture.repository)
	if err != nil {
		t.Fatal(err)
	}

	transcript, err := Execute(context.Background(), fixture.config)
	if err != nil {
		t.Fatalf("Execute: %v; runner=%v decode=%v stderr=%s", err, transcript.RunnerError, transcript.DecodeError, transcript.Runner.Stderr.Data)
	}
	if transcript.Execution != ExecutionPassed || !transcript.Runner.Started || transcript.Runner.ExitCode != 0 {
		t.Fatalf("execution = %s, runner = %+v, decode = %v", transcript.Execution, transcript.Runner, transcript.DecodeError)
	}
	if transcript.Qualification != QualificationExperimentalTranscript || transcript.Scope != ScopeUnknown ||
		transcript.Coverage != CoverageNone || transcript.Persistence != PersistenceNone ||
		transcript.LPCVQualified || !transcript.CanonicalTerminalSupported || len(transcript.Receipt.Transcript) == 0 {
		t.Fatalf("provider overstated qualification: %+v", transcript)
	}
	if !transcript.EphemeralDeletionComplete {
		t.Fatal("ephemeral deletion was not complete")
	}
	after, err := repositoryDigest(fixture.repository)
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("repository changed: before=%s after=%s", before, after)
	}
	assertDirectoryEmpty(t, fixture.temporaryParent)
}

func TestExecuteCancellationIsIncompleteAndCleans(t *testing.T) {
	fixture := newFixture(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	transcript, err := Execute(ctx, fixture.config)
	if err != nil {
		t.Fatalf("Execute returned error: %v; transcript=%+v", err, transcript)
	}
	if transcript.Execution != ExecutionIncomplete || !transcript.Runner.Cancelled {
		t.Fatalf("cancellation transcript = %+v", transcript)
	}
	if !transcript.EphemeralDeletionComplete || !transcript.Runner.ProcessCleanupDone {
		t.Fatalf("cancellation cleanup incomplete: %+v", transcript)
	}
	if !transcript.CanonicalTerminalSupported || !bytes.Contains(transcript.Receipt.Run, []byte(`"failureClass":"CANCELLATION"`)) ||
		!bytes.Contains(transcript.Receipt.Run, []byte(`"status":"INCOMPLETE"`)) {
		t.Fatalf("cancellation terminal was not closed canonically: %s", transcript.Receipt.Run)
	}
	assertDirectoryEmpty(t, fixture.temporaryParent)
}

func TestAuthorityDriftPreventsLaunchAndCleans(t *testing.T) {
	fixture := newFixture(t, false)
	fixture.authority.revalidationErr = fmt.Errorf("%w: drift", ErrAuthorityDrift)
	transcript, err := Execute(context.Background(), fixture.config)
	if !errors.Is(err, ErrAuthorityDrift) {
		t.Fatalf("Execute error = %v, want ErrAuthorityDrift", err)
	}
	if transcript.Runner.Started {
		t.Fatal("runner started after authority drift")
	}
	assertDirectoryEmpty(t, fixture.temporaryParent)
}

func TestValidateBindingRejectsWritableOrRepositoryModuleCache(t *testing.T) {
	fixture := newFixture(t, false)
	if err := os.Chmod(fixture.moduleCache, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Execute(context.Background(), fixture.config)
	if !errors.Is(err, ErrAuthorityUnavailable) {
		t.Fatalf("writable cache error = %v", err)
	}
	assertDirectoryEmpty(t, fixture.temporaryParent)

	fixture = newFixture(t, false)
	cache := filepath.Join(fixture.repository, "module-cache")
	if err := os.Mkdir(cache, 0o555); err != nil {
		t.Fatal(err)
	}
	fixture.authority.binding.ModuleCacheDirectory = cache
	fixture.authority.sourceIdentity, err = repositoryDigest(fixture.repository)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Execute(context.Background(), fixture.config)
	if !errors.Is(err, ErrAuthorityUnavailable) {
		t.Fatalf("repository cache error = %v", err)
	}
}

func TestSourceOwnedDependencyDriftIsComposedWithLeaseRevalidation(t *testing.T) {
	fixture := newFixture(t, false)
	replacement := filepath.Join(fixture.repository, "replacement")
	if err := os.Mkdir(replacement, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(replacement, "dependency.go")
	if err := os.WriteFile(path, []byte("package replacement\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err := repositoryDigest(fixture.repository)
	if err != nil {
		t.Fatal(err)
	}
	fixture.authority.sourceIdentity = identity
	fixture.authority.binding.SourceIdentity = "workspace-source:sha256:" + identity
	fixture.authority.discoveryMutator = func(_ *DiscoveryResult, _ *[]byte) {
		if err := os.WriteFile(path, []byte("package changed\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	transcript, err := Execute(context.Background(), fixture.config)
	if !errors.Is(err, ErrAuthorityDrift) || transcript.Runner.Started {
		t.Fatalf("source-owned dependency drift transcript = %+v, error = %v", transcript, err)
	}
	assertDirectoryEmpty(t, fixture.temporaryParent)
}

func TestWorkspaceModeBindsExactGoWork(t *testing.T) {
	fixture := newFixture(t, false)
	goWork := filepath.Join(fixture.repository, "go.work")
	if err := os.WriteFile(goWork, []byte("go 1.27.0\n\nuse .\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err := repositoryDigest(fixture.repository)
	if err != nil {
		t.Fatal(err)
	}
	fixture.authority.sourceIdentity = identity
	fixture.authority.binding.SourceIdentity = "workspace-source:sha256:" + identity
	fixture.authority.binding.GoWorkPath = goWork
	fixture.config.ModuleMode = ModuleWorkspace
	transcript, err := Execute(context.Background(), fixture.config)
	if err != nil || transcript.Execution != ExecutionPassed {
		t.Fatalf("workspace execution = %s, error = %v", transcript.Execution, err)
	}
}

func TestRealGo127DiscoveryExecuteWorkspaceSyntheticPackagesAndCleanup(t *testing.T) {
	fixture := newFixture(t, false)
	goWork := filepath.Join(fixture.repository, "go.work")
	if err := os.WriteFile(goWork, []byte("go 1.27.0\n\nuse .\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err := repositoryDigest(fixture.repository)
	if err != nil {
		t.Fatal(err)
	}
	fixture.authority.binding.SourceIdentity = "workspace-source:sha256:" + identity
	fixture.authority.binding.GoWorkPath = goWork
	fixture.config.ModuleMode = ModuleWorkspace
	real := &realAuthority{
		binding:        fixture.authority.binding,
		repositoryRoot: fixture.repository,
		goRoot:         fixture.config.GOROOT,
		sourceIdentity: identity,
	}
	fixture.config.Authority = real
	transcript, err := Execute(context.Background(), fixture.config)
	if err != nil {
		t.Fatalf("real Go 1.27 provider execution: %v; runner=%v decode=%v", err, transcript.RunnerError, transcript.DecodeError)
	}
	if transcript.Execution != ExecutionPassed || !transcript.EphemeralDeletionComplete ||
		!transcript.CanonicalTerminalSupported || len(transcript.Receipt.Transcript) == 0 {
		t.Fatalf("real execution boundary = %+v", transcript)
	}
	if output := os.Getenv("CORVINT_DISCOVERY_FIXTURE_OUT"); output != "" {
		if err := os.WriteFile(output, transcript.DiscoveryCanonical, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	kinds := make(map[godiscovery.PackageKind]int)
	for _, pack := range transcript.Discovery.Packages {
		kinds[pack.Kind]++
		if pack.Kind == godiscovery.TestMain && len(pack.InputFiles) != 1 {
			t.Fatalf("TEST_MAIN inputs = %d, want exactly one", len(pack.InputFiles))
		}
	}
	for _, kind := range []godiscovery.PackageKind{godiscovery.Requested, godiscovery.TestVariant, godiscovery.TestMain} {
		if kinds[kind] == 0 {
			t.Fatalf("real discovery omitted %s: %+v", kind, kinds)
		}
	}
	assertDirectoryEmpty(t, fixture.temporaryParent)
}

// GLTP-V0-021: a requested runner package whose terminal is skip (no test
// files) is not a pass; the transcript must agree with the receipt.
func TestRequestedPackageWithoutTestsIsNotPassed(t *testing.T) {
	fixture := newFixture(t, false)
	probe := filepath.Join(fixture.repository, "probe")
	if err := os.Rename(filepath.Join(probe, "probe_test.go"), filepath.Join(probe, "probe.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(probe, "probe.go"), []byte("package probe\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err := repositoryDigest(fixture.repository)
	if err != nil {
		t.Fatal(err)
	}
	fixture.authority.binding.SourceIdentity = "workspace-source:sha256:" + identity
	fixture.config.Authority = &realAuthority{
		binding:        fixture.authority.binding,
		repositoryRoot: fixture.repository,
		goRoot:         fixture.config.GOROOT,
		sourceIdentity: identity,
	}
	transcript, _ := Execute(context.Background(), fixture.config)
	var run struct {
		Execution struct {
			Status string `json:"status"`
		} `json:"execution"`
	}
	if err := json.Unmarshal(transcript.Receipt.Run, &run); err != nil {
		t.Fatal(err)
	}
	if transcript.Execution != ExecutionIncomplete || run.Execution.Status != "INCOMPLETE" {
		t.Fatalf("skipped requested package: transcript=%s receipt=%s", transcript.Execution, run.Execution.Status)
	}
	assertDirectoryEmpty(t, fixture.temporaryParent)
}

func TestInvalidPackagePatternsStopBeforeAuthority(t *testing.T) {
	for _, packages := range [][]string{{"./probe"}, {"b/package", "a/package"}, {"bad@version"}, {"a/.../b"}, {}} {
		fixture := newFixture(t, false)
		fixture.config.Packages = packages
		_, err := Execute(context.Background(), fixture.config)
		if !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("packages %q error = %v, want ErrInvalidConfig", packages, err)
		}
		if fixture.authority.acquisitions != 0 {
			t.Fatalf("authority acquired for invalid packages %q", packages)
		}
	}
}

func TestDiscoveryArgvByteBoundStopsBeforeAuthority(t *testing.T) {
	fixture := newFixture(t, false)
	packages := make([]string, gorunner.MaxPackages)
	for index := range packages {
		packages[index] = fmt.Sprintf("p%04d/%s", index, strings.Repeat("a", 1_020))
	}
	fixture.config.Packages = packages
	_, err := Execute(context.Background(), fixture.config)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("oversize discovery argv error = %v", err)
	}
	if fixture.authority.acquisitions != 0 {
		t.Fatal("authority acquired for oversize discovery argv")
	}
}

func TestExactGoVersionBindingIsRequired(t *testing.T) {
	fixture := newFixture(t, false)
	fixture.authority.binding.GoVersion = "go1.27.0"
	transcript, err := Execute(context.Background(), fixture.config)
	if !errors.Is(err, ErrAuthorityUnavailable) || transcript.Runner.Started {
		t.Fatalf("Go version mismatch transcript = %+v, error = %v", transcript, err)
	}
	assertDirectoryEmpty(t, fixture.temporaryParent)
}

func TestDiscoveryMismatchStopsBeforeTestExecutionAndCleans(t *testing.T) {
	fixture := newFixture(t, false)
	fixture.authority.discoveryMutator = func(_ *DiscoveryResult, raw *[]byte) {
		*raw = append(*raw, '{')
	}
	transcript, err := Execute(context.Background(), fixture.config)
	if !errors.Is(err, ErrAuthorityUnavailable) {
		t.Fatalf("discovery mismatch error = %v", err)
	}
	if transcript.Runner.Started {
		t.Fatal("runner started with mismatched discovery")
	}
	assertDirectoryEmpty(t, fixture.temporaryParent)
}

func TestPostRunAuthorityDriftMakesTranscriptIncompleteAndCleans(t *testing.T) {
	fixture := newFixture(t, false)
	fixture.authority.postErr = errors.New("post drift")
	transcript, err := Execute(context.Background(), fixture.config)
	if !errors.Is(err, ErrAuthorityDrift) {
		t.Fatalf("post drift error = %v", err)
	}
	if !transcript.Runner.Started || transcript.Execution != ExecutionIncomplete {
		t.Fatalf("post drift transcript = %+v", transcript)
	}
	assertInfrastructureReceipt(t, transcript)
	assertDirectoryEmpty(t, fixture.temporaryParent)
}

func TestLeaseReleaseFailureMakesTranscriptIncompleteAndCleans(t *testing.T) {
	fixture := newFixture(t, false)
	fixture.authority.releaseErr = errors.New("release failed")
	transcript, err := Execute(context.Background(), fixture.config)
	if !errors.Is(err, ErrAuthorityDrift) {
		t.Fatalf("release failure error = %v", err)
	}
	if !transcript.Runner.Started || transcript.Execution != ExecutionIncomplete {
		t.Fatalf("release failure transcript = %+v", transcript)
	}
	assertInfrastructureReceipt(t, transcript)
	assertDirectoryEmpty(t, fixture.temporaryParent)
}

func TestCreateEnvironmentIsClosedSortedAndRunOwned(t *testing.T) {
	base := resolvedTemp(t)
	repository := filepath.Join(base, "repository")
	temporaryParent := filepath.Join(base, "temporary")
	if err := os.Mkdir(repository, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(temporaryParent, 0o700); err != nil {
		t.Fatal(err)
	}
	temporaryInfo, err := os.Stat(temporaryParent)
	if err != nil {
		t.Fatal(err)
	}
	runRoot, err := newRunRoot(temporaryParent, temporaryInfo, repository)
	if err != nil {
		t.Fatal(err)
	}
	moduleCache := filepath.Join(base, "module-cache")
	if err := os.Mkdir(moduleCache, 0o555); err != nil {
		t.Fatal(err)
	}
	environment, artifactRoot, err := createEnvironment(runRoot, moduleCache, "off", resolvedConfig{goRoot: resolvedGOROOT(t)}, ModuleReadonly)
	if err != nil {
		t.Fatal(err)
	}
	if len(environment) == 0 {
		t.Fatal("environment is empty")
	}
	if !within(runRoot.path, artifactRoot) {
		t.Fatalf("artifact root escaped run root: %q", artifactRoot)
	}
	for index := 1; index < len(environment); index++ {
		if environment[index-1].Name >= environment[index].Name {
			t.Fatalf("environment is not sorted and unique: %+v", environment)
		}
	}
	values := make(map[string]string, len(environment))
	names := make([]string, 0, len(environment))
	for _, value := range environment {
		values[value.Name] = value.Value
		names = append(names, value.Name)
	}
	wantNames := []string{
		"CGO_ENABLED", "GOARCH", "GOCACHE", "GOENV", "GOFLAGS", "GOMODCACHE", "GONOPROXY",
		"GONOSUMDB", "GOOS", "GOPRIVATE", "GOPROXY", "GOROOT", "GOSUMDB", "GOTMPDIR",
		"GOTOOLCHAIN", "GOVCS", "GOWORK", "HOME", "TEMP", "TMP", "TMPDIR",
	}
	if strings.Join(names, "\x00") != strings.Join(wantNames, "\x00") {
		t.Fatalf("environment names = %q, want %q", names, wantNames)
	}
	for name, want := range map[string]string{
		"CGO_ENABLED": "0", "GOENV": "off", "GOFLAGS": "-mod=readonly",
		"GOMODCACHE": moduleCache, "GOPROXY": "off", "GOSUMDB": "off",
		"GOTOOLCHAIN": "local", "GOVCS": "*:off", "GOWORK": "off",
	} {
		if values[name] != want {
			t.Fatalf("%s = %q, want %q", name, values[name], want)
		}
	}
	for _, name := range []string{"GOCACHE", "GOTMPDIR", "HOME", "TEMP", "TMP", "TMPDIR"} {
		if !within(runRoot.path, values[name]) {
			t.Fatalf("%s escaped run root: %q", name, values[name])
		}
		info, err := os.Stat(values[name])
		if err != nil {
			t.Fatalf("%s stat: %v", name, err)
		}
		t.Run(name+" private directory mode", func(t *testing.T) {
			if runtime.GOOS == "windows" {
				t.Skip("Windows does not expose POSIX file-mode semantics")
			}
			if info.Mode().Perm() != 0o700 {
				t.Fatalf("%s mode = %v", name, info.Mode().Perm())
			}
		})
	}
	if _, ok := values["CORVINT_AMBIENT_SECRET"]; ok {
		t.Fatal("ambient variable was admitted")
	}
	if err := runRoot.remove(); err != nil {
		t.Fatal(err)
	}
	assertDirectoryEmpty(t, temporaryParent)
}

func TestNewRunRootRejectsTemporaryParentIdentitySwap(t *testing.T) {
	base := resolvedTemp(t)
	repository := filepath.Join(base, "repository")
	parent := filepath.Join(base, "temporary")
	replacement := filepath.Join(base, "replacement")
	for _, path := range []string{repository, parent, replacement} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	expected, err := os.Stat(parent)
	if err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(base, "original")
	if err := os.Rename(parent, original); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, parent); err != nil {
		t.Fatal(err)
	}
	if _, err := newRunRoot(parent, expected, repository); err == nil {
		t.Fatal("newRunRoot accepted substituted temporary parent")
	}
	assertDirectoryEmpty(t, parent)
}

func TestRunRootCleanupRejectsNameSubstitution(t *testing.T) {
	base := resolvedTemp(t)
	repository := filepath.Join(base, "repository")
	parent := filepath.Join(base, "temporary")
	if err := os.Mkdir(repository, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	parentInfo, err := os.Stat(parent)
	if err != nil {
		t.Fatal(err)
	}
	runRoot, err := newRunRoot(parent, parentInfo, repository)
	if err != nil {
		t.Fatal(err)
	}
	moved := runRoot.path + "-moved"
	if err := os.Rename(runRoot.path, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(repository, runRoot.path); err != nil {
		t.Fatal(err)
	}
	if err := runRoot.remove(); !errors.Is(err, ErrCleanupIncomplete) {
		t.Fatalf("cleanup error = %v, want ErrCleanupIncomplete", err)
	}
	if _, err := os.Stat(repository); err != nil {
		t.Fatalf("repository was affected: %v", err)
	}
	if err := os.Remove(runRoot.path); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(moved); err != nil {
		t.Fatal(err)
	}
}

func TestRunRootRejectsOwnedDirectoryReplacementWithMatchingShape(t *testing.T) {
	base := resolvedTemp(t)
	repository := filepath.Join(base, "repository")
	parent := filepath.Join(base, "temporary")
	moduleCache := filepath.Join(base, "module-cache")
	for _, path := range []string{repository, parent, moduleCache} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	parentInfo, err := os.Stat(parent)
	if err != nil {
		t.Fatal(err)
	}
	runRoot, err := newRunRoot(parent, parentInfo, repository)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := createEnvironment(runRoot, moduleCache, "off", resolvedConfig{goRoot: resolvedGOROOT(t)}, ModuleReadonly); err != nil {
		t.Fatal(err)
	}
	moved := runRoot.path + "-moved"
	if err := os.Rename(runRoot.path, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(runRoot.path, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"go-build-cache", "go-tmp", "home", "tmp"} {
		if err := os.Mkdir(filepath.Join(runRoot.path, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := runRoot.validateLayout(); !errors.Is(err, ErrAuthorityDrift) {
		t.Fatalf("replacement validation error = %v", err)
	}
	if err := runRoot.remove(); !errors.Is(err, ErrCleanupIncomplete) {
		t.Fatalf("replacement cleanup error = %v", err)
	}
	if err := os.RemoveAll(runRoot.path); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(moved); err != nil {
		t.Fatal(err)
	}
}

func TestGeneratedTestMainMustBeRegularAndBoundInsideGoCache(t *testing.T) {
	base := resolvedTemp(t)
	goCache := filepath.Join(base, "go-cache")
	if err := os.Mkdir(goCache, 0o700); err != nil {
		t.Fatal(err)
	}
	generated := filepath.Join(goCache, "testmain", "_testmain.go")
	if err := os.Mkdir(filepath.Dir(generated), 0o700); err != nil {
		t.Fatal(err)
	}
	contents := []byte("package main\n")
	if err := os.WriteFile(generated, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	rawSHA := sha256.Sum256(contents)
	commitments := godiscovery.Commitments{Packages: []godiscovery.PackageMaterial{{Files: []godiscovery.FileCommitment{{
		Field: godiscovery.FieldGoFiles, ListedName: generated, Role: godiscovery.RoleGeneratedTestMain,
		Mode: "0600", RawSHA256: hex.EncodeToString(rawSHA[:]),
	}}}}}
	if err := validateGeneratedTestMainCommitments(commitments, goCache); err != nil {
		t.Fatalf("valid generated testmain rejected: %v", err)
	}
	if err := os.WriteFile(generated, []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateGeneratedTestMainCommitments(commitments, goCache); !errors.Is(err, ErrAuthorityUnavailable) {
		t.Fatalf("changed generated testmain error = %v", err)
	}
	commitments.Packages[0].Files[0].ListedName = filepath.Join(base, "outside.go")
	if err := validateGeneratedTestMainCommitments(commitments, goCache); !errors.Is(err, ErrAuthorityUnavailable) {
		t.Fatalf("escaped generated testmain error = %v", err)
	}
}

func TestDiscoveredPackageGrammarAdmitsGoSyntheticIdentities(t *testing.T) {
	values := []string{"example.test/probe [example.test/probe.test]", "example.test/probe.test"}
	if !validDiscoveredPackages(values) {
		t.Fatalf("real Go test-variant/test-main identities rejected: %q", values)
	}
	if validImportPaths(values) {
		t.Fatalf("synthetic identities admitted as exact requested import paths: %q", values)
	}
}

func TestComposeDiscoveryRejectsDuplicateImportPaths(t *testing.T) {
	fixture := newFixture(t, false)
	request := DiscoveryRequest{PackagePatterns: fixture.config.Packages}
	result, raw, err := fixtureDiscovery(fixture.repository, fixture.authority.binding, request)
	if err != nil {
		t.Fatal(err)
	}
	duplicate := result.Commitments.Packages[0]
	result.Commitments.Packages = append(result.Commitments.Packages, duplicate)
	raw = append(raw, raw...)
	goCache := filepath.Join(fixture.temporaryParent, "go-build-cache")
	if err := os.Mkdir(goCache, 0o700); err != nil {
		t.Fatal(err)
	}
	_, _, _, _, err = composeDiscovery(
		result, raw, nil, fixture.authority.binding,
		[]gorunner.EnvironmentVariable{{Name: "CGO_ENABLED", Value: "0"}}, fixture.config,
		goCache,
	)
	if !errors.Is(err, ErrAuthorityUnavailable) {
		t.Fatalf("duplicate ImportPath error = %v", err)
	}
}

type fixture struct {
	repository      string
	temporaryParent string
	moduleCache     string
	authority       *testAuthority
	config          Config
}

func newFixture(t *testing.T, slow bool) fixture {
	t.Helper()
	base := resolvedTemp(t)
	repository := filepath.Join(base, "repository")
	temporaryParent := filepath.Join(base, "temporary")
	moduleCache := filepath.Join(base, "module-cache")
	for _, path := range []string{repository, temporaryParent, moduleCache} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(repository, "go.mod"), []byte("module example.test/providerfixture\n\ngo 1.27.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(repository, "probe"), 0o700); err != nil {
		t.Fatal(err)
	}
	body := `package probe

import (
	"os"
	"strings"
	"testing"
)

func TestEnvironment(t *testing.T) {
	if os.Getenv("CORVINT_AMBIENT_SECRET") != "" { t.Fatal("ambient secret leaked") }
	if os.Getenv("GOTOOLCHAIN") != "local" || os.Getenv("CGO_ENABLED") != "0" { t.Fatal("toolchain profile mismatch") }
	if os.Getenv("GOPROXY") != "off" || os.Getenv("GOSUMDB") != "off" || os.Getenv("GOVCS") != "*:off" { t.Fatal("offline profile mismatch") }
	for _, name := range []string{"HOME", "GOCACHE", "GOTMPDIR", "TMPDIR", "TMP", "TEMP"} {
		if value := os.Getenv(name); value == "" || strings.Contains(value, "repository") { t.Fatalf("%s is not isolated", name) }
	}
}
`
	if slow {
		body = `package probe

import (
	"testing"
	"time"
)

func TestSlow(t *testing.T) { time.Sleep(30 * time.Second) }
`
	}
	if err := os.WriteFile(filepath.Join(repository, "probe", "probe_test.go"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err := repositoryDigest(repository)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(moduleCache, 0o555); err != nil {
		t.Fatal(err)
	}
	authority := &testAuthority{
		repositoryRoot: repository,
		sourceIdentity: identity,
		binding: AuthorityBinding{
			SourceIdentity:       "workspace-source:sha256:" + identity,
			ToolchainIdentity:    "go-toolchain:sha256:" + strings.Repeat("a", 64),
			DependencyIdentity:   strings.Repeat("b", 64),
			ModuleCacheDirectory: moduleCache,
			GoVersion:            GoVersion,
			GOOS:                 runtime.GOOS,
			GOARCH:               runtime.GOARCH,
			CGOEnabled:           "0",
			EnvironmentProfile:   EnvironmentProfile,
		},
	}
	goRoot := resolvedGOROOT(t)
	return fixture{
		repository:      repository,
		temporaryParent: temporaryParent,
		moduleCache:     moduleCache,
		authority:       authority,
		config: Config{
			ExperimentalEnabled:        true,
			ExplicitTrustedLocalAction: true,
			RepositoryRoot:             repository,
			WorkingDirectory:           repository,
			TemporaryParent:            temporaryParent,
			GoExecutable:               filepath.Join(goRoot, "bin", executableName(runtime.GOOS)),
			GOROOT:                     goRoot,
			GOOS:                       runtime.GOOS,
			GOARCH:                     runtime.GOARCH,
			ModuleMode:                 ModuleReadonly,
			Packages:                   []string{"example.test/providerfixture/probe"},
			Timeout:                    gorunner.MaxRunTime,
			OutputLimitBytes:           gorunner.MaxOutputBytes,
			Authority:                  authority,
		},
	}
}

func resolvedTemp(t *testing.T) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(resolved)
}

func resolvedGOROOT(t *testing.T) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(runtime.GOROOT())
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(resolved)
}

func fixtureDiscovery(repository string, binding AuthorityBinding, request DiscoveryRequest) (DiscoveryResult, []byte, error) {
	const importPath = "example.test/providerfixture/probe"
	rawDirectory := filepath.Join(repository, "probe")
	listedName := "probe_test.go"
	logicalDirectory := "probe"
	logicalFile := "probe/probe_test.go"
	directoryDigest, err := logicalPathDigest(godiscovery.NamespaceSource, logicalDirectory)
	if err != nil {
		return DiscoveryResult{}, nil, err
	}
	fileDigest, err := logicalPathDigest(godiscovery.NamespaceSource, logicalFile)
	if err != nil {
		return DiscoveryResult{}, nil, err
	}
	contents, err := os.ReadFile(filepath.Join(rawDirectory, listedName))
	if err != nil {
		return DiscoveryResult{}, nil, err
	}
	rawSHA := sha256.Sum256(contents)
	raw := fmt.Sprintf(`{"Dir":%q,"ImportPath":%q,"Match":[%q],"Name":"probe","TestGoFiles":[%q]}`,
		rawDirectory, importPath, request.PackagePatterns[0], listedName)
	moduleMode := godiscovery.ModuleModeModule
	if binding.GoWorkPath != "" {
		moduleMode = godiscovery.ModuleModeWorkspace
	}
	commitments := godiscovery.Commitments{
		DependencyMaterializationSHA256: binding.DependencyIdentity,
		ModuleMode:                      moduleMode,
		SourceWSI:                       binding.SourceIdentity,
		Packages: []godiscovery.PackageMaterial{{
			ImportPath: importPath, RawDir: rawDirectory,
			DirNamespace: godiscovery.NamespaceSource, DirLogicalPath: logicalDirectory, DirPathSHA256: directoryDigest,
			Files: []godiscovery.FileCommitment{{
				Field: godiscovery.FieldTestGoFiles, ListedName: listedName, Role: godiscovery.RoleSource,
				Mode: "0600", Namespace: godiscovery.NamespaceSource, LogicalPath: logicalFile,
				PathSHA256: fileDigest, RawSHA256: hex.EncodeToString(rawSHA[:]),
			}},
		}},
	}
	return DiscoveryResult{ExitCode: 0, Commitments: commitments}, []byte(raw), nil
}

func logicalPathDigest(namespace godiscovery.PathNamespace, logical string) (string, error) {
	body := fmt.Sprintf(`{"namespace":%q,"path":%q}`, string(namespace), logical)
	return godiscovery.Hash("go-logical-path", "go-logical-path/0", []byte(body))
}

func realDiscoveryCommitments(raw []byte, binding AuthorityBinding, repositoryRoot, goRoot, runRoot string) (godiscovery.Commitments, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	commitments := godiscovery.Commitments{
		DependencyMaterializationSHA256: binding.DependencyIdentity,
		ModuleMode:                      godiscovery.ModuleModeModule,
		SourceWSI:                       binding.SourceIdentity,
	}
	if binding.GoWorkPath != "" {
		commitments.ModuleMode = godiscovery.ModuleModeWorkspace
	}
	modulePaths := make(map[string]godiscovery.PathCommitment)
	for {
		var pack realListPackage
		if err := decoder.Decode(&pack); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return godiscovery.Commitments{}, err
		}
		namespace, logicalDirectory, err := realLogicalPath(pack.Dir, repositoryRoot, binding.ModuleCacheDirectory, goRoot)
		if err != nil {
			return godiscovery.Commitments{}, err
		}
		directoryDigest, err := logicalPathDigest(namespace, logicalDirectory)
		if err != nil {
			return godiscovery.Commitments{}, err
		}
		material := godiscovery.PackageMaterial{
			ImportPath: pack.ImportPath, RawDir: pack.Dir, DirNamespace: namespace,
			DirLogicalPath: logicalDirectory, DirPathSHA256: directoryDigest,
		}
		fields := []struct {
			field godiscovery.FileField
			names []string
		}{
			{godiscovery.FieldGoFiles, pack.GoFiles}, {godiscovery.FieldCgoFiles, pack.CgoFiles},
			{godiscovery.FieldCFiles, pack.CFiles}, {godiscovery.FieldCXXFiles, pack.CXXFiles},
			{godiscovery.FieldMFiles, pack.MFiles}, {godiscovery.FieldHFiles, pack.HFiles},
			{godiscovery.FieldFFiles, pack.FFiles}, {godiscovery.FieldSFiles, pack.SFiles},
			{godiscovery.FieldSwigFiles, pack.SwigFiles}, {godiscovery.FieldSwigCXXFiles, pack.SwigCXXFiles},
			{godiscovery.FieldSysoFiles, pack.SysoFiles}, {godiscovery.FieldEmbedFiles, pack.EmbedFiles},
			{godiscovery.FieldTestGoFiles, pack.TestGoFiles}, {godiscovery.FieldXTestGoFiles, pack.XTestGoFiles},
			{godiscovery.FieldTestEmbedFiles, pack.TestEmbedFiles}, {godiscovery.FieldXTestEmbedFiles, pack.XTestEmbedFiles},
		}
		for _, field := range fields {
			for _, listedName := range field.names {
				file, err := realFileCommitment(pack, field.field, listedName, namespace, logicalDirectory, runRoot)
				if err != nil {
					return godiscovery.Commitments{}, err
				}
				material.Files = append(material.Files, file)
			}
		}
		commitments.Packages = append(commitments.Packages, material)
		if err := collectRealModulePaths(pack.Module, repositoryRoot, binding.ModuleCacheDirectory, modulePaths); err != nil {
			return godiscovery.Commitments{}, err
		}
	}
	for _, commitment := range modulePaths {
		commitments.ModulePaths = append(commitments.ModulePaths, commitment)
	}
	sort.Slice(commitments.ModulePaths, func(left, right int) bool {
		return commitments.ModulePaths[left].RawPath < commitments.ModulePaths[right].RawPath
	})
	return commitments, nil
}

func realFileCommitment(pack realListPackage, field godiscovery.FileField, listedName string, namespace godiscovery.PathNamespace, logicalDirectory, runRoot string) (godiscovery.FileCommitment, error) {
	path := filepath.Join(pack.Dir, listedName)
	if filepath.IsAbs(listedName) {
		path = listedName
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return godiscovery.FileCommitment{}, errors.New("real discovery input is not a regular file")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return godiscovery.FileCommitment{}, err
	}
	rawDigest := sha256.Sum256(contents)
	commitment := godiscovery.FileCommitment{
		Field: field, ListedName: listedName, Role: godiscovery.RoleSource,
		Mode: fmt.Sprintf("%04o", info.Mode().Perm()), Namespace: namespace,
		LogicalPath: filepath.ToSlash(filepath.Join(logicalDirectory, listedName)),
		RawSHA256:   hex.EncodeToString(rawDigest[:]),
	}
	if filepath.IsAbs(listedName) {
		if !within(filepath.Join(runRoot, "go-build-cache"), listedName) || pack.Name != "main" || pack.ForTest != "" || len(pack.Match) != 0 {
			return godiscovery.FileCommitment{}, errors.New("unexpected generated discovery input")
		}
		commitment.Role = godiscovery.RoleGeneratedTestMain
		commitment.Namespace = ""
		commitment.LogicalPath = ""
		body := fmt.Sprintf(`{"importPath":%q,"mode":%q,"rawSha256":%q,"role":"TEST_MAIN_GOFILE"}`,
			pack.ImportPath, commitment.Mode, commitment.RawSHA256)
		commitment.PathSHA256, err = godiscovery.Hash("go-generated-testmain", "go-generated-testmain/0", []byte(body))
		return commitment, err
	}
	commitment.PathSHA256, err = logicalPathDigest(namespace, commitment.LogicalPath)
	return commitment, err
}

func collectRealModulePaths(module *realListModule, repositoryRoot, moduleCache string, values map[string]godiscovery.PathCommitment) error {
	if module == nil {
		return nil
	}
	for _, rawPath := range []string{module.Dir, module.GoMod} {
		if rawPath == "" {
			continue
		}
		namespace, logical, err := realLogicalPath(rawPath, repositoryRoot, moduleCache, "")
		if err != nil || namespace == godiscovery.NamespaceToolchain {
			return errors.New("module path escaped source/dependency materialization")
		}
		digest, err := logicalPathDigest(namespace, logical)
		if err != nil {
			return err
		}
		values[rawPath] = godiscovery.PathCommitment{RawPath: rawPath, Namespace: namespace, LogicalPath: logical, PathSHA256: digest}
	}
	return collectRealModulePaths(module.Replace, repositoryRoot, moduleCache, values)
}

func realLogicalPath(path, repositoryRoot, moduleCache, goRoot string) (godiscovery.PathNamespace, string, error) {
	for _, candidate := range []struct {
		root      string
		namespace godiscovery.PathNamespace
	}{
		{repositoryRoot, godiscovery.NamespaceSource},
		{goRoot, godiscovery.NamespaceToolchain},
		{moduleCache, godiscovery.NamespaceDependency},
	} {
		if candidate.root == "" || !within(candidate.root, path) {
			continue
		}
		logical, err := filepath.Rel(candidate.root, path)
		if err != nil {
			return "", "", err
		}
		return candidate.namespace, filepath.ToSlash(logical), nil
	}
	return "", "", errors.New("path escaped verified materializations")
}

func repositoryDigest(root string) (string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path != root {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	digest := sha256.New()
	for _, path := range paths {
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return "", err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		_, _ = digest.Write([]byte(relative))
		_, _ = digest.Write([]byte{0})
		_, _ = digest.Write([]byte(info.Mode().String()))
		_, _ = digest.Write([]byte{0})
		if info.Mode().IsRegular() {
			contents, err := os.ReadFile(path)
			if err != nil {
				return "", err
			}
			_, _ = digest.Write(contents)
		}
		_, _ = digest.Write([]byte{0})
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func assertDirectoryEmpty(t *testing.T, path string) {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("directory contains residue: %s", strings.Join(names, ", "))
	}
}

func assertInfrastructureReceipt(t *testing.T, transcript Transcript) {
	t.Helper()
	if !transcript.CanonicalTerminalSupported ||
		!bytes.Contains(transcript.Receipt.Run, []byte(`"status":"INCOMPLETE"`)) ||
		!bytes.Contains(transcript.Receipt.Run, []byte(`"failureClass":"INFRASTRUCTURE"`)) ||
		!bytes.Contains(transcript.Receipt.Run, []byte(`"sourceCurrency":"CURRENT"`)) ||
		!bytes.Contains(transcript.Receipt.Run, []byte(`"toolchainCurrency":"CURRENT"`)) {
		t.Fatalf("authority failure receipt is not truthful: %s", transcript.Receipt.Run)
	}
}
