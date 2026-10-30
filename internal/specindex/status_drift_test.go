package specindex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSRGV0047StatusDriftNamesBothFieldsWithLines pins the refusal for an Agent digest Status that
// was edited without the header statuses: it names the digest field and both header fields with
// their file:line and says they are edited together (V1-2867).
func TestSRGV0047StatusDriftNamesBothFieldsWithLines(t *testing.T) {
	claim := "The fixture specification keeps its index claim, Agent digest and README row in agreement."
	root := indexFixture(t, claim, claim)
	spec := filepath.Join(root, "docs", "specs", "fixture-v0.md")
	body, err := os.ReadFile(spec)
	if err != nil {
		t.Fatal(err)
	}
	drifted := strings.Replace(string(body), "- Status: proposed / experimental", "- Status: accepted / experimental", 1)
	if drifted == string(body) {
		t.Fatal("fixture has no digest Status line to drift")
	}
	if err := os.WriteFile(spec, []byte(drifted), 0o644); err != nil {
		t.Fatal(err)
	}
	var findings []string
	checkIndex(root, func(format string, args ...any) {
		findings = append(findings, fmt.Sprintf(format, args...))
	})
	want := `docs/specs/fixture-v0.md:7 Agent digest "- Status: accepted / experimental" does not agree with ` +
		`the header docs/specs/fixture-v0.md:3 "Intent status: proposed" and docs/specs/fixture-v0.md:4 ` +
		`"Delivery status: experimental"; the digest Status must contain both header values, so edit ` +
		`these fields together`
	if len(findings) != 1 || findings[0] != want {
		t.Fatalf("findings %q, want exactly [%q]", findings, want)
	}
}
