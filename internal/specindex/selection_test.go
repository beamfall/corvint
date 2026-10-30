package specindex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TestSRGV0045SpecMetadataStaysInSelectedScope pins the selection half of SRG-V0-045 (V1-0515):
// every spec-metadata input of TestIndexCoversSpecsAndHeaders lies in this package's declared read
// scope, through the predicate `corvint affected` applies to a read-scoped package, so a change
// that touches only INDEX.json, a spec's Agent digest or the README table selects this package.
func TestSRGV0045SpecMetadataStaysInSelectedScope(t *testing.T) {
	var declaration struct {
		Packages map[string][]string `json:"packages"`
	}
	if err := json.Unmarshal(mustRead(t, filepath.Join("../..", affected.ReadScopesPath)), &declaration); err != nil {
		t.Fatal(err)
	}
	const directory = "internal/specindex"
	scope, declared := declaration.Packages[directory]
	if !declared {
		t.Fatalf("%s declares no read scope for %s", affected.ReadScopesPath, directory)
	}
	var entries []entry
	if err := json.Unmarshal(mustRead(t, "../../docs/specs/INDEX.json"), &entries); err != nil {
		t.Fatal(err)
	}
	inputs := []string{"docs/specs/INDEX.json", "docs/specs/README.md", "docs/specs/REQUIREMENTS.tsv"}
	for _, e := range entries {
		inputs = append(inputs, e.Path)
	}
	for _, input := range inputs {
		if !affected.InReadScope(directory, scope, input) {
			t.Errorf("a change to %s does not select %s: it is outside the declared read scope %v", input, directory, scope)
		}
	}
	// The scope is not everything: a source path this package never reads stays out of it.
	if affected.InReadScope(directory, scope, "cmd/corvint/main.go") {
		t.Errorf("the declared read scope %v of %s reaches cmd/corvint/main.go", scope, directory)
	}
}

// TestSRGV0045IndexCheckWitnesses pins the check half of SRG-V0-045 on a fixture repository: the
// V1-0515 regression, an INDEX claim of 217 characters that the Agent digest and README do not
// carry, fails the selected check on all three counts, and the aligned change passes it.
func TestSRGV0045IndexCheckWitnesses(t *testing.T) {
	aligned := "The fixture specification keeps its index claim, Agent digest and README row in agreement."
	long := strings.TrimSuffix(strings.Repeat("A claim that grew past the index bound. ", 6), " ")[:216] + "."
	if len([]rune(long)) != 217 {
		t.Fatalf("regression claim has %d characters, want 217", len([]rune(long)))
	}
	for _, testCase := range []struct {
		name  string
		claim string
		want  []string
	}{
		{name: "mismatched", claim: long, want: []string{
			"claim length is 217, want 1..160 characters",
			"digest claim",
			"README first clause",
		}},
		{name: "aligned", claim: aligned},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			root := indexFixture(t, testCase.claim, aligned)
			var findings []string
			checkIndex(root, func(format string, args ...any) {
				findings = append(findings, fmt.Sprintf(format, args...))
			})
			if len(findings) != len(testCase.want) {
				t.Fatalf("findings %q, want %d matching %q", findings, len(testCase.want), testCase.want)
			}
			for i, want := range testCase.want {
				if !strings.Contains(findings[i], want) {
					t.Errorf("finding %d %q does not report %q", i, findings[i], want)
				}
			}
		})
	}
}

// indexFixture writes a one-spec repository whose INDEX entry carries indexClaim and whose Agent
// digest and README row carry specClaim.
func indexFixture(t *testing.T, indexClaim, specClaim string) string {
	t.Helper()
	root := t.TempDir()
	index, err := json.Marshal([]map[string]any{{
		"path": "docs/specs/fixture-v0.md", "title": "Fixture V0", "reqPrefix": "FIX-V0",
		"intent": "proposed", "delivery": "experimental", "claim": indexClaim,
		"dependsOn": []string{}, "supersededBy": nil, "closableToday": false, "implementation": []string{},
	}})
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"docs/specs/INDEX.json": string(index),
		"docs/specs/fixture-v0.md": "# Fixture V0\n\nIntent status: proposed\nDelivery status: experimental\n\n" +
			"## Agent digest\n- Status: proposed / experimental\n- Claim: " + specClaim +
			"\n- Exists: nothing\n- Blocked on: nothing\n- Read next: nothing\n",
		"docs/specs/README.md": "| Title | Contract | Intent | Delivery | Description |\n|---|---|---|---|---|\n" +
			"| Fixture V0 | [`fixture-v0.md`](fixture-v0.md) | proposed | experimental | " + specClaim + " |\n",
	}
	for name, body := range files {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
