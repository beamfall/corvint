package godiscovery

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"sort"
	"strconv"
	"strings"
)

const DiscoveryProfile = "go-live-discovery/0"

func Compose(manifest Manifest, input DocumentInput) (Document, []byte, error) {
	if err := validateManifest(manifest); err != nil {
		return Document{}, nil, err
	}
	patterns, err := packagePatterns(input.PackagePatterns)
	emptyStderr := sha256.Sum256(nil)
	if err != nil || !validDigest(input.EnvironmentSHA256) || input.RawStderrSHA256 != hex.EncodeToString(emptyStderr[:]) || !validToolchainID(input.ToolchainID) {
		return Document{}, nil, failure(CodeInput)
	}
	if strings.Join(patterns, "\x00") != strings.Join(matchUnion(manifest.Packages), "\x00") {
		return Document{}, nil, failure(CodeInput)
	}
	argv := []string{"@PINNED_GO@", "list", "-deps", "-test", "-json=" + ClosedFields}
	argv = append(argv, patterns...)
	document := Document{
		Argv: argv, DependencyMaterializationSHA256: manifest.DependencyMaterializationSHA256,
		EnvironmentSHA256: input.EnvironmentSHA256, ExitCode: "0", InputsSHA256: manifest.InputsSHA256,
		ModuleMode: manifest.ModuleMode, Packages: clonePackages(manifest.Packages), Profile: DiscoveryProfile,
		RawStderrSHA256: input.RawStderrSHA256, RawStdoutSHA256: manifest.RawStdoutSHA256,
		RequestedRunnerPackages: append([]string(nil), manifest.RequestedRunnerPackages...),
		SourceWSI:               manifest.SourceWSI, ToolchainID: input.ToolchainID,
	}
	body := documentValue(document, false)
	id, _, err := prefixedID("go-live-discovery", DiscoveryProfile, body)
	if err != nil {
		return Document{}, nil, failure(CodeInput)
	}
	document.ID = id
	encoded, err := canonicalJSON(documentValue(document, true))
	if err != nil {
		return Document{}, nil, failure(CodeInput)
	}
	encoded = append(encoded, '\n')
	return document, encoded, nil
}

// VerifyDocument rejects any structural, identity, or canonical-byte drift in
// a composed discovery document. External materialization verification remains
// the caller's responsibility; this verifies the closed discovery transcript.
func VerifyDocument(document Document, encoded []byte) error {
	if document.Profile != DiscoveryProfile || document.ExitCode != "0" || !validDiscoveryID(document.ID) {
		return failure(CodeInput)
	}
	if len(document.Argv) < 6 || document.Argv[0] != "@PINNED_GO@" || document.Argv[1] != "list" ||
		document.Argv[2] != "-deps" || document.Argv[3] != "-test" || document.Argv[4] != "-json="+ClosedFields {
		return failure(CodeInput)
	}
	patterns, err := packagePatterns(document.Argv[5:])
	if err != nil || strings.Join(patterns, "\x00") != strings.Join(document.Argv[5:], "\x00") {
		return failure(CodeInput)
	}
	if strings.Join(patterns, "\x00") != strings.Join(matchUnion(document.Packages), "\x00") {
		return failure(CodeInput)
	}
	emptyStderr := sha256.Sum256(nil)
	if !validDigest(document.EnvironmentSHA256) || document.RawStderrSHA256 != hex.EncodeToString(emptyStderr[:]) ||
		!validToolchainID(document.ToolchainID) {
		return failure(CodeInput)
	}
	manifest := Manifest{
		DependencyMaterializationSHA256: document.DependencyMaterializationSHA256,
		InputsSHA256:                    document.InputsSHA256, ModuleMode: document.ModuleMode,
		Packages: clonePackages(document.Packages), RawStdoutBytes: 1,
		RawStdoutSHA256:         document.RawStdoutSHA256,
		RequestedRunnerPackages: append([]string(nil), document.RequestedRunnerPackages...),
		SourceWSI:               document.SourceWSI,
	}
	manifest.seal, err = sealManifest(manifest)
	if err != nil || validateManifest(manifest) != nil {
		return failure(CodeInput)
	}
	wantID, _, err := prefixedID("go-live-discovery", DiscoveryProfile, documentValue(document, false))
	if err != nil || wantID != document.ID {
		return failure(CodeInput)
	}
	wantBytes, err := canonicalJSON(documentValue(document, true))
	if err != nil {
		return failure(CodeInput)
	}
	wantBytes = append(wantBytes, '\n')
	if !bytes.Equal(encoded, wantBytes) {
		return failure(CodeInput)
	}
	return nil
}

func documentValue(document Document, includeID bool) map[string]any {
	packages := make([]any, len(document.Packages))
	for index, pack := range document.Packages {
		packages[index] = packageOutput(pack)
	}
	value := map[string]any{
		"argv": document.Argv, "dependencyMaterializationSha256": document.DependencyMaterializationSHA256,
		"environmentSha256": document.EnvironmentSHA256, "exitCode": document.ExitCode,
		"inputsSha256": document.InputsSHA256, "moduleMode": string(document.ModuleMode),
		"packages": packages, "profile": document.Profile, "rawStderrSha256": document.RawStderrSHA256,
		"rawStdoutSha256": document.RawStdoutSHA256, "requestedRunnerPackages": document.RequestedRunnerPackages,
		"sourceWsi": document.SourceWSI, "toolchainId": document.ToolchainID,
	}
	if includeID {
		value["id"] = document.ID
	}
	return value
}

func validateManifest(manifest Manifest) error {
	seal, sealErr := sealManifest(manifest)
	if sealErr != nil || seal != manifest.seal {
		return failure(CodeInput)
	}
	if !validDigest(manifest.DependencyMaterializationSHA256) || !validDigest(manifest.RawStdoutSHA256) ||
		!validDigest(manifest.InputsSHA256) || !validModuleMode(manifest.ModuleMode) || !validSourceWSI(manifest.SourceWSI) ||
		manifest.RawStdoutBytes == 0 || manifest.RawStdoutBytes > MaxDiscoveryBytes ||
		len(manifest.Packages) == 0 || len(manifest.Packages) > MaxPackages {
		return failure(CodeInput)
	}
	requested := make([]string, 0)
	packageInputs := make([][]byte, 0, len(manifest.Packages))
	ids := make(map[string]struct{}, len(manifest.Packages))
	importPaths := make(map[string]struct{}, len(manifest.Packages))
	var previousPackage []byte
	for _, pack := range manifest.Packages {
		if !sortedUnique(pack.Match) || !sortedUnique(pack.Imports) || !sortedUnique(pack.TestImports) || !sortedUnique(pack.XTestImports) ||
			!validDigest(pack.DirPathSHA256) || !validDigest(pack.InputFilesSHA256) || pack.Name == "" || !validText(pack.Name) ||
			pack.ImportPath == "" || !validText(pack.ImportPath) {
			return failure(CodeInput)
		}
		if pack.ForTest != nil && (*pack.ForTest == "" || !validText(*pack.ForTest)) {
			return failure(CodeInput)
		}
		if _, duplicate := importPaths[pack.ImportPath]; duplicate {
			return failure(CodeInput)
		}
		importPaths[pack.ImportPath] = struct{}{}
		for _, match := range pack.Match {
			if match != "./..." && !validExactImportPath(match) {
				return failure(CodeInput)
			}
		}
		packageBytes, packageErr := canonicalJSON(packageOutput(pack))
		if packageErr != nil || (previousPackage != nil && string(previousPackage) >= string(packageBytes)) {
			return failure(CodeInput)
		}
		previousPackage = packageBytes
		rawForClass := rawPackage{depOnly: pack.DepOnly, forTest: pack.ForTest, importPath: pack.ImportPath, match: pack.Match, name: pack.Name}
		kind, kindErr := classify(rawForClass)
		if kindErr != nil || kind != pack.Kind {
			return failure(CodeInput)
		}
		if pack.Kind == Requested && !validExactImportPath(pack.ImportPath) {
			return failure(CodeInput)
		}
		if !validInputFiles(pack.InputFiles) || !validNormalizedModule(pack.Module) {
			return failure(CodeInput)
		}
		if pack.Kind == TestMain {
			if len(pack.InputFiles) != 1 {
				return failure(CodeInput)
			}
			for _, input := range pack.InputFiles {
				want, pathErr := bareDigest("go-generated-testmain", "go-generated-testmain/0", map[string]any{
					"importPath": pack.ImportPath, "mode": input.Mode,
					"rawSha256": input.RawSHA256, "role": "TEST_MAIN_GOFILE",
				})
				if pathErr != nil || want != input.PathSHA256 {
					return failure(CodeInput)
				}
			}
		}
		filesDigest, err := bareDigest("go-input-set", "go-input-set/0", inputFileValues(pack.InputFiles))
		if err != nil || filesDigest != pack.InputFilesSHA256 {
			return failure(CodeInput)
		}
		var moduleDigest *string
		if pack.Module != nil {
			digest, digestErr := bareDigest("go-module", "go-module/0", moduleValue(pack.Module))
			if digestErr != nil {
				return failure(CodeInput)
			}
			moduleDigest = &digest
		}
		if nullable(moduleDigest) != nullable(pack.ModuleSHA256) {
			if moduleDigest == nil || pack.ModuleSHA256 == nil || *moduleDigest != *pack.ModuleSHA256 {
				return failure(CodeInput)
			}
		}
		raw := rawPackage{
			depOnly: pack.DepOnly, forTest: pack.ForTest, importPath: pack.ImportPath, match: pack.Match,
			name: pack.Name, imports: pack.Imports, testImports: pack.TestImports, xTestImports: pack.XTestImports,
		}
		id, _, err := prefixedID("go-package", "go-package/0", packageBody(raw, pack.Kind, pack.DirPathSHA256, pack.InputFilesSHA256, pack.ModuleSHA256))
		if err != nil || id != pack.ID {
			return failure(CodeInput)
		}
		if _, duplicate := ids[id]; duplicate {
			return failure(CodeInput)
		}
		ids[id] = struct{}{}
		if pack.Kind == Requested {
			requested = append(requested, pack.ImportPath)
		}
		body := map[string]any{
			"depOnly": pack.DepOnly, "forTest": nullable(pack.ForTest), "id": pack.ID,
			"imports": pack.Imports, "match": pack.Match, "moduleSha256": nullable(pack.ModuleSHA256),
			"name": pack.Name, "testImports": pack.TestImports, "xTestImports": pack.XTestImports,
		}
		encoded, err := canonicalJSON(body)
		if err != nil {
			return failure(CodeInput)
		}
		packageInputs = append(packageInputs, encoded)
	}
	sort.Strings(requested)
	if len(requested) == 0 || !sortedUnique(manifest.RequestedRunnerPackages) || strings.Join(requested, "\x00") != strings.Join(manifest.RequestedRunnerPackages, "\x00") {
		return failure(CodeInput)
	}
	sort.Slice(packageInputs, func(left, right int) bool { return string(packageInputs[left]) < string(packageInputs[right]) })
	values := make([]any, len(packageInputs))
	for index, encoded := range packageInputs {
		var value map[string]any
		if err := json.Unmarshal(encoded, &value); err != nil {
			return failure(CodeInput)
		}
		values[index] = value
	}
	digest, err := bareDigest("go-discovery-inputs", "go-discovery-inputs/0", map[string]any{
		"dependencyMaterializationSha256": manifest.DependencyMaterializationSHA256,
		"moduleMode":                      string(manifest.ModuleMode), "packages": values, "sourceWsi": manifest.SourceWSI,
	})
	if err != nil || digest != manifest.InputsSHA256 {
		return failure(CodeInput)
	}
	return nil
}

func sealManifest(manifest Manifest) (string, error) {
	packages := make([]any, len(manifest.Packages))
	for index, pack := range manifest.Packages {
		packages[index] = packageOutput(pack)
	}
	body := map[string]any{
		"dependencyMaterializationSha256": manifest.DependencyMaterializationSHA256,
		"inputsSha256":                    manifest.InputsSHA256, "moduleMode": string(manifest.ModuleMode),
		"packages": packages, "rawStdoutBytes": strconv.FormatUint(manifest.RawStdoutBytes, 10),
		"rawStdoutSha256": manifest.RawStdoutSHA256, "requestedRunnerPackages": manifest.RequestedRunnerPackages,
		"sourceWsi": manifest.SourceWSI,
	}
	return bareDigest("go-discovery-manifest", "go-discovery-manifest/0", body)
}

func sortedUnique(values []string) bool {
	for index, value := range values {
		if value == "" || !validText(value) {
			return false
		}
		if index != 0 && values[index-1] >= value {
			return false
		}
	}
	return true
}

func validInputFiles(values []InputFile) bool {
	var previous []byte
	for _, value := range values {
		if !modeRE.MatchString(value.Mode) || !validDigest(value.PathSHA256) || !validDigest(value.RawSHA256) {
			return false
		}
		encoded, err := canonicalJSON(map[string]any{"mode": value.Mode, "pathSha256": value.PathSHA256, "rawSha256": value.RawSHA256})
		if err != nil || (previous != nil && string(previous) >= string(encoded)) {
			return false
		}
		previous = encoded
	}
	return true
}

func validNormalizedModule(module *Module) bool {
	if module == nil {
		return true
	}
	if module.Path == "" || !validText(module.Path) || !optionalDigest(module.DirPathSHA256) || !optionalDigest(module.GoModPathSHA256) {
		return false
	}
	for _, value := range []*string{module.GoModSum, module.GoVersion, module.Sum, module.TimeRaw, module.Version} {
		if value != nil && (*value == "" || !validText(*value)) {
			return false
		}
	}
	return validNormalizedModule(module.Replace) && (module.Replace == nil || module.Replace.Replace == nil)
}

func optionalDigest(value *string) bool { return value == nil || validDigest(*value) }

func packagePatterns(values []string) ([]string, error) {
	if len(values) == 1 && values[0] == "./..." {
		return []string{"./..."}, nil
	}
	if len(values) == 0 || len(values) > 4_091 {
		return nil, failure(CodeInput)
	}
	result := append([]string(nil), values...)
	argvBytes := 0
	for _, argument := range []string{"@PINNED_GO@", "list", "-deps", "-test", "-json=" + ClosedFields} {
		argvBytes += len(argument) + 1
	}
	for _, value := range result {
		if value == "./..." || strings.Contains(value, "@") || !validExactImportPath(value) {
			return nil, failure(CodeInput)
		}
		argvBytes += len(value) + 1
		if argvBytes > MaxArgvBytes {
			return nil, failure(CodeInput)
		}
	}
	sort.Strings(result)
	for index := 1; index < len(result); index++ {
		if result[index] == result[index-1] {
			return nil, failure(CodeInput)
		}
	}
	return result, nil
}

func matchUnion(packages []Package) []string {
	set := make(map[string]struct{})
	for _, pack := range packages {
		for _, match := range pack.Match {
			set[match] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for match := range set {
		result = append(result, match)
	}
	sort.Strings(result)
	return result
}

func validToolchainID(value string) bool {
	const prefix = "go-toolchain:sha256:"
	return strings.HasPrefix(value, prefix) && validDigest(strings.TrimPrefix(value, prefix))
}

func validDiscoveryID(value string) bool {
	const prefix = "go-live-discovery:sha256:"
	return strings.HasPrefix(value, prefix) && validDigest(strings.TrimPrefix(value, prefix))
}

func clonePackages(values []Package) []Package {
	result := make([]Package, len(values))
	for index, value := range values {
		value.ForTest = cloneString(value.ForTest)
		value.InputFiles = append([]InputFile(nil), value.InputFiles...)
		value.Imports = append([]string(nil), value.Imports...)
		value.Match = append([]string(nil), value.Match...)
		value.Module = cloneModule(value.Module)
		value.ModuleSHA256 = cloneString(value.ModuleSHA256)
		value.TestImports = append([]string(nil), value.TestImports...)
		value.XTestImports = append([]string(nil), value.XTestImports...)
		result[index] = value
	}
	return result
}
