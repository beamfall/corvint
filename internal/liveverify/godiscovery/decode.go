package godiscovery

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"io"
	pathpkg "path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type rawMembers map[string]jsontext.Value

type rawPackage struct {
	dir          string
	importPath   string
	name         string
	forTest      *string
	match        []string
	depOnly      bool
	module       *rawModule
	files        map[FileField][]string
	imports      []string
	testImports  []string
	xTestImports []string
}

type rawModule struct {
	path      *string
	version   *string
	timeRaw   *string
	main      bool
	indirect  bool
	dir       *string
	goMod     *string
	goVersion *string
	sum       *string
	goModSum  *string
	replace   *rawModule
}

var (
	digestRE = regexp.MustCompile(`^[0-9a-f]{64}$`)
	modeRE   = regexp.MustCompile(`^0[0-7]{3}$`)
)

var packageFields = stringSet(
	"Dir", "ImportPath", "Name", "ForTest", "Match", "DepOnly", "Module",
	"GoFiles", "CgoFiles", "CFiles", "CXXFiles", "MFiles", "HFiles", "FFiles", "SFiles",
	"SwigFiles", "SwigCXXFiles", "SysoFiles", "EmbedFiles", "TestGoFiles", "XTestGoFiles",
	"TestEmbedFiles", "XTestEmbedFiles", "Imports", "TestImports", "XTestImports", "Error", "DepsErrors",
)

var moduleFields = stringSet(
	"Path", "Version", "Replace", "Time", "Main", "Indirect", "Dir", "GoMod",
	"GoVersion", "Sum", "GoModSum",
)

var packageErrorFields = stringSet("ImportStack", "Pos", "Err")

var fileFields = []FileField{
	FieldGoFiles, FieldCgoFiles, FieldCFiles, FieldCXXFiles, FieldMFiles, FieldHFiles,
	FieldFFiles, FieldSFiles, FieldSwigFiles, FieldSwigCXXFiles, FieldSysoFiles,
	FieldEmbedFiles, FieldTestGoFiles, FieldXTestGoFiles, FieldTestEmbedFiles, FieldXTestEmbedFiles,
}

func Decode(reader io.Reader, commitments Commitments) (Manifest, error) {
	if reader == nil {
		return Manifest{}, failure(CodeDecode)
	}
	value := sha256.New()
	bounded := &countingReader{reader: io.LimitReader(reader, MaxDiscoveryBytes+1), digest: value}
	decoder := jsontext.NewDecoder(bounded)
	packages := make([]rawPackage, 0)
	for {
		raw, err := decoder.ReadValue()
		if errors.Is(err, io.EOF) {
			break
		}
		if bounded.bytes > MaxDiscoveryBytes {
			return Manifest{}, failure(CodeLimit)
		}
		if err != nil || len(raw) == 0 || raw[0] != '{' {
			return Manifest{}, failure(CodeDecode)
		}
		if exceedsDepth(raw, MaxJSONDepth) {
			return Manifest{}, failure(CodeLimit)
		}
		if hasOversizeJSONString(raw) {
			return Manifest{}, failure(CodeLimit)
		}
		if len(packages) == MaxPackages {
			return Manifest{}, failure(CodeLimit)
		}
		decoded, err := decodePackage(raw)
		if err != nil {
			return Manifest{}, err
		}
		packages = append(packages, decoded)
	}
	if bounded.bytes > MaxDiscoveryBytes {
		return Manifest{}, failure(CodeLimit)
	}
	if len(packages) == 0 {
		return Manifest{}, failure(CodeDecode)
	}
	manifest, err := bind(packages, commitments)
	if err != nil {
		return Manifest{}, err
	}
	manifest.RawStdoutBytes = bounded.bytes
	manifest.RawStdoutSHA256 = hex.EncodeToString(value.Sum(nil))
	manifest.seal, err = sealManifest(manifest)
	if err != nil {
		return Manifest{}, failure(CodeInput)
	}
	return manifest, nil
}

type countingReader struct {
	reader io.Reader
	digest io.Writer
	bytes  uint64
}

func (r *countingReader) Read(buffer []byte) (int, error) {
	count, err := r.reader.Read(buffer)
	if count != 0 {
		_, _ = r.digest.Write(buffer[:count])
		r.bytes += uint64(count)
	}
	return count, err
}

func decodePackage(raw []byte) (rawPackage, error) {
	members, err := object(raw, packageFields)
	if err != nil {
		return rawPackage{}, err
	}
	dir, present, err := stringMember(members, "Dir")
	if err != nil || !present || !validPOSIXAbsolute(dir, true) {
		return rawPackage{}, failure(CodeDecode)
	}
	importPath, present, err := stringMember(members, "ImportPath")
	if err != nil || !present || importPath == "" {
		return rawPackage{}, failure(CodeDecode)
	}
	name, present, err := stringMember(members, "Name")
	if err != nil || !present || name == "" {
		return rawPackage{}, failure(CodeDecode)
	}
	forTest, err := optionalString(members, "ForTest", false)
	if err != nil {
		return rawPackage{}, err
	}
	match, err := optionalStringArray(members, "Match")
	if err != nil {
		return rawPackage{}, err
	}
	depOnly, err := optionalBool(members, "DepOnly")
	if err != nil {
		return rawPackage{}, err
	}
	module, err := optionalModule(members, "Module")
	if err != nil {
		return rawPackage{}, err
	}
	files := make(map[FileField][]string, len(fileFields))
	for _, field := range fileFields {
		items, itemErr := optionalStringArray(members, string(field))
		if itemErr != nil {
			return rawPackage{}, itemErr
		}
		files[field] = items
	}
	imports, err := optionalStringArray(members, "Imports")
	if err != nil {
		return rawPackage{}, err
	}
	testImports, err := optionalStringArray(members, "TestImports")
	if err != nil {
		return rawPackage{}, err
	}
	xTestImports, err := optionalStringArray(members, "XTestImports")
	if err != nil {
		return rawPackage{}, err
	}
	if err := rejectBuildErrors(members); err != nil {
		return rawPackage{}, err
	}
	return rawPackage{
		dir: dir, importPath: importPath, name: name, forTest: forTest, match: match,
		depOnly: depOnly, module: module, files: files, imports: imports,
		testImports: testImports, xTestImports: xTestImports,
	}, nil
}

func object(raw []byte, allowed map[string]struct{}) (rawMembers, error) {
	if len(raw) < 2 || raw[0] != '{' || raw[len(raw)-1] != '}' {
		return nil, failure(CodeDecode)
	}
	var members rawMembers
	if err := json.Unmarshal(raw, &members); err != nil {
		return nil, failure(CodeDecode)
	}
	for name := range members {
		if _, ok := allowed[name]; !ok {
			return nil, failure(CodeDecode)
		}
	}
	return members, nil
}

func stringMember(members rawMembers, name string) (string, bool, error) {
	raw, present := members[name]
	if !present {
		return "", false, nil
	}
	if len(raw) < 2 || raw[0] != '"' {
		return "", true, failure(CodeDecode)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", true, failure(CodeDecode)
	}
	if !validText(value) {
		return "", true, failure(limitOrDecode(value))
	}
	return value, true, nil
}

func optionalString(members rawMembers, name string, emptyAllowed bool) (*string, error) {
	value, present, err := stringMember(members, name)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	if !emptyAllowed && value == "" {
		return nil, failure(CodeDecode)
	}
	return &value, nil
}

func optionalNullableString(members rawMembers, name string) (*string, error) {
	value, present, err := stringMember(members, name)
	if err != nil || !present || value == "" {
		return nil, err
	}
	return &value, nil
}

func optionalBool(members rawMembers, name string) (bool, error) {
	raw, present := members[name]
	if !present {
		return false, nil
	}
	switch string(raw) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, failure(CodeDecode)
	}
}

func optionalStringArray(members rawMembers, name string) ([]string, error) {
	raw, present := members[name]
	if !present {
		return []string{}, nil
	}
	if len(raw) < 2 || raw[0] != '[' {
		return nil, failure(CodeDecode)
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return nil, failure(CodeDecode)
	}
	for _, value := range values {
		if value == "" || !validText(value) {
			return nil, failure(limitOrDecode(value))
		}
	}
	sort.Strings(values)
	unique := values[:0]
	for _, value := range values {
		if len(unique) == 0 || unique[len(unique)-1] != value {
			unique = append(unique, value)
		}
	}
	return unique, nil
}

func optionalModule(members rawMembers, name string) (*rawModule, error) {
	raw, present := members[name]
	if !present || string(raw) == "null" {
		return nil, nil
	}
	return decodeModule(raw, true)
}

func decodeModule(raw []byte, allowReplace bool) (*rawModule, error) {
	members, err := object(raw, moduleFields)
	if err != nil {
		return nil, err
	}
	path, err := optionalString(members, "Path", false)
	if err != nil || path == nil {
		return nil, failure(CodeDecode)
	}
	version, err := optionalNullableString(members, "Version")
	if err != nil {
		return nil, err
	}
	timeRaw, err := optionalNullableString(members, "Time")
	if err != nil {
		return nil, err
	}
	dir, err := optionalString(members, "Dir", false)
	if err != nil {
		return nil, err
	}
	goMod, err := optionalString(members, "GoMod", false)
	if err != nil {
		return nil, err
	}
	goVersion, err := optionalNullableString(members, "GoVersion")
	if err != nil {
		return nil, err
	}
	sum, err := optionalNullableString(members, "Sum")
	if err != nil {
		return nil, err
	}
	goModSum, err := optionalNullableString(members, "GoModSum")
	if err != nil {
		return nil, err
	}
	main, err := optionalBool(members, "Main")
	if err != nil {
		return nil, err
	}
	indirect, err := optionalBool(members, "Indirect")
	if err != nil {
		return nil, err
	}
	var replace *rawModule
	if replacement, present := members["Replace"]; present {
		if !allowReplace || string(replacement) == "null" {
			if !allowReplace && string(replacement) != "null" {
				return nil, failure(CodeDecode)
			}
		} else {
			replace, err = decodeModule(replacement, false)
			if err != nil {
				return nil, err
			}
		}
	}
	return &rawModule{
		path: path, version: version, timeRaw: timeRaw, main: main, indirect: indirect,
		dir: dir, goMod: goMod, goVersion: goVersion, sum: sum, goModSum: goModSum,
		replace: replace,
	}, nil
}

func rejectBuildErrors(members rawMembers) error {
	if raw, present := members["Error"]; present && string(raw) != "null" {
		if _, err := decodePackageError(raw); err != nil {
			return err
		}
		return failure(CodeInput)
	}
	if raw, present := members["DepsErrors"]; present {
		if len(raw) < 2 || raw[0] != '[' {
			return failure(CodeDecode)
		}
		var values []jsontext.Value
		if err := json.Unmarshal(raw, &values); err != nil || values == nil {
			return failure(CodeDecode)
		}
		for _, value := range values {
			if _, err := decodePackageError(value); err != nil {
				return err
			}
		}
		if len(values) != 0 {
			return failure(CodeInput)
		}
	}
	return nil
}

func decodePackageError(raw []byte) (rawMembers, error) {
	members, err := object(raw, packageErrorFields)
	if err != nil {
		return nil, err
	}
	if _, err := optionalStringArray(members, "ImportStack"); err != nil {
		return nil, err
	}
	if _, err := optionalString(members, "Pos", true); err != nil {
		return nil, err
	}
	if _, err := optionalString(members, "Err", true); err != nil {
		return nil, err
	}
	return members, nil
}

func bind(rawPackages []rawPackage, commitments Commitments) (Manifest, error) {
	if !validDigest(commitments.DependencyMaterializationSHA256) || !validSourceWSI(commitments.SourceWSI) || !validModuleMode(commitments.ModuleMode) {
		return Manifest{}, failure(CodeInput)
	}
	// Validate caller-supplied collection cardinalities before allocating maps
	// from them. The package stream is already bounded by MaxPackages.
	if len(commitments.Packages) != len(rawPackages) || len(commitments.ModulePaths) > 4*len(rawPackages) {
		return Manifest{}, failure(CodeInput)
	}
	rawImportPaths := make(map[string]struct{}, len(rawPackages))
	for _, pack := range rawPackages {
		if _, duplicate := rawImportPaths[pack.importPath]; duplicate {
			return Manifest{}, failure(CodeDecode)
		}
		rawImportPaths[pack.importPath] = struct{}{}
	}
	materials, err := packageMaterials(commitments.Packages)
	if err != nil || len(materials) != len(rawPackages) {
		return Manifest{}, failure(CodeInput)
	}
	paths, err := pathCommitments(commitments.ModulePaths)
	if err != nil {
		return Manifest{}, err
	}
	usedPaths := make(map[string]struct{})
	packages := make([]Package, 0, len(rawPackages))
	packageInputs := make([][]byte, 0, len(rawPackages))
	ids := make(map[string]struct{}, len(rawPackages))
	requested := make(map[string]struct{})
	for _, rawPackage := range rawPackages {
		material, ok := materials[rawPackage.importPath]
		if !ok || material.RawDir != rawPackage.dir || !validLogicalCommitment(material.DirNamespace, material.DirLogicalPath, material.DirPathSHA256) {
			return Manifest{}, failure(CodeInput)
		}
		kind, err := classify(rawPackage)
		if err != nil {
			return Manifest{}, err
		}
		inputFiles, inputDigest, err := bindFiles(rawPackage, kind, material)
		if err != nil {
			return Manifest{}, err
		}
		module, moduleDigest, err := bindModule(rawPackage.module, paths, usedPaths)
		if err != nil {
			return Manifest{}, err
		}
		body := packageBody(rawPackage, kind, material.DirPathSHA256, inputDigest, moduleDigest)
		id, _, err := prefixedID("go-package", "go-package/0", body)
		if err != nil {
			return Manifest{}, failure(CodeInput)
		}
		if _, duplicate := ids[id]; duplicate {
			return Manifest{}, failure(CodeDecode)
		}
		ids[id] = struct{}{}
		pack := Package{
			DepOnly: rawPackage.depOnly, DirPathSHA256: material.DirPathSHA256,
			ForTest: cloneString(rawPackage.forTest), ID: id, ImportPath: rawPackage.importPath,
			InputFiles: inputFiles, InputFilesSHA256: inputDigest, Imports: append([]string(nil), rawPackage.imports...),
			Kind: kind, Match: append([]string(nil), rawPackage.match...), Module: cloneModule(module),
			ModuleSHA256: cloneString(moduleDigest), Name: rawPackage.name,
			TestImports: append([]string(nil), rawPackage.testImports...), XTestImports: append([]string(nil), rawPackage.xTestImports...),
		}
		packages = append(packages, pack)
		inputBody := map[string]any{
			"depOnly": rawPackage.depOnly, "forTest": nullable(rawPackage.forTest), "id": id,
			"imports": rawPackage.imports, "match": rawPackage.match, "moduleSha256": nullable(moduleDigest),
			"name": rawPackage.name, "testImports": rawPackage.testImports, "xTestImports": rawPackage.xTestImports,
		}
		canonical, err := canonicalJSON(inputBody)
		if err != nil {
			return Manifest{}, failure(CodeInput)
		}
		packageInputs = append(packageInputs, canonical)
		if kind == Requested {
			if !validExactImportPath(rawPackage.importPath) {
				return Manifest{}, failure(CodeDecode)
			}
			requested[rawPackage.importPath] = struct{}{}
		}
	}
	if len(usedPaths) != len(paths) {
		return Manifest{}, failure(CodeInput)
	}
	sort.Slice(packages, func(left, right int) bool {
		leftBytes, _ := canonicalJSON(packageOutput(packages[left]))
		rightBytes, _ := canonicalJSON(packageOutput(packages[right]))
		return string(leftBytes) < string(rightBytes)
	})
	sort.Slice(packageInputs, func(left, right int) bool { return string(packageInputs[left]) < string(packageInputs[right]) })
	inputValues := make([]any, len(packageInputs))
	for index, encoded := range packageInputs {
		var value map[string]any
		if err := json.Unmarshal(encoded, &value); err != nil {
			return Manifest{}, failure(CodeInput)
		}
		inputValues[index] = value
	}
	inputsDigest, err := bareDigest("go-discovery-inputs", "go-discovery-inputs/0", map[string]any{
		"dependencyMaterializationSha256": commitments.DependencyMaterializationSHA256,
		"moduleMode":                      string(commitments.ModuleMode), "packages": inputValues, "sourceWsi": commitments.SourceWSI,
	})
	if err != nil {
		return Manifest{}, failure(CodeInput)
	}
	requestedPackages := make([]string, 0, len(requested))
	for importPath := range requested {
		requestedPackages = append(requestedPackages, importPath)
	}
	sort.Strings(requestedPackages)
	return Manifest{
		DependencyMaterializationSHA256: commitments.DependencyMaterializationSHA256,
		InputsSHA256:                    inputsDigest, ModuleMode: commitments.ModuleMode,
		Packages: packages, RequestedRunnerPackages: requestedPackages, SourceWSI: commitments.SourceWSI,
	}, nil
}

func bindFiles(rawPackage rawPackage, kind PackageKind, material PackageMaterial) ([]InputFile, string, error) {
	commitments := material.Files
	if kind == TestMain {
		if len(rawPackage.files[FieldGoFiles]) != 1 || len(commitments) != 1 {
			return nil, "", failure(CodeInput)
		}
		for _, field := range fileFields {
			if field != FieldGoFiles && len(rawPackage.files[field]) != 0 {
				return nil, "", failure(CodeInput)
			}
		}
	}
	expected := make(map[string]struct{})
	for _, field := range fileFields {
		for _, name := range rawPackage.files[field] {
			expected[string(field)+"\x00"+name] = struct{}{}
		}
	}
	if len(expected) != len(commitments) {
		return nil, "", failure(CodeInput)
	}
	values := make([]any, 0, len(commitments))
	seen := make(map[string]struct{}, len(commitments))
	pathInputs := make(map[string]InputFile, len(commitments))
	for _, commitment := range commitments {
		key := string(commitment.Field) + "\x00" + commitment.ListedName
		if _, ok := expected[key]; !ok {
			return nil, "", failure(CodeInput)
		}
		if _, duplicate := seen[key]; duplicate {
			return nil, "", failure(CodeInput)
		}
		seen[key] = struct{}{}
		if !validDigest(commitment.PathSHA256) || !validDigest(commitment.RawSHA256) || !modeRE.MatchString(commitment.Mode) {
			return nil, "", failure(CodeInput)
		}
		if kind == TestMain && commitment.Field == FieldGoFiles {
			if commitment.Role != RoleGeneratedTestMain || !validPOSIXAbsolute(commitment.ListedName, false) {
				return nil, "", failure(CodeInput)
			}
			generated, err := bareDigest("go-generated-testmain", "go-generated-testmain/0", map[string]any{
				"importPath": rawPackage.importPath, "mode": commitment.Mode,
				"rawSha256": commitment.RawSHA256, "role": "TEST_MAIN_GOFILE",
			})
			if err != nil || generated != commitment.PathSHA256 || commitment.Namespace != "" || commitment.LogicalPath != "" {
				return nil, "", failure(CodeInput)
			}
		} else {
			expectedPath, pathErr := joinedLogicalPath(material.DirLogicalPath, commitment.ListedName)
			if pathErr != nil || commitment.Role != RoleSource || commitment.Namespace != material.DirNamespace ||
				commitment.LogicalPath != expectedPath || !validLogicalCommitment(commitment.Namespace, commitment.LogicalPath, commitment.PathSHA256) {
				return nil, "", failure(CodeInput)
			}
		}
		input := InputFile{Mode: commitment.Mode, PathSHA256: commitment.PathSHA256, RawSHA256: commitment.RawSHA256}
		if previous, exists := pathInputs[commitment.PathSHA256]; exists && (previous.Mode != input.Mode || previous.RawSHA256 != input.RawSHA256) {
			return nil, "", failure(CodeInput)
		}
		pathInputs[commitment.PathSHA256] = input
		values = append(values, map[string]any{
			"mode": commitment.Mode, "pathSha256": commitment.PathSHA256, "rawSha256": commitment.RawSHA256,
		})
	}
	sort.Slice(values, func(left, right int) bool {
		leftBytes, _ := canonicalJSON(values[left])
		rightBytes, _ := canonicalJSON(values[right])
		return string(leftBytes) < string(rightBytes)
	})
	unique := values[:0]
	for _, value := range values {
		if len(unique) != 0 {
			left, _ := canonicalJSON(unique[len(unique)-1])
			right, _ := canonicalJSON(value)
			if string(left) == string(right) {
				continue
			}
		}
		unique = append(unique, value)
	}
	digest, err := bareDigest("go-input-set", "go-input-set/0", unique)
	if err != nil {
		return nil, "", failure(CodeInput)
	}
	inputs := make([]InputFile, len(unique))
	for index, value := range unique {
		object := value.(map[string]any)
		inputs[index] = InputFile{Mode: object["mode"].(string), PathSHA256: object["pathSha256"].(string), RawSHA256: object["rawSha256"].(string)}
	}
	return inputs, digest, nil
}

func bindModule(module *rawModule, paths map[string]string, used map[string]struct{}) (*Module, *string, error) {
	if module == nil {
		return nil, nil, nil
	}
	body, err := moduleBody(module, paths, used)
	if err != nil {
		return nil, nil, err
	}
	digest, err := bareDigest("go-module", "go-module/0", body)
	if err != nil {
		return nil, nil, failure(CodeInput)
	}
	normalized := moduleFromBody(body)
	return normalized, &digest, nil
}

func moduleBody(module *rawModule, paths map[string]string, used map[string]struct{}) (map[string]any, error) {
	dirDigest, err := committedPath(module.dir, paths, used, true)
	if err != nil {
		return nil, err
	}
	goModDigest, err := committedPath(module.goMod, paths, used, false)
	if err != nil {
		return nil, err
	}
	var replace any
	if module.replace != nil {
		replace, err = moduleBody(module.replace, paths, used)
		if err != nil {
			return nil, err
		}
	}
	return map[string]any{
		"dirPathSha256": nullable(dirDigest), "goModPathSha256": nullable(goModDigest),
		"goModSum": nullable(module.goModSum), "goVersion": nullable(module.goVersion),
		"indirect": module.indirect, "main": module.main, "path": nullable(module.path),
		"replace": replace, "sum": nullable(module.sum), "timeRaw": nullable(module.timeRaw),
		"version": nullable(module.version),
	}, nil
}

func committedPath(raw *string, paths map[string]string, used map[string]struct{}, allowRoot bool) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	if !validPOSIXAbsolute(*raw, allowRoot) {
		return nil, failure(CodeInput)
	}
	digest, ok := paths[*raw]
	if !ok {
		return nil, failure(CodeInput)
	}
	used[*raw] = struct{}{}
	return &digest, nil
}

func packageBody(rawPackage rawPackage, kind PackageKind, dirDigest, inputDigest string, moduleDigest *string) map[string]any {
	return map[string]any{
		"depOnly": rawPackage.depOnly, "dirPathSha256": dirDigest, "forTest": nullable(rawPackage.forTest),
		"importPath": rawPackage.importPath, "inputFilesSha256": inputDigest, "kind": string(kind),
		"match": rawPackage.match, "moduleSha256": nullable(moduleDigest), "name": rawPackage.name,
	}
}

func packageOutput(pack Package) map[string]any {
	return map[string]any{
		"depOnly": pack.DepOnly, "dirPathSha256": pack.DirPathSHA256, "forTest": nullable(pack.ForTest),
		"id": pack.ID, "importPath": pack.ImportPath, "inputFiles": inputFileValues(pack.InputFiles),
		"inputFilesSha256": pack.InputFilesSHA256, "imports": pack.Imports, "kind": string(pack.Kind),
		"match": pack.Match, "module": moduleValue(pack.Module), "moduleSha256": nullable(pack.ModuleSHA256),
		"name": pack.Name, "testImports": pack.TestImports, "xTestImports": pack.XTestImports,
	}
}

func classify(pack rawPackage) (PackageKind, error) {
	if pack.forTest != nil {
		return TestVariant, nil
	}
	if len(pack.match) != 0 && !pack.depOnly {
		return Requested, nil
	}
	if len(pack.match) == 0 && !pack.depOnly && pack.name == "main" {
		return TestMain, nil
	}
	if pack.depOnly {
		return Dependency, nil
	}
	return "", failure(CodeDecode)
}

func pathCommitments(values []PathCommitment) (map[string]string, error) {
	result := make(map[string]string, len(values))
	for _, value := range values {
		if !validPOSIXAbsolute(value.RawPath, true) ||
			(value.Namespace != NamespaceSource && value.Namespace != NamespaceDependency) ||
			!validLogicalCommitment(value.Namespace, value.LogicalPath, value.PathSHA256) {
			return nil, failure(CodeInput)
		}
		if _, duplicate := result[value.RawPath]; duplicate {
			return nil, failure(CodeInput)
		}
		result[value.RawPath] = value.PathSHA256
	}
	return result, nil
}

func packageMaterials(values []PackageMaterial) (map[string]PackageMaterial, error) {
	result := make(map[string]PackageMaterial, len(values))
	for _, value := range values {
		if value.ImportPath == "" || !validText(value.ImportPath) {
			return nil, failure(CodeInput)
		}
		if _, duplicate := result[value.ImportPath]; duplicate {
			return nil, failure(CodeInput)
		}
		result[value.ImportPath] = value
	}
	return result, nil
}

func validLogicalCommitment(namespace PathNamespace, path, want string) bool {
	if namespace != NamespaceSource && namespace != NamespaceDependency && namespace != NamespaceToolchain {
		return false
	}
	if !validLogicalPath(path) || !validDigest(want) {
		return false
	}
	got, err := bareDigest("go-logical-path", "go-logical-path/0", map[string]any{
		"namespace": string(namespace), "path": path,
	})
	return err == nil && got == want
}

func validLogicalPath(path string) bool {
	if path == "." {
		return true
	}
	if path == "" || strings.HasPrefix(path, "/") || strings.Contains(path, `\`) || !validText(path) {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
		for _, character := range segment {
			if unicode.IsControl(character) {
				return false
			}
		}
	}
	return true
}

func joinedLogicalPath(directory, listed string) (string, error) {
	if !validRelativePath(listed) {
		return "", failure(CodeInput)
	}
	if directory == "." {
		return listed, nil
	}
	result := directory + "/" + listed
	if !validLogicalPath(result) {
		return "", failure(CodeInput)
	}
	return result, nil
}

func validRelativePath(path string) bool {
	if path == "" || looksAbsolute(path) || strings.Contains(path, `\`) || !validText(path) {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
		for _, character := range segment {
			if unicode.IsControl(character) {
				return false
			}
		}
	}
	return true
}

func inputFileValues(values []InputFile) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = map[string]any{"mode": value.Mode, "pathSha256": value.PathSHA256, "rawSha256": value.RawSHA256}
	}
	return result
}

func moduleValue(module *Module) any {
	if module == nil {
		return nil
	}
	return map[string]any{
		"dirPathSha256": nullable(module.DirPathSHA256), "goModPathSha256": nullable(module.GoModPathSHA256),
		"goModSum": nullable(module.GoModSum), "goVersion": nullable(module.GoVersion),
		"indirect": module.Indirect, "main": module.Main, "path": module.Path,
		"replace": moduleValue(module.Replace), "sum": nullable(module.Sum), "timeRaw": nullable(module.TimeRaw),
		"version": nullable(module.Version),
	}
}

func moduleFromBody(body map[string]any) *Module {
	return &Module{
		DirPathSHA256: stringPointer(body["dirPathSha256"]), GoModPathSHA256: stringPointer(body["goModPathSha256"]),
		GoModSum: stringPointer(body["goModSum"]), GoVersion: stringPointer(body["goVersion"]),
		Indirect: body["indirect"].(bool), Main: body["main"].(bool), Path: body["path"].(string),
		Replace: modulePointer(body["replace"]), Sum: stringPointer(body["sum"]),
		TimeRaw: stringPointer(body["timeRaw"]), Version: stringPointer(body["version"]),
	}
}

func modulePointer(value any) *Module {
	if value == nil {
		return nil
	}
	return moduleFromBody(value.(map[string]any))
}

func stringPointer(value any) *string {
	if value == nil {
		return nil
	}
	text := value.(string)
	return &text
}

func cloneModule(value *Module) *Module {
	if value == nil {
		return nil
	}
	return &Module{
		DirPathSHA256: cloneString(value.DirPathSHA256), GoModPathSHA256: cloneString(value.GoModPathSHA256),
		GoModSum: cloneString(value.GoModSum), GoVersion: cloneString(value.GoVersion),
		Indirect: value.Indirect, Main: value.Main, Path: value.Path, Replace: cloneModule(value.Replace),
		Sum: cloneString(value.Sum), TimeRaw: cloneString(value.TimeRaw), Version: cloneString(value.Version),
	}
}

func validDigest(value string) bool { return digestRE.MatchString(value) }

func validSourceWSI(value string) bool {
	const prefix = "workspace-source:sha256:"
	return strings.HasPrefix(value, prefix) && validDigest(strings.TrimPrefix(value, prefix))
}

func validModuleMode(value ModuleMode) bool {
	return value == ModuleModeModule || value == ModuleModeWorkspace || value == ModuleModeVendor
}

func validExactImportPath(value string) bool {
	if value == "" || len(value) > 4_096 || strings.HasPrefix(value, "-") || strings.ContainsAny(value, "@,\\*?[] \t\r\n") {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." || segment == "..." {
			return false
		}
		for _, character := range segment {
			if unicode.IsControl(character) || unicode.IsSpace(character) {
				return false
			}
		}
	}
	return validText(value)
}

func looksAbsolute(value string) bool {
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, `\`) {
		return true
	}
	return len(value) >= 3 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':' && (value[2] == '/' || value[2] == '\\')
}

func validPOSIXAbsolute(value string, allowRoot bool) bool {
	if (!allowRoot && value == "/") || !strings.HasPrefix(value, "/") || strings.Contains(value, `\`) || !validText(value) || pathpkg.Clean(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func nullable(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func stringSet(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func failure(code Code) error { return &Error{Code: code} }

func limitOrDecode(value string) Code {
	if len(value) > MaxStringBytes {
		return CodeLimit
	}
	return CodeDecode
}

func exceedsDepth(raw []byte, maximum int) bool {
	depth := 0
	inString := false
	escaped := false
	for _, value := range raw {
		if inString {
			if escaped {
				escaped = false
			} else if value == '\\' {
				escaped = true
			} else if value == '"' {
				inString = false
			}
			continue
		}
		switch value {
		case '"':
			inString = true
		case '{', '[':
			depth++
			if depth > maximum {
				return true
			}
		case '}', ']':
			depth--
		}
	}
	return false
}

func hasOversizeJSONString(raw []byte) bool {
	for index := 0; index < len(raw); index++ {
		if raw[index] != '"' {
			continue
		}
		decodedBytes := 0
		for index++; index < len(raw) && raw[index] != '"'; index++ {
			if raw[index] != '\\' {
				decodedBytes++
			} else if index+1 < len(raw) {
				index++
				if raw[index] != 'u' || index+4 >= len(raw) {
					decodedBytes++
				} else {
					value, err := strconv.ParseUint(string(raw[index+1:index+5]), 16, 16)
					if err != nil {
						decodedBytes++
					} else {
						character := rune(value)
						index += 4
						if character >= 0xd800 && character <= 0xdbff && index+6 < len(raw) && raw[index+1] == '\\' && raw[index+2] == 'u' {
							low, lowErr := strconv.ParseUint(string(raw[index+3:index+7]), 16, 16)
							if lowErr == nil && low >= 0xdc00 && low <= 0xdfff {
								character = 0x10000 + (character-0xd800)<<10 + rune(low-0xdc00)
								index += 6
							}
						}
						width := utf8.RuneLen(character)
						if width < 1 {
							width = 1
						}
						decodedBytes += width
					}
				}
			}
			if decodedBytes > MaxStringBytes {
				return true
			}
		}
	}
	return false
}
