package parentverify

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/godiscovery"
	"github.com/Beamfall/corvint/internal/liveverify/provider"
)

func TestLeaseRevalidationDetectsSourceDrift(t *testing.T) {
	config, request, run := verifierFixture(t)
	first, err := Acquire(t.Context(), config, request, run)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Revalidate(t.Context(), config, request, first.LeaseID, run)
	if err != nil {
		t.Fatal(err)
	}
	if first.LeaseID != second.LeaseID || first.Binding != second.Binding || first.SourceObservation != second.SourceObservation || first.ToolchainObservation != second.ToolchainObservation {
		t.Fatalf("stable authority changed: first=%+v second=%+v", first, second)
	}
	if err := os.WriteFile(filepath.Join(config.RepositoryRoot, "probe.go"), []byte("package probe\nvar Changed = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Revalidate(t.Context(), config, request, first.LeaseID, run); err == nil || !strings.Contains(err.Error(), "drift") {
		t.Fatalf("source drift error = %v", err)
	}
}

func TestAcquireRejectsWritableDependencyMaterialization(t *testing.T) {
	config, request, run := verifierFixture(t)
	if err := os.Chmod(config.ModuleCacheDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Acquire(t.Context(), config, request, run); err == nil {
		t.Fatal("writable module cache accepted")
	}
}

func TestStableTreeHonorsCancellationBeforeWork(t *testing.T) {
	config, _, _ := verifierFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := stableTree(ctx, config.RepositoryRoot, godiscovery.NamespaceSource, ".", false); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled stable tree error = %v", err)
	}
}

func TestStableTreeDoesNotTrustRestoredMetadataAsContent(t *testing.T) {
	root := resolved(t, t.TempDir())
	path := filepath.Join(root, "same-metadata.go")
	modified := time.Unix(1_700_000_000, 123_000_000)
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, modified, modified); err != nil {
			t.Fatal(err)
		}
	}
	write("aaaa")
	first, err := stableTree(t.Context(), root, godiscovery.NamespaceSource, ".", false)
	if err != nil {
		t.Fatal(err)
	}
	write("bbbb")
	second, err := stableTree(t.Context(), root, godiscovery.NamespaceSource, ".", false)
	if err != nil {
		t.Fatal(err)
	}
	if first.metadataDigest != second.metadataDigest {
		t.Fatal("fixture metadata was not restored")
	}
	if first.digest == second.digest {
		t.Fatal("restored size/mode/mtime poisoned the content identity")
	}
}

func TestCanonicalBindingIsDeterministicAndAuthorityBound(t *testing.T) {
	config, request, run := verifierFixture(t)
	snapshot, err := Acquire(t.Context(), config, request, run)
	if err != nil {
		t.Fatal(err)
	}
	// GLTP-V0-007: the parent binds only the closed registered environment
	// profile; TestCanonicalBindingRefusesEnvironmentOutsideProfile covers the
	// partial and altered shapes.
	environment := profileEnvironmentFixture(config)
	environmentSHA, err := canonicalEnvironmentDigest(environment)
	if err != nil {
		t.Fatal(err)
	}
	cwdSHA, _ := nativePathDigest(config.GOOS, config.RepositoryRoot)
	input := provider.CanonicalBindingRequest{
		DiscoveryID:       "go-live-discovery:sha256:" + strings.Repeat("1", 64),
		EnvironmentSHA256: environmentSHA, PackagePatterns: append([]string(nil), config.Packages...),
		CWDPathSHA256: cwdSHA, DependencyIdentity: snapshot.Binding.DependencyIdentity,
		ModuleMode: config.ModuleMode, SourceIdentity: snapshot.Binding.SourceIdentity,
		ToolchainIdentity: snapshot.Binding.ToolchainIdentity, Environment: environment,
	}
	first, err := BindCanonical(snapshot, config, input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BindCanonical(snapshot, config, input)
	if err != nil || !reflect.DeepEqual(first, second) || !strings.HasPrefix(first.CapabilityID, "go-live-capability:sha256:") || !strings.HasPrefix(first.PlanID, "go-live-plan:sha256:") {
		t.Fatalf("canonical binding is unstable: first=%+v second=%+v err=%v", first, second, err)
	}
	// GLTP-V0-038/042/043: these are frozen-profile change detectors, not
	// evidence for resource observations, exclusions, or a shadow execution mode.
	var plan map[string]any
	if err := json.Unmarshal(first.PlanPreimage, &plan); err != nil {
		t.Fatal(err)
	}
	wantLimits := []any{"EVENT_BYTES", "EVENTS", "LINE_BYTES", "OUTPUT_BYTES", "PACKAGES", "RUN_TIME", "TESTS"}
	if !reflect.DeepEqual(plan["limitsEnforced"], wantLimits) {
		t.Fatalf("enforced limits = %v", plan["limitsEnforced"])
	}
	scope := plan["scope"].(map[string]any)
	if !reflect.DeepEqual(scope["excluded"], []any{}) {
		t.Fatalf("excluded = %v", scope["excluded"])
	}
	wantUnknown := []any{"BUILD_CONSTRAINT_VARIANTS", "CROSS_PLATFORM_VARIANTS", "DISCOVERY_INCOMPLETE", "EXTERNAL_MODULE_FRONTIER", "FUZZ_BENCHMARK_FRONTIER", "MANDATORY_GATE_OUTSIDE_RUN", "NESTED_MODULE_FRONTIER", "NETWORK_STATE_UNKNOWN", "NON_GO_TEST_FRONTIER", "NO_AFFECTED_SELECTION_PROOF", "PACKAGE_PATTERN_SEMANTICS", "PARENT_TEST_UNAVAILABLE", "SOURCE_ANCHOR_UNAVAILABLE", "UNOBSERVED_DYNAMIC_SUBTESTS"}
	if !reflect.DeepEqual(scope["unknownReasons"], wantUnknown) {
		t.Fatalf("unknown reasons = %v", scope["unknownReasons"])
	}
	receiptScope := composeScopeQualificationReceipt(t, snapshot, input, first)
	for _, field := range []string{"excluded", "unknownReasons"} {
		if !reflect.DeepEqual(scope[field], receiptScope[field]) {
			t.Fatalf("plan/receipt %s differs: plan=%v receipt=%v", field, scope[field], receiptScope[field])
		}
	}
	input.EnvironmentSHA256 = strings.Repeat("0", 64)
	if _, err := BindCanonical(snapshot, config, input); err == nil {
		t.Fatal("detached environment digest accepted")
	}
}

func verifierFixture(t *testing.T) (Config, provider.AuthorityRequest, CommandRunner) {
	t.Helper()
	base := resolved(t, t.TempDir())
	repository := filepath.Join(base, "repository")
	temporary := filepath.Join(base, "temporary")
	moduleCache := filepath.Join(base, "module-cache")
	goRoot := filepath.Join(base, "goroot")
	gitExecutable := filepath.Join(base, "git")
	toolDirectory := filepath.Join(goRoot, "pkg", "tool", runtime.GOOS+"_"+runtime.GOARCH)
	for _, path := range []string{repository, temporary, moduleCache, filepath.Join(repository, ".git", "objects"), filepath.Join(repository, ".git", "refs"), filepath.Join(goRoot, "src", "runtime"), filepath.Join(goRoot, "bin"), toolDirectory} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for path, body := range map[string]string{
		filepath.Join(repository, "go.mod"):         "module example.test/probe\n\ngo 1.27.0\n",
		filepath.Join(repository, "probe.go"):       "package probe\n",
		filepath.Join(repository, ".git", "config"): "[core]\nrepositoryformatversion = 0\nbare = false\n",
		filepath.Join(repository, ".git", "HEAD"):   strings.Repeat("a", 40) + "\n",
		gitExecutable: "pinned-git",
		filepath.Join(goRoot, "src", "runtime", "runtime.go"):      "package runtime\n",
		filepath.Join(goRoot, "bin", executableName(runtime.GOOS)): "pinned-go",
		filepath.Join(toolDirectory, "asm"):                        "asm", filepath.Join(toolDirectory, "compile"): "compile", filepath.Join(toolDirectory, "link"): "link",
	} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// The injected runner controls object/status bytes; private status still
	// requires a real, structurally valid metadata snapshot at its boundary.
	index := []byte("DIRC\x00\x00\x00\x02\x00\x00\x00\x00")
	indexDigest := sha1.Sum(index)
	if err := os.WriteFile(filepath.Join(repository, ".git", "index"), append(index, indexDigest[:]...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(moduleCache, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(moduleCache, 0o700) })
	verifier, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	verifier = resolved(t, verifier)
	verifierDigest, _, err := stableAbsoluteFile(verifier)
	if err != nil {
		t.Fatal(err)
	}
	goExecutable := filepath.Join(goRoot, "bin", executableName(runtime.GOOS))
	gitDigest, _, err := stableAbsoluteFile(gitExecutable)
	if err != nil {
		t.Fatal(err)
	}
	config := Config{
		VerifierExecutable: verifier, VerifierExecutableSHA256: verifierDigest,
		GitExecutable: gitExecutable, GitExecutableSHA256: gitDigest,
		GoExecutable: goExecutable, GOARCH: runtime.GOARCH, GOOS: runtime.GOOS, GOROOT: goRoot,
		ModuleCacheDirectory: moduleCache, ModuleMode: provider.ModuleReadonly,
		OutputLimitBytes: 8 << 20, Packages: []string{"example.test/probe"},
		RepositoryRoot: repository, TemporaryParent: temporary, Timeout: 30 * time.Minute,
	}
	request := provider.AuthorityRequest{
		RepositoryRoot: repository, WorkingDirectory: repository, GoExecutable: goExecutable, GOROOT: goRoot,
		ExpectedGoVersion: provider.GoVersion, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
		CGOEnabled: "0", EnvironmentProfile: provider.EnvironmentProfile,
		ModuleMode: provider.ModuleReadonly, Packages: []string{"example.test/probe"},
	}
	run := func(_ context.Context, executable string, argv, _ []string, _ string, _ time.Duration) (CommandResult, error) {
		var stdout string
		if executable == gitExecutable {
			joined := strings.Join(argv, " ")
			switch {
			case strings.Contains(joined, "rev-parse --show-object-format"):
				stdout = "sha1\n" + strings.Repeat("a", 40) + "\n" + strings.Repeat("b", 40) + "\n"
			case strings.Contains(joined, "rev-parse --path-format=absolute"):
				stdout = repository + "\n" + filepath.Join(repository, ".git") + "\n" + filepath.Join(repository, ".git") + "\n"
			case strings.Contains(joined, "ls-files --stage"):
				stdout = "100644 " + strings.Repeat("c", 40) + " 0\tprobe.go\x00"
			case strings.Contains(joined, "status --porcelain"):
				stdout = ""
			case strings.Contains(joined, "config --local"):
				stdout = "core.repositoryformatversion\n0\x00"
			case strings.HasSuffix(joined, " version"):
				stdout = "git version 2.test\n"
			default:
				return CommandResult{}, ErrCommand
			}
			return CommandResult{Stdout: []byte(stdout), ExitCode: 0, Started: true, Exited: true, ProcessCleanupDone: true, PipesDrained: true, WaitCompleted: true}, nil
		}
		switch argv[0] {
		case "version":
			stdout = "go version " + provider.GoVersion + " " + runtime.GOOS + "/" + runtime.GOARCH + "\n"
		case "env":
			stdout = fmt.Sprintf("{\"CGO_ENABLED\":\"0\",\"GOARCH\":%q,\"GOEXE\":%q,\"GOOS\":%q,\"GOROOT\":%q,\"GOTOOLDIR\":%q,\"GOVERSION\":%q}\n", runtime.GOARCH, executableSuffix(runtime.GOOS), runtime.GOOS, goRoot, toolDirectory, provider.GoVersion)
		default:
			return CommandResult{}, ErrCommand
		}
		return CommandResult{Stdout: []byte(stdout), ExitCode: 0, Started: true, Exited: true, ProcessCleanupDone: true, PipesDrained: true, WaitCompleted: true}, nil
	}
	return config, request, run
}

func resolved(t *testing.T, path string) string {
	t.Helper()
	value, err := filepath.EvalSymlinks(path)
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

func executableSuffix(goos string) string {
	if goos == "windows" {
		return ".exe"
	}
	return ""
}

func digestText(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func TestCanonicalBindingRefusesEnvironmentOutsideProfile(t *testing.T) {
	config, request, run := verifierFixture(t)
	snapshot, err := Acquire(t.Context(), config, request, run)
	if err != nil {
		t.Fatal(err)
	}
	cwdSHA, _ := nativePathDigest(config.GOOS, config.RepositoryRoot)
	bind := func(environment []provider.CanonicalEnvironmentVariable) error {
		environmentSHA, err := canonicalEnvironmentDigest(environment)
		if err != nil {
			t.Fatal(err)
		}
		_, err = BindCanonical(snapshot, config, provider.CanonicalBindingRequest{
			DiscoveryID:       "go-live-discovery:sha256:" + strings.Repeat("1", 64),
			EnvironmentSHA256: environmentSHA, PackagePatterns: append([]string(nil), config.Packages...),
			CWDPathSHA256: cwdSHA, DependencyIdentity: snapshot.Binding.DependencyIdentity,
			ModuleMode: config.ModuleMode, SourceIdentity: snapshot.Binding.SourceIdentity,
			ToolchainIdentity: snapshot.Binding.ToolchainIdentity, Environment: environment,
		})
		return err
	}
	if err := bind(profileEnvironmentFixture(config)); err != nil {
		t.Fatalf("closed profile environment refused: %v", err)
	}
	partial := []provider.CanonicalEnvironmentVariable{
		{Name: "CGO_ENABLED", ValueSHA256: digestText("0")},
		{Name: "GOTOOLCHAIN", ValueSHA256: digestText("local")},
	}
	altered := profileEnvironmentFixture(config)
	for index := range altered {
		if altered[index].Name == "GOFLAGS" {
			altered[index].ValueSHA256 = digestText("-mod=mod")
		}
	}
	extra := append(profileEnvironmentFixture(config), provider.CanonicalEnvironmentVariable{Name: "USER", ValueSHA256: digestText("corvint")})
	for name, environment := range map[string][]provider.CanonicalEnvironmentVariable{"partial": partial, "altered GOFLAGS": altered, "extra USER": extra} {
		if err := bind(environment); !errors.Is(err, ErrInvalid) {
			t.Fatalf("%s environment bound: %v", name, err)
		}
	}
}

// profileEnvironmentFixture is the GO127_CGO0_OFFLINE_POSIX_0 environment for config
// with placeholder run-root values, as the provider's canonical request sends it.
func profileEnvironmentFixture(config Config) []provider.CanonicalEnvironmentVariable {
	goFlags, goWork := "-mod=readonly", "off"
	if config.ModuleMode == provider.ModuleVendor {
		goFlags = "-mod=vendor"
	}
	if config.ModuleMode == provider.ModuleWorkspace {
		goWork = config.GoWorkPath
	}
	values := [][2]string{
		{"CGO_ENABLED", "0"}, {"GOARCH", config.GOARCH}, {"GOCACHE", "/run/go-build-cache"}, {"GOENV", "off"},
		{"GOFLAGS", goFlags}, {"GOMODCACHE", config.ModuleCacheDirectory}, {"GONOPROXY", ""}, {"GONOSUMDB", ""},
		{"GOOS", config.GOOS}, {"GOPRIVATE", ""}, {"GOPROXY", "off"}, {"GOROOT", config.GOROOT}, {"GOSUMDB", "off"},
		{"GOTMPDIR", "/run/go-tmp"}, {"GOTOOLCHAIN", "local"}, {"GOVCS", "*:off"}, {"GOWORK", goWork},
		{"HOME", "/run/home"}, {"TEMP", "/run/tmp"}, {"TMP", "/run/tmp"}, {"TMPDIR", "/run/tmp"},
	}
	result := make([]provider.CanonicalEnvironmentVariable, len(values))
	for index, value := range values {
		result[index] = provider.CanonicalEnvironmentVariable{Name: value[0], ValueSHA256: digestText(value[1])}
	}
	return result
}

func TestLeaseRevalidationRefusesAnotherSelectedPath(t *testing.T) {
	config, request, run := verifierFixture(t)
	for _, name := range []string{"go.work", filepath.Join("other", "go.work")} {
		path := filepath.Join(config.RepositoryRoot, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("go 1.27.0\n\nuse .\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	copied := filepath.Join(filepath.Dir(config.ModuleCacheDirectory), "module-cache-copy")
	if err := os.Mkdir(copied, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(copied, 0o700) })
	config.ModuleMode, request.ModuleMode = provider.ModuleWorkspace, provider.ModuleWorkspace
	config.GoWorkPath = filepath.Join(config.RepositoryRoot, "go.work")
	lease, err := Acquire(t.Context(), config, request, run)
	if err != nil {
		t.Fatal(err)
	}
	moved := config
	moved.ModuleCacheDirectory = copied
	switched := config
	switched.GoWorkPath = filepath.Join(config.RepositoryRoot, "other", "go.work")
	for name, other := range map[string]Config{"module cache": moved, "go.work": switched} {
		if _, err := Revalidate(t.Context(), other, request, lease.LeaseID, run); !errors.Is(err, ErrDrift) {
			t.Errorf("%s path changed under the same content: Revalidate error = %v, want ErrDrift", name, err)
		}
	}
}
