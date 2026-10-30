package extevidence

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// lspRecord fills the gopls conformance fixture for one repository.
func lspRecord(t *testing.T, r repository) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", pathFixtures, "lsp-gopls.json"))
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("git", "rev-parse", "HEAD:pkg/main_test.go")
	command.Dir = r.root
	testBlob, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{"APP_ORIGIN": r.first, "APP_REVISION": r.head, "MAIN_BLOB": r.mainBlob, "TEST_BLOB": strings.TrimSpace(string(testBlob))}
	for key, value := range values {
		data = bytes.ReplaceAll(data, []byte("{{"+key+"}}"), []byte(value))
	}
	return data
}

func inlineSection(t *testing.T, r repository, data []byte, reason string) map[string]any {
	t.Helper()
	out, err := contextindex.CanonicalJSON(InlineSection(context.Background(), r.index(), "lsp:gopls", data, reason, []string{"pkg/main.go"}, 20))
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

// TestLSPRecordConformance: the in-process gopls record takes the V2 decode
// and verification every transport shares, its rows stay external-provider
// evidence naming their hop origin, and the same bytes from a file give the
// same path relations (EEP-V0-023, EEP-V0-025).
func TestLSPRecordConformance(t *testing.T) {
	t.Parallel()
	r := newRepository(t)
	data := lspRecord(t, r)
	if record, err := Decode1(data); err != nil || record.Schema != Schema2 {
		t.Fatalf("the gopls fixture must decode as V2: %v", err)
	}
	section := inlineSection(t, r, data, "")
	row := section["providers"].([]any)[0].(map[string]any)
	if row["state"] != StateLoaded || row["id"] != "gopls" || row["revision"] != "v0.22.0" || row["source"] != "lsp:gopls" {
		t.Fatalf("provider row = %v", row)
	}
	items := section["path_relations"].([]any)
	if len(items) != 2 || len(section["results"].([]any)) != 0 {
		t.Fatalf("two path relations and no results: %v", section)
	}
	for _, raw := range items {
		item := raw.(map[string]any)
		if contextindex.TrustClass(item["authority"].(string)) != contextindex.TrustExternalProvider {
			t.Errorf("an LSP row is external-provider evidence: %v", item["authority"])
		}
		if item["relation_state"] != RelationFresh || !strings.Contains(item["relation"].(map[string]any)["reference"].(string), "from seed pkg/main.go") {
			t.Errorf("a fresh row naming its hop origin: %v", item)
		}
	}
	filed := section1(t, pair{app: r}, data, nil, "pkg/main.go")
	if !bytes.Equal(canonical(t, filed["path_relations"]), canonical(t, section["path_relations"])) {
		t.Fatal("the same record bytes must give the same path relations whatever carried them")
	}
}

// TestLSPUnavailableIsVisible: no record is one unavailable provider row
// with its reason and no path relations (EEP-V0-026).
func TestLSPUnavailableIsVisible(t *testing.T) {
	t.Parallel()
	r := newRepository(t)
	section := inlineSection(t, r, nil, "gopls executable not found")
	row := section["providers"].([]any)[0].(map[string]any)
	if row["state"] != StateUnavailable || row["reason"] != "gopls executable not found" {
		t.Fatalf("provider row = %v", row)
	}
	if _, present := section["path_relations"]; present {
		t.Fatal("an unavailable provider adds no path relations")
	}
}

func canonical(t *testing.T, value any) []byte {
	t.Helper()
	out, err := contextindex.CanonicalJSON(value)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
