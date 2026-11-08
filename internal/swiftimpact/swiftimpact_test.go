package swiftimpact

import (
	"os/exec"
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

// TestNGIV0988TasksClosureExcludesPlannerAndImpactBinariesLinkReader: the
// corvint-tasks source archive closure (PUB-V0-012, PUB-V0-013) stays free of
// the test-selection planner, and every binary that already linked the Swift
// model links this reading, so their impact answers agree.
func TestNGIV0988TasksClosureExcludesPlannerAndImpactBinariesLinkReader(t *testing.T) {
	const module = "github.com/Beamfall/corvint/"
	linked := []string{"corvint", "corvint-corpus-mcp", "corvint-docs-mcp", "corvint-lsp", "corvint-mcp",
		"corvint-postmerge-workflow", "corvint-test", "corvint-test-validity-mcp", "corvint-web-flows"}
	args := []string{"list", "-f", "{{.ImportPath}} {{join .Deps \" \"}}", module + "cmd/corvint-tasks"}
	for _, name := range linked {
		args = append(args, module+"cmd/"+name)
	}
	out, err := exec.Command("go", args...).Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	deps := map[string]map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		set := map[string]bool{}
		for _, dep := range fields[1:] {
			set[dep] = true
		}
		deps[strings.TrimPrefix(fields[0], module+"cmd/")] = set
	}
	for _, forbidden := range []string{"internal/liveverify/affected", "internal/swiftimpact"} {
		if deps["corvint-tasks"][module+forbidden] {
			t.Errorf("corvint-tasks links %s", forbidden)
		}
	}
	for _, name := range linked {
		if !deps[name][module+"internal/swiftimpact"] {
			t.Errorf("%s does not link internal/swiftimpact", name)
		}
	}
}
