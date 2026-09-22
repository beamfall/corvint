package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/godiscovery"
	"github.com/Beamfall/corvint/internal/liveverify/gorunner"
	"github.com/Beamfall/corvint/internal/liveverify/provider"
)

var independentConformance struct {
	once       sync.Once
	executable string
	directory  string
	err        error
	output     []byte
}

var providerRepositoryFixture struct {
	once       sync.Once
	directory  string
	repository string
	err        error
	output     []byte
}

var productionProviderBuild struct {
	once       sync.Once
	directory  string
	executable string
	err        error
	output     []byte
}

const (
	directCommandHelperArgument = "-test.run=^TestDirectCommandHelper$"
	helperLease                 = "test-live-parent-lease"
	helperSource                = "1111111111111111111111111111111111111111111111111111111111111111"
	helperDependency            = "2222222222222222222222222222222222222222222222222222222222222222"
	helperToolchain             = "3333333333333333333333333333333333333333333333333333333333333333"
	helperToolObs               = "4444444444444444444444444444444444444444444444444444444444444444"
)

func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == "version" && filepath.Base(os.Args[0]) == "corvint-nested-go" {
		os.Exit(runNestedAuthorityGoHelper())
	}
	if len(os.Args) == 3 && os.Args[1] == attachmentArgument {
		os.Exit(2)
	}
	if len(os.Args) == 5 && os.Args[1] == "--experimental" && os.Args[2] == "--trusted-local" && os.Args[3] == attachmentArgument {
		raw, _ := base64.RawURLEncoding.DecodeString(os.Args[4])
		var request attachmentRequest
		_ = jsonv2.Unmarshal(raw, &request, jsonv2.RejectUnknownMembers(true))
		if request.Bundle != nil {
			capability, ok := readParentCapability()
			if !ok {
				os.Exit(2)
			}
			ctx, cancel := providerContext()
			defer cancel()
			os.Exit(runQualificationAuthority(ctx, os.Args[4], capability, request))
		}
		os.Exit(runAuthorityHelper(os.Args[4]))
	}
	code := m.Run()
	if independentConformance.directory != "" {
		_ = os.RemoveAll(independentConformance.directory)
	}
	if providerRepositoryFixture.directory != "" {
		_ = os.RemoveAll(providerRepositoryFixture.directory)
	}
	if productionProviderBuild.directory != "" {
		_ = os.RemoveAll(productionProviderBuild.directory)
	}
	os.Exit(code)
}

func runNestedAuthorityGoHelper() int {
	executable, err := os.Executable()
	if err != nil {
		return 88
	}
	ready := executable + ".ready"
	descendantReady := executable + ".descendant-ready"
	command := exec.Command(executable, directCommandHelperArgument)
	command.Env = []string{"DIRECT_COMMAND_HELPER_MODE=hold", "DIRECT_COMMAND_READY=" + descendantReady}
	if err := command.Start(); err != nil {
		return 89
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(descendantReady); err == nil {
			break
		}
		if time.Now().After(deadline) {
			return 90
		}
		time.Sleep(time.Millisecond)
	}
	if err := os.WriteFile(ready, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		return 91
	}
	for {
		time.Sleep(time.Hour)
	}
}

func TestProviderCommandFailsClosedAtEveryAuthorityBoundary(t *testing.T) {
	tests := [][]string{nil, {"--experimental"}, {"--experimental", "--trusted-local"}}
	for _, arguments := range tests {
		var stdout, stderr bytes.Buffer
		if exitCode := run(arguments, &stdout, &stderr); exitCode != 2 {
			t.Fatalf("run(%q) exit = %d, want 2", arguments, exitCode)
		}
		// GLTP-V0-049: full local authority reaches the platform gate first off darwin/arm64.
		code := `"code":"IDENTITY_MISMATCH"`
		if len(arguments) == 2 && !supportedProviderHost() {
			code = `"code":"UNSUPPORTED_PLATFORM"`
		}
		if !strings.Contains(stdout.String(), `"profile":"go-live-error/0"`) ||
			!strings.Contains(stdout.String(), code) ||
			!strings.Contains(stdout.String(), `"runId":null`) || !strings.HasSuffix(stdout.String(), "\n") {
			t.Fatalf("run(%q) output = %q", arguments, stdout.String())
		}
	}
}

func TestProviderCommandUsesPinnedLiveParentAuthorityE2E(t *testing.T) {
	if !supportedProviderHost() {
		t.Skip("GLTP-V0-049: the provider executes only on darwin/arm64")
	}
	type fixture struct {
		bundle     authorityBundle
		bundlePath string
	}
	fixtures := make([]fixture, 3)
	for index := range fixtures {
		fixtures[index].bundle, fixtures[index].bundlePath = providerE2EBundle(t)
	}
	if err := os.RemoveAll(filepath.Join(fixtures[2].bundle.RepositoryRoot, ".git")); err != nil {
		t.Fatal(err)
	}
	var commandStdout, commandStderr bytes.Buffer
	var commandExit int
	var transcript provider.Transcript
	var executeErr error
	var nonGitStdout, nonGitStderr bytes.Buffer
	var nonGitExit int
	var group sync.WaitGroup
	group.Add(3)
	go func() {
		defer group.Done()
		commandExit = run([]string{"--experimental", "--trusted-local", "--authority-bundle", fixtures[0].bundlePath}, &commandStdout, &commandStderr)
	}()
	go func() {
		defer group.Done()
		transcript, executeErr = executeProvider(context.Background(), fixtures[1].bundle)
	}()
	go func() {
		defer group.Done()
		nonGitExit = run([]string{"--experimental", "--trusted-local", "--authority-bundle", fixtures[2].bundlePath}, &nonGitStdout, &nonGitStderr)
	}()
	group.Wait()
	if commandExit != 0 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", commandExit, commandStdout.String(), commandStderr.String())
	}
	if !strings.Contains(commandStdout.String(), `"profile":"go-live-run/0"`) || !strings.Contains(commandStdout.String(), `"status":"PASSED"`) {
		t.Fatalf("missing canonical PASS receipt: %s", commandStdout.String())
	}
	if executeErr != nil || !transcript.CanonicalTerminalSupported {
		t.Fatalf("direct production transcript: %v", executeErr)
	}
	verifyWithIndependentConformance(t, transcript)
	if nonGitExit == 0 || !strings.Contains(nonGitStdout.String(), `"code":"IDENTITY_MISMATCH"`) {
		t.Fatalf("non-Git root accepted: exit=%d stdout=%s", nonGitExit, nonGitStdout.String())
	}
}

func providerE2EBundle(t *testing.T) (authorityBundle, string) {
	t.Helper()
	base := resolvedTemp(t)
	repository := filepath.Join(base, "repository")
	temporary := filepath.Join(base, "temporary")
	moduleCache := filepath.Join(base, "module-cache")
	for _, path := range []string{temporary, moduleCache} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	gitExecutable := resolvedGitExecutable(t)
	runFixtureGit(t, gitExecutable, ".", "clone", "-q", "--shared", sharedProviderRepository(t, gitExecutable), repository)
	if err := os.Chmod(moduleCache, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(moduleCache, 0o700) })
	verifier := productionProviderExecutable(t)
	verifierSHA, err := regularFileDigest(verifier)
	if err != nil {
		t.Fatal(err)
	}
	gitSHA, err := regularFileDigest(gitExecutable)
	if err != nil {
		t.Fatal(err)
	}
	goRoot, err := providerFixtureGoRoot()
	if err != nil {
		t.Fatal(err)
	}
	bundle := authorityBundle{
		Profile: attachmentProfile, VerifierExecutable: verifier, VerifierExecutableSHA256: verifierSHA,
		GitExecutable: gitExecutable, GitExecutableSHA256: gitSHA,
		GoExecutable: filepath.Join(goRoot, "bin", executableName(runtime.GOOS)), GOARCH: runtime.GOARCH,
		GOOS: runtime.GOOS, GOROOT: goRoot, ModuleCacheDirectory: moduleCache,
		ModuleMode: string(provider.ModuleReadonly), OutputLimitBytes: gorunner.MaxOutputBytes,
		Packages: []string{"example.test/cli"}, RepositoryRoot: repository, TemporaryParent: temporary,
		TimeoutMilliseconds: gorunner.MaxRunTime.Milliseconds(),
	}
	bundleBytes, err := canonicalJSON(bundle)
	if err != nil {
		t.Fatal(err)
	}
	bundlePath := filepath.Join(base, "bundle.json")
	if err := os.WriteFile(bundlePath, bundleBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	return bundle, bundlePath
}

func productionProviderExecutable(t *testing.T) string {
	t.Helper()
	productionProviderBuild.once.Do(func() {
		productionProviderBuild.directory, productionProviderBuild.err = os.MkdirTemp("", "corvint-provider-build-")
		if productionProviderBuild.err != nil {
			return
		}
		productionProviderBuild.executable = filepath.Join(productionProviderBuild.directory, "corvint-go-test-provider")
		cwd, err := os.Getwd()
		if err != nil {
			productionProviderBuild.err = err
			return
		}
		command := exec.Command("go", "build", "-trimpath", "-o", productionProviderBuild.executable, "./cmd/corvint-go-test-provider")
		command.Dir = filepath.Clean(filepath.Join(cwd, "..", ".."))
		command.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOWORK=off")
		productionProviderBuild.output, productionProviderBuild.err = command.CombinedOutput()
	})
	if productionProviderBuild.err != nil {
		t.Fatalf("build production provider: %v: %s", productionProviderBuild.err, productionProviderBuild.output)
	}
	return productionProviderBuild.executable
}

func sharedProviderRepository(t *testing.T, gitExecutable string) string {
	t.Helper()
	providerRepositoryFixture.once.Do(func() {
		providerRepositoryFixture.directory, providerRepositoryFixture.err = os.MkdirTemp("", "corvint-provider-repository-")
		if providerRepositoryFixture.err != nil {
			return
		}
		providerRepositoryFixture.repository = filepath.Join(providerRepositoryFixture.directory, "repository")
		if err := os.Mkdir(providerRepositoryFixture.repository, 0o700); err != nil {
			providerRepositoryFixture.err = err
			return
		}
		if err := os.WriteFile(filepath.Join(providerRepositoryFixture.repository, "go.mod"), []byte("module example.test/cli\n\ngo 1.27.1\n"), 0o600); err != nil {
			providerRepositoryFixture.err = err
			return
		}
		if err := os.WriteFile(filepath.Join(providerRepositoryFixture.repository, "cli_test.go"), []byte("package cli\nimport \"testing\"\nfunc TestCLI(t *testing.T) {}\n"), 0o600); err != nil {
			providerRepositoryFixture.err = err
			return
		}
		for _, arguments := range [][]string{{"init", "-q"}, {"add", "go.mod", "cli_test.go"}, {"-c", "user.name=Corvint Test", "-c", "user.email=corvint@example.invalid", "commit", "-qm", "fixture"}} {
			command := exec.Command(gitExecutable, append([]string{"-C", providerRepositoryFixture.repository}, arguments...)...)
			command.Env = []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "LC_ALL=C"}
			providerRepositoryFixture.output, providerRepositoryFixture.err = command.CombinedOutput()
			if providerRepositoryFixture.err != nil {
				return
			}
		}
	})
	if providerRepositoryFixture.err != nil {
		t.Fatalf("initialize provider repository fixture: %v: %s", providerRepositoryFixture.err, providerRepositoryFixture.output)
	}
	return providerRepositoryFixture.repository
}

func verifyWithIndependentConformance(t *testing.T, transcript provider.Transcript) {
	t.Helper()
	base := resolvedTemp(t)
	verifier, buildErr, buildOutput := independentConformanceVerifier()
	if buildErr != nil {
		t.Fatalf("build independent conformance verifier: %v: %s", buildErr, buildOutput)
	}
	contextBytes, err := canonicalJSON(transcript.ConformanceContext)
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{filepath.Join(base, "discovery.json"), filepath.Join(base, "transcript.jsonl"), filepath.Join(base, "context.json")}
	for index, value := range [][]byte{transcript.DiscoveryCanonical, transcript.Receipt.Transcript, contextBytes} {
		if err := os.WriteFile(paths[index], value, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command(verifier, paths[0], paths[1], paths[2])
	output, err := command.CombinedOutput()
	if err != nil || string(output) != "{\"acquisition\":\"VERIFIED\",\"profile\":\"go-live-transcript-conformance/0\",\"status\":\"PASS\",\"transcript\":\"VERIFIED\"}\n" {
		t.Fatalf("independent conformance: %v: %s", err, output)
	}
}

func independentConformanceVerifier() (string, error, []byte) {
	independentConformance.once.Do(func() {
		independentConformance.directory, independentConformance.err = os.MkdirTemp("", "corvint-go-test-provider-")
		if independentConformance.err != nil {
			return
		}
		independentConformance.executable = filepath.Join(independentConformance.directory, "go-live-test-v0")
		cwd, err := os.Getwd()
		if err != nil {
			independentConformance.err = err
			return
		}
		command := exec.Command("go", "build", "-trimpath", "-o", independentConformance.executable, "./conformance/go-live-test-v0")
		command.Dir = filepath.Clean(filepath.Join(cwd, "..", ".."))
		command.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOWORK=off")
		independentConformance.output, independentConformance.err = command.CombinedOutput()
	})
	return independentConformance.executable, independentConformance.err, independentConformance.output
}

func TestHiddenAuthorityModeRequiresInheritedParentCapability(t *testing.T) {
	request := attachmentRequest{
		Profile: requestProfile, Operation: "ACQUIRE", Phase: "ACQUIRE", Ordinal: "1",
		Challenge: strings.Repeat("1", 32), ParentCapabilitySHA256: strings.Repeat("2", 64),
		Bundle: &authorityBundle{}, Authority: &provider.AuthorityRequest{},
	}
	raw, err := canonicalJSON(request)
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(executable, "--experimental", "--trusted-local", attachmentArgument, base64.RawURLEncoding.EncodeToString(raw))
	command.Env = []string{"CORVINT_GO_LIVE_AUTHORITY_PROTOCOL=" + responseProfile}
	output, err := command.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 2 || len(output) != 0 {
		t.Fatalf("hidden mode without parent capability: err=%v output=%q", err, output)
	}
	capability := bytes.Repeat([]byte{7}, 32)
	digest := sha256.Sum256(capability)
	request.ParentCapabilitySHA256 = hex.EncodeToString(digest[:])
	raw, err = canonicalJSON(request)
	if err != nil {
		t.Fatal(err)
	}
	result, runErr := runAuthorityCommand(t.Context(), executable, []string{attachmentArgument, base64.RawURLEncoding.EncodeToString(raw)}, []string{"CORVINT_GO_LIVE_AUTHORITY_PROTOCOL=" + responseProfile}, resolvedTemp(t), time.Second, capability)
	if runErr != nil || result.ExitCode != 2 || len(result.Stdout) != 0 || len(result.Stderr) != 0 {
		t.Fatalf("hidden mode with fd but without trust flags: result=%+v err=%v", result, runErr)
	}
}

func TestAuthorityRejectsReplayedOrDetachedResponse(t *testing.T) {
	bundle := helperBundle(t)
	authority := &localAuthority{bundle: bundle}
	request := attachmentRequest{Profile: requestProfile, Operation: "REVALIDATE", LeaseID: helperLease}
	first, err := authority.invoke(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := authority.invoke(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if first.RequestSHA256 == second.RequestSHA256 || first.Challenge == second.Challenge || first.Ordinal == second.Ordinal {
		t.Fatalf("same-operation replay not separated: first=%+v second=%+v", first, second)
	}
	request.Operation = "OBSERVE"
	third, err := authority.invoke(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if second.RequestSHA256 == third.RequestSHA256 || second.Ordinal == third.Ordinal {
		t.Fatal("cross-phase response replay not separated")
	}
}

func TestCanonicalBindingRejectsDiscoveryAndEnvironmentDetachmentBeforeTransport(t *testing.T) {
	environment := []gorunner.EnvironmentVariable{{Name: "GOENV", Value: "off"}}
	canonical, environmentSHA := canonicalAuthorityEnvironment(environment)
	lease := &localLease{
		discoveryID:    "go-live-discovery:sha256:" + strings.Repeat("1", 64),
		environmentSHA: environmentSHA,
		environment:    canonical,
	}
	request := provider.CanonicalBindingRequest{
		DiscoveryID:       lease.discoveryID,
		EnvironmentSHA256: environmentSHA,
		Environment:       append([]provider.CanonicalEnvironmentVariable(nil), canonical...),
	}
	request.DiscoveryID = "go-live-discovery:sha256:" + strings.Repeat("2", 64)
	if _, err := lease.BindCanonical(t.Context(), request); err == nil {
		t.Fatal("detached discovery accepted")
	}
	request.DiscoveryID = lease.discoveryID
	request.Environment[0].ValueSHA256 = strings.Repeat("3", 64)
	if _, err := lease.BindCanonical(t.Context(), request); err == nil {
		t.Fatal("detached environment accepted")
	}
}

func TestPostLaunchMissingReceiptEmitsNoSyntheticError(t *testing.T) {
	var output bytes.Buffer
	transcript := provider.Transcript{}
	transcript.Runner.Started = true
	if exit := emitProviderFailure(&output, transcript, fmt.Errorf("coordinator failure")); exit != 1 {
		t.Fatalf("exit=%d, want 1", exit)
	}
	if output.Len() != 0 {
		t.Fatalf("post-launch synthetic diagnostic = %q", output.String())
	}
}

func TestAuthorityBundleRejectsDuplicateUnknownAndUnpinnedVerifier(t *testing.T) {
	for _, body := range []string{`{"profile":"x","profile":"y"}`, `{"unknown":"value"}`} {
		path := filepath.Join(t.TempDir(), "bundle.json")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := readBundle(path); err == nil {
			t.Fatalf("ambiguous bundle accepted: %s", body)
		}
	}
	bundle := helperBundle(t)
	bundle.VerifierExecutableSHA256 = strings.Repeat("0", 64)
	content, _ := canonicalJSON(bundle)
	path := filepath.Join(t.TempDir(), "bundle.json")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readBundle(path); err == nil {
		t.Fatal("unpinned verifier accepted")
	}
}

func TestProviderCommandRejectsInvalidArgumentsWithCanonicalDiagnostic(t *testing.T) {
	for _, arguments := range [][]string{{"--unknown"}, {"extra"}} {
		var stdout, stderr bytes.Buffer
		if exitCode := run(arguments, &stdout, &stderr); exitCode != 2 {
			t.Fatalf("exit = %d, want 2", exitCode)
		}
		if stderr.Len() != 0 || !strings.Contains(stdout.String(), `"profile":"go-live-error/0"`) ||
			!strings.Contains(stdout.String(), `"detailSha256":"`) || !strings.HasSuffix(stdout.String(), "\n") {
			t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
		}
	}
}

func helperBundle(t *testing.T) authorityBundle {
	t.Helper()
	verifier, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	verifier, err = filepath.EvalSymlinks(verifier)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := regularFileDigest(verifier)
	if err != nil {
		t.Fatal(err)
	}
	root := resolvedTemp(t)
	return authorityBundle{Profile: attachmentProfile, VerifierExecutable: verifier, VerifierExecutableSHA256: digest, GitExecutable: verifier, GitExecutableSHA256: digest, RepositoryRoot: root, TimeoutMilliseconds: 10_000}
}

func resolvedExecutable(t *testing.T, name string) string {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Fatal(err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(path)
}

func resolvedGitExecutable(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "darwin" {
		command := exec.Command("xcrun", "--find", "git")
		command.Env = []string{"PATH=/usr/bin:/bin"}
		output, err := command.Output()
		if err == nil {
			path := strings.TrimSpace(string(output))
			if path != "" {
				return resolvedPath(t, path)
			}
		}
	}
	return resolvedExecutable(t, "git")
}

func resolvedPath(t *testing.T, path string) string {
	t.Helper()
	path, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(path)
}

func runFixtureGit(t *testing.T, executable, repository string, arguments ...string) {
	t.Helper()
	command := exec.Command(executable, append([]string{"-C", repository}, arguments...)...)
	command.Env = []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "LC_ALL=C"}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %q: %v: %s", arguments, err, output)
	}
}

func runAuthorityHelper(encoded string) int {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return 2
	}
	var request attachmentRequest
	if err := jsonv2.Unmarshal(raw, &request, jsonv2.RejectUnknownMembers(true)); err != nil || request.Profile != requestProfile {
		return 2
	}
	response := attachmentResponse{
		Profile: responseProfile, Operation: request.Operation, Status: "OK", LeaseID: request.LeaseID,
		RequestSHA256: bareID("corvint-go-live-authority-request", requestProfile, raw), VerifierSHA256: request.VerifierExecutableSHA256,
		Phase: request.Phase, Ordinal: request.Ordinal, Challenge: request.Challenge,
	}
	switch request.Operation {
	case "ACQUIRE":
		if request.Authority == nil {
			return 2
		}
		response.LeaseID = helperLease
		moduleCache := filepath.Join(filepath.Dir(request.Authority.RepositoryRoot), "module-cache")
		response.Binding = &provider.AuthorityBinding{
			SourceIdentity: "workspace-source:sha256:" + helperSource, ToolchainIdentity: "go-toolchain:sha256:" + helperToolchain,
			DependencyIdentity: helperDependency, ModuleCacheDirectory: moduleCache, GoVersion: provider.GoVersion,
			GOOS: request.Authority.GOOS, GOARCH: request.Authority.GOARCH, CGOEnabled: "0", EnvironmentProfile: provider.EnvironmentProfile,
		}
	case "DISCOVER":
		if request.LeaseID != helperLease || request.Discovery == nil || len(request.Discovery.PackagePatterns) != 1 {
			return 2
		}
		document, commitments, err := helperDiscovery(request.Discovery)
		if err != nil {
			return 2
		}
		response.DiscoveryBase64 = base64.RawStdEncoding.EncodeToString(document)
		response.Commitments = &commitments
		manifest, err := godiscovery.Decode(bytes.NewReader(document), commitments)
		if err != nil {
			return 2
		}
		environmentValues := make([]any, len(request.Discovery.Environment))
		for index, variable := range request.Discovery.Environment {
			digest := sha256.Sum256([]byte(variable.Value))
			environmentValues[index] = map[string]any{"name": variable.Name, "valueSha256": hex.EncodeToString(digest[:])}
		}
		environmentBody, err := canonicalJSON(environmentValues)
		if err != nil {
			return 2
		}
		empty := sha256.Sum256(nil)
		discoveryDocument, _, err := godiscovery.Compose(manifest, godiscovery.DocumentInput{
			EnvironmentSHA256: bareID("go-environment", "go-environment/0", environmentBody),
			PackagePatterns:   request.Discovery.PackagePatterns, RawStderrSHA256: hex.EncodeToString(empty[:]),
			ToolchainID: "go-toolchain:sha256:" + helperToolchain,
		})
		if err != nil {
			return 2
		}
		response.DiscoveryID = discoveryDocument.ID
	case "REVALIDATE", "RELEASE":
		if request.LeaseID != helperLease {
			return 2
		}
	case "OBSERVE":
		if request.LeaseID != helperLease {
			return 2
		}
		response.Observation = &provider.ReceiptIdentityObservation{SourceSHA256: helperSource, ToolchainSHA256: helperToolObs}
	case "BIND_CANONICAL":
		if request.LeaseID != helperLease || request.Canonical == nil || request.Canonical.SourceIdentity != "workspace-source:sha256:"+helperSource || request.Canonical.ToolchainIdentity != "go-toolchain:sha256:"+helperToolchain || request.Canonical.DependencyIdentity != helperDependency {
			return 2
		}
		response.Canonical = &provider.CanonicalBinding{CapabilityID: "go-live-capability:sha256:" + strings.Repeat("5", 64), PlanID: "go-live-plan:sha256:" + strings.Repeat("6", 64)}
	default:
		return 2
	}
	output, err := canonicalJSON(response)
	if err != nil {
		return 2
	}
	_, _ = os.Stdout.Write(output)
	return 0
}

func helperDiscovery(request *provider.DiscoveryRequest) ([]byte, godiscovery.Commitments, error) {
	const listedName = "cli_test.go"
	importPath := request.PackagePatterns[0]
	rawDirectory := request.WorkingDirectory
	logicalFile := listedName
	directoryDigest, err := helperLogicalDigest(godiscovery.NamespaceSource, ".")
	if err != nil {
		return nil, godiscovery.Commitments{}, err
	}
	fileDigest, err := helperLogicalDigest(godiscovery.NamespaceSource, logicalFile)
	if err != nil {
		return nil, godiscovery.Commitments{}, err
	}
	contents, err := os.ReadFile(filepath.Join(rawDirectory, listedName))
	if err != nil {
		return nil, godiscovery.Commitments{}, err
	}
	rawSHA := sha256.Sum256(contents)
	raw := []byte(fmt.Sprintf(`{"Dir":%q,"ImportPath":%q,"Match":[%q],"Name":"cli","TestGoFiles":[%q]}`, rawDirectory, importPath, importPath, listedName))
	commitments := godiscovery.Commitments{
		DependencyMaterializationSHA256: helperDependency, ModuleMode: godiscovery.ModuleModeModule,
		SourceWSI: "workspace-source:sha256:" + helperSource,
		Packages: []godiscovery.PackageMaterial{{
			ImportPath: importPath, RawDir: rawDirectory, DirNamespace: godiscovery.NamespaceSource,
			DirLogicalPath: ".", DirPathSHA256: directoryDigest,
			Files: []godiscovery.FileCommitment{{Field: godiscovery.FieldTestGoFiles, ListedName: listedName, Role: godiscovery.RoleSource,
				Mode: "0600", Namespace: godiscovery.NamespaceSource, LogicalPath: logicalFile,
				PathSHA256: fileDigest, RawSHA256: hex.EncodeToString(rawSHA[:])}},
		}},
	}
	return raw, commitments, nil
}

func helperLogicalDigest(namespace godiscovery.PathNamespace, logical string) (string, error) {
	body := []byte(fmt.Sprintf(`{"namespace":%q,"path":%q}`, namespace, logical))
	return godiscovery.Hash("go-logical-path", "go-logical-path/0", body)
}

func resolvedTemp(t *testing.T) string {
	t.Helper()
	value, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(value)
}

func executableName(goos string) string {
	if goos == "windows" {
		return "go.exe"
	}
	return "go"
}

func TestProductionParentBindsNonASCIIRepositoryPath(t *testing.T) {
	bundle, _ := providerE2EBundle(t)
	renamed := bundle.RepositoryRoot + " nbsp"
	if err := os.Rename(bundle.RepositoryRoot, renamed); err != nil {
		t.Fatal(err)
	}
	bundle.RepositoryRoot = renamed
	transcript, err := executeProvider(context.Background(), bundle)
	if err != nil || !transcript.CanonicalTerminalSupported {
		t.Fatalf("repository path with U+00A0 was not bound: %v", err)
	}
}
