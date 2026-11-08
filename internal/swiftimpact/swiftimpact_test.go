package swiftimpact

import (
	"strings"
	"testing"
	"testing/fstest"
)

// TestNGIV0988ReaderReadsSwiftPMTargetsAndImports: the registered reading
// lists only SwiftPM targets, sources then tests, and code-only imports.
func TestNGIV0988ReaderReadsSwiftPMTargetsAndImports(t *testing.T) {
	fsys := fstest.MapFS{
		"Package.swift":                     {Data: []byte("// swift-tools-version:5.9\nimport PackageDescription\n\nlet package = Package(\n    name: \"Demo\",\n    targets: [\n        .target(name: \"Core\"),\n        .testTarget(name: \"CoreTests\", dependencies: [\"Core\"]),\n    ]\n)\n")},
		"Sources/Core/Ledger.swift":         {Data: []byte("public struct Ledger {}\n")},
		"Tests/CoreTests/LedgerTests.swift": {Data: []byte("import XCTest\n@testable import Core\n\nfinal class LedgerTests: XCTestCase {}\n")},
	}
	targets, err := Reader{}.Targets(fsys)
	if err != nil {
		t.Fatal(err)
	}
	modules := map[string][]string{}
	for _, target := range targets {
		if !strings.HasPrefix(target.ID, "swift:spm:") {
			t.Fatalf("non-SwiftPM unit %q", target.ID)
		}
		modules[target.Module] = target.Members
	}
	if got := modules["Core"]; len(got) != 1 || got[0] != "Sources/Core/Ledger.swift" {
		t.Fatalf("Core members = %v", got)
	}
	if got := modules["CoreTests"]; len(got) != 1 || got[0] != "Tests/CoreTests/LedgerTests.swift" {
		t.Fatalf("CoreTests members = %v", got)
	}
	imports := Reader{}.ImportedModules("// import Hidden\nimport XCTest\n@testable import Core\n")
	if len(imports) != 2 || imports[0].Module != "XCTest" || imports[0].Line != 2 || imports[1].Module != "Core" || imports[1].Line != 3 {
		t.Fatalf("imports = %+v", imports)
	}
}
