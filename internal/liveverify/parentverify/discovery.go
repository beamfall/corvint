package parentverify

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	stdjson "encoding/json"
	json "encoding/json/v2"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/godiscovery"
	"github.com/Beamfall/corvint/internal/liveverify/gorunner"
	"github.com/Beamfall/corvint/internal/liveverify/provider"
)

type rawModule struct {
	Path, Version, Time, Dir, GoMod, GoVersion, Sum, GoModSum string
	Main, Indirect                                            bool
	Replace                                                   *rawModule
}

type rawPackage struct {
	Dir, ImportPath, Name, ForTest string
	Match                          []string
	DepOnly                        bool
	Module                         *rawModule
	GoFiles, CgoFiles, CFiles, CXXFiles, MFiles, HFiles,
	FFiles, SFiles, SwigFiles, SwigCXXFiles, SysoFiles,
	EmbedFiles, TestGoFiles, XTestGoFiles, TestEmbedFiles,
	XTestEmbedFiles []string
}

func Discover(ctx context.Context, config Config, request provider.AuthorityRequest, leaseID string, discovery provider.DiscoveryRequest, run CommandRunner) (Discovery, error) {
	snapshot, err := Revalidate(ctx, config, request, leaseID, run)
	if err != nil {
		return Discovery{}, err
	}
	if err := validateDiscoveryRequest(config, discovery); err != nil {
		return Discovery{}, err
	}
	arguments := []string{"list", "-deps", "-test", "-json=" + godiscovery.ClosedFields}
	arguments = append(arguments, discovery.PackagePatterns...)
	result, err := run(ctx, config.GoExecutable, arguments, environmentStrings(discovery.Environment), config.RepositoryRoot, config.Timeout)
	if err != nil || !successfulCommand(result) || len(result.Stderr) != 0 || len(result.Stdout) == 0 || len(result.Stdout) > godiscovery.MaxDiscoveryBytes {
		return Discovery{}, errors.Join(ErrCommand, err)
	}
	commitments, err := acquireCommitments(result.Stdout, snapshot.Binding, config, discovery.RunRoot)
	if err != nil {
		return Discovery{}, err
	}
	// The independent decoder closes the raw field grammar and proves that every
	// path/file commitment collected above was both necessary and sufficient.
	manifest, err := godiscovery.Decode(bytes.NewReader(result.Stdout), commitments)
	if err != nil {
		return Discovery{}, errors.Join(ErrUnavailable, err)
	}
	environmentSHA, err := discoveryEnvironmentDigest(discovery.Environment)
	if err != nil {
		return Discovery{}, err
	}
	empty := sha256.Sum256(nil)
	document, _, err := godiscovery.Compose(manifest, godiscovery.DocumentInput{
		EnvironmentSHA256: environmentSHA, PackagePatterns: discovery.PackagePatterns,
		RawStderrSHA256: hex.EncodeToString(empty[:]), ToolchainID: snapshot.Binding.ToolchainIdentity,
	})
	if err != nil {
		return Discovery{}, errors.Join(ErrUnavailable, err)
	}
	return Discovery{Raw: append([]byte(nil), result.Stdout...), Commitments: commitments, ID: document.ID}, nil
}

func validateDiscoveryRequest(config Config, request provider.DiscoveryRequest) error {
	if request.GoExecutable != config.GoExecutable || request.WorkingDirectory != config.RepositoryRoot ||
		!equalStrings(request.PackagePatterns, config.Packages) || !cleanAbsolute(request.RunRoot) ||
		!within(config.TemporaryParent, request.RunRoot) || within(config.RepositoryRoot, request.RunRoot) {
		return ErrInvalid
	}
	values := make(map[string]string, len(request.Environment))
	previous := ""
	for _, variable := range request.Environment {
		if variable.Name == "" || variable.Name <= previous {
			return ErrInvalid
		}
		previous = variable.Name
		values[variable.Name] = variable.Value
	}
	if len(values) != len(profileEnvironmentNames) {
		return ErrInvalid
	}
	for _, name := range profileEnvironmentNames {
		if _, ok := values[name]; !ok {
			return ErrInvalid
		}
	}
	for name, want := range profileFixedValues(config) {
		if values[name] != want {
			return ErrInvalid
		}
	}
	paths := map[string]string{
		"GOCACHE": filepath.Join(request.RunRoot, "go-build-cache"), "GOTMPDIR": filepath.Join(request.RunRoot, "go-tmp"),
		"HOME": filepath.Join(request.RunRoot, "home"), "TEMP": filepath.Join(request.RunRoot, "tmp"),
		"TMP": filepath.Join(request.RunRoot, "tmp"), "TMPDIR": filepath.Join(request.RunRoot, "tmp"),
	}
	for name, want := range paths {
		if values[name] != want {
			return ErrInvalid
		}
		info, err := os.Lstat(want)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o700 {
			return ErrUnavailable
		}
	}
	return nil
}

// profileEnvironmentNames is the closed GLTP-V0-007 name set of profile
// GO127_CGO0_OFFLINE_POSIX_0.
var profileEnvironmentNames = []string{"CGO_ENABLED", "GOARCH", "GOCACHE", "GOENV", "GOFLAGS", "GOMODCACHE", "GONOPROXY", "GONOSUMDB", "GOOS", "GOPRIVATE", "GOPROXY", "GOROOT", "GOSUMDB", "GOTOOLCHAIN", "GOTMPDIR", "GOVCS", "GOWORK", "HOME", "TEMP", "TMP", "TMPDIR"}

// profileFixedValues returns the GLTP-V0-007 values that do not depend on the
// run root.
func profileFixedValues(config Config) map[string]string {
	goFlags := "-mod=readonly"
	if config.ModuleMode == provider.ModuleVendor {
		goFlags = "-mod=vendor"
	}
	goWork := "off"
	if config.ModuleMode == provider.ModuleWorkspace {
		goWork = config.GoWorkPath
	}
	return map[string]string{
		"CGO_ENABLED": "0", "GOARCH": config.GOARCH, "GOENV": "off", "GOFLAGS": goFlags,
		"GOMODCACHE": config.ModuleCacheDirectory, "GONOPROXY": "", "GONOSUMDB": "", "GOOS": config.GOOS,
		"GOPRIVATE": "", "GOPROXY": "off", "GOROOT": config.GOROOT, "GOSUMDB": "off",
		"GOTOOLCHAIN": "local", "GOVCS": "*:off", "GOWORK": goWork,
	}
}

// profileEnvironment reports whether a canonical environment has exactly the
// GLTP-V0-007 names and the digests of every run-root-independent fixed value.
// The run-root paths were checked at discovery and are not known here.
func profileEnvironment(config Config, values []provider.CanonicalEnvironmentVariable) bool {
	if len(values) != len(profileEnvironmentNames) {
		return false
	}
	digests := make(map[string]string, len(values))
	for _, value := range values {
		digests[value.Name] = value.ValueSHA256
	}
	for _, name := range profileEnvironmentNames {
		if _, ok := digests[name]; !ok {
			return false
		}
	}
	for name, want := range profileFixedValues(config) {
		digest := sha256.Sum256([]byte(want))
		if digests[name] != hex.EncodeToString(digest[:]) {
			return false
		}
	}
	return true
}

func acquireCommitments(raw []byte, binding provider.AuthorityBinding, config Config, runRoot string) (godiscovery.Commitments, error) {
	decoder := stdjson.NewDecoder(bytes.NewReader(raw))
	commitments := godiscovery.Commitments{
		DependencyMaterializationSHA256: binding.DependencyIdentity,
		ModuleMode:                      godiscovery.ModuleMode(wireModuleMode(config.ModuleMode)),
		SourceWSI:                       binding.SourceIdentity,
	}
	modulePaths := make(map[string]godiscovery.PathCommitment)
	seenPackages := make(map[string]bool)
	for {
		var pack rawPackage
		if err := decoder.Decode(&pack); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return godiscovery.Commitments{}, ErrUnavailable
		}
		if pack.ImportPath == "" || seenPackages[pack.ImportPath] || len(commitments.Packages) == godiscovery.MaxPackages {
			return godiscovery.Commitments{}, ErrUnavailable
		}
		seenPackages[pack.ImportPath] = true
		namespace, logicalDirectory, err := classifyPath(pack.Dir, config)
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
			for _, name := range field.names {
				commitment, err := fileCommitment(pack, field.field, name, namespace, logicalDirectory, config, runRoot)
				if err != nil {
					return godiscovery.Commitments{}, err
				}
				material.Files = append(material.Files, commitment)
			}
		}
		commitments.Packages = append(commitments.Packages, material)
		if err := collectModulePaths(pack.Module, config, modulePaths); err != nil {
			return godiscovery.Commitments{}, err
		}
	}
	for _, path := range modulePaths {
		commitments.ModulePaths = append(commitments.ModulePaths, path)
	}
	sort.Slice(commitments.ModulePaths, func(i, j int) bool { return commitments.ModulePaths[i].RawPath < commitments.ModulePaths[j].RawPath })
	return commitments, nil
}

func fileCommitment(pack rawPackage, field godiscovery.FileField, name string, namespace godiscovery.PathNamespace, logicalDirectory string, config Config, runRoot string) (godiscovery.FileCommitment, error) {
	path := filepath.Join(pack.Dir, name)
	if filepath.IsAbs(name) {
		path = name
	}
	digest, mode, err := stableAbsoluteFile(path)
	if err != nil {
		return godiscovery.FileCommitment{}, err
	}
	commitment := godiscovery.FileCommitment{
		Field: field, ListedName: name, Role: godiscovery.RoleSource, Mode: mode,
		Namespace: namespace, LogicalPath: filepath.ToSlash(filepath.Join(logicalDirectory, name)), RawSHA256: digest,
	}
	if filepath.IsAbs(name) {
		cacheRoot := filepath.Join(runRoot, "go-build-cache")
		if !within(cacheRoot, name) || pack.Name != "main" || pack.ForTest != "" || len(pack.Match) != 0 || field != godiscovery.FieldGoFiles {
			return godiscovery.FileCommitment{}, ErrUnavailable
		}
		commitment.Role, commitment.Namespace, commitment.LogicalPath = godiscovery.RoleGeneratedTestMain, "", ""
		body, _ := stdjson.Marshal(map[string]any{"importPath": pack.ImportPath, "mode": mode, "rawSha256": digest, "role": "TEST_MAIN_GOFILE"})
		commitment.PathSHA256 = bareID("go-generated-testmain", "go-generated-testmain/0", body)
		return commitment, nil
	}
	if !withinNamespacePath(path, namespace, config) {
		return godiscovery.FileCommitment{}, ErrUnavailable
	}
	commitment.PathSHA256, err = logicalPathDigest(namespace, commitment.LogicalPath)
	return commitment, err
}

func collectModulePaths(module *rawModule, config Config, values map[string]godiscovery.PathCommitment) error {
	if module == nil {
		return nil
	}
	for _, rawPath := range []string{module.Dir, module.GoMod} {
		if rawPath == "" {
			continue
		}
		namespace, logical, err := classifyPath(rawPath, config)
		if err != nil || namespace == godiscovery.NamespaceToolchain {
			return ErrUnavailable
		}
		digest, err := logicalPathDigest(namespace, logical)
		if err != nil {
			return err
		}
		values[rawPath] = godiscovery.PathCommitment{RawPath: rawPath, Namespace: namespace, LogicalPath: logical, PathSHA256: digest}
	}
	return collectModulePaths(module.Replace, config, values)
}

func classifyPath(path string, config Config) (godiscovery.PathNamespace, string, error) {
	for _, candidate := range []struct {
		root      string
		namespace godiscovery.PathNamespace
	}{
		{config.RepositoryRoot, godiscovery.NamespaceSource},
		{config.ModuleCacheDirectory, godiscovery.NamespaceDependency},
		{config.GOROOT, godiscovery.NamespaceToolchain},
	} {
		if !within(candidate.root, path) {
			continue
		}
		logical, err := filepath.Rel(candidate.root, path)
		if err != nil || logical == ".." || strings.HasPrefix(logical, ".."+string(filepath.Separator)) {
			return "", "", ErrUnavailable
		}
		if err := rejectSymlinkComponents(candidate.root, logical); err != nil {
			return "", "", err
		}
		return candidate.namespace, filepath.ToSlash(logical), nil
	}
	return "", "", ErrUnavailable
}

func rejectSymlinkComponents(rootPath, relative string) error {
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return ErrUnavailable
	}
	defer root.Close()
	current := ""
	for _, component := range strings.Split(filepath.Clean(relative), string(filepath.Separator)) {
		if component == "." || component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, err := root.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return ErrUnavailable
		}
	}
	return nil
}

func withinNamespacePath(path string, namespace godiscovery.PathNamespace, config Config) bool {
	switch namespace {
	case godiscovery.NamespaceSource:
		return within(config.RepositoryRoot, path)
	case godiscovery.NamespaceDependency:
		return within(config.ModuleCacheDirectory, path)
	case godiscovery.NamespaceToolchain:
		return within(config.GOROOT, path)
	default:
		return false
	}
}

func environmentStrings(values []gorunner.EnvironmentVariable) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = value.Name + "=" + value.Value
	}
	return result
}

func discoveryEnvironmentDigest(values []gorunner.EnvironmentVariable) (string, error) {
	canonical := make([]any, len(values))
	previous := ""
	for index, value := range values {
		if value.Name == "" || value.Name <= previous {
			return "", ErrInvalid
		}
		previous = value.Name
		digest := sha256.Sum256([]byte(value.Value))
		canonical[index] = map[string]any{"name": value.Name, "valueSha256": hex.EncodeToString(digest[:])}
	}
	body, err := json.Marshal(canonical, json.Deterministic(true))
	if err != nil {
		return "", ErrUnavailable
	}
	return bareID("go-environment", "go-environment/0", body), nil
}
