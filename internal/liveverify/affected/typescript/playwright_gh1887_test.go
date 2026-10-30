package typescript

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// TJAA-V0-123 (GitHub #1887, V1-2787): a top-level metadata member, including the documented
// run-identity form, is outside test membership. Discovery ignores it as it ignores project-level
// metadata, and config.sha256 still binds the config bytes that carry it.
func TestPlaywrightDiscoveryTopLevelMetadata_GH1887(t *testing.T) {
	cases := []grepListCase{
		{file: "plain.spec.ts", title: "plain", projects: []string{"main"}},
		{file: "tagged.spec.ts", title: "x @tagged", tags: []string{"tagged"}, projects: []string{"main"}},
		{file: "tagged.spec.ts", title: "y", projects: []string{"main"}},
	}
	digests := map[string]bool{}
	for _, metadata := range []string{
		"",
		"metadata: {}, ",
		"metadata: { corvintRunIdentity: corvintRunIdentity() }, ",
	} {
		root := grepFixture(t, "{ name: 'main' }")
		write(t, root, "corvint-run-identity.ts", "export function corvintRunIdentity() { return { profile: 'corvint-run-identity/0' } }\n")
		write(t, root, "playwright.config.ts", "import { corvintRunIdentity } from './corvint-run-identity';\nexport default defineConfig({ "+metadata+"testDir: 'e2e', projects: [{ name: 'main' }] });\n")
		raw, err := PlaywrightDiscoveryFromList(root, "playwright.config.ts", discoveryFixtureRevision, grepPlaywrightListing(t, root, "e2e", cases))
		if err != nil {
			t.Fatalf("%q: discovery refused: %v", metadata, err)
		}
		receipt := decodeGrepReceipt(t, raw)
		configBytes, err := os.ReadFile(filepath.Join(root, "playwright.config.ts"))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(configBytes)
		if receipt.Config.SHA256 != hex.EncodeToString(sum[:]) || len(receipt.Units) != 2 || len(receipt.UnverifiedUnits) != 0 {
			t.Fatalf("%q: receipt %+v", metadata, receipt)
		}
		digests[receipt.Config.SHA256] = true
		plan, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"e2e/plain.spec.ts"})
		if err != nil {
			t.Fatal(err)
		}
		for _, unknown := range plan.Unknown {
			if unknown.Reason == PlaywrightUnknownConfigSyntax {
				t.Fatalf("%q: top-level metadata raised %+v", metadata, unknown)
			}
		}
	}
	if len(digests) != 3 {
		t.Fatalf("config.sha256 does not bind the metadata bytes: %v", digests)
	}
}
