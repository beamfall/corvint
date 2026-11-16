// SPDX-License-Identifier: Apache-2.0

package bridge

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/swiftimpact"
)

// LGP-V0-8600, NGI-V0-980: both registry constructors carry their explicit
// reader into native Swift impact. MCP wire admission remains separately frozen.
func TestSwiftImpactThroughProfileConstructors(t *testing.T) {
	root := makeRepository(t)
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
	gitOutput(t, root, "add", ".")
	gitOutput(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "Swift fixture")
	var baseline map[string]any
	constructors := map[string]func(string, contextindex.SwiftPMReader) (*Registry, error){
		"default": NewWithSwiftPMReader,
		"flows":   NewFlowsWithSwiftPMReader,
	}
	profiles := []string{"default", "flows"}
	for _, profile := range profiles {
		t.Run("profile="+profile, func(t *testing.T) {
			registry, err := constructors[profile](root, swiftimpact.Reader{})
			if err != nil {
				t.Fatal(err)
			}
			snapshot, snapshotErr := registry.snapshot(context.Background())
			if snapshotErr != nil {
				t.Fatal(snapshotErr)
			}
			result, err := contextindex.Impact(snapshot.index.WithSwiftPMReader(registry.swiftPMReader), []string{"Sources/Core/Ledger.swift"}, 10)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			var receipt struct {
				Results []struct{ Kind, ID string }
			}
			if err := json.Unmarshal(raw, &receipt); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, row := range receipt.Results {
				found = found || row.Kind == "reverse-import" && row.ID == "Sources/App/main.swift"
			}
			if !found || strings.Contains(string(raw), "not linked") {
				t.Fatalf("Swift reader missing: %s", raw)
			}
			if profile == "default" {
				baseline = result
			} else if !reflect.DeepEqual(baseline, result) {
				t.Fatalf("profile changed impact receipt: %s", raw)
			}
		})
	}
}
