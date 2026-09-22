package parentverify

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/godiscovery"
	"github.com/Beamfall/corvint/internal/liveverify/gorunner"
	"github.com/Beamfall/corvint/internal/liveverify/provider"
)

func validateAuthorityRequest(config Config, request provider.AuthorityRequest) error {
	if !cleanAbsolute(config.VerifierExecutable) || !cleanAbsolute(config.GoExecutable) || !cleanAbsolute(config.GitExecutable) ||
		!cleanAbsolute(config.RepositoryRoot) || !cleanAbsolute(config.TemporaryParent) ||
		!cleanAbsolute(config.GOROOT) || !cleanAbsolute(config.ModuleCacheDirectory) ||
		config.Timeout != gorunner.MaxRunTime || config.OutputLimitBytes != 8<<20 ||
		!validPrefixedDigest(config.GitExecutableSHA256, "") || config.GOOS != runtime.GOOS || config.GOARCH != runtime.GOARCH ||
		(config.ModuleMode != provider.ModuleReadonly && config.ModuleMode != provider.ModuleWorkspace && config.ModuleMode != provider.ModuleVendor) ||
		!validPackagePatterns(config.Packages) || !equalStrings(config.Packages, request.Packages) {
		return ErrInvalid
	}
	if request.RepositoryRoot != config.RepositoryRoot || request.WorkingDirectory != config.RepositoryRoot ||
		request.GoExecutable != config.GoExecutable || request.GOROOT != config.GOROOT ||
		request.ExpectedGoVersion != provider.GoVersion || request.GOOS != config.GOOS || request.GOARCH != config.GOARCH ||
		request.CGOEnabled != "0" || request.EnvironmentProfile != provider.EnvironmentProfile || request.ModuleMode != config.ModuleMode {
		return ErrInvalid
	}
	if config.ModuleMode == provider.ModuleWorkspace {
		if !cleanAbsolute(config.GoWorkPath) || !within(config.RepositoryRoot, config.GoWorkPath) {
			return ErrInvalid
		}
	} else if config.GoWorkPath != "" {
		return ErrInvalid
	}
	if !within(config.GOROOT, config.GoExecutable) || within(config.RepositoryRoot, config.TemporaryParent) || within(config.RepositoryRoot, config.ModuleCacheDirectory) {
		return ErrInvalid
	}
	return nil
}

func observe(ctx context.Context, config Config, run CommandRunner) (Snapshot, error) {
	if ctx == nil || run == nil || ctx.Err() != nil {
		return Snapshot{}, ErrUnavailable
	}
	verifierDigest, _, err := stableAbsoluteFile(config.VerifierExecutable)
	if err != nil || verifierDigest != config.VerifierExecutableSHA256 {
		return Snapshot{}, fmt.Errorf("verifier executable: %w", ErrDrift)
	}
	repositoryBefore, _, err := observeRepository(ctx, config, run)
	if err != nil {
		return Snapshot{}, err
	}
	source, err := stableTree(ctx, config.RepositoryRoot, godiscovery.NamespaceSource, ".", false)
	if err != nil {
		return Snapshot{}, err
	}
	moduleCache, err := stableTree(ctx, config.ModuleCacheDirectory, godiscovery.NamespaceDependency, "module-cache", true)
	if err != nil {
		return Snapshot{}, err
	}
	goRoot, err := stableTree(ctx, config.GOROOT, godiscovery.NamespaceToolchain, ".", false)
	if err != nil {
		return Snapshot{}, err
	}
	repositoryAfter, dirtyPaths, err := observeRepository(ctx, config, run)
	if err != nil {
		return Snapshot{}, err
	}
	if repositoryBefore != repositoryAfter {
		return Snapshot{}, fmt.Errorf("repository snapshot: %w", ErrDrift)
	}
	standardLibraryRows, err := remapRows(goRoot.inputs, "src/", godiscovery.NamespaceDependency, "standard-library")
	if err != nil || len(standardLibraryRows) == 0 {
		return Snapshot{}, ErrUnavailable
	}
	dependencyRows := append(append([]inputRow(nil), moduleCache.inputs...), standardLibraryRows...)
	dependencyDigest, err := inputSetDigest(dependencyRows)
	if err != nil {
		return Snapshot{}, err
	}
	rootPathSHA, err := nativePathDigest(config.GOOS, config.RepositoryRoot)
	if err != nil {
		return Snapshot{}, err
	}
	sourceBody, err := json.Marshal(map[string]any{
		"materializationSha256": source.digest,
		"moduleMode":            wireModuleMode(config.ModuleMode),
		"packages":              config.Packages,
		"profile":               "go-live-parent-source/0",
		"repository":            repositoryAfter,
		"rootPathSha256":        rootPathSHA,
		"frontier": map[string]any{
			"excluded": []string{"GIT_METADATA"},
			"policy":   "ALL_MATERIALIZED_FILES_HASHED",
		},
	}, json.Deterministic(true))
	if err != nil {
		return Snapshot{}, ErrUnavailable
	}
	sourceObservation := bareID("workspace-source", "workspace-source/0", sourceBody)
	toolchain, toolObservation, err := observeToolchain(ctx, config, goRoot, run)
	if err != nil {
		return Snapshot{}, err
	}
	binding := provider.AuthorityBinding{
		SourceIdentity:       "workspace-source:sha256:" + sourceObservation,
		ToolchainIdentity:    toolchain.ID,
		DependencyIdentity:   dependencyDigest,
		ModuleCacheDirectory: config.ModuleCacheDirectory,
		GoVersion:            provider.GoVersion, GOOS: config.GOOS, GOARCH: config.GOARCH,
		CGOEnabled: "0", EnvironmentProfile: provider.EnvironmentProfile,
		GoWorkPath: config.GoWorkPath,
	}
	moduleCachePathSHA, err := nativePathDigest(config.GOOS, config.ModuleCacheDirectory)
	if err != nil {
		return Snapshot{}, err
	}
	var goWorkPathSHA any
	if config.GoWorkPath != "" {
		if goWorkPathSHA, err = nativePathDigest(config.GOOS, config.GoWorkPath); err != nil {
			return Snapshot{}, err
		}
	}
	leaseBody, err := json.Marshal(map[string]any{
		"dependencyMaterializationSha256": dependencyDigest, "goWorkPathSha256": goWorkPathSHA,
		"moduleCachePathSha256": moduleCachePathSHA,
		"moduleMode":            wireModuleMode(config.ModuleMode), "packages": config.Packages,
		"sourceWsi": binding.SourceIdentity, "toolchainId": binding.ToolchainIdentity,
		"verifierExecutableRawSha256": config.VerifierExecutableSHA256,
	}, json.Deterministic(true))
	if err != nil {
		return Snapshot{}, ErrUnavailable
	}
	return Snapshot{
		Binding: binding, SourceMaterialization: source.digest,
		SourceObservation: sourceObservation, ToolchainObservation: toolObservation,
		LeaseID:   "lease-" + bareID("go-live-parent-lease", "go-live-parent-lease/0", leaseBody),
		Toolchain: toolchain, Repository: repositoryAfter, DirtyPaths: dirtyPaths,
	}, nil
}

func observeToolchain(ctx context.Context, config Config, goRoot treeSnapshot, run CommandRunner) (Toolchain, string, error) {
	goRootDigest, err := treeWalkDigest(append([]inputRow(nil), goRoot.inputs...))
	if err != nil {
		return Toolchain{}, "", err
	}
	toolPrefix := "pkg/tool/" + config.GOOS + "_" + config.GOARCH + "/"
	toolRows := filterRows(goRoot.inputs, toolPrefix)
	if len(toolRows) == 0 {
		return Toolchain{}, "", ErrUnavailable
	}
	toolDirectoryDigest, err := treeWalkDigest(append([]inputRow(nil), toolRows...))
	if err != nil {
		return Toolchain{}, "", err
	}
	invokedRows, err := requiredToolRows(config, toolRows)
	if err != nil {
		return Toolchain{}, "", err
	}
	invokedDigest, err := invokedToolSetDigest(invokedRows)
	if err != nil {
		return Toolchain{}, "", err
	}
	goExeDigest, _, err := stableAbsoluteFile(config.GoExecutable)
	if err != nil {
		return Toolchain{}, "", err
	}
	goEnvDigest, err := observeGoOutputs(ctx, config, run)
	if err != nil {
		return Toolchain{}, "", err
	}
	pathDigest, err := nativePathDigest(config.GOOS, config.GoExecutable)
	if err != nil {
		return Toolchain{}, "", err
	}
	toolchain := Toolchain{
		CGOEnabled: "0", GOARCH: config.GOARCH, GoEnvSHA256: goEnvDigest,
		GoExeSHA256: goExeDigest, GOOS: config.GOOS, GOROOTSHA256: goRootDigest,
		GoVersion: provider.GoVersion, InvokedToolsSHA256: invokedDigest,
		PathSHA256: pathDigest, ToolDirSHA256: toolDirectoryDigest,
	}
	body, err := toolchainBody(toolchain)
	if err != nil {
		return Toolchain{}, "", err
	}
	toolchain.ID = prefixedID("go-toolchain", "go-toolchain/0", body)
	observationBody, err := json.Marshal(map[string]any{
		"goenvSha256": goEnvDigest, "goexeSha256": goExeDigest,
		"gorootSha256": goRootDigest, "invokedToolsSha256": invokedDigest,
		"toolDirSha256": toolDirectoryDigest,
	}, json.Deterministic(true))
	if err != nil {
		return Toolchain{}, "", ErrUnavailable
	}
	return toolchain, bareID("go-toolchain-observation", "go-toolchain-observation/0", observationBody), nil
}

func filterRows(rows []inputRow, prefix string) []inputRow {
	result := make([]inputRow, 0)
	for _, row := range rows {
		if strings.HasPrefix(row.logical, prefix) {
			result = append(result, row)
		}
	}
	return result
}

func remapRows(rows []inputRow, sourcePrefix string, namespace godiscovery.PathNamespace, targetPrefix string) ([]inputRow, error) {
	result := make([]inputRow, 0)
	for _, row := range rows {
		if !strings.HasPrefix(row.logical, sourcePrefix) {
			continue
		}
		logical := targetPrefix + "/" + strings.TrimPrefix(row.logical, sourcePrefix)
		pathDigest, err := logicalPathDigest(namespace, logical)
		if err != nil {
			return nil, err
		}
		result = append(result, inputRow{Mode: row.Mode, PathSHA256: pathDigest, RawSHA256: row.RawSHA256, logical: logical})
	}
	return result, nil
}

func observeGoOutputs(ctx context.Context, config Config, run CommandRunner) (string, error) {
	environment := []string{
		"CGO_ENABLED=0", "GOARCH=" + config.GOARCH, "GOENV=off", "GOOS=" + config.GOOS,
		"GOROOT=" + config.GOROOT, "GOTOOLCHAIN=local",
	}
	commands := [][]string{{"version"}, {"env", "-json", "GOARCH", "CGO_ENABLED", "GOEXE", "GOOS", "GOROOT", "GOTOOLDIR", "GOVERSION"}}
	rows := make([]map[string]any, 0, len(commands))
	for _, argv := range commands {
		result, err := run(ctx, config.GoExecutable, argv, environment, config.RepositoryRoot, config.Timeout)
		if err != nil || !successfulCommand(result) || len(result.Stderr) != 0 || len(result.Stdout) == 0 {
			return "", errors.Join(ErrCommand, err)
		}
		stdoutDigest := sha256.Sum256(result.Stdout)
		stderrDigest := sha256.Sum256(result.Stderr)
		logicalArgv := append([]string{"@PINNED_GO@"}, argv...)
		rows = append(rows, map[string]any{
			"argv": logicalArgv, "exitCode": "0",
			"stderrRawSha256": hex.EncodeToString(stderrDigest[:]),
			"stdoutRawSha256": hex.EncodeToString(stdoutDigest[:]),
		})
		if argv[0] == "version" && strings.TrimSpace(string(result.Stdout)) != "go version "+provider.GoVersion+" "+config.GOOS+"/"+config.GOARCH {
			return "", ErrUnavailable
		}
		if argv[0] == "env" {
			if err := verifyGoEnv(result.Stdout, config); err != nil {
				return "", err
			}
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		left, _ := json.Marshal(rows[i], json.Deterministic(true))
		right, _ := json.Marshal(rows[j], json.Deterministic(true))
		return bytes.Compare(left, right) < 0
	})
	body, err := json.Marshal(rows, json.Deterministic(true))
	if err != nil {
		return "", ErrUnavailable
	}
	return bareID("go-tool-output-set", "go-tool-output-set/0", body), nil
}

func verifyGoEnv(raw []byte, config Config) error {
	var value struct {
		GOARCH     string `json:"GOARCH"`
		CGOEnabled string `json:"CGO_ENABLED"`
		GOEXE      string `json:"GOEXE"`
		GOOS       string `json:"GOOS"`
		GOROOT     string `json:"GOROOT"`
		GOTOOLDIR  string `json:"GOTOOLDIR"`
		GOVERSION  string `json:"GOVERSION"`
	}
	if err := json.Unmarshal(raw, &value, json.RejectUnknownMembers(true)); err != nil {
		return ErrUnavailable
	}
	if value.GOARCH != config.GOARCH || value.CGOEnabled != "0" || value.GOEXE != goExecutableSuffix(config.GOOS) || value.GOOS != config.GOOS ||
		value.GOROOT != config.GOROOT || value.GOTOOLDIR != filepath.Join(config.GOROOT, "pkg", "tool", config.GOOS+"_"+config.GOARCH) ||
		value.GOVERSION != provider.GoVersion {
		return ErrUnavailable
	}
	return nil
}

func goExecutableSuffix(goos string) string {
	if goos == "windows" {
		return ".exe"
	}
	return ""
}

func validPackagePatterns(values []string) bool {
	if len(values) == 1 && values[0] == "./..." {
		return true
	}
	if len(values) == 0 || len(values) > gorunner.MaxPackages || !sort.StringsAreSorted(values) {
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

func requiredToolRows(config Config, rows []inputRow) ([]inputRow, error) {
	wanted := map[string]bool{
		"pkg/tool/" + config.GOOS + "_" + config.GOARCH + "/asm":     false,
		"pkg/tool/" + config.GOOS + "_" + config.GOARCH + "/compile": false,
		"pkg/tool/" + config.GOOS + "_" + config.GOARCH + "/link":    false,
	}
	result := make([]inputRow, 0, len(wanted))
	for _, row := range rows {
		for path := range wanted {
			digest, _ := logicalPathDigest(godiscovery.NamespaceToolchain, path)
			if row.PathSHA256 == digest {
				wanted[path] = true
				result = append(result, row)
			}
		}
	}
	for _, found := range wanted {
		if !found {
			return nil, ErrUnavailable
		}
	}
	return result, nil
}

func successfulCommand(result CommandResult) bool {
	return result.Started && result.Exited && result.WaitCompleted && result.ExitCode == 0 &&
		result.ProcessCleanupDone && result.PipesDrained && !result.Cancelled && !result.TimedOut && !result.OutputLimitExceeded
}

func stableAbsoluteFile(path string) (string, string, error) {
	return stableAbsoluteRegular(path, true)
}

func stableExecutableFile(path string) (string, string, error) {
	return stableAbsoluteRegular(path, false)
}

func stableAbsoluteRegular(path string, requireSingleLink bool) (string, string, error) {
	if !cleanAbsolute(path) {
		return "", "", ErrInvalid
	}
	parent, name := filepath.Dir(path), filepath.Base(path)
	root, err := os.OpenRoot(parent)
	if err != nil {
		return "", "", ErrUnavailable
	}
	defer root.Close()
	info, err := root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || requireSingleLink && fileLinkCount(info) != 1 {
		return "", "", ErrUnavailable
	}
	digest, _, err := readStableFile(root, name, info)
	return digest, fmt.Sprintf("%04o", info.Mode().Perm()), err
}

func toolchainBody(toolchain Toolchain) ([]byte, error) {
	return json.Marshal(map[string]any{
		"cgoEnabled": toolchain.CGOEnabled, "goarch": toolchain.GOARCH,
		"goenvSha256": toolchain.GoEnvSHA256, "goexeSha256": toolchain.GoExeSHA256,
		"goos": toolchain.GOOS, "gorootSha256": toolchain.GOROOTSHA256,
		"goversion": toolchain.GoVersion, "invokedToolsSha256": toolchain.InvokedToolsSHA256,
		"pathSha256": toolchain.PathSHA256, "toolDirSha256": toolchain.ToolDirSHA256,
	}, json.Deterministic(true))
}

func nativePathDigest(goos, path string) (string, error) {
	body, err := json.Marshal(map[string]any{"goos": goos, "path": path}, json.Deterministic(true))
	if err != nil {
		return "", ErrUnavailable
	}
	return bareID("go-native-path", "go-native-path/0", body), nil
}

func wireModuleMode(mode provider.ModuleMode) string {
	switch mode {
	case provider.ModuleReadonly:
		return "MODULE"
	case provider.ModuleWorkspace:
		return "WORKSPACE"
	case provider.ModuleVendor:
		return "VENDOR"
	default:
		return ""
	}
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

func within(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}
