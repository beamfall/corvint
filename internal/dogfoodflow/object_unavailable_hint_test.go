package dogfoodflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDCWV0940ObjectUnavailableFixNamesObjectAndRemedy pins DCW-V0-940
// (V1-2445): a repository-object-unavailable row's fix quotes the step's own
// refusal, which names the object, and the fetch remedy; with no message it
// keeps the remedy and points at the stderr file.
func TestDCWV0940ObjectUnavailableFixNamesObjectAndRemedy(t *testing.T) {
	evidence := t.TempDir()
	const base = "503d4ea3aa2520a9c26dc4d37e2da62401387696"
	refusal := `{"code":"repository-object-unavailable","error":"repository-object-unavailable: repository object unavailable: docs/specs/x-v0.md at 7c1a864a is not the regular blob 1234abcd"}` + "\n"
	if err := os.WriteFile(filepath.Join(evidence, "ocm-prepare-001.stderr"), []byte(refusal), 0o600); err != nil {
		t.Fatal(err)
	}
	c := &change{evidence: evidence, base: base}
	got := c.fixHint(step{name: "ocm-prepare-001", reason: "repository-object-unavailable"})
	for _, want := range []string{
		`ocm-prepare-001 could not read a Git object it needs: "repository-object-unavailable: repository object unavailable: docs/specs/x-v0.md at 7c1a864a is not the regular blob 1234abcd"`,
		"git fetch origin " + base + " when BASE is not local",
		"git fetch --unshallow",
		"then rerun corvint dogfood change " + base,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("fix %q lacks %q", got, want)
		}
	}
	if err := os.WriteFile(filepath.Join(evidence, "cem-status.stderr"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	got = c.fixHint(step{name: "cem-status", reason: "repository-object-unavailable"})
	if !strings.Contains(got, "its refusal in "+evidence+"/cem-status.stderr names the missing object") || !strings.Contains(got, "git fetch origin") {
		t.Errorf("empty stderr fix %q", got)
	}
}
