package swift

import (
	"context"
	"slices"
	"testing"
)

// TestCrossRepositoryPackageIsANamedFrontier_SATSV0761 (V1-2055): a SwiftPM
// dependency on a working copy outside the root, or a revision pin, is the
// named cross-repository frontier carrying its path or url@revision; a local
// package inside the root and a version requirement are not.
func TestCrossRepositoryPackageIsANamedFrontier_SATSV0761(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Package.swift", `// swift-tools-version:5.9
import PackageDescription
let package = Package(
    name: "App",
    dependencies: [
        .package(path: "../beamfall-apple-ui"),
        .package(url: "https://example.test/ui.git", revision: "6e379d7feec8"),
        .package(url: "https://example.test/log.git", from: "1.0.0"),
        .package(path: "Packages/Local"),
    ],
    targets: [
        .target(name: "App"),
        .testTarget(name: "AppTests", dependencies: ["App"]),
    ]
)
`)
	write(t, root, "Sources/App/App.swift", "public struct App {}\n")
	write(t, root, "Tests/AppTests/AppTests.swift", "import XCTest\n@testable import App\nfinal class AppTests: XCTestCase { func testApp() {} }\n")
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		FrontierCrossRepositoryPackage + ":../beamfall-apple-ui",
		FrontierCrossRepositoryPackage + ":https://example.test/ui.git@6e379d7feec8",
	} {
		if !slices.Contains(result.Frontier, want) {
			t.Errorf("frontier %v lacks %s", result.Frontier, want)
		}
	}
	for _, unwanted := range []string{FrontierCrossRepositoryPackage + ":Packages/Local", FrontierCrossRepositoryPackage + ":https://example.test/log.git@1.0.0"} {
		if slices.Contains(result.Frontier, unwanted) {
			t.Errorf("frontier %v names %s", result.Frontier, unwanted)
		}
	}
}
