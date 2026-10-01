package affected_test

import (
	"errors"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

// DLT-V0-002: all registered providers observe the same units and selections.
func TestImmutableAllLanguageParity(t *testing.T) {
	t.Run("DLT-V0-002 eight provider immutable parity", testImmutableAllLanguageParity)
}
func testImmutableAllLanguageParity(t *testing.T) {
	files := fstest.MapFS{
		"go.mod":                     {Data: []byte("module example.invalid/demo\n\ngo 1.27\n")},
		"lib.go":                     {Data: []byte("package demo\nfunc A() {}\n")},
		"lib_test.go":                {Data: []byte("package demo\nimport \"testing\"\nfunc TestA(t *testing.T) { A() }\n")},
		"py/lib.py":                  {Data: []byte("def a():\n    return 1\n")},
		"py/test_lib.py":             {Data: []byte("from lib import a\ndef test_a():\n    assert a() == 1\n")},
		"js/a.ts":                    {Data: []byte("export function a() { return 1; }\n")},
		"js/a.test.ts":               {Data: []byte("import { a } from './a';\ntest('a', () => a());\n")},
		"rb/lib/a.rb":                {Data: []byte("class A; end\n")},
		"rb/test/a_test.rb":          {Data: []byte("require_relative '../lib/a'\n")},
		"rs/Cargo.toml":              {Data: []byte("[package]\nname = \"demo\"\nversion = \"0.1.0\"\n")},
		"rs/src/lib.rs":              {Data: []byte("pub fn a() {}\n#[test] fn check() { a(); }\n")},
		"kt/build.gradle.kts":        {Data: []byte("plugins { kotlin(\"jvm\") version \"2.0.0\" }\n")},
		"kt/src/main/kotlin/A.kt":    {Data: []byte("package demo\nfun a() = 1\n")},
		"cs/Demo.csproj":             {Data: []byte("<Project Sdk=\"Microsoft.NET.Sdk\"><PropertyGroup><TargetFramework>net8.0</TargetFramework></PropertyGroup></Project>")},
		"cs/A.cs":                    {Data: []byte("namespace Demo; public class A {}\n")},
		"sw/Package.swift":           {Data: []byte("import PackageDescription\nlet package = Package(name: \"Demo\", targets: [.target(name: \"Demo\"), .testTarget(name: \"DemoTests\", dependencies: [\"Demo\"])])\n")},
		"sw/Sources/Demo/A.swift":    {Data: []byte("public func a() {}\n")},
		"sw/Tests/DemoTests/A.swift": {Data: []byte("import XCTest\n@testable import Demo\nfinal class A: XCTestCase { func testA() { a() } }\n")},
		".maestro/flow.yaml":         {Data: []byte("appId: demo\n")},
	}
	root := t.TempDir()
	for p, f := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, f.Data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, lang := range languages.All() {
		t.Run(lang.Name(), func(t *testing.T) {
			live, err := affected.Build(root, lang)
			if err != nil {
				t.Fatal(err)
			}
			immutable, err := affected.BuildFS(files, lang)
			if err != nil {
				t.Fatal(err)
			}
			if live.Digest() != immutable.Digest() {
				a, _ := live.Canonical()
				b, _ := immutable.Canonical()
				t.Fatalf("graph mismatch\n%s\n%s", a, b)
			}
			for p := range files {
				if !reflect.DeepEqual(affected.Select(live, []string{p}), affected.Select(immutable, []string{p})) {
					t.Fatalf("selection mismatch %s", p)
				}
			}
		})
	}
}

func TestImmutableNonregularBeforeLanguageFilter(t *testing.T) {
	t.Run("DLT-V0-002 nonregular entry refusal", testImmutableNonregularBeforeLanguageFilter)
}
func testImmutableNonregularBeforeLanguageFilter(t *testing.T) {
	for _, mode := range []fs.FileMode{fs.ModeSymlink, fs.ModeIrregular, fs.ModeNamedPipe} {
		files := fstest.MapFS{"extensionless": {Mode: mode, Data: []byte("elsewhere")}}
		if _, err := affected.BuildFS(files, languages.All()...); !errors.Is(err, affected.ErrWalkUnrepresentable) {
			t.Fatalf("mode %v: %v", mode, err)
		}
	}
}

func TestImmutableSwallowedReadRefusesGraph(t *testing.T) {
	t.Run("DLT-V0-002 swallowed read refuses graph", testImmutableSwallowedReadRefusesGraph)
}
func testImmutableSwallowedReadRefusesGraph(t *testing.T) {
	// Swift's optional Appium detector ignores its Read error. BuildFS must retain it.
	files := fstest.MapFS{"package.json": {Data: []byte(strings.Repeat("x", affected.MaxSourceBytes+1))}}
	for _, lang := range languages.All() {
		if lang.Name() == "swift" {
			if _, err := affected.BuildFS(files, lang); !errors.Is(err, affected.ErrWalkLimit) {
				t.Fatalf("swallowed bound: %v", err)
			}
		}
	}
}
