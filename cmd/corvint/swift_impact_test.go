// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// LGP-V0-8600, NGI-V0-980: exercise startup wiring, not an index with a
// reader supplied by the test. Both CLI computation paths must retain SwiftPM.
func TestNativeCLISwiftReverseImpact(t *testing.T) {
	root := cliRepository(t)
	for path, body := range map[string]string{
		"Package.swift": `// swift-tools-version:5.9
import PackageDescription
let package = Package(name: "Demo", targets: [
    .target(name: "Core"),
    .target(name: "App", dependencies: ["Core"]),
])
`,
		"Sources/Core/Ledger.swift": "public struct Ledger {}\n",
		"Sources/App/main.swift":    "import Core\nlet ledger = Ledger()\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "Swift fixture")
	for _, profile := range []string{"default", "syntax"} {
		t.Run(profile, func(t *testing.T) {
			args := []string{"--root", root, "impact", "Sources/Core/Ledger.swift", "--limit", "10"}
			if profile == "syntax" {
				args = append(args, "--language-profile", "non-go-syntax-v0")
			}
			exit, stdout, stderr := runCLI(t, args...)
			if exit != 0 || stderr != "" {
				t.Fatalf("exit=%d stderr=%s stdout=%s", exit, stderr, stdout)
			}
			var receipt struct {
				Context struct{ Results []struct{ Kind, ID string } }
			}
			if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, row := range receipt.Context.Results {
				found = found || row.Kind == "reverse-import" && row.ID == "Sources/App/main.swift"
			}
			if !found || strings.Contains(stdout, "not linked") {
				t.Fatalf("CLI lost SwiftPM reverse import: %s", stdout)
			}
		})
	}
}
