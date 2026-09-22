// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

// Command go-live-test-v0 independently verifies the discovery portion of the
// experimental Corvint Go live-test protocol. It deliberately imports no Corvint
// implementation package.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxDocumentBytes = 16 << 20
	maxDepth         = 16
	maxPackages      = 4096
	maxArrayEntries  = 100_000
	maxStringBytes   = 1 << 20
	maxPatternBytes  = 4096
	maxArgvBytes     = 4 << 20
	discoveryProfile = "go-live-discovery/0"
	closedListFields = "Dir,ImportPath,Name,ForTest,Match,DepOnly,Module,GoFiles,CgoFiles,CFiles,CXXFiles,MFiles,HFiles,FFiles,SFiles,SwigFiles,SwigCXXFiles,SysoFiles,EmbedFiles,TestGoFiles,XTestGoFiles,TestEmbedFiles,XTestEmbedFiles,Imports,TestImports,XTestImports,Error,DepsErrors"
)

type rejectCode string

const (
	canonicalDuplicateKey rejectCode = "CANONICAL_DUPLICATE_KEY"
	canonicalInvalidUTF8  rejectCode = "CANONICAL_INVALID_UTF8"
	canonicalNoncanonical rejectCode = "CANONICAL_NONCANONICAL"
	canonicalUnknownField rejectCode = "CANONICAL_UNKNOWN_FIELD"
	discoveryDecode       rejectCode = "DISCOVERY_DECODE"
	discoveryInput        rejectCode = "DISCOVERY_INPUT"
	identityMismatch      rejectCode = "IDENTITY_MISMATCH"
	limitExceeded         rejectCode = "LIMIT_EXCEEDED"
)

type rejection struct {
	code rejectCode
}

func (e rejection) Error() string { return string(e.code) }

func reject(code rejectCode) error { return rejection{code: code} }

func codeOf(err error) rejectCode {
	var target rejection
	if errors.As(err, &target) {
		return target.code
	}
	return discoveryDecode
}

type packageDocument struct {
	DepOnly          bool
	DirPathSHA256    string
	ForTest          *string
	ID               string
	ImportPath       string
	InputFiles       []inputEntry
	InputFilesSHA256 string
	Imports          []string
	Kind             string
	Match            []string
	Module           *moduleBody
	ModuleSHA256     *string
	Name             string
	TestImports      []string
	XTestImports     []string
	raw              map[string]any
}

type discoveryDocument struct {
	Argv                            []string
	DependencyMaterializationSHA256 string
	EnvironmentSHA256               string
	ExitCode                        string
	ID                              string
	InputsSHA256                    string
	ModuleMode                      string
	Packages                        []packageDocument
	Profile                         string
	RawStderrSHA256                 string
	RawStdoutSHA256                 string
	RequestedRunnerPackages         []string
	SourceWSI                       string
	ToolchainID                     string
	raw                             map[string]any
}

type moduleBody struct {
	DirPathSHA256   *string
	GoModPathSHA256 *string
	GoModSum        *string
	GoVersion       *string
	Indirect        bool
	Main            bool
	Path            string
	Replace         *moduleBody
	Sum             *string
	TimeRaw         *string
	Version         *string
}

type inputEntry struct {
	Mode       string
	PathSHA256 string
	RawSHA256  string
}

type packageInput struct {
	DepOnly      bool
	ForTest      *string
	ID           string
	Imports      []string
	Match        []string
	ModuleSHA256 *string
	Name         string
	TestImports  []string
	XTestImports []string
}

type discoveryInputs struct {
	DependencyMaterializationSHA256 string
	ModuleMode                      string
	Packages                        []packageInput
	SourceWSI                       string
}

func parseCanonical(raw []byte) (map[string]any, error) {
	if len(raw) == 0 || len(raw) > maxDocumentBytes {
		return nil, reject(limitExceeded)
	}
	if !utf8.Valid(raw) {
		return nil, reject(canonicalInvalidUTF8)
	}
	if !bytes.HasSuffix(raw, []byte{'\n'}) || bytes.Count(raw[len(raw)-1:], []byte{'\n'}) != 1 {
		return nil, reject(canonicalNoncanonical)
	}
	if duplicateKeys(raw[:len(raw)-1]) {
		return nil, reject(canonicalDuplicateKey)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw[:len(raw)-1]))
	decoder.UseNumber()
	value, err := decodeValue(decoder, 1)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, reject(canonicalNoncanonical)
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, reject(canonicalNoncanonical)
	}
	encoded, err := canonical(value)
	if err != nil || !bytes.Equal(raw, append(encoded, '\n')) {
		return nil, reject(canonicalNoncanonical)
	}
	return object, nil
}

func duplicateKeys(raw []byte) bool {
	scanner := duplicateScanner{raw: raw}
	_, duplicate := scanner.value()
	return duplicate
}

type duplicateScanner struct {
	raw []byte
	pos int
}

func (s *duplicateScanner) value() (bool, bool) {
	if s.pos >= len(s.raw) {
		return false, false
	}
	switch s.raw[s.pos] {
	case '{':
		s.pos++
		keys := map[string]struct{}{}
		if s.take('}') {
			return true, false
		}
		for {
			key, ok := s.stringToken()
			if !ok || !s.take(':') {
				return false, false
			}
			if _, exists := keys[key]; exists {
				return true, true
			}
			keys[key] = struct{}{}
			ok, duplicate := s.value()
			if !ok || duplicate {
				return ok, duplicate
			}
			if s.take('}') {
				return true, false
			}
			if !s.take(',') {
				return false, false
			}
		}
	case '[':
		s.pos++
		if s.take(']') {
			return true, false
		}
		for {
			ok, duplicate := s.value()
			if !ok || duplicate {
				return ok, duplicate
			}
			if s.take(']') {
				return true, false
			}
			if !s.take(',') {
				return false, false
			}
		}
	case '"':
		_, ok := s.stringToken()
		return ok, false
	default:
		for _, literal := range []string{"true", "false", "null"} {
			if strings.HasPrefix(string(s.raw[s.pos:]), literal) {
				s.pos += len(literal)
				return true, false
			}
		}
		return false, false
	}
}

func (s *duplicateScanner) stringToken() (string, bool) {
	if !s.take('"') {
		return "", false
	}
	start := s.pos
	for s.pos < len(s.raw) {
		switch s.raw[s.pos] {
		case '"':
			result := string(s.raw[start:s.pos])
			s.pos++
			return result, true
		case '\\':
			s.pos++
			if s.pos >= len(s.raw) {
				return "", false
			}
			if s.raw[s.pos] == 'u' {
				s.pos += 5
			} else {
				s.pos++
			}
		default:
			s.pos++
		}
	}
	return "", false
}

func (s *duplicateScanner) take(want byte) bool {
	if s.pos >= len(s.raw) || s.raw[s.pos] != want {
		return false
	}
	s.pos++
	return true
}

func decodeValue(decoder *json.Decoder, depth int) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, decoderRejection(err)
	}
	if number, ok := token.(json.Number); ok {
		_ = number
		return nil, reject(canonicalNoncanonical)
	}
	if text, ok := token.(string); ok {
		if len(text) > maxStringBytes || !validText(text) {
			return nil, reject(discoveryInput)
		}
		return text, nil
	}
	delim, composite := token.(json.Delim)
	if !composite {
		return token, nil
	}
	if depth > maxDepth {
		return nil, reject(limitExceeded)
	}
	switch delim {
	case '{':
		object := map[string]any{}
		for decoder.More() {
			if len(object) >= maxArrayEntries {
				return nil, reject(limitExceeded)
			}
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, decoderRejection(err)
			}
			key, ok := keyToken.(string)
			if !ok || !validText(key) {
				return nil, reject(canonicalNoncanonical)
			}
			if _, exists := object[key]; exists {
				return nil, reject(canonicalDuplicateKey)
			}
			value, err := decodeValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		if token, err := decoder.Token(); err != nil {
			return nil, decoderRejection(err)
		} else if token != json.Delim('}') {
			return nil, reject(canonicalNoncanonical)
		}
		return object, nil
	case '[':
		array := make([]any, 0)
		for decoder.More() {
			if len(array) >= maxArrayEntries {
				return nil, reject(limitExceeded)
			}
			value, err := decodeValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		if token, err := decoder.Token(); err != nil {
			return nil, decoderRejection(err)
		} else if token != json.Delim(']') {
			return nil, reject(canonicalNoncanonical)
		}
		return array, nil
	default:
		return nil, reject(canonicalNoncanonical)
	}
}

func decoderRejection(err error) error {
	if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
		return reject(canonicalDuplicateKey)
	}
	return reject(canonicalNoncanonical)
}

func validText(value string) bool {
	for _, r := range value {
		if r >= 0xfdd0 && r <= 0xfdef || r&0xffff == 0xfffe || r&0xffff == 0xffff {
			return false
		}
	}
	return true
}

func canonical(value any) ([]byte, error) {
	var output bytes.Buffer
	if err := appendCanonical(&output, value); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func appendCanonical(output *bytes.Buffer, value any) error {
	switch value := value.(type) {
	case nil:
		output.WriteString("null")
	case bool:
		if value {
			output.WriteString("true")
		} else {
			output.WriteString("false")
		}
	case string:
		appendString(output, value)
	case []any:
		output.WriteByte('[')
		for index, child := range value {
			if index > 0 {
				output.WriteByte(',')
			}
			if err := appendCanonical(output, child); err != nil {
				return err
			}
		}
		output.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		output.WriteByte('{')
		for index, key := range keys {
			if index > 0 {
				output.WriteByte(',')
			}
			appendString(output, key)
			output.WriteByte(':')
			if err := appendCanonical(output, value[key]); err != nil {
				return err
			}
		}
		output.WriteByte('}')
	default:
		return reject(canonicalNoncanonical)
	}
	return nil
}

func appendString(output *bytes.Buffer, value string) {
	output.WriteByte('"')
	for _, r := range value {
		switch r {
		case '"', '\\':
			output.WriteByte('\\')
			output.WriteRune(r)
		case '\b':
			output.WriteString(`\b`)
		case '\t':
			output.WriteString(`\t`)
		case '\n':
			output.WriteString(`\n`)
		case '\f':
			output.WriteString(`\f`)
		case '\r':
			output.WriteString(`\r`)
		default:
			if r < 0x20 {
				fmt.Fprintf(output, `\u%04x`, r)
			} else {
				output.WriteRune(r)
			}
		}
	}
	output.WriteByte('"')
}

func exactKeys(object map[string]any, keys ...string) error {
	allowed := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		allowed[key] = struct{}{}
	}
	for key := range object {
		if _, ok := allowed[key]; !ok {
			return reject(canonicalUnknownField)
		}
	}
	if len(object) != len(keys) {
		return reject(discoveryDecode)
	}
	return nil
}

func stringField(object map[string]any, key string) (string, error) {
	value, ok := object[key].(string)
	if !ok || value == "" {
		return "", reject(discoveryDecode)
	}
	return value, nil
}

func nullableString(object map[string]any, key string) (*string, error) {
	value, exists := object[key]
	if !exists {
		return nil, reject(discoveryDecode)
	}
	if value == nil {
		return nil, nil
	}
	text, ok := value.(string)
	if !ok || text == "" {
		return nil, reject(discoveryDecode)
	}
	return &text, nil
}

func stringsField(object map[string]any, key string, ordered bool) ([]string, error) {
	raw, ok := object[key].([]any)
	if !ok {
		return nil, reject(discoveryDecode)
	}
	result := make([]string, len(raw))
	for index, item := range raw {
		text, ok := item.(string)
		if !ok || text == "" {
			return nil, reject(discoveryDecode)
		}
		result[index] = text
	}
	if !ordered && !sortedUnique(result) {
		return nil, reject(discoveryInput)
	}
	return result, nil
}

func sortedUnique(values []string) bool {
	for index := 1; index < len(values); index++ {
		if values[index-1] >= values[index] {
			return false
		}
	}
	return true
}

func isDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && value == strings.ToLower(value)
}

func isID(value, kind string) bool {
	prefix := kind + ":sha256:"
	return strings.HasPrefix(value, prefix) && isDigest(strings.TrimPrefix(value, prefix))
}

func domainID(kind, profile string, body []byte) string {
	hasher := sha256.New()
	var size4 [4]byte
	var size8 [8]byte
	binary.BigEndian.PutUint32(size4[:], uint32(len(kind)))
	hasher.Write(size4[:])
	hasher.Write([]byte(kind))
	binary.BigEndian.PutUint32(size4[:], uint32(len(profile)))
	hasher.Write(size4[:])
	hasher.Write([]byte(profile))
	binary.BigEndian.PutUint64(size8[:], uint64(len(body)))
	hasher.Write(size8[:])
	hasher.Write(body)
	return kind + ":sha256:" + hex.EncodeToString(hasher.Sum(nil))
}

func bareDomainDigest(kind, profile string, body []byte) string {
	return strings.TrimPrefix(domainID(kind, profile, body), kind+":sha256:")
}

func without(object map[string]any, key string) map[string]any {
	result := make(map[string]any, len(object)-1)
	for candidate, value := range object {
		if candidate != key {
			result[candidate] = value
		}
	}
	return result
}

func VerifyDiscovery(raw []byte) (*discoveryDocument, error) {
	if len(raw) > 0 && raw[len(raw)-1] == '\n' && duplicateKeys(raw[:len(raw)-1]) {
		return nil, reject(canonicalDuplicateKey)
	}
	object, err := parseCanonical(raw)
	if err != nil {
		return nil, err
	}
	if err := exactKeys(object, "argv", "dependencyMaterializationSha256", "environmentSha256", "exitCode", "id", "inputsSha256", "moduleMode", "packages", "profile", "rawStderrSha256", "rawStdoutSha256", "requestedRunnerPackages", "sourceWsi", "toolchainId"); err != nil {
		return nil, err
	}
	doc := &discoveryDocument{raw: object}
	if doc.Argv, err = stringsField(object, "argv", true); err != nil || len(doc.Argv) < 6 {
		return nil, reject(discoveryDecode)
	}
	argvBytes := 0
	for _, argument := range doc.Argv {
		if len(argument)+1 > maxArgvBytes-argvBytes {
			return nil, reject(limitExceeded)
		}
		argvBytes += len(argument) + 1
	}
	if doc.Argv[0] != "@PINNED_GO@" || doc.Argv[1] != "list" || doc.Argv[2] != "-deps" || doc.Argv[3] != "-test" || doc.Argv[4] != "-json="+closedListFields || !validPatterns(doc.Argv[5:]) {
		return nil, reject(discoveryInput)
	}
	for _, assignment := range []struct {
		target *string
		key    string
	}{
		{&doc.DependencyMaterializationSHA256, "dependencyMaterializationSha256"},
		{&doc.EnvironmentSHA256, "environmentSha256"}, {&doc.ExitCode, "exitCode"},
		{&doc.ID, "id"}, {&doc.InputsSHA256, "inputsSha256"}, {&doc.ModuleMode, "moduleMode"}, {&doc.Profile, "profile"},
		{&doc.RawStderrSHA256, "rawStderrSha256"}, {&doc.RawStdoutSHA256, "rawStdoutSha256"},
		{&doc.SourceWSI, "sourceWsi"}, {&doc.ToolchainID, "toolchainId"},
	} {
		if *assignment.target, err = stringField(object, assignment.key); err != nil {
			return nil, err
		}
	}
	emptyDigest := sha256.Sum256(nil)
	if doc.Profile != discoveryProfile || doc.ExitCode != "0" || !isDigest(doc.DependencyMaterializationSHA256) || !isDigest(doc.EnvironmentSHA256) || !isDigest(doc.InputsSHA256) || doc.RawStderrSHA256 != hex.EncodeToString(emptyDigest[:]) || !isDigest(doc.RawStdoutSHA256) || !isID(doc.ID, "go-live-discovery") || !isID(doc.SourceWSI, "workspace-source") || !isID(doc.ToolchainID, "go-toolchain") || doc.ModuleMode != "MODULE" && doc.ModuleMode != "WORKSPACE" && doc.ModuleMode != "VENDOR" {
		return nil, reject(discoveryInput)
	}
	if doc.RequestedRunnerPackages, err = stringsField(object, "requestedRunnerPackages", false); err != nil {
		return nil, err
	}
	if len(doc.RequestedRunnerPackages) == 0 {
		return nil, reject(discoveryInput)
	}
	rawPackages, ok := object["packages"].([]any)
	if !ok || len(rawPackages) == 0 {
		return nil, reject(discoveryDecode)
	}
	if len(rawPackages) > maxPackages {
		return nil, reject(limitExceeded)
	}
	doc.Packages = make([]packageDocument, len(rawPackages))
	packageBytes := make([]string, len(rawPackages))
	requested := make([]string, 0)
	matchedPatterns := make(map[string]struct{})
	packageInputs := make([]packageInput, 0, len(rawPackages))
	ids := map[string]struct{}{}
	importPaths := map[string]struct{}{}
	for index, value := range rawPackages {
		packageObject, ok := value.(map[string]any)
		if !ok {
			return nil, reject(discoveryDecode)
		}
		pkg, err := verifyPackage(packageObject)
		if err != nil {
			return nil, err
		}
		if _, exists := ids[pkg.ID]; exists {
			return nil, reject(discoveryDecode)
		}
		if _, exists := importPaths[pkg.ImportPath]; exists {
			return nil, reject(discoveryDecode)
		}
		ids[pkg.ID] = struct{}{}
		importPaths[pkg.ImportPath] = struct{}{}
		doc.Packages[index] = *pkg
		encoded, _ := canonical(packageObject)
		packageBytes[index] = string(encoded)
		if pkg.Kind == "REQUESTED" {
			if !validExactImportPath(pkg.ImportPath) {
				return nil, reject(discoveryInput)
			}
			requested = append(requested, pkg.ImportPath)
		}
		for _, match := range pkg.Match {
			if !containsString(doc.Argv[5:], match) {
				return nil, reject(discoveryInput)
			}
			matchedPatterns[match] = struct{}{}
		}
		packageInputs = append(packageInputs, packageInput{
			DepOnly: pkg.DepOnly, ForTest: pkg.ForTest, ID: pkg.ID, Imports: pkg.Imports,
			Match: pkg.Match, ModuleSHA256: pkg.ModuleSHA256, Name: pkg.Name,
			TestImports: pkg.TestImports, XTestImports: pkg.XTestImports,
		})
	}
	if !sortedUnique(packageBytes) {
		return nil, reject(discoveryInput)
	}
	sort.Strings(requested)
	requested = uniqueStrings(requested)
	if !equalStrings(requested, doc.RequestedRunnerPackages) {
		return nil, reject(discoveryInput)
	}
	matched := make([]string, 0, len(matchedPatterns))
	for pattern := range matchedPatterns {
		matched = append(matched, pattern)
	}
	sort.Strings(matched)
	if !equalStrings(matched, doc.Argv[5:]) {
		return nil, reject(discoveryInput)
	}
	if err := VerifyDiscoveryInputsIdentity(doc.InputsSHA256, discoveryInputs{
		DependencyMaterializationSHA256: doc.DependencyMaterializationSHA256,
		ModuleMode:                      doc.ModuleMode, Packages: packageInputs, SourceWSI: doc.SourceWSI,
	}); err != nil {
		return nil, err
	}
	body, _ := canonical(without(object, "id"))
	if expected := domainID("go-live-discovery", discoveryProfile, body); doc.ID != expected {
		return nil, reject(identityMismatch)
	}
	return doc, nil
}

func validPatterns(patterns []string) bool {
	if len(patterns) == 0 || len(patterns) > 4091 {
		return false
	}
	if len(patterns) == 1 && patterns[0] == "./..." {
		return true
	}
	if !sortedUnique(patterns) {
		return false
	}
	for _, pattern := range patterns {
		if !validExactImportPath(pattern) {
			return false
		}
	}
	return true
}

func validExactImportPath(path string) bool {
	if path == "" || len(path) > maxPatternBytes || !validText(path) || strings.HasPrefix(path, "-") || strings.ContainsAny(path, "@,*?[]\\") {
		return false
	}
	for _, r := range path {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return false
		}
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "" || segment == "." || segment == ".." || segment == "..." {
			return false
		}
	}
	return true
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func verifyPackage(object map[string]any) (*packageDocument, error) {
	if err := exactKeys(object, "depOnly", "dirPathSha256", "forTest", "id", "importPath", "inputFiles", "inputFilesSha256", "imports", "kind", "match", "module", "moduleSha256", "name", "testImports", "xTestImports"); err != nil {
		return nil, err
	}
	pkg := &packageDocument{raw: object}
	var err error
	var ok bool
	if pkg.DepOnly, ok = object["depOnly"].(bool); !ok {
		return nil, reject(discoveryDecode)
	}
	if pkg.ForTest, err = nullableString(object, "forTest"); err != nil {
		return nil, err
	}
	if pkg.ModuleSHA256, err = nullableString(object, "moduleSha256"); err != nil {
		return nil, err
	}
	for _, assignment := range []struct {
		target *string
		key    string
	}{
		{&pkg.DirPathSHA256, "dirPathSha256"}, {&pkg.ID, "id"}, {&pkg.ImportPath, "importPath"},
		{&pkg.InputFilesSHA256, "inputFilesSha256"}, {&pkg.Kind, "kind"}, {&pkg.Name, "name"},
	} {
		if *assignment.target, err = stringField(object, assignment.key); err != nil {
			return nil, err
		}
	}
	if pkg.Match, err = stringsField(object, "match", false); err != nil {
		return nil, err
	}
	if pkg.Imports, err = stringsField(object, "imports", false); err != nil {
		return nil, err
	}
	if pkg.TestImports, err = stringsField(object, "testImports", false); err != nil {
		return nil, err
	}
	if pkg.XTestImports, err = stringsField(object, "xTestImports", false); err != nil {
		return nil, err
	}
	if pkg.InputFiles, err = parseInputEntries(object["inputFiles"]); err != nil {
		return nil, err
	}
	if err := VerifyInputSetIdentity(pkg.InputFilesSHA256, pkg.InputFiles); err != nil {
		return nil, err
	}
	if object["module"] == nil {
		if pkg.ModuleSHA256 != nil {
			return nil, reject(discoveryInput)
		}
	} else {
		moduleObject, ok := object["module"].(map[string]any)
		if !ok || pkg.ModuleSHA256 == nil {
			return nil, reject(discoveryDecode)
		}
		module, err := parseModule(moduleObject, false)
		if err != nil {
			return nil, err
		}
		pkg.Module = module
		if err := VerifyModuleIdentity(*pkg.ModuleSHA256, *module); err != nil {
			return nil, err
		}
	}
	if !isDigest(pkg.DirPathSHA256) || !isDigest(pkg.InputFilesSHA256) || !isID(pkg.ID, "go-package") || pkg.ModuleSHA256 != nil && !isDigest(*pkg.ModuleSHA256) {
		return nil, reject(discoveryInput)
	}
	expectedKind, valid := classify(pkg)
	if !valid || pkg.Kind != expectedKind {
		return nil, reject(discoveryDecode)
	}
	if pkg.Kind == "TEST_MAIN" {
		if len(pkg.InputFiles) != 1 || pkg.InputFiles[0].PathSHA256 != generatedTestMainPath(pkg.ImportPath, pkg.InputFiles[0]) {
			return nil, reject(discoveryInput)
		}
	}
	body, _ := canonical(packageIdentityObject(pkg))
	if expected := domainID("go-package", "go-package/0", body); pkg.ID != expected {
		return nil, reject(identityMismatch)
	}
	return pkg, nil
}

func generatedTestMainPath(importPath string, input inputEntry) string {
	body, _ := canonical(map[string]any{
		"importPath": importPath, "mode": input.Mode, "rawSha256": input.RawSHA256,
		"role": "TEST_MAIN_GOFILE",
	})
	return bareDomainDigest("go-generated-testmain", "go-generated-testmain/0", body)
}

func packageIdentityObject(pkg *packageDocument) map[string]any {
	return map[string]any{
		"depOnly": pkg.DepOnly, "dirPathSha256": pkg.DirPathSHA256,
		"forTest": optional(pkg.ForTest), "importPath": pkg.ImportPath,
		"inputFilesSha256": pkg.InputFilesSHA256, "kind": pkg.Kind,
		"match": stringArray(pkg.Match), "moduleSha256": optional(pkg.ModuleSHA256),
		"name": pkg.Name,
	}
}

func parseInputEntries(value any) ([]inputEntry, error) {
	array, ok := value.([]any)
	if !ok || len(array) > maxArrayEntries {
		return nil, reject(discoveryInput)
	}
	result := make([]inputEntry, len(array))
	encoded := make([]string, len(array))
	for index, item := range array {
		object, ok := item.(map[string]any)
		if !ok {
			return nil, reject(discoveryDecode)
		}
		if err := exactKeys(object, "mode", "pathSha256", "rawSha256"); err != nil {
			return nil, err
		}
		var err error
		if result[index].Mode, err = stringField(object, "mode"); err != nil {
			return nil, err
		}
		if result[index].PathSHA256, err = stringField(object, "pathSha256"); err != nil {
			return nil, err
		}
		if result[index].RawSHA256, err = stringField(object, "rawSha256"); err != nil {
			return nil, err
		}
		if !validMode(result[index].Mode) || !isDigest(result[index].PathSHA256) || !isDigest(result[index].RawSHA256) {
			return nil, reject(discoveryInput)
		}
		raw, _ := canonical(object)
		encoded[index] = string(raw)
	}
	if !sortedUnique(encoded) {
		return nil, reject(discoveryInput)
	}
	return result, nil
}

func validMode(mode string) bool {
	if len(mode) != 4 || mode[0] != '0' {
		return false
	}
	for _, digit := range mode {
		if digit < '0' || digit > '7' {
			return false
		}
	}
	return true
}

func classify(pkg *packageDocument) (string, bool) {
	forTest := pkg.ForTest != nil
	hasMatch := len(pkg.Match) > 0
	switch {
	case hasMatch && !forTest && !pkg.DepOnly:
		return "REQUESTED", true
	case forTest:
		return "TEST_VARIANT", true
	case !hasMatch && !pkg.DepOnly && pkg.Name == "main":
		return "TEST_MAIN", true
	case pkg.DepOnly:
		return "DEPENDENCY", true
	default:
		return "", false
	}
}

func parseModule(object map[string]any, replacement bool) (*moduleBody, error) {
	if err := exactKeys(object, "dirPathSha256", "goModPathSha256", "goModSum", "goVersion", "indirect", "main", "path", "replace", "sum", "timeRaw", "version"); err != nil {
		return nil, err
	}
	module := &moduleBody{}
	var err error
	if module.DirPathSHA256, err = nullableString(object, "dirPathSha256"); err != nil {
		return nil, err
	}
	if module.GoModPathSHA256, err = nullableString(object, "goModPathSha256"); err != nil {
		return nil, err
	}
	if module.Path, err = stringField(object, "path"); err != nil {
		return nil, err
	}
	for key, target := range map[string]**string{
		"goModSum": &module.GoModSum, "goVersion": &module.GoVersion,
		"sum": &module.Sum, "timeRaw": &module.TimeRaw, "version": &module.Version,
	} {
		if *target, err = nullableString(object, key); err != nil {
			return nil, err
		}
	}
	var ok bool
	if module.Indirect, ok = object["indirect"].(bool); !ok {
		return nil, reject(discoveryDecode)
	}
	if module.Main, ok = object["main"].(bool); !ok {
		return nil, reject(discoveryDecode)
	}
	if object["replace"] != nil {
		if replacement {
			return nil, reject(discoveryInput)
		}
		replaceObject, ok := object["replace"].(map[string]any)
		if !ok {
			return nil, reject(discoveryDecode)
		}
		if module.Replace, err = parseModule(replaceObject, true); err != nil {
			return nil, err
		}
	}
	if err := validateModule(*module, replacement); err != nil {
		return nil, err
	}
	return module, nil
}

func VerifyModuleIdentity(expected string, module moduleBody) error {
	if err := validateModule(module, false); err != nil {
		return err
	}
	body, err := canonical(moduleObject(module))
	if err != nil || !isDigest(expected) {
		return reject(discoveryInput)
	}
	if bareDomainDigest("go-module", "go-module/0", body) != expected {
		return reject(identityMismatch)
	}
	return nil
}

func validateModule(module moduleBody, replacement bool) error {
	if module.DirPathSHA256 != nil && !isDigest(*module.DirPathSHA256) || module.GoModPathSHA256 != nil && !isDigest(*module.GoModPathSHA256) || module.Path == "" {
		return reject(discoveryInput)
	}
	for _, value := range []*string{module.GoModSum, module.GoVersion, module.Sum, module.TimeRaw, module.Version} {
		if value != nil && (*value == "" || len(*value) > maxStringBytes || !validText(*value)) {
			return reject(discoveryInput)
		}
	}
	if replacement && module.Replace != nil {
		return reject(discoveryInput)
	}
	if module.Replace != nil {
		if err := validateModule(*module.Replace, true); err != nil {
			return err
		}
	}
	return nil
}

func moduleObject(module moduleBody) map[string]any {
	var replace any
	if module.Replace != nil {
		replace = moduleObject(*module.Replace)
	}
	return map[string]any{
		"dirPathSha256": optional(module.DirPathSHA256), "goModPathSha256": optional(module.GoModPathSHA256),
		"goModSum": optional(module.GoModSum), "goVersion": optional(module.GoVersion),
		"indirect": module.Indirect, "main": module.Main, "path": module.Path,
		"replace": replace, "sum": optional(module.Sum), "timeRaw": optional(module.TimeRaw),
		"version": optional(module.Version),
	}
}

func VerifyInputSetIdentity(expected string, entries []inputEntry) error {
	objects := make([]any, len(entries))
	encoded := make([]string, len(entries))
	for index, entry := range entries {
		if !validMode(entry.Mode) || !isDigest(entry.PathSHA256) || !isDigest(entry.RawSHA256) {
			return reject(discoveryInput)
		}
		objects[index] = map[string]any{"mode": entry.Mode, "pathSha256": entry.PathSHA256, "rawSha256": entry.RawSHA256}
		raw, _ := canonical(objects[index])
		encoded[index] = string(raw)
	}
	if !sortedUnique(encoded) {
		return reject(discoveryInput)
	}
	body, _ := canonical(objects)
	if !isDigest(expected) || bareDomainDigest("go-input-set", "go-input-set/0", body) != expected {
		return reject(identityMismatch)
	}
	return nil
}

func VerifyDiscoveryInputsIdentity(expected string, inputs discoveryInputs) error {
	if !isDigest(inputs.DependencyMaterializationSHA256) || !isID(inputs.SourceWSI, "workspace-source") || inputs.ModuleMode != "MODULE" && inputs.ModuleMode != "WORKSPACE" && inputs.ModuleMode != "VENDOR" {
		return reject(discoveryInput)
	}
	packages := make([]any, len(inputs.Packages))
	for index, pkg := range inputs.Packages {
		if !isID(pkg.ID, "go-package") || !sortedUniqueOrEmpty(pkg.Imports) || !sortedUniqueOrEmpty(pkg.Match) || !sortedUniqueOrEmpty(pkg.TestImports) || !sortedUniqueOrEmpty(pkg.XTestImports) || pkg.ModuleSHA256 != nil && !isDigest(*pkg.ModuleSHA256) {
			return reject(discoveryInput)
		}
		packages[index] = map[string]any{
			"depOnly": pkg.DepOnly, "forTest": optional(pkg.ForTest), "id": pkg.ID,
			"imports": stringArray(pkg.Imports), "match": stringArray(pkg.Match),
			"moduleSha256": optional(pkg.ModuleSHA256), "name": pkg.Name,
			"testImports": stringArray(pkg.TestImports), "xTestImports": stringArray(pkg.XTestImports),
		}
	}
	sort.Slice(packages, func(left, right int) bool {
		leftRaw, _ := canonical(packages[left])
		rightRaw, _ := canonical(packages[right])
		return bytes.Compare(leftRaw, rightRaw) < 0
	})
	for index := 1; index < len(packages); index++ {
		leftRaw, _ := canonical(packages[index-1])
		rightRaw, _ := canonical(packages[index])
		if bytes.Equal(leftRaw, rightRaw) {
			return reject(discoveryInput)
		}
	}
	body, _ := canonical(map[string]any{
		"dependencyMaterializationSha256": inputs.DependencyMaterializationSHA256,
		"moduleMode":                      inputs.ModuleMode, "packages": packages, "sourceWsi": inputs.SourceWSI,
	})
	if !isDigest(expected) || bareDomainDigest("go-discovery-inputs", "go-discovery-inputs/0", body) != expected {
		return reject(identityMismatch)
	}
	return nil
}

func optional(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func stringArray(values []string) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = value
	}
	return result
}

func sortedUniqueOrEmpty(values []string) bool {
	if len(values) == 0 {
		return true
	}
	return sortedUnique(values)
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
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

func run(arguments []string, stdout, stderr io.Writer) int {
	if len(arguments) == 3 {
		return runTranscriptConformance(arguments, stdout)
	}
	if len(arguments) != 1 {
		fmt.Fprintln(stderr, "usage: go-live-test-v0 DISCOVERY.json [TRANSCRIPT.jsonl CONTEXT.json]")
		return 2
	}
	before, err := os.Lstat(arguments[0])
	if err != nil || !before.Mode().IsRegular() {
		fmt.Fprintln(stdout, `{"code":"DISCOVERY_INPUT","profile":"go-live-discovery-conformance/0","status":"FAIL"}`)
		return 1
	}
	file, err := os.Open(arguments[0])
	if err != nil {
		fmt.Fprintln(stdout, `{"code":"DISCOVERY_INPUT","profile":"go-live-discovery-conformance/0","status":"FAIL"}`)
		return 1
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(before, info) || info.Size() < 1 || info.Size() > maxDocumentBytes {
		fmt.Fprintln(stdout, `{"code":"LIMIT_EXCEEDED","profile":"go-live-discovery-conformance/0","status":"FAIL"}`)
		return 1
	}
	raw, err := io.ReadAll(io.LimitReader(file, maxDocumentBytes+1))
	if err != nil || len(raw) > maxDocumentBytes {
		fmt.Fprintln(stdout, `{"code":"LIMIT_EXCEEDED","profile":"go-live-discovery-conformance/0","status":"FAIL"}`)
		return 1
	}
	document, err := VerifyDiscovery(raw)
	if err != nil {
		fmt.Fprintf(stdout, "{\"code\":%q,\"profile\":\"go-live-discovery-conformance/0\",\"status\":\"FAIL\"}\n", codeOf(err))
		return 1
	}
	fmt.Fprintf(stdout, "{\"acquisitionPreimages\":\"NOT_RUN\",\"packages\":%q,\"profile\":\"go-live-discovery-conformance/0\",\"publishedCommitments\":\"VERIFIED\",\"runnerPackages\":%q,\"status\":\"PASS\"}\n", fmt.Sprint(len(document.Packages)), fmt.Sprint(len(document.RequestedRunnerPackages)))
	return 0
}

func runTranscriptConformance(arguments []string, stdout io.Writer) int {
	discoveryRaw, err := readConformanceFile(arguments[0], maxDocumentBytes)
	if err != nil {
		fmt.Fprintln(stdout, `{"code":"DISCOVERY_INPUT","profile":"go-live-transcript-conformance/0","status":"FAIL"}`)
		return 1
	}
	discovery, err := VerifyDiscovery(discoveryRaw)
	if err != nil {
		fmt.Fprintf(stdout, "{\"code\":%q,\"profile\":\"go-live-transcript-conformance/0\",\"status\":\"FAIL\"}\n", codeOf(err))
		return 1
	}
	transcriptRaw, err := readConformanceFile(arguments[1], maxTranscriptBytes)
	if err != nil {
		fmt.Fprintln(stdout, `{"code":"LIMIT_EXCEEDED","profile":"go-live-transcript-conformance/0","status":"FAIL"}`)
		return 1
	}
	contextRaw, err := readConformanceFile(arguments[2], maxDocumentBytes)
	if err != nil {
		fmt.Fprintln(stdout, `{"code":"DISCOVERY_INPUT","profile":"go-live-transcript-conformance/0","status":"FAIL"}`)
		return 1
	}
	var context transcriptContext
	if err := json.Unmarshal(contextRaw, &context); err != nil || !contextMatchesDiscovery(context, discovery) {
		fmt.Fprintln(stdout, `{"code":"IDENTITY_MISMATCH","profile":"go-live-transcript-conformance/0","status":"FAIL"}`)
		return 1
	}
	if err := VerifyTranscript(transcriptRaw, context); err != nil {
		fmt.Fprintf(stdout, "{\"code\":%q,\"profile\":\"go-live-transcript-conformance/0\",\"status\":\"FAIL\"}\n", codeOf(err))
		return 1
	}
	fmt.Fprintln(stdout, `{"acquisition":"VERIFIED","profile":"go-live-transcript-conformance/0","status":"PASS","transcript":"VERIFIED"}`)
	return 0
}

func readConformanceFile(path string, limit int) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Size() < 1 || before.Size() > int64(limit) {
		return nil, reject(discoveryInput)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) || opened.Mode() != before.Mode() || opened.Size() != before.Size() {
		return nil, reject(discoveryInput)
	}
	value, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil || len(value) > limit {
		return nil, reject(limitExceeded)
	}
	after, err := file.Stat()
	if err != nil || !os.SameFile(opened, after) || after.Mode() != opened.Mode() || after.Size() != opened.Size() {
		return nil, reject(discoveryInput)
	}
	return value, nil
}

func contextMatchesDiscovery(context transcriptContext, discovery *discoveryDocument) bool {
	capability, ok := decodeIdentityPreimage(context.CapabilityPreimageBase64, context.CapabilityID, "go-live-capability", "go-live-capability/0")
	if !ok || !verifyCapabilityBody(capability, context.VerifierExecutableSHA256) {
		return false
	}
	plan, ok := decodeIdentityPreimage(context.PlanPreimageBase64, context.PlanID, "go-live-plan", "go-live-plan/0")
	if !ok || !verifyPlanBody(plan, context, discovery) {
		return false
	}
	paths := make([]string, len(discovery.Packages))
	ids := make([]string, len(discovery.Packages))
	for index, pack := range discovery.Packages {
		paths[index], ids[index] = pack.ImportPath, pack.ID
	}
	sort.Strings(paths)
	sort.Strings(ids)
	return context.ActualEnvironmentSHA256 == discovery.EnvironmentSHA256 && context.DiscoveryID == discovery.ID &&
		context.ToolchainID == discovery.ToolchainID && equalStrings(context.DiscoveredPackagePaths, paths) &&
		equalStrings(context.ListedPackages, ids) && equalStrings(context.RequestedPackagePatterns, discovery.Argv[5:]) &&
		equalStrings(context.RequestedRunnerPackages, discovery.RequestedRunnerPackages)
}

func decodeIdentityPreimage(encoded, id, domain, profile string) (map[string]any, bool) {
	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil || len(raw) == 0 || len(raw) > maxDocumentBytes || !utf8.Valid(raw) || duplicateKeys(raw) {
		return nil, false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := decodeValue(decoder, 1)
	if err != nil {
		return nil, false
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, false
	}
	canonicalRaw, err := canonical(value)
	object, objectOK := value.(map[string]any)
	return object, err == nil && objectOK && bytes.Equal(raw, canonicalRaw) && id == domainID(domain, profile, raw)
}

func verifyCapabilityBody(object map[string]any, verifierExecutableSHA256 string) bool {
	if exactKeys(object, "goProtocol", "operations", "profile", "providerBuildSha256", "providerVersion", "supportedContainment", "supportedCoverage", "supportedNetwork") != nil {
		return false
	}
	protocol, _ := stringField(object, "goProtocol")
	profile, _ := stringField(object, "profile")
	build, _ := stringField(object, "providerBuildSha256")
	version, _ := stringField(object, "providerVersion")
	operations, operationsErr := stringsField(object, "operations", true)
	containment, containmentErr := stringsField(object, "supportedContainment", true)
	coverage, coverageErr := stringsField(object, "supportedCoverage", true)
	network, networkErr := stringsField(object, "supportedNetwork", true)
	providerBuildBody, err := canonical(map[string]any{"executableRawSha256": verifierExecutableSHA256, "goVersion": "go1.27.1"})
	return err == nil && isDigest(verifierExecutableSHA256) && protocol == "go1.27/test2json" && profile == "go-live-capability/0" && build == bareDomainDigest("go-provider-build", "go-provider-build/0", providerBuildBody) &&
		version == "0.1.0-experimental" && operationsErr == nil && equalStrings(operations, []string{"execute"}) &&
		containmentErr == nil && equalStrings(containment, []string{"PROCESS_GROUP_BEST_EFFORT"}) &&
		coverageErr == nil && equalStrings(coverage, []string{"NONE"}) && networkErr == nil && equalStrings(network, []string{"UNKNOWN"})
}

func verifyPlanBody(object map[string]any, context transcriptContext, discovery *discoveryDocument) bool {
	if exactKeys(object, "capabilityId", "coverage", "discoveryId", "environment", "invocation", "limits", "limitsEnforced", "network", "profile", "scope", "source", "toolchain") != nil {
		return false
	}
	capabilityID, _ := stringField(object, "capabilityId")
	discoveryID, _ := stringField(object, "discoveryId")
	profile, _ := stringField(object, "profile")
	if capabilityID != context.CapabilityID || discoveryID != discovery.ID || profile != "go-live-plan/0" {
		return false
	}
	if !canonicalEqual(object["coverage"], map[string]any{"mode": "NONE", "packagePatterns": []any{}}) ||
		!canonicalEqual(object["network"], map[string]any{"mechanism": nil, "mode": "UNKNOWN"}) ||
		!canonicalEqual(object["limits"], map[string]any{
			"coverageBytes": "268435456", "coverageFiles": "4096", "cpuMilliseconds": "1800000",
			"eventBytes": "16777216", "events": "100000", "lineBytes": "1048576", "memoryBytes": "4294967296",
			"openFiles": "4096", "outputBytes": "8388608", "packages": "4096", "processes": "1024",
			"runMilliseconds": "1800000", "tests": "100000",
		}) || !canonicalEqual(object["limitsEnforced"], []any{"EVENT_BYTES", "EVENTS", "LINE_BYTES", "OUTPUT_BYTES", "PACKAGES", "RUN_TIME", "TESTS"}) {
		return false
	}
	invocation, ok := object["invocation"].(map[string]any)
	if !ok || exactKeys(invocation, "argv", "cwdPathSha256", "packagePatterns") != nil ||
		!canonicalEqual(invocation["argv"], []any{"@PINNED_GO@", "test", "-json", "-count=1", "-vet=off"}) ||
		!canonicalEqual(invocation["packagePatterns"], stringAny(context.RequestedPackagePatterns)) {
		return false
	}
	cwd, _ := stringField(invocation, "cwdPathSha256")
	if !isDigest(cwd) || !verifyPlanEnvironment(object["environment"], context.ActualEnvironmentSHA256) {
		return false
	}
	scope, ok := object["scope"].(map[string]any)
	if !ok || exactKeys(scope, "conclusion", "excluded", "requestedPackagePatterns", "unknownReasons") != nil ||
		scope["conclusion"] != "UNKNOWN" || !canonicalEqual(scope["excluded"], []any{}) ||
		!canonicalEqual(scope["requestedPackagePatterns"], stringAny(context.RequestedPackagePatterns)) ||
		!canonicalEqual(scope["unknownReasons"], stringAny(mandatoryUnknownReasons)) {
		return false
	}
	source, ok := object["source"].(map[string]any)
	if !ok || exactKeys(source, "dependencyMaterializationSha256", "materializationSha256", "moduleMode", "rootPathSha256", "wsi") != nil ||
		source["dependencyMaterializationSha256"] != discovery.DependencyMaterializationSHA256 || source["moduleMode"] != discovery.ModuleMode || source["wsi"] != discovery.SourceWSI {
		return false
	}
	materialization, _ := stringField(source, "materializationSha256")
	rootPath, _ := stringField(source, "rootPathSha256")
	toolchain, ok := object["toolchain"].(map[string]any)
	if !ok || exactKeys(toolchain, "cgoEnabled", "goarch", "goenvSha256", "goexeSha256", "goos", "gorootSha256", "goversion", "id", "invokedToolsSha256", "pathSha256", "toolDirSha256") != nil ||
		toolchain["cgoEnabled"] != "0" || toolchain["goversion"] != "go1.27.1" || toolchain["id"] != discovery.ToolchainID {
		return false
	}
	if !isDigest(materialization) || rootPath != cwd {
		return false
	}
	for _, key := range []string{"goenvSha256", "goexeSha256", "gorootSha256", "invokedToolsSha256", "pathSha256", "toolDirSha256"} {
		value, _ := stringField(toolchain, key)
		if !isDigest(value) {
			return false
		}
	}
	goarch, _ := stringField(toolchain, "goarch")
	goos, _ := stringField(toolchain, "goos")
	toolchainBody, err := canonical(without(toolchain, "id"))
	return err == nil && toolchain["id"] == domainID("go-toolchain", "go-toolchain/0", toolchainBody) &&
		goarch == context.GOARCH && goos == context.GOOS && goarch != "" && goos != "" && validText(goarch) && validText(goos)
}

func verifyPlanEnvironment(value any, expected string) bool {
	rows, ok := value.([]any)
	if !ok || len(rows) == 0 {
		return false
	}
	previous := ""
	for _, value := range rows {
		row, ok := value.(map[string]any)
		if !ok || exactKeys(row, "name", "valueSha256") != nil {
			return false
		}
		name, _ := stringField(row, "name")
		digest, _ := stringField(row, "valueSha256")
		if name <= previous || !validText(name) || !isDigest(digest) {
			return false
		}
		previous = name
	}
	body, err := canonical(rows)
	return err == nil && bareDomainDigest("go-environment", "go-environment/0", body) == expected
}

func canonicalEqual(left, right any) bool {
	a, errA := canonical(left)
	b, errB := canonical(right)
	return errA == nil && errB == nil && bytes.Equal(a, b)
}

func stringAny(values []string) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = value
	}
	return result
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
