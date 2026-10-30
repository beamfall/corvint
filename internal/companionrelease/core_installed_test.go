package companionrelease

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoreClosedStageRequiresExactNonNullableFields(t *testing.T) {
	stage := CoreInstalledStage{Name: "console", StdoutSHA256: sha256Hex(nil), StderrSHA256: sha256Hex(nil), Stdout: "", Stderr: ""}
	raw, _ := json.Marshal(stage)
	var decoded CoreInstalledStage
	if err := decodeCoreClosed(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []string{
		strings.Replace(string(raw), `"stdout":""`, `"stdout":null`, 1),
		strings.Replace(string(raw), `"stdout":""`, `"stdout":false`, 1),
		strings.Replace(string(raw), `"stdout":"",`, ``, 1),
		strings.Replace(string(raw), `"stdout":""`, `"stdout":"","stdout":""`, 1),
		strings.Replace(string(raw), `"stdout":""`, `"stdout":"","outputSha256":"legacy"`, 1),
	} {
		if err := decodeCoreClosed([]byte(mutation), &decoded); err == nil {
			t.Fatalf("accepted malformed stage: %s", mutation)
		}
	}
}

func TestCoreRequiredBooleanAndNativeFalseAreTyped(t *testing.T) {
	var value struct {
		Observed bool `json:"observed"`
	}
	if err := decodeCoreClosed([]byte(`{"observed":false}`), &value); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{`{"observed":null}`, `{"observed":"false"}`, `{}`} {
		if err := decodeCoreClosed([]byte(raw), &value); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	for _, raw := range []string{"null", `"false"`, "0", "", "true"} {
		if rawFalse(json.RawMessage(raw)) {
			t.Fatalf("native wrong scalar accepted as false: %q", raw)
		}
	}
	if !rawFalse(json.RawMessage(" false \n")) {
		t.Fatal("actual native false rejected")
	}
	// Native receipt:null is explicit; native original projection bytes retain their own nullable axes.
	var native struct {
		Receipt  *string         `json:"receipt" coreNullable:"true"`
		Document json.RawMessage `json:"document"`
	}
	if err := decodeCoreClosed([]byte(`{"receipt":null,"document":{"anchors":null}}`), &native); err != nil {
		t.Fatal(err)
	}
}

func TestCoreProfileSelectionPreservesOriginalLineBytes(t *testing.T) {
	original := []byte(" {\"profile\":\"corvint-core-console-proof/0\",\"x\":1} \n")
	got, err := selectCoreProfile(append([]byte("diagnostic\n"), original...), "corvint-core-console-proof/0")
	if err != nil || !bytes.Equal(got, original) {
		t.Fatalf("original bytes changed: %q %v", got, err)
	}
	if _, err := selectCoreProfile(append(original, original...), "corvint-core-console-proof/0"); err == nil {
		t.Fatal("duplicate profile accepted")
	}
}

func TestCoreReaderRejectsHistoricalAndMixedShapes(t *testing.T) {
	for _, raw := range []string{`{"profile":"corvint-public-release-installed/0"}`, `{"profile":"corvint-public-release-core-installed/0","editor":{}}`, `null`} {
		if err := ValidateCoreInstalledReport([]byte(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}

// Retained native originals can exercise the reader without rerunning providers.
// This is reader compatibility evidence, never installed qualification.
func TestCoreNativeWitnessReplay(t *testing.T) {
	root := os.Getenv("CORVINT_TEST_CORE_NATIVE_EVIDENCE")
	if root == "" {
		t.Skip("set CORVINT_TEST_CORE_NATIVE_EVIDENCE to retained native witness directories")
	}
	read := func(dir, name string) InstalledEvidence {
		raw, err := os.ReadFile(filepath.Join(root, dir, name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		return InstalledEvidence{Name: name, Raw: string(raw), SHA256: sha256Hex(raw)}
	}
	for _, kind := range []string{"js-unit", "js-e2e", "go"} {
		for _, step := range []struct{ name, state string }{{"initial", "PASSED"}, {"fail", "FAILED"}, {"fix", "PASSED"}} {
			t.Run(kind+"-"+step.name, func(t *testing.T) {
				if _, err := corePair(read("providers", kind+"-"+step.name+"-provider"), read("providers", kind+"-"+step.name+"-mcp"), step.state); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
	providers := map[string]InstalledEvidence{}
	for _, name := range coreProviderNames() {
		providers[name] = read("providers", name)
	}
	if err := validateCoreProviders(providers); err != nil {
		t.Fatal(err)
	}
	docs := map[string]InstalledEvidence{}
	for _, name := range coreDocsNames {
		docs[name] = read("docs", strings.TrimSuffix(name, ".json"))
	}
	if err := validateCoreDocs(docs); err != nil {
		t.Fatal(err)
	}
	bad := docs["mcp-draft.json"]
	bad.Raw = strings.Replace(bad.Raw, `"isError":false`, `"isError":null`, 1)
	docs["mcp-draft.json"] = bad
	if err := validateCoreDocs(docs); err == nil {
		t.Fatal("native isError:null accepted")
	}
	roadmap := map[string]InstalledEvidence{}
	for _, name := range coreRoadmapNames {
		roadmap[name] = read("planning", name)
	}
	if err := validateCoreRoadmap(roadmap); err != nil {
		t.Fatal(err)
	}
}
