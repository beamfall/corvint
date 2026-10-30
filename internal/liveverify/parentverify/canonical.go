package parentverify

import (
	"encoding/hex"
	json "encoding/json/v2"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/provider"
)

var unknownReasons = []string{
	"BUILD_CONSTRAINT_VARIANTS", "CROSS_PLATFORM_VARIANTS", "DISCOVERY_INCOMPLETE",
	"EXTERNAL_MODULE_FRONTIER", "FUZZ_BENCHMARK_FRONTIER", "MANDATORY_GATE_OUTSIDE_RUN",
	"NESTED_MODULE_FRONTIER", "NETWORK_STATE_UNKNOWN", "NON_GO_TEST_FRONTIER",
	"NO_AFFECTED_SELECTION_PROOF", "PACKAGE_PATTERN_SEMANTICS", "PARENT_TEST_UNAVAILABLE",
	"SOURCE_ANCHOR_UNAVAILABLE", "UNOBSERVED_DYNAMIC_SUBTESTS",
}

func BindCanonical(snapshot Snapshot, config Config, request provider.CanonicalBindingRequest) (provider.CanonicalBinding, error) {
	if !validPrefixedDigest(request.DiscoveryID, "go-live-discovery:sha256:") || request.EnvironmentSHA256 == "" ||
		!equalStrings(request.PackagePatterns, config.Packages) || request.DependencyIdentity != snapshot.Binding.DependencyIdentity ||
		request.ModuleMode != config.ModuleMode || request.SourceIdentity != snapshot.Binding.SourceIdentity ||
		request.ToolchainIdentity != snapshot.Binding.ToolchainIdentity {
		return provider.CanonicalBinding{}, ErrInvalid
	}
	cwdDigest, err := nativePathDigest(config.GOOS, config.RepositoryRoot)
	if err != nil || request.CWDPathSHA256 != cwdDigest {
		return provider.CanonicalBinding{}, ErrInvalid
	}
	environmentDigest, err := canonicalEnvironmentDigest(request.Environment)
	if err != nil || environmentDigest != request.EnvironmentSHA256 || !profileEnvironment(config, request.Environment) {
		return provider.CanonicalBinding{}, ErrInvalid
	}
	providerBuildBody, err := json.Marshal(map[string]any{
		"executableRawSha256": config.VerifierExecutableSHA256,
		"goVersion":           provider.GoVersion,
	}, json.Deterministic(true))
	if err != nil {
		return provider.CanonicalBinding{}, ErrUnavailable
	}
	providerBuild := bareID("go-provider-build", "go-provider-build/0", providerBuildBody)
	capabilityWithoutID := map[string]any{
		"goProtocol": "go1.27/test2json", "operations": []string{"execute"},
		"profile": "go-live-capability/0", "providerBuildSha256": providerBuild,
		"providerVersion":      ProviderVersion,
		"supportedContainment": []string{"PROCESS_GROUP_BEST_EFFORT"},
		"supportedCoverage":    []string{"NONE"}, "supportedNetwork": []string{"UNKNOWN"},
	}
	capabilityBytes, err := json.Marshal(capabilityWithoutID, json.Deterministic(true))
	if err != nil {
		return provider.CanonicalBinding{}, ErrUnavailable
	}
	capabilityID := prefixedID("go-live-capability", "go-live-capability/0", capabilityBytes)
	toolchainValue := map[string]any{
		"cgoEnabled": snapshot.Toolchain.CGOEnabled, "goarch": snapshot.Toolchain.GOARCH,
		"goenvSha256": snapshot.Toolchain.GoEnvSHA256, "goexeSha256": snapshot.Toolchain.GoExeSHA256,
		"goos": snapshot.Toolchain.GOOS, "gorootSha256": snapshot.Toolchain.GOROOTSHA256,
		"goversion": snapshot.Toolchain.GoVersion, "id": snapshot.Toolchain.ID,
		"invokedToolsSha256": snapshot.Toolchain.InvokedToolsSHA256,
		"pathSha256":         snapshot.Toolchain.PathSHA256, "toolDirSha256": snapshot.Toolchain.ToolDirSHA256,
	}
	invocationArgv := []string{"@PINNED_GO@", "test", "-json", "-count=1", "-vet=off"}
	plan := map[string]any{
		"capabilityId": capabilityID,
		"coverage":     map[string]any{"mode": "NONE", "packagePatterns": []string{}},
		"discoveryId":  request.DiscoveryID,
		"environment":  canonicalEnvironmentValues(request.Environment),
		"invocation":   map[string]any{"argv": invocationArgv, "cwdPathSha256": cwdDigest, "packagePatterns": config.Packages},
		"limits": map[string]any{
			"coverageBytes": "268435456", "coverageFiles": "4096", "cpuMilliseconds": "1800000",
			"eventBytes": "16777216", "events": "100000", "lineBytes": "1048576",
			"memoryBytes": "4294967296", "openFiles": "4096", "outputBytes": "8388608",
			"packages": "4096", "processes": "1024", "runMilliseconds": "1800000", "tests": "100000",
		},
		"limitsEnforced": []string{"EVENT_BYTES", "EVENTS", "LINE_BYTES", "OUTPUT_BYTES", "PACKAGES", "RUN_TIME", "TESTS"},
		"network":        map[string]any{"mechanism": nil, "mode": "UNKNOWN"},
		"profile":        "go-live-plan/0",
		"scope": map[string]any{
			"conclusion": "UNKNOWN", "excluded": []any{}, "requestedPackagePatterns": config.Packages,
			"unknownReasons": append([]string(nil), unknownReasons...),
		},
		"source": map[string]any{
			"dependencyMaterializationSha256": snapshot.Binding.DependencyIdentity,
			"materializationSha256":           snapshot.SourceMaterialization, "moduleMode": wireModuleMode(config.ModuleMode),
			"rootPathSha256": cwdDigest, "wsi": snapshot.Binding.SourceIdentity,
		},
		"toolchain": toolchainValue,
	}
	planBytes, err := json.Marshal(plan, json.Deterministic(true))
	if err != nil {
		return provider.CanonicalBinding{}, ErrUnavailable
	}
	return provider.CanonicalBinding{
		CapabilityID: capabilityID, CapabilityPreimage: append([]byte(nil), capabilityBytes...),
		PlanID: prefixedID("go-live-plan", "go-live-plan/0", planBytes), PlanPreimage: append([]byte(nil), planBytes...),
		VerifierExecutableSHA256: config.VerifierExecutableSHA256,
	}, nil
}

func canonicalEnvironmentDigest(values []provider.CanonicalEnvironmentVariable) (string, error) {
	if len(values) == 0 || !sort.SliceIsSorted(values, func(i, j int) bool { return values[i].Name < values[j].Name }) {
		return "", ErrInvalid
	}
	previous := ""
	for _, value := range values {
		if value.Name == "" || value.Name <= previous || len(value.ValueSHA256) != 64 || value.ValueSHA256 != strings.ToLower(value.ValueSHA256) {
			return "", ErrInvalid
		}
		if _, err := hex.DecodeString(value.ValueSHA256); err != nil {
			return "", ErrInvalid
		}
		previous = value.Name
	}
	body, err := json.Marshal(canonicalEnvironmentValues(values), json.Deterministic(true))
	if err != nil {
		return "", ErrUnavailable
	}
	return bareID("go-environment", "go-environment/0", body), nil
}

func validPrefixedDigest(value, prefix string) bool {
	if !strings.HasPrefix(value, prefix) {
		return false
	}
	digest := strings.TrimPrefix(value, prefix)
	if len(digest) != 64 || digest != strings.ToLower(digest) {
		return false
	}
	_, err := hex.DecodeString(digest)
	return err == nil
}

func canonicalEnvironmentValues(values []provider.CanonicalEnvironmentVariable) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = map[string]any{"name": value.Name, "valueSha256": value.ValueSHA256}
	}
	return result
}
