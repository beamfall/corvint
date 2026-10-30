package releasegate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func init() { gitContainmentTestOverride = true }

func TestScanRejectsUnexplainedTreeMember(t *testing.T) {
	root := releaseRepository(t, map[string][]byte{"app.py": []byte("#!/usr/bin/env python3\n"), "main.go": []byte("package main\n"), "bundle.zip": zipBytes(t, "nested/fixture.py", []byte("print('archive')\n"))}, nil)
	if _, err := Scan(context.Background(), scanOptions(root)); err == nil {
		t.Fatal("omitted app.py and ZIP were accepted")
	}
}

func TestScanFindsInventoryRemnantsAndArchiveMembers(t *testing.T) {
	root := releaseRepository(t, map[string][]byte{"main.go": []byte("package main\nimport run \"os/exec\"\nfunc main() { _ = run.Command(\"/usr/bin/python3\") }\n"), "bundle.zip": zipBytes(t, "nested/fixture.py", []byte("print('archive')\n"))}, func(m *Manifest) {
		m.ArtifactInventory = append(m.ArtifactInventory, "bundle.zip")
		sort.Strings(m.ArtifactInventory)
	})
	report, err := Scan(context.Background(), scanOptions(root))
	if err != nil {
		t.Fatal(err)
	}
	assertKinds(t, report.Findings, "python-execution", "python-source")
	for _, finding := range report.Findings {
		if finding.Evidence.SpanSHA256 == "" || finding.Evidence.SpanEnd < finding.Evidence.SpanStart {
			t.Fatalf("missing exact span: %+v", finding)
		}
	}
}

func TestCanonicalManifestRejectsAmbiguousJSONAndUnknownFields(t *testing.T) {
	valid := []byte(`{"artifactInventory":["go.mod","main.go"],"buildPackages":["example/release"],"corePackages":["example/release"],"coreSizeCeilingBytes":1,"pluginSizeCeilingBytes":{"example/release":1}}`)
	if _, err := decodeManifest(valid); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range [][]byte{[]byte(`{"artifactInventory":[],"artifactInventory":[]}`), []byte(`{"artifactInventory":[],"buildPackages":[],"corePackages":[],"coreSizeCeilingBytes":1,"pluginSizeCeilingBytes":{},"extra":true}`), append(valid, []byte(" {}")...)} {
		if _, err := decodeManifest(invalid); err == nil {
			t.Fatalf("accepted ambiguous manifest %q", invalid)
		}
	}
}

func TestManifestRequiresExactUnshippedAllowance(t *testing.T) {
	fixture := []byte("print('fixture')\n")
	root := releaseRepository(t, map[string][]byte{"main.go": []byte("package main\n"), "fixtures/allowed.py": fixture}, func(m *Manifest) {
		m.Allowances = []Allowance{{Path: "fixtures/allowed.py", BlobSHA256: sha256Hex(fixture)}}
	})
	report, err := Scan(context.Background(), scanOptions(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("unshipped allowance was scanned: %+v", report.Findings)
	}
	root = releaseRepository(t, map[string][]byte{"main.go": []byte("package main\n"), "fixtures/allowed.py": fixture}, func(m *Manifest) {
		m.ArtifactInventory = append(m.ArtifactInventory, "fixtures/allowed.py")
		sort.Strings(m.ArtifactInventory)
		m.Allowances = []Allowance{{Path: "fixtures/allowed.py", BlobSHA256: sha256Hex(fixture)}}
	})
	if _, err := Scan(context.Background(), scanOptions(root)); err == nil {
		t.Fatal("shipped allowance accepted")
	}
}

func TestManifestChecksumBindsExactArtifactInventory(t *testing.T) {
	root := releaseRepository(t, map[string][]byte{"main.go": []byte("package main\n")}, func(m *Manifest) { m.ArtifactSHA256 = strings.Repeat("0", 64) })
	if _, err := Scan(context.Background(), scanOptions(root)); err == nil {
		t.Fatal("forged artifact inventory checksum accepted")
	}
}

func TestGoExecutionAnalysisFailsClosedOnDynamicAndDetectsShell(t *testing.T) {
	for name, test := range map[string]struct{ source, want string }{"dynamic": {"package main\nimport x \"os/exec\"\nfunc f(v string) { _ = x.Command(v) }\n", "go-analysis-unknown"}, "shell": {"package main\nimport x \"os/exec\"\nfunc f() { _ = x.Command(\"/bin/sh\", \"-c\", \"python3 -V\") }\n", "python-shell-execution"}} {
		t.Run(name, func(t *testing.T) {
			root := releaseRepository(t, map[string][]byte{"main.go": []byte(test.source)}, nil)
			report, err := Scan(context.Background(), scanOptions(root))
			if err != nil {
				t.Fatal(err)
			}
			assertKinds(t, report.Findings, test.want)
		})
	}
	root := releaseRepository(t, map[string][]byte{"main.go": []byte("package main\nimport x \"os/exec\"\nfunc f() { _ = x.Command(\"C:/Windows/System32/cmd.exe\", \"/c\", \"py.exe -3\") }\n")}, nil)
	report, err := Scan(context.Background(), scanOptions(root))
	if err != nil {
		t.Fatal(err)
	}
	assertKinds(t, report.Findings, "python-shell-execution")
}

func TestArchiveRejectsTraversalDuplicatesAndSpecialEntries(t *testing.T) {
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, name := range []string{"../escape.py", "same.py", "same.py", "Case.py", "case.py"} {
		member, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := member.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	root := releaseRepository(t, map[string][]byte{"main.go": []byte("package main\n"), "bundle.zip": output.Bytes()}, func(m *Manifest) {
		m.ArtifactInventory = append(m.ArtifactInventory, "bundle.zip")
		sort.Strings(m.ArtifactInventory)
	})
	report, err := Scan(context.Background(), scanOptions(root))
	if err != nil {
		t.Fatal(err)
	}
	assertKinds(t, report.Findings, "unsafe-archive-member", "duplicate-archive-member", "casefold-archive-member")
}

func TestTarArchiveRejectsTraversalAndSymlink(t *testing.T) {
	members, err := TarArchive{}.Members(tarBytes(t))
	if err == nil || members != nil {
		t.Fatalf("unsafe tar accepted: %+v, %v", members, err)
	}
}

func TestArchiveSniffingAndNestedScanFailClosed(t *testing.T) {
	parent := Evidence{Path: "bundle.zip", BlobOID: "oid", BlobSHA256: sha256Hex([]byte("x"))}
	findings, err := scanArchive("bundle.zip", parent, tarBytes(t))
	if err != nil {
		t.Fatal(err)
	}
	assertKinds(t, findings, "archive-malformed")
	inner := zipBytes(t, "nested.py", []byte("#!/usr/bin/env python3\n"))
	outer := zipBytes(t, "nested.zip", inner)
	findings, err = scanArchive("bundle.zip", parent, outer)
	if err != nil {
		t.Fatal(err)
	}
	assertKinds(t, findings, "python-source")
	if safePath("C:/escape.py") || safePath("\\\\server\\share\\escape.py") {
		t.Fatal("Windows path accepted")
	}
}

func TestNestedArchiveEvidenceKeepsEnclosingMemberPaths(t *testing.T) {
	parent := Evidence{Path: "bundle.zip", BlobOID: "oid", BlobSHA256: sha256Hex([]byte("x"))}
	middle := zipBytes(t, "inner.zip", zipBytes(t, "nested.py", []byte("#!/usr/bin/env python3\n")))
	findings, err := scanArchive("bundle.zip", parent, zipBytes(t, "middle.zip", middle))
	if err != nil {
		t.Fatal(err)
	}
	assertKinds(t, findings, "python-source")
	for _, finding := range findings {
		if finding.Evidence.MemberPath != "nested.py" || strings.Join(finding.Evidence.EnclosingMembers, "|") != "middle.zip|inner.zip" {
			t.Fatalf("nested evidence lost its enclosing member paths: %+v", finding.Evidence)
		}
	}
}

func TestArchiveSniffsExtensionlessMembersAndRequiresExactTAREnd(t *testing.T) {
	inner := zipBytes(t, "payload.py", []byte("#!/usr/bin/env python3\n"))
	root := releaseRepository(t, map[string][]byte{"main.go": []byte("package main\n"), "opaque.bin": inner}, func(m *Manifest) {
		m.ArtifactInventory = append(m.ArtifactInventory, "opaque.bin")
		sort.Strings(m.ArtifactInventory)
	})
	report, err := Scan(context.Background(), scanOptions(root))
	if err != nil {
		t.Fatal(err)
	}
	assertKinds(t, report.Findings, "python-source")
	var regular bytes.Buffer
	writer := tar.NewWriter(&regular)
	if err := writer.WriteHeader(&tar.Header{Name: "safe.txt", Mode: 0o644, Size: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if !validTARContainer(regular.Bytes()) {
		t.Fatal("valid tar rejected")
	}
	if validTARContainer(append(regular.Bytes(), make([]byte, 512)...)) {
		t.Fatal("tar with extra terminal block accepted")
	}
}

func TestArchiveRejectsHiddenPrefixConcatenationAndPaddingAtEveryLevel(t *testing.T) {
	parent := Evidence{Path: "opaque.bin", BlobOID: "oid", BlobSHA256: sha256Hex([]byte("x"))}
	zipData := zipBytes(t, "payload.py", []byte("print('x')\n"))
	var tarData bytes.Buffer
	tarWriter := tar.NewWriter(&tarData)
	if err := tarWriter.WriteHeader(&tar.Header{Name: "payload.py", Mode: 0o644, Size: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	var gzipData bytes.Buffer
	gzipWriter := gzip.NewWriter(&gzipData)
	if _, err := gzipWriter.Write(tarData.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"opaque.bin": zipData, "opaque.tar": tarData.Bytes(), "opaque.gz": gzipData.Bytes()} {
		for _, hostile := range [][]byte{append([]byte("x"), data...), append(append([]byte(nil), data...), data...), append(append([]byte(nil), data...), 0)} {
			findings, err := scanArchive(name, parent, hostile)
			if err != nil {
				t.Fatal(err)
			}
			assertKinds(t, findings, "archive-malformed")
		}
	}
	outer := zipBytes(t, "nested.bin", append([]byte("x"), zipData...))
	findings, err := scanArchive("bundle.zip", parent, outer)
	if err != nil {
		t.Fatal(err)
	}
	assertKinds(t, findings, "archive-malformed")
}

func TestArchiveClassifiesWholeV7TarAndRejectsHiddenV7Header(t *testing.T) {
	var output bytes.Buffer
	writer := tar.NewWriter(&output)
	payload := []byte("#!/usr/bin/env python3\n")
	if err := writer.WriteHeader(&tar.Header{Name: "payload.py", Mode: 0o644, Size: int64(len(payload))}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	v7 := append([]byte(nil), output.Bytes()...)
	for index := 257; index < 263; index++ {
		v7[index] = 0
	}
	for index := 148; index < 156; index++ {
		v7[index] = ' '
	}
	var checksum int64
	for _, value := range v7[:512] {
		checksum += int64(value)
	}
	field := []byte(strconv.FormatInt(checksum, 8))
	copy(v7[148:154], []byte("000000"))
	copy(v7[148+6-len(field):154], field)
	v7[154], v7[155] = 0, ' '
	parent := Evidence{Path: "opaque.bin", BlobOID: "oid", BlobSHA256: sha256Hex(v7)}
	findings, err := scanArchive("opaque.bin", parent, v7)
	if err != nil {
		t.Fatal(err)
	}
	assertKinds(t, findings, "python-source")
	hidden := append(bytes.Repeat([]byte{'x'}, 5000), v7...)
	findings, err = scanArchive("opaque.bin", parent, hidden)
	if err != nil {
		t.Fatal(err)
	}
	assertKinds(t, findings, "archive-malformed")
}

func TestPackageExecAnalysisHandlesAliasesDotImportsWindowsAndBuildTags(t *testing.T) {
	root := releaseRepository(t, map[string][]byte{
		"main.go":         []byte("package main\nimport . \"os/exec\"\nfunc main() { Command(\"C:\\\\Python311\\\\python.exe\") }\n"),
		"alias.go":        []byte("package main\nimport x \"os/exec\"\nvar launch = x.Command\nfunc alias() { launch(\"python3\") }\n"),
		"main_windows.go": []byte("package main\nimport x \"os/exec\"\nfunc windowsOnly() { x.Command(\"python3\") }\n"),
	}, func(m *Manifest) {
		m.ArtifactInventory = append(m.ArtifactInventory, "alias.go", "main_windows.go")
		sort.Strings(m.ArtifactInventory)
	})
	report, err := Scan(context.Background(), scanOptions(root))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, finding := range report.Findings {
		if finding.Kind == "python-execution" {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("python execution findings=%d: %+v", count, report.Findings)
	}
}

func TestPackageExecAnalysisDoesNotFlagShadowedSelectorOrMissCgo(t *testing.T) {
	root := releaseRepository(t, map[string][]byte{
		"main.go": []byte("package main\nimport \"C\"\ntype fake struct{}\nfunc (fake) Command(string) {}\nfunc main() { x := fake{}; x.Command(\"python3\") }\n"),
	}, nil)
	report, err := Scan(context.Background(), scanOptions(root))
	if err != nil {
		t.Fatal(err)
	}
	assertKinds(t, report.Findings, "cgo-forbidden")
	for _, finding := range report.Findings {
		if finding.Kind == "python-execution" {
			t.Fatalf("shadowed selector was treated as os/exec: %+v", finding)
		}
	}
}

func TestPackageExecAnalysisUsesFixedPointAliasesAndCompilerTags(t *testing.T) {
	root := releaseRepository(t, map[string][]byte{
		"main.go":     []byte("package main\n"),
		"a.go":        []byte("package main\nvar launch2 = launch\nfunc f(){ launch2(\"python3\") }\n"),
		"z.go":        []byte("package main\nimport x \"os/exec\"\nvar launch = x.Command\n"),
		"compiler.go": []byte("//go:build gc\npackage main\nimport x \"os/exec\"\nfunc g(){ x.Command(\"python3\") }\n"),
	}, func(m *Manifest) {
		m.ArtifactInventory = append(m.ArtifactInventory, "a.go", "compiler.go", "z.go")
		sort.Strings(m.ArtifactInventory)
	})
	report, err := Scan(context.Background(), scanOptions(root))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, finding := range report.Findings {
		if finding.Kind == "python-execution" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("fixed-point/compiler findings=%d: %+v", count, report.Findings)
	}
}

func TestGoAnalysisRejectsNetworkSyscallAndEscapedExecValues(t *testing.T) {
	root := releaseRepository(t, map[string][]byte{"main.go": []byte(`package main
import (
  "net/http"
  "syscall"
  x "os/exec"
)
var launch any = x.Command
func main() { _, _ = http.Get("https://example.invalid"); _ = syscall.Getpid(); launch.(func(string, ...string) *x.Cmd)("python3") }
`)}, nil)
	report, err := Scan(context.Background(), scanOptions(root))
	if err != nil {
		t.Fatal(err)
	}
	assertKinds(t, report.Findings, "network-capability-forbidden", "syscall-capability-forbidden", "go-analysis-unknown")
}

func TestGoAnalysisRejectsTransitiveNetworkAndLinknameCapabilities(t *testing.T) {
	for name, source := range map[string]string{
		"tls": `package main
import "crypto/tls"
func f(){ _, _ = tls.Dial("tcp", "example.invalid:443", nil) }`,
		"httptest": `package main
import "net/http/httptest"
func f(){ _ = httptest.NewServer(nil) }`,
		"linkname": `package main
import _ "unsafe"
//go:linkname launch os/exec.Command
func launch(string, ...string) any
`,
	} {
		t.Run(name, func(t *testing.T) {
			root := releaseRepository(t, map[string][]byte{"main.go": []byte(source)}, nil)
			report, err := Scan(context.Background(), scanOptions(root))
			if err != nil {
				t.Fatal(err)
			}
			if name == "linkname" {
				assertKinds(t, report.Findings, "linkname-forbidden")
			} else {
				assertKinds(t, report.Findings, "network-capability-forbidden")
			}
		})
	}
}

func TestPluginCeilingIncludesPerTargetDependencyClosure(t *testing.T) {
	heavy := []byte("package heavy\nvar Payload = \"" + strings.Repeat("x", 4096) + "\"\n")
	root := releaseRepository(t, map[string][]byte{
		"main.go":               []byte("package main\n"),
		"plugin/plugin.go":      []byte("package plugin\nimport _ \"example/release/plugin/heavy\"\n"),
		"plugin/heavy/heavy.go": heavy,
	}, func(m *Manifest) {
		m.ArtifactInventory = append(m.ArtifactInventory, "plugin/heavy/heavy.go", "plugin/plugin.go")
		sort.Strings(m.ArtifactInventory)
		m.PluginPackages = []string{"example/release/plugin", "example/release/plugin/heavy"}
		m.PluginSizeCeilings = map[string]int64{"example/release/plugin": 256, "example/release/plugin/heavy": 8192}
		m.PluginArtifacts = map[string][]string{"example/release/plugin": {}, "example/release/plugin/heavy": {}}
	})
	report, err := Scan(context.Background(), scanOptions(root))
	if err != nil {
		t.Fatal(err)
	}
	assertKinds(t, report.Findings, "plugin-size-ceiling")
}

func TestPluginBinaryRequiresExactBindingAndCountsAgainstCeiling(t *testing.T) {
	files := map[string][]byte{"main.go": []byte("package main\n"), "plugin/plugin.go": []byte("package plugin\n"), "dist/plugin.bin": bytes.Repeat([]byte{'x'}, 4096)}
	configure := func(bound bool) func(*Manifest) {
		return func(m *Manifest) {
			m.ArtifactInventory = append(m.ArtifactInventory, "dist/plugin.bin", "plugin/plugin.go")
			sort.Strings(m.ArtifactInventory)
			m.PluginPackages = []string{"example/release/plugin"}
			m.PluginSizeCeilings = map[string]int64{"example/release/plugin": 256}
			m.PluginArtifacts = map[string][]string{"example/release/plugin": {}}
			if bound {
				m.PluginArtifacts["example/release/plugin"] = []string{"dist/plugin.bin"}
			}
		}
	}
	if _, err := Scan(context.Background(), scanOptions(releaseRepository(t, files, configure(false)))); err == nil {
		t.Fatal("unbound shipped plugin binary accepted")
	}
	report, err := Scan(context.Background(), scanOptions(releaseRepository(t, files, configure(true))))
	if err != nil {
		t.Fatal(err)
	}
	assertKinds(t, report.Findings, "plugin-size-ceiling")
}

func TestPluginCeilingCountsShippedGoShapedTestSource(t *testing.T) {
	payload := []byte("package plugin\n// " + strings.Repeat("x", 4096) + "\n")
	root := releaseRepository(t, map[string][]byte{"main.go": []byte("package main\n"), "plugin/plugin.go": []byte("package plugin\n"), "plugin/payload_test.go": payload}, func(m *Manifest) {
		m.ArtifactInventory = append(m.ArtifactInventory, "plugin/payload_test.go", "plugin/plugin.go")
		sort.Strings(m.ArtifactInventory)
		m.PluginPackages = []string{"example/release/plugin"}
		m.PluginSizeCeilings = map[string]int64{"example/release/plugin": 256}
		m.PluginArtifacts = map[string][]string{"example/release/plugin": {}}
	})
	report, err := Scan(context.Background(), scanOptions(root))
	if err != nil {
		t.Fatal(err)
	}
	assertKinds(t, report.Findings, "plugin-size-ceiling")
}

func TestManualCmdProcessMethodsFailClosed(t *testing.T) {
	root := releaseRepository(t, map[string][]byte{"main.go": []byte("package main\nimport x \"os/exec\"\nfunc main(){ c := &x.Cmd{Path: \"python3\"}; c.Run(); c.Start() }\n")}, nil)
	report, err := Scan(context.Background(), scanOptions(root))
	if err != nil {
		t.Fatal(err)
	}
	assertKinds(t, report.Findings, "process-execution-forbidden")
}

func TestCmdInterfaceAndReflectionReachabilityFailClosed(t *testing.T) {
	for name, source := range map[string]string{
		"interface": `package main
import x "os/exec"
type runner interface { Run() error }
func main(){ var r runner = &x.Cmd{Path:"python3"}; r.Run() }`,
		"reflection": `package main
import ("reflect"; x "os/exec")
func main(){ c:=&x.Cmd{Path:"python3"}; reflect.ValueOf(c).MethodByName("Run").Call(nil) }`,
	} {
		t.Run(name, func(t *testing.T) {
			root := releaseRepository(t, map[string][]byte{"main.go": []byte(source)}, nil)
			report, err := Scan(context.Background(), scanOptions(root))
			if err != nil {
				t.Fatal(err)
			}
			assertKinds(t, report.Findings, "process-execution-forbidden")
			if report.Checks.Safety != Fail {
				t.Fatalf("Safety=%s, want FAIL", report.Checks.Safety)
			}
		})
	}
}

func TestManifestRejectsFutureReleaseTag(t *testing.T) {
	targets := supportedTestTargets()
	targets[0].ReleaseTags = append(targets[0].ReleaseTags, "go1.99")
	m := Manifest{ArtifactInventory: []string{"go.mod", "main.go"}, ArtifactModes: map[string]string{"go.mod": "100644", "main.go": "100644"}, ArtifactSHA256: strings.Repeat("0", 64), BuildPackages: []string{"example/release"}, BuildTargets: targets, CorePackages: []string{"example/release"}, CoreSizeCeiling: 1, PluginSizeCeilings: map[string]int64{}, PluginArtifacts: map[string][]string{}, AnalyzerSizeCeilings: map[string]int64{}}
	entries := map[string]treeEntry{"go.mod": {path: "go.mod", mode: "100644"}, "main.go": {path: "main.go", mode: "100644"}}
	if err := validateManifest(m, entries, "release-gate-manifest.json"); err == nil {
		t.Fatal("future Go release tag accepted")
	}
}

func TestPluginMissingSameModuleDependencyFailsClosed(t *testing.T) {
	root := releaseRepository(t, map[string][]byte{"main.go": []byte("package main\n"), "plugin/plugin.go": []byte("package plugin\nimport _ \"example/release/plugin/missing\"\n")}, func(m *Manifest) {
		m.ArtifactInventory = append(m.ArtifactInventory, "plugin/plugin.go")
		sort.Strings(m.ArtifactInventory)
		m.PluginPackages = []string{"example/release/plugin"}
		m.PluginSizeCeilings = map[string]int64{"example/release/plugin": 1024}
		m.PluginArtifacts = map[string][]string{"example/release/plugin": {}}
	})
	if _, err := Scan(context.Background(), scanOptions(root)); err == nil {
		t.Fatal("missing same-module plugin dependency accepted")
	}
}

func TestScanRejectsUnpinnedOrDifferentExternalPolicy(t *testing.T) {
	root := releaseRepository(t, map[string][]byte{"main.go": []byte("package main\n")}, nil)
	options := scanOptions(root)
	options.PolicySHA256 = strings.Repeat("0", 64)
	if _, err := Scan(context.Background(), options); err == nil {
		t.Fatal("forged external policy digest accepted")
	}
	options = scanOptions(root)
	if err := os.WriteFile(options.PolicyPath, []byte(`{"artifactInventory":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	options.PolicySHA256 = sha256Hex([]byte(`{"artifactInventory":[]}`))
	if _, err := Scan(context.Background(), options); err == nil {
		t.Fatal("different external policy accepted")
	}
}

func TestPolicyPinsArtifactModes(t *testing.T) {
	root := releaseRepository(t, map[string][]byte{"main.go": []byte("package main\n")}, func(m *Manifest) {
		m.ArtifactModes = map[string]string{"go.mod": "100644", "main.go": "100755"}
	})
	if _, err := Scan(context.Background(), scanOptions(root)); err == nil {
		t.Fatal("policy accepted a mode different from the committed artifact")
	}
}

func TestUnsupportedGitReportIsStructuredAndBlocking(t *testing.T) {
	report := unsupportedGitReport()
	if report.Checks != (Checks{Parity: NotRun, Safety: Unsupported, Performance: NotRun, Packaging: NotRun, CorvintDogfood: NotRun, BeamfallDogfood: NotRun}) || !report.Blocking() || len(report.Findings) != 1 || report.Findings[0].Kind != "git-process-containment-unsupported" {
		t.Fatalf("unexpected unsupported report: %+v", report)
	}
}

func BenchmarkArchiveCandidate(b *testing.B) {
	content := hostileTARChecksumCandidateBytes(1 << 20)
	_, reads := archiveCandidateWork(content)
	b.SetBytes(int64(len(content)))
	b.ReportAllocs()
	// MB/s is diagnostic-only. The non-wall-clock work/allocation ratchet is
	// enforced by TestArchiveCandidateLinearAllocationRatchet.
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		_ = archiveCandidate(content)
	}
	b.ReportMetric(float64(reads)/float64(len(content)), "checksum_bytes/B")
}

func BenchmarkArchiveCandidateAllZero(b *testing.B) {
	content := bytes.Repeat([]byte{'0'}, 1<<20)
	_, reads := archiveCandidateWork(content)
	b.SetBytes(int64(len(content)))
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		_ = archiveCandidate(content)
	}
	b.ReportMetric(float64(reads)/float64(len(content)), "checksum_bytes/B")
}

func TestArchiveCandidateLinearAllocationRatchet(t *testing.T) {
	fixtures := map[string][]byte{
		"all-zero": bytes.Repeat([]byte{'0'}, 1<<20),
		"10000000": hostileTARChecksumCandidateBytes(1 << 20),
	}
	for name, content := range fixtures {
		t.Run(name, func(t *testing.T) {
			candidate, reads := archiveCandidateWork(content)
			if candidate {
				t.Fatal("hostile checksum field was accepted")
			}
			if bound := archiveCandidateReadBound(len(content)); reads > bound {
				t.Fatalf("checksum reads=%d exceeds owner-reviewed bound=%d", reads, bound)
			}
			if allocations := testing.AllocsPerRun(20, func() { _ = archiveCandidate(content) }); allocations != 0 {
				t.Fatalf("tar candidate allocations=%g, want 0", allocations)
			}
		})
	}
}

func TestArchiveCandidateRatchetRejectsWindowRecomputation(t *testing.T) {
	content := hostileTARChecksumCandidateBytes(4096)
	reads := 0
	if restoredFullWindowArchiveCandidate(countingTARCandidateSource{content: content, reads: &reads}) {
		t.Fatal("restored full-window classifier accepted hostile input")
	}
	if bound := archiveCandidateReadBound(len(content)); reads <= bound {
		t.Fatalf("restored full-window checksum reads=%d, want > %d", reads, bound)
	}
}

func TestTarChecksumFieldReadCountingIncludesPlausibilityAndParse(t *testing.T) {
	reads := 0
	source := countingTARCandidateSource{content: []byte("10000000"), reads: &reads}
	if !tarChecksumFieldPlausible(source, 0) {
		t.Fatal("maximal plausible checksum field rejected")
	}
	_ = rollingTARChecksumMatches(source, 0, 0, 0)
	if reads != tarCandidateMaxChecksumFieldReads {
		t.Fatalf("checksum field reads=%d, want %d", reads, tarCandidateMaxChecksumFieldReads)
	}
}

var archiveCandidateProductionFunctions = []string{
	"archiveCandidate",
	"tarHeaderCandidate",
	"rollingTARChecksumMatchesDirect",
	"tarChecksumFieldPlausibleDirect",
	"tarCandidateHeaderSizeDirect",
}

const archiveCandidateProductionDigest = "c8509b9c17d7a617547ab2c67503cba4e1f406294fba0a7dd3ed210ce24d5852"

func TestArchiveCandidateProductionClosureDigestRatchet(t *testing.T) {
	source, err := os.ReadFile("archive.go")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := archiveCandidateClosureDigest(source); err != nil {
		t.Fatal(err)
	} else if got != archiveCandidateProductionDigest {
		t.Fatalf("production archive classifier closure digest=%s, want %s", got, archiveCandidateProductionDigest)
	}
}

func TestArchiveCandidateClosureDigestRejectsRangeSliceHelperBypass(t *testing.T) {
	source, err := os.ReadFile("archive.go")
	if err != nil {
		t.Fatal(err)
	}
	needle := []byte("func archiveCandidate(content []byte) bool {\n")
	bypass := []byte("func archiveCandidate(content []byte) bool {\n\t_ = archiveCandidateRangeSliceBypass(content)\n")
	hostile := bytes.Replace(source, needle, bypass, 1)
	hostile = append(hostile, []byte("\nfunc archiveCandidateRangeSliceBypass(content []byte) bool {\n\tfor offset := range content {\n\t\tif offset+512 > len(content) {\n\t\t\tbreak\n\t\t}\n\t\tfor range content[offset : offset+512] {\n\t\t}\n\t}\n\treturn false\n}\n")...)
	if got, err := archiveCandidateClosureDigest(hostile); err != nil {
		t.Fatal(err)
	} else if got == archiveCandidateProductionDigest {
		t.Fatal("range/slice helper bypass did not invalidate the archive classifier closure digest")
	}
}

func archiveCandidateClosureDigest(source []byte) (string, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "archive.go", source, 0)
	if err != nil {
		return "", err
	}
	functions := map[string]*ast.FuncDecl{}
	for _, declaration := range file.Decls {
		if function, ok := declaration.(*ast.FuncDecl); ok && function.Recv == nil {
			functions[function.Name.Name] = function
		}
	}
	hasher := sha256.New()
	for _, name := range archiveCandidateProductionFunctions {
		function := functions[name]
		if function == nil {
			return "", fmt.Errorf("production archive classifier closure misses %q", name)
		}
		var canonical bytes.Buffer
		if err := format.Node(&canonical, fileSet, function); err != nil {
			return "", err
		}
		hasher.Write([]byte(name))
		hasher.Write([]byte{0})
		hasher.Write(canonical.Bytes())
		hasher.Write([]byte{0})
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func TestArchiveCandidateProductionMatchesCountedRandomized(t *testing.T) {
	content := make([]byte, 519)
	state := uint64(0x9e3779b97f4a7c15)
	for trial := 0; trial < 100000; trial++ {
		length := 512 + int(nextTARCandidateRandom(&state)&7)
		for index := 0; index < length; index++ {
			content[index] = byte(nextTARCandidateRandom(&state) >> 56)
		}
		switch trial % 17 {
		case 0:
			for index := 0; index < length; index++ {
				content[index] = '0'
			}
		case 1:
			for index := 0; index < length; index++ {
				content[index] = "10000000"[index%8]
			}
		}
		want, reads := archiveCandidateWork(content[:length])
		if got := archiveCandidate(content[:length]); got != want {
			t.Fatalf("trial=%d production=%v counted=%v", trial, got, want)
		}
		if bound := archiveCandidateReadBound(length); reads > bound {
			t.Fatalf("trial=%d checksum reads=%d exceeds bound=%d", trial, reads, bound)
		}
	}
}

func nextTARCandidateRandom(state *uint64) uint64 {
	*state ^= *state << 7
	*state ^= *state >> 9
	*state ^= *state << 8
	return *state
}

func hostileTARChecksumCandidateBytes(length int) []byte {
	content := bytes.Repeat([]byte("10000000"), (length+7)/8)
	return content[:length]
}

func restoredFullWindowArchiveCandidate[S tarChecksumByteSource](source S) bool {
	return archiveMarkerCandidate(source) || restoredFullWindowTARHeaderCandidate(source)
}

func archiveMarkerCandidate[S tarChecksumByteSource](source S) bool {
	return tarCandidateContains(source, "PK\x03\x04") ||
		tarCandidateContains(source, "PK\x05\x06") ||
		tarCandidateContains(source, "\x1f\x8b") ||
		tarCandidateContains(source, "ustar")
}

func restoredFullWindowTARHeaderCandidate[S tarChecksumByteSource](source S) bool {
	if source.len() < 512 {
		return false
	}
	for offset := 0; offset+512 <= source.len(); offset++ {
		var unsigned, signed int64
		for index := 0; index < 512; index++ {
			value := source.byteAt(offset + index)
			unsigned += int64(value)
			signed += int64(int8(value))
		}
		fieldOffset := offset + 148
		if tarChecksumFieldPlausible(source, fieldOffset) && rollingTARChecksumMatches(source, fieldOffset, unsigned, signed) {
			return true
		}
	}
	return false
}

func TestArchiveCandidateRollingEquivalence(t *testing.T) {
	var output bytes.Buffer
	writer := tar.NewWriter(&output)
	if err := writer.WriteHeader(&tar.Header{Name: "payload", Mode: 0o644, Size: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	for _, content := range [][]byte{bytes.Repeat([]byte{'0'}, 1<<20), output.Bytes(), append(bytes.Repeat([]byte{'x'}, 777), output.Bytes()...), append(append([]byte(nil), output.Bytes()...), 0)} {
		want := false
		for offset := 0; offset+512 <= len(content); offset++ {
			if validTARHeaderChecksum(content[offset : offset+512]) {
				want = true
				break
			}
		}
		want = bytes.Contains(content, []byte("PK\x03\x04")) ||
			bytes.Contains(content, []byte("PK\x05\x06")) ||
			bytes.Contains(content, []byte{0x1f, 0x8b}) ||
			bytes.Contains(content, []byte("ustar")) || want
		if got := archiveCandidate(content); got != want {
			t.Fatalf("candidate mismatch: got=%v want=%v len=%d", got, want, len(content))
		}
		if got, _ := archiveCandidateWork(content); got != want {
			t.Fatalf("counted candidate mismatch: got=%v want=%v len=%d", got, want, len(content))
		}
	}
}

func TestUnixProductionFailsClosedWithoutDeterministicDescendantContainment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows is independently unsupported")
	}
	previous := gitContainmentTestOverride
	gitContainmentTestOverride = false
	defer func() { gitContainmentTestOverride = previous }()
	root := releaseRepository(t, map[string][]byte{"main.go": []byte("package main\n")}, nil)
	report, err := Scan(context.Background(), scanOptions(root))
	if err != nil {
		t.Fatal(err)
	}
	if report.Checks.Safety != Unsupported || len(report.Findings) != 1 || report.Findings[0].Kind != "git-process-containment-unsupported" {
		t.Fatalf("unexpected fail-closed report: %+v", report)
	}
}

func TestRunGitReapsProcessGroupOnCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process groups are Unix-specific")
	}
	directory := t.TempDir()
	pidFile := filepath.Join(directory, "child.pid")
	script := filepath.Join(directory, "git")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 30 &\necho $! > \""+pidFile+"\"\nwait\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	scriptBytes, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}
	identity := gitIdentity{path: script, sha256: sha256Hex(scriptBytes)}
	// Cancel only after the fake git has forked and written child.pid; a fixed
	// deadline can reap the parent before the fork under heavy host load, and
	// then no descendant exists to prove reaping against.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pidCh := make(chan int, 1)
	go func() {
		pid := readReleasegatePIDWithRetry(t, pidFile)
		pidCh <- pid
		cancel()
	}()
	_, err = runGitAt(ctx, identity, directory, 1024, "rev-parse", "HEAD")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	pid := <-pidCh
	defer killProcess(pid)
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if processGone(pid) {
			return
		}
	}
	t.Fatalf("descendant %d survived cancellation after 30s", pid)
}

func readReleasegatePIDWithRetry(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		data, readErr := os.ReadFile(path)
		if readErr == nil {
			if pid, convErr := strconv.Atoi(strings.TrimSpace(string(data))); convErr == nil {
				return pid
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("pid file %s did not contain a parseable pid within 30s", path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRunGitReapsProcessGroupAfterSuccessfulParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process groups are Unix-specific")
	}
	directory := t.TempDir()
	pidFile := filepath.Join(directory, "child.pid")
	script := filepath.Join(directory, "git")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 30 &\necho $! > \""+pidFile+"\"\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	scriptBytes, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}
	identity := gitIdentity{path: script, sha256: sha256Hex(scriptBytes)}
	if _, err := runGitAt(context.Background(), identity, directory, 1024, "rev-parse", "HEAD"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	defer killProcess(pid)
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if processGone(pid) {
			return
		}
	}
	t.Fatalf("success-path descendant %d survived", pid)
}

func assertKinds(t *testing.T, findings []Finding, expected ...string) {
	t.Helper()
	got := map[string]bool{}
	for _, finding := range findings {
		got[finding.Kind] = true
	}
	for _, kind := range expected {
		if !got[kind] {
			t.Fatalf("missing %s in %+v", kind, findings)
		}
	}
}
func scanOptions(root string) Options {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		panic(err)
	}
	gitPath, err = filepath.EvalSymlinks(gitPath)
	if err != nil {
		panic(err)
	}
	gitBytes, err := os.ReadFile(gitPath)
	if err != nil {
		panic(err)
	}
	policyPath := root + ".release-gate-policy.json"
	policy, err := os.ReadFile(policyPath)
	if err != nil {
		panic(err)
	}
	return Options{Root: root, Commit: head(root), ManifestPath: "release-gate-manifest.json", GitExecutable: gitPath, GitSHA256: sha256Hex(gitBytes), PolicyPath: policyPath, PolicySHA256: sha256Hex(policy)}
}
func releaseRepository(t *testing.T, files map[string][]byte, configure func(*Manifest)) string {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-q")
	git(t, root, "config", "user.email", "test@example.invalid")
	git(t, root, "config", "user.name", "release gate test")
	files["go.mod"] = []byte("module example/release\n\ngo 1.27.0\n")
	for name, data := range files {
		filename := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := Manifest{ArtifactInventory: []string{"go.mod", "main.go"}, BuildPackages: []string{"example/release"}, BuildTargets: supportedTestTargets(), CorePackages: []string{"example/release"}, CoreSizeCeiling: 1 << 20, PluginSizeCeilings: map[string]int64{}}
	if configure != nil {
		configure(&m)
	}
	entries, blobs := map[string]treeEntry{}, map[string][]byte{}
	if m.ArtifactModes == nil {
		m.ArtifactModes = map[string]string{}
	}
	for _, name := range m.ArtifactInventory {
		entries[name] = treeEntry{oid: name, size: int64(len(files[name])), path: name}
		blobs[name] = files[name]
		if _, pinned := m.ArtifactModes[name]; !pinned {
			m.ArtifactModes[name] = "100644"
		}
	}
	if m.ArtifactSHA256 == "" {
		checksum, err := artifactInventorySHA256(m.ArtifactInventory, entries, blobs)
		if err != nil {
			t.Fatal(err)
		}
		m.ArtifactSHA256 = checksum
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "release-gate-manifest.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root+".release-gate-policy.json", data, 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "fixture")
	return root
}

func supportedTestTargets() []BuildTarget {
	return []BuildTarget{
		{GOOS: "darwin", GOARCH: "amd64", Compiler: "gc", ReleaseTags: frozenReleaseTags()},
		{GOOS: "darwin", GOARCH: "arm64", Compiler: "gc", ReleaseTags: frozenReleaseTags()},
		{GOOS: "linux", GOARCH: "amd64", Compiler: "gc", ReleaseTags: frozenReleaseTags()},
		{GOOS: "linux", GOARCH: "arm64", Compiler: "gc", ReleaseTags: frozenReleaseTags()},
		{GOOS: "windows", GOARCH: "amd64", Compiler: "gc", ReleaseTags: frozenReleaseTags()},
	}
}
func git(t *testing.T, root string, arguments ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, arguments...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", arguments, err, output)
	}
}
func head(root string) string {
	command := exec.Command("git", "-C", root, "rev-parse", "HEAD")
	output, err := command.Output()
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(string(output))
}
func zipBytes(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	member, err := writer.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := member.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
func tarBytes(t *testing.T) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := tar.NewWriter(&output)
	if err := writer.WriteHeader(&tar.Header{Name: "../escape.py", Mode: 0o644, Size: 1, Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteHeader(&tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "../escape.py"}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
