package godiscovery

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
)

const go127Fixture = `{
	"Dir": "/workspace/example",
	"ImportPath": "example.test/example",
	"Name": "example",
	"Match": ["./..."],
	"Module": {
		"Path": "example.test",
		"Version": "",
		"Replace": {
			"Path": "example.test/local",
			"Dir": "/workspace/local",
			"GoMod": "/workspace/local/go.mod",
			"GoVersion": "1.27"
		},
		"Main": true,
		"Dir": "/workspace/example",
		"GoMod": "/workspace/go.mod",
		"GoVersion": "1.27"
	},
	"GoFiles": ["example.go"],
	"TestGoFiles": ["example_test.go"],
	"Imports": ["fmt"],
	"TestImports": ["testing"]
}
{
	"Dir": "/workspace/example",
	"ImportPath": "example.test/example [example.test/example.test]",
	"Name": "example",
	"ForTest": "example.test/example",
	"Match": ["./..."],
	"Module": {
		"Path": "example.test",
		"Main": true,
		"Dir": "/workspace/example",
		"GoMod": "/workspace/go.mod",
		"GoVersion": "1.27"
	},
	"GoFiles": ["example.go", "example_test.go"],
	"TestGoFiles": ["example_test.go"],
	"Imports": ["fmt", "testing", "fmt"]
}
{
	"Dir": "/workspace/example",
	"ImportPath": "example.test/example.test",
	"Name": "main",
	"Module": {
		"Path": "example.test",
		"Main": true,
		"Dir": "/workspace/example",
		"GoMod": "/workspace/go.mod",
		"GoVersion": "1.27"
	},
	"GoFiles": ["/go-cache/ab/abcdef-d"]
}
`

func TestDecodeGo127PrettyObjectStream(t *testing.T) {
	commitments := fixtureCommitments("/go-cache/ab/abcdef-d")
	manifest, err := Decode(strings.NewReader(go127Fixture), commitments)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.RawStdoutBytes != uint64(len(go127Fixture)) {
		t.Fatalf("raw bytes = %d", manifest.RawStdoutBytes)
	}
	rawDigest := sha256.Sum256([]byte(go127Fixture))
	if manifest.RawStdoutSHA256 != hex.EncodeToString(rawDigest[:]) {
		t.Fatalf("raw digest = %s", manifest.RawStdoutSHA256)
	}
	if got := strings.Join(manifest.RequestedRunnerPackages, ","); got != "example.test/example" {
		t.Fatalf("requested packages = %s", got)
	}
	if len(manifest.Packages) != 3 || !validDigest(manifest.InputsSHA256) {
		t.Fatalf("manifest = %#v", manifest)
	}
	kinds := make(map[string]PackageKind)
	for _, pack := range manifest.Packages {
		kinds[pack.ImportPath] = pack.Kind
		if !strings.HasPrefix(pack.ID, "go-package:sha256:") || !validDigest(strings.TrimPrefix(pack.ID, "go-package:sha256:")) {
			t.Fatalf("package ID = %q", pack.ID)
		}
	}
	if kinds["example.test/example"] != Requested ||
		kinds["example.test/example [example.test/example.test]"] != TestVariant ||
		kinds["example.test/example.test"] != TestMain {
		t.Fatalf("kinds = %#v", kinds)
	}
	for _, pack := range manifest.Packages {
		if pack.ImportPath == "example.test/example [example.test/example.test]" {
			if len(pack.Imports) != 2 || len(pack.InputFiles) != 2 {
				t.Fatalf("test variant was not normalized unique: %#v", pack)
			}
		}
	}
}

func TestGeneratedTestMainHostPathIsNotIdentityMaterial(t *testing.T) {
	first, err := Decode(strings.NewReader(go127Fixture), fixtureCommitments("/go-cache/ab/abcdef-d"))
	if err != nil {
		t.Fatal(err)
	}
	secondRaw := strings.Replace(go127Fixture, "/go-cache/ab/abcdef-d", `/other-cache/cd/other-d`, 1)
	second, err := Decode(strings.NewReader(secondRaw), fixtureCommitments(`/other-cache/cd/other-d`))
	if err != nil {
		t.Fatal(err)
	}
	firstID := packageID(first, "example.test/example.test")
	secondID := packageID(second, "example.test/example.test")
	if firstID != secondID {
		t.Fatalf("generated host path changed ID: %s != %s", firstID, secondID)
	}
	if first.RawStdoutSHA256 == second.RawStdoutSHA256 {
		t.Fatal("raw acquisition digest did not bind changed host path")
	}
}

func TestDecodeDeterministicAcrossCommitmentOrder(t *testing.T) {
	firstCommitments := fixtureCommitments("/go-cache/ab/abcdef-d")
	secondCommitments := fixtureCommitments("/go-cache/ab/abcdef-d")
	reversePaths(secondCommitments.ModulePaths)
	reverseMaterials(secondCommitments.Packages)
	for index := range secondCommitments.Packages {
		reverseFiles(secondCommitments.Packages[index].Files)
	}
	first, err := Decode(strings.NewReader(go127Fixture), firstCommitments)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Decode(strings.NewReader(go127Fixture), secondCommitments)
	if err != nil {
		t.Fatal(err)
	}
	if first.InputsSHA256 != second.InputsSHA256 || packageIDs(first) != packageIDs(second) {
		t.Fatalf("nondeterministic manifests:\n%#v\n%#v", first, second)
	}
}

func TestClassificationClosed(t *testing.T) {
	cases := []struct {
		name string
		pack rawPackage
		want PackageKind
		code Code
	}{
		{name: "requested-main", pack: rawPackage{name: "main", match: []string{"./..."}}, want: Requested},
		{name: "test-variant-wins-with-match", pack: rawPackage{name: "p", forTest: pointer("p"), match: []string{"./..."}}, want: TestVariant},
		{name: "test-main", pack: rawPackage{name: "main"}, want: TestMain},
		{name: "dependency", pack: rawPackage{name: "p", depOnly: true}, want: Dependency},
		{name: "nondependency-unclassified", pack: rawPackage{name: "p"}, code: CodeDecode},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := classify(test.pack)
			if test.code != "" {
				assertCode(t, err, test.code)
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("classify = %q, %v", got, err)
			}
		})
	}
}

func TestDecodeRejectsHostileDiscovery(t *testing.T) {
	minimal := `{"Dir":"/w","ImportPath":"p","Name":"p","Match":["p"]}`
	cases := []struct {
		name string
		raw  []byte
		code Code
	}{
		{name: "unknown", raw: []byte(`{"Dir":"/w","ImportPath":"p","Name":"p","Match":["p"],"Query":"x"}`), code: CodeDecode},
		{name: "duplicate-key", raw: []byte(`{"Dir":"/w","Dir":"/x","ImportPath":"p","Name":"p","Match":["p"]}`), code: CodeDecode},
		{name: "null-array", raw: []byte(`{"Dir":"/w","ImportPath":"p","Name":"p","Match":null}`), code: CodeDecode},
		{name: "invalid-utf8", raw: append([]byte(`{"Dir":"/w","ImportPath":"p","Name":"`), append([]byte{0xff}, []byte(`","Match":["p"]}`)...)...), code: CodeDecode},
		{name: "noncharacter", raw: []byte(`{"Dir":"/w","ImportPath":"p","Name":"\ufdd0","Match":["p"]}`), code: CodeDecode},
		{name: "array-root", raw: []byte(`[]`), code: CodeDecode},
		{name: "empty", raw: nil, code: CodeDecode},
		{name: "build-error", raw: []byte(`{"Dir":"/w","ImportPath":"p","Name":"p","Match":["p"],"Error":{"Err":"secret"}}`), code: CodeInput},
		{name: "deps-error", raw: []byte(`{"Dir":"/w","ImportPath":"p","Name":"p","Match":["p"],"DepsErrors":[{"Err":"secret"}]}`), code: CodeInput},
		{name: "unclassified", raw: []byte(`{"Dir":"/w","ImportPath":"p","Name":"p"}`), code: CodeDecode},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := Decode(bytes.NewReader(test.raw), minimalCommitments())
			assertCode(t, err, test.code)
			if err != nil && strings.Contains(err.Error(), "secret") {
				t.Fatalf("error leaked hostile value: %v", err)
			}
		})
	}
	oversize := ioRepeat(' ', MaxDiscoveryBytes+1)
	_, err := Decode(oversize, Commitments{})
	assertCode(t, err, CodeLimit)
	deep := strings.Repeat(`{"x":`, MaxJSONDepth) + minimal + strings.Repeat(`}`, MaxJSONDepth)
	_, err = Decode(strings.NewReader(deep), Commitments{})
	assertCode(t, err, CodeLimit)
}

func TestDecodeRejectsCommitmentMismatch(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Commitments)
	}{
		{name: "missing-package", mutate: func(value *Commitments) { value.Packages = value.Packages[:2] }},
		{name: "wrong-dir-key", mutate: func(value *Commitments) { value.Packages[0].RawDir = "/other" }},
		{name: "wrong-import", mutate: func(value *Commitments) { value.Packages[0].ImportPath = "other" }},
		{name: "missing-file", mutate: func(value *Commitments) { value.Packages[0].Files = value.Packages[0].Files[:1] }},
		{name: "duplicate-file", mutate: func(value *Commitments) { value.Packages[0].Files[1] = value.Packages[0].Files[0] }},
		{name: "bad-mode", mutate: func(value *Commitments) { value.Packages[0].Files[0].Mode = "420" }},
		{name: "absolute-source", mutate: func(value *Commitments) { value.Packages[0].Files[0].ListedName = "/etc/passwd" }},
		{name: "substituted-logical-file", mutate: func(value *Commitments) {
			value.Packages[0].Files[0].LogicalPath = "other.go"
			value.Packages[0].Files[0].PathSHA256 = logicalPathDigest(NamespaceSource, "other.go")
		}},
		{name: "substituted-namespace", mutate: func(value *Commitments) {
			value.Packages[0].Files[0].Namespace = NamespaceDependency
			value.Packages[0].Files[0].PathSHA256 = logicalPathDigest(NamespaceDependency, "example.go")
		}},
		{name: "conflicting-overlap", mutate: func(value *Commitments) {
			value.Packages[1].Files[2].RawSHA256 = digest("conflicting-content")
		}},
		{name: "generated-marked-source", mutate: func(value *Commitments) { value.Packages[2].Files[0].Role = RoleSource }},
		{name: "generated-windows-path", mutate: func(value *Commitments) { value.Packages[2].Files[0].ListedName = `C:\cache\testmain` }},
		{name: "generated-root-path", mutate: func(value *Commitments) { value.Packages[2].Files[0].ListedName = "/" }},
		{name: "source-marked-generated", mutate: func(value *Commitments) { value.Packages[0].Files[0].Role = RoleGeneratedTestMain }},
		{name: "missing-module-path", mutate: func(value *Commitments) { value.ModulePaths = value.ModulePaths[1:] }},
		{name: "toolchain-module-path", mutate: func(value *Commitments) {
			value.ModulePaths[0].Namespace = NamespaceToolchain
			value.ModulePaths[0].PathSHA256 = logicalPathDigest(NamespaceToolchain, value.ModulePaths[0].LogicalPath)
		}},
		{name: "unused-module-path", mutate: func(value *Commitments) {
			value.ModulePaths = append(value.ModulePaths, logicalPathCommitment("/unused", NamespaceSource, "unused"))
		}},
		{name: "bad-source-wsi", mutate: func(value *Commitments) { value.SourceWSI = "workspace-source:sha256:BAD" }},
		{name: "bad-module-mode", mutate: func(value *Commitments) { value.ModuleMode = "AUTO" }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			commitments := fixtureCommitments("/go-cache/ab/abcdef-d")
			test.mutate(&commitments)
			_, err := Decode(strings.NewReader(go127Fixture), commitments)
			assertCode(t, err, CodeInput)
		})
	}
}

func TestDecodeRejectsOversizedCommitmentCollectionsBeforeBinding(t *testing.T) {
	values := minimalCommitments()
	values.Packages = append(values.Packages, packageMaterial("other", "/w", nil))
	_, err := Decode(strings.NewReader(`{"Dir":"/w","ImportPath":"p","Name":"p","Match":["p"]}`), values)
	assertCode(t, err, CodeInput)

	values = minimalCommitments()
	values.ModulePaths = make([]PathCommitment, 5)
	_, err = Decode(strings.NewReader(`{"Dir":"/w","ImportPath":"p","Name":"p","Match":["p"]}`), values)
	assertCode(t, err, CodeInput)
}

func TestDecodeRejectsDuplicatePackageIdentity(t *testing.T) {
	raw := `{"Dir":"/w","ImportPath":"p","Name":"p","DepOnly":true}`
	commitments := minimalCommitments()
	commitments.Packages = []PackageMaterial{
		packageMaterial("p", "/w", nil),
		packageMaterial("p", "/w", nil),
	}
	_, err := Decode(strings.NewReader(raw+raw), commitments)
	assertCode(t, err, CodeDecode)
}

func TestDecodePackageAndByteBounds(t *testing.T) {
	var raw strings.Builder
	object := `{"Dir":"/w","ImportPath":"p","Name":"p","DepOnly":true}`
	for range MaxPackages + 1 {
		raw.WriteString(object)
		raw.WriteByte('\n')
	}
	_, err := Decode(strings.NewReader(raw.String()), Commitments{})
	assertCode(t, err, CodeLimit)
	tooLong := strings.Repeat("x", MaxStringBytes+1)
	_, err = Decode(strings.NewReader(`{"Dir":"/w","ImportPath":"p","Name":"`+tooLong+`","DepOnly":true}`), Commitments{})
	assertCode(t, err, CodeLimit)
}

func TestMalformedBytesAfterPackageCeilingStayDecode(t *testing.T) {
	var raw strings.Builder
	object := `{"Dir":"/w","ImportPath":"p","Name":"p","DepOnly":true}`
	for range MaxPackages {
		raw.WriteString(object)
		raw.WriteByte('\n')
	}
	raw.WriteByte('!')
	_, err := Decode(strings.NewReader(raw.String()), Commitments{})
	assertCode(t, err, CodeDecode)
}

func TestPathGrammarsRejectC1Controls(t *testing.T) {
	if validLogicalPath("dir/\u0085file") || validRelativePath("dir/\u0085file") || validExactImportPath("example.test/\u0085pkg") {
		t.Fatal("C1 control was accepted in a path")
	}
}

func TestHashFramingKnownVector(t *testing.T) {
	got, err := Hash("go-package", "go-package/0", []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	const want = "7cf5cfd18f7c14e006e98da823537c85030335ab5b0a633ec71febf49f82e17f"
	if got != want {
		t.Fatalf("digest = %s, want %s", got, want)
	}
}

func FuzzDecodeNeverPanics(f *testing.F) {
	f.Add([]byte(go127Fixture))
	f.Add([]byte(`{"Dir":"/w","ImportPath":"p","Name":"p","Match":["p"]}`))
	f.Add([]byte{0xff, '{', '}'})
	f.Fuzz(func(t *testing.T, raw []byte) {
		_, _ = Decode(bytes.NewReader(raw), Commitments{})
	})
}

func fixtureCommitments(generatedName string) Commitments {
	paths := []string{"/workspace/example", "/workspace/go.mod", "/workspace/local", "/workspace/local/go.mod"}
	modulePaths := make([]PathCommitment, 0, len(paths))
	for _, path := range paths {
		logical := strings.TrimPrefix(path, "/workspace/")
		if path == "/workspace/example" {
			logical = "."
		}
		modulePaths = append(modulePaths, logicalPathCommitment(path, NamespaceSource, logical))
	}
	return Commitments{
		DependencyMaterializationSHA256: digest("dependency-materialization"),
		ModuleMode:                      ModuleModeModule,
		ModulePaths:                     modulePaths,
		Packages: []PackageMaterial{
			packageMaterial("example.test/example", "/workspace/example", []FileCommitment{
				file(FieldGoFiles, "example.go", RoleSource, "example.go"),
				file(FieldTestGoFiles, "example_test.go", RoleSource, "example_test.go"),
			}),
			packageMaterial("example.test/example [example.test/example.test]", "/workspace/example", []FileCommitment{
				file(FieldGoFiles, "example.go", RoleSource, "example.go"),
				file(FieldGoFiles, "example_test.go", RoleSource, "example_test.go"),
				file(FieldTestGoFiles, "example_test.go", RoleSource, "example_test.go"),
			}),
			packageMaterial("example.test/example.test", "/workspace/example", []FileCommitment{
				generatedFile("example.test/example.test", generatedName),
			}),
		},
		SourceWSI: "workspace-source:sha256:" + digest("source"),
	}
}

func minimalCommitments() Commitments {
	return Commitments{
		DependencyMaterializationSHA256: digest("dependency-materialization"),
		ModuleMode:                      ModuleModeModule,
		Packages:                        []PackageMaterial{packageMaterial("p", "/w", nil)},
		SourceWSI:                       "workspace-source:sha256:" + digest("source"),
	}
}

func file(field FileField, name string, role InputRole, identity string) FileCommitment {
	return FileCommitment{
		Field: field, ListedName: name, Role: role, Mode: "0644",
		Namespace: NamespaceSource, LogicalPath: identity,
		PathSHA256: logicalPathDigest(NamespaceSource, identity), RawSHA256: digest("raw:" + identity),
	}
}

func generatedFile(importPath, name string) FileCommitment {
	rawDigest := digest("raw:logical:testmain")
	pathDigest, err := bareDigest("go-generated-testmain", "go-generated-testmain/0", map[string]any{
		"importPath": importPath, "mode": "0644", "rawSha256": rawDigest, "role": "TEST_MAIN_GOFILE",
	})
	if err != nil {
		panic(err)
	}
	return FileCommitment{
		Field: FieldGoFiles, ListedName: name, Role: RoleGeneratedTestMain, Mode: "0644",
		PathSHA256: pathDigest, RawSHA256: rawDigest,
	}
}

func packageMaterial(importPath, rawDir string, files []FileCommitment) PackageMaterial {
	return PackageMaterial{
		ImportPath: importPath, RawDir: rawDir, DirNamespace: NamespaceSource,
		DirLogicalPath: ".", DirPathSHA256: logicalPathDigest(NamespaceSource, "."), Files: files,
	}
}

func logicalPathCommitment(raw string, namespace PathNamespace, logical string) PathCommitment {
	return PathCommitment{RawPath: raw, Namespace: namespace, LogicalPath: logical, PathSHA256: logicalPathDigest(namespace, logical)}
}

func logicalPathDigest(namespace PathNamespace, logical string) string {
	value, err := bareDigest("go-logical-path", "go-logical-path/0", map[string]any{
		"namespace": string(namespace), "path": logical,
	})
	if err != nil {
		panic(err)
	}
	return value
}

func digest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func packageID(manifest Manifest, importPath string) string {
	for _, pack := range manifest.Packages {
		if pack.ImportPath == importPath {
			return pack.ID
		}
	}
	return ""
}

func packageIDs(manifest Manifest) string {
	ids := make([]string, len(manifest.Packages))
	for index, pack := range manifest.Packages {
		ids[index] = pack.ID
	}
	return strings.Join(ids, ",")
}

func pointer(value string) *string { return &value }

func reversePaths(values []PathCommitment) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}

func reverseFiles(values []FileCommitment) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}

func reverseMaterials(values []PackageMaterial) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}

func assertCode(t *testing.T, err error, want Code) {
	t.Helper()
	var discoveryError *Error
	if !errors.As(err, &discoveryError) || discoveryError.Code != want {
		t.Fatalf("error = %v, want %s", err, want)
	}
}

func ioRepeat(value byte, count int) *strings.Reader {
	return strings.NewReader(strings.Repeat(string(value), count))
}

func ExampleClosedFields() {
	fmt.Println(strings.HasPrefix(ClosedFields, "Dir,ImportPath,Name"))
	// Output: true
}
