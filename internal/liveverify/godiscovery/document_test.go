package godiscovery

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

func TestComposeAndVerifyDocument(t *testing.T) {
	manifest, err := Decode(strings.NewReader(go127Fixture), fixtureCommitments("/go-cache/ab/abcdef-d"))
	if err != nil {
		t.Fatal(err)
	}
	empty := sha256.Sum256(nil)
	document, encoded, err := Compose(manifest, DocumentInput{
		EnvironmentSHA256: digest("environment"), PackagePatterns: []string{"./..."},
		RawStderrSHA256: hex.EncodeToString(empty[:]), ToolchainID: "go-toolchain:sha256:" + digest("toolchain"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyDocument(document, encoded); err != nil {
		t.Fatal(err)
	}
	if len(encoded) == 0 || encoded[len(encoded)-1] != '\n' || encoded[len(encoded)-2] == '\n' {
		t.Fatalf("document is not exactly one LF-terminated object: %q", encoded)
	}
	for _, secretPath := range []string{"/workspace", "/go-cache", `C:\\cache`} {
		if strings.Contains(string(encoded), secretPath) {
			t.Fatalf("document leaked raw host path %q", secretPath)
		}
	}
	for _, required := range []string{`"dependencyMaterializationSha256"`, `"inputFiles"`, `"imports"`, `"module"`, `"moduleMode"`, `"testImports"`, `"xTestImports"`} {
		if !strings.Contains(string(encoded), required) {
			t.Fatalf("document omitted recomputation field %s", required)
		}
	}
}

func TestVerifyDocumentRejectsForgery(t *testing.T) {
	document, encoded := fixtureDocument(t)
	cases := []struct {
		name   string
		mutate func(*Document, *[]byte)
	}{
		{name: "bytes", mutate: func(_ *Document, raw *[]byte) { *raw = append(append([]byte(nil), (*raw)...), ' ') }},
		{name: "id", mutate: func(value *Document, _ *[]byte) { value.ID = "go-live-discovery:sha256:" + digest("forged") }},
		{name: "profile", mutate: func(value *Document, _ *[]byte) { value.Profile = "go-live-discovery/1" }},
		{name: "exit", mutate: func(value *Document, _ *[]byte) { value.ExitCode = "1" }},
		{name: "stderr", mutate: func(value *Document, _ *[]byte) { value.RawStderrSHA256 = digest("stderr") }},
		{name: "argv", mutate: func(value *Document, _ *[]byte) { value.Argv[2] = "-e" }},
		{name: "unsorted-patterns", mutate: func(value *Document, _ *[]byte) { value.Argv = append(value.Argv[:5], "z.test/p", "a.test/p") }},
		{name: "package-order", mutate: func(value *Document, _ *[]byte) {
			value.Packages[0], value.Packages[1] = value.Packages[1], value.Packages[0]
		}},
		{name: "package-kind", mutate: func(value *Document, _ *[]byte) { value.Packages[0].Kind = Dependency }},
		{name: "package-id", mutate: func(value *Document, _ *[]byte) { value.Packages[0].ID = "go-package:sha256:" + digest("forged") }},
		{name: "duplicate-import-path", mutate: func(value *Document, _ *[]byte) { value.Packages[1].ImportPath = value.Packages[0].ImportPath }},
		{name: "input-file", mutate: func(value *Document, _ *[]byte) { value.Packages[0].InputFiles[0].RawSHA256 = digest("forged") }},
		{name: "unsorted-imports", mutate: func(value *Document, _ *[]byte) { value.Packages[0].Imports = []string{"z", "a"} }},
		{name: "module", mutate: func(value *Document, _ *[]byte) { value.Packages[0].Module.Path = "forged.test" }},
		{name: "nested-replace", mutate: func(value *Document, _ *[]byte) {
			for index := range value.Packages {
				if value.Packages[index].Module != nil && value.Packages[index].Module.Replace != nil {
					value.Packages[index].Module.Replace.Replace = &Module{Path: "nested"}
					return
				}
			}
		}},
		{name: "inputs", mutate: func(value *Document, _ *[]byte) { value.InputsSHA256 = digest("forged") }},
		{name: "requested", mutate: func(value *Document, _ *[]byte) { value.RequestedRunnerPackages = nil }},
		{name: "source", mutate: func(value *Document, _ *[]byte) { value.SourceWSI = "workspace-source:sha256:" + digest("forged") }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			value := cloneDocument(document)
			raw := append([]byte(nil), encoded...)
			test.mutate(&value, &raw)
			assertCode(t, VerifyDocument(value, raw), CodeInput)
		})
	}
}

func TestComposeRejectsNonemptyStderrAndForgedManifest(t *testing.T) {
	manifest, err := Decode(strings.NewReader(go127Fixture), fixtureCommitments("/go-cache/ab/abcdef-d"))
	if err != nil {
		t.Fatal(err)
	}
	input := DocumentInput{
		EnvironmentSHA256: digest("environment"), PackagePatterns: []string{"./..."},
		RawStderrSHA256: digest("stderr"), ToolchainID: "go-toolchain:sha256:" + digest("toolchain"),
	}
	_, _, err = Compose(manifest, input)
	assertCode(t, err, CodeInput)
	empty := sha256.Sum256(nil)
	input.RawStderrSHA256 = hex.EncodeToString(empty[:])
	manifest.Packages[0].Name = "forged"
	_, _, err = Compose(manifest, input)
	assertCode(t, err, CodeInput)
}

func TestComposeEmptyInputPackageAndSortedPatterns(t *testing.T) {
	raw := `{"Dir":"/w","ImportPath":"p","Name":"p","Match":["p"]}`
	manifest, err := Decode(strings.NewReader(raw), minimalCommitments())
	if err != nil {
		t.Fatal(err)
	}
	empty := sha256.Sum256(nil)
	document, encoded, err := Compose(manifest, DocumentInput{
		EnvironmentSHA256: digest("environment"), PackagePatterns: []string{"p"},
		RawStderrSHA256: hex.EncodeToString(empty[:]), ToolchainID: "go-toolchain:sha256:" + digest("toolchain"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(document.Argv[5:], ",") != "p" {
		t.Fatalf("patterns = %#v", document.Argv[5:])
	}
	if err := VerifyDocument(document, encoded); err != nil {
		t.Fatal(err)
	}
}

func TestComposeRejectsPatternsOutsideDiscoveryMatchUnion(t *testing.T) {
	manifest, err := Decode(strings.NewReader(go127Fixture), fixtureCommitments("/go-cache/ab/abcdef-d"))
	if err != nil {
		t.Fatal(err)
	}
	empty := sha256.Sum256(nil)
	_, _, err = Compose(manifest, DocumentInput{
		EnvironmentSHA256: digest("environment"), PackagePatterns: []string{"example.test/unrelated"},
		RawStderrSHA256: hex.EncodeToString(empty[:]), ToolchainID: "go-toolchain:sha256:" + digest("toolchain"),
	})
	assertCode(t, err, CodeInput)
}

func TestPackagePatternsNormalizeOrder(t *testing.T) {
	patterns, err := packagePatterns([]string{"z.test/p", "a.test/p"})
	if err != nil || strings.Join(patterns, ",") != "a.test/p,z.test/p" {
		t.Fatalf("patterns = %#v, %v", patterns, err)
	}
}

func TestPackagePatternAggregateBound(t *testing.T) {
	patterns := make([]string, 4_091)
	for index := range patterns {
		patterns[index] = "example.test/" + strings.Repeat("a", 4_070) + fmt.Sprintf("%04d", index)
	}
	_, err := packagePatterns(patterns)
	assertCode(t, err, CodeInput)
}

func TestPackagePatternBoundCountsArgumentTerminators(t *testing.T) {
	fixedBytes := 0
	for _, argument := range []string{"@PINNED_GO@", "list", "-deps", "-test", "-json=" + ClosedFields} {
		fixedBytes += len(argument) + 1
	}
	remaining := MaxArgvBytes - fixedBytes
	patterns := make([]string, 4_091)
	for index := range patterns {
		argumentBytes := remaining / len(patterns)
		if index < remaining%len(patterns) {
			argumentBytes++
		}
		wantLength := argumentBytes - 1
		prefix := fmt.Sprintf("e/%04d/", index)
		patterns[index] = prefix + strings.Repeat("a", wantLength-len(prefix))
	}
	if _, err := packagePatterns(patterns); err != nil {
		t.Fatalf("exact argv bound rejected: %v", err)
	}
	patterns[len(patterns)-1] += "a"
	assertCode(t, func() error { _, err := packagePatterns(patterns); return err }(), CodeInput)
}

func TestAllGo127FileFieldsNormalizeToUniqueInputUnion(t *testing.T) {
	var fields strings.Builder
	commitments := make([]FileCommitment, 0, len(fileFields))
	for index, field := range fileFields {
		if index != 0 {
			fields.WriteByte(',')
		}
		fields.WriteString(`"` + string(field) + `":["same.input"]`)
		commitments = append(commitments, file(field, "same.input", RoleSource, "same.input"))
	}
	raw := `{"Dir":"/w","ImportPath":"p","Name":"p","DepOnly":true,` + fields.String() + `}`
	values := minimalCommitments()
	values.Packages[0].Files = commitments
	manifest, err := Decode(strings.NewReader(raw), values)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Packages) != 1 || len(manifest.Packages[0].InputFiles) != 1 {
		t.Fatalf("input union = %#v", manifest.Packages)
	}
}

func TestModuleReplaceOmissionAndNull(t *testing.T) {
	template := `{"Dir":"/w","ImportPath":"p","Name":"p","Match":["p"],"Module":{"Path":"m","Version":"","Replace":REPLACE}}`
	for _, replacement := range []string{"null", `{"Path":"r","Replace":null}`} {
		raw := strings.Replace(template, "REPLACE", replacement, 1)
		values := minimalCommitments()
		manifest, err := Decode(strings.NewReader(raw), values)
		if err != nil {
			t.Fatal(err)
		}
		module := manifest.Packages[0].Module
		if module == nil || module.Version != nil {
			t.Fatalf("empty module version was not normalized to null: %#v", module)
		}
	}
	nested := strings.Replace(template, "REPLACE", `{"Path":"r","Replace":{"Path":"nested"}}`, 1)
	_, err := Decode(strings.NewReader(nested), minimalCommitments())
	assertCode(t, err, CodeDecode)
	unknown := strings.Replace(template, "REPLACE", `{"Path":"r","Query":"latest"}`, 1)
	_, err = Decode(strings.NewReader(unknown), minimalCommitments())
	assertCode(t, err, CodeDecode)
}

func TestTestMainRequiresOneGeneratedGoFile(t *testing.T) {
	raw := `{"Dir":"/w","ImportPath":"p.test","Name":"main"}`
	values := minimalCommitments()
	values.Packages[0].ImportPath = "p.test"
	_, err := Decode(strings.NewReader(raw), values)
	assertCode(t, err, CodeInput)
	raw = `{"Dir":"/w","ImportPath":"p.test","Name":"main","TestGoFiles":["x_test.go"]}`
	values.Packages[0].Files = []FileCommitment{file(FieldTestGoFiles, "x_test.go", RoleSource, "x_test.go")}
	_, err = Decode(strings.NewReader(raw), values)
	assertCode(t, err, CodeInput)
}

func TestPackageDirectoryMayBeFilesystemRoot(t *testing.T) {
	raw := `{"Dir":"/","ImportPath":"p","Name":"p","Match":["p"]}`
	values := minimalCommitments()
	values.Packages[0].RawDir = "/"
	if _, err := Decode(strings.NewReader(raw), values); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeRejectsPartialAndTrailingJSON(t *testing.T) {
	for _, raw := range []string{
		`{"Dir":"/w","ImportPath":"p","Name":"p","Match":["p"]`,
		`{"Dir":"/w","ImportPath":"p","Name":"p","Match":["p"]} trailing`,
	} {
		_, err := Decode(strings.NewReader(raw), minimalCommitments())
		assertCode(t, err, CodeDecode)
	}
}

func TestMalformedLargeShortStringObjectIsDecodeNotLimit(t *testing.T) {
	raw := `{"Dir":"/w","ImportPath":"p","Name":"p","Match":["p"],"Unknown":[` + strings.Repeat(`"x",`, 300_000) + `null]}`
	if len(raw) <= MaxStringBytes {
		t.Fatal("test vector is not large")
	}
	_, err := Decode(strings.NewReader(raw), minimalCommitments())
	assertCode(t, err, CodeDecode)
}

func TestMalformedLargeUnicodeEscapeStringIsDecodeNotLimit(t *testing.T) {
	raw := `{"Dir":"/w","ImportPath":"p","Name":"` + strings.Repeat(`\uZZZZ`, 180_000) + `","Match":["p"]}`
	if len(raw) <= MaxStringBytes {
		t.Fatal("test vector is not large")
	}
	_, err := Decode(strings.NewReader(raw), minimalCommitments())
	assertCode(t, err, CodeDecode)
}

func TestExactImportPathRejectsEllipsisSegmentOnly(t *testing.T) {
	if !validExactImportPath("example.test/foo...bar") {
		t.Fatal("ellipsis substring was rejected")
	}
	if validExactImportPath("example.test/.../bar") {
		t.Fatal("ellipsis segment was accepted")
	}
	if validExactImportPath("example.test/pkg@v1") {
		t.Fatal("at-sign path was accepted")
	}
}

func fixtureDocument(t *testing.T) (Document, []byte) {
	t.Helper()
	manifest, err := Decode(strings.NewReader(go127Fixture), fixtureCommitments("/go-cache/ab/abcdef-d"))
	if err != nil {
		t.Fatal(err)
	}
	empty := sha256.Sum256(nil)
	document, encoded, err := Compose(manifest, DocumentInput{
		EnvironmentSHA256: digest("environment"), PackagePatterns: []string{"./..."},
		RawStderrSHA256: hex.EncodeToString(empty[:]), ToolchainID: "go-toolchain:sha256:" + digest("toolchain"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return document, encoded
}

func cloneDocument(value Document) Document {
	value.Argv = append([]string(nil), value.Argv...)
	value.Packages = clonePackages(value.Packages)
	value.RequestedRunnerPackages = append([]string(nil), value.RequestedRunnerPackages...)
	return value
}
